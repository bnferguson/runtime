package lsvd_v1alpha

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/fxamacker/cbor/v2"
	rpc "miren.dev/runtime/pkg/rpc"
	"miren.dev/runtime/pkg/rpc/standard"
)

type volumeInfoData struct {
	EntityId   *string `cbor:"0,keyasint,omitempty" json:"entity_id,omitempty"`
	VolumeId   *string `cbor:"1,keyasint,omitempty" json:"volume_id,omitempty"`
	DiskPath   *string `cbor:"2,keyasint,omitempty" json:"disk_path,omitempty"`
	SizeBytes  *int64  `cbor:"3,keyasint,omitempty" json:"size_bytes,omitempty"`
	Filesystem *string `cbor:"4,keyasint,omitempty" json:"filesystem,omitempty"`
	RemoteOnly *bool   `cbor:"5,keyasint,omitempty" json:"remote_only,omitempty"`
}

type VolumeInfo struct {
	data volumeInfoData
}

func (v *VolumeInfo) HasEntityId() bool {
	return v.data.EntityId != nil
}

func (v *VolumeInfo) EntityId() string {
	if v.data.EntityId == nil {
		return ""
	}
	return *v.data.EntityId
}

func (v *VolumeInfo) SetEntityId(entity_id string) {
	v.data.EntityId = &entity_id
}

func (v *VolumeInfo) HasVolumeId() bool {
	return v.data.VolumeId != nil
}

func (v *VolumeInfo) VolumeId() string {
	if v.data.VolumeId == nil {
		return ""
	}
	return *v.data.VolumeId
}

func (v *VolumeInfo) SetVolumeId(volume_id string) {
	v.data.VolumeId = &volume_id
}

func (v *VolumeInfo) HasDiskPath() bool {
	return v.data.DiskPath != nil
}

func (v *VolumeInfo) DiskPath() string {
	if v.data.DiskPath == nil {
		return ""
	}
	return *v.data.DiskPath
}

func (v *VolumeInfo) SetDiskPath(disk_path string) {
	v.data.DiskPath = &disk_path
}

func (v *VolumeInfo) HasSizeBytes() bool {
	return v.data.SizeBytes != nil
}

func (v *VolumeInfo) SizeBytes() int64 {
	if v.data.SizeBytes == nil {
		return 0
	}
	return *v.data.SizeBytes
}

func (v *VolumeInfo) SetSizeBytes(size_bytes int64) {
	v.data.SizeBytes = &size_bytes
}

func (v *VolumeInfo) HasFilesystem() bool {
	return v.data.Filesystem != nil
}

func (v *VolumeInfo) Filesystem() string {
	if v.data.Filesystem == nil {
		return ""
	}
	return *v.data.Filesystem
}

func (v *VolumeInfo) SetFilesystem(filesystem string) {
	v.data.Filesystem = &filesystem
}

func (v *VolumeInfo) HasRemoteOnly() bool {
	return v.data.RemoteOnly != nil
}

func (v *VolumeInfo) RemoteOnly() bool {
	if v.data.RemoteOnly == nil {
		return false
	}
	return *v.data.RemoteOnly
}

func (v *VolumeInfo) SetRemoteOnly(remote_only bool) {
	v.data.RemoteOnly = &remote_only
}

func (v *VolumeInfo) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(v.data)
}

func (v *VolumeInfo) UnmarshalCBOR(data []byte) error {
	return cbor.Unmarshal(data, &v.data)
}

func (v *VolumeInfo) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.data)
}

func (v *VolumeInfo) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &v.data)
}

type mountInfoData struct {
	EntityId   *string `cbor:"0,keyasint,omitempty" json:"entity_id,omitempty"`
	VolumeId   *string `cbor:"1,keyasint,omitempty" json:"volume_id,omitempty"`
	NbdIndex   *uint32 `cbor:"2,keyasint,omitempty" json:"nbd_index,omitempty"`
	DevicePath *string `cbor:"3,keyasint,omitempty" json:"device_path,omitempty"`
	MountPath  *string `cbor:"4,keyasint,omitempty" json:"mount_path,omitempty"`
	Mounted    *bool   `cbor:"5,keyasint,omitempty" json:"mounted,omitempty"`
	ReadOnly   *bool   `cbor:"6,keyasint,omitempty" json:"read_only,omitempty"`
	LeaseNonce *string `cbor:"7,keyasint,omitempty" json:"lease_nonce,omitempty"`
}

type MountInfo struct {
	data mountInfoData
}

func (v *MountInfo) HasEntityId() bool {
	return v.data.EntityId != nil
}

func (v *MountInfo) EntityId() string {
	if v.data.EntityId == nil {
		return ""
	}
	return *v.data.EntityId
}

func (v *MountInfo) SetEntityId(entity_id string) {
	v.data.EntityId = &entity_id
}

func (v *MountInfo) HasVolumeId() bool {
	return v.data.VolumeId != nil
}

func (v *MountInfo) VolumeId() string {
	if v.data.VolumeId == nil {
		return ""
	}
	return *v.data.VolumeId
}

func (v *MountInfo) SetVolumeId(volume_id string) {
	v.data.VolumeId = &volume_id
}

func (v *MountInfo) HasNbdIndex() bool {
	return v.data.NbdIndex != nil
}

func (v *MountInfo) NbdIndex() uint32 {
	if v.data.NbdIndex == nil {
		return 0
	}
	return *v.data.NbdIndex
}

func (v *MountInfo) SetNbdIndex(nbd_index uint32) {
	v.data.NbdIndex = &nbd_index
}

func (v *MountInfo) HasDevicePath() bool {
	return v.data.DevicePath != nil
}

func (v *MountInfo) DevicePath() string {
	if v.data.DevicePath == nil {
		return ""
	}
	return *v.data.DevicePath
}

func (v *MountInfo) SetDevicePath(device_path string) {
	v.data.DevicePath = &device_path
}

func (v *MountInfo) HasMountPath() bool {
	return v.data.MountPath != nil
}

func (v *MountInfo) MountPath() string {
	if v.data.MountPath == nil {
		return ""
	}
	return *v.data.MountPath
}

func (v *MountInfo) SetMountPath(mount_path string) {
	v.data.MountPath = &mount_path
}

func (v *MountInfo) HasMounted() bool {
	return v.data.Mounted != nil
}

func (v *MountInfo) Mounted() bool {
	if v.data.Mounted == nil {
		return false
	}
	return *v.data.Mounted
}

func (v *MountInfo) SetMounted(mounted bool) {
	v.data.Mounted = &mounted
}

func (v *MountInfo) HasReadOnly() bool {
	return v.data.ReadOnly != nil
}

func (v *MountInfo) ReadOnly() bool {
	if v.data.ReadOnly == nil {
		return false
	}
	return *v.data.ReadOnly
}

func (v *MountInfo) SetReadOnly(read_only bool) {
	v.data.ReadOnly = &read_only
}

func (v *MountInfo) HasLeaseNonce() bool {
	return v.data.LeaseNonce != nil
}

func (v *MountInfo) LeaseNonce() string {
	if v.data.LeaseNonce == nil {
		return ""
	}
	return *v.data.LeaseNonce
}

func (v *MountInfo) SetLeaseNonce(lease_nonce string) {
	v.data.LeaseNonce = &lease_nonce
}

func (v *MountInfo) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(v.data)
}

func (v *MountInfo) UnmarshalCBOR(data []byte) error {
	return cbor.Unmarshal(data, &v.data)
}

func (v *MountInfo) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.data)
}

func (v *MountInfo) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &v.data)
}

type reconcileMetricsData struct {
	VolumeReconcileCount *int64             `cbor:"0,keyasint,omitempty" json:"volume_reconcile_count,omitempty"`
	MountReconcileCount  *int64             `cbor:"1,keyasint,omitempty" json:"mount_reconcile_count,omitempty"`
	VolumeErrorCount     *int64             `cbor:"2,keyasint,omitempty" json:"volume_error_count,omitempty"`
	MountErrorCount      *int64             `cbor:"3,keyasint,omitempty" json:"mount_error_count,omitempty"`
	LastVolumeDuration   *standard.Duration `cbor:"4,keyasint,omitempty" json:"last_volume_duration,omitempty"`
	LastMountDuration    *standard.Duration `cbor:"5,keyasint,omitempty" json:"last_mount_duration,omitempty"`
}

type ReconcileMetrics struct {
	data reconcileMetricsData
}

func (v *ReconcileMetrics) HasVolumeReconcileCount() bool {
	return v.data.VolumeReconcileCount != nil
}

func (v *ReconcileMetrics) VolumeReconcileCount() int64 {
	if v.data.VolumeReconcileCount == nil {
		return 0
	}
	return *v.data.VolumeReconcileCount
}

func (v *ReconcileMetrics) SetVolumeReconcileCount(volume_reconcile_count int64) {
	v.data.VolumeReconcileCount = &volume_reconcile_count
}

func (v *ReconcileMetrics) HasMountReconcileCount() bool {
	return v.data.MountReconcileCount != nil
}

func (v *ReconcileMetrics) MountReconcileCount() int64 {
	if v.data.MountReconcileCount == nil {
		return 0
	}
	return *v.data.MountReconcileCount
}

func (v *ReconcileMetrics) SetMountReconcileCount(mount_reconcile_count int64) {
	v.data.MountReconcileCount = &mount_reconcile_count
}

func (v *ReconcileMetrics) HasVolumeErrorCount() bool {
	return v.data.VolumeErrorCount != nil
}

func (v *ReconcileMetrics) VolumeErrorCount() int64 {
	if v.data.VolumeErrorCount == nil {
		return 0
	}
	return *v.data.VolumeErrorCount
}

func (v *ReconcileMetrics) SetVolumeErrorCount(volume_error_count int64) {
	v.data.VolumeErrorCount = &volume_error_count
}

func (v *ReconcileMetrics) HasMountErrorCount() bool {
	return v.data.MountErrorCount != nil
}

func (v *ReconcileMetrics) MountErrorCount() int64 {
	if v.data.MountErrorCount == nil {
		return 0
	}
	return *v.data.MountErrorCount
}

func (v *ReconcileMetrics) SetMountErrorCount(mount_error_count int64) {
	v.data.MountErrorCount = &mount_error_count
}

func (v *ReconcileMetrics) HasLastVolumeDuration() bool {
	return v.data.LastVolumeDuration != nil
}

func (v *ReconcileMetrics) LastVolumeDuration() *standard.Duration {
	return v.data.LastVolumeDuration
}

func (v *ReconcileMetrics) SetLastVolumeDuration(last_volume_duration *standard.Duration) {
	v.data.LastVolumeDuration = last_volume_duration
}

func (v *ReconcileMetrics) HasLastMountDuration() bool {
	return v.data.LastMountDuration != nil
}

func (v *ReconcileMetrics) LastMountDuration() *standard.Duration {
	return v.data.LastMountDuration
}

func (v *ReconcileMetrics) SetLastMountDuration(last_mount_duration *standard.Duration) {
	v.data.LastMountDuration = last_mount_duration
}

func (v *ReconcileMetrics) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(v.data)
}

func (v *ReconcileMetrics) UnmarshalCBOR(data []byte) error {
	return cbor.Unmarshal(data, &v.data)
}

func (v *ReconcileMetrics) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.data)
}

func (v *ReconcileMetrics) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &v.data)
}

type lsvdDebugListVolumesArgsData struct{}

type LsvdDebugListVolumesArgs struct {
	call rpc.Call
	data lsvdDebugListVolumesArgsData
}

func (v *LsvdDebugListVolumesArgs) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(v.data)
}

func (v *LsvdDebugListVolumesArgs) UnmarshalCBOR(data []byte) error {
	return cbor.Unmarshal(data, &v.data)
}

func (v *LsvdDebugListVolumesArgs) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.data)
}

func (v *LsvdDebugListVolumesArgs) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &v.data)
}

type lsvdDebugListVolumesResultsData struct {
	Volumes *[]*VolumeInfo `cbor:"0,keyasint,omitempty" json:"volumes,omitempty"`
}

type LsvdDebugListVolumesResults struct {
	call rpc.Call
	data lsvdDebugListVolumesResultsData
}

func (v *LsvdDebugListVolumesResults) SetVolumes(volumes []*VolumeInfo) {
	x := slices.Clone(volumes)
	v.data.Volumes = &x
}

func (v *LsvdDebugListVolumesResults) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(v.data)
}

func (v *LsvdDebugListVolumesResults) UnmarshalCBOR(data []byte) error {
	return cbor.Unmarshal(data, &v.data)
}

func (v *LsvdDebugListVolumesResults) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.data)
}

func (v *LsvdDebugListVolumesResults) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &v.data)
}

type lsvdDebugListMountsArgsData struct{}

type LsvdDebugListMountsArgs struct {
	call rpc.Call
	data lsvdDebugListMountsArgsData
}

func (v *LsvdDebugListMountsArgs) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(v.data)
}

func (v *LsvdDebugListMountsArgs) UnmarshalCBOR(data []byte) error {
	return cbor.Unmarshal(data, &v.data)
}

func (v *LsvdDebugListMountsArgs) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.data)
}

func (v *LsvdDebugListMountsArgs) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &v.data)
}

type lsvdDebugListMountsResultsData struct {
	Mounts *[]*MountInfo `cbor:"0,keyasint,omitempty" json:"mounts,omitempty"`
}

type LsvdDebugListMountsResults struct {
	call rpc.Call
	data lsvdDebugListMountsResultsData
}

func (v *LsvdDebugListMountsResults) SetMounts(mounts []*MountInfo) {
	x := slices.Clone(mounts)
	v.data.Mounts = &x
}

func (v *LsvdDebugListMountsResults) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(v.data)
}

func (v *LsvdDebugListMountsResults) UnmarshalCBOR(data []byte) error {
	return cbor.Unmarshal(data, &v.data)
}

func (v *LsvdDebugListMountsResults) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.data)
}

func (v *LsvdDebugListMountsResults) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &v.data)
}

type lsvdDebugGetMetricsArgsData struct{}

type LsvdDebugGetMetricsArgs struct {
	call rpc.Call
	data lsvdDebugGetMetricsArgsData
}

func (v *LsvdDebugGetMetricsArgs) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(v.data)
}

func (v *LsvdDebugGetMetricsArgs) UnmarshalCBOR(data []byte) error {
	return cbor.Unmarshal(data, &v.data)
}

func (v *LsvdDebugGetMetricsArgs) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.data)
}

func (v *LsvdDebugGetMetricsArgs) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &v.data)
}

type lsvdDebugGetMetricsResultsData struct {
	Metrics *ReconcileMetrics `cbor:"0,keyasint,omitempty" json:"metrics,omitempty"`
}

type LsvdDebugGetMetricsResults struct {
	call rpc.Call
	data lsvdDebugGetMetricsResultsData
}

func (v *LsvdDebugGetMetricsResults) SetMetrics(metrics *ReconcileMetrics) {
	v.data.Metrics = metrics
}

func (v *LsvdDebugGetMetricsResults) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(v.data)
}

func (v *LsvdDebugGetMetricsResults) UnmarshalCBOR(data []byte) error {
	return cbor.Unmarshal(data, &v.data)
}

func (v *LsvdDebugGetMetricsResults) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.data)
}

func (v *LsvdDebugGetMetricsResults) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &v.data)
}

type LsvdDebugListVolumes struct {
	rpc.Call
	args    LsvdDebugListVolumesArgs
	results LsvdDebugListVolumesResults
}

func (t *LsvdDebugListVolumes) Args() *LsvdDebugListVolumesArgs {
	args := &t.args
	if args.call != nil {
		return args
	}
	args.call = t.Call
	t.Call.Args(args)
	return args
}

func (t *LsvdDebugListVolumes) Results() *LsvdDebugListVolumesResults {
	results := &t.results
	if results.call != nil {
		return results
	}
	results.call = t.Call
	t.Call.Results(results)
	return results
}

type LsvdDebugListMounts struct {
	rpc.Call
	args    LsvdDebugListMountsArgs
	results LsvdDebugListMountsResults
}

func (t *LsvdDebugListMounts) Args() *LsvdDebugListMountsArgs {
	args := &t.args
	if args.call != nil {
		return args
	}
	args.call = t.Call
	t.Call.Args(args)
	return args
}

func (t *LsvdDebugListMounts) Results() *LsvdDebugListMountsResults {
	results := &t.results
	if results.call != nil {
		return results
	}
	results.call = t.Call
	t.Call.Results(results)
	return results
}

type LsvdDebugGetMetrics struct {
	rpc.Call
	args    LsvdDebugGetMetricsArgs
	results LsvdDebugGetMetricsResults
}

func (t *LsvdDebugGetMetrics) Args() *LsvdDebugGetMetricsArgs {
	args := &t.args
	if args.call != nil {
		return args
	}
	args.call = t.Call
	t.Call.Args(args)
	return args
}

func (t *LsvdDebugGetMetrics) Results() *LsvdDebugGetMetricsResults {
	results := &t.results
	if results.call != nil {
		return results
	}
	results.call = t.Call
	t.Call.Results(results)
	return results
}

type LsvdDebug interface {
	ListVolumes(ctx context.Context, state *LsvdDebugListVolumes) error
	ListMounts(ctx context.Context, state *LsvdDebugListMounts) error
	GetMetrics(ctx context.Context, state *LsvdDebugGetMetrics) error
}

type reexportLsvdDebug struct {
	client rpc.Client
}

func (reexportLsvdDebug) ListVolumes(ctx context.Context, state *LsvdDebugListVolumes) error {
	panic("not implemented")
}

func (reexportLsvdDebug) ListMounts(ctx context.Context, state *LsvdDebugListMounts) error {
	panic("not implemented")
}

func (reexportLsvdDebug) GetMetrics(ctx context.Context, state *LsvdDebugGetMetrics) error {
	panic("not implemented")
}

func (t reexportLsvdDebug) CapabilityClient() rpc.Client {
	return t.client
}

func AdaptLsvdDebug(t LsvdDebug) *rpc.Interface {
	methods := []rpc.Method{
		{
			Name:          "listVolumes",
			InterfaceName: "LsvdDebug",
			Index:         0,
			Public:        false,
			Handler: func(ctx context.Context, call rpc.Call) error {
				return t.ListVolumes(ctx, &LsvdDebugListVolumes{Call: call})
			},
		},
		{
			Name:          "listMounts",
			InterfaceName: "LsvdDebug",
			Index:         0,
			Public:        false,
			Handler: func(ctx context.Context, call rpc.Call) error {
				return t.ListMounts(ctx, &LsvdDebugListMounts{Call: call})
			},
		},
		{
			Name:          "getMetrics",
			InterfaceName: "LsvdDebug",
			Index:         0,
			Public:        false,
			Handler: func(ctx context.Context, call rpc.Call) error {
				return t.GetMetrics(ctx, &LsvdDebugGetMetrics{Call: call})
			},
		},
	}

	return rpc.NewInterface(methods, t)
}

type LsvdDebugClient struct {
	rpc.Client
}

func NewLsvdDebugClient(client rpc.Client) *LsvdDebugClient {
	return &LsvdDebugClient{Client: client}
}

func (c LsvdDebugClient) Export() LsvdDebug {
	return reexportLsvdDebug{client: c.Client}
}

type LsvdDebugClientListVolumesResults struct {
	client rpc.Client
	data   lsvdDebugListVolumesResultsData
}

func (v *LsvdDebugClientListVolumesResults) HasVolumes() bool {
	return v.data.Volumes != nil
}

func (v *LsvdDebugClientListVolumesResults) Volumes() []*VolumeInfo {
	if v.data.Volumes == nil {
		return nil
	}
	return *v.data.Volumes
}

func (v LsvdDebugClient) ListVolumes(ctx context.Context) (*LsvdDebugClientListVolumesResults, error) {
	args := LsvdDebugListVolumesArgs{}

	var ret lsvdDebugListVolumesResultsData

	err := v.Call(ctx, "listVolumes", &args, &ret)
	if err != nil {
		return nil, err
	}

	return &LsvdDebugClientListVolumesResults{client: v.Client, data: ret}, nil
}

type LsvdDebugClientListMountsResults struct {
	client rpc.Client
	data   lsvdDebugListMountsResultsData
}

func (v *LsvdDebugClientListMountsResults) HasMounts() bool {
	return v.data.Mounts != nil
}

func (v *LsvdDebugClientListMountsResults) Mounts() []*MountInfo {
	if v.data.Mounts == nil {
		return nil
	}
	return *v.data.Mounts
}

func (v LsvdDebugClient) ListMounts(ctx context.Context) (*LsvdDebugClientListMountsResults, error) {
	args := LsvdDebugListMountsArgs{}

	var ret lsvdDebugListMountsResultsData

	err := v.Call(ctx, "listMounts", &args, &ret)
	if err != nil {
		return nil, err
	}

	return &LsvdDebugClientListMountsResults{client: v.Client, data: ret}, nil
}

type LsvdDebugClientGetMetricsResults struct {
	client rpc.Client
	data   lsvdDebugGetMetricsResultsData
}

func (v *LsvdDebugClientGetMetricsResults) HasMetrics() bool {
	return v.data.Metrics != nil
}

func (v *LsvdDebugClientGetMetricsResults) Metrics() *ReconcileMetrics {
	return v.data.Metrics
}

func (v LsvdDebugClient) GetMetrics(ctx context.Context) (*LsvdDebugClientGetMetricsResults, error) {
	args := LsvdDebugGetMetricsArgs{}

	var ret lsvdDebugGetMetricsResultsData

	err := v.Call(ctx, "getMetrics", &args, &ret)
	if err != nil {
		return nil, err
	}

	return &LsvdDebugClientGetMetricsResults{client: v.Client, data: ret}, nil
}
