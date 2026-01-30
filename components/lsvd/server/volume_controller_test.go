package server

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
	"miren.dev/runtime/pkg/units"
)

// mockVolumeOps implements VolumeOps for testing
type mockVolumeOps struct {
	createdDirs   []string
	removedDirs   []string
	existingPaths map[string]bool
	initedVolumes []mockInitVolume

	// Error injection
	createDirErr  error
	removeDirErr  error
	initVolumeErr error
}

type mockInitVolume struct {
	path       string
	volumeId   string
	size       units.Bytes
	metadata   map[string]any
	remoteOnly bool
}

func newMockVolumeOps() *mockVolumeOps {
	return &mockVolumeOps{
		existingPaths: make(map[string]bool),
	}
}

func (m *mockVolumeOps) CreateVolumeDir(path string) error {
	if m.createDirErr != nil {
		return m.createDirErr
	}
	m.createdDirs = append(m.createdDirs, path)
	m.existingPaths[path] = true
	return nil
}

func (m *mockVolumeOps) RemoveVolumeDir(path string) error {
	if m.removeDirErr != nil {
		return m.removeDirErr
	}
	m.removedDirs = append(m.removedDirs, path)
	delete(m.existingPaths, path)
	return nil
}

func (m *mockVolumeOps) VolumePathExists(path string) bool {
	return m.existingPaths[path]
}

func (m *mockVolumeOps) InitLSVDVolume(ctx context.Context, path, volumeId string, size units.Bytes, metadata map[string]any, remoteOnly bool) (string, error) {
	if m.initVolumeErr != nil {
		return "", m.initVolumeErr
	}
	m.initedVolumes = append(m.initedVolumes, mockInitVolume{
		path:       path,
		volumeId:   volumeId,
		size:       size,
		metadata:   metadata,
		remoteOnly: remoteOnly,
	})
	return volumeId, nil
}

// newTestVolumeController creates a VolumeController with EAC pre-set for testing
func newTestVolumeController(log *slog.Logger, dataPath, nodeId string, eac *entityserver_v1alpha.EntityAccessClient, state *State, ops VolumeOps) *VolumeController {
	vc := NewVolumeController(log, dataPath, nodeId, state, ops)
	vc.SetEAC(eac)
	return vc
}

// createLsvdVolumeEntity creates an lsvd_volume entity in the test entity server
func createLsvdVolumeEntity(ctx context.Context, t *testing.T, es *testutils.InMemEntityServer, vol *storage_v1alpha.LsvdVolume) {
	_, err := es.EAC.Create(ctx, entity.New(
		entity.DBId, vol.ID,
		vol.Encode,
	).Attrs())
	require.NoError(t, err)
}

func TestVolumeControllerReconcileVolumePresent(t *testing.T) {
	ctx := t.Context()
	log := testutils.TestLogger(t)

	// Create in-memory entity server
	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	// Create test infrastructure
	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	ops := newMockVolumeOps()

	// Create the volume controller
	vc := newTestVolumeController(log, dataPath, nodeId, es.EAC, state, ops)

	// Create a volume entity in PENDING state
	vol := &storage_v1alpha.LsvdVolume{
		ID:           "lsvd_volume/vol-123",
		NodeId:       entity.Id("node/" + nodeId),
		SizeGb:       10,
		Filesystem:   "ext4",
		RemoteOnly:   false,
		DesiredState: storage_v1alpha.VOL_PRESENT,
		ActualState:  storage_v1alpha.VOL_PENDING,
	}
	createLsvdVolumeEntity(ctx, t, es, vol)

	// Run reconciliation
	err := vc.ReconcileWithEntities(ctx)
	require.NoError(t, err)

	// Verify volume was created
	assert.Len(t, ops.createdDirs, 1)
	assert.Len(t, ops.initedVolumes, 1)
	assert.Equal(t, units.GigaBytes(10).Bytes(), ops.initedVolumes[0].size)
	assert.Equal(t, map[string]any{"filesystem": "ext4"}, ops.initedVolumes[0].metadata)

	// Verify state was updated
	volState := state.GetVolume("lsvd_volume/vol-123")
	require.NotNil(t, volState)
	assert.Equal(t, "lsvd_volume/vol-123", volState.EntityId)
	assert.Equal(t, int64(10*1024*1024*1024), volState.SizeBytes)
	assert.Equal(t, "ext4", volState.Filesystem)
	assert.False(t, volState.RemoteOnly)

	// Verify entity was updated
	resp, err := es.EAC.Get(ctx, "lsvd_volume/vol-123")
	require.NoError(t, err)
	var updated storage_v1alpha.LsvdVolume
	updated.Decode(resp.Entity().Entity())
	assert.Equal(t, storage_v1alpha.VOL_READY, updated.ActualState)
	assert.NotEmpty(t, updated.VolumeId)
}

func TestVolumeControllerReconcileVolumeAbsent(t *testing.T) {
	ctx := t.Context()
	log := testutils.TestLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	ops := newMockVolumeOps()

	// Pre-populate state with existing volume
	state.SetVolume("lsvd_volume/vol-456", &VolumeState{
		EntityId:   "lsvd_volume/vol-456",
		VolumeId:   "actual-vol-id",
		DiskPath:   "/data/volumes/actual-vol-id",
		SizeBytes:  5 * 1024 * 1024 * 1024,
		Filesystem: "xfs",
	})
	ops.existingPaths["/data/volumes/actual-vol-id"] = true

	vc := newTestVolumeController(log, dataPath, nodeId, es.EAC, state, ops)

	// Create volume entity requesting deletion
	vol := &storage_v1alpha.LsvdVolume{
		ID:           "lsvd_volume/vol-456",
		NodeId:       entity.Id("node/" + nodeId),
		SizeGb:       5,
		Filesystem:   "xfs",
		DesiredState: storage_v1alpha.VOL_ABSENT,
		ActualState:  storage_v1alpha.VOL_READY,
	}
	createLsvdVolumeEntity(ctx, t, es, vol)

	// Run reconciliation
	err := vc.ReconcileWithEntities(ctx)
	require.NoError(t, err)

	// Verify volume was deleted
	assert.Len(t, ops.removedDirs, 1)
	assert.Equal(t, "/data/volumes/actual-vol-id", ops.removedDirs[0])

	// Verify state was updated
	assert.Nil(t, state.GetVolume("lsvd_volume/vol-456"))

	// Verify entity was updated
	resp, err := es.EAC.Get(ctx, "lsvd_volume/vol-456")
	require.NoError(t, err)
	var updated storage_v1alpha.LsvdVolume
	updated.Decode(resp.Entity().Entity())
	assert.Equal(t, storage_v1alpha.VOL_DELETED, updated.ActualState)
}

func TestVolumeControllerReconcileSkipsOtherNodes(t *testing.T) {
	ctx := t.Context()
	log := testutils.TestLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	ops := newMockVolumeOps()

	vc := newTestVolumeController(log, dataPath, nodeId, es.EAC, state, ops)

	// Create volume entity for a different node
	vol := &storage_v1alpha.LsvdVolume{
		ID:           "lsvd_volume/vol-other",
		NodeId:       entity.Id("node/other-node"),
		SizeGb:       10,
		DesiredState: storage_v1alpha.VOL_PRESENT,
		ActualState:  storage_v1alpha.VOL_PENDING,
	}
	createLsvdVolumeEntity(ctx, t, es, vol)

	// Run reconciliation
	err := vc.ReconcileWithEntities(ctx)
	require.NoError(t, err)

	// Verify nothing was created
	assert.Empty(t, ops.createdDirs)
	assert.Empty(t, ops.initedVolumes)
}

func TestVolumeControllerReconcileVolumeAlreadyReady(t *testing.T) {
	ctx := t.Context()
	log := testutils.TestLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	ops := newMockVolumeOps()

	// Pre-populate state with existing volume
	state.SetVolume("lsvd_volume/vol-ready", &VolumeState{
		EntityId:   "lsvd_volume/vol-ready",
		VolumeId:   "ready-vol-id",
		DiskPath:   "/data/volumes/ready-vol-id",
		SizeBytes:  10 * 1024 * 1024 * 1024,
		Filesystem: "ext4",
	})

	vc := newTestVolumeController(log, dataPath, nodeId, es.EAC, state, ops)

	// Create volume entity that is already ready
	vol := &storage_v1alpha.LsvdVolume{
		ID:           "lsvd_volume/vol-ready",
		NodeId:       entity.Id("node/" + nodeId),
		SizeGb:       10,
		Filesystem:   "ext4",
		DesiredState: storage_v1alpha.VOL_PRESENT,
		ActualState:  storage_v1alpha.VOL_READY,
		VolumeId:     "ready-vol-id",
	}
	createLsvdVolumeEntity(ctx, t, es, vol)

	// Run reconciliation
	err := vc.ReconcileWithEntities(ctx)
	require.NoError(t, err)

	// Verify nothing was created (already ready)
	assert.Empty(t, ops.createdDirs)
	assert.Empty(t, ops.initedVolumes)
}

func TestVolumeControllerReconcileWithSystem(t *testing.T) {
	ctx := t.Context()
	log := testutils.TestLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	ops := newMockVolumeOps()

	// Add volume to state
	state.SetVolume("vol-sys", &VolumeState{
		EntityId: "vol-sys",
		VolumeId: "sys-vol-id",
		DiskPath: "/data/volumes/sys-vol-id",
	})

	// Mark path as existing
	ops.existingPaths["/data/volumes/sys-vol-id"] = true

	vc := newTestVolumeController(log, dataPath, nodeId, es.EAC, state, ops)

	// Run system reconciliation
	err := vc.ReconcileWithSystem(ctx)
	require.NoError(t, err)

	// Volume should still be in state (path exists)
	assert.NotNil(t, state.GetVolume("vol-sys"))
}

func TestVolumeControllerReconcileWithSystemMissingPath(t *testing.T) {
	ctx := t.Context()
	log := testutils.TestLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	ops := newMockVolumeOps()

	// Add volume to state but don't mark path as existing
	state.SetVolume("vol-missing", &VolumeState{
		EntityId:   "vol-missing",
		VolumeId:   "missing-vol-id",
		DiskPath:   "/data/volumes/missing-vol-id",
		SizeBytes:  10 * 1024 * 1024 * 1024,
		Filesystem: "ext4",
	})

	vc := newTestVolumeController(log, dataPath, nodeId, es.EAC, state, ops)

	// Run system reconciliation — should recreate the volume directory
	err := vc.ReconcileWithSystem(ctx)
	require.NoError(t, err)

	// Verify the volume directory was recreated
	assert.Contains(t, ops.createdDirs, "/data/volumes/missing-vol-id")

	// Verify the volume was reinitialized
	require.Len(t, ops.initedVolumes, 1)
	assert.Equal(t, "/data/volumes/missing-vol-id", ops.initedVolumes[0].path)
	assert.Equal(t, "missing-vol-id", ops.initedVolumes[0].volumeId)
	assert.Equal(t, units.Bytes(10*1024*1024*1024), ops.initedVolumes[0].size)
	assert.Equal(t, map[string]any{"filesystem": "ext4"}, ops.initedVolumes[0].metadata)
}

func TestVolumeControllerReconcileCleansUpOrphanedVolumes(t *testing.T) {
	ctx := t.Context()
	log := testutils.TestLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	ops := newMockVolumeOps()

	// Pre-populate local state with a volume that has no corresponding entity
	state.SetVolume("lsvd_volume/vol-orphan", &VolumeState{
		EntityId:   "lsvd_volume/vol-orphan",
		VolumeId:   "orphan-vol-id",
		DiskPath:   "/data/volumes/orphan-vol-id",
		SizeBytes:  10 * 1024 * 1024 * 1024,
		Filesystem: "ext4",
	})
	ops.existingPaths["/data/volumes/orphan-vol-id"] = true

	vc := newTestVolumeController(log, dataPath, nodeId, es.EAC, state, ops)

	// Do NOT create any entity in the entity server — making this volume an orphan

	// Run reconciliation
	err := vc.ReconcileWithEntities(ctx)
	require.NoError(t, err)

	// Verify volume directory was removed
	assert.Contains(t, ops.removedDirs, "/data/volumes/orphan-vol-id")

	// Verify state was cleaned up
	assert.Nil(t, state.GetVolume("lsvd_volume/vol-orphan"))
}

func TestVolumeControllerReconcileKeepsNonOrphanedVolumes(t *testing.T) {
	ctx := t.Context()
	log := testutils.TestLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	ops := newMockVolumeOps()

	// Pre-populate local state with a volume that HAS a corresponding entity
	state.SetVolume("lsvd_volume/vol-keep", &VolumeState{
		EntityId:   "lsvd_volume/vol-keep",
		VolumeId:   "keep-vol-id",
		DiskPath:   "/data/volumes/keep-vol-id",
		SizeBytes:  10 * 1024 * 1024 * 1024,
		Filesystem: "ext4",
	})
	ops.existingPaths["/data/volumes/keep-vol-id"] = true

	// Also add an orphan
	state.SetVolume("lsvd_volume/vol-orphan2", &VolumeState{
		EntityId:   "lsvd_volume/vol-orphan2",
		VolumeId:   "orphan2-vol-id",
		DiskPath:   "/data/volumes/orphan2-vol-id",
		SizeBytes:  5 * 1024 * 1024 * 1024,
		Filesystem: "ext4",
	})
	ops.existingPaths["/data/volumes/orphan2-vol-id"] = true

	vc := newTestVolumeController(log, dataPath, nodeId, es.EAC, state, ops)

	// Create entity only for the kept volume
	vol := &storage_v1alpha.LsvdVolume{
		ID:           "lsvd_volume/vol-keep",
		NodeId:       entity.Id("node/" + nodeId),
		SizeGb:       10,
		Filesystem:   "ext4",
		DesiredState: storage_v1alpha.VOL_PRESENT,
		ActualState:  storage_v1alpha.VOL_READY,
		VolumeId:     "keep-vol-id",
	}
	createLsvdVolumeEntity(ctx, t, es, vol)

	// Run reconciliation
	err := vc.ReconcileWithEntities(ctx)
	require.NoError(t, err)

	// The kept volume should still exist in state
	assert.NotNil(t, state.GetVolume("lsvd_volume/vol-keep"))

	// The orphan should be cleaned up
	assert.Nil(t, state.GetVolume("lsvd_volume/vol-orphan2"))

	// Only the orphan directory should have been removed
	assert.Contains(t, ops.removedDirs, "/data/volumes/orphan2-vol-id")
	assert.NotContains(t, ops.removedDirs, "/data/volumes/keep-vol-id")
}

func TestVolumeControllerMultipleVolumes(t *testing.T) {
	ctx := t.Context()
	log := testutils.TestLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	ops := newMockVolumeOps()

	vc := newTestVolumeController(log, dataPath, nodeId, es.EAC, state, ops)

	// Create multiple volume entities
	for i := 1; i <= 3; i++ {
		vol := &storage_v1alpha.LsvdVolume{
			ID:           entity.Id("lsvd_volume/vol-" + string(rune('0'+i))),
			NodeId:       entity.Id("node/" + nodeId),
			SizeGb:       int64(i * 10),
			Filesystem:   "ext4",
			DesiredState: storage_v1alpha.VOL_PRESENT,
			ActualState:  storage_v1alpha.VOL_PENDING,
		}
		createLsvdVolumeEntity(ctx, t, es, vol)
	}

	// Run reconciliation
	err := vc.ReconcileWithEntities(ctx)
	require.NoError(t, err)

	// Verify all volumes were created
	assert.Len(t, ops.createdDirs, 3)
	assert.Len(t, ops.initedVolumes, 3)
	assert.Len(t, state.Volumes, 3)
}

