package entityserver_v1alpha

// EntityOperation represents the type of operation on an entity
type EntityOperation int64

const (
	// EntityOperationCreate indicates an entity was created
	EntityOperationCreate EntityOperation = 1
	// EntityOperationUpdate indicates an entity was updated
	EntityOperationUpdate EntityOperation = 2
	// EntityOperationDelete indicates an entity was deleted
	EntityOperationDelete EntityOperation = 3
	// EntityOperationProgress is a watch watermark carrying the latest observed
	// revision with no entity payload. It lets a client advance its resume cursor
	// during idle periods (etcd progress notifications).
	EntityOperationProgress EntityOperation = 4
	// EntityOperationCompacted signals that the requested watch start revision has
	// been compacted away. The client must re-list to obtain a fresh snapshot and
	// resume from the new revision. Revision carries the compaction revision.
	EntityOperationCompacted EntityOperation = 5
)

// OperationType returns the typed operation for this EntityOp
func (v *EntityOp) OperationType() EntityOperation {
	return EntityOperation(v.Operation())
}

// IsCreate returns true if this is a create operation
func (v *EntityOp) IsCreate() bool {
	return v.Operation() == int64(EntityOperationCreate)
}

// IsUpdate returns true if this is an update operation
func (v *EntityOp) IsUpdate() bool {
	return v.Operation() == int64(EntityOperationUpdate)
}

// IsDelete returns true if this is a delete operation
func (v *EntityOp) IsDelete() bool {
	return v.Operation() == int64(EntityOperationDelete)
}

// IsProgress returns true if this is a watch progress watermark
func (v *EntityOp) IsProgress() bool {
	return v.Operation() == int64(EntityOperationProgress)
}

// IsCompacted returns true if this signals the watch start revision was compacted
func (v *EntityOp) IsCompacted() bool {
	return v.Operation() == int64(EntityOperationCompacted)
}
