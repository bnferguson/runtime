package ingress_v1alpha

import (
	entity "miren.dev/runtime/pkg/entity"
	schema "miren.dev/runtime/pkg/entity/schema"
)

const (
	HttpRouteAppId           = entity.Id("dev.miren.ingress/http_route.app")
	HttpRouteClaimMappingsId = entity.Id("dev.miren.ingress/http_route.claim_mappings")
	HttpRouteDefaultId       = entity.Id("dev.miren.ingress/http_route.default")
	HttpRouteHostId          = entity.Id("dev.miren.ingress/http_route.host")
	HttpRouteOidcProviderId  = entity.Id("dev.miren.ingress/http_route.oidc_provider")
	HttpRouteWafProfileId    = entity.Id("dev.miren.ingress/http_route.waf_profile")
)

type HttpRoute struct {
	ID            entity.Id       `json:"id"`
	App           entity.Id       `cbor:"app,omitempty" json:"app,omitempty"`
	ClaimMappings []ClaimMappings `cbor:"claim_mappings,omitempty" json:"claim_mappings,omitempty"`
	Default       bool            `cbor:"default,omitempty" json:"default,omitempty"`
	Host          string          `cbor:"host,omitempty" json:"host,omitempty"`
	OidcProvider  entity.Id       `cbor:"oidc_provider,omitempty" json:"oidc_provider,omitempty"`
	WafProfile    entity.Id       `cbor:"waf_profile,omitempty" json:"waf_profile,omitempty"`
}

func (o *HttpRoute) Decode(e entity.AttrGetter) {
	o.ID = entity.MustGet(e, entity.DBId).Value.Id()
	if a, ok := e.Get(HttpRouteAppId); ok && a.Value.Kind() == entity.KindId {
		o.App = a.Value.Id()
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
	if a, ok := e.Get(HttpRouteOidcProviderId); ok && a.Value.Kind() == entity.KindId {
		o.OidcProvider = a.Value.Id()
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
	for _, v := range o.ClaimMappings {
		attrs = append(attrs, entity.Component(HttpRouteClaimMappingsId, v.Encode()))
	}
	attrs = append(attrs, entity.Bool(HttpRouteDefaultId, o.Default))
	if !entity.Empty(o.Host) {
		attrs = append(attrs, entity.String(HttpRouteHostId, o.Host))
	}
	if !entity.Empty(o.OidcProvider) {
		attrs = append(attrs, entity.Ref(HttpRouteOidcProviderId, o.OidcProvider))
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
	if len(o.ClaimMappings) != 0 {
		return false
	}
	if !entity.Empty(o.Default) {
		return false
	}
	if !entity.Empty(o.Host) {
		return false
	}
	if !entity.Empty(o.OidcProvider) {
		return false
	}
	if !entity.Empty(o.WafProfile) {
		return false
	}
	return true
}

func (o *HttpRoute) InitSchema(sb *schema.SchemaBuilder) {
	sb.Ref("app", "dev.miren.ingress/http_route.app", schema.Doc("The application to route to"), schema.Indexed, schema.Tags("dev.miren.app_ref"))
	sb.Component("claim_mappings", "dev.miren.ingress/http_route.claim_mappings", schema.Doc("Mappings from JWT claims to HTTP headers"), schema.Many)
	(&ClaimMappings{}).InitSchema(sb.Builder("http_route.claim_mappings"))
	sb.Bool("default", "dev.miren.ingress/http_route.default", schema.Doc("Whether this is the default route for routing"), schema.Indexed)
	sb.String("host", "dev.miren.ingress/http_route.host", schema.Doc("The hostname to match on for the application"), schema.Indexed)
	sb.Ref("oidc_provider", "dev.miren.ingress/http_route.oidc_provider", schema.Doc("Reference to an OIDC provider for authentication"), schema.Indexed)
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
	OidcProviderClientIdId     = entity.Id("dev.miren.ingress/oidc_provider.client_id")
	OidcProviderClientSecretId = entity.Id("dev.miren.ingress/oidc_provider.client_secret")
	OidcProviderNameId         = entity.Id("dev.miren.ingress/oidc_provider.name")
	OidcProviderProviderUrlId  = entity.Id("dev.miren.ingress/oidc_provider.provider_url")
	OidcProviderScopesId       = entity.Id("dev.miren.ingress/oidc_provider.scopes")
)

type OidcProvider struct {
	ID           entity.Id `json:"id"`
	ClientId     string    `cbor:"client_id,omitempty" json:"client_id,omitempty"`
	ClientSecret string    `cbor:"client_secret,omitempty" json:"client_secret,omitempty"`
	Name         string    `cbor:"name,omitempty" json:"name,omitempty"`
	ProviderUrl  string    `cbor:"provider_url,omitempty" json:"provider_url,omitempty"`
	Scopes       string    `cbor:"scopes,omitempty" json:"scopes,omitempty"`
}

func (o *OidcProvider) Decode(e entity.AttrGetter) {
	o.ID = entity.MustGet(e, entity.DBId).Value.Id()
	if a, ok := e.Get(OidcProviderClientIdId); ok && a.Value.Kind() == entity.KindString {
		o.ClientId = a.Value.String()
	}
	if a, ok := e.Get(OidcProviderClientSecretId); ok && a.Value.Kind() == entity.KindString {
		o.ClientSecret = a.Value.String()
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
	sb.String("name", "dev.miren.ingress/oidc_provider.name", schema.Doc("A unique name for this OIDC provider"), schema.Indexed)
	sb.String("provider_url", "dev.miren.ingress/oidc_provider.provider_url", schema.Doc("The OIDC provider URL (e.g. https://accounts.google.com)"), schema.Indexed)
	sb.String("scopes", "dev.miren.ingress/oidc_provider.scopes", schema.Doc("Space-separated list of OAuth2 scopes (e.g. \"openid email profile\")"))
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
	KindHttpRoute    = entity.Id("dev.miren.ingress/kind.http_route")
	KindOidcProvider = entity.Id("dev.miren.ingress/kind.oidc_provider")
	KindWafProfile   = entity.Id("dev.miren.ingress/kind.waf_profile")
	Schema           = entity.Id("dev.miren.ingress/schema.v1alpha")
)

func init() {
	schema.Register("dev.miren.ingress", "v1alpha", func(sb *schema.SchemaBuilder) {
		(&HttpRoute{}).InitSchema(sb)
		(&OidcProvider{}).InitSchema(sb)
		(&WafProfile{}).InitSchema(sb)
	})
	schema.RegisterEncodedSchema("dev.miren.ingress", "v1alpha", []byte("\x1f\x8b\b\x00\x00\x00\x00\x00\x00\xff\x8c\x95\xdb\xf2\xd3 \x10\xc6_D\xc7\xc38\x9e\x8d\xe3\x13ehX\x92\xb5\x9c\x04\x1a\xdbK\x9d\xd1\x17\xf9\xabo\xa8\xd7\x0eK:\t$\xa5\xb9\xe9la\xf7\xb7|\xf0A~q\xcd\x14|\xe106\n\x1d\xe8\x06u\xef\xc0{8\xa2\xe6\xfe\xe1\xfcl5\xf31\xce4C\b\xb6u\xe6\x14\xe0\x0f\x11Ώ։sN\xa2\xfd\x13\xdc(\x86z\xddM\b\x04\xc9\xfdχ\x03\xf2\xf3\xd3\x1a\xa9a\xd6R\xc3.\x06\xe1b\xe1\x80\xfcw,{W-\xeb$C\xd5*f-\xea\xdes\xc5\xf4\xe5/qt1\x13\x91\xd8\x19e\x8d\x06\x1d\xe6h\x92\xb9\xee\xd2\xdc\xec\xb2S\xf5wR\xfdr\xbd\xfc\x9c\x96\xe0\xb4\nHa\\\xaa\xf0\xc1\xa1\xee\t\xf1\xea.b\x00\xc6\xc1\x11CL\xf1\x02ҏ\xe0<\x1aݏ\x9f\x98\xb4\x03\x93֡b\xee\xd2F\x1d\x8aPW\x12\xf5{Q\xddq\x0e\x82\x9dd\xa0f\xfd\xf5O\xec\xc6\x0f\xc6H\x02l\x98k\x01\x18\x8cO՜\xa2R\xed\xdbj\xb1A\u07b5֙\x11\xaf\x82U>4Y\x87P\xaf\xab\xa8\xafL\xc42\x81\x12\bt\\\x0eL\x98\xea\xd6}\x9ea\xe7\xe77\xeeӂ99\xed\xf1:s\x91\xb4\xd3[\xdfH\xdf\xfb*\xaa\xb1\xcc1m\x90\xb5\x12F\x90\xe9V\x14cQf\x87:Tu.7f\xcb\x1c$4;\x85I\xea\x93un\x96\xb6S\xec\x0f\x12\xfb\xe6\x0e\xac\xe9$\x82\x0e-rj\x8e\xf3\xdf\xd2a\x1fv\x92<t\x0e\x92UU>T\x1276%'\x92\xdd矲~\xe3 \xf3\xfakО\\:H\x99\x8d\x94\xbc\x8dG'\xe7\xf9\xceX\xf0\xe9\xc1\x98\xe2\xdd\x0fFF*S\x8f~0.\xb4\xe9+\xb3\xbc \xf7?8\x99\xcdvܧ|!\xfb\x8c\xf9\x1f\x00\x00\xff\xff\x01\x00\x00\xff\xff\xe5\x12\x94\f\x17\a\x00\x00"))
}
