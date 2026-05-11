package waf

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/corazawaf/coraza/v3"
	corazahttp "github.com/corazawaf/coraza/v3/http"
	"github.com/corazawaf/coraza/v3/types"

	coreruleset "github.com/corazawaf/coraza-coreruleset/v4"
)

const maxParanoiaLevel = 4

type Engine struct {
	log *slog.Logger

	mu      sync.RWMutex
	engines map[int]coraza.WAF
}

func NewEngine(log *slog.Logger) *Engine {
	return &Engine{
		log:     log,
		engines: make(map[int]coraza.WAF),
	}
}

func (e *Engine) getOrCreate(level int) (coraza.WAF, error) {
	e.mu.RLock()
	w, ok := e.engines[level]
	e.mu.RUnlock()
	if ok {
		return w, nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if w, ok := e.engines[level]; ok {
		return w, nil
	}

	w, err := createWAF(level, e.log)
	if err != nil {
		return nil, err
	}

	e.engines[level] = w
	return w, nil
}

func createWAF(paranoiaLevel int, log *slog.Logger) (coraza.WAF, error) {
	if paranoiaLevel < 1 || paranoiaLevel > maxParanoiaLevel {
		return nil, fmt.Errorf("paranoia level must be between 1 and %d, got %d", maxParanoiaLevel, paranoiaLevel)
	}

	directives := fmt.Sprintf(`
SecRuleEngine On
SecRequestBodyAccess On
SecRequestBodyLimit 10485760
SecRequestBodyInMemoryLimit 131072
SecRequestBodyLimitAction Reject
SecResponseBodyAccess Off

SecAction "id:900000,phase:1,pass,t:none,nolog,setvar:tx.blocking_paranoia_level=%d"
SecAction "id:900990,phase:1,pass,t:none,nolog,setvar:tx.crs_setup_version=4250"

Include @owasp_crs/*.conf
`, paranoiaLevel)

	cfg := coraza.NewWAFConfig().
		WithDirectives(directives).
		WithRootFS(coreruleset.FS).
		WithErrorCallback(func(mr types.MatchedRule) {
			path, _, _ := strings.Cut(mr.URI(), "?")
			log.Warn("WAF rule matched",
				"id", mr.Rule().ID(),
				"msg", mr.Message(),
				"severity", mr.Rule().Severity().String(),
				"path", path,
			)
		})

	return coraza.NewWAF(cfg)
}

func (e *Engine) Handler(level int, next http.Handler) (http.Handler, error) {
	w, err := e.getOrCreate(level)
	if err != nil {
		return nil, err
	}

	return corazahttp.WrapHandler(w, next), nil
}
