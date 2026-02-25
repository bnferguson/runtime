package oidcauth

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"miren.dev/runtime/pkg/entity/testutils"
	"miren.dev/runtime/pkg/rpc"
)

// mockAuthenticator is a configurable authenticator for testing.
type mockAuthenticator struct {
	identity *rpc.Identity
	err      error
}

func (m *mockAuthenticator) Authenticate(ctx context.Context, r *http.Request) (*rpc.Identity, error) {
	return m.identity, m.err
}

// mockAuthorizer is a configurable authorizer for testing.
type mockAuthorizer struct {
	err error
}

func (m *mockAuthorizer) Authorize(ctx context.Context, identity *rpc.Identity, resource, action string) error {
	return m.err
}

func TestCompositeAuthenticator_PrimarySucceeds(t *testing.T) {
	primary := &mockAuthenticator{
		identity: &rpc.Identity{
			Subject: "user@example.com",
			Method:  rpc.AuthMethodJWT,
		},
	}
	oidcAuth := NewOIDCAuthenticator(testutils.TestLogger(t))
	comp := NewCompositeAuthenticator(primary, oidcAuth)

	req := httptest.NewRequest("GET", "/", nil)
	identity, err := comp.Authenticate(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if identity == nil {
		t.Fatal("expected identity from primary")
		return
	}
	if identity.Method != rpc.AuthMethodJWT {
		t.Errorf("method = %q, want %q", identity.Method, rpc.AuthMethodJWT)
	}
}

func TestCompositeAuthenticator_PrimaryError(t *testing.T) {
	primary := &mockAuthenticator{
		err: fmt.Errorf("auth server unavailable"),
	}
	oidcAuth := NewOIDCAuthenticator(testutils.TestLogger(t))
	comp := NewCompositeAuthenticator(primary, oidcAuth)

	req := httptest.NewRequest("GET", "/", nil)
	_, err := comp.Authenticate(context.Background(), req)
	if err == nil {
		t.Fatal("expected error to propagate from primary")
	}
}

func TestCompositeAuthenticator_FallbackToOIDC(t *testing.T) {
	// Primary returns nil (no credentials recognized)
	primary := &mockAuthenticator{identity: nil, err: nil}
	oidcAuth := NewOIDCAuthenticator(testutils.TestLogger(t)) // No EAC set, so OIDC will also return nil
	comp := NewCompositeAuthenticator(primary, oidcAuth)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer some-token")

	identity, err := comp.Authenticate(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if identity != nil {
		t.Error("expected nil identity when both authenticators fail to match")
	}
}

func TestCompositeAuthorizer_CertBypass(t *testing.T) {
	comp := NewCompositeAuthorizer(nil)

	identity := &rpc.Identity{
		Subject: "local-client",
		Method:  rpc.AuthMethodCert,
	}

	// Cert auth should bypass all checks, even for unknown resources
	err := comp.Authorize(context.Background(), identity, "anything", "anything")
	if err != nil {
		t.Errorf("cert auth should bypass all checks: %v", err)
	}
}

func TestCompositeAuthorizer_OIDCAllowed(t *testing.T) {
	comp := NewCompositeAuthorizer(nil)

	identity := &rpc.Identity{
		Subject: "repo:acme/app:ref:refs/heads/main",
		Method:  rpc.AuthMethodOIDC,
	}

	allowed := []struct {
		resource string
		action   string
	}{
		{"deployment", "deployversion"},
		{"deployment", "createdeployment"},
		{"logs", "applogs"},
		{"logs", "streamlogs"},
		{"logs", "streamlogchunks"},
		{"crud", "list"},
		{"crud", "getconfiguration"},
		{"builder", "buildfromtar"},
		{"builder", "analyzeapp"},
		{"telemetry", "reportspans"},
		{"appstatus", "appinfo"},
	}

	for _, tc := range allowed {
		err := comp.Authorize(context.Background(), identity, tc.resource, tc.action)
		if err != nil {
			t.Errorf("Authorize(%q, %q) should be allowed for OIDC: %v", tc.resource, tc.action, err)
		}
	}
}

func TestCompositeAuthorizer_OIDCDenied(t *testing.T) {
	comp := NewCompositeAuthorizer(nil)

	identity := &rpc.Identity{
		Subject: "repo:acme/app:ref:refs/heads/main",
		Method:  rpc.AuthMethodOIDC,
	}

	denied := []struct {
		resource string
		action   string
	}{
		{"admin", "runcommand"},
		{"oidcbindings", "add"},
		{"oidcbindings", "remove"},
		{"deployment", "unknownmethod"},
		{"sandbox", "stop"},
		{"sandbox", "delete"},
		{"entities", "delete"},
	}

	for _, tc := range denied {
		err := comp.Authorize(context.Background(), identity, tc.resource, tc.action)
		if err == nil {
			t.Errorf("Authorize(%q, %q) should be denied for OIDC", tc.resource, tc.action)
		}
	}
}

func TestCompositeAuthorizer_JWTDelegatesToPrimary(t *testing.T) {
	primary := &mockAuthorizer{err: nil}
	comp := NewCompositeAuthorizer(primary)

	identity := &rpc.Identity{
		Subject: "user@example.com",
		Method:  rpc.AuthMethodJWT,
	}

	err := comp.Authorize(context.Background(), identity, "deployment", "deployversion")
	if err != nil {
		t.Errorf("JWT auth should delegate to primary: %v", err)
	}

	// Now make primary deny
	primary.err = fmt.Errorf("access denied by RBAC")
	err = comp.Authorize(context.Background(), identity, "deployment", "deployversion")
	if err == nil {
		t.Error("JWT auth should propagate primary denial")
	}
}

func TestCompositeAuthorizer_NilPrimary(t *testing.T) {
	comp := NewCompositeAuthorizer(nil)

	identity := &rpc.Identity{
		Subject: "user@example.com",
		Method:  rpc.AuthMethodJWT,
	}

	// With nil primary, JWT auth should allow (no-op)
	err := comp.Authorize(context.Background(), identity, "deployment", "deployversion")
	if err != nil {
		t.Errorf("nil primary should allow all for JWT: %v", err)
	}
}

func TestAuthorizeOIDC(t *testing.T) {
	tests := []struct {
		resource string
		action   string
		allowed  bool
	}{
		{"deployment", "deployversion", true},
		{"deployment", "canceldeployment", true},
		{"logs", "applogs", true},
		{"crud", "list", true},
		{"crud", "getconfiguration", true},
		{"crud", "delete", false},
		{"builder", "buildfromtar", true},
		{"builder", "analyzeapp", true},
		{"telemetry", "reportspans", true},
		{"unknown", "anything", false},
		{"deployment", "unknown", false},
		{"", "", false},
	}

	for _, tt := range tests {
		err := authorizeOIDC(tt.resource, tt.action)
		if tt.allowed && err != nil {
			t.Errorf("authorizeOIDC(%q, %q) should be allowed: %v", tt.resource, tt.action, err)
		}
		if !tt.allowed && err == nil {
			t.Errorf("authorizeOIDC(%q, %q) should be denied", tt.resource, tt.action)
		}
	}
}
