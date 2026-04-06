package addon_v1alpha

import (
	entity "miren.dev/runtime/pkg/entity"
	schema "miren.dev/runtime/pkg/entity/schema"
)

const (
	AddonDefaultVariantId = entity.Id("dev.miren.addon/addon.default_variant")
	AddonDescriptionId    = entity.Id("dev.miren.addon/addon.description")
	AddonDisplayNameId    = entity.Id("dev.miren.addon/addon.display_name")
	AddonLocalityModeId   = entity.Id("dev.miren.addon/addon.locality_mode")
	AddonNameId           = entity.Id("dev.miren.addon/addon.name")
	AddonVariantsId       = entity.Id("dev.miren.addon/addon.variants")
)

type Addon struct {
	ID             entity.Id  `json:"id"`
	DefaultVariant string     `cbor:"default_variant,omitempty" json:"default_variant,omitempty"`
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
)

type AddonAssociation struct {
	ID           entity.Id   `json:"id"`
	Addon        entity.Id   `cbor:"addon,omitempty" json:"addon,omitempty"`
	App          entity.Id   `cbor:"app,omitempty" json:"app,omitempty"`
	ErrorMessage string      `cbor:"error_message,omitempty" json:"error_message,omitempty"`
	Status       string      `cbor:"status,omitempty" json:"status,omitempty"`
	Variables    []Variables `cbor:"variables,omitempty" json:"variables,omitempty"`
	Variant      string      `cbor:"variant,omitempty" json:"variant,omitempty"`
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
	schema.RegisterEncodedSchema("dev.miren.addon", "v1alpha", []byte("\x1f\x8b\b\x00\x00\x00\x00\x00\x00\xff\xac\x99\xd9\xce\xeb&\x10ǟ\xa3\xfb\xbeW9:mժOc\x11C\x12\x1a\xdb\xf8\x03\xe2\xf3岭\xd4>H\x97˾]{]\x19\x92\x18fX\xeds\x139#\xf8\x19\x86\xff\f0\xfe\x93\x0e\xa4g\x82\xb2i\xd7sɆ\x1d\xa1T\f\xec\xcc\a\xaa\xfey~\x13\xd8_\xccv\xfb\xf8\xb7\xe9x\x81\r\x9c\xee\xff\x1d\xa8\xe8\t\x1f \xfcpଣ\xea\xb7?\xf6\x9c>\x7f\x12\x04\xec(;\x90K\xa7\x9b\x89HN\x06}\x1f\xa4o\xd4ב\x1d\x94\x96|8\x1a\xd6\a1\x96j%\x1f5\x17\x83\xe1\x9c]\x03d|\x18ap5v\xe4\xda\xcc\xfd\r\xa4\xf3,\x90\xf2Q\x98҉\x96t\\_\x9b^P\x8b\xe9}\x13\xe4 \xff[\xcec\x14\x14\xbe\xfd\xaf\xb9\u05fb\xe1^7\xb7)ړ\xe1\xfa\xaf\xe9zz\xd8f\x06oE?\x8a\x81\rzy\xb2ˌ\x90;\x1fY\xb4࿚)}\f\awg\x14\xaf\x93\x99\xe3\xfb\t\x8c&\xbcsgy\xbc\x9b2\x93\xfc4=\xc9;\xb9h\xb2\xbf\x98ɾ\x05GyC\xec\xce\xecj\xde\xd9\xce\x0fp\xd5߉\xf5\x9aHw\xb1k\xce\xec\xa3\xd3\xf381\xa9\xb8\x18\x8e\xd3Kҍ'ҍ\x92\xf7D^\x9by\xb4w\x0f\x84\xf1\x8f\t\xc6u\x95\xa4?T\x94l\xc5\xcc\xcbp\x90\x9a\x94\xd2_\xd5S\xd7(&'&o\xab\xf16l\xe8\xb6)Z\x83\xdf\xcdt?Kq\xaci\t럜\xffpYvi\x90R\xa2\xe5d\x16kӊ\xcb-g=a\xf3\x8cm\xf9\xa0\r\xf3\xcb$S\n\xa1\x9b\x91(\xf5JHj\xf3\x85o\x82C\xfc\"\x89Sd\xa0{\xf1܌Bt6\x89y\x96\x19\xb6\xe74\x1c\xa5>\x88ɉ\xb7\xd6c\xc7\xfb\x1f\xb7;\xca\x7f~wM\xf4E\x99އ\xdb3\x9cH\xfa\xfd\xee\xaep\f\xec\x06I\x1dv.\n\xe7{#ǉtgv\xf5\xf5\x18\b\x1b\xa7Q\x85 ?O\x82j\x14\xf9\"CZ%I\x98\x05\x01ԓ\xe3)\xaaD$l\x9fR!EtB\x00\xa4\x9c\x16\x03\x1b\x8e\xd7?/\xc6\xcc\b6\xa9\xb1\xf7Xx\xb06;\xb2\xbe%\xed\x89\xf9\x82|\x0fň߬\xe2\x14\x86Ӑ\x8f\xaa\x11\xe5\xcb,k\x95,\xbf\xcea+$\x85\xb7\x04\xc8ʉ\n\x05\t\"\xe4e\x95\x1d\xc5&a\t@\x8bHk\x14J\x1f%S\x19i\x81f\x15\xd9\x0eI\v\xa06I\v\xb1^\x8f\xb4 v\x8b\xb4\x10\xabZZ\x88\x90\x97\xd67Y\xc6ed\xf2\xa2\x98\xf4\x8f\x172`\xcf\xca\x16\xb2\xb7\xc9\x16\xd0\"\xb2\x95d\xbf\xe7\xba\x7f\xca\xc8\x164\xdb\"[\x80\xda$[\xc4Z%[t\x8e\x80\xd8\u00ad\x1a\xc9\x1fr\xb6\xc8\x1f\xb1\xaa\xe5\x8f\b+2+dl\x93(\xa0\xe1!/U\x92\xc6YÛH\xc3U\n\xb7a\xc5ƍf\x8a`\xd6n/\x8d\xf61y\xb8\n\xf4\x1fG{U\x9d\x1fܾ\xe8\xf4\x89\xfb2)\x85lz\xa6\x149\xde\n\x1e\xbe\t\xae\x1c\xd24f\xa6\xd7\xdf\x14\a\xbe\xcaS̢\xef;\xe6\xd6\t\xf8b\xccT\n\xe0\v\xf0b//\xa8\xa8\x8d\x84o\xe63$]0@\u05ed\xa5\x9fb\x83\xe2\x9aO\xd6\xfb|\xf9;3\xe8^\x88\xce\x10P\xe6\\\b\xab\x8b\x0e\x8b;\xc3wӈ\xcfֆ\xe5\x13\xe2E\x02\xf3v\xff;\x11\xc9hC\x89&\xb1\xc0D\r+\x96\x12\x05\a\x82\xed\xe6\x9f=Ql\xd9Dz\xdfTZ\x80p\x98\xee\xdd֦j\xcf\xe2\x86o\xac\xf4\xe0\xd0\xe6\x83\xc0cp\xa7ǿ\xe2\x15AD,\x02gE(\xa3\xbc%\xda_\x94H!\xc0o[\xb4.?Gn'\x01^\xa1\x1b\x93\x93\xd7!0v\xba[n\b: v\xf9\\\xed\x81o\x8b\x80~\tĪ\xd37\x958\xe1\x12d\xe3\xfc\xec]E\x82\xc1\x19=\x15\xaf\x8e\xd0\xefʈ\xf5a\xfa}!\x18\x9ct\xed7\x0ehLn\xb8\x11\xf0ư\x9d\xc2X|N\xf4k\x13A\xf5\xc6︫\xf5\xfbC!\x12VM\xac\x7f\xa1\xb1Dů\"o\x88x\xe4q<,\xf3H\xa4\xf9\x16\x8fD\x90\xf0\xd6d=\x02\x8dE\x1e\x89\xbc\x01oQ0\xb2\x83>A\xbbB\xb4C\x85W~,\x86VEb\xd2/\xcfoD_\x02\xfb\x9d\xd5IH\xdd\xd8\x0f\xad\xb7o#\x89ϭ~\xd5:\xff\x11\x05\xd4\x15\v\xca܅\x05#Ъ\xf0\xbe\x0eZ\xa1+T\xd1-\xbf\xf8\x84\x87\xda\x05N\"\x85g\xc3\xf06^~\x8c\x89\xec\x80\x15\xe7\x80X:\xae\xd9D+3X\xa4u4\xea\xab\xf2`\"Fj\xd3\xc7\xff\x00\x00\x00\xff\xff\x01\x00\x00\xff\xff\x94\x7f:8\xbd \x00\x00"))
}
