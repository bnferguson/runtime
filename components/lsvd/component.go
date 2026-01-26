// Package lsvd provides a component that manages LSVD volumes and mounts
// as a separate process that survives server restarts.
package lsvd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"miren.dev/runtime/api/lsvd/lsvd_v1alpha"
	"miren.dev/runtime/components/lsvd/server"
	"miren.dev/runtime/pkg/rpc"
	"miren.dev/runtime/pkg/rpc/standard"
	"miren.dev/runtime/pkg/slogout"
)

// Config defines the configuration for running lsvd-server
type Config struct {
	// Path to the lsvd-server binary
	BinaryPath string

	// Base directory for LSVD data (volumes, state file)
	DataPath string

	// Entity server RPC address (e.g., localhost:9000)
	EntityServerAddr string

	// Node ID for filtering entities
	NodeId string

	// Optional: additional environment variables
	Env []string
}

// Component manages an lsvd-server process
type Component struct {
	log      *slog.Logger
	dataPath string

	mu       sync.Mutex
	config   *Config
	cmd      *exec.Cmd
	running  bool
	waitDone chan struct{}

	// RPC client for debug interface
	rpcState    *rpc.State
	debugClient *lsvd_v1alpha.LsvdDebugClient
}

// NewComponent creates a new LSVD component
func NewComponent(log *slog.Logger, dataPath string) *Component {
	return &Component{
		log:      log.With("module", "lsvd"),
		dataPath: dataPath,
	}
}

// Start starts the lsvd-server process
func (c *Component) Start(ctx context.Context, config *Config) error {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return fmt.Errorf("lsvd-server is already running")
	}
	c.config = config
	c.mu.Unlock()

	// Validate binary exists
	if _, err := os.Stat(config.BinaryPath); err != nil {
		return fmt.Errorf("lsvd-server binary not found at %s: %w", config.BinaryPath, err)
	}

	// Create data directory if it doesn't exist
	if err := os.MkdirAll(config.DataPath, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Build command arguments
	args := []string{
		"--data-path", config.DataPath,
		"--node-id", config.NodeId,
		"--entity-server", config.EntityServerAddr,
	}

	cmd := exec.Command(config.BinaryPath, args...)

	// Set process group so it survives parent death
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	}

	c.log.Info("starting lsvd-server",
		"binary", config.BinaryPath,
		"data_path", config.DataPath,
		"node_id", config.NodeId,
	)

	// Set environment
	cmd.Env = append(os.Environ(), config.Env...)

	// Setup logging for lsvd-server output
	stdout := slogout.NewWriter(c.log, slog.LevelInfo,
		slogout.WithKeyValueParsing(), slogout.WithMaxLevel(slog.LevelInfo))
	stderr := slogout.NewWriter(c.log, slog.LevelError, slogout.WithKeyValueParsing(),
		slogout.WithMaxLevel(slog.LevelInfo))
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	// Start lsvd-server
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start lsvd-server: %w", err)
	}

	// Update state with lock
	c.mu.Lock()
	c.cmd = cmd
	c.running = true
	c.waitDone = make(chan struct{})
	c.mu.Unlock()

	c.log.Info("lsvd-server started", "pid", cmd.Process.Pid)

	// Monitor the process in background
	go func() {
		defer close(c.waitDone)

		if err := cmd.Wait(); err != nil {
			c.log.Error("lsvd-server exited with error", "error", err)
		} else {
			c.log.Info("lsvd-server exited")
		}

		// Close the log writers to flush any remaining data
		stdout.Close()
		stderr.Close()

		c.mu.Lock()
		c.running = false
		c.cmd = nil
		c.mu.Unlock()
	}()

	// Wait for ready file to be created
	readyFile := filepath.Join(config.DataPath, server.ReadyFileName)
	if err := c.waitForReady(ctx, readyFile); err != nil {
		// Clean up on failure
		cmd.Process.Kill()
		<-c.waitDone
		return fmt.Errorf("lsvd-server failed to start: %w", err)
	}

	// Connect to the debug RPC server
	if err := c.connectRPC(ctx, config.DataPath); err != nil {
		c.log.Warn("failed to connect to debug RPC", "error", err)
		// Don't fail startup if RPC connection fails
	}

	// Check if the running server needs to be upgraded
	upgraded, err := c.checkAndUpgradeVersion(ctx, config)
	if err != nil {
		c.log.Warn("failed to check version", "error", err)
	} else if upgraded {
		// Server was restarted, get fresh PID
		c.mu.Lock()
		cmd = c.cmd
		c.mu.Unlock()
	}

	pid := 0
	if cmd != nil && cmd.Process != nil {
		pid = cmd.Process.Pid
	}

	c.log.Info("lsvd-server ready",
		"data_path", config.DataPath,
		"pid", pid,
	)

	return nil
}

// Stop stops the lsvd-server process
func (c *Component) Stop(ctx context.Context) error {
	c.mu.Lock()

	if !c.running || c.cmd == nil {
		c.mu.Unlock()
		return nil
	}

	cmd := c.cmd
	c.mu.Unlock()

	c.log.Info("stopping lsvd-server")

	// Get waitDone channel before sending signal
	c.mu.Lock()
	waitDone := c.waitDone
	c.mu.Unlock()

	// Send SIGTERM for graceful shutdown
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		c.log.Warn("failed to send SIGTERM", "error", err)
	}

	// Wait for process to exit with timeout
	select {
	case <-ctx.Done():
		c.log.Warn("context cancelled, force killing lsvd-server")
		cmd.Process.Kill()
		<-waitDone
	case <-waitDone:
		// Process exited normally
	case <-time.After(30 * time.Second):
		c.log.Warn("shutdown timeout, force killing lsvd-server")
		cmd.Process.Kill()
		<-waitDone
	}

	c.mu.Lock()
	c.running = false
	c.cmd = nil
	c.debugClient = nil
	if c.rpcState != nil {
		c.rpcState.Close()
		c.rpcState = nil
	}
	c.mu.Unlock()

	c.log.Info("lsvd-server stopped")

	return nil
}

// Close implements io.Closer for use in closers list
func (c *Component) Close() error {
	return c.Stop(context.Background())
}

// IsRunning returns true if lsvd-server is running
func (c *Component) IsRunning() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}

// PID returns the PID of the running lsvd-server process
func (c *Component) PID() (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running || c.cmd == nil || c.cmd.Process == nil {
		return 0, fmt.Errorf("lsvd-server is not running")
	}

	return c.cmd.Process.Pid, nil
}

// waitForReady waits for the ready file to be created
func (c *Component) waitForReady(ctx context.Context, readyFile string) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(60 * time.Second)
	startTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("timeout waiting for lsvd-server ready file at %s", readyFile)
		case <-ticker.C:
			if _, err := os.Stat(readyFile); err == nil {
				c.log.Info("lsvd-server ready file found", "path", readyFile, "waited", time.Since(startTime))
				return nil
			} else if !os.IsNotExist(err) {
				c.log.Warn("error checking ready file", "path", readyFile, "error", err)
			}
		}
	}
}

// connectRPC connects to the lsvd-server debug RPC interface
func (c *Component) connectRPC(ctx context.Context, dataPath string) error {
	addrFile := filepath.Join(dataPath, server.AddrFileName)

	// Read the address from file
	data, err := os.ReadFile(addrFile)
	if err != nil {
		return fmt.Errorf("failed to read address file: %w", err)
	}

	addr := strings.TrimSpace(string(data))
	if addr == "" {
		return fmt.Errorf("empty address in file")
	}

	// Create RPC state
	rpcState, err := rpc.NewState(ctx, rpc.WithLogger(c.log), rpc.WithSkipVerify)
	if err != nil {
		return fmt.Errorf("failed to create RPC state: %w", err)
	}

	// Connect to the debug service
	client, err := rpcState.Connect(addr, "lsvd-debug")
	if err != nil {
		rpcState.Close()
		return fmt.Errorf("failed to connect to debug RPC: %w", err)
	}

	c.mu.Lock()
	c.rpcState = rpcState
	c.debugClient = lsvd_v1alpha.NewLsvdDebugClient(client)
	c.mu.Unlock()

	c.log.Info("connected to lsvd-server debug RPC", "addr", addr)
	return nil
}

// HealthStatus represents the health status of lsvd-server
type HealthStatus struct {
	Healthy               bool      `json:"healthy"`
	Timestamp             time.Time `json:"timestamp"`
	PID                   int       `json:"pid"`
	EntityServerConnected bool      `json:"entity_server_connected"`
	VolumeCount           int       `json:"volume_count"`
	MountCount            int       `json:"mount_count"`
	LastVolumeReconcile   time.Time `json:"last_volume_reconcile,omitempty"`
	LastMountReconcile    time.Time `json:"last_mount_reconcile,omitempty"`
	LastError             string    `json:"last_error,omitempty"`
}

// Health returns the current health status of lsvd-server via RPC
func (c *Component) Health() (*HealthStatus, error) {
	c.mu.Lock()
	client := c.debugClient
	c.mu.Unlock()

	if client == nil {
		return nil, fmt.Errorf("lsvd-server RPC client not connected")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := client.Health(ctx)
	if err != nil {
		return nil, fmt.Errorf("RPC health call failed: %w", err)
	}

	rpcStatus := result.Status()
	if rpcStatus == nil {
		return nil, fmt.Errorf("nil status from RPC")
	}

	status := &HealthStatus{
		Healthy:               rpcStatus.Healthy(),
		Timestamp:             standard.FromTimestamp(rpcStatus.Timestamp()),
		PID:                   int(rpcStatus.Pid()),
		EntityServerConnected: rpcStatus.EntityServerConnected(),
		VolumeCount:           int(rpcStatus.VolumeCount()),
		MountCount:            int(rpcStatus.MountCount()),
		LastVolumeReconcile:   standard.FromTimestamp(rpcStatus.LastVolumeReconcile()),
		LastMountReconcile:    standard.FromTimestamp(rpcStatus.LastMountReconcile()),
		LastError:             rpcStatus.LastError(),
	}

	return status, nil
}

// IsHealthy returns true if lsvd-server is running and healthy
func (c *Component) IsHealthy() bool {
	if !c.IsRunning() {
		return false
	}

	status, err := c.Health()
	if err != nil {
		return false
	}

	return status.Healthy
}

// checkAndUpgradeVersion checks if the running lsvd-server needs to be upgraded.
// If an upgrade is needed, it waits for the old process to exit and starts a new one.
// Returns true if an upgrade was performed.
func (c *Component) checkAndUpgradeVersion(ctx context.Context, config *Config) (bool, error) {
	c.mu.Lock()
	client := c.debugClient
	c.mu.Unlock()

	if client == nil {
		return false, fmt.Errorf("RPC client not connected")
	}

	// Check version with the expected version from the server package
	result, err := client.CheckVersion(ctx, server.ServerVersion)
	if err != nil {
		return false, fmt.Errorf("version check failed: %w", err)
	}

	versionResult := result.Result()
	if versionResult == nil {
		return false, fmt.Errorf("nil result from version check")
	}

	c.log.Info("version check result",
		"current_version", versionResult.CurrentVersion(),
		"expected_version", server.ServerVersion,
		"needs_restart", versionResult.NeedsRestart(),
	)

	if !versionResult.NeedsRestart() {
		return false, nil
	}

	// Server indicated it will exit for upgrade
	pid := int(versionResult.Pid())
	c.log.Info("waiting for lsvd-server to exit for upgrade", "pid", pid)

	// Close existing RPC connection and capture waitDone for synchronization
	c.mu.Lock()
	if c.rpcState != nil {
		c.rpcState.Close()
		c.rpcState = nil
	}
	c.debugClient = nil
	waitDone := c.waitDone
	c.mu.Unlock()

	// Wait for the process to exit
	if err := c.waitForPidExit(ctx, pid, 30*time.Second); err != nil {
		return false, fmt.Errorf("failed waiting for process to exit: %w", err)
	}

	// Wait for the monitor goroutine to finish before clearing state
	// This prevents a race where the monitor's deferred close(c.waitDone)
	// could close a new waitDone channel created by the subsequent Start()
	if waitDone != nil {
		<-waitDone
	}

	// Clear our state
	c.mu.Lock()
	c.running = false
	c.cmd = nil
	c.mu.Unlock()

	c.log.Info("old lsvd-server exited, starting new version")

	// Start the new version
	if err := c.Start(ctx, config); err != nil {
		return false, fmt.Errorf("failed to start new version: %w", err)
	}

	return true, nil
}

// waitForPidExit waits for a process with the given PID to exit
func (c *Component) waitForPidExit(ctx context.Context, pid int, timeout time.Duration) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	deadline := time.After(timeout)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			return fmt.Errorf("timeout waiting for pid %d to exit", pid)
		case <-ticker.C:
			// Check if process still exists by sending signal 0
			process, err := os.FindProcess(pid)
			if err != nil {
				// Process not found
				return nil
			}

			// On Unix, FindProcess always succeeds, so we need to send signal 0
			err = process.Signal(syscall.Signal(0))
			if err != nil {
				// Process doesn't exist or we can't signal it
				return nil
			}
		}
	}
}
