package appconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupStateTest(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	appStatePathOverride = filepath.Join(dir, appStateFile)
	t.Cleanup(func() { appStatePathOverride = "" })
}

func TestAppStateRoundTrip(t *testing.T) {
	setupStateTest(t)

	require.NoError(t, SaveAppState("my-app", &AppState{Cluster: "prod-us-east"}))

	loaded, err := LoadAppState("my-app")
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Equal(t, "prod-us-east", loaded.Cluster)
}

func TestLoadAppStateMissingApp(t *testing.T) {
	setupStateTest(t)

	state, err := LoadAppState("nonexistent")
	assert.NoError(t, err)
	assert.Nil(t, state)
}

func TestLoadAppStateMissingFile(t *testing.T) {
	setupStateTest(t)

	state, err := LoadAppState("any-app")
	assert.NoError(t, err)
	assert.Nil(t, state)
}

func TestSaveAppStateOverwrites(t *testing.T) {
	setupStateTest(t)

	require.NoError(t, SaveAppState("my-app", &AppState{Cluster: "old-cluster"}))
	require.NoError(t, SaveAppState("my-app", &AppState{Cluster: "new-cluster"}))

	loaded, err := LoadAppState("my-app")
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Equal(t, "new-cluster", loaded.Cluster)
}

func TestSaveAppStateMultipleApps(t *testing.T) {
	setupStateTest(t)

	require.NoError(t, SaveAppState("app-a", &AppState{Cluster: "cluster-1"}))
	require.NoError(t, SaveAppState("app-b", &AppState{Cluster: "cluster-2"}))

	a, err := LoadAppState("app-a")
	require.NoError(t, err)
	require.NotNil(t, a)
	assert.Equal(t, "cluster-1", a.Cluster)

	b, err := LoadAppState("app-b")
	require.NoError(t, err)
	require.NotNil(t, b)
	assert.Equal(t, "cluster-2", b.Cluster)
}

func TestSaveAppStateCreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	appStatePathOverride = filepath.Join(dir, "nested", "dir", appStateFile)
	t.Cleanup(func() { appStatePathOverride = "" })

	require.NoError(t, SaveAppState("my-app", &AppState{Cluster: "test"}))

	_, err := os.Stat(appStatePathOverride)
	require.NoError(t, err)
}
