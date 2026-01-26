package server

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"miren.dev/runtime/api/lsvd/lsvd_v1alpha"
	"miren.dev/runtime/pkg/rpc"
	"miren.dev/runtime/pkg/rpc/standard"
)

func TestDebugServiceHealth(t *testing.T) {
	ctx := t.Context()

	// Create a minimal server with test state
	server := &Server{
		log:      slog.Default(),
		dataPath: t.TempDir(),
		state:    NewState(),
	}

	// Set up some test state
	server.entityServerConnected = true
	server.lastVolumeReconcile = time.Now().Add(-5 * time.Minute)
	server.lastMountReconcile = time.Now().Add(-2 * time.Minute)
	server.lastError = "test error"

	// Add some volumes and mounts
	server.state.SetVolume("vol1", &VolumeState{EntityId: "vol1", VolumeId: "v1"})
	server.state.SetVolume("vol2", &VolumeState{EntityId: "vol2", VolumeId: "v2"})
	server.state.SetMount("mnt1", &MountState{EntityId: "mnt1", VolumeId: "vol1"})

	// Create debug service and expose via RPC
	debugService := NewDebugService(server)

	ss, err := rpc.NewState(ctx, rpc.WithSkipVerify)
	require.NoError(t, err)

	ss.Server().ExposeValue("lsvd-debug", lsvd_v1alpha.AdaptLsvdDebug(debugService))

	// Create client
	cs, err := rpc.NewState(ctx, rpc.WithSkipVerify)
	require.NoError(t, err)

	c, err := cs.Connect(ss.ListenAddr(), "lsvd-debug")
	require.NoError(t, err)

	client := lsvd_v1alpha.NewLsvdDebugClient(c)

	// Call Health
	result, err := client.Health(ctx)
	require.NoError(t, err)

	status := result.Status()
	require.NotNil(t, status)

	assert.True(t, status.Healthy())
	assert.True(t, status.EntityServerConnected())
	assert.Equal(t, int32(2), status.VolumeCount())
	assert.Equal(t, int32(1), status.MountCount())
	assert.Equal(t, int32(os.Getpid()), status.Pid())
	assert.Equal(t, "test error", status.LastError())

	// Verify timestamp is recent
	ts := standard.FromTimestamp(status.Timestamp())
	assert.WithinDuration(t, time.Now(), ts, 2*time.Second)

	// Verify reconcile times are present
	assert.True(t, status.HasLastVolumeReconcile())
	assert.True(t, status.HasLastMountReconcile())
}

func TestDebugServiceHealthUnhealthy(t *testing.T) {
	ctx := t.Context()

	// Create a server that is not connected to entity server
	server := &Server{
		log:      slog.Default(),
		dataPath: t.TempDir(),
		state:    NewState(),
	}
	server.entityServerConnected = false

	debugService := NewDebugService(server)

	ss, err := rpc.NewState(ctx, rpc.WithSkipVerify)
	require.NoError(t, err)

	ss.Server().ExposeValue("lsvd-debug", lsvd_v1alpha.AdaptLsvdDebug(debugService))

	cs, err := rpc.NewState(ctx, rpc.WithSkipVerify)
	require.NoError(t, err)

	c, err := cs.Connect(ss.ListenAddr(), "lsvd-debug")
	require.NoError(t, err)

	client := lsvd_v1alpha.NewLsvdDebugClient(c)

	result, err := client.Health(ctx)
	require.NoError(t, err)

	status := result.Status()
	require.NotNil(t, status)

	assert.False(t, status.Healthy())
	assert.False(t, status.EntityServerConnected())
}

func TestDebugServiceListVolumes(t *testing.T) {
	ctx := t.Context()

	server := &Server{
		log:      slog.Default(),
		dataPath: t.TempDir(),
		state:    NewState(),
	}

	// Add test volumes
	server.state.SetVolume("vol1", &VolumeState{
		EntityId:   "vol1",
		VolumeId:   "volume-001",
		DiskPath:   "/data/vol1",
		SizeBytes:  1024 * 1024 * 1024, // 1GB
		Filesystem: "ext4",
		RemoteOnly: false,
	})
	server.state.SetVolume("vol2", &VolumeState{
		EntityId:   "vol2",
		VolumeId:   "volume-002",
		DiskPath:   "/data/vol2",
		SizeBytes:  2 * 1024 * 1024 * 1024, // 2GB
		Filesystem: "xfs",
		RemoteOnly: true,
	})

	debugService := NewDebugService(server)

	ss, err := rpc.NewState(ctx, rpc.WithSkipVerify)
	require.NoError(t, err)

	ss.Server().ExposeValue("lsvd-debug", lsvd_v1alpha.AdaptLsvdDebug(debugService))

	cs, err := rpc.NewState(ctx, rpc.WithSkipVerify)
	require.NoError(t, err)

	c, err := cs.Connect(ss.ListenAddr(), "lsvd-debug")
	require.NoError(t, err)

	client := lsvd_v1alpha.NewLsvdDebugClient(c)

	result, err := client.ListVolumes(ctx)
	require.NoError(t, err)

	volumes := result.Volumes()
	assert.Len(t, volumes, 2)

	// Find volume by entity ID (order not guaranteed from map iteration)
	var vol1, vol2 *lsvd_v1alpha.VolumeInfo
	for _, v := range volumes {
		if v.EntityId() == "vol1" {
			vol1 = v
		} else if v.EntityId() == "vol2" {
			vol2 = v
		}
	}

	require.NotNil(t, vol1)
	assert.Equal(t, "volume-001", vol1.VolumeId())
	assert.Equal(t, "/data/vol1", vol1.DiskPath())
	assert.Equal(t, int64(1024*1024*1024), vol1.SizeBytes())
	assert.Equal(t, "ext4", vol1.Filesystem())
	assert.False(t, vol1.RemoteOnly())

	require.NotNil(t, vol2)
	assert.Equal(t, "volume-002", vol2.VolumeId())
	assert.True(t, vol2.RemoteOnly())
}

func TestDebugServiceListVolumesEmpty(t *testing.T) {
	ctx := t.Context()

	server := &Server{
		log:      slog.Default(),
		dataPath: t.TempDir(),
		state:    NewState(),
	}

	debugService := NewDebugService(server)

	ss, err := rpc.NewState(ctx, rpc.WithSkipVerify)
	require.NoError(t, err)

	ss.Server().ExposeValue("lsvd-debug", lsvd_v1alpha.AdaptLsvdDebug(debugService))

	cs, err := rpc.NewState(ctx, rpc.WithSkipVerify)
	require.NoError(t, err)

	c, err := cs.Connect(ss.ListenAddr(), "lsvd-debug")
	require.NoError(t, err)

	client := lsvd_v1alpha.NewLsvdDebugClient(c)

	result, err := client.ListVolumes(ctx)
	require.NoError(t, err)

	volumes := result.Volumes()
	assert.Empty(t, volumes)
}

func TestDebugServiceListVolumesNilState(t *testing.T) {
	ctx := t.Context()

	server := &Server{
		log:      slog.Default(),
		dataPath: t.TempDir(),
		state:    nil, // nil state
	}

	debugService := NewDebugService(server)

	ss, err := rpc.NewState(ctx, rpc.WithSkipVerify)
	require.NoError(t, err)

	ss.Server().ExposeValue("lsvd-debug", lsvd_v1alpha.AdaptLsvdDebug(debugService))

	cs, err := rpc.NewState(ctx, rpc.WithSkipVerify)
	require.NoError(t, err)

	c, err := cs.Connect(ss.ListenAddr(), "lsvd-debug")
	require.NoError(t, err)

	client := lsvd_v1alpha.NewLsvdDebugClient(c)

	result, err := client.ListVolumes(ctx)
	require.NoError(t, err)

	// Should return nil/empty, not error
	volumes := result.Volumes()
	assert.Nil(t, volumes)
}

func TestDebugServiceListMounts(t *testing.T) {
	ctx := t.Context()

	server := &Server{
		log:      slog.Default(),
		dataPath: t.TempDir(),
		state:    NewState(),
	}

	// Add test mounts
	server.state.SetMount("mnt1", &MountState{
		EntityId:   "mnt1",
		VolumeId:   "vol1",
		NbdIndex:   0,
		DevicePath: "/dev/nbd0",
		MountPath:  "/mnt/vol1",
		Mounted:    true,
		ReadOnly:   false,
		LeaseNonce: "lease-abc-123",
	})
	server.state.SetMount("mnt2", &MountState{
		EntityId:   "mnt2",
		VolumeId:   "vol2",
		NbdIndex:   1,
		DevicePath: "/dev/nbd1",
		MountPath:  "/mnt/vol2",
		Mounted:    false,
		ReadOnly:   true,
		LeaseNonce: "",
	})

	debugService := NewDebugService(server)

	ss, err := rpc.NewState(ctx, rpc.WithSkipVerify)
	require.NoError(t, err)

	ss.Server().ExposeValue("lsvd-debug", lsvd_v1alpha.AdaptLsvdDebug(debugService))

	cs, err := rpc.NewState(ctx, rpc.WithSkipVerify)
	require.NoError(t, err)

	c, err := cs.Connect(ss.ListenAddr(), "lsvd-debug")
	require.NoError(t, err)

	client := lsvd_v1alpha.NewLsvdDebugClient(c)

	result, err := client.ListMounts(ctx)
	require.NoError(t, err)

	mounts := result.Mounts()
	assert.Len(t, mounts, 2)

	// Find mount by entity ID
	var mnt1, mnt2 *lsvd_v1alpha.MountInfo
	for _, m := range mounts {
		if m.EntityId() == "mnt1" {
			mnt1 = m
		} else if m.EntityId() == "mnt2" {
			mnt2 = m
		}
	}

	require.NotNil(t, mnt1)
	assert.Equal(t, "vol1", mnt1.VolumeId())
	assert.Equal(t, uint32(0), mnt1.NbdIndex())
	assert.Equal(t, "/dev/nbd0", mnt1.DevicePath())
	assert.Equal(t, "/mnt/vol1", mnt1.MountPath())
	assert.True(t, mnt1.Mounted())
	assert.False(t, mnt1.ReadOnly())
	assert.Equal(t, "lease-abc-123", mnt1.LeaseNonce())

	require.NotNil(t, mnt2)
	assert.False(t, mnt2.Mounted())
	assert.True(t, mnt2.ReadOnly())
	assert.Equal(t, "", mnt2.LeaseNonce())
}

func TestDebugServiceGetMetrics(t *testing.T) {
	ctx := t.Context()

	server := &Server{
		log:      slog.Default(),
		dataPath: t.TempDir(),
		state:    NewState(),
	}

	// Set up test metrics
	server.volumeReconcileCount = 100
	server.mountReconcileCount = 50
	server.volumeErrorCount = 5
	server.mountErrorCount = 2
	server.lastVolumeDuration = 150 * time.Millisecond
	server.lastMountDuration = 75 * time.Millisecond

	debugService := NewDebugService(server)

	ss, err := rpc.NewState(ctx, rpc.WithSkipVerify)
	require.NoError(t, err)

	ss.Server().ExposeValue("lsvd-debug", lsvd_v1alpha.AdaptLsvdDebug(debugService))

	cs, err := rpc.NewState(ctx, rpc.WithSkipVerify)
	require.NoError(t, err)

	c, err := cs.Connect(ss.ListenAddr(), "lsvd-debug")
	require.NoError(t, err)

	client := lsvd_v1alpha.NewLsvdDebugClient(c)

	result, err := client.GetMetrics(ctx)
	require.NoError(t, err)

	metrics := result.Metrics()
	require.NotNil(t, metrics)

	assert.Equal(t, int64(100), metrics.VolumeReconcileCount())
	assert.Equal(t, int64(50), metrics.MountReconcileCount())
	assert.Equal(t, int64(5), metrics.VolumeErrorCount())
	assert.Equal(t, int64(2), metrics.MountErrorCount())

	// Check durations
	volDur := standard.FromDuration(metrics.LastVolumeDuration())
	assert.Equal(t, 150*time.Millisecond, volDur)

	mntDur := standard.FromDuration(metrics.LastMountDuration())
	assert.Equal(t, 75*time.Millisecond, mntDur)
}

func TestDebugServiceGetMetricsZeroDurations(t *testing.T) {
	ctx := t.Context()

	server := &Server{
		log:      slog.Default(),
		dataPath: t.TempDir(),
		state:    NewState(),
	}

	// Set up metrics with zero durations
	server.volumeReconcileCount = 10
	server.lastVolumeDuration = 0
	server.lastMountDuration = 0

	debugService := NewDebugService(server)

	ss, err := rpc.NewState(ctx, rpc.WithSkipVerify)
	require.NoError(t, err)

	ss.Server().ExposeValue("lsvd-debug", lsvd_v1alpha.AdaptLsvdDebug(debugService))

	cs, err := rpc.NewState(ctx, rpc.WithSkipVerify)
	require.NoError(t, err)

	c, err := cs.Connect(ss.ListenAddr(), "lsvd-debug")
	require.NoError(t, err)

	client := lsvd_v1alpha.NewLsvdDebugClient(c)

	result, err := client.GetMetrics(ctx)
	require.NoError(t, err)

	metrics := result.Metrics()
	require.NotNil(t, metrics)

	// Zero durations should not have values set
	assert.False(t, metrics.HasLastVolumeDuration())
	assert.False(t, metrics.HasLastMountDuration())
}

func TestDebugServiceCheckVersionSameVersion(t *testing.T) {
	ctx := t.Context()

	server := &Server{
		log:      slog.Default(),
		dataPath: t.TempDir(),
		state:    NewState(),
	}

	debugService := NewDebugService(server)

	ss, err := rpc.NewState(ctx, rpc.WithSkipVerify)
	require.NoError(t, err)

	ss.Server().ExposeValue("lsvd-debug", lsvd_v1alpha.AdaptLsvdDebug(debugService))

	cs, err := rpc.NewState(ctx, rpc.WithSkipVerify)
	require.NoError(t, err)

	c, err := cs.Connect(ss.ListenAddr(), "lsvd-debug")
	require.NoError(t, err)

	client := lsvd_v1alpha.NewLsvdDebugClient(c)

	// Check with the same version - should not need restart
	result, err := client.CheckVersion(ctx, ServerVersion)
	require.NoError(t, err)

	versionResult := result.Result()
	require.NotNil(t, versionResult)

	assert.Equal(t, ServerVersion, versionResult.CurrentVersion())
	assert.False(t, versionResult.NeedsRestart())
	assert.Equal(t, int32(os.Getpid()), versionResult.Pid())
}

func TestDebugServiceCheckVersionOlderVersion(t *testing.T) {
	ctx := t.Context()

	server := &Server{
		log:        slog.Default(),
		dataPath:   t.TempDir(),
		state:      NewState(),
		shutdownCh: make(chan struct{}),
	}

	debugService := NewDebugService(server)

	ss, err := rpc.NewState(ctx, rpc.WithSkipVerify)
	require.NoError(t, err)

	ss.Server().ExposeValue("lsvd-debug", lsvd_v1alpha.AdaptLsvdDebug(debugService))

	cs, err := rpc.NewState(ctx, rpc.WithSkipVerify)
	require.NoError(t, err)

	c, err := cs.Connect(ss.ListenAddr(), "lsvd-debug")
	require.NoError(t, err)

	client := lsvd_v1alpha.NewLsvdDebugClient(c)

	// Check with a lower version - should not need restart
	result, err := client.CheckVersion(ctx, ServerVersion-1)
	require.NoError(t, err)

	versionResult := result.Result()
	require.NotNil(t, versionResult)

	assert.Equal(t, ServerVersion, versionResult.CurrentVersion())
	assert.False(t, versionResult.NeedsRestart())
}

func TestDebugServiceCheckVersionNewerVersion(t *testing.T) {
	ctx := t.Context()

	server := &Server{
		log:        slog.Default(),
		dataPath:   t.TempDir(),
		state:      NewState(),
		shutdownCh: make(chan struct{}),
	}

	debugService := NewDebugService(server)

	ss, err := rpc.NewState(ctx, rpc.WithSkipVerify)
	require.NoError(t, err)

	ss.Server().ExposeValue("lsvd-debug", lsvd_v1alpha.AdaptLsvdDebug(debugService))

	cs, err := rpc.NewState(ctx, rpc.WithSkipVerify)
	require.NoError(t, err)

	c, err := cs.Connect(ss.ListenAddr(), "lsvd-debug")
	require.NoError(t, err)

	client := lsvd_v1alpha.NewLsvdDebugClient(c)

	// Check with a higher version - should need restart
	result, err := client.CheckVersion(ctx, ServerVersion+1)
	require.NoError(t, err)

	versionResult := result.Result()
	require.NotNil(t, versionResult)

	assert.Equal(t, ServerVersion, versionResult.CurrentVersion())
	assert.True(t, versionResult.NeedsRestart())
	assert.Equal(t, int32(os.Getpid()), versionResult.Pid())

	// Verify shutdown was triggered (will be signaled after a short delay)
	select {
	case <-server.shutdownCh:
		// Shutdown was triggered
	case <-time.After(500 * time.Millisecond):
		t.Fatal("shutdown was not triggered")
	}
}
