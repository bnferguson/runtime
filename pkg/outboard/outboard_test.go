package outboard

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "outboard.json")

	original := &Config{
		Token:      "test-token-abc123",
		FIFOStdout: "/tmp/stdout.fifo",
		FIFOStderr: "/tmp/stderr.fifo",
		PID:        12345,
		RPCAddr:    "localhost:9999",
		Ready:      true,
	}

	err := WriteConfig(path, original)
	require.NoError(t, err)

	loaded, err := ReadConfig(path)
	require.NoError(t, err)

	assert.Equal(t, original.Token, loaded.Token)
	assert.Equal(t, original.FIFOStdout, loaded.FIFOStdout)
	assert.Equal(t, original.FIFOStderr, loaded.FIFOStderr)
	assert.Equal(t, original.PID, loaded.PID)
	assert.Equal(t, original.RPCAddr, loaded.RPCAddr)
	assert.Equal(t, original.Ready, loaded.Ready)
}

func TestConfigReadMissing(t *testing.T) {
	_, err := ReadConfig("/nonexistent/path")
	assert.Error(t, err)
}

func TestConfigWritePermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "outboard.json")

	cfg := &Config{Token: "test"}
	err := WriteConfig(path, cfg)
	require.NoError(t, err)

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

func TestTokenAuthenticatorValid(t *testing.T) {
	auth := NewTokenAuthenticator("secret-token")

	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer secret-token")

	ok, identity, err := auth.AuthenticateRequest(context.Background(), req)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "outboard", identity)
}

func TestTokenAuthenticatorInvalid(t *testing.T) {
	auth := NewTokenAuthenticator("secret-token")

	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")

	ok, _, err := auth.AuthenticateRequest(context.Background(), req)
	assert.Error(t, err)
	assert.False(t, ok)
}

func TestTokenAuthenticatorMissing(t *testing.T) {
	auth := NewTokenAuthenticator("secret-token")

	req, _ := http.NewRequest("GET", "/", nil)

	ok, _, err := auth.AuthenticateRequest(context.Background(), req)
	assert.Error(t, err)
	assert.False(t, ok)
}

func TestTokenAuthenticatorNoAuthorization(t *testing.T) {
	auth := NewTokenAuthenticator("secret-token")

	req, _ := http.NewRequest("GET", "/", nil)

	ok, _, err := auth.NoAuthorization(context.Background(), req)
	assert.Error(t, err)
	assert.False(t, ok)
}

func TestTokenAuthenticatorBadScheme(t *testing.T) {
	auth := NewTokenAuthenticator("secret-token")

	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")

	ok, _, err := auth.AuthenticateRequest(context.Background(), req)
	assert.Error(t, err)
	assert.False(t, ok)
}

func TestCreateFIFO(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.fifo")

	err := createFIFO(path)
	require.NoError(t, err)

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.True(t, info.Mode()&os.ModeNamedPipe != 0)
}

func TestCreateFIFOOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.fifo")

	// Create first FIFO
	err := createFIFO(path)
	require.NoError(t, err)

	// Create again should succeed (removes old one)
	err = createFIFO(path)
	require.NoError(t, err)

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.True(t, info.Mode()&os.ModeNamedPipe != 0)
}

func TestDefaultRestartPolicy(t *testing.T) {
	p := DefaultRestartPolicy()
	assert.Equal(t, 0, p.MaxRestarts)
	assert.NotZero(t, p.BackoffBase)
	assert.NotZero(t, p.BackoffMax)
	assert.NotZero(t, p.ResetWindow)
}

func TestConnectorNotRunning(t *testing.T) {
	log := defaultTestLogger()
	c := NewConnector(log, ConnectorConfig{
		Name:     "test",
		DataPath: t.TempDir(),
	})

	assert.False(t, c.IsRunning())

	_, err := c.PID()
	assert.Error(t, err)
}

func TestConnectorStopNotRunning(t *testing.T) {
	log := defaultTestLogger()
	c := NewConnector(log, ConnectorConfig{
		Name:     "test",
		DataPath: t.TempDir(),
	})

	err := c.Stop(context.Background())
	assert.NoError(t, err)
}

func defaultTestLogger() *slog.Logger {
	return slog.Default()
}
