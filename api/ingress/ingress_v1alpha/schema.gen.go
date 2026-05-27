package ingress_v1alpha

import (
	entity "miren.dev/runtime/pkg/entity"
	schema "miren.dev/runtime/pkg/entity/schema"
)

const (
	HttpRouteAppId           = entity.Id("dev.miren.ingress/http_route.app")
	HttpRouteAuthProviderId  = entity.Id("dev.miren.ingress/http_route.auth_provider")
	HttpRouteClaimMappingsId = entity.Id("dev.miren.ingress/http_route.claim_mappings")
	HttpRouteDefaultId       = entity.Id("dev.miren.ingress/http_route.default")
	HttpRouteHostId          = entity.Id("dev.miren.ingress/http_route.host")
	HttpRouteWafProfileId    = entity.Id("dev.miren.ingress/http_route.waf_profile")
)

type HttpRoute struct {
	ID            entity.Id       `json:"id"`
	App           entity.Id       `cbor:"app,omitempty" json:"app,omitempty"`
	AuthProvider  entity.Id       `cbor:"auth_provider,omitempty" json:"auth_provider,omitempty"`
	ClaimMappings []ClaimMappings `cbor:"claim_mappings,omitempty" json:"claim_mappings,omitempty"`
	Default       bool            `cbor:"default,omitempty" json:"default,omitempty"`
	Host          string          `cbor:"host,omitempty" json:"host,omitempty"`
	WafProfile    entity.Id       `cbor:"waf_profile,omitempty" json:"waf_profile,omitempty"`
}

func (o *HttpRoute) Decode(e entity.AttrGetter) {
	o.ID = entity.MustGet(e, entity.DBId).Value.Id()
	if a, ok := e.Get(HttpRouteAppId); ok && a.Value.Kind() == entity.KindId {
		o.App = a.Value.Id()
	}
	if a, ok := e.Get(HttpRouteAuthProviderId); ok && a.Value.Kind() == entity.KindId {
		o.AuthProvider = a.Value.Id()
	}
	for _, a := range e.GetAll(HttpRouteClaimMappingsId) {
		if a.Value.Kind() == entity.KindComponent {
			var v ClaimMappings
			v.Decode(a.Value.Component())
			o.ClaimMappings = append(o.ClaimMappings, v)
		}
	}
	if a, ok := e.Get(HttpRouteDefaultId); ok && a.Value.Kind() == entity.KindBool {
		o.Default = a.Value.Bool()
	}
	if a, ok := e.Get(HttpRouteHostId); ok && a.Value.Kind() == entity.KindString {
		o.Host = a.Value.String()
	}
	if a, ok := e.Get(HttpRouteWafProfileId); ok && a.Value.Kind() == entity.KindId {
		o.WafProfile = a.Value.Id()
	}
}

func (o *HttpRoute) Is(e entity.AttrGetter) bool {
	return entity.Is(e, KindHttpRoute)
}

func (o *HttpRoute) ShortKind() string {
	return "http_route"
}

func (o *HttpRoute) Kind() entity.Id {
	return KindHttpRoute
}

func (o *HttpRoute) EntityId() entity.Id {
	return o.ID
}

func (o *HttpRoute) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.App) {
		attrs = append(attrs, entity.Ref(HttpRouteAppId, o.App))
	}
	if !entity.Empty(o.AuthProvider) {
		attrs = append(attrs, entity.Ref(HttpRouteAuthProviderId, o.AuthProvider))
	}
	for _, v := range o.ClaimMappings {
		attrs = append(attrs, entity.Component(HttpRouteClaimMappingsId, v.Encode()))
	}
	attrs = append(attrs, entity.Bool(HttpRouteDefaultId, o.Default))
	if !entity.Empty(o.Host) {
		attrs = append(attrs, entity.String(HttpRouteHostId, o.Host))
	}
	if !entity.Empty(o.WafProfile) {
		attrs = append(attrs, entity.Ref(HttpRouteWafProfileId, o.WafProfile))
	}
	attrs = append(attrs, entity.Ref(entity.EntityKind, KindHttpRoute))
	return
}

func (o *HttpRoute) Empty() bool {
	if !entity.Empty(o.App) {
		return false
	}
	if !entity.Empty(o.AuthProvider) {
		return false
	}
	if len(o.ClaimMappings) != 0 {
		return false
	}
	if !entity.Empty(o.Default) {
		return false
	}
	if !entity.Empty(o.Host) {
		return false
	}
	if !entity.Empty(o.WafProfile) {
		return false
	}
	return true
}

func (o *HttpRoute) InitSchema(sb *schema.SchemaBuilder) {
	sb.Ref("app", "dev.miren.ingress/http_route.app", schema.Doc("The application to route to"), schema.Indexed, schema.Tags("dev.miren.app_ref"))
	sb.Ref("auth_provider", "dev.miren.ingress/http_route.auth_provider", schema.Doc("Reference to an auth provider (OIDC or password) for authentication"), schema.Indexed)
	sb.Component("claim_mappings", "dev.miren.ingress/http_route.claim_mappings", schema.Doc("Mappings from JWT claims to HTTP headers"), schema.Many)
	(&ClaimMappings{}).InitSchema(sb.Builder("http_route.claim_mappings"))
	sb.Bool("default", "dev.miren.ingress/http_route.default", schema.Doc("Whether this is the default route for routing"), schema.Indexed)
	sb.String("host", "dev.miren.ingress/http_route.host", schema.Doc("The hostname to match on for the application"), schema.Indexed)
	sb.Ref("waf_profile", "dev.miren.ingress/http_route.waf_profile", schema.Doc("Reference to a WAF profile for request filtering"))
}

const (
	ClaimMappingsClaimId  = entity.Id("dev.miren.ingress/claim_mappings.claim")
	ClaimMappingsHeaderId = entity.Id("dev.miren.ingress/claim_mappings.header")
)

type ClaimMappings struct {
	Claim  string `cbor:"claim,omitempty" json:"claim,omitempty"`
	Header string `cbor:"header,omitempty" json:"header,omitempty"`
}

func (o *ClaimMappings) Decode(e entity.AttrGetter) {
	if a, ok := e.Get(ClaimMappingsClaimId); ok && a.Value.Kind() == entity.KindString {
		o.Claim = a.Value.String()
	}
	if a, ok := e.Get(ClaimMappingsHeaderId); ok && a.Value.Kind() == entity.KindString {
		o.Header = a.Value.String()
	}
}

func (o *ClaimMappings) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.Claim) {
		attrs = append(attrs, entity.String(ClaimMappingsClaimId, o.Claim))
	}
	if !entity.Empty(o.Header) {
		attrs = append(attrs, entity.String(ClaimMappingsHeaderId, o.Header))
	}
	return
}

func (o *ClaimMappings) Empty() bool {
	if !entity.Empty(o.Claim) {
		return false
	}
	if !entity.Empty(o.Header) {
		return false
	}
	return true
}

func (o *ClaimMappings) InitSchema(sb *schema.SchemaBuilder) {
	sb.String("claim", "dev.miren.ingress/claim_mappings.claim", schema.Doc("The JWT claim name (e.g. email, sub, name)"))
	sb.String("header", "dev.miren.ingress/claim_mappings.header", schema.Doc("The HTTP header name to inject (e.g. X-User-Email)"))
}

const (
	OidcProviderClientIdId      = entity.Id("dev.miren.ingress/oidc_provider.client_id")
	OidcProviderClientSecretId  = entity.Id("dev.miren.ingress/oidc_provider.client_secret")
	OidcProviderConfigJsonId    = entity.Id("dev.miren.ingress/oidc_provider.config_json")
	OidcProviderConnectorTypeId = entity.Id("dev.miren.ingress/oidc_provider.connector_type")
	OidcProviderNameId          = entity.Id("dev.miren.ingress/oidc_provider.name")
	OidcProviderProviderUrlId   = entity.Id("dev.miren.ingress/oidc_provider.provider_url")
	OidcProviderScopesId        = entity.Id("dev.miren.ingress/oidc_provider.scopes")
)

type OidcProvider struct {
	ID            entity.Id `json:"id"`
	ClientId      string    `cbor:"client_id,omitempty" json:"client_id,omitempty"`
	ClientSecret  string    `cbor:"client_secret,omitempty" json:"client_secret,omitempty"`
	ConfigJson    string    `cbor:"config_json,omitempty" json:"config_json,omitempty"`
	ConnectorType string    `cbor:"connector_type,omitempty" json:"connector_type,omitempty"`
	Name          string    `cbor:"name,omitempty" json:"name,omitempty"`
	ProviderUrl   string    `cbor:"provider_url,omitempty" json:"provider_url,omitempty"`
	Scopes        string    `cbor:"scopes,omitempty" json:"scopes,omitempty"`
}

func (o *OidcProvider) Decode(e entity.AttrGetter) {
	o.ID = entity.MustGet(e, entity.DBId).Value.Id()
	if a, ok := e.Get(OidcProviderClientIdId); ok && a.Value.Kind() == entity.KindString {
		o.ClientId = a.Value.String()
	}
	if a, ok := e.Get(OidcProviderClientSecretId); ok && a.Value.Kind() == entity.KindString {
		o.ClientSecret = a.Value.String()
	}
	if a, ok := e.Get(OidcProviderConfigJsonId); ok && a.Value.Kind() == entity.KindString {
		o.ConfigJson = a.Value.String()
	}
	if a, ok := e.Get(OidcProviderConnectorTypeId); ok && a.Value.Kind() == entity.KindString {
		o.ConnectorType = a.Value.String()
	}
	if a, ok := e.Get(OidcProviderNameId); ok && a.Value.Kind() == entity.KindString {
		o.Name = a.Value.String()
	}
	if a, ok := e.Get(OidcProviderProviderUrlId); ok && a.Value.Kind() == entity.KindString {
		o.ProviderUrl = a.Value.String()
	}
	if a, ok := e.Get(OidcProviderScopesId); ok && a.Value.Kind() == entity.KindString {
		o.Scopes = a.Value.String()
	}
}

func (o *OidcProvider) Is(e entity.AttrGetter) bool {
	return entity.Is(e, KindOidcProvider)
}

func (o *OidcProvider) ShortKind() string {
	return "oidc_provider"
}

func (o *OidcProvider) Kind() entity.Id {
	return KindOidcProvider
}

func (o *OidcProvider) EntityId() entity.Id {
	return o.ID
}

func (o *OidcProvider) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.ClientId) {
		attrs = append(attrs, entity.String(OidcProviderClientIdId, o.ClientId))
	}
	if !entity.Empty(o.ClientSecret) {
		attrs = append(attrs, entity.String(OidcProviderClientSecretId, o.ClientSecret))
	}
	if !entity.Empty(o.ConfigJson) {
		attrs = append(attrs, entity.String(OidcProviderConfigJsonId, o.ConfigJson))
	}
	if !entity.Empty(o.ConnectorType) {
		attrs = append(attrs, entity.String(OidcProviderConnectorTypeId, o.ConnectorType))
	}
	if !entity.Empty(o.Name) {
		attrs = append(attrs, entity.String(OidcProviderNameId, o.Name))
	}
	if !entity.Empty(o.ProviderUrl) {
		attrs = append(attrs, entity.String(OidcProviderProviderUrlId, o.ProviderUrl))
	}
	if !entity.Empty(o.Scopes) {
		attrs = append(attrs, entity.String(OidcProviderScopesId, o.Scopes))
	}
	attrs = append(attrs, entity.Ref(entity.EntityKind, KindOidcProvider))
	return
}

func (o *OidcProvider) Empty() bool {
	if !entity.Empty(o.ClientId) {
		return false
	}
	if !entity.Empty(o.ClientSecret) {
		return false
	}
	if !entity.Empty(o.ConfigJson) {
		return false
	}
	if !entity.Empty(o.ConnectorType) {
		return false
	}
	if !entity.Empty(o.Name) {
		return false
	}
	if !entity.Empty(o.ProviderUrl) {
		return false
	}
	if !entity.Empty(o.Scopes) {
		return false
	}
	return true
}

func (o *OidcProvider) InitSchema(sb *schema.SchemaBuilder) {
	sb.String("client_id", "dev.miren.ingress/oidc_provider.client_id", schema.Doc("The OAuth2 client ID"))
	sb.String("client_secret", "dev.miren.ingress/oidc_provider.client_secret", schema.Doc("The OAuth2 client secret"))
	sb.String("config_json", "dev.miren.ingress/oidc_provider.config_json", schema.Doc("Connector-specific configuration as a JSON object (e.g. {\"orgs\":[{\"name\":\"mirendev\"}]}). Only meaningful when connector_type is non-empty and not \"oidc\"."))
	sb.String("connector_type", "dev.miren.ingress/oidc_provider.connector_type", schema.Doc("The provider implementation. Empty or \"oidc\" uses the built-in OIDC discovery client (consumes provider_url + scopes). Any other value names a Dex-backed connector (e.g. \"github\") and uses config_json for connector-specific configuration."), schema.Indexed)
	sb.String("name", "dev.miren.ingress/oidc_provider.name", schema.Doc("A unique name for this auth provider. Despite the kind name, this entity backs all OAuth2-flavored providers (OIDC discovery, GitHub, and other connector-based ones). Kept under the `oidc_provider` kind for backward compatibility with existing v0.8.0 entities."), schema.Indexed)
	sb.String("provider_url", "dev.miren.ingress/oidc_provider.provider_url", schema.Doc("The OIDC provider URL (e.g. https://accounts.google.com). Only meaningful when connector_type is empty or \"oidc\"."), schema.Indexed)
	sb.String("scopes", "dev.miren.ingress/oidc_provider.scopes", schema.Doc("Space-separated list of OAuth2 scopes (e.g. \"openid email profile\"). Only meaningful when connector_type is empty or \"oidc\"; connectors choose their own scopes."))
}

const (
	PasswordProviderNameId         = entity.Id("dev.miren.ingress/password_provider.name")
	PasswordProviderPasswordHashId = entity.Id("dev.miren.ingress/password_provider.password_hash")
)

type PasswordProvider struct {
	ID           entity.Id `json:"id"`
	Name         string    `cbor:"name,omitempty" json:"name,omitempty"`
	PasswordHash string    `cbor:"password_hash,omitempty" json:"password_hash,omitempty"`
}

func (o *PasswordProvider) Decode(e entity.AttrGetter) {
	o.ID = entity.MustGet(e, entity.DBId).Value.Id()
	if a, ok := e.Get(PasswordProviderNameId); ok && a.Value.Kind() == entity.KindString {
		o.Name = a.Value.String()
	}
	if a, ok := e.Get(PasswordProviderPasswordHashId); ok && a.Value.Kind() == entity.KindString {
		o.PasswordHash = a.Value.String()
	}
}

func (o *PasswordProvider) Is(e entity.AttrGetter) bool {
	return entity.Is(e, KindPasswordProvider)
}

func (o *PasswordProvider) ShortKind() string {
	return "password_provider"
}

func (o *PasswordProvider) Kind() entity.Id {
	return KindPasswordProvider
}

func (o *PasswordProvider) EntityId() entity.Id {
	return o.ID
}

func (o *PasswordProvider) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.Name) {
		attrs = append(attrs, entity.String(PasswordProviderNameId, o.Name))
	}
	if !entity.Empty(o.PasswordHash) {
		attrs = append(attrs, entity.String(PasswordProviderPasswordHashId, o.PasswordHash))
	}
	attrs = append(attrs, entity.Ref(entity.EntityKind, KindPasswordProvider))
	return
}

func (o *PasswordProvider) Empty() bool {
	if !entity.Empty(o.Name) {
		return false
	}
	if !entity.Empty(o.PasswordHash) {
		return false
	}
	return true
}

func (o *PasswordProvider) InitSchema(sb *schema.SchemaBuilder) {
	sb.String("name", "dev.miren.ingress/password_provider.name", schema.Doc("A unique name for this password provider"), schema.Indexed)
	sb.String("password_hash", "dev.miren.ingress/password_provider.password_hash", schema.Doc("bcrypt hash of the shared password"))
}

const (
	WafProfileParanoiaLevelId = entity.Id("dev.miren.ingress/waf_profile.paranoia_level")
)

type WafProfile struct {
	ID            entity.Id `json:"id"`
	ParanoiaLevel int64     `cbor:"paranoia_level,omitempty" json:"paranoia_level,omitempty"`
}

func (o *WafProfile) Decode(e entity.AttrGetter) {
	o.ID = entity.MustGet(e, entity.DBId).Value.Id()
	if a, ok := e.Get(WafProfileParanoiaLevelId); ok && a.Value.Kind() == entity.KindInt64 {
		o.ParanoiaLevel = a.Value.Int64()
	}
}

func (o *WafProfile) Is(e entity.AttrGetter) bool {
	return entity.Is(e, KindWafProfile)
}

func (o *WafProfile) ShortKind() string {
	return "waf_profile"
}

func (o *WafProfile) Kind() entity.Id {
	return KindWafProfile
}

func (o *WafProfile) EntityId() entity.Id {
	return o.ID
}

func (o *WafProfile) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.ParanoiaLevel) {
		attrs = append(attrs, entity.Int64(WafProfileParanoiaLevelId, o.ParanoiaLevel))
	}
	attrs = append(attrs, entity.Ref(entity.EntityKind, KindWafProfile))
	return
}

func (o *WafProfile) Empty() bool {
	return entity.Empty(o.ParanoiaLevel)
}

func (o *WafProfile) InitSchema(sb *schema.SchemaBuilder) {
	sb.Int64("paranoia_level", "dev.miren.ingress/waf_profile.paranoia_level", schema.Doc("OWASP CRS paranoia level (1-4)"))
}

var (
	KindHttpRoute        = entity.Id("dev.miren.ingress/kind.http_route")
	KindOidcProvider     = entity.Id("dev.miren.ingress/kind.oidc_provider")
	KindPasswordProvider = entity.Id("dev.miren.ingress/kind.password_provider")
	KindWafProfile       = entity.Id("dev.miren.ingress/kind.waf_profile")
	Schema               = entity.Id("dev.miren.ingress/schema.v1alpha")
)

func init() {
	schema.Register("dev.miren.ingress", "v1alpha", func(sb *schema.SchemaBuilder) {
		(&HttpRoute{}).InitSchema(sb)
		(&OidcProvider{}).InitSchema(sb)
		(&PasswordProvider{}).InitSchema(sb)
		(&WafProfile{}).InitSchema(sb)
	})
	schema.RegisterEncodedSchema("dev.miren.ingress", "v1alpha", []byte("\x1f\x8b\b\x00\x00\x00\x00\x00\x00\xff\x94\x96ے\xd30\f\x86_\x04\x86\xe3p&\xcc>\x91\xc7\x1b+\x89\xb6>\xad\xedv\xdbK\xb8\x80\aa\xe1\rᚉ\x9cnb;\x9b\x9a\x9b\x8e\xaaH\x9f,\xe5\xb7&\xf7Bs\x05\xb7\x02\x0e\x8dB\a\xbaA\xdd;\xf0\x1ev\xa8\x85\xbf?\xbe(\x9e|\x19\x9f4C\b\x969\xb3\x0f\xf0\x9b\b\xc7'e\xe0\x1c\x13i\x7f;a\x14G]V\xeb:\x04)\xfc\xf7\x9f\xd7(\x8eϷH\r\xb7\x96\n\xb6\xa3\x11N\x16\xaeQPڇ\xed\xb4}\x18\x98u\xe6\x80\x02\x1c\x01T\xea\x9aP\xbfF\xd4\xc7MT+9*\xa6\xb8\xb5\xa8{/\x14ק?D\xd4ٓ\x11\x89\xadQ\xd6h\xd0a\xb6\xa6\x89\x95U\x9aG\xabT\x0e\xf0\x1bM\xe2My\xfc\x94\x16\xe1t\n\x88\xe6x\xd4\xce\a\x87\xba'\xc4ۋ\x88\x01\xf8y\x92\xddd/ \xfd\x01\x9cG\xa3\xfb\xc3\x15\x97v\xe0\xd2:Tܝ\xd8؇\"ԙD\xf5^oN\\@\xc7\xf72P\xb1\xfe\xfcg\xac&\xae\x8d\x91\x04X\xd1\xe9\x020\x18\x1f\xb3\x05Yy\xb7\xef6\x93\xefx7ʤC\t\xc4\xd8-\x1d\x93l6\xfb\xbd\x99aǗ\x8fܧ\x05s\x92\xc7\xd32r\x11T)\x88\xaf\xd4ߧMTc\xb9\xe3\xda g\x12\x0e \xa3\x943\xdf\xd8f\x8b:l\xf6\xb9\x1c\xcc\xda\x1b\xa5F\r\x8a\xf6\xe1\xd6M\xad>+c\x93\xb0\xcaf\x7fP\xb3\xef/\xc0\x9aV\"\xe8\xc0PPq\x9c\xff\xe6\xb2\xf8\\I\xf2\xd0:\x88\xfaR\xa9+'\xae,\x96\x8cht\x87=\xbb\xf1FG\xad-\x1d9\xad\xa9\xa0ih\x83q\x8c.K\xdcQ\xa9/g\xae\xbc\xb6\x94I\xb7h\xfe\xc9\xf3W\xa4\x96\xe6\x9f\r\xb6wQj2\xf1伕]\x96\xf2|k,\xf8\xb8\x87&\xbbz\x0f%\xa4\xb5-@\x8a\xb5\xdc\xfb;\xe3D\xae\xdaWe|\x11\xfa_{{\xe5\x00\x05\xf0\xd2\xfc\xafj\x18\x0f\x9e\x81\xfb!\xea6u\xd5N\xf0\xb6`\xe7\xe1;?\x18\x17X\xfc\x9aX.\xc2\xcb\x1f\x16\xc9:\xa9؛\xd9\xeb\xacZ@e\x03\xf52\xf8\a\x00\x00\xff\xff\x01\x00\x00\xff\xff\xfc\xecL2;\t\x00\x00"))
}
