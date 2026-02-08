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
)

type HttpRoute struct {
	ID            entity.Id       `json:"id"`
	App           entity.Id       `cbor:"app,omitempty" json:"app,omitempty"`
	ClaimMappings []ClaimMappings `cbor:"claim_mappings,omitempty" json:"claim_mappings,omitempty"`
	Default       bool            `cbor:"default,omitempty" json:"default,omitempty"`
	Host          string          `cbor:"host,omitempty" json:"host,omitempty"`
	OidcProvider  entity.Id       `cbor:"oidc_provider,omitempty" json:"oidc_provider,omitempty"`
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
	return true
}

func (o *HttpRoute) InitSchema(sb *schema.SchemaBuilder) {
	sb.Ref("app", "dev.miren.ingress/http_route.app", schema.Doc("The application to route to"), schema.Indexed, schema.Tags("dev.miren.app_ref"))
	sb.Component("claim_mappings", "dev.miren.ingress/http_route.claim_mappings", schema.Doc("Mappings from JWT claims to HTTP headers"), schema.Many)
	(&ClaimMappings{}).InitSchema(sb.Builder("http_route.claim_mappings"))
	sb.Bool("default", "dev.miren.ingress/http_route.default", schema.Doc("Whether this is the default route for routing"), schema.Indexed)
	sb.String("host", "dev.miren.ingress/http_route.host", schema.Doc("The hostname to match on for the application"), schema.Indexed)
	sb.Ref("oidc_provider", "dev.miren.ingress/http_route.oidc_provider", schema.Doc("Reference to an OIDC provider for authentication"), schema.Indexed)
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

var (
	KindHttpRoute    = entity.Id("dev.miren.ingress/kind.http_route")
	KindOidcProvider = entity.Id("dev.miren.ingress/kind.oidc_provider")
	Schema           = entity.Id("dev.miren.ingress/schema.v1alpha")
)

func init() {
	schema.Register("dev.miren.ingress", "v1alpha", func(sb *schema.SchemaBuilder) {
		(&HttpRoute{}).InitSchema(sb)
		(&OidcProvider{}).InitSchema(sb)
	})
	schema.RegisterEncodedSchema("dev.miren.ingress", "v1alpha", []byte("\x1f\x8b\b\x00\x00\x00\x00\x00\x00\xff\x94\x94\xdbN\x840\x10\x86_\xc4DM\x8c\xc6\x13\xc6'\"]f\x80q{\xb2\xed\x92\xdd[\x13_\xc4\xd3\x1b\xea\xb5a\x80\xb0\x05\x04\xbc\xd9\f\xed\xcc\xf7\xf7\xef\xce\xf4\x03\xb4P\xf8\fX%\x8a\x1c\xea\x84t\xe1\xd0{ܒ\x06\xff\xb6?\x1f\xed<\xd4;I\x19\x82M\x9d\xd9\x05\xfcb\xc2\xfed\x9c\xd8\xe74\xb4\x9f\x1c\x8c\x12\xa4\xc7jyN(\xc1\xbf\xbeo\b\xf6gs\xa4DX˂Y\x1d\x84\x83\xc5\r\xc1g]v;[\x96IA*U\xc2Z҅\a%\xf4\xe1\x9b9z\xb0S#)3\xca\x1a\x8d:\xf4Qks\xac\x92\xfc\xa9\xb2\xd2\xf5\v\xbb\xbe\x1c\x1f?\xa65p>\x056a}\xd4\xdc\aG\xba`\xc4\xd5\"\xa2D\x01蘑\xb7\xf1\x11\xa4\xa8\xd0y2\xba\xa8\x1e\x85\xb4\xa5\x90֑\x12\xee\x90\xd6>\x14\xa3:\x12\xeb]\xcc\xde8`.v2\xb0X\xd1}\xd4j\xb01F2`\xa2\xb9\x8e\x00\xa5\xf1M5p4t{3[l\b\xb2\xd4:SQgX\xc5Km\xeb\xccz~\xea\x81Sfy\x10\"j\xdb$\xa7\xe3\xdc(\xed_\xe3p\xbd\x00K2I\xa8CJ\xc0\xe2\xd4\x7f\x0eo\xec~%\xc9c氹z\x15/\r\x89\x13\x97\x12\x13\xf9\xef\xeb\x7f\x86\xf5wK\xf5]\x90\xee\x9cd\x84\x8cV\x86\xbc\x89!\x8ay>3\x16}3\x00m\xbcz\x00\"\xd20u\xebK\xe3Bڼ\x9a\xc7}\xb3\xfc\x80\xc6\xe0u\x8d\xf6\v\x00\x00\xff\xff\x01\x00\x00\xff\xff+\xdeU\"\xb7\x05\x00\x00"))
}
