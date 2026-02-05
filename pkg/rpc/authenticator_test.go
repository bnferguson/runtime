package rpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"net/http"
	"testing"
)

// TestAuthenticator verifies the authenticator integration
func TestAuthenticator(t *testing.T) {
	tests := []struct {
		name          string
		authHeader    string
		expectAllowed bool
		authenticator Authenticator
	}{
		{
			name:          "NoOpAuthenticator allows all requests",
			authHeader:    "",
			expectAllowed: true,
			authenticator: &NoOpAuthenticator{},
		},
		{
			name:          "NoOpAuthenticator allows with auth header",
			authHeader:    "Bearer token123",
			expectAllowed: true,
			authenticator: &NoOpAuthenticator{},
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

			if tt.authHeader != "" {
				allowed, identity, err := tt.authenticator.AuthenticateRequest(context.Background(), req)
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if allowed != tt.expectAllowed {
					t.Errorf("expected allowed=%v, got %v", tt.expectAllowed, allowed)
				}
				if allowed && identity == "" {
					t.Error("expected non-empty identity for allowed request")
				}
			} else {
				allowed, identity, err := tt.authenticator.NoAuthorization(context.Background(), req)
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if allowed != tt.expectAllowed {
					t.Errorf("expected allowed=%v, got %v", tt.expectAllowed, allowed)
				}
				if allowed && identity == "" {
					t.Error("expected non-empty identity for allowed request")
				}
			}
		})
	}
}

// TestLocalOnlyAuthenticator verifies the LocalOnlyAuthenticator behavior
func TestLocalOnlyAuthenticator(t *testing.T) {
	// Create a mock certificate for testing
	mockCert := &x509.Certificate{
		Subject: pkix.Name{
			CommonName: "test-client",
		},
	}

	tests := []struct {
		name           string
		path           string
		authHeader     string
		hasCert        bool
		expectAllowed  bool
		expectIdentity string
		expectError    bool
	}{
		// Non-RPC paths require certificates
		{
			name:          "rejects non-RPC request without certificate",
			path:          "/api/health",
			authHeader:    "",
			hasCert:       false,
			expectAllowed: false,
			expectError:   true,
		},
		{
			name:          "rejects non-RPC request with auth header but no certificate",
			path:          "/api/health",
			authHeader:    "Bearer token123",
			hasCert:       false,
			expectAllowed: false,
			expectError:   true,
		},
		{
			name:           "allows non-RPC request with certificate",
			path:           "/api/health",
			authHeader:     "",
			hasCert:        true,
			expectAllowed:  true,
			expectIdentity: "test-client",
		},
		{
			name:           "allows non-RPC request with certificate and auth header",
			path:           "/api/health",
			authHeader:     "Bearer token123",
			hasCert:        true,
			expectAllowed:  true,
			expectIdentity: "test-client",
		},
		// RPC paths are allowed through (RPC layer handles auth)
		{
			name:           "allows RPC request without certificate (RPC layer handles auth)",
			path:           "/_rpc/call/test/method",
			authHeader:     "",
			hasCert:        false,
			expectAllowed:  true,
			expectIdentity: "",
		},
		{
			name:           "allows RPC request with certificate",
			path:           "/_rpc/call/test/method",
			authHeader:     "",
			hasCert:        true,
			expectAllowed:  true,
			expectIdentity: "test-client",
		},
	}

	auth := &LocalOnlyAuthenticator{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("POST", tt.path, nil)
			if err != nil {
				t.Fatal(err)
			}

			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			if tt.hasCert {
				req.TLS = &tls.ConnectionState{
					PeerCertificates: []*x509.Certificate{mockCert},
				}
			}

			var allowed bool
			var identity string

			if tt.authHeader != "" {
				allowed, identity, err = auth.AuthenticateRequest(context.Background(), req)
			} else {
				allowed, identity, err = auth.NoAuthorization(context.Background(), req)
			}

			if err != nil {
				if !tt.expectError {
					t.Errorf("unexpected error: %v", err)
				}
			}
			if allowed != tt.expectAllowed {
				t.Errorf("expected allowed=%v, got %v", tt.expectAllowed, allowed)
			}
			if tt.expectAllowed && identity != tt.expectIdentity {
				t.Errorf("expected identity=%q, got %q", tt.expectIdentity, identity)
			}
		})
	}
}
