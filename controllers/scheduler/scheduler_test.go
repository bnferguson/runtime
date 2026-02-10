package scheduler

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"miren.dev/runtime/api/compute/compute_v1alpha"
	"miren.dev/runtime/pkg/controller"
	"miren.dev/runtime/pkg/entity"
	"miren.dev/runtime/pkg/entity/testutils"
	"miren.dev/runtime/pkg/entity/types"
)

// reconcileSandbox is a test helper that processes a sandbox through the real controller framework.
// It creates a ReconcileController and calls ProcessEventForTest, which runs the exact same code
// path as production: handler invocation, diff calculation, and Patch application.
func reconcileSandbox(t *testing.T, ctx context.Context, server *testutils.InMemEntityServer, scheduler *Controller, sandboxID entity.Id) {
	t.Helper()

	// Create a real ReconcileController - this gives us the exact production code path
	rc := controller.NewReconcileController(
		"test-scheduler",
		testutils.TestLogger(t),
		entity.Ref(entity.EntityKind, compute_v1alpha.KindSandbox),
		server.EAC,
		controller.AdaptReconcileController[compute_v1alpha.Sandbox](scheduler),
		0, // resync period (not used for ProcessEventForTest)
		1, // workers (not used for ProcessEventForTest)
	)

	// Fetch current entity state
	resp, err := server.EAC.Get(ctx, sandboxID.String())
	require.NoError(t, err)

	// Create an event like the controller framework would
	event := controller.Event{
		Type:   controller.EventAdded,
		Id:     sandboxID,
		Entity: resp.Entity().Entity(),
	}

	// ProcessEventForTest runs processItem + applyUpdates - the exact production code path
	err = rc.ProcessEventForTest(ctx, event)
	require.NoError(t, err)
}

// TestSchedulerAssignsUnscheduledSandbox tests that the scheduler assigns
// a node to a sandbox that doesn't have a ScheduleKey
func TestSchedulerAssignsUnscheduledSandbox(t *testing.T) {
	ctx := context.Background()
	log := testutils.TestLogger(t)

	server, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	// Create a ready node
	node := &compute_v1alpha.Node{
		Status: compute_v1alpha.READY,
	}
	nodeID, err := server.Client.Create(ctx, "test-node", node)
	require.NoError(t, err)

	// Create scheduler and initialize (gathers nodes)
	scheduler := NewController(log, server.EAC)
	err = scheduler.Init(ctx)
	require.NoError(t, err)

	// Create an unscheduled sandbox
	sandbox := &compute_v1alpha.Sandbox{
		Status: compute_v1alpha.PENDING,
		Spec: compute_v1alpha.SandboxSpec{
			Container: []compute_v1alpha.SandboxSpecContainer{
				{Image: "test:latest"},
			},
		},
	}
	sandboxID, err := server.Client.Create(ctx, "test-sandbox", sandbox)
	require.NoError(t, err)

	// Run reconciliation through the real controller framework
	reconcileSandbox(t, ctx, server, scheduler, sandboxID)

	// Fetch the updated sandbox and verify it was assigned to the node
	resp, err := server.EAC.Get(ctx, sandboxID.String())
	require.NoError(t, err)

	var schedule compute_v1alpha.Schedule
	schedule.Decode(resp.Entity().Entity())

	assert.Equal(t, nodeID, schedule.Key.Node, "sandbox should be assigned to our node")
	assert.Equal(t, compute_v1alpha.KindSandbox, schedule.Key.Kind, "schedule key should have sandbox kind")
}

// TestSchedulerSkipsAlreadyScheduledSandbox tests that the scheduler
// doesn't re-assign a sandbox that already has a ScheduleKey
func TestSchedulerSkipsAlreadyScheduledSandbox(t *testing.T) {
	ctx := context.Background()
	log := testutils.TestLogger(t)

	server, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	// Create two ready nodes
	node1 := &compute_v1alpha.Node{Status: compute_v1alpha.READY}
	node1ID, err := server.Client.Create(ctx, "node-1", node1)
	require.NoError(t, err)

	node2 := &compute_v1alpha.Node{Status: compute_v1alpha.READY}
	_, err = server.Client.Create(ctx, "node-2", node2)
	require.NoError(t, err)

	// Create scheduler and initialize
	scheduler := NewController(log, server.EAC)
	err = scheduler.Init(ctx)
	require.NoError(t, err)

	// Create a sandbox that's already scheduled to node1
	sandbox := &compute_v1alpha.Sandbox{
		Status: compute_v1alpha.PENDING,
		Spec: compute_v1alpha.SandboxSpec{
			Container: []compute_v1alpha.SandboxSpecContainer{
				{Image: "test:latest"},
			},
		},
	}
	sandboxID, err := server.Client.Create(ctx, "test-sandbox", sandbox)
	require.NoError(t, err)

	// Manually add the schedule key to simulate already-scheduled sandbox
	schedule := compute_v1alpha.Schedule{
		Key: compute_v1alpha.Key{
			Kind: compute_v1alpha.KindSandbox,
			Node: node1ID,
		},
	}

	resp, err := server.EAC.Get(ctx, sandboxID.String())
	require.NoError(t, err)

	ent := resp.Entity().Entity()
	err = ent.Update(schedule.Encode())
	require.NoError(t, err)

	server.Store.AddEntity(sandboxID, ent)

	// Run reconciliation - should not change the assignment
	reconcileSandbox(t, ctx, server, scheduler, sandboxID)

	// Verify the sandbox is still assigned to node1 (not reassigned)
	resp, err = server.EAC.Get(ctx, sandboxID.String())
	require.NoError(t, err)

	var updatedSchedule compute_v1alpha.Schedule
	updatedSchedule.Decode(resp.Entity().Entity())
	assert.Equal(t, node1ID, updatedSchedule.Key.Node, "sandbox should still be assigned to node1")
}

// TestSchedulerNoAvailableNodes tests that the scheduler handles
// the case where no nodes are available (all not ready or none exist)
func TestSchedulerNoAvailableNodes(t *testing.T) {
	ctx := context.Background()
	log := testutils.TestLogger(t)

	server, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	// Create a node that's not ready
	node := &compute_v1alpha.Node{
		Status: compute_v1alpha.DISABLED,
	}
	_, err := server.Client.Create(ctx, "disabled-node", node)
	require.NoError(t, err)

	// Create scheduler and initialize
	scheduler := NewController(log, server.EAC)
	err = scheduler.Init(ctx)
	require.NoError(t, err)

	// Create an unscheduled sandbox
	sandbox := &compute_v1alpha.Sandbox{
		Status: compute_v1alpha.PENDING,
	}
	sandboxID, err := server.Client.Create(ctx, "test-sandbox", sandbox)
	require.NoError(t, err)

	// Run reconciliation - should not error, just not assign
	reconcileSandbox(t, ctx, server, scheduler, sandboxID)

	// Verify the sandbox was NOT assigned (no schedule key added)
	resp, err := server.EAC.Get(ctx, sandboxID.String())
	require.NoError(t, err)

	_, ok := resp.Entity().Entity().Get(compute_v1alpha.ScheduleKeyId)
	assert.False(t, ok, "sandbox should not have schedule key when no nodes available")
}

// TestSchedulerMultipleNodes tests that the scheduler can assign
// sandboxes when multiple ready nodes are available
func TestSchedulerMultipleNodes(t *testing.T) {
	ctx := context.Background()
	log := testutils.TestLogger(t)

	server, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	// Create multiple ready nodes
	nodeIDs := make(map[entity.Id]bool)
	for i := 0; i < 3; i++ {
		node := &compute_v1alpha.Node{Status: compute_v1alpha.READY}
		nodeID, err := server.Client.Create(ctx, "", node)
		require.NoError(t, err)
		nodeIDs[nodeID] = true
	}

	// Create scheduler and initialize
	scheduler := NewController(log, server.EAC)
	err := scheduler.Init(ctx)
	require.NoError(t, err)

	// Create and schedule multiple sandboxes
	for i := 0; i < 5; i++ {
		sandbox := &compute_v1alpha.Sandbox{
			Status: compute_v1alpha.PENDING,
		}
		sandboxID, err := server.Client.Create(ctx, "", sandbox)
		require.NoError(t, err)

		reconcileSandbox(t, ctx, server, scheduler, sandboxID)

		// Fetch and verify assigned to one of our nodes
		resp, err := server.EAC.Get(ctx, sandboxID.String())
		require.NoError(t, err)

		var schedule compute_v1alpha.Schedule
		schedule.Decode(resp.Entity().Entity())
		assert.True(t, nodeIDs[schedule.Key.Node], "sandbox should be assigned to one of our nodes")
	}
}

// TestSchedulerStatefulSandboxGoesToCoordinator tests that stateful sandboxes
// (those with miren disk volumes) are scheduled to the coordinator node
func TestSchedulerStatefulSandboxGoesToCoordinator(t *testing.T) {
	ctx := context.Background()
	log := testutils.TestLogger(t)

	server, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	// Create a coordinator node (role=coordinator constraint)
	coordNode := &compute_v1alpha.Node{
		Status:      compute_v1alpha.READY,
		Constraints: types.LabelSet("role", "coordinator"),
	}
	coordNodeID, err := server.Client.Create(ctx, "coordinator", coordNode)
	require.NoError(t, err)

	// Create a runner node
	runnerNode := &compute_v1alpha.Node{
		Status:   compute_v1alpha.READY,
		RunnerId: "550e8400-e29b-41d4-a716-446655440000",
	}
	_, err = server.Client.Create(ctx, "runner", runnerNode)
	require.NoError(t, err)

	// Create scheduler and initialize
	scheduler := NewController(log, server.EAC)
	err = scheduler.Init(ctx)
	require.NoError(t, err)

	// Create a stateful sandbox (has miren disk volume)
	sandbox := &compute_v1alpha.Sandbox{
		Status: compute_v1alpha.PENDING,
		Spec: compute_v1alpha.SandboxSpec{
			Volume: []compute_v1alpha.SandboxSpecVolume{
				{
					Name:     "data",
					Provider: "miren",
					DiskName: "my-disk",
				},
			},
			Container: []compute_v1alpha.SandboxSpecContainer{
				{Image: "test:latest"},
			},
		},
	}
	sandboxID, err := server.Client.Create(ctx, "stateful-sandbox", sandbox)
	require.NoError(t, err)

	// Run reconciliation
	reconcileSandbox(t, ctx, server, scheduler, sandboxID)

	// Verify the stateful sandbox was assigned to the coordinator
	resp, err := server.EAC.Get(ctx, sandboxID.String())
	require.NoError(t, err)

	var schedule compute_v1alpha.Schedule
	schedule.Decode(resp.Entity().Entity())
	assert.Equal(t, coordNodeID, schedule.Key.Node, "stateful sandbox should be assigned to coordinator")
}

// TestSchedulerStatelessSandboxPrefersRunners tests that stateless sandboxes
// prefer runner nodes over the coordinator when runners are available
func TestSchedulerStatelessSandboxPrefersRunners(t *testing.T) {
	ctx := context.Background()
	log := testutils.TestLogger(t)

	server, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	// Create a coordinator node (role=coordinator constraint)
	coordNode := &compute_v1alpha.Node{
		Status:      compute_v1alpha.READY,
		Constraints: types.LabelSet("role", "coordinator"),
	}
	coordNodeID, err := server.Client.Create(ctx, "coordinator", coordNode)
	require.NoError(t, err)

	// Create multiple runner nodes
	runnerIDs := make(map[entity.Id]bool)
	for i := 0; i < 3; i++ {
		runnerNode := &compute_v1alpha.Node{
			Status:   compute_v1alpha.READY,
			RunnerId: "550e8400-e29b-41d4-a716-44665544000" + string(rune('0'+i)),
		}
		runnerID, err := server.Client.Create(ctx, "", runnerNode)
		require.NoError(t, err)
		runnerIDs[runnerID] = true
	}

	// Create scheduler and initialize
	scheduler := NewController(log, server.EAC)
	err = scheduler.Init(ctx)
	require.NoError(t, err)

	// Create and schedule multiple stateless sandboxes
	for i := 0; i < 10; i++ {
		sandbox := &compute_v1alpha.Sandbox{
			Status: compute_v1alpha.PENDING,
			Spec: compute_v1alpha.SandboxSpec{
				Container: []compute_v1alpha.SandboxSpecContainer{
					{Image: "test:latest"},
				},
			},
		}
		sandboxID, err := server.Client.Create(ctx, "", sandbox)
		require.NoError(t, err)

		reconcileSandbox(t, ctx, server, scheduler, sandboxID)

		// Verify the stateless sandbox was assigned to a runner, not the coordinator
		resp, err := server.EAC.Get(ctx, sandboxID.String())
		require.NoError(t, err)

		var schedule compute_v1alpha.Schedule
		schedule.Decode(resp.Entity().Entity())
		assert.True(t, runnerIDs[schedule.Key.Node], "stateless sandbox should be assigned to a runner")
		assert.NotEqual(t, coordNodeID, schedule.Key.Node, "stateless sandbox should NOT be assigned to coordinator")
	}
}

// TestSchedulerStatelessFallsBackToCoordinator tests that stateless sandboxes
// fall back to the coordinator when no runners are available
func TestSchedulerStatelessFallsBackToCoordinator(t *testing.T) {
	ctx := context.Background()
	log := testutils.TestLogger(t)

	server, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	// Create only a coordinator node (no runners)
	coordNode := &compute_v1alpha.Node{
		Status:      compute_v1alpha.READY,
		Constraints: types.LabelSet("role", "coordinator"),
	}
	coordNodeID, err := server.Client.Create(ctx, "coordinator", coordNode)
	require.NoError(t, err)

	// Create scheduler and initialize
	scheduler := NewController(log, server.EAC)
	err = scheduler.Init(ctx)
	require.NoError(t, err)

	// Create a stateless sandbox
	sandbox := &compute_v1alpha.Sandbox{
		Status: compute_v1alpha.PENDING,
		Spec: compute_v1alpha.SandboxSpec{
			Container: []compute_v1alpha.SandboxSpecContainer{
				{Image: "test:latest"},
			},
		},
	}
	sandboxID, err := server.Client.Create(ctx, "stateless-sandbox", sandbox)
	require.NoError(t, err)

	// Run reconciliation
	reconcileSandbox(t, ctx, server, scheduler, sandboxID)

	// Verify the stateless sandbox falls back to coordinator when no runners available
	resp, err := server.EAC.Get(ctx, sandboxID.String())
	require.NoError(t, err)

	var schedule compute_v1alpha.Schedule
	schedule.Decode(resp.Entity().Entity())
	assert.Equal(t, coordNodeID, schedule.Key.Node, "stateless sandbox should fall back to coordinator when no runners")
}

// TestIsStatefulDetection tests the isStateful helper function
func TestIsStatefulDetection(t *testing.T) {
	log := testutils.TestLogger(t)
	scheduler := NewController(log, nil)

	tests := []struct {
		name     string
		sandbox  *compute_v1alpha.Sandbox
		expected bool
	}{
		{
			name: "stateful - spec volume with miren provider",
			sandbox: &compute_v1alpha.Sandbox{
				Spec: compute_v1alpha.SandboxSpec{
					Volume: []compute_v1alpha.SandboxSpecVolume{
						{Name: "data", Provider: "miren"},
					},
				},
			},
			expected: true,
		},
		{
			name: "stateful - legacy volume with miren provider",
			sandbox: &compute_v1alpha.Sandbox{
				Volume: []compute_v1alpha.Volume{
					{Name: "data", Provider: "miren"},
				},
			},
			expected: true,
		},
		{
			name: "stateless - no volumes",
			sandbox: &compute_v1alpha.Sandbox{
				Spec: compute_v1alpha.SandboxSpec{
					Container: []compute_v1alpha.SandboxSpecContainer{
						{Image: "test:latest"},
					},
				},
			},
			expected: false,
		},
		{
			name: "stateless - volume with different provider",
			sandbox: &compute_v1alpha.Sandbox{
				Spec: compute_v1alpha.SandboxSpec{
					Volume: []compute_v1alpha.SandboxSpecVolume{
						{Name: "data", Provider: "local"},
					},
				},
			},
			expected: false,
		},
		{
			name: "stateful - mixed volumes with one miren",
			sandbox: &compute_v1alpha.Sandbox{
				Spec: compute_v1alpha.SandboxSpec{
					Volume: []compute_v1alpha.SandboxSpecVolume{
						{Name: "cache", Provider: "local"},
						{Name: "data", Provider: "miren"},
					},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scheduler.isStateful(tt.sandbox)
			assert.Equal(t, tt.expected, result)
		})
	}
}
