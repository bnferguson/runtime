package httpingress

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"miren.dev/runtime/api/ingress/ingress_v1alpha"
	"miren.dev/runtime/pkg/entity"
	"miren.dev/runtime/pkg/oidc"
)

const (
	// Well-known path for OIDC callback
	oidcCallbackPath = "/.well-known/miren/oidc/callback"
)

// oidcHandler manages OIDC authentication for a route
type oidcHandler struct {
	route          *ingress_v1alpha.HttpRoute
	provider       *ingress_v1alpha.OidcProvider
	client         *oidc.Client
	sessionManager *oidc.SessionManager
	logger         *slog.Logger
}

// newOIDCHandler creates a new OIDC handler for a route with a resolved provider
func newOIDCHandler(route *ingress_v1alpha.HttpRoute, provider *ingress_v1alpha.OidcProvider, baseURL string, logger *slog.Logger) (*oidcHandler, error) {
	if provider.ProviderUrl == "" || provider.ClientId == "" {
		return nil, fmt.Errorf("OIDC provider missing required fields")
	}

	// Parse scopes
	scopes := []string{"openid"}
	if provider.Scopes != "" {
		scopes = strings.Fields(provider.Scopes)
	}

	// Build redirect URL
	redirectURL := fmt.Sprintf("%s%s", baseURL, oidcCallbackPath)

	// Create OIDC client
	client := oidc.NewClient(
		provider.ProviderUrl,
		provider.ClientId,
		provider.ClientSecret,
		redirectURL,
		scopes,
		logger,
	)

	// Create session manager
	// Use secure cookies if we're on HTTPS
	cookieSecure := strings.HasPrefix(baseURL, "https://")
	sessionManager := oidc.NewSessionManager(cookieSecure, "")

	return &oidcHandler{
		route:          route,
		provider:       provider,
		client:         client,
		sessionManager: sessionManager,
		logger:         logger.With("module", "oidc", "host", route.Host, "provider", provider.Name),
	}, nil
}

// checkAuth verifies if the request has a valid OIDC session.
// Returns true if authenticated, false if redirect to OIDC provider is needed.
func (h *oidcHandler) checkAuth(w http.ResponseWriter, r *http.Request) (authenticated bool, claims map[string]interface{}) {
	// Check for existing session
	session, err := h.sessionManager.GetSession(r)
	if err != nil {
		h.logger.Error("failed to get session", "error", err)
		return false, nil
	}

	if session != nil {
		// Parse claims from ID token
		claims, err := h.client.ParseIDToken(session.IDToken)
		if err != nil {
			h.logger.Error("failed to parse ID token", "error", err)
			// Clear invalid session and redirect to login
			h.sessionManager.ClearSession(w)
			return false, nil
		}
		return true, claims
	}

	return false, nil
}

// redirectToProvider redirects the user to the OIDC provider for authentication
func (h *oidcHandler) redirectToProvider(w http.ResponseWriter, r *http.Request) {
	// Generate state and PKCE verifier
	state, err := h.sessionManager.GenerateState(r.URL.Path)
	if err != nil {
		h.logger.Error("failed to generate state", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Store state in cookie
	if err := h.sessionManager.SetState(w, state); err != nil {
		h.logger.Error("failed to set state", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Generate authorization URL
	authURL, err := h.client.AuthorizationURL(state.State, state.PKCEVerifier)
	if err != nil {
		h.logger.Error("failed to generate auth URL", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Redirect to provider
	http.Redirect(w, r, authURL, http.StatusFound)
}

// handleCallback processes the OAuth2 callback from the OIDC provider
func (h *oidcHandler) handleCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get state from cookie
	state, err := h.sessionManager.GetState(r)
	if err != nil {
		h.logger.Error("failed to get state", "error", err)
		http.Error(w, "Invalid state", http.StatusBadRequest)
		return
	}

	// Verify state parameter matches
	queryState := r.URL.Query().Get("state")
	if queryState != state.State {
		h.logger.Error("state mismatch", "expected", state.State, "got", queryState)
		http.Error(w, "State mismatch", http.StatusBadRequest)
		return
	}

	// Check for error from provider
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		errDesc := r.URL.Query().Get("error_description")
		h.logger.Error("OIDC provider error", "error", errParam, "description", errDesc)
		http.Error(w, fmt.Sprintf("Authentication failed: %s", errDesc), http.StatusBadRequest)
		return
	}

	// Get authorization code
	code := r.URL.Query().Get("code")
	if code == "" {
		h.logger.Error("authorization code missing")
		http.Error(w, "Authorization code missing", http.StatusBadRequest)
		return
	}

	// Exchange code for tokens
	token, err := h.client.ExchangeCode(ctx, code, state.PKCEVerifier)
	if err != nil {
		h.logger.Error("failed to exchange code", "error", err)
		http.Error(w, "Failed to exchange authorization code", http.StatusInternalServerError)
		return
	}

	// Extract ID token
	idToken, ok := token.Extra("id_token").(string)
	if !ok || idToken == "" {
		h.logger.Error("ID token missing from token response")
		http.Error(w, "ID token missing", http.StatusInternalServerError)
		return
	}

	// Parse claims from ID token (without verification for now)
	claims, err := h.client.ParseIDToken(idToken)
	if err != nil {
		h.logger.Error("failed to parse ID token", "error", err)
		http.Error(w, "Failed to parse ID token", http.StatusInternalServerError)
		return
	}

	// Create session
	session := &oidc.SessionData{
		IDToken:      idToken,
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		Claims:       claims,
	}

	// Store session
	if err := h.sessionManager.SetSession(w, session); err != nil {
		h.logger.Error("failed to set session", "error", err)
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	// Clear state cookie
	h.sessionManager.ClearState(w)

	// Redirect to original path
	returnPath := state.ReturnPath
	if returnPath == "" {
		returnPath = "/"
	}

	h.logger.Info("OIDC authentication successful", "subject", claims["sub"], "return_path", returnPath)
	http.Redirect(w, r, returnPath, http.StatusFound)
}

// injectClaims adds JWT claims as HTTP headers based on the route configuration
func (h *oidcHandler) injectClaims(r *http.Request, claims map[string]interface{}) {
	for _, mapping := range h.route.ClaimMappings {
		if mapping.Claim == "" || mapping.Header == "" {
			continue
		}

		// Get claim value
		value, ok := claims[mapping.Claim]
		if !ok {
			continue
		}

		// Convert to string
		var strValue string
		switch v := value.(type) {
		case string:
			strValue = v
		case float64:
			strValue = fmt.Sprintf("%v", v)
		case bool:
			strValue = fmt.Sprintf("%v", v)
		default:
			// For complex types, skip
			continue
		}

		// Set header
		r.Header.Set(mapping.Header, strValue)
	}
}

// oidcMiddleware wraps the request handling with OIDC authentication
func (s *Server) oidcMiddleware(route *ingress_v1alpha.HttpRoute, next http.HandlerFunc) http.HandlerFunc {
	// If no OIDC provider reference, just pass through
	if entity.Empty(route.OidcProvider) {
		return next
	}

	// Resolve the OIDC provider entity
	ctx := context.Background()
	resp, err := s.eac.Get(ctx, string(route.OidcProvider))
	if err != nil {
		s.Log.Error("failed to get OIDC provider", "error", err, "provider_id", route.OidcProvider)
		return next
	}

	var provider ingress_v1alpha.OidcProvider
	provider.Decode(resp.Entity().Entity())

	// Build base URL from host
	// In production, this should use the actual scheme (http/https)
	baseURL := fmt.Sprintf("https://%s", route.Host)

	// Create OIDC handler
	handler, err := newOIDCHandler(route, &provider, baseURL, s.Log)
	if err != nil {
		s.Log.Error("failed to create OIDC handler", "error", err, "host", route.Host)
		return next
	}

	return func(w http.ResponseWriter, r *http.Request) {
		// Handle callback path specially
		if r.URL.Path == oidcCallbackPath {
			handler.handleCallback(w, r)
			return
		}

		// Check authentication
		authenticated, claims := handler.checkAuth(w, r)
		if !authenticated {
			handler.redirectToProvider(w, r)
			return
		}

		// Inject claims as headers
		handler.injectClaims(r, claims)

		// Continue to app
		next(w, r)
	}
}
