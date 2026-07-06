package scheduler

import (
	"context"
	"log/slog"
	"math/rand"

	"miren.dev/runtime/api/compute/compute_v1alpha"
	"miren.dev/runtime/api/core/core_v1alpha"
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

	// Find available READY nodes with a valid address that are schedulable.
	// All three conditions are required: status=READY (session-scoped, proves
	// the runner process is alive), a non-empty ApiAddress (proves the runner
	// has fully started and is reachable), and a schedulable scheduling state
	// (a persistent operator flag that survives runner restarts; the zero value
	// means schedulable, and anything other than SCHEDULABLE — e.g. cordoned —
	// keeps the node out of scheduling until explicitly uncordoned).
	var nodes []*compute_v1alpha.Node
	for _, node := range allNodes {
		schedulable := node.Scheduling == "" || node.Scheduling == compute_v1alpha.SCHEDULABLE
		if node.Status == compute_v1alpha.READY && node.ApiAddress != "" && schedulable {
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
		candidates := runnerNodes
		fallback := "runner"
		if len(candidates) == 0 {
			candidates = nodes
			fallback = "available node (no runners)"
		}

		assignedNode = c.pickWithAntiAffinity(ctx, meta, candidates)
		c.log.Debug("scheduling stateless sandbox to "+fallback,
			"sandbox", sandbox.ID,
			"node", assignedNode.ID)
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

// pickWithAntiAffinity picks a node from candidates that minimizes co-location
// with other replicas in the same pool. Sandboxes labelled `pool: <id>` (the
// label is set by the sandbox pool manager) are treated as siblings. Among
// candidates with the fewest active siblings, one is chosen randomly. Sandboxes
// without a pool label, or when sibling lookup fails, fall back to a uniform
// random pick — one-off sandboxes don't carry a spread guarantee.
//
// "Active" here means a sibling that is already scheduled and is not in a
// terminal status (STOPPED, DEAD). Terminal siblings have released their slot
// on the node and shouldn't influence placement.
func (c *Controller) pickWithAntiAffinity(ctx context.Context, meta *entity.Meta, candidates []*compute_v1alpha.Node) *compute_v1alpha.Node {
	if len(candidates) == 1 {
		return candidates[0]
	}

	var md core_v1alpha.Metadata
	md.Decode(meta)
	poolLabel, ok := md.Labels.Get("pool")
	if !ok || poolLabel == "" {
		return candidates[rand.Intn(len(candidates))]
	}

	counts, err := c.countSiblingsPerNode(ctx, poolLabel, meta.Id())
	if err != nil {
		c.log.Warn("anti-affinity sibling lookup failed, falling back to random",
			"pool", poolLabel, "error", err)
		return candidates[rand.Intn(len(candidates))]
	}

	minCount := -1
	var winners []*compute_v1alpha.Node
	for _, node := range candidates {
		n := counts[node.ID]
		if minCount == -1 || n < minCount {
			minCount = n
			winners = winners[:0]
			winners = append(winners, node)
		} else if n == minCount {
			winners = append(winners, node)
		}
	}

	return winners[rand.Intn(len(winners))]
}

// countSiblingsPerNode tallies active sandboxes in the same pool by their
// assigned node. Siblings are sandboxes carrying the same `pool` label, with
// a Schedule already attached, and not in a terminal status. The sandbox
// currently being scheduled is excluded from the count.
func (c *Controller) countSiblingsPerNode(ctx context.Context, poolLabel string, selfID entity.Id) (map[entity.Id]int, error) {
	results, err := c.eac.List(ctx, entity.Ref(entity.EntityKind, compute_v1alpha.KindSandbox))
	if err != nil {
		return nil, err
	}

	counts := make(map[entity.Id]int)
	for _, ent := range results.Values() {
		if entity.Id(ent.Id()) == selfID {
			continue
		}

		e := ent.Entity()

		var md core_v1alpha.Metadata
		md.Decode(e)
		if label, ok := md.Labels.Get("pool"); !ok || label != poolLabel {
			continue
		}

		var sibling compute_v1alpha.Sandbox
		sibling.Decode(e)
		if sibling.Status == compute_v1alpha.STOPPED || sibling.Status == compute_v1alpha.DEAD {
			continue
		}

		var schedule compute_v1alpha.Schedule
		schedule.Decode(e)
		if schedule.Key.Node == "" {
			continue
		}

		counts[schedule.Key.Node]++
	}

	return counts, nil
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
