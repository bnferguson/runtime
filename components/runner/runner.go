package runner

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	"golang.org/x/sync/errgroup"
	"miren.dev/runtime/api/compute/compute_v1alpha"
	"miren.dev/runtime/api/core/core_v1alpha"
	"miren.dev/runtime/api/entityserver"
	es "miren.dev/runtime/api/entityserver/entityserver_v1alpha"
	"miren.dev/runtime/api/exec/exec_v1alpha"
	"miren.dev/runtime/api/ingress/ingress_v1alpha"
	"miren.dev/runtime/api/metric/metric_v1alpha"
	"miren.dev/runtime/api/network/network_v1alpha"
	"miren.dev/runtime/api/storage/storage_v1alpha"
	"miren.dev/runtime/clientconfig"
	"miren.dev/runtime/components/coordinate"
	"miren.dev/runtime/components/diskio"
	"miren.dev/runtime/components/netresolve"
	"miren.dev/runtime/controllers/disk"
	"miren.dev/runtime/controllers/ingress"
	"miren.dev/runtime/controllers/sandbox"
	"miren.dev/runtime/controllers/service"
	"miren.dev/runtime/network"
	"miren.dev/runtime/observability"
	"miren.dev/runtime/pkg/cloudauth"
	"miren.dev/runtime/pkg/controller"
	"miren.dev/runtime/pkg/entity"
	"miren.dev/runtime/pkg/entity/types"
	"miren.dev/runtime/pkg/grunge"
	"miren.dev/runtime/pkg/multierror"
	"miren.dev/runtime/pkg/netdb"
	"miren.dev/runtime/pkg/rpc"
	"miren.dev/runtime/servers/exec"
)

type RunnerConfig struct {
	Id            string `json:"id" cbor:"id" yaml:"id"`
	ListenAddress string `json:"listen_address" cbor:"listen_address" yaml:"listen_address"`
	Workers       int    `json:"workers" cbor:"workers" yaml:"workers"`
	DataPath      string `json:"data_path" cbor:"data_path" yaml:"data_path"`

	// Optional RPC configuration for advanced setups
	// If not provided, a default insecure connection will be used
	// to connect to the server address.
	Config *clientconfig.Config `json:"config" cbor:"config" yaml:"config"`

	// Optional cloud authentication configuration for disk replication
	CloudAuth *coordinate.CloudAuthConfig `json:"cloud_auth,omitempty" cbor:"cloud_auth,omitempty" yaml:"cloud_auth,omitempty"`
}

// RunnerDeps holds dependencies needed by the Runner to construct controllers.
type RunnerDeps struct {
	CC        *containerd.Client
	Namespace string
	Bridge    string
	Tempdir   string
	Subnet    *netdb.Subnet

	// Network dependencies
	NetServ *network.ServiceManager

	// Observability dependencies
	LogsMaintainer *observability.LogsMaintainer
	LogWriter      observability.LogWriter
	StatusMon      *observability.StatusMonitor

	// Network config
	IPv4Routable    netip.Prefix
	ServicePrefixes []netip.Prefix
	DisableLocalNet bool

	// Resolver
	Resolver netresolve.Resolver

	// Sandbox metrics
	SandboxMetrics *sandbox.Metrics

	// IsCoordinator indicates this runner is the coordinator node.
	// Affects scheduling: stateful sandboxes are routed to the coordinator.
	IsCoordinator bool

	// Flannel network configuration (for distributed runners)
	// If EtcdEndpoints is non-empty, the runner will join the Flannel network
	EtcdEndpoints  []string
	EtcdPrefix     string
	NetworkBackend string

	// TLS configuration for etcd mTLS (for distributed runners)
	EtcdTLSCert   []byte // Client certificate PEM
	EtcdTLSKey    []byte // Client private key PEM
	EtcdTLSCACert []byte // CA certificate PEM
}

const (
	DefaulWorkers = 3
)

type shutdownCloser struct{ s interface{ Shutdown() } }

func (c shutdownCloser) Close() error { c.s.Shutdown(); return nil }

type waitCloser struct{ s interface{ Wait() } }

func (c waitCloser) Close() error { c.s.Wait(); return nil }

func NewRunner(log *slog.Logger, deps RunnerDeps, cfg RunnerConfig) (*Runner, error) {
	if cfg.DataPath == "" {
		return nil, fmt.Errorf("data_path is required")
	}

	if cfg.Id == "" {
		return nil, fmt.Errorf("id is required")
	}

	if deps.CC == nil {
		return nil, fmt.Errorf("containerd client is required")
	}

	return &Runner{
		RunnerConfig: cfg,
		Log:          log.With("module", "runner"),
		deps:         deps,
	}, nil
}

type Runner struct {
	RunnerConfig

	Log *slog.Logger

	deps RunnerDeps

	cc *containerd.Client

	ec *entityserver.Client
	se *entityserver.Session

	closers []io.Closer

	namespace string

	sbController *sandbox.SandboxController

	// Disk controllers, stored for SetRestartMode propagation
	dvc *diskio.DiskVolumeController
	dmc *diskio.DiskMountController
}

func (r *Runner) Close() error {
	var err error

	for _, c := range r.closers {
		xerr := c.Close()
		if xerr != nil {
			err = multierror.Append(err, xerr)
		}
	}

	return err
}

// SetRestartMode sets whether outboard processes should be preserved when closing.
// When true, disk mounts are left in place so the replacement process can pick them up.
func (r *Runner) SetRestartMode(v bool) {
	if r.dvc != nil {
		r.dvc.SetKeepMounts(v)
	}
	if r.dmc != nil {
		r.dmc.SetKeepMounts(v)
	}
}

// Drain sets the runner's node status to disabled and stops all running sandboxes
func (r *Runner) Drain(ctx context.Context) error {
	if r.ec == nil || r.Id == "" {
		return fmt.Errorf("runner not initialized with entity client")
	}

	r.Log.Info("draining runner", "id", r.Id)

	// Set node status to disabled
	r.Log.Info("setting node status to disabled", "id", r.Id)
	err := r.ec.UpdateAttrs(ctx, entity.Id(r.Id), (&compute_v1alpha.Node{
		Status: compute_v1alpha.DISABLED,
	}).Encode)
	if err != nil {
		return fmt.Errorf("failed to set node status to disabled: %w", err)
	}

	r.Log.Info("node status set to disabled", "id", r.Id)

	// List all sandboxes scheduled to this node
	idx := compute_v1alpha.Index(compute_v1alpha.KindSandbox, entity.Id("node/"+r.Id))
	results, err := r.ec.List(ctx, idx)
	if err != nil {
		return fmt.Errorf("failed to query sandboxes on node: %w", err)
	}

	sandboxCount := results.Length()
	r.Log.Info("found sandboxes to drain", "count", sandboxCount, "node", r.Id)

	// Stop each sandbox
	var drainErr error
	stoppedCount := 0
	for results.Next() {
		md := results.Metadata()
		if md == nil {
			continue
		}

		r.Log.Info("stopping sandbox", "id", md.ID)
		err := r.sbController.Delete(ctx, md.ID, nil)
		if err != nil {
			r.Log.Error("failed to stop sandbox", "id", md.ID, "error", err)
			drainErr = multierror.Append(drainErr, fmt.Errorf("failed to stop sandbox %s: %w", md.ID, err))
		} else {
			r.Log.Info("stopped sandbox", "id", md.ID)
			stoppedCount++
		}
	}

	if drainErr != nil {
		return fmt.Errorf("errors during drain: %w", drainErr)
	}

	r.Log.Info("runner drained successfully", "id", r.Id, "sandboxes_stopped", stoppedCount)
	return nil
}

func (r *Runner) ContainerdNamespace() string {
	return r.namespace
}

// stopUserContainers stops all running user workload containers so their file
// descriptors to disk images are released before migration. Containers with
// IDs starting with "miren-" are infrastructure (etcd, etc.) and are left
// running. The sandbox controller will recreate stopped containers during its
// subsequent reconciliation.
func (r *Runner) stopUserContainers(ctx context.Context, log *slog.Logger) {
	cc := r.deps.CC
	containers, err := cc.Containers(ctx)
	if err != nil {
		log.Warn("failed to list containers for stop", "error", err)
		return
	}

	// Collect tasks to stop and set up wait channels before sending signals
	type pendingStop struct {
		id     string
		task   containerd.Task
		exitCh <-chan containerd.ExitStatus
	}

	var pending []pendingStop
	for _, c := range containers {
		if strings.HasPrefix(c.ID(), "miren-") {
			continue
		}

		task, err := c.Task(ctx, nil)
		if err != nil {
			continue
		}

		status, err := task.Status(ctx)
		if err != nil || status.Status == containerd.Stopped {
			continue
		}

		exitCh, err := task.Wait(ctx)
		if err != nil {
			continue
		}

		log.Info("sending SIGTERM to container for disk migration", "id", c.ID())
		if err := task.Kill(ctx, syscall.SIGTERM); err != nil {
			log.Warn("failed to send SIGTERM, migration will proceed anyway", "id", c.ID(), "error", err)
			continue
		}

		pending = append(pending, pendingStop{id: c.ID(), task: task, exitCh: exitCh})
	}

	if len(pending) == 0 {
		return
	}

	// Wait for all containers in parallel with a shared 10s deadline
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()

	remaining := make(map[int]struct{})
	for i := range pending {
		remaining[i] = struct{}{}
	}

	for len(remaining) > 0 {
		select {
		case <-timer.C:
			// Timeout — SIGKILL everything still running
			for i := range remaining {
				p := &pending[i]
				log.Warn("container did not exit in 10s, sending SIGKILL", "id", p.id)
				_ = p.task.Kill(ctx, syscall.SIGKILL)
			}
			// Wait briefly for SIGKILL to take effect
			for i := range remaining {
				p := &pending[i]
				killCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
				select {
				case <-p.exitCh:
				case <-killCtx.Done():
				}
				cancel()
				if _, err := p.task.Delete(ctx); err != nil {
					log.Warn("failed to delete task after kill", "id", p.id, "error", err)
				}
			}
			remaining = nil
		case <-ctx.Done():
			return
		default:
			// Poll all remaining exit channels without blocking
			for i := range remaining {
				select {
				case <-pending[i].exitCh:
					if _, err := pending[i].task.Delete(ctx); err != nil {
						log.Warn("failed to delete task after stop", "id", pending[i].id, "error", err)
					}
					delete(remaining, i)
				default:
				}
			}
			if len(remaining) > 0 {
				time.Sleep(50 * time.Millisecond)
			}
		}
	}

	log.Info("stopped containers for disk migration", "count", len(pending))
}

func (r *Runner) ContainerdContainerForSandbox(ctx context.Context, id entity.Id) (containerd.Container, error) {
	cl, err := r.cc.ContainerService().List(ctx)
	if err != nil {
		return nil, err
	}

	for _, c := range cl {
		if c.Labels["runtime.computer/entity-id"] == string(id) {
			return r.cc.LoadContainer(ctx, c.ID)
		}
	}

	return nil, nil
}

// Start initializes and starts the runner.
// The optional errgroup parameter is used for running background tasks like the Flannel network.
// If eg is nil and the runner needs to join a Flannel network, an internal errgroup will be created.
func (r *Runner) Start(ctx context.Context, eg ...*errgroup.Group) error {
	r.Log.Info("Starting runner", "id", r.Id)

	// Initialize Flannel/WireGuard network if distributed runner configuration is provided
	if len(r.deps.EtcdEndpoints) > 0 {
		if err := r.initializeNetwork(ctx, eg...); err != nil {
			return fmt.Errorf("failed to initialize network: %w", err)
		}
	}

	var (
		rs     *rpc.State
		err    error
		client *rpc.NetworkClient
	)

	if r.Config == nil {
		rs, err = rpc.NewState(ctx, rpc.WithLogger(r.Log), rpc.WithBindAddr(r.ListenAddress), rpc.WithSkipVerify)
		if err != nil {
			return err
		}

		client, err = rs.Connect("", "entities")
		if err != nil {
			return err
		}
	} else {
		rs, err = r.Config.State(ctx, rpc.WithLogger(r.Log), rpc.WithBindAddr(r.ListenAddress))
		if err != nil {
			return err
		}

		client, err = rs.Client("entities")
		if err != nil {
			return err
		}
	}

	eas := es.NewEntityAccessClient(client)

	ec := entityserver.NewClient(r.Log, eas)

	cm, err := r.SetupControllers(ctx, eas, rs.Server())
	if err != nil {
		return err
	}

	err = r.setupEntity(ctx, ec)
	if err != nil {
		return err
	}

	// Create exec server with explicit dependencies
	execServer := exec.NewServer(r.Log, r.deps.CC, eas, r.deps.Namespace)

	rs.Server().ExposeValue("dev.miren.runtime/exec", exec_v1alpha.AdaptSandboxExec(execServer))

	r.Log.Info("Registered exec server")

	err = cm.Start(ctx)
	if err != nil {
		return err
	}

	r.Log.Info("Runner running", "id", r.Id)

	return nil
}

// initializeNetwork sets up the Flannel network for distributed runners.
// This is only called when EtcdEndpoints are configured (distributed runner mode).
func (r *Runner) initializeNetwork(ctx context.Context, eg ...*errgroup.Group) error {
	r.Log.Info("Initializing distributed runner network",
		"etcd_endpoints", r.deps.EtcdEndpoints,
		"etcd_prefix", r.deps.EtcdPrefix,
		"backend", r.deps.NetworkBackend)

	grungeOpts := grunge.NetworkOptions{
		EtcdEndpoints: r.deps.EtcdEndpoints,
		EtcdPrefix:    r.deps.EtcdPrefix,
		BackendType:   r.deps.NetworkBackend,
		PrevIPv4:      r.deps.IPv4Routable,
	}

	// Add TLS config if provided
	if r.deps.EtcdTLSCert != nil && r.deps.EtcdTLSKey != nil && r.deps.EtcdTLSCACert != nil {
		grungeOpts.TLSCert = r.deps.EtcdTLSCert
		grungeOpts.TLSKey = r.deps.EtcdTLSKey
		grungeOpts.TLSCACert = r.deps.EtcdTLSCACert
	}

	gn, err := grunge.NewNetwork(r.Log, grungeOpts)
	if err != nil {
		return fmt.Errorf("failed to create grunge network: %w", err)
	}

	// Get or create an errgroup for running the network
	var runGroup *errgroup.Group
	localGroup := false
	if len(eg) > 0 && eg[0] != nil {
		runGroup = eg[0]
	} else {
		runGroup, ctx = errgroup.WithContext(ctx)
		localGroup = true
	}

	// Start the network (joins the Flannel mesh, doesn't setup config - coordinator did that)
	if err := gn.Start(ctx, runGroup); err != nil {
		return fmt.Errorf("failed to start grunge network: %w", err)
	}

	// If we created a local errgroup, monitor it so errors aren't silently lost
	if localGroup {
		go func() {
			if err := runGroup.Wait(); err != nil {
				r.Log.Error("network errgroup failed", "error", err)
			}
		}()
	}

	// Update deps with the leased IP
	lease := gn.Lease()
	r.deps.IPv4Routable = lease.IPv4()

	r.Log.Info("Joined Flannel network", "ipv4", lease.IPv4().String())

	return nil
}

func (r *Runner) setupEntity(ctx context.Context, ec *entityserver.Client) error {
	if r.Id == "" {
		return nil
	}

	sess, ec, err := ec.NewSession(ctx, "runner health")
	if err != nil {
		return err
	}

	r.ec = ec
	r.se = sess

	role := "runner"
	if r.deps.IsCoordinator {
		role = "coordinator"
	}

	node := compute_v1alpha.Node{
		Constraints: types.LabelSet("compute", "generic", "role", role),
		ApiAddress:  r.ListenAddress,
	}

	res, err := ec.CreateOrUpdate(ctx, r.Id, &node)
	if err != nil {
		return err
	}

	err = ec.UpdateAttrs(ctx, res, (&compute_v1alpha.Node{
		Status: compute_v1alpha.READY,
	}).Encode)
	if err != nil {
		return err
	}

	r.Log.Info("Registered runner", "id", res)

	return nil
}

func (r *Runner) SetupControllers(
	ctx context.Context,
	eas *es.EntityAccessClient,
	rs *rpc.Server,
) (
	_ *controller.ControllerManager,
	retErr error,
) {
	cm := controller.NewControllerManager()

	// Create sandbox controller with explicit dependencies
	sbc, err := sandbox.NewSandboxController(sandbox.SandboxControllerDeps{
		Log:            r.Log,
		CC:             r.deps.CC,
		EAC:            eas,
		Namespace:      r.deps.Namespace,
		NodeId:         r.Id,
		NetServ:        r.deps.NetServ,
		Bridge:         r.deps.Bridge,
		Subnet:         r.deps.Subnet,
		DataPath:       r.DataPath,
		Tempdir:        r.deps.Tempdir,
		LogsMaintainer: r.deps.LogsMaintainer,
		LogWriter:      r.deps.LogWriter,
		StatusMon:      r.deps.StatusMon,
		Resolver:       r.deps.Resolver,
		Metrics:        r.deps.SandboxMetrics,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create sandbox controller: %w", err)
	}

	r.closers = append(r.closers, sbc)

	rs.ExposeValue("dev.miren.runtime/sandbox.metrics", metric_v1alpha.AdaptSandboxMetrics(sbc.Metrics))

	// Create service controller with explicit dependencies
	serviceController, err := service.NewServiceController(service.ServiceControllerDeps{
		Log:             r.Log,
		EAC:             eas,
		IPv4Routable:    r.deps.IPv4Routable,
		ServicePrefixes: r.deps.ServicePrefixes,
		DisableLocalNet: r.deps.DisableLocalNet,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create service controller: %w", err)
	}

	log := r.Log

	defaultRouteAppController := ingress.NewDefaultRouteAppController(log, eas)
	defaultRouteController := ingress.NewDefaultRouteController(log, eas)

	workers := r.Workers
	if workers <= 0 {
		workers = DefaulWorkers
	}

	// Stop any orphaned lsvd-server process left over from a previous version.
	// Before universal mode, the lsvd-server ran as an outboard process; during
	// upgrade it must be stopped since the new code no longer manages it.
	stopOrphanedLSVDServer(log, r.DataPath)

	// Initialize disk I/O controllers for universal mode (loop devices)
	dataPath := filepath.Join(r.DataPath, "disk-data")
	err = os.MkdirAll(dataPath, 0700)
	if err != nil {
		return nil, fmt.Errorf("failed to create disk data path: %w", err)
	}

	// Ensure loop device nodes exist (they may be missing in containers)
	if err := diskio.EnsureLoopDevices(log); err != nil {
		log.Warn("Loop devices not available, disk mounts will fail", "error", err)
	}

	// Try to set up lbd devices for accelerator mode
	if err := diskio.EnsureLbdDevices(log); err != nil {
		log.Info("lbd devices not available, accelerator mode will not work", "error", err)
	}

	diskioState, err := diskio.LoadState(dataPath)
	if err != nil {
		log.Warn("failed to load disk state, starting fresh", "error", err)
		diskioState = diskio.NewState()
		diskioState.SetPath(dataPath)
	}

	volOps := diskio.NewRealDiskVolumeOps(log)
	mntOps := diskio.NewRealDiskMountOps(log)

	r.dvc = diskio.NewDiskVolumeController(log, dataPath, r.Id, diskioState, volOps, mntOps)
	r.dvc.SetEAC(eas)

	if err := r.dvc.Init(ctx); err != nil {
		return nil, fmt.Errorf("disk volume controller init: %w", err)
	}

	// If LSVD migration is pending, stop user containers so their file
	// descriptors to disk images are released before migration. The sandbox
	// controller will recreate them during its subsequent reconciliation.
	if r.dvc.HasPendingMigration(ctx) {
		r.stopUserContainers(ctx, log)
	}

	// Reconcile volumes with entity server on startup to re-mount any
	// universal mode volumes that were mounted before the last shutdown.
	if err := r.dvc.ReconcileWithEntities(ctx); err != nil {
		log.Warn("failed to reconcile disk volumes on startup", "error", err)
	}

	r.dmc = diskio.NewDiskMountController(log, dataPath, r.Id, diskioState, mntOps)
	r.dmc.SetEAC(eas)

	// Reconcile mounts with entity server on startup to re-mount any
	// accelerator volumes that were mounted before the last shutdown.
	if err := r.dmc.ReconcileWithEntities(ctx); err != nil {
		log.Warn("failed to reconcile disk mounts on startup", "error", err)
	}

	// Register volume controller shutdown before mount controller so mounts
	// owned by the volume controller are cleaned up first
	r.closers = append(r.closers, shutdownCloser{r.dvc})
	r.closers = append(r.closers, shutdownCloser{r.dmc})

	// Set up cloud auth for disk replication if configured
	var logUploader diskio.LogSegmentUploader
	if r.CloudAuth != nil && r.CloudAuth.Enabled && r.CloudAuth.PrivateKey != "" {
		cloudURL := r.CloudAuth.CloudURL
		if cloudURL == "" {
			cloudURL = coordinate.DefaultCloudURL
		}

		var keyData []byte
		if strings.HasPrefix(r.CloudAuth.PrivateKey, "-----BEGIN PRIVATE KEY-----") {
			keyData = []byte(r.CloudAuth.PrivateKey)
		} else {
			keyData, err = os.ReadFile(r.CloudAuth.PrivateKey)
			if err != nil {
				log.Warn("failed to load cloud auth private key for log watcher", "error", err)
			}
		}

		if keyData != nil {
			keyPair, kerr := cloudauth.LoadKeyPairFromPEM(string(keyData))
			if kerr != nil {
				log.Warn("failed to parse cloud auth private key for log watcher", "error", kerr)
			} else {
				authClient, aerr := cloudauth.NewAuthClient(cloudURL, keyPair)
				if aerr != nil {
					log.Warn("failed to create auth client for log watcher", "error", aerr)
				} else {
					cloudDiskClient := diskio.NewCloudDiskClient(log, cloudURL, authClient)
					r.dmc.SetCloudClient(cloudDiskClient)

					logUploader = diskio.NewCloudSegmentUploader(log, cloudURL, authClient, diskioState)
				}
			}
		}
	}

	// Always start the log watcher so accelerator mode log segments are
	// cleaned up even when cloud is not configured (nil uploader = delete only).
	watcher := diskio.NewLogWatcher(log, diskioState, logUploader, 5*time.Second)
	go func() {
		if werr := watcher.Run(ctx); werr != nil {
			log.Error("log watcher stopped", "error", werr)
		}
	}()
	r.closers = append(r.closers, waitCloser{watcher})
	if logUploader != nil {
		log.Info("started log watcher with cloud upload")
	} else {
		log.Info("started log watcher in delete-only mode (no cloud configured)")
	}

	volHandler := controller.AdaptReconcileController[storage_v1alpha.DiskVolume](r.dvc)
	cm.AddController(controller.NewReconcileController(
		"disk-volume", log, r.dvc.Index(), eas, volHandler, 5*time.Minute, workers,
	))

	mntHandler := controller.AdaptReconcileController[storage_v1alpha.DiskMount](r.dmc)
	cm.AddController(controller.NewReconcileController(
		"disk-mount", log, r.dmc.Index(), eas, mntHandler, 5*time.Minute, workers,
	))

	// Use entity mode controllers
	diskController := disk.NewDiskController(log, eas, r.Id)
	diskLeaseController := disk.NewDiskLeaseController(log, eas, r.Id)

	// Add disk controller to closers list so it gets cleaned up on shutdown
	r.closers = append(r.closers, diskController)

	err = sbc.Init(ctx)
	if err != nil {
		return nil, err
	}

	err = serviceController.Init(ctx)
	if err != nil {
		return nil, err
	}

	err = diskController.Init(ctx)
	if err != nil {
		return nil, err
	}

	err = diskLeaseController.Init(ctx)
	if err != nil {
		return nil, err
	}

	r.cc = sbc.CC
	r.namespace = sbc.Namespace
	r.sbController = sbc

	sbController := controller.NewReconcileController(
		"sandbox",
		log,
		compute_v1alpha.Index(compute_v1alpha.KindSandbox, entity.Id("node/"+r.Id)),
		eas,
		controller.AdaptController(sbc),
		time.Minute,
		workers,
	)

	// Wire up write tracker so manual Patch calls can skip self-generated watch events
	sbc.SetWriteTracker(sbController.WriteTracker())

	sbController.SetPeriodic(5*time.Minute, func(ctx context.Context) error {
		return sbc.Periodic(ctx, time.Hour)
	})

	cm.AddController(sbController)

	cm.AddController(
		controller.NewReconcileController(
			"service",
			log,
			entity.Ref(entity.EntityKind, network_v1alpha.KindService),
			eas,
			controller.AdaptController(serviceController),
			time.Minute,
			workers,
		),
	)

	cm.AddController(
		controller.NewReconcileController(
			"endpoints",
			log,
			entity.Ref(entity.EntityKind, network_v1alpha.KindEndpoints),
			eas,
			serviceController.UpdateEndpoints,
			0,
			workers,
		),
	)

	cm.AddController(
		controller.NewReconcileController(
			"default-route-app",
			log,
			entity.Ref(entity.EntityKind, core_v1alpha.KindApp),
			eas,
			controller.AdaptController(defaultRouteAppController),
			0, // No periodic resync needed
			1, // Single worker is sufficient for this controller
		),
	)

	cm.AddController(
		controller.NewReconcileController(
			"default-route",
			log,
			entity.Ref(entity.EntityKind, ingress_v1alpha.KindHttpRoute),
			eas,
			controller.AdaptController(defaultRouteController),
			0, // No periodic resync needed
			1, // Single worker is sufficient for this controller
		),
	)

	// Add disk controller
	diskRC := controller.NewReconcileController(
		"disk",
		log,
		entity.Ref(entity.EntityKind, storage_v1alpha.KindDisk),
		eas,
		controller.AdaptController(diskController),
		time.Minute,
		workers,
	)
	cm.AddController(diskRC)

	// Add disk lease controller
	diskLeaseRC := controller.NewReconcileController(
		"disk-lease",
		log,
		entity.Ref(entity.EntityKind, storage_v1alpha.KindDiskLease),
		eas,
		controller.AdaptController(diskLeaseController),
		time.Minute,
		workers,
	)

	// Set up periodic cleanup of old released leases (every 5 minutes)
	diskLeaseRC.SetPeriodic(5*time.Minute, func(ctx context.Context) error {
		return diskLeaseController.CleanupOldReleasedLeases(ctx)
	})

	cm.AddController(diskLeaseRC)

	// Add disk watch controller to trigger lease reconciliation on disk changes
	diskWatchController := disk.NewDiskWatchController(log, eas, diskLeaseRC)
	cm.AddController(
		controller.NewReconcileController(
			"disk-watch",
			log,
			entity.Ref(entity.EntityKind, storage_v1alpha.KindDisk),
			eas,
			controller.AdaptController(diskWatchController),
			time.Minute,
			1,
		),
	)

	// Add disk_volume watch controller to trigger disk re-reconciliation when
	// disk_volume entities change (e.g. volume becomes DV_READY after provisioning)
	diskVolumeWatchController := disk.NewDiskVolumeWatchController(log, eas, diskRC)
	cm.AddController(
		controller.NewReconcileController(
			"disk-volume-watch",
			log,
			entity.Ref(entity.EntityKind, storage_v1alpha.KindDiskVolume),
			eas,
			controller.AdaptController(diskVolumeWatchController),
			0,
			1,
		),
	)

	// Add disk_mount watch controller to trigger disk lease re-reconciliation when
	// disk_mount entities change (e.g. mount becomes DM_MOUNTED after mounting)
	diskMountWatchController := disk.NewDiskMountWatchController(log, eas, diskLeaseRC)
	cm.AddController(
		controller.NewReconcileController(
			"disk-mount-watch",
			log,
			entity.Ref(entity.EntityKind, storage_v1alpha.KindDiskMount),
			eas,
			controller.AdaptController(diskMountWatchController),
			0,
			1,
		),
	)

	return cm, nil
}
