package certificate

import (
	"context"
	"crypto/tls"
	"log/slog"
	"os"
	"testing"
	"time"

	"miren.dev/runtime/api/ingress/ingress_v1alpha"
	"miren.dev/runtime/pkg/entity"
	"miren.dev/runtime/pkg/entity/testutils"
)

func newTestAutocertController(t *testing.T) *AutocertController {
	t.Helper()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	c := NewAutocertController(AutocertControllerOpts{
		Log:      log,
		DataPath: t.TempDir(),
		Email:    "test@example.com",
	})
	if err := c.Init(context.Background()); err != nil {
		t.Fatalf("failed to init autocert controller: %v", err)
	}
	return c
}

func testRouteMeta(id string, host string) (*ingress_v1alpha.HttpRoute, *entity.Meta) {
	route := &ingress_v1alpha.HttpRoute{
		ID:   entity.Id(id),
		Host: host,
	}
	ent := entity.New(entity.Ident, entity.Id(id), route.Encode)
	return route, &entity.Meta{Entity: ent, Revision: 1}
}

func TestAutocertController_Init(t *testing.T) {
	c := newTestAutocertController(t)
	if c.mgr == nil {
		t.Fatal("expected autocert.Manager to be initialized")
	}
}

func TestAutocertController_Reconcile_AddsAllowedHost(t *testing.T) {
	c := newTestAutocertController(t)
	c.SetReady()

	route, meta := testRouteMeta("test-route", "example.com")
	if err := c.Reconcile(context.Background(), route, meta); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := c.allowedHosts.Load("example.com"); !ok {
		t.Error("expected example.com to be in allowed hosts")
	}
}

func TestAutocertController_Reconcile_EmptyHost(t *testing.T) {
	c := newTestAutocertController(t)
	c.SetReady()

	route, meta := testRouteMeta("test-route", "")
	if err := c.Reconcile(context.Background(), route, meta); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	count := 0
	c.allowedHosts.Range(func(_, _ any) bool {
		count++
		return true
	})
	if count != 0 {
		t.Errorf("expected no allowed hosts, got %d", count)
	}
}

func TestAutocertController_GetCertificate_FallbackForUnknownHost(t *testing.T) {
	c := newTestAutocertController(t)

	hello := &tls.ClientHelloInfo{ServerName: "unknown.example.com"}
	cert, err := c.GetCertificate(hello)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cert == nil {
		t.Fatal("expected a fallback certificate, got nil")
		return
	}
	if len(cert.Certificate) == 0 {
		t.Error("expected fallback cert to have certificate data")
	}
}

func TestAutocertController_GetCertificate_FallbackForAllowedHostWithoutCert(t *testing.T) {
	c := newTestAutocertController(t)
	c.allowedHosts.Store("example.com", struct{}{})

	hello := &tls.ClientHelloInfo{ServerName: "example.com"}
	cert, err := c.GetCertificate(hello)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cert == nil {
		t.Fatal("expected a fallback certificate, got nil")
	}
}

func TestAutocertController_HostPolicy(t *testing.T) {
	c := newTestAutocertController(t)

	err := c.mgr.HostPolicy(context.Background(), "unknown.example.com")
	if err == nil {
		t.Error("expected host policy to reject unknown host")
	}

	c.allowedHosts.Store("allowed.example.com", struct{}{})
	err = c.mgr.HostPolicy(context.Background(), "allowed.example.com")
	if err != nil {
		t.Errorf("expected host policy to accept allowed host, got: %v", err)
	}
}

func TestAutocertController_Reconcile_WildcardStoresPattern(t *testing.T) {
	c := newTestAutocertController(t)
	c.SetReady()

	route, meta := testRouteMeta("wildcard-route", "*.example.com")
	if err := c.Reconcile(context.Background(), route, meta); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := c.allowedHosts.Load("*.example.com"); !ok {
		t.Error("expected *.example.com to be in allowed hosts")
	}
}

func TestAutocertController_IsAllowedHost_WildcardMatching(t *testing.T) {
	c := newTestAutocertController(t)
	c.allowedHosts.Store("*.example.com", struct{}{})

	tests := []struct {
		host    string
		allowed bool
	}{
		{"foo.example.com", true},
		{"bar.example.com", true},
		{"example.com", false},          // bare domain requires its own route
		{"other.com", false},            // unrelated domain
		{"deep.sub.example.com", false}, // only one level of wildcard
	}

	for _, tt := range tests {
		got := c.isAllowedHost(tt.host)
		if got != tt.allowed {
			t.Errorf("isAllowedHost(%q) = %v, want %v", tt.host, got, tt.allowed)
		}
	}
}

func TestAutocertController_GetCertificate_WildcardSubdomain(t *testing.T) {
	c := newTestAutocertController(t)
	c.allowedHosts.Store("*.example.com", struct{}{})

	// A subdomain covered by the wildcard should attempt autocert (and fall back)
	hello := &tls.ClientHelloInfo{ServerName: "foo.example.com"}
	cert, err := c.GetCertificate(hello)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cert == nil {
		t.Fatal("expected a certificate, got nil")
	}
}

func TestAutocertController_HostPolicy_WildcardMatching(t *testing.T) {
	c := newTestAutocertController(t)
	c.allowedHosts.Store("*.example.com", struct{}{})

	// Subdomain covered by wildcard should be accepted
	if err := c.mgr.HostPolicy(context.Background(), "foo.example.com"); err != nil {
		t.Errorf("expected wildcard to accept foo.example.com, got: %v", err)
	}

	// Unrelated domain should be rejected
	if err := c.mgr.HostPolicy(context.Background(), "other.com"); err == nil {
		t.Error("expected host policy to reject other.com")
	}
}

// TestAutocertController_IsAllowedHost_EphemeralSubdomain verifies that an
// ephemeral subdomain of a normal (non-wildcard) route is allowed, matching
// the request-routing behavior in httpingress.lookupEphemeralRoute. Without
// this, ephemeral deploy URLs like pr-33.app.example.com would serve the
// fallback self-signed cert instead of provisioning via autocert.
func TestAutocertController_IsAllowedHost_EphemeralSubdomain(t *testing.T) {
	c := newTestAutocertController(t)
	c.allowedHosts.Store("app.example.com", struct{}{})

	tests := []struct {
		host    string
		allowed bool
	}{
		{"app.example.com", true},        // exact match (the registered route)
		{"pr-33.app.example.com", true},  // ephemeral subdomain of the route
		{"feat-x.app.example.com", true}, // another ephemeral label
		{"app.example.org", false},       // unrelated TLD
		{"example.com", false},           // parent of the route, not a subdomain
		{"other.com", false},             // unrelated domain
	}

	for _, tt := range tests {
		got := c.isAllowedHost(tt.host)
		if got != tt.allowed {
			t.Errorf("isAllowedHost(%q) = %v, want %v", tt.host, got, tt.allowed)
		}
	}
}

func TestAutocertController_HostPolicy_EphemeralSubdomain(t *testing.T) {
	c := newTestAutocertController(t)
	c.allowedHosts.Store("app.example.com", struct{}{})

	// Ephemeral subdomain of a normal route should be accepted by HostPolicy
	// so autocert will attempt ACME provisioning instead of falling back.
	if err := c.mgr.HostPolicy(context.Background(), "pr-33.app.example.com"); err != nil {
		t.Errorf("expected host policy to accept ephemeral subdomain, got: %v", err)
	}
}

func TestAutocertController_Init_PrePopulatesAllowedHosts(t *testing.T) {
	ctx := context.Background()

	server, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	// Create http_route entities before Init
	routes := []struct {
		id   string
		host string
	}{
		{"route-1", "example.com"},
		{"route-2", "api.example.com"},
		{"route-3", "*.staging.example.com"},
	}
	for _, r := range routes {
		route := &ingress_v1alpha.HttpRoute{Host: r.host}
		if _, err := server.Client.Create(ctx, r.id, route); err != nil {
			t.Fatalf("failed to create route %s: %v", r.id, err)
		}
	}

	// Create controller with real EAC and init
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	c := NewAutocertController(AutocertControllerOpts{
		Log:      log,
		EAC:      server.EAC,
		DataPath: t.TempDir(),
		Email:    "test@example.com",
	})
	if err := c.Init(ctx); err != nil {
		t.Fatalf("failed to init: %v", err)
	}

	// Verify all hosts were pre-populated
	for _, r := range routes {
		if _, ok := c.allowedHosts.Load(r.host); !ok {
			t.Errorf("expected %q to be in allowed hosts after Init", r.host)
		}
	}

	// Verify unknown hosts are NOT in allowedHosts
	if _, ok := c.allowedHosts.Load("unknown.com"); ok {
		t.Error("unexpected host in allowed hosts")
	}
}

func TestAutocertController_Reconcile_SkipsDuringFailureCooldown(t *testing.T) {
	c := newTestAutocertController(t)
	c.SetReady()

	// Simulate a recent failure for this domain
	c.failures.Store("cooldown.example.com", acmeFailure{when: time.Now(), cooldown: acmeFailureCooldown})

	route, meta := testRouteMeta("cooldown-route", "cooldown.example.com")
	if err := c.Reconcile(context.Background(), route, meta); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Domain should still be in allowedHosts (Reconcile always adds it)
	if _, ok := c.allowedHosts.Load("cooldown.example.com"); !ok {
		t.Error("expected domain to be in allowed hosts")
	}

	// Failure entry should still be present (not cleared by a successful provision)
	if _, ok := c.failures.Load("cooldown.example.com"); !ok {
		t.Error("expected failure entry to remain during cooldown")
	}
}

func TestAutocertController_Reconcile_ClearsExpiredCooldown(t *testing.T) {
	c := newTestAutocertController(t)
	c.SetReady()

	// Simulate an old failure (well past cooldown)
	c.failures.Store("expired.example.com", acmeFailure{when: time.Now().Add(-10 * time.Minute), cooldown: acmeFailureCooldown})

	route, meta := testRouteMeta("expired-route", "expired.example.com")
	// This will attempt ACME (and fail since there's no real ACME server),
	// but the important thing is it doesn't skip due to cooldown.
	_ = c.Reconcile(context.Background(), route, meta)

	// The old failure entry should have been deleted before the attempt.
	// A new one may have been stored if the ACME attempt itself failed,
	// but the original stale timestamp should be gone.
	if v, ok := c.failures.Load("expired.example.com"); ok {
		f := v.(acmeFailure)
		if time.Since(f.when) > time.Minute {
			t.Error("expected stale failure entry to be replaced, but old timestamp remains")
		}
	}
}

func TestAutocertController_GetCertificate_RecordsFailureOnTimeout(t *testing.T) {
	c := newTestAutocertController(t)
	c.allowedHosts.Store("timeout.example.com", struct{}{})

	// GetCertificate will attempt autocert, which will fail/timeout since
	// there's no real ACME server. It should record a failure.
	hello := &tls.ClientHelloInfo{ServerName: "timeout.example.com"}
	cert, err := c.GetCertificate(hello)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cert == nil {
		t.Fatal("expected fallback cert, got nil")
	}

	// After the call completes (via timeout or error), the domain should be in failures
	if _, ok := c.failures.Load("timeout.example.com"); !ok {
		t.Error("expected failure to be recorded after GetCertificate timeout/error")
	}
}

func TestAutocertController_GetCertificate_SkipsDuringCooldown(t *testing.T) {
	c := newTestAutocertController(t)
	c.allowedHosts.Store("cooldown.example.com", struct{}{})
	c.failures.Store("cooldown.example.com", acmeFailure{when: time.Now(), cooldown: acmeFailureCooldown})

	hello := &tls.ClientHelloInfo{ServerName: "cooldown.example.com"}
	cert, err := c.GetCertificate(hello)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cert == nil {
		t.Fatal("expected fallback cert, got nil")
	}
}

func TestAutocertController_SetReady_Idempotent(t *testing.T) {
	c := newTestAutocertController(t)
	c.SetReady()
	c.SetReady() // should not panic
}

func TestAutocertController_Reconcile_BlocksUntilReady(t *testing.T) {
	c := newTestAutocertController(t)

	route, meta := testRouteMeta("test-route", "example.com")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := c.Reconcile(ctx, route, meta)
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}
