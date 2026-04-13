package disk

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	compute "miren.dev/runtime/api/compute/compute_v1alpha"
	"miren.dev/runtime/api/storage/storage_v1alpha"
	"miren.dev/runtime/pkg/entity"
	"miren.dev/runtime/pkg/entity/testutils"
)

// TestReconcileOrphanLeasesReleasesDeadSandboxLease verifies that a BOUND
// lease whose owning sandbox has status DEAD gets transitioned to RELEASED
// at boot. This catches leases stranded after a SIGKILL that bypassed the
// normal StopSandbox → ReleaseDiskLeases path.
func TestReconcileOrphanLeasesReleasesDeadSandboxLease(t *testing.T) {
	ctx := context.Background()
	log := slog.Default()

	es, cleanup := testutils.NewInMemEntityServer(t)
	t.Cleanup(cleanup)

	// Create a dead sandbox.
	deadSandboxId := entity.Id("sandbox/dead-postgres")
	_, err := es.EAC.Create(ctx, entity.New(
		entity.DBId, deadSandboxId,
		(&compute.Sandbox{
			ID:     deadSandboxId,
			Status: compute.DEAD,
		}).Encode,
	).Attrs())
	require.NoError(t, err)

	// Create a BOUND lease owned by it.
	leaseId := entity.Id("disk-lease/orphan-bound")
	_, err = es.EAC.Create(ctx, entity.New(
		entity.DBId, leaseId,
		(&storage_v1alpha.DiskLease{
			ID:         leaseId,
			DiskId:     entity.Id("disk/pg-shared"),
			SandboxId:  deadSandboxId,
			Status:     storage_v1alpha.BOUND,
			AcquiredAt: time.Now(),
			NodeId:     entity.Id("node/test-node"),
		}).Encode,
	).Attrs())
	require.NoError(t, err)

	// A second lease owned by a LIVE sandbox must NOT be released.
	liveSandboxId := entity.Id("sandbox/live-web")
	_, err = es.EAC.Create(ctx, entity.New(
		entity.DBId, liveSandboxId,
		(&compute.Sandbox{
			ID:     liveSandboxId,
			Status: compute.RUNNING,
		}).Encode,
	).Attrs())
	require.NoError(t, err)

	liveLeaseId := entity.Id("disk-lease/live-bound")
	_, err = es.EAC.Create(ctx, entity.New(
		entity.DBId, liveLeaseId,
		(&storage_v1alpha.DiskLease{
			ID:         liveLeaseId,
			DiskId:     entity.Id("disk/web-data"),
			SandboxId:  liveSandboxId,
			Status:     storage_v1alpha.BOUND,
			AcquiredAt: time.Now(),
			NodeId:     entity.Id("node/test-node"),
		}).Encode,
	).Attrs())
	require.NoError(t, err)

	leaseController := NewDiskLeaseController(log, es.EAC, "test-node", "")
	require.NoError(t, leaseController.Init(ctx))

	// Orphan lease should now be RELEASED.
	resp, err := es.EAC.Get(ctx, leaseId.String())
	require.NoError(t, err)
	var orphan storage_v1alpha.DiskLease
	orphan.Decode(resp.Entity().Entity())
	assert.Equal(t, storage_v1alpha.RELEASED, orphan.Status,
		"lease for a DEAD sandbox must be released by the orphan sweep")

	// Live lease should be untouched.
	resp, err = es.EAC.Get(ctx, liveLeaseId.String())
	require.NoError(t, err)
	var live storage_v1alpha.DiskLease
	live.Decode(resp.Entity().Entity())
	assert.Equal(t, storage_v1alpha.BOUND, live.Status,
		"lease for a LIVE sandbox must not be touched")
}

// TestReconcileOrphanLeasesReleasesLeaseForMissingSandbox verifies that
// if the sandbox entity has been deleted entirely (not just marked
// DEAD), the dangling lease gets cleaned up.
func TestReconcileOrphanLeasesReleasesLeaseForMissingSandbox(t *testing.T) {
	ctx := context.Background()
	log := slog.Default()

	es, cleanup := testutils.NewInMemEntityServer(t)
	t.Cleanup(cleanup)

	leaseId := entity.Id("disk-lease/orphan-missing")
	_, err := es.EAC.Create(ctx, entity.New(
		entity.DBId, leaseId,
		(&storage_v1alpha.DiskLease{
			ID:         leaseId,
			DiskId:     entity.Id("disk/some-disk"),
			SandboxId:  entity.Id("sandbox/does-not-exist"),
			Status:     storage_v1alpha.BOUND,
			AcquiredAt: time.Now(),
			NodeId:     entity.Id("node/test-node"),
		}).Encode,
	).Attrs())
	require.NoError(t, err)

	leaseController := NewDiskLeaseController(log, es.EAC, "test-node", "")
	require.NoError(t, leaseController.Init(ctx))

	resp, err := es.EAC.Get(ctx, leaseId.String())
	require.NoError(t, err)
	var lease storage_v1alpha.DiskLease
	lease.Decode(resp.Entity().Entity())
	assert.Equal(t, storage_v1alpha.RELEASED, lease.Status,
		"lease for a missing sandbox must be released by the orphan sweep")
}
