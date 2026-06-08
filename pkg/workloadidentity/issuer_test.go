package workloadidentity

import (
	"crypto/ed25519"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

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
	assert.Equal(t, "https://example.miren.cloud/.well-known/miren/jwks", parsed["jwks_uri"])
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

func TestJWKSDocument_WithPreviousKey(t *testing.T) {
	dir := t.TempDir()

	// Create initial issuer (generates key)
	iss1, err := NewIssuer(IssuerConfig{
		DataPath:  dir,
		IssuerURL: "https://example.miren.cloud",
	})
	require.NoError(t, err)
	oldKID := iss1.kid

	// Simulate key rotation: move current key to .prev
	keyPath := filepath.Join(dir, "server", "workload-identity.key")
	err = os.Rename(keyPath, keyPath+".prev")
	require.NoError(t, err)

	// Create new issuer (generates new key, loads old as previous)
	iss2, err := NewIssuer(IssuerConfig{
		DataPath:  dir,
		IssuerURL: "https://example.miren.cloud",
	})
	require.NoError(t, err)
	assert.NotEqual(t, oldKID, iss2.kid)
	require.NotNil(t, iss2.previousKey)

	// JWKS should contain both keys
	data, err := iss2.JWKSDocument()
	require.NoError(t, err)

	var jwks jose.JSONWebKeySet
	err = json.Unmarshal(data, &jwks)
	require.NoError(t, err)

	require.Len(t, jwks.Keys, 2)
	assert.Equal(t, iss2.kid, jwks.Keys[0].KeyID)
	assert.Equal(t, oldKID, jwks.Keys[1].KeyID)

	// Token signed by the new key should be verifiable via JWKS
	tokenStr, err := iss2.IssueToken("myapp", "sb-1")
	require.NoError(t, err)

	_, err = jwt.Parse(tokenStr, func(tok *jwt.Token) (interface{}, error) {
		kid := tok.Header["kid"].(string)
		keys := jwks.Key(kid)
		require.NotEmpty(t, keys)
		return keys[0].Key, nil
	})
	require.NoError(t, err)

	// Token signed by the OLD key (simulated via iss1) should also verify via JWKS
	oldTokenStr, err := iss1.IssueToken("myapp", "sb-2")
	require.NoError(t, err)

	_, err = jwt.Parse(oldTokenStr, func(tok *jwt.Token) (interface{}, error) {
		kid := tok.Header["kid"].(string)
		keys := jwks.Key(kid)
		require.NotEmpty(t, keys)
		return keys[0].Key, nil
	})
	require.NoError(t, err)
}

func TestIssueTokenWithOptions_CustomAudience(t *testing.T) {
	dir := t.TempDir()

	iss, err := NewIssuer(IssuerConfig{
		DataPath:  dir,
		IssuerURL: "https://example.miren.cloud",
	})
	require.NoError(t, err)

	tokenStr, err := iss.IssueTokenWithOptions("myapp", "sb-1", TokenOptions{
		Audience: []string{"sts.amazonaws.com"},
	})
	require.NoError(t, err)

	token, err := jwt.ParseWithClaims(tokenStr, &WorkloadClaims{}, func(tok *jwt.Token) (interface{}, error) {
		return iss.publicKey, nil
	})
	require.NoError(t, err)

	claims := token.Claims.(*WorkloadClaims)
	assert.Equal(t, jwt.ClaimStrings{"sts.amazonaws.com"}, claims.Audience)
}

func TestIssueTokenWithOptions_CustomTTL(t *testing.T) {
	dir := t.TempDir()

	iss, err := NewIssuer(IssuerConfig{
		DataPath:  dir,
		IssuerURL: "https://example.miren.cloud",
	})
	require.NoError(t, err)

	tokenStr, err := iss.IssueTokenWithOptions("myapp", "sb-1", TokenOptions{
		TTL: 5 * time.Minute,
	})
	require.NoError(t, err)

	token, err := jwt.ParseWithClaims(tokenStr, &WorkloadClaims{}, func(tok *jwt.Token) (interface{}, error) {
		return iss.publicKey, nil
	})
	require.NoError(t, err)

	claims := token.Claims.(*WorkloadClaims)
	ttl := claims.ExpiresAt.Sub(claims.IssuedAt.Time)

	assert.Equal(t, 5*time.Minute, ttl)
}

func TestIssueTokenWithOptions_TTLClamping(t *testing.T) {
	dir := t.TempDir()

	iss, err := NewIssuer(IssuerConfig{
		DataPath:  dir,
		IssuerURL: "https://example.miren.cloud",
	})
	require.NoError(t, err)

	// Max TTL clamped to 24h
	tokenStr, err := iss.IssueTokenWithOptions("myapp", "sb-1", TokenOptions{
		TTL: 48 * time.Hour,
	})
	require.NoError(t, err)

	token, err := jwt.ParseWithClaims(tokenStr, &WorkloadClaims{}, func(tok *jwt.Token) (interface{}, error) {
		return iss.publicKey, nil
	})
	require.NoError(t, err)

	claims := token.Claims.(*WorkloadClaims)
	ttl := claims.ExpiresAt.Sub(claims.IssuedAt.Time)

	assert.Equal(t, 24*time.Hour, ttl)

	// Min TTL clamped to 60s
	tokenStr2, err := iss.IssueTokenWithOptions("myapp", "sb-1", TokenOptions{
		TTL: 5 * time.Second,
	})
	require.NoError(t, err)

	token2, err := jwt.ParseWithClaims(tokenStr2, &WorkloadClaims{}, func(tok *jwt.Token) (interface{}, error) {
		return iss.publicKey, nil
	})
	require.NoError(t, err)

	claims2 := token2.Claims.(*WorkloadClaims)
	ttl2 := claims2.ExpiresAt.Sub(claims2.IssuedAt.Time)

	assert.Equal(t, 60*time.Second, ttl2)
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
