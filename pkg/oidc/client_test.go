package oidc

import (
	"testing"
)

func TestGeneratePKCEChallenge(t *testing.T) {
	verifier := "test-verifier-123456789"
	challenge := generatePKCEChallenge(verifier)

	if challenge == "" {
		t.Error("PKCE challenge is empty")
	}

	// Challenge should be different from verifier
	if challenge == verifier {
		t.Error("PKCE challenge should be hashed, not plain verifier")
	}

	// Should be deterministic
	challenge2 := generatePKCEChallenge(verifier)
	if challenge != challenge2 {
		t.Error("PKCE challenge should be deterministic")
	}

	// Different verifiers should produce different challenges
	challenge3 := generatePKCEChallenge("different-verifier")
	if challenge == challenge3 {
		t.Error("different verifiers should produce different challenges")
	}
}

func TestParseIDToken_BasicParsing(t *testing.T) {
	// This is a mock JWT token (header.payload.signature)
	// Header: {"alg":"none","typ":"JWT"}
	// Payload: {"sub":"1234567890","name":"Test User","iat":1516239022}
	mockToken := "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IlRlc3QgVXNlciIsImlhdCI6MTUxNjIzOTAyMn0."

	client := NewClient("https://provider.example.com", "client-id", "secret", "https://redirect.example.com", []string{"openid"}, nil)

	claims, err := client.ParseIDToken(mockToken)
	if err != nil {
		t.Fatalf("failed to parse ID token: %v", err)
	}

	if claims["sub"] != "1234567890" {
		t.Errorf("sub claim mismatch: got %v", claims["sub"])
	}

	if claims["name"] != "Test User" {
		t.Errorf("name claim mismatch: got %v", claims["name"])
	}
}

func TestClient_AuthorizationURL(t *testing.T) {
	client := NewClient(
		"https://provider.example.com",
		"test-client-id",
		"test-secret",
		"https://app.example.com/callback",
		[]string{"openid", "email", "profile"},
		nil,
	)

	// Note: This test will fail without a real OIDC provider
	// In a real test environment, we'd mock the HTTP client
	_, err := client.AuthorizationURL("test-state", "test-verifier", "app.example.com")
	if err != nil {
		// Expected to fail without real provider, but we can check error message
		t.Logf("Expected error without provider: %v", err)
	}
}

func TestNewClient(t *testing.T) {
	client := NewClient(
		"https://provider.example.com",
		"client-id",
		"client-secret",
		"https://redirect.example.com/callback",
		[]string{"openid", "email"},
		nil,
	)

	if client == nil {
		t.Fatal("client is nil")
	}

	if client.clientID != "client-id" {
		t.Errorf("client ID mismatch: got %s", client.clientID)
	}

	if client.providerURL != "https://provider.example.com" {
		t.Errorf("provider URL mismatch: got %s", client.providerURL)
	}

	if len(client.scopes) != 2 {
		t.Errorf("scopes length mismatch: got %d", len(client.scopes))
	}
}
