package server

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"miren.dev/runtime/api/entityserver/entityserver_v1alpha"
	"miren.dev/runtime/api/lsvd/lsvd_v1alpha"
	"miren.dev/runtime/pkg/rpc"
)

const (
	// AddrFileName is the name of the file containing the debug server address
	AddrFileName = "lsvd-server.addr"
	// ReadyFileName is the name of the file indicating the server is ready
	ReadyFileName = "lsvd-server.ready"
)

// Server manages LSVD volumes and mounts by watching entities
type Server struct {
	log              *slog.Logger
	dataPath         string
	nodeId           string
	entityServerAddr string
	debugAddr        string

	rpcState      *rpc.State
	debugRpcState *rpc.State
	eac           *entityserver_v1alpha.EntityAccessClient
	state         *State

	volumeController *VolumeController
	mountController  *MountController

	// Shutdown control for version upgrades
	shutdownMu     sync.Mutex
	shutdownCh     chan struct{}
	shutdownCancel context.CancelFunc

	// Health tracking
	healthMu              sync.RWMutex
	entityServerConnected bool
	lastVolumeReconcile   time.Time
	lastMountReconcile    time.Time
	lastError             string

	// Metrics tracking
	metricsMu            sync.RWMutex
	volumeReconcileCount int64
	mountReconcileCount  int64
	volumeErrorCount     int64
	mountErrorCount      int64
	lastVolumeDuration   time.Duration
	lastMountDuration    time.Duration
}

// NewServer creates a new LSVD server
func NewServer(log *slog.Logger, dataPath, nodeId, entityServerAddr, debugAddr string) (*Server, error) {
	return &Server{
		log:              log,
		dataPath:         dataPath,
		nodeId:           nodeId,
		entityServerAddr: entityServerAddr,
		debugAddr:        debugAddr,
	}, nil
}

// Run starts the server and blocks until context is cancelled
func (s *Server) Run(ctx context.Context) error {
	// Initialize shutdown channel for version upgrades
	s.shutdownMu.Lock()
	s.shutdownCh = make(chan struct{})
	s.shutdownMu.Unlock()

	s.log.Info("loading persisted state")

	// Load state from disk
	state, err := LoadState(s.dataPath)
	if err != nil {
		s.log.Warn("failed to load state, starting fresh", "error", err)
		state = NewState()
		state.SetPath(s.dataPath)
	}
	s.state = state

	s.log.Info("loaded state",
		"volumes", len(state.Volumes),
		"mounts", len(state.Mounts),
	)

	// Create controllers with nil ops (will use real ops)
	s.volumeController = NewVolumeController(s.log, s.dataPath, s.nodeId, nil, s.state, nil)
	s.mountController = NewMountController(s.log, s.dataPath, s.nodeId, nil, s.state, nil)

	// Reconcile with system (NBD devices, mounts)
	s.log.Info("reconciling with system")
	if err := s.reconcileWithSystem(ctx); err != nil {
		s.log.Error("failed to reconcile with system", "error", err)
		// Continue anyway, we'll try to recover
	}

	// Connect to entity server via RPC
	s.log.Info("connecting to entity server", "address", s.entityServerAddr)
	rpcState, err := rpc.NewState(ctx, rpc.WithLogger(s.log), rpc.WithSkipVerify)
	if err != nil {
		return fmt.Errorf("failed to create RPC state: %w", err)
	}
	s.rpcState = rpcState

	client, err := rpcState.Connect(s.entityServerAddr, "entities")
	if err != nil {
		rpcState.Close()
		return fmt.Errorf("failed to connect to entity server: %w", err)
	}

	s.eac = entityserver_v1alpha.NewEntityAccessClient(client)

	s.healthMu.Lock()
	s.entityServerConnected = true
	s.healthMu.Unlock()

	s.log.Info("connected to entity server")

	// Update controllers with entity access client
	s.volumeController = NewVolumeController(s.log, s.dataPath, s.nodeId, s.eac, s.state, nil)
	s.mountController = NewMountController(s.log, s.dataPath, s.nodeId, s.eac, s.state, nil)

	// Reconcile with entities
	s.log.Info("reconciling with entities")
	if err := s.reconcileWithEntities(ctx); err != nil {
		s.log.Error("failed to reconcile with entities", "error", err)
		// Continue anyway, watches will catch up
	}

	// Start debug RPC server
	s.log.Info("starting debug RPC server", "address", s.debugAddr)
	debugRpcState, err := rpc.NewState(ctx,
		rpc.WithLogger(s.log),
		rpc.WithBindAddr(s.debugAddr),
		rpc.WithSkipVerify,
	)
	if err != nil {
		rpcState.Close()
		return fmt.Errorf("failed to create debug RPC state: %w", err)
	}
	s.debugRpcState = debugRpcState

	// Expose debug service
	debugService := NewDebugService(s)
	debugRpcState.Server().ExposeValue("lsvd-debug", lsvd_v1alpha.AdaptLsvdDebug(debugService))

	// Write debug server address file
	listenAddr := debugRpcState.ListenAddr()
	addrFile := filepath.Join(s.dataPath, AddrFileName)
	if err := os.WriteFile(addrFile, []byte(listenAddr+"\n"), 0644); err != nil {
		s.log.Warn("failed to write debug address file", "error", err)
	}
	defer os.Remove(addrFile)

	s.log.Info("debug RPC server started", "listen_addr", listenAddr)

	// Write ready file
	readyFile := filepath.Join(s.dataPath, ReadyFileName)
	if err := os.WriteFile(readyFile, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0644); err != nil {
		s.log.Warn("failed to write ready file", "error", err)
	}
	defer os.Remove(readyFile)

	s.log.Info("lsvd-server is ready")

	// Start watching entities
	errCh := make(chan error, 3)

	go func() {
		errCh <- s.watchVolumes(ctx)
	}()

	go func() {
		errCh <- s.watchMounts(ctx)
	}()

	// Wait for context cancellation, shutdown signal, or error
	select {
	case <-ctx.Done():
		s.log.Info("context cancelled, shutting down")
		return nil
	case <-s.shutdownCh:
		s.log.Info("shutdown requested for version upgrade")
		return nil
	case err := <-errCh:
		return err
	}
}

// triggerShutdown signals the server to shut down gracefully for an upgrade
func (s *Server) triggerShutdown() {
	s.shutdownMu.Lock()
	defer s.shutdownMu.Unlock()

	if s.shutdownCh != nil {
		select {
		case <-s.shutdownCh:
			// Already closed
		default:
			close(s.shutdownCh)
		}
	}
}

// reconcileWithSystem reconciles the persisted state with the actual system state
func (s *Server) reconcileWithSystem(ctx context.Context) error {
	// Reconcile volumes first (verifies directories exist)
	if err := s.volumeController.ReconcileWithSystem(ctx); err != nil {
		s.log.Error("failed to reconcile volumes with system", "error", err)
	}

	// Reconcile mounts (reconnects NBD handlers, verifies mounts)
	if err := s.mountController.ReconcileWithSystem(ctx); err != nil {
		s.log.Error("failed to reconcile mounts with system", "error", err)
	}

	return nil
}

// reconcileWithEntities reconciles the persisted state with entity server
func (s *Server) reconcileWithEntities(ctx context.Context) error {
	// Reconcile volumes
	if err := s.volumeController.ReconcileWithEntities(ctx); err != nil {
		s.log.Error("failed to reconcile volumes with entities", "error", err)
	}

	// Reconcile mounts
	if err := s.mountController.ReconcileWithEntities(ctx); err != nil {
		s.log.Error("failed to reconcile mounts with entities", "error", err)
	}

	return nil
}

// watchVolumes watches for lsvd_volume entity changes
func (s *Server) watchVolumes(ctx context.Context) error {
	return s.volumeController.Run(ctx)
}

// watchMounts watches for lsvd_mount entity changes
func (s *Server) watchMounts(ctx context.Context) error {
	return s.mountController.Run(ctx)
}

// setLastVolumeReconcile updates the last volume reconcile timestamp
func (s *Server) setLastVolumeReconcile() {
	s.healthMu.Lock()
	s.lastVolumeReconcile = time.Now()
	s.healthMu.Unlock()
}

// setLastMountReconcile updates the last mount reconcile timestamp
func (s *Server) setLastMountReconcile() {
	s.healthMu.Lock()
	s.lastMountReconcile = time.Now()
	s.healthMu.Unlock()
}

// setLastError sets the last error message
func (s *Server) setLastError(err string) {
	s.healthMu.Lock()
	s.lastError = err
	s.healthMu.Unlock()
}

// recordVolumeReconcile records a volume reconciliation
func (s *Server) recordVolumeReconcile(duration time.Duration, err error) {
	s.metricsMu.Lock()
	s.volumeReconcileCount++
	s.lastVolumeDuration = duration
	if err != nil {
		s.volumeErrorCount++
	}
	s.metricsMu.Unlock()

	s.setLastVolumeReconcile()
	if err != nil {
		s.setLastError(err.Error())
	}
}

// recordMountReconcile records a mount reconciliation
func (s *Server) recordMountReconcile(duration time.Duration, err error) {
	s.metricsMu.Lock()
	s.mountReconcileCount++
	s.lastMountDuration = duration
	if err != nil {
		s.mountErrorCount++
	}
	s.metricsMu.Unlock()

	s.setLastMountReconcile()
	if err != nil {
		s.setLastError(err.Error())
	}
}
