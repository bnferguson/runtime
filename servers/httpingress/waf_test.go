package httpingress

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"miren.dev/runtime/api/ingress/ingress_v1alpha"
	"miren.dev/runtime/pkg/entity"
	"miren.dev/runtime/pkg/waf"
)

func newTestWAFServer() *Server {
	return &Server{
		Log:             slog.Default(),
		wafEngine:       waf.NewEngine(slog.Default()),
		wafProfileCache: make(map[entity.Id]*wafProfileEntry),
	}
}

func newTestRoute(profileID entity.Id, level int, s *Server) *ingress_v1alpha.HttpRoute {
	if level > 0 {
		s.wafProfileCache[profileID] = &wafProfileEntry{
			paranoiaLevel: level,
			fetchedAt:     time.Now(),
		}
	}
	return &ingress_v1alpha.HttpRoute{WafProfile: profileID}
}

func TestWafMiddlewareDisabledWhenNoProfile(t *testing.T) {
	s := newTestWAFServer()

	route := &ingress_v1alpha.HttpRoute{}
	called := false
	next := func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}

	handler := s.wafMiddleware(route, next)

	req := httptest.NewRequest("GET", "http://example.com/?id=1%20OR%201=1--", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	assert.True(t, called, "next handler should be called when WAF is disabled")
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestWafMiddlewareBlocksSQLInjection(t *testing.T) {
	s := newTestWAFServer()

	route := newTestRoute("waf-l1", 1, s)
	called := false
	next := func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}

	handler := s.wafMiddleware(route, next)

	req := httptest.NewRequest("GET", "http://example.com/?id=1%20OR%201=1--", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	assert.False(t, called, "next handler should not be called when WAF blocks")
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestWafMiddlewareBlocksXSS(t *testing.T) {
	s := newTestWAFServer()

	route := newTestRoute("waf-l1", 1, s)
	next := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}

	handler := s.wafMiddleware(route, next)

	req := httptest.NewRequest("GET", "http://example.com/?q=%3Cscript%3Ealert(1)%3C/script%3E", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestWafMiddlewareAllowsCleanRequest(t *testing.T) {
	s := newTestWAFServer()

	route := newTestRoute("waf-l1", 1, s)
	next := func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}

	handler := s.wafMiddleware(route, next)

	req := httptest.NewRequest("GET", "http://example.com/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "ok", rec.Body.String())
}

func TestWafMiddlewareRespectsParanoiaLevel(t *testing.T) {
	s := newTestWAFServer()

	for _, level := range []int{1, 2, 3, 4} {
		t.Run("level"+string(rune('0'+level)), func(t *testing.T) {
			profileID := entity.Id("waf-l" + string(rune('0'+level)))
			route := newTestRoute(profileID, level, s)
			next := func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}

			handler := s.wafMiddleware(route, next)

			req := httptest.NewRequest("GET", "http://example.com/", nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusOK, rec.Code)

			req = httptest.NewRequest("GET", "http://example.com/?id=1%20OR%201=1--", nil)
			rec = httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusForbidden, rec.Code)
		})
	}
}

func TestWafMiddlewareZeroLevelProfile(t *testing.T) {
	s := newTestWAFServer()

	profileID := entity.Id("waf-zero")
	s.wafProfileCache[profileID] = &wafProfileEntry{
		paranoiaLevel: 0,
		fetchedAt:     time.Now(),
	}

	route := &ingress_v1alpha.HttpRoute{WafProfile: profileID}
	called := false
	next := func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}

	handler := s.wafMiddleware(route, next)

	req := httptest.NewRequest("GET", "http://example.com/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	assert.True(t, called, "zero-level profile should fail open")
}

func TestWafEngineInitialized(t *testing.T) {
	s := newTestWAFServer()
	require.NotNil(t, s.wafEngine)
}
