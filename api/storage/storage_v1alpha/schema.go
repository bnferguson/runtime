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
)

type Disk struct {
	ID           entity.Id      `json:"id"`
	CreatedBy    entity.Id      `cbor:"created_by,omitempty" json:"created_by,omitempty"`
	Filesystem   DiskFilesystem `cbor:"filesystem,omitempty" json:"filesystem,omitempty"`
	LsvdVolumeId string         `cbor:"lsvd_volume_id,omitempty" json:"lsvd_volume_id,omitempty"`
	Name         string         `cbor:"name" json:"name"`
	RemoteOnly   bool           `cbor:"remote_only,omitempty" json:"remote_only,omitempty"`
	SizeGb       int64          `cbor:"size_gb" json:"size_gb"`
	Status       DiskStatus     `cbor:"status,omitempty" json:"status,omitempty"`
}

type DiskFilesystem string

const (
	EXT4  DiskFilesystem = "filesystem.ext4"
	XFS   DiskFilesystem = "filesystem.xfs"
	BTRFS DiskFilesystem = "filesystem.btrfs"
)

var diskfilesystemFromId = map[entity.Id]DiskFilesystem{DiskFilesystemExt4Id: EXT4, DiskFilesystemXfsId: XFS, DiskFilesystemBtrfsId: BTRFS}
var diskfilesystemToId = map[DiskFilesystem]entity.Id{EXT4: DiskFilesystemExt4Id, XFS: DiskFilesystemXfsId, BTRFS: DiskFilesystemBtrfsId}

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
	if !entity.Empty(o.Name) {
		attrs = append(attrs, entity.String(DiskNameId, o.Name))
	}
	attrs = append(attrs, entity.Bool(DiskRemoteOnlyId, o.RemoteOnly))
	attrs = append(attrs, entity.Int64(DiskSizeGbId, o.SizeGb))
	if a, ok := diskstatusToId[o.Status]; ok {
		attrs = append(attrs, entity.Ref(DiskStatusId, a))
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
	return true
}

func (o *Disk) InitSchema(sb *schema.SchemaBuilder) {
	sb.Ref("created_by", "dev.miren.storage/disk.created_by", schema.Doc("Application that created this disk (for tracking purposes)"), schema.Indexed, schema.Tags("dev.miren.app_ref"))
	sb.Singleton("dev.miren.storage/filesystem.ext4")
	sb.Singleton("dev.miren.storage/filesystem.xfs")
	sb.Singleton("dev.miren.storage/filesystem.btrfs")
	sb.Ref("filesystem", "dev.miren.storage/disk.filesystem", schema.Doc("Filesystem type for the disk"), schema.Choices(DiskFilesystemExt4Id, DiskFilesystemXfsId, DiskFilesystemBtrfsId))
	sb.String("lsvd_volume_id", "dev.miren.storage/disk.lsvd_volume_id", schema.Doc("LSVD backend volume identifier"), schema.Indexed)
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
	LsvdMountActualStateId                  = entity.Id("dev.miren.storage/lsvd_mount.actual_state")
	LsvdMountActualStateMntPendingId        = entity.Id("dev.miren.storage/actual_state.mnt_pending")
	LsvdMountActualStateMntAttachingId      = entity.Id("dev.miren.storage/actual_state.mnt_attaching")
	LsvdMountActualStateMntAttachedId       = entity.Id("dev.miren.storage/actual_state.mnt_attached")
	LsvdMountActualStateMntMountingId       = entity.Id("dev.miren.storage/actual_state.mnt_mounting")
	LsvdMountActualStateMntMountedId        = entity.Id("dev.miren.storage/actual_state.mnt_mounted")
	LsvdMountActualStateMntUnmountingId     = entity.Id("dev.miren.storage/actual_state.mnt_unmounting")
	LsvdMountActualStateMntDetachingId      = entity.Id("dev.miren.storage/actual_state.mnt_detaching")
	LsvdMountActualStateMntDetachedId       = entity.Id("dev.miren.storage/actual_state.mnt_detached")
	LsvdMountActualStateMntErrorId          = entity.Id("dev.miren.storage/actual_state.mnt_error")
	LsvdMountDesiredStateId                 = entity.Id("dev.miren.storage/lsvd_mount.desired_state")
	LsvdMountDesiredStateMntWantMountedId   = entity.Id("dev.miren.storage/desired_state.mnt_want_mounted")
	LsvdMountDesiredStateMntWantUnmountedId = entity.Id("dev.miren.storage/desired_state.mnt_want_unmounted")
	LsvdMountDevicePathId                   = entity.Id("dev.miren.storage/lsvd_mount.device_path")
	LsvdMountDiskLeaseIdId                  = entity.Id("dev.miren.storage/lsvd_mount.disk_lease_id")
	LsvdMountErrorMessageId                 = entity.Id("dev.miren.storage/lsvd_mount.error_message")
	LsvdMountLeaseNonceId                   = entity.Id("dev.miren.storage/lsvd_mount.lease_nonce")
	LsvdMountMountPathId                    = entity.Id("dev.miren.storage/lsvd_mount.mount_path")
	LsvdMountNbdIndexId                     = entity.Id("dev.miren.storage/lsvd_mount.nbd_index")
	LsvdMountNodeIdId                       = entity.Id("dev.miren.storage/lsvd_mount.node_id")
	LsvdMountReadOnlyId                     = entity.Id("dev.miren.storage/lsvd_mount.read_only")
	LsvdMountVolumeIdId                     = entity.Id("dev.miren.storage/lsvd_mount.volume_id")
)

type LsvdMount struct {
	ID           entity.Id             `json:"id"`
	ActualState  LsvdMountActualState  `cbor:"actual_state,omitempty" json:"actual_state,omitempty"`
	DesiredState LsvdMountDesiredState `cbor:"desired_state,omitempty" json:"desired_state,omitempty"`
	DevicePath   string                `cbor:"device_path,omitempty" json:"device_path,omitempty"`
	DiskLeaseId  entity.Id             `cbor:"disk_lease_id,omitempty" json:"disk_lease_id,omitempty"`
	ErrorMessage string                `cbor:"error_message,omitempty" json:"error_message,omitempty"`
	LeaseNonce   string                `cbor:"lease_nonce,omitempty" json:"lease_nonce,omitempty"`
	MountPath    string                `cbor:"mount_path" json:"mount_path"`
	NbdIndex     int64                 `cbor:"nbd_index,omitempty" json:"nbd_index,omitempty"`
	NodeId       entity.Id             `cbor:"node_id" json:"node_id"`
	ReadOnly     bool                  `cbor:"read_only,omitempty" json:"read_only,omitempty"`
	VolumeId     entity.Id             `cbor:"volume_id" json:"volume_id"`
}

type LsvdMountActualState string

const (
	MNT_PENDING    LsvdMountActualState = "actual_state.mnt_pending"
	MNT_ATTACHING  LsvdMountActualState = "actual_state.mnt_attaching"
	MNT_ATTACHED   LsvdMountActualState = "actual_state.mnt_attached"
	MNT_MOUNTING   LsvdMountActualState = "actual_state.mnt_mounting"
	MNT_MOUNTED    LsvdMountActualState = "actual_state.mnt_mounted"
	MNT_UNMOUNTING LsvdMountActualState = "actual_state.mnt_unmounting"
	MNT_DETACHING  LsvdMountActualState = "actual_state.mnt_detaching"
	MNT_DETACHED   LsvdMountActualState = "actual_state.mnt_detached"
	MNT_ERROR      LsvdMountActualState = "actual_state.mnt_error"
)

var lsvd_mountactual_stateFromId = map[entity.Id]LsvdMountActualState{LsvdMountActualStateMntPendingId: MNT_PENDING, LsvdMountActualStateMntAttachingId: MNT_ATTACHING, LsvdMountActualStateMntAttachedId: MNT_ATTACHED, LsvdMountActualStateMntMountingId: MNT_MOUNTING, LsvdMountActualStateMntMountedId: MNT_MOUNTED, LsvdMountActualStateMntUnmountingId: MNT_UNMOUNTING, LsvdMountActualStateMntDetachingId: MNT_DETACHING, LsvdMountActualStateMntDetachedId: MNT_DETACHED, LsvdMountActualStateMntErrorId: MNT_ERROR}
var lsvd_mountactual_stateToId = map[LsvdMountActualState]entity.Id{MNT_PENDING: LsvdMountActualStateMntPendingId, MNT_ATTACHING: LsvdMountActualStateMntAttachingId, MNT_ATTACHED: LsvdMountActualStateMntAttachedId, MNT_MOUNTING: LsvdMountActualStateMntMountingId, MNT_MOUNTED: LsvdMountActualStateMntMountedId, MNT_UNMOUNTING: LsvdMountActualStateMntUnmountingId, MNT_DETACHING: LsvdMountActualStateMntDetachingId, MNT_DETACHED: LsvdMountActualStateMntDetachedId, MNT_ERROR: LsvdMountActualStateMntErrorId}

type LsvdMountDesiredState string

const (
	MNT_WANT_MOUNTED   LsvdMountDesiredState = "desired_state.mnt_want_mounted"
	MNT_WANT_UNMOUNTED LsvdMountDesiredState = "desired_state.mnt_want_unmounted"
)

var lsvd_mountdesired_stateFromId = map[entity.Id]LsvdMountDesiredState{LsvdMountDesiredStateMntWantMountedId: MNT_WANT_MOUNTED, LsvdMountDesiredStateMntWantUnmountedId: MNT_WANT_UNMOUNTED}
var lsvd_mountdesired_stateToId = map[LsvdMountDesiredState]entity.Id{MNT_WANT_MOUNTED: LsvdMountDesiredStateMntWantMountedId, MNT_WANT_UNMOUNTED: LsvdMountDesiredStateMntWantUnmountedId}

func (o *LsvdMount) Decode(e entity.AttrGetter) {
	o.ID = entity.MustGet(e, entity.DBId).Value.Id()
	if a, ok := e.Get(LsvdMountActualStateId); ok && a.Value.Kind() == entity.KindId {
		o.ActualState = lsvd_mountactual_stateFromId[a.Value.Id()]
	}
	if a, ok := e.Get(LsvdMountDesiredStateId); ok && a.Value.Kind() == entity.KindId {
		o.DesiredState = lsvd_mountdesired_stateFromId[a.Value.Id()]
	}
	if a, ok := e.Get(LsvdMountDevicePathId); ok && a.Value.Kind() == entity.KindString {
		o.DevicePath = a.Value.String()
	}
	if a, ok := e.Get(LsvdMountDiskLeaseIdId); ok && a.Value.Kind() == entity.KindId {
		o.DiskLeaseId = a.Value.Id()
	}
	if a, ok := e.Get(LsvdMountErrorMessageId); ok && a.Value.Kind() == entity.KindString {
		o.ErrorMessage = a.Value.String()
	}
	if a, ok := e.Get(LsvdMountLeaseNonceId); ok && a.Value.Kind() == entity.KindString {
		o.LeaseNonce = a.Value.String()
	}
	if a, ok := e.Get(LsvdMountMountPathId); ok && a.Value.Kind() == entity.KindString {
		o.MountPath = a.Value.String()
	}
	if a, ok := e.Get(LsvdMountNbdIndexId); ok && a.Value.Kind() == entity.KindInt64 {
		o.NbdIndex = a.Value.Int64()
	}
	if a, ok := e.Get(LsvdMountNodeIdId); ok && a.Value.Kind() == entity.KindId {
		o.NodeId = a.Value.Id()
	}
	if a, ok := e.Get(LsvdMountReadOnlyId); ok && a.Value.Kind() == entity.KindBool {
		o.ReadOnly = a.Value.Bool()
	}
	if a, ok := e.Get(LsvdMountVolumeIdId); ok && a.Value.Kind() == entity.KindId {
		o.VolumeId = a.Value.Id()
	}
}

func (o *LsvdMount) Is(e entity.AttrGetter) bool {
	return entity.Is(e, KindLsvdMount)
}

func (o *LsvdMount) ShortKind() string {
	return "lsvd_mount"
}

func (o *LsvdMount) Kind() entity.Id {
	return KindLsvdMount
}

func (o *LsvdMount) EntityId() entity.Id {
	return o.ID
}

func (o *LsvdMount) Encode() (attrs []entity.Attr) {
	if a, ok := lsvd_mountactual_stateToId[o.ActualState]; ok {
		attrs = append(attrs, entity.Ref(LsvdMountActualStateId, a))
	}
	if a, ok := lsvd_mountdesired_stateToId[o.DesiredState]; ok {
		attrs = append(attrs, entity.Ref(LsvdMountDesiredStateId, a))
	}
	if !entity.Empty(o.DevicePath) {
		attrs = append(attrs, entity.String(LsvdMountDevicePathId, o.DevicePath))
	}
	if !entity.Empty(o.DiskLeaseId) {
		attrs = append(attrs, entity.Ref(LsvdMountDiskLeaseIdId, o.DiskLeaseId))
	}
	if !entity.Empty(o.ErrorMessage) {
		attrs = append(attrs, entity.String(LsvdMountErrorMessageId, o.ErrorMessage))
	}
	if !entity.Empty(o.LeaseNonce) {
		attrs = append(attrs, entity.String(LsvdMountLeaseNonceId, o.LeaseNonce))
	}
	if !entity.Empty(o.MountPath) {
		attrs = append(attrs, entity.String(LsvdMountMountPathId, o.MountPath))
	}
	if !entity.Empty(o.NbdIndex) {
		attrs = append(attrs, entity.Int64(LsvdMountNbdIndexId, o.NbdIndex))
	}
	if !entity.Empty(o.NodeId) {
		attrs = append(attrs, entity.Ref(LsvdMountNodeIdId, o.NodeId))
	}
	attrs = append(attrs, entity.Bool(LsvdMountReadOnlyId, o.ReadOnly))
	if !entity.Empty(o.VolumeId) {
		attrs = append(attrs, entity.Ref(LsvdMountVolumeIdId, o.VolumeId))
	}
	attrs = append(attrs, entity.Ref(entity.EntityKind, KindLsvdMount))
	return
}

func (o *LsvdMount) Empty() bool {
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
	if !entity.Empty(o.LeaseNonce) {
		return false
	}
	if !entity.Empty(o.MountPath) {
		return false
	}
	if !entity.Empty(o.NbdIndex) {
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

func (o *LsvdMount) InitSchema(sb *schema.SchemaBuilder) {
	sb.Singleton("dev.miren.storage/actual_state.mnt_pending")
	sb.Singleton("dev.miren.storage/actual_state.mnt_attaching")
	sb.Singleton("dev.miren.storage/actual_state.mnt_attached")
	sb.Singleton("dev.miren.storage/actual_state.mnt_mounting")
	sb.Singleton("dev.miren.storage/actual_state.mnt_mounted")
	sb.Singleton("dev.miren.storage/actual_state.mnt_unmounting")
	sb.Singleton("dev.miren.storage/actual_state.mnt_detaching")
	sb.Singleton("dev.miren.storage/actual_state.mnt_detached")
	sb.Singleton("dev.miren.storage/actual_state.mnt_error")
	sb.Ref("actual_state", "dev.miren.storage/lsvd_mount.actual_state", schema.Doc("Current state of the mount (set by lsvd-server)"), schema.Indexed, schema.Choices(LsvdMountActualStateMntPendingId, LsvdMountActualStateMntAttachingId, LsvdMountActualStateMntAttachedId, LsvdMountActualStateMntMountingId, LsvdMountActualStateMntMountedId, LsvdMountActualStateMntUnmountingId, LsvdMountActualStateMntDetachingId, LsvdMountActualStateMntDetachedId, LsvdMountActualStateMntErrorId))
	sb.Singleton("dev.miren.storage/desired_state.mnt_want_mounted")
	sb.Singleton("dev.miren.storage/desired_state.mnt_want_unmounted")
	sb.Ref("desired_state", "dev.miren.storage/lsvd_mount.desired_state", schema.Doc("What state should this mount be in"), schema.Indexed, schema.Choices(LsvdMountDesiredStateMntWantMountedId, LsvdMountDesiredStateMntWantUnmountedId))
	sb.String("device_path", "dev.miren.storage/lsvd_mount.device_path", schema.Doc("Full path to the device node (set by lsvd-server)"))
	sb.Ref("disk_lease_id", "dev.miren.storage/lsvd_mount.disk_lease_id", schema.Doc("Reference to the parent DiskLease entity"), schema.Indexed)
	sb.String("error_message", "dev.miren.storage/lsvd_mount.error_message", schema.Doc("Error details if actual_state is error"))
	sb.String("lease_nonce", "dev.miren.storage/lsvd_mount.lease_nonce", schema.Doc("Volume lease nonce from remote Disk API"))
	sb.String("mount_path", "dev.miren.storage/lsvd_mount.mount_path", schema.Doc("Path where the volume should be mounted"), schema.Required)
	sb.Int64("nbd_index", "dev.miren.storage/lsvd_mount.nbd_index", schema.Doc("NBD device index (set by lsvd-server)"))
	sb.Ref("node_id", "dev.miren.storage/lsvd_mount.node_id", schema.Doc("Node where this mount exists"), schema.Required, schema.Indexed)
	sb.Bool("read_only", "dev.miren.storage/lsvd_mount.read_only", schema.Doc("Whether the mount is read-only"))
	sb.Ref("volume_id", "dev.miren.storage/lsvd_mount.volume_id", schema.Doc("Reference to the lsvd_volume entity"), schema.Required, schema.Indexed)
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
	KindLsvdMount  = entity.Id("dev.miren.storage/kind.lsvd_mount")
	KindLsvdVolume = entity.Id("dev.miren.storage/kind.lsvd_volume")
	Schema         = entity.Id("dev.miren.storage/schema.v1alpha")
)

func init() {
	schema.Register("dev.miren.storage", "v1alpha", func(sb *schema.SchemaBuilder) {
		(&Disk{}).InitSchema(sb)
		(&DiskLease{}).InitSchema(sb)
		(&LsvdMount{}).InitSchema(sb)
		(&LsvdVolume{}).InitSchema(sb)
	})
	schema.RegisterEncodedSchema("dev.miren.storage", "v1alpha", []byte("\x1f\x8b\b\x00\x00\x00\x00\x00\x00\xff\xacX[Ҭ&\x10^H\xee\xf7\xbb\xb9T\xf6c\xe1\xd0:\x8c\b\x1e`&N^\xf3\x90T\xb2\x8b\x9c\xa9\xbf*\x1b\xccs\x8a\x06\x14\x1dDΩ\xbcX\x80\xdf\xf7\xd9\xd0M\xb7\xf0\xa0\x82\f\xf0\x8a\u00ad\x1a\x98\x02Qi#\x15\xe9\x00z&\xa8~L\xef<\xbd\xf9\u07be\xa9(\xd3\xfd\vro\xcf\b\xfb\xd2\t\xfc\xdbR9\x10&\x9e?ж\f8տ\xbfn\x18\x9d>JkT'\x05\xc4\x00\xad\x9b;~\xea\x12\xf5\xcd}\x84\x86\xd1G\x8e\xde2\x0e\xfa\xae\r\f\x8e\x1e\xf5-\x9d\x82\xb8\x0e\xbd}\xd47¯\xa0_\x9f\xa6VO\x1f>\xab-\xc4jj5\x85\xc9\xfc\x9c\xfah\x04\xb3\x10h\x8cj\xf5\xf4q\x16\x88\x18\\\x84\xcfvf\xc1\xf5\x8d\xd67ɯ\x03Ԍ\xe2L\xc4f\xccΦ\xd5F1ѡT\xc2k(e\xb9tyli\tK\x91\xa6`\x90\x06j)\xb8\xf3C\x1f\x0f\xe0J6Rr\x94x\x7fGB\xb3_\xa1\xee\x1a\xa4w\xa1c\xa9'&\f:\xf1\xbd=\xa6!檑\xd8\xfav\xd2y/\x00JI\x95\xb2\xc0\xd1*|\x7f&Ɛ\xd3\x19\x92Q\xe3\x81\x01r\xa6\xc0\xc10\xd1e\xb0\x01r\xa6p\xa8\x1b \xfd\xa8\xe4\x8di&\x05\xd0\xe9\xd3]x\x84\xe2s\xdbZ\x93\x88\x94-\x85\x89\xae\xbb\x81\xb2\xcd\xee\xf6#\xe1\xe3\x99\xf0Q\xb1\x81\xa8{mw&\xb5k\x9b2u\xde\xdd5\a\xa2\xc1\xed\xf1\xe9ݴs\x1c\xa6p\xab\xff\x81\x01\xf2eN\xa9\"\xa7WW\xa6\x80\xd6ĸH\x8b\a\xd0\xed\x86\r\x80B\x9f\xe4\x85\xc61l\x96ַ}\xc6@rb\xd1#26=\xbb\v\x9d\x98\xfeu\x96\x8eqV\x0f\xa05\xe9\xdcN\x1b\xd6CѾ{d\xf6\x9d\x97\x1b\xe4U\xb8\xd5\x00״tv\x92\xc3(\x05\b\xb3\xb4\xbc\xaf\x9eժ\xadZ\xa1\xc7~\xc3\xc9~\xf0l\x1d\x8aTr4L\n\xb75\xbb\xd0\xd9\xe6\x94D\xe48\xf6H\xcc٥!lmy\x89\xd0t<\x05\x84.\xa9\x88-\xdd9\x11e\x03߭aA\x10\bI\xe7|ۅN\x1c\x04_d\xe9\x9a\b\xda\xc8)(\\\xa2~\\\xba\xf2Q\\\x9a\xfb\x1e\xd0ȫHf_\x9f\x18\xf0}\xdb\x12\xc6!\xe9Q\x0fs\x80n\x04Am\xa2I\x94\u0090h\x1c\xe2\xac\x00-\xcde\xbd\x00ɺ\xe5\xb2\xccz7+a\xd1C\xf7\xedg\xa5\x05S\x18\xe3\x7f\xa1\x1b\xbe\xca)U\xe4d\xae\x84\xd7v>n?\xf3\xd5H\xd2%\xff\xb0A\x98ڕ\xa4D\u038b\x05\xaa\x19\xd9ۖ\xb3>\x99d\x9eX\x1e\x8b\xbc\xe0\xb4\x12\x9e\xc7r۞\v\xe27\x05\xc4\x00F\xe6\\\xf2J\x98\x01\xccg\xbb\xad\xb1%\xcc\x00\x1e\x16\x03,\xf5\xdbbs\x03יPʝ\xd1\xc2\xf6\xaeb\xb6\xf9\xbb\x02\xf2\x02\x7f\xec\x14\x8c(\xbe(h\xacqK\x80\r\xeb\xa1d\x84\xfd=\xda\x0f\xfdB\xa2\x90\xf9!\x91Mb\xa1j\xcbP\xf3\x807\x18\xe8\xf4S\xb1\xc8\xcc٫\xec\xab9\xde\xd8\t\xea9\xeb\xf7\xf1\xc06\xf9\x1f,ל*Bv\x1d\xd6C\a\xa5:\x92z\x93R]0Ig\x81\x90\xe2\xe4\xc4\xfax`+\x95\xa8\x1f\x91\x14>\x97\xe5\xbaD\xfd\xad\xd0\xe7Y!\xd1\xd0\xdaֽ\xc9\xd5˥\x1b\xfe\xbe\xf7ja\xacqT\v\xf3&\x1c\x97\xec\x02\x91\xf5\t\x88\xad\x0f?\r;(/\x8bP\xeaok)/N\xd6חđ$\x02\x15\x16\x98?\xb3\t\xc0I\xbdU\x85y\xb1kPXaf$r\xac\x03\xeeE\x1cD\xf6\xb6\x85眂\xaa\x14a\x91WZ\x95\",\xb7m<\xea\x97T\x88\x18\xcc篗2\x03\x18\x9d\x94 \xc4Nz\xcb4}\xb1\xdf!\x8d\x06a\x92\a\x87Un]\xb0n\xf5\x14 /a\xd93σ\xf7.\x13VS9:\xe1\x1c,\xc5\xff\x967\xbd\xde\xc1M\xcd\xc1%E\xactp\xc5q\xb0.\x87\x89n\xefG\xd1\xf3\xdf\xe0\xa6\xe4\xc0\x92\xec\x85I\xb6~x\x81l\xbe\xf4K\x92͙}\xa4\xb6\x05\xf6\xfa,\x95\xa9\xdd%\xa1\xbbK\xc8\xdd\x14\x96\xfc\xdd/\x908Y\x1f\x9f\x05b3Kr\xfb\x7f\x00\x00\x00\xff\xff\x01\x00\x00\xff\xff\xdf,\x8f\xef\xf2\x14\x00\x00"))
}
