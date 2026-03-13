package build

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"miren.dev/runtime/api/build/build_v1alpha"
)

func TestSourceCacheRoundtrip(t *testing.T) {
	dataPath := t.TempDir()
	cache := &sourceCache{dataPath: dataPath, log: slog.Default(), locks: newAppLocks()}

	// Create a build directory with some files
	buildDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(buildDir, "main.go"), []byte("package main"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(buildDir, "lib"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(buildDir, "lib/util.go"), []byte("package lib"), 0644))

	// Save source image
	err := cache.saveSourceImage("myapp", buildDir)
	require.NoError(t, err)

	// Verify files were created
	require.FileExists(t, filepath.Join(dataPath, "source-code", "myapp", "layer.tar.gz"))
	require.FileExists(t, filepath.Join(dataPath, "source-code", "myapp", "manifest.json"))

	// Load manifest and verify contents
	manifest, err := cache.loadManifest("myapp")
	require.NoError(t, err)
	require.Contains(t, manifest, "main.go")
	require.Contains(t, manifest, "lib/util.go")
	require.NotEmpty(t, manifest["main.go"].Hash)
	require.Equal(t, int64(12), manifest["main.go"].Size) // "package main" = 12 bytes
}

func TestSourceCacheStageMatchingFiles(t *testing.T) {
	dataPath := t.TempDir()
	cache := &sourceCache{dataPath: dataPath, log: slog.Default(), locks: newAppLocks()}

	// Create and save an initial source image
	buildDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(buildDir, "main.go"), []byte("package main"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(buildDir, "config.go"), []byte("package config"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(buildDir, "lib"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(buildDir, "lib/util.go"), []byte("package lib"), 0644))
	require.NoError(t, cache.saveSourceImage("myapp", buildDir))

	// Load the manifest to get hashes
	cachedManifest, err := cache.loadManifest("myapp")
	require.NoError(t, err)

	// Create client manifest: main.go unchanged, config.go changed, lib/util.go unchanged, new.go added
	clientManifest := []*build_v1alpha.FileManifestEntry{
		makeManifestEntry("main.go", cachedManifest["main.go"].Hash, 12, 0644),
		makeManifestEntry("config.go", "different-hash", 20, 0644),
		makeManifestEntry("lib/util.go", cachedManifest["lib/util.go"].Hash, 11, 0644),
		makeManifestEntry("new.go", "brand-new-hash", 50, 0644),
	}

	// Stage matching files into a new directory
	stageDir := t.TempDir()
	matched, needed, err := cache.stageMatchingFiles("myapp", stageDir, clientManifest)
	require.NoError(t, err)

	// main.go and lib/util.go should be cached
	require.Equal(t, 2, matched)

	// config.go (changed) and new.go (new) should be needed
	require.ElementsMatch(t, []string{"config.go", "new.go"}, needed)

	// Verify cached files were extracted
	mainContent, err := os.ReadFile(filepath.Join(stageDir, "main.go"))
	require.NoError(t, err)
	require.Equal(t, "package main", string(mainContent))

	utilContent, err := os.ReadFile(filepath.Join(stageDir, "lib/util.go"))
	require.NoError(t, err)
	require.Equal(t, "package lib", string(utilContent))
}

func TestSourceCacheNoExistingCache(t *testing.T) {
	dataPath := t.TempDir()
	cache := &sourceCache{dataPath: dataPath, log: slog.Default(), locks: newAppLocks()}

	clientManifest := []*build_v1alpha.FileManifestEntry{
		makeManifestEntry("main.go", "some-hash", 12, 0644),
		makeManifestEntry("lib/util.go", "another-hash", 11, 0644),
	}

	stageDir := t.TempDir()
	matched, needed, err := cache.stageMatchingFiles("newapp", stageDir, clientManifest)
	require.NoError(t, err)
	require.Equal(t, 0, matched)
	require.ElementsMatch(t, []string{"main.go", "lib/util.go"}, needed)
}

func TestSourceCacheOverwrite(t *testing.T) {
	dataPath := t.TempDir()
	cache := &sourceCache{dataPath: dataPath, log: slog.Default(), locks: newAppLocks()}

	// Save first version
	buildDir1 := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(buildDir1, "main.go"), []byte("v1"), 0644))
	require.NoError(t, cache.saveSourceImage("myapp", buildDir1))

	manifest1, err := cache.loadManifest("myapp")
	require.NoError(t, err)

	// Save second version
	buildDir2 := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(buildDir2, "main.go"), []byte("v2"), 0644))
	require.NoError(t, cache.saveSourceImage("myapp", buildDir2))

	manifest2, err := cache.loadManifest("myapp")
	require.NoError(t, err)

	// Hashes should differ
	require.NotEqual(t, manifest1["main.go"].Hash, manifest2["main.go"].Hash)
}

func makeManifestEntry(path, hash string, size int64, mode int32) *build_v1alpha.FileManifestEntry {
	entry := &build_v1alpha.FileManifestEntry{}
	entry.SetPath(path)
	entry.SetHash(hash)
	entry.SetSize(size)
	entry.SetMode(mode)
	return entry
}
