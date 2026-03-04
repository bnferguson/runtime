package disk

import (
	"context"
	"fmt"
	"log/slog"
	"os"
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
	leaseId    string
	diskId     string
	sandboxId  string
	volumeId   string // Store volume ID to avoid lookups during delete
	leaseNonce string // Volume lease nonce from remote Disk API
}

// DiskLeaseController manages disk lease entities and exclusive access.
// It uses lsvd_mount entities to coordinate with lsvd-server for mount operations.
//
// Operational flow:
// 1. Disks are created via lsvd_volume entities when provisioned
// 2. When a lease is bound, an lsvd_mount entity is created for lsvd-server to mount
// 3. Leases control exclusive access to these mounted volumes
// 4. The lease.Mount.Path specifies where to mount within the sandbox's filesystem
type DiskLeaseController struct {
	Log *slog.Logger
	EAC *entityserver_v1alpha.EntityAccessClient

	// NodeId is the ID of this node, used for creating lsvd_mount entities
	NodeId string

	// Base path for disk mounts (e.g., /var/lib/miren/disks)
	mountBasePath string

	// Track active leases: diskId -> leaseId
	mu           sync.RWMutex
	activeLeases map[string]string
	leaseDetails map[string]*leaseInfo

	// Test-only cache for disk entities (when EAC is not available)
	testDiskCache map[string]*storage_v1alpha.Disk

	// directoryMode is enabled when NBD is unavailable - leases bind to simple directories
	directoryMode bool
}

// NewDiskLeaseController creates a disk lease controller that uses lsvd_mount entities.
// The lsvd-server process watches these entities and performs the actual mount/unmount operations.
func NewDiskLeaseController(log *slog.Logger, eac *entityserver_v1alpha.EntityAccessClient, nodeId string) *DiskLeaseController {
	return &DiskLeaseController{
		Log:           log.With("module", "disk-lease"),
		EAC:           eac,
		NodeId:        nodeId,
		mountBasePath: "/var/lib/miren/disks",
		activeLeases:  make(map[string]string),
		leaseDetails:  make(map[string]*leaseInfo),
	}
}

// SetTestDisk is a test helper to set disk information when EAC is not available
func (d *DiskLeaseController) SetTestDisk(disk *storage_v1alpha.Disk) {
	if d.testDiskCache == nil {
		d.testDiskCache = make(map[string]*storage_v1alpha.Disk)
	}
	d.testDiskCache[disk.ID.String()] = disk
}

// GetTestDisk is a test helper to retrieve disk information from test cache
func (d *DiskLeaseController) GetTestDisk(diskId entity.Id) *storage_v1alpha.Disk {
	if d.testDiskCache == nil {
		return nil
	}
	return d.testDiskCache[diskId.String()]
}

// ForceLSVDMode forces the controller to use lsvd_mount entities instead of
// directory mode, regardless of NBD availability. This is used by integration
// tests where the LSVD volume/mount ops are mocked.
func (d *DiskLeaseController) ForceLSVDMode() {
	d.directoryMode = false
}

// Init initializes the disk lease controller
func (d *DiskLeaseController) Init(ctx context.Context) error {
	// Check if NBD is available
	if !isNBDAvailable() {
		d.directoryMode = true
		d.Log.Warn("NBD kernel module not available - using directory-only mode for disk leases")
	} else {
		d.Log.Info("NBD kernel module available - using full LSVD mounting mode")
	}
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

	// Find and clean up the lsvd_mount entity
	mount, err := d.getLsvdMountForLease(ctx, id)
	if err != nil {
		d.Log.Warn("Error looking up lsvd_mount for deleted lease", "lease", leaseId, "error", err)
	}

	if mount != nil {
		// Set desired_state to unmounted if not already detached
		if mount.ActualState != storage_v1alpha.MNT_DETACHED {
			if mount.DesiredState != storage_v1alpha.MNT_WANT_UNMOUNTED {
				d.Log.Info("Setting lsvd_mount desired_state to unmounted for deleted lease",
					"lease", leaseId,
					"lsvd_mount", mount.ID)

				updateAttrs := []entity.Attr{
					entity.Ref(entity.DBId, mount.ID),
					entity.Ref(storage_v1alpha.LsvdMountDesiredStateId, storage_v1alpha.LsvdMountDesiredStateMntWantUnmountedId),
				}
				if _, err := d.EAC.Patch(ctx, updateAttrs, 0); err != nil {
					d.Log.Warn("Failed to update lsvd_mount desired_state",
						"lsvd_mount", mount.ID,
						"error", err)
				}
			}
		} else {
			// Already detached, delete the mount entity
			d.Log.Info("Deleting lsvd_mount entity for deleted lease",
				"lease", leaseId,
				"lsvd_mount", mount.ID)

			if _, err := d.EAC.Delete(ctx, mount.ID.String()); err != nil {
				d.Log.Warn("Failed to delete lsvd_mount entity",
					"lsvd_mount", mount.ID,
					"error", err)
			}
		}
	}

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

// reconcileLease reconciles the lease state
func (d *DiskLeaseController) reconcileLease(ctx context.Context, lease *storage_v1alpha.DiskLease, meta *entity.Meta) error {
	var err error

	switch lease.Status {
	case storage_v1alpha.PENDING:
		err = d.handlePendingLease(ctx, lease)
	case storage_v1alpha.RELEASED:
		err = d.handleReleasedLease(ctx, lease)
	case storage_v1alpha.BOUND:
		// Verify disk is actually mounted, mount if needed
		err = d.handleBoundLease(ctx, lease)
		// Update lease details for expiry tracking
		d.updateLeaseDetails(lease)
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

// handlePendingLease attempts to bind a pending lease via lsvd_mount entity
func (d *DiskLeaseController) handlePendingLease(ctx context.Context, lease *storage_v1alpha.DiskLease) error {
	diskId := lease.DiskId.String()
	leaseId := lease.ID.String()

	// Check if disk is already leased (with lock)
	d.mu.Lock()
	if existingLease, exists := d.activeLeases[diskId]; exists && existingLease != leaseId {
		// Conflict - disk is already leased by a different lease that is being released.
		// Leave the new lease as PENDING so the periodic resync will retry it
		// after the old lease cleanup completes.
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
	var disk *storage_v1alpha.Disk

	// Check test cache first (for unit tests)
	if d.testDiskCache != nil {
		if cachedDisk, ok := d.testDiskCache[diskId]; ok {
			disk = cachedDisk
		}
	}

	// If not in test cache, get from EAC
	if disk == nil {
		diskEntity, err := d.EAC.Get(ctx, diskId)
		if err != nil {
			d.Log.Error("Failed to get disk entity", "disk", diskId, "error", err)
			d.cleanupLeaseReservation(diskId)

			lease.Status = storage_v1alpha.FAILED
			lease.ErrorMessage = fmt.Sprintf("Failed to get disk entity: %v", err)

			return nil
		}

		// Decode disk entity
		disk = &storage_v1alpha.Disk{}
		disk.Decode(diskEntity.Entity().Entity())
		if disk.ID == "" {
			d.Log.Error("Failed to decode disk entity", "disk", diskId, "error", err)
			d.cleanupLeaseReservation(diskId)

			lease.Status = storage_v1alpha.FAILED
			lease.ErrorMessage = fmt.Sprintf("Failed to decode disk entity: %v", err)

			return nil
		}

	}

	// Check disk provisioning status
	if disk.Status != storage_v1alpha.PROVISIONED {
		if disk.Status == storage_v1alpha.PROVISIONING {
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

	volumeId := disk.LsvdVolumeId
	if volumeId == "" {
		d.cleanupLeaseReservation(diskId)

		lease.Status = storage_v1alpha.FAILED
		lease.ErrorMessage = "Disk has no associated volume"

		return nil
	}

	// In directory mode or when EAC is nil (test mode), just verify the directory exists
	if d.directoryMode || d.EAC == nil {
		diskDataPath := filepath.Join(d.mountBasePath, "disk-data", volumeId)
		if _, err := os.Stat(diskDataPath); err != nil {
			d.Log.Error("Failed to find directory for disk", "volume", volumeId, "path", diskDataPath, "error", err)
			d.cleanupLeaseReservation(diskId)

			lease.Status = storage_v1alpha.FAILED
			lease.ErrorMessage = fmt.Sprintf("Directory not found: %v", err)

			return nil
		}

		d.Log.Info("Successfully bound lease to directory (NBD unavailable)",
			"disk", diskId,
			"volume", volumeId,
			"path", diskDataPath)

		// Bind the lease
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

		return nil
	}

	// Check if an lsvd_mount entity already exists for this lease
	existingMount, err := d.getLsvdMountForLease(ctx, lease.ID)
	if err != nil {
		d.Log.Warn("Error looking up existing lsvd_mount", "lease", leaseId, "error", err)
	}

	if existingMount != nil {
		// Mount entity exists, check its state
		d.Log.Debug("Found existing lsvd_mount for lease",
			"lease", leaseId,
			"lsvd_mount", existingMount.ID,
			"actual_state", existingMount.ActualState)

		switch existingMount.ActualState {
		case storage_v1alpha.MNT_MOUNTED:
			// Mount is ready, bind the lease
			d.mu.Lock()
			d.leaseDetails[leaseId] = &leaseInfo{
				leaseId:    leaseId,
				diskId:     diskId,
				sandboxId:  lease.SandboxId.String(),
				volumeId:   volumeId,
				leaseNonce: existingMount.LeaseNonce,
			}
			d.mu.Unlock()

			lease.Status = storage_v1alpha.BOUND
			lease.ErrorMessage = ""
			lease.AcquiredAt = time.Now()

			d.Log.Info("Lease bound via lsvd_mount entity",
				"lease", leaseId,
				"lsvd_mount", existingMount.ID)
			return nil

		case storage_v1alpha.MNT_ERROR:
			// Mount failed
			d.Log.Warn("lsvd_mount in error state",
				"lease", leaseId,
				"lsvd_mount", existingMount.ID,
				"error", existingMount.ErrorMessage)
			d.cleanupLeaseReservation(diskId)

			lease.Status = storage_v1alpha.FAILED
			lease.ErrorMessage = fmt.Sprintf("Mount failed: %s", existingMount.ErrorMessage)
			return nil

		case storage_v1alpha.MNT_DETACHED:
			// Mount is in terminal DETACHED state — delete stale entity
			// and fall through to create a fresh mount.
			d.Log.Info("existing lsvd_mount in DETACHED state, deleting stale mount",
				"lease", leaseId,
				"lsvd_mount", existingMount.ID)
			if _, err := d.EAC.Delete(ctx, existingMount.ID.String()); err != nil {
				d.Log.Warn("failed to delete stale lsvd_mount, aborting mount creation",
					"lsvd_mount", existingMount.ID,
					"error", err)
				d.cleanupLeaseReservation(diskId)
				return nil
			}
			// Fall through to create a new mount entity

		default:
			// Mount is still in progress, wait
			d.Log.Debug("lsvd_mount still in progress",
				"lease", leaseId,
				"lsvd_mount", existingMount.ID,
				"actual_state", existingMount.ActualState)
			return nil
		}
	}

	// Need to find the lsvd_volume entity for this disk
	lsvdVolume, err := d.getLsvdVolumeForDisk(ctx, disk.ID)
	if err != nil {
		d.Log.Error("Failed to look up lsvd_volume", "disk", diskId, "error", err)
		d.cleanupLeaseReservation(diskId)

		lease.Status = storage_v1alpha.FAILED
		lease.ErrorMessage = fmt.Sprintf("Failed to look up lsvd_volume: %v", err)
		return nil
	}

	if lsvdVolume == nil {
		d.Log.Error("No lsvd_volume found for disk", "disk", diskId)
		d.cleanupLeaseReservation(diskId)

		lease.Status = storage_v1alpha.FAILED
		lease.ErrorMessage = "No lsvd_volume entity found for disk"
		return nil
	}

	if lsvdVolume.ActualState != storage_v1alpha.VOL_READY {
		// Volume not ready yet, wait
		d.cleanupLeaseReservation(diskId)
		d.Log.Info("lsvd_volume not ready, lease will retry",
			"disk", diskId,
			"lsvd_volume", lsvdVolume.ID,
			"actual_state", lsvdVolume.ActualState)
		return nil
	}

	// Create new lsvd_mount entity
	mountPath := d.getDiskMountPath(volumeId)

	lsvdMount := &storage_v1alpha.LsvdMount{
		VolumeId:     lsvdVolume.ID,
		DiskLeaseId:  lease.ID,
		MountPath:    mountPath,
		ReadOnly:     lease.Mount.ReadOnly,
		DesiredState: storage_v1alpha.MNT_WANT_MOUNTED,
		ActualState:  storage_v1alpha.MNT_PENDING,
		NodeId:       entity.Id("node/" + d.NodeId),
	}

	d.Log.Info("Creating lsvd_mount entity",
		"lease", leaseId,
		"lsvd_volume", lsvdVolume.ID,
		"mount_path", mountPath,
		"read_only", lease.Mount.ReadOnly,
		"node_id", d.NodeId)

	// Build entity with id and encoded attributes
	mountId := idgen.GenNS("lsvd-mnt")
	mountEntityId := entity.Id("lsvd_mount/" + mountId)
	createAttrs := entity.New(
		entity.DBId, mountEntityId,
		lsvdMount.Encode,
	).Attrs()

	_, err = d.EAC.Create(ctx, createAttrs)
	if err != nil {
		d.Log.Error("Failed to create lsvd_mount entity", "error", err)
		d.cleanupLeaseReservation(diskId)

		lease.Status = storage_v1alpha.FAILED
		lease.ErrorMessage = fmt.Sprintf("Failed to create lsvd_mount entity: %v", err)
		return nil
	}

	d.Log.Info("Created lsvd_mount entity, waiting for lsvd-server to mount",
		"lease", leaseId)

	// Lease remains in PENDING state until lsvd_mount becomes mounted
	return nil
}

// handleBoundLease verifies a bound lease has a mounted lsvd_mount entity
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

	// In directory mode, just verify directory exists
	if d.directoryMode {
		d.mu.RLock()
		details := d.leaseDetails[leaseId]
		d.mu.RUnlock()

		if details != nil && details.volumeId != "" {
			diskDataPath := filepath.Join(d.mountBasePath, "disk-data", details.volumeId)
			if _, err := os.Stat(diskDataPath); err == nil {
				d.Log.Debug("Bound lease already properly set up (directory mode)",
					"lease", leaseId,
					"disk", diskId,
					"volume", details.volumeId,
					"path", diskDataPath)
				return nil
			}
		}
		return nil
	}

	// Check the lsvd_mount entity's state
	mount, err := d.getLsvdMountForLease(ctx, lease.ID)
	if err != nil {
		d.Log.Warn("Error looking up lsvd_mount for bound lease", "lease", leaseId, "error", err)
		return nil
	}

	if mount == nil {
		// No mount entity - this shouldn't happen for a bound lease
		d.Log.Warn("Bound lease has no lsvd_mount entity, reverting to pending",
			"lease", leaseId)
		lease.Status = storage_v1alpha.PENDING
		return nil
	}

	// Update lease details with volume info
	d.mu.Lock()
	if details, exists := d.leaseDetails[leaseId]; exists {
		details.volumeId = string(mount.VolumeId)
		details.leaseNonce = mount.LeaseNonce
	}
	d.mu.Unlock()

	if mount.ActualState != storage_v1alpha.MNT_MOUNTED {
		if mount.ActualState == storage_v1alpha.MNT_ERROR {
			lease.Status = storage_v1alpha.FAILED
			lease.ErrorMessage = fmt.Sprintf("Mount failed: %s", mount.ErrorMessage)
		} else if mount.ActualState == storage_v1alpha.MNT_DETACHED {
			d.Log.Warn("lsvd_mount detached for bound lease, reverting to pending",
				"lease", leaseId,
				"lsvd_mount", mount.ID)
			lease.Status = storage_v1alpha.PENDING
		} else {
			d.Log.Debug("lsvd_mount not yet mounted for bound lease",
				"lease", leaseId,
				"lsvd_mount", mount.ID,
				"actual_state", mount.ActualState)
		}
	}

	return nil
}

// handleReleasedLease sets desired_state=MNT_WANT_UNMOUNTED on the lsvd_mount entity
func (d *DiskLeaseController) handleReleasedLease(ctx context.Context, lease *storage_v1alpha.DiskLease) error {
	leaseId := lease.ID.String()
	diskId := lease.DiskId.String()

	// Check if this lease is currently active
	d.mu.Lock()
	currentLease, exists := d.activeLeases[diskId]
	isActiveForThisLease := exists && currentLease == leaseId

	// Release the lease from local tracking immediately so new leases for the
	// same disk can proceed. The mount cleanup below continues independently
	// via lsvd_mount entities.
	if isActiveForThisLease {
		d.releaseLease(leaseId, diskId)
	}
	d.mu.Unlock()

	if !isActiveForThisLease {
		return nil
	}

	// In directory mode, nothing more to do
	if d.directoryMode {
		return nil
	}

	// Find the lsvd_mount entity
	mount, err := d.getLsvdMountForLease(ctx, lease.ID)
	if err != nil {
		d.Log.Warn("Error looking up lsvd_mount for released lease", "lease", leaseId, "error", err)
	}

	if mount != nil {
		// Check if already detached/unmounted
		if mount.ActualState == storage_v1alpha.MNT_DETACHED {
			d.Log.Info("lsvd_mount already detached, cleaning up",
				"lease", leaseId,
				"lsvd_mount", mount.ID)

			// Delete the lsvd_mount entity
			if _, err := d.EAC.Delete(ctx, mount.ID.String()); err != nil {
				d.Log.Warn("Failed to delete lsvd_mount entity",
					"lsvd_mount", mount.ID,
					"error", err)
			}
		} else if mount.DesiredState != storage_v1alpha.MNT_WANT_UNMOUNTED {
			// Set desired_state to unmounted
			d.Log.Info("Setting lsvd_mount desired_state to unmounted",
				"lease", leaseId,
				"lsvd_mount", mount.ID)

			updateAttrs := []entity.Attr{
				entity.Ref(entity.DBId, mount.ID),
				entity.Ref(storage_v1alpha.LsvdMountDesiredStateId, storage_v1alpha.LsvdMountDesiredStateMntWantUnmountedId),
			}
			if _, err := d.EAC.Patch(ctx, updateAttrs, 0); err != nil {
				d.Log.Error("Failed to update lsvd_mount desired_state",
					"lsvd_mount", mount.ID,
					"error", err)
			}
		} else {
			d.Log.Debug("lsvd_mount already marked for unmount",
				"lease", leaseId,
				"lsvd_mount", mount.ID,
				"actual_state", mount.ActualState)
		}
	}

	return nil
}

// handleFailedLease cleans up the lsvd_mount entity for a failed lease.
// FAILED is a terminal state, but we still need to ensure the mount is unmounted
// to avoid leaking mounted resources.
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

	if d.directoryMode {
		return nil
	}

	// Find and clean up the lsvd_mount entity
	mount, err := d.getLsvdMountForLease(ctx, lease.ID)
	if err != nil {
		d.Log.Warn("Error looking up lsvd_mount for failed lease", "lease", leaseId, "error", err)
		return nil
	}

	if mount == nil {
		return nil
	}

	if mount.ActualState == storage_v1alpha.MNT_DETACHED {
		d.Log.Info("lsvd_mount already detached for failed lease, cleaning up",
			"lease", leaseId,
			"lsvd_mount", mount.ID)

		if _, err := d.EAC.Delete(ctx, mount.ID.String()); err != nil {
			d.Log.Warn("Failed to delete lsvd_mount entity",
				"lsvd_mount", mount.ID,
				"error", err)
		}
	} else if mount.DesiredState != storage_v1alpha.MNT_WANT_UNMOUNTED {
		d.Log.Info("Setting lsvd_mount desired_state to unmounted for failed lease",
			"lease", leaseId,
			"lsvd_mount", mount.ID)

		updateAttrs := []entity.Attr{
			entity.Ref(entity.DBId, mount.ID),
			entity.Ref(storage_v1alpha.LsvdMountDesiredStateId, storage_v1alpha.LsvdMountDesiredStateMntWantUnmountedId),
		}
		if _, err := d.EAC.Patch(ctx, updateAttrs, 0); err != nil {
			d.Log.Error("Failed to update lsvd_mount desired_state",
				"lsvd_mount", mount.ID,
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

// updateLeaseDetails updates lease information
func (d *DiskLeaseController) updateLeaseDetails(lease *storage_v1alpha.DiskLease) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Currently just ensures the lease is tracked
	// Could be extended to update other lease details if needed
	_ = d.leaseDetails[lease.ID.String()]
}

// getDiskMountPath returns the standard mount path for a disk volume
func (d *DiskLeaseController) getDiskMountPath(volumeId string) string {
	return filepath.Join(d.mountBasePath, volumeId)
}

// getLsvdMountForLease finds the lsvd_mount entity for a lease
func (d *DiskLeaseController) getLsvdMountForLease(ctx context.Context, leaseId entity.Id) (*storage_v1alpha.LsvdMount, error) {
	// No EAC in test mode
	if d.EAC == nil {
		return nil, nil
	}

	// Query by disk_lease_id index
	indexAttr := entity.Ref(storage_v1alpha.LsvdMountDiskLeaseIdId, leaseId)

	resp, err := d.EAC.List(ctx, indexAttr)
	if err != nil {
		return nil, fmt.Errorf("failed to list lsvd_mount entities: %w", err)
	}

	values := resp.Values()
	if len(values) == 0 {
		return nil, nil
	}

	// Return the first matching entity
	var mount storage_v1alpha.LsvdMount
	mount.Decode(values[0].Entity())

	return &mount, nil
}

// getLsvdVolumeForDisk finds the lsvd_volume entity for a disk
func (d *DiskLeaseController) getLsvdVolumeForDisk(ctx context.Context, diskId entity.Id) (*storage_v1alpha.LsvdVolume, error) {
	// No EAC in test mode
	if d.EAC == nil {
		return nil, nil
	}

	// Query by disk_id index
	indexAttr := entity.Ref(storage_v1alpha.LsvdVolumeDiskIdId, diskId)

	resp, err := d.EAC.List(ctx, indexAttr)
	if err != nil {
		return nil, fmt.Errorf("failed to list lsvd_volume entities: %w", err)
	}

	values := resp.Values()
	if len(values) == 0 {
		return nil, nil
	}

	// Return the first matching entity
	var volume storage_v1alpha.LsvdVolume
	volume.Decode(values[0].Entity())

	return &volume, nil
}

// CleanupOldReleasedLeases deletes released leases that haven't been updated for over 1 hour
func (d *DiskLeaseController) CleanupOldReleasedLeases(ctx context.Context) error {
	if d.EAC == nil {
		// No EAC available (test mode), skip cleanup
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

		// Only delete if:
		// 1. Status is RELEASED
		// 2. UpdatedAt is more than 1 hour ago
		if lease.Status == storage_v1alpha.RELEASED && e.Entity().GetUpdatedAt().Before(cutoffTime) {
			updatedAtTime := e.Entity().GetUpdatedAt()
			age := time.Since(updatedAtTime)
			d.Log.Info("Deleting old released lease",
				"lease", lease.ID,
				"disk", lease.DiskId,
				"age", age.Round(time.Second),
				"updated_at", updatedAtTime.Format(time.RFC3339))

			// Use entity server client to delete the entity
			ec := entityserver.NewClient(d.Log, d.EAC)
			if err := ec.Delete(ctx, lease.ID); err != nil {
				d.Log.Error("Failed to delete old released lease",
					"lease", lease.ID,
					"error", err)
				// Continue with other leases even if one fails
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
