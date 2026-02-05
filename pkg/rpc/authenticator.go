package rpc

import (
	"context"
	"net/http"
)

// contextKey is a private type for context keys to avoid collisions
type contextKey string

const (
	// identityContextKey is the context key for storing the authenticated Identity
	identityContextKey contextKey = "rpc-identity"
)

// IdentityFromContext retrieves the Identity from the context, if present
func IdentityFromContext(ctx context.Context) *Identity {
	if id, ok := ctx.Value(identityContextKey).(*Identity); ok {
		return id
	}
	return nil
}

// ContextWithIdentity returns a new context with the Identity stored
func ContextWithIdentity(ctx context.Context, identity *Identity) context.Context {
	return context.WithValue(ctx, identityContextKey, identity)
}

// AuthMethod indicates how a caller was authenticated
type AuthMethod string

const (
	AuthMethodCert      AuthMethod = "cert"      // TLS client certificate
	AuthMethodJWT       AuthMethod = "jwt"       // JWT token (e.g., from Miren Cloud)
	AuthMethodAnonymous AuthMethod = "anonymous" // No authentication (public methods)
	AuthMethodToken     AuthMethod = "token"     // Bearer token (e.g., outboard)
)

// Identity represents an authenticated caller
type Identity struct {
	// Subject is the primary identifier (cert CN, JWT subject, etc.)
	Subject string

	// Groups contains group memberships (from JWT claims, etc.)
	Groups []string

	// Method indicates how the caller was authenticated
	Method AuthMethod

	// Metadata holds auth-method-specific data (e.g., OrganizationID for cloud auth)
	Metadata map[string]any
}

// Authenticator validates credentials and returns caller identity
type Authenticator interface {
	// Authenticate validates the request's credentials and returns the caller's identity.
	// Returns:
	//   - (*Identity, nil) if credentials are valid
	//   - (nil, nil) if no credentials present or credentials are invalid
	//   - (nil, error) if an error occurred during authentication
	Authenticate(ctx context.Context, r *http.Request) (*Identity, error)
}

// NoOpAuthenticator allows all requests without checking credentials.
// Used for testing only.
type NoOpAuthenticator struct{}

func (n *NoOpAuthenticator) Authenticate(ctx context.Context, r *http.Request) (*Identity, error) {
	return &Identity{
		Subject: "anonymous",
		Method:  AuthMethodAnonymous,
	}, nil
}

// LocalOnlyAuthenticator requires a valid TLS client certificate.
// Used when cloud authentication is not enabled.
type LocalOnlyAuthenticator struct{}

func (l *LocalOnlyAuthenticator) Authenticate(ctx context.Context, r *http.Request) (*Identity, error) {
	// Check for TLS client certificate
	if r.TLS != nil && len(r.TLS.PeerCertificates) > 0 {
		cert := r.TLS.PeerCertificates[0]
		return &Identity{
			Subject: cert.Subject.CommonName,
			Method:  AuthMethodCert,
		}, nil
	}

	// No valid credentials
	return nil, nil
}
