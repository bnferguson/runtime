package outboard

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"
	"strings"
)

// TokenAuthenticator implements rpc.Authenticator using a shared bearer token.
// It validates the Authorization header using constant-time comparison.
type TokenAuthenticator struct {
	token string
}

// NewTokenAuthenticator creates a new TokenAuthenticator with the given token.
func NewTokenAuthenticator(token string) *TokenAuthenticator {
	return &TokenAuthenticator{token: token}
}

func (t *TokenAuthenticator) AuthenticateRequest(ctx context.Context, r *http.Request) (bool, string, error) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return false, "", fmt.Errorf("missing authorization header")
	}

	const prefix = "Bearer "
	if !strings.HasPrefix(auth, prefix) {
		return false, "", fmt.Errorf("invalid authorization scheme")
	}

	provided := auth[len(prefix):]
	if subtle.ConstantTimeCompare([]byte(provided), []byte(t.token)) != 1 {
		return false, "", fmt.Errorf("invalid token")
	}

	return true, "outboard", nil
}

func (t *TokenAuthenticator) NoAuthorization(ctx context.Context, r *http.Request) (bool, string, error) {
	return false, "", fmt.Errorf("authorization required")
}
