package globalrouter

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"miren.dev/runtime/pkg/cloudauth"
	"miren.dev/runtime/servers/httpingress"
)

// Config holds the configuration for the global router.
type Config struct {
	CloudURL   string
	ClusterXID string
	AuthClient *cloudauth.AuthClient
	Ingress    *httpingress.Server
	Log        *slog.Logger
}

// GlobalRouter connects the cluster to the cloud coordination service
// and manages POP connections for inbound traffic forwarding.
type GlobalRouter struct {
	client *Client
	pops   *POPManager
	log    *slog.Logger

	mu             sync.RWMutex
	timeOffset     time.Duration
	organizationID string
}

// New creates a new GlobalRouter.
func New(cfg Config) *GlobalRouter {
	log := cfg.Log
	if log == nil {
		log = slog.Default()
	}

	router := NewMessageRouter()

	client := NewClient(cfg.CloudURL, cfg.AuthClient, router, log)
	pops := NewPOPManager(cfg.ClusterXID, cfg.Ingress, log)

	gr := &GlobalRouter{
		client: client,
		pops:   pops,
		log:    log,
	}

	// Register message handlers
	router.Handle(TypeConnectionRequest, gr.handleConnectionRequest)
	router.Handle(TypeTimeResponse, gr.handleTimeResponse)
	router.Handle(TypeOrgInfoResponse, gr.handleOrgInfoResponse)

	// On each new connection, request time sync and org info
	client.OnConnect(func(ctx context.Context) {
		gr.log.Info("sending initial requests")
		client.SendMessage(TypeTimeRequest, TimeRequest{
			ClientTransmitTime: time.Now().UTC(),
		})
		client.SendMessage(TypeOrgInfoRequest, struct{}{})
	})

	return gr
}

// Run starts the global router. It blocks until the context is cancelled.
func (gr *GlobalRouter) Run(ctx context.Context) error {
	gr.log.Info("global router starting")
	defer gr.log.Info("global router stopped")
	defer gr.pops.Close()
	return gr.client.Run(ctx)
}

// TimeOffset returns the estimated clock offset between this cluster
// and the cloud, computed via simplified NTP.
func (gr *GlobalRouter) TimeOffset() time.Duration {
	gr.mu.RLock()
	defer gr.mu.RUnlock()
	return gr.timeOffset
}

// OrganizationID returns the organization ID reported by the cloud.
func (gr *GlobalRouter) OrganizationID() string {
	gr.mu.RLock()
	defer gr.mu.RUnlock()
	return gr.organizationID
}

func (gr *GlobalRouter) handleConnectionRequest(ctx context.Context, data json.RawMessage) error {
	var req ConnectionRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return err
	}
	return gr.pops.HandleConnectionRequest(ctx, req)
}

func (gr *GlobalRouter) handleTimeResponse(ctx context.Context, data json.RawMessage) error {
	t4 := time.Now().UTC()

	var resp TimeResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return err
	}

	// Simplified NTP offset calculation:
	// offset = ((T2 - T1) + (T3 - T4)) / 2
	t1 := resp.ClientTransmitTime
	t2 := resp.ServerReceiveTime
	t3 := resp.ServerTransmitTime

	offset := (t2.Sub(t1) + t3.Sub(t4)) / 2

	gr.mu.Lock()
	gr.timeOffset = offset
	gr.mu.Unlock()

	gr.log.Info("clock sync complete", "offset", offset)
	return nil
}

func (gr *GlobalRouter) handleOrgInfoResponse(ctx context.Context, data json.RawMessage) error {
	var resp OrgInfoResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return err
	}

	gr.mu.Lock()
	gr.organizationID = resp.OrganizationID
	gr.mu.Unlock()

	gr.log.Info("organization info received", "org_id", resp.OrganizationID)
	return nil
}
