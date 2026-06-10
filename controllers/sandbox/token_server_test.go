package sandbox

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"miren.dev/runtime/network"
	"miren.dev/runtime/pkg/dns"
	"miren.dev/runtime/pkg/workloadidentity"
)

const testSandboxIP = "10.0.0.5"
const testSandboxID = "sandbox/myapp-web-abc123"
const testSecret = "test-secret-token-value"

func newTestTokenController(t *testing.T) *SandboxController {
	t.Helper()

	dir := t.TempDir()
	issuer, err := workloadidentity.NewIssuer(workloadidentity.IssuerConfig{
		DataPath:       dir,
		IssuerURL:      "https://test.miren.systems",
		OrganizationID: "org-test",
		ClusterID:      "cluster-test",
	})
	require.NoError(t, err)

	log := slog.Default()

	sm := network.NewServiceManager(log, nil)
	sm.AddTestDNSServer(t, func(s *dns.Server) {
		s.AddSandboxMapping(testSandboxID, testSandboxIP, "myapp", "web")
	})

	secrets := newTokenSecretRegistry()
	secrets.register(testSandboxID, testSecret)

	return &SandboxController{
		Log:            log,
		NetServ:        sm,
		WorkloadIssuer: issuer,
		tokenSecrets:   secrets,
	}
}

func authedRequest(method, url string) *http.Request {
	req := httptest.NewRequest(method, url, nil)
	req.RemoteAddr = testSandboxIP + ":12345"
	req.Header.Set("Authorization", "Bearer "+testSecret)
	return req
}

func TestTokenServer_DefaultToken(t *testing.T) {
	c := newTestTokenController(t)
	w := httptest.NewRecorder()

	c.handleTokenRequest(w, authedRequest("GET", "/v1/token"))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var resp tokenResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.NotEmpty(t, resp.Value)

	token, err := jwt.ParseWithClaims(resp.Value, &workloadidentity.WorkloadClaims{}, func(tok *jwt.Token) (interface{}, error) {
		assert.Equal(t, "RS256", tok.Method.Alg())
		return c.WorkloadIssuer.(*workloadidentity.Issuer).PublicKey(), nil
	})
	require.NoError(t, err)

	claims := token.Claims.(*workloadidentity.WorkloadClaims)
	assert.Equal(t, "myapp", claims.App)
	assert.Equal(t, testSandboxID, claims.SandboxID)
	assert.Equal(t, "org-test", claims.OrganizationID)
	assert.Equal(t, jwt.ClaimStrings{"miren"}, claims.Audience)
}

func TestTokenServer_CustomAudience(t *testing.T) {
	c := newTestTokenController(t)
	w := httptest.NewRecorder()

	c.handleTokenRequest(w, authedRequest("GET", "/v1/token?audience=sts.amazonaws.com&audience=myapi.example.com"))

	assert.Equal(t, http.StatusOK, w.Code)

	var resp tokenResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	token, err := jwt.ParseWithClaims(resp.Value, &workloadidentity.WorkloadClaims{}, func(tok *jwt.Token) (interface{}, error) {
		assert.Equal(t, "RS256", tok.Method.Alg())
		return c.WorkloadIssuer.(*workloadidentity.Issuer).PublicKey(), nil
	}, jwt.WithAudience("sts.amazonaws.com"))
	require.NoError(t, err)

	claims := token.Claims.(*workloadidentity.WorkloadClaims)
	assert.Equal(t, jwt.ClaimStrings{"sts.amazonaws.com", "myapi.example.com"}, claims.Audience)
}

func TestTokenServer_CustomTTL(t *testing.T) {
	c := newTestTokenController(t)
	w := httptest.NewRecorder()

	c.handleTokenRequest(w, authedRequest("GET", "/v1/token?ttl=300"))

	assert.Equal(t, http.StatusOK, w.Code)

	var resp tokenResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	token, err := jwt.ParseWithClaims(resp.Value, &workloadidentity.WorkloadClaims{}, func(tok *jwt.Token) (interface{}, error) {
		assert.Equal(t, "RS256", tok.Method.Alg())
		return c.WorkloadIssuer.(*workloadidentity.Issuer).PublicKey(), nil
	})
	require.NoError(t, err)

	claims := token.Claims.(*workloadidentity.WorkloadClaims)
	ttl := claims.ExpiresAt.Sub(claims.IssuedAt.Time)
	assert.Equal(t, 300.0, ttl.Seconds())
}

func TestTokenServer_MissingAuth(t *testing.T) {
	c := newTestTokenController(t)

	req := httptest.NewRequest("GET", "/v1/token", nil)
	req.RemoteAddr = testSandboxIP + ":12345"
	w := httptest.NewRecorder()

	c.handleTokenRequest(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestTokenServer_WrongSecret(t *testing.T) {
	c := newTestTokenController(t)

	req := httptest.NewRequest("GET", "/v1/token", nil)
	req.RemoteAddr = testSandboxIP + ":12345"
	req.Header.Set("Authorization", "Bearer wrong-secret")
	w := httptest.NewRecorder()

	c.handleTokenRequest(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestTokenServer_UnknownIP(t *testing.T) {
	c := newTestTokenController(t)

	req := httptest.NewRequest("GET", "/v1/token", nil)
	req.RemoteAddr = "10.0.0.99:12345"
	req.Header.Set("Authorization", "Bearer "+testSecret)
	w := httptest.NewRecorder()

	c.handleTokenRequest(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestTokenServer_RejectsPost(t *testing.T) {
	c := newTestTokenController(t)
	w := httptest.NewRecorder()

	c.handleTokenRequest(w, authedRequest("POST", "/v1/token"))

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestTokenServer_InvalidTTL(t *testing.T) {
	c := newTestTokenController(t)
	w := httptest.NewRecorder()

	c.handleTokenRequest(w, authedRequest("GET", "/v1/token?ttl=notanumber"))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestTokenSecretRegistry_KeyedBySandboxIdentity pins the property behind keying the
// registry by sandbox identity rather than raw IP: a secret is bound to one sandbox and
// cannot authenticate a different sandbox (e.g. one that later reused a recycled pod IP).
func TestTokenSecretRegistry_KeyedBySandboxIdentity(t *testing.T) {
	r := newTokenSecretRegistry()
	r.register("sandbox/old", "secret-old")

	assert.True(t, r.verify("sandbox/old", "secret-old"))
	assert.False(t, r.verify("sandbox/new", "secret-old"))

	r.unregister("sandbox/old")
	assert.False(t, r.verify("sandbox/old", "secret-old"))
}

func TestWriteLoadTokenSecret_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), tokenSecretFilename)

	secret, err := generateTokenSecret()
	require.NoError(t, err)

	require.NoError(t, writeTokenSecret(path, secret))

	got, ok, err := loadTokenSecret(path)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, secret, got)

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

func TestLoadTokenSecret_Missing(t *testing.T) {
	got, ok, err := loadTokenSecret(filepath.Join(t.TempDir(), tokenSecretFilename))

	require.NoError(t, err)
	assert.False(t, ok)
	assert.Empty(t, got)
}

// TestTokenServer_RecoversSecretAfterRestart reproduces MIR-1235: a still-running sandbox
// 403s after the controller/token-server restarts and the in-memory registry is lost, then
// recovers once the persisted secret is reloaded and re-registered for the sandbox —
// without restarting the sandbox.
func TestTokenServer_RecoversSecretAfterRestart(t *testing.T) {
	c := newTestTokenController(t)

	// Simulate a controller/token-server restart: the registry is recreated empty.
	c.tokenSecrets = newTokenSecretRegistry()

	w := httptest.NewRecorder()
	c.handleTokenRequest(w, authedRequest("GET", "/v1/token"))
	require.Equal(t, http.StatusForbidden, w.Code)

	// On start the secret was persisted host-side; boot reconcile reloads it and
	// re-registers it under the sandbox identity.
	path := filepath.Join(t.TempDir(), tokenSecretFilename)
	require.NoError(t, writeTokenSecret(path, testSecret))

	secret, ok, err := loadTokenSecret(path)
	require.NoError(t, err)
	require.True(t, ok)
	c.tokenSecrets.register(testSandboxID, secret)

	w = httptest.NewRecorder()
	c.handleTokenRequest(w, authedRequest("GET", "/v1/token"))
	assert.Equal(t, http.StatusOK, w.Code)
}
