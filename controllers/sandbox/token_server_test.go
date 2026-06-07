package sandbox

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
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
	secrets.register(testSandboxIP, testSandboxID, testSecret)

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
