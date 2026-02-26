package diskio

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"miren.dev/runtime/api/entityserver/entityserver_v1alpha"
	"miren.dev/runtime/api/storage/storage_v1alpha"
	"miren.dev/runtime/pkg/entity"
	"miren.dev/runtime/pkg/entity/testutils"
)

func newTestDiskMountController(log *slog.Logger, dataPath, nodeId string, eac *entityserver_v1alpha.EntityAccessClient, state *State, ops DiskMountOps) *DiskMountController {
	mc := NewDiskMountController(log, dataPath, nodeId, state, ops)
	mc.SetEAC(eac)
	return mc
}

func createDiskMountEntity(ctx context.Context, t *testing.T, es *testutils.InMemEntityServer, mount *storage_v1alpha.DiskMount) {
	_, err := es.EAC.Create(ctx, entity.New(
		entity.DBId, mount.ID,
		mount.Encode,
	).Attrs())
	require.NoError(t, err)
}

func TestDiskMountControllerReconcileMountMounted(t *testing.T) {
	ctx := t.Context()
	log := testutils.TestLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	ops := newMockDiskMountOps()

	// Pre-populate volume state (mount requires an existing volume)
	state.SetVolume("disk_volume/vol-123", &VolumeState{
		EntityId:   "disk_volume/vol-123",
		VolumeId:   "vol-123",
		DiskPath:   "/data/volumes/vol-123",
		SizeBytes:  10 * 1024 * 1024 * 1024,
		Filesystem: "ext4",
	})

	mc := newTestDiskMountController(log, dataPath, nodeId, es.EAC, state, ops)

	mount := &storage_v1alpha.DiskMount{
		ID:           "disk_mount/mnt-123",
		NodeId:       entity.Id("node/" + nodeId),
		VolumeId:     "disk_volume/vol-123",
		MountPath:    "/mnt/data",
		ReadOnly:     false,
		DesiredState: storage_v1alpha.DM_WANT_MOUNTED,
		ActualState:  storage_v1alpha.DM_PENDING,
	}
	createDiskMountEntity(ctx, t, es, mount)

	err := mc.ReconcileWithEntities(ctx)
	require.NoError(t, err)

	// Verify loop device was attached
	assert.Len(t, ops.attachedLoops, 1)
	assert.Equal(t, "/data/volumes/vol-123/disk.img", ops.attachedLoops[0])

	// Verify filesystem was formatted
	assert.Len(t, ops.formatCalls, 1)
	assert.Equal(t, "/dev/loop0", ops.formatCalls[0].device)
	assert.Equal(t, "ext4", ops.formatCalls[0].filesystem)

	// Verify mount was performed
	assert.Len(t, ops.mounts, 1)
	assert.Equal(t, "/dev/loop0", ops.mounts[0].device)
	assert.Equal(t, "/mnt/data", ops.mounts[0].mountPath)
	assert.Equal(t, "ext4", ops.mounts[0].filesystem)
	assert.False(t, ops.mounts[0].readOnly)

	// Verify state was updated
	mountState := state.GetMount("disk_mount/mnt-123")
	require.NotNil(t, mountState)
	assert.True(t, mountState.Mounted)
	assert.Equal(t, "/mnt/data", mountState.MountPath)
	assert.Equal(t, "/dev/loop0", mountState.DevicePath)

	// Verify entity was updated to MOUNTED
	resp, err := es.EAC.Get(ctx, "disk_mount/mnt-123")
	require.NoError(t, err)
	var updated storage_v1alpha.DiskMount
	updated.Decode(resp.Entity().Entity())
	assert.Equal(t, storage_v1alpha.DM_MOUNTED, updated.ActualState)
	assert.Equal(t, "/dev/loop0", updated.DevicePath)
	assert.Equal(t, "/dev/loop0", updated.LoopDevice)
}

func TestDiskMountControllerReconcileMountUnmounted(t *testing.T) {
	ctx := t.Context()
	log := testutils.TestLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	ops := newMockDiskMountOps()

	// Pre-populate state with existing mount
	state.SetMount("disk_mount/mnt-456", &MountState{
		EntityId:   "disk_mount/mnt-456",
		VolumeId:   "disk_volume/vol-456",
		DevicePath: "/dev/loop5",
		MountPath:  "/mnt/data",
		Mounted:    true,
		ReadOnly:   false,
	})
	ops.mountedPaths["/mnt/data"] = true

	mc := newTestDiskMountController(log, dataPath, nodeId, es.EAC, state, ops)

	mount := &storage_v1alpha.DiskMount{
		ID:           "disk_mount/mnt-456",
		NodeId:       entity.Id("node/" + nodeId),
		VolumeId:     "disk_volume/vol-456",
		MountPath:    "/mnt/data",
		DesiredState: storage_v1alpha.DM_WANT_UNMOUNTED,
		ActualState:  storage_v1alpha.DM_MOUNTED,
	}
	createDiskMountEntity(ctx, t, es, mount)

	err := mc.ReconcileWithEntities(ctx)
	require.NoError(t, err)

	// Verify unmount was called
	assert.Len(t, ops.unmounts, 1)
	assert.Equal(t, "/mnt/data", ops.unmounts[0])

	// Verify loop device was detached
	assert.Len(t, ops.detachedLoops, 1)
	assert.Equal(t, "/dev/loop5", ops.detachedLoops[0])

	// Verify state was cleaned up
	assert.Nil(t, state.GetMount("disk_mount/mnt-456"))

	// Verify entity was updated to DETACHED
	resp, err := es.EAC.Get(ctx, "disk_mount/mnt-456")
	require.NoError(t, err)
	var updated storage_v1alpha.DiskMount
	updated.Decode(resp.Entity().Entity())
	assert.Equal(t, storage_v1alpha.DM_DETACHED, updated.ActualState)
}

func TestDiskMountControllerReconcileSkipsOtherNodes(t *testing.T) {
	ctx := t.Context()
	log := testutils.TestLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	ops := newMockDiskMountOps()

	mc := newTestDiskMountController(log, dataPath, nodeId, es.EAC, state, ops)

	mount := &storage_v1alpha.DiskMount{
		ID:           "disk_mount/mnt-other",
		NodeId:       entity.Id("node/other-node"),
		VolumeId:     "disk_volume/vol-other",
		MountPath:    "/mnt/other",
		DesiredState: storage_v1alpha.DM_WANT_MOUNTED,
		ActualState:  storage_v1alpha.DM_PENDING,
	}
	createDiskMountEntity(ctx, t, es, mount)

	err := mc.ReconcileWithEntities(ctx)
	require.NoError(t, err)

	assert.Empty(t, ops.attachedLoops)
	assert.Empty(t, ops.mounts)
}

func TestDiskMountControllerReconcileMountAlreadyMounted(t *testing.T) {
	ctx := t.Context()
	log := testutils.TestLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	ops := newMockDiskMountOps()

	// Pre-populate state with existing mounted volume
	state.SetMount("disk_mount/mnt-ready", &MountState{
		EntityId:   "disk_mount/mnt-ready",
		VolumeId:   "disk_volume/vol-ready",
		DevicePath: "/dev/loop3",
		MountPath:  "/mnt/ready",
		Mounted:    true,
	})
	ops.mountedPaths["/mnt/ready"] = true

	mc := newTestDiskMountController(log, dataPath, nodeId, es.EAC, state, ops)

	mount := &storage_v1alpha.DiskMount{
		ID:           "disk_mount/mnt-ready",
		NodeId:       entity.Id("node/" + nodeId),
		VolumeId:     "disk_volume/vol-ready",
		MountPath:    "/mnt/ready",
		DesiredState: storage_v1alpha.DM_WANT_MOUNTED,
		ActualState:  storage_v1alpha.DM_MOUNTED,
	}
	createDiskMountEntity(ctx, t, es, mount)

	err := mc.ReconcileWithEntities(ctx)
	require.NoError(t, err)

	// No new operations should be performed
	assert.Empty(t, ops.attachedLoops)
	assert.Empty(t, ops.mounts)
	assert.Empty(t, ops.formatCalls)
}

func TestDiskMountControllerReconcileVolumeNotFound(t *testing.T) {
	ctx := t.Context()
	log := testutils.TestLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	ops := newMockDiskMountOps()

	mc := newTestDiskMountController(log, dataPath, nodeId, es.EAC, state, ops)

	mount := &storage_v1alpha.DiskMount{
		ID:           "disk_mount/mnt-missing",
		NodeId:       entity.Id("node/" + nodeId),
		VolumeId:     "disk_volume/vol-missing",
		MountPath:    "/mnt/missing",
		DesiredState: storage_v1alpha.DM_WANT_MOUNTED,
		ActualState:  storage_v1alpha.DM_PENDING,
	}
	createDiskMountEntity(ctx, t, es, mount)

	err := mc.ReconcileWithEntities(ctx)
	require.NoError(t, err) // ReconcileWithEntities logs errors but doesn't return them

	// Verify entity was updated to error state
	resp, err := es.EAC.Get(ctx, "disk_mount/mnt-missing")
	require.NoError(t, err)
	var updated storage_v1alpha.DiskMount
	updated.Decode(resp.Entity().Entity())
	assert.Equal(t, storage_v1alpha.DM_ERROR, updated.ActualState)
	assert.Contains(t, updated.ErrorMessage, "not found")
}

func TestDiskMountControllerReconcileMountReadOnly(t *testing.T) {
	ctx := t.Context()
	log := testutils.TestLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	ops := newMockDiskMountOps()

	state.SetVolume("disk_volume/vol-ro", &VolumeState{
		EntityId:   "disk_volume/vol-ro",
		VolumeId:   "vol-ro",
		DiskPath:   "/data/volumes/vol-ro",
		SizeBytes:  5 * 1024 * 1024 * 1024,
		Filesystem: "xfs",
	})

	// Pre-format the device to skip formatting
	ops.formattedDevs["/dev/loop0"] = "xfs"

	mc := newTestDiskMountController(log, dataPath, nodeId, es.EAC, state, ops)

	mount := &storage_v1alpha.DiskMount{
		ID:           "disk_mount/mnt-ro",
		NodeId:       entity.Id("node/" + nodeId),
		VolumeId:     "disk_volume/vol-ro",
		MountPath:    "/mnt/readonly",
		ReadOnly:     true,
		DesiredState: storage_v1alpha.DM_WANT_MOUNTED,
		ActualState:  storage_v1alpha.DM_PENDING,
	}
	createDiskMountEntity(ctx, t, es, mount)

	err := mc.ReconcileWithEntities(ctx)
	require.NoError(t, err)

	// Verify mount was called with readOnly flag
	assert.Len(t, ops.mounts, 1)
	assert.True(t, ops.mounts[0].readOnly)
	assert.Equal(t, "xfs", ops.mounts[0].filesystem)

	// Verify no formatting occurred (already formatted)
	assert.Empty(t, ops.formatCalls)

	// Verify state reflects read-only
	mountState := state.GetMount("disk_mount/mnt-ro")
	require.NotNil(t, mountState)
	assert.True(t, mountState.ReadOnly)
}

func TestDiskMountControllerReconcileAlreadyDetached(t *testing.T) {
	ctx := t.Context()
	log := testutils.TestLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	ops := newMockDiskMountOps()

	mc := newTestDiskMountController(log, dataPath, nodeId, es.EAC, state, ops)

	mount := &storage_v1alpha.DiskMount{
		ID:           "disk_mount/mnt-detached",
		NodeId:       entity.Id("node/" + nodeId),
		VolumeId:     "disk_volume/vol-detached",
		MountPath:    "/mnt/detached",
		DesiredState: storage_v1alpha.DM_WANT_UNMOUNTED,
		ActualState:  storage_v1alpha.DM_DETACHED,
	}
	createDiskMountEntity(ctx, t, es, mount)

	err := mc.ReconcileWithEntities(ctx)
	require.NoError(t, err)

	assert.Empty(t, ops.unmounts)
	assert.Empty(t, ops.detachedLoops)
}

func TestDiskMountControllerReconcileErrorRecovery(t *testing.T) {
	ctx := t.Context()
	log := testutils.TestLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	ops := newMockDiskMountOps()

	state.SetVolume("disk_volume/vol-err", &VolumeState{
		EntityId:   "disk_volume/vol-err",
		VolumeId:   "vol-err",
		DiskPath:   "/data/volumes/vol-err",
		SizeBytes:  10 * 1024 * 1024 * 1024,
		Filesystem: "ext4",
	})

	mc := newTestDiskMountController(log, dataPath, nodeId, es.EAC, state, ops)

	mount := &storage_v1alpha.DiskMount{
		ID:           "disk_mount/mnt-err",
		NodeId:       entity.Id("node/" + nodeId),
		VolumeId:     "disk_volume/vol-err",
		MountPath:    "/mnt/recover",
		DesiredState: storage_v1alpha.DM_WANT_MOUNTED,
		ActualState:  storage_v1alpha.DM_ERROR,
	}
	createDiskMountEntity(ctx, t, es, mount)

	err := mc.ReconcileWithEntities(ctx)
	require.NoError(t, err)

	// Should have attempted recovery (attach + mount)
	assert.Len(t, ops.attachedLoops, 1)
	assert.Len(t, ops.mounts, 1)
}

func TestDiskMountControllerMultipleMounts(t *testing.T) {
	ctx := t.Context()
	log := testutils.TestLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	ops := newMockDiskMountOps()

	for i := 1; i <= 3; i++ {
		volId := entity.Id("disk_volume/vol-" + string(rune('0'+i)))
		state.SetVolume(string(volId), &VolumeState{
			EntityId:   string(volId),
			VolumeId:   "vol-" + string(rune('0'+i)),
			DiskPath:   "/data/volumes/vol-" + string(rune('0'+i)),
			SizeBytes:  int64(i * 10 * 1024 * 1024 * 1024),
			Filesystem: "ext4",
		})
	}

	mc := newTestDiskMountController(log, dataPath, nodeId, es.EAC, state, ops)

	for i := 1; i <= 3; i++ {
		mount := &storage_v1alpha.DiskMount{
			ID:           entity.Id("disk_mount/mnt-" + string(rune('0'+i))),
			NodeId:       entity.Id("node/" + nodeId),
			VolumeId:     entity.Id("disk_volume/vol-" + string(rune('0'+i))),
			MountPath:    "/mnt/data" + string(rune('0'+i)),
			DesiredState: storage_v1alpha.DM_WANT_MOUNTED,
			ActualState:  storage_v1alpha.DM_PENDING,
		}
		createDiskMountEntity(ctx, t, es, mount)
	}

	err := mc.ReconcileWithEntities(ctx)
	require.NoError(t, err)

	assert.Len(t, ops.attachedLoops, 3)
	assert.Len(t, ops.mounts, 3)
	assert.Len(t, state.Mounts, 3)
}

func TestDiskMountControllerReconcileCleansUpOrphanedMounts(t *testing.T) {
	ctx := t.Context()
	log := testutils.TestLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	ops := newMockDiskMountOps()

	state.SetMount("disk_mount/mnt-orphan", &MountState{
		EntityId:   "disk_mount/mnt-orphan",
		VolumeId:   "disk_volume/vol-orphan",
		DevicePath: "/dev/loop7",
		MountPath:  "/mnt/orphan",
		Mounted:    true,
		ReadOnly:   false,
	})
	ops.mountedPaths["/mnt/orphan"] = true

	mc := newTestDiskMountController(log, dataPath, nodeId, es.EAC, state, ops)

	err := mc.ReconcileWithEntities(ctx)
	require.NoError(t, err)

	assert.Contains(t, ops.unmounts, "/mnt/orphan")
	assert.Contains(t, ops.detachedLoops, "/dev/loop7")
	assert.Nil(t, state.GetMount("disk_mount/mnt-orphan"))
}

func TestDiskMountControllerReconcileKeepsNonOrphanedMounts(t *testing.T) {
	ctx := t.Context()
	log := testutils.TestLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	ops := newMockDiskMountOps()

	state.SetVolume("disk_volume/vol-keep", &VolumeState{
		EntityId:   "disk_volume/vol-keep",
		VolumeId:   "vol-keep",
		DiskPath:   "/data/volumes/vol-keep",
		SizeBytes:  10 * 1024 * 1024 * 1024,
		Filesystem: "ext4",
	})

	state.SetMount("disk_mount/mnt-keep", &MountState{
		EntityId:   "disk_mount/mnt-keep",
		VolumeId:   "disk_volume/vol-keep",
		DevicePath: "/dev/loop8",
		MountPath:  "/mnt/keep",
		Mounted:    true,
		ReadOnly:   false,
	})
	ops.mountedPaths["/mnt/keep"] = true

	state.SetMount("disk_mount/mnt-orphan2", &MountState{
		EntityId:   "disk_mount/mnt-orphan2",
		VolumeId:   "disk_volume/vol-orphan2",
		DevicePath: "/dev/loop9",
		MountPath:  "/mnt/orphan2",
		Mounted:    true,
		ReadOnly:   false,
	})
	ops.mountedPaths["/mnt/orphan2"] = true

	mc := newTestDiskMountController(log, dataPath, nodeId, es.EAC, state, ops)

	mount := &storage_v1alpha.DiskMount{
		ID:           "disk_mount/mnt-keep",
		NodeId:       entity.Id("node/" + nodeId),
		VolumeId:     "disk_volume/vol-keep",
		MountPath:    "/mnt/keep",
		DesiredState: storage_v1alpha.DM_WANT_MOUNTED,
		ActualState:  storage_v1alpha.DM_MOUNTED,
	}
	createDiskMountEntity(ctx, t, es, mount)

	err := mc.ReconcileWithEntities(ctx)
	require.NoError(t, err)

	assert.NotNil(t, state.GetMount("disk_mount/mnt-keep"))
	assert.Nil(t, state.GetMount("disk_mount/mnt-orphan2"))
	assert.Contains(t, ops.unmounts, "/mnt/orphan2")
	assert.NotContains(t, ops.unmounts, "/mnt/keep")
}

func TestDiskMountControllerShutdown(t *testing.T) {
	log := testutils.TestLogger(t)

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	ops := newMockDiskMountOps()

	mc := NewDiskMountController(log, dataPath, nodeId, state, ops)

	// Track active mounts manually
	ops.mountedPaths["/mnt/a"] = true
	ops.mountedPaths["/mnt/b"] = true

	mc.mu.Lock()
	mc.mounts["disk_mount/mnt-a"] = &diskMountInfo{
		imagePath:  "/data/volumes/vol-a/disk.img",
		devicePath: "/dev/loop1",
		mountPath:  "/mnt/a",
	}
	mc.mounts["disk_mount/mnt-b"] = &diskMountInfo{
		imagePath:  "/data/volumes/vol-b/disk.img",
		devicePath: "/dev/loop2",
		mountPath:  "/mnt/b",
	}
	mc.mu.Unlock()

	mc.Shutdown()

	// Verify all mounts were unmounted
	assert.Len(t, ops.unmounts, 2)
	assert.Contains(t, ops.unmounts, "/mnt/a")
	assert.Contains(t, ops.unmounts, "/mnt/b")

	// Verify all loop devices were detached
	assert.Len(t, ops.detachedLoops, 2)
	assert.Contains(t, ops.detachedLoops, "/dev/loop1")
	assert.Contains(t, ops.detachedLoops, "/dev/loop2")

	// Verify mounts map is cleared
	mc.mu.RLock()
	assert.Empty(t, mc.mounts)
	mc.mu.RUnlock()
}

func TestDiskMountControllerUnmountNotInState(t *testing.T) {
	ctx := t.Context()
	log := testutils.TestLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	ops := newMockDiskMountOps()

	mc := newTestDiskMountController(log, dataPath, nodeId, es.EAC, state, ops)

	// Mount not in local state but entity requests unmount
	mount := &storage_v1alpha.DiskMount{
		ID:           "disk_mount/mnt-gone",
		NodeId:       entity.Id("node/" + nodeId),
		VolumeId:     "disk_volume/vol-gone",
		MountPath:    "/mnt/gone",
		DesiredState: storage_v1alpha.DM_WANT_UNMOUNTED,
		ActualState:  storage_v1alpha.DM_MOUNTED,
	}
	createDiskMountEntity(ctx, t, es, mount)

	err := mc.ReconcileWithEntities(ctx)
	require.NoError(t, err)

	// Should still transition to DETACHED even without local state
	resp, err := es.EAC.Get(ctx, "disk_mount/mnt-gone")
	require.NoError(t, err)
	var updated storage_v1alpha.DiskMount
	updated.Decode(resp.Entity().Entity())
	assert.Equal(t, storage_v1alpha.DM_DETACHED, updated.ActualState)
}
