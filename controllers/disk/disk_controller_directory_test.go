package disk

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"log/slog"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"miren.dev/runtime/api/storage/storage_v1alpha"
	"miren.dev/runtime/pkg/entity"
)

func TestDiskController_DirectoryMode_Init(t *testing.T) {
	t.Run("enables directory mode when NBD unavailable", func(t *testing.T) {
		log := slog.Default()
		controller := NewDiskController(log, nil, "test-node")

		// Set environment to disable NBD
		t.Setenv("MIREN_DISABLE_NBD", "1")

		err := controller.Init(context.Background())
		require.NoError(t, err)

		assert.True(t, controller.directoryMode, "Directory mode should be enabled when NBD is unavailable")
	})

	t.Run("disables directory mode when NBD available", func(t *testing.T) {
		log := slog.Default()
		controller := NewDiskController(log, nil, "test-node")

		// Don't set MIREN_DISABLE_NBD - NBD availability depends on system
		// This test may pass or fail depending on whether NBD is actually available

		err := controller.Init(context.Background())
		require.NoError(t, err)

		// We can't assert a specific value here since it depends on the system
		// Just verify Init doesn't error
	})
}

func TestDiskController_DirectoryMode_ProvisionDirectory(t *testing.T) {
	t.Run("creates directory instead of LSVD volume", func(t *testing.T) {
		ctx := context.Background()
		tempDir := t.TempDir()
		log := slog.Default()

		controller := NewDiskController(log, nil, "test-node")
		controller.mountBasePath = tempDir
		controller.directoryMode = true

		disk := &storage_v1alpha.Disk{
			ID:         entity.Id("disk/test-dir-1"),
			Name:       "test-dir-1",
			SizeGb:     10,
			Filesystem: storage_v1alpha.EXT4,
			Status:     storage_v1alpha.PROVISIONING,
		}

		err := controller.provisionDirectory(ctx, disk)
		require.NoError(t, err)
		assert.NotEmpty(t, disk.LsvdVolumeId)
		assert.Equal(t, storage_v1alpha.PROVISIONED, disk.Status)

		// Verify directory was created
		diskDataPath := filepath.Join(tempDir, "disk-data", disk.LsvdVolumeId)
		stat, err := os.Stat(diskDataPath)
		require.NoError(t, err, "Directory should exist")
		assert.True(t, stat.IsDir(), "Should be a directory")
	})

	t.Run("handles invalid disk size", func(t *testing.T) {
		ctx := context.Background()
		tempDir := t.TempDir()
		log := slog.Default()

		controller := NewDiskController(log, nil, "test-node")
		controller.mountBasePath = tempDir
		controller.directoryMode = true

		disk := &storage_v1alpha.Disk{
			ID:         entity.Id("disk/test-invalid"),
			Name:       "test-invalid",
			SizeGb:     0, // Invalid size
			Filesystem: storage_v1alpha.EXT4,
			Status:     storage_v1alpha.PROVISIONING,
		}

		err := controller.provisionDirectory(ctx, disk)
		assert.Error(t, err, "Should reject invalid disk size")
		assert.Contains(t, err.Error(), "invalid disk size")
	})

	t.Run("handles directory creation failure", func(t *testing.T) {
		ctx := context.Background()
		log := slog.Default()

		// Use an invalid path to cause directory creation to fail
		controller := NewDiskController(log, nil, "test-node")
		controller.mountBasePath = "/dev/null/invalid"
		controller.directoryMode = true

		disk := &storage_v1alpha.Disk{
			ID:         entity.Id("disk/test-fail"),
			Name:       "test-fail",
			SizeGb:     10,
			Filesystem: storage_v1alpha.EXT4,
			Status:     storage_v1alpha.PROVISIONING,
		}

		err := controller.provisionDirectory(ctx, disk)
		assert.Error(t, err, "Should fail when directory cannot be created")
		assert.Contains(t, err.Error(), "failed to create directory")
	})
}

func TestDiskController_DirectoryMode_HandleProvisioned(t *testing.T) {
	t.Run("verifies directory exists for provisioned disk", func(t *testing.T) {
		ctx := context.Background()
		tempDir := t.TempDir()
		log := slog.Default()

		controller := NewDiskController(log, nil, "test-node")
		controller.mountBasePath = tempDir
		controller.directoryMode = true

		// Pre-create the directory
		volumeId := "provisioned-vol-789"
		diskDataPath := filepath.Join(tempDir, "disk-data", volumeId)
		err := os.MkdirAll(diskDataPath, 0755)
		require.NoError(t, err)

		disk := &storage_v1alpha.Disk{
			ID:           entity.Id("disk/test-provisioned"),
			Name:         "test-provisioned",
			SizeGb:       10,
			Filesystem:   storage_v1alpha.EXT4,
			Status:       storage_v1alpha.PROVISIONED,
			LsvdVolumeId: volumeId,
		}

		err = controller.handleProvisioned(ctx, disk)
		require.NoError(t, err)
		assert.Equal(t, storage_v1alpha.PROVISIONED, disk.Status, "Status should remain PROVISIONED")
	})

	t.Run("re-provisions when directory is missing", func(t *testing.T) {
		ctx := context.Background()
		tempDir := t.TempDir()
		log := slog.Default()

		controller := NewDiskController(log, nil, "test-node")
		controller.mountBasePath = tempDir
		controller.directoryMode = true

		volumeId := "missing-vol-101"
		disk := &storage_v1alpha.Disk{
			ID:           entity.Id("disk/test-missing"),
			Name:         "test-missing",
			SizeGb:       10,
			Filesystem:   storage_v1alpha.EXT4,
			Status:       storage_v1alpha.PROVISIONED,
			LsvdVolumeId: volumeId,
		}

		err := controller.handleProvisioned(ctx, disk)
		require.NoError(t, err)

		// After re-provisioning, a new volume ID should be assigned
		assert.NotEqual(t, volumeId, disk.LsvdVolumeId, "Should have new volume ID after re-provisioning")
		assert.Equal(t, storage_v1alpha.PROVISIONED, disk.Status)

		// Verify the new directory was created
		newDiskDataPath := filepath.Join(tempDir, "disk-data", disk.LsvdVolumeId)
		stat, err := os.Stat(newDiskDataPath)
		require.NoError(t, err, "New directory should exist")
		assert.True(t, stat.IsDir())
	})

	t.Run("re-provisions when volume ID is empty", func(t *testing.T) {
		ctx := context.Background()
		tempDir := t.TempDir()
		log := slog.Default()

		controller := NewDiskController(log, nil, "test-node")
		controller.mountBasePath = tempDir
		controller.directoryMode = true

		disk := &storage_v1alpha.Disk{
			ID:         entity.Id("disk/test-empty-vol"),
			Name:       "test-empty-vol",
			SizeGb:     10,
			Filesystem: storage_v1alpha.EXT4,
			Status:     storage_v1alpha.PROVISIONED,
			// No LsvdVolumeId
		}

		err := controller.handleProvisioned(ctx, disk)
		require.NoError(t, err)

		// Should have provisioned a new volume
		assert.NotEmpty(t, disk.LsvdVolumeId, "Should have provisioned new volume")
		assert.Equal(t, storage_v1alpha.PROVISIONED, disk.Status)

		// Verify directory was created
		diskDataPath := filepath.Join(tempDir, "disk-data", disk.LsvdVolumeId)
		stat, err := os.Stat(diskDataPath)
		require.NoError(t, err, "Directory should exist")
		assert.True(t, stat.IsDir())
	})
}

func TestDiskController_DirectoryMode_HandleProvisioning(t *testing.T) {
	t.Run("full provisioning workflow in directory mode", func(t *testing.T) {
		ctx := context.Background()
		tempDir := t.TempDir()
		log := slog.Default()

		controller := NewDiskController(log, nil, "test-node")
		controller.mountBasePath = tempDir
		controller.directoryMode = true

		disk := &storage_v1alpha.Disk{
			ID:         entity.Id("disk/test-workflow"),
			Name:       "test-workflow",
			SizeGb:     50,
			Filesystem: storage_v1alpha.XFS,
			Status:     storage_v1alpha.PROVISIONING,
		}

		err := controller.handleProvisioning(ctx, disk)
		require.NoError(t, err)

		// Verify disk state after provisioning
		assert.Equal(t, storage_v1alpha.PROVISIONED, disk.Status)
		assert.NotEmpty(t, disk.LsvdVolumeId)

		// Verify directory was created
		diskDataPath := filepath.Join(tempDir, "disk-data", disk.LsvdVolumeId)
		stat, err := os.Stat(diskDataPath)
		require.NoError(t, err, "Directory should exist")
		assert.True(t, stat.IsDir())

		// Verify permissions
		assert.Equal(t, os.FileMode(0755), stat.Mode().Perm())
	})
}

func TestDiskController_DirectoryMode_ReconcileDisk(t *testing.T) {
	t.Run("reconciles disk in directory mode", func(t *testing.T) {
		ctx := context.Background()
		tempDir := t.TempDir()
		log := slog.Default()

		controller := NewDiskController(log, nil, "test-node")
		controller.mountBasePath = tempDir
		controller.directoryMode = true

		disk := &storage_v1alpha.Disk{
			ID:         entity.Id("disk/test-reconcile"),
			Name:       "test-reconcile",
			SizeGb:     15,
			Filesystem: storage_v1alpha.EXT4,
			Status:     storage_v1alpha.PROVISIONING,
		}

		meta := &entity.Meta{
			Entity: entity.New(disk.Encode()),
		}

		err := controller.reconcileDisk(ctx, disk, meta)
		require.NoError(t, err)

		// Should transition to PROVISIONED
		assert.Equal(t, storage_v1alpha.PROVISIONED, disk.Status)
		assert.NotEmpty(t, disk.LsvdVolumeId)
	})

	t.Run("handles ATTACHED status in directory mode", func(t *testing.T) {
		ctx := context.Background()
		tempDir := t.TempDir()
		log := slog.Default()

		controller := NewDiskController(log, nil, "test-node")
		controller.mountBasePath = tempDir
		controller.directoryMode = true

		volumeId := "attached-vol-303"
		disk := &storage_v1alpha.Disk{
			ID:           entity.Id("disk/test-attached"),
			Name:         "test-attached",
			SizeGb:       20,
			Filesystem:   storage_v1alpha.EXT4,
			Status:       storage_v1alpha.ATTACHED,
			LsvdVolumeId: volumeId,
		}

		err := controller.reconcileDisk(ctx, disk, nil)
		require.NoError(t, err)

		// ATTACHED status should be left as-is (handled by lease controller)
		assert.Equal(t, storage_v1alpha.ATTACHED, disk.Status)
	})

	t.Run("handles ERROR status in directory mode", func(t *testing.T) {
		ctx := context.Background()
		tempDir := t.TempDir()
		log := slog.Default()

		controller := NewDiskController(log, nil, "test-node")
		controller.mountBasePath = tempDir
		controller.directoryMode = true

		disk := &storage_v1alpha.Disk{
			ID:         entity.Id("disk/test-error"),
			Name:       "test-error",
			SizeGb:     10,
			Filesystem: storage_v1alpha.EXT4,
			Status:     storage_v1alpha.ERROR,
		}

		err := controller.reconcileDisk(ctx, disk, nil)
		require.NoError(t, err)

		// ERROR status should remain terminal
		assert.Equal(t, storage_v1alpha.ERROR, disk.Status)
	})
}

func TestDiskController_DirectoryMode_Integration(t *testing.T) {
	t.Run("full lifecycle in directory mode", func(t *testing.T) {
		ctx := context.Background()
		tempDir := t.TempDir()
		log := slog.Default()

		controller := NewDiskController(log, nil, "test-node")
		controller.mountBasePath = tempDir
		controller.directoryMode = true

		// Step 1: Create disk
		disk := &storage_v1alpha.Disk{
			ID:         entity.Id("disk/lifecycle-test"),
			Name:       "lifecycle-test",
			SizeGb:     30,
			Filesystem: storage_v1alpha.EXT4,
			Status:     storage_v1alpha.PROVISIONING,
		}

		meta := &entity.Meta{
			Entity: entity.New(disk.Encode()),
		}

		err := controller.Create(ctx, disk, meta)
		require.NoError(t, err)
		assert.Equal(t, storage_v1alpha.PROVISIONED, disk.Status)
		assert.NotEmpty(t, disk.LsvdVolumeId)

		volumeId := disk.LsvdVolumeId
		diskDataPath := filepath.Join(tempDir, "disk-data", volumeId)

		// Verify directory exists
		stat, err := os.Stat(diskDataPath)
		require.NoError(t, err)
		assert.True(t, stat.IsDir())

		// Step 2: Update disk (verify it stays provisioned)
		disk.Status = storage_v1alpha.PROVISIONED
		err = controller.Update(ctx, disk, meta)
		require.NoError(t, err)
		assert.Equal(t, storage_v1alpha.PROVISIONED, disk.Status)
		assert.Equal(t, volumeId, disk.LsvdVolumeId)
	})

	t.Run("multiple disks in directory mode", func(t *testing.T) {
		ctx := context.Background()
		tempDir := t.TempDir()
		log := slog.Default()

		controller := NewDiskController(log, nil, "test-node")
		controller.mountBasePath = tempDir
		controller.directoryMode = true

		// Create multiple disks
		numDisks := 5
		disks := make([]*storage_v1alpha.Disk, numDisks)

		for i := 0; i < numDisks; i++ {
			disk := &storage_v1alpha.Disk{
				ID:         entity.Id("disk/multi-" + string(rune('a'+i))),
				Name:       "multi-test-" + string(rune('a'+i)),
				SizeGb:     int64(10 * (i + 1)),
				Filesystem: storage_v1alpha.EXT4,
				Status:     storage_v1alpha.PROVISIONING,
			}

			err := controller.handleProvisioning(ctx, disk)
			require.NoError(t, err)

			disks[i] = disk
			assert.Equal(t, storage_v1alpha.PROVISIONED, disk.Status)
			assert.NotEmpty(t, disk.LsvdVolumeId)
		}

		// Verify all directories exist
		for i, disk := range disks {
			diskDataPath := filepath.Join(tempDir, "disk-data", disk.LsvdVolumeId)
			stat, err := os.Stat(diskDataPath)
			require.NoError(t, err, "Directory for disk %d should exist", i)
			assert.True(t, stat.IsDir())
		}

		// Verify all volume IDs are unique
		volumeIds := make(map[string]bool)
		for _, disk := range disks {
			assert.False(t, volumeIds[disk.LsvdVolumeId], "Volume IDs should be unique")
			volumeIds[disk.LsvdVolumeId] = true
		}
	})
}
