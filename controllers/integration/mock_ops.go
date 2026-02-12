package integration

import (
	"context"
	"net"
	"os"

	lsvdserver "miren.dev/runtime/components/lsvd/server"
	"miren.dev/runtime/pkg/units"
)

// mockVolumeOps implements lsvdserver.VolumeOps for testing.
type mockVolumeOps struct {
	createdDirs   []string
	existingPaths map[string]bool
}

func newMockVolumeOps() *mockVolumeOps {
	return &mockVolumeOps{
		existingPaths: make(map[string]bool),
	}
}

func (m *mockVolumeOps) CreateVolumeDir(path string) error {
	m.createdDirs = append(m.createdDirs, path)
	m.existingPaths[path] = true
	return nil
}

func (m *mockVolumeOps) RemoveVolumeDir(path string) error {
	delete(m.existingPaths, path)
	return nil
}

func (m *mockVolumeOps) VolumePathExists(path string) bool {
	return m.existingPaths[path]
}

func (m *mockVolumeOps) InitLSVDVolume(_ context.Context, _, volumeId string, _ units.Bytes, _ map[string]any, _ bool) (string, error) {
	return volumeId, nil
}

// Verify interface compliance
var _ lsvdserver.VolumeOps = (*mockVolumeOps)(nil)

// mockMountOps implements lsvdserver.MountOps for testing.
// In integration tests we use directory-mode on the DiskController/DiskLeaseController
// side, so the MountController's ops are only called for lsvd_mount entity reconciliation.
// The mock ops simulate successful NBD attach/mount/format flows without real OS interaction.
type mockMountOps struct {
	existingMounts map[string]bool
	formattedDisks map[string]string // device -> filesystem
	nbdStatuses    map[uint32]error
	nextNBDIndex   uint32
	openDiskCalls  int

	mockDisk *mockLSVDDisk
}

func newMockMountOps() *mockMountOps {
	return &mockMountOps{
		existingMounts: make(map[string]bool),
		formattedDisks: make(map[string]string),
		nbdStatuses:    make(map[uint32]error),
		nextNBDIndex:   1,
		mockDisk:       &mockLSVDDisk{size: 10 * 1024 * 1024 * 1024}, // 10GB
	}
}

func (m *mockMountOps) CreateDir(_ string, _ os.FileMode) error {
	return nil
}

func (m *mockMountOps) RemoveFile(_ string) error {
	return nil
}

func (m *mockMountOps) NBDLoopback(_ context.Context, _ uint64) (uint32, net.Conn, *os.File, func() error, error) {
	idx := m.nextNBDIndex
	m.nextNBDIndex++
	m.nbdStatuses[idx] = nil

	// Create a pipe for mock conn
	serverConn, clientConn := net.Pipe()
	_ = serverConn // not used in tests

	tmpFile, err := os.CreateTemp("", "mock-nbd-*")
	if err != nil {
		return 0, nil, nil, nil, err
	}

	cleanup := func() error {
		clientConn.Close()
		serverConn.Close()
		_ = tmpFile.Close()
		os.Remove(tmpFile.Name())
		return nil
	}

	return idx, clientConn, tmpFile, cleanup, nil
}

func (m *mockMountOps) NBDStatus(idx uint32) error {
	if err, ok := m.nbdStatuses[idx]; ok {
		return err
	}
	return nil
}

func (m *mockMountOps) NBDDisconnect(_ uint32) error {
	return nil
}

func (m *mockMountOps) CreateDeviceNode(_ string, _ uint32) error {
	return nil
}

func (m *mockMountOps) Mount(_, mountPath, _ string, _ bool) error {
	m.existingMounts[mountPath] = true
	return nil
}

func (m *mockMountOps) Unmount(path string) error {
	delete(m.existingMounts, path)
	return nil
}

func (m *mockMountOps) IsMounted(path string) bool {
	return m.existingMounts[path]
}

func (m *mockMountOps) IsFormatted(device, _ string) (bool, error) {
	_, ok := m.formattedDisks[device]
	return ok, nil
}

func (m *mockMountOps) FormatDevice(_ context.Context, device, filesystem string) error {
	m.formattedDisks[device] = filesystem
	return nil
}

func (m *mockMountOps) OpenLSVDDisk(_ context.Context, _, _ string, _ bool, _ string) (lsvdserver.LSVDDisk, error) {
	m.openDiskCalls++
	return m.mockDisk, nil
}

func (m *mockMountOps) AcquireVolumeLease(_ context.Context, _ string, _ map[string]any) (string, error) {
	return "mock-nonce", nil
}

func (m *mockMountOps) ReleaseVolumeLease(_ context.Context, _, _ string) error {
	return nil
}

// Verify interface compliance
var _ lsvdserver.MountOps = (*mockMountOps)(nil)

// mockLSVDDisk implements lsvdserver.LSVDDisk for testing.
type mockLSVDDisk struct {
	size int64
}

func (d *mockLSVDDisk) Close(_ context.Context) error {
	return nil
}

func (d *mockLSVDDisk) Size() int64 {
	return d.size
}

func (d *mockLSVDDisk) HandleNBD(ctx context.Context, _ net.Conn, _ *os.File) error {
	<-ctx.Done()
	return ctx.Err()
}
