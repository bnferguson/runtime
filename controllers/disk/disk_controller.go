package disk

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	"miren.dev/runtime/api/entityserver/entityserver_v1alpha"
	"miren.dev/runtime/api/storage/storage_v1alpha"
	"miren.dev/runtime/pkg/controller"
	"miren.dev/runtime/pkg/entity"
	"miren.dev/runtime/pkg/idgen"
)

// detectDiskMode determines which disk I/O mode to use.
func detectDiskMode() storage_v1alpha.DiskMode {
	if mode := os.Getenv("MIREN_DISK_MODE"); mode != "" {
		switch mode {
		case "universal":
			return storage_v1alpha.UNIVERSAL
		case "accelerator":
			return storage_v1alpha.ACCELERATOR
		}
	}

	// Use accelerator mode if lbd is available
	if _, err := exec.LookPath("lbdctl"); err == nil {
		return storage_v1alpha.ACCELERATOR
	}

	return storage_v1alpha.UNIVERSAL
}

// DiskController manages disk entities and their lifecycle.
// It uses disk_volume entities to coordinate volume operations via loop devices.
type DiskController struct {
	Log *slog.Logger
	EAC *entityserver_v1alpha.EntityAccessClient

	// NodeId is the ID of this node, used for creating volume entities
	NodeId string

	// Base path for disk mounts (e.g., /var/lib/miren/disks)
	mountBasePath string

	// diskMode determines how disks are provisioned (universal or accelerator)
	diskMode storage_v1alpha.DiskMode
}

// NewDiskController creates a disk controller that uses disk_volume entities.
func NewDiskController(log *slog.Logger, eac *entityserver_v1alpha.EntityAccessClient, nodeId string) *DiskController {
	return &DiskController{
		Log:           log.With("module", "disk"),
		EAC:           eac,
		NodeId:        nodeId,
		mountBasePath: "/var/lib/miren/disks",
	}
}

// ForceUniversalMode forces the controller to use disk_volume entities with
// loop devices. This is used by integration tests.
func (d *DiskController) ForceUniversalMode() {
	d.diskMode = storage_v1alpha.UNIVERSAL
}

// Init initializes the disk controller
func (d *DiskController) Init(ctx context.Context) error {
	d.diskMode = detectDiskMode()
	d.Log.Info("disk controller initialized", "mode", d.diskMode)
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
func (d *DiskController) Delete(ctx context.Context, id entity.Id, obj *storage_v1alpha.Disk) error {
	d.Log.Info("Processing disk deletion", "disk", id)
	// Deletion is handled through the DELETING status in reconcileDisk
	return nil
}

// reconcileDisk reconciles the disk state
func (d *DiskController) reconcileDisk(ctx context.Context, disk *storage_v1alpha.Disk, meta *entity.Meta) error {
	var err error

	switch disk.Status {
	case storage_v1alpha.PROVISIONED:
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

// handleProvisioning provisions a new disk volume using disk_volume entities
func (d *DiskController) handleProvisioning(ctx context.Context, disk *storage_v1alpha.Disk) error {
	// Check if a disk_volume entity already exists for this disk
	existingVolume, err := d.getDiskVolumeForDisk(ctx, disk.ID)
	if err != nil {
		return fmt.Errorf("error looking up existing disk_volume for disk %s: %w", disk.ID, err)
	}

	if existingVolume != nil {
		d.Log.Debug("found existing disk_volume for disk",
			"disk", disk.ID,
			"disk_volume", existingVolume.ID,
			"actual_state", existingVolume.ActualState,
			"volume_id", existingVolume.VolumeId)

		switch existingVolume.ActualState {
		case storage_v1alpha.DV_READY:
			disk.Status = storage_v1alpha.PROVISIONED
			disk.VolumeId = existingVolume.VolumeId
			disk.Mode = d.diskMode
			d.Log.Info("disk provisioned via disk_volume entity",
				"disk", disk.ID,
				"volume_id", existingVolume.VolumeId)
			return nil

		case storage_v1alpha.DV_ERROR:
			d.Log.Warn("disk_volume in error state",
				"disk", disk.ID,
				"disk_volume", existingVolume.ID,
				"error", existingVolume.ErrorMessage)
			return nil

		default:
			d.Log.Debug("disk_volume still provisioning",
				"disk", disk.ID,
				"disk_volume", existingVolume.ID,
				"actual_state", existingVolume.ActualState)
			return nil
		}
	}

	// Create new disk_volume entity
	filesystem := strings.TrimPrefix(string(disk.Filesystem), "filesystem.")

	diskVolume := &storage_v1alpha.DiskVolume{
		Name:         disk.Name,
		DiskId:       disk.ID,
		SizeGb:       disk.SizeGb,
		Filesystem:   filesystem,
		VolumeMode:   diskModeToVolumeMode(d.diskMode),
		DesiredState: storage_v1alpha.DV_PRESENT,
		ActualState:  storage_v1alpha.DV_PENDING,
		NodeId:       entity.Id("node/" + strings.TrimPrefix(d.NodeId, "node/")),
	}

	// When migrating from LSVD, set MountId to the LSVD volume UUID so the
	// mount path matches the old one.
	volumeId := idgen.GenNS("disk-vol")
	if disk.LsvdVolumeId != "" {
		if lsvdVolId := d.findLSVDVolumeId(ctx, disk.ID); lsvdVolId != "" {
			diskVolume.MountId = lsvdVolId
		}
	}

	d.Log.Info("creating disk_volume entity",
		"disk", disk.ID,
		"volume_id", volumeId,
		"size_gb", disk.SizeGb,
		"filesystem", filesystem,
		"node_id", d.NodeId)

	createAttrs := entity.New(
		entity.DBId, entity.Id("disk_volume/"+volumeId),
		diskVolume.Encode,
	).Attrs()

	_, err = d.EAC.Create(ctx, createAttrs)
	if err != nil {
		return fmt.Errorf("failed to create disk_volume entity: %w", err)
	}

	disk.Mode = d.diskMode

	d.Log.Info("created disk_volume entity, waiting for provisioning",
		"disk", disk.ID)

	return nil
}

// handleProvisioned verifies a provisioned disk has a ready volume entity
func (d *DiskController) handleProvisioned(ctx context.Context, disk *storage_v1alpha.Disk) error {
	if disk.VolumeId == "" {
		d.Log.Warn("provisioned disk has no volume ID, re-provisioning", "disk", disk.ID)
		disk.Status = storage_v1alpha.PROVISIONING
		return d.handleProvisioning(ctx, disk)
	}

	volume, err := d.getDiskVolumeForDisk(ctx, disk.ID)
	if err != nil {
		return fmt.Errorf("error looking up disk_volume for provisioned disk %s: %w", disk.ID, err)
	}

	if volume == nil {
		d.Log.Info("provisioned disk has no disk_volume entity, creating one",
			"disk", disk.ID,
			"volume_id", disk.VolumeId)
		disk.VolumeId = ""
		disk.Status = storage_v1alpha.PROVISIONING
		return d.handleProvisioning(ctx, disk)
	}

	if volume.ActualState != storage_v1alpha.DV_READY {
		d.Log.Warn("disk_volume not ready for provisioned disk",
			"disk", disk.ID,
			"disk_volume", volume.ID,
			"actual_state", volume.ActualState)
		disk.Status = storage_v1alpha.PROVISIONING
		disk.VolumeId = ""
		return nil
	}

	return nil
}

// handleDeletion sets desired_state=absent on the volume entity
func (d *DiskController) handleDeletion(ctx context.Context, disk *storage_v1alpha.Disk) error {
	volume, err := d.getDiskVolumeForDisk(ctx, disk.ID)
	if err != nil {
		d.Log.Warn("error looking up disk_volume for deletion",
			"disk", disk.ID,
			"error", err)
		return err
	}

	if volume != nil {
		if volume.ActualState == storage_v1alpha.DV_DELETED {
			d.Log.Info("disk_volume already deleted, cleaning up disk",
				"disk", disk.ID,
				"disk_volume", volume.ID)

			if _, err := d.EAC.Delete(ctx, volume.ID.String()); err != nil {
				d.Log.Warn("failed to delete disk_volume entity",
					"disk_volume", volume.ID,
					"error", err)
				return err
			}
		} else if volume.DesiredState != storage_v1alpha.DV_ABSENT {
			d.Log.Info("setting disk_volume desired_state to absent",
				"disk", disk.ID,
				"disk_volume", volume.ID)

			updateAttrs := []entity.Attr{
				entity.Ref(entity.DBId, volume.ID),
				entity.Ref(storage_v1alpha.DiskVolumeDesiredStateId, storage_v1alpha.DiskVolumeDesiredStateDvAbsentId),
			}
			if _, err := d.EAC.Patch(ctx, updateAttrs, 0); err != nil {
				d.Log.Error("failed to update disk_volume desired_state",
					"disk_volume", volume.ID,
					"error", err)
				return err
			}

			return nil
		} else {
			d.Log.Debug("disk_volume already marked for deletion",
				"disk", disk.ID,
				"disk_volume", volume.ID,
				"actual_state", volume.ActualState)
			return nil
		}
	}

	// No disk_volume or it's been deleted - delete the disk entity
	if d.EAC != nil {
		if _, err := d.EAC.Delete(ctx, disk.ID.String()); err != nil {
			d.Log.Error("failed to delete disk entity", "disk", disk.ID, "error", err)
			return err
		}
	}

	return nil
}

// findLSVDVolumeId looks up the lsvd_volume entity for a disk and returns
// the VolumeId (UUID used as the old mount point name).
func (d *DiskController) findLSVDVolumeId(ctx context.Context, diskId entity.Id) string {
	if d.EAC == nil {
		return ""
	}

	resp, err := d.EAC.List(ctx, entity.Ref(storage_v1alpha.LsvdVolumeDiskIdId, diskId))
	if err != nil {
		d.Log.Debug("failed to list lsvd_volume for disk", "disk", diskId, "error", err)
		return ""
	}

	values := resp.Values()
	if len(values) == 0 {
		return ""
	}

	var lsvdVol storage_v1alpha.LsvdVolume
	lsvdVol.Decode(values[0].Entity())

	return lsvdVol.VolumeId
}

// getDiskVolumeForDisk finds the disk_volume entity for a disk
func (d *DiskController) getDiskVolumeForDisk(ctx context.Context, diskId entity.Id) (*storage_v1alpha.DiskVolume, error) {
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

func diskModeToVolumeMode(mode storage_v1alpha.DiskMode) storage_v1alpha.DiskVolumeVolumeMode {
	switch mode {
	case storage_v1alpha.ACCELERATOR:
		return storage_v1alpha.VM_ACCELERATOR
	default:
		return storage_v1alpha.VM_UNIVERSAL
	}
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
