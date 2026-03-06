package saga

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	saga_v1alpha "miren.dev/runtime/api/saga/saga_v1alpha"
	"miren.dev/runtime/pkg/entity"

	es "miren.dev/runtime/api/entityserver/entityserver_v1alpha"
	"miren.dev/runtime/pkg/cond"
)

// EACStorage implements Storage using an EntityAccessClient RPC connection.
// This is used by runners which don't have direct entity.Store access.
type EACStorage struct {
	eac *es.EntityAccessClient
	log *slog.Logger
}

// NewEACStorage creates a storage backed by an EntityAccessClient.
func NewEACStorage(eac *es.EntityAccessClient, log *slog.Logger) *EACStorage {
	if log == nil {
		log = slog.Default()
	}
	return &EACStorage{eac: eac, log: log}
}

// Save persists the execution state as an entity via EAC.
func (s *EACStorage) Save(ctx context.Context, exec *Execution) error {
	initialInputs, err := json.Marshal(exec.InitialInputs)
	if err != nil {
		return fmt.Errorf("marshaling initial inputs: %w", err)
	}

	executedActions, err := json.Marshal(exec.ExecutedActions)
	if err != nil {
		return fmt.Errorf("marshaling executed actions: %w", err)
	}

	executionOrder, err := json.Marshal(exec.ExecutionOrder)
	if err != nil {
		return fmt.Errorf("marshaling execution order: %w", err)
	}

	status := statusToEntity(exec.Status)

	sagaEntity := &saga_v1alpha.Saga{
		ID:                entity.Id(exec.ID),
		DefinitionName:    exec.DefinitionName,
		DefinitionVersion: int64(exec.DefinitionVersion),
		ParentExecutionId: entity.Id(exec.ParentExecutionID),
		Status:            status,
		InitialInputs:     initialInputs,
		ExecutedActions:   executedActions,
		ExecutionOrder:    executionOrder,
		Error:             exec.Error,
	}

	ent := entity.New(
		entity.DBId, exec.ID,
		sagaEntity.Encode(),
	)

	_, err = s.eac.Ensure(ctx, ent.Attrs())
	if err != nil {
		return fmt.Errorf("saving saga entity via EAC: %w", err)
	}

	return nil
}

// Get retrieves an execution by ID via EAC.
func (s *EACStorage) Get(ctx context.Context, id string) (*Execution, error) {
	resp, err := s.eac.Get(ctx, id)
	if err != nil {
		if errors.Is(err, cond.ErrNotFound{}) {
			return nil, fmt.Errorf("%w: %s", ErrExecutionNotFound, id)
		}
		return nil, fmt.Errorf("getting saga entity via EAC: %w", err)
	}

	ent := resp.Entity().Entity()
	sagaEntity, ok := entity.As[saga_v1alpha.Saga](ent)
	if !ok {
		return nil, fmt.Errorf("entity %s is not a saga", id)
	}

	return entityToExecution(sagaEntity)
}

// ListIncomplete returns all executions that need recovery via EAC.
func (s *EACStorage) ListIncomplete(ctx context.Context) ([]*Execution, error) {
	// Query for each incomplete status
	var allEntities []*es.Entity

	for _, statusRef := range []entity.Id{
		saga_v1alpha.SagaStatusPendingId,
		saga_v1alpha.SagaStatusRunningId,
		saga_v1alpha.SagaStatusUndoingId,
	} {
		resp, err := s.eac.List(ctx, entity.Ref(
			saga_v1alpha.SagaStatusId,
			statusRef,
		))
		if err != nil {
			return nil, fmt.Errorf("listing sagas with status %s: %w", statusRef, err)
		}
		allEntities = append(allEntities, resp.Values()...)
	}

	if len(allEntities) == 0 {
		return nil, nil
	}

	var executions []*Execution
	for _, eacEnt := range allEntities {
		ent := eacEnt.Entity()
		sagaEntity, ok := entity.As[saga_v1alpha.Saga](ent)
		if !ok {
			s.log.Warn("entity is not a saga, skipping", "id", eacEnt.Id())
			continue
		}
		exec, err := entityToExecution(sagaEntity)
		if err != nil {
			s.log.Warn("failed to convert saga entity, skipping", "id", eacEnt.Id(), "error", err)
			continue
		}
		executions = append(executions, exec)
	}

	return executions, nil
}
