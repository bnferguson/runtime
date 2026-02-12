package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"miren.dev/runtime/api/entityserver/entityserver_v1alpha"
	"miren.dev/runtime/api/storage/storage_v1alpha"
	"miren.dev/runtime/pkg/cond"
	"miren.dev/runtime/pkg/entity"
)

// Init implements controller.ReconcileControllerI.
func (c *MountController) Init(ctx context.Context) error {
	return nil
}

// Reconcile implements controller.ReconcileControllerI.
func (c *MountController) Reconcile(ctx context.Context, mount *storage_v1alpha.LsvdMount, meta *entity.Meta) error {
	return c.reconcileMount(ctx, mount)
}

// Index returns the entity index attribute for watching mount entities on this node.
func (c *MountController) Index() entity.Attr {
	fullNodeId := "node/" + c.nodeId
	return entity.Ref(storage_v1alpha.LsvdMountNodeIdId, entity.Id(fullNodeId))
}

// Shutdown unmounts filesystems, disconnects NBD devices, and cleans up handlers.
func (c *MountController) Shutdown() {
	// Snapshot and clear all handlers under lock
	c.handlersMu.Lock()
	handlers := make(map[string]*nbdHandler, len(c.handlers))
	for entityId, h := range c.handlers {
		handlers[entityId] = h
	}
	c.handlers = make(map[string]*nbdHandler)
	c.handlersMu.Unlock()

	// Use a fresh context for cleanup since the parent context is already cancelled
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for entityId, h := range handlers {
		c.log.Info("shutting down handler", "entity_id", entityId)

		mountState := c.state.GetMount(entityId)

		// Unmount filesystem first
		if mountState != nil && mountState.Mounted && mountState.MountPath != "" {
			c.log.Info("unmounting on shutdown", "entity_id", entityId, "mount_path", mountState.MountPath)
			if err := c.ops.Unmount(mountState.MountPath); err != nil {
				c.log.Warn("failed to unmount on shutdown", "entity_id", entityId, "error", err)
			} else {
				mountState.Mounted = false
				c.state.SetMount(entityId, mountState)
			}
		}

		// Cancel handler context to signal NBD goroutine to stop
		if h.cancel != nil {
			h.cancel()
		}

		// Disconnect NBD device BEFORE waiting for handler - this unblocks HandleNBD
		if mountState != nil && mountState.DevicePath != "" {
			c.log.Info("disconnecting NBD on shutdown", "entity_id", entityId, "nbd_index", mountState.NbdIndex)
			if err := c.ops.NBDDisconnect(mountState.NbdIndex); err != nil {
				c.log.Warn("failed to disconnect NBD on shutdown", "entity_id", entityId, "error", err)
			}
		}

		// Wait for handler goroutine to exit (now unblocked by NBD disconnect)
		if h.done != nil {
			c.log.Info("waiting for NBD handler to exit", "entity_id", entityId)
			<-h.done
		}

		// Close disk resources
		if h.disk != nil {
			h.disk.Close(ctx)
		}

		// Release lease after closing disk
		if mountState != nil && mountState.LeaseNonce != "" {
			volState := c.state.GetVolume(mountState.VolumeId)
			if volState != nil {
				if err := c.ops.ReleaseVolumeLease(ctx, volState.VolumeId, mountState.LeaseNonce); err != nil {
					c.log.Warn("failed to release lease on shutdown", "entity_id", entityId, "error", err)
				} else {
					c.log.Info("released volume lease on shutdown", "entity_id", entityId, "volume_id", volState.VolumeId)
				}
			}
		}

		// Remove device node
		if mountState != nil && mountState.DevicePath != "" {
			c.ops.RemoveFile(mountState.DevicePath)
		}
	}

	if len(handlers) > 0 {
		if err := c.state.Save(); err != nil {
			c.log.Warn("failed to save state after shutdown cleanup", "error", err)
		}
	}
}

// MountController watches lsvd_mount entities and manages NBD devices and mounts
type MountController struct {
	log      *slog.Logger
	dataPath string
	nodeId   string
	eac      *entityserver_v1alpha.EntityAccessClient
	state    *State
	ops      MountOps

	// handlersMu protects concurrent access to the handlers map
	handlersMu sync.RWMutex
	// Active NBD handlers, keyed by entity ID
	handlers map[string]*nbdHandler
}

type nbdHandler struct {
	conn       net.Conn
	clientFile *os.File
	cancel     context.CancelFunc
	disk       LSVDDisk
	done       chan struct{} // closed when handler goroutine exits
}

// NewMountController creates a new mount controller.
// The controller is created without an EntityAccessClient so that local system
// reconciliation (ReconcileWithSystem) can run immediately at startup, even if
// the entity server is unavailable. Call SetEAC after establishing a connection
// to the entity server to enable entity-based reconciliation.
func NewMountController(log *slog.Logger, dataPath, nodeId string, state *State, ops MountOps) *MountController {
	if ops == nil {
		ops = NewRealMountOps(log, nil, "")
	}
	return &MountController{
		log:      log.With("module", "lsvd-mount"),
		dataPath: dataPath,
		nodeId:   nodeId,
		state:    state,
		ops:      ops,
		handlers: make(map[string]*nbdHandler),
	}
}

// SetEAC sets the EntityAccessClient for entity server communication.
// This is separate from construction because the controller must be usable
// for local system reconciliation before the entity server connection is
// established — ensuring disks remain available even during entity server outages.
func (c *MountController) SetEAC(eac *entityserver_v1alpha.EntityAccessClient) {
	c.eac = eac
}

// reconcileMount reconciles a single lsvd_mount entity
func (c *MountController) reconcileMount(ctx context.Context, mount *storage_v1alpha.LsvdMount) error {
	entityId := string(mount.ID)
	c.log.Info("reconciling mount",
		"entity_id", entityId,
		"desired_state", mount.DesiredState,
		"actual_state", mount.ActualState,
	)

	switch mount.DesiredState {
	case storage_v1alpha.MNT_WANT_MOUNTED:
		return c.reconcileMountMounted(ctx, mount)
	case storage_v1alpha.MNT_WANT_UNMOUNTED:
		return c.reconcileMountUnmounted(ctx, mount)
	default:
		c.log.Warn("unknown desired state", "desired_state", mount.DesiredState)
		return nil
	}
}

// reconcileMountMounted ensures the volume is mounted
func (c *MountController) reconcileMountMounted(ctx context.Context, mount *storage_v1alpha.LsvdMount) error {
	entityId := string(mount.ID)

	switch mount.ActualState {
	case storage_v1alpha.MNT_PENDING:
		return c.attachAndMount(ctx, mount)
	case storage_v1alpha.MNT_ATTACHING:
		// Already attaching, wait
		return nil
	case storage_v1alpha.MNT_ATTACHED:
		// Attached but not mounted, mount it
		return c.mountVolume(ctx, mount)
	case storage_v1alpha.MNT_MOUNTING:
		// Already mounting, wait
		return nil
	case storage_v1alpha.MNT_MOUNTED:
		// Verify the mount is actually present on the system
		mountState := c.state.GetMount(entityId)
		if mountState == nil {
			c.log.Warn("entity says MNT_MOUNTED but no local state found", "entity_id", entityId)
			return nil
		}

		// Check NBD is still connected
		if mountState.DevicePath != "" {
			if err := c.ops.NBDStatus(mountState.NbdIndex); err != nil {
				c.log.Warn("entity reports mounted but NBD disconnected, recovering",
					"entity_id", entityId,
					"nbd_index", mountState.NbdIndex,
					"error", err,
				)
				c.cleanupHandler(ctx, entityId)
				return c.attachAndMount(ctx, mount)
			}
		}

		// Check mount is still present
		if mountState.MountPath != "" && !c.ops.IsMounted(mountState.MountPath) {
			c.log.Warn("entity reports mounted but mount not found on system, recovering",
				"entity_id", entityId,
				"mount_path", mountState.MountPath,
			)
			c.cleanupHandler(ctx, entityId)
			return c.attachAndMount(ctx, mount)
		}
		return nil
	case storage_v1alpha.MNT_ERROR:
		// Error state, try to recover
		c.log.Info("mount in error state, attempting recovery", "entity_id", entityId)
		c.cleanupHandler(ctx, entityId)
		return c.attachAndMount(ctx, mount)
	case storage_v1alpha.MNT_DETACHED:
		// Detached but desired mounted — recover by re-attaching from scratch
		c.log.Info("mount detached but desired mounted, recovering", "entity_id", entityId)
		c.cleanupHandler(ctx, entityId)
		return c.attachAndMount(ctx, mount)
	default:
		c.log.Warn("unexpected actual state for mounted", "actual_state", mount.ActualState)
		return nil
	}
}

// reconcileMountUnmounted ensures the volume is unmounted
func (c *MountController) reconcileMountUnmounted(ctx context.Context, mount *storage_v1alpha.LsvdMount) error {
	entityId := string(mount.ID)

	switch mount.ActualState {
	case storage_v1alpha.MNT_DETACHED:
		// Already detached
		c.state.DeleteMount(entityId)
		if err := c.state.Save(); err != nil {
			c.log.Warn("failed to save state after mount cleanup", "error", err)
		}
		return nil
	case storage_v1alpha.MNT_UNMOUNTING, storage_v1alpha.MNT_DETACHING:
		// Already in progress
		return nil
	default:
		return c.unmountAndDetach(ctx, mount)
	}
}

// attachAndMount attaches NBD device and mounts the filesystem
func (c *MountController) attachAndMount(ctx context.Context, mount *storage_v1alpha.LsvdMount) error {
	entityId := string(mount.ID)
	volumeId := string(mount.VolumeId)

	// Check if we already have a handler for this mount (attach already in progress or done)
	c.handlersMu.Lock()
	_, hasHandler := c.handlers[entityId]
	c.handlersMu.Unlock()
	if hasHandler {
		c.log.Info("handler already exists, skipping attach", "entity_id", entityId)
		return nil
	}

	c.log.Info("attaching and mounting volume",
		"entity_id", entityId,
		"volume_id", volumeId,
		"mount_path", mount.MountPath,
	)

	// Update actual_state to MNT_ATTACHING
	if err := c.updateMountState(ctx, mount.ID, storage_v1alpha.MNT_ATTACHING, nil, nil, nil); err != nil {
		c.log.Warn("failed to update mount state to attaching", "error", err)
	}

	// Get volume state
	volState := c.state.GetVolume(volumeId)
	if volState == nil {
		c.setMountError(ctx, mount.ID, fmt.Sprintf("volume %s not found in state", volumeId))
		return fmt.Errorf("volume %s not found in state", volumeId)
	}

	// Acquire lease from cloud BEFORE opening disk
	leaseNonce, err := c.ops.AcquireVolumeLease(ctx, volState.VolumeId, map[string]any{
		"node":   c.nodeId,
		"entity": entityId,
	})
	if err != nil {
		c.setMountError(ctx, mount.ID, fmt.Sprintf("failed to acquire lease: %v", err))
		return fmt.Errorf("failed to acquire lease: %w", err)
	}
	c.log.Info("acquired volume lease", "entity_id", entityId, "volume_id", volState.VolumeId, "has_nonce", leaseNonce != "")

	// Open LSVD disk with lease nonce (for write operations)
	disk, err := c.ops.OpenLSVDDisk(ctx, volState.DiskPath, volState.VolumeId, volState.RemoteOnly, leaseNonce)
	if err != nil {
		// Release lease on failure to prevent blocking the volume
		if leaseNonce != "" {
			if releaseErr := c.ops.ReleaseVolumeLease(ctx, volState.VolumeId, leaseNonce); releaseErr != nil {
				c.log.Warn("failed to release lease after disk open failure", "error", releaseErr)
			}
		}
		c.setMountError(ctx, mount.ID, fmt.Sprintf("failed to open disk: %v", err))
		return fmt.Errorf("failed to open disk: %w", err)
	}

	// Attach NBD device
	sizeBytes := uint64(disk.Size())
	c.log.Info("attaching NBD device",
		"entity_id", entityId,
		"size_bytes", sizeBytes,
		"size_gb", sizeBytes/(1024*1024*1024),
	)
	idx, conn, clientFile, cleanup, err := c.ops.NBDLoopback(ctx, sizeBytes)
	if err != nil {
		disk.Close(ctx)
		if leaseNonce != "" {
			if releaseErr := c.ops.ReleaseVolumeLease(ctx, volState.VolumeId, leaseNonce); releaseErr != nil {
				c.log.Warn("failed to release lease after NBD loopback failure", "error", releaseErr)
			}
		}
		c.setMountError(ctx, mount.ID, fmt.Sprintf("failed to setup NBD loopback: %v", err))
		return fmt.Errorf("failed to setup NBD loopback: %w", err)
	}

	// Create device node using mount entity ID
	devicePath := c.getDevicePath(entityId)
	dir := filepath.Dir(devicePath)
	if err := c.ops.CreateDir(dir, 0755); err != nil {
		cleanup()
		disk.Close(ctx)
		if leaseNonce != "" {
			if releaseErr := c.ops.ReleaseVolumeLease(ctx, volState.VolumeId, leaseNonce); releaseErr != nil {
				c.log.Warn("failed to release lease after create dir failure", "error", releaseErr)
			}
		}
		c.setMountError(ctx, mount.ID, fmt.Sprintf("failed to create device directory: %v", err))
		return fmt.Errorf("failed to create device directory: %w", err)
	}

	if err := c.ops.CreateDeviceNode(devicePath, idx); err != nil {
		cleanup()
		disk.Close(ctx)
		if leaseNonce != "" {
			if releaseErr := c.ops.ReleaseVolumeLease(ctx, volState.VolumeId, leaseNonce); releaseErr != nil {
				c.log.Warn("failed to release lease after create device node failure", "error", releaseErr)
			}
		}
		c.setMountError(ctx, mount.ID, fmt.Sprintf("failed to create device node: %v", err))
		return fmt.Errorf("failed to create device node: %w", err)
	}

	// Start NBD handler
	handlerCtx, handlerCancel := context.WithCancel(ctx)
	handlerDone := make(chan struct{})
	go c.runNBDHandler(handlerCtx, entityId, disk, conn, clientFile, handlerDone)

	// Wait for NBD device to be ready
	deadline := time.Now().Add(10 * time.Second)
	nbdReady := false
	for time.Now().Before(deadline) {
		if err := c.ops.NBDStatus(idx); err == nil {
			nbdReady = true
			break
		}
		select {
		case <-ctx.Done():
			handlerCancel()
			<-handlerDone // wait for handler to exit
			cleanup()
			disk.Close(context.Background()) // Use background context for cleanup
			if leaseNonce != "" {
				if releaseErr := c.ops.ReleaseVolumeLease(context.Background(), volState.VolumeId, leaseNonce); releaseErr != nil {
					c.log.Warn("failed to release lease after context cancellation", "error", releaseErr)
				}
			}
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}

	if !nbdReady {
		handlerCancel()
		<-handlerDone // wait for handler to exit
		cleanup()
		disk.Close(context.Background()) // Use background context for cleanup
		if leaseNonce != "" {
			if releaseErr := c.ops.ReleaseVolumeLease(context.Background(), volState.VolumeId, leaseNonce); releaseErr != nil {
				c.log.Warn("failed to release lease after NBD ready timeout", "error", releaseErr)
			}
		}
		c.setMountError(ctx, mount.ID, "NBD device did not become ready: timeout")
		return fmt.Errorf("NBD device did not become ready: timeout")
	}

	// Update state with the lease nonce
	c.state.SetMount(entityId, &MountState{
		EntityId:   entityId,
		VolumeId:   volumeId,
		NbdIndex:   idx,
		DevicePath: devicePath,
		MountPath:  mount.MountPath,
		Mounted:    false,
		ReadOnly:   mount.ReadOnly,
		LeaseNonce: leaseNonce,
	})

	c.handlersMu.Lock()
	c.handlers[entityId] = &nbdHandler{
		conn:       conn,
		clientFile: clientFile,
		cancel:     handlerCancel,
		disk:       disk,
		done:       handlerDone,
	}
	c.handlersMu.Unlock()

	if err := c.state.Save(); err != nil {
		c.log.Warn("failed to save state after NBD attach", "error", err)
	}

	// Update actual_state to MNT_ATTACHED
	nbdIdx := int64(idx)
	if err := c.updateMountState(ctx, mount.ID, storage_v1alpha.MNT_ATTACHED, &nbdIdx, &devicePath, nil); err != nil {
		c.log.Warn("failed to update mount state to attached", "error", err)
	}

	// Update entity with lease nonce
	if leaseNonce != "" {
		attrs := []entity.Attr{
			entity.Ref(entity.DBId, mount.ID),
			entity.String(storage_v1alpha.LsvdMountLeaseNonceId, leaseNonce),
		}
		if _, err := c.eac.Patch(ctx, attrs, 0); err != nil {
			c.log.Warn("failed to update lease nonce in entity", "error", err)
		}
	}

	// Now mount the volume
	return c.mountVolume(ctx, mount)
}

// mountVolume mounts the filesystem
func (c *MountController) mountVolume(ctx context.Context, mount *storage_v1alpha.LsvdMount) error {
	entityId := string(mount.ID)

	mountState := c.state.GetMount(entityId)
	if mountState == nil {
		return fmt.Errorf("mount state not found for %s", entityId)
	}

	// Already mounted per local state, nothing to do
	if mountState.Mounted {
		c.log.Info("already mounted, skipping", "entity_id", entityId)
		return nil
	}

	c.log.Info("mounting filesystem",
		"entity_id", entityId,
		"device", mountState.DevicePath,
		"mount_path", mount.MountPath,
	)

	// Update actual_state to MNT_MOUNTING
	if err := c.updateMountState(ctx, mount.ID, storage_v1alpha.MNT_MOUNTING, nil, nil, nil); err != nil {
		c.log.Warn("failed to update mount state to mounting", "error", err)
	}

	// Create mount point
	if err := c.ops.CreateDir(mount.MountPath, 0755); err != nil {
		c.setMountError(ctx, mount.ID, fmt.Sprintf("failed to create mount point: %v", err))
		return fmt.Errorf("failed to create mount point: %w", err)
	}

	// Get volume state for filesystem info
	volState := c.state.GetVolume(mountState.VolumeId)
	filesystem := "ext4"
	if volState != nil && volState.Filesystem != "" {
		filesystem = volState.Filesystem
	}

	// Format if needed (check for existing filesystem)
	formatted, err := c.ops.IsFormatted(mountState.DevicePath, filesystem)
	if err != nil {
		// Treat probe errors as fatal to avoid accidentally formatting over existing data
		c.setMountError(ctx, mount.ID, fmt.Sprintf("failed to check if formatted: %v", err))
		return fmt.Errorf("failed to check if formatted: %w", err)
	}

	if !formatted {
		c.log.Info("formatting device", "device", mountState.DevicePath, "filesystem", filesystem)

		// Retry formatting with exponential backoff for up to 10 minutes
		formatDeadline := time.Now().Add(10 * time.Minute)
		backoff := 1 * time.Second
		maxBackoff := 30 * time.Second
		attempt := 0

		for {
			attempt++
			err := c.ops.FormatDevice(ctx, mountState.DevicePath, filesystem)
			if err == nil {
				c.log.Info("device formatted successfully", "device", mountState.DevicePath, "attempt", attempt)
				break
			}

			c.log.Error("format device failed, will retry",
				"device", mountState.DevicePath,
				"filesystem", filesystem,
				"attempt", attempt,
				"error", err,
			)

			if time.Now().After(formatDeadline) {
				c.setMountError(ctx, mount.ID, fmt.Sprintf("failed to format device after 10 minutes: %v", err))
				return fmt.Errorf("failed to format device after 10 minutes: %w", err)
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}

			// Exponential backoff with cap
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}

	// Mount the filesystem
	if err := c.ops.Mount(mountState.DevicePath, mount.MountPath, filesystem, mount.ReadOnly); err != nil {
		c.setMountError(ctx, mount.ID, fmt.Sprintf("failed to mount: %v", err))
		return fmt.Errorf("failed to mount: %w", err)
	}

	// Update state
	mountState.Mounted = true
	c.state.SetMount(entityId, mountState)
	if err := c.state.Save(); err != nil {
		c.log.Warn("failed to save state after mount", "error", err)
	}

	c.log.Info("filesystem mounted",
		"entity_id", entityId,
		"mount_path", mount.MountPath,
	)

	// Update actual_state to MNT_MOUNTED
	if err := c.updateMountState(ctx, mount.ID, storage_v1alpha.MNT_MOUNTED, nil, nil, nil); err != nil {
		c.log.Warn("failed to update mount state to mounted", "error", err)
	}

	return nil
}

// unmountAndDetach unmounts the filesystem and detaches the NBD device
func (c *MountController) unmountAndDetach(ctx context.Context, mount *storage_v1alpha.LsvdMount) error {
	entityId := string(mount.ID)

	c.log.Info("unmounting and detaching",
		"entity_id", entityId,
		"mount_path", mount.MountPath,
	)

	// Update actual_state to MNT_UNMOUNTING
	if err := c.updateMountState(ctx, mount.ID, storage_v1alpha.MNT_UNMOUNTING, nil, nil, nil); err != nil {
		c.log.Warn("failed to update mount state to unmounting", "error", err)
	}

	mountState := c.state.GetMount(entityId)
	if mountState == nil {
		c.log.Warn("mount state not found", "entity_id", entityId)
		// Update actual_state to MNT_DETACHED and clear stale fields
		clearNbd := int64(0)
		clearPath := ""
		clearErr := ""
		if err := c.updateMountState(ctx, mount.ID, storage_v1alpha.MNT_DETACHED, &clearNbd, &clearPath, &clearErr); err != nil {
			c.log.Warn("failed to update mount state to detached", "error", err)
		}
		return nil
	}

	// Unmount if mounted, but skip the filesystem unmount if another mount
	// entity is actively using the same path (e.g., during rapid sandbox
	// replacement where two mount entities reference the same disk/volume).
	if mountState.Mounted {
		skipUnmount := false
		for _, otherMount := range c.state.ListMounts() {
			if otherMount.EntityId != entityId && otherMount.MountPath == mountState.MountPath && otherMount.Mounted {
				c.log.Info("skipping unmount, path in use by another mount",
					"entity_id", entityId,
					"other_entity_id", otherMount.EntityId,
					"mount_path", mountState.MountPath,
				)
				skipUnmount = true
				break
			}
		}

		if !skipUnmount {
			c.log.Debug("unmounting filesystem", "entity_id", entityId, "mount_path", mountState.MountPath)
			if err := c.ops.Unmount(mountState.MountPath); err != nil {
				c.log.Warn("failed to unmount", "error", err)
				return fmt.Errorf("failed to unmount: %w", err)
			}
			c.log.Debug("filesystem unmounted", "entity_id", entityId, "mount_path", mountState.MountPath)
		}
		mountState.Mounted = false
		c.state.SetMount(entityId, mountState)
	}

	// Update actual_state to MNT_DETACHING
	if err := c.updateMountState(ctx, mount.ID, storage_v1alpha.MNT_DETACHING, nil, nil, nil); err != nil {
		c.log.Warn("failed to update mount state to detaching", "error", err)
	}

	// Stop NBD handler
	c.handlersMu.Lock()
	h, hasHandler := c.handlers[entityId]
	if hasHandler {
		if h.cancel != nil {
			h.cancel()
		}
		delete(c.handlers, entityId)
	}
	c.handlersMu.Unlock()

	// Disconnect NBD device BEFORE waiting for handler — this unblocks
	// HandleNBD's read loop (HandleTransport polls with 500ms deadlines
	// and only exits when the connection breaks).
	if mountState.DevicePath != "" {
		if err := c.ops.NBDDisconnect(mountState.NbdIndex); err != nil {
			c.log.Warn("failed to disconnect NBD", "error", err)
		}
	}

	// Now safe to wait for handler goroutine to exit
	if hasHandler && h.done != nil {
		c.log.Info("waiting for NBD handler to exit", "entity_id", entityId)
		<-h.done
		c.log.Info("NBD handler exited", "entity_id", entityId)
	}
	if hasHandler && h.disk != nil {
		h.disk.Close(ctx)
	}

	// Remove device node
	if mountState.DevicePath != "" {
		c.ops.RemoveFile(mountState.DevicePath)
	}

	// Release lease from cloud after unmount
	if mountState.LeaseNonce != "" {
		volState := c.state.GetVolume(mountState.VolumeId)
		if volState != nil {
			if err := c.ops.ReleaseVolumeLease(ctx, volState.VolumeId, mountState.LeaseNonce); err != nil {
				c.log.Warn("failed to release lease", "entity_id", entityId, "error", err)
				// Continue anyway - cloud will eventually clean up expired leases
			} else {
				c.log.Info("released volume lease", "entity_id", entityId, "volume_id", volState.VolumeId)
			}
		}
	}

	// Update state
	c.state.DeleteMount(entityId)
	if err := c.state.Save(); err != nil {
		c.log.Warn("failed to save state after unmount", "error", err)
	}

	c.log.Info("volume unmounted and detached", "entity_id", entityId)

	// Update actual_state to MNT_DETACHED and clear stale NBD/device/error fields
	clearNbd := int64(0)
	clearPath := ""
	clearErr := ""
	if err := c.updateMountState(ctx, mount.ID, storage_v1alpha.MNT_DETACHED, &clearNbd, &clearPath, &clearErr); err != nil {
		c.log.Warn("failed to update mount state to detached", "error", err)
	}

	return nil
}

// cleanupHandler cleans up any existing NBD handler for the given entity.
// This should be called before retrying a failed mount to avoid leaking NBD devices.
func (c *MountController) cleanupHandler(ctx context.Context, entityId string) {
	c.handlersMu.Lock()
	h, ok := c.handlers[entityId]
	if ok {
		c.log.Info("cleaning up existing handler before retry", "entity_id", entityId)
		if h.cancel != nil {
			h.cancel()
		}
		delete(c.handlers, entityId)
	}
	c.handlersMu.Unlock()

	// Disconnect NBD device BEFORE waiting for handler — this unblocks
	// HandleNBD's read loop.
	mountState := c.state.GetMount(entityId)
	if mountState != nil && mountState.DevicePath != "" {
		c.log.Info("disconnecting stale NBD device", "entity_id", entityId, "nbd_index", mountState.NbdIndex)
		_ = c.ops.NBDDisconnect(mountState.NbdIndex)
		c.ops.RemoveFile(mountState.DevicePath)
	}

	// Now safe to wait for handler goroutine to exit
	if ok && h.done != nil {
		c.log.Info("waiting for NBD handler to exit", "entity_id", entityId)
		<-h.done
	}

	// Now safe to close the disk
	if ok && h.disk != nil {
		h.disk.Close(ctx)
	}

	// Release lease if present
	if mountState != nil && mountState.LeaseNonce != "" {
		volState := c.state.GetVolume(mountState.VolumeId)
		if volState != nil {
			if err := c.ops.ReleaseVolumeLease(ctx, volState.VolumeId, mountState.LeaseNonce); err != nil {
				c.log.Warn("failed to release lease in cleanupHandler", "entity_id", entityId, "error", err)
			} else {
				c.log.Info("released lease in cleanupHandler", "entity_id", entityId, "volume_id", volState.VolumeId)
			}
		}
	}

	// Clean up state so attachAndMount starts fresh
	c.state.DeleteMount(entityId)
}

// runNBDHandler runs the NBD handler for a mounted volume
func (c *MountController) runNBDHandler(ctx context.Context, entityId string, disk LSVDDisk, conn net.Conn, clientFile *os.File, done chan struct{}) {
	c.log.Info("starting NBD handler", "entity_id", entityId)
	defer close(done)
	defer c.log.Info("NBD handler stopped", "entity_id", entityId)
	defer clientFile.Close()
	defer conn.Close()

	if err := disk.HandleNBD(ctx, conn, clientFile); err != nil {
		if ctx.Err() != nil {
			c.log.Debug("NBD handler stopped due to context cancellation", "entity_id", entityId)
		} else {
			c.log.Warn("NBD handler error", "entity_id", entityId, "error", err)
		}
	}
}

// getDevicePath returns the path to a device node based on the mount entity ID.
// It extracts the part after "/" in the entity ID (e.g., "lsvd_mount/abc123" -> "abc123").
func (c *MountController) getDevicePath(mountEntityId string) string {
	// Extract the ID part after the "/"
	deviceId := mountEntityId
	if idx := strings.LastIndex(mountEntityId, "/"); idx != -1 {
		deviceId = mountEntityId[idx+1:]
	}
	return filepath.Join(c.dataPath, "devices", deviceId)
}

// ReconcileWithSystem reconciles mount state with the actual system
func (c *MountController) ReconcileWithSystem(ctx context.Context) error {
	c.log.Info("reconciling mounts with system")

	// Use thread-safe accessor to get a snapshot of mounts
	for _, mountState := range c.state.ListMounts() {
		entityId := mountState.EntityId
		// Check if we have an active handler for this mount.
		// If we have a handler, the NBD device is managed by us — no reconnection needed.
		// If we don't have a handler, it means the process restarted and we need to
		// reconnect the NBD device.
		c.handlersMu.RLock()
		_, hasHandler := c.handlers[entityId]
		c.handlersMu.RUnlock()

		if !hasHandler {
			// Before reconnecting, check whether this mount is being unmounted.
			// There is a race window where unmountAndDetach has deleted the handler
			// from the map but hasn't yet cleaned up local state. Reconnecting
			// in that window wastes time and can leak NBD devices.
			if c.eac != nil {
				if skip := c.shouldSkipReconnect(ctx, entityId); skip {
					continue
				}
			}

			c.log.Info("no NBD handler found, reconnecting",
				"entity_id", entityId,
				"nbd_index", mountState.NbdIndex,
			)

			// Try to reconnect the NBD device
			if err := c.reconnectNBD(ctx, entityId, mountState); err != nil {
				c.log.Error("failed to reconnect NBD",
					"entity_id", entityId,
					"error", err,
				)
				// Mark mount state as needing recovery
				mountState.Mounted = false
				c.state.SetMount(entityId, mountState)
				continue
			}
		}

		// Check if mounted (regardless of local flag, to catch reconnect recovery)
		if !c.ops.IsMounted(mountState.MountPath) {
			c.log.Warn("volume not mounted but should be",
				"entity_id", entityId,
				"mount_path", mountState.MountPath,
			)
			mountState.Mounted = false
			c.state.SetMount(entityId, mountState)

			// Try to remount
			if err := c.remountFilesystem(ctx, entityId, mountState); err != nil {
				c.log.Error("failed to remount filesystem",
					"entity_id", entityId,
					"error", err,
				)
			}
		}
	}

	if err := c.state.Save(); err != nil {
		c.log.Warn("failed to save state after system reconciliation", "error", err)
	}

	return nil
}

// reconnectNBD reconnects the NBD device for a mount after process restart
func (c *MountController) reconnectNBD(ctx context.Context, entityId string, mountState *MountState) error {
	// Get volume state
	volState := c.state.GetVolume(mountState.VolumeId)
	if volState == nil {
		return fmt.Errorf("volume %s not found in state", mountState.VolumeId)
	}

	// Use the stored lease nonce from local state (don't re-acquire)
	leaseNonce := mountState.LeaseNonce

	c.log.Info("reconnecting NBD device",
		"entity_id", entityId,
		"volume_id", mountState.VolumeId,
		"disk_path", volState.DiskPath,
		"has_lease_nonce", leaseNonce != "",
	)

	// Clean up old handler if exists
	c.handlersMu.Lock()
	h, hasHandler := c.handlers[entityId]
	if hasHandler {
		if h.cancel != nil {
			h.cancel()
		}
		delete(c.handlers, entityId)
	}
	c.handlersMu.Unlock()

	// Disconnect old NBD device BEFORE waiting for handler — this unblocks
	// HandleNBD's read loop.
	if mountState.DevicePath != "" {
		_ = c.ops.NBDDisconnect(mountState.NbdIndex)
	}

	// Now safe to wait for handler goroutine to exit
	if hasHandler && h.done != nil {
		<-h.done
	}
	if hasHandler && h.disk != nil {
		h.disk.Close(context.Background())
	}

	// Open LSVD disk with stored lease nonce
	disk, err := c.ops.OpenLSVDDisk(ctx, volState.DiskPath, volState.VolumeId, volState.RemoteOnly, leaseNonce)
	if err != nil {
		return fmt.Errorf("failed to open disk: %w", err)
	}

	// Attach NBD device
	sizeBytes := uint64(disk.Size())
	idx, conn, clientFile, cleanup, err := c.ops.NBDLoopback(ctx, sizeBytes)
	if err != nil {
		disk.Close(ctx)
		return fmt.Errorf("failed to setup NBD loopback: %w", err)
	}

	// Create device node using mount entity ID
	devicePath := c.getDevicePath(entityId)
	dir := filepath.Dir(devicePath)
	if err := c.ops.CreateDir(dir, 0755); err != nil {
		cleanup()
		disk.Close(ctx)
		return fmt.Errorf("failed to create device directory: %w", err)
	}

	if err := c.ops.CreateDeviceNode(devicePath, idx); err != nil {
		cleanup()
		disk.Close(ctx)
		return fmt.Errorf("failed to create device node: %w", err)
	}

	// Start NBD handler
	handlerCtx, handlerCancel := context.WithCancel(ctx)
	handlerDone := make(chan struct{})
	go c.runNBDHandler(handlerCtx, entityId, disk, conn, clientFile, handlerDone)

	// Wait for NBD device to be ready
	deadline := time.Now().Add(10 * time.Second)
	nbdReady := false
	for time.Now().Before(deadline) {
		if err := c.ops.NBDStatus(idx); err == nil {
			nbdReady = true
			break
		}
		select {
		case <-ctx.Done():
			handlerCancel()
			<-handlerDone // wait for handler to exit
			cleanup()
			disk.Close(context.Background()) // Use background context for cleanup
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
	if !nbdReady {
		handlerCancel()
		<-handlerDone // wait for handler to exit
		cleanup()
		disk.Close(context.Background()) // Use background context for cleanup
		return fmt.Errorf("NBD device did not become ready after reconnect: timeout")
	}

	// Update state with new NBD index
	mountState.NbdIndex = idx
	mountState.DevicePath = devicePath
	c.state.SetMount(entityId, mountState)

	c.handlersMu.Lock()
	c.handlers[entityId] = &nbdHandler{
		conn:       conn,
		clientFile: clientFile,
		cancel:     handlerCancel,
		disk:       disk,
		done:       handlerDone,
	}
	c.handlersMu.Unlock()

	c.log.Info("NBD device reconnected",
		"entity_id", entityId,
		"nbd_index", idx,
		"device_path", devicePath,
	)

	return nil
}

// shouldSkipReconnect checks the entity store to determine if a mount should
// not be reconnected. Returns true if the entity doesn't exist (already cleaned
// up) or has desired_state=MNT_WANT_UNMOUNTED (being torn down).
func (c *MountController) shouldSkipReconnect(ctx context.Context, entityId string) bool {
	resp, err := c.eac.Get(ctx, entityId)
	if err != nil {
		if errors.Is(err, cond.ErrNotFound{}) {
			c.log.Info("mount entity not found, skipping reconnect",
				"entity_id", entityId,
			)
			return true
		}
		c.log.Warn("failed to check mount entity, allowing reconnect",
			"entity_id", entityId,
			"error", err,
		)
		return false
	}

	var mount storage_v1alpha.LsvdMount
	mount.Decode(resp.Entity().Entity())

	if mount.DesiredState == storage_v1alpha.MNT_WANT_UNMOUNTED {
		c.log.Info("mount is being unmounted, skipping reconnect",
			"entity_id", entityId,
		)
		return true
	}

	return false
}

// remountFilesystem remounts the filesystem after recovery
func (c *MountController) remountFilesystem(ctx context.Context, entityId string, mountState *MountState) error {
	// Get volume state for filesystem info
	volState := c.state.GetVolume(mountState.VolumeId)
	filesystem := "ext4"
	if volState != nil && volState.Filesystem != "" {
		filesystem = volState.Filesystem
	}

	c.log.Info("remounting filesystem",
		"entity_id", entityId,
		"device", mountState.DevicePath,
		"mount_path", mountState.MountPath,
		"filesystem", filesystem,
	)

	// Create mount point if needed
	if err := c.ops.CreateDir(mountState.MountPath, 0755); err != nil {
		return fmt.Errorf("failed to create mount point: %w", err)
	}

	// Mount the filesystem
	if err := c.ops.Mount(mountState.DevicePath, mountState.MountPath, filesystem, mountState.ReadOnly); err != nil {
		return fmt.Errorf("failed to mount: %w", err)
	}

	// Update state
	mountState.Mounted = true
	c.state.SetMount(entityId, mountState)

	c.log.Info("filesystem remounted",
		"entity_id", entityId,
		"mount_path", mountState.MountPath,
	)

	return nil
}

// ReconcileWithEntities reconciles local state with entity server
func (c *MountController) ReconcileWithEntities(ctx context.Context) error {
	// List all lsvd_mount entities for this node
	// Node ID in entities uses full entity path format: "node/<name>"
	fullNodeId := "node/" + c.nodeId
	nodeIdRef := entity.Id(fullNodeId)
	indexAttr := entity.Ref(storage_v1alpha.LsvdMountNodeIdId, nodeIdRef)

	resp, err := c.eac.List(ctx, indexAttr)
	if err != nil {
		return fmt.Errorf("failed to list mount entities: %w", err)
	}

	values := resp.Values()

	// Build set of entity IDs from the server response
	entityIds := make(map[string]struct{}, len(values))

	for _, entResp := range values {
		var mount storage_v1alpha.LsvdMount
		mount.Decode(entResp.Entity())

		entityIds[string(mount.ID)] = struct{}{}

		// Skip if not for this node
		if string(mount.NodeId) != fullNodeId {
			continue
		}

		// Reconcile the mount
		if err := c.reconcileMount(ctx, &mount); err != nil {
			c.log.Error("failed to reconcile mount",
				"entity_id", mount.ID,
				"error", err,
			)
		}
	}

	// Clean up orphaned mounts: local state entries with no corresponding entity
	orphanCleaned := false
	for _, mountState := range c.state.ListMounts() {
		if _, exists := entityIds[mountState.EntityId]; exists {
			continue
		}

		c.log.Info("cleaning up orphaned mount", "entity_id", mountState.EntityId)

		// Unmount filesystem before tearing down handler
		if mountState.Mounted && mountState.MountPath != "" {
			if err := c.ops.Unmount(mountState.MountPath); err != nil {
				c.log.Warn("failed to unmount orphaned mount", "entity_id", mountState.EntityId, "error", err)
			}
		}

		devicePath := mountState.DevicePath
		leaseNonce := mountState.LeaseNonce
		volumeId := mountState.VolumeId

		// cleanupHandler tears down the NBD handler, disconnects NBD, and deletes mount state
		c.cleanupHandler(ctx, mountState.EntityId)

		// Remove the device node (not handled by cleanupHandler)
		if devicePath != "" {
			c.ops.RemoveFile(devicePath)
		}

		// Release lease for orphaned mount
		if leaseNonce != "" {
			volState := c.state.GetVolume(volumeId)
			if volState != nil {
				if err := c.ops.ReleaseVolumeLease(ctx, volState.VolumeId, leaseNonce); err != nil {
					c.log.Warn("failed to release lease for orphaned mount", "entity_id", mountState.EntityId, "error", err)
				} else {
					c.log.Info("released lease for orphaned mount", "entity_id", mountState.EntityId, "volume_id", volState.VolumeId)
				}
			}
		}

		orphanCleaned = true
	}

	if orphanCleaned {
		if err := c.state.Save(); err != nil {
			c.log.Warn("failed to save state after orphan cleanup", "error", err)
		}
	}

	return nil
}

// mountActualStateToId maps LsvdMountActualState to entity.Id
func mountActualStateToId(state storage_v1alpha.LsvdMountActualState) entity.Id {
	switch state {
	case storage_v1alpha.MNT_PENDING:
		return storage_v1alpha.LsvdMountActualStateMntPendingId
	case storage_v1alpha.MNT_ATTACHING:
		return storage_v1alpha.LsvdMountActualStateMntAttachingId
	case storage_v1alpha.MNT_ATTACHED:
		return storage_v1alpha.LsvdMountActualStateMntAttachedId
	case storage_v1alpha.MNT_MOUNTING:
		return storage_v1alpha.LsvdMountActualStateMntMountingId
	case storage_v1alpha.MNT_MOUNTED:
		return storage_v1alpha.LsvdMountActualStateMntMountedId
	case storage_v1alpha.MNT_UNMOUNTING:
		return storage_v1alpha.LsvdMountActualStateMntUnmountingId
	case storage_v1alpha.MNT_DETACHING:
		return storage_v1alpha.LsvdMountActualStateMntDetachingId
	case storage_v1alpha.MNT_DETACHED:
		return storage_v1alpha.LsvdMountActualStateMntDetachedId
	case storage_v1alpha.MNT_ERROR:
		return storage_v1alpha.LsvdMountActualStateMntErrorId
	default:
		return storage_v1alpha.LsvdMountActualStateMntPendingId
	}
}

// updateMountState updates the actual_state and optionally other fields in the entity
// updateMountState updates the actual_state and optionally other fields.
// Use nil pointers to leave fields unchanged, or pass a pointer to explicitly set a value (including zero/empty to clear).
func (c *MountController) updateMountState(ctx context.Context, id entity.Id, state storage_v1alpha.LsvdMountActualState, nbdIndex *int64, devicePath, errorMsg *string) error {
	// Get the entity.Id for the state
	stateId := mountActualStateToId(state)

	// Build attrs for the update - include entity ID for Patch
	attrs := []entity.Attr{
		entity.Ref(entity.DBId, id),
		entity.Ref(storage_v1alpha.LsvdMountActualStateId, stateId),
	}

	if nbdIndex != nil {
		attrs = append(attrs, entity.Int64(storage_v1alpha.LsvdMountNbdIndexId, *nbdIndex))
	}

	if devicePath != nil {
		attrs = append(attrs, entity.String(storage_v1alpha.LsvdMountDevicePathId, *devicePath))
	}

	if errorMsg != nil {
		attrs = append(attrs, entity.String(storage_v1alpha.LsvdMountErrorMessageId, *errorMsg))
	}

	_, err := c.eac.Patch(ctx, attrs, 0)
	return err
}

// setMountError sets the mount to error state with a message
func (c *MountController) setMountError(ctx context.Context, id entity.Id, errorMsg string) {
	if err := c.updateMountState(ctx, id, storage_v1alpha.MNT_ERROR, nil, nil, &errorMsg); err != nil {
		c.log.Warn("failed to set mount error state", "entity_id", id, "error", err)
	}
}
