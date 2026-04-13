package diskio

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"syscall"
)

// isDirtyFilesystemErr reports whether err indicates that the kernel
// rejected a mount because the filesystem's on-disk state was not
// clean — i.e. "Structure needs cleaning". An unclean shutdown left
// the journal in a state that needs fsck repair before the filesystem
// can be mounted again.
func isDirtyFilesystemErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, syscall.EUCLEAN) {
		return true
	}
	// Defensive string match for environments where the errno has been
	// wrapped in a way that breaks errors.Is unwrapping.
	return strings.Contains(err.Error(), "Structure needs cleaning")
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

	// Safety check: never fsck a device that's mounted somewhere else.
	// In practice this should not happen because our caller just tried
	// to mount it and failed, but better a wasted check than a corrupted
	// filesystem.
	if ops.IsMounted(mountPath) {
		return fmt.Errorf("mount returned dirty filesystem but path is mounted; refusing to fsck: %w", err)
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
