package addon_v1alpha

import (
	entity "miren.dev/runtime/pkg/entity"
	schema "miren.dev/runtime/pkg/entity/schema"
)

const (
	AddonDefaultVariantId = entity.Id("dev.miren.addon/addon.default_variant")
	AddonDefaultVersionId = entity.Id("dev.miren.addon/addon.default_version")
	AddonDescriptionId    = entity.Id("dev.miren.addon/addon.description")
	AddonDisplayNameId    = entity.Id("dev.miren.addon/addon.display_name")
	AddonLocalityModeId   = entity.Id("dev.miren.addon/addon.locality_mode")
	AddonNameId           = entity.Id("dev.miren.addon/addon.name")
	AddonVariantsId       = entity.Id("dev.miren.addon/addon.variants")
)

type Addon struct {
	ID             entity.Id  `json:"id"`
	DefaultVariant string     `cbor:"default_variant,omitempty" json:"default_variant,omitempty"`
	DefaultVersion string     `cbor:"default_version,omitempty" json:"default_version,omitempty"`
	Description    string     `cbor:"description,omitempty" json:"description,omitempty"`
	DisplayName    string     `cbor:"display_name,omitempty" json:"display_name,omitempty"`
	LocalityMode   string     `cbor:"locality_mode,omitempty" json:"locality_mode,omitempty"`
	Name           string     `cbor:"name,omitempty" json:"name,omitempty"`
	Variants       []Variants `cbor:"variants,omitempty" json:"variants,omitempty"`
}

func (o *Addon) Decode(e entity.AttrGetter) {
	o.ID = entity.MustGet(e, entity.DBId).Value.Id()
	if a, ok := e.Get(AddonDefaultVariantId); ok && a.Value.Kind() == entity.KindString {
		o.DefaultVariant = a.Value.String()
	}
	if a, ok := e.Get(AddonDefaultVersionId); ok && a.Value.Kind() == entity.KindString {
		o.DefaultVersion = a.Value.String()
	}
	if a, ok := e.Get(AddonDescriptionId); ok && a.Value.Kind() == entity.KindString {
		o.Description = a.Value.String()
	}
	if a, ok := e.Get(AddonDisplayNameId); ok && a.Value.Kind() == entity.KindString {
		o.DisplayName = a.Value.String()
	}
	if a, ok := e.Get(AddonLocalityModeId); ok && a.Value.Kind() == entity.KindString {
		o.LocalityMode = a.Value.String()
	}
	if a, ok := e.Get(AddonNameId); ok && a.Value.Kind() == entity.KindString {
		o.Name = a.Value.String()
	}
	for _, a := range e.GetAll(AddonVariantsId) {
		if a.Value.Kind() == entity.KindComponent {
			var v Variants
			v.Decode(a.Value.Component())
			o.Variants = append(o.Variants, v)
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
	if !entity.Empty(o.DefaultVariant) {
		attrs = append(attrs, entity.String(AddonDefaultVariantId, o.DefaultVariant))
	}
	if !entity.Empty(o.DefaultVersion) {
		attrs = append(attrs, entity.String(AddonDefaultVersionId, o.DefaultVersion))
	}
	if !entity.Empty(o.Description) {
		attrs = append(attrs, entity.String(AddonDescriptionId, o.Description))
	}
	if !entity.Empty(o.DisplayName) {
		attrs = append(attrs, entity.String(AddonDisplayNameId, o.DisplayName))
	}
	if !entity.Empty(o.LocalityMode) {
		attrs = append(attrs, entity.String(AddonLocalityModeId, o.LocalityMode))
	}
	if !entity.Empty(o.Name) {
		attrs = append(attrs, entity.String(AddonNameId, o.Name))
	}
	for _, v := range o.Variants {
		attrs = append(attrs, entity.Component(AddonVariantsId, v.Encode()))
	}
	attrs = append(attrs, entity.Ref(entity.EntityKind, KindAddon))
	return
}

func (o *Addon) Empty() bool {
	if !entity.Empty(o.DefaultVariant) {
		return false
	}
	if !entity.Empty(o.DefaultVersion) {
		return false
	}
	if !entity.Empty(o.Description) {
		return false
	}
	if !entity.Empty(o.DisplayName) {
		return false
	}
	if !entity.Empty(o.LocalityMode) {
		return false
	}
	if !entity.Empty(o.Name) {
		return false
	}
	if len(o.Variants) != 0 {
		return false
	}
	return true
}

func (o *Addon) InitSchema(sb *schema.SchemaBuilder) {
	sb.String("default_variant", "dev.miren.addon/addon.default_variant")
	sb.String("default_version", "dev.miren.addon/addon.default_version")
	sb.String("description", "dev.miren.addon/addon.description")
	sb.String("display_name", "dev.miren.addon/addon.display_name")
	sb.String("locality_mode", "dev.miren.addon/addon.locality_mode")
	sb.String("name", "dev.miren.addon/addon.name", schema.Indexed)
	sb.Component("variants", "dev.miren.addon/addon.variants", schema.Many)
	(&Variants{}).InitSchema(sb.Builder("addon.variants"))
}

const (
	VariantsDescriptionId = entity.Id("dev.miren.addon/variants.description")
	VariantsDetailsId     = entity.Id("dev.miren.addon/variants.details")
	VariantsNameId        = entity.Id("dev.miren.addon/variants.name")
)

type Variants struct {
	Description string    `cbor:"description,omitempty" json:"description,omitempty"`
	Details     []Details `cbor:"details,omitempty" json:"details,omitempty"`
	Name        string    `cbor:"name,omitempty" json:"name,omitempty"`
}

func (o *Variants) Decode(e entity.AttrGetter) {
	if a, ok := e.Get(VariantsDescriptionId); ok && a.Value.Kind() == entity.KindString {
		o.Description = a.Value.String()
	}
	for _, a := range e.GetAll(VariantsDetailsId) {
		if a.Value.Kind() == entity.KindComponent {
			var v Details
			v.Decode(a.Value.Component())
			o.Details = append(o.Details, v)
		}
	}
	if a, ok := e.Get(VariantsNameId); ok && a.Value.Kind() == entity.KindString {
		o.Name = a.Value.String()
	}
}

func (o *Variants) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.Description) {
		attrs = append(attrs, entity.String(VariantsDescriptionId, o.Description))
	}
	for _, v := range o.Details {
		attrs = append(attrs, entity.Component(VariantsDetailsId, v.Encode()))
	}
	if !entity.Empty(o.Name) {
		attrs = append(attrs, entity.String(VariantsNameId, o.Name))
	}
	return
}

func (o *Variants) Empty() bool {
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

func (o *Variants) InitSchema(sb *schema.SchemaBuilder) {
	sb.String("description", "dev.miren.addon/variants.description")
	sb.Component("details", "dev.miren.addon/variants.details", schema.Many)
	(&Details{}).InitSchema(sb.Builder("variants.details"))
	sb.String("name", "dev.miren.addon/variants.name")
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
	AddonAssociationStatusId       = entity.Id("dev.miren.addon/addon_association.status")
	AddonAssociationVariablesId    = entity.Id("dev.miren.addon/addon_association.variables")
	AddonAssociationVariantId      = entity.Id("dev.miren.addon/addon_association.variant")
	AddonAssociationVersionId      = entity.Id("dev.miren.addon/addon_association.version")
)

type AddonAssociation struct {
	ID           entity.Id   `json:"id"`
	Addon        entity.Id   `cbor:"addon,omitempty" json:"addon,omitempty"`
	App          entity.Id   `cbor:"app,omitempty" json:"app,omitempty"`
	ErrorMessage string      `cbor:"error_message,omitempty" json:"error_message,omitempty"`
	Status       string      `cbor:"status,omitempty" json:"status,omitempty"`
	Variables    []Variables `cbor:"variables,omitempty" json:"variables,omitempty"`
	Variant      string      `cbor:"variant,omitempty" json:"variant,omitempty"`
	Version      string      `cbor:"version,omitempty" json:"version,omitempty"`
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
	if a, ok := e.Get(AddonAssociationVariantId); ok && a.Value.Kind() == entity.KindString {
		o.Variant = a.Value.String()
	}
	if a, ok := e.Get(AddonAssociationVersionId); ok && a.Value.Kind() == entity.KindString {
		o.Version = a.Value.String()
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
	if !entity.Empty(o.Status) {
		attrs = append(attrs, entity.String(AddonAssociationStatusId, o.Status))
	}
	for _, v := range o.Variables {
		attrs = append(attrs, entity.Component(AddonAssociationVariablesId, v.Encode()))
	}
	if !entity.Empty(o.Variant) {
		attrs = append(attrs, entity.String(AddonAssociationVariantId, o.Variant))
	}
	if !entity.Empty(o.Version) {
		attrs = append(attrs, entity.String(AddonAssociationVersionId, o.Version))
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
	if !entity.Empty(o.Status) {
		return false
	}
	if len(o.Variables) != 0 {
		return false
	}
	if !entity.Empty(o.Variant) {
		return false
	}
	if !entity.Empty(o.Version) {
		return false
	}
	return true
}

func (o *AddonAssociation) InitSchema(sb *schema.SchemaBuilder) {
	sb.Ref("addon", "dev.miren.addon/addon_association.addon", schema.Indexed)
	sb.Ref("app", "dev.miren.addon/addon_association.app", schema.Indexed, schema.Tags("dev.miren.app_ref"))
	sb.String("error_message", "dev.miren.addon/addon_association.error_message")
	sb.String("status", "dev.miren.addon/addon_association.status", schema.Indexed)
	sb.Component("variables", "dev.miren.addon/addon_association.variables", schema.Many)
	(&Variables{}).InitSchema(sb.Builder("addon_association.variables"))
	sb.String("variant", "dev.miren.addon/addon_association.variant")
	sb.String("version", "dev.miren.addon/addon_association.version")
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
	MemcacheDedicatedDataMemcacheServerId = entity.Id("dev.miren.addon/memcache_dedicated_data.memcache_server")
)

type MemcacheDedicatedData struct {
	ID             entity.Id `json:"id"`
	MemcacheServer entity.Id `cbor:"memcache_server,omitempty" json:"memcache_server,omitempty"`
}

func (o *MemcacheDedicatedData) Decode(e entity.AttrGetter) {
	o.ID = entity.MustGet(e, entity.DBId).Value.Id()
	if a, ok := e.Get(MemcacheDedicatedDataMemcacheServerId); ok && a.Value.Kind() == entity.KindId {
		o.MemcacheServer = a.Value.Id()
	}
}

func (o *MemcacheDedicatedData) Is(e entity.AttrGetter) bool {
	return entity.Is(e, KindMemcacheDedicatedData)
}

func (o *MemcacheDedicatedData) ShortKind() string {
	return "memcache_dedicated_data"
}

func (o *MemcacheDedicatedData) Kind() entity.Id {
	return KindMemcacheDedicatedData
}

func (o *MemcacheDedicatedData) EntityId() entity.Id {
	return o.ID
}

func (o *MemcacheDedicatedData) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.MemcacheServer) {
		attrs = append(attrs, entity.Ref(MemcacheDedicatedDataMemcacheServerId, o.MemcacheServer))
	}
	attrs = append(attrs, entity.Ref(entity.EntityKind, KindMemcacheDedicatedData))
	return
}

func (o *MemcacheDedicatedData) Empty() bool {
	return entity.Empty(o.MemcacheServer)
}

func (o *MemcacheDedicatedData) InitSchema(sb *schema.SchemaBuilder) {
	sb.Ref("memcache_server", "dev.miren.addon/memcache_dedicated_data.memcache_server")
}

const (
	MemcacheServerAddonNameId        = entity.Id("dev.miren.addon/memcache_server.addon_name")
	MemcacheServerAssociationCountId = entity.Id("dev.miren.addon/memcache_server.association_count")
	MemcacheServerSandboxPoolId      = entity.Id("dev.miren.addon/memcache_server.sandbox_pool")
	MemcacheServerServiceId          = entity.Id("dev.miren.addon/memcache_server.service")
	MemcacheServerStatusId           = entity.Id("dev.miren.addon/memcache_server.status")
	MemcacheServerVariantId          = entity.Id("dev.miren.addon/memcache_server.variant")
)

type MemcacheServer struct {
	ID               entity.Id `json:"id"`
	AddonName        string    `cbor:"addon_name,omitempty" json:"addon_name,omitempty"`
	AssociationCount int64     `cbor:"association_count,omitempty" json:"association_count,omitempty"`
	SandboxPool      entity.Id `cbor:"sandbox_pool,omitempty" json:"sandbox_pool,omitempty"`
	Service          entity.Id `cbor:"service,omitempty" json:"service,omitempty"`
	Status           string    `cbor:"status,omitempty" json:"status,omitempty"`
	Variant          string    `cbor:"variant,omitempty" json:"variant,omitempty"`
}

func (o *MemcacheServer) Decode(e entity.AttrGetter) {
	o.ID = entity.MustGet(e, entity.DBId).Value.Id()
	if a, ok := e.Get(MemcacheServerAddonNameId); ok && a.Value.Kind() == entity.KindString {
		o.AddonName = a.Value.String()
	}
	if a, ok := e.Get(MemcacheServerAssociationCountId); ok && a.Value.Kind() == entity.KindInt64 {
		o.AssociationCount = a.Value.Int64()
	}
	if a, ok := e.Get(MemcacheServerSandboxPoolId); ok && a.Value.Kind() == entity.KindId {
		o.SandboxPool = a.Value.Id()
	}
	if a, ok := e.Get(MemcacheServerServiceId); ok && a.Value.Kind() == entity.KindId {
		o.Service = a.Value.Id()
	}
	if a, ok := e.Get(MemcacheServerStatusId); ok && a.Value.Kind() == entity.KindString {
		o.Status = a.Value.String()
	}
	if a, ok := e.Get(MemcacheServerVariantId); ok && a.Value.Kind() == entity.KindString {
		o.Variant = a.Value.String()
	}
}

func (o *MemcacheServer) Is(e entity.AttrGetter) bool {
	return entity.Is(e, KindMemcacheServer)
}

func (o *MemcacheServer) ShortKind() string {
	return "memcache_server"
}

func (o *MemcacheServer) Kind() entity.Id {
	return KindMemcacheServer
}

func (o *MemcacheServer) EntityId() entity.Id {
	return o.ID
}

func (o *MemcacheServer) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.AddonName) {
		attrs = append(attrs, entity.String(MemcacheServerAddonNameId, o.AddonName))
	}
	if !entity.Empty(o.AssociationCount) {
		attrs = append(attrs, entity.Int64(MemcacheServerAssociationCountId, o.AssociationCount))
	}
	if !entity.Empty(o.SandboxPool) {
		attrs = append(attrs, entity.Ref(MemcacheServerSandboxPoolId, o.SandboxPool))
	}
	if !entity.Empty(o.Service) {
		attrs = append(attrs, entity.Ref(MemcacheServerServiceId, o.Service))
	}
	if !entity.Empty(o.Status) {
		attrs = append(attrs, entity.String(MemcacheServerStatusId, o.Status))
	}
	if !entity.Empty(o.Variant) {
		attrs = append(attrs, entity.String(MemcacheServerVariantId, o.Variant))
	}
	attrs = append(attrs, entity.Ref(entity.EntityKind, KindMemcacheServer))
	return
}

func (o *MemcacheServer) Empty() bool {
	if !entity.Empty(o.AddonName) {
		return false
	}
	if !entity.Empty(o.AssociationCount) {
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
	if !entity.Empty(o.Variant) {
		return false
	}
	return true
}

func (o *MemcacheServer) InitSchema(sb *schema.SchemaBuilder) {
	sb.String("addon_name", "dev.miren.addon/memcache_server.addon_name", schema.Indexed)
	sb.Int64("association_count", "dev.miren.addon/memcache_server.association_count")
	sb.Ref("sandbox_pool", "dev.miren.addon/memcache_server.sandbox_pool")
	sb.Ref("service", "dev.miren.addon/memcache_server.service")
	sb.String("status", "dev.miren.addon/memcache_server.status")
	sb.String("variant", "dev.miren.addon/memcache_server.variant")
}

const (
	MysqlDedicatedDataMysqlServerId = entity.Id("dev.miren.addon/mysql_dedicated_data.mysql_server")
)

type MysqlDedicatedData struct {
	ID          entity.Id `json:"id"`
	MysqlServer entity.Id `cbor:"mysql_server,omitempty" json:"mysql_server,omitempty"`
}

func (o *MysqlDedicatedData) Decode(e entity.AttrGetter) {
	o.ID = entity.MustGet(e, entity.DBId).Value.Id()
	if a, ok := e.Get(MysqlDedicatedDataMysqlServerId); ok && a.Value.Kind() == entity.KindId {
		o.MysqlServer = a.Value.Id()
	}
}

func (o *MysqlDedicatedData) Is(e entity.AttrGetter) bool {
	return entity.Is(e, KindMysqlDedicatedData)
}

func (o *MysqlDedicatedData) ShortKind() string {
	return "mysql_dedicated_data"
}

func (o *MysqlDedicatedData) Kind() entity.Id {
	return KindMysqlDedicatedData
}

func (o *MysqlDedicatedData) EntityId() entity.Id {
	return o.ID
}

func (o *MysqlDedicatedData) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.MysqlServer) {
		attrs = append(attrs, entity.Ref(MysqlDedicatedDataMysqlServerId, o.MysqlServer))
	}
	attrs = append(attrs, entity.Ref(entity.EntityKind, KindMysqlDedicatedData))
	return
}

func (o *MysqlDedicatedData) Empty() bool {
	return entity.Empty(o.MysqlServer)
}

func (o *MysqlDedicatedData) InitSchema(sb *schema.SchemaBuilder) {
	sb.Ref("mysql_server", "dev.miren.addon/mysql_dedicated_data.mysql_server")
}

const (
	MysqlServerAddonNameId        = entity.Id("dev.miren.addon/mysql_server.addon_name")
	MysqlServerAssociationCountId = entity.Id("dev.miren.addon/mysql_server.association_count")
	MysqlServerRootPasswordId     = entity.Id("dev.miren.addon/mysql_server.root_password")
	MysqlServerSandboxPoolId      = entity.Id("dev.miren.addon/mysql_server.sandbox_pool")
	MysqlServerServiceId          = entity.Id("dev.miren.addon/mysql_server.service")
	MysqlServerStatusId           = entity.Id("dev.miren.addon/mysql_server.status")
	MysqlServerVariantId          = entity.Id("dev.miren.addon/mysql_server.variant")
)

type MysqlServer struct {
	ID               entity.Id `json:"id"`
	AddonName        string    `cbor:"addon_name,omitempty" json:"addon_name,omitempty"`
	AssociationCount int64     `cbor:"association_count,omitempty" json:"association_count,omitempty"`
	RootPassword     string    `cbor:"root_password,omitempty" json:"root_password,omitempty"`
	SandboxPool      entity.Id `cbor:"sandbox_pool,omitempty" json:"sandbox_pool,omitempty"`
	Service          entity.Id `cbor:"service,omitempty" json:"service,omitempty"`
	Status           string    `cbor:"status,omitempty" json:"status,omitempty"`
	Variant          string    `cbor:"variant,omitempty" json:"variant,omitempty"`
}

func (o *MysqlServer) Decode(e entity.AttrGetter) {
	o.ID = entity.MustGet(e, entity.DBId).Value.Id()
	if a, ok := e.Get(MysqlServerAddonNameId); ok && a.Value.Kind() == entity.KindString {
		o.AddonName = a.Value.String()
	}
	if a, ok := e.Get(MysqlServerAssociationCountId); ok && a.Value.Kind() == entity.KindInt64 {
		o.AssociationCount = a.Value.Int64()
	}
	if a, ok := e.Get(MysqlServerRootPasswordId); ok && a.Value.Kind() == entity.KindString {
		o.RootPassword = a.Value.String()
	}
	if a, ok := e.Get(MysqlServerSandboxPoolId); ok && a.Value.Kind() == entity.KindId {
		o.SandboxPool = a.Value.Id()
	}
	if a, ok := e.Get(MysqlServerServiceId); ok && a.Value.Kind() == entity.KindId {
		o.Service = a.Value.Id()
	}
	if a, ok := e.Get(MysqlServerStatusId); ok && a.Value.Kind() == entity.KindString {
		o.Status = a.Value.String()
	}
	if a, ok := e.Get(MysqlServerVariantId); ok && a.Value.Kind() == entity.KindString {
		o.Variant = a.Value.String()
	}
}

func (o *MysqlServer) Is(e entity.AttrGetter) bool {
	return entity.Is(e, KindMysqlServer)
}

func (o *MysqlServer) ShortKind() string {
	return "mysql_server"
}

func (o *MysqlServer) Kind() entity.Id {
	return KindMysqlServer
}

func (o *MysqlServer) EntityId() entity.Id {
	return o.ID
}

func (o *MysqlServer) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.AddonName) {
		attrs = append(attrs, entity.String(MysqlServerAddonNameId, o.AddonName))
	}
	if !entity.Empty(o.AssociationCount) {
		attrs = append(attrs, entity.Int64(MysqlServerAssociationCountId, o.AssociationCount))
	}
	if !entity.Empty(o.RootPassword) {
		attrs = append(attrs, entity.String(MysqlServerRootPasswordId, o.RootPassword))
	}
	if !entity.Empty(o.SandboxPool) {
		attrs = append(attrs, entity.Ref(MysqlServerSandboxPoolId, o.SandboxPool))
	}
	if !entity.Empty(o.Service) {
		attrs = append(attrs, entity.Ref(MysqlServerServiceId, o.Service))
	}
	if !entity.Empty(o.Status) {
		attrs = append(attrs, entity.String(MysqlServerStatusId, o.Status))
	}
	if !entity.Empty(o.Variant) {
		attrs = append(attrs, entity.String(MysqlServerVariantId, o.Variant))
	}
	attrs = append(attrs, entity.Ref(entity.EntityKind, KindMysqlServer))
	return
}

func (o *MysqlServer) Empty() bool {
	if !entity.Empty(o.AddonName) {
		return false
	}
	if !entity.Empty(o.AssociationCount) {
		return false
	}
	if !entity.Empty(o.RootPassword) {
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
	if !entity.Empty(o.Variant) {
		return false
	}
	return true
}

func (o *MysqlServer) InitSchema(sb *schema.SchemaBuilder) {
	sb.String("addon_name", "dev.miren.addon/mysql_server.addon_name", schema.Indexed)
	sb.Int64("association_count", "dev.miren.addon/mysql_server.association_count")
	sb.String("root_password", "dev.miren.addon/mysql_server.root_password")
	sb.Ref("sandbox_pool", "dev.miren.addon/mysql_server.sandbox_pool")
	sb.Ref("service", "dev.miren.addon/mysql_server.service")
	sb.String("status", "dev.miren.addon/mysql_server.status")
	sb.String("variant", "dev.miren.addon/mysql_server.variant")
}

const (
	MysqlSharedDataDatabaseNameId = entity.Id("dev.miren.addon/mysql_shared_data.database_name")
	MysqlSharedDataMysqlServerId  = entity.Id("dev.miren.addon/mysql_shared_data.mysql_server")
	MysqlSharedDataUsernameId     = entity.Id("dev.miren.addon/mysql_shared_data.username")
)

type MysqlSharedData struct {
	ID           entity.Id `json:"id"`
	DatabaseName string    `cbor:"database_name,omitempty" json:"database_name,omitempty"`
	MysqlServer  entity.Id `cbor:"mysql_server,omitempty" json:"mysql_server,omitempty"`
	Username     string    `cbor:"username,omitempty" json:"username,omitempty"`
}

func (o *MysqlSharedData) Decode(e entity.AttrGetter) {
	o.ID = entity.MustGet(e, entity.DBId).Value.Id()
	if a, ok := e.Get(MysqlSharedDataDatabaseNameId); ok && a.Value.Kind() == entity.KindString {
		o.DatabaseName = a.Value.String()
	}
	if a, ok := e.Get(MysqlSharedDataMysqlServerId); ok && a.Value.Kind() == entity.KindId {
		o.MysqlServer = a.Value.Id()
	}
	if a, ok := e.Get(MysqlSharedDataUsernameId); ok && a.Value.Kind() == entity.KindString {
		o.Username = a.Value.String()
	}
}

func (o *MysqlSharedData) Is(e entity.AttrGetter) bool {
	return entity.Is(e, KindMysqlSharedData)
}

func (o *MysqlSharedData) ShortKind() string {
	return "mysql_shared_data"
}

func (o *MysqlSharedData) Kind() entity.Id {
	return KindMysqlSharedData
}

func (o *MysqlSharedData) EntityId() entity.Id {
	return o.ID
}

func (o *MysqlSharedData) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.DatabaseName) {
		attrs = append(attrs, entity.String(MysqlSharedDataDatabaseNameId, o.DatabaseName))
	}
	if !entity.Empty(o.MysqlServer) {
		attrs = append(attrs, entity.Ref(MysqlSharedDataMysqlServerId, o.MysqlServer))
	}
	if !entity.Empty(o.Username) {
		attrs = append(attrs, entity.String(MysqlSharedDataUsernameId, o.Username))
	}
	attrs = append(attrs, entity.Ref(entity.EntityKind, KindMysqlSharedData))
	return
}

func (o *MysqlSharedData) Empty() bool {
	if !entity.Empty(o.DatabaseName) {
		return false
	}
	if !entity.Empty(o.MysqlServer) {
		return false
	}
	if !entity.Empty(o.Username) {
		return false
	}
	return true
}

func (o *MysqlSharedData) InitSchema(sb *schema.SchemaBuilder) {
	sb.String("database_name", "dev.miren.addon/mysql_shared_data.database_name")
	sb.Ref("mysql_server", "dev.miren.addon/mysql_shared_data.mysql_server")
	sb.String("username", "dev.miren.addon/mysql_shared_data.username")
}

const (
	PostgresServerAddonNameId         = entity.Id("dev.miren.addon/postgres_server.addon_name")
	PostgresServerAssociationCountId  = entity.Id("dev.miren.addon/postgres_server.association_count")
	PostgresServerSandboxPoolId       = entity.Id("dev.miren.addon/postgres_server.sandbox_pool")
	PostgresServerServiceId           = entity.Id("dev.miren.addon/postgres_server.service")
	PostgresServerStatusId            = entity.Id("dev.miren.addon/postgres_server.status")
	PostgresServerSuperuserPasswordId = entity.Id("dev.miren.addon/postgres_server.superuser_password")
	PostgresServerVariantId           = entity.Id("dev.miren.addon/postgres_server.variant")
)

type PostgresServer struct {
	ID                entity.Id `json:"id"`
	AddonName         string    `cbor:"addon_name,omitempty" json:"addon_name,omitempty"`
	AssociationCount  int64     `cbor:"association_count,omitempty" json:"association_count,omitempty"`
	SandboxPool       entity.Id `cbor:"sandbox_pool,omitempty" json:"sandbox_pool,omitempty"`
	Service           entity.Id `cbor:"service,omitempty" json:"service,omitempty"`
	Status            string    `cbor:"status,omitempty" json:"status,omitempty"`
	SuperuserPassword string    `cbor:"superuser_password,omitempty" json:"superuser_password,omitempty"`
	Variant           string    `cbor:"variant,omitempty" json:"variant,omitempty"`
}

func (o *PostgresServer) Decode(e entity.AttrGetter) {
	o.ID = entity.MustGet(e, entity.DBId).Value.Id()
	if a, ok := e.Get(PostgresServerAddonNameId); ok && a.Value.Kind() == entity.KindString {
		o.AddonName = a.Value.String()
	}
	if a, ok := e.Get(PostgresServerAssociationCountId); ok && a.Value.Kind() == entity.KindInt64 {
		o.AssociationCount = a.Value.Int64()
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
	if a, ok := e.Get(PostgresServerVariantId); ok && a.Value.Kind() == entity.KindString {
		o.Variant = a.Value.String()
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
	if !entity.Empty(o.Variant) {
		attrs = append(attrs, entity.String(PostgresServerVariantId, o.Variant))
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
	if !entity.Empty(o.Variant) {
		return false
	}
	return true
}

func (o *PostgresServer) InitSchema(sb *schema.SchemaBuilder) {
	sb.String("addon_name", "dev.miren.addon/postgres_server.addon_name", schema.Indexed)
	sb.Int64("association_count", "dev.miren.addon/postgres_server.association_count")
	sb.Ref("sandbox_pool", "dev.miren.addon/postgres_server.sandbox_pool")
	sb.Ref("service", "dev.miren.addon/postgres_server.service")
	sb.String("status", "dev.miren.addon/postgres_server.status")
	sb.String("superuser_password", "dev.miren.addon/postgres_server.superuser_password")
	sb.String("variant", "dev.miren.addon/postgres_server.variant")
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
	return entity.Empty(o.PostgresServer)
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

const (
	RabbitmqDedicatedDataRabbitmqServerId = entity.Id("dev.miren.addon/rabbitmq_dedicated_data.rabbitmq_server")
)

type RabbitmqDedicatedData struct {
	ID             entity.Id `json:"id"`
	RabbitmqServer entity.Id `cbor:"rabbitmq_server,omitempty" json:"rabbitmq_server,omitempty"`
}

func (o *RabbitmqDedicatedData) Decode(e entity.AttrGetter) {
	o.ID = entity.MustGet(e, entity.DBId).Value.Id()
	if a, ok := e.Get(RabbitmqDedicatedDataRabbitmqServerId); ok && a.Value.Kind() == entity.KindId {
		o.RabbitmqServer = a.Value.Id()
	}
}

func (o *RabbitmqDedicatedData) Is(e entity.AttrGetter) bool {
	return entity.Is(e, KindRabbitmqDedicatedData)
}

func (o *RabbitmqDedicatedData) ShortKind() string {
	return "rabbitmq_dedicated_data"
}

func (o *RabbitmqDedicatedData) Kind() entity.Id {
	return KindRabbitmqDedicatedData
}

func (o *RabbitmqDedicatedData) EntityId() entity.Id {
	return o.ID
}

func (o *RabbitmqDedicatedData) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.RabbitmqServer) {
		attrs = append(attrs, entity.Ref(RabbitmqDedicatedDataRabbitmqServerId, o.RabbitmqServer))
	}
	attrs = append(attrs, entity.Ref(entity.EntityKind, KindRabbitmqDedicatedData))
	return
}

func (o *RabbitmqDedicatedData) Empty() bool {
	return entity.Empty(o.RabbitmqServer)
}

func (o *RabbitmqDedicatedData) InitSchema(sb *schema.SchemaBuilder) {
	sb.Ref("rabbitmq_server", "dev.miren.addon/rabbitmq_dedicated_data.rabbitmq_server")
}

const (
	RabbitmqServerAddonNameId        = entity.Id("dev.miren.addon/rabbitmq_server.addon_name")
	RabbitmqServerAssociationCountId = entity.Id("dev.miren.addon/rabbitmq_server.association_count")
	RabbitmqServerPasswordId         = entity.Id("dev.miren.addon/rabbitmq_server.password")
	RabbitmqServerSandboxPoolId      = entity.Id("dev.miren.addon/rabbitmq_server.sandbox_pool")
	RabbitmqServerServiceId          = entity.Id("dev.miren.addon/rabbitmq_server.service")
	RabbitmqServerStatusId           = entity.Id("dev.miren.addon/rabbitmq_server.status")
	RabbitmqServerVariantId          = entity.Id("dev.miren.addon/rabbitmq_server.variant")
)

type RabbitmqServer struct {
	ID               entity.Id `json:"id"`
	AddonName        string    `cbor:"addon_name,omitempty" json:"addon_name,omitempty"`
	AssociationCount int64     `cbor:"association_count,omitempty" json:"association_count,omitempty"`
	Password         string    `cbor:"password,omitempty" json:"password,omitempty"`
	SandboxPool      entity.Id `cbor:"sandbox_pool,omitempty" json:"sandbox_pool,omitempty"`
	Service          entity.Id `cbor:"service,omitempty" json:"service,omitempty"`
	Status           string    `cbor:"status,omitempty" json:"status,omitempty"`
	Variant          string    `cbor:"variant,omitempty" json:"variant,omitempty"`
}

func (o *RabbitmqServer) Decode(e entity.AttrGetter) {
	o.ID = entity.MustGet(e, entity.DBId).Value.Id()
	if a, ok := e.Get(RabbitmqServerAddonNameId); ok && a.Value.Kind() == entity.KindString {
		o.AddonName = a.Value.String()
	}
	if a, ok := e.Get(RabbitmqServerAssociationCountId); ok && a.Value.Kind() == entity.KindInt64 {
		o.AssociationCount = a.Value.Int64()
	}
	if a, ok := e.Get(RabbitmqServerPasswordId); ok && a.Value.Kind() == entity.KindString {
		o.Password = a.Value.String()
	}
	if a, ok := e.Get(RabbitmqServerSandboxPoolId); ok && a.Value.Kind() == entity.KindId {
		o.SandboxPool = a.Value.Id()
	}
	if a, ok := e.Get(RabbitmqServerServiceId); ok && a.Value.Kind() == entity.KindId {
		o.Service = a.Value.Id()
	}
	if a, ok := e.Get(RabbitmqServerStatusId); ok && a.Value.Kind() == entity.KindString {
		o.Status = a.Value.String()
	}
	if a, ok := e.Get(RabbitmqServerVariantId); ok && a.Value.Kind() == entity.KindString {
		o.Variant = a.Value.String()
	}
}

func (o *RabbitmqServer) Is(e entity.AttrGetter) bool {
	return entity.Is(e, KindRabbitmqServer)
}

func (o *RabbitmqServer) ShortKind() string {
	return "rabbitmq_server"
}

func (o *RabbitmqServer) Kind() entity.Id {
	return KindRabbitmqServer
}

func (o *RabbitmqServer) EntityId() entity.Id {
	return o.ID
}

func (o *RabbitmqServer) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.AddonName) {
		attrs = append(attrs, entity.String(RabbitmqServerAddonNameId, o.AddonName))
	}
	if !entity.Empty(o.AssociationCount) {
		attrs = append(attrs, entity.Int64(RabbitmqServerAssociationCountId, o.AssociationCount))
	}
	if !entity.Empty(o.Password) {
		attrs = append(attrs, entity.String(RabbitmqServerPasswordId, o.Password))
	}
	if !entity.Empty(o.SandboxPool) {
		attrs = append(attrs, entity.Ref(RabbitmqServerSandboxPoolId, o.SandboxPool))
	}
	if !entity.Empty(o.Service) {
		attrs = append(attrs, entity.Ref(RabbitmqServerServiceId, o.Service))
	}
	if !entity.Empty(o.Status) {
		attrs = append(attrs, entity.String(RabbitmqServerStatusId, o.Status))
	}
	if !entity.Empty(o.Variant) {
		attrs = append(attrs, entity.String(RabbitmqServerVariantId, o.Variant))
	}
	attrs = append(attrs, entity.Ref(entity.EntityKind, KindRabbitmqServer))
	return
}

func (o *RabbitmqServer) Empty() bool {
	if !entity.Empty(o.AddonName) {
		return false
	}
	if !entity.Empty(o.AssociationCount) {
		return false
	}
	if !entity.Empty(o.Password) {
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
	if !entity.Empty(o.Variant) {
		return false
	}
	return true
}

func (o *RabbitmqServer) InitSchema(sb *schema.SchemaBuilder) {
	sb.String("addon_name", "dev.miren.addon/rabbitmq_server.addon_name", schema.Indexed)
	sb.Int64("association_count", "dev.miren.addon/rabbitmq_server.association_count")
	sb.String("password", "dev.miren.addon/rabbitmq_server.password")
	sb.Ref("sandbox_pool", "dev.miren.addon/rabbitmq_server.sandbox_pool")
	sb.Ref("service", "dev.miren.addon/rabbitmq_server.service")
	sb.String("status", "dev.miren.addon/rabbitmq_server.status")
	sb.String("variant", "dev.miren.addon/rabbitmq_server.variant")
}

const (
	ValkeyDedicatedDataValkeyServerId = entity.Id("dev.miren.addon/valkey_dedicated_data.valkey_server")
)

type ValkeyDedicatedData struct {
	ID           entity.Id `json:"id"`
	ValkeyServer entity.Id `cbor:"valkey_server,omitempty" json:"valkey_server,omitempty"`
}

func (o *ValkeyDedicatedData) Decode(e entity.AttrGetter) {
	o.ID = entity.MustGet(e, entity.DBId).Value.Id()
	if a, ok := e.Get(ValkeyDedicatedDataValkeyServerId); ok && a.Value.Kind() == entity.KindId {
		o.ValkeyServer = a.Value.Id()
	}
}

func (o *ValkeyDedicatedData) Is(e entity.AttrGetter) bool {
	return entity.Is(e, KindValkeyDedicatedData)
}

func (o *ValkeyDedicatedData) ShortKind() string {
	return "valkey_dedicated_data"
}

func (o *ValkeyDedicatedData) Kind() entity.Id {
	return KindValkeyDedicatedData
}

func (o *ValkeyDedicatedData) EntityId() entity.Id {
	return o.ID
}

func (o *ValkeyDedicatedData) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.ValkeyServer) {
		attrs = append(attrs, entity.Ref(ValkeyDedicatedDataValkeyServerId, o.ValkeyServer))
	}
	attrs = append(attrs, entity.Ref(entity.EntityKind, KindValkeyDedicatedData))
	return
}

func (o *ValkeyDedicatedData) Empty() bool {
	return entity.Empty(o.ValkeyServer)
}

func (o *ValkeyDedicatedData) InitSchema(sb *schema.SchemaBuilder) {
	sb.Ref("valkey_server", "dev.miren.addon/valkey_dedicated_data.valkey_server")
}

const (
	ValkeyServerAddonNameId        = entity.Id("dev.miren.addon/valkey_server.addon_name")
	ValkeyServerAssociationCountId = entity.Id("dev.miren.addon/valkey_server.association_count")
	ValkeyServerPasswordId         = entity.Id("dev.miren.addon/valkey_server.password")
	ValkeyServerSandboxPoolId      = entity.Id("dev.miren.addon/valkey_server.sandbox_pool")
	ValkeyServerServiceId          = entity.Id("dev.miren.addon/valkey_server.service")
	ValkeyServerStatusId           = entity.Id("dev.miren.addon/valkey_server.status")
	ValkeyServerVariantId          = entity.Id("dev.miren.addon/valkey_server.variant")
)

type ValkeyServer struct {
	ID               entity.Id `json:"id"`
	AddonName        string    `cbor:"addon_name,omitempty" json:"addon_name,omitempty"`
	AssociationCount int64     `cbor:"association_count,omitempty" json:"association_count,omitempty"`
	Password         string    `cbor:"password,omitempty" json:"password,omitempty"`
	SandboxPool      entity.Id `cbor:"sandbox_pool,omitempty" json:"sandbox_pool,omitempty"`
	Service          entity.Id `cbor:"service,omitempty" json:"service,omitempty"`
	Status           string    `cbor:"status,omitempty" json:"status,omitempty"`
	Variant          string    `cbor:"variant,omitempty" json:"variant,omitempty"`
}

func (o *ValkeyServer) Decode(e entity.AttrGetter) {
	o.ID = entity.MustGet(e, entity.DBId).Value.Id()
	if a, ok := e.Get(ValkeyServerAddonNameId); ok && a.Value.Kind() == entity.KindString {
		o.AddonName = a.Value.String()
	}
	if a, ok := e.Get(ValkeyServerAssociationCountId); ok && a.Value.Kind() == entity.KindInt64 {
		o.AssociationCount = a.Value.Int64()
	}
	if a, ok := e.Get(ValkeyServerPasswordId); ok && a.Value.Kind() == entity.KindString {
		o.Password = a.Value.String()
	}
	if a, ok := e.Get(ValkeyServerSandboxPoolId); ok && a.Value.Kind() == entity.KindId {
		o.SandboxPool = a.Value.Id()
	}
	if a, ok := e.Get(ValkeyServerServiceId); ok && a.Value.Kind() == entity.KindId {
		o.Service = a.Value.Id()
	}
	if a, ok := e.Get(ValkeyServerStatusId); ok && a.Value.Kind() == entity.KindString {
		o.Status = a.Value.String()
	}
	if a, ok := e.Get(ValkeyServerVariantId); ok && a.Value.Kind() == entity.KindString {
		o.Variant = a.Value.String()
	}
}

func (o *ValkeyServer) Is(e entity.AttrGetter) bool {
	return entity.Is(e, KindValkeyServer)
}

func (o *ValkeyServer) ShortKind() string {
	return "valkey_server"
}

func (o *ValkeyServer) Kind() entity.Id {
	return KindValkeyServer
}

func (o *ValkeyServer) EntityId() entity.Id {
	return o.ID
}

func (o *ValkeyServer) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.AddonName) {
		attrs = append(attrs, entity.String(ValkeyServerAddonNameId, o.AddonName))
	}
	if !entity.Empty(o.AssociationCount) {
		attrs = append(attrs, entity.Int64(ValkeyServerAssociationCountId, o.AssociationCount))
	}
	if !entity.Empty(o.Password) {
		attrs = append(attrs, entity.String(ValkeyServerPasswordId, o.Password))
	}
	if !entity.Empty(o.SandboxPool) {
		attrs = append(attrs, entity.Ref(ValkeyServerSandboxPoolId, o.SandboxPool))
	}
	if !entity.Empty(o.Service) {
		attrs = append(attrs, entity.Ref(ValkeyServerServiceId, o.Service))
	}
	if !entity.Empty(o.Status) {
		attrs = append(attrs, entity.String(ValkeyServerStatusId, o.Status))
	}
	if !entity.Empty(o.Variant) {
		attrs = append(attrs, entity.String(ValkeyServerVariantId, o.Variant))
	}
	attrs = append(attrs, entity.Ref(entity.EntityKind, KindValkeyServer))
	return
}

func (o *ValkeyServer) Empty() bool {
	if !entity.Empty(o.AddonName) {
		return false
	}
	if !entity.Empty(o.AssociationCount) {
		return false
	}
	if !entity.Empty(o.Password) {
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
	if !entity.Empty(o.Variant) {
		return false
	}
	return true
}

func (o *ValkeyServer) InitSchema(sb *schema.SchemaBuilder) {
	sb.String("addon_name", "dev.miren.addon/valkey_server.addon_name", schema.Indexed)
	sb.Int64("association_count", "dev.miren.addon/valkey_server.association_count")
	sb.String("password", "dev.miren.addon/valkey_server.password")
	sb.Ref("sandbox_pool", "dev.miren.addon/valkey_server.sandbox_pool")
	sb.Ref("service", "dev.miren.addon/valkey_server.service")
	sb.String("status", "dev.miren.addon/valkey_server.status")
	sb.String("variant", "dev.miren.addon/valkey_server.variant")
}

var (
	KindAddon                   = entity.Id("dev.miren.addon/kind.addon")
	KindAddonAssociation        = entity.Id("dev.miren.addon/kind.addon_association")
	KindMemcacheDedicatedData   = entity.Id("dev.miren.addon/kind.memcache_dedicated_data")
	KindMemcacheServer          = entity.Id("dev.miren.addon/kind.memcache_server")
	KindMysqlDedicatedData      = entity.Id("dev.miren.addon/kind.mysql_dedicated_data")
	KindMysqlServer             = entity.Id("dev.miren.addon/kind.mysql_server")
	KindMysqlSharedData         = entity.Id("dev.miren.addon/kind.mysql_shared_data")
	KindPostgresServer          = entity.Id("dev.miren.addon/kind.postgres_server")
	KindPostgresqlDedicatedData = entity.Id("dev.miren.addon/kind.postgresql_dedicated_data")
	KindPostgresqlSharedData    = entity.Id("dev.miren.addon/kind.postgresql_shared_data")
	KindRabbitmqDedicatedData   = entity.Id("dev.miren.addon/kind.rabbitmq_dedicated_data")
	KindRabbitmqServer          = entity.Id("dev.miren.addon/kind.rabbitmq_server")
	KindValkeyDedicatedData     = entity.Id("dev.miren.addon/kind.valkey_dedicated_data")
	KindValkeyServer            = entity.Id("dev.miren.addon/kind.valkey_server")
	Schema                      = entity.Id("dev.miren.addon/schema.v1alpha")
)

func init() {
	schema.Register("dev.miren.addon", "v1alpha", func(sb *schema.SchemaBuilder) {
		(&Addon{}).InitSchema(sb)
		(&AddonAssociation{}).InitSchema(sb)
		(&MemcacheDedicatedData{}).InitSchema(sb)
		(&MemcacheServer{}).InitSchema(sb)
		(&MysqlDedicatedData{}).InitSchema(sb)
		(&MysqlServer{}).InitSchema(sb)
		(&MysqlSharedData{}).InitSchema(sb)
		(&PostgresServer{}).InitSchema(sb)
		(&PostgresqlDedicatedData{}).InitSchema(sb)
		(&PostgresqlSharedData{}).InitSchema(sb)
		(&RabbitmqDedicatedData{}).InitSchema(sb)
		(&RabbitmqServer{}).InitSchema(sb)
		(&ValkeyDedicatedData{}).InitSchema(sb)
		(&ValkeyServer{}).InitSchema(sb)
	})
	schema.RegisterEncodedSchema("dev.miren.addon", "v1alpha", []byte("\x1f\x8b\b\x00\x00\x00\x00\x00\x00\xff\xac\x99\xd9\xce\xeb&\x10ǟ\xa3\xfb\xbeW9:mժOc\x11C\x1c\x1a\xdb\xf8\x03\xe2\xf3岭\xd4>H\x97˾]{]\xd981̰\xda\xe7&rF\xf03\f\xff\x19`\xfc'\xedI\xc7\x04e\xe3\xa1\xe3\x92\xf5\aB\xa9\xe8م\xf7T\xfd\xf3\xfc&\xb0\xbf\x98\xec\xe6\xf1\xef\xb9\xe3\x156\xb0\xba\xffw\xa2\xa2#\xbc\x87\xf0Ӊ\xb3\x96\xaa\xdf\xff8r\xfa\xfc\x89\x17p\xa0\xecD\xae\xad\xaeF\"9\xe9\xf5}\x90\xaeQ\xdf\x06vRZ\xf2\xbe\xc9b1\xa9\xb8\xe8\x01k1B\xd6\a!\x96\xaa%\x1f\xf4\x9ds\xb1\r\x90\xf1a\x80\xc1\xd5В[5\xf5\x9f!\xadc\x81\x94\x8f\xfc\x94VԤ\xe5\xfaVu\x82\x1aL\xe7\x9a \a\xad\xa5\xe1<FA\xe1\xdb\xff\x9az\xbd\xeb\xef\xb5,\x81\xa2\x1d\xe9o\xff\xce]\xcf\x0f\xdb\xc4\xe0\xb5\xe8\x06ѳ^\xafOF2\byp\x91Y\xe2\xf9u\x9e\xd2\xc7ppwF\xf6:\xcds|?\x82ф\xb7\xf6,\x9b\xbb)1\xc9O㓼\x93\xb3&\xfb\xcb<ٷ\xe0(\x17\xc4\xe1\xc2n\xf3;\xeb\xe9\x01\xae\xfa;\xa1^#i\xaff͙y\xb4z6KT4\xe3K\xd2\x0eg\xd2\x0e\x92wDުi\xb4w\x0f\xf8\xf1\x8f\t\x86u\x15\xa5?T\x14m\xc5\xe6\x97\xe1 \x9d\xd3SwSOm\xa5\x98\x1c\x99\\V\xe3m\xd8\xd0nS\x90\xad>\x8bq\x8ci\r러\xffpY\x0eq\x90R\xa2\xe6d\x12kU\x8b\xeb\x92\xff\x9e\xb0y\xc2ּ\xd73\xf3\xcb(S\n\xa1\xab\x81(\xf5JHj\xf2\x85k\x82C\xfc\"\x8aS\xa4\xa7G\xf1\\\rB\xb4&\x899\x96\tv\xe4\xd4\x1f\xa5.\x88ɑ\xd7\xc6c\xcd\xfd\x8f\xdd\x1d\xe5?\xb7\xbb&\xfa\xaa\xe6ާ\xe5\x19N$\xfe~{\x87i<;KT\x87\xad\x8d\xc2\xf9~\x96\xe3H\xda\v\xbb\xb9z\xf4\x84\x8dը@\x90\x9fGA%\x8a|\x91 m\x92$̂\x00\xea\xc8\xf1\x1cT\"\x12\xb6K)\x90\":!\x00RJ\x8b\x9e\r\xc7\xe9\x9f\x16cb\x04\xbb\xd4\xd89,<X\x93\x1dYW\x93\xfa\xcc\\A\xbe\x87b\xc4m\x96%\xc9\xdf\x02i\xc8E\x95\x88\xf2e\x92\xb5I\x96_\xa7\xb0\x05\x92\xc2[\x02d\xa5D\x85\x82\x04\x11ҲJ\x8eb\x97\xb0\x04\xa0\x05\xa45\b\xa5\x1b\xc9TBZ\xa0YA\xb6C\xd2\x02\xa8]\xd2B\xac\xd7#-\x88\xdd#-\xc4*\x96\x16\"\xa4\xa5\xf5M\x92q\x1d\x98\xbc*&\xdd\xe3\x85\xf4ؓ\xb2\x85\xec}\xb2\x05\xb4\x80l%9\x1e\xb9\xee\x9e\x12\xb2\x05\xcd\xf6\xc8\x16\xa0v\xc9\x16\xb16\xc9\x16\x9d# 6s\xabF\xf2\x87\x9c=\xf2G\xacb\xf9#\u0086\xcc\n\x19\xfb$\nhx\xc8kť\xb2\xd6p\x11\xa9\xbfJa7\xdcs\xb9A0c7\x97F\xf3\x18=\\y\xfa\x0f\x83\xb9\xaaN\x0fv_t\xfa\xc4}\x99\x94BV\x1dS\x8a4K\xc1\xc35\xc1\x95C\x9a\xc6\xcc\xf8\xfa\xcfŁ\xafҔyя-\xb3\xeb\x04|5&*\x05\xf0\x05x\xb1\xd7\x17\x14\xd4F\xfc7\xf3\t\x12/\x18\xa0\xeb\xd6\xdaO\xb1^q\xcdG\xe3}\xbe\xfe\x9d\x18\xf4(D;\x13P\xe6\\\t\x9b\x8b\x0e\xab;\xfdwӀ\xcf\"a\x99ͱ\xaa\x86\x8d\xa7Z\x18\x1d\xf6\x13\xe2\x05\x02|\xb9G\x9e\x89d\xb4\xa2D\x93P\x80\xa3\x86\x05\x92@A\x86`\x87\xe9\xe7H\x14[7\xa3\xce5\xe5\x162,\xa6}G6)߱\xd8i T°hӁ\xe21\xb8\xf3\xe3_\xf6\x8a \"\x16\x81\xb5\"\x94Q^\x13\xed.J\xa0\xa0\xe0\xb6\xcdZ\x97\x9f\x03\xb7\x1c\x0f/Ӎ\xd1\xc9k\x1f\x18;\xdd.[x\x1d\x10\xba\xc4n\xf6\xc0\xb7Y@\xb7\x94b\xd4\xe9\x9ar\x9cp\xf5\xb2q\x9ew\xae4\xde\xe0\f\x9e\xae7G\xe8wy\xc4\xf20\xfd>\x13\fN\xcc\xe6[\t4F7\xee\x00xg؎~,>o\xba5\x0e\xafz\xc3w\xe5\xcd\xfa\xfd!\x13\t\xab/ƿИ\xa3\xe2W\x817\x04<\xf28f\xe6y$\xd0|\x8fG\x02Hx\xfb2\x1e\x81\xc6,\x8f\x04ހ\xb7(\x18\xd9^\x9f\xa0]!ء\xc0+?fC\x8b\"1\xea\x97\xe77\x82/\x81\xfd.\xea,\xa4\xae\xcc\xc7\xdf\xe5\x1bK\xe4\x13\xb0[\xfdN\x7f\x8c\x01\xf5Ɍryf\xe1\t\xb4ʼ\xf7\x83V\xe8*\x96U-\xc8>\xe1\xa1v\x9e\x93H\xe6\xd9п\x8d\xe7\x1fc\x02;`\xc19 \x94\x8eK6\xd1\xc2\f\x16h\x1d\x8c\xfa\xa2<\x18\x89\x91\xd2\xf4\xf1?\x00\x00\x00\xff\xff\x01\x00\x00\xff\xff\x8d\x96\x9aVQ!\x00\x00"))
}
