package outboard

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"miren.dev/runtime/api/outboard/outboard_v1alpha"
	"miren.dev/runtime/pkg/rpc"
	"miren.dev/runtime/pkg/rpc/standard"
)

// Server is the outboard process side of the outboard framework.
// It reads the config written by the connector, sets up token-authenticated
// RPC, exposes the OutboardControl interface, and signals readiness.
type Server struct {
	log        *slog.Logger
	configPath string
	config     *Config
	rpcState   *rpc.State
	startTime  time.Time

	version      uint64
	shutdownFunc func()
}

// ServerOption configures the outboard Server.
type ServerOption func(*Server)

// WithVersion sets the version number reported by this outboard process.
func WithVersion(v uint64) ServerOption {
	return func(s *Server) {
		s.version = v
	}
}

// WithShutdownFunc sets a function to call when a version mismatch triggers shutdown.
func WithShutdownFunc(fn func()) ServerOption {
	return func(s *Server) {
		s.shutdownFunc = fn
	}
}

// NewServer creates a new outboard Server by reading the config file at configPath.
// It creates a JSON slog handler writing to stdout (which the connector reads via FIFO),
// starts an RPC server with token authentication, and exposes the OutboardControl interface.
func NewServer(ctx context.Context, configPath string, opts ...ServerOption) (*Server, error) {
	// Ignore SIGPIPE to survive parent restarts. Outboard processes have their
	// stdout/stderr connected to FIFOs. When the parent exits, the FIFO reader
	// closes, and writes would trigger SIGPIPE. Go's default behavior is to
	// kill the process on SIGPIPE to fd 1/2, so we must explicitly ignore it.
	signal.Ignore(syscall.SIGPIPE)

	cfg, err := ReadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("reading outboard config: %w", err)
	}

	s := &Server{
		configPath: configPath,
		config:     cfg,
		startTime:  time.Now(),
	}

	for _, opt := range opts {
		opt(s)
	}

	// Create JSON slog handler writing to stdout (connected to FIFO by connector)
	s.log = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// Create RPC state with token auth
	auth := NewTokenAuthenticator(cfg.Token)
	s.rpcState, err = rpc.NewState(ctx,
		rpc.WithLogger(s.log),
		rpc.WithAuthenticator(auth),
		rpc.WithSkipVerify,
	)
	if err != nil {
		return nil, fmt.Errorf("creating RPC state: %w", err)
	}

	// Expose the OutboardControl interface
	s.rpcState.Server().ExposeValue("outboard-control", outboard_v1alpha.AdaptOutboardControl(s))

	// Write back the RPC address, PID, and ready flag
	cfg.RPCAddr = s.rpcState.ListenAddr()
	cfg.PID = os.Getpid()
	cfg.Ready = true
	if err := WriteConfig(configPath, cfg); err != nil {
		s.rpcState.Close()
		return nil, fmt.Errorf("writing config with RPC address: %w", err)
	}

	s.log.Info("outboard server ready",
		"addr", cfg.RPCAddr,
		"pid", cfg.PID,
	)

	return s, nil
}

// RPCState returns the RPC state for registering additional domain-specific interfaces.
func (s *Server) RPCState() *rpc.State {
	return s.rpcState
}

// Logger returns the server's logger (JSON handler writing to stdout FIFO).
func (s *Server) Logger() *slog.Logger {
	return s.log
}

// Close shuts down the outboard server.
func (s *Server) Close() error {
	if s.rpcState != nil {
		s.rpcState.Close()
	}
	return nil
}

// Health implements OutboardControl.Health
func (s *Server) Health(ctx context.Context, state *outboard_v1alpha.OutboardControlHealth) error {
	status := &outboard_v1alpha.OutboardHealthStatus{}
	status.SetHealthy(true)
	status.SetTimestamp(standard.ToTimestamp(time.Now()))
	status.SetPid(int32(os.Getpid()))
	status.SetUptime(standard.ToDuration(time.Since(s.startTime)))
	state.Results().SetStatus(status)
	return nil
}

// CheckVersion implements OutboardControl.CheckVersion
func (s *Server) CheckVersion(ctx context.Context, state *outboard_v1alpha.OutboardControlCheckVersion) error {
	expected := state.Args().ExpectedVersion()
	result := &outboard_v1alpha.OutboardVersionResult{}
	result.SetCurrentVersion(s.version)

	if s.version != expected && expected > s.version {
		result.SetNeedsRestart(true)
		s.log.Info("version mismatch, will restart",
			"current", s.version,
			"expected", expected,
		)
		if s.shutdownFunc != nil {
			go s.shutdownFunc()
		}
	} else {
		result.SetNeedsRestart(false)
	}

	state.Results().SetResult(result)
	return nil
}

// UpdateConfig re-reads the config and updates the ready state.
// This is used internally after the process has finished initializing.
func (s *Server) UpdateConfig(fn func(*Config)) error {
	cfg, err := ReadConfig(s.configPath)
	if err != nil {
		return err
	}
	fn(cfg)
	if err := WriteConfig(s.configPath, cfg); err != nil {
		return err
	}
	// Update in-memory config to keep it in sync with disk
	s.config = cfg
	return nil
}
