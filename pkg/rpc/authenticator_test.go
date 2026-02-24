package rpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"net/http"
	"testing"
)

// TestNoOpAuthenticator verifies that NoOpAuthenticator always returns an anonymous identity
func TestNoOpAuthenticator(t *testing.T) {
	auth := &NoOpAuthenticator{}

	tests := []struct {
		name       string
		authHeader string
	}{
		{
			name:       "no auth header",
			authHeader: "",
		},
		{
			name:       "with bearer token",
			authHeader: "Bearer token123",
		},
		{
			name:       "with basic auth",
			authHeader: "Basic dXNlcjpwYXNz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("POST", "/_rpc/call/test/method", nil)
			if err != nil {
				t.Fatal(err)
			}

			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			identity, err := auth.Authenticate(context.Background(), req)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if identity == nil {
				t.Error("expected non-nil identity")
				return
			}
			if identity.Subject != "anonymous" {
				t.Errorf("expected subject=anonymous, got %q", identity.Subject)
			}
			if identity.Method != AuthMethodAnonymous {
				t.Errorf("expected method=%v, got %v", AuthMethodAnonymous, identity.Method)
			}
		})
	}
}

// TestLocalOnlyAuthenticator verifies the LocalOnlyAuthenticator behavior
func TestLocalOnlyAuthenticator(t *testing.T) {
	mockCert := &x509.Certificate{
		Subject: pkix.Name{
			CommonName: "test-client",
		},
	}

	tests := []struct {
		name           string
		hasCert        bool
		expectIdentity bool
		expectSubject  string
		expectMethod   AuthMethod
	}{
		{
			name:           "with certificate returns identity",
			hasCert:        true,
			expectIdentity: true,
			expectSubject:  "test-client",
			expectMethod:   AuthMethodCert,
		},
		{
			name:           "without certificate returns nil",
			hasCert:        false,
			expectIdentity: false,
		},
		{
			name:           "TLS without peer certs returns nil",
			hasCert:        false,
			expectIdentity: false,
		},
	}

	auth := &LocalOnlyAuthenticator{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("POST", "/_rpc/call/test/method", nil)
			if err != nil {
				t.Fatal(err)
			}

			if tt.hasCert {
				req.TLS = &tls.ConnectionState{
					PeerCertificates: []*x509.Certificate{mockCert},
				}
			} else if tt.name == "TLS without peer certs returns nil" {
				// TLS connection but no peer certificates
				req.TLS = &tls.ConnectionState{}
			}

			identity, err := auth.Authenticate(context.Background(), req)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if tt.expectIdentity {
				if identity == nil {
					t.Error("expected non-nil identity")
					return
				}
				if identity.Subject != tt.expectSubject {
					t.Errorf("expected subject=%q, got %q", tt.expectSubject, identity.Subject)
				}
				if identity.Method != tt.expectMethod {
					t.Errorf("expected method=%v, got %v", tt.expectMethod, identity.Method)
				}
			} else {
				if identity != nil {
					t.Errorf("expected nil identity, got %+v", identity)
				}
			}
		})
	}
}

// TestIdentityContext verifies the context helpers for identity propagation
func TestIdentityContext(t *testing.T) {
	t.Run("stores and retrieves identity", func(t *testing.T) {
		identity := &Identity{
			Subject: "test-user",
			Groups:  []string{"admin", "users"},
			Method:  AuthMethodJWT,
			Metadata: map[string]any{
				"organization_id": "org-123",
			},
		}

		ctx := ContextWithIdentity(context.Background(), identity)
		retrieved := IdentityFromContext(ctx)

		if retrieved == nil {
			t.Fatal("expected non-nil identity from context")
			return
		}
		if retrieved.Subject != identity.Subject {
			t.Errorf("expected subject=%q, got %q", identity.Subject, retrieved.Subject)
		}
		if len(retrieved.Groups) != len(identity.Groups) {
			t.Errorf("expected %d groups, got %d", len(identity.Groups), len(retrieved.Groups))
		}
		if retrieved.Method != identity.Method {
			t.Errorf("expected method=%v, got %v", identity.Method, retrieved.Method)
		}
		if retrieved.Metadata["organization_id"] != identity.Metadata["organization_id"] {
			t.Errorf("expected org_id=%v, got %v",
				identity.Metadata["organization_id"],
				retrieved.Metadata["organization_id"])
		}
	})

	t.Run("returns nil for empty context", func(t *testing.T) {
		retrieved := IdentityFromContext(context.Background())
		if retrieved != nil {
			t.Errorf("expected nil identity, got %+v", retrieved)
		}
	})
}
