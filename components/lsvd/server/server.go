package server

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"miren.dev/runtime/api/entityserver/entityserver_v1alpha"
	"miren.dev/runtime/api/storage/storage_v1alpha"
	"miren.dev/runtime/pkg/cloudauth"
	"miren.dev/runtime/pkg/controller"
	"miren.dev/runtime/pkg/rpc"
)

// ServerOption configures the server
type ServerOption func(*serverOptions)

type serverOptions struct {
	clientCert []byte
	clientKey  []byte
	skipVerify bool
	cloudURL   string
	privateKey string
}

// WithClientCredentials sets the client TLS credentials for connecting to entity server
func WithClientCredentials(cert, key []byte) ServerOption {
	return func(o *serverOptions) {
		o.clientCert = cert
		o.clientKey = key
	}
}

// WithSkipVerify skips TLS verification when connecting to entity server
func WithSkipVerify(skip bool) ServerOption {
	return func(o *serverOptions) {
		o.skipVerify = skip
	}
}

// WithCloudAuth configures cloud authentication for disk replication
func WithCloudAuth(cloudURL, privateKey string) ServerOption {
	return func(o *serverOptions) {
		o.cloudURL = cloudURL
		o.privateKey = privateKey
	}
}

// Server manages LSVD volumes and mounts by watching entities
type Server struct {
	log              *slog.Logger
	dataPath         string
	nodeId           string
	entityServerAddr string

	// Client credentials for entity server
	clientCert []byte
	clientKey  []byte
	skipVerify bool

	// Cloud auth for disk replication
	cloudURL   string
	privateKey string

	rpcState *rpc.State
	eac      *entityserver_v1alpha.EntityAccessClient
	state    *State

	volumeController *VolumeController
	mountController  *MountController

	volumeRC *controller.ReconcileController
	mountRC  *controller.ReconcileController

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
func NewServer(log *slog.Logger, dataPath, nodeId, entityServerAddr string, opts ...ServerOption) (*Server, error) {
	var so serverOptions
	for _, opt := range opts {
		opt(&so)
	}

	return &Server{
		log:              log,
		dataPath:         dataPath,
		nodeId:           nodeId,
		entityServerAddr: entityServerAddr,
		clientCert:       so.clientCert,
		clientKey:        so.clientKey,
		skipVerify:       so.skipVerify,
		cloudURL:         so.cloudURL,
		privateKey:       so.privateKey,
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

	// Create cloud auth client if configured
	var authClient *cloudauth.AuthClient
	if s.cloudURL != "" && s.privateKey != "" {
		keyPair, err := cloudauth.LoadKeyPairFromPEM(s.privateKey)
		if err != nil {
			return fmt.Errorf("failed to load cloud auth keypair: %w", err)
		}
		authClient, err = cloudauth.NewAuthClient(s.cloudURL, keyPair)
		if err != nil {
			return fmt.Errorf("failed to create cloud auth client: %w", err)
		}
		s.log.Info("cloud auth configured for disk replication", "cloud_url", s.cloudURL)
	}

	// Create ops with cloud auth if available
	volumeOps := NewRealVolumeOps(s.log, authClient, s.cloudURL)
	mountOps := NewRealMountOps(s.log, authClient, s.cloudURL)

	// Create controllers without EAC so system reconciliation can run before
	// connecting to the entity server
	s.volumeController = NewVolumeController(s.log, s.dataPath, s.nodeId, s.state, volumeOps)
	s.mountController = NewMountController(s.log, s.dataPath, s.nodeId, s.state, mountOps)

	// Reconcile with system (NBD devices, mounts)
	s.log.Info("reconciling with system")
	if err := s.reconcileWithSystem(ctx); err != nil {
		s.log.Error("failed to reconcile with system", "error", err)
		// Continue anyway, we'll try to recover
	}

	// Connect to entity server via RPC
	s.log.Info("connecting to entity server", "address", s.entityServerAddr)

	// Build RPC options
	rpcOpts := []rpc.StateOption{rpc.WithLogger(s.log)}

	// Add client credentials if provided
	if len(s.clientCert) > 0 && len(s.clientKey) > 0 {
		rpcOpts = append(rpcOpts, rpc.WithCertPEMs(s.clientCert, s.clientKey))
		s.log.Info("using client credentials for entity server connection")
	}

	// Add skip verify if configured
	if s.skipVerify {
		rpcOpts = append(rpcOpts, rpc.WithSkipVerify)
	}

	rpcState, err := rpc.NewState(ctx, rpcOpts...)
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

	// Now that we have an entity server connection, enable entity-based reconciliation
	s.volumeController.SetEAC(s.eac)
	s.mountController.SetEAC(s.eac)

	// Reconcile with entities
	s.log.Info("reconciling with entities")
	if err := s.reconcileWithEntities(ctx); err != nil {
		s.log.Error("failed to reconcile with entities", "error", err)
		// Continue anyway, watches will catch up
	}

	s.log.Info("lsvd-server is ready")

	// Start reconcile controllers (watch-based with periodic resync)
	s.volumeRC = controller.NewReconcileController(
		"lsvd-volume", s.log,
		s.volumeController.Index(),
		s.eac,
		controller.AdaptReconcileController[storage_v1alpha.LsvdVolume](s.volumeController),
		30*time.Second, // resync period
		1,              // workers
	)
	s.volumeRC.SetPeriodic(5*time.Minute, func(ctx context.Context) error {
		return s.volumeController.ReconcileWithSystem(ctx)
	})

	s.mountRC = controller.NewReconcileController(
		"lsvd-mount", s.log,
		s.mountController.Index(),
		s.eac,
		controller.AdaptReconcileController[storage_v1alpha.LsvdMount](s.mountController),
		30*time.Second, // resync period
		1,              // workers
	)
	s.mountRC.SetPeriodic(30*time.Second, func(ctx context.Context) error {
		return s.mountController.ReconcileWithSystem(ctx)
	})

	// Periodically reconcile with entities to clean up orphaned local state.
	// This runs on a slower interval since it's more expensive.
	// Uses a goroutine since SetPeriodic only supports one callback per controller.
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.reconcileWithEntities(ctx); err != nil {
					s.log.Error("failed periodic entity reconciliation", "error", err)
				}
			}
		}
	}()

	if err := s.volumeRC.Start(ctx); err != nil {
		return fmt.Errorf("starting volume controller: %w", err)
	}
	if err := s.mountRC.Start(ctx); err != nil {
		s.volumeRC.Stop()
		return fmt.Errorf("starting mount controller: %w", err)
	}

	// Wait for context cancellation or shutdown signal
	select {
	case <-ctx.Done():
		s.log.Info("context cancelled, shutting down")
	case <-s.shutdownCh:
		s.log.Info("shutdown requested for version upgrade")
	}

	s.mountRC.Stop()
	s.volumeRC.Stop()
	s.mountController.Shutdown()

	if s.rpcState != nil {
		s.rpcState.Close()
	}

	return nil
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

// reconcileWithEntities reconciles the persisted state with entity server.
// Mounts are reconciled before volumes so that orphaned mounts are cleaned up
// before their backing volume directories are removed.
func (s *Server) reconcileWithEntities(ctx context.Context) error {
	// Reconcile mounts first (clean up orphaned mounts before removing volumes)
	if err := s.mountController.ReconcileWithEntities(ctx); err != nil {
		s.log.Error("failed to reconcile mounts with entities", "error", err)
	}

	// Reconcile volumes
	if err := s.volumeController.ReconcileWithEntities(ctx); err != nil {
		s.log.Error("failed to reconcile volumes with entities", "error", err)
	}

	return nil
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
