package disk

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"miren.dev/runtime/api/storage/storage_v1alpha"
	"miren.dev/runtime/pkg/entity"
)

func TestDiskAndLeaseIntegration(t *testing.T) {
	t.Run("lease conflict detection", func(t *testing.T) {
		log := slog.Default()
		leaseController := NewDiskLeaseController(log, nil, "test-node")
		ctx := context.Background()

		// Create a mock disk
		disk := &storage_v1alpha.Disk{
			ID:           entity.Id("disk/integration-disk"),
			Name:         "integration-disk",
			SizeGb:       50,
			Filesystem:   storage_v1alpha.EXT4,
			Status:       storage_v1alpha.PROVISIONED,
			LsvdVolumeId: "test-volume-123",
		}

		// Set disk in lease controller's test cache
		leaseController.SetTestDisk(disk)

		// Manually track an active lease
		leaseController.activeLeases[disk.ID.String()] = "disk-lease/existing-lease"
		leaseController.leaseDetails["disk-lease/existing-lease"] = &leaseInfo{
			leaseId:   "disk-lease/existing-lease",
			diskId:    disk.ID.String(),
			sandboxId: "sandbox/existing-sandbox",
			volumeId:  disk.LsvdVolumeId,
		}

		// Try to bind another lease for the same disk (should fail)
		conflictLease := &storage_v1alpha.DiskLease{
			ID:        entity.Id("disk-lease/conflict-lease"),
			DiskId:    disk.ID,
			SandboxId: entity.Id("sandbox/another-sandbox"),
			Status:    storage_v1alpha.PENDING,
			Mount: storage_v1alpha.Mount{
				Path: "/data/conflict",
			},
		}

		conflictMeta := &entity.Meta{}
		err := leaseController.Create(ctx, conflictLease, conflictMeta)
		require.NoError(t, err)

		// Should fail with conflict
		hasFailure := false
		for _, attr := range conflictMeta.Attrs() {
			if attr.ID == storage_v1alpha.DiskLeaseStatusId {
				assert.Equal(t, storage_v1alpha.DiskLeaseStatusFailedId, attr.Value.Id())
				hasFailure = true
			}
			if attr.ID == storage_v1alpha.DiskLeaseErrorMessageId {
				assert.Contains(t, attr.Value.String(), "already leased")
			}
		}
		assert.True(t, hasFailure, "Conflicting lease should fail")
	})

	t.Run("lease release flow", func(t *testing.T) {
		log := slog.Default()
		leaseController := NewDiskLeaseController(log, nil, "test-node")
		ctx := context.Background()

		// Setup active lease
		diskId := "disk/release-test-disk"
		leaseId := "disk-lease/release-test-lease"
		leaseController.activeLeases[diskId] = leaseId
		leaseController.leaseDetails[leaseId] = &leaseInfo{
			leaseId:   leaseId,
			diskId:    diskId,
			sandboxId: "sandbox/test-sandbox",
			volumeId:  "test-volume-456",
		}

		// Release the lease
		lease := &storage_v1alpha.DiskLease{
			ID:        entity.Id(leaseId),
			DiskId:    entity.Id(diskId),
			SandboxId: entity.Id("sandbox/test-sandbox"),
			Status:    storage_v1alpha.RELEASED,
		}

		releaseMeta := &entity.Meta{}
		err := leaseController.Update(ctx, lease, releaseMeta)
		require.NoError(t, err)

		// Verify lease is no longer tracked
		leaseController.mu.RLock()
		_, exists := leaseController.activeLeases[diskId]
		leaseController.mu.RUnlock()
		assert.False(t, exists, "Released lease should not be tracked")
	})

	t.Run("cleanup old released leases", func(t *testing.T) {
		log := slog.Default()
		leaseController := NewDiskLeaseController(log, nil, "test-node")
		ctx := context.Background()

		// Test that cleanup doesn't fail in test mode (no EAC)
		err := leaseController.CleanupOldReleasedLeases(ctx)
		assert.NoError(t, err, "Cleanup should handle nil EAC gracefully")
	})
}
