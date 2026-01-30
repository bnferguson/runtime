package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"miren.dev/runtime/api/entityserver/entityserver_v1alpha"
	"miren.dev/runtime/api/storage/storage_v1alpha"
	"miren.dev/runtime/pkg/entity"
	"miren.dev/runtime/pkg/entity/testutils"
)

// newTestMountController creates a MountController with EAC pre-set for testing
func newTestMountController(log *slog.Logger, dataPath, nodeId string, eac *entityserver_v1alpha.EntityAccessClient, state *State, ops MountOps) *MountController {
	mc := NewMountController(log, dataPath, nodeId, state, ops)
	mc.SetEAC(eac)
	return mc
}

// mockMountOps implements MountOps for testing
type mockMountOps struct {
	// Track calls
	createdDirs        []mockDirCreate
	removedFiles       []string
	nbdLoopbackCalls   []mockNBDLoopback
	nbdStatusCalls     []uint32
	nbdDisconnectCalls []uint32
	deviceNodes        []mockDeviceNode
	mounts             []mockMount
	unmounts           []string
	formattedDevices   []mockFormat

	// Return values and state
	existingMounts map[string]bool
	formattedDisks map[string]string // device -> filesystem
	nbdStatuses    map[uint32]error  // idx -> status error (nil = connected)
	nextNBDIndex   uint32

	// Error injection
	createDirErr     error
	removeFileErr    error
	nbdLoopbackErr   error
	nbdStatusErr     error
	nbdDisconnectErr error
	createDevNodeErr error
	mountErr         error
	unmountErr       error
	formatErr        error
	openDiskErr      error

	// Mock disk to return
	mockDisk *mockLSVDDisk
}

type mockDirCreate struct {
	path string
	perm os.FileMode
}

type mockNBDLoopback struct {
	sizeBytes uint64
}

type mockDeviceNode struct {
	path     string
	nbdIndex uint32
}

type mockMount struct {
	device     string
	mountPath  string
	filesystem string
	readOnly   bool
}

type mockFormat struct {
	device     string
	filesystem string
}

func newMockMountOps() *mockMountOps {
	return &mockMountOps{
		existingMounts: make(map[string]bool),
		formattedDisks: make(map[string]string),
		nbdStatuses:    make(map[uint32]error),
		nextNBDIndex:   1,
	}
}

func (m *mockMountOps) CreateDir(path string, perm os.FileMode) error {
	if m.createDirErr != nil {
		return m.createDirErr
	}
	m.createdDirs = append(m.createdDirs, mockDirCreate{path: path, perm: perm})
	return nil
}

func (m *mockMountOps) RemoveFile(path string) error {
	if m.removeFileErr != nil {
		return m.removeFileErr
	}
	m.removedFiles = append(m.removedFiles, path)
	return nil
}

func (m *mockMountOps) NBDLoopback(ctx context.Context, sizeBytes uint64) (uint32, net.Conn, *os.File, func() error, error) {
	if m.nbdLoopbackErr != nil {
		return 0, nil, nil, nil, m.nbdLoopbackErr
	}

	m.nbdLoopbackCalls = append(m.nbdLoopbackCalls, mockNBDLoopback{sizeBytes: sizeBytes})

	idx := m.nextNBDIndex
	m.nextNBDIndex++
	m.nbdStatuses[idx] = nil // Mark as connected

	// Create a mock connection using net.Pipe
	client, server := net.Pipe()

	// Create a temp file for clientFile to avoid nil pointer panic
	clientFile, err := os.CreateTemp("", "mock-nbd-client-*")
	if err != nil {
		return 0, nil, nil, nil, fmt.Errorf("failed to create mock client file: %w", err)
	}

	cleanup := func() error {
		client.Close()
		server.Close()
		clientFile.Close()
		os.Remove(clientFile.Name())
		delete(m.nbdStatuses, idx)
		return nil
	}

	// Return server side as conn
	return idx, server, clientFile, cleanup, nil
}

func (m *mockMountOps) NBDStatus(idx uint32) error {
	m.nbdStatusCalls = append(m.nbdStatusCalls, idx)

	if m.nbdStatusErr != nil {
		return m.nbdStatusErr
	}

	if err, ok := m.nbdStatuses[idx]; ok {
		return err
	}
	return os.ErrNotExist // Not connected
}

func (m *mockMountOps) NBDDisconnect(idx uint32) error {
	m.nbdDisconnectCalls = append(m.nbdDisconnectCalls, idx)

	if m.nbdDisconnectErr != nil {
		return m.nbdDisconnectErr
	}

	delete(m.nbdStatuses, idx)
	return nil
}

func (m *mockMountOps) CreateDeviceNode(path string, nbdIndex uint32) error {
	if m.createDevNodeErr != nil {
		return m.createDevNodeErr
	}
	m.deviceNodes = append(m.deviceNodes, mockDeviceNode{path: path, nbdIndex: nbdIndex})
	return nil
}

func (m *mockMountOps) Mount(device, mountPath, filesystem string, readOnly bool) error {
	if m.mountErr != nil {
		return m.mountErr
	}
	m.mounts = append(m.mounts, mockMount{
		device:     device,
		mountPath:  mountPath,
		filesystem: filesystem,
		readOnly:   readOnly,
	})
	m.existingMounts[mountPath] = true
	return nil
}

func (m *mockMountOps) Unmount(path string) error {
	if m.unmountErr != nil {
		return m.unmountErr
	}
	m.unmounts = append(m.unmounts, path)
	delete(m.existingMounts, path)
	return nil
}

func (m *mockMountOps) IsMounted(path string) bool {
	return m.existingMounts[path]
}

func (m *mockMountOps) IsFormatted(device, filesystem string) (bool, error) {
	if fs, ok := m.formattedDisks[device]; ok {
		return fs == filesystem, nil
	}
	return false, nil
}

func (m *mockMountOps) FormatDevice(ctx context.Context, device, filesystem string) error {
	if m.formatErr != nil {
		return m.formatErr
	}
	m.formattedDevices = append(m.formattedDevices, mockFormat{device: device, filesystem: filesystem})
	m.formattedDisks[device] = filesystem
	return nil
}

func (m *mockMountOps) OpenLSVDDisk(ctx context.Context, diskPath, volumeId string, remoteOnly bool, leaseNonce string) (LSVDDisk, error) {
	if m.openDiskErr != nil {
		return nil, m.openDiskErr
	}
	if m.mockDisk == nil {
		m.mockDisk = &mockLSVDDisk{size: 10 * 1024 * 1024 * 1024} // 10GB default
	}
	return m.mockDisk, nil
}

func (m *mockMountOps) AcquireVolumeLease(ctx context.Context, volumeId string, metadata map[string]any) (string, error) {
	return "mock-nonce", nil
}

func (m *mockMountOps) ReleaseVolumeLease(ctx context.Context, volumeId, nonce string) error {
	return nil
}

// mockLSVDDisk implements LSVDDisk for testing
type mockLSVDDisk struct {
	size      int64
	closed    bool
	handleErr error
}

func (d *mockLSVDDisk) Close(ctx context.Context) error {
	d.closed = true
	return nil
}

func (d *mockLSVDDisk) Size() int64 {
	return d.size
}

func (d *mockLSVDDisk) HandleNBD(ctx context.Context, conn net.Conn, clientFile *os.File) error {
	if d.handleErr != nil {
		return d.handleErr
	}
	// Wait for context cancellation
	<-ctx.Done()
	return ctx.Err()
}

// createLsvdMountEntity creates an lsvd_mount entity in the test entity server
func createLsvdMountEntity(ctx context.Context, t *testing.T, es *testutils.InMemEntityServer, mount *storage_v1alpha.LsvdMount) {
	_, err := es.EAC.Create(ctx, entity.New(
		entity.DBId, mount.ID,
		mount.Encode,
	).Attrs())
	require.NoError(t, err)
}

func TestMountControllerReconcileMountMounted(t *testing.T) {
	ctx := t.Context()
	log := testutils.TestLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	ops := newMockMountOps()

	// Pre-populate volume state (mount requires an existing volume)
	state.SetVolume("lsvd_volume/vol-123", &VolumeState{
		EntityId:   "lsvd_volume/vol-123",
		VolumeId:   "actual-vol-id",
		DiskPath:   "/data/volumes/actual-vol-id",
		SizeBytes:  10 * 1024 * 1024 * 1024,
		Filesystem: "ext4",
	})

	mc := newTestMountController(log, dataPath, nodeId, es.EAC, state, ops)

	// Create mount entity in PENDING state
	// Note: NodeId must use full entity ID format "node/<name>"
	fullNodeId := "node/" + nodeId
	mount := &storage_v1alpha.LsvdMount{
		ID:           "lsvd_mount/mnt-123",
		NodeId:       entity.Id(fullNodeId),
		VolumeId:     "lsvd_volume/vol-123",
		MountPath:    "/mnt/data",
		ReadOnly:     false,
		DesiredState: storage_v1alpha.MNT_WANT_MOUNTED,
		ActualState:  storage_v1alpha.MNT_PENDING,
	}
	createLsvdMountEntity(ctx, t, es, mount)

	// Run reconciliation
	err := mc.ReconcileWithEntities(ctx)
	require.NoError(t, err)

	// Verify NBD was set up
	assert.Len(t, ops.nbdLoopbackCalls, 1)
	assert.Equal(t, uint64(10*1024*1024*1024), ops.nbdLoopbackCalls[0].sizeBytes)

	// Verify filesystem was formatted (device path uses mount entity ID, not volume ID)
	expectedDevPath := filepath.Join(dataPath, "devices", "mnt-123")
	assert.Len(t, ops.formattedDevices, 1)
	assert.Equal(t, "ext4", ops.formattedDevices[0].filesystem)
	assert.Equal(t, expectedDevPath, ops.formattedDevices[0].device)

	// Verify mount was performed with the correct device path
	assert.Len(t, ops.mounts, 1)
	assert.Equal(t, expectedDevPath, ops.mounts[0].device)
	assert.Equal(t, "/mnt/data", ops.mounts[0].mountPath)
	assert.Equal(t, "ext4", ops.mounts[0].filesystem)
	assert.False(t, ops.mounts[0].readOnly)

	// Verify state was updated
	mountState := state.GetMount("lsvd_mount/mnt-123")
	require.NotNil(t, mountState)
	assert.True(t, mountState.Mounted)
	assert.Equal(t, "/mnt/data", mountState.MountPath)

	// Verify entity was updated
	resp, err := es.EAC.Get(ctx, "lsvd_mount/mnt-123")
	require.NoError(t, err)
	var updated storage_v1alpha.LsvdMount
	updated.Decode(resp.Entity().Entity())
	assert.Equal(t, storage_v1alpha.MNT_MOUNTED, updated.ActualState)
}

func TestMountControllerReconcileMountUnmounted(t *testing.T) {
	ctx := t.Context()
	log := testutils.TestLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	ops := newMockMountOps()

	// Pre-populate state with existing mount
	state.SetMount("lsvd_mount/mnt-456", &MountState{
		EntityId:   "lsvd_mount/mnt-456",
		VolumeId:   "lsvd_volume/vol-456",
		NbdIndex:   5,
		DevicePath: "/data/devices/vol-456",
		MountPath:  "/mnt/data",
		Mounted:    true,
		ReadOnly:   false,
	})
	ops.existingMounts["/mnt/data"] = true
	ops.nbdStatuses[5] = nil // NBD is connected

	mc := newTestMountController(log, dataPath, nodeId, es.EAC, state, ops)

	// Create mount entity requesting unmount
	mount := &storage_v1alpha.LsvdMount{
		ID:           "lsvd_mount/mnt-456",
		NodeId:       entity.Id("node/" + nodeId),
		VolumeId:     "lsvd_volume/vol-456",
		MountPath:    "/mnt/data",
		DesiredState: storage_v1alpha.MNT_WANT_UNMOUNTED,
		ActualState:  storage_v1alpha.MNT_MOUNTED,
	}
	createLsvdMountEntity(ctx, t, es, mount)

	// Run reconciliation
	err := mc.ReconcileWithEntities(ctx)
	require.NoError(t, err)

	// Verify unmount was called
	assert.Len(t, ops.unmounts, 1)
	assert.Equal(t, "/mnt/data", ops.unmounts[0])

	// Verify NBD was disconnected
	assert.Contains(t, ops.nbdDisconnectCalls, uint32(5))

	// Verify device file was removed
	assert.Contains(t, ops.removedFiles, "/data/devices/vol-456")

	// Verify state was cleaned up
	assert.Nil(t, state.GetMount("lsvd_mount/mnt-456"))

	// Verify entity was updated
	resp, err := es.EAC.Get(ctx, "lsvd_mount/mnt-456")
	require.NoError(t, err)
	var updated storage_v1alpha.LsvdMount
	updated.Decode(resp.Entity().Entity())
	assert.Equal(t, storage_v1alpha.MNT_DETACHED, updated.ActualState)
}

func TestMountControllerReconcileSkipsOtherNodes(t *testing.T) {
	ctx := t.Context()
	log := testutils.TestLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	ops := newMockMountOps()

	mc := newTestMountController(log, dataPath, nodeId, es.EAC, state, ops)

	// Create mount entity for a different node
	mount := &storage_v1alpha.LsvdMount{
		ID:           "lsvd_mount/mnt-other",
		NodeId:       entity.Id("other-node"),
		VolumeId:     "lsvd_volume/vol-other",
		MountPath:    "/mnt/other",
		DesiredState: storage_v1alpha.MNT_WANT_MOUNTED,
		ActualState:  storage_v1alpha.MNT_PENDING,
	}
	createLsvdMountEntity(ctx, t, es, mount)

	// Run reconciliation
	err := mc.ReconcileWithEntities(ctx)
	require.NoError(t, err)

	// Verify nothing was done
	assert.Empty(t, ops.nbdLoopbackCalls)
	assert.Empty(t, ops.mounts)
}

func TestMountControllerReconcileMountAlreadyMounted(t *testing.T) {
	ctx := t.Context()
	log := testutils.TestLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	ops := newMockMountOps()

	// Pre-populate state with existing mounted volume
	state.SetMount("lsvd_mount/mnt-ready", &MountState{
		EntityId:   "lsvd_mount/mnt-ready",
		VolumeId:   "lsvd_volume/vol-ready",
		NbdIndex:   3,
		DevicePath: "/data/devices/vol-ready",
		MountPath:  "/mnt/ready",
		Mounted:    true,
	})

	mc := newTestMountController(log, dataPath, nodeId, es.EAC, state, ops)

	// Create mount entity that is already mounted
	mount := &storage_v1alpha.LsvdMount{
		ID:           "lsvd_mount/mnt-ready",
		NodeId:       entity.Id("node/" + nodeId),
		VolumeId:     "lsvd_volume/vol-ready",
		MountPath:    "/mnt/ready",
		DesiredState: storage_v1alpha.MNT_WANT_MOUNTED,
		ActualState:  storage_v1alpha.MNT_MOUNTED,
	}
	createLsvdMountEntity(ctx, t, es, mount)

	// Run reconciliation
	err := mc.ReconcileWithEntities(ctx)
	require.NoError(t, err)

	// Verify no new operations were performed
	assert.Empty(t, ops.nbdLoopbackCalls)
	assert.Empty(t, ops.mounts)
	assert.Empty(t, ops.formattedDevices)
}

func TestMountControllerReconcileVolumeNotFound(t *testing.T) {
	ctx := t.Context()
	log := testutils.TestLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	ops := newMockMountOps()

	// Don't pre-populate volume state - volume doesn't exist

	mc := newTestMountController(log, dataPath, nodeId, es.EAC, state, ops)

	// Create mount entity referencing non-existent volume
	mount := &storage_v1alpha.LsvdMount{
		ID:           "lsvd_mount/mnt-missing",
		NodeId:       entity.Id("node/" + nodeId),
		VolumeId:     "lsvd_volume/vol-missing",
		MountPath:    "/mnt/missing",
		DesiredState: storage_v1alpha.MNT_WANT_MOUNTED,
		ActualState:  storage_v1alpha.MNT_PENDING,
	}
	createLsvdMountEntity(ctx, t, es, mount)

	// Run reconciliation - should not panic, should set error state
	err := mc.ReconcileWithEntities(ctx)
	require.NoError(t, err) // ReconcileWithEntities logs errors but doesn't return them

	// Verify entity was updated to error state
	resp, err := es.EAC.Get(ctx, "lsvd_mount/mnt-missing")
	require.NoError(t, err)
	var updated storage_v1alpha.LsvdMount
	updated.Decode(resp.Entity().Entity())
	assert.Equal(t, storage_v1alpha.MNT_ERROR, updated.ActualState)
	assert.Contains(t, updated.ErrorMessage, "not found")
}

func TestMountControllerReconcileMountReadOnly(t *testing.T) {
	ctx := t.Context()
	log := testutils.TestLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	ops := newMockMountOps()

	// Pre-populate volume state
	state.SetVolume("lsvd_volume/vol-ro", &VolumeState{
		EntityId:   "lsvd_volume/vol-ro",
		VolumeId:   "ro-vol-id",
		DiskPath:   "/data/volumes/ro-vol-id",
		SizeBytes:  5 * 1024 * 1024 * 1024,
		Filesystem: "xfs",
	})

	// Pre-format the device to skip formatting
	ops.formattedDisks[dataPath+"/devices/ro-vol-id"] = "xfs"

	mc := newTestMountController(log, dataPath, nodeId, es.EAC, state, ops)

	// Create read-only mount request
	mount := &storage_v1alpha.LsvdMount{
		ID:           "lsvd_mount/mnt-ro",
		NodeId:       entity.Id("node/" + nodeId),
		VolumeId:     "lsvd_volume/vol-ro",
		MountPath:    "/mnt/readonly",
		ReadOnly:     true,
		DesiredState: storage_v1alpha.MNT_WANT_MOUNTED,
		ActualState:  storage_v1alpha.MNT_PENDING,
	}
	createLsvdMountEntity(ctx, t, es, mount)

	// Run reconciliation
	err := mc.ReconcileWithEntities(ctx)
	require.NoError(t, err)

	// Verify mount was called with readOnly flag
	assert.Len(t, ops.mounts, 1)
	assert.True(t, ops.mounts[0].readOnly)
	assert.Equal(t, "xfs", ops.mounts[0].filesystem)

	// Verify state reflects read-only
	mountState := state.GetMount("lsvd_mount/mnt-ro")
	require.NotNil(t, mountState)
	assert.True(t, mountState.ReadOnly)
}

func TestMountControllerReconcileWithSystemNBDDisconnected(t *testing.T) {
	ctx := t.Context()
	log := testutils.TestLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	ops := newMockMountOps()

	// Pre-populate volume state (needed for reconnection)
	state.SetVolume("lsvd_volume/vol-sys", &VolumeState{
		EntityId:   "lsvd_volume/vol-sys",
		VolumeId:   "sys-vol-id",
		DiskPath:   "/data/volumes/sys-vol-id",
		SizeBytes:  10 * 1024 * 1024 * 1024,
		Filesystem: "ext4",
	})

	// Pre-populate mount state with "connected" NBD that is actually disconnected
	state.SetMount("lsvd_mount/mnt-sys", &MountState{
		EntityId:   "lsvd_mount/mnt-sys",
		VolumeId:   "lsvd_volume/vol-sys",
		NbdIndex:   10,
		DevicePath: "/data/devices/sys-vol-id",
		MountPath:  "/mnt/sys",
		Mounted:    true,
	})
	// Note: ops.nbdStatuses[10] is NOT set, so NBD status will return error

	mc := newTestMountController(log, dataPath, nodeId, es.EAC, state, ops)

	// Run system reconciliation
	err := mc.ReconcileWithSystem(ctx)
	require.NoError(t, err)

	// Verify reconnection was attempted (no handler exists, so reconnect triggers)
	assert.NotEmpty(t, ops.nbdLoopbackCalls)
}

func TestMountControllerReconcileWithSystemMountMissing(t *testing.T) {
	ctx := t.Context()
	log := testutils.TestLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	ops := newMockMountOps()

	// Pre-populate volume state
	state.SetVolume("lsvd_volume/vol-unmounted", &VolumeState{
		EntityId:   "lsvd_volume/vol-unmounted",
		VolumeId:   "unmounted-vol-id",
		DiskPath:   "/data/volumes/unmounted-vol-id",
		SizeBytes:  10 * 1024 * 1024 * 1024,
		Filesystem: "ext4",
	})

	// Pre-populate mount state claiming it's mounted
	state.SetMount("lsvd_mount/mnt-unmounted", &MountState{
		EntityId:   "lsvd_mount/mnt-unmounted",
		VolumeId:   "lsvd_volume/vol-unmounted",
		NbdIndex:   15,
		DevicePath: "/data/devices/unmounted-vol-id",
		MountPath:  "/mnt/unmounted",
		Mounted:    true,
	})
	// NBD is connected but filesystem is not mounted
	ops.nbdStatuses[15] = nil
	// Note: ops.existingMounts doesn't contain /mnt/unmounted

	mc := newTestMountController(log, dataPath, nodeId, es.EAC, state, ops)

	// Add a handler so reconnect isn't triggered
	mc.handlers["lsvd_mount/mnt-unmounted"] = &nbdHandler{
		cancel: func() {},
	}

	// Pre-format the device
	ops.formattedDisks["/data/devices/unmounted-vol-id"] = "ext4"

	// Run system reconciliation
	err := mc.ReconcileWithSystem(ctx)
	require.NoError(t, err)

	// Verify remount was attempted
	assert.NotEmpty(t, ops.mounts)
	assert.Equal(t, "/mnt/unmounted", ops.mounts[0].mountPath)
}

func TestMountControllerMultipleMounts(t *testing.T) {
	ctx := t.Context()
	log := testutils.TestLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	ops := newMockMountOps()

	// Pre-populate volume states for multiple mounts
	for i := 1; i <= 3; i++ {
		volId := entity.Id("lsvd_volume/vol-" + string(rune('0'+i)))
		state.SetVolume(string(volId), &VolumeState{
			EntityId:   string(volId),
			VolumeId:   "multi-vol-" + string(rune('0'+i)),
			DiskPath:   "/data/volumes/multi-vol-" + string(rune('0'+i)),
			SizeBytes:  int64(i * 10 * 1024 * 1024 * 1024),
			Filesystem: "ext4",
		})
	}

	mc := newTestMountController(log, dataPath, nodeId, es.EAC, state, ops)

	// Create multiple mount entities
	for i := 1; i <= 3; i++ {
		mount := &storage_v1alpha.LsvdMount{
			ID:           entity.Id("lsvd_mount/mnt-" + string(rune('0'+i))),
			NodeId:       entity.Id("node/" + nodeId),
			VolumeId:     entity.Id("lsvd_volume/vol-" + string(rune('0'+i))),
			MountPath:    "/mnt/data" + string(rune('0'+i)),
			DesiredState: storage_v1alpha.MNT_WANT_MOUNTED,
			ActualState:  storage_v1alpha.MNT_PENDING,
		}
		createLsvdMountEntity(ctx, t, es, mount)
	}

	// Run reconciliation
	err := mc.ReconcileWithEntities(ctx)
	require.NoError(t, err)

	// Verify all mounts were processed
	assert.Len(t, ops.nbdLoopbackCalls, 3)
	assert.Len(t, ops.mounts, 3)
	assert.Len(t, state.Mounts, 3)
}

func TestMountControllerAlreadyDetached(t *testing.T) {
	ctx := t.Context()
	log := testutils.TestLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	ops := newMockMountOps()

	mc := newTestMountController(log, dataPath, nodeId, es.EAC, state, ops)

	// Create mount entity already in DETACHED state
	mount := &storage_v1alpha.LsvdMount{
		ID:           "lsvd_mount/mnt-detached",
		NodeId:       entity.Id("node/" + nodeId),
		VolumeId:     "lsvd_volume/vol-detached",
		MountPath:    "/mnt/detached",
		DesiredState: storage_v1alpha.MNT_WANT_UNMOUNTED,
		ActualState:  storage_v1alpha.MNT_DETACHED,
	}
	createLsvdMountEntity(ctx, t, es, mount)

	// Run reconciliation
	err := mc.ReconcileWithEntities(ctx)
	require.NoError(t, err)

	// Verify nothing was done (already detached)
	assert.Empty(t, ops.unmounts)
	assert.Empty(t, ops.nbdDisconnectCalls)
}

func TestMountControllerReconcileCleansUpOrphanedMounts(t *testing.T) {
	ctx := t.Context()
	log := testutils.TestLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	ops := newMockMountOps()

	// Pre-populate local state with a mount that has no corresponding entity
	state.SetMount("lsvd_mount/mnt-orphan", &MountState{
		EntityId:   "lsvd_mount/mnt-orphan",
		VolumeId:   "lsvd_volume/vol-orphan",
		NbdIndex:   7,
		DevicePath: "/data/devices/orphan-vol-id",
		MountPath:  "/mnt/orphan",
		Mounted:    true,
		ReadOnly:   false,
	})
	ops.existingMounts["/mnt/orphan"] = true
	ops.nbdStatuses[7] = nil

	mc := newTestMountController(log, dataPath, nodeId, es.EAC, state, ops)

	// Do NOT create any entity in the entity server — making this mount an orphan

	// Run reconciliation
	err := mc.ReconcileWithEntities(ctx)
	require.NoError(t, err)

	// Verify unmount was called
	assert.Contains(t, ops.unmounts, "/mnt/orphan")

	// Verify NBD was disconnected
	assert.Contains(t, ops.nbdDisconnectCalls, uint32(7))

	// Verify device file was removed
	assert.Contains(t, ops.removedFiles, "/data/devices/orphan-vol-id")

	// Verify state was cleaned up
	assert.Nil(t, state.GetMount("lsvd_mount/mnt-orphan"))
}

func TestMountControllerReconcileKeepsNonOrphanedMounts(t *testing.T) {
	ctx := t.Context()
	log := testutils.TestLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	ops := newMockMountOps()

	// Pre-populate volume state
	state.SetVolume("lsvd_volume/vol-keep", &VolumeState{
		EntityId:   "lsvd_volume/vol-keep",
		VolumeId:   "keep-vol-id",
		DiskPath:   "/data/volumes/keep-vol-id",
		SizeBytes:  10 * 1024 * 1024 * 1024,
		Filesystem: "ext4",
	})

	// Pre-populate local state with a mount that HAS a corresponding entity
	state.SetMount("lsvd_mount/mnt-keep", &MountState{
		EntityId:   "lsvd_mount/mnt-keep",
		VolumeId:   "lsvd_volume/vol-keep",
		NbdIndex:   8,
		DevicePath: "/data/devices/keep-vol-id",
		MountPath:  "/mnt/keep",
		Mounted:    true,
		ReadOnly:   false,
	})
	ops.existingMounts["/mnt/keep"] = true

	// Also add an orphan
	state.SetMount("lsvd_mount/mnt-orphan2", &MountState{
		EntityId:   "lsvd_mount/mnt-orphan2",
		VolumeId:   "lsvd_volume/vol-orphan2",
		NbdIndex:   9,
		DevicePath: "/data/devices/orphan2-vol-id",
		MountPath:  "/mnt/orphan2",
		Mounted:    true,
		ReadOnly:   false,
	})
	ops.existingMounts["/mnt/orphan2"] = true

	mc := newTestMountController(log, dataPath, nodeId, es.EAC, state, ops)

	// Create entity only for the kept mount
	mount := &storage_v1alpha.LsvdMount{
		ID:           "lsvd_mount/mnt-keep",
		NodeId:       entity.Id("node/" + nodeId),
		VolumeId:     "lsvd_volume/vol-keep",
		MountPath:    "/mnt/keep",
		DesiredState: storage_v1alpha.MNT_WANT_MOUNTED,
		ActualState:  storage_v1alpha.MNT_MOUNTED,
	}
	createLsvdMountEntity(ctx, t, es, mount)

	// Run reconciliation
	err := mc.ReconcileWithEntities(ctx)
	require.NoError(t, err)

	// The kept mount should still exist in state
	assert.NotNil(t, state.GetMount("lsvd_mount/mnt-keep"))

	// The orphan should be cleaned up
	assert.Nil(t, state.GetMount("lsvd_mount/mnt-orphan2"))

	// Only the orphan should have been unmounted
	assert.Contains(t, ops.unmounts, "/mnt/orphan2")
	assert.NotContains(t, ops.unmounts, "/mnt/keep")
}

func TestMountControllerShutdownCleansUpHandlers(t *testing.T) {
	log := testutils.TestLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	dataPath := t.TempDir()
	nodeId := "test-node-1"
	state := NewState()
	ops := newMockMountOps()

	mc := newTestMountController(log, dataPath, nodeId, es.EAC, state, ops)

	// Add a handler manually
	cancelled := false
	mc.handlers["test-entity"] = &nbdHandler{
		cancel: func() { cancelled = true },
	}

	mc.Shutdown()

	assert.True(t, cancelled)
}
