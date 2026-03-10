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
	DiskStatusRestoringId    = entity.Id("dev.miren.storage/status.restoring")
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
	RESTORING    DiskStatus = "status.restoring"
)

var diskstatusFromId = map[entity.Id]DiskStatus{DiskStatusProvisioningId: PROVISIONING, DiskStatusProvisionedId: PROVISIONED, DiskStatusAttachedId: ATTACHED, DiskStatusDetachedId: DETACHED, DiskStatusDeletingId: DELETING, DiskStatusErrorId: ERROR, DiskStatusRestoringId: RESTORING}
var diskstatusToId = map[DiskStatus]entity.Id{PROVISIONING: DiskStatusProvisioningId, PROVISIONED: DiskStatusProvisionedId, ATTACHED: DiskStatusAttachedId, DETACHED: DiskStatusDetachedId, DELETING: DiskStatusDeletingId, ERROR: DiskStatusErrorId, RESTORING: DiskStatusRestoringId}

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
	sb.Singleton("dev.miren.storage/status.restoring")
	sb.Ref("status", "dev.miren.storage/disk.status", schema.Doc("Current state of the disk"), schema.Indexed, schema.Choices(DiskStatusProvisioningId, DiskStatusProvisionedId, DiskStatusAttachedId, DiskStatusDetachedId, DiskStatusDeletingId, DiskStatusErrorId, DiskStatusRestoringId))
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
	DiskVolumeActualStateId             = entity.Id("dev.miren.storage/disk_volume.actual_state")
	DiskVolumeActualStateDvPendingId    = entity.Id("dev.miren.storage/actual_state.dv_pending")
	DiskVolumeActualStateDvCreatingId   = entity.Id("dev.miren.storage/actual_state.dv_creating")
	DiskVolumeActualStateDvReadyId      = entity.Id("dev.miren.storage/actual_state.dv_ready")
	DiskVolumeActualStateDvDeletingId   = entity.Id("dev.miren.storage/actual_state.dv_deleting")
	DiskVolumeActualStateDvDeletedId    = entity.Id("dev.miren.storage/actual_state.dv_deleted")
	DiskVolumeActualStateDvErrorId      = entity.Id("dev.miren.storage/actual_state.dv_error")
	DiskVolumeDesiredStateId            = entity.Id("dev.miren.storage/disk_volume.desired_state")
	DiskVolumeDesiredStateDvPresentId   = entity.Id("dev.miren.storage/desired_state.dv_present")
	DiskVolumeDesiredStateDvAbsentId    = entity.Id("dev.miren.storage/desired_state.dv_absent")
	DiskVolumeDiskIdId                  = entity.Id("dev.miren.storage/disk_volume.disk_id")
	DiskVolumeErrorMessageId            = entity.Id("dev.miren.storage/disk_volume.error_message")
	DiskVolumeFilesystemId              = entity.Id("dev.miren.storage/disk_volume.filesystem")
	DiskVolumeImagePathId               = entity.Id("dev.miren.storage/disk_volume.image_path")
	DiskVolumeMountIdId                 = entity.Id("dev.miren.storage/disk_volume.mount_id")
	DiskVolumeNameId                    = entity.Id("dev.miren.storage/disk_volume.name")
	DiskVolumeNodeIdId                  = entity.Id("dev.miren.storage/disk_volume.node_id")
	DiskVolumeSizeGbId                  = entity.Id("dev.miren.storage/disk_volume.size_gb")
	DiskVolumeVolumeIdId                = entity.Id("dev.miren.storage/disk_volume.volume_id")
	DiskVolumeVolumeModeId              = entity.Id("dev.miren.storage/disk_volume.volume_mode")
	DiskVolumeVolumeModeVmUniversalId   = entity.Id("dev.miren.storage/volume_mode.vm_universal")
	DiskVolumeVolumeModeVmAcceleratorId = entity.Id("dev.miren.storage/volume_mode.vm_accelerator")
)

type DiskVolume struct {
	ID           entity.Id              `json:"id"`
	ActualState  DiskVolumeActualState  `cbor:"actual_state,omitempty" json:"actual_state,omitempty"`
	DesiredState DiskVolumeDesiredState `cbor:"desired_state,omitempty" json:"desired_state,omitempty"`
	DiskId       entity.Id              `cbor:"disk_id" json:"disk_id"`
	ErrorMessage string                 `cbor:"error_message,omitempty" json:"error_message,omitempty"`
	Filesystem   string                 `cbor:"filesystem,omitempty" json:"filesystem,omitempty"`
	ImagePath    string                 `cbor:"image_path,omitempty" json:"image_path,omitempty"`
	MountId      string                 `cbor:"mount_id,omitempty" json:"mount_id,omitempty"`
	Name         string                 `cbor:"name,omitempty" json:"name,omitempty"`
	NodeId       entity.Id              `cbor:"node_id" json:"node_id"`
	SizeGb       int64                  `cbor:"size_gb" json:"size_gb"`
	VolumeId     string                 `cbor:"volume_id,omitempty" json:"volume_id,omitempty"`
	VolumeMode   DiskVolumeVolumeMode   `cbor:"volume_mode,omitempty" json:"volume_mode,omitempty"`
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

type DiskVolumeVolumeMode string

const (
	VM_UNIVERSAL   DiskVolumeVolumeMode = "volume_mode.vm_universal"
	VM_ACCELERATOR DiskVolumeVolumeMode = "volume_mode.vm_accelerator"
)

var disk_volumevolume_modeFromId = map[entity.Id]DiskVolumeVolumeMode{DiskVolumeVolumeModeVmUniversalId: VM_UNIVERSAL, DiskVolumeVolumeModeVmAcceleratorId: VM_ACCELERATOR}
var disk_volumevolume_modeToId = map[DiskVolumeVolumeMode]entity.Id{VM_UNIVERSAL: DiskVolumeVolumeModeVmUniversalId, VM_ACCELERATOR: DiskVolumeVolumeModeVmAcceleratorId}

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
	if a, ok := e.Get(DiskVolumeMountIdId); ok && a.Value.Kind() == entity.KindString {
		o.MountId = a.Value.String()
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
	if a, ok := e.Get(DiskVolumeVolumeModeId); ok && a.Value.Kind() == entity.KindId {
		o.VolumeMode = disk_volumevolume_modeFromId[a.Value.Id()]
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
	if !entity.Empty(o.MountId) {
		attrs = append(attrs, entity.String(DiskVolumeMountIdId, o.MountId))
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
	if a, ok := disk_volumevolume_modeToId[o.VolumeMode]; ok {
		attrs = append(attrs, entity.Ref(DiskVolumeVolumeModeId, a))
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
	if !entity.Empty(o.MountId) {
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
	if o.VolumeMode != "" {
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
	sb.String("mount_id", "dev.miren.storage/disk_volume.mount_id", schema.Doc("Override for the mount point directory name (defaults to entity suffix if empty)"))
	sb.String("name", "dev.miren.storage/disk_volume.name", schema.Doc("Human-readable name for the volume (from parent disk)"))
	sb.Ref("node_id", "dev.miren.storage/disk_volume.node_id", schema.Doc("Node where this volume should be provisioned"), schema.Required, schema.Indexed)
	sb.Int64("size_gb", "dev.miren.storage/disk_volume.size_gb", schema.Doc("Volume size in gigabytes"), schema.Required)
	sb.String("volume_id", "dev.miren.storage/disk_volume.volume_id", schema.Doc("Volume identifier (generated during creation)"), schema.Indexed)
	sb.Singleton("dev.miren.storage/volume_mode.vm_universal")
	sb.Singleton("dev.miren.storage/volume_mode.vm_accelerator")
	sb.Ref("volume_mode", "dev.miren.storage/disk_volume.volume_mode", schema.Doc("Disk I/O mode"), schema.Choices(DiskVolumeVolumeModeVmUniversalId, DiskVolumeVolumeModeVmAcceleratorId))
}

const (
	LsvdVolumeActualStateId            = entity.Id("dev.miren.storage/lsvd_volume.actual_state")
	LsvdVolumeActualStateVolPendingId  = entity.Id("dev.miren.storage/actual_state.vol_pending")
	LsvdVolumeActualStateVolCreatingId = entity.Id("dev.miren.storage/actual_state.vol_creating")
	LsvdVolumeActualStateVolReadyId    = entity.Id("dev.miren.storage/actual_state.vol_ready")
	LsvdVolumeActualStateVolDeletingId = entity.Id("dev.miren.storage/actual_state.vol_deleting")
	LsvdVolumeActualStateVolDeletedId  = entity.Id("dev.miren.storage/actual_state.vol_deleted")
	LsvdVolumeActualStateVolErrorId    = entity.Id("dev.miren.storage/actual_state.vol_error")
	LsvdVolumeDesiredStateId           = entity.Id("dev.miren.storage/lsvd_volume.desired_state")
	LsvdVolumeDesiredStateVolPresentId = entity.Id("dev.miren.storage/desired_state.vol_present")
	LsvdVolumeDesiredStateVolAbsentId  = entity.Id("dev.miren.storage/desired_state.vol_absent")
	LsvdVolumeDiskIdId                 = entity.Id("dev.miren.storage/lsvd_volume.disk_id")
	LsvdVolumeErrorMessageId           = entity.Id("dev.miren.storage/lsvd_volume.error_message")
	LsvdVolumeFilesystemId             = entity.Id("dev.miren.storage/lsvd_volume.filesystem")
	LsvdVolumeNameId                   = entity.Id("dev.miren.storage/lsvd_volume.name")
	LsvdVolumeNodeIdId                 = entity.Id("dev.miren.storage/lsvd_volume.node_id")
	LsvdVolumeRemoteOnlyId             = entity.Id("dev.miren.storage/lsvd_volume.remote_only")
	LsvdVolumeSizeGbId                 = entity.Id("dev.miren.storage/lsvd_volume.size_gb")
	LsvdVolumeVolumeIdId               = entity.Id("dev.miren.storage/lsvd_volume.volume_id")
)

type LsvdVolume struct {
	ID           entity.Id              `json:"id"`
	ActualState  LsvdVolumeActualState  `cbor:"actual_state,omitempty" json:"actual_state,omitempty"`
	DesiredState LsvdVolumeDesiredState `cbor:"desired_state,omitempty" json:"desired_state,omitempty"`
	DiskId       entity.Id              `cbor:"disk_id" json:"disk_id"`
	ErrorMessage string                 `cbor:"error_message,omitempty" json:"error_message,omitempty"`
	Filesystem   string                 `cbor:"filesystem,omitempty" json:"filesystem,omitempty"`
	Name         string                 `cbor:"name,omitempty" json:"name,omitempty"`
	NodeId       entity.Id              `cbor:"node_id" json:"node_id"`
	RemoteOnly   bool                   `cbor:"remote_only,omitempty" json:"remote_only,omitempty"`
	SizeGb       int64                  `cbor:"size_gb" json:"size_gb"`
	VolumeId     string                 `cbor:"volume_id,omitempty" json:"volume_id,omitempty"`
}

type LsvdVolumeActualState string

const (
	VOL_PENDING  LsvdVolumeActualState = "actual_state.vol_pending"
	VOL_CREATING LsvdVolumeActualState = "actual_state.vol_creating"
	VOL_READY    LsvdVolumeActualState = "actual_state.vol_ready"
	VOL_DELETING LsvdVolumeActualState = "actual_state.vol_deleting"
	VOL_DELETED  LsvdVolumeActualState = "actual_state.vol_deleted"
	VOL_ERROR    LsvdVolumeActualState = "actual_state.vol_error"
)

var lsvd_volumeactual_stateFromId = map[entity.Id]LsvdVolumeActualState{LsvdVolumeActualStateVolPendingId: VOL_PENDING, LsvdVolumeActualStateVolCreatingId: VOL_CREATING, LsvdVolumeActualStateVolReadyId: VOL_READY, LsvdVolumeActualStateVolDeletingId: VOL_DELETING, LsvdVolumeActualStateVolDeletedId: VOL_DELETED, LsvdVolumeActualStateVolErrorId: VOL_ERROR}
var lsvd_volumeactual_stateToId = map[LsvdVolumeActualState]entity.Id{VOL_PENDING: LsvdVolumeActualStateVolPendingId, VOL_CREATING: LsvdVolumeActualStateVolCreatingId, VOL_READY: LsvdVolumeActualStateVolReadyId, VOL_DELETING: LsvdVolumeActualStateVolDeletingId, VOL_DELETED: LsvdVolumeActualStateVolDeletedId, VOL_ERROR: LsvdVolumeActualStateVolErrorId}

type LsvdVolumeDesiredState string

const (
	VOL_PRESENT LsvdVolumeDesiredState = "desired_state.vol_present"
	VOL_ABSENT  LsvdVolumeDesiredState = "desired_state.vol_absent"
)

var lsvd_volumedesired_stateFromId = map[entity.Id]LsvdVolumeDesiredState{LsvdVolumeDesiredStateVolPresentId: VOL_PRESENT, LsvdVolumeDesiredStateVolAbsentId: VOL_ABSENT}
var lsvd_volumedesired_stateToId = map[LsvdVolumeDesiredState]entity.Id{VOL_PRESENT: LsvdVolumeDesiredStateVolPresentId, VOL_ABSENT: LsvdVolumeDesiredStateVolAbsentId}

func (o *LsvdVolume) Decode(e entity.AttrGetter) {
	o.ID = entity.MustGet(e, entity.DBId).Value.Id()
	if a, ok := e.Get(LsvdVolumeActualStateId); ok && a.Value.Kind() == entity.KindId {
		o.ActualState = lsvd_volumeactual_stateFromId[a.Value.Id()]
	}
	if a, ok := e.Get(LsvdVolumeDesiredStateId); ok && a.Value.Kind() == entity.KindId {
		o.DesiredState = lsvd_volumedesired_stateFromId[a.Value.Id()]
	}
	if a, ok := e.Get(LsvdVolumeDiskIdId); ok && a.Value.Kind() == entity.KindId {
		o.DiskId = a.Value.Id()
	}
	if a, ok := e.Get(LsvdVolumeErrorMessageId); ok && a.Value.Kind() == entity.KindString {
		o.ErrorMessage = a.Value.String()
	}
	if a, ok := e.Get(LsvdVolumeFilesystemId); ok && a.Value.Kind() == entity.KindString {
		o.Filesystem = a.Value.String()
	}
	if a, ok := e.Get(LsvdVolumeNameId); ok && a.Value.Kind() == entity.KindString {
		o.Name = a.Value.String()
	}
	if a, ok := e.Get(LsvdVolumeNodeIdId); ok && a.Value.Kind() == entity.KindId {
		o.NodeId = a.Value.Id()
	}
	if a, ok := e.Get(LsvdVolumeRemoteOnlyId); ok && a.Value.Kind() == entity.KindBool {
		o.RemoteOnly = a.Value.Bool()
	}
	if a, ok := e.Get(LsvdVolumeSizeGbId); ok && a.Value.Kind() == entity.KindInt64 {
		o.SizeGb = a.Value.Int64()
	}
	if a, ok := e.Get(LsvdVolumeVolumeIdId); ok && a.Value.Kind() == entity.KindString {
		o.VolumeId = a.Value.String()
	}
}

func (o *LsvdVolume) Is(e entity.AttrGetter) bool {
	return entity.Is(e, KindLsvdVolume)
}

func (o *LsvdVolume) ShortKind() string {
	return "lsvd_volume"
}

func (o *LsvdVolume) Kind() entity.Id {
	return KindLsvdVolume
}

func (o *LsvdVolume) EntityId() entity.Id {
	return o.ID
}

func (o *LsvdVolume) Encode() (attrs []entity.Attr) {
	if a, ok := lsvd_volumeactual_stateToId[o.ActualState]; ok {
		attrs = append(attrs, entity.Ref(LsvdVolumeActualStateId, a))
	}
	if a, ok := lsvd_volumedesired_stateToId[o.DesiredState]; ok {
		attrs = append(attrs, entity.Ref(LsvdVolumeDesiredStateId, a))
	}
	if !entity.Empty(o.DiskId) {
		attrs = append(attrs, entity.Ref(LsvdVolumeDiskIdId, o.DiskId))
	}
	if !entity.Empty(o.ErrorMessage) {
		attrs = append(attrs, entity.String(LsvdVolumeErrorMessageId, o.ErrorMessage))
	}
	if !entity.Empty(o.Filesystem) {
		attrs = append(attrs, entity.String(LsvdVolumeFilesystemId, o.Filesystem))
	}
	if !entity.Empty(o.Name) {
		attrs = append(attrs, entity.String(LsvdVolumeNameId, o.Name))
	}
	if !entity.Empty(o.NodeId) {
		attrs = append(attrs, entity.Ref(LsvdVolumeNodeIdId, o.NodeId))
	}
	attrs = append(attrs, entity.Bool(LsvdVolumeRemoteOnlyId, o.RemoteOnly))
	attrs = append(attrs, entity.Int64(LsvdVolumeSizeGbId, o.SizeGb))
	if !entity.Empty(o.VolumeId) {
		attrs = append(attrs, entity.String(LsvdVolumeVolumeIdId, o.VolumeId))
	}
	attrs = append(attrs, entity.Ref(entity.EntityKind, KindLsvdVolume))
	return
}

func (o *LsvdVolume) Empty() bool {
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
	if !entity.Empty(o.Name) {
		return false
	}
	if !entity.Empty(o.NodeId) {
		return false
	}
	if !entity.Empty(o.RemoteOnly) {
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

func (o *LsvdVolume) InitSchema(sb *schema.SchemaBuilder) {
	sb.Singleton("dev.miren.storage/actual_state.vol_pending")
	sb.Singleton("dev.miren.storage/actual_state.vol_creating")
	sb.Singleton("dev.miren.storage/actual_state.vol_ready")
	sb.Singleton("dev.miren.storage/actual_state.vol_deleting")
	sb.Singleton("dev.miren.storage/actual_state.vol_deleted")
	sb.Singleton("dev.miren.storage/actual_state.vol_error")
	sb.Ref("actual_state", "dev.miren.storage/lsvd_volume.actual_state", schema.Doc("Current state of the volume (set by lsvd-server)"), schema.Indexed, schema.Choices(LsvdVolumeActualStateVolPendingId, LsvdVolumeActualStateVolCreatingId, LsvdVolumeActualStateVolReadyId, LsvdVolumeActualStateVolDeletingId, LsvdVolumeActualStateVolDeletedId, LsvdVolumeActualStateVolErrorId))
	sb.Singleton("dev.miren.storage/desired_state.vol_present")
	sb.Singleton("dev.miren.storage/desired_state.vol_absent")
	sb.Ref("desired_state", "dev.miren.storage/lsvd_volume.desired_state", schema.Doc("What state should this volume be in"), schema.Indexed, schema.Choices(LsvdVolumeDesiredStateVolPresentId, LsvdVolumeDesiredStateVolAbsentId))
	sb.Ref("disk_id", "dev.miren.storage/lsvd_volume.disk_id", schema.Doc("Reference to the parent Disk entity"), schema.Required, schema.Indexed)
	sb.String("error_message", "dev.miren.storage/lsvd_volume.error_message", schema.Doc("Error details if actual_state is error"))
	sb.String("filesystem", "dev.miren.storage/lsvd_volume.filesystem", schema.Doc("Filesystem type (ext4, xfs, btrfs)"))
	sb.String("name", "dev.miren.storage/lsvd_volume.name", schema.Doc("Human-readable name for the volume (from parent disk)"))
	sb.Ref("node_id", "dev.miren.storage/lsvd_volume.node_id", schema.Doc("Node where this volume should be provisioned"), schema.Required, schema.Indexed)
	sb.Bool("remote_only", "dev.miren.storage/lsvd_volume.remote_only", schema.Doc("If true, use only remote storage"))
	sb.Int64("size_gb", "dev.miren.storage/lsvd_volume.size_gb", schema.Doc("Volume size in gigabytes"), schema.Required)
	sb.String("volume_id", "dev.miren.storage/lsvd_volume.volume_id", schema.Doc("The LSVD volume identifier (generated by lsvd-server)"), schema.Indexed)
}

var (
	KindDisk       = entity.Id("dev.miren.storage/kind.disk")
	KindDiskLease  = entity.Id("dev.miren.storage/kind.disk_lease")
	KindDiskMount  = entity.Id("dev.miren.storage/kind.disk_mount")
	KindDiskVolume = entity.Id("dev.miren.storage/kind.disk_volume")
	KindLsvdVolume = entity.Id("dev.miren.storage/kind.lsvd_volume")
	Schema         = entity.Id("dev.miren.storage/schema.v1alpha")
)

func init() {
	schema.Register("dev.miren.storage", "v1alpha", func(sb *schema.SchemaBuilder) {
		(&Disk{}).InitSchema(sb)
		(&DiskLease{}).InitSchema(sb)
		(&DiskMount{}).InitSchema(sb)
		(&DiskVolume{}).InitSchema(sb)
		(&LsvdVolume{}).InitSchema(sb)
	})
	schema.RegisterEncodedSchema("dev.miren.storage", "v1alpha", []byte("\x1f\x8b\b\x00\x00\x00\x00\x00\x00\xff\xacYY\x8e\xeb\xb6\x16\\\xc8{\x99\xe7\x01\xba\xb8@\xf6#\xd0\xe6\x91L\x8b\x83[\xa4\x15w~\xf3\x93\x04YE\xbas\x83l0\xdf\x01\x0f\a\xd12E\xd1\xce\xfdi\x88RU\x89\xc3a\x95\xcc~\xa5\x92\bx\xa205\x82\x8d \x1bm\xd4Hz\x80\x81I\xaa\xff\xbc\xfc\xef\xe6\xc9\x1b\xfb\xa4\xa1L\x0f\xef\x90;\xdd\"\xecC'\xf0OG\x95 L\u07be\xa0\xeb\x18p\xaa\x7f}\xd91z\xf9$\xaf\xd1\xecG \x06h\xbb{\xc6W\x1d\x93\xb6y>\xc1\x8e\xd1\xd7\x12\xbdc\x1c\xf4\xb36 \x1c=i[:\x05y\x16\x83\xfd\xd3N\x84\x9fA\xbf\xec/\x9d\xbe||\xab6\x13\x9bK\xa7)\\\xcc\x0f\xb9\x97&0\v\x81\x9d\x19;}\xf9\xb4\bD\fN\xc2\x17+\xa3\xe0z\xa2\xed\xa4\xf8Y@\xcb(\x8eD.\xee\xd9\xd1tڌL\xf68!\x99UC)\xa1(\xa0\x00ū\xec$\xfc\xc1ΒM0j\xc2sSa\x89MD\fd\xbf\a\x0e#1j\xcc\r\x14\xd1\t\xe6\xa5\xd4;\xec\xd8\xfc'\x19\x14\xd22\xf2H\x1bA(\x03\xad\x92\xdcUɐ\xde\xc0!\xee\x94\xe2(\xf1ኄf?A\xdb\xef\x90އ\x86\xa5\xee\x9948\xa3\x1f\xac1\r1g\x8d\xc4\xce_gg\xf5/\x80qTc\xae\a\x8e\xd6\xe0\xf3\x031\x86\xec\x0f\x90\xadi\x0f\f\x90\x03\x05\x0e\x86ɾ\x80\r\x90\x03\x85M\xdd\x00a#\xd8'V83\xe5\x1e\x1c1\xc3iT\x13\xd3LI\xa0\x97\xcfW\xf1\t\x8a\xc7k\xfb\x8aL\xd5/)\xa1\x002Ոkp\xbd7Xv[\xf4\xb6^\x99\x92\xfd\xf4\x96\xf0Ӂ\xf0\xd3\xc8\x04\x19\x9f[kS\xd4\xca\xe4f&Z]ˁhp\x86w\xf9\x7f\xbe\x1f\x0eS\xe9{\xbf\xe0\x88\xbe.)5d\xfftf#Ж\x18W\xd8\xe9\r\xac2\xc3\x04\xa0\xd0ge\xa1\xd3)\xccN篽}\"9\xb3j\t\x19/=\xbb\x0f\x8d\x94\xfem\x91\x8ee\xdd\nК\xf4nc\x8b\xeb[K\xefZ\xd9\xe6^N\xa8\xb3t\xb3\x01\xee\xd2\xd2\xd9^\x89\x93\x92 \xcd|\xe5\xd7\xeaV\xadY\xaaU\xae\xd8\xcf8؏r\x1ew\x96\xa6Q'ÔtNЇ\xc6\xd2\xc22\x95\xe3\xd8'b\x0e\xce\xf5\xf0j\xc9˔\xa6\xe3\x8d@\xe8\xec|lnF\xdf+\x16\xbe\x9bÊ\"\x90\x8a\xc6\rևFZ\x04_\x15\xe9\x9aH\xbaS\x97\xa0pL\xdai\x8e\x97\xab\xb8\xd6j_a\xa7\xce2k\xf6\xdeY\xf0y\xd7\x11\xc6!\xbb\xa2\x1e\xe6\x00\xfd\t$\xb5N\x95\xb1\x9f\xe0T\x0eq\x18\x01{Z2\xd9\x00).\xcbq\x1euٕp\xf96\\\xe9\x9e\x1a\xff\r\x97ᛒRC\xf6\xe6Lxk\xc7\xe3\xf63\xbf\xba\x93]\x92\xbf\x0fT\xb4.\x003\x85\x92\xf2\x9b\x00<RỞ\xedВ㡖\x15֫\x82\xe5\xa1\x03\x15m\fތ\x9d-i\x01ky1X+x\x01;\x84\x0e\xdbnV\xf0\x02\x96\xc7w[\xe2w\xb5\x1d\xf5L\xf7\xf6Jf\x04\v*ڳ\x8c\xbd\xfd~\x9b:\xa3_K\xf1ઉ\x82\xc6D\x9b\xcbI\\\xdf\xca\x7f\xa3**\xda\x1f\x894\xb1D\xdedޒ\xea4\v\xc2Sh\xfb\xde\x02\xbd\xbc\xad\x95\x88\x94b\x86\x87\xf1Ml\x0fm\xf4\xf7!\xbd\xb1\xb4\xf9\x8d\xa9\x8a\xa6\x10|T\\ߪ\te'uO(W\f\x92+uj\xdd\xc0\xdc \xd3\x1bK\xa9\xb5\xa4pR\xf8w\x9e\xaec\xd2^\n\xad%\x96\x13\xdaL\xac/\x8b\xf4\xed`\xad\x10)~\x98\xeeXM\b\xa0P\xee\x9bh\x0e\x01'\xebS`\xe5w\x8a\aU\xc6\xc0\xefō\xeb\xa4\x1eʁw\a:\xd5\xe6\x80\aZ\x86\x9d\xfd\xe7\x1a\x06\x02\x8ftj\xf1wOMrD\xa8eU'\xc74'\xc7\xd4\xe2\xa1D\x95\x93\xcf\xd8!\xbc\xb8\x92\x17\xb0\xb80\x19\xfbN\x17\xe6AKetj\xc9N\x834\xd9\x0f\x80k'\fP\x9c\xb5\x11\x90\x95\xab\x97%\xcbcK\xe7\x1dq\x18[\xbf;6\xa6\xe1\xbdy\x9c\xd7\xdb8L\xbaC\x89\t\xd2'\x91pL\xdaK\xa55\x7f\xf1J\xce\x1d\xfd$\x1db\xab\xf2\xe4$\xa8l\x9c\xbbl\xacӦ\xd1n\xf0\x8bg/\xc5\xc4\xf0\x025?\xff\x8bߵ\xd7:\xf1\x8clHo\xe4\xf7\f\x9f\xec\xe7N8-\xcbl\x80D\xa2I\xb1r\x12mzt\x96\xf9\xacZP\x13t16\x86dH\xab\xb9\x91\x1c\x1f\xae\xe7F\x02\xba\xeb\xe7Cf\x1a\x12\xa9\xc7rî\xaa\x0f\x8e\xcc\xe6\xbarˈD\x8e\x8b\x8e\x1a\x0e\"\xed\x9a\xc7\xf0\xd8\xf2\xe5\x04\x8b\xbc\x10\x1f5<\x8f\xe5\xf6:\x06\xc8\xd6wy\n\xe6\xf1\xed\xb5̭\fI\x17\xe9\xc1\f9\xda\xf7\xf8\x10ٌ\x83\x19\xebf\xcf\xc7H\xce\xd6ox\x1b9r5\x94\ar$忏\x1cI\xf5\xeeȑ\xcc\xf6M\x95\xee\xf7\xed+\xf6\x96og\xec2\xe5\xdfq\xec\xbeѓ\x87\x12 \x15\xf8\xcf\a\xc0C\xa2\xb6\x04\x0e\xfa\xa0FӺ\xff\x87\xb9\x93\xe2\xd2?Ū\xcfn\x10\x92~\xe4o\x9f\xf4Ty{\x82I\x87U\x93\x05\xff\x02\x00\x00\xff\xff\x01\x00\x00\xff\xffHt\xaa[\r\x1c\x00\x00"))
}
