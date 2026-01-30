package server

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"miren.dev/runtime/lsvd"
	"miren.dev/runtime/pkg/units"
)

// =============================================================================
// VolumeOps Tests
// =============================================================================

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

	t.Run("different volume sizes", func(t *testing.T) {
		tmpDir := t.TempDir()
		ops := NewRealVolumeOps(log, nil, "")

		testCases := []struct {
			name string
			size units.Bytes
		}{
			{"1GB", units.GigaBytes(1).Bytes()},
			{"10GB", units.GigaBytes(10).Bytes()},
			{"100GB", units.GigaBytes(100).Bytes()},
			{"512MB", units.MegaBytes(512).Bytes()},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				volDir := filepath.Join(tmpDir, tc.name)
				volumeId := "vol-" + tc.name

				// Create the parent directory - InitLSVDVolume creates subdirectories
				// but the parent path must exist
				err := os.MkdirAll(volDir, 0755)
				require.NoError(t, err)

				returnedId, err := ops.InitLSVDVolume(ctx, volDir, volumeId, tc.size, nil, false)
				require.NoError(t, err)

				localSA := &lsvd.LocalFileAccess{Dir: volDir, Log: log}
				volInfo, err := localSA.GetVolumeInfo(ctx, returnedId)
				require.NoError(t, err)
				assert.Equal(t, tc.size.Int64(), volInfo.Size.Bytes().Int64())
			})
		}
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

	t.Run("CreateVolumeDir is idempotent", func(t *testing.T) {
		tmpDir := t.TempDir()
		testPath := filepath.Join(tmpDir, "idempotent-dir")

		err := ops.CreateVolumeDir(testPath)
		require.NoError(t, err)

		// Call again - should not error
		err = ops.CreateVolumeDir(testPath)
		require.NoError(t, err)
		assert.DirExists(t, testPath)
	})

	t.Run("VolumePathExists returns true for existing path", func(t *testing.T) {
		tmpDir := t.TempDir()
		assert.True(t, ops.VolumePathExists(tmpDir))
	})

	t.Run("VolumePathExists returns true for existing file", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "testfile")
		err := os.WriteFile(filePath, []byte("test"), 0644)
		require.NoError(t, err)

		assert.True(t, ops.VolumePathExists(filePath))
	})

	t.Run("VolumePathExists returns false for non-existing path", func(t *testing.T) {
		assert.False(t, ops.VolumePathExists("/non/existent/path/12345"))
	})

	t.Run("RemoveVolumeDir removes empty directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		testPath := filepath.Join(tmpDir, "to-remove")

		err := os.MkdirAll(testPath, 0755)
		require.NoError(t, err)

		err = ops.RemoveVolumeDir(testPath)
		require.NoError(t, err)
		assert.NoDirExists(t, testPath)
	})

	t.Run("RemoveVolumeDir removes directory with contents", func(t *testing.T) {
		tmpDir := t.TempDir()
		testPath := filepath.Join(tmpDir, "to-remove")

		err := os.MkdirAll(testPath, 0755)
		require.NoError(t, err)

		// Create files and subdirectories
		err = os.WriteFile(filepath.Join(testPath, "file.txt"), []byte("test"), 0644)
		require.NoError(t, err)
		err = os.MkdirAll(filepath.Join(testPath, "subdir", "nested"), 0755)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(testPath, "subdir", "nested", "deep.txt"), []byte("deep"), 0644)
		require.NoError(t, err)

		err = ops.RemoveVolumeDir(testPath)
		require.NoError(t, err)
		assert.NoDirExists(t, testPath)
	})

	t.Run("RemoveVolumeDir on non-existent path returns no error", func(t *testing.T) {
		err := ops.RemoveVolumeDir("/non/existent/path/12345")
		assert.NoError(t, err)
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
		{
			name:     "name matches volumeId",
			volumeId: "same-name-vol",
			metadata: map[string]any{
				"name": "same-name-vol",
			},
			shouldFind: true,
		},
		{
			name:     "empty name in metadata",
			volumeId: "vol-empty-name",
			metadata: map[string]any{
				"name": "",
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

// =============================================================================
// MountOps Tests
// =============================================================================

func TestRealMountOps_CreateDir(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("MountOps tests only run on Linux")
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ops := NewRealMountOps(log, nil, "")

	t.Run("creates directory with permissions", func(t *testing.T) {
		tmpDir := t.TempDir()
		testPath := filepath.Join(tmpDir, "new-dir")

		err := ops.CreateDir(testPath, 0755)
		require.NoError(t, err)
		assert.DirExists(t, testPath)

		info, err := os.Stat(testPath)
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	})

	t.Run("creates nested directories", func(t *testing.T) {
		tmpDir := t.TempDir()
		testPath := filepath.Join(tmpDir, "a", "b", "c")

		err := ops.CreateDir(testPath, 0755)
		require.NoError(t, err)
		assert.DirExists(t, testPath)
	})

	t.Run("is idempotent", func(t *testing.T) {
		tmpDir := t.TempDir()
		testPath := filepath.Join(tmpDir, "idempotent")

		err := ops.CreateDir(testPath, 0755)
		require.NoError(t, err)

		err = ops.CreateDir(testPath, 0755)
		require.NoError(t, err)
	})
}

func TestRealMountOps_RemoveFile(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("MountOps tests only run on Linux")
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ops := NewRealMountOps(log, nil, "")

	t.Run("removes existing file", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "to-delete.txt")

		err := os.WriteFile(filePath, []byte("content"), 0644)
		require.NoError(t, err)
		assert.FileExists(t, filePath)

		err = ops.RemoveFile(filePath)
		require.NoError(t, err)
		assert.NoFileExists(t, filePath)
	})

	t.Run("returns error for non-existent file", func(t *testing.T) {
		err := ops.RemoveFile("/non/existent/file/12345.txt")
		assert.Error(t, err)
	})

	t.Run("returns error for non-empty directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		dirPath := filepath.Join(tmpDir, "a-directory")

		err := os.MkdirAll(dirPath, 0755)
		require.NoError(t, err)

		// Create a file inside the directory - os.Remove on empty dir succeeds on Linux
		err = os.WriteFile(filepath.Join(dirPath, "file.txt"), []byte("content"), 0644)
		require.NoError(t, err)

		err = ops.RemoveFile(dirPath)
		assert.Error(t, err)
	})
}

func TestRealMountOps_IsMounted(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("MountOps tests only run on Linux")
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ops := NewRealMountOps(log, nil, "")

	t.Run("returns true for root filesystem", func(t *testing.T) {
		// Root is always mounted
		assert.True(t, ops.IsMounted("/"))
	})

	t.Run("returns false for non-mount path", func(t *testing.T) {
		tmpDir := t.TempDir()
		assert.False(t, ops.IsMounted(tmpDir))
	})

	t.Run("returns false for non-existent path", func(t *testing.T) {
		assert.False(t, ops.IsMounted("/non/existent/path/12345"))
	})
}

func TestRealMountOps_FormatDevice_UnsupportedFilesystem(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("MountOps tests only run on Linux")
	}

	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ops := NewRealMountOps(log, nil, "")

	t.Run("returns error for unsupported filesystem", func(t *testing.T) {
		err := ops.FormatDevice(ctx, "/dev/null", "unsupported-fs")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported filesystem")
	})

	t.Run("supported filesystems are recognized", func(t *testing.T) {
		// These should fail because /dev/null is not a real block device,
		// but NOT because the filesystem is unsupported
		for _, fs := range []string{"ext4", "xfs", "btrfs"} {
			err := ops.FormatDevice(ctx, "/dev/null", fs)
			if err != nil {
				assert.NotContains(t, err.Error(), "unsupported filesystem",
					"filesystem %s should be recognized", fs)
			}
		}
	})
}

func TestRealMountOps_LeaseOperations_NoAuth(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("MountOps tests only run on Linux")
	}

	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ops := NewRealMountOps(log, nil, "") // No cloud auth

	t.Run("AcquireVolumeLease returns empty without auth", func(t *testing.T) {
		nonce, err := ops.AcquireVolumeLease(ctx, "any-volume", map[string]any{"key": "value"})
		assert.NoError(t, err)
		assert.Empty(t, nonce)
	})

	t.Run("ReleaseVolumeLease does nothing without auth", func(t *testing.T) {
		err := ops.ReleaseVolumeLease(ctx, "any-volume", "any-nonce")
		assert.NoError(t, err)
	})

	t.Run("ReleaseVolumeLease does nothing with empty nonce", func(t *testing.T) {
		err := ops.ReleaseVolumeLease(ctx, "any-volume", "")
		assert.NoError(t, err)
	})
}

func TestRealMountOps_OpenLSVDDisk_LocalOnly(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("MountOps tests only run on Linux")
	}

	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	t.Run("opens local volume successfully", func(t *testing.T) {
		tmpDir := t.TempDir()

		// First create a volume
		volOps := NewRealVolumeOps(log, nil, "")
		volumeId := "local-disk-test"
		size := units.MegaBytes(64).Bytes()

		returnedId, err := volOps.InitLSVDVolume(ctx, tmpDir, volumeId, size, nil, false)
		require.NoError(t, err)

		// Now open it with MountOps
		mountOps := NewRealMountOps(log, nil, "")
		disk, err := mountOps.OpenLSVDDisk(ctx, tmpDir, returnedId, false, "")
		require.NoError(t, err)
		require.NotNil(t, disk)

		// Verify size
		assert.Equal(t, size.Int64(), disk.Size())

		// Clean up
		err = disk.Close(ctx)
		assert.NoError(t, err)
	})

	t.Run("fails for non-existent volume", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Initialize container structure but no volume
		localSA := &lsvd.LocalFileAccess{Dir: tmpDir, Log: log}
		err := localSA.InitContainer(ctx)
		require.NoError(t, err)

		mountOps := NewRealMountOps(log, nil, "")
		_, err = mountOps.OpenLSVDDisk(ctx, tmpDir, "non-existent-volume", false, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get volume info")
	})

	t.Run("remote-only fails without auth", func(t *testing.T) {
		tmpDir := t.TempDir()

		mountOps := NewRealMountOps(log, nil, "")
		_, err := mountOps.OpenLSVDDisk(ctx, tmpDir, "any-volume", true, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "remote-only disk requires cloud auth")
	})
}

func TestRealMountOps_OpenLSVDDisk_WithLeaseNonce(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("MountOps tests only run on Linux")
	}

	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	tmpDir := t.TempDir()

	// Create a volume first
	volOps := NewRealVolumeOps(log, nil, "")
	volumeId := "lease-nonce-test"
	size := units.MegaBytes(64).Bytes()

	returnedId, err := volOps.InitLSVDVolume(ctx, tmpDir, volumeId, size, nil, false)
	require.NoError(t, err)

	// Open with a lease nonce (should work even without cloud auth for local volumes)
	mountOps := NewRealMountOps(log, nil, "")
	disk, err := mountOps.OpenLSVDDisk(ctx, tmpDir, returnedId, false, "test-lease-nonce")
	require.NoError(t, err)
	require.NotNil(t, disk)

	defer disk.Close(ctx)

	assert.Equal(t, size.Int64(), disk.Size())
}

// =============================================================================
// LSVDDisk Tests
// =============================================================================

func TestRealLSVDDisk_Size(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("LSVD tests only run on Linux")
	}

	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	tmpDir := t.TempDir()

	// Create volumes of different sizes and verify Size() returns correctly
	testCases := []struct {
		name string
		size units.Bytes
	}{
		{"64MB", units.MegaBytes(64).Bytes()},
		{"128MB", units.MegaBytes(128).Bytes()},
		{"1GB", units.GigaBytes(1).Bytes()},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			volDir := filepath.Join(tmpDir, tc.name)
			volOps := NewRealVolumeOps(log, nil, "")
			volumeId := "size-test-" + tc.name

			// Create the parent directory
			err := os.MkdirAll(volDir, 0755)
			require.NoError(t, err)

			returnedId, err := volOps.InitLSVDVolume(ctx, volDir, volumeId, tc.size, nil, false)
			require.NoError(t, err)

			mountOps := NewRealMountOps(log, nil, "")
			disk, err := mountOps.OpenLSVDDisk(ctx, volDir, returnedId, false, "")
			require.NoError(t, err)

			assert.Equal(t, tc.size.Int64(), disk.Size())

			err = disk.Close(ctx)
			assert.NoError(t, err)
		})
	}
}

func TestRealLSVDDisk_Close(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("LSVD tests only run on Linux")
	}

	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	tmpDir := t.TempDir()

	// Create and open a volume
	volOps := NewRealVolumeOps(log, nil, "")
	volumeId := "close-test"
	size := units.MegaBytes(64).Bytes()

	returnedId, err := volOps.InitLSVDVolume(ctx, tmpDir, volumeId, size, nil, false)
	require.NoError(t, err)

	mountOps := NewRealMountOps(log, nil, "")
	disk, err := mountOps.OpenLSVDDisk(ctx, tmpDir, returnedId, false, "")
	require.NoError(t, err)

	// Close should succeed
	err = disk.Close(ctx)
	assert.NoError(t, err)

	// Double close should also not panic (but may error)
	// This tests graceful handling
	_ = disk.Close(ctx)
}

// =============================================================================
// Integration Tests
// =============================================================================

func TestVolumeOps_CreateThenOpen(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Integration tests only run on Linux")
	}

	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	tmpDir := t.TempDir()

	volOps := NewRealVolumeOps(log, nil, "")
	mountOps := NewRealMountOps(log, nil, "")

	// Test the full workflow: create volume, then open as disk
	t.Run("create and open workflow", func(t *testing.T) {
		volumeId := "workflow-test"
		size := units.MegaBytes(64).Bytes()
		metadata := map[string]any{
			"name":       "my-test-disk",
			"filesystem": "ext4",
		}

		// Create volume
		returnedId, err := volOps.InitLSVDVolume(ctx, tmpDir, volumeId, size, metadata, false)
		require.NoError(t, err)
		assert.Equal(t, volumeId, returnedId)

		// Verify volume exists
		assert.True(t, volOps.VolumePathExists(filepath.Join(tmpDir, "volumes", returnedId)))

		// Open as disk
		disk, err := mountOps.OpenLSVDDisk(ctx, tmpDir, returnedId, false, "")
		require.NoError(t, err)
		require.NotNil(t, disk)

		// Verify size matches
		assert.Equal(t, size.Int64(), disk.Size())

		// Clean up
		err = disk.Close(ctx)
		require.NoError(t, err)

		// Remove volume
		err = volOps.RemoveVolumeDir(filepath.Join(tmpDir, "volumes", returnedId))
		require.NoError(t, err)
		assert.False(t, volOps.VolumePathExists(filepath.Join(tmpDir, "volumes", returnedId)))
	})
}

func TestVolumeOps_MultipleVolumes(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Integration tests only run on Linux")
	}

	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	tmpDir := t.TempDir()

	volOps := NewRealVolumeOps(log, nil, "")
	mountOps := NewRealMountOps(log, nil, "")

	// Create multiple volumes in the same data directory
	volumes := []struct {
		id   string
		size units.Bytes
		name string
	}{
		{"vol-1", units.MegaBytes(64).Bytes(), "first-disk"},
		{"vol-2", units.MegaBytes(128).Bytes(), "second-disk"},
		{"vol-3", units.MegaBytes(256).Bytes(), "third-disk"},
	}

	// Create all volumes
	for _, vol := range volumes {
		volDir := filepath.Join(tmpDir, vol.id)
		metadata := map[string]any{"name": vol.name}

		// Create the parent directory
		err := os.MkdirAll(volDir, 0755)
		require.NoError(t, err)

		returnedId, err := volOps.InitLSVDVolume(ctx, volDir, vol.id, vol.size, metadata, false)
		require.NoError(t, err)
		assert.Equal(t, vol.id, returnedId)
	}

	// Open all volumes and verify they don't interfere
	var disks []LSVDDisk
	for _, vol := range volumes {
		volDir := filepath.Join(tmpDir, vol.id)

		disk, err := mountOps.OpenLSVDDisk(ctx, volDir, vol.id, false, "")
		require.NoError(t, err, "failed to open volume %s", vol.id)
		require.NotNil(t, disk)

		assert.Equal(t, vol.size.Int64(), disk.Size(), "size mismatch for volume %s", vol.id)
		disks = append(disks, disk)
	}

	// Clean up all disks
	for _, disk := range disks {
		err := disk.Close(ctx)
		assert.NoError(t, err)
	}
}

// =============================================================================
// Edge Cases and Error Handling
// =============================================================================

func TestVolumeOps_ErrorCases(t *testing.T) {
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	t.Run("InitLSVDVolume with invalid path", func(t *testing.T) {
		ops := NewRealVolumeOps(log, nil, "")

		// Try to create volume in a path that can't be created (e.g., under /proc)
		_, err := ops.InitLSVDVolume(ctx, "/proc/invalid/path", "vol-id", units.GigaBytes(1).Bytes(), nil, false)
		assert.Error(t, err)
	})
}
