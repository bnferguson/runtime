package rpc

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// Authenticator is an interface for authenticating RPC requests
type Authenticator interface {
	// AuthenticateRequest authenticates an HTTP request and returns whether it's allowed
	// It can check JWT tokens, certificates, or other authentication methods
	AuthenticateRequest(ctx context.Context, r *http.Request) (authenticated bool, identity string, err error)

	// NoAuthorization is called when a request has no Authorization header
	// This allows the authenticator to decide if such requests should be allowed
	// (e.g., based on client certificates or to enforce mandatory authentication)
	NoAuthorization(ctx context.Context, r *http.Request) (allowed bool, identity string, err error)
}

// NoOpAuthenticator is a no-op authenticator that allows all requests
type NoOpAuthenticator struct{}

func (n *NoOpAuthenticator) AuthenticateRequest(ctx context.Context, r *http.Request) (bool, string, error) {
	return true, "anonymous", nil
}

func (n *NoOpAuthenticator) NoAuthorization(ctx context.Context, r *http.Request) (bool, string, error) {
	return true, "anonymous", nil
}

// LocalOnlyAuthenticator requires a valid client certificate for most requests.
// This is used when cloud authentication is not enabled, ensuring that only
// clients with certificates issued by the local CA can access the server.
//
// RPC paths (/_rpc/) are allowed through without TLS certs at this layer because
// the RPC layer handles authentication:
// - Capability-based auth (Ed25519 signatures) is always enforced
// - Per-method auth checks TLS certs for non-public methods
// - Only methods marked public: true in the schema allow unauthenticated access
type LocalOnlyAuthenticator struct{}

func (l *LocalOnlyAuthenticator) AuthenticateRequest(ctx context.Context, r *http.Request) (bool, string, error) {
	// Even with an Authorization header, we require a valid client certificate
	return l.NoAuthorization(ctx, r)
}

func (l *LocalOnlyAuthenticator) NoAuthorization(ctx context.Context, r *http.Request) (bool, string, error) {
	// Extract identity from cert if present
	var identity string
	if r.TLS != nil && len(r.TLS.PeerCertificates) > 0 {
		identity = r.TLS.PeerCertificates[0].Subject.CommonName
	}

	// Allow RPC paths through - the RPC layer handles auth:
	// - Capability signature auth is always enforced
	// - Method-level auth rejects unauthenticated calls to non-public methods
	if strings.HasPrefix(r.URL.Path, "/_rpc/") {
		return true, identity, nil
	}

	// For non-RPC paths, require a valid client certificate
	if identity != "" {
		return true, identity, nil
	}
	return false, "", fmt.Errorf("authentication required")
}
