package addon_v1alpha

import (
	entity "miren.dev/runtime/pkg/entity"
	schema "miren.dev/runtime/pkg/entity/schema"
)

const (
	AddonDefaultPlanId = entity.Id("dev.miren.addon/addon.default_plan")
	AddonDescriptionId = entity.Id("dev.miren.addon/addon.description")
	AddonDisplayNameId = entity.Id("dev.miren.addon/addon.display_name")
	AddonNameId        = entity.Id("dev.miren.addon/addon.name")
	AddonPlansId       = entity.Id("dev.miren.addon/addon.plans")
)

type Addon struct {
	ID          entity.Id `json:"id"`
	DefaultPlan string    `cbor:"default_plan,omitempty" json:"default_plan,omitempty"`
	Description string    `cbor:"description,omitempty" json:"description,omitempty"`
	DisplayName string    `cbor:"display_name,omitempty" json:"display_name,omitempty"`
	Name        string    `cbor:"name,omitempty" json:"name,omitempty"`
	Plans       []Plans   `cbor:"plans,omitempty" json:"plans,omitempty"`
}

func (o *Addon) Decode(e entity.AttrGetter) {
	o.ID = entity.MustGet(e, entity.DBId).Value.Id()
	if a, ok := e.Get(AddonDefaultPlanId); ok && a.Value.Kind() == entity.KindString {
		o.DefaultPlan = a.Value.String()
	}
	if a, ok := e.Get(AddonDescriptionId); ok && a.Value.Kind() == entity.KindString {
		o.Description = a.Value.String()
	}
	if a, ok := e.Get(AddonDisplayNameId); ok && a.Value.Kind() == entity.KindString {
		o.DisplayName = a.Value.String()
	}
	if a, ok := e.Get(AddonNameId); ok && a.Value.Kind() == entity.KindString {
		o.Name = a.Value.String()
	}
	for _, a := range e.GetAll(AddonPlansId) {
		if a.Value.Kind() == entity.KindComponent {
			var v Plans
			v.Decode(a.Value.Component())
			o.Plans = append(o.Plans, v)
		}
	}
}

func (o *Addon) Is(e entity.AttrGetter) bool {
	return entity.Is(e, KindAddon)
}

func (o *Addon) ShortKind() string {
	return "addon"
}

func (o *Addon) Kind() entity.Id {
	return KindAddon
}

func (o *Addon) EntityId() entity.Id {
	return o.ID
}

func (o *Addon) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.DefaultPlan) {
		attrs = append(attrs, entity.String(AddonDefaultPlanId, o.DefaultPlan))
	}
	if !entity.Empty(o.Description) {
		attrs = append(attrs, entity.String(AddonDescriptionId, o.Description))
	}
	if !entity.Empty(o.DisplayName) {
		attrs = append(attrs, entity.String(AddonDisplayNameId, o.DisplayName))
	}
	if !entity.Empty(o.Name) {
		attrs = append(attrs, entity.String(AddonNameId, o.Name))
	}
	for _, v := range o.Plans {
		attrs = append(attrs, entity.Component(AddonPlansId, v.Encode()))
	}
	attrs = append(attrs, entity.Ref(entity.EntityKind, KindAddon))
	return
}

func (o *Addon) Empty() bool {
	if !entity.Empty(o.DefaultPlan) {
		return false
	}
	if !entity.Empty(o.Description) {
		return false
	}
	if !entity.Empty(o.DisplayName) {
		return false
	}
	if !entity.Empty(o.Name) {
		return false
	}
	if len(o.Plans) != 0 {
		return false
	}
	return true
}

func (o *Addon) InitSchema(sb *schema.SchemaBuilder) {
	sb.String("default_plan", "dev.miren.addon/addon.default_plan")
	sb.String("description", "dev.miren.addon/addon.description")
	sb.String("display_name", "dev.miren.addon/addon.display_name")
	sb.String("name", "dev.miren.addon/addon.name", schema.Indexed)
	sb.Component("plans", "dev.miren.addon/addon.plans", schema.Many)
	(&Plans{}).InitSchema(sb.Builder("addon.plans"))
}

const (
	PlansDescriptionId = entity.Id("dev.miren.addon/plans.description")
	PlansDetailsId     = entity.Id("dev.miren.addon/plans.details")
	PlansNameId        = entity.Id("dev.miren.addon/plans.name")
)

type Plans struct {
	Description string    `cbor:"description,omitempty" json:"description,omitempty"`
	Details     []Details `cbor:"details,omitempty" json:"details,omitempty"`
	Name        string    `cbor:"name,omitempty" json:"name,omitempty"`
}

func (o *Plans) Decode(e entity.AttrGetter) {
	if a, ok := e.Get(PlansDescriptionId); ok && a.Value.Kind() == entity.KindString {
		o.Description = a.Value.String()
	}
	for _, a := range e.GetAll(PlansDetailsId) {
		if a.Value.Kind() == entity.KindComponent {
			var v Details
			v.Decode(a.Value.Component())
			o.Details = append(o.Details, v)
		}
	}
	if a, ok := e.Get(PlansNameId); ok && a.Value.Kind() == entity.KindString {
		o.Name = a.Value.String()
	}
}

func (o *Plans) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.Description) {
		attrs = append(attrs, entity.String(PlansDescriptionId, o.Description))
	}
	for _, v := range o.Details {
		attrs = append(attrs, entity.Component(PlansDetailsId, v.Encode()))
	}
	if !entity.Empty(o.Name) {
		attrs = append(attrs, entity.String(PlansNameId, o.Name))
	}
	return
}

func (o *Plans) Empty() bool {
	if !entity.Empty(o.Description) {
		return false
	}
	if len(o.Details) != 0 {
		return false
	}
	if !entity.Empty(o.Name) {
		return false
	}
	return true
}

func (o *Plans) InitSchema(sb *schema.SchemaBuilder) {
	sb.String("description", "dev.miren.addon/plans.description")
	sb.Component("details", "dev.miren.addon/plans.details", schema.Many)
	(&Details{}).InitSchema(sb.Builder("plans.details"))
	sb.String("name", "dev.miren.addon/plans.name")
}

const (
	DetailsKeyId   = entity.Id("dev.miren.addon/details.key")
	DetailsValueId = entity.Id("dev.miren.addon/details.value")
)

type Details struct {
	Key   string `cbor:"key,omitempty" json:"key,omitempty"`
	Value string `cbor:"value,omitempty" json:"value,omitempty"`
}

func (o *Details) Decode(e entity.AttrGetter) {
	if a, ok := e.Get(DetailsKeyId); ok && a.Value.Kind() == entity.KindString {
		o.Key = a.Value.String()
	}
	if a, ok := e.Get(DetailsValueId); ok && a.Value.Kind() == entity.KindString {
		o.Value = a.Value.String()
	}
}

func (o *Details) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.Key) {
		attrs = append(attrs, entity.String(DetailsKeyId, o.Key))
	}
	if !entity.Empty(o.Value) {
		attrs = append(attrs, entity.String(DetailsValueId, o.Value))
	}
	return
}

func (o *Details) Empty() bool {
	if !entity.Empty(o.Key) {
		return false
	}
	if !entity.Empty(o.Value) {
		return false
	}
	return true
}

func (o *Details) InitSchema(sb *schema.SchemaBuilder) {
	sb.String("key", "dev.miren.addon/details.key")
	sb.String("value", "dev.miren.addon/details.value")
}

const (
	AddonAssociationAddonId        = entity.Id("dev.miren.addon/addon_association.addon")
	AddonAssociationAppId          = entity.Id("dev.miren.addon/addon_association.app")
	AddonAssociationErrorMessageId = entity.Id("dev.miren.addon/addon_association.error_message")
	AddonAssociationPlanId         = entity.Id("dev.miren.addon/addon_association.plan")
	AddonAssociationStatusId       = entity.Id("dev.miren.addon/addon_association.status")
	AddonAssociationVariablesId    = entity.Id("dev.miren.addon/addon_association.variables")
)

type AddonAssociation struct {
	ID           entity.Id   `json:"id"`
	Addon        entity.Id   `cbor:"addon,omitempty" json:"addon,omitempty"`
	App          entity.Id   `cbor:"app,omitempty" json:"app,omitempty"`
	ErrorMessage string      `cbor:"error_message,omitempty" json:"error_message,omitempty"`
	Plan         string      `cbor:"plan,omitempty" json:"plan,omitempty"`
	Status       string      `cbor:"status,omitempty" json:"status,omitempty"`
	Variables    []Variables `cbor:"variables,omitempty" json:"variables,omitempty"`
}

func (o *AddonAssociation) Decode(e entity.AttrGetter) {
	o.ID = entity.MustGet(e, entity.DBId).Value.Id()
	if a, ok := e.Get(AddonAssociationAddonId); ok && a.Value.Kind() == entity.KindId {
		o.Addon = a.Value.Id()
	}
	if a, ok := e.Get(AddonAssociationAppId); ok && a.Value.Kind() == entity.KindId {
		o.App = a.Value.Id()
	}
	if a, ok := e.Get(AddonAssociationErrorMessageId); ok && a.Value.Kind() == entity.KindString {
		o.ErrorMessage = a.Value.String()
	}
	if a, ok := e.Get(AddonAssociationPlanId); ok && a.Value.Kind() == entity.KindString {
		o.Plan = a.Value.String()
	}
	if a, ok := e.Get(AddonAssociationStatusId); ok && a.Value.Kind() == entity.KindString {
		o.Status = a.Value.String()
	}
	for _, a := range e.GetAll(AddonAssociationVariablesId) {
		if a.Value.Kind() == entity.KindComponent {
			var v Variables
			v.Decode(a.Value.Component())
			o.Variables = append(o.Variables, v)
		}
	}
}

func (o *AddonAssociation) Is(e entity.AttrGetter) bool {
	return entity.Is(e, KindAddonAssociation)
}

func (o *AddonAssociation) ShortKind() string {
	return "addon_association"
}

func (o *AddonAssociation) Kind() entity.Id {
	return KindAddonAssociation
}

func (o *AddonAssociation) EntityId() entity.Id {
	return o.ID
}

func (o *AddonAssociation) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.Addon) {
		attrs = append(attrs, entity.Ref(AddonAssociationAddonId, o.Addon))
	}
	if !entity.Empty(o.App) {
		attrs = append(attrs, entity.Ref(AddonAssociationAppId, o.App))
	}
	if !entity.Empty(o.ErrorMessage) {
		attrs = append(attrs, entity.String(AddonAssociationErrorMessageId, o.ErrorMessage))
	}
	if !entity.Empty(o.Plan) {
		attrs = append(attrs, entity.String(AddonAssociationPlanId, o.Plan))
	}
	if !entity.Empty(o.Status) {
		attrs = append(attrs, entity.String(AddonAssociationStatusId, o.Status))
	}
	for _, v := range o.Variables {
		attrs = append(attrs, entity.Component(AddonAssociationVariablesId, v.Encode()))
	}
	attrs = append(attrs, entity.Ref(entity.EntityKind, KindAddonAssociation))
	return
}

func (o *AddonAssociation) Empty() bool {
	if !entity.Empty(o.Addon) {
		return false
	}
	if !entity.Empty(o.App) {
		return false
	}
	if !entity.Empty(o.ErrorMessage) {
		return false
	}
	if !entity.Empty(o.Plan) {
		return false
	}
	if !entity.Empty(o.Status) {
		return false
	}
	if len(o.Variables) != 0 {
		return false
	}
	return true
}

func (o *AddonAssociation) InitSchema(sb *schema.SchemaBuilder) {
	sb.Ref("addon", "dev.miren.addon/addon_association.addon", schema.Indexed)
	sb.Ref("app", "dev.miren.addon/addon_association.app", schema.Indexed, schema.Tags("dev.miren.app_ref"))
	sb.String("error_message", "dev.miren.addon/addon_association.error_message")
	sb.String("plan", "dev.miren.addon/addon_association.plan")
	sb.String("status", "dev.miren.addon/addon_association.status", schema.Indexed)
	sb.Component("variables", "dev.miren.addon/addon_association.variables", schema.Many)
	(&Variables{}).InitSchema(sb.Builder("addon_association.variables"))
}

const (
	VariablesKeyId       = entity.Id("dev.miren.addon/variables.key")
	VariablesSensitiveId = entity.Id("dev.miren.addon/variables.sensitive")
	VariablesValueId     = entity.Id("dev.miren.addon/variables.value")
)

type Variables struct {
	Key       string `cbor:"key,omitempty" json:"key,omitempty"`
	Sensitive bool   `cbor:"sensitive,omitempty" json:"sensitive,omitempty"`
	Value     string `cbor:"value,omitempty" json:"value,omitempty"`
}

func (o *Variables) Decode(e entity.AttrGetter) {
	if a, ok := e.Get(VariablesKeyId); ok && a.Value.Kind() == entity.KindString {
		o.Key = a.Value.String()
	}
	if a, ok := e.Get(VariablesSensitiveId); ok && a.Value.Kind() == entity.KindBool {
		o.Sensitive = a.Value.Bool()
	}
	if a, ok := e.Get(VariablesValueId); ok && a.Value.Kind() == entity.KindString {
		o.Value = a.Value.String()
	}
}

func (o *Variables) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.Key) {
		attrs = append(attrs, entity.String(VariablesKeyId, o.Key))
	}
	attrs = append(attrs, entity.Bool(VariablesSensitiveId, o.Sensitive))
	if !entity.Empty(o.Value) {
		attrs = append(attrs, entity.String(VariablesValueId, o.Value))
	}
	return
}

func (o *Variables) Empty() bool {
	if !entity.Empty(o.Key) {
		return false
	}
	if !entity.Empty(o.Sensitive) {
		return false
	}
	if !entity.Empty(o.Value) {
		return false
	}
	return true
}

func (o *Variables) InitSchema(sb *schema.SchemaBuilder) {
	sb.String("key", "dev.miren.addon/variables.key")
	sb.Bool("sensitive", "dev.miren.addon/variables.sensitive")
	sb.String("value", "dev.miren.addon/variables.value")
}

const (
	PostgresServerAddonNameId         = entity.Id("dev.miren.addon/postgres_server.addon_name")
	PostgresServerAssociationCountId  = entity.Id("dev.miren.addon/postgres_server.association_count")
	PostgresServerPlanId              = entity.Id("dev.miren.addon/postgres_server.plan")
	PostgresServerSandboxPoolId       = entity.Id("dev.miren.addon/postgres_server.sandbox_pool")
	PostgresServerServiceId           = entity.Id("dev.miren.addon/postgres_server.service")
	PostgresServerStatusId            = entity.Id("dev.miren.addon/postgres_server.status")
	PostgresServerSuperuserPasswordId = entity.Id("dev.miren.addon/postgres_server.superuser_password")
)

type PostgresServer struct {
	ID                entity.Id `json:"id"`
	AddonName         string    `cbor:"addon_name,omitempty" json:"addon_name,omitempty"`
	AssociationCount  int64     `cbor:"association_count,omitempty" json:"association_count,omitempty"`
	Plan              string    `cbor:"plan,omitempty" json:"plan,omitempty"`
	SandboxPool       entity.Id `cbor:"sandbox_pool,omitempty" json:"sandbox_pool,omitempty"`
	Service           entity.Id `cbor:"service,omitempty" json:"service,omitempty"`
	Status            string    `cbor:"status,omitempty" json:"status,omitempty"`
	SuperuserPassword string    `cbor:"superuser_password,omitempty" json:"superuser_password,omitempty"`
}

func (o *PostgresServer) Decode(e entity.AttrGetter) {
	o.ID = entity.MustGet(e, entity.DBId).Value.Id()
	if a, ok := e.Get(PostgresServerAddonNameId); ok && a.Value.Kind() == entity.KindString {
		o.AddonName = a.Value.String()
	}
	if a, ok := e.Get(PostgresServerAssociationCountId); ok && a.Value.Kind() == entity.KindInt64 {
		o.AssociationCount = a.Value.Int64()
	}
	if a, ok := e.Get(PostgresServerPlanId); ok && a.Value.Kind() == entity.KindString {
		o.Plan = a.Value.String()
	}
	if a, ok := e.Get(PostgresServerSandboxPoolId); ok && a.Value.Kind() == entity.KindId {
		o.SandboxPool = a.Value.Id()
	}
	if a, ok := e.Get(PostgresServerServiceId); ok && a.Value.Kind() == entity.KindId {
		o.Service = a.Value.Id()
	}
	if a, ok := e.Get(PostgresServerStatusId); ok && a.Value.Kind() == entity.KindString {
		o.Status = a.Value.String()
	}
	if a, ok := e.Get(PostgresServerSuperuserPasswordId); ok && a.Value.Kind() == entity.KindString {
		o.SuperuserPassword = a.Value.String()
	}
}

func (o *PostgresServer) Is(e entity.AttrGetter) bool {
	return entity.Is(e, KindPostgresServer)
}

func (o *PostgresServer) ShortKind() string {
	return "postgres_server"
}

func (o *PostgresServer) Kind() entity.Id {
	return KindPostgresServer
}

func (o *PostgresServer) EntityId() entity.Id {
	return o.ID
}

func (o *PostgresServer) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.AddonName) {
		attrs = append(attrs, entity.String(PostgresServerAddonNameId, o.AddonName))
	}
	if !entity.Empty(o.AssociationCount) {
		attrs = append(attrs, entity.Int64(PostgresServerAssociationCountId, o.AssociationCount))
	}
	if !entity.Empty(o.Plan) {
		attrs = append(attrs, entity.String(PostgresServerPlanId, o.Plan))
	}
	if !entity.Empty(o.SandboxPool) {
		attrs = append(attrs, entity.Ref(PostgresServerSandboxPoolId, o.SandboxPool))
	}
	if !entity.Empty(o.Service) {
		attrs = append(attrs, entity.Ref(PostgresServerServiceId, o.Service))
	}
	if !entity.Empty(o.Status) {
		attrs = append(attrs, entity.String(PostgresServerStatusId, o.Status))
	}
	if !entity.Empty(o.SuperuserPassword) {
		attrs = append(attrs, entity.String(PostgresServerSuperuserPasswordId, o.SuperuserPassword))
	}
	attrs = append(attrs, entity.Ref(entity.EntityKind, KindPostgresServer))
	return
}

func (o *PostgresServer) Empty() bool {
	if !entity.Empty(o.AddonName) {
		return false
	}
	if !entity.Empty(o.AssociationCount) {
		return false
	}
	if !entity.Empty(o.Plan) {
		return false
	}
	if !entity.Empty(o.SandboxPool) {
		return false
	}
	if !entity.Empty(o.Service) {
		return false
	}
	if !entity.Empty(o.Status) {
		return false
	}
	if !entity.Empty(o.SuperuserPassword) {
		return false
	}
	return true
}

func (o *PostgresServer) InitSchema(sb *schema.SchemaBuilder) {
	sb.String("addon_name", "dev.miren.addon/postgres_server.addon_name", schema.Indexed)
	sb.Int64("association_count", "dev.miren.addon/postgres_server.association_count")
	sb.String("plan", "dev.miren.addon/postgres_server.plan")
	sb.Ref("sandbox_pool", "dev.miren.addon/postgres_server.sandbox_pool")
	sb.Ref("service", "dev.miren.addon/postgres_server.service")
	sb.String("status", "dev.miren.addon/postgres_server.status")
	sb.String("superuser_password", "dev.miren.addon/postgres_server.superuser_password")
}

const (
	PostgresqlDedicatedDataPostgresServerId = entity.Id("dev.miren.addon/postgresql_dedicated_data.postgres_server")
)

type PostgresqlDedicatedData struct {
	ID             entity.Id `json:"id"`
	PostgresServer entity.Id `cbor:"postgres_server,omitempty" json:"postgres_server,omitempty"`
}

func (o *PostgresqlDedicatedData) Decode(e entity.AttrGetter) {
	o.ID = entity.MustGet(e, entity.DBId).Value.Id()
	if a, ok := e.Get(PostgresqlDedicatedDataPostgresServerId); ok && a.Value.Kind() == entity.KindId {
		o.PostgresServer = a.Value.Id()
	}
}

func (o *PostgresqlDedicatedData) Is(e entity.AttrGetter) bool {
	return entity.Is(e, KindPostgresqlDedicatedData)
}

func (o *PostgresqlDedicatedData) ShortKind() string {
	return "postgresql_dedicated_data"
}

func (o *PostgresqlDedicatedData) Kind() entity.Id {
	return KindPostgresqlDedicatedData
}

func (o *PostgresqlDedicatedData) EntityId() entity.Id {
	return o.ID
}

func (o *PostgresqlDedicatedData) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.PostgresServer) {
		attrs = append(attrs, entity.Ref(PostgresqlDedicatedDataPostgresServerId, o.PostgresServer))
	}
	attrs = append(attrs, entity.Ref(entity.EntityKind, KindPostgresqlDedicatedData))
	return
}

func (o *PostgresqlDedicatedData) Empty() bool {
	if !entity.Empty(o.PostgresServer) {
		return false
	}
	return true
}

func (o *PostgresqlDedicatedData) InitSchema(sb *schema.SchemaBuilder) {
	sb.Ref("postgres_server", "dev.miren.addon/postgresql_dedicated_data.postgres_server")
}

const (
	PostgresqlSharedDataDatabaseNameId   = entity.Id("dev.miren.addon/postgresql_shared_data.database_name")
	PostgresqlSharedDataPostgresServerId = entity.Id("dev.miren.addon/postgresql_shared_data.postgres_server")
	PostgresqlSharedDataUsernameId       = entity.Id("dev.miren.addon/postgresql_shared_data.username")
)

type PostgresqlSharedData struct {
	ID             entity.Id `json:"id"`
	DatabaseName   string    `cbor:"database_name,omitempty" json:"database_name,omitempty"`
	PostgresServer entity.Id `cbor:"postgres_server,omitempty" json:"postgres_server,omitempty"`
	Username       string    `cbor:"username,omitempty" json:"username,omitempty"`
}

func (o *PostgresqlSharedData) Decode(e entity.AttrGetter) {
	o.ID = entity.MustGet(e, entity.DBId).Value.Id()
	if a, ok := e.Get(PostgresqlSharedDataDatabaseNameId); ok && a.Value.Kind() == entity.KindString {
		o.DatabaseName = a.Value.String()
	}
	if a, ok := e.Get(PostgresqlSharedDataPostgresServerId); ok && a.Value.Kind() == entity.KindId {
		o.PostgresServer = a.Value.Id()
	}
	if a, ok := e.Get(PostgresqlSharedDataUsernameId); ok && a.Value.Kind() == entity.KindString {
		o.Username = a.Value.String()
	}
}

func (o *PostgresqlSharedData) Is(e entity.AttrGetter) bool {
	return entity.Is(e, KindPostgresqlSharedData)
}

func (o *PostgresqlSharedData) ShortKind() string {
	return "postgresql_shared_data"
}

func (o *PostgresqlSharedData) Kind() entity.Id {
	return KindPostgresqlSharedData
}

func (o *PostgresqlSharedData) EntityId() entity.Id {
	return o.ID
}

func (o *PostgresqlSharedData) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.DatabaseName) {
		attrs = append(attrs, entity.String(PostgresqlSharedDataDatabaseNameId, o.DatabaseName))
	}
	if !entity.Empty(o.PostgresServer) {
		attrs = append(attrs, entity.Ref(PostgresqlSharedDataPostgresServerId, o.PostgresServer))
	}
	if !entity.Empty(o.Username) {
		attrs = append(attrs, entity.String(PostgresqlSharedDataUsernameId, o.Username))
	}
	attrs = append(attrs, entity.Ref(entity.EntityKind, KindPostgresqlSharedData))
	return
}

func (o *PostgresqlSharedData) Empty() bool {
	if !entity.Empty(o.DatabaseName) {
		return false
	}
	if !entity.Empty(o.PostgresServer) {
		return false
	}
	if !entity.Empty(o.Username) {
		return false
	}
	return true
}

func (o *PostgresqlSharedData) InitSchema(sb *schema.SchemaBuilder) {
	sb.String("database_name", "dev.miren.addon/postgresql_shared_data.database_name")
	sb.Ref("postgres_server", "dev.miren.addon/postgresql_shared_data.postgres_server")
	sb.String("username", "dev.miren.addon/postgresql_shared_data.username")
}

var (
	KindAddon                   = entity.Id("dev.miren.addon/kind.addon")
	KindAddonAssociation        = entity.Id("dev.miren.addon/kind.addon_association")
	KindPostgresServer          = entity.Id("dev.miren.addon/kind.postgres_server")
	KindPostgresqlDedicatedData = entity.Id("dev.miren.addon/kind.postgresql_dedicated_data")
	KindPostgresqlSharedData    = entity.Id("dev.miren.addon/kind.postgresql_shared_data")
	Schema                      = entity.Id("dev.miren.addon/schema.v1alpha")
)

func init() {
	schema.Register("dev.miren.addon", "v1alpha", func(sb *schema.SchemaBuilder) {
		(&Addon{}).InitSchema(sb)
		(&AddonAssociation{}).InitSchema(sb)
		(&PostgresServer{}).InitSchema(sb)
		(&PostgresqlDedicatedData{}).InitSchema(sb)
		(&PostgresqlSharedData{}).InitSchema(sb)
	})
	schema.RegisterEncodedSchema("dev.miren.addon", "v1alpha", []byte("\x1f\x8b\b\x00\x00\x00\x00\x00\x00\xff\x9c\x97\xed\xae\xd4,\x10ǯ\xe3y\x8c1\xbe\xbf\xa6'\xc7\x18\x13\xaf\xa6a\xcbl\x17\x97\x02\ah\xdd\xfd\xa8F\xbd\x90\xe3\xf1\x0e\xf5\xb3\x01\xba[\n\xb4\xb0~\xd9\xd0\t\xf3\x83\x99\xfft\xa6\xfb\x133\xd4\x01\xc70T\x1d\x91\xc0*\x841g\xb0'\f\xab\xbb\xc3\xff\x81\xfd\xca\xd8\xdd\xf2\x97u\xec\xc3\r\x9e\xfb\x9f-\xe6\x1d\",\x84o\xb7\x04(V\xdfn7\x04\x1f\x1e%\x01\x15\x86-ꩮ\x05E\xcc\x1eDg\x16}\x14\xb0UZ\x12\xd6Z\xca\xc3%\x8aj$\x11\x9ap\a\xd9\xfb\x86\x90\xb1t\x13\xa2\x04E\xc7\xda\xf8\x8f7\xf1-!%ʘ\xa3\x9c\xbdq\xe8ug\xbc\ue97dL\xac\nw\x88\x1d\x7f[?p\x06\xe3M\x1a\xde\t\u0380\xe9i\xe5$\x89`\x95\a+R\xe6k:\xa7\x16P\x9cS\x1b\xd7\xfd%\x86F\x84\xfa\x91\xb5'S&\xb6\xc7+\xb1\x9d\xb0E1~\xb9M\xe6}DT{8\xda\x03\x1b\xb3\b5\x8e\xa2:y\r\x88\xf6NapKϳ\x1d@*\xc2Y;\\#*v\x88\nI:$\x8f\xb5\xb9\xed)\xfct\t\xb9\xe8\x96Kh\x15\xedjf}\x8b=\xe6\xf0$\xf9\xb2\v\xaet+A\xd5\n\xe4\x00r\xd4\xe1At\xc7\xf9\xb6\"\r~\xd8p_eP\xce:\xbd\x7f\x1f\xbd\xe7P\x99\xeb,K)\xde\x10d\x8a\xb5nxϴE\xde\xc4fCn\b\xd3\x16\x1be&Ğ\xdb\x14N\xb6\xa779\x7f\x85\x18\xde\xf0C-8\xa7\xae\xc9\xcc,\x86\xb7!ز\x9egY \aҸ\\\xb5\xa7\a\x9f\xf0,K\xd0H\xf7\xca\x02\xb6\xe3:\x8c\xe8m\x96\xd1\v\x90\xbd\x02Y\v\xa4\xd4'.\xb1\xe5Ʉ\xbd\xb4\x92ypF\x1c\xc84\x9djOѱd\xd33\xc2\xdfXT\xb4\xdf\xd3*D0gw\xcd\xc0-}\r\x9e\x16\xf8\v\xe1Z\x90Y\xf8\xbeWy_\x90\x92˺\x03\xa5P\xeb*\xa1\x9b\x9bB=\xa3T\xc6\xccL\x8d\xbf\xc8\x13\xd6\xeb\xca\x0e\x8c\xd7yʀ$A\x1b\n\xfe\xf8 \x9313@\xc2\x03\xe2r\x99\x0e\xb8`XF#\xe1\fY\x1f%\xe1<\xf3\xfc\x140E4\x19\x9c~dz4\f\xbc\xe1\x9cZBԉ'\xc2?\x8f\xa3)\x9d\xab\xdbn\xa2\xdc\xc5\xfa\xcdf\xc8\r\xad\xd5\x0eI\xc05F\x1a\x8d\x8a,\xb6\xa3\xf9\xee\v\xc4xWF\xac\xcc\xcf\x06)\x98fK77\x85j\xbd/\x04\a\x8d\xca}b\x87\xc6\xd5Wz\x01l\x1a\xe7\xf9\xb2\xbb\xf3S\xa9\xaeC\x1a{\xa8r\xaaa\xc0\xa4Az.\xdc˕K\xcf\x1d\x8a\xb4\xfbl3\xf1\xa1\x18zQ\x96W\xd3r\xf8o\xf1\x90\xd0o\xafv\\\xea\xda\xfd=\x1a\xbf\x9bV\xfe$E#\xab\xe8\x1b+\xf1^\x15\x8e\xba%\x81/y-W\xb2qi\xa1\xfc\x05\x00\x00\xff\xff\x01\x00\x00\xff\xff\x8a蟀]\x0e\x00\x00"))
}
