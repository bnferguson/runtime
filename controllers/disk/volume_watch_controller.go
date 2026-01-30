package disk

import (
	"context"
	"log/slog"

	"miren.dev/runtime/api/entityserver/entityserver_v1alpha"
	"miren.dev/runtime/api/storage/storage_v1alpha"
	"miren.dev/runtime/pkg/controller"
	"miren.dev/runtime/pkg/entity"
)

// VolumeWatchController watches for lsvd_volume state changes and triggers
// re-reconciliation of the parent disk entity. This bridges the gap where
// the disk controller creates an lsvd_volume and needs to know when lsvd-server
// finishes provisioning it.
type VolumeWatchController struct {
	Log *slog.Logger
	EAC *entityserver_v1alpha.EntityAccessClient

	DiskController *controller.ReconcileController
}

// NewVolumeWatchController creates a new volume watch controller.
func NewVolumeWatchController(log *slog.Logger, eac *entityserver_v1alpha.EntityAccessClient, diskController *controller.ReconcileController) *VolumeWatchController {
	return &VolumeWatchController{
		Log:            log.With("module", "volume-watch"),
		EAC:            eac,
		DiskController: diskController,
	}
}

func (v *VolumeWatchController) Init(ctx context.Context) error {
	return nil
}

func (v *VolumeWatchController) Create(ctx context.Context, vol *storage_v1alpha.LsvdVolume, meta *entity.Meta) error {
	return nil
}

func (v *VolumeWatchController) Update(ctx context.Context, vol *storage_v1alpha.LsvdVolume, meta *entity.Meta) error {
	if vol.DiskId == "" {
		return nil
	}

	v.Log.Debug("lsvd_volume changed, re-reconciling parent disk",
		"volume", vol.ID,
		"disk", vol.DiskId,
		"actual_state", vol.ActualState)

	resp, err := v.EAC.Get(ctx, string(vol.DiskId))
	if err != nil {
		v.Log.Warn("failed to get parent disk for volume change",
			"volume", vol.ID,
			"disk", vol.DiskId,
			"error", err)
		return nil
	}

	v.DiskController.Enqueue(controller.Event{
		Type:   controller.EventUpdated,
		Id:     vol.DiskId,
		Entity: resp.Entity().Entity(),
	})

	return nil
}

func (v *VolumeWatchController) Delete(ctx context.Context, id entity.Id) error {
	return nil
}
