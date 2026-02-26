package oidcauth

import (
	"context"
	"fmt"
	"net/http"

	"miren.dev/runtime/pkg/rpc"
)

// oidcDeployRole defines the RPC interfaces and actions allowed for OIDC-authenticated callers.
var oidcDeployRole = map[string]map[string]bool{
	"deployment": {
		"deployversion":              true,
		"createdeployment":           true,
		"updatedeploymentstatus":     true,
		"updatedeploymentphase":      true,
		"listdeployments":            true,
		"getdeploymentbyid":          true,
		"updatefaileddeployment":     true,
		"getactivedeployment":        true,
		"canceldeployment":           true,
		"updatedeploymentappversion": true,
	},
	"logs": {
		"applogs":         true,
		"streamlogs":      true,
		"streamlogchunks": true,
	},
	"crud": {
		"list":             true,
		"getconfiguration": true,
	},
	"builder": {
		"buildfromtar": true,
		"analyzeapp":   true,
	},
	"telemetry": {
		"reportspans": true,
	},
	"appstatus": {
		"appinfo": true,
	},
}

// CompositeAuthenticator chains a primary authenticator with the OIDC authenticator.
// It tries the primary first and falls back to OIDC.
type CompositeAuthenticator struct {
	primary rpc.Authenticator
	oidc    *OIDCAuthenticator
}

// NewCompositeAuthenticator creates a composite authenticator that chains primary and OIDC auth.
func NewCompositeAuthenticator(primary rpc.Authenticator, oidc *OIDCAuthenticator) *CompositeAuthenticator {
	return &CompositeAuthenticator{primary: primary, oidc: oidc}
}

func (c *CompositeAuthenticator) Authenticate(ctx context.Context, r *http.Request) (*rpc.Identity, error) {
	// Try primary authenticator first
	identity, err := c.primary.Authenticate(ctx, r)
	if err != nil {
		return nil, err
	}
	if identity != nil {
		return identity, nil
	}

	// Fall back to OIDC authenticator
	return c.oidc.Authenticate(ctx, r)
}

// CompositeAuthorizer handles authorization for both primary and OIDC auth methods.
type CompositeAuthorizer struct {
	primary rpc.Authorizer
}

// NewCompositeAuthorizer creates a composite authorizer that handles both primary and OIDC authorization.
func NewCompositeAuthorizer(primary rpc.Authorizer) *CompositeAuthorizer {
	return &CompositeAuthorizer{primary: primary}
}

func (c *CompositeAuthorizer) Authorize(ctx context.Context, identity *rpc.Identity, resource, action string) error {
	switch identity.Method {
	case rpc.AuthMethodCert:
		// Local/internal callers bypass all checks
		return nil

	case rpc.AuthMethodOIDC:
		// OIDC callers are restricted to the oidc-deploy role
		return authorizeOIDC(resource, action)

	default:
		// JWT and other methods → delegate to primary (cloud RBAC)
		if c.primary != nil {
			return c.primary.Authorize(ctx, identity, resource, action)
		}
		return nil
	}
}

func authorizeOIDC(resource, action string) error {
	actions, ok := oidcDeployRole[resource]
	if !ok {
		return fmt.Errorf("OIDC access denied: resource %q not permitted", resource)
	}
	if !actions[action] {
		return fmt.Errorf("OIDC access denied: action %q on resource %q not permitted", action, resource)
	}
	return nil
}
