package diskio

import (
	"errors"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"miren.dev/runtime/pkg/entity/testutils"
)

func TestIsDirtyFilesystemErr(t *testing.T) {
	assert.False(t, isDirtyFilesystemErr(nil))
	assert.True(t, isDirtyFilesystemErr(syscall.EUCLEAN))
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
	ops.mountErr = syscall.EUCLEAN
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
	ops.mountErr = syscall.EUCLEAN
	ops.fsckErr = errors.New("fsck exploded")

	err := mountWithFsckRetry(ctx, log, ops, "/dev/loop0", "/mnt/data", "ext4", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fsck")

	// Fsck was attempted exactly once; no second mount attempt.
	assert.Len(t, ops.fsckCalls, 1)
	assert.Empty(t, ops.mounts)
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
