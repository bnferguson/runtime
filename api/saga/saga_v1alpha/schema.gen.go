package saga_v1alpha

import (
	entity "miren.dev/runtime/pkg/entity"
	schema "miren.dev/runtime/pkg/entity/schema"
)

const (
	SagaDefinitionNameId    = entity.Id("dev.miren.saga/saga.definition_name")
	SagaDefinitionVersionId = entity.Id("dev.miren.saga/saga.definition_version")
	SagaErrorId             = entity.Id("dev.miren.saga/saga.error")
	SagaExecutedActionsId   = entity.Id("dev.miren.saga/saga.executed_actions")
	SagaExecutionOrderId    = entity.Id("dev.miren.saga/saga.execution_order")
	SagaInitialInputsId     = entity.Id("dev.miren.saga/saga.initial_inputs")
	SagaStatusId            = entity.Id("dev.miren.saga/saga.status")
	SagaStatusPendingId     = entity.Id("dev.miren.saga/status.pending")
	SagaStatusRunningId     = entity.Id("dev.miren.saga/status.running")
	SagaStatusUndoingId     = entity.Id("dev.miren.saga/status.undoing")
	SagaStatusCompletedId   = entity.Id("dev.miren.saga/status.completed")
	SagaStatusFailedId      = entity.Id("dev.miren.saga/status.failed")
)

type Saga struct {
	ID                entity.Id  `json:"id"`
	DefinitionName    string     `cbor:"definition_name,omitempty" json:"definition_name,omitempty"`
	DefinitionVersion int64      `cbor:"definition_version,omitempty" json:"definition_version,omitempty"`
	Error             string     `cbor:"error,omitempty" json:"error,omitempty"`
	ExecutedActions   []byte     `cbor:"executed_actions,omitempty" json:"executed_actions,omitempty"`
	ExecutionOrder    []byte     `cbor:"execution_order,omitempty" json:"execution_order,omitempty"`
	InitialInputs     []byte     `cbor:"initial_inputs,omitempty" json:"initial_inputs,omitempty"`
	Status            SagaStatus `cbor:"status,omitempty" json:"status,omitempty"`
}

type SagaStatus string

const (
	PENDING   SagaStatus = "status.pending"
	RUNNING   SagaStatus = "status.running"
	UNDOING   SagaStatus = "status.undoing"
	COMPLETED SagaStatus = "status.completed"
	FAILED    SagaStatus = "status.failed"
)

var sagastatusFromId = map[entity.Id]SagaStatus{SagaStatusPendingId: PENDING, SagaStatusRunningId: RUNNING, SagaStatusUndoingId: UNDOING, SagaStatusCompletedId: COMPLETED, SagaStatusFailedId: FAILED}
var sagastatusToId = map[SagaStatus]entity.Id{PENDING: SagaStatusPendingId, RUNNING: SagaStatusRunningId, UNDOING: SagaStatusUndoingId, COMPLETED: SagaStatusCompletedId, FAILED: SagaStatusFailedId}

func (o *Saga) Decode(e entity.AttrGetter) {
	o.ID = entity.MustGet(e, entity.DBId).Value.Id()
	if a, ok := e.Get(SagaDefinitionNameId); ok && a.Value.Kind() == entity.KindString {
		o.DefinitionName = a.Value.String()
	}
	if a, ok := e.Get(SagaDefinitionVersionId); ok && a.Value.Kind() == entity.KindInt64 {
		o.DefinitionVersion = a.Value.Int64()
	}
	if a, ok := e.Get(SagaErrorId); ok && a.Value.Kind() == entity.KindString {
		o.Error = a.Value.String()
	}
	if a, ok := e.Get(SagaExecutedActionsId); ok && a.Value.Kind() == entity.KindBytes {
		o.ExecutedActions = a.Value.Bytes()
	}
	if a, ok := e.Get(SagaExecutionOrderId); ok && a.Value.Kind() == entity.KindBytes {
		o.ExecutionOrder = a.Value.Bytes()
	}
	if a, ok := e.Get(SagaInitialInputsId); ok && a.Value.Kind() == entity.KindBytes {
		o.InitialInputs = a.Value.Bytes()
	}
	if a, ok := e.Get(SagaStatusId); ok && a.Value.Kind() == entity.KindId {
		o.Status = sagastatusFromId[a.Value.Id()]
	}
}

func (o *Saga) Is(e entity.AttrGetter) bool {
	return entity.Is(e, KindSaga)
}

func (o *Saga) ShortKind() string {
	return "saga"
}

func (o *Saga) Kind() entity.Id {
	return KindSaga
}

func (o *Saga) EntityId() entity.Id {
	return o.ID
}

func (o *Saga) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.DefinitionName) {
		attrs = append(attrs, entity.String(SagaDefinitionNameId, o.DefinitionName))
	}
	if !entity.Empty(o.DefinitionVersion) {
		attrs = append(attrs, entity.Int64(SagaDefinitionVersionId, o.DefinitionVersion))
	}
	if !entity.Empty(o.Error) {
		attrs = append(attrs, entity.String(SagaErrorId, o.Error))
	}
	if len(o.ExecutedActions) > 0 {
		attrs = append(attrs, entity.Bytes(SagaExecutedActionsId, o.ExecutedActions))
	}
	if len(o.ExecutionOrder) > 0 {
		attrs = append(attrs, entity.Bytes(SagaExecutionOrderId, o.ExecutionOrder))
	}
	if len(o.InitialInputs) > 0 {
		attrs = append(attrs, entity.Bytes(SagaInitialInputsId, o.InitialInputs))
	}
	if a, ok := sagastatusToId[o.Status]; ok {
		attrs = append(attrs, entity.Ref(SagaStatusId, a))
	}
	attrs = append(attrs, entity.Ref(entity.EntityKind, KindSaga))
	return
}

func (o *Saga) Empty() bool {
	if !entity.Empty(o.DefinitionName) {
		return false
	}
	if !entity.Empty(o.DefinitionVersion) {
		return false
	}
	if !entity.Empty(o.Error) {
		return false
	}
	if len(o.ExecutedActions) > 0 {
		return false
	}
	if len(o.ExecutionOrder) > 0 {
		return false
	}
	if len(o.InitialInputs) > 0 {
		return false
	}
	if o.Status != "" {
		return false
	}
	return true
}

func (o *Saga) InitSchema(sb *schema.SchemaBuilder) {
	sb.String("definition_name", "dev.miren.saga/saga.definition_name", schema.Doc("The name of the registered saga definition"), schema.Indexed)
	sb.Int64("definition_version", "dev.miren.saga/saga.definition_version", schema.Doc("The version of the definition when this execution started"))
	sb.String("error", "dev.miren.saga/saga.error", schema.Doc("Error message if the saga failed"))
	sb.Bytes("executed_actions", "dev.miren.saga/saga.executed_actions", schema.Doc("JSON-encoded map of action name to ActionResult"))
	sb.Bytes("execution_order", "dev.miren.saga/saga.execution_order", schema.Doc("JSON-encoded array of action names in execution order"))
	sb.Bytes("initial_inputs", "dev.miren.saga/saga.initial_inputs", schema.Doc("JSON-encoded initial inputs for the saga"))
	sb.Singleton("dev.miren.saga/status.pending")
	sb.Singleton("dev.miren.saga/status.running")
	sb.Singleton("dev.miren.saga/status.undoing")
	sb.Singleton("dev.miren.saga/status.completed")
	sb.Singleton("dev.miren.saga/status.failed")
	sb.Ref("status", "dev.miren.saga/saga.status", schema.Doc("Current execution status"), schema.Indexed, schema.Choices(SagaStatusPendingId, SagaStatusRunningId, SagaStatusUndoingId, SagaStatusCompletedId, SagaStatusFailedId))
}

var (
	KindSaga = entity.Id("dev.miren.saga/kind.saga")
	Schema   = entity.Id("dev.miren.saga/schema.v1alpha")
)

func init() {
	schema.Register("dev.miren.saga", "v1alpha", func(sb *schema.SchemaBuilder) {
		(&Saga{}).InitSchema(sb)
	})
	schema.RegisterEncodedSchema("dev.miren.saga", "v1alpha", []byte("\x1f\x8b\b\x00\x00\x00\x00\x00\x00\xff\x84\x93\xdfJ\xf40\x10\xc5_\xe3\xfbD\x11T\xbc\xac\xf8D%ۙd\x87M'!\x7fJ\xfa\x04>\x87\xb8\xfa\x88^K\x92\xe2b̮7e8s\xceo\xa6!9\x02\x8b\x19\x19p\x19frȃ\x17J\xe0\x81\x18\xfck\xfa\xf7S~\xcar\xa9>J\xca7\xedS\xf4S\x82\x99\x05qÕ\x92P\x83\x7fy\xdb\x11\xa4\xfbNz\x00\x94\xc4\x14\xc8\xf0\x98'\x941\xa6\x15\xc3jQ\xfa\xe0\x88U!=\xfeAZ\xd0y2\\`\xae\xa3g\xdeD\x1c\n\xec\x7f\x0f\x86\xce\x19W\xf2X\xcbv\x85\x87n*\xe1\x14\x03\xc2(\xa6<\xcf\x17\x80\xfd\xa5f\x16\xeeր\xfe\xfc\xb9\xd4P^\xda8\xc0\xba\x8ai\xc5\x06t\xd7\x03\x95\x7f\x17z$\xb61ԍ\xb8\xd1N\x98c\xc6\\\xf50>\x88\x10k\\nu\x8e\x01r\x9c\x0f\xf93.BG\xf4\xefR\n\xd2\b麥\x94\xd0P\xbb\xca\"\x03\xb1J7}\xd7\xd6V.2_\xb0mm\x15\x19\xcc\x05\xdb֦\xc9\xccVc@H\xb7}\xe3\xb7Am7E-\xcfB۽\xd0\xd6\xd1,\xdc:\xe6\xcb\x0e9\xd2:\x0e~o\\\x18\xeb;*\x8e\xf3\x8f\xe9\v\x00\x00\xff\xff\x01\x00\x00\xff\xff\x84\xdaF\x10\x83\x03\x00\x00"))
}
