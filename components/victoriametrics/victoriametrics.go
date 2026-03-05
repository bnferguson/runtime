// Package victoriametrics provides a component for managing a VictoriaMetrics server using containerd.
// VictoriaMetrics is a metrics storage system that uses MetricsQL for querying.
package victoriametrics

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/opencontainers/runtime-spec/specs-go"
	"miren.dev/runtime/components/base"
	"miren.dev/runtime/pkg/containerdx"
	"miren.dev/runtime/pkg/imagerefs"
	"miren.dev/runtime/pkg/slogout"
)

const (
	victoriaMetricsContainerName = "miren-victoriametrics"
	defaultHTTPPort              = 8428
)

var (
	victoriaMetricsImage = imagerefs.VictoriaMetrics
)

type VictoriaMetricsConfig struct {
	HTTPPort        int
	DataPath        string
	RetentionPeriod string
}

type VictoriaMetricsComponent struct {
	*base.BaseComponent

	httpPort int
	config   VictoriaMetricsConfig
}

func NewVictoriaMetricsComponent(log *slog.Logger, cc *containerd.Client, namespace, dataPath string) *VictoriaMetricsComponent {
	bc := base.NewBaseComponent(log, cc, namespace, dataPath, "victoriametrics")

	c := &VictoriaMetricsComponent{
		BaseComponent: bc,
	}

	// Set up callbacks for the base component
	bc.CreateTask = c.createTask
	bc.GetReadyPort = c.getReadyPort

	return c
}

func (c *VictoriaMetricsComponent) createTask(ctx context.Context, container containerd.Container) (containerd.Task, error) {
	return container.NewTask(ctx, slogout.WithLogger(c.Log, "victoriametrics"))
}

func (c *VictoriaMetricsComponent) getReadyPort() int {
	return c.httpPort
}

func (c *VictoriaMetricsComponent) Start(ctx context.Context, config VictoriaMetricsConfig) error {
	c.LockOp()
	defer c.UnlockOp()

	if c.IsRunning() {
		return fmt.Errorf("victoriametrics component already running")
	}

	ctx = namespaces.WithNamespace(ctx, c.Namespace)

	c.Log.Info("pulling victoriametrics image", "image", victoriaMetricsImage)
	image, err := c.CC.Pull(ctx, victoriaMetricsImage, containerd.WithPullUnpack)
	if err != nil {
		return fmt.Errorf("failed to pull victoriametrics image: %w", err)
	}

	dataPath := filepath.Join(c.DataPath, "victoriametrics")

	err = os.MkdirAll(dataPath, 0755)
	if err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Set defaults
	if config.HTTPPort == 0 {
		config.HTTPPort = defaultHTTPPort
	}
	if config.RetentionPeriod == "" {
		config.RetentionPeriod = "1"
	}

	c.httpPort = config.HTTPPort
	c.config = config

	// Check if container already exists
	existingContainer, err := c.CC.LoadContainer(ctx, victoriaMetricsContainerName)
	if err == nil {
		c.Log.Info("found existing victoriametrics container, attempting restart", "container_id", existingContainer.ID())
		err = c.restartExistingContainer(ctx, existingContainer, config)
		if err == nil {
			return nil
		}
		// If restart failed, try deleting the container and creating fresh
		c.Log.Warn("restart of existing container failed, recreating", "error", err)
		c.CleanupExistingContainer(ctx, existingContainer)
	}

	c.Log.Info("starting victoriametrics with host networking", "http_port", config.HTTPPort)

	// Create container
	container, err := c.createContainer(ctx, image, dataPath, config)
	if err != nil {
		return fmt.Errorf("failed to create victoriametrics container: %w", err)
	}

	c.SetContainer(container)

	// Start container with structured logging
	task, err := c.createTask(ctx, container)
	if err != nil {
		container.Delete(ctx, containerd.WithSnapshotCleanup)
		return fmt.Errorf("failed to create victoriametrics task: %w", err)
	}

	err = task.Start(ctx)
	if err != nil {
		task.Delete(ctx)
		container.Delete(ctx, containerd.WithSnapshotCleanup)
		return fmt.Errorf("failed to start victoriametrics task: %w", err)
	}

	// Wait for VictoriaMetrics to be ready
	if err := c.WaitForReady(ctx, "localhost", config.HTTPPort); err != nil {
		task.Delete(ctx)
		container.Delete(ctx, containerd.WithSnapshotCleanup)
		return err
	}

	c.SetTask(task)
	c.Log.Info("victoriametrics server started", "container_id", container.ID(), "http_port", config.HTTPPort)

	// Start monitoring for unexpected exits
	c.StartExitMonitor(ctx)

	return nil
}

func (c *VictoriaMetricsComponent) HTTPEndpoint() string {
	return c.IfRunning(func() string {
		return fmt.Sprintf("localhost:%d", c.httpPort)
	})
}

func (c *VictoriaMetricsComponent) restartExistingContainer(ctx context.Context, container containerd.Container, config VictoriaMetricsConfig) error {
	c.SetContainer(container)
	c.httpPort = config.HTTPPort
	c.config = config

	task, err := container.Task(ctx, nil)
	if err == nil {
		status, err := task.Status(ctx)
		if err != nil {
			c.Log.Warn("failed to get task status", "error", err)
		} else if status.Status == containerd.Running {
			c.Log.Info("victoriametrics container is already running")
			c.SetTask(task)
			if err := c.WaitForReady(ctx, "localhost", config.HTTPPort); err != nil {
				return err
			}
			c.StartExitMonitor(ctx)
			return nil
		}

		c.Log.Info("starting existing victoriametrics task")
		err = task.Start(ctx)
		if err == nil {
			c.SetTask(task)
			c.Log.Info("victoriametrics server restarted", "container_id", container.ID(), "http_port", config.HTTPPort)
			if err := c.WaitForReady(ctx, "localhost", config.HTTPPort); err != nil {
				return err
			}
			c.StartExitMonitor(ctx)
			return nil
		}

		c.Log.Warn("failed to start existing task, deleting it", "error", err)
		task.Delete(ctx)
	}

	c.Log.Info("creating new task for existing container")
	task, err = c.createTask(ctx, container)
	if err != nil {
		return fmt.Errorf("failed to create new task for existing container: %w", err)
	}

	err = task.Start(ctx)
	if err != nil {
		task.Delete(ctx)
		return fmt.Errorf("failed to start new task for existing container: %w", err)
	}

	if err := c.WaitForReady(ctx, "localhost", config.HTTPPort); err != nil {
		task.Delete(ctx)
		return err
	}

	c.SetTask(task)
	c.Log.Info("victoriametrics server restarted with new task", "container_id", container.ID(), "http_port", config.HTTPPort)

	// Start monitoring for unexpected exits
	c.StartExitMonitor(ctx)

	return nil
}

func (c *VictoriaMetricsComponent) createContainer(ctx context.Context, image containerd.Image, dataPath string, config VictoriaMetricsConfig) (containerd.Container, error) {
	listenAddr := fmt.Sprintf(":%d", config.HTTPPort)

	opts := []oci.SpecOpts{
		oci.WithImageConfig(image),
		oci.WithHostNamespace(specs.NetworkNamespace),
		oci.WithProcessArgs(
			"/victoria-metrics-prod",
			"-storageDataPath=/victoria-metrics-data",
			"-retentionPeriod="+config.RetentionPeriod,
			"-httpListenAddr="+listenAddr,
			"-search.latencyOffset=2s",
			"-enableTCP6",
		),
		oci.WithHostHostsFile,
		oci.WithHostResolvconf,
		containerdx.WithRlimitNOFILE(65536),

		oci.WithMounts([]specs.Mount{
			{
				Destination: "/victoria-metrics-data",
				Type:        "bind",
				Source:      dataPath,
				Options:     []string{"rbind", "rw"},
			},
		}),
	}

	container, err := c.CC.NewContainer(
		ctx,
		victoriaMetricsContainerName,
		containerd.WithImage(image),
		containerd.WithNewSnapshot(victoriaMetricsContainerName+"-snapshot", image),
		containerd.WithNewSpec(opts...),
	)
	if err != nil {
		return nil, err
	}

	return container, nil
}
