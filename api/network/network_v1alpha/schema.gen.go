package network_v1alpha

import (
	entity "miren.dev/runtime/pkg/entity"
	schema "miren.dev/runtime/pkg/entity/schema"
	types "miren.dev/runtime/pkg/entity/types"
)

const (
	EndpointsEndpointId = entity.Id("dev.miren.network/endpoints.endpoint")
	EndpointsTargetId   = entity.Id("dev.miren.network/endpoints.target")
)

type Endpoints struct {
	ID       entity.Id  `json:"id"`
	Endpoint []Endpoint `cbor:"endpoint,omitempty" json:"endpoint,omitempty"`
	Target   entity.Id  `cbor:"target,omitempty" json:"target,omitempty"`
}

func (o *Endpoints) Decode(e entity.AttrGetter) {
	o.ID = entity.MustGet(e, entity.DBId).Value.Id()
	for _, a := range e.GetAll(EndpointsEndpointId) {
		if a.Value.Kind() == entity.KindComponent {
			var v Endpoint
			v.Decode(a.Value.Component())
			o.Endpoint = append(o.Endpoint, v)
		}
	}
	if a, ok := e.Get(EndpointsTargetId); ok && a.Value.Kind() == entity.KindId {
		o.Target = a.Value.Id()
	}
}

func (o *Endpoints) Is(e entity.AttrGetter) bool {
	return entity.Is(e, KindEndpoints)
}

func (o *Endpoints) ShortKind() string {
	return "endpoints"
}

func (o *Endpoints) Kind() entity.Id {
	return KindEndpoints
}

func (o *Endpoints) EntityId() entity.Id {
	return o.ID
}

func (o *Endpoints) Encode() (attrs []entity.Attr) {
	for _, v := range o.Endpoint {
		attrs = append(attrs, entity.Component(EndpointsEndpointId, v.Encode()))
	}
	if !entity.Empty(o.Target) {
		attrs = append(attrs, entity.Ref(EndpointsTargetId, o.Target))
	}
	attrs = append(attrs, entity.Ref(entity.EntityKind, KindEndpoints))
	return
}

func (o *Endpoints) Empty() bool {
	if len(o.Endpoint) != 0 {
		return false
	}
	if !entity.Empty(o.Target) {
		return false
	}
	return true
}

func (o *Endpoints) InitSchema(sb *schema.SchemaBuilder) {
	sb.Component("endpoint", "dev.miren.network/endpoints.endpoint", schema.Doc("The endpoint configuration, per endpoint"), schema.Many)
	(&Endpoint{}).InitSchema(sb.Builder("endpoints.endpoint"))
	sb.Ref("target", "dev.miren.network/endpoints.target", schema.Doc("The target that uses these endpoints"), schema.Indexed)
}

const (
	EndpointIpId   = entity.Id("dev.miren.network/endpoint.ip")
	EndpointPortId = entity.Id("dev.miren.network/endpoint.port")
)

type Endpoint struct {
	Ip   string `cbor:"ip,omitempty" json:"ip,omitempty"`
	Port int64  `cbor:"port,omitempty" json:"port,omitempty"`
}

func (o *Endpoint) Decode(e entity.AttrGetter) {
	if a, ok := e.Get(EndpointIpId); ok && a.Value.Kind() == entity.KindString {
		o.Ip = a.Value.String()
	}
	if a, ok := e.Get(EndpointPortId); ok && a.Value.Kind() == entity.KindInt64 {
		o.Port = a.Value.Int64()
	}
}

func (o *Endpoint) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.Ip) {
		attrs = append(attrs, entity.String(EndpointIpId, o.Ip))
	}
	if !entity.Empty(o.Port) {
		attrs = append(attrs, entity.Int64(EndpointPortId, o.Port))
	}
	return
}

func (o *Endpoint) Empty() bool {
	if !entity.Empty(o.Ip) {
		return false
	}
	if !entity.Empty(o.Port) {
		return false
	}
	return true
}

func (o *Endpoint) InitSchema(sb *schema.SchemaBuilder) {
	sb.String("ip", "dev.miren.network/endpoint.ip", schema.Doc("The IP of the endpoint"))
	sb.Int64("port", "dev.miren.network/endpoint.port", schema.Doc("The port number"))
}

const (
	TargetIpId    = entity.Id("dev.miren.network/target.ip")
	TargetMatchId = entity.Id("dev.miren.network/target.match")
	TargetPortId  = entity.Id("dev.miren.network/target.port")
)

type Target struct {
	ID    entity.Id    `json:"id"`
	Ip    []string     `cbor:"ip,omitempty" json:"ip,omitempty"`
	Match types.Labels `cbor:"match,omitempty" json:"match,omitempty"`
	Port  []Port       `cbor:"port,omitempty" json:"port,omitempty"`
}

func (o *Target) Decode(e entity.AttrGetter) {
	o.ID = entity.MustGet(e, entity.DBId).Value.Id()
	for _, a := range e.GetAll(TargetIpId) {
		if a.Value.Kind() == entity.KindString {
			o.Ip = append(o.Ip, a.Value.String())
		}
	}
	for _, a := range e.GetAll(TargetMatchId) {
		if a.Value.Kind() == entity.KindLabel {
			o.Match = append(o.Match, a.Value.Label())
		}
	}
	for _, a := range e.GetAll(TargetPortId) {
		if a.Value.Kind() == entity.KindComponent {
			var v Port
			v.Decode(a.Value.Component())
			o.Port = append(o.Port, v)
		}
	}
}

func (o *Target) Is(e entity.AttrGetter) bool {
	return entity.Is(e, KindTarget)
}

func (o *Target) ShortKind() string {
	return "target"
}

func (o *Target) Kind() entity.Id {
	return KindTarget
}

func (o *Target) EntityId() entity.Id {
	return o.ID
}

func (o *Target) Encode() (attrs []entity.Attr) {
	for _, v := range o.Ip {
		attrs = append(attrs, entity.String(TargetIpId, v))
	}
	for _, v := range o.Match {
		attrs = append(attrs, entity.Label(TargetMatchId, v.Key, v.Value))
	}
	for _, v := range o.Port {
		attrs = append(attrs, entity.Component(TargetPortId, v.Encode()))
	}
	attrs = append(attrs, entity.Ref(entity.EntityKind, KindTarget))
	return
}

func (o *Target) Empty() bool {
	if len(o.Ip) != 0 {
		return false
	}
	if len(o.Match) != 0 {
		return false
	}
	if len(o.Port) != 0 {
		return false
	}
	return true
}

func (o *Target) InitSchema(sb *schema.SchemaBuilder) {
	sb.String("ip", "dev.miren.network/target.ip", schema.Doc("The IP allocated to the target"), schema.Many)
	sb.Label("match", "dev.miren.network/target.match", schema.Doc("A label to match against a sandbox"), schema.Many)
	sb.Component("port", "dev.miren.network/target.port", schema.Doc("A network port the target exposes"), schema.Many)
	(&Port{}).InitSchema(sb.Builder("target.port"))
}

const (
	PortNameId        = entity.Id("dev.miren.network/port.name")
	PortNodePortId    = entity.Id("dev.miren.network/port.node_port")
	PortPortId        = entity.Id("dev.miren.network/port.port")
	PortProtocolId    = entity.Id("dev.miren.network/port.protocol")
	PortProtocolTcpId = entity.Id("dev.miren.network/protocol.tcp")
	PortProtocolUdpId = entity.Id("dev.miren.network/protocol.udp")
	PortTargetPortId  = entity.Id("dev.miren.network/port.target_port")
	PortTypeId        = entity.Id("dev.miren.network/port.type")
)

type Port struct {
	Name       string       `cbor:"name" json:"name"`
	NodePort   int64        `cbor:"node_port,omitempty" json:"node_port,omitempty"`
	Port       int64        `cbor:"port" json:"port"`
	Protocol   PortProtocol `cbor:"protocol,omitempty" json:"protocol,omitempty"`
	TargetPort int64        `cbor:"target_port,omitempty" json:"target_port,omitempty"`
	Type       string       `cbor:"type,omitempty" json:"type,omitempty"`
}

type PortProtocol string

const (
	TCP PortProtocol = "protocol.tcp"
	UDP PortProtocol = "protocol.udp"
)

var PortprotocolFromId = map[entity.Id]PortProtocol{PortProtocolTcpId: TCP, PortProtocolUdpId: UDP}
var PortprotocolToId = map[PortProtocol]entity.Id{TCP: PortProtocolTcpId, UDP: PortProtocolUdpId}

func (o *Port) Decode(e entity.AttrGetter) {
	if a, ok := e.Get(PortNameId); ok && a.Value.Kind() == entity.KindString {
		o.Name = a.Value.String()
	}
	if a, ok := e.Get(PortNodePortId); ok && a.Value.Kind() == entity.KindInt64 {
		o.NodePort = a.Value.Int64()
	}
	if a, ok := e.Get(PortPortId); ok && a.Value.Kind() == entity.KindInt64 {
		o.Port = a.Value.Int64()
	}
	if a, ok := e.Get(PortProtocolId); ok && a.Value.Kind() == entity.KindId {
		o.Protocol = PortprotocolFromId[a.Value.Id()]
	}
	if a, ok := e.Get(PortTargetPortId); ok && a.Value.Kind() == entity.KindInt64 {
		o.TargetPort = a.Value.Int64()
	}
	if a, ok := e.Get(PortTypeId); ok && a.Value.Kind() == entity.KindString {
		o.Type = a.Value.String()
	}
}

func (o *Port) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.Name) {
		attrs = append(attrs, entity.String(PortNameId, o.Name))
	}
	if !entity.Empty(o.NodePort) {
		attrs = append(attrs, entity.Int64(PortNodePortId, o.NodePort))
	}
	attrs = append(attrs, entity.Int64(PortPortId, o.Port))
	if a, ok := PortprotocolToId[o.Protocol]; ok {
		attrs = append(attrs, entity.Ref(PortProtocolId, a))
	}
	if !entity.Empty(o.TargetPort) {
		attrs = append(attrs, entity.Int64(PortTargetPortId, o.TargetPort))
	}
	if !entity.Empty(o.Type) {
		attrs = append(attrs, entity.String(PortTypeId, o.Type))
	}
	return
}

func (o *Port) Empty() bool {
	if !entity.Empty(o.Name) {
		return false
	}
	if !entity.Empty(o.NodePort) {
		return false
	}
	if !entity.Empty(o.Port) {
		return false
	}
	if o.Protocol != "" {
		return false
	}
	if !entity.Empty(o.TargetPort) {
		return false
	}
	if !entity.Empty(o.Type) {
		return false
	}
	return true
}

func (o *Port) InitSchema(sb *schema.SchemaBuilder) {
	sb.String("name", "dev.miren.network/port.name", schema.Doc("Name of the port for reference"), schema.Required)
	sb.Int64("node_port", "dev.miren.network/port.node_port", schema.Doc("The port number that should be forwarded from the node to the container"))
	sb.Int64("port", "dev.miren.network/port.port", schema.Doc("Port number to listen on"), schema.Required)
	sb.Singleton("dev.miren.network/protocol.tcp")
	sb.Singleton("dev.miren.network/protocol.udp")
	sb.Ref("protocol", "dev.miren.network/port.protocol", schema.Doc("Port protocol"), schema.Choices(PortProtocolTcpId, PortProtocolUdpId))
	sb.Int64("target_port", "dev.miren.network/port.target_port", schema.Doc("Port number to target on the pod side"))
	sb.String("type", "dev.miren.network/port.type", schema.Doc("The highlevel type of the port"))
}

var (
	KindEndpoints = entity.Id("dev.miren.network/kind.endpoints")
	KindTarget    = entity.Id("dev.miren.network/kind.target")
	Schema        = entity.Id("dev.miren.network/schema.v1alpha")
)

func init() {
	schema.Register("dev.miren.network", "v1alpha", func(sb *schema.SchemaBuilder) {
		(&Endpoints{}).InitSchema(sb)
		(&Target{}).InitSchema(sb)
	})
	schema.RegisterEncodedSchema("dev.miren.network", "v1alpha", []byte("\x1f\x8b\b\x00\x00\x00\x00\x00\x00\xff\x8c\x94\xdb\xee\x940\x10\xc6_\xc3\xc4x\x88\xf1\xba\xc6'\"\xa53\xc0\x04z\xb0\x14ܽ\xd5\xc4\a\xf9\xef\xea\x1b\xea\xb5\xe9\xc0\x16\x14\xcarC\xe8\xe1\xf7\xcd\xf4\x9bi\xef`\xa4\xc6/\x80\xa3\xd0\xe4\xd1\b\x83\xe1\xab\xf5-\xb6d\xa0\x7f\xb9\xbc٬|\x8a+\"H_c\xf8\xc5\xf4\xe5\xd5vӴ>\xa9\xfc\xa9\xc0jIf\x1b\xa5\xaa\b;\xe8\xbf\xdfK\x82\xcb뜊 \aZ\x9a\xebo\x8eV\x92\x83puX\xf5\xc1\x93\xa9\x19}\x9bE\xb5\f\xaaY\xd18MD\x01\xecd\x89\xdd\xcf\xc8\xef\x9cr\xe6\x9d\xf5a\x85\x03\x8f#M\xcajg\r\x9a\xb0\xfc\xcdvl\xd5\xc4J\xed\xa4'?n\x19O\xa2\x86\xe0\\\x96\xcf\xca\x0f\xc6\xde\xe70\vX\xf0\x11\"F\xcb0\n(2\xe10h\x02\xe1\x1f\x86+\xf0.\xc7x\x1b\xac\xb2\x1dsM\x1aE\x16\xd0\f\xba\x8d\x9fb\x94݀\xfd\x8b\n\xca\xed\xd5\U00081260\x9c\x1a\xe0x\xcf\x00\x8eO\xf1!\x93\xd1T\x8aŅv=q\xca\aN>\xf9\xbe6\xbf\x1e\xd1\xf7dM=~\x96\x9dkd\xe7<i\xe9\xafE\xac9\xbbv\xb8\xa3\x9aR\xd9+\x1f_:4\xe0,\x99\xd0ύ\xb6\x93`\xdar\xb2;q\xff\x7f<\x10JQWנIsO\xae\xc2VXl\x85Ϧz\xcb\\ՇN|'v^\x88[\xa6?\x13\x96\xe9\xeb\xc3R%\ar\xbd\xb6\x1cs**\a\x98\v\xcc!J\x82\xc3\b\x94\x14\xfe\xdf\xd6\xf6\x8d\xf5\xa1\x98^\xe8G\xcf\x1c?ԋ\xd8\xf3\xe6\xfa\v\x00\x00\xff\xff\x01\x00\x00\xff\xff@\xd7\xc1\xe4\x13\x06\x00\x00"))
}
