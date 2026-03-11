package certificate

import (
	"context"
	"crypto/tls"
	"log/slog"
	"os"
	"testing"

	"miren.dev/runtime/api/ingress/ingress_v1alpha"
	"miren.dev/runtime/pkg/entity"
)

func newTestAutocertController(t *testing.T) *AutocertController {
	t.Helper()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	dataPath := t.TempDir()
	// nil eac is fine for unit tests that don't exercise Delete
	c := NewAutocertController(log, nil, dataPath, "test@example.com")
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
		{"example.com", false},          // bare domain not covered by wildcard alone
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
