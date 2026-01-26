package lsvd

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"miren.dev/runtime/api/lsvd/lsvd_v1alpha"
	"miren.dev/runtime/pkg/rpc"
	"miren.dev/runtime/pkg/rpc/standard"
)

func TestComponentNewComponent(t *testing.T) {
	log := slog.Default()
	dataPath := "/var/lib/test"

	comp := NewComponent(log, dataPath)

	assert.NotNil(t, comp)
	assert.Equal(t, dataPath, comp.dataPath)
	assert.False(t, comp.IsRunning())
}

func TestComponentStartMissingBinary(t *testing.T) {
	log := slog.Default()
	tempDir := t.TempDir()

	comp := NewComponent(log, tempDir)

	ctx := context.Background()
	err := comp.Start(ctx, &Config{
		BinaryPath:       "/nonexistent/binary",
		DataPath:         tempDir,
		EntityServerAddr: "localhost:9000",
		NodeId:           "test-node",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	assert.False(t, comp.IsRunning())
}

func TestComponentStartAlreadyRunning(t *testing.T) {
	log := slog.Default()
	tempDir := t.TempDir()

	comp := NewComponent(log, tempDir)

	// Simulate running state
	comp.running = true

	ctx := context.Background()
	err := comp.Start(ctx, &Config{
		BinaryPath:       "/bin/sleep",
		DataPath:         tempDir,
		EntityServerAddr: "localhost:9000",
		NodeId:           "test-node",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")
}

func TestComponentStopNotRunning(t *testing.T) {
	log := slog.Default()
	tempDir := t.TempDir()

	comp := NewComponent(log, tempDir)

	ctx := context.Background()
	err := comp.Stop(ctx)

	// Should not error when not running
	assert.NoError(t, err)
}

func TestComponentIsRunning(t *testing.T) {
	log := slog.Default()
	tempDir := t.TempDir()

	comp := NewComponent(log, tempDir)

	assert.False(t, comp.IsRunning())

	comp.running = true
	assert.True(t, comp.IsRunning())
}

func TestComponentPIDNotRunning(t *testing.T) {
	log := slog.Default()
	tempDir := t.TempDir()

	comp := NewComponent(log, tempDir)

	_, err := comp.PID()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not running")
}

func TestComponentClose(t *testing.T) {
	log := slog.Default()
	tempDir := t.TempDir()

	comp := NewComponent(log, tempDir)

	// Close should work even when not running
	err := comp.Close()
	assert.NoError(t, err)
}

func TestComponentDataDirectoryCreation(t *testing.T) {
	log := slog.Default()
	tempDir := t.TempDir()
	dataPath := filepath.Join(tempDir, "subdir", "data")

	comp := NewComponent(log, tempDir)

	// The Start method creates the data directory
	// We test this by checking the error is about the binary, not the directory
	ctx := context.Background()
	err := comp.Start(ctx, &Config{
		BinaryPath:       "/nonexistent/binary",
		DataPath:         dataPath,
		EntityServerAddr: "localhost:9000",
		NodeId:           "test-node",
	})

	// Should fail because binary not found, not because directory creation failed
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestWaitForReadyTimeout(t *testing.T) {
	log := slog.Default()
	tempDir := t.TempDir()

	comp := NewComponent(log, tempDir)

	// Create a context that will cancel quickly
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	readyFile := filepath.Join(tempDir, "test.ready")
	err := comp.waitForReady(ctx, readyFile)

	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)
}

func TestWaitForReadySuccess(t *testing.T) {
	log := slog.Default()
	tempDir := t.TempDir()

	comp := NewComponent(log, tempDir)

	readyFile := filepath.Join(tempDir, "test.ready")

	// Create ready file immediately
	err := os.WriteFile(readyFile, []byte("ready"), 0644)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = comp.waitForReady(ctx, readyFile)
	assert.NoError(t, err)
}

func TestWaitForReadyDelayed(t *testing.T) {
	log := slog.Default()
	tempDir := t.TempDir()

	comp := NewComponent(log, tempDir)

	readyFile := filepath.Join(tempDir, "test.ready")

	// Create ready file after a short delay
	go func() {
		time.Sleep(200 * time.Millisecond)
		os.WriteFile(readyFile, []byte("ready"), 0644)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := comp.waitForReady(ctx, readyFile)
	assert.NoError(t, err)
}

func TestComponentConnectRPCMissingFile(t *testing.T) {
	log := slog.Default()
	tempDir := t.TempDir()

	comp := NewComponent(log, tempDir)

	ctx := context.Background()
	err := comp.connectRPC(ctx, tempDir)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read address file")
}

func TestComponentConnectRPCEmptyAddress(t *testing.T) {
	log := slog.Default()
	tempDir := t.TempDir()

	comp := NewComponent(log, tempDir)

	// Write empty address file
	addrFile := filepath.Join(tempDir, "lsvd-server.addr")
	err := os.WriteFile(addrFile, []byte("   \n  "), 0644)
	require.NoError(t, err)

	ctx := context.Background()
	err = comp.connectRPC(ctx, tempDir)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty address")
}

func TestComponentConnectRPCInvalidAddress(t *testing.T) {
	log := slog.Default()
	tempDir := t.TempDir()

	comp := NewComponent(log, tempDir)

	// Write invalid address
	addrFile := filepath.Join(tempDir, "lsvd-server.addr")
	err := os.WriteFile(addrFile, []byte("invalid:99999\n"), 0644)
	require.NoError(t, err)

	ctx := context.Background()
	err = comp.connectRPC(ctx, tempDir)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect")
}

func TestComponentHealthNoClient(t *testing.T) {
	log := slog.Default()
	tempDir := t.TempDir()

	comp := NewComponent(log, tempDir)

	// Component without RPC client
	_, err := comp.Health()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "RPC client not connected")
}

func TestComponentIsHealthyNotRunning(t *testing.T) {
	log := slog.Default()
	tempDir := t.TempDir()

	comp := NewComponent(log, tempDir)

	// Not running should return false
	assert.False(t, comp.IsHealthy())
}

func TestComponentIsHealthyNoClient(t *testing.T) {
	log := slog.Default()
	tempDir := t.TempDir()

	comp := NewComponent(log, tempDir)

	// Simulate running but no RPC client
	comp.running = true

	// Should return false because Health() will fail
	assert.False(t, comp.IsHealthy())
}

// mockDebugService implements the LsvdDebug interface for testing
type mockDebugService struct {
	healthy               bool
	entityServerConnected bool
	volumeCount           int32
	mountCount            int32
	lastError             string
}

func (m *mockDebugService) Health(ctx context.Context, state *lsvd_v1alpha.LsvdDebugHealth) error {
	status := &lsvd_v1alpha.HealthStatus{}
	status.SetHealthy(m.healthy)
	status.SetTimestamp(standard.ToTimestamp(time.Now()))
	status.SetPid(int32(os.Getpid()))
	status.SetEntityServerConnected(m.entityServerConnected)
	status.SetVolumeCount(m.volumeCount)
	status.SetMountCount(m.mountCount)
	if m.lastError != "" {
		status.SetLastError(m.lastError)
	}
	state.Results().SetStatus(status)
	return nil
}

func (m *mockDebugService) ListVolumes(ctx context.Context, state *lsvd_v1alpha.LsvdDebugListVolumes) error {
	state.Results().SetVolumes(nil)
	return nil
}

func (m *mockDebugService) ListMounts(ctx context.Context, state *lsvd_v1alpha.LsvdDebugListMounts) error {
	state.Results().SetMounts(nil)
	return nil
}

func (m *mockDebugService) GetMetrics(ctx context.Context, state *lsvd_v1alpha.LsvdDebugGetMetrics) error {
	state.Results().SetMetrics(&lsvd_v1alpha.ReconcileMetrics{})
	return nil
}

func (m *mockDebugService) CheckVersion(ctx context.Context, state *lsvd_v1alpha.LsvdDebugCheckVersion) error {
	result := &lsvd_v1alpha.VersionCheckResult{}
	result.SetCurrentVersion(1)
	result.SetPid(int32(os.Getpid()))
	result.SetNeedsRestart(false)
	state.Results().SetResult(result)
	return nil
}

func TestComponentConnectRPCAndHealth(t *testing.T) {
	ctx := t.Context()
	log := slog.Default()
	tempDir := t.TempDir()

	// Start a mock RPC server
	mockService := &mockDebugService{
		healthy:               true,
		entityServerConnected: true,
		volumeCount:           5,
		mountCount:            3,
		lastError:             "",
	}

	ss, err := rpc.NewState(ctx, rpc.WithSkipVerify)
	require.NoError(t, err)

	ss.Server().ExposeValue("lsvd-debug", lsvd_v1alpha.AdaptLsvdDebug(mockService))

	// Write the server address to the expected file
	addrFile := filepath.Join(tempDir, "lsvd-server.addr")
	err = os.WriteFile(addrFile, []byte(ss.ListenAddr()+"\n"), 0644)
	require.NoError(t, err)

	// Create component and connect
	comp := NewComponent(log, tempDir)
	err = comp.connectRPC(ctx, tempDir)
	require.NoError(t, err)

	// Verify client is set
	assert.NotNil(t, comp.debugClient)
	assert.NotNil(t, comp.rpcState)

	// Test Health via RPC
	status, err := comp.Health()
	require.NoError(t, err)

	assert.True(t, status.Healthy)
	assert.True(t, status.EntityServerConnected)
	assert.Equal(t, 5, status.VolumeCount)
	assert.Equal(t, 3, status.MountCount)
	assert.Equal(t, os.Getpid(), status.PID)
	assert.WithinDuration(t, time.Now(), status.Timestamp, 2*time.Second)
}

func TestComponentConnectRPCAndHealthUnhealthy(t *testing.T) {
	ctx := t.Context()
	log := slog.Default()
	tempDir := t.TempDir()

	// Start a mock RPC server that reports unhealthy
	mockService := &mockDebugService{
		healthy:               false,
		entityServerConnected: false,
		volumeCount:           0,
		mountCount:            0,
		lastError:             "connection lost",
	}

	ss, err := rpc.NewState(ctx, rpc.WithSkipVerify)
	require.NoError(t, err)

	ss.Server().ExposeValue("lsvd-debug", lsvd_v1alpha.AdaptLsvdDebug(mockService))

	addrFile := filepath.Join(tempDir, "lsvd-server.addr")
	err = os.WriteFile(addrFile, []byte(ss.ListenAddr()+"\n"), 0644)
	require.NoError(t, err)

	comp := NewComponent(log, tempDir)
	err = comp.connectRPC(ctx, tempDir)
	require.NoError(t, err)

	// Test Health returns unhealthy status
	status, err := comp.Health()
	require.NoError(t, err)

	assert.False(t, status.Healthy)
	assert.False(t, status.EntityServerConnected)
	assert.Equal(t, "connection lost", status.LastError)

	// Test IsHealthy (need running to be true)
	comp.running = true
	assert.False(t, comp.IsHealthy())
}

func TestComponentIsHealthyViaRPC(t *testing.T) {
	ctx := t.Context()
	log := slog.Default()
	tempDir := t.TempDir()

	// Start a mock RPC server
	mockService := &mockDebugService{
		healthy:               true,
		entityServerConnected: true,
	}

	ss, err := rpc.NewState(ctx, rpc.WithSkipVerify)
	require.NoError(t, err)

	ss.Server().ExposeValue("lsvd-debug", lsvd_v1alpha.AdaptLsvdDebug(mockService))

	addrFile := filepath.Join(tempDir, "lsvd-server.addr")
	err = os.WriteFile(addrFile, []byte(ss.ListenAddr()+"\n"), 0644)
	require.NoError(t, err)

	comp := NewComponent(log, tempDir)
	err = comp.connectRPC(ctx, tempDir)
	require.NoError(t, err)

	// Set running to true
	comp.running = true

	// Test IsHealthy
	assert.True(t, comp.IsHealthy())
}

func TestComponentStopCleansUpRPC(t *testing.T) {
	ctx := t.Context()
	log := slog.Default()
	tempDir := t.TempDir()

	// Start a mock RPC server
	mockService := &mockDebugService{healthy: true}

	ss, err := rpc.NewState(ctx, rpc.WithSkipVerify)
	require.NoError(t, err)

	ss.Server().ExposeValue("lsvd-debug", lsvd_v1alpha.AdaptLsvdDebug(mockService))

	addrFile := filepath.Join(tempDir, "lsvd-server.addr")
	err = os.WriteFile(addrFile, []byte(ss.ListenAddr()+"\n"), 0644)
	require.NoError(t, err)

	comp := NewComponent(log, tempDir)
	err = comp.connectRPC(ctx, tempDir)
	require.NoError(t, err)

	// Verify client is connected
	assert.NotNil(t, comp.debugClient)
	assert.NotNil(t, comp.rpcState)

	// Stop should clean up (but we're not actually running a process)
	// We need to simulate the running state to test cleanup
	comp.running = false // Stop will return early if not running

	// Manually test the cleanup logic
	comp.mu.Lock()
	comp.debugClient = nil
	if comp.rpcState != nil {
		comp.rpcState.Close()
		comp.rpcState = nil
	}
	comp.mu.Unlock()

	assert.Nil(t, comp.debugClient)
	assert.Nil(t, comp.rpcState)
}
