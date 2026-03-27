//go:build linux

package commands

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	"golang.org/x/sync/errgroup"
	"miren.dev/runtime/clientconfig"
	containerdcomp "miren.dev/runtime/components/containerd"
	"miren.dev/runtime/components/netresolve"
	"miren.dev/runtime/components/runner"
	"miren.dev/runtime/controllers/sandbox"
	"miren.dev/runtime/observability"
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

	var cc *containerd.Client

	if opts.ContainerdSocket != "" {
		// Explicit socket provided — connect to external containerd
		ctx.Log.Info("connecting to external containerd", "socket", opts.ContainerdSocket)
		cc, err = containerd.New(opts.ContainerdSocket,
			containerd.WithDefaultNamespace("miren"))
		if err != nil {
			return fmt.Errorf("failed to connect to containerd: %w", err)
		}
		defer cc.Close()
	} else {
		// Start embedded containerd (same as server standalone mode)
		var (
			containerdBinary string
			binDir           string
		)

		if releasePath := FindReleasePath(); releasePath != "" {
			candidate := filepath.Join(releasePath, "containerd")
			if _, err := os.Stat(candidate); err == nil {
				binDir = releasePath
				containerdBinary = candidate
			}
		}
		if containerdBinary == "" {
			var err error
			containerdBinary, err = exec.LookPath("containerd")
			if err != nil {
				return fmt.Errorf("containerd binary not found in PATH or release directory: %w", err)
			}
		}

		containerdComponent := containerdcomp.NewContainerdComponent(ctx.Log, opts.DataPath)

		envPath := os.Getenv("PATH")
		if binDir != "" {
			envPath = binDir + ":" + envPath
		}

		baseDir := filepath.Join(opts.DataPath, "containerd")
		containerdConfig := &containerdcomp.Config{
			BinaryPath: containerdBinary,
			BaseDir:    baseDir,
			BinDir:     binDir,
			SocketPath: filepath.Join(baseDir, "containerd.sock"),
			Env:        []string{"PATH=" + envPath},
		}

		if err := containerdComponent.Start(ctx, containerdConfig); err != nil {
			return fmt.Errorf("failed to start embedded containerd: %w", err)
		}
		defer func() {
			ctx.Log.Info("stopping embedded containerd")
			stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := containerdComponent.Stop(stopCtx); err != nil {
				ctx.Log.Error("failed to stop embedded containerd", "error", err)
			}
		}()

		cc, err = containerd.New(containerdComponent.SocketPath(),
			containerd.WithDefaultNamespace("miren"))
		if err != nil {
			return fmt.Errorf("failed to connect to embedded containerd: %w", err)
		}
		defer cc.Close()

		ctx.Log.Info("embedded containerd started", "socket", containerdComponent.SocketPath())
	}

	// Ensure data directory exists
	if err := os.MkdirAll(opts.DataPath, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Set up signal handling
	sigCtx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Create errgroup for background tasks
	eg, egCtx := errgroup.WithContext(sigCtx)

	// Determine listen address. If no explicit address is given, discover the
	// machine's outbound IP (the one that would route to the coordinator) and
	// advertise that so the coordinator knows how to reach this runner.
	listenAddr := opts.ListenAddr
	if listenAddr == "" {
		port := "8444"
		ip, err := discoverOutboundIP(cfg.CoordinatorAddress)
		if err != nil {
			return fmt.Errorf("could not discover outbound IP for listen address (use --listen to set manually): %w", err)
		}
		listenAddr = net.JoinHostPort(ip.String(), port)
		ctx.Log.Info("discovered listen address", "addr", listenAddr)
	}

	// Build runner configuration
	runnerCfg := runner.RunnerConfig{
		Id:            cfg.RunnerID,
		ListenAddress: listenAddr,
		Workers:       runner.DefaulWorkers,
		DataPath:      opts.DataPath,
		Config:        clientCfg,
		DiskMode:      cfg.DiskMode,
	}

	// Create resolver for network operations and map cluster.local to the
	// coordinator so the runner can pull images from the coordinator's registry.
	resolver, hostMapper := netresolve.NewLocalResolver()
	coordinatorHost, _, _ := net.SplitHostPort(cfg.CoordinatorAddress)
	if addr, err := netip.ParseAddr(coordinatorHost); err == nil {
		hostMapper.SetHost("cluster.local", addr)
		ctx.Log.Info("mapped cluster.local to coordinator", "addr", addr)
	}

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

		// Service prefixes - same as coordinator
		// TODO: These should come from join response
		ServicePrefixes: []netip.Prefix{
			netip.MustParsePrefix("10.10.0.0/16"),
			netip.MustParsePrefix("fd47:cafe:d00d::/64"),
		},

		// Flannel network configuration
		EtcdEndpoints:  cfg.EtcdEndpoints,
		EtcdPrefix:     cfg.EtcdPrefix,
		NetworkBackend: cfg.NetworkBackend,
	}

	// Write etcd TLS certs to disk for flannel (which requires file paths)
	if cfg.ClientCert != "" && cfg.ClientKey != "" && cfg.CACert != "" {
		etcdCertsDir := filepath.Join(opts.DataPath, "etcd-certs")
		if err := os.MkdirAll(etcdCertsDir, 0700); err != nil {
			return fmt.Errorf("failed to create etcd certs directory: %w", err)
		}
		certFile := filepath.Join(etcdCertsDir, "client.crt")
		keyFile := filepath.Join(etcdCertsDir, "client.key")
		caFile := filepath.Join(etcdCertsDir, "ca.crt")
		if err := os.WriteFile(certFile, []byte(cfg.ClientCert), 0644); err != nil {
			return fmt.Errorf("failed to write etcd client cert: %w", err)
		}
		if err := os.WriteFile(keyFile, []byte(cfg.ClientKey), 0600); err != nil {
			return fmt.Errorf("failed to write etcd client key: %w", err)
		}
		if err := os.WriteFile(caFile, []byte(cfg.CACert), 0644); err != nil {
			return fmt.Errorf("failed to write etcd CA cert: %w", err)
		}
		deps.EtcdTLSCertFile = certFile
		deps.EtcdTLSKeyFile = keyFile
		deps.EtcdTLSCAFile = caFile
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

// discoverOutboundIP finds the local IP that would be used to reach the given
// remote address. This gives us the machine's IP on the right interface without
// actually connecting.
func discoverOutboundIP(remoteAddr string) (netip.Addr, error) {
	conn, err := net.Dial("udp4", remoteAddr)
	if err != nil {
		return netip.Addr{}, err
	}
	defer conn.Close()
	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return netip.Addr{}, fmt.Errorf("unexpected local address type")
	}
	ip4 := addr.IP.To4()
	if ip4 == nil {
		return netip.Addr{}, fmt.Errorf("discovered non-IPv4 address: %s", addr.IP)
	}
	return netip.AddrFrom4([4]byte(ip4)), nil
}
