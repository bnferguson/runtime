package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"

	"miren.dev/runtime/lsvd"
	"miren.dev/runtime/pkg/cloudauth"
	"miren.dev/runtime/pkg/units"
)

// VolumeOps abstracts OS and LSVD operations for volume management.
// This interface enables testing without requiring actual filesystem or LSVD operations.
type VolumeOps interface {
	// CreateVolumeDir creates the directory for a volume
	CreateVolumeDir(path string) error

	// RemoveVolumeDir removes a volume directory and all its contents
	RemoveVolumeDir(path string) error

	// VolumePathExists checks if a volume path exists
	VolumePathExists(path string) bool

	// InitLSVDVolume initializes an LSVD volume at the given path.
	// Returns the actual volume ID used (which may differ from the input when
	// cloud auth is configured and the server generates the ID).
	// If remoteOnly is true, the volume is only initialized on the remote cloud
	// (no local storage).
	InitLSVDVolume(ctx context.Context, path, volumeId string, size units.Bytes, metadata map[string]any, remoteOnly bool) (string, error)
}

// MountOps abstracts OS operations for mount management.
// This interface enables testing without requiring actual NBD, device, or mount operations.
type MountOps interface {
	// CreateDir creates a directory with the specified permissions
	CreateDir(path string, perm os.FileMode) error

	// RemoveFile removes a file
	RemoveFile(path string) error

	// NBDLoopback sets up an NBD loopback device
	// Returns: index, conn, clientFile, cleanup function, error
	NBDLoopback(ctx context.Context, sizeBytes uint64) (uint32, net.Conn, *os.File, func() error, error)

	// NBDStatus checks the status of an NBD device
	NBDStatus(idx uint32) error

	// NBDDisconnect disconnects an NBD device
	NBDDisconnect(idx uint32) error

	// CreateDeviceNode creates a block device node
	CreateDeviceNode(path string, nbdIndex uint32) error

	// Mount mounts a device at the specified path
	Mount(device, mountPath, filesystem string, readOnly bool) error

	// Unmount unmounts a path
	Unmount(path string) error

	// IsMounted checks if a path is a mount point
	IsMounted(path string) bool

	// IsFormatted checks if a device has a filesystem
	IsFormatted(device, filesystem string) (bool, error)

	// FormatDevice formats a device with the specified filesystem
	FormatDevice(ctx context.Context, device, filesystem string) error

	// OpenLSVDDisk opens an LSVD disk for the given volume.
	// If remoteOnly is true, the disk uses only remote cloud storage (no local).
	// The leaseNonce is passed to the segment access for write operations.
	OpenLSVDDisk(ctx context.Context, diskPath, volumeId string, remoteOnly bool, leaseNonce string) (LSVDDisk, error)

	// AcquireVolumeLease acquires a lease from the cloud for exclusive access.
	// Returns the lease nonce, or empty string if cloud auth is not configured.
	AcquireVolumeLease(ctx context.Context, volumeId string, metadata map[string]any) (string, error)

	// ReleaseVolumeLease releases a lease from the cloud.
	// Does nothing if cloud auth is not configured or nonce is empty.
	ReleaseVolumeLease(ctx context.Context, volumeId, nonce string) error
}

// LSVDDisk abstracts LSVD disk operations for NBD handling
type LSVDDisk interface {
	// Close closes the disk
	Close(ctx context.Context) error

	// Size returns the disk size in bytes
	Size() int64

	// HandleNBD handles NBD protocol on the given connection
	HandleNBD(ctx context.Context, conn net.Conn, clientFile *os.File) error
}

// realVolumeOps implements VolumeOps with real OS/LSVD operations
type realVolumeOps struct {
	log        *slog.Logger
	authClient *cloudauth.AuthClient
	cloudURL   string
}

// NewRealVolumeOps creates a VolumeOps that performs real OS operations.
// If authClient is non-nil, volumes are also initialized on the remote cloud.
func NewRealVolumeOps(log *slog.Logger, authClient *cloudauth.AuthClient, cloudURL string) VolumeOps {
	return &realVolumeOps{
		log:        log,
		authClient: authClient,
		cloudURL:   cloudURL,
	}
}

func (r *realVolumeOps) CreateVolumeDir(path string) error {
	return os.MkdirAll(path, 0755)
}

func (r *realVolumeOps) RemoveVolumeDir(path string) error {
	return os.RemoveAll(path)
}

func (r *realVolumeOps) VolumePathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (r *realVolumeOps) InitLSVDVolume(ctx context.Context, path, volumeId string, size units.Bytes, metadata map[string]any, remoteOnly bool) (string, error) {
	// Use human-readable name from metadata if provided, otherwise fall back to volumeId
	displayName := volumeId
	if name, ok := metadata["name"].(string); ok && name != "" {
		displayName = name
	}

	volInfo := &lsvd.VolumeInfo{
		Name:     displayName,
		Size:     size,
		UUID:     volumeId,
		Metadata: metadata,
	}

	// Remote-only mode: only init on the cloud, no local storage
	if remoteOnly {
		if r.authClient == nil {
			return "", fmt.Errorf("remote-only volume requires cloud auth")
		}

		remoteSA := lsvd.NewDiskAPISegmentAccess(r.log, r.cloudURL, r.authClient)
		serverVolumeId, err := remoteSA.InitVolumeWithID(ctx, volInfo)
		if err != nil {
			return "", fmt.Errorf("failed to init remote volume: %w", err)
		}
		return serverVolumeId, nil
	}

	// Local or replica mode
	localSA := &lsvd.LocalFileAccess{Dir: path, Log: r.log}

	if err := localSA.InitContainer(ctx); err != nil {
		return "", err
	}

	if r.authClient != nil {
		remoteSA := lsvd.NewDiskAPISegmentAccess(r.log, r.cloudURL, r.authClient)

		// Init on remote first to get server-generated volume ID
		serverVolumeId, err := remoteSA.InitVolumeWithID(ctx, volInfo)
		if err != nil {
			return "", fmt.Errorf("failed to init remote volume: %w", err)
		}

		// Use the server-generated ID for local init
		volInfo.Name = serverVolumeId
		volInfo.UUID = serverVolumeId
		if err := localSA.InitVolume(ctx, volInfo); err != nil {
			return "", fmt.Errorf("failed to init local volume: %w", err)
		}

		return serverVolumeId, nil
	}

	// Local-only mode: use volumeId for the directory name to ensure consistency
	// with what we return. The human-readable name is preserved in Metadata.
	volInfo.Name = volumeId
	if err := localSA.InitVolume(ctx, volInfo); err != nil {
		return "", err
	}
	return volumeId, nil
}
