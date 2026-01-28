package disk

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"miren.dev/runtime/api/entityserver/entityserver_v1alpha"
	"miren.dev/runtime/api/storage/storage_v1alpha"
	"miren.dev/runtime/pkg/controller"
	"miren.dev/runtime/pkg/entity"
	"miren.dev/runtime/pkg/idgen"
)

// isNBDAvailable checks if NBD is available (either as a module or built into the kernel)
func isNBDAvailable() bool {
	if os.Getenv("MIREN_DISABLE_NBD") != "" {
		return false
	}

	// Check if NBD module is loaded
	if _, err := os.Stat("/sys/module/nbd"); err == nil {
		return true
	}

	// Check if NBD devices exist (NBD might be compiled into the kernel)
	matches, err := filepath.Glob("/sys/devices/virtual/block/nbd*")
	if err == nil && len(matches) > 0 {
		return true
	}

	return false
}

// DiskController manages disk entities and their lifecycle.
// It uses lsvd_volume entities to coordinate with lsvd-server for volume operations.
type DiskController struct {
	Log *slog.Logger
	EAC *entityserver_v1alpha.EntityAccessClient

	// NodeId is the ID of this node, used for creating lsvd_volume entities
	NodeId string

	// Base path for disk mounts (e.g., /var/lib/miren/disks)
	mountBasePath string

	// directoryMode is enabled when NBD is unavailable - disks are simple directories
	directoryMode bool
}

// NewDiskController creates a disk controller that uses lsvd_volume entities.
// The lsvd-server process watches these entities and performs the actual volume operations.
func NewDiskController(log *slog.Logger, eac *entityserver_v1alpha.EntityAccessClient, nodeId string) *DiskController {
	return &DiskController{
		Log:           log.With("module", "disk"),
		EAC:           eac,
		NodeId:        nodeId,
		mountBasePath: "/var/lib/miren/disks",
	}
}

// Init initializes the disk controller
func (d *DiskController) Init(ctx context.Context) error {
	// Check if NBD is available
	if !isNBDAvailable() {
		d.directoryMode = true
		d.Log.Warn("NBD kernel module not available - using directory-only mode for disks")
	} else {
		d.Log.Info("NBD kernel module available - using full LSVD mode")
	}
	return nil
}

// Create handles creation of a new disk entity
func (d *DiskController) Create(ctx context.Context, disk *storage_v1alpha.Disk, meta *entity.Meta) error {
	d.Log.Info("Processing disk creation",
		"disk", disk.ID,
		"status", disk.Status)

	return d.reconcileDisk(ctx, disk, meta)
}

// Update handles updates to an existing disk entity
func (d *DiskController) Update(ctx context.Context, disk *storage_v1alpha.Disk, meta *entity.Meta) error {
	d.Log.Info("Processing disk update",
		"disk", disk.ID,
		"status", disk.Status)

	return d.reconcileDisk(ctx, disk, meta)
}

// Delete handles deletion of a disk entity
func (d *DiskController) Delete(ctx context.Context, id entity.Id) error {
	d.Log.Info("Processing disk deletion", "disk", id)
	// Deletion is handled through the DELETING status in reconcileDisk
	return nil
}

// reconcileDisk reconciles the disk state
func (d *DiskController) reconcileDisk(ctx context.Context, disk *storage_v1alpha.Disk, meta *entity.Meta) error {
	var err error

	switch disk.Status {
	case storage_v1alpha.PROVISIONED:
		// Verify the disk is actually provisioned
		err = d.handleProvisioned(ctx, disk)
	case storage_v1alpha.PROVISIONING:
		err = d.handleProvisioning(ctx, disk)
	case storage_v1alpha.DELETING:
		err = d.handleDeletion(ctx, disk)
	case storage_v1alpha.ATTACHED, storage_v1alpha.DETACHED:
		// These states are managed by disk lease controller
		return nil
	case storage_v1alpha.ERROR:
		// Error state is terminal, no action needed
		return nil
	default:
		// Unknown status, log warning
		d.Log.Warn("Unknown disk status", "disk", disk.ID, "status", disk.Status)
		return nil
	}

	if err != nil {
		return err
	}

	// Update entity attributes if any changes
	if meta != nil {
		// Ensure meta.Entity is initialized
		if meta.Entity == nil {
			meta.Entity = entity.New(disk.Encode())
		} else {
			// Caller does a diff so we can always send it back
			meta.Entity.Update(disk.Encode())
		}
	}

	return nil
}

// handleProvisioning provisions a new LSVD volume via lsvd_volume entity
func (d *DiskController) handleProvisioning(ctx context.Context, disk *storage_v1alpha.Disk) error {
	// In directory mode or when EAC is nil (test mode), just create a directory
	if d.directoryMode || d.EAC == nil {
		return d.provisionDirectory(ctx, disk)
	}

	// Check if an lsvd_volume entity already exists for this disk
	existingVolume, err := d.getLsvdVolumeForDisk(ctx, disk.ID)
	if err != nil {
		d.Log.Warn("Error looking up existing lsvd_volume", "disk", disk.ID, "error", err)
	}

	if existingVolume != nil {
		// Volume entity exists, check its state
		d.Log.Debug("Found existing lsvd_volume for disk",
			"disk", disk.ID,
			"lsvd_volume", existingVolume.ID,
			"actual_state", existingVolume.ActualState,
			"volume_id", existingVolume.VolumeId)

		switch existingVolume.ActualState {
		case storage_v1alpha.VOL_READY:
			// Volume is ready, update disk status
			disk.Status = storage_v1alpha.PROVISIONED
			disk.LsvdVolumeId = existingVolume.VolumeId
			d.Log.Info("Disk provisioned via lsvd_volume entity",
				"disk", disk.ID,
				"volume", existingVolume.VolumeId)
			return nil

		case storage_v1alpha.VOL_ERROR:
			// Volume failed, could retry by resetting the entity
			d.Log.Warn("lsvd_volume in error state",
				"disk", disk.ID,
				"lsvd_volume", existingVolume.ID,
				"error", existingVolume.ErrorMessage)
			// Don't update disk status - leave it in PROVISIONING for retry
			return nil

		default:
			// Volume is still being created, wait
			d.Log.Debug("lsvd_volume still provisioning",
				"disk", disk.ID,
				"lsvd_volume", existingVolume.ID,
				"actual_state", existingVolume.ActualState)
			return nil
		}
	}

	// Create new lsvd_volume entity
	filesystem := strings.TrimPrefix(string(disk.Filesystem), "filesystem.")

	lsvdVolume := &storage_v1alpha.LsvdVolume{
		DiskId:       disk.ID,
		SizeGb:       disk.SizeGb,
		Filesystem:   filesystem,
		RemoteOnly:   disk.RemoteOnly,
		DesiredState: storage_v1alpha.VOL_PRESENT,
		ActualState:  storage_v1alpha.VOL_PENDING,
		NodeId:       entity.Id("node/" + d.NodeId),
	}

	d.Log.Info("Creating lsvd_volume entity",
		"disk", disk.ID,
		"size_gb", disk.SizeGb,
		"filesystem", filesystem,
		"remote_only", disk.RemoteOnly,
		"node_id", d.NodeId)

	// Build entity with id and encoded attributes
	volumeId := idgen.GenNS("lsvd-vol")
	createAttrs := entity.New(
		entity.DBId, storage_v1alpha.KindLsvdVolume.String()+"/"+volumeId,
		lsvdVolume.Encode,
	).Attrs()

	_, err = d.EAC.Create(ctx, createAttrs)
	if err != nil {
		return fmt.Errorf("failed to create lsvd_volume entity: %w", err)
	}

	d.Log.Info("Created lsvd_volume entity, waiting for lsvd-server to provision",
		"disk", disk.ID)

	// Disk remains in PROVISIONING state until lsvd_volume becomes ready
	return nil
}

// provisionDirectory creates a directory for directory-mode disks
func (d *DiskController) provisionDirectory(ctx context.Context, disk *storage_v1alpha.Disk) error {
	// Validate disk size
	if disk.SizeGb <= 0 {
		return fmt.Errorf("invalid disk size: %d GB", disk.SizeGb)
	}

	// Generate org-scoped deterministic volume ID for directory mode
	volumeId := idgen.GenNS("vol")
	diskDataPath := filepath.Join(d.mountBasePath, "disk-data", volumeId)
	d.Log.Info("Creating directory-only disk (NBD unavailable)",
		"volume", volumeId,
		"path", diskDataPath,
		"size_gb", disk.SizeGb,
		"filesystem", disk.Filesystem)

	if err := os.MkdirAll(diskDataPath, 0755); err != nil {
		return fmt.Errorf("failed to create directory for disk: %w", err)
	}

	disk.Status = storage_v1alpha.PROVISIONED
	disk.LsvdVolumeId = volumeId

	return nil
}

// handleProvisioned verifies a provisioned disk has a ready lsvd_volume entity
func (d *DiskController) handleProvisioned(ctx context.Context, disk *storage_v1alpha.Disk) error {
	// Check if volume ID exists
	if disk.LsvdVolumeId == "" {
		d.Log.Warn("Provisioned disk has no volume ID, re-provisioning", "disk", disk.ID)
		disk.Status = storage_v1alpha.PROVISIONING
		return d.handleProvisioning(ctx, disk)
	}

	// In directory mode or when EAC is nil (test mode), verify directory exists
	if d.directoryMode || d.EAC == nil {
		diskDataPath := filepath.Join(d.mountBasePath, "disk-data", disk.LsvdVolumeId)
		if _, err := os.Stat(diskDataPath); err != nil {
			if os.IsNotExist(err) {
				d.Log.Warn("Directory not found for provisioned disk, re-provisioning",
					"disk", disk.ID,
					"volume", disk.LsvdVolumeId,
					"path", diskDataPath)
				disk.LsvdVolumeId = ""
				disk.Status = storage_v1alpha.PROVISIONING
				return d.handleProvisioning(ctx, disk)
			}
			return fmt.Errorf("failed to check directory: %w", err)
		}

		d.Log.Debug("Provisioned disk directory exists",
			"disk", disk.ID,
			"volume", disk.LsvdVolumeId,
			"path", diskDataPath)

		return nil
	}

	// Verify via lsvd_volume entity
	volume, err := d.getLsvdVolumeForDisk(ctx, disk.ID)
	if err != nil {
		d.Log.Warn("Error looking up lsvd_volume for provisioned disk",
			"disk", disk.ID,
			"error", err)
		return nil
	}

	if volume == nil {
		// No lsvd_volume entity - this shouldn't happen for a provisioned disk
		d.Log.Warn("Provisioned disk has no lsvd_volume entity, clearing volume ID",
			"disk", disk.ID,
			"volume", disk.LsvdVolumeId)
		disk.LsvdVolumeId = ""
		disk.Status = storage_v1alpha.PROVISIONING
		return nil
	}

	// Check the actual state
	if volume.ActualState != storage_v1alpha.VOL_READY {
		d.Log.Warn("lsvd_volume not ready for provisioned disk",
			"disk", disk.ID,
			"lsvd_volume", volume.ID,
			"actual_state", volume.ActualState)
		// Revert to provisioning state
		disk.Status = storage_v1alpha.PROVISIONING
		disk.LsvdVolumeId = ""
		return nil
	}

	d.Log.Debug("Provisioned disk has ready lsvd_volume",
		"disk", disk.ID,
		"lsvd_volume", volume.ID,
		"volume_id", volume.VolumeId)

	return nil
}

// handleDeletion sets desired_state=absent on the lsvd_volume entity
func (d *DiskController) handleDeletion(ctx context.Context, disk *storage_v1alpha.Disk) error {
	volume, err := d.getLsvdVolumeForDisk(ctx, disk.ID)
	if err != nil {
		d.Log.Warn("Error looking up lsvd_volume for deletion",
			"disk", disk.ID,
			"error", err)
		// Return error so resync can retry - prevents orphaning volumes if lookup fails
		return err
	}

	if volume != nil {
		// Check if already deleted
		if volume.ActualState == storage_v1alpha.VOL_DELETED {
			d.Log.Info("lsvd_volume already deleted, cleaning up disk",
				"disk", disk.ID,
				"lsvd_volume", volume.ID)

			// Delete the lsvd_volume entity
			if _, err := d.EAC.Delete(ctx, volume.ID.String()); err != nil {
				d.Log.Warn("Failed to delete lsvd_volume entity",
					"lsvd_volume", volume.ID,
					"error", err)
			}
		} else if volume.DesiredState != storage_v1alpha.VOL_ABSENT {
			// Set desired_state to absent
			d.Log.Info("Setting lsvd_volume desired_state to absent",
				"disk", disk.ID,
				"lsvd_volume", volume.ID)

			volume.DesiredState = storage_v1alpha.VOL_ABSENT
			// Use Patch to update the desired_state
			updateAttrs := []entity.Attr{
				entity.Ref(entity.DBId, volume.ID),
				entity.Ref(storage_v1alpha.LsvdVolumeDesiredStateId, storage_v1alpha.LsvdVolumeDesiredStateVolAbsentId),
			}
			if _, err := d.EAC.Patch(ctx, updateAttrs, 0); err != nil {
				d.Log.Error("Failed to update lsvd_volume desired_state",
					"lsvd_volume", volume.ID,
					"error", err)
				return err
			}

			// Wait for lsvd-server to delete the volume
			return nil
		} else {
			// Already marked for deletion, wait for it
			d.Log.Debug("lsvd_volume already marked for deletion",
				"disk", disk.ID,
				"lsvd_volume", volume.ID,
				"actual_state", volume.ActualState)
			return nil
		}
	}

	// No lsvd_volume or it's been deleted - delete the disk entity
	if d.EAC != nil {
		if _, err := d.EAC.Delete(ctx, disk.ID.String()); err != nil {
			d.Log.Error("Failed to delete disk entity", "disk", disk.ID, "error", err)
			return err
		}
	}

	return nil
}

// getLsvdVolumeForDisk finds the lsvd_volume entity for a disk
func (d *DiskController) getLsvdVolumeForDisk(ctx context.Context, diskId entity.Id) (*storage_v1alpha.LsvdVolume, error) {
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

// Close gracefully shuts down the disk controller
func (d *DiskController) Close() error {
	d.Log.Info("Shutting down disk controller")
	return nil
}

// Start starts the disk controller
func (d *DiskController) Start(ctx context.Context) error {
	// Create reconcile controller using AdaptController
	rc := controller.NewReconcileController(
		"disk",
		d.Log,
		entity.Ref(entity.EntityKind, storage_v1alpha.KindDisk),
		d.EAC,
		controller.AdaptController(d),
		0, // No resync period
		1, // Single worker for now
	)

	return rc.Start(ctx)
}
