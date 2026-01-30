// Command lsvd-server is a separate process that manages LSVD volumes and mounts.
// It watches lsvd_volume and lsvd_mount entities and reconciles them.
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"miren.dev/runtime/components/lsvd/server"
)

var (
	dataPath         = flag.String("data-path", "/var/lib/miren/disk-data", "Path for LSVD data")
	nodeId           = flag.String("node-id", "", "Node ID for filtering entities")
	entityServerAddr = flag.String("entity-server", "", "Entity server RPC address (e.g., localhost:9000)")
	logLevel         = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
)

func main() {
	flag.Parse()

	// Setup logger
	var level slog.Level
	switch *logLevel {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	})).With("module", "lsvd-server")

	log.Info("starting lsvd-server",
		"data_path", *dataPath,
		"node_id", *nodeId,
		"entity_server", *entityServerAddr,
	)

	// Validate required flags
	if *nodeId == "" {
		log.Error("--node-id is required")
		os.Exit(1)
	}

	if *entityServerAddr == "" {
		log.Error("--entity-server is required")
		os.Exit(1)
	}

	// Create data directory
	if err := os.MkdirAll(*dataPath, 0755); err != nil {
		log.Error("failed to create data directory", "error", err)
		os.Exit(1)
	}

	// Create server
	srv, err := server.NewServer(log, *dataPath, *nodeId, *entityServerAddr)
	if err != nil {
		log.Error("failed to create server", "error", err)
		os.Exit(1)
	}

	// Setup signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		sig := <-sigCh
		log.Info("received signal, shutting down", "signal", sig)
		cancel()
	}()

	// Run the server
	if err := srv.Run(ctx); err != nil {
		log.Error("server exited with error", "error", err)
		os.Exit(1)
	}

	log.Info("lsvd-server stopped")
}
