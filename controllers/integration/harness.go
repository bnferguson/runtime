package integration

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	"miren.dev/runtime/api/entityserver/entityserver_v1alpha"
	"miren.dev/runtime/api/storage/storage_v1alpha"
	lsvdserver "miren.dev/runtime/components/lsvd/server"
	"miren.dev/runtime/controllers/disk"
	"miren.dev/runtime/pkg/controller"
	"miren.dev/runtime/pkg/entity"
	"miren.dev/runtime/pkg/entity/testutils"
)

const testNodeId = "test-node-1"

// TestHarness wires all disk-lifecycle controllers together with a shared
// in-memory entity server for integration testing.
type TestHarness struct {
	T      *testing.T
	Server *testutils.InMemEntityServer
	Log    *slog.Logger
	EAC    *entityserver_v1alpha.EntityAccessClient

	// Runner-side controllers
	DiskCtrl      *disk.DiskController
	DiskLeaseCtrl *disk.DiskLeaseController

	// LSVD controllers (with mock ops)
	LsvdVolumeCtrl *lsvdserver.VolumeController
	LsvdMountCtrl  *lsvdserver.MountController
	LsvdState      *lsvdserver.State

	// Fake sandbox
	FakeSandbox *FakeSandboxController

	// ReconcileControllers (for ProcessEventForTest)
	DiskRC      *controller.ReconcileController
	DiskLeaseRC *controller.ReconcileController
	LsvdVolRC   *controller.ReconcileController
	LsvdMntRC   *controller.ReconcileController

	// Mock ops for test inspection
	MockVolumeOps *mockVolumeOps
	MockMountOps  *mockMountOps
}

// NewTestHarness creates a fully wired test harness for disk lifecycle integration tests.
func NewTestHarness(t *testing.T) *TestHarness {
	ctx := context.Background()
	log := testutils.TestDebugLogger(t)

	es, cleanup := testutils.NewInMemEntityServer(t)
	t.Cleanup(cleanup)

	eac := es.EAC

	// Create LSVD state backed by a temp file so Save() succeeds
	dataPath := t.TempDir()
	lsvdState := lsvdserver.NewState()
	lsvdState.SetPath(dataPath)

	// Create mock ops
	volOps := newMockVolumeOps()
	mntOps := newMockMountOps()
	lsvdVolCtrl := lsvdserver.NewVolumeController(log, dataPath, testNodeId, lsvdState, volOps)
	lsvdVolCtrl.SetEAC(eac)

	lsvdMntCtrl := lsvdserver.NewMountController(log, dataPath, testNodeId, lsvdState, mntOps)
	lsvdMntCtrl.SetEAC(eac)

	// Create disk controllers
	diskCtrl := disk.NewDiskController(log, eac, testNodeId)
	diskCtrl.Init(ctx) //nolint:errcheck
	diskLeaseCtrl := disk.NewDiskLeaseController(log, eac, testNodeId)
	diskLeaseCtrl.Init(ctx) //nolint:errcheck

	// Create ReconcileControllers for each.
	// We do NOT create watch controllers because ReconcileAll already reconciles
	// all entity kinds each iteration, making the watch→Enqueue pattern unnecessary.
	diskRC := controller.NewReconcileController(
		"disk",
		log,
		entity.Ref(entity.EntityKind, storage_v1alpha.KindDisk),
		eac,
		controller.AdaptController(diskCtrl),
		0, 1,
	)

	diskLeaseRC := controller.NewReconcileController(
		"disk-lease",
		log,
		entity.Ref(entity.EntityKind, storage_v1alpha.KindDiskLease),
		eac,
		controller.AdaptController(diskLeaseCtrl),
		0, 1,
	)

	lsvdVolRC := controller.NewReconcileController(
		"lsvd-volume",
		log,
		lsvdVolCtrl.Index(),
		eac,
		controller.AdaptReconcileController(lsvdVolCtrl),
		0, 1,
	)

	lsvdMntRC := controller.NewReconcileController(
		"lsvd-mount",
		log,
		lsvdMntCtrl.Index(),
		eac,
		controller.AdaptReconcileController(lsvdMntCtrl),
		0, 1,
	)

	// Create fake sandbox controller
	fakeSandbox := NewFakeSandboxController(log, eac, testNodeId)

	return &TestHarness{
		T:              t,
		Server:         es,
		Log:            log,
		EAC:            eac,
		DiskCtrl:       diskCtrl,
		DiskLeaseCtrl:  diskLeaseCtrl,
		LsvdVolumeCtrl: lsvdVolCtrl,
		LsvdMountCtrl:  lsvdMntCtrl,
		LsvdState:      lsvdState,
		FakeSandbox:    fakeSandbox,
		DiskRC:         diskRC,
		DiskLeaseRC:    diskLeaseRC,
		LsvdVolRC:      lsvdVolRC,
		LsvdMntRC:      lsvdMntRC,
		MockVolumeOps:  volOps,
		MockMountOps:   mntOps,
	}
}

// ReconcileEntity fetches an entity by ID and processes it through the appropriate controller.
func (h *TestHarness) ReconcileEntity(ctx context.Context, id entity.Id) error {
	resp, err := h.EAC.Get(ctx, id.String())
	if err != nil {
		return err
	}

	ent := resp.Entity().Entity()
	event := controller.Event{
		Type:   controller.EventUpdated,
		Id:     id,
		Entity: ent,
	}

	rc := h.controllerForEntity(id)
	if rc == nil {
		return fmt.Errorf("no controller for entity %s", id)
	}

	return rc.ProcessEventForTest(ctx, event)
}

// controllerForEntity returns the appropriate ReconcileController based on entity kind prefix.
func (h *TestHarness) controllerForEntity(id entity.Id) *controller.ReconcileController {
	s := id.String()
	switch {
	case hasPrefix(s, "disk-lease/"):
		return h.DiskLeaseRC
	case hasPrefix(s, "disk/"):
		return h.DiskRC
	case hasPrefix(s, "lsvd_volume/"), hasPrefix(s, "dev.miren.storage/kind.lsvd_volume/"):
		return h.LsvdVolRC
	case hasPrefix(s, "lsvd_mount/"), hasPrefix(s, "dev.miren.storage/kind.lsvd_mount/"):
		return h.LsvdMntRC
	default:
		return nil
	}
}

// ReconcileAll reconciles all disk-related entities across all controllers until
// the system converges (no entity revisions change) or maxIterations is reached.
func (h *TestHarness) ReconcileAll(ctx context.Context, maxIterations int) {
	h.T.Helper()

	if maxIterations <= 0 {
		maxIterations = 20
	}

	nodeId := entity.Id("node/" + testNodeId)

	indexes := []struct {
		index entity.Attr
		rc    *controller.ReconcileController
	}{
		{entity.Ref(entity.EntityKind, storage_v1alpha.KindDisk), h.DiskRC},
		{entity.Ref(storage_v1alpha.LsvdVolumeNodeIdId, nodeId), h.LsvdVolRC},
		{entity.Ref(storage_v1alpha.LsvdMountNodeIdId, nodeId), h.LsvdMntRC},
		{entity.Ref(entity.EntityKind, storage_v1alpha.KindDiskLease), h.DiskLeaseRC},
	}

	for i := 0; i < maxIterations; i++ {
		before := h.snapshotRevisions(ctx)

		for _, idx := range indexes {
			h.reconcileByIndex(ctx, idx.index, idx.rc)
		}

		after := h.snapshotRevisions(ctx)

		if revisionsEqual(before, after) {
			h.Log.Info("ReconcileAll converged", "iterations", i+1)
			return
		}
	}

	h.Log.Warn("ReconcileAll hit max iterations", "max", maxIterations)
}

// reconcileByIndex lists all entities matching the given index attr and processes each through the controller.
func (h *TestHarness) reconcileByIndex(ctx context.Context, index entity.Attr, rc *controller.ReconcileController) {
	resp, err := h.EAC.List(ctx, index)
	if err != nil {
		h.T.Errorf("reconcileByIndex: List(%s) failed: %v", index, err)
		return
	}

	for _, e := range resp.Values() {
		ent := e.Entity()
		id := ent.Id()
		if id == "" {
			continue
		}

		event := controller.Event{
			Type:   controller.EventUpdated,
			Id:     id,
			Entity: ent,
		}

		if err := rc.ProcessEventForTest(ctx, event); err != nil {
			h.Log.Debug("reconcile error", "id", id, "error", err)
		}
	}
}

// snapshotRevisions returns a map of entity ID → revision for all disk-related entities.
func (h *TestHarness) snapshotRevisions(ctx context.Context) map[string]int64 {
	snap := make(map[string]int64)

	for _, kind := range []entity.Id{
		storage_v1alpha.KindDisk,
		storage_v1alpha.KindDiskLease,
		storage_v1alpha.KindLsvdVolume,
		storage_v1alpha.KindLsvdMount,
	} {
		resp, err := h.EAC.List(ctx, entity.Ref(entity.EntityKind, kind))
		if err != nil {
			continue
		}
		for _, e := range resp.Values() {
			snap[e.Id()] = e.Revision()
		}
	}

	return snap
}

// reconcileKind is a convenience for reconciling by EntityKind index.
func (h *TestHarness) reconcileKind(ctx context.Context, kind entity.Id, rc *controller.ReconcileController) {
	h.reconcileByIndex(ctx, entity.Ref(entity.EntityKind, kind), rc)
}

// revisionsEqual returns true if two revision snapshots are identical.
func revisionsEqual(a, b map[string]int64) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
