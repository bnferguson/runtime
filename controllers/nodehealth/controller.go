package nodehealth

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"miren.dev/runtime/api/compute/compute_v1alpha"
	"miren.dev/runtime/api/entityserver/entityserver_v1alpha"
	"miren.dev/runtime/pkg/entity"
)

const DefaultGracePeriod = 5 * time.Minute

// Controller watches Node entities and marks sandboxes DEAD when their
// runner has been non-READY for longer than the grace period. This handles
// the case where a runner dies permanently (node failure, crash without
// recovery) and its sandboxes need to be cleaned up so the pool controller
// can create replacements.
//
// The grace period exists because containers survive miren process restarts.
// A runner that's just restarting (upgrade, deploy) will come back and
// re-adopt its containers via reconcileSandboxesOnBoot(). We only want to
// intervene when the runner is truly gone.
//
// Implements controller.ReconcileControllerI[*compute_v1alpha.Node]
type Controller struct {
	log         *slog.Logger
	eac         *entityserver_v1alpha.EntityAccessClient
	gracePeriod time.Duration

	mu          sync.Mutex
	unhealthyAt map[entity.Id]time.Time
	now         func() time.Time // for testing
}

func NewController(
	log *slog.Logger,
	eac *entityserver_v1alpha.EntityAccessClient,
) *Controller {
	return &Controller{
		log:         log.With("module", "nodehealth"),
		eac:         eac,
		gracePeriod: DefaultGracePeriod,
		unhealthyAt: make(map[entity.Id]time.Time),
		now:         time.Now,
	}
}

func (c *Controller) Init(ctx context.Context) error {
	c.log.Info("initializing node health controller", "grace_period", c.gracePeriod)
	return nil
}

func (c *Controller) Reconcile(ctx context.Context, node *compute_v1alpha.Node, meta *entity.Meta) error {
	if node.Status == compute_v1alpha.READY {
		c.clearTracking(node.ID)
		return nil
	}

	firstSeen := c.trackUnhealthy(node.ID)
	elapsed := c.now().Sub(firstSeen)

	if elapsed < c.gracePeriod {
		c.log.Info("node not ready, within grace period",
			"node", node.ID,
			"status", node.Status,
			"elapsed", elapsed.Round(time.Second),
			"grace_period", c.gracePeriod)
		return nil
	}

	c.log.Warn("node not ready, grace period expired, marking sandboxes dead",
		"node", node.ID,
		"status", node.Status,
		"elapsed", elapsed.Round(time.Second))

	return c.markNodeSandboxesDead(ctx, node.ID)
}

// clearTracking removes a node from the unhealthy tracker (it recovered).
func (c *Controller) clearTracking(nodeID entity.Id) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, was := c.unhealthyAt[nodeID]; was {
		c.log.Info("node recovered, clearing grace period tracking", "node", nodeID)
		delete(c.unhealthyAt, nodeID)
	}
}

// trackUnhealthy records when a node was first seen as non-READY. Returns
// the timestamp of first observation.
func (c *Controller) trackUnhealthy(nodeID entity.Id) time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	if t, ok := c.unhealthyAt[nodeID]; ok {
		return t
	}
	now := c.now()
	c.unhealthyAt[nodeID] = now
	return now
}

func (c *Controller) markNodeSandboxesDead(ctx context.Context, nodeID entity.Id) error {
	idx := compute_v1alpha.Index(compute_v1alpha.KindSandbox, nodeID)
	results, err := c.eac.List(ctx, idx)
	if err != nil {
		return err
	}

	marked := 0
	for _, e := range results.Values() {
		var sb compute_v1alpha.Sandbox
		sb.Decode(e.Entity())

		if sb.Status == compute_v1alpha.DEAD || sb.Status == compute_v1alpha.STOPPED {
			continue
		}

		c.log.Info("marking sandbox dead",
			"sandbox", sb.ID,
			"node", nodeID,
			"previous_status", sb.Status)

		_, err := c.eac.Patch(ctx, entity.New(
			entity.DBId, sb.ID,
			(&compute_v1alpha.Sandbox{
				Status: compute_v1alpha.DEAD,
			}).Encode,
		).Attrs(), 0)
		if err != nil {
			c.log.Error("failed to mark sandbox dead", "sandbox", sb.ID, "error", err)
			continue
		}
		marked++
	}

	if marked > 0 {
		c.log.Warn("marked sandboxes dead for failed node",
			"node", nodeID,
			"count", marked)
	}

	return nil
}
