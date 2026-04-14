package diskio

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

// isDirtyFilesystemErr reports whether err indicates that the kernel
// rejected a mount because the filesystem's on-disk state was not
// clean — i.e. "Structure needs cleaning" (Linux EUCLEAN / errno 117).
// An unclean shutdown left the journal in a state that needs fsck
// repair before the filesystem can be mounted again. The match is
// done by string because the named EUCLEAN constant only exists in
// the Linux syscall package, and a case-insensitive substring search
// on the wrapped errno's message is portable and robust to any error
// wrapping that breaks errors.Is unwrapping.
func isDirtyFilesystemErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "structure needs cleaning")
}

// mountWithFsckRetry wraps a mount call so that a dirty-filesystem
// failure triggers an automatic fsck and a single retry. The caller is
// responsible for ensuring the device is not mounted anywhere else
// before this runs — fsck on a live mount would corrupt the filesystem.
//
// On success, nil is returned. On failure, the original mount error is
// returned if fsck cannot repair the device, or a wrapped error if the
// second mount attempt also fails.
func mountWithFsckRetry(
	ctx context.Context,
	log *slog.Logger,
	ops DiskMountOps,
	device, mountPath, filesystem string,
	readOnly bool,
) error {
	err := ops.Mount(device, mountPath, filesystem, readOnly)
	if err == nil {
		return nil
	}
	if !isDirtyFilesystemErr(err) {
		return err
	}

	// Safety check: never fsck a device that's mounted anywhere in the
	// kernel mount table. Running fsck on a live filesystem will
	// corrupt it. In practice this should not happen because our
	// caller just tried to mount this device and failed, but better a
	// wasted check than a corrupted filesystem. We match on the device
	// itself because the same device can be mounted at multiple paths
	// and checking only the target mountPath would miss those.
	//
	// Fail closed: if we cannot read the mount table at all, we have
	// no way to know whether the device is live, so we must refuse.
	mounted, err2 := ops.IsDeviceMounted(device)
	if err2 != nil {
		return fmt.Errorf("cannot determine whether device %s is mounted (refusing to fsck): check=%v mount=%w", device, err2, err)
	}
	if mounted {
		return fmt.Errorf("mount returned dirty filesystem but device %s is mounted elsewhere; refusing to fsck: %w", device, err)
	}

	log.Warn("mount failed with dirty filesystem, running fsck",
		"device", device,
		"mount_path", mountPath,
		"filesystem", filesystem,
		"error", err,
	)

	if fsckErr := ops.Fsck(ctx, device, filesystem); fsckErr != nil {
		return fmt.Errorf("mount failed with dirty filesystem and fsck could not repair: mount=%v fsck=%w", err, fsckErr)
	}

	log.Info("fsck succeeded, retrying mount",
		"device", device,
		"mount_path", mountPath,
	)

	if err := ops.Mount(device, mountPath, filesystem, readOnly); err != nil {
		return fmt.Errorf("mount failed after fsck repair: %w", err)
	}
	return nil
}
