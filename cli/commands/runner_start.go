//go:build linux

package commands

import (
	"context"
	"crypto/x509"
	"encoding/pem"
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
	"miren.dev/runtime/api/runner/runner_v1alpha"
	"miren.dev/runtime/clientconfig"
	containerdcomp "miren.dev/runtime/components/containerd"
	"miren.dev/runtime/components/netresolve"
	"miren.dev/runtime/components/runner"
	"miren.dev/runtime/controllers/sandbox"
	"miren.dev/runtime/metrics"
	"miren.dev/runtime/observability"
	"miren.dev/runtime/pkg/rpc"
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

	// The runner's certificate is persisted in the config (often on a disk that
	// outlives the VM). If the listen address has changed since the cert was
	// issued, the persisted cert no longer covers our IP and clients (e.g.
	// sandbox exec) will fail TLS verification. Re-issue the cert before we use
	// it to serve. Refreshing is fatal when the address isn't covered: serving a
	// stale cert would leave the runner silently unreachable.
	if err := ensureRunnerCertificate(ctx, cfg, opts.ConfigPath, listenAddr); err != nil {
		return err
	}

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

	// Build runner configuration
	runnerCfg := runner.RunnerConfig{
		Id:            cfg.RunnerID,
		Name:          cfg.Name,
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
	} else {
		// Coordinator address is a hostname, resolve it to an IP.
		addrs, lookupErr := net.LookupHost(coordinatorHost)
		if lookupErr == nil && len(addrs) > 0 {
			if resolved, parseErr := netip.ParseAddr(addrs[0]); parseErr == nil {
				hostMapper.SetHost("cluster.local", resolved)
				ctx.Log.Info("mapped cluster.local to coordinator (resolved)", "hostname", coordinatorHost, "addr", resolved)
			}
		} else {
			ctx.Log.Warn("could not resolve coordinator hostname for cluster.local mapping", "host", coordinatorHost, "error", lookupErr)
		}
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

		LogsMaintainer: observability.NewLogsMaintainer(),
		StatusMon:      observability.NewStatusMonitor(ctx.Log),

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

	// Initialize observability subsystems. When the coordinator provided
	// VictoriaMetrics/VictoriaLogs addresses at join time, the runner ships
	// metrics and logs over flannel to the coordinator's instances. Otherwise
	// we fall back to debug logging.
	var metricsWriter *metrics.VictoriaMetricsWriter
	if cfg.VictoriametricsAddress != "" {
		metricsWriter = metrics.NewVictoriaMetricsWriter(ctx.Log, cfg.VictoriametricsAddress, 30*time.Second)
		metricsWriter.Start()
		defer func() {
			if err := metricsWriter.Close(); err != nil {
				ctx.Log.Error("failed to close metrics writer", "error", err)
			}
		}()
		ctx.Log.Info("metrics writer started", "address", cfg.VictoriametricsAddress)
	} else {
		ctx.Log.Warn("no VictoriaMetrics address configured, sandbox metrics will not be recorded")
	}

	sbMetrics := sandbox.NewMetrics()
	sbMetrics.Log = ctx.Log
	sbMetrics.CPUUsage = metrics.NewCPUUsage(ctx.Log, metricsWriter, nil)
	sbMetrics.MemUsage = metrics.NewMemoryUsage(ctx.Log, metricsWriter, nil)
	deps.SandboxMetrics = sbMetrics

	if cfg.VictorialogsAddress != "" {
		deps.LogWriter = observability.NewPersistentLogWriter(cfg.VictorialogsAddress, 30*time.Second)
		ctx.Log.Info("log writer started", "address", cfg.VictorialogsAddress)
	} else {
		deps.LogWriter = observability.NewDebugLogWriter(ctx.Log)
		ctx.Log.Warn("no VictoriaLogs address configured, sandbox logs will only be written to debug output")
	}

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

// ensureRunnerCertificate re-issues the runner's server certificate when the
// current listen address is not covered by the persisted certificate's SANs.
// This happens when a VM is recreated with a new IP but a persistent disk keeps
// the old config (cert + runner ID). The refreshed cert is written back to the
// config so it survives subsequent restarts. A required-but-failed refresh is
// fatal: serving a stale cert would leave the runner silently unreachable.
func ensureRunnerCertificate(ctx *Context, cfg *runnerconfig.Config, configPath, listenAddr string) error {
	if cfg.ClientCert == "" {
		return nil
	}

	covered, err := certCoversListenAddr(cfg.ClientCert, listenAddr)
	if err != nil {
		return fmt.Errorf("failed to inspect runner certificate: %w", err)
	}
	if covered {
		return nil
	}

	ctx.Log.Info("runner certificate does not cover listen address; refreshing",
		"listen_addr", listenAddr)

	cs, err := rpc.NewState(ctx,
		rpc.WithLogger(ctx.Log),
		rpc.WithBindAddr("[::]:0"),
		rpc.WithCertPEMs([]byte(cfg.ClientCert), []byte(cfg.ClientKey)),
		rpc.WithCertificateVerification([]byte(cfg.CACert)),
	)
	if err != nil {
		return fmt.Errorf("failed to create RPC state for certificate refresh: %w", err)
	}
	defer cs.Close()

	client, err := cs.Connect(cfg.CoordinatorAddress, rpc.ServiceRunner)
	if err != nil {
		return fmt.Errorf("failed to connect to coordinator for certificate refresh: %w", err)
	}
	defer client.Close()

	rc := runner_v1alpha.NewRunnerRegistrationClient(client)
	res, err := rc.RefreshCertificate(ctx, listenAddr)
	if err != nil {
		return fmt.Errorf("certificate refresh request failed: %w", err)
	}
	if res.Error() != "" {
		return fmt.Errorf("certificate refresh rejected by coordinator: %s", res.Error())
	}
	if len(res.CertPem()) == 0 || len(res.KeyPem()) == 0 {
		return fmt.Errorf("coordinator returned an empty certificate")
	}

	cfg.ClientCert = string(res.CertPem())
	cfg.ClientKey = string(res.KeyPem())
	if len(res.CaPem()) > 0 {
		cfg.CACert = string(res.CaPem())
	}

	if err := cfg.Save(configPath); err != nil {
		return fmt.Errorf("failed to save refreshed certificate: %w", err)
	}

	ctx.Log.Info("runner certificate refreshed", "listen_addr", listenAddr)
	return nil
}

// certCoversListenAddr reports whether the leaf certificate in certPEM carries a
// SAN matching the host of listenAddr: an IP SAN for an IP host, or a DNS SAN
// for a hostname. This mirrors how the coordinator builds the certificate's SANs
// from the listen address.
func certCoversListenAddr(certPEM, listenAddr string) (bool, error) {
	host, _, err := net.SplitHostPort(listenAddr)
	if err != nil {
		host = listenAddr
	}
	if host == "" {
		return true, nil
	}

	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		return false, fmt.Errorf("no PEM block found in certificate")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false, fmt.Errorf("parsing certificate: %w", err)
	}

	if ip := net.ParseIP(host); ip != nil {
		for _, certIP := range cert.IPAddresses {
			if certIP.Equal(ip) {
				return true, nil
			}
		}
		return false, nil
	}

	for _, name := range cert.DNSNames {
		if name == host {
			return true, nil
		}
	}
	return false, nil
}
