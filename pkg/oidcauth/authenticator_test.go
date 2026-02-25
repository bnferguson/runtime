package oidcauth

import (
	"context"
	"crypto/tls"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"miren.dev/runtime/api/core/core_v1alpha"
	"miren.dev/runtime/pkg/entity/testutils"
	"miren.dev/runtime/pkg/rpc"
)

func TestPeekIssuer(t *testing.T) {
	tests := []struct {
		name    string
		token   string
		want    string
		wantErr bool
	}{
		{
			name:    "not a JWT",
			token:   "not-a-jwt",
			wantErr: true,
		},
		{
			name:    "empty string",
			token:   "",
			wantErr: true,
		},
		{
			name:    "two parts only",
			token:   "header.payload",
			wantErr: true,
		},
		{
			name:    "invalid base64 payload",
			token:   "aaa.!!!invalid!!!.ccc",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := peekIssuer(tt.token)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPeekIssuer_ValidJWT(t *testing.T) {
	ts := newTestOIDCServer(t)
	defer ts.Close()

	token := ts.SignToken(jwt.MapClaims{
		"iss": "https://token.actions.githubusercontent.com",
		"sub": "repo:acme/app:ref:refs/heads/main",
		"aud": "test",
		"exp": time.Now().Add(10 * time.Minute).Unix(),
	})

	got, err := peekIssuer(token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "https://token.actions.githubusercontent.com" {
		t.Errorf("got %q, want GitHub issuer", got)
	}
}

func TestResolveAppName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"my-app", "my-app"},
		{"app/my-app", "my-app"},
		{"some/deep/path/my-app", "my-app"},
		{"", ""},
	}
	for _, tt := range tests {
		got := resolveAppName(tt.input)
		if got != tt.want {
			t.Errorf("resolveAppName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestOIDCAuthenticator_NoEAC(t *testing.T) {
	auth := NewOIDCAuthenticator(testutils.TestLogger(t))

	ts := newTestOIDCServer(t)
	defer ts.Close()

	token := ts.SignToken(jwt.MapClaims{
		"iss": ts.URL(),
		"sub": "repo:acme/app:ref:refs/heads/main",
		"aud": "test-host",
		"exp": time.Now().Add(10 * time.Minute).Unix(),
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	identity, err := auth.Authenticate(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if identity != nil {
		t.Error("expected nil identity when EAC not set")
	}
}

func TestOIDCAuthenticator_NoBearerToken(t *testing.T) {
	auth := NewOIDCAuthenticator(testutils.TestLogger(t))

	req := httptest.NewRequest("GET", "/", nil)
	identity, err := auth.Authenticate(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if identity != nil {
		t.Error("expected nil identity for request without bearer token")
	}
}

func TestOIDCAuthenticator_NonJWTBearer(t *testing.T) {
	inmem, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	auth := NewOIDCAuthenticator(testutils.TestLogger(t))
	auth.SetEAC(inmem.EAC)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer not-a-jwt-token")

	identity, err := auth.Authenticate(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if identity != nil {
		t.Error("expected nil identity for non-JWT bearer token")
	}
}

func TestOIDCAuthenticator_NoMatchingBindings(t *testing.T) {
	inmem, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	auth := NewOIDCAuthenticator(testutils.TestLogger(t))
	auth.SetEAC(inmem.EAC)

	ts := newTestOIDCServer(t)
	defer ts.Close()

	token := ts.SignToken(jwt.MapClaims{
		"iss": ts.URL(),
		"sub": "repo:acme/app:ref:refs/heads/main",
		"aud": "test-host",
		"exp": time.Now().Add(10 * time.Minute).Unix(),
	})

	req := httptest.NewRequest("GET", "https://test-host/", nil)
	req.Host = "test-host"
	req.Header.Set("Authorization", "Bearer "+token)

	identity, err := auth.Authenticate(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if identity != nil {
		t.Error("expected nil identity when no bindings match issuer")
	}
}

func TestOIDCAuthenticator_MatchingBinding(t *testing.T) {
	ctx := context.Background()
	inmem, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	ts := newTestOIDCServer(t)
	defer ts.Close()

	// Create an app first
	app := &core_v1alpha.App{}
	_, err := inmem.Client.Create(ctx, "my-app", app)
	if err != nil {
		t.Fatalf("failed to create app: %v", err)
	}

	// Get the app entity ID
	var appRec core_v1alpha.App
	if err := inmem.Client.Get(ctx, "my-app", &appRec); err != nil {
		t.Fatalf("failed to get app: %v", err)
	}

	// Create an OIDC binding pointing at our test OIDC server
	binding := &core_v1alpha.OidcBinding{
		App:            appRec.EntityId(),
		Provider:       "github",
		Issuer:         ts.URL(),
		SubjectPattern: "repo:acme/app:*",
		ClaimConditions: []core_v1alpha.ClaimConditions{
			{Key: "event_name", Pattern: "push,workflow_dispatch"},
		},
	}
	_, err = inmem.Client.Create(ctx, "oidcb-test1", binding)
	if err != nil {
		t.Fatalf("failed to create OIDC binding: %v", err)
	}

	auth := NewOIDCAuthenticator(testutils.TestLogger(t))
	auth.SetEAC(inmem.EAC)

	token := ts.SignToken(jwt.MapClaims{
		"iss":        ts.URL(),
		"sub":        "repo:acme/app:ref:refs/heads/main",
		"aud":        "test-host",
		"exp":        time.Now().Add(10 * time.Minute).Unix(),
		"iat":        time.Now().Unix(),
		"event_name": "push",
	})

	req := httptest.NewRequest("GET", "https://test-host/", nil)
	req.Host = "test-host"
	req.Header.Set("Authorization", "Bearer "+token)

	identity, err := auth.Authenticate(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if identity == nil {
		t.Fatal("expected identity, got nil")
	}
	if identity.Method != rpc.AuthMethodOIDC {
		t.Errorf("method = %q, want %q", identity.Method, rpc.AuthMethodOIDC)
	}
	if identity.Subject != "repo:acme/app:ref:refs/heads/main" {
		t.Errorf("subject = %q, want repo:acme/app:ref:refs/heads/main", identity.Subject)
	}
	if identity.Metadata["provider"] != "github" {
		t.Errorf("provider = %v, want github", identity.Metadata["provider"])
	}
	if identity.Metadata["bound_app"] == nil || identity.Metadata["bound_app"] == "" {
		t.Error("bound_app should be set")
	}
}

func TestOIDCAuthenticator_SubjectMismatch(t *testing.T) {
	ctx := context.Background()
	inmem, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	ts := newTestOIDCServer(t)
	defer ts.Close()

	app := &core_v1alpha.App{}
	_, err := inmem.Client.Create(ctx, "my-app", app)
	if err != nil {
		t.Fatalf("failed to create app: %v", err)
	}

	var appRec core_v1alpha.App
	if err := inmem.Client.Get(ctx, "my-app", &appRec); err != nil {
		t.Fatalf("failed to get app: %v", err)
	}

	binding := &core_v1alpha.OidcBinding{
		App:            appRec.EntityId(),
		Provider:       "github",
		Issuer:         ts.URL(),
		SubjectPattern: "repo:acme/other-app:*",
	}
	_, err = inmem.Client.Create(ctx, "oidcb-test2", binding)
	if err != nil {
		t.Fatalf("failed to create OIDC binding: %v", err)
	}

	auth := NewOIDCAuthenticator(testutils.TestLogger(t))
	auth.SetEAC(inmem.EAC)

	token := ts.SignToken(jwt.MapClaims{
		"iss": ts.URL(),
		"sub": "repo:acme/app:ref:refs/heads/main",
		"aud": "test-host",
		"exp": time.Now().Add(10 * time.Minute).Unix(),
		"iat": time.Now().Unix(),
	})

	req := httptest.NewRequest("GET", "https://test-host/", nil)
	req.Host = "test-host"
	req.Header.Set("Authorization", "Bearer "+token)

	identity, err := auth.Authenticate(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if identity != nil {
		t.Error("expected nil identity when subject doesn't match binding pattern")
	}
}

func TestOIDCAuthenticator_ClaimConditionMismatch(t *testing.T) {
	ctx := context.Background()
	inmem, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	ts := newTestOIDCServer(t)
	defer ts.Close()

	app := &core_v1alpha.App{}
	_, err := inmem.Client.Create(ctx, "my-app", app)
	if err != nil {
		t.Fatalf("failed to create app: %v", err)
	}

	var appRec core_v1alpha.App
	if err := inmem.Client.Get(ctx, "my-app", &appRec); err != nil {
		t.Fatalf("failed to get app: %v", err)
	}

	binding := &core_v1alpha.OidcBinding{
		App:            appRec.EntityId(),
		Provider:       "github",
		Issuer:         ts.URL(),
		SubjectPattern: "repo:acme/app:*",
		ClaimConditions: []core_v1alpha.ClaimConditions{
			{Key: "event_name", Pattern: "push"},
		},
	}
	_, err = inmem.Client.Create(ctx, "oidcb-test3", binding)
	if err != nil {
		t.Fatalf("failed to create OIDC binding: %v", err)
	}

	auth := NewOIDCAuthenticator(testutils.TestLogger(t))
	auth.SetEAC(inmem.EAC)

	// Token has event_name=pull_request, binding requires push
	token := ts.SignToken(jwt.MapClaims{
		"iss":        ts.URL(),
		"sub":        "repo:acme/app:ref:refs/heads/main",
		"aud":        "test-host",
		"exp":        time.Now().Add(10 * time.Minute).Unix(),
		"iat":        time.Now().Unix(),
		"event_name": "pull_request",
	})

	req := httptest.NewRequest("GET", "https://test-host/", nil)
	req.Host = "test-host"
	req.Header.Set("Authorization", "Bearer "+token)

	identity, err := auth.Authenticate(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if identity != nil {
		t.Error("expected nil identity when claim condition doesn't match")
	}
}

func TestOIDCAuthenticator_ExpiredToken(t *testing.T) {
	ctx := context.Background()
	inmem, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	ts := newTestOIDCServer(t)
	defer ts.Close()

	app := &core_v1alpha.App{}
	_, err := inmem.Client.Create(ctx, "my-app", app)
	if err != nil {
		t.Fatalf("failed to create app: %v", err)
	}

	var appRec core_v1alpha.App
	if err := inmem.Client.Get(ctx, "my-app", &appRec); err != nil {
		t.Fatalf("failed to get app: %v", err)
	}

	binding := &core_v1alpha.OidcBinding{
		App:            appRec.EntityId(),
		Provider:       "github",
		Issuer:         ts.URL(),
		SubjectPattern: "repo:acme/app:*",
	}
	_, err = inmem.Client.Create(ctx, "oidcb-test4", binding)
	if err != nil {
		t.Fatalf("failed to create OIDC binding: %v", err)
	}

	auth := NewOIDCAuthenticator(testutils.TestLogger(t))
	auth.SetEAC(inmem.EAC)

	token := ts.SignToken(jwt.MapClaims{
		"iss": ts.URL(),
		"sub": "repo:acme/app:ref:refs/heads/main",
		"aud": "test-host",
		"exp": time.Now().Add(-10 * time.Minute).Unix(),
		"iat": time.Now().Add(-20 * time.Minute).Unix(),
	})

	req := httptest.NewRequest("GET", "https://test-host/", nil)
	req.Host = "test-host"
	req.Header.Set("Authorization", "Bearer "+token)

	identity, err := auth.Authenticate(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if identity != nil {
		t.Error("expected nil identity for expired token")
	}
}

func TestOIDCAuthenticator_MultipleBindings_FirstMatch(t *testing.T) {
	ctx := context.Background()
	inmem, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	ts := newTestOIDCServer(t)
	defer ts.Close()

	// Create two apps
	app1 := &core_v1alpha.App{}
	_, err := inmem.Client.Create(ctx, "app-one", app1)
	if err != nil {
		t.Fatalf("failed to create app1: %v", err)
	}
	var appRec1 core_v1alpha.App
	if err := inmem.Client.Get(ctx, "app-one", &appRec1); err != nil {
		t.Fatalf("failed to get app1: %v", err)
	}

	app2 := &core_v1alpha.App{}
	_, err = inmem.Client.Create(ctx, "app-two", app2)
	if err != nil {
		t.Fatalf("failed to create app2: %v", err)
	}
	var appRec2 core_v1alpha.App
	if err := inmem.Client.Get(ctx, "app-two", &appRec2); err != nil {
		t.Fatalf("failed to get app2: %v", err)
	}

	// Binding for app-one: matches repo:acme/app-one:*
	b1 := &core_v1alpha.OidcBinding{
		App:            appRec1.EntityId(),
		Provider:       "github",
		Issuer:         ts.URL(),
		SubjectPattern: "repo:acme/app-one:*",
	}
	_, err = inmem.Client.Create(ctx, "oidcb-multi1", b1)
	if err != nil {
		t.Fatalf("failed to create binding1: %v", err)
	}

	// Binding for app-two: matches repo:acme/app-two:*
	b2 := &core_v1alpha.OidcBinding{
		App:            appRec2.EntityId(),
		Provider:       "github",
		Issuer:         ts.URL(),
		SubjectPattern: "repo:acme/app-two:*",
	}
	_, err = inmem.Client.Create(ctx, "oidcb-multi2", b2)
	if err != nil {
		t.Fatalf("failed to create binding2: %v", err)
	}

	auth := NewOIDCAuthenticator(testutils.TestLogger(t))
	auth.SetEAC(inmem.EAC)

	// Token subject matches app-two's binding
	token := ts.SignToken(jwt.MapClaims{
		"iss": ts.URL(),
		"sub": "repo:acme/app-two:ref:refs/heads/main",
		"aud": "test-host",
		"exp": time.Now().Add(10 * time.Minute).Unix(),
		"iat": time.Now().Unix(),
	})

	req := httptest.NewRequest("GET", "https://test-host/", nil)
	req.Host = "test-host"
	req.Header.Set("Authorization", "Bearer "+token)

	identity, err := auth.Authenticate(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if identity == nil {
		t.Fatal("expected identity, got nil")
	}
	if identity.Method != rpc.AuthMethodOIDC {
		t.Errorf("method = %q, want %q", identity.Method, rpc.AuthMethodOIDC)
	}
}

func TestOIDCAuthenticator_AudienceFromHost(t *testing.T) {
	ctx := context.Background()
	inmem, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	ts := newTestOIDCServer(t)
	defer ts.Close()

	app := &core_v1alpha.App{}
	_, err := inmem.Client.Create(ctx, "my-app", app)
	if err != nil {
		t.Fatalf("failed to create app: %v", err)
	}

	var appRec core_v1alpha.App
	if err := inmem.Client.Get(ctx, "my-app", &appRec); err != nil {
		t.Fatalf("failed to get app: %v", err)
	}

	binding := &core_v1alpha.OidcBinding{
		App:            appRec.EntityId(),
		Provider:       "github",
		Issuer:         ts.URL(),
		SubjectPattern: "repo:acme/app:*",
	}
	_, err = inmem.Client.Create(ctx, "oidcb-aud1", binding)
	if err != nil {
		t.Fatalf("failed to create OIDC binding: %v", err)
	}

	auth := NewOIDCAuthenticator(testutils.TestLogger(t))
	auth.SetEAC(inmem.EAC)

	// Token with audience matching the Host header
	token := ts.SignToken(jwt.MapClaims{
		"iss": ts.URL(),
		"sub": "repo:acme/app:ref:refs/heads/main",
		"aud": "my-cluster.example.com",
		"exp": time.Now().Add(10 * time.Minute).Unix(),
		"iat": time.Now().Unix(),
	})

	req := httptest.NewRequest("GET", "https://my-cluster.example.com/", nil)
	req.Host = "my-cluster.example.com"
	req.Header.Set("Authorization", "Bearer "+token)

	identity, err := auth.Authenticate(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if identity == nil {
		t.Fatal("expected identity, got nil")
	}
}

func TestOIDCAuthenticator_AudienceFromTLS(t *testing.T) {
	ctx := context.Background()
	inmem, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	ts := newTestOIDCServer(t)
	defer ts.Close()

	app := &core_v1alpha.App{}
	_, err := inmem.Client.Create(ctx, "my-app", app)
	if err != nil {
		t.Fatalf("failed to create app: %v", err)
	}

	var appRec core_v1alpha.App
	if err := inmem.Client.Get(ctx, "my-app", &appRec); err != nil {
		t.Fatalf("failed to get app: %v", err)
	}

	binding := &core_v1alpha.OidcBinding{
		App:            appRec.EntityId(),
		Provider:       "github",
		Issuer:         ts.URL(),
		SubjectPattern: "repo:acme/app:*",
	}
	_, err = inmem.Client.Create(ctx, "oidcb-aud2", binding)
	if err != nil {
		t.Fatalf("failed to create OIDC binding: %v", err)
	}

	auth := NewOIDCAuthenticator(testutils.TestLogger(t))
	auth.SetEAC(inmem.EAC)

	// Token with audience matching the TLS ServerName
	token := ts.SignToken(jwt.MapClaims{
		"iss": ts.URL(),
		"sub": "repo:acme/app:ref:refs/heads/main",
		"aud": "tls-server.example.com",
		"exp": time.Now().Add(10 * time.Minute).Unix(),
		"iat": time.Now().Unix(),
	})

	req := httptest.NewRequest("GET", "https://tls-server.example.com/", nil)
	req.Host = "" // Empty Host, should fall back to TLS ServerName
	req.TLS = &tls.ConnectionState{ServerName: "tls-server.example.com"}
	req.Header.Set("Authorization", "Bearer "+token)

	identity, err := auth.Authenticate(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if identity == nil {
		t.Fatal("expected identity, got nil")
	}
}

func TestOIDCAuthenticator_WrongAudience(t *testing.T) {
	ctx := context.Background()
	inmem, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	ts := newTestOIDCServer(t)
	defer ts.Close()

	app := &core_v1alpha.App{}
	_, err := inmem.Client.Create(ctx, "my-app", app)
	if err != nil {
		t.Fatalf("failed to create app: %v", err)
	}

	var appRec core_v1alpha.App
	if err := inmem.Client.Get(ctx, "my-app", &appRec); err != nil {
		t.Fatalf("failed to get app: %v", err)
	}

	binding := &core_v1alpha.OidcBinding{
		App:            appRec.EntityId(),
		Provider:       "github",
		Issuer:         ts.URL(),
		SubjectPattern: "repo:acme/app:*",
	}
	_, err = inmem.Client.Create(ctx, "oidcb-wrongaud", binding)
	if err != nil {
		t.Fatalf("failed to create OIDC binding: %v", err)
	}

	auth := NewOIDCAuthenticator(testutils.TestLogger(t))
	auth.SetEAC(inmem.EAC)

	// Token has wrong audience
	token := ts.SignToken(jwt.MapClaims{
		"iss": ts.URL(),
		"sub": "repo:acme/app:ref:refs/heads/main",
		"aud": "wrong-audience",
		"exp": time.Now().Add(10 * time.Minute).Unix(),
		"iat": time.Now().Unix(),
	})

	req := httptest.NewRequest("GET", "https://test-host/", nil)
	req.Host = "test-host"
	req.Header.Set("Authorization", "Bearer "+token)

	identity, err := auth.Authenticate(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if identity != nil {
		t.Error("expected nil identity for wrong audience")
	}
}
