//go:build linux

package commands

import (
	"context"
	"fmt"
	"net/netip"
	"os"
	"os/signal"
	"syscall"

	containerd "github.com/containerd/containerd/v2/client"
	"golang.org/x/sync/errgroup"
	"miren.dev/runtime/clientconfig"
	"miren.dev/runtime/components/netresolve"
	"miren.dev/runtime/components/runner"
	"miren.dev/runtime/controllers/sandbox"
	"miren.dev/runtime/observability"
	"miren.dev/runtime/pkg/containerdx"
	"miren.dev/runtime/pkg/runnerconfig"
)

func RunnerStart(ctx *Context, opts struct {
	ConfigPath       string `long:"config" description:"Path to runner config" default:"/var/lib/miren/runner/config.yaml"`
	DataPath         string `long:"data-path" description:"Path to store runner data" default:"/var/lib/miren/runner"`
	ContainerdSocket string `long:"containerd-socket" description:"Path to containerd socket"`
	ListenAddr       string `short:"l" long:"listen" description:"Address this runner will listen on (overrides config)"`
}) error {
	// Load saved runner configuration
	cfg, err := runnerconfig.Load(opts.ConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load runner config: %w (did you run 'miren runner join' first?)", err)
	}

	ctx.Log.Info("starting distributed runner",
		"runner_id", cfg.RunnerID,
		"coordinator", cfg.CoordinatorAddress,
		"etcd_endpoints", cfg.EtcdEndpoints,
		"network_backend", cfg.NetworkBackend)

	// Create clientconfig from saved certs for RPC authentication
	clientCfg := clientconfig.NewConfig()
	clientCfg.SetCluster("coordinator", &clientconfig.ClusterConfig{
		Hostname:   cfg.CoordinatorAddress,
		CACert:     cfg.CACert,
		ClientCert: cfg.ClientCert,
		ClientKey:  cfg.ClientKey,
	})
	clientCfg.SetActiveCluster("coordinator")

	// Determine containerd socket
	containerdSocket := opts.ContainerdSocket
	if containerdSocket == "" {
		containerdSocket = containerdx.DefaultSocket
	}

	// Initialize containerd client
	cc, err := containerd.New(containerdSocket,
		containerd.WithDefaultNamespace("miren"))
	if err != nil {
		return fmt.Errorf("failed to connect to containerd: %w", err)
	}
	defer cc.Close()

	// Ensure data directory exists
	if err := os.MkdirAll(opts.DataPath, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Set up signal handling
	sigCtx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Create errgroup for background tasks
	eg, egCtx := errgroup.WithContext(sigCtx)

	// Determine listen address
	listenAddr := opts.ListenAddr
	if listenAddr == "" {
		listenAddr = ":8444" // Default runner listen port
	}

	// Build runner configuration
	runnerCfg := runner.RunnerConfig{
		Id:            cfg.RunnerID,
		ListenAddress: listenAddr,
		Workers:       runner.DefaulWorkers,
		DataPath:      opts.DataPath,
		Config:        clientCfg,
	}

	// Create resolver for network operations
	resolver, _ := netresolve.NewLocalResolver()

	// Build runner dependencies
	// Note: Some deps like NetServ require entity access client which we get after connecting
	// The runner will set up its own entity client connection via RPC
	deps := runner.RunnerDeps{
		CC:        cc,
		Namespace: "miren",
		Bridge:    "rt0",
		Tempdir:   os.TempDir(),

		// Distributed runner doesn't use local network for sandboxes
		DisableLocalNet: true,

		// Observability - create minimal instances
		LogsMaintainer: observability.NewLogsMaintainer(),
		LogWriter:      observability.NewDebugLogWriter(ctx.Log),
		StatusMon:      observability.NewStatusMonitor(ctx.Log),
		SandboxMetrics: sandbox.NewMetrics(),

		// Resolver for network operations
		Resolver: resolver,

		// Target prefixes - same as coordinator
		// TODO: These should come from join response
		TargetPrefixes: []netip.Prefix{
			netip.MustParsePrefix("10.10.0.0/16"),
			netip.MustParsePrefix("fd47:cafe:d00d::/64"),
		},

		// Flannel network configuration
		EtcdEndpoints:  cfg.EtcdEndpoints,
		EtcdPrefix:     cfg.EtcdPrefix,
		NetworkBackend: cfg.NetworkBackend,

		// etcd TLS - use the same certs as RPC auth
		EtcdTLSCert:   []byte(cfg.ClientCert),
		EtcdTLSKey:    []byte(cfg.ClientKey),
		EtcdTLSCACert: []byte(cfg.CACert),
	}

	// Initialize sandbox metrics
	deps.SandboxMetrics.Log = ctx.Log

	// Create runner
	r, err := runner.NewRunner(ctx.Log, deps, runnerCfg)
	if err != nil {
		return fmt.Errorf("failed to create runner: %w", err)
	}

	// Start runner with errgroup for background network tasks
	if err := r.Start(egCtx, eg); err != nil {
		return fmt.Errorf("failed to start runner: %w", err)
	}

	ctx.Log.Info("runner started successfully",
		"runner_id", cfg.RunnerID,
		"listen_address", listenAddr)

	// Wait for shutdown signal or error
	<-egCtx.Done()
	ctx.Log.Info("shutting down runner")

	// Close runner
	if err := r.Close(); err != nil {
		ctx.Log.Error("error closing runner", "error", err)
	}

	// Wait for background tasks to complete
	if err := eg.Wait(); err != nil && err != context.Canceled {
		ctx.Log.Error("background task error", "error", err)
	}

	ctx.Log.Info("runner stopped")
	return nil
}
