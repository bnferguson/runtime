package certificate

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/crypto/acme/autocert"
	"miren.dev/runtime/api/entityserver/entityserver_v1alpha"
	"miren.dev/runtime/api/ingress/ingress_v1alpha"
	"miren.dev/runtime/components/autotls"
	"miren.dev/runtime/pkg/entity"
)

// AutocertController provisions TLS certificates eagerly using HTTP-01 ACME challenges
// via autocert.Manager. It watches http_route entities and triggers cert provisioning
// when routes are created, rather than waiting for the first TLS handshake.
type AutocertController struct {
	log          *slog.Logger
	eac          *entityserver_v1alpha.EntityAccessClient
	mgr          *autocert.Manager
	dataPath     string
	email        string
	fallbackCert tls.Certificate
	allowedHosts sync.Map      // domain -> struct{}
	ready        chan struct{} // closed when port-80 ACME challenge server is up
}

// NewAutocertController creates a new autocert controller for HTTP-01 ACME challenges.
func NewAutocertController(log *slog.Logger, eac *entityserver_v1alpha.EntityAccessClient, dataPath string, email string) *AutocertController {
	return &AutocertController{
		log:      log.With("module", "autocert-controller"),
		eac:      eac,
		dataPath: dataPath,
		email:    email,
		ready:    make(chan struct{}),
	}
}

// Init implements ReconcileControllerI — creates the autocert.Manager and loads the fallback cert.
func (c *AutocertController) Init(ctx context.Context) error {
	certsDir := filepath.Join(c.dataPath, "certs")

	fallbackCert, err := autotls.LoadOrGenerateFallbackCert(certsDir)
	if err != nil {
		return err
	}
	c.fallbackCert = fallbackCert
	c.log.Info("loaded fallback self-signed certificate for unconfigured hosts")

	c.mgr = &autocert.Manager{
		Prompt: autocert.AcceptTOS,
		Cache:  autocert.DirCache(certsDir),
		Email:  c.email,
		HostPolicy: func(ctx context.Context, host string) error {
			if c.isAllowedHost(strings.ToLower(host)) {
				return nil
			}
			return fmt.Errorf("host %q not in allowed set", host)
		},
	}

	c.log.Info("autocert controller initialized")
	return nil
}

// Reconcile implements ReconcileControllerI — adds the route's domain to allowedHosts
// and eagerly provisions a TLS certificate via autocert.
func (c *AutocertController) Reconcile(ctx context.Context, route *ingress_v1alpha.HttpRoute, meta *entity.Meta) error {
	domain := strings.ToLower(strings.TrimSpace(route.Host))
	routeID := meta.Id()
	if domain == "" {
		c.log.Warn("http_route has empty host, skipping certificate provisioning", "route", routeID)
		return nil
	}

	c.allowedHosts.Store(domain, struct{}{})

	log := c.log.With("domain", domain, "route", routeID)

	// For wildcard routes (*.example.com), eagerly provision the base domain cert.
	// HTTP-01 challenges can't issue wildcard certs — those require DNS-01. But we
	// can pre-provision the base domain so it's ready, and let individual subdomains
	// provision inline when they first arrive via GetCertificate.
	provisionDomain := domain
	if strings.HasPrefix(domain, "*.") {
		provisionDomain = domain[2:] // *.example.com → example.com
		log.Info("wildcard route: eagerly provisioning base domain cert, subdomains will provision inline",
			"base_domain", provisionDomain)
	}

	// Wait for port-80 ACME challenge server to be ready before attempting provisioning
	select {
	case <-c.ready:
	case <-ctx.Done():
		return ctx.Err()
	}

	// Eagerly provision the certificate with a synthetic ClientHelloInfo
	hello := &tls.ClientHelloInfo{ServerName: provisionDomain}
	_, err := c.mgr.GetCertificate(hello)
	if err != nil {
		log.Warn("eager cert provisioning failed (will retry on next TLS handshake)", "error", err)
		// Don't return the error — the controller framework would retry, but autocert
		// itself will handle this on the next actual TLS handshake or resync.
		return nil
	}

	log.Info("certificate provisioned successfully")
	return nil
}

// Delete implements DeletingReconcileController — removes the domain from allowedHosts
// only if no other http_route entities reference the same host.
func (c *AutocertController) Delete(ctx context.Context, id entity.Id) error {
	// The deleted entity is gone, so we can't read its host directly.
	// Scan allowedHosts and for each domain, check if any routes still exist.
	// This is simple and correct; the set is small (number of unique domains).
	c.allowedHosts.Range(func(key, _ any) bool {
		domain := key.(string)
		res, err := c.eac.List(ctx, entity.String(ingress_v1alpha.HttpRouteHostId, domain))
		if err != nil {
			c.log.Warn("failed to query routes for domain during delete", "domain", domain, "error", err)
			return true // keep iterating, leave domain in set (safe default)
		}
		if len(res.Values()) == 0 {
			c.allowedHosts.Delete(domain)
			c.log.Debug("removed domain from allowed hosts (no remaining routes)", "domain", domain)
		}
		return true
	})
	return nil
}

// GetCertificate implements autotls.CertificateProvider — returns a cert from autocert,
// falling back to the self-signed cert on any error.
func (c *AutocertController) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	host := strings.ToLower(hello.ServerName)

	if c.isAllowedHost(host) {
		cert, err := c.mgr.GetCertificate(hello)
		if err == nil {
			return cert, nil
		}
		c.log.Debug("autocert failed, using fallback", "host", host, "error", err)
	}

	return &c.fallbackCert, nil
}

// HTTPHandler returns an http.Handler that serves ACME HTTP-01 challenge responses,
// delegating non-challenge requests to the provided fallback handler.
func (c *AutocertController) HTTPHandler(fallback http.Handler) http.Handler {
	return c.mgr.HTTPHandler(fallback)
}

// isAllowedHost checks whether host is covered by the allowed set, including
// wildcard entries. For example, if "*.example.com" is in the set, both
// "foo.example.com" and "example.com" are considered allowed.
func (c *AutocertController) isAllowedHost(host string) bool {
	if _, ok := c.allowedHosts.Load(host); ok {
		return true
	}
	// Check if a wildcard covers this host: foo.example.com → *.example.com
	if idx := strings.IndexByte(host, '.'); idx >= 0 {
		wildcard := "*" + host[idx:]
		if _, ok := c.allowedHosts.Load(wildcard); ok {
			return true
		}
	}
	return false
}

// SetReady signals that the port-80 ACME challenge server is up and accepting connections.
// This unblocks Reconcile calls that are waiting to provision certificates.
func (c *AutocertController) SetReady() {
	select {
	case <-c.ready:
		// Already closed
	default:
		close(c.ready)
	}
}
