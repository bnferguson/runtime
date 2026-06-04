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
	"strconv"
	"strings"
	"sync"
	"time"

	"miren.dev/runtime/pkg/workloadidentity"
)

const tokenServerPort = 7123

type tokenResponse struct {
	Value string `json:"value"`
}

type tokenErrorResponse struct {
	Error string `json:"error"`
}

type tokenSecretRegistry struct {
	mu        sync.RWMutex
	byAddr    map[string]string // IP → secret
	bySandbox map[string]string // sandboxID → IP (for cleanup)
}

func newTokenSecretRegistry() *tokenSecretRegistry {
	return &tokenSecretRegistry{
		byAddr:    make(map[string]string),
		bySandbox: make(map[string]string),
	}
}

func (r *tokenSecretRegistry) register(ip, sandboxID, secret string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byAddr[ip] = secret
	r.bySandbox[sandboxID] = ip
}

func (r *tokenSecretRegistry) unregister(sandboxID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if ip, ok := r.bySandbox[sandboxID]; ok {
		delete(r.byAddr, ip)
		delete(r.bySandbox, sandboxID)
	}
}

func (r *tokenSecretRegistry) verify(ip, secret string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	expected, ok := r.byAddr[ip]
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

	if c.tokenSecrets == nil || !c.tokenSecrets.verify(remoteHost, bearerToken) {
		writeTokenError(w, http.StatusForbidden, "invalid token")
		return
	}

	sandboxID, appName, ok := c.NetServ.LookupSandboxByIP(remoteHost)
	if !ok {
		writeTokenError(w, http.StatusForbidden, "unknown source address")
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
