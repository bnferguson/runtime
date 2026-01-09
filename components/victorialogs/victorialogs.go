// Package victorialogs provides a component for managing a VictoriaLogs server using containerd.
// VictoriaLogs is a log storage system that uses LogsQL for querying.
package victorialogs

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/unix"
	"miren.dev/runtime/pkg/imagerefs"
	"miren.dev/runtime/pkg/slogout"
)

const (
	victoriaLogsContainerName = "miren-victorialogs"
	defaultHTTPPort           = 9428
)

var (
	victoriaLogsImage = imagerefs.VictoriaLogs
)

type VictoriaLogsConfig struct {
	HTTPPort        int
	DataPath        string
	RetentionPeriod string
}

type VictoriaLogsComponent struct {
	Log *slog.Logger
	CC  *containerd.Client

	Namespace string
	DataPath  string

	mu        sync.Mutex
	container containerd.Container
	task      containerd.Task
	running   bool
	httpPort  int

	// For exit monitoring and restart
	stopMonitor   chan struct{}
	monitorDone   chan struct{}
	config        VictoriaLogsConfig
	restartCount  int
	lastRestartAt time.Time
}

func NewVictoriaLogsComponent(log *slog.Logger, cc *containerd.Client, namespace, dataPath string) *VictoriaLogsComponent {
	return &VictoriaLogsComponent{
		Log:       log,
		CC:        cc,
		Namespace: namespace,
		DataPath:  dataPath,
	}
}

func (c *VictoriaLogsComponent) Start(ctx context.Context, config VictoriaLogsConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return fmt.Errorf("victorialogs component already running")
	}

	ctx = namespaces.WithNamespace(ctx, c.Namespace)

	c.Log.Info("pulling victorialogs image", "image", victoriaLogsImage)
	image, err := c.CC.Pull(ctx, victoriaLogsImage, containerd.WithPullUnpack)
	if err != nil {
		return fmt.Errorf("failed to pull victorialogs image: %w", err)
	}

	dataPath := filepath.Join(c.DataPath, "victorialogs")

	err = os.MkdirAll(dataPath, 0755)
	if err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Set defaults
	if config.HTTPPort == 0 {
		config.HTTPPort = defaultHTTPPort
	}
	if config.RetentionPeriod == "" {
		config.RetentionPeriod = "30d"
	}

	c.httpPort = config.HTTPPort

	// Check if container already exists
	existingContainer, err := c.CC.LoadContainer(ctx, victoriaLogsContainerName)
	if err == nil {
		c.Log.Info("found existing victorialogs container, attempting restart", "container_id", existingContainer.ID())
		err = c.restartExistingContainer(ctx, existingContainer, config)
		if err == nil {
			return nil
		}
		// If restart failed (e.g., port mismatch), try deleting the container and creating fresh
		c.Log.Warn("restart of existing container failed, recreating", "error", err)
		c.cleanupExistingContainer(ctx, existingContainer)
	}

	c.Log.Info("starting victorialogs with host networking", "http_port", config.HTTPPort)

	// Create container
	container, err := c.createContainer(ctx, image, dataPath, config)
	if err != nil {
		return fmt.Errorf("failed to create victorialogs container: %w", err)
	}

	c.container = container

	// Start container with structured logging
	task, err := container.NewTask(ctx, slogout.WithLogger(c.Log, "victorialogs"))
	if err != nil {
		container.Delete(ctx, containerd.WithSnapshotCleanup)
		return fmt.Errorf("failed to create victorialogs task: %w", err)
	}

	err = task.Start(ctx)
	if err != nil {
		task.Delete(ctx)
		container.Delete(ctx, containerd.WithSnapshotCleanup)
		return fmt.Errorf("failed to start victorialogs task: %w", err)
	}

	// Wait for VictoriaLogs to be ready
	if err := c.waitForReady(ctx, "localhost", config.HTTPPort); err != nil {
		task.Delete(ctx)
		container.Delete(ctx, containerd.WithSnapshotCleanup)
		return err
	}

	c.task = task
	c.running = true
	c.config = config
	c.Log.Info("victorialogs server started", "container_id", container.ID(), "http_port", config.HTTPPort)

	// Start monitoring for unexpected exits
	c.startExitMonitor(ctx)

	return nil
}

func (c *VictoriaLogsComponent) Stop(ctx context.Context) error {
	c.mu.Lock()

	if !c.running {
		c.mu.Unlock()
		return nil
	}

	// Stop the exit monitor first
	if c.stopMonitor != nil {
		close(c.stopMonitor)
		c.stopMonitor = nil
	}
	monitorDone := c.monitorDone
	c.mu.Unlock()

	// Wait for monitor to finish (outside the lock)
	if monitorDone != nil {
		<-monitorDone
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	ctx = namespaces.WithNamespace(ctx, c.Namespace)

	if c.task != nil {
		c.stopTask(ctx, c.task)
		c.task = nil
	}

	if c.container != nil {
		c.deleteContainerWithRetry(ctx)
		c.container = nil
	}

	c.running = false
	c.Log.Info("victorialogs server stopped")

	return nil
}

func (c *VictoriaLogsComponent) stopTask(ctx context.Context, task containerd.Task) {
	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := task.Kill(shutdownCtx, unix.SIGTERM); err != nil {
		c.Log.Error("failed to send SIGTERM to victorialogs task", "error", err)
		return
	}

	status, err := task.Wait(shutdownCtx)
	if err == nil {
		select {
		case es := <-status:
			c.Log.Info("victorialogs task exited", "code", es.ExitCode())

		case <-shutdownCtx.Done():
			c.Log.Warn("victorialogs task did not exit gracefully, sending SIGKILL")
			killCtx, killCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer killCancel()

			if err := task.Kill(killCtx, unix.SIGKILL); err != nil {
				c.Log.Error("failed to send SIGKILL to victorialogs task", "error", err)
			} else {
				if _, waitErr := task.Wait(killCtx); waitErr != nil {
					c.Log.Error("victorialogs task wait after SIGKILL failed", "error", waitErr)
				}
			}
		}
	}

	deleteCtx, deleteCancel := context.WithTimeout(ctx, 10*time.Second)
	defer deleteCancel()

	if _, err := task.Delete(deleteCtx); err != nil {
		c.Log.Error("failed to delete victorialogs task", "error", err)
	}
}

func (c *VictoriaLogsComponent) deleteContainerWithRetry(ctx context.Context) {
	const maxRetries = 3
	const retryDelay = 2 * time.Second

	for attempt := 1; attempt <= maxRetries; attempt++ {
		deleteCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		err := c.container.Delete(deleteCtx, containerd.WithSnapshotCleanup)
		cancel()

		if err == nil {
			c.Log.Info("victorialogs container deleted successfully")
			return
		}

		c.Log.Error("failed to delete victorialogs container", "error", err, "attempt", attempt, "max_retries", maxRetries)

		if attempt < maxRetries {
			time.Sleep(retryDelay)
		}
	}

	c.Log.Error("failed to delete victorialogs container after all retries, potential snapshot leak")
}

func (c *VictoriaLogsComponent) HTTPEndpoint() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.running {
		return ""
	}
	return fmt.Sprintf("localhost:%d", c.httpPort)
}

func (c *VictoriaLogsComponent) IsRunning() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}

func (c *VictoriaLogsComponent) cleanupExistingContainer(ctx context.Context, container containerd.Container) {
	task, err := container.Task(ctx, nil)
	if err == nil {
		c.stopTask(ctx, task)
	}

	deleteCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := container.Delete(deleteCtx, containerd.WithSnapshotCleanup); err != nil {
		c.Log.Warn("failed to delete existing container during cleanup", "error", err)
	}
}

func (c *VictoriaLogsComponent) restartExistingContainer(ctx context.Context, container containerd.Container, config VictoriaLogsConfig) error {
	c.container = container
	c.httpPort = config.HTTPPort
	c.config = config

	task, err := container.Task(ctx, nil)
	if err == nil {
		status, err := task.Status(ctx)
		if err != nil {
			c.Log.Warn("failed to get task status", "error", err)
		} else if status.Status == containerd.Running {
			c.Log.Info("victorialogs container is already running")
			c.task = task
			c.running = true
			if err := c.waitForReady(ctx, "localhost", config.HTTPPort); err != nil {
				return err
			}
			c.startExitMonitor(ctx)
			return nil
		}

		c.Log.Info("starting existing victorialogs task")
		err = task.Start(ctx)
		if err == nil {
			c.task = task
			c.running = true
			c.Log.Info("victorialogs server restarted", "container_id", container.ID(), "http_port", config.HTTPPort)
			if err := c.waitForReady(ctx, "localhost", config.HTTPPort); err != nil {
				return err
			}
			c.startExitMonitor(ctx)
			return nil
		}

		c.Log.Warn("failed to start existing task, deleting it", "error", err)
		task.Delete(ctx)
	}

	c.Log.Info("creating new task for existing container")
	task, err = container.NewTask(ctx, slogout.WithLogger(c.Log, "victorialogs"))
	if err != nil {
		return fmt.Errorf("failed to create new task for existing container: %w", err)
	}

	err = task.Start(ctx)
	if err != nil {
		task.Delete(ctx)
		return fmt.Errorf("failed to start new task for existing container: %w", err)
	}

	if err := c.waitForReady(ctx, "localhost", config.HTTPPort); err != nil {
		task.Delete(ctx)
		return err
	}

	c.task = task
	c.running = true
	c.Log.Info("victorialogs server restarted with new task", "container_id", container.ID(), "http_port", config.HTTPPort)

	// Start monitoring for unexpected exits
	c.startExitMonitor(ctx)

	return nil
}

func (c *VictoriaLogsComponent) createContainer(ctx context.Context, image containerd.Image, dataPath string, config VictoriaLogsConfig) (containerd.Container, error) {
	listenAddr := fmt.Sprintf(":%d", config.HTTPPort)

	opts := []oci.SpecOpts{
		oci.WithImageConfig(image),
		oci.WithHostNamespace(specs.NetworkNamespace),
		oci.WithProcessArgs(
			"/victoria-logs-prod",
			"-storageDataPath=/victoria-logs-data",
			"-retentionPeriod="+config.RetentionPeriod,
			"-httpListenAddr="+listenAddr,
			"-enableTCP6",
		),
		oci.WithHostHostsFile,
		oci.WithHostResolvconf,

		oci.WithMounts([]specs.Mount{
			{
				Destination: "/victoria-logs-data",
				Type:        "bind",
				Source:      dataPath,
				Options:     []string{"rbind", "rw"},
			},
		}),
	}

	container, err := c.CC.NewContainer(
		ctx,
		victoriaLogsContainerName,
		containerd.WithImage(image),
		containerd.WithNewSnapshot(victoriaLogsContainerName+"-snapshot", image),
		containerd.WithNewSpec(opts...),
	)
	if err != nil {
		return nil, err
	}

	return container, nil
}

func (c *VictoriaLogsComponent) waitForReady(ctx context.Context, host string, port int) error {
	endpoint := net.JoinHostPort(host, fmt.Sprintf("%d", port))

	for i := 0; i < 30; i++ {
		conn, err := net.DialTimeout("tcp", endpoint, 2*time.Second)
		if err == nil {
			conn.Close()
			c.Log.Info("victorialogs server is ready", "endpoint", endpoint)
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
			continue
		}
	}

	c.Log.Warn("victorialogs server readiness check timed out", "endpoint", endpoint)
	return fmt.Errorf("victorialogs readiness check timed out after 30 seconds")
}

func (c *VictoriaLogsComponent) startExitMonitor(ctx context.Context) {
	c.stopMonitor = make(chan struct{})
	c.monitorDone = make(chan struct{})

	ctx = namespaces.WithNamespace(ctx, c.Namespace)

	exitCh, err := c.task.Wait(ctx)
	if err != nil {
		c.Log.Error("failed to get exit channel for victorialogs task", "error", err)
		close(c.monitorDone)
		return
	}

	go c.monitorExit(ctx, exitCh)
}

func (c *VictoriaLogsComponent) monitorExit(ctx context.Context, exitCh <-chan containerd.ExitStatus) {
	defer close(c.monitorDone)

	for {
		select {
		case <-c.stopMonitor:
			c.Log.Debug("victorialogs exit monitor stopped")
			return

		case exitStatus := <-exitCh:
			c.Log.Warn("victorialogs process exited unexpectedly",
				"exit_code", exitStatus.ExitCode(),
				"exit_time", exitStatus.ExitTime(),
			)

			// Attempt restart
			if err := c.handleRestart(ctx); err != nil {
				c.Log.Error("failed to restart victorialogs", "error", err)
				return
			}

			// Get new exit channel for the restarted task
			c.mu.Lock()
			if c.task == nil {
				c.mu.Unlock()
				return
			}
			newExitCh, err := c.task.Wait(ctx)
			c.mu.Unlock()
			if err != nil {
				c.Log.Error("failed to get exit channel after restart", "error", err)
				return
			}
			exitCh = newExitCh
		}
	}
}

func (c *VictoriaLogsComponent) handleRestart(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Apply exponential backoff for restarts
	const maxRestarts = 5
	const backoffBase = 2 * time.Second
	const backoffMax = 60 * time.Second
	const resetWindow = 5 * time.Minute

	// Reset restart count if enough time has passed
	if time.Since(c.lastRestartAt) > resetWindow {
		c.restartCount = 0
	}

	c.restartCount++
	if c.restartCount > maxRestarts {
		return fmt.Errorf("exceeded maximum restart attempts (%d)", maxRestarts)
	}

	// Calculate backoff
	backoff := backoffBase * time.Duration(1<<(c.restartCount-1))
	if backoff > backoffMax {
		backoff = backoffMax
	}

	c.Log.Info("restarting victorialogs after backoff",
		"restart_count", c.restartCount,
		"backoff", backoff,
	)

	select {
	case <-time.After(backoff):
	case <-c.stopMonitor:
		return fmt.Errorf("restart cancelled")
	}

	c.lastRestartAt = time.Now()

	ctx = namespaces.WithNamespace(ctx, c.Namespace)

	// Clean up old task
	if c.task != nil {
		deleteCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		c.task.Delete(deleteCtx)
		cancel()
		c.task = nil
	}

	// Create and start new task
	task, err := c.container.NewTask(ctx, slogout.WithLogger(c.Log, "victorialogs"))
	if err != nil {
		return fmt.Errorf("failed to create new victorialogs task: %w", err)
	}

	if err := task.Start(ctx); err != nil {
		task.Delete(ctx)
		return fmt.Errorf("failed to start new victorialogs task: %w", err)
	}

	c.task = task

	// Wait for victorialogs to be ready
	if err := c.waitForReady(ctx, "localhost", c.config.HTTPPort); err != nil {
		c.Log.Warn("victorialogs readiness check failed after restart", "error", err)
	}

	c.Log.Info("victorialogs restarted successfully", "restart_count", c.restartCount)
	return nil
}
