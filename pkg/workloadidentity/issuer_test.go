package workloadidentity

import (
	"crypto/ed25519"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-jose/go-jose/v4"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewIssuer_GeneratesKey(t *testing.T) {
	dir := t.TempDir()

	iss, err := NewIssuer(IssuerConfig{
		DataPath:       dir,
		IssuerURL:      "https://example.miren.cloud",
		OrganizationID: "org-123",
		ClusterID:      "cluster-456",
	})
	require.NoError(t, err)
	require.NotNil(t, iss)

	assert.Equal(t, "https://example.miren.cloud", iss.IssuerURL())
	assert.NotEmpty(t, iss.kid)

	// Key file should exist
	keyPath := filepath.Join(dir, "server", "workload-identity.key")
	_, err = os.Stat(keyPath)
	assert.NoError(t, err)
}

func TestNewIssuer_LoadsExistingKey(t *testing.T) {
	dir := t.TempDir()

	iss1, err := NewIssuer(IssuerConfig{
		DataPath:  dir,
		IssuerURL: "https://example.miren.cloud",
	})
	require.NoError(t, err)

	iss2, err := NewIssuer(IssuerConfig{
		DataPath:  dir,
		IssuerURL: "https://example.miren.cloud",
	})
	require.NoError(t, err)

	// Same key should produce same kid
	assert.Equal(t, iss1.kid, iss2.kid)
	assert.Equal(t, iss1.publicKey, iss2.publicKey)
}

func TestIssueToken(t *testing.T) {
	dir := t.TempDir()

	iss, err := NewIssuer(IssuerConfig{
		DataPath:       dir,
		IssuerURL:      "https://example.miren.cloud",
		OrganizationID: "org-123",
		ClusterID:      "cluster-456",
	})
	require.NoError(t, err)

	tokenStr, err := iss.IssueToken("myapp", "sandbox-789")
	require.NoError(t, err)
	require.NotEmpty(t, tokenStr)

	// Parse and verify the token
	token, err := jwt.ParseWithClaims(tokenStr, &WorkloadClaims{}, func(tok *jwt.Token) (interface{}, error) {
		assert.Equal(t, jwt.SigningMethodEdDSA, tok.Method)
		assert.Equal(t, iss.kid, tok.Header["kid"])
		return iss.publicKey, nil
	})
	require.NoError(t, err)
	require.True(t, token.Valid)

	claims := token.Claims.(*WorkloadClaims)
	assert.Equal(t, "https://example.miren.cloud", claims.Issuer)
	assert.Equal(t, "org:org-123:app:myapp:sandbox:sandbox-789", claims.Subject)
	assert.Equal(t, jwt.ClaimStrings{"miren"}, claims.Audience)
	assert.Equal(t, "org-123", claims.OrganizationID)
	assert.Equal(t, "cluster-456", claims.ClusterID)
	assert.Equal(t, "myapp", claims.App)
	assert.Equal(t, "sandbox-789", claims.SandboxID)
	assert.NotNil(t, claims.IssuedAt)
	assert.NotNil(t, claims.ExpiresAt)
}

func TestIssueToken_NoOrg(t *testing.T) {
	dir := t.TempDir()

	iss, err := NewIssuer(IssuerConfig{
		DataPath:  dir,
		IssuerURL: "https://example.miren.cloud",
	})
	require.NoError(t, err)

	tokenStr, err := iss.IssueToken("myapp", "sandbox-789")
	require.NoError(t, err)

	token, err := jwt.ParseWithClaims(tokenStr, &WorkloadClaims{}, func(tok *jwt.Token) (interface{}, error) {
		return iss.publicKey, nil
	})
	require.NoError(t, err)

	claims := token.Claims.(*WorkloadClaims)
	assert.Equal(t, "app:myapp:sandbox:sandbox-789", claims.Subject)
	assert.Empty(t, claims.OrganizationID)
}

func TestIssueToken_NoApp(t *testing.T) {
	dir := t.TempDir()

	iss, err := NewIssuer(IssuerConfig{
		DataPath:       dir,
		IssuerURL:      "https://example.miren.cloud",
		OrganizationID: "org-123",
	})
	require.NoError(t, err)

	tokenStr, err := iss.IssueToken("", "sandbox-789")
	require.NoError(t, err)

	token, err := jwt.ParseWithClaims(tokenStr, &WorkloadClaims{}, func(tok *jwt.Token) (interface{}, error) {
		return iss.publicKey, nil
	})
	require.NoError(t, err)

	claims := token.Claims.(*WorkloadClaims)
	assert.Equal(t, "org:org-123:sandbox:sandbox-789", claims.Subject)
}

func TestDiscoveryDocument(t *testing.T) {
	dir := t.TempDir()

	iss, err := NewIssuer(IssuerConfig{
		DataPath:  dir,
		IssuerURL: "https://example.miren.cloud",
	})
	require.NoError(t, err)

	doc := iss.DiscoveryDocument()

	var parsed map[string]any
	err = json.Unmarshal(doc, &parsed)
	require.NoError(t, err)

	assert.Equal(t, "https://example.miren.cloud", parsed["issuer"])
	assert.Equal(t, "https://example.miren.cloud/.well-known/jwks", parsed["jwks_uri"])
	assert.Contains(t, parsed["id_token_signing_alg_values_supported"], "EdDSA")
}

func TestJWKSDocument(t *testing.T) {
	dir := t.TempDir()

	iss, err := NewIssuer(IssuerConfig{
		DataPath:  dir,
		IssuerURL: "https://example.miren.cloud",
	})
	require.NoError(t, err)

	data, err := iss.JWKSDocument()
	require.NoError(t, err)

	var jwks jose.JSONWebKeySet
	err = json.Unmarshal(data, &jwks)
	require.NoError(t, err)

	require.Len(t, jwks.Keys, 1)
	assert.Equal(t, iss.kid, jwks.Keys[0].KeyID)
	assert.Equal(t, "EdDSA", jwks.Keys[0].Algorithm)
	assert.Equal(t, "sig", jwks.Keys[0].Use)

	// The key should be usable to verify tokens
	pubKey, ok := jwks.Keys[0].Key.(ed25519.PublicKey)
	require.True(t, ok)
	assert.Equal(t, iss.publicKey, pubKey)
}

func TestBuildSubject(t *testing.T) {
	tests := []struct {
		org, app, sandbox string
		expected          string
	}{
		{"org-1", "app-1", "sb-1", "org:org-1:app:app-1:sandbox:sb-1"},
		{"", "app-1", "sb-1", "app:app-1:sandbox:sb-1"},
		{"org-1", "", "sb-1", "org:org-1:sandbox:sb-1"},
		{"", "", "sb-1", "sandbox:sb-1"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, buildSubject(tt.org, tt.app, tt.sandbox))
		})
	}
}
