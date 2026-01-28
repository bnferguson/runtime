package server

import (
	"context"

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
