package saga

import (
	"context"
	"fmt"
	"sync"
)

// MemoryStorage is a simple in-memory storage implementation for testing and examples.
type MemoryStorage struct {
	mu         sync.Mutex
	executions map[string]*Execution
}

// NewMemoryStorage creates a new in-memory storage.
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		executions: make(map[string]*Execution),
	}
}

// Save persists an execution to memory.
func (m *MemoryStorage) Save(ctx context.Context, exec *Execution) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Deep copy to simulate real storage behavior
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

// Get retrieves an execution by ID.
func (m *MemoryStorage) Get(ctx context.Context, id string) (*Execution, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	exec, ok := m.executions[id]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrExecutionNotFound, id)
	}
	return exec, nil
}

// ListIncomplete returns all executions that need recovery.
// This includes pending (crashed before starting), running, and undoing sagas.
func (m *MemoryStorage) ListIncomplete(ctx context.Context) ([]*Execution, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var result []*Execution
	for _, exec := range m.executions {
		switch exec.Status {
		case StatusPending, StatusRunning, StatusUndoing:
			result = append(result, exec)
		case StatusCompleted, StatusFailed:
			// Terminal states are complete; skip them.
		}
	}
	return result, nil
}
