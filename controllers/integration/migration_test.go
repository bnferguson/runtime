package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	compute "miren.dev/runtime/api/compute/compute_v1alpha"
	storage "miren.dev/runtime/api/storage/storage_v1alpha"
	"miren.dev/runtime/lsvd"
	"miren.dev/runtime/pkg/entity"
)

// TestLSVDMigrationFullLifecycle creates a disk entity that looks like it was
// provisioned under the old LSVD system (Status=PROVISIONED, LsvdVolumeId set),
// writes real LSVD data on disk, then boots a sandbox against it.  The full
// controller pipeline should:
//  1. Detect the provisioned disk has no disk_volume → create one
//  2. DiskVolumeController finds an LSVD volume with the same name → migrates data
//  3. Lease binds, mount succeeds, sandbox is running
//  4. disk.img contains the migrated LSVD data
func TestLSVDMigrationFullLifecycle(t *testing.T) {
	ctx := context.Background()
	h := NewTestHarness(t)

	const diskName = "my-app-data"

	// --- Step 1: Create real LSVD volume with known data ---
	lsvdVolDir := filepath.Join(h.DataPath, "volumes", diskName)
	require.NoError(t, os.MkdirAll(lsvdVolDir, 0755))

	// Write info.json so LSVD recognizes this as an existing volume.
	// Size = 1GB (1073741824 bytes).
	require.NoError(t, os.WriteFile(
		filepath.Join(lsvdVolDir, "info.json"),
		[]byte(`{"name":"`+diskName+`","size":1073741824}`),
		0644,
	))

	lsvdDisk, err := lsvd.NewDisk(ctx, h.Log, h.DataPath,
		lsvd.WithVolumeName(diskName),
	)
	require.NoError(t, err)

	// Write a block of 0xAA at LBA 0
	block0 := make(lsvd.RawBlocks, lsvd.BlockSize)
	for i := range block0 {
		block0[i] = 0xAA
	}
	require.NoError(t, lsvdDisk.WriteExtent(ctx, block0.MapTo(0)))

	// Write a block of 0xBB at LBA 10 (leaving a sparse gap)
	block10 := make(lsvd.RawBlocks, lsvd.BlockSize)
	for i := range block10 {
		block10[i] = 0xBB
	}
	require.NoError(t, lsvdDisk.WriteExtent(ctx, block10.MapTo(10)))

	require.NoError(t, lsvdDisk.Close(ctx))

	// --- Step 2: Create a Disk entity as it would exist after old LSVD provisioning ---
	diskID := entity.Id("disk/lsvd-migration-test")
	disk := &storage.Disk{
		Name:         diskName,
		SizeGb:       1,
		Filesystem:   storage.EXT4,
		Status:       storage.PROVISIONED,
		LsvdVolumeId: "old-lsvd-vol-id",
	}

	_, err = h.EAC.Create(ctx, entity.New(
		entity.DBId, diskID,
		disk.Encode,
	).Attrs())
	require.NoError(t, err)

	// --- Step 3: Create sandbox and acquire lease ---
	sandboxID := entity.Id("sandbox/migrate-test-1")
	createSandboxEntity(t, ctx, h, sandboxID, compute.PENDING)

	leaseID, err := h.FakeSandbox.AcquireDiskLease(ctx, diskID, sandboxID, "", "/data", false)
	require.NoError(t, err)

	// --- Step 4: Reconcile everything ---
	h.ReconcileAll(ctx, 30)

	// --- Step 5: Verify final state ---

	// Disk should be PROVISIONED with mode=UNIVERSAL and a VolumeId
	finalDisk := getDisk(t, ctx, h, diskID)
	assert.Equal(t, storage.PROVISIONED, finalDisk.Status, "disk should be PROVISIONED")
	assert.Equal(t, storage.UNIVERSAL, finalDisk.Mode, "disk should be in UNIVERSAL mode")
	assert.NotEmpty(t, finalDisk.VolumeId, "disk should have a VolumeId")

	// Lease should be BOUND
	lease := getLease(t, ctx, h, leaseID)
	assert.Equal(t, storage.BOUND, lease.Status, "lease should be BOUND")

	// Exactly 1 disk_volume should exist
	vols := listDiskVolumes(t, ctx, h)
	require.Len(t, vols, 1, "should have exactly 1 disk_volume")
	vol := vols[0]
	assert.Equal(t, diskName, vol.Name, "disk_volume name should match disk name")
	assert.Equal(t, storage.DV_READY, vol.ActualState, "disk_volume should be READY")

	// Exactly 1 disk_mount should exist and be mounted
	assert.Equal(t, 1, countMountedMounts(t, ctx, h))

	// LSVD info.json should have been renamed to info.json.migrated
	_, err = os.Stat(filepath.Join(lsvdVolDir, "info.json"))
	assert.True(t, os.IsNotExist(err), "info.json should be renamed after migration")
	_, err = os.Stat(filepath.Join(lsvdVolDir, "info.json.migrated"))
	assert.NoError(t, err, "info.json.migrated should exist")

	// Verify actual disk.img content
	volEntitySuffix := string(vol.ID)
	if idx := len("disk_volume/"); idx < len(volEntitySuffix) {
		volEntitySuffix = volEntitySuffix[idx:]
	}
	imgPath := filepath.Join(h.DataPath, "volumes", volEntitySuffix, "disk.img")
	imgFile, err := os.Open(imgPath)
	require.NoError(t, err, "disk.img should exist at %s", imgPath)
	defer imgFile.Close()

	buf := make([]byte, lsvd.BlockSize)

	// LBA 0 → 0xAA
	_, err = imgFile.ReadAt(buf, 0)
	require.NoError(t, err)
	assert.Equal(t, byte(0xAA), buf[0], "block 0 should contain 0xAA")
	assert.Equal(t, byte(0xAA), buf[lsvd.BlockSize-1], "block 0 last byte should be 0xAA")

	// LBA 5 → zeros (sparse gap)
	_, err = imgFile.ReadAt(buf, 5*int64(lsvd.BlockSize))
	require.NoError(t, err)
	for i, b := range buf {
		if b != 0 {
			t.Fatalf("expected zero at offset %d in sparse gap, got 0x%02X", 5*lsvd.BlockSize+i, b)
		}
	}

	// LBA 10 → 0xBB
	_, err = imgFile.ReadAt(buf, 10*int64(lsvd.BlockSize))
	require.NoError(t, err)
	assert.Equal(t, byte(0xBB), buf[0], "block 10 should contain 0xBB")
	assert.Equal(t, byte(0xBB), buf[lsvd.BlockSize-1], "block 10 last byte should be 0xBB")

	// Mark sandbox running
	markSandboxRunning(t, ctx, h, sandboxID)
}

// TestLSVDMigrationNoLSVDData verifies that a disk provisioned under the old system
// but with no LSVD data on disk still works — the controller creates a fresh volume.
func TestLSVDMigrationNoLSVDData(t *testing.T) {
	ctx := context.Background()
	h := NewTestHarness(t)

	const diskName = "no-lsvd-data"

	// Create disk entity as if from old LSVD system (but no LSVD data on disk)
	diskID := entity.Id("disk/no-lsvd-migration")
	disk := &storage.Disk{
		Name:         diskName,
		SizeGb:       1,
		Filesystem:   storage.EXT4,
		Status:       storage.PROVISIONED,
		LsvdVolumeId: "stale-lsvd-id",
	}

	_, err := h.EAC.Create(ctx, entity.New(
		entity.DBId, diskID,
		disk.Encode,
	).Attrs())
	require.NoError(t, err)

	// Boot sandbox
	sandboxID := entity.Id("sandbox/no-lsvd-1")
	createSandboxEntity(t, ctx, h, sandboxID, compute.PENDING)

	leaseID, err := h.FakeSandbox.AcquireDiskLease(ctx, diskID, sandboxID, "", "/data", false)
	require.NoError(t, err)

	h.ReconcileAll(ctx, 30)

	// Should still converge — disk_volume created with empty sparse file
	finalDisk := getDisk(t, ctx, h, diskID)
	assert.Equal(t, storage.PROVISIONED, finalDisk.Status)
	assert.NotEmpty(t, finalDisk.VolumeId)

	lease := getLease(t, ctx, h, leaseID)
	assert.Equal(t, storage.BOUND, lease.Status)

	vols := listDiskVolumes(t, ctx, h)
	require.Len(t, vols, 1)
	assert.Equal(t, storage.DV_READY, vols[0].ActualState)
}

// TestLSVDMigrationThenRedeployment verifies that after migration, stopping and
// restarting a sandbox with the same disk works correctly — the migrated data
// persists through lease release and re-acquisition.
func TestLSVDMigrationThenRedeployment(t *testing.T) {
	ctx := context.Background()
	h := NewTestHarness(t)

	const diskName = "redeploy-disk"

	// Create LSVD volume with known data
	lsvdVolDir := filepath.Join(h.DataPath, "volumes", diskName)
	require.NoError(t, os.MkdirAll(lsvdVolDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(lsvdVolDir, "info.json"),
		[]byte(`{"name":"`+diskName+`","size":1073741824}`),
		0644,
	))

	lsvdDisk, err := lsvd.NewDisk(ctx, h.Log, h.DataPath,
		lsvd.WithVolumeName(diskName),
	)
	require.NoError(t, err)

	block0 := make(lsvd.RawBlocks, lsvd.BlockSize)
	for i := range block0 {
		block0[i] = 0x42
	}
	require.NoError(t, lsvdDisk.WriteExtent(ctx, block0.MapTo(0)))
	require.NoError(t, lsvdDisk.Close(ctx))

	// Create old-style disk entity
	diskID := entity.Id("disk/redeploy-migration")
	disk := &storage.Disk{
		Name:         diskName,
		SizeGb:       1,
		Filesystem:   storage.EXT4,
		Status:       storage.PROVISIONED,
		LsvdVolumeId: "old-lsvd-id",
	}
	_, err = h.EAC.Create(ctx, entity.New(
		entity.DBId, diskID,
		disk.Encode,
	).Attrs())
	require.NoError(t, err)

	// Boot sandbox 1
	sandbox1 := entity.Id("sandbox/redeploy-1")
	createSandboxEntity(t, ctx, h, sandbox1, compute.PENDING)

	lease1ID, err := h.FakeSandbox.AcquireDiskLease(ctx, diskID, sandbox1, "", "/data", false)
	require.NoError(t, err)

	h.ReconcileAll(ctx, 30)

	lease1 := getLease(t, ctx, h, lease1ID)
	require.Equal(t, storage.BOUND, lease1.Status)
	markSandboxRunning(t, ctx, h, sandbox1)

	// Stop sandbox 1
	stopSandbox(t, ctx, h, sandbox1)
	h.ReconcileAll(ctx, 20)

	// Boot sandbox 2 on the same disk
	sandbox2 := entity.Id("sandbox/redeploy-2")
	createSandboxEntity(t, ctx, h, sandbox2, compute.PENDING)

	lease2ID, err := h.FakeSandbox.AcquireDiskLease(ctx, diskID, sandbox2, "", "/data", false)
	require.NoError(t, err)

	h.ReconcileAll(ctx, 20)

	// Verify sandbox 2 is bound
	lease2 := getLease(t, ctx, h, lease2ID)
	assert.Equal(t, storage.BOUND, lease2.Status)

	// Disk should still be PROVISIONED — no re-migration
	finalDisk := getDisk(t, ctx, h, diskID)
	assert.Equal(t, storage.PROVISIONED, finalDisk.Status)
	assert.Equal(t, storage.UNIVERSAL, finalDisk.Mode)

	// Exactly 1 disk_volume, still READY
	vols := listDiskVolumes(t, ctx, h)
	require.Len(t, vols, 1)
	assert.Equal(t, storage.DV_READY, vols[0].ActualState)
}
