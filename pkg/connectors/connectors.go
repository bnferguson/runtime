// Package connectors wraps github.com/dexidp/dex/connector with a small
// Miren-flavored interface. Route protection uses these to authenticate
// users against upstream providers that don't speak OIDC (e.g. GitHub).
//
// The wrapper layer exists to keep our existing httpingress code unaware
// of Dex's Scopes/connData/Identity types and to give us a stable shape
// for adding more connectors over time without leaking Dex internals.
package connectors

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	dexconnector "github.com/dexidp/dex/connector"
	dexgithub "github.com/dexidp/dex/connector/github"
)

// Identity is the normalized result of a successful connector login.
type Identity struct {
	UserID            string
	Username          string
	PreferredUsername string
	Email             string
	EmailVerified     bool
	Groups            []string

	// ConnectorData is opaque state that the underlying connector wants
	// to round-trip into a future Refresh call. Currently unused by the
	// route-protection flow; kept on the type so we can plumb it through
	// the session cookie later without changing the interface.
	ConnectorData []byte
}

// Claims renders the identity as the map[string]interface{} shape that
// httpingress.injectClaims walks for claim-header mapping. The keys mirror
// the OIDC claim names so route operators can mix providers transparently.
func (i Identity) Claims() map[string]any {
	claims := map[string]any{
		"sub":                i.UserID,
		"email":              i.Email,
		"email_verified":     i.EmailVerified,
		"name":               i.Username,
		"preferred_username": i.PreferredUsername,
	}
	// Only surface groups when the upstream actually returned membership.
	// injectClaims treats missing keys as "skip the header", so omitting
	// here keeps an empty-groups identity from materializing the literal
	// string "null" in X-User-Groups.
	if len(i.Groups) > 0 {
		claims["groups"] = i.Groups
	}
	return claims
}

// Connector is the Miren-side abstraction over Dex's CallbackConnector.
// The wrapper hides connector.Scopes (we always request groups), and
// normalizes the (loginURL, connData, error) tuple from LoginURL so the
// caller round-trips connData through the state cookie alongside the
// PKCE/state values already living there.
type Connector interface {
	// LoginURL returns the upstream URL to redirect the user to. The
	// returned connData blob must be persisted with the OAuth state and
	// handed back to HandleCallback unchanged.
	LoginURL(callbackURL, state string) (loginURL string, connData []byte, err error)

	// HandleCallback processes the OAuth callback request and returns the
	// resolved Identity.
	HandleCallback(ctx context.Context, connData []byte, r *http.Request) (Identity, error)
}

// GitHubConfig configures the GitHub connector. RedirectURI is pinned at
// construction time because Dex's connector validates it matches the
// callbackURL passed to LoginURL — callers should cache one Connector per
// (route, baseURL) pair.
type GitHubConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string

	// Orgs restricts login to members of these GitHub organizations.
	// If an Org includes Teams, the user must belong to one of those teams.
	Orgs []GitHubOrg

	// UseLoginAsID, when true, surfaces the GitHub login (e.g. "phinze")
	// as Identity.UserID instead of the numeric user ID. Defaults to false
	// to match OIDC's stable-`sub` convention.
	UseLoginAsID bool
}

type GitHubOrg struct {
	Name  string
	Teams []string
}

// NewGitHub returns a Connector backed by github.com/dexidp/dex/connector/github.
func NewGitHub(cfg GitHubConfig, logger *slog.Logger) (Connector, error) {
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil, fmt.Errorf("github connector: ClientID and ClientSecret required")
	}
	if cfg.RedirectURI == "" {
		return nil, fmt.Errorf("github connector: RedirectURI required")
	}
	if logger == nil {
		logger = slog.Default()
	}

	orgs := make([]dexgithub.Org, 0, len(cfg.Orgs))
	for _, o := range cfg.Orgs {
		orgs = append(orgs, dexgithub.Org{Name: o.Name, Teams: o.Teams})
	}

	dexCfg := &dexgithub.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURI:  cfg.RedirectURI,
		Orgs:         orgs,
		UseLoginAsID: cfg.UseLoginAsID,
	}

	raw, err := dexCfg.Open("miren-github", logger.With("connector", "github"))
	if err != nil {
		return nil, fmt.Errorf("github connector: open: %w", err)
	}

	cb, ok := raw.(dexconnector.CallbackConnector)
	if !ok {
		return nil, fmt.Errorf("github connector: dex returned a %T, expected CallbackConnector", raw)
	}

	return &dexAdapter{inner: cb, name: "github"}, nil
}

// dexAdapter bridges dexconnector.CallbackConnector to our Connector
// interface. It pins a Scopes value with Groups=true since route
// protection's primary use of upstream data is org/team membership.
type dexAdapter struct {
	inner dexconnector.CallbackConnector
	name  string
}

func (d *dexAdapter) LoginURL(callbackURL, state string) (string, []byte, error) {
	url, connData, err := d.inner.LoginURL(dexconnector.Scopes{Groups: true}, callbackURL, state)
	if err != nil {
		return "", nil, fmt.Errorf("%s connector: login url: %w", d.name, err)
	}
	return url, connData, nil
}

func (d *dexAdapter) HandleCallback(ctx context.Context, connData []byte, r *http.Request) (Identity, error) {
	// Dex's CallbackConnector.HandleCallback doesn't take a context — the
	// connector uses r.Context() internally. We accept ctx in our interface
	// for forward-compatibility and to keep call sites idiomatic.
	_ = ctx

	id, err := d.inner.HandleCallback(dexconnector.Scopes{Groups: true}, connData, r)
	if err != nil {
		return Identity{}, fmt.Errorf("%s connector: handle callback: %w", d.name, err)
	}
	return Identity{
		UserID:            id.UserID,
		Username:          id.Username,
		PreferredUsername: id.PreferredUsername,
		Email:             id.Email,
		EmailVerified:     id.EmailVerified,
		Groups:            id.Groups,
		ConnectorData:     id.ConnectorData,
	}, nil
}
