package connectors

import (
	"net/url"
	"strings"
	"testing"
)

func TestGitHubConnector_LoginURL(t *testing.T) {
	callback := "https://app.example.com/.well-known/miren/oidc/callback"

	conn, err := NewGitHub(GitHubConfig{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURI:  callback,
		Orgs:         []GitHubOrg{{Name: "mirendev"}},
	}, nil)
	if err != nil {
		t.Fatalf("NewGitHub: %v", err)
	}

	loginURL, connData, err := conn.LoginURL(callback, "the-state")
	if err != nil {
		t.Fatalf("LoginURL: %v", err)
	}

	u, err := url.Parse(loginURL)
	if err != nil {
		t.Fatalf("parse login url %q: %v", loginURL, err)
	}

	if u.Host != "github.com" {
		t.Errorf("login URL host = %q, want github.com", u.Host)
	}
	if got := u.Query().Get("client_id"); got != "test-client-id" {
		t.Errorf("client_id = %q, want test-client-id", got)
	}
	if got := u.Query().Get("state"); got != "the-state" {
		t.Errorf("state = %q, want the-state", got)
	}
	if got := u.Query().Get("redirect_uri"); got != callback {
		t.Errorf("redirect_uri = %q, want %q", got, callback)
	}
	scope := u.Query().Get("scope")
	if !strings.Contains(scope, "user:email") {
		t.Errorf("scope = %q, missing user:email", scope)
	}
	// Orgs are set, so read:org should be requested.
	if !strings.Contains(scope, "read:org") {
		t.Errorf("scope = %q, missing read:org (required when orgs are configured)", scope)
	}

	// connData is opaque, but for the GitHub connector it's nil pre-callback
	// (it carries the access token, populated on HandleCallback). We just
	// want to confirm the wrapper doesn't choke on a nil round-trip.
	if connData != nil {
		t.Errorf("connData = %v, want nil pre-callback for github", connData)
	}
}

func TestGitHubConnector_LoginURL_RedirectMismatch(t *testing.T) {
	conn, err := NewGitHub(GitHubConfig{
		ClientID:     "id",
		ClientSecret: "secret",
		RedirectURI:  "https://a.example.com/cb",
		Orgs:         []GitHubOrg{{Name: "mirendev"}},
	}, nil)
	if err != nil {
		t.Fatalf("NewGitHub: %v", err)
	}

	// Dex's GitHub connector validates callbackURL matches the configured
	// RedirectURI. We rely on this in httpingress by caching one Connector
	// per (route, baseURL).
	_, _, err = conn.LoginURL("https://b.example.com/cb", "state")
	if err == nil {
		t.Fatal("LoginURL with mismatched callback should error")
	}
}

func TestIdentity_Claims(t *testing.T) {
	id := Identity{
		UserID:            "1234",
		Username:          "phinze",
		PreferredUsername: "phinze",
		Email:             "paul@miren.dev",
		EmailVerified:     true,
		Groups:            []string{"mirendev:engineering"},
	}

	claims := id.Claims()
	if claims["sub"] != "1234" {
		t.Errorf("sub = %v", claims["sub"])
	}
	if claims["email"] != "paul@miren.dev" {
		t.Errorf("email = %v", claims["email"])
	}
	groups, ok := claims["groups"].([]string)
	if !ok || len(groups) != 1 || groups[0] != "mirendev:engineering" {
		t.Errorf("groups = %v", claims["groups"])
	}

	// Empty groups should not surface — injectClaims relies on "missing key
	// means skip the header" to keep X-User-Groups from materializing the
	// literal string "null" for users with no org/team membership.
	empty := Identity{UserID: "1234"}.Claims()
	if _, present := empty["groups"]; present {
		t.Errorf("groups should be omitted for empty membership, got %v", empty["groups"])
	}
}

func TestNewGitHub_RequiresCredentials(t *testing.T) {
	_, err := NewGitHub(GitHubConfig{
		RedirectURI: "https://app.example.com/cb",
	}, nil)
	if err == nil {
		t.Fatal("NewGitHub with no client id/secret should error")
	}

	_, err = NewGitHub(GitHubConfig{
		ClientID:     "id",
		ClientSecret: "secret",
	}, nil)
	if err == nil {
		t.Fatal("NewGitHub with no redirect uri should error")
	}
}
