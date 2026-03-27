package scheduler

import (
	"context"
	"log/slog"
	"math/rand"

	"miren.dev/runtime/api/compute/compute_v1alpha"
	"miren.dev/runtime/api/entityserver/entityserver_v1alpha"
	"miren.dev/runtime/pkg/entity"
)

// Controller assigns sandboxes to nodes for execution.
// It watches sandbox entities and adds a ScheduleKey attribute to assign
// each sandbox to an available node.
//
// Stateful sandboxes (those with miren disk volumes) are scheduled to the
// coordinator node, while stateless sandboxes prefer runner nodes when available.
//
// Implements controller.ReconcileControllerI[*compute_v1alpha.Sandbox]
type Controller struct {
	log *slog.Logger
	eac *entityserver_v1alpha.EntityAccessClient
}

// NewController creates a new scheduler controller
func NewController(
	log *slog.Logger,
	eac *entityserver_v1alpha.EntityAccessClient,
) *Controller {
	return &Controller{
		log: log.With("module", "scheduler"),
		eac: eac,
	}
}

// Init initializes the controller.
// Required by ReconcileControllerI.
func (c *Controller) Init(ctx context.Context) error {
	c.log.Info("initializing scheduler controller")
	return nil
}

// Reconcile ensures the sandbox is assigned to a node.
// Called by the controller framework for both Add and Update events.
func (c *Controller) Reconcile(ctx context.Context, sandbox *compute_v1alpha.Sandbox, meta *entity.Meta) error {
	// Skip if already scheduled
	if _, ok := meta.Get(compute_v1alpha.ScheduleKeyId); ok {
		return nil
	}

	c.log.Debug("scheduling sandbox", "id", sandbox.ID)

	// Fetch fresh node data
	allNodes, err := c.gatherNodes(ctx)
	if err != nil {
		c.log.Error("failed to gather nodes", "error", err)
		return err
	}

	// Find available READY nodes
	var nodes []*compute_v1alpha.Node
	for _, node := range allNodes {
		if node.Status == compute_v1alpha.READY {
			nodes = append(nodes, node)
		}
	}

	if len(nodes) == 0 {
		c.log.Error("no nodes available for scheduling", "sandbox", sandbox.ID)
		return nil
	}

	var assignedNode *compute_v1alpha.Node

	if c.isStateful(sandbox) {
		// Stateful sandboxes must run on the coordinator (for disk access)
		assignedNode = c.findCoordinatorNode(nodes)
		if assignedNode == nil {
			for _, node := range nodes {
				c.log.Debug("node observed when looking for coordinator", "node", node.ID, "constraints", node.Constraints)
			}
			c.log.Error("no coordinator node available for stateful sandbox", "sandbox", sandbox.ID, "nodes", len(nodes))
			return nil
		}
		c.log.Debug("scheduling stateful sandbox to coordinator",
			"sandbox", sandbox.ID,
			"node", assignedNode.ID)
	} else {
		// Stateless sandboxes prefer runner nodes when available
		runnerNodes := c.filterRunnerNodes(nodes)
		if len(runnerNodes) > 0 {
			assignedNode = runnerNodes[rand.Intn(len(runnerNodes))]
			c.log.Debug("scheduling stateless sandbox to runner",
				"sandbox", sandbox.ID,
				"node", assignedNode.ID)
		} else {
			// Fall back to any available node (including coordinator)
			assignedNode = nodes[rand.Intn(len(nodes))]
			c.log.Debug("scheduling stateless sandbox to available node (no runners)",
				"sandbox", sandbox.ID,
				"node", assignedNode.ID)
		}
	}

	c.log.Info("assigning sandbox to node",
		"sandbox", sandbox.ID,
		"node", assignedNode.ID,
		"stateful", c.isStateful(sandbox))

	// Add schedule key to the entity
	schedule := compute_v1alpha.Schedule{
		Key: compute_v1alpha.Key{
			Kind: compute_v1alpha.KindSandbox,
			Node: assignedNode.ID,
		},
	}

	if err := meta.Update(schedule.Encode()); err != nil {
		c.log.Error("failed to update sandbox with schedule", "error", err)
		return err
	}

	return nil
}

// isStateful determines if a sandbox requires node affinity.
// Any sandbox with volumes is considered stateful because all current volume
// types (miren disks, local storage, host bind mounts) are node-local and
// can't float between runners.
func (c *Controller) isStateful(sandbox *compute_v1alpha.Sandbox) bool {
	return len(sandbox.Spec.Volume) > 0 || len(sandbox.Volume) > 0
}

// findCoordinatorNode finds the coordinator node among the available nodes.
// The coordinator is identified by having a "role=coordinator" constraint label.
func (c *Controller) findCoordinatorNode(nodes []*compute_v1alpha.Node) *compute_v1alpha.Node {
	for _, node := range nodes {
		if role, ok := node.Constraints.Get("role"); ok && role == "coordinator" {
			return node
		}
	}
	return nil
}

// filterRunnerNodes returns only nodes that are distributed runners (not the coordinator).
func (c *Controller) filterRunnerNodes(nodes []*compute_v1alpha.Node) []*compute_v1alpha.Node {
	var runners []*compute_v1alpha.Node
	for _, node := range nodes {
		if role, ok := node.Constraints.Get("role"); !ok || role != "coordinator" {
			runners = append(runners, node)
		}
	}
	return runners
}

// gatherNodes fetches all node entities from the entity store
func (c *Controller) gatherNodes(ctx context.Context) ([]*compute_v1alpha.Node, error) {
	results, err := c.eac.List(ctx, entity.Ref(entity.EntityKind, compute_v1alpha.KindNode))
	if err != nil {
		return nil, err
	}

	entities := results.Values()

	var ret []*compute_v1alpha.Node
	for _, ent := range entities {
		var node compute_v1alpha.Node
		node.Decode(ent.Entity())
		ret = append(ret, &node)
	}

	c.log.Debug("gathered nodes", "count", len(ret))
	return ret, nil
}
