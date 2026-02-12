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
	SagaParentExecutionIdId = entity.Id("dev.miren.saga/saga.parent_execution_id")
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
	ParentExecutionId entity.Id  `cbor:"parent_execution_id,omitempty" json:"parent_execution_id,omitempty"`
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
	if a, ok := e.Get(SagaParentExecutionIdId); ok && a.Value.Kind() == entity.KindId {
		o.ParentExecutionId = a.Value.Id()
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
	if !entity.Empty(o.ParentExecutionId) {
		attrs = append(attrs, entity.Ref(SagaParentExecutionIdId, o.ParentExecutionId))
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
	if !entity.Empty(o.ParentExecutionId) {
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
	sb.Ref("parent_execution_id", "dev.miren.saga/saga.parent_execution_id", schema.Doc("Reference to the parent saga execution (set for child/nested sagas)"), schema.Indexed)
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
	schema.RegisterEncodedSchema("dev.miren.saga", "v1alpha", []byte("\x1f\x8b\b\x00\x00\x00\x00\x00\x00\xff\x84\x93oJ\x031\x10ů\xa1\xa2\b*\xfam\xc5\x13\x85tg\x92\x0e\xcdNB\xfe,\xdb\x1bx\r\xb1zD?K\x92\xc5bL\xeb\x97\x12\u07bc\xf7\xcb\xeb\x929\x00\xcb\t\x19p\x1e&\xf2\xc8C\x90Z\xe2\x8e\x18\xc2\xdbr\xf1[~\xcer9}\x96Th\xc6\xc7\xe8\x97\x02;I↫\x14\xa1\x81\xf0\xfa\xbe!X\xee;\xe9\x01P\x11S$\xcb\"\xdfP\xae\xb1\xad\x18\xf7\x0eU\x88\x9eX\x17\xd2\xe3?\xa4\x19} \xcb\x05\xe6;z\xe6\x8dı\xc0.{0\xf4\xde\xfa\x92\xc7zl+<tS\v\x8e)\"\b9\xe6\xfbB\x01\xb8?jf\xe1f\x1f1\x9c\xfe.5\x94K[\x0fX\xab\xd8Vl@w=P\xf9\xef\xd2\bb\x97bmč\xd6`\x9ez\x18'=r\x14\xc7\x06\x04\xf5I\xf4\x06\x19\xb8!8d\xdaU\x8f\x16\xa2\x8c\xa9\x96Q\xeb9g\x009M\xbb\xfc#fi\x12\x86\x0f\xa5$\x19\x84庥\x94\xd0P\xa7\xda!\x03\xb1^n\xfa\xaeu\xac}b>c[\xc7:1\xd83\xb6uL\xa3\x9d\x9c\xc1\x88\xb0\xdc\xf6\x8d?\x06\xbd\xbe;=\xbfH\xe3\xb6\xd28O\x93\xf4{\x91W\ar\xa4u\xec\xc2\xd6\xfa(\xeaV\x16\xc7\xe9\xd5\xfc\x06\x00\x00\xff\xff\x01\x00\x00\xff\xff\xce\xdck\x92\xd1\x03\x00\x00"))
}
