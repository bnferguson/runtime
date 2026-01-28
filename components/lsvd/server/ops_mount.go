//go:build linux

package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"

	"miren.dev/runtime/lsvd"
	"miren.dev/runtime/lsvd/pkg/nbd"
	"miren.dev/runtime/lsvd/pkg/nbdnl"
	"miren.dev/runtime/pkg/cloudauth"
)

// realMountOps implements MountOps with real OS operations
type realMountOps struct {
	log        *slog.Logger
	authClient *cloudauth.AuthClient
	cloudURL   string
}

// NewRealMountOps creates a MountOps that performs real OS operations.
// If authClient is non-nil, disks are opened with a ReplicaWriter that
// replicates writes to the remote cloud.
func NewRealMountOps(log *slog.Logger, authClient *cloudauth.AuthClient, cloudURL string) MountOps {
	return &realMountOps{
		log:        log,
		authClient: authClient,
		cloudURL:   cloudURL,
	}
}

func (r *realMountOps) CreateDir(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (r *realMountOps) RemoveFile(path string) error {
	return os.Remove(path)
}

func (r *realMountOps) NBDLoopback(ctx context.Context, sizeBytes uint64) (uint32, net.Conn, *os.File, func() error, error) {
	return nbdnl.Loopback(ctx, sizeBytes, nbdnl.IndexAny)
}

func (r *realMountOps) NBDStatus(idx uint32) error {
	_, err := nbdnl.Status(idx)
	return err
}

func (r *realMountOps) NBDDisconnect(idx uint32) error {
	return nbdnl.Disconnect(idx)
}

func (r *realMountOps) CreateDeviceNode(path string, nbdIndex uint32) error {
	// Remove stale device if exists
	os.Remove(path)

	// Read max_part from sysfs to compute the correct minor number.
	// The minor for /dev/nbdN is N * (max_part + 1).
	maxPart, err := readNBDMaxPart()
	if err != nil {
		return fmt.Errorf("failed to read NBD max_part: %w", err)
	}
	minor := nbdIndex * (maxPart + 1)

	devNum := int(unix.Mkdev(43, minor))
	r.log.Info("creating device node",
		"path", path,
		"nbd_index", nbdIndex,
		"major", 43,
		"minor", minor,
		"max_part", maxPart,
		"dev_num", devNum,
	)

	if err := unix.Mknod(path, unix.S_IFBLK|0660, devNum); err != nil && !os.IsExist(err) {
		return fmt.Errorf("failed to create device node: %w", err)
	}

	return nil
}

// readNBDMaxPart reads the max_part parameter from the nbd kernel module.
func readNBDMaxPart() (uint32, error) {
	data, err := os.ReadFile("/sys/module/nbd/parameters/max_part")
	if err != nil {
		return 0, err
	}

	val, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 32)
	if err != nil {
		return 0, fmt.Errorf("parsing max_part value %q: %w", string(data), err)
	}

	return uint32(val), nil
}

func (r *realMountOps) Mount(device, mountPath, filesystem string, readOnly bool) error {
	var flags uintptr
	if readOnly {
		flags |= syscall.MS_RDONLY
	}

	r.log.Info("performing mount syscall",
		"device", device,
		"mount_path", mountPath,
		"filesystem", filesystem,
		"read_only", readOnly,
	)

	if err := syscall.Mount(device, mountPath, filesystem, flags, ""); err != nil {
		return err
	}

	// Verify the mount actually took effect
	if r.IsMounted(mountPath) {
		r.log.Info("mount verified in /proc/mounts", "mount_path", mountPath)
	} else {
		r.log.Error("mount syscall succeeded but NOT visible in /proc/mounts",
			"mount_path", mountPath,
			"device", device,
		)
	}

	return nil
}

func (r *realMountOps) Unmount(path string) error {
	if err := syscall.Unmount(path, 0); err != nil {
		// Try lazy unmount
		return syscall.Unmount(path, syscall.MNT_DETACH)
	}
	return nil
}

func (r *realMountOps) IsMounted(path string) bool {
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return false
	}

	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == path {
			return true
		}
	}

	return false
}

// findMountPoint reads /proc/self/mountinfo to find the mount point
// that contains the given path.
func (r *realMountOps) findMountPoint(path string) (string, error) {
	data, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return "", fmt.Errorf("failed to read mountinfo: %w", err)
	}

	// Walk up from path to find the longest matching mount point
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}
	absPath = filepath.Clean(absPath)

	// Parse mount points from mountinfo (field 5 is the mount point)
	var bestMatch string
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		mp := filepath.Clean(fields[4])

		// Check for exact match or proper prefix (followed by path separator)
		// to avoid "/mnt/vol" matching "/mnt/vol2"
		isMatch := absPath == mp || strings.HasPrefix(absPath, mp+string(os.PathSeparator))
		if isMatch && len(mp) > len(bestMatch) {
			bestMatch = mp
		}
	}

	if bestMatch == "" {
		return "", fmt.Errorf("no mount point found for %s", path)
	}

	return bestMatch, nil
}

func (r *realMountOps) IsFormatted(device, filesystem string) (bool, error) {
	cmd := exec.Command("blkid", "-o", "value", "-s", "TYPE", device)
	output, err := cmd.Output()
	if err != nil {
		// Only treat exit status 2 (no filesystem found) as non-error.
		// Other errors (missing binary, permissions, I/O) should be returned
		// to prevent unsafe formatting decisions.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.ExitStatus() == 2 {
				// No filesystem found - this is expected for unformatted devices
				return false, nil
			}
		}
		return false, fmt.Errorf("blkid failed: %w", err)
	}

	fsType := strings.TrimSpace(string(output))
	return fsType == filesystem, nil
}

func (r *realMountOps) FormatDevice(ctx context.Context, device, filesystem string) error {
	var cmd *exec.Cmd

	// Use background context intentionally: killing mkfs mid-format can leave
	// the device in an inconsistent state. The caller's format retry loop
	// (in mount_controller.go) handles timeouts at a higher level.
	ctx = context.Background()

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

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("mkfs %+v failed: %w: %s", cmd.Args, err, string(output))
	}

	return nil
}

func (r *realMountOps) OpenLSVDDisk(ctx context.Context, diskPath, volumeId string, remoteOnly bool) (LSVDDisk, error) {
	var sa lsvd.SegmentAccess
	var sizeBytes int64

	if remoteOnly {
		// Remote-only mode: only use cloud storage
		if r.authClient == nil {
			return nil, fmt.Errorf("remote-only disk requires cloud auth")
		}

		remoteSA := lsvd.NewDiskAPISegmentAccess(r.log, r.cloudURL, r.authClient)
		sa = remoteSA

		volInfo, err := remoteSA.GetVolumeInfo(ctx, volumeId)
		if err != nil {
			return nil, fmt.Errorf("failed to get remote volume info: %w", err)
		}
		sizeBytes = volInfo.Size.Bytes().Int64()

		r.log.Info("OpenLSVDDisk: remote-only volume",
			"volume_id", volumeId,
			"size_bytes", sizeBytes,
			"size_gb", sizeBytes/(1024*1024*1024),
		)
	} else {
		// Local or replica mode
		localSA := &lsvd.LocalFileAccess{
			Dir: diskPath,
			Log: r.log,
		}

		if err := localSA.InitContainer(ctx); err != nil {
			return nil, fmt.Errorf("failed to init container: %w", err)
		}

		volInfo, err := localSA.GetVolumeInfo(ctx, volumeId)
		if err != nil {
			return nil, fmt.Errorf("failed to get volume info: %w", err)
		}

		sizeBytes = volInfo.Size.Bytes().Int64()
		r.log.Info("OpenLSVDDisk: volume info",
			"disk_path", diskPath,
			"volume_id", volumeId,
			"size_bytes", sizeBytes,
			"size_gb", sizeBytes/(1024*1024*1024),
		)

		// Build segment access: use ReplicaWriter when cloud auth is available
		if r.authClient != nil {
			remoteSA := lsvd.NewDiskAPISegmentAccess(r.log, r.cloudURL, r.authClient)
			sa = lsvd.ReplicaWriter(r.log, localSA, remoteSA)
		} else {
			sa = localSA
		}
	}

	disk, err := lsvd.NewDisk(ctx, r.log, diskPath,
		lsvd.WithVolumeName(volumeId),
		lsvd.WithSegmentAccess(sa),
		lsvd.EnableAutoGC,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create disk: %w", err)
	}

	return &realLSVDDisk{
		disk: disk,
		log:  r.log,
		size: sizeBytes,
	}, nil
}

// realLSVDDisk wraps an lsvd.Disk
type realLSVDDisk struct {
	disk *lsvd.Disk
	log  *slog.Logger
	size int64
}

func (d *realLSVDDisk) Close(ctx context.Context) error {
	return d.disk.Close(ctx)
}

func (d *realLSVDDisk) Size() int64 {
	return d.size
}

func (d *realLSVDDisk) HandleNBD(ctx context.Context, conn net.Conn, clientFile *os.File) error {
	nbdOpts := &nbd.Options{
		MinimumBlockSize:   4096,
		PreferredBlockSize: 4096,
		RawFile:            clientFile,
	}

	backend := lsvd.NBDWrapper(ctx, d.log, d.disk)
	return nbd.HandleTransport(d.log, conn, backend, nbdOpts)
}
