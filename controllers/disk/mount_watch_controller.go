package disk

import (
	"context"
	"log/slog"

	"miren.dev/runtime/api/entityserver/entityserver_v1alpha"
	"miren.dev/runtime/api/storage/storage_v1alpha"
	"miren.dev/runtime/pkg/controller"
	"miren.dev/runtime/pkg/entity"
)

// MountWatchController watches for lsvd_mount state changes and triggers
// re-reconciliation of the parent disk_lease entity. This bridges the gap where
// the disk lease controller creates an lsvd_mount and needs to know when the
// mount controller finishes mounting it.
type MountWatchController struct {
	Log *slog.Logger
	EAC *entityserver_v1alpha.EntityAccessClient

	LeaseController *controller.ReconcileController
}

// NewMountWatchController creates a new mount watch controller.
func NewMountWatchController(log *slog.Logger, eac *entityserver_v1alpha.EntityAccessClient, leaseController *controller.ReconcileController) *MountWatchController {
	return &MountWatchController{
		Log:             log.With("module", "mount-watch"),
		EAC:             eac,
		LeaseController: leaseController,
	}
}

func (m *MountWatchController) Init(ctx context.Context) error {
	return nil
}

func (m *MountWatchController) Create(ctx context.Context, mount *storage_v1alpha.LsvdMount, meta *entity.Meta) error {
	return nil
}

func (m *MountWatchController) Update(ctx context.Context, mount *storage_v1alpha.LsvdMount, meta *entity.Meta) error {
	if mount.DiskLeaseId == "" {
		return nil
	}

	m.Log.Debug("lsvd_mount changed, re-reconciling parent disk lease",
		"mount", mount.ID,
		"lease", mount.DiskLeaseId,
		"actual_state", mount.ActualState)

	resp, err := m.EAC.Get(ctx, string(mount.DiskLeaseId))
	if err != nil {
		m.Log.Warn("failed to get parent disk lease for mount change",
			"mount", mount.ID,
			"lease", mount.DiskLeaseId,
			"error", err)
		return nil
	}

	m.LeaseController.Enqueue(controller.Event{
		Type:   controller.EventUpdated,
		Id:     mount.DiskLeaseId,
		Entity: resp.Entity().Entity(),
	})

	return nil
}

func (m *MountWatchController) Delete(ctx context.Context, id entity.Id, obj *storage_v1alpha.LsvdMount) error {
	return nil
}
