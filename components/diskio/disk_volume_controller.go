package diskio

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"miren.dev/runtime/api/entityserver/entityserver_v1alpha"
	"miren.dev/runtime/api/storage/storage_v1alpha"
	"miren.dev/runtime/lsvd"
	"miren.dev/runtime/pkg/entity"
	"miren.dev/runtime/pkg/units"
)

// DiskVolumeController watches disk_volume entities and manages sparse disk images
// using loop devices.
type DiskVolumeController struct {
	log      *slog.Logger
	dataPath string
	nodeId   string
	eac      *entityserver_v1alpha.EntityAccessClient
	state    *State
	ops      DiskVolumeOps
}

func NewDiskVolumeController(log *slog.Logger, dataPath, nodeId string, state *State, ops DiskVolumeOps) *DiskVolumeController {
	return &DiskVolumeController{
		log:      log.With("module", "disk-volume"),
		dataPath: dataPath,
		nodeId:   nodeId,
		state:    state,
		ops:      ops,
	}
}

func (c *DiskVolumeController) SetEAC(eac *entityserver_v1alpha.EntityAccessClient) {
	c.eac = eac
}

func (c *DiskVolumeController) Init(ctx context.Context) error {
	c.cleanupMigratedLSVD()
	return nil
}

func (c *DiskVolumeController) cleanupMigratedLSVD() {
	volumesDir := filepath.Join(c.dataPath, "volumes")
	entries, err := os.ReadDir(volumesDir)
	if err != nil {
		return
	}

	for _, ent := range entries {
		infoPath := filepath.Join(volumesDir, ent.Name(), "info.json")
		if _, err := os.Stat(infoPath); err == nil {
			return // Unmigrated LSVD volume exists, don't clean up
		}
	}

	segmentsDir := filepath.Join(c.dataPath, "segments")
	if _, err := os.Stat(segmentsDir); err == nil {
		c.log.Info("all LSVD volumes migrated, cleaning up segments directory")
		os.RemoveAll(segmentsDir)
	}

	for _, ent := range entries {
		migratedPath := filepath.Join(volumesDir, ent.Name(), "info.json.migrated")
		if _, err := os.Stat(migratedPath); err == nil {
			os.Remove(migratedPath)
			segFile := filepath.Join(volumesDir, ent.Name(), "segments")
			os.Remove(segFile)
			os.Remove(filepath.Join(volumesDir, ent.Name()))
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
		volumeId := entityId
		if idx := strings.LastIndex(entityId, "/"); idx != -1 {
			volumeId = entityId[idx+1:]
		}
		c.state.SetVolume(entityId, &VolumeState{
			EntityId:   entityId,
			VolumeId:   volumeId,
			Name:       volume.Name,
			DiskPath:   volumePath,
			SizeBytes:  units.GigaBytes(volume.SizeGb).Bytes().Int64(),
			Filesystem: volume.Filesystem,
			Mode:       volume.VolumeMode,
		})
		if err := c.state.Save(); err != nil {
			c.log.Warn("failed to save recovered volume state", "error", err)
		}
		c.log.Info("recovered volume state from entity", "entity_id", entityId)
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
	migrated, err := c.migrateLSVDVolume(ctx, volume.Name, imagePath, sizeBytes)
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

	// Use the entity ID suffix as the volume ID
	volumeId := entityId
	if idx := strings.LastIndex(entityId, "/"); idx != -1 {
		volumeId = entityId[idx+1:]
	}

	// Update state
	c.state.SetVolume(entityId, &VolumeState{
		EntityId:   entityId,
		VolumeId:   volumeId,
		Name:       volume.Name,
		DiskPath:   volumePath,
		SizeBytes:  sizeBytes,
		Filesystem: volume.Filesystem,
		Mode:       volume.VolumeMode,
	})

	if err := c.state.Save(); err != nil {
		c.log.Warn("failed to save state after volume creation", "error", err)
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

	if volState.DiskPath != "" {
		if err := c.ops.RemoveVolumeDir(volState.DiskPath); err != nil {
			c.log.Warn("failed to remove volume directory", "path", volState.DiskPath, "error", err)
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

// migrateLSVDVolume checks if an LSVD volume with the given name exists and migrates
// its data to a universal mode disk image. Returns true if migration was performed.
func (c *DiskVolumeController) migrateLSVDVolume(ctx context.Context, volumeName, destImagePath string, sizeBytes int64) (bool, error) {
	infoPath := filepath.Join(c.dataPath, "volumes", volumeName, "info.json")
	if _, err := os.Stat(infoPath); err != nil {
		return false, nil
	}

	c.log.Info("found LSVD volume, migrating to universal mode",
		"volume_name", volumeName,
		"dest", destImagePath)

	disk, err := lsvd.NewDisk(ctx, c.log, c.dataPath,
		lsvd.WithVolumeName(volumeName),
		lsvd.ReadOnly(),
		lsvd.AutoCreate(false))
	if err != nil {
		return false, fmt.Errorf("opening LSVD volume %q: %w", volumeName, err)
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

	if lsvdSize%int64(lsvd.BlockSize) != 0 {
		return false, fmt.Errorf("LSVD volume size %d is not aligned to block size %d", lsvdSize, lsvd.BlockSize)
	}
	totalBlocks := lsvdSize / int64(lsvd.BlockSize)
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

	return nil
}
