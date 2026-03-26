package diskio

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeletedVolumeGC_PurgesExpired(t *testing.T) {
	dataPath := t.TempDir()
	deletedDir := filepath.Join(dataPath, deletedVolumesDir)
	require.NoError(t, os.MkdirAll(deletedDir, 0755))

	// Create an expired entry (8 days old)
	expiredDir := filepath.Join(deletedDir, "vol-expired")
	require.NoError(t, os.MkdirAll(expiredDir, 0755))
	require.NoError(t, SaveDeletedVolumeMetadata(expiredDir, &DeletedVolumeMetadata{
		DiskName:  "old-disk",
		VolumeID:  "vol-expired",
		DeletedAt: time.Now().Add(-8 * 24 * time.Hour),
	}))
	// Add a fake disk.img so we can verify it's deleted
	require.NoError(t, os.WriteFile(filepath.Join(expiredDir, "disk.img"), []byte("data"), 0644))

	// Create a recent entry (1 day old)
	recentDir := filepath.Join(deletedDir, "vol-recent")
	require.NoError(t, os.MkdirAll(recentDir, 0755))
	require.NoError(t, SaveDeletedVolumeMetadata(recentDir, &DeletedVolumeMetadata{
		DiskName:  "new-disk",
		VolumeID:  "vol-recent",
		DeletedAt: time.Now().Add(-1 * 24 * time.Hour),
	}))

	gc := &DeletedVolumeGC{
		Log:      slog.Default(),
		DataPath: dataPath,
		Config: DeletedVolumeGCConfig{
			CheckInterval: time.Hour,
			RetentionDays: 7,
		},
	}

	result, err := gc.RunGC()
	require.NoError(t, err)

	assert.Equal(t, 1, result.Purged)
	assert.Equal(t, 1, result.Retained)
	assert.Equal(t, 0, result.Errors)

	// Verify expired dir is gone
	_, err = os.Stat(expiredDir)
	assert.True(t, os.IsNotExist(err))

	// Verify recent dir still exists
	_, err = os.Stat(recentDir)
	assert.NoError(t, err)
}

func TestDeletedVolumeGC_EmptyDir(t *testing.T) {
	dataPath := t.TempDir()

	gc := &DeletedVolumeGC{
		Log:      slog.Default(),
		DataPath: dataPath,
		Config:   DefaultDeletedVolumeGCConfig(),
	}

	result, err := gc.RunGC()
	require.NoError(t, err)
	assert.Equal(t, 0, result.Purged)
	assert.Equal(t, 0, result.Retained)
}

func TestDeletedVolumeGC_AllRetained(t *testing.T) {
	dataPath := t.TempDir()
	deletedDir := filepath.Join(dataPath, deletedVolumesDir)
	require.NoError(t, os.MkdirAll(deletedDir, 0755))

	// Create a recent entry
	dir := filepath.Join(deletedDir, "vol-new")
	require.NoError(t, os.MkdirAll(dir, 0755))
	require.NoError(t, SaveDeletedVolumeMetadata(dir, &DeletedVolumeMetadata{
		DiskName:  "recent-disk",
		VolumeID:  "vol-new",
		DeletedAt: time.Now().Add(-1 * time.Hour),
	}))

	gc := &DeletedVolumeGC{
		Log:      slog.Default(),
		DataPath: dataPath,
		Config:   DefaultDeletedVolumeGCConfig(),
	}

	result, err := gc.RunGC()
	require.NoError(t, err)
	assert.Equal(t, 0, result.Purged)
	assert.Equal(t, 1, result.Retained)
}
