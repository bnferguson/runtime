package oidcauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsGitHubActions(t *testing.T) {
	t.Run("not in GitHub Actions", func(t *testing.T) {
		t.Setenv("ACTIONS_ID_TOKEN_REQUEST_URL", "")
		t.Setenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN", "")
		if IsGitHubActions() {
			t.Error("expected false when env vars are not set")
		}
	})

	t.Run("only URL set", func(t *testing.T) {
		t.Setenv("ACTIONS_ID_TOKEN_REQUEST_URL", "https://example.com")
		t.Setenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN", "")
		if IsGitHubActions() {
			t.Error("expected false when only URL is set")
		}
	})

	t.Run("both set", func(t *testing.T) {
		t.Setenv("ACTIONS_ID_TOKEN_REQUEST_URL", "https://example.com")
		t.Setenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN", "test-token")
		if !IsGitHubActions() {
			t.Error("expected true when both env vars are set")
		}
	})
}

func TestRequestGitHubToken(t *testing.T) {
	t.Run("successful token request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify the authorization header
			if got := r.Header.Get("Authorization"); got != "bearer test-request-token" {
				t.Errorf("expected bearer test-request-token, got %s", got)
			}

			// Verify the audience parameter
			if got := r.URL.Query().Get("audience"); got != "mycluster.example.com" {
				t.Errorf("expected audience mycluster.example.com, got %s", got)
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"value": "eyJhbGciOiJSUzI1NiJ9.test-token-body.signature",
			})
		}))
		defer server.Close()

		t.Setenv("ACTIONS_ID_TOKEN_REQUEST_URL", server.URL+"?foo=bar")
		t.Setenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN", "test-request-token")

		token, err := RequestGitHubToken(context.Background(), "mycluster.example.com")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if token != "eyJhbGciOiJSUzI1NiJ9.test-token-body.signature" {
			t.Errorf("unexpected token: %s", token)
		}
	})

	t.Run("server error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("forbidden"))
		}))
		defer server.Close()

		t.Setenv("ACTIONS_ID_TOKEN_REQUEST_URL", server.URL+"?foo=bar")
		t.Setenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN", "test-token")

		_, err := RequestGitHubToken(context.Background(), "test")
		if err == nil {
			t.Fatal("expected error for server error response")
		}
	})

	t.Run("empty token response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"value": ""})
		}))
		defer server.Close()

		t.Setenv("ACTIONS_ID_TOKEN_REQUEST_URL", server.URL+"?foo=bar")
		t.Setenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN", "test-token")

		_, err := RequestGitHubToken(context.Background(), "test")
		if err == nil {
			t.Fatal("expected error for empty token")
		}
	})

	t.Run("env vars not set", func(t *testing.T) {
		t.Setenv("ACTIONS_ID_TOKEN_REQUEST_URL", "")
		t.Setenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN", "")

		_, err := RequestGitHubToken(context.Background(), "test")
		if err == nil {
			t.Fatal("expected error when env vars not set")
		}
	})
}
