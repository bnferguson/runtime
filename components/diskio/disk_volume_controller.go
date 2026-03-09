package diskio

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"miren.dev/runtime/api/entityserver/entityserver_v1alpha"
	"miren.dev/runtime/api/storage/storage_v1alpha"
	"miren.dev/runtime/lsvd"
	"miren.dev/runtime/pkg/entity"
	"miren.dev/runtime/pkg/units"
)

// alwaysMount returns true if volumes with this mode should be mounted
// at creation time and stay mounted regardless of lease lifecycle.
func alwaysMount(mode storage_v1alpha.DiskVolumeVolumeMode) bool {
	return mode == storage_v1alpha.VM_UNIVERSAL
}

// DiskVolumeController watches disk_volume entities and manages sparse disk images
// using loop devices.
type DiskVolumeController struct {
	log      *slog.Logger
	dataPath string
	nodeId   string
	eac      *entityserver_v1alpha.EntityAccessClient
	state    *State
	ops      DiskVolumeOps
	mntOps   DiskMountOps

	// keepMounts, when true, causes Shutdown to skip unmounting volumes.
	// Set during reload (SIGUSR2) so the new process can pick them up.
	keepMounts bool
}

func NewDiskVolumeController(log *slog.Logger, dataPath, nodeId string, state *State, ops DiskVolumeOps, mntOps DiskMountOps) *DiskVolumeController {
	return &DiskVolumeController{
		log:      log.With("module", "disk-volume"),
		dataPath: dataPath,
		nodeId:   nodeId,
		state:    state,
		ops:      ops,
		mntOps:   mntOps,
	}
}

func (c *DiskVolumeController) SetEAC(eac *entityserver_v1alpha.EntityAccessClient) {
	c.eac = eac
}

// SetKeepMounts tells the controller to skip unmounting during Shutdown.
// Used during reload so the replacement process inherits the mounts.
func (c *DiskVolumeController) SetKeepMounts(v bool) {
	c.keepMounts = v
}

func (c *DiskVolumeController) Init(ctx context.Context) error {
	c.cleanupMigratedLSVD()
	c.cleanupLSVDState()
	return nil
}

// cleanupLSVDState removes old lsvd_volume entries from the persisted state
// file left over from the previous LSVD-based system.
func (c *DiskVolumeController) cleanupLSVDState() {
	cleaned := false
	for _, vol := range c.state.ListVolumes() {
		if strings.HasPrefix(vol.EntityId, "lsvd_volume/") {
			c.log.Info("removing old lsvd_volume state entry", "entity_id", vol.EntityId)
			c.state.DeleteVolume(vol.EntityId)
			cleaned = true
		}
	}
	for _, mnt := range c.state.ListMounts() {
		if strings.HasPrefix(mnt.EntityId, "lsvd_mount/") || strings.HasPrefix(mnt.EntityId, "lsvd_volume/") ||
			strings.HasPrefix(mnt.VolumeId, "lsvd_volume/") {
			c.log.Info("removing old lsvd mount state entry", "entity_id", mnt.EntityId)
			c.state.DeleteMount(mnt.EntityId)
			cleaned = true
		}
	}
	if cleaned {
		if err := c.state.Save(); err != nil {
			c.log.Warn("failed to save state after LSVD cleanup", "error", err)
		}
	}
}

func (c *DiskVolumeController) cleanupMigratedLSVD() {
	volumesDir := filepath.Join(c.dataPath, "volumes")
	entries, err := os.ReadDir(volumesDir)
	if err != nil {
		return
	}

	// Check if any unmigrated LSVD volumes remain. We need to check both the
	// flat layout (volumes/{name}/info.json) used by tests and the nested
	// layout (volumes/lsvd-vol-{id}/volumes/{volId}/info.json) from production.
	for _, ent := range entries {
		if !ent.IsDir() {
			continue
		}
		dir := filepath.Join(volumesDir, ent.Name())

		// Flat layout check
		if _, err := os.Stat(filepath.Join(dir, "info.json")); err == nil {
			return // Unmigrated LSVD volume exists, don't clean up
		}

		// Nested layout check
		nestedVols := filepath.Join(dir, "volumes")
		nestedEntries, nerr := os.ReadDir(nestedVols)
		if nerr != nil {
			continue
		}
		for _, nent := range nestedEntries {
			if _, err := os.Stat(filepath.Join(nestedVols, nent.Name(), "info.json")); err == nil {
				return // Unmigrated LSVD volume exists, don't clean up
			}
		}
	}

	// All LSVD volumes have been migrated — clean up old data
	segmentsDir := filepath.Join(c.dataPath, "segments")
	if _, err := os.Stat(segmentsDir); err == nil {
		c.log.Info("all LSVD volumes migrated, cleaning up segments directory")
		os.RemoveAll(segmentsDir)
	}

	for _, ent := range entries {
		if !ent.IsDir() {
			continue
		}
		dir := filepath.Join(volumesDir, ent.Name())

		// Clean up flat layout migration markers
		if _, err := os.Stat(filepath.Join(dir, "info.json.migrated")); err == nil {
			os.Remove(filepath.Join(dir, "info.json.migrated"))
			os.Remove(filepath.Join(dir, "segments"))
			os.Remove(dir)
			continue
		}

		// Clean up nested layout (old lsvd-vol-* directories)
		if !strings.HasPrefix(ent.Name(), "lsvd-vol-") {
			continue
		}
		// If this is an old LSVD entity dir with only migrated markers remaining, remove it
		nestedVols := filepath.Join(dir, "volumes")
		nestedEntries, nerr := os.ReadDir(nestedVols)
		if nerr != nil {
			continue
		}
		allMigrated := true
		for _, nent := range nestedEntries {
			volDir := filepath.Join(nestedVols, nent.Name())
			if _, err := os.Stat(filepath.Join(volDir, "info.json.migrated")); err != nil {
				allMigrated = false
				break
			}
		}
		if allMigrated {
			c.log.Info("cleaning up migrated LSVD entity directory", "path", dir)
			os.RemoveAll(dir)
		}
	}
}

func (c *DiskVolumeController) Reconcile(ctx context.Context, volume *storage_v1alpha.DiskVolume, meta *entity.Meta) error {
	return c.reconcileVolume(ctx, volume)
}

func (c *DiskVolumeController) Index() entity.Attr {
	fullNodeId := "node/" + c.nodeId
	return entity.Ref(storage_v1alpha.DiskVolumeNodeIdId, entity.Id(fullNodeId))
}

func (c *DiskVolumeController) reconcileVolume(ctx context.Context, volume *storage_v1alpha.DiskVolume) error {
	entityId := string(volume.ID)
	c.log.Info("reconciling disk volume",
		"entity_id", entityId,
		"desired_state", volume.DesiredState,
		"actual_state", volume.ActualState,
	)

	switch volume.DesiredState {
	case storage_v1alpha.DV_PRESENT:
		return c.reconcileVolumePresent(ctx, volume)
	case storage_v1alpha.DV_ABSENT:
		return c.reconcileVolumeAbsent(ctx, volume)
	default:
		c.log.Warn("unknown desired state", "desired_state", volume.DesiredState)
		return nil
	}
}

func (c *DiskVolumeController) reconcileVolumePresent(ctx context.Context, volume *storage_v1alpha.DiskVolume) error {
	entityId := string(volume.ID)

	// Check if volume already exists in our state
	if existing := c.state.GetVolume(entityId); existing != nil {
		if volume.ActualState == storage_v1alpha.DV_READY {
			if existing.DiskPath != "" && !c.ops.VolumePathExists(existing.DiskPath) {
				c.log.Warn("volume directory missing, setting error state",
					"entity_id", entityId,
					"disk_path", existing.DiskPath,
				)
				c.setVolumeError(ctx, volume.ID, "volume directory missing")
				return fmt.Errorf("volume directory missing: %s", existing.DiskPath)
			}
			// For alwaysMount volumes, verify the mount is present
			if alwaysMount(existing.Mode) && (!existing.Mounted || !c.mntOps.IsMounted(existing.MountPath)) {
				c.log.Info("volume ready but not mounted, re-mounting", "entity_id", entityId)
				if err := c.ensureVolumeMount(ctx, entityId, existing); err != nil {
					c.log.Warn("failed to re-mount volume", "entity_id", entityId, "error", err)
					return err
				}
			}
			c.log.Debug("volume already ready", "entity_id", entityId)
			return nil
		}
		// Persisted state has a disk path and it exists on disk — reconcile entity
		if existing.DiskPath != "" && c.ops.VolumePathExists(existing.DiskPath) {
			c.log.Info("found persisted volume on disk, reconciling entity state",
				"entity_id", entityId,
				"disk_path", existing.DiskPath,
			)
			if err := c.updateVolumeState(ctx, volume.ID, storage_v1alpha.DV_READY, existing.VolumeId, ""); err != nil {
				c.log.Warn("failed to update volume state from persisted volume", "entity_id", entityId, "error", err)
			}
			// Ensure mount for alwaysMount volumes
			if alwaysMount(existing.Mode) {
				if err := c.ensureVolumeMount(ctx, entityId, existing); err != nil {
					c.log.Warn("failed to mount persisted volume", "entity_id", entityId, "error", err)
					return err
				}
			}
			return nil
		}
	}

	switch volume.ActualState {
	case storage_v1alpha.DV_PENDING:
		return c.createVolume(ctx, volume)
	case storage_v1alpha.DV_CREATING:
		c.log.Debug("volume is being created", "entity_id", entityId)
		return nil
	case storage_v1alpha.DV_READY:
		c.log.Warn("entity says DV_READY but no local state found, recovering", "entity_id", entityId)
		volumePath := c.getVolumePath(entityId)
		if !c.ops.VolumePathExists(volumePath) {
			c.log.Warn("volume directory missing despite DV_READY, resetting to pending", "entity_id", entityId)
			return c.createVolume(ctx, volume)
		}
		volumeId := volume.MountId
		if volumeId == "" {
			volumeId = entityId
			if idx := strings.LastIndex(entityId, "/"); idx != -1 {
				volumeId = entityId[idx+1:]
			}
		}
		volState := &VolumeState{
			EntityId:   entityId,
			VolumeId:   volumeId,
			Name:       volume.Name,
			DiskPath:   volumePath,
			SizeBytes:  units.GigaBytes(volume.SizeGb).Bytes().Int64(),
			Filesystem: volume.Filesystem,
			Mode:       volume.VolumeMode,
		}
		c.state.SetVolume(entityId, volState)
		if err := c.state.Save(); err != nil {
			c.log.Warn("failed to save recovered volume state", "error", err)
		}
		c.log.Info("recovered volume state from entity", "entity_id", entityId)
		// Mount the recovered volume if it's an alwaysMount volume
		if alwaysMount(volume.VolumeMode) {
			if err := c.ensureVolumeMount(ctx, entityId, volState); err != nil {
				c.log.Warn("failed to mount recovered volume", "entity_id", entityId, "error", err)
				return err
			}
		}
		return nil
	case storage_v1alpha.DV_ERROR:
		c.log.Info("volume in error state, attempting recreation", "entity_id", entityId)
		return c.createVolume(ctx, volume)
	default:
		c.log.Warn("unexpected actual state for present volume", "actual_state", volume.ActualState)
		return nil
	}
}

func (c *DiskVolumeController) reconcileVolumeAbsent(ctx context.Context, volume *storage_v1alpha.DiskVolume) error {
	entityId := string(volume.ID)

	switch volume.ActualState {
	case storage_v1alpha.DV_DELETED:
		volState := c.state.GetVolume(entityId)
		if volState != nil && volState.DiskPath != "" && c.ops.VolumePathExists(volState.DiskPath) {
			c.log.Info("cleaning up local volume data", "entity_id", entityId, "disk_path", volState.DiskPath)
			if err := c.ops.RemoveVolumeDir(volState.DiskPath); err != nil {
				c.log.Warn("failed to remove volume directory", "entity_id", entityId, "error", err)
			}
		}
		c.state.DeleteVolume(entityId)
		if err := c.state.Save(); err != nil {
			c.log.Warn("failed to save state after volume deletion", "error", err)
		}
		return nil
	case storage_v1alpha.DV_DELETING:
		return nil
	default:
		return c.deleteVolume(ctx, volume)
	}
}

func (c *DiskVolumeController) createVolume(ctx context.Context, volume *storage_v1alpha.DiskVolume) error {
	entityId := string(volume.ID)

	c.log.Info("creating disk volume",
		"entity_id", entityId,
		"size_gb", volume.SizeGb,
		"filesystem", volume.Filesystem,
	)

	if err := c.updateVolumeState(ctx, volume.ID, storage_v1alpha.DV_CREATING, "", ""); err != nil {
		c.log.Warn("failed to update volume state to creating", "error", err)
	}

	// Create volume directory
	volumePath := c.getVolumePath(entityId)
	if err := c.ops.CreateVolumeDir(volumePath); err != nil {
		c.setVolumeError(ctx, volume.ID, fmt.Sprintf("failed to create volume directory: %v", err))
		return fmt.Errorf("failed to create volume directory: %w", err)
	}

	// Create sparse disk image
	imagePath := filepath.Join(volumePath, "disk.img")
	sizeBytes := units.GigaBytes(volume.SizeGb).Bytes().Int64()

	// Check for LSVD volume to migrate
	migrated, err := c.migrateLSVDVolume(ctx, volume.DiskId, volume.Name, imagePath, sizeBytes)
	if err != nil {
		c.setVolumeError(ctx, volume.ID, fmt.Sprintf("LSVD migration failed: %v", err))
		return fmt.Errorf("LSVD migration failed for %s: %w", volume.Name, err)
	}

	if !migrated {
		if err := c.ops.CreateDiskImage(imagePath, sizeBytes); err != nil {
			c.setVolumeError(ctx, volume.ID, fmt.Sprintf("failed to create disk image: %v", err))
			return fmt.Errorf("failed to create disk image: %w", err)
		}
	}

	// Create log directory for accelerator volumes
	if volume.VolumeMode == storage_v1alpha.VM_ACCELERATOR {
		logDir := filepath.Join(volumePath, "logs")
		if err := c.ops.CreateVolumeDir(logDir); err != nil {
			c.setVolumeError(ctx, volume.ID, fmt.Sprintf("failed to create log directory: %v", err))
			return fmt.Errorf("failed to create log directory: %w", err)
		}
	}

	// Use MountId if set (e.g. from LSVD migration), otherwise entity suffix
	volumeId := volume.MountId
	if volumeId == "" {
		volumeId = entityId
		if idx := strings.LastIndex(entityId, "/"); idx != -1 {
			volumeId = entityId[idx+1:]
		}
	}

	// Update state
	volState := &VolumeState{
		EntityId:   entityId,
		VolumeId:   volumeId,
		Name:       volume.Name,
		DiskPath:   volumePath,
		SizeBytes:  sizeBytes,
		Filesystem: volume.Filesystem,
		Mode:       volume.VolumeMode,
	}
	c.state.SetVolume(entityId, volState)

	if err := c.state.Save(); err != nil {
		c.log.Warn("failed to save state after volume creation", "error", err)
	}

	// For alwaysMount volumes, attach and mount immediately
	if alwaysMount(volume.VolumeMode) {
		if err := c.ensureVolumeMount(ctx, entityId, volState); err != nil {
			c.setVolumeError(ctx, volume.ID, fmt.Sprintf("failed to mount volume: %v", err))
			return fmt.Errorf("failed to mount volume: %w", err)
		}
	}

	c.log.Info("disk volume created",
		"entity_id", entityId,
		"volume_id", volumeId,
		"image_path", imagePath,
	)

	if err := c.updateVolumeState(ctx, volume.ID, storage_v1alpha.DV_READY, volumeId, ""); err != nil {
		c.log.Warn("failed to update volume state to ready", "error", err)
	}

	// Also update the image_path in the entity
	if c.eac != nil {
		attrs := []entity.Attr{
			entity.Ref(entity.DBId, volume.ID),
			entity.String(storage_v1alpha.DiskVolumeImagePathId, imagePath),
		}
		if _, err := c.eac.Patch(ctx, attrs, 0); err != nil {
			c.log.Warn("failed to update image_path in entity", "error", err)
		}
	}

	return nil
}

func (c *DiskVolumeController) deleteVolume(ctx context.Context, volume *storage_v1alpha.DiskVolume) error {
	entityId := string(volume.ID)

	c.log.Info("deleting disk volume", "entity_id", entityId)

	if err := c.updateVolumeState(ctx, volume.ID, storage_v1alpha.DV_DELETING, "", ""); err != nil {
		c.log.Warn("failed to update volume state to deleting", "error", err)
	}

	volState := c.state.GetVolume(entityId)
	if volState == nil {
		c.log.Warn("volume not found in state", "entity_id", entityId)
		if err := c.updateVolumeState(ctx, volume.ID, storage_v1alpha.DV_DELETED, "", ""); err != nil {
			c.log.Warn("failed to update volume state to deleted", "error", err)
		}
		return nil
	}

	// Unmount before deleting if alwaysMount
	if alwaysMount(volState.Mode) && volState.Mounted {
		c.unmountVolume(volState)
	}

	if volState.DiskPath != "" {
		if err := c.ops.RemoveVolumeDir(volState.DiskPath); err != nil {
			if !os.IsNotExist(err) {
				return fmt.Errorf("failed to remove volume directory %s: %w", volState.DiskPath, err)
			}
		}
	}

	c.state.DeleteVolume(entityId)
	if err := c.state.Save(); err != nil {
		c.log.Warn("failed to save state after volume deletion", "error", err)
	}

	c.log.Info("disk volume deleted", "entity_id", entityId)

	if err := c.updateVolumeState(ctx, volume.ID, storage_v1alpha.DV_DELETED, "", ""); err != nil {
		c.log.Warn("failed to update volume state to deleted", "error", err)
	}

	return nil
}

func (c *DiskVolumeController) getVolumePath(volumeEntityId string) string {
	dirName := volumeEntityId
	if idx := strings.LastIndex(volumeEntityId, "/"); idx != -1 {
		dirName = volumeEntityId[idx+1:]
	}
	return filepath.Join(c.dataPath, "volumes", dirName)
}

func diskVolumeActualStateToId(state storage_v1alpha.DiskVolumeActualState) entity.Id {
	switch state {
	case storage_v1alpha.DV_PENDING:
		return storage_v1alpha.DiskVolumeActualStateDvPendingId
	case storage_v1alpha.DV_CREATING:
		return storage_v1alpha.DiskVolumeActualStateDvCreatingId
	case storage_v1alpha.DV_READY:
		return storage_v1alpha.DiskVolumeActualStateDvReadyId
	case storage_v1alpha.DV_DELETING:
		return storage_v1alpha.DiskVolumeActualStateDvDeletingId
	case storage_v1alpha.DV_DELETED:
		return storage_v1alpha.DiskVolumeActualStateDvDeletedId
	case storage_v1alpha.DV_ERROR:
		return storage_v1alpha.DiskVolumeActualStateDvErrorId
	default:
		return storage_v1alpha.DiskVolumeActualStateDvPendingId
	}
}

func (c *DiskVolumeController) updateVolumeState(ctx context.Context, id entity.Id, state storage_v1alpha.DiskVolumeActualState, volumeId, errorMsg string) error {
	if c.eac == nil {
		return nil
	}

	stateId := diskVolumeActualStateToId(state)

	attrs := []entity.Attr{
		entity.Ref(entity.DBId, id),
		entity.Ref(storage_v1alpha.DiskVolumeActualStateId, stateId),
	}

	if volumeId != "" {
		attrs = append(attrs, entity.String(storage_v1alpha.DiskVolumeVolumeIdId, volumeId))
	}

	attrs = append(attrs, entity.String(storage_v1alpha.DiskVolumeErrorMessageId, errorMsg))

	_, err := c.eac.Patch(ctx, attrs, 0)
	return err
}

func (c *DiskVolumeController) setVolumeError(ctx context.Context, id entity.Id, errorMsg string) {
	if err := c.updateVolumeState(ctx, id, storage_v1alpha.DV_ERROR, "", errorMsg); err != nil {
		c.log.Warn("failed to set volume error state", "entity_id", id, "error", err)
	}
}

// migrateLSVDVolume checks if the parent disk has an LsvdVolumeId, locates the
// old LSVD data directory, and migrates its contents to a new disk image.
// Returns true if migration was performed.
func (c *DiskVolumeController) migrateLSVDVolume(ctx context.Context, diskId entity.Id, volumeName, destImagePath string, sizeBytes int64) (bool, error) {
	lsvdDir, lsvdVolName, err := c.findLSVDVolume(ctx, diskId)
	if err != nil {
		return false, err
	}
	if lsvdDir == "" {
		return false, nil
	}

	return c.copyLSVDToImage(ctx, lsvdDir, lsvdVolName, volumeName, destImagePath, sizeBytes)
}

// copyLSVDToImage reads an LSVD volume from lsvdDir/volumes/lsvdVolName and
// writes its contents into a sparse disk image at destImagePath.
func (c *DiskVolumeController) copyLSVDToImage(ctx context.Context, lsvdDir, lsvdVolName, volumeName, destImagePath string, sizeBytes int64) (bool, error) {
	// Verify info.json exists before attempting migration
	infoPath := filepath.Join(lsvdDir, "volumes", lsvdVolName, "info.json")
	if _, err := os.Stat(infoPath); err != nil {
		if os.IsNotExist(err) {
			c.log.Info("LSVD directory found but info.json missing, skipping migration",
				"lsvd_dir", lsvdDir, "volume", lsvdVolName)
			return false, nil
		}
		return false, fmt.Errorf("checking LSVD info.json at %s: %w", infoPath, err)
	}

	c.log.Info("found LSVD volume, migrating data",
		"disk_name", volumeName,
		"lsvd_dir", lsvdDir,
		"lsvd_volume", lsvdVolName,
		"dest", destImagePath)

	disk, err := lsvd.NewDisk(ctx, c.log, lsvdDir,
		lsvd.WithVolumeName(lsvdVolName),
		lsvd.ReadOnly(),
		lsvd.AutoCreate(false))
	if err != nil {
		return false, fmt.Errorf("opening LSVD volume %q in %s: %w", lsvdVolName, lsvdDir, err)
	}
	defer disk.Close(ctx)

	lsvdSize := disk.Size()
	if lsvdSize > sizeBytes {
		sizeBytes = lsvdSize
	}

	out, err := os.Create(destImagePath)
	if err != nil {
		return false, fmt.Errorf("creating image file: %w", err)
	}
	defer out.Close()

	if err := out.Truncate(sizeBytes); err != nil {
		return false, fmt.Errorf("truncating image to %d bytes: %w", sizeBytes, err)
	}

	if sizeBytes%int64(lsvd.BlockSize) != 0 {
		return false, fmt.Errorf("volume size %d is not aligned to block size %d", sizeBytes, lsvd.BlockSize)
	}
	totalBlocks := sizeBytes / int64(lsvd.BlockSize)
	const chunkBlocks = 1024
	zeros := make([]byte, chunkBlocks*lsvd.BlockSize)
	lsvdCtx := lsvd.NewContext(ctx)
	defer lsvdCtx.Close()

	var written int64
	for lba := int64(0); lba < totalBlocks; lba += chunkBlocks {
		blocks := chunkBlocks
		if lba+int64(blocks) > totalBlocks {
			blocks = int(totalBlocks - lba)
		}
		lsvdCtx.Reset()

		data, err := disk.ReadExtent(lsvdCtx, lsvd.Extent{
			LBA:    lsvd.LBA(lba),
			Blocks: uint32(blocks),
		})
		if err != nil {
			return false, fmt.Errorf("reading LSVD extent at LBA %d: %w", lba, err)
		}
		raw := data.ReadData()

		if isAllZeros(raw, zeros[:len(raw)]) {
			continue
		}

		offset := lba * int64(lsvd.BlockSize)
		if _, err := out.WriteAt(raw, offset); err != nil {
			return false, fmt.Errorf("writing at offset %d: %w", offset, err)
		}
		written += int64(len(raw))
	}

	c.log.Info("LSVD migration complete",
		"volume_name", volumeName,
		"total_bytes", lsvdSize,
		"written_bytes", written)

	migratedPath := infoPath + ".migrated"
	if err := os.Rename(infoPath, migratedPath); err != nil {
		return false, fmt.Errorf("renaming migration marker %s: %w", infoPath, err)
	}

	return true, nil
}

// findLSVDVolume looks up the parent disk entity and its associated lsvd_volume
// entity to deterministically locate the old LSVD data directory. The old LSVD
// system stored volume data at:
//
//	disk-data/volumes/{lsvd-vol-entity-suffix}/volumes/{volumeId}/info.json
//
// where the entity suffix comes from the lsvd_volume entity ID and volumeId
// is stored in the lsvd_volume.volume_id field.
//
// Returns the LSVD root dir and the volume name within it, or empty strings
// if no LSVD data is found.
func (c *DiskVolumeController) findLSVDVolume(ctx context.Context, diskId entity.Id) (lsvdDir, lsvdVolName string, err error) {
	if c.eac == nil || diskId == "" {
		return "", "", nil
	}

	// Look up the disk to check if it has an LsvdVolumeId at all.
	resp, err := c.eac.Get(ctx, string(diskId))
	if err != nil {
		return "", "", fmt.Errorf("looking up disk %s: %w", diskId, err)
	}

	var disk storage_v1alpha.Disk
	disk.Decode(resp.Entity().Entity())

	if disk.LsvdVolumeId == "" {
		return "", "", nil
	}

	// Find the lsvd_volume entity for this disk using the disk_id index.
	// disk.LsvdVolumeId is the LSVD volume UUID, but we need the entity
	// suffix (used as the directory name) and the volume_id field.
	lsvdResp, err := c.eac.List(ctx, entity.Ref(storage_v1alpha.LsvdVolumeDiskIdId, diskId))
	if err != nil {
		c.log.Info("failed to list lsvd_volume entities, falling back to directory scan",
			"disk", diskId, "error", err)
		return c.findLSVDVolumeByDirScan(diskId)
	}

	values := lsvdResp.Values()
	if len(values) == 0 {
		c.log.Info("no lsvd_volume entity found for disk, falling back to directory scan",
			"disk", diskId)
		return c.findLSVDVolumeByDirScan(diskId)
	}

	var lsvdVol storage_v1alpha.LsvdVolume
	lsvdVol.Decode(values[0].Entity())

	if lsvdVol.VolumeId == "" {
		c.log.Info("lsvd_volume entity has no volume_id, falling back to directory scan",
			"disk", diskId, "lsvd_volume", lsvdVol.ID)
		return c.findLSVDVolumeByDirScan(diskId)
	}

	// The old volume controller stored data at disk-data/volumes/{entity-suffix}/.
	// Extract the suffix from the entity ID (e.g., "lsvd_volume/lsvd-vol-ABC" → "lsvd-vol-ABC").
	entitySuffix := string(lsvdVol.ID)
	if idx := strings.LastIndex(entitySuffix, "/"); idx != -1 {
		entitySuffix = entitySuffix[idx+1:]
	}

	lsvdEntityDir := filepath.Join(c.dataPath, "volumes", entitySuffix)
	if _, err := os.Stat(lsvdEntityDir); err != nil {
		if os.IsNotExist(err) {
			c.log.Info("LSVD entity directory not found on disk",
				"disk", diskId,
				"lsvd_volume", lsvdVol.ID,
				"expected_path", lsvdEntityDir)
			return "", "", nil
		}
		return "", "", fmt.Errorf("checking LSVD directory %s: %w", lsvdEntityDir, err)
	}

	c.log.Info("found LSVD volume via entity lookup",
		"disk", diskId,
		"lsvd_volume", lsvdVol.ID,
		"volume_id", lsvdVol.VolumeId,
		"dir", lsvdEntityDir)

	return lsvdEntityDir, lsvdVol.VolumeId, nil
}

// findLSVDVolumeByDirScan is the fallback path when the lsvd_volume entity
// is missing or has no volume_id. It scans the volumes directory for any
// lsvd-vol-* directory containing a nested volume with info.json.
func (c *DiskVolumeController) findLSVDVolumeByDirScan(diskId entity.Id) (string, string, error) {
	volumesDir := filepath.Join(c.dataPath, "volumes")
	entries, err := os.ReadDir(volumesDir)
	if err != nil {
		return "", "", nil
	}

	for _, ent := range entries {
		if !ent.IsDir() || !strings.HasPrefix(ent.Name(), "lsvd-vol-") {
			continue
		}

		lsvdEntityDir := filepath.Join(volumesDir, ent.Name())
		nestedVolsDir := filepath.Join(lsvdEntityDir, "volumes")
		nestedEntries, nerr := os.ReadDir(nestedVolsDir)
		if nerr != nil {
			continue
		}

		for _, nent := range nestedEntries {
			if !nent.IsDir() {
				continue
			}
			infoPath := filepath.Join(nestedVolsDir, nent.Name(), "info.json")
			if _, serr := os.Stat(infoPath); serr == nil {
				c.log.Info("found LSVD volume via directory scan",
					"disk", diskId,
					"dir", lsvdEntityDir,
					"volume", nent.Name())
				return lsvdEntityDir, nent.Name(), nil
			}
		}
	}

	c.log.Info("no LSVD volume data found on disk", "disk", diskId)
	return "", "", nil
}

// diskMountBasePath is the standard path where disk volumes are mounted,
// matching the path used by the DiskLeaseController.
const diskMountBasePath = "/var/lib/miren/disks"

// getMountPath returns the mount path for a volume.
func (c *DiskVolumeController) getMountPath(volumeId string) string {
	return filepath.Join(diskMountBasePath, volumeId)
}

// ensureVolumeMount loop-attaches, formats if needed, and mounts a volume.
// It updates the VolumeState with mount info and persists state.
// If the volume is already mounted, this is a no-op.
func (c *DiskVolumeController) ensureVolumeMount(ctx context.Context, entityId string, volState *VolumeState) error {
	mountPath := c.getMountPath(volState.VolumeId)

	// Already mounted — nothing to do
	if c.mntOps.IsMounted(mountPath) {
		if !volState.Mounted || volState.MountPath != mountPath {
			volState.MountPath = mountPath
			volState.Mounted = true
			c.state.SetVolume(entityId, volState)
			c.state.Save()
		}
		return nil
	}

	imagePath := filepath.Join(volState.DiskPath, "disk.img")

	c.log.Info("mounting volume",
		"entity_id", entityId,
		"image_path", imagePath,
		"mount_path", mountPath,
	)

	// If we had a previous loop device recorded, try to detach it first.
	// After a restart the device is already gone, so ignore errors.
	if volState.DevicePath != "" {
		_ = c.mntOps.LoopDetach(volState.DevicePath)
		volState.DevicePath = ""
	}

	devicePath, err := c.mntOps.LoopAttach(imagePath)
	if err != nil {
		return fmt.Errorf("failed to attach loop device: %w", err)
	}

	filesystem := volState.Filesystem
	if filesystem == "" {
		filesystem = "ext4"
	}

	formatted, err := c.mntOps.IsFormatted(ctx, devicePath, filesystem)
	if err != nil {
		c.mntOps.LoopDetach(devicePath)
		return fmt.Errorf("failed to check if formatted: %w", err)
	}

	if !formatted {
		c.log.Info("formatting device", "device", devicePath, "filesystem", filesystem)
		formatDeadline := time.Now().Add(1 * time.Minute)
		backoff := 1 * time.Second
		maxBackoff := 10 * time.Second

		for {
			err := c.mntOps.FormatDevice(ctx, devicePath, filesystem)
			if err == nil {
				break
			}

			c.log.Error("format device failed, will retry", "device", devicePath, "error", err)

			if time.Now().After(formatDeadline) {
				c.mntOps.LoopDetach(devicePath)
				return fmt.Errorf("failed to format device after retries: %w", err)
			}

			select {
			case <-ctx.Done():
				c.mntOps.LoopDetach(devicePath)
				return ctx.Err()
			case <-time.After(backoff):
			}

			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}

	if err := c.mntOps.CreateDir(mountPath, 0755); err != nil {
		c.mntOps.LoopDetach(devicePath)
		return fmt.Errorf("failed to create mount point: %w", err)
	}

	if err := c.mntOps.Mount(devicePath, mountPath, filesystem, false); err != nil {
		c.mntOps.LoopDetach(devicePath)
		return fmt.Errorf("failed to mount: %w", err)
	}

	// Update state with mount info
	volState.DevicePath = devicePath
	volState.MountPath = mountPath
	volState.Mounted = true
	c.state.SetVolume(entityId, volState)

	if err := c.state.Save(); err != nil {
		c.log.Warn("failed to save state after volume mount", "error", err)
	}

	c.log.Info("volume mounted",
		"entity_id", entityId,
		"device", devicePath,
		"mount_path", mountPath,
	)

	return nil
}

// unmountVolume unmounts and detaches a loop device for an alwaysMount volume.
func (c *DiskVolumeController) unmountVolume(volState *VolumeState) {
	if volState.MountPath != "" && c.mntOps.IsMounted(volState.MountPath) {
		if err := c.mntOps.Unmount(volState.MountPath); err != nil {
			c.log.Warn("failed to unmount volume", "entity_id", volState.EntityId, "error", err)
		}
	}
	if volState.DevicePath != "" {
		if err := c.mntOps.LoopDetach(volState.DevicePath); err != nil {
			c.log.Warn("failed to detach loop device", "entity_id", volState.EntityId, "error", err)
		}
	}
	volState.Mounted = false
	volState.DevicePath = ""
	volState.MountPath = ""
}

// Shutdown unmounts all disk volumes and detaches their backing devices.
// It uses the actual kernel mount table rather than trusting persisted state,
// finding all mounts under diskMountBasePath and tearing them down.
// If keepMounts is set (reload), everything is left in place for the new process.
func (c *DiskVolumeController) Shutdown() {
	if c.keepMounts {
		c.log.Info("keeping volumes mounted for reload")
		return
	}

	// Scan the actual kernel mount table for our mounts
	activeMounts := c.mntOps.FindMounts(diskMountBasePath)

	for _, am := range activeMounts {
		c.log.Info("shutting down disk mount",
			"mount_path", am.MountPath,
			"device", am.Device,
		)

		if err := c.mntOps.Unmount(am.MountPath); err != nil {
			c.log.Warn("failed to unmount on shutdown", "mount_path", am.MountPath, "error", err)
			continue
		}

		if strings.HasPrefix(am.Device, "/dev/lbd") {
			if err := c.mntOps.LbdDetach(context.Background(), am.Device); err != nil {
				c.log.Warn("failed to detach lbd on shutdown", "device", am.Device, "error", err)
			}
		} else if strings.HasPrefix(am.Device, "/dev/loop") {
			if err := c.mntOps.LoopDetach(am.Device); err != nil {
				c.log.Warn("failed to detach loop on shutdown", "device", am.Device, "error", err)
			}
		}
	}

	// Update persisted state to reflect unmounted volumes
	for _, vol := range c.state.ListVolumes() {
		if vol.Mounted {
			vol.Mounted = false
			vol.DevicePath = ""
			vol.MountPath = ""
			c.state.SetVolume(vol.EntityId, vol)
		}
	}

	if err := c.state.Save(); err != nil {
		c.log.Warn("failed to save state after shutdown", "error", err)
	}
}

func isAllZeros(data, zeros []byte) bool {
	for i := range data {
		if data[i] != zeros[i] {
			return false
		}
	}
	return true
}

// ReconcileWithEntities reconciles local state with entity server
func (c *DiskVolumeController) ReconcileWithEntities(ctx context.Context) error {
	if c.eac == nil {
		return fmt.Errorf("entity access client not set; call SetEAC before reconciling")
	}

	fullNodeId := "node/" + c.nodeId
	nodeIdRef := entity.Id(fullNodeId)
	indexAttr := entity.Ref(storage_v1alpha.DiskVolumeNodeIdId, nodeIdRef)

	resp, err := c.eac.List(ctx, indexAttr)
	if err != nil {
		return fmt.Errorf("failed to list disk_volume entities: %w", err)
	}

	values := resp.Values()

	entityIds := make(map[string]struct{}, len(values))

	for _, entResp := range values {
		var volume storage_v1alpha.DiskVolume
		volume.Decode(entResp.Entity())

		entityIds[string(volume.ID)] = struct{}{}

		if string(volume.NodeId) != fullNodeId {
			continue
		}

		if err := c.reconcileVolume(ctx, &volume); err != nil {
			c.log.Error("failed to reconcile disk volume",
				"entity_id", volume.ID,
				"error", err,
			)
		}
	}

	// Clean up orphaned volumes
	orphanCleaned := false
	for _, volState := range c.state.ListVolumes() {
		if !strings.HasPrefix(volState.EntityId, "disk_volume/") {
			continue
		}
		if _, exists := entityIds[volState.EntityId]; exists {
			continue
		}

		c.log.Info("cleaning up orphaned disk volume", "entity_id", volState.EntityId)

		// Unmount orphaned alwaysMount volumes before removing
		if alwaysMount(volState.Mode) && volState.Mounted {
			c.unmountVolume(volState)
		}

		if volState.DiskPath != "" {
			if err := c.ops.RemoveVolumeDir(volState.DiskPath); err != nil {
				c.log.Warn("failed to remove orphaned volume directory", "entity_id", volState.EntityId, "error", err)
			}
		}

		c.state.DeleteVolume(volState.EntityId)
		orphanCleaned = true
	}

	if orphanCleaned {
		if err := c.state.Save(); err != nil {
			c.log.Warn("failed to save state after orphan cleanup", "error", err)
		}
	}

	// Re-mount any alwaysMount volumes that are DV_READY but not currently mounted (restart recovery)
	for _, volState := range c.state.ListVolumes() {
		if !alwaysMount(volState.Mode) {
			continue
		}
		if !volState.Mounted || !c.mntOps.IsMounted(volState.MountPath) {
			c.log.Info("re-mounting alwaysMount volume after reconcile", "entity_id", volState.EntityId)
			if err := c.ensureVolumeMount(ctx, volState.EntityId, volState); err != nil {
				c.log.Warn("failed to re-mount volume", "entity_id", volState.EntityId, "error", err)
			}
		}
	}

	return nil
}
