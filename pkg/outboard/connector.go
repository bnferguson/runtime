package outboard

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"miren.dev/runtime/api/outboard/outboard_v1alpha"
	"miren.dev/runtime/pkg/rpc"
)

// RestartPolicy configures automatic restart behavior.
type RestartPolicy struct {
	MaxRestarts int           // 0 = unlimited
	BackoffBase time.Duration // Base delay for exponential backoff
	BackoffMax  time.Duration // Maximum backoff delay
	ResetWindow time.Duration // Time after which restart count resets
}

// DefaultRestartPolicy returns a restart policy with unlimited restarts.
func DefaultRestartPolicy() RestartPolicy {
	return RestartPolicy{
		MaxRestarts: 0,
		BackoffBase: 2 * time.Second,
		BackoffMax:  60 * time.Second,
		ResetWindow: 5 * time.Minute,
	}
}

// ConnectorConfig configures how the connector manages the outboard process.
type ConnectorConfig struct {
	Name          string
	BinaryPath    string
	Args          []string
	Env           []string
	DataPath      string
	RestartPolicy RestartPolicy
	ReadyTimeout  time.Duration // default 60s
}

// Connector manages the lifecycle of an outboard process from the parent side.
// It handles starting, monitoring, restarting, and RPC connection to the child.
type Connector struct {
	log    *slog.Logger
	rlog   *slog.Logger
	cfg    ConnectorConfig
	mu     sync.Mutex
	cmd    *exec.Cmd
	cancel context.CancelFunc

	rpcState      *rpc.State
	controlClient *outboard_v1alpha.OutboardControlClient

	configPath string
	token      string
	running    bool

	restartCount int
	lastRestart  time.Time

	// Channels for coordinating shutdown
	exitCh chan struct{}
	stopCh chan struct{}

	// stopSignaled indicates a graceful stop was requested (guarded by mu)
	stopSignaled bool
}

// NewConnector creates a new Connector for managing an outboard process.
func NewConnector(log *slog.Logger, cfg ConnectorConfig) *Connector {
	if cfg.ReadyTimeout == 0 {
		cfg.ReadyTimeout = 60 * time.Second
	}
	// Apply defaults per-field to preserve caller-set values like MaxRestarts
	if cfg.RestartPolicy.BackoffBase == 0 {
		cfg.RestartPolicy.BackoffBase = 2 * time.Second
	}
	if cfg.RestartPolicy.BackoffMax == 0 {
		cfg.RestartPolicy.BackoffMax = 60 * time.Second
	}
	if cfg.RestartPolicy.ResetWindow == 0 {
		cfg.RestartPolicy.ResetWindow = 5 * time.Minute
	}

	return &Connector{
		log:  log.With("module", "outboard"),
		rlog: log,
		cfg:  cfg,
	}
}

// Start starts the outboard process and waits for it to become ready.
func (c *Connector) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return fmt.Errorf("outboard process %s is already running", c.cfg.Name)
	}

	return c.startLocked(ctx)
}

func (c *Connector) startLocked(ctx context.Context) error {
	// Generate random token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return fmt.Errorf("generating token: %w", err)
	}
	c.token = hex.EncodeToString(tokenBytes)

	// Create data directory
	if err := os.MkdirAll(c.cfg.DataPath, 0755); err != nil {
		return fmt.Errorf("creating data directory: %w", err)
	}

	// Create FIFOs
	stdoutFIFO := filepath.Join(c.cfg.DataPath, "stdout.fifo")
	stderrFIFO := filepath.Join(c.cfg.DataPath, "stderr.fifo")

	if err := createFIFO(stdoutFIFO); err != nil {
		return fmt.Errorf("creating stdout FIFO: %w", err)
	}
	if err := createFIFO(stderrFIFO); err != nil {
		os.Remove(stdoutFIFO)
		return fmt.Errorf("creating stderr FIFO: %w", err)
	}

	// Write config
	c.configPath = filepath.Join(c.cfg.DataPath, "outboard.json")
	cfg := &Config{
		Token:      c.token,
		FIFOStdout: stdoutFIFO,
		FIFOStderr: stderrFIFO,
	}
	if err := WriteConfig(c.configPath, cfg); err != nil {
		os.Remove(stdoutFIFO)
		os.Remove(stderrFIFO)
		return fmt.Errorf("writing outboard config: %w", err)
	}

	// Start FIFO forwarding goroutines
	stdoutDone := make(chan struct{})
	stderrDone := make(chan struct{})

	rlog := c.rlog.With("outboard", c.cfg.Name)

	go forwardFIFO(stdoutFIFO, rlog, stdoutDone)
	go forwardFIFO(stderrFIFO, rlog, stderrDone)

	// Open FIFOs for writing to connect as stdout/stderr of the process.
	// If either open fails, remove both FIFOs to unblock the forwardFIFO goroutines
	// which are blocked on os.Open waiting for a writer.
	stdoutW, err := os.OpenFile(stdoutFIFO, os.O_WRONLY, 0)
	if err != nil {
		os.Remove(stdoutFIFO) // Unblock forwardFIFO goroutine
		os.Remove(stderrFIFO) // Unblock forwardFIFO goroutine
		<-stdoutDone
		<-stderrDone
		return fmt.Errorf("opening stdout FIFO for writing: %w", err)
	}
	stderrW, err := os.OpenFile(stderrFIFO, os.O_WRONLY, 0)
	if err != nil {
		stdoutW.Close()
		os.Remove(stdoutFIFO) // Trigger EOF for forwardFIFO
		os.Remove(stderrFIFO) // Unblock forwardFIFO goroutine
		<-stdoutDone
		<-stderrDone
		return fmt.Errorf("opening stderr FIFO for writing: %w", err)
	}

	// Build command
	cmd := exec.Command(c.cfg.BinaryPath, c.cfg.Args...)
	cmd.Stdout = stdoutW
	cmd.Stderr = stderrW
	cmd.Env = append(os.Environ(), c.cfg.Env...)
	cmd.Env = append(cmd.Env, "OUTBOARD_CONFIG="+c.configPath)
	// Put child in its own process group so signals to parent don't kill it
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	c.log.Info("starting outboard process",
		"binary", c.cfg.BinaryPath,
		"args", c.cfg.Args,
	)

	if err := cmd.Start(); err != nil {
		stdoutW.Close()
		stderrW.Close()
		// Wait for FIFO goroutines to exit (they'll get EOF since we closed the writers)
		<-stdoutDone
		<-stderrDone
		os.Remove(stdoutFIFO)
		os.Remove(stderrFIFO)
		os.Remove(c.configPath)
		return fmt.Errorf("starting outboard process: %w", err)
	}

	c.cmd = cmd
	c.exitCh = make(chan struct{})
	c.stopCh = make(chan struct{})
	c.stopSignaled = false

	// Close our write ends of the FIFOs now that the child has inherited them.
	// The child writes to them via its stdout/stderr.
	stdoutW.Close()
	stderrW.Close()

	c.log.Info("outboard process started", "pid", cmd.Process.Pid)

	// Helper to clean up on late failures
	cleanupOnError := func() {
		cmd.Process.Kill()
		cmd.Wait() // This closes child's FDs, triggering EOF for FIFO goroutines
		<-stdoutDone
		<-stderrDone
		os.Remove(stdoutFIFO)
		os.Remove(stderrFIFO)
		os.Remove(c.configPath)
	}

	// Wait for ready
	if err := c.waitForReady(ctx); err != nil {
		cleanupOnError()
		return fmt.Errorf("waiting for outboard process to be ready: %w", err)
	}

	// Read back the updated config with RPC address
	updatedCfg, err := ReadConfig(c.configPath)
	if err != nil {
		cleanupOnError()
		return fmt.Errorf("reading updated config: %w", err)
	}

	// Connect RPC
	if err := c.connectRPC(ctx, updatedCfg.RPCAddr); err != nil {
		cleanupOnError()
		return fmt.Errorf("connecting to outboard RPC: %w", err)
	}

	c.running = true

	// Start exit monitor
	monitorCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	go c.monitorExit(monitorCtx, cmd, stdoutDone, stderrDone)

	return nil
}

func (c *Connector) waitForReady(ctx context.Context) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(c.cfg.ReadyTimeout)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("timeout waiting for outboard process %s to be ready", c.cfg.Name)
		case <-ticker.C:
			cfg, err := ReadConfig(c.configPath)
			if err != nil {
				continue
			}
			if cfg.Ready {
				return nil
			}
		}
	}
}

func (c *Connector) connectRPC(ctx context.Context, addr string) error {
	if addr == "" {
		return fmt.Errorf("empty RPC address")
	}

	rpcState, err := rpc.NewState(ctx,
		rpc.WithLogger(c.log),
		rpc.WithBearerToken(c.token),
		rpc.WithSkipVerify,
	)
	if err != nil {
		return fmt.Errorf("creating RPC state: %w", err)
	}

	client, err := rpcState.Connect(addr, "outboard-control")
	if err != nil {
		rpcState.Close()
		return fmt.Errorf("connecting to outboard-control: %w", err)
	}

	c.rpcState = rpcState
	c.controlClient = outboard_v1alpha.NewOutboardControlClient(client)

	c.log.Info("connected to outboard RPC", "addr", addr)
	return nil
}

func (c *Connector) monitorExit(ctx context.Context, cmd *exec.Cmd, stdoutDone, stderrDone chan struct{}) {
	// Wait for process to exit
	err := cmd.Wait()

	// Wait for FIFO readers to finish draining
	<-stdoutDone
	<-stderrDone

	c.mu.Lock()
	c.running = false
	close(c.exitCh)
	stopSignaled := c.stopSignaled
	// Close RPC state immediately on process exit to avoid leaks
	if c.rpcState != nil {
		c.rpcState.Close()
		c.rpcState = nil
		c.controlClient = nil
	}
	c.mu.Unlock()

	if stopSignaled {
		// Graceful stop requested, don't restart
		c.log.Info("outboard process stopped gracefully")
		return
	}

	if err != nil {
		c.log.Warn("outboard process exited unexpectedly", "error", err)
	} else {
		c.log.Warn("outboard process exited unexpectedly with status 0")
	}

	// Attempt restart with backoff
	c.mu.Lock()
	now := time.Now()
	if !c.lastRestart.IsZero() && now.Sub(c.lastRestart) > c.cfg.RestartPolicy.ResetWindow {
		c.restartCount = 0
	}

	maxRestarts := c.cfg.RestartPolicy.MaxRestarts
	if maxRestarts > 0 && c.restartCount >= maxRestarts {
		c.mu.Unlock()
		c.log.Error("outboard process exceeded max restarts", "max", maxRestarts)
		return
	}

	backoff := c.cfg.RestartPolicy.BackoffBase
	for i := 0; i < c.restartCount; i++ {
		backoff *= 2
		if backoff > c.cfg.RestartPolicy.BackoffMax {
			backoff = c.cfg.RestartPolicy.BackoffMax
			break
		}
	}

	c.restartCount++
	c.lastRestart = now
	c.mu.Unlock()

	c.log.Info("restarting outboard process", "backoff", backoff, "restart_count", c.restartCount)

	select {
	case <-ctx.Done():
		return
	case <-time.After(backoff):
	}

	c.mu.Lock()
	err = c.startLocked(ctx)
	c.mu.Unlock()

	if err != nil {
		c.log.Error("failed to restart outboard process", "error", err)
	}
}

// Stop gracefully stops the outboard process.
func (c *Connector) Stop(ctx context.Context) error {
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return nil
	}

	// Signal to monitor that this is a graceful stop
	c.stopSignaled = true
	if c.stopCh != nil {
		close(c.stopCh)
	}

	cmd := c.cmd
	c.mu.Unlock()

	if cmd != nil && cmd.Process != nil {
		cmd.Process.Signal(os.Interrupt)

		// Wait for exit with timeout
		select {
		case <-c.exitCh:
			// Process exited
		case <-time.After(10 * time.Second):
			c.log.Warn("outboard process did not exit in time, killing")
			cmd.Process.Kill()
			<-c.exitCh
		case <-ctx.Done():
			cmd.Process.Kill()
			<-c.exitCh
		}
	}

	return nil
}

// Close stops the process and cleans up resources.
func (c *Connector) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := c.Stop(ctx); err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cancel != nil {
		c.cancel()
	}

	if c.rpcState != nil {
		c.rpcState.Close()
		c.rpcState = nil
		c.controlClient = nil
	}

	return nil
}

// IsRunning returns whether the outboard process is currently running.
func (c *Connector) IsRunning() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}

// PID returns the PID of the running outboard process.
func (c *Connector) PID() (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running || c.cmd == nil || c.cmd.Process == nil {
		return 0, fmt.Errorf("outboard process %s is not running", c.cfg.Name)
	}

	return c.cmd.Process.Pid, nil
}

// RPCState returns the parent-side RPC state for connecting to additional
// interfaces exposed by the outboard process.
func (c *Connector) RPCState() *rpc.State {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.rpcState
}

// ControlClient returns the OutboardControl RPC client.
func (c *Connector) ControlClient() *outboard_v1alpha.OutboardControlClient {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.controlClient
}

// ConnectInterface connects to an additional RPC interface exposed by the outboard process.
func (c *Connector) ConnectInterface(name string) (*rpc.NetworkClient, error) {
	c.mu.Lock()
	rs := c.rpcState
	configPath := c.configPath // Copy while holding lock to avoid race
	c.mu.Unlock()

	if rs == nil {
		return nil, fmt.Errorf("RPC not connected")
	}

	cfg, err := ReadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("reading config for RPC address: %w", err)
	}

	return rs.Connect(cfg.RPCAddr, name)
}
