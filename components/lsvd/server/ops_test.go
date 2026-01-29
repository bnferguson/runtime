package server

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"miren.dev/runtime/lsvd"
	"miren.dev/runtime/pkg/units"
)

func TestRealVolumeOps_InitLSVDVolume_LocalOnly(t *testing.T) {
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	t.Run("basic volume creation", func(t *testing.T) {
		tmpDir := t.TempDir()
		ops := NewRealVolumeOps(log, nil, "") // No cloud auth

		volumeId := "test-volume-id-123"
		size := units.GigaBytes(1).Bytes()

		returnedId, err := ops.InitLSVDVolume(ctx, tmpDir, volumeId, size, nil, false)
		require.NoError(t, err)

		// Returned ID should match input ID
		assert.Equal(t, volumeId, returnedId)

		// Verify volume can be looked up with the returned ID
		localSA := &lsvd.LocalFileAccess{Dir: tmpDir, Log: log}
		volInfo, err := localSA.GetVolumeInfo(ctx, returnedId)
		require.NoError(t, err, "should be able to look up volume with returned ID")
		assert.Equal(t, size.Int64(), volInfo.Size.Bytes().Int64())
	})

	t.Run("volume with human-readable name in metadata", func(t *testing.T) {
		tmpDir := t.TempDir()
		ops := NewRealVolumeOps(log, nil, "") // No cloud auth

		volumeId := "uuid-style-id-456"
		humanName := "my-awesome-disk"
		size := units.GigaBytes(2).Bytes()
		metadata := map[string]any{
			"name":       humanName,
			"filesystem": "ext4",
		}

		returnedId, err := ops.InitLSVDVolume(ctx, tmpDir, volumeId, size, metadata, false)
		require.NoError(t, err)

		// Returned ID should be the volumeId, not the human name
		assert.Equal(t, volumeId, returnedId)
		assert.NotEqual(t, humanName, returnedId)

		// Verify volume can be looked up with the returned ID (not the human name)
		localSA := &lsvd.LocalFileAccess{Dir: tmpDir, Log: log}
		volInfo, err := localSA.GetVolumeInfo(ctx, returnedId)
		require.NoError(t, err, "should be able to look up volume with returned ID")
		assert.Equal(t, size.Int64(), volInfo.Size.Bytes().Int64())

		// Verify looking up by human name fails (directory should not be named with human name)
		_, err = localSA.GetVolumeInfo(ctx, humanName)
		assert.Error(t, err, "should NOT be able to look up volume with human name")
	})

	t.Run("directory structure is correct", func(t *testing.T) {
		tmpDir := t.TempDir()
		ops := NewRealVolumeOps(log, nil, "")

		volumeId := "dir-test-vol-789"
		metadata := map[string]any{
			"name": "human-readable-name",
		}

		returnedId, err := ops.InitLSVDVolume(ctx, tmpDir, volumeId, units.GigaBytes(1).Bytes(), metadata, false)
		require.NoError(t, err)

		// Verify directory structure
		// Should have: tmpDir/segments/ and tmpDir/volumes/<volumeId>/
		segmentsDir := filepath.Join(tmpDir, "segments")
		assert.DirExists(t, segmentsDir)

		volumeDir := filepath.Join(tmpDir, "volumes", returnedId)
		assert.DirExists(t, volumeDir)

		infoFile := filepath.Join(volumeDir, "info.json")
		assert.FileExists(t, infoFile)

		// Directory should NOT exist with human name
		humanNameDir := filepath.Join(tmpDir, "volumes", "human-readable-name")
		_, err = os.Stat(humanNameDir)
		assert.True(t, os.IsNotExist(err), "directory should not exist with human-readable name")
	})

	t.Run("metadata is preserved in volume info", func(t *testing.T) {
		tmpDir := t.TempDir()
		ops := NewRealVolumeOps(log, nil, "")

		volumeId := "metadata-test-vol"
		metadata := map[string]any{
			"name":       "my-disk",
			"filesystem": "xfs",
			"custom":     "value",
		}

		returnedId, err := ops.InitLSVDVolume(ctx, tmpDir, volumeId, units.GigaBytes(1).Bytes(), metadata, false)
		require.NoError(t, err)

		localSA := &lsvd.LocalFileAccess{Dir: tmpDir, Log: log}
		volInfo, err := localSA.GetVolumeInfo(ctx, returnedId)
		require.NoError(t, err)

		// Metadata should be preserved
		assert.Equal(t, "my-disk", volInfo.Metadata["name"])
		assert.Equal(t, "xfs", volInfo.Metadata["filesystem"])
		assert.Equal(t, "value", volInfo.Metadata["custom"])
	})
}

func TestRealVolumeOps_VolumePathOperations(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ops := NewRealVolumeOps(log, nil, "")

	t.Run("CreateVolumeDir creates directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		testPath := filepath.Join(tmpDir, "test-volume-dir")

		err := ops.CreateVolumeDir(testPath)
		require.NoError(t, err)
		assert.DirExists(t, testPath)
	})

	t.Run("CreateVolumeDir creates nested directories", func(t *testing.T) {
		tmpDir := t.TempDir()
		testPath := filepath.Join(tmpDir, "a", "b", "c", "volume")

		err := ops.CreateVolumeDir(testPath)
		require.NoError(t, err)
		assert.DirExists(t, testPath)
	})

	t.Run("VolumePathExists returns true for existing path", func(t *testing.T) {
		tmpDir := t.TempDir()
		assert.True(t, ops.VolumePathExists(tmpDir))
	})

	t.Run("VolumePathExists returns false for non-existing path", func(t *testing.T) {
		assert.False(t, ops.VolumePathExists("/non/existent/path/12345"))
	})

	t.Run("RemoveVolumeDir removes directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		testPath := filepath.Join(tmpDir, "to-remove")

		err := os.MkdirAll(testPath, 0755)
		require.NoError(t, err)

		// Create a file inside
		err = os.WriteFile(filepath.Join(testPath, "file.txt"), []byte("test"), 0644)
		require.NoError(t, err)

		err = ops.RemoveVolumeDir(testPath)
		require.NoError(t, err)
		assert.NoDirExists(t, testPath)
	})
}

func TestRealVolumeOps_InitLSVDVolume_RemoteOnly(t *testing.T) {
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	t.Run("remote-only without auth fails", func(t *testing.T) {
		tmpDir := t.TempDir()
		ops := NewRealVolumeOps(log, nil, "") // No cloud auth

		_, err := ops.InitLSVDVolume(ctx, tmpDir, "vol-id", units.GigaBytes(1).Bytes(), nil, true)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "remote-only volume requires cloud auth")
	})
}

// TestVolumeDirectoryNamingConsistency is a regression test for the bug where
// volumes with human-readable names would have the directory created with the
// name but the returned ID would be the UUID, causing lookups to fail.
func TestVolumeDirectoryNamingConsistency(t *testing.T) {
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	testCases := []struct {
		name       string
		volumeId   string
		metadata   map[string]any
		shouldFind bool
	}{
		{
			name:       "no metadata",
			volumeId:   "vol-no-meta",
			metadata:   nil,
			shouldFind: true,
		},
		{
			name:       "empty metadata",
			volumeId:   "vol-empty-meta",
			metadata:   map[string]any{},
			shouldFind: true,
		},
		{
			name:     "with name in metadata",
			volumeId: "vol-with-name",
			metadata: map[string]any{
				"name": "human-readable-name",
			},
			shouldFind: true,
		},
		{
			name:     "with various metadata including name",
			volumeId: "vol-full-meta",
			metadata: map[string]any{
				"name":       "my-production-disk",
				"filesystem": "ext4",
				"project":    "test-project",
			},
			shouldFind: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			ops := NewRealVolumeOps(log, nil, "")

			returnedId, err := ops.InitLSVDVolume(ctx, tmpDir, tc.volumeId, units.GigaBytes(1).Bytes(), tc.metadata, false)
			require.NoError(t, err)

			// The returned ID must always equal the input volumeId
			assert.Equal(t, tc.volumeId, returnedId, "returned ID should match input volumeId")

			// The volume must be findable using the returned ID
			localSA := &lsvd.LocalFileAccess{Dir: tmpDir, Log: log}
			volInfo, err := localSA.GetVolumeInfo(ctx, returnedId)

			if tc.shouldFind {
				require.NoError(t, err, "volume should be findable with returned ID")
				assert.NotNil(t, volInfo)
			} else {
				assert.Error(t, err)
			}
		})
	}
}
