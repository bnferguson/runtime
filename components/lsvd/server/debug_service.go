package server

import (
	"context"
	"os"
	"time"

	"miren.dev/runtime/api/lsvd/lsvd_v1alpha"
	"miren.dev/runtime/pkg/rpc/standard"
)

// DebugService implements the LsvdDebug RPC interface
type DebugService struct {
	server *Server
}

// NewDebugService creates a new debug service
func NewDebugService(server *Server) *DebugService {
	return &DebugService{server: server}
}

// Health returns the current health status
func (d *DebugService) Health(ctx context.Context, state *lsvd_v1alpha.LsvdDebugHealth) error {
	status := &lsvd_v1alpha.HealthStatus{}

	d.server.healthMu.RLock()
	healthy := d.server.entityServerConnected
	entityServerConnected := d.server.entityServerConnected
	lastVolumeReconcile := d.server.lastVolumeReconcile
	lastMountReconcile := d.server.lastMountReconcile
	lastError := d.server.lastError
	d.server.healthMu.RUnlock()

	status.SetHealthy(healthy)
	status.SetTimestamp(standard.ToTimestamp(time.Now()))
	status.SetPid(int32(os.Getpid()))
	status.SetEntityServerConnected(entityServerConnected)

	if d.server.state != nil {
		status.SetVolumeCount(int32(len(d.server.state.Volumes)))
		status.SetMountCount(int32(len(d.server.state.Mounts)))
	}

	if !lastVolumeReconcile.IsZero() {
		status.SetLastVolumeReconcile(standard.ToTimestamp(lastVolumeReconcile))
	}
	if !lastMountReconcile.IsZero() {
		status.SetLastMountReconcile(standard.ToTimestamp(lastMountReconcile))
	}
	if lastError != "" {
		status.SetLastError(lastError)
	}

	state.Results().SetStatus(status)
	return nil
}

// ListVolumes returns all volumes
func (d *DebugService) ListVolumes(ctx context.Context, state *lsvd_v1alpha.LsvdDebugListVolumes) error {
	if d.server.state == nil {
		state.Results().SetVolumes(nil)
		return nil
	}

	volumes := d.server.state.ListVolumes()
	result := make([]*lsvd_v1alpha.VolumeInfo, 0, len(volumes))

	for _, v := range volumes {
		info := &lsvd_v1alpha.VolumeInfo{}
		info.SetEntityId(v.EntityId)
		info.SetVolumeId(v.VolumeId)
		info.SetDiskPath(v.DiskPath)
		info.SetSizeBytes(v.SizeBytes)
		info.SetFilesystem(v.Filesystem)
		info.SetRemoteOnly(v.RemoteOnly)
		result = append(result, info)
	}

	state.Results().SetVolumes(result)
	return nil
}

// ListMounts returns all mounts
func (d *DebugService) ListMounts(ctx context.Context, state *lsvd_v1alpha.LsvdDebugListMounts) error {
	if d.server.state == nil {
		state.Results().SetMounts(nil)
		return nil
	}

	mounts := d.server.state.ListMounts()
	result := make([]*lsvd_v1alpha.MountInfo, 0, len(mounts))

	for _, m := range mounts {
		info := &lsvd_v1alpha.MountInfo{}
		info.SetEntityId(m.EntityId)
		info.SetVolumeId(m.VolumeId)
		info.SetNbdIndex(m.NbdIndex)
		info.SetDevicePath(m.DevicePath)
		info.SetMountPath(m.MountPath)
		info.SetMounted(m.Mounted)
		info.SetReadOnly(m.ReadOnly)
		if m.LeaseNonce != "" {
			info.SetLeaseNonce(m.LeaseNonce)
		}
		result = append(result, info)
	}

	state.Results().SetMounts(result)
	return nil
}

// GetMetrics returns reconciliation metrics
func (d *DebugService) GetMetrics(ctx context.Context, state *lsvd_v1alpha.LsvdDebugGetMetrics) error {
	metrics := &lsvd_v1alpha.ReconcileMetrics{}

	d.server.metricsMu.RLock()
	metrics.SetVolumeReconcileCount(d.server.volumeReconcileCount)
	metrics.SetMountReconcileCount(d.server.mountReconcileCount)
	metrics.SetVolumeErrorCount(d.server.volumeErrorCount)
	metrics.SetMountErrorCount(d.server.mountErrorCount)
	if d.server.lastVolumeDuration > 0 {
		metrics.SetLastVolumeDuration(standard.ToDuration(d.server.lastVolumeDuration))
	}
	if d.server.lastMountDuration > 0 {
		metrics.SetLastMountDuration(standard.ToDuration(d.server.lastMountDuration))
	}
	d.server.metricsMu.RUnlock()

	state.Results().SetMetrics(metrics)
	return nil
}

// CheckVersion checks if the server version matches the expected version.
// If the expected version is greater than the current version, the server
// will initiate a graceful shutdown to allow for an upgrade.
func (d *DebugService) CheckVersion(ctx context.Context, state *lsvd_v1alpha.LsvdDebugCheckVersion) error {
	expectedVersion := state.Args().ExpectedVersion()
	currentVersion := ServerVersion

	result := &lsvd_v1alpha.VersionCheckResult{}
	result.SetCurrentVersion(currentVersion)
	result.SetPid(int32(os.Getpid()))

	if expectedVersion > currentVersion {
		d.server.log.Info("version upgrade requested",
			"current_version", currentVersion,
			"expected_version", expectedVersion,
		)
		result.SetNeedsRestart(true)

		// Trigger graceful shutdown in a goroutine so the RPC can return first
		go func() {
			// Small delay to ensure RPC response is sent
			time.Sleep(100 * time.Millisecond)
			d.server.triggerShutdown()
		}()
	} else {
		result.SetNeedsRestart(false)
	}

	state.Results().SetResult(result)
	return nil
}
