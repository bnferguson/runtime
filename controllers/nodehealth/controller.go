package nodehealth

import (
	"context"
	"errors"
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
// Nodes with DISABLED status are intentionally excluded. DISABLED is set
// by Drain() during graceful shutdown, which handles its own sandbox
// cleanup. We don't want to race with that.
//
// Implements controller.ReconcileControllerI[*compute_v1alpha.Node]
// Implements controller.DeletingReconcileController
type Controller struct {
	log         *slog.Logger
	eac         *entityserver_v1alpha.EntityAccessClient
	gracePeriod time.Duration

	mu          sync.Mutex
	unhealthyAt map[entity.Id]time.Time
	handled     map[entity.Id]bool
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
		handled:     make(map[entity.Id]bool),
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

	// DISABLED is operator-initiated (Drain). The drain process handles
	// its own sandbox cleanup, so we stay out of its way.
	if node.Status == compute_v1alpha.DISABLED {
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

	if c.isHandled(node.ID) {
		return nil
	}

	c.log.Warn("node not ready, grace period expired, marking sandboxes dead",
		"node", node.ID,
		"status", node.Status,
		"elapsed", elapsed.Round(time.Second))

	err := c.markNodeSandboxesDead(ctx, node.ID)
	if err == nil {
		c.setHandled(node.ID)
	}
	return err
}

// Delete cleans up tracking state when a node entity is removed
// (e.g., via "miren runner remove").
func (c *Controller) Delete(ctx context.Context, id entity.Id) error {
	c.clearTracking(id)
	return nil
}

// clearTracking removes a node from all tracking maps (it recovered,
// was removed, or entered a managed state like DISABLED).
func (c *Controller) clearTracking(nodeID entity.Id) {
	c.mu.Lock()
	defer c.mu.Unlock()
	wasTracked := false
	if _, ok := c.unhealthyAt[nodeID]; ok {
		delete(c.unhealthyAt, nodeID)
		wasTracked = true
	}
	if c.handled[nodeID] {
		delete(c.handled, nodeID)
		wasTracked = true
	}
	if wasTracked {
		c.log.Info("node recovered, clearing grace period tracking", "node", nodeID)
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

func (c *Controller) isHandled(nodeID entity.Id) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.handled[nodeID]
}

func (c *Controller) setHandled(nodeID entity.Id) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handled[nodeID] = true
}

func (c *Controller) markNodeSandboxesDead(ctx context.Context, nodeID entity.Id) error {
	idx := compute_v1alpha.Index(compute_v1alpha.KindSandbox, nodeID)
	results, err := c.eac.List(ctx, idx)
	if err != nil {
		return err
	}

	marked := 0
	var patchErr error
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
			patchErr = errors.Join(patchErr, err)
			continue
		}
		marked++
	}

	if marked > 0 {
		c.log.Warn("marked sandboxes dead for failed node",
			"node", nodeID,
			"count", marked)
	}

	return patchErr
}
