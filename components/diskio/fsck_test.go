package diskio

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"miren.dev/runtime/pkg/entity/testutils"
)

func TestIsDirtyFilesystemErr(t *testing.T) {
	assert.False(t, isDirtyFilesystemErr(nil))
	assert.True(t, isDirtyFilesystemErr(errors.New("mount /dev/loop0 failed: structure needs cleaning")))
	assert.True(t, isDirtyFilesystemErr(errors.New("mount /dev/loop0 failed: Structure needs cleaning")))
	assert.False(t, isDirtyFilesystemErr(errors.New("permission denied")))
}

// TestMountWithFsckRetryRecoversFromDirtyFs verifies that a dirty
// filesystem error from Mount triggers fsck and a successful retry —
// the "Structure needs cleaning" self-healing path.
func TestMountWithFsckRetryRecoversFromDirtyFs(t *testing.T) {
	log := testutils.TestLogger(t)
	ctx := t.Context()

	ops := newMockDiskMountOps()

	// First Mount call returns EUCLEAN, second succeeds. The mock's
	// Mount function doesn't have per-call behavior, so use the fsckFn
	// hook to clear mountErr after fsck runs, allowing the retry to
	// proceed.
	ops.mountErr = errors.New("mount /dev/loop0 failed: structure needs cleaning")
	ops.fsckFn = func(_, _ string) error {
		ops.mountErr = nil
		return nil
	}

	err := mountWithFsckRetry(ctx, log, ops, "/dev/loop0", "/mnt/data", "ext4", false)
	require.NoError(t, err)

	// Fsck should have been called exactly once.
	require.Len(t, ops.fsckCalls, 1)
	assert.Equal(t, "/dev/loop0", ops.fsckCalls[0].device)
	assert.Equal(t, "ext4", ops.fsckCalls[0].filesystem)

	// Mount should have been recorded (the successful retry).
	require.Len(t, ops.mounts, 1)
	assert.Equal(t, "/mnt/data", ops.mounts[0].mountPath)
}

// TestMountWithFsckRetryPropagatesFsckFailure verifies that if fsck
// itself fails, the wrapped error surfaces so callers can record it.
func TestMountWithFsckRetryPropagatesFsckFailure(t *testing.T) {
	log := testutils.TestLogger(t)
	ctx := t.Context()

	ops := newMockDiskMountOps()
	ops.mountErr = errors.New("mount /dev/loop0 failed: structure needs cleaning")
	ops.fsckErr = errors.New("fsck exploded")

	err := mountWithFsckRetry(ctx, log, ops, "/dev/loop0", "/mnt/data", "ext4", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fsck")

	// Fsck was attempted exactly once; no second mount attempt.
	assert.Len(t, ops.fsckCalls, 1)
	assert.Empty(t, ops.mounts)
}

// TestMountWithFsckRetryRefusesWhenDeviceStillMounted verifies that the
// safety check rejects fsck if the device is mounted anywhere in the
// kernel mount table — even at a path other than the one we're trying
// to mount at. fsck on a live filesystem corrupts it.
func TestMountWithFsckRetryRefusesWhenDeviceStillMounted(t *testing.T) {
	log := testutils.TestLogger(t)
	ctx := t.Context()

	ops := newMockDiskMountOps()
	ops.mountErr = errors.New("mount /dev/loop0 failed: structure needs cleaning")

	// Simulate the device being mounted at some OTHER path. The mock
	// tracks device↔path in mountDevices/mountedPaths; populate both so
	// IsDeviceMounted returns true even though IsMounted(targetPath)
	// returns false.
	ops.mountedPaths["/mnt/other"] = true
	ops.mountDevices["/mnt/other"] = "/dev/loop0"

	err := mountWithFsckRetry(ctx, log, ops, "/dev/loop0", "/mnt/data", "ext4", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "/dev/loop0")
	assert.Contains(t, err.Error(), "mounted elsewhere")

	// Fsck must not have been called.
	assert.Empty(t, ops.fsckCalls)
}

// TestMountWithFsckRetryRefusesWhenMountTableUnreadable verifies that
// if IsDeviceMounted returns an error (e.g. /proc/mounts can't be
// read), the fsck safety check fails closed instead of assuming the
// device is not mounted.
func TestMountWithFsckRetryRefusesWhenMountTableUnreadable(t *testing.T) {
	log := testutils.TestLogger(t)
	ctx := t.Context()

	ops := newMockDiskMountOps()
	ops.mountErr = errors.New("mount /dev/loop0 failed: structure needs cleaning")
	ops.isDeviceMountedErr = errors.New("read /proc/mounts: permission denied")

	err := mountWithFsckRetry(ctx, log, ops, "/dev/loop0", "/mnt/data", "ext4", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot determine whether device")
	assert.Contains(t, err.Error(), "/dev/loop0")

	// Fsck must not have been called.
	assert.Empty(t, ops.fsckCalls)
}

// TestMountWithFsckRetrySkipsNonDirtyErrors verifies that non-EUCLEAN
// mount errors are returned immediately without running fsck.
func TestMountWithFsckRetrySkipsNonDirtyErrors(t *testing.T) {
	log := testutils.TestLogger(t)
	ctx := t.Context()

	ops := newMockDiskMountOps()
	ops.mountErr = errors.New("mount: permission denied")

	err := mountWithFsckRetry(ctx, log, ops, "/dev/loop0", "/mnt/data", "ext4", false)
	require.Error(t, err)

	// Fsck must not have been called for a non-dirty error.
	assert.Empty(t, ops.fsckCalls)
}
