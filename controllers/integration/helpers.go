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

// listDiskVolumes lists all DiskVolume entities in the store.
func listDiskVolumes(t *testing.T, ctx context.Context, h *TestHarness) []*storage.DiskVolume {
	t.Helper()
	resp, err := h.EAC.List(ctx, entity.Ref(entity.EntityKind, storage.KindDiskVolume))
	if err != nil {
		t.Fatalf("listDiskVolumes: %v", err)
	}

	var vols []*storage.DiskVolume
	for _, e := range resp.Values() {
		var vol storage.DiskVolume
		vol.Decode(e.Entity())
		vols = append(vols, &vol)
	}
	return vols
}

// listDiskMounts lists all DiskMount entities in the store.
func listDiskMounts(t *testing.T, ctx context.Context, h *TestHarness) []*storage.DiskMount {
	t.Helper()
	resp, err := h.EAC.List(ctx, entity.Ref(entity.EntityKind, storage.KindDiskMount))
	if err != nil {
		t.Fatalf("listDiskMounts: %v", err)
	}

	var mounts []*storage.DiskMount
	for _, e := range resp.Values() {
		var mount storage.DiskMount
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

// countMountedMounts returns the number of disk_mount entities in DM_MOUNTED state.
func countMountedMounts(t *testing.T, ctx context.Context, h *TestHarness) int {
	t.Helper()
	mounts := listDiskMounts(t, ctx, h)
	count := 0
	for _, m := range mounts {
		if m.ActualState == storage.DM_MOUNTED {
			count++
		}
	}
	return count
}

// getMountForLease returns the disk_mount entity for a given lease, or nil if not found.
func getMountForLease(t *testing.T, ctx context.Context, h *TestHarness, leaseID entity.Id) *storage.DiskMount {
	t.Helper()
	resp, err := h.EAC.List(ctx, entity.Ref(storage.DiskMountDiskLeaseIdId, leaseID))
	if err != nil {
		t.Fatalf("getMountForLease(%s): %v", leaseID, err)
	}
	values := resp.Values()
	if len(values) == 0 {
		return nil
	}
	var mount storage.DiskMount
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

// patchMountActualState patches a disk_mount entity's actual_state.
func patchMountActualState(t *testing.T, ctx context.Context, h *TestHarness, mountID entity.Id, stateId entity.Id) {
	t.Helper()
	_, err := h.EAC.Patch(ctx, []entity.Attr{
		entity.Ref(entity.DBId, mountID),
		entity.Ref(storage.DiskMountActualStateId, stateId),
	}, 0)
	if err != nil {
		t.Fatalf("patchMountActualState(%s): %v", mountID, err)
	}
}

// patchMountError patches a disk_mount entity to DM_ERROR with an error message.
func patchMountError(t *testing.T, ctx context.Context, h *TestHarness, mountID entity.Id, errorMsg string) {
	t.Helper()
	_, err := h.EAC.Patch(ctx, []entity.Attr{
		entity.Ref(entity.DBId, mountID),
		entity.Ref(storage.DiskMountActualStateId, storage.DiskMountActualStateDmErrorId),
		entity.String(storage.DiskMountErrorMessageId, errorMsg),
	}, 0)
	if err != nil {
		t.Fatalf("patchMountError(%s): %v", mountID, err)
	}
}

// getMountByID fetches a DiskMount entity by its entity ID.
func getMountByID(t *testing.T, ctx context.Context, h *TestHarness, id entity.Id) *storage.DiskMount {
	t.Helper()
	resp, err := h.EAC.Get(ctx, id.String())
	if err != nil {
		t.Fatalf("getMountByID(%s): %v", id, err)
	}
	var mount storage.DiskMount
	mount.Decode(resp.Entity().Entity())
	return &mount
}

// deleteMountEntity deletes a disk_mount entity from the store.
func deleteMountEntity(t *testing.T, ctx context.Context, h *TestHarness, mountID entity.Id) {
	t.Helper()
	_, err := h.EAC.Delete(ctx, mountID.String())
	if err != nil {
		t.Fatalf("deleteMountEntity(%s): %v", mountID, err)
	}
}
