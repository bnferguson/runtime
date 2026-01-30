//go:build darwin

package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"

	"miren.dev/runtime/pkg/cloudauth"
)

// realMountOps implements MountOps with stub operations for Darwin.
// This is a noop implementation to allow the package to compile on macOS.
type realMountOps struct {
	log        *slog.Logger
	authClient *cloudauth.AuthClient
	cloudURL   string
}

// NewRealMountOps creates a MountOps that performs real OS operations.
// On Darwin, this returns a noop implementation.
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
	return 0, nil, nil, nil, fmt.Errorf("NBD not supported on Darwin")
}

func (r *realMountOps) NBDStatus(idx uint32) error {
	return fmt.Errorf("NBD not supported on Darwin")
}

func (r *realMountOps) NBDDisconnect(idx uint32) error {
	return fmt.Errorf("NBD not supported on Darwin")
}

func (r *realMountOps) CreateDeviceNode(path string, nbdIndex uint32) error {
	return fmt.Errorf("NBD not supported on Darwin")
}

func (r *realMountOps) Mount(device, mountPath, filesystem string, readOnly bool) error {
	return fmt.Errorf("mount not supported on Darwin")
}

func (r *realMountOps) Unmount(path string) error {
	return fmt.Errorf("unmount not supported on Darwin")
}

func (r *realMountOps) IsMounted(path string) bool {
	return false
}

func (r *realMountOps) IsFormatted(device, filesystem string) (bool, error) {
	return false, fmt.Errorf("IsFormatted not supported on Darwin")
}

func (r *realMountOps) FormatDevice(ctx context.Context, device, filesystem string) error {
	return fmt.Errorf("FormatDevice not supported on Darwin")
}

func (r *realMountOps) OpenLSVDDisk(ctx context.Context, diskPath, volumeId string, remoteOnly bool, leaseNonce string) (LSVDDisk, error) {
	return nil, fmt.Errorf("LSVD not supported on Darwin")
}

func (r *realMountOps) AcquireVolumeLease(ctx context.Context, volumeId string, metadata map[string]any) (string, error) {
	return "", nil
}

func (r *realMountOps) ReleaseVolumeLease(ctx context.Context, volumeId, nonce string) error {
	return nil
}
