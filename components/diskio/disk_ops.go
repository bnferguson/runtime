package diskio

import (
	"context"
	"os"
)

// DiskVolumeOps abstracts OS operations for disk volume management.
// This interface enables testing without requiring actual filesystem operations.
type DiskVolumeOps interface {
	CreateVolumeDir(path string) error
	RemoveVolumeDir(path string) error
	VolumePathExists(path string) bool
	CreateDiskImage(path string, sizeBytes int64) error
	RemoveDiskImage(path string) error
}

// DiskMountOps abstracts OS operations for disk mount management.
// This interface enables testing without requiring actual loop device or mount operations.
type DiskMountOps interface {
	CreateDir(path string, perm os.FileMode) error
	RemoveFile(path string) error
	LoopAttach(imagePath string) (devicePath string, err error)
	LoopDetach(devicePath string) error
	Mount(device, mountPath, filesystem string, readOnly bool) error
	Unmount(path string) error
	IsMounted(path string) bool
	IsFormatted(device, filesystem string) (bool, error)
	FormatDevice(ctx context.Context, device, filesystem string) error
}
