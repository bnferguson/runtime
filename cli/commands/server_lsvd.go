package commands

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"miren.dev/runtime/api/lsvd/lsvd_v1alpha"
	"miren.dev/runtime/components/lsvd/server"
	"miren.dev/runtime/pkg/outboard"
)

// ServerLsvd runs the LSVD server for managing disk volumes and mounts.
// This is typically invoked by the main server process, not directly by users.
func ServerLsvd(ctx *Context, opts struct {
	DataPath         string `long:"data-path" description:"Path for LSVD data" default:"/var/lib/miren/disk-data"`
	NodeId           string `long:"node-id" description:"Node ID for filtering entities" required:"true"`
	EntityServerAddr string `long:"entity-server" description:"Entity server RPC address" required:"true"`
	SkipVerify       bool   `long:"skip-verify" description:"Skip TLS verification"`
}) error {
	// Check for outboard mode
	if configPath := os.Getenv("OUTBOARD_CONFIG"); configPath != "" {
		return runOutboardLsvd(ctx, configPath, opts.DataPath, opts.NodeId, opts.EntityServerAddr, opts.SkipVerify)
	}

	log := ctx.Log.With("module", "lsvd-server")

	log.Info("starting lsvd-server",
		"data_path", opts.DataPath,
		"node_id", opts.NodeId,
		"entity_server", opts.EntityServerAddr,
	)

	// Create data directory
	if err := os.MkdirAll(opts.DataPath, 0755); err != nil {
		return err
	}

	// Load service config if it exists
	var svcConfig *server.ServiceConfig
	svcConfigPath := filepath.Join(opts.DataPath, "service.config")
	if cfg, err := server.LoadServiceConfig(svcConfigPath); err == nil {
		svcConfig = cfg
		log.Info("loaded service config")
	} else if !os.IsNotExist(err) {
		log.Warn("failed to load service config", "error", err)
	}

	// Build server options
	serverOpts := []server.ServerOption{
		server.WithSkipVerify(opts.SkipVerify),
	}
	if svcConfig != nil {
		serverOpts = append(serverOpts, server.WithClientCredentials(svcConfig.ClientCert, svcConfig.ClientKey))
		if svcConfig.CloudURL != "" && svcConfig.PrivateKey != "" {
			serverOpts = append(serverOpts, server.WithCloudAuth(svcConfig.CloudURL, svcConfig.PrivateKey))
		}
	}

	// Create server
	srv, err := server.NewServer(log, opts.DataPath, opts.NodeId, opts.EntityServerAddr,
		serverOpts...,
	)
	if err != nil {
		return err
	}

	// Setup signal handling
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		sig := <-sigCh
		log.Info("received signal, shutting down", "signal", sig)
		cancel()
	}()

	// Run the server
	if err := srv.Run(runCtx); err != nil {
		return err
	}

	log.Info("lsvd-server stopped")
	return nil
}

// runOutboardLsvd runs lsvd-server in outboard mode, managed by the parent
// process via the outboard framework.
func runOutboardLsvd(ctx *Context, configPath, dataPath, nodeId, entityServerAddr string, skipVerify bool) error {
	// Create outboard server (reads config, sets up token auth RPC, writes ready)
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	obs, err := outboard.NewServer(runCtx, configPath,
		outboard.WithVersion(server.ServerVersion),
		outboard.WithShutdownFunc(cancel),
	)
	if err != nil {
		return err
	}
	defer obs.Close()

	log := obs.Logger()

	log.Info("starting lsvd-server in outboard mode",
		"data_path", dataPath,
		"node_id", nodeId,
		"entity_server", entityServerAddr,
	)

	// Create data directory
	if err := os.MkdirAll(dataPath, 0755); err != nil {
		return err
	}

	// Load service config if it exists
	var svcConfig *server.ServiceConfig
	svcConfigPath := filepath.Join(dataPath, "service.config")
	if cfg, err := server.LoadServiceConfig(svcConfigPath); err == nil {
		svcConfig = cfg
		log.Info("loaded service config")
	} else if !os.IsNotExist(err) {
		log.Warn("failed to load service config", "error", err)
	}

	// Build server options
	serverOpts := []server.ServerOption{
		server.WithSkipVerify(skipVerify),
	}
	if svcConfig != nil {
		serverOpts = append(serverOpts, server.WithClientCredentials(svcConfig.ClientCert, svcConfig.ClientKey))
		if svcConfig.CloudURL != "" && svcConfig.PrivateKey != "" {
			serverOpts = append(serverOpts, server.WithCloudAuth(svcConfig.CloudURL, svcConfig.PrivateKey))
		}
	}

	// Create the lsvd server
	srv, err := server.NewServer(log, dataPath, nodeId, entityServerAddr,
		serverOpts...,
	)
	if err != nil {
		return err
	}

	// Expose lsvd-debug interface on the outboard RPC state
	debugService := server.NewDebugService(srv)
	obs.RPCState().Server().ExposeValue("lsvd-debug", lsvd_v1alpha.AdaptLsvdDebug(debugService))

	// Setup signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		sig := <-sigCh
		log.Info("received signal, shutting down", "signal", sig)
		cancel()
	}()

	// Run the server
	if err := srv.Run(runCtx); err != nil {
		return err
	}

	log.Info("lsvd-server stopped")
	return nil
}
