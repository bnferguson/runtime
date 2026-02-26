package diskio

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	loopCtlGetFree = 0x4C82
	loopSetFd      = 0x4C00
	loopClrFd      = 0x4C01
)

// realDiskVolumeOps implements DiskVolumeOps with real OS operations
type realDiskVolumeOps struct {
	log *slog.Logger
}

func NewRealDiskVolumeOps(log *slog.Logger) DiskVolumeOps {
	return &realDiskVolumeOps{log: log}
}

func (r *realDiskVolumeOps) CreateVolumeDir(path string) error {
	return os.MkdirAll(path, 0755)
}

func (r *realDiskVolumeOps) RemoveVolumeDir(path string) error {
	return os.RemoveAll(path)
}

func (r *realDiskVolumeOps) VolumePathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (r *realDiskVolumeOps) CreateDiskImage(path string, sizeBytes int64) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create disk image: %w", err)
	}
	defer f.Close()

	if err := f.Truncate(sizeBytes); err != nil {
		return fmt.Errorf("failed to truncate disk image to %d bytes: %w", sizeBytes, err)
	}

	return nil
}

func (r *realDiskVolumeOps) RemoveDiskImage(path string) error {
	return os.Remove(path)
}

// realDiskMountOps implements DiskMountOps with real loop device operations
type realDiskMountOps struct {
	log *slog.Logger
}

func NewRealDiskMountOps(log *slog.Logger) DiskMountOps {
	return &realDiskMountOps{log: log}
}

func (r *realDiskMountOps) CreateDir(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (r *realDiskMountOps) RemoveFile(path string) error {
	return os.Remove(path)
}

func (r *realDiskMountOps) LoopAttach(imagePath string) (string, error) {
	// Open loop-control to get a free device
	ctl, err := os.OpenFile("/dev/loop-control", os.O_RDWR, 0)
	if err != nil {
		return "", fmt.Errorf("failed to open /dev/loop-control: %w", err)
	}
	defer ctl.Close()

	idx, _, errno := syscall.Syscall(syscall.SYS_IOCTL, ctl.Fd(), loopCtlGetFree, 0)
	if errno != 0 {
		return "", fmt.Errorf("LOOP_CTL_GET_FREE failed: %w", errno)
	}

	devicePath := fmt.Sprintf("/dev/loop%d", idx)

	// Ensure the device node exists (may be missing in containers)
	if _, err := os.Stat(devicePath); err != nil {
		dev := unix.Mkdev(loopMajor, uint32(idx))
		if err := unix.Mknod(devicePath, unix.S_IFBLK|0660, int(dev)); err != nil {
			return "", fmt.Errorf("failed to create device node %s: %w", devicePath, err)
		}
	}

	// Open the loop device
	loopDev, err := os.OpenFile(devicePath, os.O_RDWR, 0)
	if err != nil {
		return "", fmt.Errorf("failed to open loop device %s: %w", devicePath, err)
	}
	defer loopDev.Close()

	// Open the backing file
	backingFile, err := os.OpenFile(imagePath, os.O_RDWR, 0)
	if err != nil {
		return "", fmt.Errorf("failed to open backing file %s: %w", imagePath, err)
	}
	defer backingFile.Close()

	// Attach the backing file to the loop device
	_, _, errno = syscall.Syscall(syscall.SYS_IOCTL, loopDev.Fd(), loopSetFd, backingFile.Fd())
	if errno != 0 {
		return "", fmt.Errorf("LOOP_SET_FD failed for %s: %w", devicePath, errno)
	}

	return devicePath, nil
}

func (r *realDiskMountOps) LoopDetach(devicePath string) error {
	loopDev, err := os.OpenFile(devicePath, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("failed to open loop device %s: %w", devicePath, err)
	}
	defer loopDev.Close()

	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, loopDev.Fd(), loopClrFd, 0)
	if errno != 0 {
		return fmt.Errorf("LOOP_CLR_FD failed for %s: %w", devicePath, errno)
	}

	return nil
}

func (r *realDiskMountOps) Mount(device, mountPath, filesystem string, readOnly bool) error {
	flags := uintptr(0)
	if readOnly {
		flags |= syscall.MS_RDONLY
	}

	err := syscall.Mount(device, mountPath, filesystem, flags, "")
	if err != nil {
		return fmt.Errorf("mount %s on %s failed: %w", device, mountPath, err)
	}
	return nil
}

func (r *realDiskMountOps) Unmount(path string) error {
	err := syscall.Unmount(path, 0)
	if err != nil {
		return fmt.Errorf("unmount %s failed: %w", path, err)
	}
	return nil
}

func (r *realDiskMountOps) IsMounted(path string) bool {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && fields[1] == path {
			return true
		}
	}
	return false
}

func (r *realDiskMountOps) IsFormatted(device, filesystem string) (bool, error) {
	cmd := exec.Command("blkid", "-o", "value", "-s", "TYPE", device)
	output, err := cmd.Output()
	if err != nil {
		// blkid returns exit code 2 if no filesystem is found
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 2 {
			return false, nil
		}
		return false, fmt.Errorf("blkid probe failed: %w", err)
	}

	detectedFS := strings.TrimSpace(string(output))
	return detectedFS == filesystem, nil
}

func (r *realDiskMountOps) FormatDevice(ctx context.Context, device, filesystem string) error {
	var cmd *exec.Cmd
	switch filesystem {
	case "ext4":
		cmd = exec.CommandContext(ctx, "mkfs.ext4", "-F", device)
	case "xfs":
		cmd = exec.CommandContext(ctx, "mkfs.xfs", "-f", device)
	case "btrfs":
		cmd = exec.CommandContext(ctx, "mkfs.btrfs", "-f", device)
	default:
		return fmt.Errorf("unsupported filesystem: %s", filesystem)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mkfs.%s failed: %w\noutput: %s", filesystem, err, string(output))
	}

	return nil
}

const (
	// Device numbers for loop devices
	loopControlMajor = 10  // misc device
	loopControlMinor = 237 // loop-control minor
	loopMajor        = 7   // loop block devices
)

// EnsureLoopDevices ensures /dev/loop-control and loop device nodes are available.
// In containers, /dev may be a fresh mount that doesn't include loop devices even
// when the kernel supports them. This function creates the device nodes via mknod
// if they're missing.
func EnsureLoopDevices(log *slog.Logger) error {
	if err := ensureLoopControl(log); err != nil {
		return err
	}

	// Verify the kernel actually supports loop devices by issuing an ioctl.
	idx, err := probeLoopControl()
	if err != nil {
		return fmt.Errorf("loop device support not available: %w", err)
	}

	// Ensure the device node for the free loop device exists.
	if err := ensureLoopDeviceNode(log, idx); err != nil {
		return err
	}

	log.Info("Loop devices available", "free_device_index", idx)
	return nil
}

// ensureLoopControl makes sure /dev/loop-control exists.
func ensureLoopControl(log *slog.Logger) error {
	if _, err := os.Stat("/dev/loop-control"); err == nil {
		return nil
	}

	// Try modprobe first — it may create the device node automatically.
	log.Info("Loading loop kernel module")
	if out, err := exec.Command("modprobe", "loop").CombinedOutput(); err != nil {
		log.Warn("modprobe loop failed, will try mknod", "error", err, "output", string(out))
	}

	if _, err := os.Stat("/dev/loop-control"); err == nil {
		return nil
	}

	// Create the device node directly.
	log.Info("Creating /dev/loop-control via mknod")
	dev := unix.Mkdev(loopControlMajor, loopControlMinor)
	if err := unix.Mknod("/dev/loop-control", unix.S_IFCHR|0660, int(dev)); err != nil {
		return fmt.Errorf("mknod /dev/loop-control: %w", err)
	}

	return nil
}

// probeLoopControl opens /dev/loop-control and issues LOOP_CTL_GET_FREE to
// verify the kernel supports loop devices. Returns the index of a free device.
func probeLoopControl() (int, error) {
	ctl, err := os.OpenFile("/dev/loop-control", os.O_RDWR, 0)
	if err != nil {
		return 0, fmt.Errorf("failed to open /dev/loop-control: %w", err)
	}
	defer ctl.Close()

	idx, _, errno := syscall.Syscall(syscall.SYS_IOCTL, ctl.Fd(), loopCtlGetFree, 0)
	if errno != 0 {
		return 0, fmt.Errorf("LOOP_CTL_GET_FREE ioctl failed: %w", errno)
	}

	return int(idx), nil
}

// ensureLoopDeviceNode ensures /dev/loopN exists, creating it via mknod if needed.
func ensureLoopDeviceNode(log *slog.Logger, index int) error {
	path := fmt.Sprintf("/dev/loop%d", index)
	if _, err := os.Stat(path); err == nil {
		return nil
	}

	log.Info("Creating loop device node via mknod", "path", path)
	dev := unix.Mkdev(loopMajor, uint32(index))
	if err := unix.Mknod(path, unix.S_IFBLK|0660, int(dev)); err != nil {
		return fmt.Errorf("mknod %s: %w", path, err)
	}

	return nil
}

// LoopDeviceAvailable checks if loop devices can be used.
func LoopDeviceAvailable() bool {
	_, err := os.Stat("/dev/loop-control")
	return err == nil
}

// ensure unsafe is used (needed for ioctl alignment)
var _ = unsafe.Sizeof(0)
