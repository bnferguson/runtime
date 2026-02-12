package integration

import (
	"context"
	"testing"

	compute "miren.dev/runtime/api/compute/compute_v1alpha"
	storage "miren.dev/runtime/api/storage/storage_v1alpha"
	"miren.dev/runtime/pkg/entity"
)

// getDisk fetches a Disk entity by ID.
func getDisk(t *testing.T, ctx context.Context, h *TestHarness, id entity.Id) *storage.Disk {
	t.Helper()
	resp, err := h.EAC.Get(ctx, id.String())
	if err != nil {
		t.Fatalf("getDisk(%s): %v", id, err)
	}
	var disk storage.Disk
	disk.Decode(resp.Entity().Entity())
	return &disk
}

// getLease fetches a DiskLease entity by ID.
func getLease(t *testing.T, ctx context.Context, h *TestHarness, id entity.Id) *storage.DiskLease {
	t.Helper()
	resp, err := h.EAC.Get(ctx, id.String())
	if err != nil {
		t.Fatalf("getLease(%s): %v", id, err)
	}
	var lease storage.DiskLease
	lease.Decode(resp.Entity().Entity())
	return &lease
}

// getSandbox fetches a Sandbox entity by ID.
func getSandbox(t *testing.T, ctx context.Context, h *TestHarness, id entity.Id) *compute.Sandbox {
	t.Helper()
	resp, err := h.EAC.Get(ctx, id.String())
	if err != nil {
		t.Fatalf("getSandbox(%s): %v", id, err)
	}
	var sb compute.Sandbox
	sb.Decode(resp.Entity().Entity())
	return &sb
}

// listLeases lists all DiskLease entities in the store.
func listLeases(t *testing.T, ctx context.Context, h *TestHarness) []*storage.DiskLease {
	t.Helper()
	resp, err := h.EAC.List(ctx, entity.Ref(entity.EntityKind, storage.KindDiskLease))
	if err != nil {
		t.Fatalf("listLeases: %v", err)
	}

	var leases []*storage.DiskLease
	for _, e := range resp.Values() {
		var lease storage.DiskLease
		lease.Decode(e.Entity())
		leases = append(leases, &lease)
	}
	return leases
}

// listDisks lists all Disk entities in the store.
func listDisks(t *testing.T, ctx context.Context, h *TestHarness) []*storage.Disk {
	t.Helper()
	resp, err := h.EAC.List(ctx, entity.Ref(entity.EntityKind, storage.KindDisk))
	if err != nil {
		t.Fatalf("listDisks: %v", err)
	}

	var disks []*storage.Disk
	for _, e := range resp.Values() {
		var disk storage.Disk
		disk.Decode(e.Entity())
		disks = append(disks, &disk)
	}
	return disks
}

// listLsvdVolumes lists all LsvdVolume entities in the store.
func listLsvdVolumes(t *testing.T, ctx context.Context, h *TestHarness) []*storage.LsvdVolume {
	t.Helper()
	resp, err := h.EAC.List(ctx, entity.Ref(entity.EntityKind, storage.KindLsvdVolume))
	if err != nil {
		t.Fatalf("listLsvdVolumes: %v", err)
	}

	var vols []*storage.LsvdVolume
	for _, e := range resp.Values() {
		var vol storage.LsvdVolume
		vol.Decode(e.Entity())
		vols = append(vols, &vol)
	}
	return vols
}

// listLsvdMounts lists all LsvdMount entities in the store.
func listLsvdMounts(t *testing.T, ctx context.Context, h *TestHarness) []*storage.LsvdMount {
	t.Helper()
	resp, err := h.EAC.List(ctx, entity.Ref(entity.EntityKind, storage.KindLsvdMount))
	if err != nil {
		t.Fatalf("listLsvdMounts: %v", err)
	}

	var mounts []*storage.LsvdMount
	for _, e := range resp.Values() {
		var mount storage.LsvdMount
		mount.Decode(e.Entity())
		mounts = append(mounts, &mount)
	}
	return mounts
}

// leasesForSandbox returns all leases owned by a specific sandbox.
func leasesForSandbox(t *testing.T, ctx context.Context, h *TestHarness, sandboxID entity.Id) []*storage.DiskLease {
	t.Helper()
	all := listLeases(t, ctx, h)
	var result []*storage.DiskLease
	for _, l := range all {
		if l.SandboxId == sandboxID {
			result = append(result, l)
		}
	}
	return result
}

// createSandboxEntity creates a minimal sandbox entity in the store.
func createSandboxEntity(t *testing.T, ctx context.Context, h *TestHarness, sandboxID entity.Id, status compute.SandboxStatus) {
	t.Helper()
	sb := &compute.Sandbox{
		Status: status,
	}
	_, err := h.EAC.Create(ctx, entity.New(
		entity.DBId, sandboxID,
		sb.Encode,
	).Attrs())
	if err != nil {
		t.Fatalf("createSandboxEntity(%s): %v", sandboxID, err)
	}
}

// markSandboxDead patches a sandbox entity to DEAD status.
func markSandboxDead(t *testing.T, ctx context.Context, h *TestHarness, sandboxID entity.Id) {
	t.Helper()
	_, err := h.EAC.Patch(ctx, entity.New(
		entity.DBId, sandboxID,
		(&compute.Sandbox{
			Status: compute.DEAD,
		}).Encode,
	).Attrs(), 0)
	if err != nil {
		t.Fatalf("markSandboxDead(%s): %v", sandboxID, err)
	}
}

// markSandboxStopped patches a sandbox entity to STOPPED status.
func markSandboxStopped(t *testing.T, ctx context.Context, h *TestHarness, sandboxID entity.Id) {
	t.Helper()
	_, err := h.EAC.Patch(ctx, entity.New(
		entity.DBId, sandboxID,
		(&compute.Sandbox{
			Status: compute.STOPPED,
		}).Encode,
	).Attrs(), 0)
	if err != nil {
		t.Fatalf("markSandboxStopped(%s): %v", sandboxID, err)
	}
}

// markSandboxRunning patches a sandbox entity to RUNNING status.
func markSandboxRunning(t *testing.T, ctx context.Context, h *TestHarness, sandboxID entity.Id) {
	t.Helper()
	_, err := h.EAC.Patch(ctx, entity.New(
		entity.DBId, sandboxID,
		(&compute.Sandbox{
			Status: compute.RUNNING,
		}).Encode,
	).Attrs(), 0)
	if err != nil {
		t.Fatalf("markSandboxRunning(%s): %v", sandboxID, err)
	}
}

// deleteSandboxEntity deletes a sandbox entity from the store.
func deleteSandboxEntity(t *testing.T, ctx context.Context, h *TestHarness, sandboxID entity.Id) {
	t.Helper()
	_, err := h.EAC.Delete(ctx, sandboxID.String())
	if err != nil {
		t.Fatalf("deleteSandboxEntity(%s): %v", sandboxID, err)
	}
}

// deleteLeaseEntity deletes a lease entity from the store.
func deleteLeaseEntity(t *testing.T, ctx context.Context, h *TestHarness, leaseID entity.Id) {
	t.Helper()
	_, err := h.EAC.Delete(ctx, leaseID.String())
	if err != nil {
		t.Fatalf("deleteLeaseEntity(%s): %v", leaseID, err)
	}
}

// patchLeaseStatus patches a lease entity to the given status.
func patchLeaseStatus(t *testing.T, ctx context.Context, h *TestHarness, leaseID entity.Id, status storage.DiskLeaseStatus) {
	t.Helper()
	_, err := h.EAC.Patch(ctx, entity.New(
		entity.DBId, leaseID,
		(&storage.DiskLease{
			Status: status,
		}).Encode,
	).Attrs(), 0)
	if err != nil {
		t.Fatalf("patchLeaseStatus(%s, %s): %v", leaseID, status, err)
	}
}

// countMountedMounts returns the number of lsvd_mount entities in MNT_MOUNTED state.
func countMountedMounts(t *testing.T, ctx context.Context, h *TestHarness) int {
	t.Helper()
	mounts := listLsvdMounts(t, ctx, h)
	count := 0
	for _, m := range mounts {
		if m.ActualState == storage.MNT_MOUNTED {
			count++
		}
	}
	return count
}

// getMountForLease returns the lsvd_mount entity for a given lease, or nil if not found.
func getMountForLease(t *testing.T, ctx context.Context, h *TestHarness, leaseID entity.Id) *storage.LsvdMount {
	t.Helper()
	resp, err := h.EAC.List(ctx, entity.Ref(storage.LsvdMountDiskLeaseIdId, leaseID))
	if err != nil {
		t.Fatalf("getMountForLease(%s): %v", leaseID, err)
	}
	values := resp.Values()
	if len(values) == 0 {
		return nil
	}
	var mount storage.LsvdMount
	mount.Decode(values[0].Entity())
	return &mount
}

// patchDiskStatus patches a disk entity to the given status.
func patchDiskStatus(t *testing.T, ctx context.Context, h *TestHarness, diskID entity.Id, status storage.DiskStatus) {
	t.Helper()
	_, err := h.EAC.Patch(ctx, entity.New(
		entity.DBId, diskID,
		(&storage.Disk{
			Status: status,
		}).Encode,
	).Attrs(), 0)
	if err != nil {
		t.Fatalf("patchDiskStatus(%s, %s): %v", diskID, status, err)
	}
}

// patchMountActualState patches an lsvd_mount entity's actual_state.
func patchMountActualState(t *testing.T, ctx context.Context, h *TestHarness, mountID entity.Id, stateId entity.Id) {
	t.Helper()
	_, err := h.EAC.Patch(ctx, []entity.Attr{
		entity.Ref(entity.DBId, mountID),
		entity.Ref(storage.LsvdMountActualStateId, stateId),
	}, 0)
	if err != nil {
		t.Fatalf("patchMountActualState(%s): %v", mountID, err)
	}
}

// patchMountError patches an lsvd_mount entity to MNT_ERROR with an error message.
func patchMountError(t *testing.T, ctx context.Context, h *TestHarness, mountID entity.Id, errorMsg string) {
	t.Helper()
	_, err := h.EAC.Patch(ctx, []entity.Attr{
		entity.Ref(entity.DBId, mountID),
		entity.Ref(storage.LsvdMountActualStateId, storage.LsvdMountActualStateMntErrorId),
		entity.String(storage.LsvdMountErrorMessageId, errorMsg),
	}, 0)
	if err != nil {
		t.Fatalf("patchMountError(%s): %v", mountID, err)
	}
}

// getMountByID fetches an LsvdMount entity by its entity ID.
func getMountByID(t *testing.T, ctx context.Context, h *TestHarness, id entity.Id) *storage.LsvdMount {
	t.Helper()
	resp, err := h.EAC.Get(ctx, id.String())
	if err != nil {
		t.Fatalf("getMountByID(%s): %v", id, err)
	}
	var mount storage.LsvdMount
	mount.Decode(resp.Entity().Entity())
	return &mount
}

// deleteMountEntity deletes an lsvd_mount entity from the store.
func deleteMountEntity(t *testing.T, ctx context.Context, h *TestHarness, mountID entity.Id) {
	t.Helper()
	_, err := h.EAC.Delete(ctx, mountID.String())
	if err != nil {
		t.Fatalf("deleteMountEntity(%s): %v", mountID, err)
	}
}
