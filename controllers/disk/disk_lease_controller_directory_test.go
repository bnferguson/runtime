package disk

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"miren.dev/runtime/api/storage/storage_v1alpha"
	"miren.dev/runtime/pkg/entity"
)

func TestDiskLeaseController_DirectoryMode_Init(t *testing.T) {
	t.Run("enables directory mode when NBD unavailable", func(t *testing.T) {
		log := slog.Default()
		controller := NewDiskLeaseController(log, nil, "test-node")

		// Set environment to disable NBD
		t.Setenv("MIREN_DISABLE_NBD", "1")

		err := controller.Init(context.Background())
		require.NoError(t, err)

		assert.True(t, controller.directoryMode, "Directory mode should be enabled when NBD is unavailable")
	})

	t.Run("disables directory mode when NBD available", func(t *testing.T) {
		log := slog.Default()
		controller := NewDiskLeaseController(log, nil, "test-node")

		// Don't set MIREN_DISABLE_NBD - NBD availability depends on system
		err := controller.Init(context.Background())
		require.NoError(t, err)

		// We can't assert a specific value here since it depends on the system
		// Just verify Init doesn't error
	})
}

func TestDiskLeaseController_DirectoryMode_HandlePendingLease(t *testing.T) {
	t.Run("binds lease to directory", func(t *testing.T) {
		ctx := context.Background()
		tempDir := t.TempDir()
		log := slog.Default()

		dlc := NewDiskLeaseController(log, nil, "test-node")
		dlc.mountBasePath = tempDir
		dlc.directoryMode = true

		// Create directory for the volume
		volumeId := "dir-vol-123"
		diskDataPath := filepath.Join(tempDir, "disk-data", volumeId)
		err := os.MkdirAll(diskDataPath, 0755)
		require.NoError(t, err)

		// Create a test disk
		disk := &storage_v1alpha.Disk{
			ID:           entity.Id("disk/dir-test-disk"),
			Status:       storage_v1alpha.PROVISIONED,
			LsvdVolumeId: volumeId,
			Filesystem:   storage_v1alpha.EXT4,
		}
		dlc.SetTestDisk(disk)

		// Create a pending lease
		lease := &storage_v1alpha.DiskLease{
			ID:        entity.Id("disk-lease/dir-test-lease"),
			DiskId:    entity.Id("disk/dir-test-disk"),
			SandboxId: entity.Id("sandbox/dir-test-sandbox"),
			Status:    storage_v1alpha.PENDING,
			Mount: storage_v1alpha.Mount{
				Path:     "/data",
				ReadOnly: false,
			},
		}

		// Process the lease
		meta := &entity.Meta{}
		err = dlc.Create(ctx, lease, meta)
		require.NoError(t, err)

		// Should update status to BOUND
		assert.Equal(t, storage_v1alpha.BOUND, lease.Status)
		assert.Empty(t, lease.ErrorMessage)

		// Verify lease is tracked
		currentLease, exists := dlc.activeLeases["disk/dir-test-disk"]
		assert.True(t, exists)
		assert.Equal(t, "disk-lease/dir-test-lease", currentLease)
	})

	t.Run("fails when directory does not exist", func(t *testing.T) {
		ctx := context.Background()
		tempDir := t.TempDir()
		log := slog.Default()

		dlc := NewDiskLeaseController(log, nil, "test-node")
		dlc.mountBasePath = tempDir
		dlc.directoryMode = true

		volumeId := "missing-dir-vol"
		// Don't create the directory

		// Create a test disk
		disk := &storage_v1alpha.Disk{
			ID:           entity.Id("disk/missing-dir-disk"),
			Status:       storage_v1alpha.PROVISIONED,
			LsvdVolumeId: volumeId,
			Filesystem:   storage_v1alpha.EXT4,
		}
		dlc.SetTestDisk(disk)

		// Create a pending lease
		lease := &storage_v1alpha.DiskLease{
			ID:        entity.Id("disk-lease/missing-dir-lease"),
			DiskId:    entity.Id("disk/missing-dir-disk"),
			SandboxId: entity.Id("sandbox/missing-dir-sandbox"),
			Status:    storage_v1alpha.PENDING,
			Mount: storage_v1alpha.Mount{
				Path: "/data",
			},
		}

		// Process the lease
		meta := &entity.Meta{}
		err := dlc.Create(ctx, lease, meta)
		require.NoError(t, err)

		// Should update status to FAILED
		assert.Equal(t, storage_v1alpha.FAILED, lease.Status)
		assert.Contains(t, lease.ErrorMessage, "Directory not found")

		// Verify lease is not tracked
		_, exists := dlc.activeLeases["disk/missing-dir-disk"]
		assert.False(t, exists, "Failed lease should not be tracked")
	})
}

func TestDiskLeaseController_DirectoryMode_HandleReleasedLease(t *testing.T) {
	t.Run("releases lease in directory mode", func(t *testing.T) {
		ctx := context.Background()
		tempDir := t.TempDir()
		log := slog.Default()

		dlc := NewDiskLeaseController(log, nil, "test-node")
		dlc.mountBasePath = tempDir
		dlc.directoryMode = true

		volumeId := "released-dir-vol"

		// Setup active lease
		dlc.activeLeases["disk/released-dir-disk"] = "disk-lease/released-dir-lease"
		dlc.leaseDetails["disk-lease/released-dir-lease"] = &leaseInfo{
			leaseId:   "disk-lease/released-dir-lease",
			diskId:    "disk/released-dir-disk",
			sandboxId: "sandbox/released-dir-sandbox",
			volumeId:  volumeId,
		}

		// Create a released lease
		releasedLease := &storage_v1alpha.DiskLease{
			ID:        entity.Id("disk-lease/released-dir-lease"),
			DiskId:    entity.Id("disk/released-dir-disk"),
			SandboxId: entity.Id("sandbox/released-dir-sandbox"),
			Status:    storage_v1alpha.RELEASED,
		}

		// Process the release
		meta := &entity.Meta{}
		err := dlc.Update(ctx, releasedLease, meta)
		require.NoError(t, err)

		// Should remove from active leases
		_, exists := dlc.activeLeases["disk/released-dir-disk"]
		assert.False(t, exists, "Should remove released lease from active leases")

		// Should remove from lease details
		_, detailsExist := dlc.leaseDetails["disk-lease/released-dir-lease"]
		assert.False(t, detailsExist, "Should remove from lease details")
	})
}

func TestDiskLeaseController_DirectoryMode_Integration(t *testing.T) {
	t.Run("full lifecycle in directory mode", func(t *testing.T) {
		ctx := context.Background()
		tempDir := t.TempDir()
		log := slog.Default()

		dlc := NewDiskLeaseController(log, nil, "test-node")
		dlc.mountBasePath = tempDir
		dlc.directoryMode = true

		// Create directory for the volume
		volumeId := "lifecycle-dir-vol"
		diskDataPath := filepath.Join(tempDir, "disk-data", volumeId)
		err := os.MkdirAll(diskDataPath, 0755)
		require.NoError(t, err)

		// Create a test disk
		disk := &storage_v1alpha.Disk{
			ID:           entity.Id("disk/lifecycle-dir-disk"),
			Status:       storage_v1alpha.PROVISIONED,
			LsvdVolumeId: volumeId,
			Filesystem:   storage_v1alpha.EXT4,
		}
		dlc.SetTestDisk(disk)

		// Step 1: Create and bind a pending lease
		lease := &storage_v1alpha.DiskLease{
			ID:        entity.Id("disk-lease/lifecycle-dir-lease"),
			DiskId:    entity.Id("disk/lifecycle-dir-disk"),
			SandboxId: entity.Id("sandbox/lifecycle-dir-sandbox"),
			Status:    storage_v1alpha.PENDING,
			Mount: storage_v1alpha.Mount{
				Path: "/data",
			},
		}

		meta := &entity.Meta{}
		err = dlc.Create(ctx, lease, meta)
		require.NoError(t, err)

		// Verify lease is bound
		assert.Equal(t, storage_v1alpha.BOUND, lease.Status)
		currentLease, exists := dlc.activeLeases["disk/lifecycle-dir-disk"]
		assert.True(t, exists)
		assert.Equal(t, "disk-lease/lifecycle-dir-lease", currentLease)

		// Step 2: Release the lease
		lease.Status = storage_v1alpha.RELEASED
		meta3 := &entity.Meta{}
		err = dlc.Update(ctx, lease, meta3)
		require.NoError(t, err)

		// Verify lease is released
		_, exists = dlc.activeLeases["disk/lifecycle-dir-disk"]
		assert.False(t, exists)

		// Step 3: Delete the lease
		err = dlc.Delete(ctx, lease.ID, nil)
		require.NoError(t, err)

		// Verify complete cleanup
		assert.Empty(t, dlc.activeLeases)
		assert.Empty(t, dlc.leaseDetails)
	})
}
