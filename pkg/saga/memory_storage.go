package saga

import (
	"context"
	"errors"
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
		return nil, errors.New("execution not found")
	}
	return exec, nil
}

// ListIncomplete returns all executions that are still running or undoing.
func (m *MemoryStorage) ListIncomplete(ctx context.Context) ([]*Execution, error) {
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
