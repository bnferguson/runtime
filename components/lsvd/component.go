package lsvd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"miren.dev/runtime/api/lsvd/lsvd_v1alpha"
	"miren.dev/runtime/components/lsvd/server"
	"miren.dev/runtime/pkg/outboard"
	"miren.dev/runtime/pkg/rpc"
)

// ServiceConfig re-exports server.ServiceConfig for use by the runner
type ServiceConfig = server.ServiceConfig

// LoadServiceConfig re-exports server.LoadServiceConfig for use by the runner
var LoadServiceConfig = server.LoadServiceConfig

// SaveServiceConfig re-exports server.SaveServiceConfig for use by the runner
var SaveServiceConfig = server.SaveServiceConfig

// Config defines the configuration for running lsvd-server
type Config struct {
	// Base directory for LSVD data (volumes, state file)
	DataPath string

	// Directory for outboard process files (config, FIFOs)
	// e.g., /var/lib/miren/outboard/lsvd-server
	OutboardPath string

	// Entity server RPC address (e.g., localhost:9000)
	EntityServerAddr string

	// Node ID for filtering entities
	NodeId string

	// Optional: additional environment variables
	Env []string
}

// Component manages an lsvd-server as an outboard process
type Component struct {
	log      *slog.Logger
	olog     *slog.Logger
	dataPath string

	mu          sync.Mutex
	config      *Config
	connector   *outboard.Connector
	running     bool
	restartMode bool // If true, Close() will detach instead of stopping

	// RPC client for debug interface
	debugClient *lsvd_v1alpha.LsvdDebugClient
}

// NewComponent creates a new LSVD component
func NewComponent(log *slog.Logger, dataPath string) *Component {
	return &Component{
		log:      log.With("module", "lsvd"),
		olog:     log,
		dataPath: dataPath,
	}
}

// Start starts the lsvd-server as an outboard process
func (c *Component) Start(ctx context.Context, config *Config) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return fmt.Errorf("lsvd-server is already running")
	}

	c.config = config

	// Create data directory if it doesn't exist
	if err := os.MkdirAll(config.DataPath, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Create outboard directory for process management files
	if err := os.MkdirAll(config.OutboardPath, 0755); err != nil {
		return fmt.Errorf("failed to create outboard directory: %w", err)
	}

	// Get the path to the current executable (miren binary)
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Build command arguments for "miren internal lsvd"
	args := []string{
		"internal", "lsvd",
		"-vv",
		"--data-path", config.DataPath,
		"--node-id", config.NodeId,
		"--entity-server", config.EntityServerAddr,
		"--skip-verify",
	}

	connCfg := outboard.ConnectorConfig{
		Name:          "lsvd-server",
		BinaryPath:    execPath,
		Args:          args,
		Env:           config.Env,
		DataPath:      config.OutboardPath,
		RestartPolicy: outboard.DefaultRestartPolicy(),
		ReadyTimeout:  60 * time.Second,
	}

	conn := outboard.NewConnector(c.olog, connCfg)
	if err := conn.Start(ctx); err != nil {
		return fmt.Errorf("failed to start lsvd-server: %w", err)
	}

	c.connector = conn
	c.running = true

	c.log.Info("lsvd-server started via outboard connector",
		"data_path", config.DataPath,
		"node_id", config.NodeId,
	)

	// Connect to the debug RPC interface on the outboard process
	if err := c.connectDebugRPC(); err != nil {
		c.log.Warn("failed to connect to debug RPC", "error", err)
	}

	// Check if the running server needs to be upgraded
	upgraded, err := c.checkAndUpgradeVersion(ctx, config)
	if err != nil {
		c.log.Warn("failed to check version", "error", err)
	} else if upgraded {
		c.log.Info("lsvd-server was upgraded")
	}

	c.log.Info("lsvd-server ready", "data_path", config.DataPath)
	return nil
}

func (c *Component) connectDebugRPC() error {
	client, err := c.connector.ConnectInterface("lsvd-debug")
	if err != nil {
		return fmt.Errorf("failed to connect to lsvd-debug: %w", err)
	}

	c.debugClient = lsvd_v1alpha.NewLsvdDebugClient(client)
	c.log.Info("connected to lsvd-server debug RPC")
	return nil
}

// Stop stops the lsvd-server process
func (c *Component) Stop(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.debugClient = nil

	if c.connector != nil {
		if err := c.connector.Stop(ctx); err != nil {
			return err
		}
	}

	c.running = false
	return nil
}

// Close implements io.Closer for use in closers list.
// If restart mode is set, it detaches from the process instead of stopping it.
func (c *Component) Close() error {
	c.mu.Lock()
	restartMode := c.restartMode
	c.mu.Unlock()

	if restartMode {
		c.log.Info("restart mode - detaching from lsvd-server (keeping it running)")
		return c.Detach()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return c.Stop(ctx)
}

// SetRestartMode sets whether the component should detach (preserve process)
// instead of stopping when Close() is called.
func (c *Component) SetRestartMode(v bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.restartMode = v
}

// Detach disconnects from lsvd-server without stopping it.
// The process keeps running and can be reconnected later.
func (c *Component) Detach() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.debugClient = nil

	if c.connector != nil {
		if err := c.connector.Detach(); err != nil {
			return err
		}
	}

	c.running = false
	return nil
}

// StartOrReconnect tries to reconnect to an existing lsvd-server, falling back to start.
func (c *Component) StartOrReconnect(ctx context.Context, config *Config) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return fmt.Errorf("lsvd-server is already running")
	}

	c.config = config

	// Create directories
	if err := os.MkdirAll(config.DataPath, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}
	if err := os.MkdirAll(config.OutboardPath, 0755); err != nil {
		return fmt.Errorf("failed to create outboard directory: %w", err)
	}

	// Get the path to the current executable (miren binary)
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Build command arguments for "miren internal lsvd"
	args := []string{
		"internal", "lsvd",
		"-vv",
		"--data-path", config.DataPath,
		"--node-id", config.NodeId,
		"--entity-server", config.EntityServerAddr,
		"--skip-verify",
	}

	connCfg := outboard.ConnectorConfig{
		Name:          "lsvd-server",
		BinaryPath:    execPath,
		Args:          args,
		Env:           config.Env,
		DataPath:      config.OutboardPath,
		RestartPolicy: outboard.DefaultRestartPolicy(),
		ReadyTimeout:  60 * time.Second,
	}

	conn := outboard.NewConnector(c.olog, connCfg)

	// Try to reconnect to existing process first
	if err := conn.Reconnect(ctx); err == nil {
		c.connector = conn
		c.running = true
		c.log.Info("reconnected to existing lsvd-server",
			"data_path", config.DataPath,
			"node_id", config.NodeId,
		)

		// Reconnect debug RPC
		if err := c.connectDebugRPC(); err != nil {
			c.log.Warn("failed to connect debug RPC after reconnect", "error", err)
		}

		// Check if the running server needs to be upgraded
		upgraded, err := c.checkAndUpgradeVersion(ctx, config)
		if err != nil {
			c.log.Warn("failed to check version", "error", err)
		} else if upgraded {
			c.log.Info("lsvd-server was upgraded")
		}

		c.log.Info("lsvd-server ready", "data_path", config.DataPath)
		return nil
	} else {
		c.log.Info("no existing lsvd-server to reconnect, starting fresh", "reason", err)
	}

	// Start fresh
	if err := conn.Start(ctx); err != nil {
		return fmt.Errorf("failed to start lsvd-server: %w", err)
	}

	c.connector = conn
	c.running = true

	c.log.Info("lsvd-server started via outboard connector",
		"data_path", config.DataPath,
		"node_id", config.NodeId,
	)

	// Connect to the debug RPC interface
	if err := c.connectDebugRPC(); err != nil {
		c.log.Warn("failed to connect to debug RPC", "error", err)
	}

	// Check if the running server needs to be upgraded
	upgraded, err := c.checkAndUpgradeVersion(ctx, config)
	if err != nil {
		c.log.Warn("failed to check version", "error", err)
	} else if upgraded {
		c.log.Info("lsvd-server was upgraded")
	}

	c.log.Info("lsvd-server ready", "data_path", config.DataPath)
	return nil
}

// IsRunning returns whether lsvd-server is running
func (c *Component) IsRunning() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connector != nil {
		return c.connector.IsRunning()
	}
	return false
}

// PID returns the PID of the running lsvd-server process
func (c *Component) PID() (int, error) {
	c.mu.Lock()
	conn := c.connector
	c.mu.Unlock()

	if conn == nil {
		return 0, fmt.Errorf("lsvd-server is not running")
	}
	return conn.PID()
}

// RPCState returns the outboard RPC state for additional connections.
func (c *Component) RPCState() *rpc.State {
	c.mu.Lock()
	conn := c.connector
	c.mu.Unlock()

	if conn == nil {
		return nil
	}
	return conn.RPCState()
}

// checkAndUpgradeVersion checks if the running lsvd-server needs to be upgraded.
func (c *Component) checkAndUpgradeVersion(ctx context.Context, config *Config) (bool, error) {
	controlClient := c.connector.ControlClient()
	if controlClient == nil {
		return false, fmt.Errorf("outboard control client not connected")
	}

	result, err := controlClient.CheckVersion(ctx, server.ServerVersion)
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

	c.log.Info("lsvd-server needs upgrade, restarting")

	// Close debug RPC client
	c.debugClient = nil

	// Stop the connector (waits for process to exit)
	if err := c.connector.Stop(ctx); err != nil {
		return false, fmt.Errorf("failed to stop old version: %w", err)
	}
	c.running = false

	// Restart
	if err := c.connector.Start(ctx); err != nil {
		return false, fmt.Errorf("failed to start new version: %w", err)
	}
	c.running = true

	// Reconnect debug RPC
	if err := c.connectDebugRPC(); err != nil {
		c.log.Warn("failed to reconnect debug RPC after upgrade", "error", err)
	}

	return true, nil
}
