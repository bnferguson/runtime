package storage_v1alpha

import (
	"time"

	entity "miren.dev/runtime/pkg/entity"
	schema "miren.dev/runtime/pkg/entity/schema"
)

const (
	DiskCreatedById          = entity.Id("dev.miren.storage/disk.created_by")
	DiskFilesystemId         = entity.Id("dev.miren.storage/disk.filesystem")
	DiskFilesystemExt4Id     = entity.Id("dev.miren.storage/filesystem.ext4")
	DiskFilesystemXfsId      = entity.Id("dev.miren.storage/filesystem.xfs")
	DiskFilesystemBtrfsId    = entity.Id("dev.miren.storage/filesystem.btrfs")
	DiskLsvdVolumeIdId       = entity.Id("dev.miren.storage/disk.lsvd_volume_id")
	DiskModeId               = entity.Id("dev.miren.storage/disk.mode")
	DiskModeUniversalId      = entity.Id("dev.miren.storage/mode.universal")
	DiskModeAcceleratorId    = entity.Id("dev.miren.storage/mode.accelerator")
	DiskNameId               = entity.Id("dev.miren.storage/disk.name")
	DiskRemoteOnlyId         = entity.Id("dev.miren.storage/disk.remote_only")
	DiskSizeGbId             = entity.Id("dev.miren.storage/disk.size_gb")
	DiskStatusId             = entity.Id("dev.miren.storage/disk.status")
	DiskStatusProvisioningId = entity.Id("dev.miren.storage/status.provisioning")
	DiskStatusProvisionedId  = entity.Id("dev.miren.storage/status.provisioned")
	DiskStatusAttachedId     = entity.Id("dev.miren.storage/status.attached")
	DiskStatusDetachedId     = entity.Id("dev.miren.storage/status.detached")
	DiskStatusDeletingId     = entity.Id("dev.miren.storage/status.deleting")
	DiskStatusErrorId        = entity.Id("dev.miren.storage/status.error")
	DiskVolumeIdId           = entity.Id("dev.miren.storage/disk.volume_id")
)

type Disk struct {
	ID           entity.Id      `json:"id"`
	CreatedBy    entity.Id      `cbor:"created_by,omitempty" json:"created_by,omitempty"`
	Filesystem   DiskFilesystem `cbor:"filesystem,omitempty" json:"filesystem,omitempty"`
	LsvdVolumeId string         `cbor:"lsvd_volume_id,omitempty" json:"lsvd_volume_id,omitempty"`
	Mode         DiskMode       `cbor:"mode,omitempty" json:"mode,omitempty"`
	Name         string         `cbor:"name" json:"name"`
	RemoteOnly   bool           `cbor:"remote_only,omitempty" json:"remote_only,omitempty"`
	SizeGb       int64          `cbor:"size_gb" json:"size_gb"`
	Status       DiskStatus     `cbor:"status,omitempty" json:"status,omitempty"`
	VolumeId     string         `cbor:"volume_id,omitempty" json:"volume_id,omitempty"`
}

type DiskFilesystem string

const (
	EXT4  DiskFilesystem = "filesystem.ext4"
	XFS   DiskFilesystem = "filesystem.xfs"
	BTRFS DiskFilesystem = "filesystem.btrfs"
)

var diskfilesystemFromId = map[entity.Id]DiskFilesystem{DiskFilesystemExt4Id: EXT4, DiskFilesystemXfsId: XFS, DiskFilesystemBtrfsId: BTRFS}
var diskfilesystemToId = map[DiskFilesystem]entity.Id{EXT4: DiskFilesystemExt4Id, XFS: DiskFilesystemXfsId, BTRFS: DiskFilesystemBtrfsId}

type DiskMode string

const (
	UNIVERSAL   DiskMode = "mode.universal"
	ACCELERATOR DiskMode = "mode.accelerator"
)

var diskmodeFromId = map[entity.Id]DiskMode{DiskModeUniversalId: UNIVERSAL, DiskModeAcceleratorId: ACCELERATOR}
var diskmodeToId = map[DiskMode]entity.Id{UNIVERSAL: DiskModeUniversalId, ACCELERATOR: DiskModeAcceleratorId}

type DiskStatus string

const (
	PROVISIONING DiskStatus = "status.provisioning"
	PROVISIONED  DiskStatus = "status.provisioned"
	ATTACHED     DiskStatus = "status.attached"
	DETACHED     DiskStatus = "status.detached"
	DELETING     DiskStatus = "status.deleting"
	ERROR        DiskStatus = "status.error"
)

var diskstatusFromId = map[entity.Id]DiskStatus{DiskStatusProvisioningId: PROVISIONING, DiskStatusProvisionedId: PROVISIONED, DiskStatusAttachedId: ATTACHED, DiskStatusDetachedId: DETACHED, DiskStatusDeletingId: DELETING, DiskStatusErrorId: ERROR}
var diskstatusToId = map[DiskStatus]entity.Id{PROVISIONING: DiskStatusProvisioningId, PROVISIONED: DiskStatusProvisionedId, ATTACHED: DiskStatusAttachedId, DETACHED: DiskStatusDetachedId, DELETING: DiskStatusDeletingId, ERROR: DiskStatusErrorId}

func (o *Disk) Decode(e entity.AttrGetter) {
	o.ID = entity.MustGet(e, entity.DBId).Value.Id()
	if a, ok := e.Get(DiskCreatedById); ok && a.Value.Kind() == entity.KindId {
		o.CreatedBy = a.Value.Id()
	}
	if a, ok := e.Get(DiskFilesystemId); ok && a.Value.Kind() == entity.KindId {
		o.Filesystem = diskfilesystemFromId[a.Value.Id()]
	}
	if a, ok := e.Get(DiskLsvdVolumeIdId); ok && a.Value.Kind() == entity.KindString {
		o.LsvdVolumeId = a.Value.String()
	}
	if a, ok := e.Get(DiskModeId); ok && a.Value.Kind() == entity.KindId {
		o.Mode = diskmodeFromId[a.Value.Id()]
	}
	if a, ok := e.Get(DiskNameId); ok && a.Value.Kind() == entity.KindString {
		o.Name = a.Value.String()
	}
	if a, ok := e.Get(DiskRemoteOnlyId); ok && a.Value.Kind() == entity.KindBool {
		o.RemoteOnly = a.Value.Bool()
	}
	if a, ok := e.Get(DiskSizeGbId); ok && a.Value.Kind() == entity.KindInt64 {
		o.SizeGb = a.Value.Int64()
	}
	if a, ok := e.Get(DiskStatusId); ok && a.Value.Kind() == entity.KindId {
		o.Status = diskstatusFromId[a.Value.Id()]
	}
	if a, ok := e.Get(DiskVolumeIdId); ok && a.Value.Kind() == entity.KindString {
		o.VolumeId = a.Value.String()
	}
}

func (o *Disk) Is(e entity.AttrGetter) bool {
	return entity.Is(e, KindDisk)
}

func (o *Disk) ShortKind() string {
	return "disk"
}

func (o *Disk) Kind() entity.Id {
	return KindDisk
}

func (o *Disk) EntityId() entity.Id {
	return o.ID
}

func (o *Disk) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.CreatedBy) {
		attrs = append(attrs, entity.Ref(DiskCreatedById, o.CreatedBy))
	}
	if a, ok := diskfilesystemToId[o.Filesystem]; ok {
		attrs = append(attrs, entity.Ref(DiskFilesystemId, a))
	}
	if !entity.Empty(o.LsvdVolumeId) {
		attrs = append(attrs, entity.String(DiskLsvdVolumeIdId, o.LsvdVolumeId))
	}
	if a, ok := diskmodeToId[o.Mode]; ok {
		attrs = append(attrs, entity.Ref(DiskModeId, a))
	}
	if !entity.Empty(o.Name) {
		attrs = append(attrs, entity.String(DiskNameId, o.Name))
	}
	attrs = append(attrs, entity.Bool(DiskRemoteOnlyId, o.RemoteOnly))
	attrs = append(attrs, entity.Int64(DiskSizeGbId, o.SizeGb))
	if a, ok := diskstatusToId[o.Status]; ok {
		attrs = append(attrs, entity.Ref(DiskStatusId, a))
	}
	if !entity.Empty(o.VolumeId) {
		attrs = append(attrs, entity.String(DiskVolumeIdId, o.VolumeId))
	}
	attrs = append(attrs, entity.Ref(entity.EntityKind, KindDisk))
	return
}

func (o *Disk) Empty() bool {
	if !entity.Empty(o.CreatedBy) {
		return false
	}
	if o.Filesystem != "" {
		return false
	}
	if !entity.Empty(o.LsvdVolumeId) {
		return false
	}
	if o.Mode != "" {
		return false
	}
	if !entity.Empty(o.Name) {
		return false
	}
	if !entity.Empty(o.RemoteOnly) {
		return false
	}
	if !entity.Empty(o.SizeGb) {
		return false
	}
	if o.Status != "" {
		return false
	}
	if !entity.Empty(o.VolumeId) {
		return false
	}
	return true
}

func (o *Disk) InitSchema(sb *schema.SchemaBuilder) {
	sb.Ref("created_by", "dev.miren.storage/disk.created_by", schema.Doc("Application that created this disk (for tracking purposes)"), schema.Indexed, schema.Tags("dev.miren.app_ref"))
	sb.Singleton("dev.miren.storage/filesystem.ext4")
	sb.Singleton("dev.miren.storage/filesystem.xfs")
	sb.Singleton("dev.miren.storage/filesystem.btrfs")
	sb.Ref("filesystem", "dev.miren.storage/disk.filesystem", schema.Doc("Filesystem type for the disk"), schema.Choices(DiskFilesystemExt4Id, DiskFilesystemXfsId, DiskFilesystemBtrfsId))
	sb.String("lsvd_volume_id", "dev.miren.storage/disk.lsvd_volume_id", schema.Doc("LSVD backend volume identifier"), schema.Indexed)
	sb.Singleton("dev.miren.storage/mode.universal")
	sb.Singleton("dev.miren.storage/mode.accelerator")
	sb.Ref("mode", "dev.miren.storage/disk.mode", schema.Doc("Disk I/O mode"), schema.Indexed, schema.Choices(DiskModeUniversalId, DiskModeAcceleratorId))
	sb.String("name", "dev.miren.storage/disk.name", schema.Doc("Human-readable name for the disk"), schema.Required, schema.Indexed)
	sb.Bool("remote_only", "dev.miren.storage/disk.remote_only", schema.Doc("If true, disk is stored only remotely without local replica"))
	sb.Int64("size_gb", "dev.miren.storage/disk.size_gb", schema.Doc("Storage capacity in gigabytes"), schema.Required)
	sb.Singleton("dev.miren.storage/status.provisioning")
	sb.Singleton("dev.miren.storage/status.provisioned")
	sb.Singleton("dev.miren.storage/status.attached")
	sb.Singleton("dev.miren.storage/status.detached")
	sb.Singleton("dev.miren.storage/status.deleting")
	sb.Singleton("dev.miren.storage/status.error")
	sb.Ref("status", "dev.miren.storage/disk.status", schema.Doc("Current state of the disk"), schema.Indexed, schema.Choices(DiskStatusProvisioningId, DiskStatusProvisionedId, DiskStatusAttachedId, DiskStatusDetachedId, DiskStatusDeletingId, DiskStatusErrorId))
	sb.String("volume_id", "dev.miren.storage/disk.volume_id", schema.Doc("Volume identifier for universal/accelerator mode disks"), schema.Indexed)
}

const (
	DiskLeaseAcquiredAtId     = entity.Id("dev.miren.storage/disk_lease.acquired_at")
	DiskLeaseAppIdId          = entity.Id("dev.miren.storage/disk_lease.app_id")
	DiskLeaseDiskIdId         = entity.Id("dev.miren.storage/disk_lease.disk_id")
	DiskLeaseErrorMessageId   = entity.Id("dev.miren.storage/disk_lease.error_message")
	DiskLeaseMountId          = entity.Id("dev.miren.storage/disk_lease.mount")
	DiskLeaseNodeIdId         = entity.Id("dev.miren.storage/disk_lease.node_id")
	DiskLeaseSandboxIdId      = entity.Id("dev.miren.storage/disk_lease.sandbox_id")
	DiskLeaseStatusId         = entity.Id("dev.miren.storage/disk_lease.status")
	DiskLeaseStatusPendingId  = entity.Id("dev.miren.storage/status.pending")
	DiskLeaseStatusBoundId    = entity.Id("dev.miren.storage/status.bound")
	DiskLeaseStatusFailedId   = entity.Id("dev.miren.storage/status.failed")
	DiskLeaseStatusReleasedId = entity.Id("dev.miren.storage/status.released")
)

type DiskLease struct {
	ID           entity.Id       `json:"id"`
	AcquiredAt   time.Time       `cbor:"acquired_at,omitempty" json:"acquired_at,omitempty"`
	AppId        entity.Id       `cbor:"app_id,omitempty" json:"app_id,omitempty"`
	DiskId       entity.Id       `cbor:"disk_id" json:"disk_id"`
	ErrorMessage string          `cbor:"error_message,omitempty" json:"error_message,omitempty"`
	Mount        Mount           `cbor:"mount,omitempty" json:"mount,omitempty"`
	NodeId       entity.Id       `cbor:"node_id" json:"node_id"`
	SandboxId    entity.Id       `cbor:"sandbox_id,omitempty" json:"sandbox_id,omitempty"`
	Status       DiskLeaseStatus `cbor:"status,omitempty" json:"status,omitempty"`
}

type DiskLeaseStatus string

const (
	PENDING  DiskLeaseStatus = "status.pending"
	BOUND    DiskLeaseStatus = "status.bound"
	FAILED   DiskLeaseStatus = "status.failed"
	RELEASED DiskLeaseStatus = "status.released"
)

var disk_leasestatusFromId = map[entity.Id]DiskLeaseStatus{DiskLeaseStatusPendingId: PENDING, DiskLeaseStatusBoundId: BOUND, DiskLeaseStatusFailedId: FAILED, DiskLeaseStatusReleasedId: RELEASED}
var disk_leasestatusToId = map[DiskLeaseStatus]entity.Id{PENDING: DiskLeaseStatusPendingId, BOUND: DiskLeaseStatusBoundId, FAILED: DiskLeaseStatusFailedId, RELEASED: DiskLeaseStatusReleasedId}

func (o *DiskLease) Decode(e entity.AttrGetter) {
	o.ID = entity.MustGet(e, entity.DBId).Value.Id()
	if a, ok := e.Get(DiskLeaseAcquiredAtId); ok && a.Value.Kind() == entity.KindTime {
		o.AcquiredAt = a.Value.Time()
	}
	if a, ok := e.Get(DiskLeaseAppIdId); ok && a.Value.Kind() == entity.KindId {
		o.AppId = a.Value.Id()
	}
	if a, ok := e.Get(DiskLeaseDiskIdId); ok && a.Value.Kind() == entity.KindId {
		o.DiskId = a.Value.Id()
	}
	if a, ok := e.Get(DiskLeaseErrorMessageId); ok && a.Value.Kind() == entity.KindString {
		o.ErrorMessage = a.Value.String()
	}
	if a, ok := e.Get(DiskLeaseMountId); ok && a.Value.Kind() == entity.KindComponent {
		o.Mount.Decode(a.Value.Component())
	}
	if a, ok := e.Get(DiskLeaseNodeIdId); ok && a.Value.Kind() == entity.KindId {
		o.NodeId = a.Value.Id()
	}
	if a, ok := e.Get(DiskLeaseSandboxIdId); ok && a.Value.Kind() == entity.KindId {
		o.SandboxId = a.Value.Id()
	}
	if a, ok := e.Get(DiskLeaseStatusId); ok && a.Value.Kind() == entity.KindId {
		o.Status = disk_leasestatusFromId[a.Value.Id()]
	}
}

func (o *DiskLease) Is(e entity.AttrGetter) bool {
	return entity.Is(e, KindDiskLease)
}

func (o *DiskLease) ShortKind() string {
	return "disk_lease"
}

func (o *DiskLease) Kind() entity.Id {
	return KindDiskLease
}

func (o *DiskLease) EntityId() entity.Id {
	return o.ID
}

func (o *DiskLease) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.AcquiredAt) {
		attrs = append(attrs, entity.Time(DiskLeaseAcquiredAtId, o.AcquiredAt))
	}
	if !entity.Empty(o.AppId) {
		attrs = append(attrs, entity.Ref(DiskLeaseAppIdId, o.AppId))
	}
	if !entity.Empty(o.DiskId) {
		attrs = append(attrs, entity.Ref(DiskLeaseDiskIdId, o.DiskId))
	}
	if !entity.Empty(o.ErrorMessage) {
		attrs = append(attrs, entity.String(DiskLeaseErrorMessageId, o.ErrorMessage))
	}
	if !o.Mount.Empty() {
		attrs = append(attrs, entity.Component(DiskLeaseMountId, o.Mount.Encode()))
	}
	if !entity.Empty(o.NodeId) {
		attrs = append(attrs, entity.Ref(DiskLeaseNodeIdId, o.NodeId))
	}
	if !entity.Empty(o.SandboxId) {
		attrs = append(attrs, entity.Ref(DiskLeaseSandboxIdId, o.SandboxId))
	}
	if a, ok := disk_leasestatusToId[o.Status]; ok {
		attrs = append(attrs, entity.Ref(DiskLeaseStatusId, a))
	}
	attrs = append(attrs, entity.Ref(entity.EntityKind, KindDiskLease))
	return
}

func (o *DiskLease) Empty() bool {
	if !entity.Empty(o.AcquiredAt) {
		return false
	}
	if !entity.Empty(o.AppId) {
		return false
	}
	if !entity.Empty(o.DiskId) {
		return false
	}
	if !entity.Empty(o.ErrorMessage) {
		return false
	}
	if !o.Mount.Empty() {
		return false
	}
	if !entity.Empty(o.NodeId) {
		return false
	}
	if !entity.Empty(o.SandboxId) {
		return false
	}
	if o.Status != "" {
		return false
	}
	return true
}

func (o *DiskLease) InitSchema(sb *schema.SchemaBuilder) {
	sb.Time("acquired_at", "dev.miren.storage/disk_lease.acquired_at", schema.Doc("When the lease was acquired"))
	sb.Ref("app_id", "dev.miren.storage/disk_lease.app_id", schema.Doc("Reference to the application (for debugging)"), schema.Indexed, schema.Tags("dev.miren.app_ref"))
	sb.Ref("disk_id", "dev.miren.storage/disk_lease.disk_id", schema.Doc("Reference to the leased disk"), schema.Required, schema.Indexed)
	sb.String("error_message", "dev.miren.storage/disk_lease.error_message", schema.Doc("Error details if lease binding failed"))
	sb.Component("mount", "dev.miren.storage/disk_lease.mount", schema.Doc("Mount configuration for the disk"))
	(&Mount{}).InitSchema(sb.Builder("disk_lease.mount"))
	sb.Ref("node_id", "dev.miren.storage/disk_lease.node_id", schema.Doc("Node where the disk is mounted"), schema.Required)
	sb.Ref("sandbox_id", "dev.miren.storage/disk_lease.sandbox_id", schema.Doc("Reference to the sandbox using the disk"), schema.Indexed)
	sb.Singleton("dev.miren.storage/status.pending")
	sb.Singleton("dev.miren.storage/status.bound")
	sb.Singleton("dev.miren.storage/status.failed")
	sb.Singleton("dev.miren.storage/status.released")
	sb.Ref("status", "dev.miren.storage/disk_lease.status", schema.Doc("Current state of the lease"), schema.Indexed, schema.Choices(DiskLeaseStatusPendingId, DiskLeaseStatusBoundId, DiskLeaseStatusFailedId, DiskLeaseStatusReleasedId))
}

const (
	MountOptionsId  = entity.Id("dev.miren.storage/mount.options")
	MountPathId     = entity.Id("dev.miren.storage/mount.path")
	MountReadOnlyId = entity.Id("dev.miren.storage/mount.read_only")
)

type Mount struct {
	Options  string `cbor:"options,omitempty" json:"options,omitempty"`
	Path     string `cbor:"path" json:"path"`
	ReadOnly bool   `cbor:"read_only,omitempty" json:"read_only,omitempty"`
}

func (o *Mount) Decode(e entity.AttrGetter) {
	if a, ok := e.Get(MountOptionsId); ok && a.Value.Kind() == entity.KindString {
		o.Options = a.Value.String()
	}
	if a, ok := e.Get(MountPathId); ok && a.Value.Kind() == entity.KindString {
		o.Path = a.Value.String()
	}
	if a, ok := e.Get(MountReadOnlyId); ok && a.Value.Kind() == entity.KindBool {
		o.ReadOnly = a.Value.Bool()
	}
}

func (o *Mount) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.Options) {
		attrs = append(attrs, entity.String(MountOptionsId, o.Options))
	}
	if !entity.Empty(o.Path) {
		attrs = append(attrs, entity.String(MountPathId, o.Path))
	}
	attrs = append(attrs, entity.Bool(MountReadOnlyId, o.ReadOnly))
	return
}

func (o *Mount) Empty() bool {
	if !entity.Empty(o.Options) {
		return false
	}
	if !entity.Empty(o.Path) {
		return false
	}
	if !entity.Empty(o.ReadOnly) {
		return false
	}
	return true
}

func (o *Mount) InitSchema(sb *schema.SchemaBuilder) {
	sb.String("options", "dev.miren.storage/mount.options", schema.Doc("Mount options (e.g., \"rw,noatime\")"))
	sb.String("path", "dev.miren.storage/mount.path", schema.Doc("Mount path in the container"), schema.Required)
	sb.Bool("read_only", "dev.miren.storage/mount.read_only", schema.Doc("Whether the mount is read-only"))
}

const (
	DiskMountActualStateId                 = entity.Id("dev.miren.storage/disk_mount.actual_state")
	DiskMountActualStateDmPendingId        = entity.Id("dev.miren.storage/actual_state.dm_pending")
	DiskMountActualStateDmAttachingId      = entity.Id("dev.miren.storage/actual_state.dm_attaching")
	DiskMountActualStateDmAttachedId       = entity.Id("dev.miren.storage/actual_state.dm_attached")
	DiskMountActualStateDmMountingId       = entity.Id("dev.miren.storage/actual_state.dm_mounting")
	DiskMountActualStateDmMountedId        = entity.Id("dev.miren.storage/actual_state.dm_mounted")
	DiskMountActualStateDmUnmountingId     = entity.Id("dev.miren.storage/actual_state.dm_unmounting")
	DiskMountActualStateDmDetachingId      = entity.Id("dev.miren.storage/actual_state.dm_detaching")
	DiskMountActualStateDmDetachedId       = entity.Id("dev.miren.storage/actual_state.dm_detached")
	DiskMountActualStateDmErrorId          = entity.Id("dev.miren.storage/actual_state.dm_error")
	DiskMountDesiredStateId                = entity.Id("dev.miren.storage/disk_mount.desired_state")
	DiskMountDesiredStateDmWantMountedId   = entity.Id("dev.miren.storage/desired_state.dm_want_mounted")
	DiskMountDesiredStateDmWantUnmountedId = entity.Id("dev.miren.storage/desired_state.dm_want_unmounted")
	DiskMountDevicePathId                  = entity.Id("dev.miren.storage/disk_mount.device_path")
	DiskMountDiskLeaseIdId                 = entity.Id("dev.miren.storage/disk_mount.disk_lease_id")
	DiskMountErrorMessageId                = entity.Id("dev.miren.storage/disk_mount.error_message")
	DiskMountLoopDeviceId                  = entity.Id("dev.miren.storage/disk_mount.loop_device")
	DiskMountMountPathId                   = entity.Id("dev.miren.storage/disk_mount.mount_path")
	DiskMountNodeIdId                      = entity.Id("dev.miren.storage/disk_mount.node_id")
	DiskMountReadOnlyId                    = entity.Id("dev.miren.storage/disk_mount.read_only")
	DiskMountVolumeIdId                    = entity.Id("dev.miren.storage/disk_mount.volume_id")
)

type DiskMount struct {
	ID           entity.Id             `json:"id"`
	ActualState  DiskMountActualState  `cbor:"actual_state,omitempty" json:"actual_state,omitempty"`
	DesiredState DiskMountDesiredState `cbor:"desired_state,omitempty" json:"desired_state,omitempty"`
	DevicePath   string                `cbor:"device_path,omitempty" json:"device_path,omitempty"`
	DiskLeaseId  entity.Id             `cbor:"disk_lease_id,omitempty" json:"disk_lease_id,omitempty"`
	ErrorMessage string                `cbor:"error_message,omitempty" json:"error_message,omitempty"`
	LoopDevice   string                `cbor:"loop_device,omitempty" json:"loop_device,omitempty"`
	MountPath    string                `cbor:"mount_path" json:"mount_path"`
	NodeId       entity.Id             `cbor:"node_id" json:"node_id"`
	ReadOnly     bool                  `cbor:"read_only,omitempty" json:"read_only,omitempty"`
	VolumeId     entity.Id             `cbor:"volume_id" json:"volume_id"`
}

type DiskMountActualState string

const (
	DM_PENDING    DiskMountActualState = "actual_state.dm_pending"
	DM_ATTACHING  DiskMountActualState = "actual_state.dm_attaching"
	DM_ATTACHED   DiskMountActualState = "actual_state.dm_attached"
	DM_MOUNTING   DiskMountActualState = "actual_state.dm_mounting"
	DM_MOUNTED    DiskMountActualState = "actual_state.dm_mounted"
	DM_UNMOUNTING DiskMountActualState = "actual_state.dm_unmounting"
	DM_DETACHING  DiskMountActualState = "actual_state.dm_detaching"
	DM_DETACHED   DiskMountActualState = "actual_state.dm_detached"
	DM_ERROR      DiskMountActualState = "actual_state.dm_error"
)

var disk_mountactual_stateFromId = map[entity.Id]DiskMountActualState{DiskMountActualStateDmPendingId: DM_PENDING, DiskMountActualStateDmAttachingId: DM_ATTACHING, DiskMountActualStateDmAttachedId: DM_ATTACHED, DiskMountActualStateDmMountingId: DM_MOUNTING, DiskMountActualStateDmMountedId: DM_MOUNTED, DiskMountActualStateDmUnmountingId: DM_UNMOUNTING, DiskMountActualStateDmDetachingId: DM_DETACHING, DiskMountActualStateDmDetachedId: DM_DETACHED, DiskMountActualStateDmErrorId: DM_ERROR}
var disk_mountactual_stateToId = map[DiskMountActualState]entity.Id{DM_PENDING: DiskMountActualStateDmPendingId, DM_ATTACHING: DiskMountActualStateDmAttachingId, DM_ATTACHED: DiskMountActualStateDmAttachedId, DM_MOUNTING: DiskMountActualStateDmMountingId, DM_MOUNTED: DiskMountActualStateDmMountedId, DM_UNMOUNTING: DiskMountActualStateDmUnmountingId, DM_DETACHING: DiskMountActualStateDmDetachingId, DM_DETACHED: DiskMountActualStateDmDetachedId, DM_ERROR: DiskMountActualStateDmErrorId}

type DiskMountDesiredState string

const (
	DM_WANT_MOUNTED   DiskMountDesiredState = "desired_state.dm_want_mounted"
	DM_WANT_UNMOUNTED DiskMountDesiredState = "desired_state.dm_want_unmounted"
)

var disk_mountdesired_stateFromId = map[entity.Id]DiskMountDesiredState{DiskMountDesiredStateDmWantMountedId: DM_WANT_MOUNTED, DiskMountDesiredStateDmWantUnmountedId: DM_WANT_UNMOUNTED}
var disk_mountdesired_stateToId = map[DiskMountDesiredState]entity.Id{DM_WANT_MOUNTED: DiskMountDesiredStateDmWantMountedId, DM_WANT_UNMOUNTED: DiskMountDesiredStateDmWantUnmountedId}

func (o *DiskMount) Decode(e entity.AttrGetter) {
	o.ID = entity.MustGet(e, entity.DBId).Value.Id()
	if a, ok := e.Get(DiskMountActualStateId); ok && a.Value.Kind() == entity.KindId {
		o.ActualState = disk_mountactual_stateFromId[a.Value.Id()]
	}
	if a, ok := e.Get(DiskMountDesiredStateId); ok && a.Value.Kind() == entity.KindId {
		o.DesiredState = disk_mountdesired_stateFromId[a.Value.Id()]
	}
	if a, ok := e.Get(DiskMountDevicePathId); ok && a.Value.Kind() == entity.KindString {
		o.DevicePath = a.Value.String()
	}
	if a, ok := e.Get(DiskMountDiskLeaseIdId); ok && a.Value.Kind() == entity.KindId {
		o.DiskLeaseId = a.Value.Id()
	}
	if a, ok := e.Get(DiskMountErrorMessageId); ok && a.Value.Kind() == entity.KindString {
		o.ErrorMessage = a.Value.String()
	}
	if a, ok := e.Get(DiskMountLoopDeviceId); ok && a.Value.Kind() == entity.KindString {
		o.LoopDevice = a.Value.String()
	}
	if a, ok := e.Get(DiskMountMountPathId); ok && a.Value.Kind() == entity.KindString {
		o.MountPath = a.Value.String()
	}
	if a, ok := e.Get(DiskMountNodeIdId); ok && a.Value.Kind() == entity.KindId {
		o.NodeId = a.Value.Id()
	}
	if a, ok := e.Get(DiskMountReadOnlyId); ok && a.Value.Kind() == entity.KindBool {
		o.ReadOnly = a.Value.Bool()
	}
	if a, ok := e.Get(DiskMountVolumeIdId); ok && a.Value.Kind() == entity.KindId {
		o.VolumeId = a.Value.Id()
	}
}

func (o *DiskMount) Is(e entity.AttrGetter) bool {
	return entity.Is(e, KindDiskMount)
}

func (o *DiskMount) ShortKind() string {
	return "disk_mount"
}

func (o *DiskMount) Kind() entity.Id {
	return KindDiskMount
}

func (o *DiskMount) EntityId() entity.Id {
	return o.ID
}

func (o *DiskMount) Encode() (attrs []entity.Attr) {
	if a, ok := disk_mountactual_stateToId[o.ActualState]; ok {
		attrs = append(attrs, entity.Ref(DiskMountActualStateId, a))
	}
	if a, ok := disk_mountdesired_stateToId[o.DesiredState]; ok {
		attrs = append(attrs, entity.Ref(DiskMountDesiredStateId, a))
	}
	if !entity.Empty(o.DevicePath) {
		attrs = append(attrs, entity.String(DiskMountDevicePathId, o.DevicePath))
	}
	if !entity.Empty(o.DiskLeaseId) {
		attrs = append(attrs, entity.Ref(DiskMountDiskLeaseIdId, o.DiskLeaseId))
	}
	if !entity.Empty(o.ErrorMessage) {
		attrs = append(attrs, entity.String(DiskMountErrorMessageId, o.ErrorMessage))
	}
	if !entity.Empty(o.LoopDevice) {
		attrs = append(attrs, entity.String(DiskMountLoopDeviceId, o.LoopDevice))
	}
	if !entity.Empty(o.MountPath) {
		attrs = append(attrs, entity.String(DiskMountMountPathId, o.MountPath))
	}
	if !entity.Empty(o.NodeId) {
		attrs = append(attrs, entity.Ref(DiskMountNodeIdId, o.NodeId))
	}
	attrs = append(attrs, entity.Bool(DiskMountReadOnlyId, o.ReadOnly))
	if !entity.Empty(o.VolumeId) {
		attrs = append(attrs, entity.Ref(DiskMountVolumeIdId, o.VolumeId))
	}
	attrs = append(attrs, entity.Ref(entity.EntityKind, KindDiskMount))
	return
}

func (o *DiskMount) Empty() bool {
	if o.ActualState != "" {
		return false
	}
	if o.DesiredState != "" {
		return false
	}
	if !entity.Empty(o.DevicePath) {
		return false
	}
	if !entity.Empty(o.DiskLeaseId) {
		return false
	}
	if !entity.Empty(o.ErrorMessage) {
		return false
	}
	if !entity.Empty(o.LoopDevice) {
		return false
	}
	if !entity.Empty(o.MountPath) {
		return false
	}
	if !entity.Empty(o.NodeId) {
		return false
	}
	if !entity.Empty(o.ReadOnly) {
		return false
	}
	if !entity.Empty(o.VolumeId) {
		return false
	}
	return true
}

func (o *DiskMount) InitSchema(sb *schema.SchemaBuilder) {
	sb.Singleton("dev.miren.storage/actual_state.dm_pending")
	sb.Singleton("dev.miren.storage/actual_state.dm_attaching")
	sb.Singleton("dev.miren.storage/actual_state.dm_attached")
	sb.Singleton("dev.miren.storage/actual_state.dm_mounting")
	sb.Singleton("dev.miren.storage/actual_state.dm_mounted")
	sb.Singleton("dev.miren.storage/actual_state.dm_unmounting")
	sb.Singleton("dev.miren.storage/actual_state.dm_detaching")
	sb.Singleton("dev.miren.storage/actual_state.dm_detached")
	sb.Singleton("dev.miren.storage/actual_state.dm_error")
	sb.Ref("actual_state", "dev.miren.storage/disk_mount.actual_state", schema.Doc("Current state of the mount"), schema.Indexed, schema.Choices(DiskMountActualStateDmPendingId, DiskMountActualStateDmAttachingId, DiskMountActualStateDmAttachedId, DiskMountActualStateDmMountingId, DiskMountActualStateDmMountedId, DiskMountActualStateDmUnmountingId, DiskMountActualStateDmDetachingId, DiskMountActualStateDmDetachedId, DiskMountActualStateDmErrorId))
	sb.Singleton("dev.miren.storage/desired_state.dm_want_mounted")
	sb.Singleton("dev.miren.storage/desired_state.dm_want_unmounted")
	sb.Ref("desired_state", "dev.miren.storage/disk_mount.desired_state", schema.Doc("What state should this mount be in"), schema.Indexed, schema.Choices(DiskMountDesiredStateDmWantMountedId, DiskMountDesiredStateDmWantUnmountedId))
	sb.String("device_path", "dev.miren.storage/disk_mount.device_path", schema.Doc("Full path to the device node (e.g., /dev/loopN)"))
	sb.Ref("disk_lease_id", "dev.miren.storage/disk_mount.disk_lease_id", schema.Doc("Reference to the parent DiskLease entity"), schema.Indexed)
	sb.String("error_message", "dev.miren.storage/disk_mount.error_message", schema.Doc("Error details if actual_state is error"))
	sb.String("loop_device", "dev.miren.storage/disk_mount.loop_device", schema.Doc("Loop device name for universal mode"))
	sb.String("mount_path", "dev.miren.storage/disk_mount.mount_path", schema.Doc("Path where the volume should be mounted"), schema.Required)
	sb.Ref("node_id", "dev.miren.storage/disk_mount.node_id", schema.Doc("Node where this mount exists"), schema.Required, schema.Indexed)
	sb.Bool("read_only", "dev.miren.storage/disk_mount.read_only", schema.Doc("Whether the mount is read-only"))
	sb.Ref("volume_id", "dev.miren.storage/disk_mount.volume_id", schema.Doc("Reference to the disk_volume entity"), schema.Required, schema.Indexed)
}

const (
	DiskVolumeActualStateId           = entity.Id("dev.miren.storage/disk_volume.actual_state")
	DiskVolumeActualStateDvPendingId  = entity.Id("dev.miren.storage/actual_state.dv_pending")
	DiskVolumeActualStateDvCreatingId = entity.Id("dev.miren.storage/actual_state.dv_creating")
	DiskVolumeActualStateDvReadyId    = entity.Id("dev.miren.storage/actual_state.dv_ready")
	DiskVolumeActualStateDvDeletingId = entity.Id("dev.miren.storage/actual_state.dv_deleting")
	DiskVolumeActualStateDvDeletedId  = entity.Id("dev.miren.storage/actual_state.dv_deleted")
	DiskVolumeActualStateDvErrorId    = entity.Id("dev.miren.storage/actual_state.dv_error")
	DiskVolumeDesiredStateId          = entity.Id("dev.miren.storage/disk_volume.desired_state")
	DiskVolumeDesiredStateDvPresentId = entity.Id("dev.miren.storage/desired_state.dv_present")
	DiskVolumeDesiredStateDvAbsentId  = entity.Id("dev.miren.storage/desired_state.dv_absent")
	DiskVolumeDiskIdId                = entity.Id("dev.miren.storage/disk_volume.disk_id")
	DiskVolumeErrorMessageId          = entity.Id("dev.miren.storage/disk_volume.error_message")
	DiskVolumeFilesystemId            = entity.Id("dev.miren.storage/disk_volume.filesystem")
	DiskVolumeImagePathId             = entity.Id("dev.miren.storage/disk_volume.image_path")
	DiskVolumeModeId                  = entity.Id("dev.miren.storage/disk_volume.mode")
	DiskVolumeNameId                  = entity.Id("dev.miren.storage/disk_volume.name")
	DiskVolumeNodeIdId                = entity.Id("dev.miren.storage/disk_volume.node_id")
	DiskVolumeSizeGbId                = entity.Id("dev.miren.storage/disk_volume.size_gb")
	DiskVolumeVolumeIdId              = entity.Id("dev.miren.storage/disk_volume.volume_id")
)

type DiskVolume struct {
	ID           entity.Id              `json:"id"`
	ActualState  DiskVolumeActualState  `cbor:"actual_state,omitempty" json:"actual_state,omitempty"`
	DesiredState DiskVolumeDesiredState `cbor:"desired_state,omitempty" json:"desired_state,omitempty"`
	DiskId       entity.Id              `cbor:"disk_id" json:"disk_id"`
	ErrorMessage string                 `cbor:"error_message,omitempty" json:"error_message,omitempty"`
	Filesystem   string                 `cbor:"filesystem,omitempty" json:"filesystem,omitempty"`
	ImagePath    string                 `cbor:"image_path,omitempty" json:"image_path,omitempty"`
	Mode         string                 `cbor:"mode,omitempty" json:"mode,omitempty"`
	Name         string                 `cbor:"name,omitempty" json:"name,omitempty"`
	NodeId       entity.Id              `cbor:"node_id" json:"node_id"`
	SizeGb       int64                  `cbor:"size_gb" json:"size_gb"`
	VolumeId     string                 `cbor:"volume_id,omitempty" json:"volume_id,omitempty"`
}

type DiskVolumeActualState string

const (
	DV_PENDING  DiskVolumeActualState = "actual_state.dv_pending"
	DV_CREATING DiskVolumeActualState = "actual_state.dv_creating"
	DV_READY    DiskVolumeActualState = "actual_state.dv_ready"
	DV_DELETING DiskVolumeActualState = "actual_state.dv_deleting"
	DV_DELETED  DiskVolumeActualState = "actual_state.dv_deleted"
	DV_ERROR    DiskVolumeActualState = "actual_state.dv_error"
)

var disk_volumeactual_stateFromId = map[entity.Id]DiskVolumeActualState{DiskVolumeActualStateDvPendingId: DV_PENDING, DiskVolumeActualStateDvCreatingId: DV_CREATING, DiskVolumeActualStateDvReadyId: DV_READY, DiskVolumeActualStateDvDeletingId: DV_DELETING, DiskVolumeActualStateDvDeletedId: DV_DELETED, DiskVolumeActualStateDvErrorId: DV_ERROR}
var disk_volumeactual_stateToId = map[DiskVolumeActualState]entity.Id{DV_PENDING: DiskVolumeActualStateDvPendingId, DV_CREATING: DiskVolumeActualStateDvCreatingId, DV_READY: DiskVolumeActualStateDvReadyId, DV_DELETING: DiskVolumeActualStateDvDeletingId, DV_DELETED: DiskVolumeActualStateDvDeletedId, DV_ERROR: DiskVolumeActualStateDvErrorId}

type DiskVolumeDesiredState string

const (
	DV_PRESENT DiskVolumeDesiredState = "desired_state.dv_present"
	DV_ABSENT  DiskVolumeDesiredState = "desired_state.dv_absent"
)

var disk_volumedesired_stateFromId = map[entity.Id]DiskVolumeDesiredState{DiskVolumeDesiredStateDvPresentId: DV_PRESENT, DiskVolumeDesiredStateDvAbsentId: DV_ABSENT}
var disk_volumedesired_stateToId = map[DiskVolumeDesiredState]entity.Id{DV_PRESENT: DiskVolumeDesiredStateDvPresentId, DV_ABSENT: DiskVolumeDesiredStateDvAbsentId}

func (o *DiskVolume) Decode(e entity.AttrGetter) {
	o.ID = entity.MustGet(e, entity.DBId).Value.Id()
	if a, ok := e.Get(DiskVolumeActualStateId); ok && a.Value.Kind() == entity.KindId {
		o.ActualState = disk_volumeactual_stateFromId[a.Value.Id()]
	}
	if a, ok := e.Get(DiskVolumeDesiredStateId); ok && a.Value.Kind() == entity.KindId {
		o.DesiredState = disk_volumedesired_stateFromId[a.Value.Id()]
	}
	if a, ok := e.Get(DiskVolumeDiskIdId); ok && a.Value.Kind() == entity.KindId {
		o.DiskId = a.Value.Id()
	}
	if a, ok := e.Get(DiskVolumeErrorMessageId); ok && a.Value.Kind() == entity.KindString {
		o.ErrorMessage = a.Value.String()
	}
	if a, ok := e.Get(DiskVolumeFilesystemId); ok && a.Value.Kind() == entity.KindString {
		o.Filesystem = a.Value.String()
	}
	if a, ok := e.Get(DiskVolumeImagePathId); ok && a.Value.Kind() == entity.KindString {
		o.ImagePath = a.Value.String()
	}
	if a, ok := e.Get(DiskVolumeModeId); ok && a.Value.Kind() == entity.KindString {
		o.Mode = a.Value.String()
	}
	if a, ok := e.Get(DiskVolumeNameId); ok && a.Value.Kind() == entity.KindString {
		o.Name = a.Value.String()
	}
	if a, ok := e.Get(DiskVolumeNodeIdId); ok && a.Value.Kind() == entity.KindId {
		o.NodeId = a.Value.Id()
	}
	if a, ok := e.Get(DiskVolumeSizeGbId); ok && a.Value.Kind() == entity.KindInt64 {
		o.SizeGb = a.Value.Int64()
	}
	if a, ok := e.Get(DiskVolumeVolumeIdId); ok && a.Value.Kind() == entity.KindString {
		o.VolumeId = a.Value.String()
	}
}

func (o *DiskVolume) Is(e entity.AttrGetter) bool {
	return entity.Is(e, KindDiskVolume)
}

func (o *DiskVolume) ShortKind() string {
	return "disk_volume"
}

func (o *DiskVolume) Kind() entity.Id {
	return KindDiskVolume
}

func (o *DiskVolume) EntityId() entity.Id {
	return o.ID
}

func (o *DiskVolume) Encode() (attrs []entity.Attr) {
	if a, ok := disk_volumeactual_stateToId[o.ActualState]; ok {
		attrs = append(attrs, entity.Ref(DiskVolumeActualStateId, a))
	}
	if a, ok := disk_volumedesired_stateToId[o.DesiredState]; ok {
		attrs = append(attrs, entity.Ref(DiskVolumeDesiredStateId, a))
	}
	if !entity.Empty(o.DiskId) {
		attrs = append(attrs, entity.Ref(DiskVolumeDiskIdId, o.DiskId))
	}
	if !entity.Empty(o.ErrorMessage) {
		attrs = append(attrs, entity.String(DiskVolumeErrorMessageId, o.ErrorMessage))
	}
	if !entity.Empty(o.Filesystem) {
		attrs = append(attrs, entity.String(DiskVolumeFilesystemId, o.Filesystem))
	}
	if !entity.Empty(o.ImagePath) {
		attrs = append(attrs, entity.String(DiskVolumeImagePathId, o.ImagePath))
	}
	if !entity.Empty(o.Mode) {
		attrs = append(attrs, entity.String(DiskVolumeModeId, o.Mode))
	}
	if !entity.Empty(o.Name) {
		attrs = append(attrs, entity.String(DiskVolumeNameId, o.Name))
	}
	if !entity.Empty(o.NodeId) {
		attrs = append(attrs, entity.Ref(DiskVolumeNodeIdId, o.NodeId))
	}
	attrs = append(attrs, entity.Int64(DiskVolumeSizeGbId, o.SizeGb))
	if !entity.Empty(o.VolumeId) {
		attrs = append(attrs, entity.String(DiskVolumeVolumeIdId, o.VolumeId))
	}
	attrs = append(attrs, entity.Ref(entity.EntityKind, KindDiskVolume))
	return
}

func (o *DiskVolume) Empty() bool {
	if o.ActualState != "" {
		return false
	}
	if o.DesiredState != "" {
		return false
	}
	if !entity.Empty(o.DiskId) {
		return false
	}
	if !entity.Empty(o.ErrorMessage) {
		return false
	}
	if !entity.Empty(o.Filesystem) {
		return false
	}
	if !entity.Empty(o.ImagePath) {
		return false
	}
	if !entity.Empty(o.Mode) {
		return false
	}
	if !entity.Empty(o.Name) {
		return false
	}
	if !entity.Empty(o.NodeId) {
		return false
	}
	if !entity.Empty(o.SizeGb) {
		return false
	}
	if !entity.Empty(o.VolumeId) {
		return false
	}
	return true
}

func (o *DiskVolume) InitSchema(sb *schema.SchemaBuilder) {
	sb.Singleton("dev.miren.storage/actual_state.dv_pending")
	sb.Singleton("dev.miren.storage/actual_state.dv_creating")
	sb.Singleton("dev.miren.storage/actual_state.dv_ready")
	sb.Singleton("dev.miren.storage/actual_state.dv_deleting")
	sb.Singleton("dev.miren.storage/actual_state.dv_deleted")
	sb.Singleton("dev.miren.storage/actual_state.dv_error")
	sb.Ref("actual_state", "dev.miren.storage/disk_volume.actual_state", schema.Doc("Current state of the volume"), schema.Indexed, schema.Choices(DiskVolumeActualStateDvPendingId, DiskVolumeActualStateDvCreatingId, DiskVolumeActualStateDvReadyId, DiskVolumeActualStateDvDeletingId, DiskVolumeActualStateDvDeletedId, DiskVolumeActualStateDvErrorId))
	sb.Singleton("dev.miren.storage/desired_state.dv_present")
	sb.Singleton("dev.miren.storage/desired_state.dv_absent")
	sb.Ref("desired_state", "dev.miren.storage/disk_volume.desired_state", schema.Doc("What state should this volume be in"), schema.Indexed, schema.Choices(DiskVolumeDesiredStateDvPresentId, DiskVolumeDesiredStateDvAbsentId))
	sb.Ref("disk_id", "dev.miren.storage/disk_volume.disk_id", schema.Doc("Reference to the parent Disk entity"), schema.Required, schema.Indexed)
	sb.String("error_message", "dev.miren.storage/disk_volume.error_message", schema.Doc("Error details if actual_state is error"))
	sb.String("filesystem", "dev.miren.storage/disk_volume.filesystem", schema.Doc("Filesystem type (ext4, xfs, btrfs)"))
	sb.String("image_path", "dev.miren.storage/disk_volume.image_path", schema.Doc("Path to backing image file"))
	sb.String("mode", "dev.miren.storage/disk_volume.mode", schema.Doc("Disk I/O mode (universal or accelerator)"))
	sb.String("name", "dev.miren.storage/disk_volume.name", schema.Doc("Human-readable name for the volume (from parent disk)"))
	sb.Ref("node_id", "dev.miren.storage/disk_volume.node_id", schema.Doc("Node where this volume should be provisioned"), schema.Required, schema.Indexed)
	sb.Int64("size_gb", "dev.miren.storage/disk_volume.size_gb", schema.Doc("Volume size in gigabytes"), schema.Required)
	sb.String("volume_id", "dev.miren.storage/disk_volume.volume_id", schema.Doc("Volume identifier (generated during creation)"), schema.Indexed)
}

var (
	KindDisk       = entity.Id("dev.miren.storage/kind.disk")
	KindDiskLease  = entity.Id("dev.miren.storage/kind.disk_lease")
	KindDiskMount  = entity.Id("dev.miren.storage/kind.disk_mount")
	KindDiskVolume = entity.Id("dev.miren.storage/kind.disk_volume")
	Schema         = entity.Id("dev.miren.storage/schema.v1alpha")
)

func init() {
	schema.Register("dev.miren.storage", "v1alpha", func(sb *schema.SchemaBuilder) {
		(&Disk{}).InitSchema(sb)
		(&DiskLease{}).InitSchema(sb)
		(&DiskMount{}).InitSchema(sb)
		(&DiskVolume{}).InitSchema(sb)
	})
	schema.RegisterEncodedSchema("dev.miren.storage", "v1alpha", []byte("\x1f\x8b\b\x00\x00\x00\x00\x00\x00\xff\xacX[\x8e\xec&\x10]H\xde\xef\xa7|u\xa5\xec\a\xd1M\xd9Mۀ/`Ǔ\xdf\xfc䱋\xcch\xa4l0\xdf\x11\x05ش\x87\xc6\xccM~Z\x06\x9fsL\x15\xe5Sn\x9e\x98\xa4\x02\xde1\x98\x1b\xc15\xc8\xc6X\xa5i\a\xd0s\xc9\xcc\xd3\xf2\xc1\x8b;oܝ\x86q\xd3?#w~\x89p7\xbd\xc0?-S\x82r\xf9\xf2\x01m\xcba`\xe6\xf7\xc7\x13g\xcbgy\x8d欁Z`\xe4\U001003fa&c\xfb0\u0089\xb3\xa7\x12\xbd\xe5\x03\x98\acAxz2vt\x06r\x12\xbd\xfb!3\x1d&0\x8f\xe7\xa55˧/\xd56b\xb3\xb4\x86\xc1b\x7f\xca=4\x819\b\x9c\xacn\xcd\xf2y\x11\x88\x18L\xc2Ww\xa2\x18\xcc\xccȬ\x86I\x00\xe1\f#\x91\xbb9\x17Mk\xac\xe6\xb2Ädv\r\xa5\x84b\x80\x02\f\xaf\xb2I\xf8\x8bO\x92Ϡ\r\x1dr\xa9p\xc4fE\xf4\xf4|\x86\x014\xb5J\xe7\x02Et\x82y,\xad\x0e\x17\xb6\xfd$A!-#\x8f4\rBY J\x0e\xbeJ\xfat\x02C<)5\xa0\xc4\xc7w$\f\xff\x05HwBz\x17\a\x8ez\xe6\xd2bF?\xbaǴ\xd4N\x06\x89m\xb8\xcef\xf5\x19@k\xa5s+\xf0\xb4\x06\xef_\xa8\xb5\xf4|\x81lM\a`\x84\\\x18\f`\xb9\xec\n\xd8\b\xb908ԍ\x90~\xd4j\xe6\x86+\tl\xf9\xf2.<A\r\xeb\xb5[M\xa6\x8e\xf7\x94\xb8\xa5\x99\xfa¬\xdeV;\xcf\x16z\xe7*\x90+\xd9\xcdo\xe90^\xe80j.\xa8~ \xcex\x98\x93\xc9ź\x9a\x17\x19\x80\x1a\xf0\x16\xb6|\x98_\x87\xc7T:\xd9o\x18ѷ%\xa5\x86\x9e\xdfM\\\x03#\xd4\xfaRM'\xb0n,\x17\x80B_\x94\x85\xc61f\xa7\r\xd7\xc1\x10\x91\x9cٵ\x84\x8c\x97\x81\xdd\xc5AJ\xff\xbeH\xc7B%\x02\x8c\xa1\x9d\x7fU\xc5\xed\xd4ލ\uef38AN\xa8I\xfal\x80\xbftt~VbT\x12\xa4ݮ\xc2^\xbdTk\xf6j\x95;\xf6+\x06\xfbIε&i\x1b5Z\xae\xa4\x7f\xb7\xbb8؛R\xa6r<{\xa4\xf6\xe2}\f\xaf\xf6\xbcLiz\x9e\x06\xca6/\xe3\xdbpu\xb2b\xe1\xfb\x1cV\x14\x81Tl}\xc1\xba8H\x8b\xe0\x9b\"\xddP\xc9Nj\x89\n\xd7d\x9cv\xe6r\x15ך\xe7\x13\x9c\xd4$\xb3\xf6\x1d\x9c\x05\xef\xb7-\xe5\x03dw4\xc0<\xa0\x1bA2\xe7T\x19\xfb\x89N\xe5\x11\x17\r\xb8ҒmFHq[\xae[\xd4eW\xc2\xed;p\xa5\xd7\xd4\xf8\x1f\xb8\rߕ\x94\x1az\xb6\x13\x1d\x88\x8bǿ\xcf\xc3\xcdLvK\xfe\xbe0A|K\xcb\x14J\xcao\"\xf0\xcaDXzvA{N\x80:Vܯ\nV\x80\xf6L\x90\xb5\x95f\xeclO\x8bX\xc7[[e\x05o\xed\x99q\xc1n\x99\x15\xbc\x88\x1d\xd6g;\xe2\x0f\xb5\v\rL\xff\xf4J\xe6\n\x16L\x90I\xae\xab\xfd\U00058ea1\x9fJ\xed\xc1W\x13\x03\x83\x1dm+'q;\x95\xff\xeaTL\x90\x9f\xa9\xb4k\x89\xbc\xc9<%\xd5iv\x84wq\x1cV\vly[+\xb1R\x8a=<\xc67\xf33\x90\xd5\xdf\xfbtbo\xf3\a\xa9ZM!\xfa\xa8\xb8\x9d\xaai\xca^\xea5M\xb9\"\xc8A\xa9\x91\xf8\xc0|\x90\xe9\xc4^\xea^\xa7\xf0R\xf8\xbb\xa5뚌\xf7B\xf7:\x96\x17:\xecX_\x17\xe9Ǎ\xb5B\xa4\xf8az\xe25M\x00\x85r\xdfD[\x13\xf0\xb2\xa1\v\xdc\xf9\xe7\x11@\x95m\xe0\xcf\xe2\x8b\xeb\xa5ޫ\x0f<_\xd8\\\xdb\a\x02\xd01\\\xf6\x1fj\x18\b\xbc\xb2\x99\xe0?\x99\x9aαB\x1d\xab\xbas\xcc[\xe7\x98\t\x1e3T9\xf9\x86\xed\xe3\x83+y\x11\x8b\x1b\x93\xb1\xeftc\xde\xd3R9\x9b\t=\x19\x906\xfb\x01p\xeb\x84\x11\x8aYӀ\xac\\\xbd\xecY\x01[:\xc1X\xc38\xfa\xdfq\x90\x86\xff\xcd\xe3\x82\xde\xc1\xf1\xd0+\x94\xb8\xa0]\xd2\x12\xaeɸ\xf2\x14#*eNh^\xc1>8A9؟C\x83=\xe0\x17OQ\x8a\x9d\"\b\xfc\xe7\xbf\xfd}\xa2\xb6\a\xf6梴%\xfe\\ӟ\x0f\x94\x0e7\xab\xbf\xd8\x11\x92Z\xfb\xf1\xf7}\xba̚N\xf0/\x00\x00\x00\xff\xff\x01\x00\x00\xff\xff\x10V\x84\x8b\xa5\x15\x00\x00"))
}
