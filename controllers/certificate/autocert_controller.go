package certificate

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/acme/autocert"
	"miren.dev/runtime/api/entityserver/entityserver_v1alpha"
	"miren.dev/runtime/api/ingress/ingress_v1alpha"
	"miren.dev/runtime/components/autotls"
	"miren.dev/runtime/pkg/entity"
)

const (
	// How long to suppress ACME retries for a domain after a failure.
	acmeFailureCooldown = 5 * time.Minute

	// Max time to wait for autocert before falling back to the self-signed cert.
	// The upstream autocert.Manager uses a hardcoded 5-minute timeout internally;
	// this shorter deadline lets TLS handshakes complete promptly with the fallback.
	inlineGetCertTimeout = 10 * time.Second
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
	publicIPs    func() []net.IP
	failures     sync.Map // domain -> time.Time of last failure
}

type AutocertControllerOpts struct {
	Log      *slog.Logger
	EAC      *entityserver_v1alpha.EntityAccessClient
	DataPath string
	Email    string

	// PublicIPs, if non-nil, is called before eager provisioning to verify DNS
	// points to this cluster; when nil the check is skipped.
	PublicIPs func() []net.IP
}

func NewAutocertController(opts AutocertControllerOpts) *AutocertController {
	return &AutocertController{
		log:       opts.Log.With("module", "autocert-controller"),
		eac:       opts.EAC,
		dataPath:  opts.DataPath,
		email:     opts.Email,
		ready:     make(chan struct{}),
		publicIPs: opts.PublicIPs,
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

	// Pre-populate allowedHosts from existing http_route entities so the
	// isAllowedHost guard in GetCertificate works immediately — before the
	// controller manager starts reconciling routes one by one.
	if err := c.loadExistingRoutes(ctx); err != nil {
		c.log.Warn("failed to pre-populate allowed hosts from existing routes", "error", err)
	}

	c.log.Info("autocert controller initialized")
	return nil
}

// loadExistingRoutes queries all http_route entities and adds their hosts to allowedHosts.
func (c *AutocertController) loadExistingRoutes(ctx context.Context) error {
	if c.eac == nil {
		return nil
	}
	res, err := c.eac.List(ctx, entity.Ref(entity.EntityKind, ingress_v1alpha.KindHttpRoute))
	if err != nil {
		return fmt.Errorf("failed to list http_route entities: %w", err)
	}

	count := 0
	for _, ent := range res.Values() {
		var route ingress_v1alpha.HttpRoute
		route.Decode(ent.Entity())
		domain := strings.ToLower(strings.TrimSpace(route.Host))
		if domain != "" {
			c.allowedHosts.Store(domain, struct{}{})
			count++
		}
	}

	if count > 0 {
		c.log.Info("pre-populated allowed hosts from existing routes", "count", count)
	}
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

	// Wildcard routes (*.example.com) can't be eagerly provisioned — HTTP-01 can't
	// issue wildcard certs and we don't know which subdomains will be requested.
	// Just add the pattern to allowedHosts so HostPolicy accepts subdomains inline.
	if strings.HasPrefix(domain, "*.") {
		log.Info("wildcard route: subdomains will provision certs inline on first request")
		return nil
	}

	// Check DNS before attempting ACME to avoid wasting rate-limited authorizations
	// on domains that don't resolve to this cluster yet (e.g., during DNS migration).
	if c.publicIPs != nil {
		if !c.dnsPointsToUs(domain) {
			log.Info("skipping eager cert provisioning: DNS does not point to this cluster (will provision inline when DNS propagates)")
			return nil
		}
	}

	// Wait for port-80 ACME challenge server to be ready before attempting provisioning
	select {
	case <-c.ready:
	case <-ctx.Done():
		return ctx.Err()
	}

	// Eagerly provision the certificate with a synthetic ClientHelloInfo
	hello := &tls.ClientHelloInfo{ServerName: domain}
	_, err := c.mgr.GetCertificate(hello)
	if err != nil {
		c.failures.Store(domain, time.Now())
		log.Warn("eager cert provisioning failed (will retry on next TLS handshake)", "error", err)
		// Don't return the error — the controller framework would retry, but autocert
		// itself will handle this on the next actual TLS handshake or resync.
		return nil
	}

	c.failures.Delete(domain)
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
// falling back to the self-signed cert on any error or timeout.
func (c *AutocertController) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	host := strings.ToLower(hello.ServerName)

	if !c.isAllowedHost(host) {
		return &c.fallbackCert, nil
	}

	// Skip ACME if this domain recently failed — prevents rapid-fire attempts
	// from every incoming TLS handshake while rate-limited.
	if lastFail, ok := c.failures.Load(host); ok {
		if time.Since(lastFail.(time.Time)) < acmeFailureCooldown {
			return &c.fallbackCert, nil
		}
		c.failures.Delete(host)
	}

	// Run autocert with a deadline so handshakes fall back to the self-signed
	// cert promptly instead of blocking for the upstream 5-minute timeout.
	type certResult struct {
		cert *tls.Certificate
		err  error
	}
	ch := make(chan certResult, 1)
	go func() {
		cert, err := c.mgr.GetCertificate(hello)
		ch <- certResult{cert, err}
	}()

	select {
	case res := <-ch:
		if res.err == nil {
			return res.cert, nil
		}
		c.failures.Store(host, time.Now())
		c.log.Debug("autocert failed, using fallback", "host", host, "error", res.err)
	case <-time.After(inlineGetCertTimeout):
		c.log.Warn("autocert timed out, using fallback", "host", host, "timeout", inlineGetCertTimeout)
	}

	return &c.fallbackCert, nil
}

// HTTPHandler returns an http.Handler that serves ACME HTTP-01 challenge responses,
// delegating non-challenge requests to the provided fallback handler.
func (c *AutocertController) HTTPHandler(fallback http.Handler) http.Handler {
	return c.mgr.HTTPHandler(fallback)
}

// dnsPointsToUs resolves domain and checks if any of the returned IPs match
// one of the cluster's known public IPs. Returns true if there's a match, or
// if public IPs are unavailable (fail open so we don't block provisioning
// when netcheck hasn't run yet).
func (c *AutocertController) dnsPointsToUs(domain string) bool {
	ips := c.publicIPs()
	if len(ips) == 0 {
		return true // no known IPs — skip the check
	}

	addrs, err := net.LookupHost(domain)
	if err != nil {
		return false
	}

	for _, addr := range addrs {
		resolved := net.ParseIP(addr)
		if resolved == nil {
			continue
		}
		for _, pub := range ips {
			if resolved.Equal(pub) {
				return true
			}
		}
	}
	return false
}

// isAllowedHost checks whether host is covered by the allowed set, including
// wildcard entries. For example, if "*.example.com" is in the set,
// "foo.example.com" is allowed but "example.com" is not — the wildcard
// matches exactly one DNS label.
func (c *AutocertController) isAllowedHost(host string) bool {
	if _, ok := c.allowedHosts.Load(host); ok {
		return true
	}
	// Check if a wildcard covers this subdomain: foo.example.com → *.example.com
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
