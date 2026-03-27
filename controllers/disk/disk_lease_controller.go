package disk

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"miren.dev/runtime/api/entityserver"
	"miren.dev/runtime/api/entityserver/entityserver_v1alpha"
	"miren.dev/runtime/api/storage/storage_v1alpha"
	"miren.dev/runtime/pkg/entity"
	"miren.dev/runtime/pkg/idgen"
)

// leaseInfo tracks active lease details
type leaseInfo struct {
	leaseId   string
	diskId    string
	sandboxId string
	volumeId  string // Store volume ID to avoid lookups during delete
}

// DiskLeaseController manages disk lease entities and exclusive access.
// It uses disk_mount entities to coordinate mount operations via loop devices.
type DiskLeaseController struct {
	Log *slog.Logger
	EAC *entityserver_v1alpha.EntityAccessClient

	// NodeId is the ID of this node, used for creating disk_mount entities
	NodeId string

	// Base path for disk mounts (e.g., /var/lib/miren/disks)
	mountBasePath string

	// Track active leases: diskId -> leaseId
	mu           sync.RWMutex
	activeLeases map[string]string
	leaseDetails map[string]*leaseInfo

	// configuredMode is the disk mode from server config ("", "auto", "universal", "accelerator")
	configuredMode string

	// diskMode determines how disk mounts are performed (universal or accelerator)
	diskMode storage_v1alpha.DiskMode
}

// NewDiskLeaseController creates a disk lease controller that uses disk_mount entities.
// The diskMode parameter comes from server config (MIREN_DISK_MODE); pass "" for auto-detection.
func NewDiskLeaseController(log *slog.Logger, eac *entityserver_v1alpha.EntityAccessClient, nodeId string, diskMode string) *DiskLeaseController {
	return &DiskLeaseController{
		Log:            log.With("module", "disk-lease"),
		EAC:            eac,
		NodeId:         nodeId,
		mountBasePath:  "/var/lib/miren/disks",
		activeLeases:   make(map[string]string),
		leaseDetails:   make(map[string]*leaseInfo),
		configuredMode: diskMode,
	}
}

// ForceUniversalMode forces the controller to use disk_mount entities with
// loop devices. This is used by integration tests.
func (d *DiskLeaseController) ForceUniversalMode() {
	d.diskMode = storage_v1alpha.UNIVERSAL
}

// Init initializes the disk lease controller
func (d *DiskLeaseController) Init(ctx context.Context) error {
	d.diskMode = detectDiskMode(d.configuredMode)
	d.Log.Info("disk lease controller initialized", "mode", d.diskMode)
	return nil
}

// Create handles creation of a new disk lease entity
func (d *DiskLeaseController) Create(ctx context.Context, lease *storage_v1alpha.DiskLease, meta *entity.Meta) error {
	d.Log.Info("Processing lease creation",
		"lease", lease.ID,
		"disk", lease.DiskId,
		"status", lease.Status)

	return d.reconcileLease(ctx, lease, meta)
}

// Update handles updates to an existing disk lease entity
func (d *DiskLeaseController) Update(ctx context.Context, lease *storage_v1alpha.DiskLease, meta *entity.Meta) error {
	d.Log.Info("Processing lease update",
		"lease", lease.ID,
		"disk", lease.DiskId,
		"status", lease.Status)

	return d.reconcileLease(ctx, lease, meta)
}

// Delete handles deletion of a disk lease entity
func (d *DiskLeaseController) Delete(ctx context.Context, id entity.Id, obj *storage_v1alpha.DiskLease) error {
	d.Log.Info("Processing lease deletion", "lease", id)

	leaseId := id.String()

	// Get lease details before cleaning up
	d.mu.Lock()
	details, hasDetails := d.leaseDetails[leaseId]
	d.mu.Unlock()

	// Clean up disk_mount for this lease
	d.cleanupDiskMountForLease(ctx, id, leaseId)

	// Release the lease from local tracking
	d.mu.Lock()
	defer d.mu.Unlock()

	if hasDetails {
		delete(d.activeLeases, details.diskId)
		delete(d.leaseDetails, leaseId)
		d.Log.Info("Lease released and cleaned up", "lease", id, "disk", details.diskId)
	}

	return nil
}

func (d *DiskLeaseController) cleanupDiskMountForLease(ctx context.Context, leaseId entity.Id, leaseIdStr string) {
	mount, err := d.getDiskMountForLease(ctx, leaseId)
	if err != nil {
		d.Log.Warn("error looking up disk_mount for deleted lease", "lease", leaseIdStr, "error", err)
		return
	}
	if mount == nil {
		return
	}

	if mount.ActualState != storage_v1alpha.DM_DETACHED {
		if mount.DesiredState != storage_v1alpha.DM_WANT_UNMOUNTED {
			d.Log.Info("setting disk_mount desired_state to unmounted for deleted lease",
				"lease", leaseIdStr,
				"disk_mount", mount.ID)

			updateAttrs := []entity.Attr{
				entity.Ref(entity.DBId, mount.ID),
				entity.Ref(storage_v1alpha.DiskMountDesiredStateId, storage_v1alpha.DiskMountDesiredStateDmWantUnmountedId),
			}
			if _, err := d.EAC.Patch(ctx, updateAttrs, 0); err != nil {
				d.Log.Warn("failed to update disk_mount desired_state",
					"disk_mount", mount.ID,
					"error", err)
			}
		}
	} else {
		d.Log.Info("deleting disk_mount entity for deleted lease",
			"lease", leaseIdStr,
			"disk_mount", mount.ID)

		if _, err := d.EAC.Delete(ctx, mount.ID.String()); err != nil {
			d.Log.Warn("failed to delete disk_mount entity",
				"disk_mount", mount.ID,
				"error", err)
		}
	}
}

// reconcileLease reconciles the lease state
func (d *DiskLeaseController) reconcileLease(ctx context.Context, lease *storage_v1alpha.DiskLease, meta *entity.Meta) error {
	// Only reconcile leases assigned to this node
	myNodeId := entity.Id("node/" + d.NodeId)
	if lease.NodeId != "" && lease.NodeId != myNodeId {
		return nil
	}

	var err error

	switch lease.Status {
	case storage_v1alpha.PENDING:
		err = d.handlePendingLease(ctx, lease)
	case storage_v1alpha.RELEASED:
		err = d.handleReleasedLease(ctx, lease)
	case storage_v1alpha.BOUND:
		// Verify disk is actually mounted, mount if needed
		err = d.handleBoundLease(ctx, lease)
	case storage_v1alpha.FAILED:
		err = d.handleFailedLease(ctx, lease)
	default:
		d.Log.Warn("Unknown lease status", "lease", lease.ID, "status", lease.Status)
		return nil
	}

	// Update entity attributes if any changes
	if meta != nil {
		// Ensure meta.Entity is initialized
		if meta.Entity == nil {
			meta.Entity = entity.New(lease.Encode())
		} else {
			// Update meta.Entity with the new attributes
			meta.Entity.Update(lease.Encode())
		}
	}

	return err
}

// cleanupLeaseReservation removes a lease reservation (used when binding fails)
func (d *DiskLeaseController) cleanupLeaseReservation(diskId string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.activeLeases, diskId)
}

// handlePendingLease attempts to bind a pending lease via disk_mount entity
func (d *DiskLeaseController) handlePendingLease(ctx context.Context, lease *storage_v1alpha.DiskLease) error {
	diskId := lease.DiskId.String()
	leaseId := lease.ID.String()

	// Check if disk is already leased (with lock)
	d.mu.Lock()
	if existingLease, exists := d.activeLeases[diskId]; exists && existingLease != leaseId {
		d.Log.Info("disk has active lease being released, will retry",
			"disk", diskId,
			"requested_lease", leaseId,
			"existing_lease", existingLease)
		d.mu.Unlock()
		return nil
	}

	// Reserve the lease (or confirm existing reservation)
	d.activeLeases[diskId] = leaseId
	d.mu.Unlock()

	// Get the disk entity to find the volume ID
	diskEntity, err := d.EAC.Get(ctx, diskId)
	if err != nil {
		d.Log.Error("Failed to get disk entity", "disk", diskId, "error", err)
		d.cleanupLeaseReservation(diskId)

		lease.Status = storage_v1alpha.FAILED
		lease.ErrorMessage = fmt.Sprintf("Failed to get disk entity: %v", err)

		return nil
	}

	// Decode disk entity
	disk := &storage_v1alpha.Disk{}
	disk.Decode(diskEntity.Entity().Entity())
	if disk.ID == "" {
		d.Log.Error("Failed to decode disk entity", "disk", diskId)
		d.cleanupLeaseReservation(diskId)

		lease.Status = storage_v1alpha.FAILED
		lease.ErrorMessage = "Failed to decode disk entity"

		return nil
	}

	// Check disk provisioning status
	if disk.Status != storage_v1alpha.PROVISIONED {
		if disk.Status == storage_v1alpha.PROVISIONING || disk.Status == storage_v1alpha.RESTORING {
			d.cleanupLeaseReservation(diskId)
			d.Log.Info("Disk is still provisioning, lease will retry",
				"disk", diskId,
				"lease", leaseId,
				"disk_status", disk.Status)
			return nil
		}

		d.cleanupLeaseReservation(diskId)

		lease.Status = storage_v1alpha.FAILED
		lease.ErrorMessage = fmt.Sprintf("Disk is not provisioned, status: %s", disk.Status)

		return nil
	}

	volumeId := disk.VolumeId
	if volumeId == "" {
		d.cleanupLeaseReservation(diskId)
		d.Log.Info("Disk has no volume ID yet, lease will retry",
			"disk", diskId,
			"lease", leaseId)
		return nil
	}

	// Check if a disk_mount entity already exists for this lease
	existingMount, err := d.getDiskMountForLease(ctx, lease.ID)
	if err != nil {
		d.Log.Warn("Error looking up existing disk_mount", "lease", leaseId, "error", err)
	}

	if existingMount != nil {
		d.Log.Debug("Found existing disk_mount for lease",
			"lease", leaseId,
			"disk_mount", existingMount.ID,
			"actual_state", existingMount.ActualState)

		switch existingMount.ActualState {
		case storage_v1alpha.DM_MOUNTED:
			d.mu.Lock()
			d.leaseDetails[leaseId] = &leaseInfo{
				leaseId:   leaseId,
				diskId:    diskId,
				sandboxId: lease.SandboxId.String(),
				volumeId:  volumeId,
			}
			d.mu.Unlock()

			lease.Status = storage_v1alpha.BOUND
			lease.ErrorMessage = ""
			lease.AcquiredAt = time.Now()

			d.Log.Info("Lease bound via disk_mount entity",
				"lease", leaseId,
				"disk_mount", existingMount.ID)
			return nil

		case storage_v1alpha.DM_ERROR:
			d.Log.Warn("disk_mount in error state",
				"lease", leaseId,
				"disk_mount", existingMount.ID,
				"error", existingMount.ErrorMessage)
			d.cleanupLeaseReservation(diskId)

			lease.Status = storage_v1alpha.FAILED
			lease.ErrorMessage = fmt.Sprintf("Mount failed: %s", existingMount.ErrorMessage)
			return nil

		case storage_v1alpha.DM_DETACHED:
			d.Log.Info("existing disk_mount in DETACHED state, deleting stale mount",
				"lease", leaseId,
				"disk_mount", existingMount.ID)
			if _, err := d.EAC.Delete(ctx, existingMount.ID.String()); err != nil {
				d.Log.Warn("failed to delete stale disk_mount, aborting mount creation",
					"disk_mount", existingMount.ID,
					"error", err)
				d.cleanupLeaseReservation(diskId)
				return nil
			}
			// Fall through to create a new mount entity

		default:
			d.Log.Debug("disk_mount still in progress",
				"lease", leaseId,
				"disk_mount", existingMount.ID,
				"actual_state", existingMount.ActualState)
			return nil
		}
	}

	// Find the disk_volume entity for this disk
	diskVolume, err := d.getDiskVolumeForDisk(ctx, disk.ID)
	if err != nil {
		d.Log.Error("Failed to look up disk_volume", "disk", diskId, "error", err)
		d.cleanupLeaseReservation(diskId)

		lease.Status = storage_v1alpha.FAILED
		lease.ErrorMessage = fmt.Sprintf("Failed to look up disk_volume: %v", err)
		return nil
	}

	if diskVolume == nil {
		d.Log.Error("No disk_volume found for disk", "disk", diskId)
		d.cleanupLeaseReservation(diskId)

		lease.Status = storage_v1alpha.FAILED
		lease.ErrorMessage = "No disk_volume entity found for disk"
		return nil
	}

	if diskVolume.ActualState != storage_v1alpha.DV_READY {
		d.cleanupLeaseReservation(diskId)
		d.Log.Info("disk_volume not ready, lease will retry",
			"disk", diskId,
			"disk_volume", diskVolume.ID,
			"actual_state", diskVolume.ActualState)
		return nil
	}

	// Create new disk_mount entity
	mountPath := d.getDiskMountPath(volumeId)

	diskMount := &storage_v1alpha.DiskMount{
		VolumeId:     diskVolume.ID,
		DiskLeaseId:  lease.ID,
		MountPath:    mountPath,
		ReadOnly:     lease.Mount.ReadOnly,
		DesiredState: storage_v1alpha.DM_WANT_MOUNTED,
		ActualState:  storage_v1alpha.DM_PENDING,
		NodeId:       entity.Id("node/" + d.NodeId),
	}

	d.Log.Info("Creating disk_mount entity",
		"lease", leaseId,
		"disk_volume", diskVolume.ID,
		"mount_path", mountPath,
		"read_only", lease.Mount.ReadOnly,
		"node_id", d.NodeId)

	mountId := idgen.GenNS("disk-mnt")
	mountEntityId := entity.Id("disk_mount/" + mountId)
	createAttrs := entity.New(
		entity.DBId, mountEntityId,
		diskMount.Encode,
	).Attrs()

	_, err = d.EAC.Create(ctx, createAttrs)
	if err != nil {
		d.Log.Error("Failed to create disk_mount entity", "error", err)
		d.cleanupLeaseReservation(diskId)

		lease.Status = storage_v1alpha.FAILED
		lease.ErrorMessage = fmt.Sprintf("Failed to create disk_mount entity: %v", err)
		return nil
	}

	d.Log.Info("Created disk_mount entity, waiting for mount controller to mount",
		"lease", leaseId)

	return nil
}

// handleBoundLease verifies a bound lease has a mounted disk_mount entity
func (d *DiskLeaseController) handleBoundLease(ctx context.Context, lease *storage_v1alpha.DiskLease) error {
	leaseId := lease.ID.String()
	diskId := lease.DiskId.String()

	// First, ensure this bound lease is tracked as active (EAS is source of truth)
	d.mu.Lock()
	currentLease, hasLease := d.activeLeases[diskId]

	if !hasLease || currentLease != leaseId {
		if hasLease && currentLease != leaseId {
			d.mu.Unlock()

			lease.Status = storage_v1alpha.FAILED
			lease.ErrorMessage = fmt.Sprintf("Lease conflict detected, disk %s was leased by %s but now bound to %s", diskId, currentLease, leaseId)

			d.Log.Error("Lease conflict detected when tracking bound lease",
				"disk", diskId,
				"requested_lease", leaseId,
				"existing_lease", currentLease)

			return nil
		}

		d.activeLeases[diskId] = leaseId
		d.leaseDetails[leaseId] = &leaseInfo{
			leaseId:   leaseId,
			diskId:    diskId,
			sandboxId: lease.SandboxId.String(),
		}
	}
	d.mu.Unlock()

	diskMount, err := d.getDiskMountForLease(ctx, lease.ID)
	if err != nil {
		d.Log.Warn("Error looking up disk_mount for bound lease", "lease", leaseId, "error", err)
		return nil
	}

	if diskMount == nil {
		d.Log.Warn("Bound lease has no disk_mount entity, reverting to pending",
			"lease", leaseId)
		lease.Status = storage_v1alpha.PENDING
		return nil
	}

	d.mu.Lock()
	if details, exists := d.leaseDetails[leaseId]; exists {
		details.volumeId = string(diskMount.VolumeId)
	}
	d.mu.Unlock()

	if diskMount.ActualState != storage_v1alpha.DM_MOUNTED {
		if diskMount.ActualState == storage_v1alpha.DM_ERROR {
			lease.Status = storage_v1alpha.FAILED
			lease.ErrorMessage = fmt.Sprintf("Mount failed: %s", diskMount.ErrorMessage)
		} else if diskMount.ActualState == storage_v1alpha.DM_DETACHED {
			d.Log.Warn("disk_mount detached for bound lease, reverting to pending",
				"lease", leaseId,
				"disk_mount", diskMount.ID)
			lease.Status = storage_v1alpha.PENDING
		} else {
			d.Log.Debug("disk_mount not yet mounted for bound lease",
				"lease", leaseId,
				"disk_mount", diskMount.ID,
				"actual_state", diskMount.ActualState)
		}
	}

	return nil
}

// handleReleasedLease sets desired_state=DM_WANT_UNMOUNTED on the disk_mount entity
func (d *DiskLeaseController) handleReleasedLease(ctx context.Context, lease *storage_v1alpha.DiskLease) error {
	leaseId := lease.ID.String()
	diskId := lease.DiskId.String()

	// Check if this lease is currently active
	d.mu.Lock()
	currentLease, exists := d.activeLeases[diskId]
	isActiveForThisLease := exists && currentLease == leaseId

	if isActiveForThisLease {
		d.releaseLease(leaseId, diskId)
	}
	d.mu.Unlock()

	if !isActiveForThisLease {
		return nil
	}

	diskMount, err := d.getDiskMountForLease(ctx, lease.ID)
	if err != nil {
		d.Log.Warn("Error looking up disk_mount for released lease", "lease", leaseId, "error", err)
		return nil
	}

	if diskMount == nil {
		return nil
	}

	if diskMount.ActualState == storage_v1alpha.DM_DETACHED {
		d.Log.Info("disk_mount already detached, cleaning up",
			"lease", leaseId,
			"disk_mount", diskMount.ID)

		if _, err := d.EAC.Delete(ctx, diskMount.ID.String()); err != nil {
			d.Log.Warn("Failed to delete disk_mount entity",
				"disk_mount", diskMount.ID,
				"error", err)
		}
	} else if diskMount.DesiredState != storage_v1alpha.DM_WANT_UNMOUNTED {
		d.Log.Info("Setting disk_mount desired_state to unmounted",
			"lease", leaseId,
			"disk_mount", diskMount.ID)

		updateAttrs := []entity.Attr{
			entity.Ref(entity.DBId, diskMount.ID),
			entity.Ref(storage_v1alpha.DiskMountDesiredStateId, storage_v1alpha.DiskMountDesiredStateDmWantUnmountedId),
		}
		if _, err := d.EAC.Patch(ctx, updateAttrs, 0); err != nil {
			d.Log.Error("Failed to update disk_mount desired_state",
				"disk_mount", diskMount.ID,
				"error", err)
		}
	} else {
		d.Log.Debug("disk_mount already marked for unmount",
			"lease", leaseId,
			"disk_mount", diskMount.ID,
			"actual_state", diskMount.ActualState)
	}

	return nil
}

// handleFailedLease cleans up the disk_mount entity for a failed lease.
func (d *DiskLeaseController) handleFailedLease(ctx context.Context, lease *storage_v1alpha.DiskLease) error {
	leaseId := lease.ID.String()
	diskId := lease.DiskId.String()

	// Release from active tracking if still tracked
	d.mu.Lock()
	currentLease, exists := d.activeLeases[diskId]
	if exists && currentLease == leaseId {
		d.releaseLease(leaseId, diskId)
	}
	d.mu.Unlock()

	diskMount, err := d.getDiskMountForLease(ctx, lease.ID)
	if err != nil {
		d.Log.Warn("Error looking up disk_mount for failed lease", "lease", leaseId, "error", err)
		return nil
	}

	if diskMount == nil {
		return nil
	}

	if diskMount.ActualState == storage_v1alpha.DM_DETACHED {
		d.Log.Info("disk_mount already detached for failed lease, cleaning up",
			"lease", leaseId,
			"disk_mount", diskMount.ID)

		if _, err := d.EAC.Delete(ctx, diskMount.ID.String()); err != nil {
			d.Log.Warn("Failed to delete disk_mount entity",
				"disk_mount", diskMount.ID,
				"error", err)
		}
	} else if diskMount.DesiredState != storage_v1alpha.DM_WANT_UNMOUNTED {
		d.Log.Info("Setting disk_mount desired_state to unmounted for failed lease",
			"lease", leaseId,
			"disk_mount", diskMount.ID)

		updateAttrs := []entity.Attr{
			entity.Ref(entity.DBId, diskMount.ID),
			entity.Ref(storage_v1alpha.DiskMountDesiredStateId, storage_v1alpha.DiskMountDesiredStateDmWantUnmountedId),
		}
		if _, err := d.EAC.Patch(ctx, updateAttrs, 0); err != nil {
			d.Log.Error("Failed to update disk_mount desired_state",
				"disk_mount", diskMount.ID,
				"error", err)
		}
	}

	return nil
}

// releaseLease removes a lease from active tracking (must be called with lock held)
func (d *DiskLeaseController) releaseLease(leaseId, diskId string) {
	if currentLease, exists := d.activeLeases[diskId]; exists && currentLease == leaseId {
		delete(d.activeLeases, diskId)
		delete(d.leaseDetails, leaseId)
		d.Log.Info("Lease released", "lease", leaseId, "disk", diskId)
	}
}

// getDiskMountPath returns the standard mount path for a disk volume
func (d *DiskLeaseController) getDiskMountPath(volumeId string) string {
	return filepath.Join(d.mountBasePath, volumeId)
}

// getDiskMountForLease finds the disk_mount entity for a lease
func (d *DiskLeaseController) getDiskMountForLease(ctx context.Context, leaseId entity.Id) (*storage_v1alpha.DiskMount, error) {
	if d.EAC == nil {
		return nil, nil
	}

	indexAttr := entity.Ref(storage_v1alpha.DiskMountDiskLeaseIdId, leaseId)

	resp, err := d.EAC.List(ctx, indexAttr)
	if err != nil {
		return nil, fmt.Errorf("failed to list disk_mount entities: %w", err)
	}

	values := resp.Values()
	if len(values) == 0 {
		return nil, nil
	}

	var mount storage_v1alpha.DiskMount
	mount.Decode(values[0].Entity())

	return &mount, nil
}

// getDiskVolumeForDisk finds the disk_volume entity for a disk
func (d *DiskLeaseController) getDiskVolumeForDisk(ctx context.Context, diskId entity.Id) (*storage_v1alpha.DiskVolume, error) {
	if d.EAC == nil {
		return nil, nil
	}

	indexAttr := entity.Ref(storage_v1alpha.DiskVolumeDiskIdId, diskId)

	resp, err := d.EAC.List(ctx, indexAttr)
	if err != nil {
		return nil, fmt.Errorf("failed to list disk_volume entities: %w", err)
	}

	values := resp.Values()
	if len(values) == 0 {
		return nil, nil
	}

	var volume storage_v1alpha.DiskVolume
	volume.Decode(values[0].Entity())

	return &volume, nil
}

// CleanupOldReleasedLeases deletes released leases that haven't been updated for over 1 hour
func (d *DiskLeaseController) CleanupOldReleasedLeases(ctx context.Context) error {
	if d.EAC == nil {
		return nil
	}

	// List all disk lease entities
	ref := entity.Ref(entity.EntityKind, storage_v1alpha.KindDiskLease)
	results, err := d.EAC.List(ctx, ref)
	if err != nil {
		d.Log.Error("Failed to list disk leases for cleanup", "error", err)
		return err
	}

	now := time.Now()
	cutoffTime := now.Add(-1 * time.Hour) // 1 hour ago
	deletedCount := 0

	for _, e := range results.Values() {
		// Decode the lease to check its status
		var lease storage_v1alpha.DiskLease
		lease.Decode(e.Entity())

		if lease.Status == storage_v1alpha.RELEASED && e.Entity().GetUpdatedAt().Before(cutoffTime) {
			updatedAtTime := e.Entity().GetUpdatedAt()
			age := time.Since(updatedAtTime)
			d.Log.Info("Deleting old released lease",
				"lease", lease.ID,
				"disk", lease.DiskId,
				"age", age.Round(time.Second),
				"updated_at", updatedAtTime.Format(time.RFC3339))

			ec := entityserver.NewClient(d.Log, d.EAC)
			if err := ec.Delete(ctx, lease.ID); err != nil {
				d.Log.Error("Failed to delete old released lease",
					"lease", lease.ID,
					"error", err)
				continue
			}

			deletedCount++
		}
	}

	if deletedCount > 0 {
		d.Log.Info("Cleaned up old released leases", "count", deletedCount)
	}

	return nil
}
