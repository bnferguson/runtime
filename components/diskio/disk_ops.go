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

// ActiveMount describes a mount found on the running system.
type ActiveMount struct {
	Device    string
	MountPath string
}

// DiskMountOps abstracts OS operations for disk mount management.
// This interface enables testing without requiring actual loop device or mount operations.
type DiskMountOps interface {
	CreateDir(path string, perm os.FileMode) error
	RemoveFile(path string) error
	LoopAttach(imagePath string) (devicePath string, err error)
	LoopDetach(devicePath string) error
	LbdAttach(ctx context.Context, imagePath, logDir string) (devicePath string, err error)
	LbdDetach(ctx context.Context, devicePath string) error
	LbdAvailable() bool
	Mount(device, mountPath, filesystem string, readOnly bool) error
	Unmount(path string) error
	IsMounted(path string) bool
	IsFormatted(ctx context.Context, device, filesystem string) (bool, error)
	FormatDevice(ctx context.Context, device, filesystem string) error

	// FindMounts returns all mounts whose mount path starts with the given prefix.
	FindMounts(pathPrefix string) []ActiveMount
}
