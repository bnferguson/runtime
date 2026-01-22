package autotls

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/acme/autocert"
)

// RouteWatcher provides access to the set of hosts with configured routes.
// Implementations should watch for route changes and keep the set current.
type RouteWatcher interface {
	// HasRoute returns true if a route exists for the given host.
	// This should be a fast in-memory lookup.
	HasRoute(host string) bool
}

// RouteSet is a thread-safe set of hosts with configured routes.
// It implements RouteWatcher and can be updated as routes change.
type RouteSet struct {
	mu    sync.RWMutex
	hosts map[string]struct{}
}

// NewRouteSet creates a new empty RouteSet.
func NewRouteSet() *RouteSet {
	return &RouteSet{
		hosts: make(map[string]struct{}),
	}
}

// HasRoute returns true if the host has a configured route.
func (rs *RouteSet) HasRoute(host string) bool {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	_, ok := rs.hosts[host]
	return ok
}

// Add adds a host to the route set.
func (rs *RouteSet) Add(host string) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.hosts[host] = struct{}{}
}

// Remove removes a host from the route set.
func (rs *RouteSet) Remove(host string) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	delete(rs.hosts, host)
}

// Replace replaces all hosts in the set with the provided list.
func (rs *RouteSet) Replace(hosts []string) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.hosts = make(map[string]struct{}, len(hosts))
	for _, h := range hosts {
		rs.hosts[h] = struct{}{}
	}
}

func ServeTLS(ctx context.Context, log *slog.Logger, dataPath string, email string, routeWatcher RouteWatcher, h http.Handler) error {
	log = log.With("module", "autotls")

	// Load or generate a self-signed fallback cert for hosts without configured routes.
	// This allows default routes and unconfigured hosts to still work over HTTPS
	// (with a browser warning) while only provisioning real ACME certs for
	// explicitly configured domains. The cert is persisted to disk so users
	// who accept the browser warning don't have to re-accept on every restart.
	certsDir := filepath.Join(dataPath, "certs")
	fallbackCert, err := loadOrGenerateFallbackCert(certsDir)
	if err != nil {
		return err
	}
	log.Info("loaded fallback self-signed certificate for unconfigured hosts")

	mgr := &autocert.Manager{
		Prompt: autocert.AcceptTOS,
		Cache:  autocert.DirCache(certsDir),
		Email:  email,
	}

	// Custom GetCertificate that falls back to self-signed for unconfigured hosts
	getCertificate := func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		host := strings.ToLower(hello.ServerName)

		if routeWatcher.HasRoute(host) {
			log.Debug("using ACME cert for configured host", "host", host)
			return mgr.GetCertificate(hello)
		}

		log.Debug("using self-signed fallback for unconfigured host", "host", host)
		return &fallbackCert, nil
	}

	tlsConfig := &tls.Config{
		GetCertificate: getCertificate,
		NextProtos:     []string{"h2", "http/1.1", "acme-tls/1"},
		MinVersion:     tls.VersionTLS12,
	}

	log.Info("serving TLS with autocert (self-signed fallback for unconfigured hosts)")

	server := &http.Server{
		Addr:      ":443",
		Handler:   h,
		TLSConfig: tlsConfig,
	}

	go func() {
		err := server.ListenAndServeTLS("", "")
		if err != nil && err != http.ErrServerClosed {
			log.Error("error serving TLS", "error", err)
		}
	}()

	// Create a handler that redirects to HTTPS unless the host is localhost or an IP address
	redirectHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		// Strip port if present
		if hostWithoutPort, _, err := net.SplitHostPort(host); err == nil {
			host = hostWithoutPort
		}

		// Check if host is localhost or an IP address
		isLocalhost := host == "localhost" || host == "127.0.0.1" || host == "::1"
		isIPAddress := net.ParseIP(host) != nil

		if isLocalhost || isIPAddress {
			// Serve the request directly without redirecting
			h.ServeHTTP(w, r)
			return
		}

		// Redirect to HTTPS
		if r.Method != "GET" && r.Method != "HEAD" {
			http.Error(w, "Use HTTPS", http.StatusBadRequest)
			return
		}

		target := "https://" + host + r.URL.RequestURI()
		http.Redirect(w, r, target, http.StatusFound)
	})

	// Start HTTP server on port 80 for ACME challenges and HTTP to HTTPS redirect
	httpServer := &http.Server{
		Addr:              ":80",
		Handler:           mgr.HTTPHandler(redirectHandler),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	go func() {
		log.Info("starting HTTP server for ACME challenges and HTTPS redirect", "addr", ":80")
		err := httpServer.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Error("error serving HTTP", "error", err)
		}
	}()

	// Monitor for context cancellation and gracefully shutdown both servers
	go func() {
		<-ctx.Done()
		log.Info("shutting down TLS and HTTP servers")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Shutdown both servers
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Error("TLS server shutdown error", "error", err)
		}
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Error("HTTP server shutdown error", "error", err)
		}
		log.Info("TLS and HTTP servers shutdown complete")
	}()

	return nil
}

// CertificateProvider provides certificates via GetCertificate callback
type CertificateProvider interface {
	GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error)
}

// ServeTLSWithController serves HTTPS using certificates provided by a controller
func ServeTLSWithController(ctx context.Context, log *slog.Logger, certProvider CertificateProvider, h http.Handler) error {
	log = log.With("module", "autotls", "mode", "controller")
	log.Info("serving TLS with certificate controller")

	tlsConfig := &tls.Config{
		GetCertificate: certProvider.GetCertificate,
		MinVersion:     tls.VersionTLS12,
	}

	server := &http.Server{
		Addr:      ":443",
		Handler:   h,
		TLSConfig: tlsConfig,
	}

	go func() {
		log.Info("starting HTTPS server", "addr", ":443")
		err := server.ListenAndServeTLS("", "")
		if err != nil && err != http.ErrServerClosed {
			log.Error("error serving HTTPS", "error", err)
		}
	}()

	// Monitor for context cancellation
	go func() {
		<-ctx.Done()
		log.Info("shutting down HTTPS server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Error("HTTPS server shutdown error", "error", err)
		}
		log.Info("HTTPS server shutdown complete")
	}()

	return nil
}
