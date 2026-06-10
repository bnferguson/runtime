package sandbox

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"miren.dev/runtime/pkg/workloadidentity"
)

const tokenServerPort = 7123

// tokenSecretFilename is the host-side file (under the sandbox's data dir) where a
// sandbox's token-request secret is persisted so it can be re-registered with the
// in-memory tokenSecretRegistry after a controller/token-server restart.
const tokenSecretFilename = "token-secret"

type tokenResponse struct {
	Value string `json:"value"`
}

type tokenErrorResponse struct {
	Error string `json:"error"`
}

// tokenSecretRegistry maps a sandbox's identity to its token-request secret. Keying by
// sandbox identity (rather than raw source IP) means a recycled pod IP can never match a
// stale secret left behind by a previous sandbox: the caller's identity is resolved from
// the IP via the authoritative netdb lookup, and the secret is checked against that.
type tokenSecretRegistry struct {
	mu        sync.RWMutex
	bySandbox map[string]string // sandboxID → secret
}

func newTokenSecretRegistry() *tokenSecretRegistry {
	return &tokenSecretRegistry{
		bySandbox: make(map[string]string),
	}
}

func (r *tokenSecretRegistry) register(sandboxID, secret string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bySandbox[sandboxID] = secret
}

func (r *tokenSecretRegistry) unregister(sandboxID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.bySandbox, sandboxID)
}

func (r *tokenSecretRegistry) verify(sandboxID, secret string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	expected, ok := r.bySandbox[sandboxID]
	if !ok {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(secret)) == 1
}

func generateTokenSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// writeTokenSecret persists a sandbox's token-request secret host-side at 0600. It is
// never bind-mounted into the container (the container receives the secret via the
// MIREN_IDENTITY_TOKEN_SECRET env var); persisting it lets the controller re-register
// the same secret after a restart so the still-running sandbox keeps authenticating.
func writeTokenSecret(path, secret string) error {
	return atomicWriteFile(path, []byte(secret), 0600)
}

// loadTokenSecret reads a persisted token-request secret. It returns ok=false (with a nil
// error) when no secret file exists — e.g. a sandbox started before secret persistence was
// added — so callers can skip re-registration without treating absence as a failure.
func loadTokenSecret(path string) (secret string, ok bool, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	// Tolerate a trailing newline so a secret written by a text editor or
	// fmt.Fprintln still matches the in-process env value.
	return strings.TrimRight(string(data), "\r\n"), true, nil
}

func (c *SandboxController) startTokenServer(ctx context.Context) {
	listenAddr := fmt.Sprintf("%s:%d", c.Subnet.Router().Addr(), tokenServerPort)

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/token", c.handleTokenRequest)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeTokenError(w, http.StatusNotFound, "not found")
	})

	server := &http.Server{
		Addr:              listenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		BaseContext: func(_ net.Listener) context.Context {
			return ctx
		},
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)
	}()

	c.Log.Info("starting workload identity token server", "addr", listenAddr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		c.Log.Error("token server failed", "error", err)
	}
}

func (c *SandboxController) handleTokenRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeTokenError(w, http.StatusMethodNotAllowed, "only GET is allowed")
		return
	}

	remoteHost, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		writeTokenError(w, http.StatusBadRequest, "invalid remote address")
		return
	}

	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		writeTokenError(w, http.StatusUnauthorized, "missing or invalid Authorization header")
		return
	}
	bearerToken := strings.TrimPrefix(authHeader, "Bearer ")

	sandboxID, appName, ok := c.NetServ.LookupSandboxByIP(remoteHost)
	if !ok {
		writeTokenError(w, http.StatusForbidden, "unknown source address")
		return
	}

	if c.tokenSecrets == nil || !c.tokenSecrets.verify(sandboxID, bearerToken) {
		writeTokenError(w, http.StatusForbidden, "invalid token")
		return
	}

	opts := workloadidentity.TokenOptions{}

	if auds := r.URL.Query()["audience"]; len(auds) > 0 {
		opts.Audience = auds
	}

	if ttlStr := r.URL.Query().Get("ttl"); ttlStr != "" {
		ttlSec, err := strconv.Atoi(ttlStr)
		if err != nil || ttlSec <= 0 {
			writeTokenError(w, http.StatusBadRequest, "ttl must be a positive integer (seconds)")
			return
		}
		opts.TTL = time.Duration(ttlSec) * time.Second
	}

	token, err := c.WorkloadIssuer.IssueTokenWithOptions(appName, sandboxID, opts)
	if err != nil {
		c.Log.Error("failed to issue token", "sandbox", sandboxID, "error", err)
		writeTokenError(w, http.StatusInternalServerError, "failed to issue token")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tokenResponse{
		Value: token,
	})
}

func writeTokenError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(tokenErrorResponse{Error: msg})
}
