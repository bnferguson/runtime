package saga

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test input/output types
type AddNumbersIn struct {
	A int `saga:"a"`
	B int `saga:"b"`
}

type AddNumbersOut struct {
	Sum int `saga:"sum"`
}

type MultiplyIn struct {
	Sum    int `saga:"sum"`
	Factor int `saga:"factor"`
}

type MultiplyOut struct {
	Result int `saga:"result"`
}

// Test controller to track calls
type testController struct {
	mu            sync.Mutex
	addCalls      []AddNumbersIn
	multiplyCalls []MultiplyIn
	undoAddCalls  []AddNumbersOut
	undoMultCalls []MultiplyOut
	failAdd       bool
	failMultiply  bool
}

func (c *testController) recordAdd(in AddNumbersIn) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.addCalls = append(c.addCalls, in)
}

func (c *testController) recordMultiply(in MultiplyIn) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.multiplyCalls = append(c.multiplyCalls, in)
}

func (c *testController) recordUndoAdd(out AddNumbersOut) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.undoAddCalls = append(c.undoAddCalls, out)
}

func (c *testController) recordUndoMult(out MultiplyOut) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.undoMultCalls = append(c.undoMultCalls, out)
}

// Action functions
func AddNumbers(ctx context.Context, in AddNumbersIn) (AddNumbersOut, error) {
	ctrl := From[*testController](ctx)
	ctrl.recordAdd(in)
	if ctrl.failAdd {
		return AddNumbersOut{}, errors.New("add failed")
	}
	return AddNumbersOut{Sum: in.A + in.B}, nil
}

func UndoAddNumbers(ctx context.Context, in AddNumbersIn, out AddNumbersOut) error {
	ctrl := From[*testController](ctx)
	ctrl.recordUndoAdd(out)
	return nil
}

func Multiply(ctx context.Context, in MultiplyIn) (MultiplyOut, error) {
	ctrl := From[*testController](ctx)
	ctrl.recordMultiply(in)
	if ctrl.failMultiply {
		return MultiplyOut{}, errors.New("multiply failed")
	}
	return MultiplyOut{Result: in.Sum * in.Factor}, nil
}

func UndoMultiply(ctx context.Context, in MultiplyIn, out MultiplyOut) error {
	ctrl := From[*testController](ctx)
	ctrl.recordUndoMult(out)
	return nil
}

// In-memory storage for testing
type memoryStorage struct {
	mu         sync.Mutex
	executions map[string]*Execution
}

func newMemoryStorage() *memoryStorage {
	return &memoryStorage{
		executions: make(map[string]*Execution),
	}
}

func (m *memoryStorage) Save(ctx context.Context, exec *Execution) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Deep copy to simulate real storage
	copied := *exec
	copied.ExecutedActions = make(map[string]*ActionResult)
	for k, v := range exec.ExecutedActions {
		copiedResult := *v
		copied.ExecutedActions[k] = &copiedResult
	}
	copied.ExecutionOrder = make([]string, len(exec.ExecutionOrder))
	copy(copied.ExecutionOrder, exec.ExecutionOrder)
	m.executions[exec.ID] = &copied
	return nil
}

func (m *memoryStorage) Get(ctx context.Context, id string) (*Execution, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	exec, ok := m.executions[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return exec, nil
}

func (m *memoryStorage) ListIncomplete(ctx context.Context) ([]*Execution, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*Execution
	for _, exec := range m.executions {
		if exec.Status == StatusRunning || exec.Status == StatusUndoing {
			result = append(result, exec)
		}
	}
	return result, nil
}

func TestBuilder_SingleAction(t *testing.T) {
	registry := NewRegistry()

	ctrl := &testController{}
	err := Define("single-action").
		With(ctrl).
		Action("add", AddNumbers).Undo(UndoAddNumbers).
		RegisterTo(registry)
	require.NoError(t, err)

	def, ok := registry.Get("single-action")
	require.True(t, ok)
	assert.Equal(t, "single-action", def.Name)
	assert.Len(t, def.Actions, 1)
	assert.Contains(t, def.Actions, "add")
}

func TestBuilder_MultipleActionsWithDependencies(t *testing.T) {
	registry := NewRegistry()

	ctrl := &testController{}
	err := Define("calc").
		With(ctrl).
		Action("add", AddNumbers).Undo(UndoAddNumbers).
		Action("multiply", Multiply).Undo(UndoMultiply).
		RegisterTo(registry)
	require.NoError(t, err)

	def, ok := registry.Get("calc")
	require.True(t, ok)
	assert.Len(t, def.Actions, 2)

	// Multiply depends on add (because it needs "sum")
	multNode := def.Actions["multiply"]
	assert.Contains(t, multNode.Dependencies, "add")

	// Execution order should have add before multiply
	addIdx := -1
	multIdx := -1
	for i, name := range def.executionOrder {
		if name == "add" {
			addIdx = i
		}
		if name == "multiply" {
			multIdx = i
		}
	}
	assert.True(t, addIdx < multIdx, "add should come before multiply")
}

func TestBuilder_DuplicateOutputsError(t *testing.T) {
	ctrl := &testController{}
	// Both actions produce "sum"
	_, err := Define("duplicate").
		With(ctrl).
		Action("add1", AddNumbers).Undo(UndoAddNumbers).
		Action("add2", AddNumbers).Undo(UndoAddNumbers).
		Build()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sum")
}

func TestExecutor_Success(t *testing.T) {
	registry := NewRegistry()

	ctrl := &testController{}
	err := Define("calc").
		With(ctrl).
		Action("add", AddNumbers).Undo(UndoAddNumbers).
		Action("multiply", Multiply).Undo(UndoMultiply).
		RegisterTo(registry)
	require.NoError(t, err)

	storage := newMemoryStorage()
	executor := NewExecutor(storage, WithRegistry(registry))

	ctx := context.Background()
	err = executor.Start("calc").
		With("a", 2).
		With("b", 3).
		With("factor", 4).
		WithID("test-exec-1").
		Execute(ctx)
	require.NoError(t, err)

	// Verify actions were called
	assert.Len(t, ctrl.addCalls, 1)
	assert.Equal(t, AddNumbersIn{A: 2, B: 3}, ctrl.addCalls[0])

	assert.Len(t, ctrl.multiplyCalls, 1)
	assert.Equal(t, MultiplyIn{Sum: 5, Factor: 4}, ctrl.multiplyCalls[0])

	// Verify no undos
	assert.Empty(t, ctrl.undoAddCalls)
	assert.Empty(t, ctrl.undoMultCalls)

	// Verify final state
	exec, err := storage.Get(ctx, "test-exec-1")
	require.NoError(t, err)
	assert.Equal(t, StatusCompleted, exec.Status)
	assert.Len(t, exec.ExecutionOrder, 2)
}

func TestExecutor_FailureAndUndo(t *testing.T) {
	registry := NewRegistry()

	ctrl := &testController{failMultiply: true}
	err := Define("calc-fail").
		With(ctrl).
		Action("add", AddNumbers).Undo(UndoAddNumbers).
		Action("multiply", Multiply).Undo(UndoMultiply).
		RegisterTo(registry)
	require.NoError(t, err)

	storage := newMemoryStorage()
	executor := NewExecutor(storage, WithRegistry(registry))

	ctx := context.Background()
	err = executor.Start("calc-fail").
		With("a", 2).
		With("b", 3).
		With("factor", 4).
		WithID("test-exec-2").
		Execute(ctx)
	require.Error(t, err)

	// Verify add was called
	assert.Len(t, ctrl.addCalls, 1)

	// Verify multiply was attempted
	assert.Len(t, ctrl.multiplyCalls, 1)

	// Verify undo was called for add (not multiply since it failed)
	assert.Len(t, ctrl.undoAddCalls, 1)
	assert.Equal(t, AddNumbersOut{Sum: 5}, ctrl.undoAddCalls[0])

	// Multiply doesn't produce output on failure, so no undo
	assert.Empty(t, ctrl.undoMultCalls)

	// Verify final state
	exec, err := storage.Get(ctx, "test-exec-2")
	require.NoError(t, err)
	assert.Equal(t, StatusFailed, exec.Status)
}

func TestExecutor_Recovery(t *testing.T) {
	registry := NewRegistry()

	ctrl := &testController{}
	err := Define("recoverable").
		With(ctrl).
		Action("add", AddNumbers).Undo(UndoAddNumbers).
		Action("multiply", Multiply).Undo(UndoMultiply).
		RegisterTo(registry)
	require.NoError(t, err)

	storage := newMemoryStorage()

	// Simulate a crashed execution
	// Note: Output uses uppercase "Sum" because Go's json.Marshal uses field names as-is
	crashedExec := &Execution{
		ID:                "crashed-exec",
		DefinitionName:    "recoverable",
		DefinitionVersion: 1,
		InitialInputs:     map[string]any{"a": float64(2), "b": float64(3), "factor": float64(4)},
		Status:            StatusRunning,
		ExecutedActions: map[string]*ActionResult{
			"add": {
				Output:     []byte(`{"Sum":5}`),
				ExecutedAt: time.Now(),
			},
		},
		ExecutionOrder: []string{"add"},
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	storage.Save(context.Background(), crashedExec)

	// Create new executor and recover
	executor := NewExecutor(storage, WithRegistry(registry))
	err = executor.Recover(context.Background())
	require.NoError(t, err)

	// Verify only multiply was called (add was already done)
	assert.Empty(t, ctrl.addCalls) // add not called again
	assert.Len(t, ctrl.multiplyCalls, 1)

	// Verify final state
	exec, err := storage.Get(context.Background(), "crashed-exec")
	require.NoError(t, err)
	assert.Equal(t, StatusCompleted, exec.Status)
}

func TestExecutor_ContextCancellation(t *testing.T) {
	registry := NewRegistry()

	ctrl := &testController{}
	err := Define("cancellable").
		With(ctrl).
		Action("add", AddNumbers).Undo(UndoAddNumbers).
		Action("multiply", Multiply).Undo(UndoMultiply).
		RegisterTo(registry)
	require.NoError(t, err)

	storage := newMemoryStorage()
	executor := NewExecutor(storage, WithRegistry(registry))

	// Create an already-cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = executor.Start("cancellable").
		With("a", 2).
		With("b", 3).
		With("factor", 4).
		WithID("cancel-exec").
		Execute(ctx)

	// Should return an error due to cancellation
	require.Error(t, err)
	assert.Contains(t, err.Error(), "interrupted")

	// Verify saga is left in Running state (no actions executed)
	exec, err := storage.Get(context.Background(), "cancel-exec")
	require.NoError(t, err)
	assert.Equal(t, StatusRunning, exec.Status)
	assert.Empty(t, exec.ExecutionOrder)

	// No actions should have been called
	assert.Empty(t, ctrl.addCalls)
	assert.Empty(t, ctrl.multiplyCalls)
}

func TestFrom_Panic(t *testing.T) {
	ctx := context.Background()
	assert.Panics(t, func() {
		From[*testController](ctx)
	})
}

func TestTryFrom(t *testing.T) {
	ctx := context.Background()

	// Without dependency
	ctrl, ok := TryFrom[*testController](ctx)
	assert.False(t, ok)
	assert.Nil(t, ctrl)

	// With dependency
	ctrl = &testController{}
	ctx = injectDependencies(ctx, []any{ctrl})
	got, ok := TryFrom[*testController](ctx)
	assert.True(t, ok)
	assert.Same(t, ctrl, got)
}

func TestInputs_Get(t *testing.T) {
	initial := map[string]any{
		"name":  "test",
		"count": float64(42),
	}
	outputs := map[string]json.RawMessage{
		"result": json.RawMessage(`"success"`),
	}

	inputs := newInputs(initial, outputs)

	// Get from initial
	var name string
	err := inputs.Get("name", &name)
	require.NoError(t, err)
	assert.Equal(t, "test", name)

	// Get from outputs (takes precedence)
	var result string
	err = inputs.Get("result", &result)
	require.NoError(t, err)
	assert.Equal(t, "success", result)

	// Missing key
	var missing string
	err = inputs.Get("missing", &missing)
	require.Error(t, err)
}

func TestInputs_Has(t *testing.T) {
	initial := map[string]any{"a": 1}
	outputs := map[string]json.RawMessage{"b": json.RawMessage("2")}

	inputs := newInputs(initial, outputs)

	assert.True(t, inputs.Has("a"))
	assert.True(t, inputs.Has("b"))
	assert.False(t, inputs.Has("c"))
}

func TestInputs_Keys(t *testing.T) {
	initial := map[string]any{"a": 1, "b": 2}
	outputs := map[string]json.RawMessage{"b": json.RawMessage("3"), "c": json.RawMessage("4")}

	inputs := newInputs(initial, outputs)
	keys := inputs.Keys()

	assert.Len(t, keys, 3)
	assert.Contains(t, keys, "a")
	assert.Contains(t, keys, "b")
	assert.Contains(t, keys, "c")
}

// Test types for optional input testing
type OptionalIn struct {
	Required int `saga:"required"`
	Optional int `saga:"optional,optional"`
}

type OptionalOut struct {
	Result int `saga:"result"`
}

func OptionalAction(ctx context.Context, in OptionalIn) (OptionalOut, error) {
	return OptionalOut{Result: in.Required + in.Optional}, nil
}

func UndoOptionalAction(ctx context.Context, in OptionalIn, out OptionalOut) error {
	return nil
}

func TestExecutor_MissingRequiredInput(t *testing.T) {
	registry := NewRegistry()

	err := Define("required-test").
		Action("optional-action", OptionalAction).Undo(UndoOptionalAction).
		RegisterTo(registry)
	require.NoError(t, err)

	storage := newMemoryStorage()
	executor := NewExecutor(storage, WithRegistry(registry))

	ctx := context.Background()
	// Missing "required" input should cause an error
	err = executor.Start("required-test").
		With("optional", 10).
		WithID("missing-required").
		Execute(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required input")
	assert.Contains(t, err.Error(), "required")
}

func TestExecutor_OptionalInput(t *testing.T) {
	registry := NewRegistry()

	err := Define("optional-test").
		Action("optional-action", OptionalAction).Undo(UndoOptionalAction).
		RegisterTo(registry)
	require.NoError(t, err)

	storage := newMemoryStorage()
	executor := NewExecutor(storage, WithRegistry(registry))

	ctx := context.Background()
	// Missing "optional" input should use zero value (0)
	err = executor.Start("optional-test").
		With("required", 5).
		WithID("optional-missing").
		Execute(ctx)
	require.NoError(t, err)

	// Verify execution completed
	exec, err := storage.Get(ctx, "optional-missing")
	require.NoError(t, err)
	assert.Equal(t, StatusCompleted, exec.Status)

	// Result should be 5 + 0 = 5
	var output OptionalOut
	err = json.Unmarshal(exec.ExecutedActions["optional-action"].Output, &output)
	require.NoError(t, err)
	assert.Equal(t, 5, output.Result)
}
