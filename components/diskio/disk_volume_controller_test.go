package diskio

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"miren.dev/runtime/api/entityserver/entityserver_v1alpha"
	"miren.dev/runtime/api/storage/storage_v1alpha"
	"miren.dev/runtime/lsvd"
	"miren.dev/runtime/pkg/entity"
	"miren.dev/runtime/pkg/entity/testutils"
)

func newTestDiskVolumeController(log *slog.Logger, dataPath, nodeId string, eac *entityserver_v1alpha.EntityAccessClient, state *State, ops DiskVolumeOps) *DiskVolumeController {
	mntOps := newMockDiskMountOps()
	vc := NewDiskVolumeController(log, dataPath, nodeId, state, ops, mntOps)
	vc.SetEAC(eac)
	return vc
}

func createDiskVolumeEntity(ctx context.Context, t *testing.T, es *testutils.InMemEntityServer, vol *storage_v1alpha.DiskVolume) {
	_, err := es.EAC.Create(ctx, entity.New(
		entity.DBId, vol.ID,
		vol.Encode,
	).Attrs())
	require.NoError(t, err)
}

func TestDiskVolumeControllerReconcileVolumePresent(t *testing.T) {
	ctx := t.Context()
	log := testutils.TestLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	ops := newMockDiskVolumeOps()

	vc := newTestDiskVolumeController(log, dataPath, nodeId, es.EAC, state, ops)

	vol := &storage_v1alpha.DiskVolume{
		ID:           "disk_volume/vol-123",
		NodeId:       entity.Id("node/" + nodeId),
		SizeGb:       10,
		Filesystem:   "ext4",
		DesiredState: storage_v1alpha.DV_PRESENT,
		ActualState:  storage_v1alpha.DV_PENDING,
	}
	createDiskVolumeEntity(ctx, t, es, vol)

	err := vc.ReconcileWithEntities(ctx)
	require.NoError(t, err)

	// Verify volume directory was created
	assert.Len(t, ops.createdDirs, 1)
	expectedVolPath := filepath.Join(dataPath, "volumes", "vol-123")
	assert.Equal(t, expectedVolPath, ops.createdDirs[0])

	// Verify sparse disk image was created
	assert.Len(t, ops.createdImages, 1)
	assert.Equal(t, filepath.Join(expectedVolPath, "disk.img"), ops.createdImages[0].path)
	assert.Equal(t, int64(10*1024*1024*1024), ops.createdImages[0].sizeBytes)

	// Verify state was updated
	volState := state.GetVolume("disk_volume/vol-123")
	require.NotNil(t, volState)
	assert.Equal(t, "disk_volume/vol-123", volState.EntityId)
	assert.Equal(t, "vol-123", volState.VolumeId)
	assert.Equal(t, int64(10*1024*1024*1024), volState.SizeBytes)
	assert.Equal(t, "ext4", volState.Filesystem)

	// Verify entity was updated to READY
	resp, err := es.EAC.Get(ctx, "disk_volume/vol-123")
	require.NoError(t, err)
	var updated storage_v1alpha.DiskVolume
	updated.Decode(resp.Entity().Entity())
	assert.Equal(t, storage_v1alpha.DV_READY, updated.ActualState)
	assert.Equal(t, "vol-123", updated.VolumeId)
	assert.Equal(t, filepath.Join(expectedVolPath, "disk.img"), updated.ImagePath)
}

func TestDiskVolumeControllerReconcileVolumeAbsent(t *testing.T) {
	ctx := t.Context()
	log := testutils.TestLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	ops := newMockDiskVolumeOps()

	// Pre-populate state with existing volume
	state.SetVolume("disk_volume/vol-456", &VolumeState{
		EntityId:   "disk_volume/vol-456",
		VolumeId:   "vol-456",
		DiskPath:   "/data/volumes/vol-456",
		SizeBytes:  5 * 1024 * 1024 * 1024,
		Filesystem: "xfs",
	})
	ops.existingPaths["/data/volumes/vol-456"] = true

	vc := newTestDiskVolumeController(log, dataPath, nodeId, es.EAC, state, ops)

	vol := &storage_v1alpha.DiskVolume{
		ID:           "disk_volume/vol-456",
		NodeId:       entity.Id("node/" + nodeId),
		SizeGb:       5,
		Filesystem:   "xfs",
		DesiredState: storage_v1alpha.DV_ABSENT,
		ActualState:  storage_v1alpha.DV_READY,
	}
	createDiskVolumeEntity(ctx, t, es, vol)

	err := vc.ReconcileWithEntities(ctx)
	require.NoError(t, err)

	// Verify volume directory was removed
	assert.Len(t, ops.removedDirs, 1)
	assert.Equal(t, "/data/volumes/vol-456", ops.removedDirs[0])

	// Verify state was cleaned up
	assert.Nil(t, state.GetVolume("disk_volume/vol-456"))

	// Verify entity was updated to DELETED
	resp, err := es.EAC.Get(ctx, "disk_volume/vol-456")
	require.NoError(t, err)
	var updated storage_v1alpha.DiskVolume
	updated.Decode(resp.Entity().Entity())
	assert.Equal(t, storage_v1alpha.DV_DELETED, updated.ActualState)
}

func TestDiskVolumeControllerReconcileSkipsOtherNodes(t *testing.T) {
	ctx := t.Context()
	log := testutils.TestLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	ops := newMockDiskVolumeOps()

	vc := newTestDiskVolumeController(log, dataPath, nodeId, es.EAC, state, ops)

	vol := &storage_v1alpha.DiskVolume{
		ID:           "disk_volume/vol-other",
		NodeId:       entity.Id("node/other-node"),
		SizeGb:       10,
		DesiredState: storage_v1alpha.DV_PRESENT,
		ActualState:  storage_v1alpha.DV_PENDING,
	}
	createDiskVolumeEntity(ctx, t, es, vol)

	err := vc.ReconcileWithEntities(ctx)
	require.NoError(t, err)

	assert.Empty(t, ops.createdDirs)
	assert.Empty(t, ops.createdImages)
}

func TestDiskVolumeControllerReconcileVolumeAlreadyReady(t *testing.T) {
	ctx := t.Context()
	log := testutils.TestLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	ops := newMockDiskVolumeOps()

	volPath := filepath.Join(dataPath, "volumes", "vol-ready")
	state.SetVolume("disk_volume/vol-ready", &VolumeState{
		EntityId:   "disk_volume/vol-ready",
		VolumeId:   "vol-ready",
		DiskPath:   volPath,
		SizeBytes:  10 * 1024 * 1024 * 1024,
		Filesystem: "ext4",
	})
	ops.existingPaths[volPath] = true

	vc := newTestDiskVolumeController(log, dataPath, nodeId, es.EAC, state, ops)

	vol := &storage_v1alpha.DiskVolume{
		ID:           "disk_volume/vol-ready",
		NodeId:       entity.Id("node/" + nodeId),
		SizeGb:       10,
		Filesystem:   "ext4",
		DesiredState: storage_v1alpha.DV_PRESENT,
		ActualState:  storage_v1alpha.DV_READY,
		VolumeId:     "vol-ready",
	}
	createDiskVolumeEntity(ctx, t, es, vol)

	err := vc.ReconcileWithEntities(ctx)
	require.NoError(t, err)

	assert.Empty(t, ops.createdDirs)
	assert.Empty(t, ops.createdImages)
}

func TestDiskVolumeControllerReconcileVolumeReadyButMissing(t *testing.T) {
	ctx := t.Context()
	log := testutils.TestLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	ops := newMockDiskVolumeOps()

	// State says volume exists, but path does NOT exist on disk
	state.SetVolume("disk_volume/vol-missing", &VolumeState{
		EntityId:   "disk_volume/vol-missing",
		VolumeId:   "vol-missing",
		DiskPath:   "/data/volumes/vol-missing",
		SizeBytes:  10 * 1024 * 1024 * 1024,
		Filesystem: "ext4",
	})
	// Do NOT mark path as existing

	vc := newTestDiskVolumeController(log, dataPath, nodeId, es.EAC, state, ops)

	vol := &storage_v1alpha.DiskVolume{
		ID:           "disk_volume/vol-missing",
		NodeId:       entity.Id("node/" + nodeId),
		SizeGb:       10,
		Filesystem:   "ext4",
		DesiredState: storage_v1alpha.DV_PRESENT,
		ActualState:  storage_v1alpha.DV_READY,
		VolumeId:     "vol-missing",
	}
	createDiskVolumeEntity(ctx, t, es, vol)

	// Reconciliation should fail and set error state
	err := vc.ReconcileWithEntities(ctx)
	require.NoError(t, err) // ReconcileWithEntities logs errors but doesn't return them

	// Verify entity was set to error state
	resp, err := es.EAC.Get(ctx, "disk_volume/vol-missing")
	require.NoError(t, err)
	var updated storage_v1alpha.DiskVolume
	updated.Decode(resp.Entity().Entity())
	assert.Equal(t, storage_v1alpha.DV_ERROR, updated.ActualState)
	assert.Contains(t, updated.ErrorMessage, "volume directory missing")
}

func TestDiskVolumeControllerReconcileVolumeErrorRetry(t *testing.T) {
	ctx := t.Context()
	log := testutils.TestLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	ops := newMockDiskVolumeOps()

	vc := newTestDiskVolumeController(log, dataPath, nodeId, es.EAC, state, ops)

	vol := &storage_v1alpha.DiskVolume{
		ID:           "disk_volume/vol-err",
		NodeId:       entity.Id("node/" + nodeId),
		SizeGb:       10,
		Filesystem:   "ext4",
		DesiredState: storage_v1alpha.DV_PRESENT,
		ActualState:  storage_v1alpha.DV_ERROR,
	}
	createDiskVolumeEntity(ctx, t, es, vol)

	err := vc.ReconcileWithEntities(ctx)
	require.NoError(t, err)

	// Should have recreated the volume
	assert.Len(t, ops.createdDirs, 1)
	assert.Len(t, ops.createdImages, 1)
}

func TestDiskVolumeControllerReconcileCleansUpOrphanedVolumes(t *testing.T) {
	ctx := t.Context()
	log := testutils.TestLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	ops := newMockDiskVolumeOps()

	// Pre-populate local state with a volume that has no corresponding entity
	state.SetVolume("disk_volume/vol-orphan", &VolumeState{
		EntityId:   "disk_volume/vol-orphan",
		VolumeId:   "vol-orphan",
		DiskPath:   "/data/volumes/vol-orphan",
		SizeBytes:  10 * 1024 * 1024 * 1024,
		Filesystem: "ext4",
	})
	ops.existingPaths["/data/volumes/vol-orphan"] = true

	vc := newTestDiskVolumeController(log, dataPath, nodeId, es.EAC, state, ops)

	err := vc.ReconcileWithEntities(ctx)
	require.NoError(t, err)

	// Verify volume directory was removed
	assert.Contains(t, ops.removedDirs, "/data/volumes/vol-orphan")

	// Verify state was cleaned up
	assert.Nil(t, state.GetVolume("disk_volume/vol-orphan"))
}

func TestDiskVolumeControllerReconcileKeepsNonOrphanedVolumes(t *testing.T) {
	ctx := t.Context()
	log := testutils.TestLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	ops := newMockDiskVolumeOps()

	volPath := filepath.Join(dataPath, "volumes", "vol-keep")
	state.SetVolume("disk_volume/vol-keep", &VolumeState{
		EntityId:   "disk_volume/vol-keep",
		VolumeId:   "vol-keep",
		DiskPath:   volPath,
		SizeBytes:  10 * 1024 * 1024 * 1024,
		Filesystem: "ext4",
	})
	ops.existingPaths[volPath] = true

	// Also add an orphan
	state.SetVolume("disk_volume/vol-orphan2", &VolumeState{
		EntityId:   "disk_volume/vol-orphan2",
		VolumeId:   "vol-orphan2",
		DiskPath:   "/data/volumes/vol-orphan2",
		SizeBytes:  5 * 1024 * 1024 * 1024,
		Filesystem: "ext4",
	})
	ops.existingPaths["/data/volumes/vol-orphan2"] = true

	vc := newTestDiskVolumeController(log, dataPath, nodeId, es.EAC, state, ops)

	vol := &storage_v1alpha.DiskVolume{
		ID:           "disk_volume/vol-keep",
		NodeId:       entity.Id("node/" + nodeId),
		SizeGb:       10,
		Filesystem:   "ext4",
		DesiredState: storage_v1alpha.DV_PRESENT,
		ActualState:  storage_v1alpha.DV_READY,
		VolumeId:     "vol-keep",
	}
	createDiskVolumeEntity(ctx, t, es, vol)

	err := vc.ReconcileWithEntities(ctx)
	require.NoError(t, err)

	assert.NotNil(t, state.GetVolume("disk_volume/vol-keep"))
	assert.Nil(t, state.GetVolume("disk_volume/vol-orphan2"))
	assert.Contains(t, ops.removedDirs, "/data/volumes/vol-orphan2")
	assert.NotContains(t, ops.removedDirs, volPath)
}

func TestDiskVolumeControllerMultipleVolumes(t *testing.T) {
	ctx := t.Context()
	log := testutils.TestLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	ops := newMockDiskVolumeOps()

	vc := newTestDiskVolumeController(log, dataPath, nodeId, es.EAC, state, ops)

	for i := 1; i <= 3; i++ {
		vol := &storage_v1alpha.DiskVolume{
			ID:           entity.Id("disk_volume/vol-" + string(rune('0'+i))),
			NodeId:       entity.Id("node/" + nodeId),
			SizeGb:       int64(i * 10),
			Filesystem:   "ext4",
			DesiredState: storage_v1alpha.DV_PRESENT,
			ActualState:  storage_v1alpha.DV_PENDING,
		}
		createDiskVolumeEntity(ctx, t, es, vol)
	}

	err := vc.ReconcileWithEntities(ctx)
	require.NoError(t, err)

	assert.Len(t, ops.createdDirs, 3)
	assert.Len(t, ops.createdImages, 3)
	assert.Len(t, state.Volumes, 3)
}

func TestDiskVolumeControllerReconcilePersistedVolumeOnDisk(t *testing.T) {
	ctx := t.Context()
	log := testutils.TestLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	ops := newMockDiskVolumeOps()

	volPath := filepath.Join(dataPath, "volumes", "vol-persist")
	// State has disk path and it exists on disk
	state.SetVolume("disk_volume/vol-persist", &VolumeState{
		EntityId:   "disk_volume/vol-persist",
		VolumeId:   "vol-persist",
		DiskPath:   volPath,
		SizeBytes:  10 * 1024 * 1024 * 1024,
		Filesystem: "ext4",
	})
	ops.existingPaths[volPath] = true

	vc := newTestDiskVolumeController(log, dataPath, nodeId, es.EAC, state, ops)

	// Entity is in PENDING state but local state has the volume on disk
	vol := &storage_v1alpha.DiskVolume{
		ID:           "disk_volume/vol-persist",
		NodeId:       entity.Id("node/" + nodeId),
		SizeGb:       10,
		Filesystem:   "ext4",
		DesiredState: storage_v1alpha.DV_PRESENT,
		ActualState:  storage_v1alpha.DV_PENDING,
	}
	createDiskVolumeEntity(ctx, t, es, vol)

	err := vc.ReconcileWithEntities(ctx)
	require.NoError(t, err)

	// Should NOT recreate (state found on disk), just update entity
	assert.Empty(t, ops.createdDirs)
	assert.Empty(t, ops.createdImages)

	// Entity should now be READY
	resp, err := es.EAC.Get(ctx, "disk_volume/vol-persist")
	require.NoError(t, err)
	var updated storage_v1alpha.DiskVolume
	updated.Decode(resp.Entity().Entity())
	assert.Equal(t, storage_v1alpha.DV_READY, updated.ActualState)
}

func TestDiskVolumeControllerDeleteNotInState(t *testing.T) {
	ctx := t.Context()
	log := testutils.TestLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	ops := newMockDiskVolumeOps()

	vc := newTestDiskVolumeController(log, dataPath, nodeId, es.EAC, state, ops)

	// Volume not in local state but entity requests deletion
	vol := &storage_v1alpha.DiskVolume{
		ID:           "disk_volume/vol-gone",
		NodeId:       entity.Id("node/" + nodeId),
		SizeGb:       10,
		DesiredState: storage_v1alpha.DV_ABSENT,
		ActualState:  storage_v1alpha.DV_READY,
	}
	createDiskVolumeEntity(ctx, t, es, vol)

	err := vc.ReconcileWithEntities(ctx)
	require.NoError(t, err)

	// Should still transition to DELETED even without local state
	resp, err := es.EAC.Get(ctx, "disk_volume/vol-gone")
	require.NoError(t, err)
	var updated storage_v1alpha.DiskVolume
	updated.Decode(resp.Entity().Entity())
	assert.Equal(t, storage_v1alpha.DV_DELETED, updated.ActualState)
}

func TestDiskVolumeControllerMigrateLSVDVolume(t *testing.T) {
	ctx := t.Context()
	log := testutils.TestLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	dataPath := t.TempDir()
	nodeId := "test-node-1"

	const lsvdEntitySuffix = "lsvd-vol-migrate1"
	const lsvdVolumeUUID = "uuid-migrate1"
	const diskName = "test-disk"
	diskID := entity.Id("disk/test-migrate")

	// Create LSVD data with production nested directory layout:
	// volumes/{lsvdEntitySuffix}/volumes/{uuid}/info.json
	lsvdEntityDir := filepath.Join(dataPath, "volumes", lsvdEntitySuffix)
	require.NoError(t, os.MkdirAll(lsvdEntityDir, 0755))

	lsvdDisk, err := lsvd.NewDisk(ctx, log, lsvdEntityDir,
		lsvd.WithVolumeName(lsvdVolumeUUID))
	require.NoError(t, err)

	// Write block of 0x42 at LBA 0
	block0 := make(lsvd.RawBlocks, lsvd.BlockSize)
	for i := range block0 {
		block0[i] = 0x42
	}
	require.NoError(t, lsvdDisk.WriteExtent(ctx, block0.MapTo(0)))

	// Write block of 0xFF at LBA 5
	block5 := make(lsvd.RawBlocks, lsvd.BlockSize)
	for i := range block5 {
		block5[i] = 0xFF
	}
	require.NoError(t, lsvdDisk.WriteExtent(ctx, block5.MapTo(5)))
	require.NoError(t, lsvdDisk.Close(ctx))

	// Create the disk entity with LsvdVolumeId set
	diskEnt := &storage_v1alpha.Disk{
		Name:         diskName,
		SizeGb:       1,
		Filesystem:   storage_v1alpha.EXT4,
		Status:       storage_v1alpha.PROVISIONED,
		LsvdVolumeId: lsvdVolumeUUID, // In production this is the LSVD volume UUID
	}
	_, err = es.EAC.Create(ctx, entity.New(
		entity.DBId, diskID,
		diskEnt.Encode,
	).Attrs())
	require.NoError(t, err)

	// Create the lsvd_volume entity
	lsvdVolEnt := &storage_v1alpha.LsvdVolume{
		DiskId:      diskID,
		VolumeId:    lsvdVolumeUUID,
		Name:        diskName,
		SizeGb:      1,
		Filesystem:  "ext4",
		ActualState: storage_v1alpha.VOL_READY,
		NodeId:      entity.Id("node/" + nodeId),
	}
	_, err = es.EAC.Create(ctx, entity.New(
		entity.DBId, entity.Id("lsvd_volume/"+lsvdEntitySuffix),
		lsvdVolEnt.Encode,
	).Attrs())
	require.NoError(t, err)

	state := NewState()
	ops := newMockDiskVolumeOps()
	vc := newTestDiskVolumeController(log, dataPath, nodeId, es.EAC, state, ops)

	// The disk_volume entity uses the lsvd entity suffix as its ID
	// (matching production flow where DiskController reuses lsvd suffix)
	vol := &storage_v1alpha.DiskVolume{
		ID:           entity.Id("disk_volume/" + lsvdEntitySuffix),
		Name:         diskName,
		DiskId:       diskID,
		NodeId:       entity.Id("node/" + nodeId),
		SizeGb:       1,
		Filesystem:   "ext4",
		DesiredState: storage_v1alpha.DV_PRESENT,
		ActualState:  storage_v1alpha.DV_PENDING,
	}
	createDiskVolumeEntity(ctx, t, es, vol)

	err = vc.ReconcileWithEntities(ctx)
	require.NoError(t, err)

	// Verify disk.img was created via migration (not mock CreateDiskImage)
	assert.Empty(t, ops.createdImages, "should not use CreateDiskImage for migrated volume")

	imgPath := filepath.Join(lsvdEntityDir, "disk.img")
	imgFile, err := os.Open(imgPath)
	require.NoError(t, err)
	defer imgFile.Close()

	// Read and verify block at LBA 0
	buf := make([]byte, lsvd.BlockSize)
	_, err = imgFile.ReadAt(buf, 0)
	require.NoError(t, err)
	assert.Equal(t, byte(0x42), buf[0])
	assert.Equal(t, byte(0x42), buf[lsvd.BlockSize-1])

	// Read and verify block at LBA 5
	_, err = imgFile.ReadAt(buf, 5*int64(lsvd.BlockSize))
	require.NoError(t, err)
	assert.Equal(t, byte(0xFF), buf[0])

	// Verify zero gap at LBA 2 (should be sparse/zeros)
	_, err = imgFile.ReadAt(buf, 2*int64(lsvd.BlockSize))
	require.NoError(t, err)
	for _, b := range buf {
		assert.Equal(t, byte(0), b)
	}

	// Verify LSVD volume marked as migrated
	lsvdVolDir := filepath.Join(lsvdEntityDir, "volumes", lsvdVolumeUUID)
	_, err = os.Stat(filepath.Join(lsvdVolDir, "info.json"))
	assert.True(t, os.IsNotExist(err), "info.json should be renamed")
	_, err = os.Stat(filepath.Join(lsvdVolDir, "info.json.migrated"))
	assert.NoError(t, err, "info.json.migrated should exist")

	// Verify entity was updated to READY
	resp, err := es.EAC.Get(ctx, "disk_volume/"+lsvdEntitySuffix)
	require.NoError(t, err)
	var updated storage_v1alpha.DiskVolume
	updated.Decode(resp.Entity().Entity())
	assert.Equal(t, storage_v1alpha.DV_READY, updated.ActualState)
}

func TestDiskVolumeControllerNoLSVDVolume(t *testing.T) {
	ctx := t.Context()
	log := testutils.TestLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	ops := newMockDiskVolumeOps()

	vc := newTestDiskVolumeController(log, dataPath, nodeId, es.EAC, state, ops)

	vol := &storage_v1alpha.DiskVolume{
		ID:           "disk_volume/vol-456",
		Name:         "nonexistent-disk",
		NodeId:       entity.Id("node/" + nodeId),
		SizeGb:       1,
		Filesystem:   "ext4",
		DesiredState: storage_v1alpha.DV_PRESENT,
		ActualState:  storage_v1alpha.DV_PENDING,
	}
	createDiskVolumeEntity(ctx, t, es, vol)

	err := vc.ReconcileWithEntities(ctx)
	require.NoError(t, err)

	// Should have created the volume via normal path (mock CreateDiskImage)
	assert.Len(t, ops.createdImages, 1)

	// Verify no migrated files were created
	_, err = os.Stat(filepath.Join(dataPath, "volumes", "nonexistent-disk", "info.json.migrated"))
	assert.True(t, os.IsNotExist(err))
}

// createTestLSVDVolume creates an LSVD volume at dataPath with the given name and size,
// writes the provided blocks, and closes it. Each block entry is (lba, fill_byte).
func createTestLSVDVolume(t *testing.T, ctx context.Context, log *slog.Logger, dataPath, volumeName string, sizeBytes int64, blocks []struct {
	lba  int
	fill byte
}) {
	t.Helper()

	volDir := filepath.Join(dataPath, "volumes", volumeName)
	require.NoError(t, os.MkdirAll(volDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(volDir, "info.json"),
		[]byte(fmt.Sprintf(`{"name":%q,"size":%d}`, volumeName, sizeBytes)),
		0644,
	))

	disk, err := lsvd.NewDisk(ctx, log, dataPath, lsvd.WithVolumeName(volumeName))
	require.NoError(t, err)

	for _, b := range blocks {
		data := make(lsvd.RawBlocks, lsvd.BlockSize)
		for i := range data {
			data[i] = b.fill
		}
		require.NoError(t, disk.WriteExtent(ctx, data.MapTo(lsvd.LBA(b.lba))))
	}

	require.NoError(t, disk.Close(ctx))
}

// newMigrateTestController creates a DiskVolumeController for migration unit tests.
// It does not require an entity server.
func newMigrateTestController(t *testing.T, dataPath string) *DiskVolumeController {
	t.Helper()
	log := testutils.TestLogger(t)
	state := NewState()
	ops := newMockDiskVolumeOps()
	mntOps := newMockDiskMountOps()
	return NewDiskVolumeController(log, dataPath, "test-node", state, ops, mntOps)
}

func TestMigrateLSVDVolume(t *testing.T) {
	t.Run("no LSVD volume returns false", func(t *testing.T) {
		ctx := t.Context()
		dataPath := t.TempDir()
		vc := newMigrateTestController(t, dataPath)

		destPath := filepath.Join(dataPath, "output.img")
		migrated, err := vc.copyLSVDToImage(ctx, dataPath, "no-such-volume", "no-such-volume", destPath, 1<<30)

		require.NoError(t, err)
		assert.False(t, migrated)

		// Output file should not exist
		_, err = os.Stat(destPath)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("migrates single block", func(t *testing.T) {
		ctx := t.Context()
		log := testutils.TestLogger(t)
		dataPath := t.TempDir()

		createTestLSVDVolume(t, ctx, log, dataPath, "single-block", 1<<20, // 1MB
			[]struct {
				lba  int
				fill byte
			}{{0, 0xAB}})

		destDir := filepath.Join(dataPath, "dest")
		require.NoError(t, os.MkdirAll(destDir, 0755))
		destPath := filepath.Join(destDir, "disk.img")

		vc := newMigrateTestController(t, dataPath)
		migrated, err := vc.copyLSVDToImage(ctx, dataPath, "single-block", "single-block", destPath, 1<<20)

		require.NoError(t, err)
		assert.True(t, migrated)

		// Verify output file content
		buf := make([]byte, lsvd.BlockSize)
		f, err := os.Open(destPath)
		require.NoError(t, err)
		defer f.Close()

		_, err = f.ReadAt(buf, 0)
		require.NoError(t, err)
		for i, b := range buf {
			assert.Equal(t, byte(0xAB), b, "byte %d mismatch", i)
		}
	})

	t.Run("preserves sparse gaps as zeros", func(t *testing.T) {
		ctx := t.Context()
		log := testutils.TestLogger(t)
		dataPath := t.TempDir()

		// Write at LBA 0 and LBA 10, gap between should be zeros
		createTestLSVDVolume(t, ctx, log, dataPath, "sparse-vol", 1<<20,
			[]struct {
				lba  int
				fill byte
			}{{0, 0x11}, {10, 0x22}})

		destDir := filepath.Join(dataPath, "dest")
		require.NoError(t, os.MkdirAll(destDir, 0755))
		destPath := filepath.Join(destDir, "disk.img")

		vc := newMigrateTestController(t, dataPath)
		migrated, err := vc.copyLSVDToImage(ctx, dataPath, "sparse-vol", "sparse-vol", destPath, 1<<20)

		require.NoError(t, err)
		assert.True(t, migrated)

		f, err := os.Open(destPath)
		require.NoError(t, err)
		defer f.Close()

		buf := make([]byte, lsvd.BlockSize)

		// LBA 0: should be 0x11
		_, err = f.ReadAt(buf, 0)
		require.NoError(t, err)
		assert.Equal(t, byte(0x11), buf[0])
		assert.Equal(t, byte(0x11), buf[lsvd.BlockSize-1])

		// LBA 5: gap, should be zeros
		_, err = f.ReadAt(buf, 5*int64(lsvd.BlockSize))
		require.NoError(t, err)
		for _, b := range buf {
			assert.Equal(t, byte(0), b)
		}

		// LBA 10: should be 0x22
		_, err = f.ReadAt(buf, 10*int64(lsvd.BlockSize))
		require.NoError(t, err)
		assert.Equal(t, byte(0x22), buf[0])
		assert.Equal(t, byte(0x22), buf[lsvd.BlockSize-1])
	})

	t.Run("renames info.json to info.json.migrated", func(t *testing.T) {
		ctx := t.Context()
		log := testutils.TestLogger(t)
		dataPath := t.TempDir()

		createTestLSVDVolume(t, ctx, log, dataPath, "rename-test", 1<<20,
			[]struct {
				lba  int
				fill byte
			}{{0, 0x01}})

		destDir := filepath.Join(dataPath, "dest")
		require.NoError(t, os.MkdirAll(destDir, 0755))
		destPath := filepath.Join(destDir, "disk.img")

		vc := newMigrateTestController(t, dataPath)
		migrated, err := vc.copyLSVDToImage(ctx, dataPath, "rename-test", "rename-test", destPath, 1<<20)

		require.NoError(t, err)
		assert.True(t, migrated)

		infoPath := filepath.Join(dataPath, "volumes", "rename-test", "info.json")
		migratedPath := infoPath + ".migrated"

		_, err = os.Stat(infoPath)
		assert.True(t, os.IsNotExist(err), "info.json should no longer exist")

		_, err = os.Stat(migratedPath)
		assert.NoError(t, err, "info.json.migrated should exist")
	})

	t.Run("output file size uses LSVD size when larger than requested", func(t *testing.T) {
		ctx := t.Context()
		log := testutils.TestLogger(t)
		dataPath := t.TempDir()

		lsvdSize := int64(2 * 1024 * 1024) // 2MB LSVD volume
		createTestLSVDVolume(t, ctx, log, dataPath, "big-vol", lsvdSize,
			[]struct {
				lba  int
				fill byte
			}{{0, 0xCC}})

		destDir := filepath.Join(dataPath, "dest")
		require.NoError(t, os.MkdirAll(destDir, 0755))
		destPath := filepath.Join(destDir, "disk.img")

		vc := newMigrateTestController(t, dataPath)

		// Request a smaller size than the LSVD volume — output should use the larger LSVD size
		migrated, err := vc.copyLSVDToImage(ctx, dataPath, "big-vol", "big-vol", destPath, 1*1024*1024)

		require.NoError(t, err)
		assert.True(t, migrated)

		info, err := os.Stat(destPath)
		require.NoError(t, err)
		assert.Equal(t, lsvdSize, info.Size(), "output file should be at least LSVD volume size")
	})

	t.Run("output file size uses requested size when larger than LSVD", func(t *testing.T) {
		ctx := t.Context()
		log := testutils.TestLogger(t)
		dataPath := t.TempDir()

		lsvdSize := int64(1 * 1024 * 1024) // 1MB LSVD volume
		createTestLSVDVolume(t, ctx, log, dataPath, "small-vol", lsvdSize,
			[]struct {
				lba  int
				fill byte
			}{{0, 0xDD}})

		destDir := filepath.Join(dataPath, "dest")
		require.NoError(t, os.MkdirAll(destDir, 0755))
		destPath := filepath.Join(destDir, "disk.img")

		vc := newMigrateTestController(t, dataPath)

		requestedSize := int64(5 * 1024 * 1024)
		migrated, err := vc.copyLSVDToImage(ctx, dataPath, "small-vol", "small-vol", destPath, requestedSize)

		require.NoError(t, err)
		assert.True(t, migrated)

		info, err := os.Stat(destPath)
		require.NoError(t, err)
		assert.Equal(t, requestedSize, info.Size(), "output file should use requested size")
	})

	t.Run("migrates multiple blocks across chunk boundary", func(t *testing.T) {
		ctx := t.Context()
		log := testutils.TestLogger(t)
		dataPath := t.TempDir()

		// chunkBlocks is 1024 in the migration code, so write at LBA 1023, 1024, 1025
		// to straddle a chunk boundary
		createTestLSVDVolume(t, ctx, log, dataPath, "chunk-boundary", 8*1024*1024,
			[]struct {
				lba  int
				fill byte
			}{
				{1023, 0xAA}, // last block of first chunk
				{1024, 0xBB}, // first block of second chunk
				{1025, 0xCC}, // second block of second chunk
			})

		destDir := filepath.Join(dataPath, "dest")
		require.NoError(t, os.MkdirAll(destDir, 0755))
		destPath := filepath.Join(destDir, "disk.img")

		vc := newMigrateTestController(t, dataPath)
		migrated, err := vc.copyLSVDToImage(ctx, dataPath, "chunk-boundary", "chunk-boundary", destPath, 8*1024*1024)

		require.NoError(t, err)
		assert.True(t, migrated)

		f, err := os.Open(destPath)
		require.NoError(t, err)
		defer f.Close()

		buf := make([]byte, lsvd.BlockSize)

		for _, tc := range []struct {
			lba  int
			fill byte
		}{
			{1023, 0xAA},
			{1024, 0xBB},
			{1025, 0xCC},
		} {
			_, err = f.ReadAt(buf, int64(tc.lba)*int64(lsvd.BlockSize))
			require.NoError(t, err)
			assert.Equal(t, tc.fill, buf[0], "LBA %d first byte", tc.lba)
			assert.Equal(t, tc.fill, buf[lsvd.BlockSize-1], "LBA %d last byte", tc.lba)
		}
	})

	t.Run("empty LSVD volume produces sparse file", func(t *testing.T) {
		ctx := t.Context()
		log := testutils.TestLogger(t)
		dataPath := t.TempDir()

		// Volume with no written blocks
		createTestLSVDVolume(t, ctx, log, dataPath, "empty-vol", 1<<20, nil)

		destDir := filepath.Join(dataPath, "dest")
		require.NoError(t, os.MkdirAll(destDir, 0755))
		destPath := filepath.Join(destDir, "disk.img")

		vc := newMigrateTestController(t, dataPath)
		migrated, err := vc.copyLSVDToImage(ctx, dataPath, "empty-vol", "empty-vol", destPath, 1<<20)

		require.NoError(t, err)
		assert.True(t, migrated)

		info, err := os.Stat(destPath)
		require.NoError(t, err)
		assert.Equal(t, int64(1<<20), info.Size())

		// Read some blocks — all should be zeros
		f, err := os.Open(destPath)
		require.NoError(t, err)
		defer f.Close()

		buf := make([]byte, lsvd.BlockSize)
		_, err = f.ReadAt(buf, 0)
		require.NoError(t, err)
		for _, b := range buf {
			assert.Equal(t, byte(0), b)
		}
	})

	t.Run("overwrites at same LBA uses latest value", func(t *testing.T) {
		ctx := t.Context()
		log := testutils.TestLogger(t)
		dataPath := t.TempDir()

		// Create volume then write two different values to the same LBA
		volDir := filepath.Join(dataPath, "volumes", "overwrite-vol")
		require.NoError(t, os.MkdirAll(volDir, 0755))
		require.NoError(t, os.WriteFile(
			filepath.Join(volDir, "info.json"),
			[]byte(`{"name":"overwrite-vol","size":1048576}`),
			0644,
		))

		disk, err := lsvd.NewDisk(ctx, log, dataPath, lsvd.WithVolumeName("overwrite-vol"))
		require.NoError(t, err)

		// First write: 0xAA
		data1 := make(lsvd.RawBlocks, lsvd.BlockSize)
		for i := range data1 {
			data1[i] = 0xAA
		}
		require.NoError(t, disk.WriteExtent(ctx, data1.MapTo(0)))

		// Second write: 0xBB (overwrites)
		data2 := make(lsvd.RawBlocks, lsvd.BlockSize)
		for i := range data2 {
			data2[i] = 0xBB
		}
		require.NoError(t, disk.WriteExtent(ctx, data2.MapTo(0)))

		require.NoError(t, disk.Close(ctx))

		destDir := filepath.Join(dataPath, "dest")
		require.NoError(t, os.MkdirAll(destDir, 0755))
		destPath := filepath.Join(destDir, "disk.img")

		vc := newMigrateTestController(t, dataPath)
		migrated, err := vc.copyLSVDToImage(ctx, dataPath, "overwrite-vol", "overwrite-vol", destPath, 1<<20)

		require.NoError(t, err)
		assert.True(t, migrated)

		f, err := os.Open(destPath)
		require.NoError(t, err)
		defer f.Close()

		buf := make([]byte, lsvd.BlockSize)
		_, err = f.ReadAt(buf, 0)
		require.NoError(t, err)
		// Should see the second write
		for i, b := range buf {
			assert.Equal(t, byte(0xBB), b, "byte %d should be overwritten value", i)
		}
	})

	t.Run("multi-block contiguous write", func(t *testing.T) {
		ctx := t.Context()
		log := testutils.TestLogger(t)
		dataPath := t.TempDir()

		// Write 4 contiguous blocks at once
		volDir := filepath.Join(dataPath, "volumes", "multi-block")
		require.NoError(t, os.MkdirAll(volDir, 0755))
		require.NoError(t, os.WriteFile(
			filepath.Join(volDir, "info.json"),
			[]byte(`{"name":"multi-block","size":1048576}`),
			0644,
		))

		disk, err := lsvd.NewDisk(ctx, log, dataPath, lsvd.WithVolumeName("multi-block"))
		require.NoError(t, err)

		multiData := make(lsvd.RawBlocks, 4*lsvd.BlockSize)
		for i := 0; i < 4; i++ {
			for j := 0; j < lsvd.BlockSize; j++ {
				multiData[i*lsvd.BlockSize+j] = byte(i + 1) // 0x01, 0x02, 0x03, 0x04
			}
		}
		require.NoError(t, disk.WriteExtent(ctx, multiData.MapTo(2))) // LBAs 2-5

		require.NoError(t, disk.Close(ctx))

		destDir := filepath.Join(dataPath, "dest")
		require.NoError(t, os.MkdirAll(destDir, 0755))
		destPath := filepath.Join(destDir, "disk.img")

		vc := newMigrateTestController(t, dataPath)
		migrated, err := vc.copyLSVDToImage(ctx, dataPath, "multi-block", "multi-block", destPath, 1<<20)

		require.NoError(t, err)
		assert.True(t, migrated)

		f, err := os.Open(destPath)
		require.NoError(t, err)
		defer f.Close()

		buf := make([]byte, lsvd.BlockSize)

		// LBA 0-1: should be zeros (before the write)
		_, err = f.ReadAt(buf, 0)
		require.NoError(t, err)
		for _, b := range buf {
			assert.Equal(t, byte(0), b)
		}

		// LBAs 2-5: each should be filled with its block number
		for i := 0; i < 4; i++ {
			_, err = f.ReadAt(buf, int64(i+2)*int64(lsvd.BlockSize))
			require.NoError(t, err)
			expected := byte(i + 1)
			assert.Equal(t, expected, buf[0], "LBA %d first byte", i+2)
			assert.Equal(t, expected, buf[lsvd.BlockSize-1], "LBA %d last byte", i+2)
		}
	})

	t.Run("invalid dest path returns error", func(t *testing.T) {
		ctx := t.Context()
		log := testutils.TestLogger(t)
		dataPath := t.TempDir()

		createTestLSVDVolume(t, ctx, log, dataPath, "dest-err", 1<<20,
			[]struct {
				lba  int
				fill byte
			}{{0, 0x01}})

		vc := newMigrateTestController(t, dataPath)

		// Point to a non-existent directory
		destPath := filepath.Join(dataPath, "no-such-dir", "disk.img")
		migrated, err := vc.copyLSVDToImage(ctx, dataPath, "dest-err", "dest-err", destPath, 1<<20)

		assert.False(t, migrated)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "creating image file")
	})

	t.Run("corrupt info.json returns error from LSVD open", func(t *testing.T) {
		ctx := t.Context()
		dataPath := t.TempDir()

		// Create a volume dir with an invalid info.json
		volDir := filepath.Join(dataPath, "volumes", "corrupt-vol")
		require.NoError(t, os.MkdirAll(volDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(volDir, "info.json"), []byte("not json{{{"), 0644))

		destDir := filepath.Join(dataPath, "dest")
		require.NoError(t, os.MkdirAll(destDir, 0755))
		destPath := filepath.Join(destDir, "disk.img")

		vc := newMigrateTestController(t, dataPath)
		migrated, err := vc.copyLSVDToImage(ctx, dataPath, "corrupt-vol", "corrupt-vol", destPath, 1<<20)

		assert.False(t, migrated)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "opening LSVD volume")
	})
}

func TestDiskVolumeControllerCleanupMigratedLSVD(t *testing.T) {
	log := testutils.TestLogger(t)

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	ops := newMockDiskVolumeOps()

	// Create segments directory with fake segment files
	segmentsDir := filepath.Join(dataPath, "segments")
	require.NoError(t, os.MkdirAll(segmentsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(segmentsDir, "segment.01"), []byte("fake"), 0644))

	// Create a migrated LSVD volume directory
	oldVolDir := filepath.Join(dataPath, "volumes", "old-vol")
	require.NoError(t, os.MkdirAll(oldVolDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(oldVolDir, "info.json.migrated"), []byte("{}"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(oldVolDir, "segments"), []byte("seg-data"), 0644))

	vc := NewDiskVolumeController(log, dataPath, nodeId, state, ops, newMockDiskMountOps())

	err := vc.Init(t.Context())
	require.NoError(t, err)

	// Verify segments directory was removed
	_, err = os.Stat(segmentsDir)
	assert.True(t, os.IsNotExist(err), "segments dir should be removed")

	// Verify migrated volume artifacts were cleaned up
	_, err = os.Stat(filepath.Join(oldVolDir, "info.json.migrated"))
	assert.True(t, os.IsNotExist(err), "info.json.migrated should be removed")
	_, err = os.Stat(filepath.Join(oldVolDir, "segments"))
	assert.True(t, os.IsNotExist(err), "segments file should be removed")
}

func TestDiskVolumeControllerCleanupSkipsUnmigrated(t *testing.T) {
	log := testutils.TestLogger(t)

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	ops := newMockDiskVolumeOps()

	// Create segments directory
	segmentsDir := filepath.Join(dataPath, "segments")
	require.NoError(t, os.MkdirAll(segmentsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(segmentsDir, "segment.01"), []byte("fake"), 0644))

	// Create a migrated volume
	migratedDir := filepath.Join(dataPath, "volumes", "migrated-vol")
	require.NoError(t, os.MkdirAll(migratedDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(migratedDir, "info.json.migrated"), []byte("{}"), 0644))

	// Create an unmigrated volume (still has info.json)
	unmigratedDir := filepath.Join(dataPath, "volumes", "unmigrated-vol")
	require.NoError(t, os.MkdirAll(unmigratedDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(unmigratedDir, "info.json"), []byte("{}"), 0644))

	vc := NewDiskVolumeController(log, dataPath, nodeId, state, ops, newMockDiskMountOps())

	err := vc.Init(t.Context())
	require.NoError(t, err)

	// Verify segments directory was NOT removed (unmigrated volume still exists)
	_, err = os.Stat(segmentsDir)
	assert.NoError(t, err, "segments dir should NOT be removed")

	// Verify migrated marker was NOT removed
	_, err = os.Stat(filepath.Join(migratedDir, "info.json.migrated"))
	assert.NoError(t, err, "info.json.migrated should NOT be removed")
}

func TestDiskVolumeControllerUniversalMountAtCreation(t *testing.T) {
	ctx := t.Context()
	log := testutils.TestLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	volOps := newMockDiskVolumeOps()
	mntOps := newMockDiskMountOps()

	vc := NewDiskVolumeController(log, dataPath, nodeId, state, volOps, mntOps)
	vc.SetEAC(es.EAC)

	vol := &storage_v1alpha.DiskVolume{
		ID:           "disk_volume/vol-uni",
		NodeId:       entity.Id("node/" + nodeId),
		SizeGb:       10,
		Filesystem:   "ext4",
		VolumeMode:   storage_v1alpha.VM_UNIVERSAL,
		DesiredState: storage_v1alpha.DV_PRESENT,
		ActualState:  storage_v1alpha.DV_PENDING,
	}
	createDiskVolumeEntity(ctx, t, es, vol)

	err := vc.ReconcileWithEntities(ctx)
	require.NoError(t, err)

	// Verify volume was created
	assert.Len(t, volOps.createdDirs, 1)
	assert.Len(t, volOps.createdImages, 1)

	// Verify loop attach was called (alwaysMount)
	assert.Len(t, mntOps.attachedLoops, 1)

	// Verify format was called
	assert.Len(t, mntOps.formatCalls, 1)
	assert.Equal(t, "ext4", mntOps.formatCalls[0].filesystem)

	// Verify mount was called
	assert.Len(t, mntOps.mounts, 1)

	// Verify state reflects mount
	volState := state.GetVolume("disk_volume/vol-uni")
	require.NotNil(t, volState)
	assert.True(t, volState.Mounted)
	assert.Equal(t, "/dev/loop0", volState.DevicePath)
	assert.NotEmpty(t, volState.MountPath)
	assert.Equal(t, storage_v1alpha.VM_UNIVERSAL, volState.Mode)
}

func TestDiskVolumeControllerUniversalRemountOnReconcile(t *testing.T) {
	ctx := t.Context()
	log := testutils.TestLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	volOps := newMockDiskVolumeOps()
	mntOps := newMockDiskMountOps()

	volPath := filepath.Join(dataPath, "volumes", "vol-remount")
	mountPath := filepath.Join(dataPath, "vol-remount")

	// Pre-populate state: volume is ready but NOT mounted (simulating restart)
	state.SetVolume("disk_volume/vol-remount", &VolumeState{
		EntityId:   "disk_volume/vol-remount",
		VolumeId:   "vol-remount",
		DiskPath:   volPath,
		SizeBytes:  10 * 1024 * 1024 * 1024,
		Filesystem: "ext4",
		Mode:       storage_v1alpha.VM_UNIVERSAL,
		Mounted:    false,
		MountPath:  mountPath,
	})
	volOps.existingPaths[volPath] = true

	vc := NewDiskVolumeController(log, dataPath, nodeId, state, volOps, mntOps)
	vc.SetEAC(es.EAC)

	vol := &storage_v1alpha.DiskVolume{
		ID:           "disk_volume/vol-remount",
		NodeId:       entity.Id("node/" + nodeId),
		SizeGb:       10,
		Filesystem:   "ext4",
		VolumeMode:   storage_v1alpha.VM_UNIVERSAL,
		DesiredState: storage_v1alpha.DV_PRESENT,
		ActualState:  storage_v1alpha.DV_READY,
	}
	createDiskVolumeEntity(ctx, t, es, vol)

	err := vc.ReconcileWithEntities(ctx)
	require.NoError(t, err)

	// Verify loop attach was called for remount
	assert.Len(t, mntOps.attachedLoops, 1)
	assert.Len(t, mntOps.mounts, 1)

	// Verify state reflects mount
	volState := state.GetVolume("disk_volume/vol-remount")
	require.NotNil(t, volState)
	assert.True(t, volState.Mounted)
}

func TestDiskVolumeControllerUniversalUnmountOnDelete(t *testing.T) {
	ctx := t.Context()
	log := testutils.TestLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	volOps := newMockDiskVolumeOps()
	mntOps := newMockDiskMountOps()

	mountPath := filepath.Join(dataPath, "vol-del")
	state.SetVolume("disk_volume/vol-del", &VolumeState{
		EntityId:   "disk_volume/vol-del",
		VolumeId:   "vol-del",
		DiskPath:   "/data/volumes/vol-del",
		SizeBytes:  10 * 1024 * 1024 * 1024,
		Filesystem: "ext4",
		Mode:       storage_v1alpha.VM_UNIVERSAL,
		DevicePath: "/dev/loop3",
		MountPath:  mountPath,
		Mounted:    true,
	})
	volOps.existingPaths["/data/volumes/vol-del"] = true
	mntOps.mountedPaths[mountPath] = true

	vc := NewDiskVolumeController(log, dataPath, nodeId, state, volOps, mntOps)
	vc.SetEAC(es.EAC)

	vol := &storage_v1alpha.DiskVolume{
		ID:           "disk_volume/vol-del",
		NodeId:       entity.Id("node/" + nodeId),
		SizeGb:       10,
		Filesystem:   "ext4",
		VolumeMode:   storage_v1alpha.VM_UNIVERSAL,
		DesiredState: storage_v1alpha.DV_ABSENT,
		ActualState:  storage_v1alpha.DV_READY,
	}
	createDiskVolumeEntity(ctx, t, es, vol)

	err := vc.ReconcileWithEntities(ctx)
	require.NoError(t, err)

	// Verify unmount was called
	assert.Contains(t, mntOps.unmounts, mountPath)
	// Verify loop detach was called
	assert.Contains(t, mntOps.detachedLoops, "/dev/loop3")
	// Verify volume directory was removed
	assert.Contains(t, volOps.removedDirs, "/data/volumes/vol-del")
}

func TestDiskVolumeControllerShutdown(t *testing.T) {
	log := testutils.TestLogger(t)

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	volOps := newMockDiskVolumeOps()
	mntOps := newMockDiskMountOps()

	// Mount paths under diskMountBasePath (what the real system uses)
	mountPath1 := diskMountBasePath + "/vol-a"
	mountPath2 := diskMountBasePath + "/vol-b"
	accMountPath := diskMountBasePath + "/vol-acc"

	state.SetVolume("disk_volume/vol-a", &VolumeState{
		EntityId:   "disk_volume/vol-a",
		VolumeId:   "vol-a",
		DiskPath:   "/data/volumes/vol-a",
		Mode:       storage_v1alpha.VM_UNIVERSAL,
		DevicePath: "/dev/loop1",
		MountPath:  mountPath1,
		Mounted:    true,
	})
	state.SetVolume("disk_volume/vol-b", &VolumeState{
		EntityId:   "disk_volume/vol-b",
		VolumeId:   "vol-b",
		DiskPath:   "/data/volumes/vol-b",
		Mode:       storage_v1alpha.VM_UNIVERSAL,
		DevicePath: "/dev/loop2",
		MountPath:  mountPath2,
		Mounted:    true,
	})

	// Simulate kernel mount table: these are the actual mounts the kernel reports
	mntOps.mountedPaths[mountPath1] = true
	mntOps.mountDevices[mountPath1] = "/dev/loop1"
	mntOps.mountedPaths[mountPath2] = true
	mntOps.mountDevices[mountPath2] = "/dev/loop2"
	mntOps.mountedPaths[accMountPath] = true
	mntOps.mountDevices[accMountPath] = "/dev/lbd0"

	vc := NewDiskVolumeController(log, dataPath, nodeId, state, volOps, mntOps)

	vc.Shutdown()

	// All three mounts should be unmounted (found via kernel mount table scan)
	assert.Len(t, mntOps.unmounts, 3)
	assert.Contains(t, mntOps.unmounts, mountPath1)
	assert.Contains(t, mntOps.unmounts, mountPath2)
	assert.Contains(t, mntOps.unmounts, accMountPath)

	// Loop devices detached for universal volumes
	assert.Contains(t, mntOps.detachedLoops, "/dev/loop1")
	assert.Contains(t, mntOps.detachedLoops, "/dev/loop2")

	// LBD device detached for accelerator volume
	assert.Contains(t, mntOps.detachedLbds, "/dev/lbd0")

	// Verify volume state reflects unmounted
	volA := state.GetVolume("disk_volume/vol-a")
	require.NotNil(t, volA)
	assert.False(t, volA.Mounted)
}

func TestDiskVolumeControllerAcceleratorNoMountAtCreation(t *testing.T) {
	ctx := t.Context()
	log := testutils.TestLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	volOps := newMockDiskVolumeOps()
	mntOps := newMockDiskMountOps()

	vc := NewDiskVolumeController(log, dataPath, nodeId, state, volOps, mntOps)
	vc.SetEAC(es.EAC)

	vol := &storage_v1alpha.DiskVolume{
		ID:           "disk_volume/vol-acc",
		NodeId:       entity.Id("node/" + nodeId),
		SizeGb:       10,
		Filesystem:   "ext4",
		VolumeMode:   storage_v1alpha.VM_ACCELERATOR,
		DesiredState: storage_v1alpha.DV_PRESENT,
		ActualState:  storage_v1alpha.DV_PENDING,
	}
	createDiskVolumeEntity(ctx, t, es, vol)

	err := vc.ReconcileWithEntities(ctx)
	require.NoError(t, err)

	// Volume should be created but NOT mounted
	assert.Len(t, volOps.createdDirs, 2) // volume dir + log dir
	assert.Len(t, volOps.createdImages, 1)
	assert.Empty(t, mntOps.attachedLoops, "accelerator mode should not mount at creation")
	assert.Empty(t, mntOps.mounts, "accelerator mode should not mount at creation")

	volState := state.GetVolume("disk_volume/vol-acc")
	require.NotNil(t, volState)
	assert.False(t, volState.Mounted)
}
