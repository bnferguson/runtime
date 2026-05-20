package core_v1alpha

import (
	"time"

	entity "miren.dev/runtime/pkg/entity"
	schema "miren.dev/runtime/pkg/entity/schema"
	types "miren.dev/runtime/pkg/entity/types"
)

const (
	ConfigSpecEntrypointId     = entity.Id("dev.miren.core/component.config_spec.entrypoint")
	ConfigSpecServicesId       = entity.Id("dev.miren.core/component.config_spec.services")
	ConfigSpecStartDirectoryId = entity.Id("dev.miren.core/component.config_spec.start_directory")
	ConfigSpecVariablesId      = entity.Id("dev.miren.core/component.config_spec.variables")
)

type ConfigSpec struct {
	Entrypoint     string                `cbor:"entrypoint,omitempty" json:"entrypoint,omitempty"`
	Services       []ConfigSpecServices  `cbor:"services,omitempty" json:"services,omitempty"`
	StartDirectory string                `cbor:"start_directory,omitempty" json:"start_directory,omitempty"`
	Variables      []ConfigSpecVariables `cbor:"variables,omitempty" json:"variables,omitempty"`
}

func (o *ConfigSpec) Decode(e entity.AttrGetter) {
	if a, ok := e.Get(ConfigSpecEntrypointId); ok && a.Value.Kind() == entity.KindString {
		o.Entrypoint = a.Value.String()
	}
	for _, a := range e.GetAll(ConfigSpecServicesId) {
		if a.Value.Kind() == entity.KindComponent {
			var v ConfigSpecServices
			v.Decode(a.Value.Component())
			o.Services = append(o.Services, v)
		}
	}
	if a, ok := e.Get(ConfigSpecStartDirectoryId); ok && a.Value.Kind() == entity.KindString {
		o.StartDirectory = a.Value.String()
	}
	for _, a := range e.GetAll(ConfigSpecVariablesId) {
		if a.Value.Kind() == entity.KindComponent {
			var v ConfigSpecVariables
			v.Decode(a.Value.Component())
			o.Variables = append(o.Variables, v)
		}
	}
}

func (o *ConfigSpec) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.Entrypoint) {
		attrs = append(attrs, entity.String(ConfigSpecEntrypointId, o.Entrypoint))
	}
	for _, v := range o.Services {
		attrs = append(attrs, entity.Component(ConfigSpecServicesId, v.Encode()))
	}
	if !entity.Empty(o.StartDirectory) {
		attrs = append(attrs, entity.String(ConfigSpecStartDirectoryId, o.StartDirectory))
	}
	for _, v := range o.Variables {
		attrs = append(attrs, entity.Component(ConfigSpecVariablesId, v.Encode()))
	}
	return
}

func (o *ConfigSpec) Empty() bool {
	if !entity.Empty(o.Entrypoint) {
		return false
	}
	if len(o.Services) != 0 {
		return false
	}
	if !entity.Empty(o.StartDirectory) {
		return false
	}
	if len(o.Variables) != 0 {
		return false
	}
	return true
}

func (o *ConfigSpec) InitSchema(sb *schema.SchemaBuilder) {
	sb.String("entrypoint", "dev.miren.core/component.config_spec.entrypoint", schema.Doc("The container entrypoint command"))
	sb.Component("services", "dev.miren.core/component.config_spec.services", schema.Doc("Per-service configuration"), schema.Many)
	(&ConfigSpecServices{}).InitSchema(sb.Builder("component.config_spec.services"))
	sb.String("start_directory", "dev.miren.core/component.config_spec.start_directory", schema.Doc("Directory to start the process in; defaults to /app."))
	sb.Component("variables", "dev.miren.core/component.config_spec.variables", schema.Doc("Environment variables and configuration values"), schema.Many)
	(&ConfigSpecVariables{}).InitSchema(sb.Builder("component.config_spec.variables"))
}

const (
	ConfigSpecServicesCommandId     = entity.Id("dev.miren.core/component.config_spec.services.command")
	ConfigSpecServicesConcurrencyId = entity.Id("dev.miren.core/component.config_spec.services.concurrency")
	ConfigSpecServicesDisksId       = entity.Id("dev.miren.core/component.config_spec.services.disks")
	ConfigSpecServicesEnvId         = entity.Id("dev.miren.core/component.config_spec.services.env")
	ConfigSpecServicesImageId       = entity.Id("dev.miren.core/component.config_spec.services.image")
	ConfigSpecServicesNameId        = entity.Id("dev.miren.core/component.config_spec.services.name")
	ConfigSpecServicesPortId        = entity.Id("dev.miren.core/component.config_spec.services.port")
	ConfigSpecServicesPortNameId    = entity.Id("dev.miren.core/component.config_spec.services.port_name")
	ConfigSpecServicesPortTimeoutId = entity.Id("dev.miren.core/component.config_spec.services.port_timeout")
	ConfigSpecServicesPortTypeId    = entity.Id("dev.miren.core/component.config_spec.services.port_type")
	ConfigSpecServicesPortsId       = entity.Id("dev.miren.core/component.config_spec.services.ports")
)

type ConfigSpecServices struct {
	Command     string                        `cbor:"command,omitempty" json:"command,omitempty"`
	Concurrency ConfigSpecServicesConcurrency `cbor:"concurrency,omitempty" json:"concurrency,omitempty"`
	Disks       []ConfigSpecServicesDisks     `cbor:"disks,omitempty" json:"disks,omitempty"`
	Env         []ConfigSpecServicesEnv       `cbor:"env,omitempty" json:"env,omitempty"`
	Image       string                        `cbor:"image,omitempty" json:"image,omitempty"`
	Name        string                        `cbor:"name,omitempty" json:"name,omitempty"`
	Port        int64                         `cbor:"port,omitempty" json:"port,omitempty"`
	PortName    string                        `cbor:"port_name,omitempty" json:"port_name,omitempty"`
	PortTimeout string                        `cbor:"port_timeout,omitempty" json:"port_timeout,omitempty"`
	PortType    string                        `cbor:"port_type,omitempty" json:"port_type,omitempty"`
	Ports       []ConfigSpecServicesPorts     `cbor:"ports,omitempty" json:"ports,omitempty"`
}

func (o *ConfigSpecServices) Decode(e entity.AttrGetter) {
	if a, ok := e.Get(ConfigSpecServicesCommandId); ok && a.Value.Kind() == entity.KindString {
		o.Command = a.Value.String()
	}
	if a, ok := e.Get(ConfigSpecServicesConcurrencyId); ok && a.Value.Kind() == entity.KindComponent {
		o.Concurrency.Decode(a.Value.Component())
	}
	for _, a := range e.GetAll(ConfigSpecServicesDisksId) {
		if a.Value.Kind() == entity.KindComponent {
			var v ConfigSpecServicesDisks
			v.Decode(a.Value.Component())
			o.Disks = append(o.Disks, v)
		}
	}
	for _, a := range e.GetAll(ConfigSpecServicesEnvId) {
		if a.Value.Kind() == entity.KindComponent {
			var v ConfigSpecServicesEnv
			v.Decode(a.Value.Component())
			o.Env = append(o.Env, v)
		}
	}
	if a, ok := e.Get(ConfigSpecServicesImageId); ok && a.Value.Kind() == entity.KindString {
		o.Image = a.Value.String()
	}
	if a, ok := e.Get(ConfigSpecServicesNameId); ok && a.Value.Kind() == entity.KindString {
		o.Name = a.Value.String()
	}
	if a, ok := e.Get(ConfigSpecServicesPortId); ok && a.Value.Kind() == entity.KindInt64 {
		o.Port = a.Value.Int64()
	}
	if a, ok := e.Get(ConfigSpecServicesPortNameId); ok && a.Value.Kind() == entity.KindString {
		o.PortName = a.Value.String()
	}
	if a, ok := e.Get(ConfigSpecServicesPortTimeoutId); ok && a.Value.Kind() == entity.KindString {
		o.PortTimeout = a.Value.String()
	}
	if a, ok := e.Get(ConfigSpecServicesPortTypeId); ok && a.Value.Kind() == entity.KindString {
		o.PortType = a.Value.String()
	}
	for _, a := range e.GetAll(ConfigSpecServicesPortsId) {
		if a.Value.Kind() == entity.KindComponent {
			var v ConfigSpecServicesPorts
			v.Decode(a.Value.Component())
			o.Ports = append(o.Ports, v)
		}
	}
}

func (o *ConfigSpecServices) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.Command) {
		attrs = append(attrs, entity.String(ConfigSpecServicesCommandId, o.Command))
	}
	if !o.Concurrency.Empty() {
		attrs = append(attrs, entity.Component(ConfigSpecServicesConcurrencyId, o.Concurrency.Encode()))
	}
	for _, v := range o.Disks {
		attrs = append(attrs, entity.Component(ConfigSpecServicesDisksId, v.Encode()))
	}
	for _, v := range o.Env {
		attrs = append(attrs, entity.Component(ConfigSpecServicesEnvId, v.Encode()))
	}
	if !entity.Empty(o.Image) {
		attrs = append(attrs, entity.String(ConfigSpecServicesImageId, o.Image))
	}
	if !entity.Empty(o.Name) {
		attrs = append(attrs, entity.String(ConfigSpecServicesNameId, o.Name))
	}
	if !entity.Empty(o.Port) {
		attrs = append(attrs, entity.Int64(ConfigSpecServicesPortId, o.Port))
	}
	if !entity.Empty(o.PortName) {
		attrs = append(attrs, entity.String(ConfigSpecServicesPortNameId, o.PortName))
	}
	if !entity.Empty(o.PortTimeout) {
		attrs = append(attrs, entity.String(ConfigSpecServicesPortTimeoutId, o.PortTimeout))
	}
	if !entity.Empty(o.PortType) {
		attrs = append(attrs, entity.String(ConfigSpecServicesPortTypeId, o.PortType))
	}
	for _, v := range o.Ports {
		attrs = append(attrs, entity.Component(ConfigSpecServicesPortsId, v.Encode()))
	}
	return
}

func (o *ConfigSpecServices) Empty() bool {
	if !entity.Empty(o.Command) {
		return false
	}
	if !o.Concurrency.Empty() {
		return false
	}
	if len(o.Disks) != 0 {
		return false
	}
	if len(o.Env) != 0 {
		return false
	}
	if !entity.Empty(o.Image) {
		return false
	}
	if !entity.Empty(o.Name) {
		return false
	}
	if !entity.Empty(o.Port) {
		return false
	}
	if !entity.Empty(o.PortName) {
		return false
	}
	if !entity.Empty(o.PortTimeout) {
		return false
	}
	if !entity.Empty(o.PortType) {
		return false
	}
	if len(o.Ports) != 0 {
		return false
	}
	return true
}

func (o *ConfigSpecServices) InitSchema(sb *schema.SchemaBuilder) {
	sb.String("command", "dev.miren.core/component.config_spec.services.command", schema.Doc("The command to run for the service"))
	sb.Component("concurrency", "dev.miren.core/component.config_spec.services.concurrency", schema.Doc("Concurrency configuration for this service"))
	(&ConfigSpecServicesConcurrency{}).InitSchema(sb.Builder("component.config_spec.services.concurrency"))
	sb.Component("disks", "dev.miren.core/component.config_spec.services.disks", schema.Doc("Disk attachments for this service"), schema.Many)
	(&ConfigSpecServicesDisks{}).InitSchema(sb.Builder("component.config_spec.services.disks"))
	sb.Component("env", "dev.miren.core/component.config_spec.services.env", schema.Doc("Environment variables for this service only"), schema.Many)
	(&ConfigSpecServicesEnv{}).InitSchema(sb.Builder("component.config_spec.services.env"))
	sb.String("image", "dev.miren.core/component.config_spec.services.image", schema.Doc("Optional container image for this service"))
	sb.String("name", "dev.miren.core/component.config_spec.services.name", schema.Doc("The service name (e.g. web, worker)"))
	sb.Int64("port", "dev.miren.core/component.config_spec.services.port", schema.Doc("The TCP port the service listens on"))
	sb.String("port_name", "dev.miren.core/component.config_spec.services.port_name", schema.Doc("The name of the port (e.g. http, grpc)"))
	sb.String("port_timeout", "dev.miren.core/component.config_spec.services.port_timeout", schema.Doc("Custom port-wait timeout (e.g. \"60s\", \"2m\"). Empty falls back to the 15s default; invalid duration strings are rejected at parse time."))
	sb.String("port_type", "dev.miren.core/component.config_spec.services.port_type", schema.Doc("The type of the port (e.g. http, tcp)"))
	sb.Component("ports", "dev.miren.core/component.config_spec.services.ports", schema.Doc("Network ports this service listens on. Overrides scalar port/port_name/port_type."), schema.Many)
	(&ConfigSpecServicesPorts{}).InitSchema(sb.Builder("component.config_spec.services.ports"))
}

const (
	ConfigSpecServicesConcurrencyModeId                = entity.Id("dev.miren.core/component.config_spec.services.concurrency.mode")
	ConfigSpecServicesConcurrencyNumInstancesId        = entity.Id("dev.miren.core/component.config_spec.services.concurrency.num_instances")
	ConfigSpecServicesConcurrencyRequestsPerInstanceId = entity.Id("dev.miren.core/component.config_spec.services.concurrency.requests_per_instance")
	ConfigSpecServicesConcurrencyScaleDownDelayId      = entity.Id("dev.miren.core/component.config_spec.services.concurrency.scale_down_delay")
	ConfigSpecServicesConcurrencyShutdownTimeoutId     = entity.Id("dev.miren.core/component.config_spec.services.concurrency.shutdown_timeout")
)

type ConfigSpecServicesConcurrency struct {
	Mode                string `cbor:"mode,omitempty" json:"mode,omitempty"`
	NumInstances        int64  `cbor:"num_instances,omitempty" json:"num_instances,omitempty"`
	RequestsPerInstance int64  `cbor:"requests_per_instance,omitempty" json:"requests_per_instance,omitempty"`
	ScaleDownDelay      string `cbor:"scale_down_delay,omitempty" json:"scale_down_delay,omitempty"`
	ShutdownTimeout     string `cbor:"shutdown_timeout,omitempty" json:"shutdown_timeout,omitempty"`
}

func (o *ConfigSpecServicesConcurrency) Decode(e entity.AttrGetter) {
	if a, ok := e.Get(ConfigSpecServicesConcurrencyModeId); ok && a.Value.Kind() == entity.KindString {
		o.Mode = a.Value.String()
	}
	if a, ok := e.Get(ConfigSpecServicesConcurrencyNumInstancesId); ok && a.Value.Kind() == entity.KindInt64 {
		o.NumInstances = a.Value.Int64()
	}
	if a, ok := e.Get(ConfigSpecServicesConcurrencyRequestsPerInstanceId); ok && a.Value.Kind() == entity.KindInt64 {
		o.RequestsPerInstance = a.Value.Int64()
	}
	if a, ok := e.Get(ConfigSpecServicesConcurrencyScaleDownDelayId); ok && a.Value.Kind() == entity.KindString {
		o.ScaleDownDelay = a.Value.String()
	}
	if a, ok := e.Get(ConfigSpecServicesConcurrencyShutdownTimeoutId); ok && a.Value.Kind() == entity.KindString {
		o.ShutdownTimeout = a.Value.String()
	}
}

func (o *ConfigSpecServicesConcurrency) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.Mode) {
		attrs = append(attrs, entity.String(ConfigSpecServicesConcurrencyModeId, o.Mode))
	}
	if !entity.Empty(o.NumInstances) {
		attrs = append(attrs, entity.Int64(ConfigSpecServicesConcurrencyNumInstancesId, o.NumInstances))
	}
	if !entity.Empty(o.RequestsPerInstance) {
		attrs = append(attrs, entity.Int64(ConfigSpecServicesConcurrencyRequestsPerInstanceId, o.RequestsPerInstance))
	}
	if !entity.Empty(o.ScaleDownDelay) {
		attrs = append(attrs, entity.String(ConfigSpecServicesConcurrencyScaleDownDelayId, o.ScaleDownDelay))
	}
	if !entity.Empty(o.ShutdownTimeout) {
		attrs = append(attrs, entity.String(ConfigSpecServicesConcurrencyShutdownTimeoutId, o.ShutdownTimeout))
	}
	return
}

func (o *ConfigSpecServicesConcurrency) Empty() bool {
	if !entity.Empty(o.Mode) {
		return false
	}
	if !entity.Empty(o.NumInstances) {
		return false
	}
	if !entity.Empty(o.RequestsPerInstance) {
		return false
	}
	if !entity.Empty(o.ScaleDownDelay) {
		return false
	}
	if !entity.Empty(o.ShutdownTimeout) {
		return false
	}
	return true
}

func (o *ConfigSpecServicesConcurrency) InitSchema(sb *schema.SchemaBuilder) {
	sb.String("mode", "dev.miren.core/component.config_spec.services.concurrency.mode", schema.Doc("The concurrency mode (auto or fixed)"))
	sb.Int64("num_instances", "dev.miren.core/component.config_spec.services.concurrency.num_instances", schema.Doc("For fixed mode, number of instances to maintain"))
	sb.Int64("requests_per_instance", "dev.miren.core/component.config_spec.services.concurrency.requests_per_instance", schema.Doc("For auto mode, number of concurrent requests per instance"))
	sb.String("scale_down_delay", "dev.miren.core/component.config_spec.services.concurrency.scale_down_delay", schema.Doc("For auto mode, delay before scaling down idle instances"))
	sb.String("shutdown_timeout", "dev.miren.core/component.config_spec.services.concurrency.shutdown_timeout", schema.Doc("Time to wait for graceful shutdown before force-killing"))
}

const (
	ConfigSpecServicesDisksFilesystemId    = entity.Id("dev.miren.core/component.config_spec.services.disks.filesystem")
	ConfigSpecServicesDisksLeaseTimeoutId  = entity.Id("dev.miren.core/component.config_spec.services.disks.lease_timeout")
	ConfigSpecServicesDisksMountPathId     = entity.Id("dev.miren.core/component.config_spec.services.disks.mount_path")
	ConfigSpecServicesDisksNameId          = entity.Id("dev.miren.core/component.config_spec.services.disks.name")
	ConfigSpecServicesDisksProviderId      = entity.Id("dev.miren.core/component.config_spec.services.disks.provider")
	ConfigSpecServicesDisksProviderMirenId = entity.Id("dev.miren.core/component.config_spec.services.disks.provider.miren")
	ConfigSpecServicesDisksProviderLocalId = entity.Id("dev.miren.core/component.config_spec.services.disks.provider.local")
	ConfigSpecServicesDisksReadOnlyId      = entity.Id("dev.miren.core/component.config_spec.services.disks.read_only")
	ConfigSpecServicesDisksSizeGbId        = entity.Id("dev.miren.core/component.config_spec.services.disks.size_gb")
)

type ConfigSpecServicesDisks struct {
	Filesystem   string                          `cbor:"filesystem,omitempty" json:"filesystem,omitempty"`
	LeaseTimeout string                          `cbor:"lease_timeout,omitempty" json:"lease_timeout,omitempty"`
	MountPath    string                          `cbor:"mount_path,omitempty" json:"mount_path,omitempty"`
	Name         string                          `cbor:"name,omitempty" json:"name,omitempty"`
	Provider     ConfigSpecServicesDisksProvider `cbor:"provider,omitempty" json:"provider,omitempty"`
	ReadOnly     bool                            `cbor:"read_only,omitempty" json:"read_only,omitempty"`
	SizeGb       int64                           `cbor:"size_gb,omitempty" json:"size_gb,omitempty"`
}

type ConfigSpecServicesDisksProvider string

const (
	ConfigSpecServicesDisksMIREN ConfigSpecServicesDisksProvider = "component.config_spec.services.disks.provider.miren"
	ConfigSpecServicesDisksLOCAL ConfigSpecServicesDisksProvider = "component.config_spec.services.disks.provider.local"
)

var ConfigSpecServicesDisksproviderFromId = map[entity.Id]ConfigSpecServicesDisksProvider{ConfigSpecServicesDisksProviderMirenId: ConfigSpecServicesDisksMIREN, ConfigSpecServicesDisksProviderLocalId: ConfigSpecServicesDisksLOCAL}
var ConfigSpecServicesDisksproviderToId = map[ConfigSpecServicesDisksProvider]entity.Id{ConfigSpecServicesDisksMIREN: ConfigSpecServicesDisksProviderMirenId, ConfigSpecServicesDisksLOCAL: ConfigSpecServicesDisksProviderLocalId}

func (o *ConfigSpecServicesDisks) Decode(e entity.AttrGetter) {
	if a, ok := e.Get(ConfigSpecServicesDisksFilesystemId); ok && a.Value.Kind() == entity.KindString {
		o.Filesystem = a.Value.String()
	}
	if a, ok := e.Get(ConfigSpecServicesDisksLeaseTimeoutId); ok && a.Value.Kind() == entity.KindString {
		o.LeaseTimeout = a.Value.String()
	}
	if a, ok := e.Get(ConfigSpecServicesDisksMountPathId); ok && a.Value.Kind() == entity.KindString {
		o.MountPath = a.Value.String()
	}
	if a, ok := e.Get(ConfigSpecServicesDisksNameId); ok && a.Value.Kind() == entity.KindString {
		o.Name = a.Value.String()
	}
	if a, ok := e.Get(ConfigSpecServicesDisksProviderId); ok && a.Value.Kind() == entity.KindId {
		o.Provider = ConfigSpecServicesDisksproviderFromId[a.Value.Id()]
	}
	if a, ok := e.Get(ConfigSpecServicesDisksReadOnlyId); ok && a.Value.Kind() == entity.KindBool {
		o.ReadOnly = a.Value.Bool()
	}
	if a, ok := e.Get(ConfigSpecServicesDisksSizeGbId); ok && a.Value.Kind() == entity.KindInt64 {
		o.SizeGb = a.Value.Int64()
	}
}

func (o *ConfigSpecServicesDisks) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.Filesystem) {
		attrs = append(attrs, entity.String(ConfigSpecServicesDisksFilesystemId, o.Filesystem))
	}
	if !entity.Empty(o.LeaseTimeout) {
		attrs = append(attrs, entity.String(ConfigSpecServicesDisksLeaseTimeoutId, o.LeaseTimeout))
	}
	if !entity.Empty(o.MountPath) {
		attrs = append(attrs, entity.String(ConfigSpecServicesDisksMountPathId, o.MountPath))
	}
	if !entity.Empty(o.Name) {
		attrs = append(attrs, entity.String(ConfigSpecServicesDisksNameId, o.Name))
	}
	if a, ok := ConfigSpecServicesDisksproviderToId[o.Provider]; ok {
		attrs = append(attrs, entity.Ref(ConfigSpecServicesDisksProviderId, a))
	}
	attrs = append(attrs, entity.Bool(ConfigSpecServicesDisksReadOnlyId, o.ReadOnly))
	if !entity.Empty(o.SizeGb) {
		attrs = append(attrs, entity.Int64(ConfigSpecServicesDisksSizeGbId, o.SizeGb))
	}
	return
}

func (o *ConfigSpecServicesDisks) Empty() bool {
	if !entity.Empty(o.Filesystem) {
		return false
	}
	if !entity.Empty(o.LeaseTimeout) {
		return false
	}
	if !entity.Empty(o.MountPath) {
		return false
	}
	if !entity.Empty(o.Name) {
		return false
	}
	if o.Provider != "" {
		return false
	}
	if !entity.Empty(o.ReadOnly) {
		return false
	}
	if !entity.Empty(o.SizeGb) {
		return false
	}
	return true
}

func (o *ConfigSpecServicesDisks) InitSchema(sb *schema.SchemaBuilder) {
	sb.String("filesystem", "dev.miren.core/component.config_spec.services.disks.filesystem", schema.Doc("Filesystem type (ext4, xfs, btrfs) for auto-creating the disk"))
	sb.String("lease_timeout", "dev.miren.core/component.config_spec.services.disks.lease_timeout", schema.Doc("Timeout for acquiring the disk lease"))
	sb.String("mount_path", "dev.miren.core/component.config_spec.services.disks.mount_path", schema.Doc("The path inside the container where the disk will be mounted"))
	sb.String("name", "dev.miren.core/component.config_spec.services.disks.name", schema.Doc("The name of the disk"))
	sb.Singleton("dev.miren.core/component.config_spec.services.disks.provider.miren")
	sb.Singleton("dev.miren.core/component.config_spec.services.disks.provider.local")
	sb.Ref("provider", "dev.miren.core/component.config_spec.services.disks.provider", schema.Doc("Disk provider: 'miren' (default) for network disks, 'local' for node-local persistent storage"), schema.Choices(ConfigSpecServicesDisksProviderMirenId, ConfigSpecServicesDisksProviderLocalId))
	sb.Bool("read_only", "dev.miren.core/component.config_spec.services.disks.read_only", schema.Doc("Whether to mount the disk as read-only"))
	sb.Int64("size_gb", "dev.miren.core/component.config_spec.services.disks.size_gb", schema.Doc("Size in GB for auto-creating the disk if it doesn't exist"))
}

const (
	ConfigSpecServicesEnvDescriptionId = entity.Id("dev.miren.core/component.config_spec.services.env.description")
	ConfigSpecServicesEnvKeyId         = entity.Id("dev.miren.core/component.config_spec.services.env.key")
	ConfigSpecServicesEnvOriginId      = entity.Id("dev.miren.core/component.config_spec.services.env.origin")
	ConfigSpecServicesEnvRequiredId    = entity.Id("dev.miren.core/component.config_spec.services.env.required")
	ConfigSpecServicesEnvSensitiveId   = entity.Id("dev.miren.core/component.config_spec.services.env.sensitive")
	ConfigSpecServicesEnvSourceId      = entity.Id("dev.miren.core/component.config_spec.services.env.source")
	ConfigSpecServicesEnvValueId       = entity.Id("dev.miren.core/component.config_spec.services.env.value")
)

type ConfigSpecServicesEnv struct {
	Description string `cbor:"description,omitempty" json:"description,omitempty"`
	Key         string `cbor:"key,omitempty" json:"key,omitempty"`
	Origin      string `cbor:"origin,omitempty" json:"origin,omitempty"`
	Required    bool   `cbor:"required,omitempty" json:"required,omitempty"`
	Sensitive   bool   `cbor:"sensitive,omitempty" json:"sensitive,omitempty"`
	Source      string `cbor:"source,omitempty" json:"source,omitempty"`
	Value       string `cbor:"value,omitempty" json:"value,omitempty"`
}

func (o *ConfigSpecServicesEnv) Decode(e entity.AttrGetter) {
	if a, ok := e.Get(ConfigSpecServicesEnvDescriptionId); ok && a.Value.Kind() == entity.KindString {
		o.Description = a.Value.String()
	}
	if a, ok := e.Get(ConfigSpecServicesEnvKeyId); ok && a.Value.Kind() == entity.KindString {
		o.Key = a.Value.String()
	}
	if a, ok := e.Get(ConfigSpecServicesEnvOriginId); ok && a.Value.Kind() == entity.KindString {
		o.Origin = a.Value.String()
	}
	if a, ok := e.Get(ConfigSpecServicesEnvRequiredId); ok && a.Value.Kind() == entity.KindBool {
		o.Required = a.Value.Bool()
	}
	if a, ok := e.Get(ConfigSpecServicesEnvSensitiveId); ok && a.Value.Kind() == entity.KindBool {
		o.Sensitive = a.Value.Bool()
	}
	if a, ok := e.Get(ConfigSpecServicesEnvSourceId); ok && a.Value.Kind() == entity.KindString {
		o.Source = a.Value.String()
	}
	if a, ok := e.Get(ConfigSpecServicesEnvValueId); ok && a.Value.Kind() == entity.KindString {
		o.Value = a.Value.String()
	}
}

func (o *ConfigSpecServicesEnv) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.Description) {
		attrs = append(attrs, entity.String(ConfigSpecServicesEnvDescriptionId, o.Description))
	}
	if !entity.Empty(o.Key) {
		attrs = append(attrs, entity.String(ConfigSpecServicesEnvKeyId, o.Key))
	}
	if !entity.Empty(o.Origin) {
		attrs = append(attrs, entity.String(ConfigSpecServicesEnvOriginId, o.Origin))
	}
	attrs = append(attrs, entity.Bool(ConfigSpecServicesEnvRequiredId, o.Required))
	attrs = append(attrs, entity.Bool(ConfigSpecServicesEnvSensitiveId, o.Sensitive))
	if !entity.Empty(o.Source) {
		attrs = append(attrs, entity.String(ConfigSpecServicesEnvSourceId, o.Source))
	}
	if !entity.Empty(o.Value) {
		attrs = append(attrs, entity.String(ConfigSpecServicesEnvValueId, o.Value))
	}
	return
}

func (o *ConfigSpecServicesEnv) Empty() bool {
	if !entity.Empty(o.Description) {
		return false
	}
	if !entity.Empty(o.Key) {
		return false
	}
	if !entity.Empty(o.Origin) {
		return false
	}
	if !entity.Empty(o.Required) {
		return false
	}
	if !entity.Empty(o.Sensitive) {
		return false
	}
	if !entity.Empty(o.Source) {
		return false
	}
	if !entity.Empty(o.Value) {
		return false
	}
	return true
}

func (o *ConfigSpecServicesEnv) InitSchema(sb *schema.SchemaBuilder) {
	sb.String("description", "dev.miren.core/component.config_spec.services.env.description", schema.Doc("Human-readable description of this variable's purpose"))
	sb.String("key", "dev.miren.core/component.config_spec.services.env.key", schema.Doc("The name of the variable"))
	sb.String("origin", "dev.miren.core/component.config_spec.services.env.origin", schema.Doc("The provenance of the variable (user, file, generated, detected)"))
	sb.Bool("required", "dev.miren.core/component.config_spec.services.env.required", schema.Doc("Whether this variable must have a non-empty value for deploy to succeed"))
	sb.Bool("sensitive", "dev.miren.core/component.config_spec.services.env.sensitive", schema.Doc("Whether or not the value is sensitive"))
	sb.String("source", "dev.miren.core/component.config_spec.services.env.source", schema.Doc("The source of the variable (config or manual). Defaults to config for backward compatibility."))
	sb.String("value", "dev.miren.core/component.config_spec.services.env.value", schema.Doc("The value of the variable"))
}

const (
	ConfigSpecServicesPortsNameId        = entity.Id("dev.miren.core/component.config_spec.services.ports.name")
	ConfigSpecServicesPortsNodePortId    = entity.Id("dev.miren.core/component.config_spec.services.ports.node_port")
	ConfigSpecServicesPortsPortId        = entity.Id("dev.miren.core/component.config_spec.services.ports.port")
	ConfigSpecServicesPortsProtocolId    = entity.Id("dev.miren.core/component.config_spec.services.ports.protocol")
	ConfigSpecServicesPortsProtocolTcpId = entity.Id("dev.miren.core/component.config_spec.services.ports.protocol.tcp")
	ConfigSpecServicesPortsProtocolUdpId = entity.Id("dev.miren.core/component.config_spec.services.ports.protocol.udp")
	ConfigSpecServicesPortsTypeId        = entity.Id("dev.miren.core/component.config_spec.services.ports.type")
)

type ConfigSpecServicesPorts struct {
	Name     string                          `cbor:"name" json:"name"`
	NodePort int64                           `cbor:"node_port,omitempty" json:"node_port,omitempty"`
	Port     int64                           `cbor:"port" json:"port"`
	Protocol ConfigSpecServicesPortsProtocol `cbor:"protocol,omitempty" json:"protocol,omitempty"`
	Type     string                          `cbor:"type,omitempty" json:"type,omitempty"`
}

type ConfigSpecServicesPortsProtocol string

const (
	ConfigSpecServicesPortsTCP ConfigSpecServicesPortsProtocol = "component.config_spec.services.ports.protocol.tcp"
	ConfigSpecServicesPortsUDP ConfigSpecServicesPortsProtocol = "component.config_spec.services.ports.protocol.udp"
)

var ConfigSpecServicesPortsprotocolFromId = map[entity.Id]ConfigSpecServicesPortsProtocol{ConfigSpecServicesPortsProtocolTcpId: ConfigSpecServicesPortsTCP, ConfigSpecServicesPortsProtocolUdpId: ConfigSpecServicesPortsUDP}
var ConfigSpecServicesPortsprotocolToId = map[ConfigSpecServicesPortsProtocol]entity.Id{ConfigSpecServicesPortsTCP: ConfigSpecServicesPortsProtocolTcpId, ConfigSpecServicesPortsUDP: ConfigSpecServicesPortsProtocolUdpId}

func (o *ConfigSpecServicesPorts) Decode(e entity.AttrGetter) {
	if a, ok := e.Get(ConfigSpecServicesPortsNameId); ok && a.Value.Kind() == entity.KindString {
		o.Name = a.Value.String()
	}
	if a, ok := e.Get(ConfigSpecServicesPortsNodePortId); ok && a.Value.Kind() == entity.KindInt64 {
		o.NodePort = a.Value.Int64()
	}
	if a, ok := e.Get(ConfigSpecServicesPortsPortId); ok && a.Value.Kind() == entity.KindInt64 {
		o.Port = a.Value.Int64()
	}
	if a, ok := e.Get(ConfigSpecServicesPortsProtocolId); ok && a.Value.Kind() == entity.KindId {
		o.Protocol = ConfigSpecServicesPortsprotocolFromId[a.Value.Id()]
	}
	if a, ok := e.Get(ConfigSpecServicesPortsTypeId); ok && a.Value.Kind() == entity.KindString {
		o.Type = a.Value.String()
	}
}

func (o *ConfigSpecServicesPorts) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.Name) {
		attrs = append(attrs, entity.String(ConfigSpecServicesPortsNameId, o.Name))
	}
	if !entity.Empty(o.NodePort) {
		attrs = append(attrs, entity.Int64(ConfigSpecServicesPortsNodePortId, o.NodePort))
	}
	attrs = append(attrs, entity.Int64(ConfigSpecServicesPortsPortId, o.Port))
	if a, ok := ConfigSpecServicesPortsprotocolToId[o.Protocol]; ok {
		attrs = append(attrs, entity.Ref(ConfigSpecServicesPortsProtocolId, a))
	}
	if !entity.Empty(o.Type) {
		attrs = append(attrs, entity.String(ConfigSpecServicesPortsTypeId, o.Type))
	}
	return
}

func (o *ConfigSpecServicesPorts) Empty() bool {
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
	if !entity.Empty(o.Type) {
		return false
	}
	return true
}

func (o *ConfigSpecServicesPorts) InitSchema(sb *schema.SchemaBuilder) {
	sb.String("name", "dev.miren.core/component.config_spec.services.ports.name", schema.Required)
	sb.Int64("node_port", "dev.miren.core/component.config_spec.services.ports.node_port")
	sb.Int64("port", "dev.miren.core/component.config_spec.services.ports.port", schema.Required)
	sb.Singleton("dev.miren.core/component.config_spec.services.ports.protocol.tcp")
	sb.Singleton("dev.miren.core/component.config_spec.services.ports.protocol.udp")
	sb.Ref("protocol", "dev.miren.core/component.config_spec.services.ports.protocol", schema.Choices(ConfigSpecServicesPortsProtocolTcpId, ConfigSpecServicesPortsProtocolUdpId))
	sb.String("type", "dev.miren.core/component.config_spec.services.ports.type")
}

const (
	ConfigSpecVariablesDescriptionId = entity.Id("dev.miren.core/component.config_spec.variables.description")
	ConfigSpecVariablesKeyId         = entity.Id("dev.miren.core/component.config_spec.variables.key")
	ConfigSpecVariablesOriginId      = entity.Id("dev.miren.core/component.config_spec.variables.origin")
	ConfigSpecVariablesRequiredId    = entity.Id("dev.miren.core/component.config_spec.variables.required")
	ConfigSpecVariablesSensitiveId   = entity.Id("dev.miren.core/component.config_spec.variables.sensitive")
	ConfigSpecVariablesSourceId      = entity.Id("dev.miren.core/component.config_spec.variables.source")
	ConfigSpecVariablesValueId       = entity.Id("dev.miren.core/component.config_spec.variables.value")
)

type ConfigSpecVariables struct {
	Description string `cbor:"description,omitempty" json:"description,omitempty"`
	Key         string `cbor:"key,omitempty" json:"key,omitempty"`
	Origin      string `cbor:"origin,omitempty" json:"origin,omitempty"`
	Required    bool   `cbor:"required,omitempty" json:"required,omitempty"`
	Sensitive   bool   `cbor:"sensitive,omitempty" json:"sensitive,omitempty"`
	Source      string `cbor:"source,omitempty" json:"source,omitempty"`
	Value       string `cbor:"value,omitempty" json:"value,omitempty"`
}

func (o *ConfigSpecVariables) Decode(e entity.AttrGetter) {
	if a, ok := e.Get(ConfigSpecVariablesDescriptionId); ok && a.Value.Kind() == entity.KindString {
		o.Description = a.Value.String()
	}
	if a, ok := e.Get(ConfigSpecVariablesKeyId); ok && a.Value.Kind() == entity.KindString {
		o.Key = a.Value.String()
	}
	if a, ok := e.Get(ConfigSpecVariablesOriginId); ok && a.Value.Kind() == entity.KindString {
		o.Origin = a.Value.String()
	}
	if a, ok := e.Get(ConfigSpecVariablesRequiredId); ok && a.Value.Kind() == entity.KindBool {
		o.Required = a.Value.Bool()
	}
	if a, ok := e.Get(ConfigSpecVariablesSensitiveId); ok && a.Value.Kind() == entity.KindBool {
		o.Sensitive = a.Value.Bool()
	}
	if a, ok := e.Get(ConfigSpecVariablesSourceId); ok && a.Value.Kind() == entity.KindString {
		o.Source = a.Value.String()
	}
	if a, ok := e.Get(ConfigSpecVariablesValueId); ok && a.Value.Kind() == entity.KindString {
		o.Value = a.Value.String()
	}
}

func (o *ConfigSpecVariables) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.Description) {
		attrs = append(attrs, entity.String(ConfigSpecVariablesDescriptionId, o.Description))
	}
	if !entity.Empty(o.Key) {
		attrs = append(attrs, entity.String(ConfigSpecVariablesKeyId, o.Key))
	}
	if !entity.Empty(o.Origin) {
		attrs = append(attrs, entity.String(ConfigSpecVariablesOriginId, o.Origin))
	}
	attrs = append(attrs, entity.Bool(ConfigSpecVariablesRequiredId, o.Required))
	attrs = append(attrs, entity.Bool(ConfigSpecVariablesSensitiveId, o.Sensitive))
	if !entity.Empty(o.Source) {
		attrs = append(attrs, entity.String(ConfigSpecVariablesSourceId, o.Source))
	}
	if !entity.Empty(o.Value) {
		attrs = append(attrs, entity.String(ConfigSpecVariablesValueId, o.Value))
	}
	return
}

func (o *ConfigSpecVariables) Empty() bool {
	if !entity.Empty(o.Description) {
		return false
	}
	if !entity.Empty(o.Key) {
		return false
	}
	if !entity.Empty(o.Origin) {
		return false
	}
	if !entity.Empty(o.Required) {
		return false
	}
	if !entity.Empty(o.Sensitive) {
		return false
	}
	if !entity.Empty(o.Source) {
		return false
	}
	if !entity.Empty(o.Value) {
		return false
	}
	return true
}

func (o *ConfigSpecVariables) InitSchema(sb *schema.SchemaBuilder) {
	sb.String("description", "dev.miren.core/component.config_spec.variables.description", schema.Doc("Human-readable description of this variable's purpose"))
	sb.String("key", "dev.miren.core/component.config_spec.variables.key", schema.Doc("The name of the variable"))
	sb.String("origin", "dev.miren.core/component.config_spec.variables.origin", schema.Doc("The provenance of the variable (user, file, generated, detected)."))
	sb.Bool("required", "dev.miren.core/component.config_spec.variables.required", schema.Doc("Whether this variable must have a non-empty value for deploy to succeed"))
	sb.Bool("sensitive", "dev.miren.core/component.config_spec.variables.sensitive", schema.Doc("Whether or not the value is sensitive"))
	sb.String("source", "dev.miren.core/component.config_spec.variables.source", schema.Doc("The source of the variable (config or manual). Defaults to config for backward compatibility."))
	sb.String("value", "dev.miren.core/component.config_spec.variables.value", schema.Doc("The value of the variable"))
}

const (
	AppActiveVersionId = entity.Id("dev.miren.core/app.active_version")
	AppInitialConfigId = entity.Id("dev.miren.core/app.initial_config")
	AppProjectId       = entity.Id("dev.miren.core/app.project")
)

type App struct {
	ID            entity.Id `json:"id"`
	ActiveVersion entity.Id `cbor:"active_version,omitempty" json:"active_version,omitempty"`
	InitialConfig entity.Id `cbor:"initial_config,omitempty" json:"initial_config,omitempty"`
	Project       entity.Id `cbor:"project,omitempty" json:"project,omitempty"`
}

func (o *App) Decode(e entity.AttrGetter) {
	o.ID = entity.MustGet(e, entity.DBId).Value.Id()
	if a, ok := e.Get(AppActiveVersionId); ok && a.Value.Kind() == entity.KindId {
		o.ActiveVersion = a.Value.Id()
	}
	if a, ok := e.Get(AppInitialConfigId); ok && a.Value.Kind() == entity.KindId {
		o.InitialConfig = a.Value.Id()
	}
	if a, ok := e.Get(AppProjectId); ok && a.Value.Kind() == entity.KindId {
		o.Project = a.Value.Id()
	}
}

func (o *App) Is(e entity.AttrGetter) bool {
	return entity.Is(e, KindApp)
}

func (o *App) ShortKind() string {
	return "app"
}

func (o *App) Kind() entity.Id {
	return KindApp
}

func (o *App) EntityId() entity.Id {
	return o.ID
}

func (o *App) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.ActiveVersion) {
		attrs = append(attrs, entity.Ref(AppActiveVersionId, o.ActiveVersion))
	}
	if !entity.Empty(o.InitialConfig) {
		attrs = append(attrs, entity.Ref(AppInitialConfigId, o.InitialConfig))
	}
	if !entity.Empty(o.Project) {
		attrs = append(attrs, entity.Ref(AppProjectId, o.Project))
	}
	attrs = append(attrs, entity.Ref(entity.EntityKind, KindApp))
	return
}

func (o *App) Empty() bool {
	if !entity.Empty(o.ActiveVersion) {
		return false
	}
	if !entity.Empty(o.InitialConfig) {
		return false
	}
	if !entity.Empty(o.Project) {
		return false
	}
	return true
}

func (o *App) InitSchema(sb *schema.SchemaBuilder) {
	sb.Ref("active_version", "dev.miren.core/app.active_version", schema.Doc("The version of the project that should be used"))
	sb.Ref("initial_config", "dev.miren.core/app.initial_config", schema.Doc("Reference to the initial ConfigVersion entity created before the first deploy"))
	sb.Ref("project", "dev.miren.core/app.project", schema.Doc("The project that the app belongs to"))
}

const (
	AppVersionAdminTokenId         = entity.Id("dev.miren.core/app_version.admin_token")
	AppVersionAppId                = entity.Id("dev.miren.core/app_version.app")
	AppVersionArtifactId           = entity.Id("dev.miren.core/app_version.artifact")
	AppVersionConfigId             = entity.Id("dev.miren.core/app_version.config")
	AppVersionConfigVersionId      = entity.Id("dev.miren.core/app_version.config_version")
	AppVersionEphemeralExpiresAtId = entity.Id("dev.miren.core/app_version.ephemeral_expires_at")
	AppVersionEphemeralLabelId     = entity.Id("dev.miren.core/app_version.ephemeral_label")
	AppVersionEphemeralTtlId       = entity.Id("dev.miren.core/app_version.ephemeral_ttl")
	AppVersionImageUrlId           = entity.Id("dev.miren.core/app_version.image_url")
	AppVersionManifestId           = entity.Id("dev.miren.core/app_version.manifest")
	AppVersionManifestDigestId     = entity.Id("dev.miren.core/app_version.manifest_digest")
	AppVersionVersionId            = entity.Id("dev.miren.core/app_version.version")
)

type AppVersion struct {
	ID                 entity.Id `json:"id"`
	AdminToken         string    `cbor:"admin_token,omitempty" json:"admin_token,omitempty"`
	App                entity.Id `cbor:"app,omitempty" json:"app,omitempty"`
	Artifact           entity.Id `cbor:"artifact,omitempty" json:"artifact,omitempty"`
	Config             Config    `cbor:"config,omitempty" json:"config,omitempty"`
	ConfigVersion      entity.Id `cbor:"config_version,omitempty" json:"config_version,omitempty"`
	EphemeralExpiresAt time.Time `cbor:"ephemeral_expires_at,omitempty" json:"ephemeral_expires_at,omitempty"`
	EphemeralLabel     string    `cbor:"ephemeral_label,omitempty" json:"ephemeral_label,omitempty"`
	EphemeralTtl       string    `cbor:"ephemeral_ttl,omitempty" json:"ephemeral_ttl,omitempty"`
	ImageUrl           string    `cbor:"image_url,omitempty" json:"image_url,omitempty"`
	Manifest           string    `cbor:"manifest,omitempty" json:"manifest,omitempty"`
	ManifestDigest     string    `cbor:"manifest_digest,omitempty" json:"manifest_digest,omitempty"`
	Version            string    `cbor:"version,omitempty" json:"version,omitempty"`
}

func (o *AppVersion) Decode(e entity.AttrGetter) {
	o.ID = entity.MustGet(e, entity.DBId).Value.Id()
	if a, ok := e.Get(AppVersionAdminTokenId); ok && a.Value.Kind() == entity.KindString {
		o.AdminToken = a.Value.String()
	}
	if a, ok := e.Get(AppVersionAppId); ok && a.Value.Kind() == entity.KindId {
		o.App = a.Value.Id()
	}
	if a, ok := e.Get(AppVersionArtifactId); ok && a.Value.Kind() == entity.KindId {
		o.Artifact = a.Value.Id()
	}
	if a, ok := e.Get(AppVersionConfigId); ok && a.Value.Kind() == entity.KindComponent {
		o.Config.Decode(a.Value.Component())
	}
	if a, ok := e.Get(AppVersionConfigVersionId); ok && a.Value.Kind() == entity.KindId {
		o.ConfigVersion = a.Value.Id()
	}
	if a, ok := e.Get(AppVersionEphemeralExpiresAtId); ok && a.Value.Kind() == entity.KindTime {
		o.EphemeralExpiresAt = a.Value.Time()
	}
	if a, ok := e.Get(AppVersionEphemeralLabelId); ok && a.Value.Kind() == entity.KindString {
		o.EphemeralLabel = a.Value.String()
	}
	if a, ok := e.Get(AppVersionEphemeralTtlId); ok && a.Value.Kind() == entity.KindString {
		o.EphemeralTtl = a.Value.String()
	}
	if a, ok := e.Get(AppVersionImageUrlId); ok && a.Value.Kind() == entity.KindString {
		o.ImageUrl = a.Value.String()
	}
	if a, ok := e.Get(AppVersionManifestId); ok && a.Value.Kind() == entity.KindString {
		o.Manifest = a.Value.String()
	}
	if a, ok := e.Get(AppVersionManifestDigestId); ok && a.Value.Kind() == entity.KindString {
		o.ManifestDigest = a.Value.String()
	}
	if a, ok := e.Get(AppVersionVersionId); ok && a.Value.Kind() == entity.KindString {
		o.Version = a.Value.String()
	}
}

func (o *AppVersion) Is(e entity.AttrGetter) bool {
	return entity.Is(e, KindAppVersion)
}

func (o *AppVersion) ShortKind() string {
	return "app_version"
}

func (o *AppVersion) Kind() entity.Id {
	return KindAppVersion
}

func (o *AppVersion) EntityId() entity.Id {
	return o.ID
}

func (o *AppVersion) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.AdminToken) {
		attrs = append(attrs, entity.String(AppVersionAdminTokenId, o.AdminToken))
	}
	if !entity.Empty(o.App) {
		attrs = append(attrs, entity.Ref(AppVersionAppId, o.App))
	}
	if !entity.Empty(o.Artifact) {
		attrs = append(attrs, entity.Ref(AppVersionArtifactId, o.Artifact))
	}
	if !o.Config.Empty() {
		attrs = append(attrs, entity.Component(AppVersionConfigId, o.Config.Encode()))
	}
	if !entity.Empty(o.ConfigVersion) {
		attrs = append(attrs, entity.Ref(AppVersionConfigVersionId, o.ConfigVersion))
	}
	if !entity.Empty(o.EphemeralExpiresAt) {
		attrs = append(attrs, entity.Time(AppVersionEphemeralExpiresAtId, o.EphemeralExpiresAt))
	}
	if !entity.Empty(o.EphemeralLabel) {
		attrs = append(attrs, entity.String(AppVersionEphemeralLabelId, o.EphemeralLabel))
	}
	if !entity.Empty(o.EphemeralTtl) {
		attrs = append(attrs, entity.String(AppVersionEphemeralTtlId, o.EphemeralTtl))
	}
	if !entity.Empty(o.ImageUrl) {
		attrs = append(attrs, entity.String(AppVersionImageUrlId, o.ImageUrl))
	}
	if !entity.Empty(o.Manifest) {
		attrs = append(attrs, entity.String(AppVersionManifestId, o.Manifest))
	}
	if !entity.Empty(o.ManifestDigest) {
		attrs = append(attrs, entity.String(AppVersionManifestDigestId, o.ManifestDigest))
	}
	if !entity.Empty(o.Version) {
		attrs = append(attrs, entity.String(AppVersionVersionId, o.Version))
	}
	attrs = append(attrs, entity.Ref(entity.EntityKind, KindAppVersion))
	return
}

func (o *AppVersion) Empty() bool {
	if !entity.Empty(o.AdminToken) {
		return false
	}
	if !entity.Empty(o.App) {
		return false
	}
	if !entity.Empty(o.Artifact) {
		return false
	}
	if !o.Config.Empty() {
		return false
	}
	if !entity.Empty(o.ConfigVersion) {
		return false
	}
	if !entity.Empty(o.EphemeralExpiresAt) {
		return false
	}
	if !entity.Empty(o.EphemeralLabel) {
		return false
	}
	if !entity.Empty(o.EphemeralTtl) {
		return false
	}
	if !entity.Empty(o.ImageUrl) {
		return false
	}
	if !entity.Empty(o.Manifest) {
		return false
	}
	if !entity.Empty(o.ManifestDigest) {
		return false
	}
	if !entity.Empty(o.Version) {
		return false
	}
	return true
}

func (o *AppVersion) InitSchema(sb *schema.SchemaBuilder) {
	sb.String("admin_token", "dev.miren.core/app_version.admin_token", schema.Doc("Cryptographically random token for authenticating admin API calls. Generated per-version and exposed to the app via ADMIN_TOKEN env var."))
	sb.Ref("app", "dev.miren.core/app_version.app", schema.Doc("The application the version is for"), schema.Indexed, schema.Tags("dev.miren.app_ref"))
	sb.Ref("artifact", "dev.miren.core/app_version.artifact", schema.Doc("The artifact to deploy for the version"))
	sb.Component("config", "dev.miren.core/app_version.config", schema.Doc("The configuration of the version"))
	(&Config{}).InitSchema(sb.Builder("app_version.config"))
	sb.Ref("config_version", "dev.miren.core/app_version.config_version", schema.Doc("Reference to the ConfigVersion entity containing the resolved configuration for this version"))
	sb.Time("ephemeral_expires_at", "dev.miren.core/app_version.ephemeral_expires_at", schema.Doc("Computed expiration timestamp (creation + TTL). Used by the cleanup controller."), schema.Indexed)
	sb.String("ephemeral_label", "dev.miren.core/app_version.ephemeral_label", schema.Doc("DNS-safe label for ephemeral subdomain routing (e.g., \"feat-x\"). Empty for non-ephemeral versions."), schema.Indexed)
	sb.String("ephemeral_ttl", "dev.miren.core/app_version.ephemeral_ttl", schema.Doc("TTL duration string (e.g., \"48h\") for display. Empty for non-ephemeral versions."))
	sb.String("image_url", "dev.miren.core/app_version.image_url", schema.Doc("The OCI url for the versions code"))
	sb.String("manifest", "dev.miren.core/app_version.manifest", schema.Doc("The OCI image manifest for the version"))
	sb.String("manifest_digest", "dev.miren.core/app_version.manifest_digest", schema.Doc("The digest of the manifest"), schema.Indexed)
	sb.String("version", "dev.miren.core/app_version.version", schema.Doc("The version of this app"))
}

const (
	ConfigCommandsId       = entity.Id("dev.miren.core/config.commands")
	ConfigEntrypointId     = entity.Id("dev.miren.core/config.entrypoint")
	ConfigPortId           = entity.Id("dev.miren.core/config.port")
	ConfigServicesId       = entity.Id("dev.miren.core/config.services")
	ConfigStartDirectoryId = entity.Id("dev.miren.core/config.start_directory")
	ConfigVariableId       = entity.Id("dev.miren.core/config.variable")
)

type Config struct {
	Commands       []Commands `cbor:"commands,omitempty" json:"commands,omitempty"`
	Entrypoint     string     `cbor:"entrypoint,omitempty" json:"entrypoint,omitempty"`
	Port           int64      `cbor:"port,omitempty" json:"port,omitempty"`
	Services       []Services `cbor:"services,omitempty" json:"services,omitempty"`
	StartDirectory string     `cbor:"start_directory,omitempty" json:"start_directory,omitempty"`
	Variable       []Variable `cbor:"variable,omitempty" json:"variable,omitempty"`
}

func (o *Config) Decode(e entity.AttrGetter) {
	for _, a := range e.GetAll(ConfigCommandsId) {
		if a.Value.Kind() == entity.KindComponent {
			var v Commands
			v.Decode(a.Value.Component())
			o.Commands = append(o.Commands, v)
		}
	}
	if a, ok := e.Get(ConfigEntrypointId); ok && a.Value.Kind() == entity.KindString {
		o.Entrypoint = a.Value.String()
	}
	if a, ok := e.Get(ConfigPortId); ok && a.Value.Kind() == entity.KindInt64 {
		o.Port = a.Value.Int64()
	}
	for _, a := range e.GetAll(ConfigServicesId) {
		if a.Value.Kind() == entity.KindComponent {
			var v Services
			v.Decode(a.Value.Component())
			o.Services = append(o.Services, v)
		}
	}
	if a, ok := e.Get(ConfigStartDirectoryId); ok && a.Value.Kind() == entity.KindString {
		o.StartDirectory = a.Value.String()
	}
	for _, a := range e.GetAll(ConfigVariableId) {
		if a.Value.Kind() == entity.KindComponent {
			var v Variable
			v.Decode(a.Value.Component())
			o.Variable = append(o.Variable, v)
		}
	}
}

func (o *Config) Encode() (attrs []entity.Attr) {
	for _, v := range o.Commands {
		attrs = append(attrs, entity.Component(ConfigCommandsId, v.Encode()))
	}
	if !entity.Empty(o.Entrypoint) {
		attrs = append(attrs, entity.String(ConfigEntrypointId, o.Entrypoint))
	}
	if !entity.Empty(o.Port) {
		attrs = append(attrs, entity.Int64(ConfigPortId, o.Port))
	}
	for _, v := range o.Services {
		attrs = append(attrs, entity.Component(ConfigServicesId, v.Encode()))
	}
	if !entity.Empty(o.StartDirectory) {
		attrs = append(attrs, entity.String(ConfigStartDirectoryId, o.StartDirectory))
	}
	for _, v := range o.Variable {
		attrs = append(attrs, entity.Component(ConfigVariableId, v.Encode()))
	}
	return
}

func (o *Config) Empty() bool {
	if len(o.Commands) != 0 {
		return false
	}
	if !entity.Empty(o.Entrypoint) {
		return false
	}
	if !entity.Empty(o.Port) {
		return false
	}
	if len(o.Services) != 0 {
		return false
	}
	if !entity.Empty(o.StartDirectory) {
		return false
	}
	if len(o.Variable) != 0 {
		return false
	}
	return true
}

func (o *Config) InitSchema(sb *schema.SchemaBuilder) {
	sb.Component("commands", "dev.miren.core/config.commands", schema.Doc("The command to run for a specific service type"), schema.Many)
	(&Commands{}).InitSchema(sb.Builder("config.commands"))
	sb.String("entrypoint", "dev.miren.core/config.entrypoint", schema.Doc("The container entrypoint command"))
	sb.Int64("port", "dev.miren.core/config.port", schema.Doc("[DEPRECATED] Port used for the web service; defaults to 3000. Prefer per-service ports."))
	sb.Component("services", "dev.miren.core/config.services", schema.Doc("Per-service configuration including concurrency controls"), schema.Many)
	(&Services{}).InitSchema(sb.Builder("config.services"))
	sb.String("start_directory", "dev.miren.core/config.start_directory", schema.Doc("Directory to start the process in; defaults to /app."))
	sb.Component("variable", "dev.miren.core/config.variable", schema.Doc("A variable to be exposed to the app"), schema.Many)
	(&Variable{}).InitSchema(sb.Builder("config.variable"))
}

const (
	CommandsCommandId = entity.Id("dev.miren.core/commands.command")
	CommandsServiceId = entity.Id("dev.miren.core/commands.service")
)

type Commands struct {
	Command string `cbor:"command,omitempty" json:"command,omitempty"`
	Service string `cbor:"service,omitempty" json:"service,omitempty"`
}

func (o *Commands) Decode(e entity.AttrGetter) {
	if a, ok := e.Get(CommandsCommandId); ok && a.Value.Kind() == entity.KindString {
		o.Command = a.Value.String()
	}
	if a, ok := e.Get(CommandsServiceId); ok && a.Value.Kind() == entity.KindString {
		o.Service = a.Value.String()
	}
}

func (o *Commands) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.Command) {
		attrs = append(attrs, entity.String(CommandsCommandId, o.Command))
	}
	if !entity.Empty(o.Service) {
		attrs = append(attrs, entity.String(CommandsServiceId, o.Service))
	}
	return
}

func (o *Commands) Empty() bool {
	if !entity.Empty(o.Command) {
		return false
	}
	if !entity.Empty(o.Service) {
		return false
	}
	return true
}

func (o *Commands) InitSchema(sb *schema.SchemaBuilder) {
	sb.String("command", "dev.miren.core/commands.command", schema.Doc("The command to run for the service"))
	sb.String("service", "dev.miren.core/commands.service", schema.Doc("The service name"))
}

const (
	ServicesDisksId              = entity.Id("dev.miren.core/services.disks")
	ServicesEnvId                = entity.Id("dev.miren.core/services.env")
	ServicesImageId              = entity.Id("dev.miren.core/services.image")
	ServicesNameId               = entity.Id("dev.miren.core/services.name")
	ServicesPortId               = entity.Id("dev.miren.core/services.port")
	ServicesPortNameId           = entity.Id("dev.miren.core/services.port_name")
	ServicesPortTypeId           = entity.Id("dev.miren.core/services.port_type")
	ServicesPortsId              = entity.Id("dev.miren.core/services.ports")
	ServicesServiceConcurrencyId = entity.Id("dev.miren.core/services.service_concurrency")
)

type Services struct {
	Disks              []Disks            `cbor:"disks,omitempty" json:"disks,omitempty"`
	Env                []Env              `cbor:"env,omitempty" json:"env,omitempty"`
	Image              string             `cbor:"image,omitempty" json:"image,omitempty"`
	Name               string             `cbor:"name,omitempty" json:"name,omitempty"`
	Port               int64              `cbor:"port,omitempty" json:"port,omitempty"`
	PortName           string             `cbor:"port_name,omitempty" json:"port_name,omitempty"`
	PortType           string             `cbor:"port_type,omitempty" json:"port_type,omitempty"`
	Ports              []Ports            `cbor:"ports,omitempty" json:"ports,omitempty"`
	ServiceConcurrency ServiceConcurrency `cbor:"service_concurrency,omitempty" json:"service_concurrency,omitempty"`
}

func (o *Services) Decode(e entity.AttrGetter) {
	for _, a := range e.GetAll(ServicesDisksId) {
		if a.Value.Kind() == entity.KindComponent {
			var v Disks
			v.Decode(a.Value.Component())
			o.Disks = append(o.Disks, v)
		}
	}
	for _, a := range e.GetAll(ServicesEnvId) {
		if a.Value.Kind() == entity.KindComponent {
			var v Env
			v.Decode(a.Value.Component())
			o.Env = append(o.Env, v)
		}
	}
	if a, ok := e.Get(ServicesImageId); ok && a.Value.Kind() == entity.KindString {
		o.Image = a.Value.String()
	}
	if a, ok := e.Get(ServicesNameId); ok && a.Value.Kind() == entity.KindString {
		o.Name = a.Value.String()
	}
	if a, ok := e.Get(ServicesPortId); ok && a.Value.Kind() == entity.KindInt64 {
		o.Port = a.Value.Int64()
	}
	if a, ok := e.Get(ServicesPortNameId); ok && a.Value.Kind() == entity.KindString {
		o.PortName = a.Value.String()
	}
	if a, ok := e.Get(ServicesPortTypeId); ok && a.Value.Kind() == entity.KindString {
		o.PortType = a.Value.String()
	}
	for _, a := range e.GetAll(ServicesPortsId) {
		if a.Value.Kind() == entity.KindComponent {
			var v Ports
			v.Decode(a.Value.Component())
			o.Ports = append(o.Ports, v)
		}
	}
	if a, ok := e.Get(ServicesServiceConcurrencyId); ok && a.Value.Kind() == entity.KindComponent {
		o.ServiceConcurrency.Decode(a.Value.Component())
	}
}

func (o *Services) Encode() (attrs []entity.Attr) {
	for _, v := range o.Disks {
		attrs = append(attrs, entity.Component(ServicesDisksId, v.Encode()))
	}
	for _, v := range o.Env {
		attrs = append(attrs, entity.Component(ServicesEnvId, v.Encode()))
	}
	if !entity.Empty(o.Image) {
		attrs = append(attrs, entity.String(ServicesImageId, o.Image))
	}
	if !entity.Empty(o.Name) {
		attrs = append(attrs, entity.String(ServicesNameId, o.Name))
	}
	if !entity.Empty(o.Port) {
		attrs = append(attrs, entity.Int64(ServicesPortId, o.Port))
	}
	if !entity.Empty(o.PortName) {
		attrs = append(attrs, entity.String(ServicesPortNameId, o.PortName))
	}
	if !entity.Empty(o.PortType) {
		attrs = append(attrs, entity.String(ServicesPortTypeId, o.PortType))
	}
	for _, v := range o.Ports {
		attrs = append(attrs, entity.Component(ServicesPortsId, v.Encode()))
	}
	if !o.ServiceConcurrency.Empty() {
		attrs = append(attrs, entity.Component(ServicesServiceConcurrencyId, o.ServiceConcurrency.Encode()))
	}
	return
}

func (o *Services) Empty() bool {
	if len(o.Disks) != 0 {
		return false
	}
	if len(o.Env) != 0 {
		return false
	}
	if !entity.Empty(o.Image) {
		return false
	}
	if !entity.Empty(o.Name) {
		return false
	}
	if !entity.Empty(o.Port) {
		return false
	}
	if !entity.Empty(o.PortName) {
		return false
	}
	if !entity.Empty(o.PortType) {
		return false
	}
	if len(o.Ports) != 0 {
		return false
	}
	if !o.ServiceConcurrency.Empty() {
		return false
	}
	return true
}

func (o *Services) InitSchema(sb *schema.SchemaBuilder) {
	sb.Component("disks", "dev.miren.core/services.disks", schema.Doc("Disk attachments for this service"), schema.Many)
	(&Disks{}).InitSchema(sb.Builder("services.disks"))
	sb.Component("env", "dev.miren.core/services.env", schema.Doc("Environment variables for this service only"), schema.Many)
	(&Env{}).InitSchema(sb.Builder("services.env"))
	sb.String("image", "dev.miren.core/services.image", schema.Doc("Optional container image for this service (e.g. postgres:16). If not specified, uses the app-level built image."))
	sb.String("name", "dev.miren.core/services.name", schema.Doc("The service name (e.g. web, worker)"))
	sb.Int64("port", "dev.miren.core/services.port", schema.Doc("The TCP port the service listens on. For the web service, if not specified it falls back to the deprecated top-level port (if set) or 3000. Other services must specify services.port explicitly and do not inherit the top-level port."))
	sb.String("port_name", "dev.miren.core/services.port_name", schema.Doc("The name of the port (e.g. http, grpc). Defaults to \"http\" if not specified."))
	sb.String("port_type", "dev.miren.core/services.port_type", schema.Doc("The type of the port (e.g. http, tcp). Defaults to \"http\" if not specified."))
	sb.Component("ports", "dev.miren.core/services.ports", schema.Doc("Network ports this service listens on. Overrides scalar port/port_name/port_type."), schema.Many)
	(&Ports{}).InitSchema(sb.Builder("services.ports"))
	sb.Component("service_concurrency", "dev.miren.core/services.service_concurrency", schema.Doc("Concurrency configuration for this service"))
	(&ServiceConcurrency{}).InitSchema(sb.Builder("services.service_concurrency"))
}

const (
	DisksFilesystemId    = entity.Id("dev.miren.core/disks.filesystem")
	DisksLeaseTimeoutId  = entity.Id("dev.miren.core/disks.lease_timeout")
	DisksMountPathId     = entity.Id("dev.miren.core/disks.mount_path")
	DisksNameId          = entity.Id("dev.miren.core/disks.name")
	DisksProviderId      = entity.Id("dev.miren.core/disks.provider")
	DisksProviderMirenId = entity.Id("dev.miren.core/provider.miren")
	DisksProviderLocalId = entity.Id("dev.miren.core/provider.local")
	DisksReadOnlyId      = entity.Id("dev.miren.core/disks.read_only")
	DisksSizeGbId        = entity.Id("dev.miren.core/disks.size_gb")
)

type Disks struct {
	Filesystem   string        `cbor:"filesystem,omitempty" json:"filesystem,omitempty"`
	LeaseTimeout string        `cbor:"lease_timeout,omitempty" json:"lease_timeout,omitempty"`
	MountPath    string        `cbor:"mount_path,omitempty" json:"mount_path,omitempty"`
	Name         string        `cbor:"name,omitempty" json:"name,omitempty"`
	Provider     DisksProvider `cbor:"provider,omitempty" json:"provider,omitempty"`
	ReadOnly     bool          `cbor:"read_only,omitempty" json:"read_only,omitempty"`
	SizeGb       int64         `cbor:"size_gb,omitempty" json:"size_gb,omitempty"`
}

type DisksProvider string

const (
	MIREN DisksProvider = "provider.miren"
	LOCAL DisksProvider = "provider.local"
)

var DisksproviderFromId = map[entity.Id]DisksProvider{DisksProviderMirenId: MIREN, DisksProviderLocalId: LOCAL}
var DisksproviderToId = map[DisksProvider]entity.Id{MIREN: DisksProviderMirenId, LOCAL: DisksProviderLocalId}

func (o *Disks) Decode(e entity.AttrGetter) {
	if a, ok := e.Get(DisksFilesystemId); ok && a.Value.Kind() == entity.KindString {
		o.Filesystem = a.Value.String()
	}
	if a, ok := e.Get(DisksLeaseTimeoutId); ok && a.Value.Kind() == entity.KindString {
		o.LeaseTimeout = a.Value.String()
	}
	if a, ok := e.Get(DisksMountPathId); ok && a.Value.Kind() == entity.KindString {
		o.MountPath = a.Value.String()
	}
	if a, ok := e.Get(DisksNameId); ok && a.Value.Kind() == entity.KindString {
		o.Name = a.Value.String()
	}
	if a, ok := e.Get(DisksProviderId); ok && a.Value.Kind() == entity.KindId {
		o.Provider = DisksproviderFromId[a.Value.Id()]
	}
	if a, ok := e.Get(DisksReadOnlyId); ok && a.Value.Kind() == entity.KindBool {
		o.ReadOnly = a.Value.Bool()
	}
	if a, ok := e.Get(DisksSizeGbId); ok && a.Value.Kind() == entity.KindInt64 {
		o.SizeGb = a.Value.Int64()
	}
}

func (o *Disks) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.Filesystem) {
		attrs = append(attrs, entity.String(DisksFilesystemId, o.Filesystem))
	}
	if !entity.Empty(o.LeaseTimeout) {
		attrs = append(attrs, entity.String(DisksLeaseTimeoutId, o.LeaseTimeout))
	}
	if !entity.Empty(o.MountPath) {
		attrs = append(attrs, entity.String(DisksMountPathId, o.MountPath))
	}
	if !entity.Empty(o.Name) {
		attrs = append(attrs, entity.String(DisksNameId, o.Name))
	}
	if a, ok := DisksproviderToId[o.Provider]; ok {
		attrs = append(attrs, entity.Ref(DisksProviderId, a))
	}
	attrs = append(attrs, entity.Bool(DisksReadOnlyId, o.ReadOnly))
	if !entity.Empty(o.SizeGb) {
		attrs = append(attrs, entity.Int64(DisksSizeGbId, o.SizeGb))
	}
	return
}

func (o *Disks) Empty() bool {
	if !entity.Empty(o.Filesystem) {
		return false
	}
	if !entity.Empty(o.LeaseTimeout) {
		return false
	}
	if !entity.Empty(o.MountPath) {
		return false
	}
	if !entity.Empty(o.Name) {
		return false
	}
	if o.Provider != "" {
		return false
	}
	if !entity.Empty(o.ReadOnly) {
		return false
	}
	if !entity.Empty(o.SizeGb) {
		return false
	}
	return true
}

func (o *Disks) InitSchema(sb *schema.SchemaBuilder) {
	sb.String("filesystem", "dev.miren.core/disks.filesystem", schema.Doc("Filesystem type (ext4, xfs, btrfs) for auto-creating the disk"))
	sb.String("lease_timeout", "dev.miren.core/disks.lease_timeout", schema.Doc("Timeout for acquiring the disk lease (e.g. 5m, 10m)"))
	sb.String("mount_path", "dev.miren.core/disks.mount_path", schema.Doc("The path inside the container where the disk will be mounted"))
	sb.String("name", "dev.miren.core/disks.name", schema.Doc("The name of the disk"))
	sb.Singleton("dev.miren.core/provider.miren")
	sb.Singleton("dev.miren.core/provider.local")
	sb.Ref("provider", "dev.miren.core/disks.provider", schema.Doc("Disk provider: 'miren' (default) for network disks, 'local' for node-local persistent storage"), schema.Choices(DisksProviderMirenId, DisksProviderLocalId))
	sb.Bool("read_only", "dev.miren.core/disks.read_only", schema.Doc("Whether to mount the disk as read-only"))
	sb.Int64("size_gb", "dev.miren.core/disks.size_gb", schema.Doc("Size in GB for auto-creating the disk if it doesn't exist"))
}

const (
	EnvDescriptionId = entity.Id("dev.miren.core/env.description")
	EnvKeyId         = entity.Id("dev.miren.core/env.key")
	EnvOriginId      = entity.Id("dev.miren.core/env.origin")
	EnvRequiredId    = entity.Id("dev.miren.core/env.required")
	EnvSensitiveId   = entity.Id("dev.miren.core/env.sensitive")
	EnvSourceId      = entity.Id("dev.miren.core/env.source")
	EnvValueId       = entity.Id("dev.miren.core/env.value")
)

type Env struct {
	Description string `cbor:"description,omitempty" json:"description,omitempty"`
	Key         string `cbor:"key,omitempty" json:"key,omitempty"`
	Origin      string `cbor:"origin,omitempty" json:"origin,omitempty"`
	Required    bool   `cbor:"required,omitempty" json:"required,omitempty"`
	Sensitive   bool   `cbor:"sensitive,omitempty" json:"sensitive,omitempty"`
	Source      string `cbor:"source,omitempty" json:"source,omitempty"`
	Value       string `cbor:"value,omitempty" json:"value,omitempty"`
}

func (o *Env) Decode(e entity.AttrGetter) {
	if a, ok := e.Get(EnvDescriptionId); ok && a.Value.Kind() == entity.KindString {
		o.Description = a.Value.String()
	}
	if a, ok := e.Get(EnvKeyId); ok && a.Value.Kind() == entity.KindString {
		o.Key = a.Value.String()
	}
	if a, ok := e.Get(EnvOriginId); ok && a.Value.Kind() == entity.KindString {
		o.Origin = a.Value.String()
	}
	if a, ok := e.Get(EnvRequiredId); ok && a.Value.Kind() == entity.KindBool {
		o.Required = a.Value.Bool()
	}
	if a, ok := e.Get(EnvSensitiveId); ok && a.Value.Kind() == entity.KindBool {
		o.Sensitive = a.Value.Bool()
	}
	if a, ok := e.Get(EnvSourceId); ok && a.Value.Kind() == entity.KindString {
		o.Source = a.Value.String()
	}
	if a, ok := e.Get(EnvValueId); ok && a.Value.Kind() == entity.KindString {
		o.Value = a.Value.String()
	}
}

func (o *Env) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.Description) {
		attrs = append(attrs, entity.String(EnvDescriptionId, o.Description))
	}
	if !entity.Empty(o.Key) {
		attrs = append(attrs, entity.String(EnvKeyId, o.Key))
	}
	if !entity.Empty(o.Origin) {
		attrs = append(attrs, entity.String(EnvOriginId, o.Origin))
	}
	attrs = append(attrs, entity.Bool(EnvRequiredId, o.Required))
	attrs = append(attrs, entity.Bool(EnvSensitiveId, o.Sensitive))
	if !entity.Empty(o.Source) {
		attrs = append(attrs, entity.String(EnvSourceId, o.Source))
	}
	if !entity.Empty(o.Value) {
		attrs = append(attrs, entity.String(EnvValueId, o.Value))
	}
	return
}

func (o *Env) Empty() bool {
	if !entity.Empty(o.Description) {
		return false
	}
	if !entity.Empty(o.Key) {
		return false
	}
	if !entity.Empty(o.Origin) {
		return false
	}
	if !entity.Empty(o.Required) {
		return false
	}
	if !entity.Empty(o.Sensitive) {
		return false
	}
	if !entity.Empty(o.Source) {
		return false
	}
	if !entity.Empty(o.Value) {
		return false
	}
	return true
}

func (o *Env) InitSchema(sb *schema.SchemaBuilder) {
	sb.String("description", "dev.miren.core/env.description", schema.Doc("Human-readable description of this variable's purpose"))
	sb.String("key", "dev.miren.core/env.key", schema.Doc("The name of the variable"))
	sb.String("origin", "dev.miren.core/env.origin", schema.Doc("The provenance of the variable (user, file, generated, detected)."))
	sb.Bool("required", "dev.miren.core/env.required", schema.Doc("Whether this variable must have a non-empty value for deploy to succeed"))
	sb.Bool("sensitive", "dev.miren.core/env.sensitive", schema.Doc("Whether or not the value is sensitive"))
	sb.String("source", "dev.miren.core/env.source", schema.Doc("The source of the variable (config or manual). Defaults to config for backward compatibility."))
	sb.String("value", "dev.miren.core/env.value", schema.Doc("The value of the variable"))
}

const (
	PortsNameId        = entity.Id("dev.miren.core/ports.name")
	PortsNodePortId    = entity.Id("dev.miren.core/ports.node_port")
	PortsPortId        = entity.Id("dev.miren.core/ports.port")
	PortsProtocolId    = entity.Id("dev.miren.core/ports.protocol")
	PortsProtocolTcpId = entity.Id("dev.miren.core/protocol.tcp")
	PortsProtocolUdpId = entity.Id("dev.miren.core/protocol.udp")
	PortsTypeId        = entity.Id("dev.miren.core/ports.type")
)

type Ports struct {
	Name     string        `cbor:"name" json:"name"`
	NodePort int64         `cbor:"node_port,omitempty" json:"node_port,omitempty"`
	Port     int64         `cbor:"port" json:"port"`
	Protocol PortsProtocol `cbor:"protocol,omitempty" json:"protocol,omitempty"`
	Type     string        `cbor:"type,omitempty" json:"type,omitempty"`
}

type PortsProtocol string

const (
	TCP PortsProtocol = "protocol.tcp"
	UDP PortsProtocol = "protocol.udp"
)

var PortsprotocolFromId = map[entity.Id]PortsProtocol{PortsProtocolTcpId: TCP, PortsProtocolUdpId: UDP}
var PortsprotocolToId = map[PortsProtocol]entity.Id{TCP: PortsProtocolTcpId, UDP: PortsProtocolUdpId}

func (o *Ports) Decode(e entity.AttrGetter) {
	if a, ok := e.Get(PortsNameId); ok && a.Value.Kind() == entity.KindString {
		o.Name = a.Value.String()
	}
	if a, ok := e.Get(PortsNodePortId); ok && a.Value.Kind() == entity.KindInt64 {
		o.NodePort = a.Value.Int64()
	}
	if a, ok := e.Get(PortsPortId); ok && a.Value.Kind() == entity.KindInt64 {
		o.Port = a.Value.Int64()
	}
	if a, ok := e.Get(PortsProtocolId); ok && a.Value.Kind() == entity.KindId {
		o.Protocol = PortsprotocolFromId[a.Value.Id()]
	}
	if a, ok := e.Get(PortsTypeId); ok && a.Value.Kind() == entity.KindString {
		o.Type = a.Value.String()
	}
}

func (o *Ports) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.Name) {
		attrs = append(attrs, entity.String(PortsNameId, o.Name))
	}
	if !entity.Empty(o.NodePort) {
		attrs = append(attrs, entity.Int64(PortsNodePortId, o.NodePort))
	}
	attrs = append(attrs, entity.Int64(PortsPortId, o.Port))
	if a, ok := PortsprotocolToId[o.Protocol]; ok {
		attrs = append(attrs, entity.Ref(PortsProtocolId, a))
	}
	if !entity.Empty(o.Type) {
		attrs = append(attrs, entity.String(PortsTypeId, o.Type))
	}
	return
}

func (o *Ports) Empty() bool {
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
	if !entity.Empty(o.Type) {
		return false
	}
	return true
}

func (o *Ports) InitSchema(sb *schema.SchemaBuilder) {
	sb.String("name", "dev.miren.core/ports.name", schema.Required)
	sb.Int64("node_port", "dev.miren.core/ports.node_port")
	sb.Int64("port", "dev.miren.core/ports.port", schema.Required)
	sb.Singleton("dev.miren.core/protocol.tcp")
	sb.Singleton("dev.miren.core/protocol.udp")
	sb.Ref("protocol", "dev.miren.core/ports.protocol", schema.Choices(PortsProtocolTcpId, PortsProtocolUdpId))
	sb.String("type", "dev.miren.core/ports.type")
}

const (
	ServiceConcurrencyModeId                = entity.Id("dev.miren.core/service_concurrency.mode")
	ServiceConcurrencyNumInstancesId        = entity.Id("dev.miren.core/service_concurrency.num_instances")
	ServiceConcurrencyRequestsPerInstanceId = entity.Id("dev.miren.core/service_concurrency.requests_per_instance")
	ServiceConcurrencyScaleDownDelayId      = entity.Id("dev.miren.core/service_concurrency.scale_down_delay")
	ServiceConcurrencyShutdownTimeoutId     = entity.Id("dev.miren.core/service_concurrency.shutdown_timeout")
)

type ServiceConcurrency struct {
	Mode                string `cbor:"mode,omitempty" json:"mode,omitempty"`
	NumInstances        int64  `cbor:"num_instances,omitempty" json:"num_instances,omitempty"`
	RequestsPerInstance int64  `cbor:"requests_per_instance,omitempty" json:"requests_per_instance,omitempty"`
	ScaleDownDelay      string `cbor:"scale_down_delay,omitempty" json:"scale_down_delay,omitempty"`
	ShutdownTimeout     string `cbor:"shutdown_timeout,omitempty" json:"shutdown_timeout,omitempty"`
}

func (o *ServiceConcurrency) Decode(e entity.AttrGetter) {
	if a, ok := e.Get(ServiceConcurrencyModeId); ok && a.Value.Kind() == entity.KindString {
		o.Mode = a.Value.String()
	}
	if a, ok := e.Get(ServiceConcurrencyNumInstancesId); ok && a.Value.Kind() == entity.KindInt64 {
		o.NumInstances = a.Value.Int64()
	}
	if a, ok := e.Get(ServiceConcurrencyRequestsPerInstanceId); ok && a.Value.Kind() == entity.KindInt64 {
		o.RequestsPerInstance = a.Value.Int64()
	}
	if a, ok := e.Get(ServiceConcurrencyScaleDownDelayId); ok && a.Value.Kind() == entity.KindString {
		o.ScaleDownDelay = a.Value.String()
	}
	if a, ok := e.Get(ServiceConcurrencyShutdownTimeoutId); ok && a.Value.Kind() == entity.KindString {
		o.ShutdownTimeout = a.Value.String()
	}
}

func (o *ServiceConcurrency) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.Mode) {
		attrs = append(attrs, entity.String(ServiceConcurrencyModeId, o.Mode))
	}
	if !entity.Empty(o.NumInstances) {
		attrs = append(attrs, entity.Int64(ServiceConcurrencyNumInstancesId, o.NumInstances))
	}
	if !entity.Empty(o.RequestsPerInstance) {
		attrs = append(attrs, entity.Int64(ServiceConcurrencyRequestsPerInstanceId, o.RequestsPerInstance))
	}
	if !entity.Empty(o.ScaleDownDelay) {
		attrs = append(attrs, entity.String(ServiceConcurrencyScaleDownDelayId, o.ScaleDownDelay))
	}
	if !entity.Empty(o.ShutdownTimeout) {
		attrs = append(attrs, entity.String(ServiceConcurrencyShutdownTimeoutId, o.ShutdownTimeout))
	}
	return
}

func (o *ServiceConcurrency) Empty() bool {
	if !entity.Empty(o.Mode) {
		return false
	}
	if !entity.Empty(o.NumInstances) {
		return false
	}
	if !entity.Empty(o.RequestsPerInstance) {
		return false
	}
	if !entity.Empty(o.ScaleDownDelay) {
		return false
	}
	if !entity.Empty(o.ShutdownTimeout) {
		return false
	}
	return true
}

func (o *ServiceConcurrency) InitSchema(sb *schema.SchemaBuilder) {
	sb.String("mode", "dev.miren.core/service_concurrency.mode", schema.Doc("The concurrency mode (auto or fixed)"))
	sb.Int64("num_instances", "dev.miren.core/service_concurrency.num_instances", schema.Doc("For fixed mode, number of instances to maintain"))
	sb.Int64("requests_per_instance", "dev.miren.core/service_concurrency.requests_per_instance", schema.Doc("For auto mode, number of concurrent requests per instance"))
	sb.String("scale_down_delay", "dev.miren.core/service_concurrency.scale_down_delay", schema.Doc("For auto mode, delay before scaling down idle instances (e.g. 2m, 15m)"))
	sb.String("shutdown_timeout", "dev.miren.core/service_concurrency.shutdown_timeout", schema.Doc("Time to wait for graceful shutdown before force-killing (e.g. 10s, 30s). Defaults to 10s."))
}

const (
	VariableDescriptionId = entity.Id("dev.miren.core/variable.description")
	VariableKeyId         = entity.Id("dev.miren.core/variable.key")
	VariableOriginId      = entity.Id("dev.miren.core/variable.origin")
	VariableRequiredId    = entity.Id("dev.miren.core/variable.required")
	VariableSensitiveId   = entity.Id("dev.miren.core/variable.sensitive")
	VariableSourceId      = entity.Id("dev.miren.core/variable.source")
	VariableValueId       = entity.Id("dev.miren.core/variable.value")
)

type Variable struct {
	Description string `cbor:"description,omitempty" json:"description,omitempty"`
	Key         string `cbor:"key,omitempty" json:"key,omitempty"`
	Origin      string `cbor:"origin,omitempty" json:"origin,omitempty"`
	Required    bool   `cbor:"required,omitempty" json:"required,omitempty"`
	Sensitive   bool   `cbor:"sensitive,omitempty" json:"sensitive,omitempty"`
	Source      string `cbor:"source,omitempty" json:"source,omitempty"`
	Value       string `cbor:"value,omitempty" json:"value,omitempty"`
}

func (o *Variable) Decode(e entity.AttrGetter) {
	if a, ok := e.Get(VariableDescriptionId); ok && a.Value.Kind() == entity.KindString {
		o.Description = a.Value.String()
	}
	if a, ok := e.Get(VariableKeyId); ok && a.Value.Kind() == entity.KindString {
		o.Key = a.Value.String()
	}
	if a, ok := e.Get(VariableOriginId); ok && a.Value.Kind() == entity.KindString {
		o.Origin = a.Value.String()
	}
	if a, ok := e.Get(VariableRequiredId); ok && a.Value.Kind() == entity.KindBool {
		o.Required = a.Value.Bool()
	}
	if a, ok := e.Get(VariableSensitiveId); ok && a.Value.Kind() == entity.KindBool {
		o.Sensitive = a.Value.Bool()
	}
	if a, ok := e.Get(VariableSourceId); ok && a.Value.Kind() == entity.KindString {
		o.Source = a.Value.String()
	}
	if a, ok := e.Get(VariableValueId); ok && a.Value.Kind() == entity.KindString {
		o.Value = a.Value.String()
	}
}

func (o *Variable) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.Description) {
		attrs = append(attrs, entity.String(VariableDescriptionId, o.Description))
	}
	if !entity.Empty(o.Key) {
		attrs = append(attrs, entity.String(VariableKeyId, o.Key))
	}
	if !entity.Empty(o.Origin) {
		attrs = append(attrs, entity.String(VariableOriginId, o.Origin))
	}
	attrs = append(attrs, entity.Bool(VariableRequiredId, o.Required))
	attrs = append(attrs, entity.Bool(VariableSensitiveId, o.Sensitive))
	if !entity.Empty(o.Source) {
		attrs = append(attrs, entity.String(VariableSourceId, o.Source))
	}
	if !entity.Empty(o.Value) {
		attrs = append(attrs, entity.String(VariableValueId, o.Value))
	}
	return
}

func (o *Variable) Empty() bool {
	if !entity.Empty(o.Description) {
		return false
	}
	if !entity.Empty(o.Key) {
		return false
	}
	if !entity.Empty(o.Origin) {
		return false
	}
	if !entity.Empty(o.Required) {
		return false
	}
	if !entity.Empty(o.Sensitive) {
		return false
	}
	if !entity.Empty(o.Source) {
		return false
	}
	if !entity.Empty(o.Value) {
		return false
	}
	return true
}

func (o *Variable) InitSchema(sb *schema.SchemaBuilder) {
	sb.String("description", "dev.miren.core/variable.description", schema.Doc("Human-readable description of this variable's purpose"))
	sb.String("key", "dev.miren.core/variable.key", schema.Doc("The name of the variable"))
	sb.String("origin", "dev.miren.core/variable.origin", schema.Doc("The provenance of the variable (user, file, generated, detected)."))
	sb.Bool("required", "dev.miren.core/variable.required", schema.Doc("Whether this variable must have a non-empty value for deploy to succeed"))
	sb.Bool("sensitive", "dev.miren.core/variable.sensitive", schema.Doc("Whether or not the value is sensitive"))
	sb.String("source", "dev.miren.core/variable.source", schema.Doc("The source of the variable (config or manual). Defaults to config for backward compatibility."))
	sb.String("value", "dev.miren.core/variable.value", schema.Doc("The value of the value"))
}

const (
	ArtifactAppId            = entity.Id("dev.miren.core/artifact.app")
	ArtifactManifestId       = entity.Id("dev.miren.core/artifact.manifest")
	ArtifactManifestDigestId = entity.Id("dev.miren.core/artifact.manifest_digest")
	ArtifactStatusId         = entity.Id("dev.miren.core/artifact.status")
	ArtifactStatusActiveId   = entity.Id("dev.miren.core/status.active")
	ArtifactStatusArchivedId = entity.Id("dev.miren.core/status.archived")
)

type Artifact struct {
	ID             entity.Id      `json:"id"`
	App            entity.Id      `cbor:"app,omitempty" json:"app,omitempty"`
	Manifest       string         `cbor:"manifest,omitempty" json:"manifest,omitempty"`
	ManifestDigest string         `cbor:"manifest_digest,omitempty" json:"manifest_digest,omitempty"`
	Status         ArtifactStatus `cbor:"status,omitempty" json:"status,omitempty"`
}

type ArtifactStatus string

const (
	ACTIVE   ArtifactStatus = "status.active"
	ARCHIVED ArtifactStatus = "status.archived"
)

var artifactstatusFromId = map[entity.Id]ArtifactStatus{ArtifactStatusActiveId: ACTIVE, ArtifactStatusArchivedId: ARCHIVED}
var artifactstatusToId = map[ArtifactStatus]entity.Id{ACTIVE: ArtifactStatusActiveId, ARCHIVED: ArtifactStatusArchivedId}

func (o *Artifact) Decode(e entity.AttrGetter) {
	o.ID = entity.MustGet(e, entity.DBId).Value.Id()
	if a, ok := e.Get(ArtifactAppId); ok && a.Value.Kind() == entity.KindId {
		o.App = a.Value.Id()
	}
	if a, ok := e.Get(ArtifactManifestId); ok && a.Value.Kind() == entity.KindString {
		o.Manifest = a.Value.String()
	}
	if a, ok := e.Get(ArtifactManifestDigestId); ok && a.Value.Kind() == entity.KindString {
		o.ManifestDigest = a.Value.String()
	}
	if a, ok := e.Get(ArtifactStatusId); ok && a.Value.Kind() == entity.KindId {
		o.Status = artifactstatusFromId[a.Value.Id()]
	}
}

func (o *Artifact) Is(e entity.AttrGetter) bool {
	return entity.Is(e, KindArtifact)
}

func (o *Artifact) ShortKind() string {
	return "artifact"
}

func (o *Artifact) Kind() entity.Id {
	return KindArtifact
}

func (o *Artifact) EntityId() entity.Id {
	return o.ID
}

func (o *Artifact) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.App) {
		attrs = append(attrs, entity.Ref(ArtifactAppId, o.App))
	}
	if !entity.Empty(o.Manifest) {
		attrs = append(attrs, entity.String(ArtifactManifestId, o.Manifest))
	}
	if !entity.Empty(o.ManifestDigest) {
		attrs = append(attrs, entity.String(ArtifactManifestDigestId, o.ManifestDigest))
	}
	if a, ok := artifactstatusToId[o.Status]; ok {
		attrs = append(attrs, entity.Ref(ArtifactStatusId, a))
	}
	attrs = append(attrs, entity.Ref(entity.EntityKind, KindArtifact))
	return
}

func (o *Artifact) Empty() bool {
	if !entity.Empty(o.App) {
		return false
	}
	if !entity.Empty(o.Manifest) {
		return false
	}
	if !entity.Empty(o.ManifestDigest) {
		return false
	}
	if o.Status != "" {
		return false
	}
	return true
}

func (o *Artifact) InitSchema(sb *schema.SchemaBuilder) {
	sb.Ref("app", "dev.miren.core/artifact.app", schema.Doc("The application the artifact is for"), schema.Indexed, schema.Tags("dev.miren.app_ref"))
	sb.String("manifest", "dev.miren.core/artifact.manifest", schema.Doc("The OCI image manifest for the version"))
	sb.String("manifest_digest", "dev.miren.core/artifact.manifest_digest", schema.Doc("The digest of the manifest"), schema.Indexed)
	sb.Singleton("dev.miren.core/status.active")
	sb.Singleton("dev.miren.core/status.archived")
	sb.Ref("status", "dev.miren.core/artifact.status", schema.Doc("Artifact lifecycle status"), schema.Indexed, schema.Choices(ArtifactStatusActiveId, ArtifactStatusArchivedId))
}

const (
	ConfigVersionAppId  = entity.Id("dev.miren.core/config_version.app")
	ConfigVersionSpecId = entity.Id("dev.miren.core/config_version.spec")
)

type ConfigVersion struct {
	ID   entity.Id  `json:"id"`
	App  entity.Id  `cbor:"app,omitempty" json:"app,omitempty"`
	Spec ConfigSpec `cbor:"spec,omitempty" json:"spec,omitempty"`
}

func (o *ConfigVersion) Decode(e entity.AttrGetter) {
	o.ID = entity.MustGet(e, entity.DBId).Value.Id()
	if a, ok := e.Get(ConfigVersionAppId); ok && a.Value.Kind() == entity.KindId {
		o.App = a.Value.Id()
	}
	if a, ok := e.Get(ConfigVersionSpecId); ok && a.Value.Kind() == entity.KindComponent {
		o.Spec.Decode(a.Value.Component())
	}
}

func (o *ConfigVersion) Is(e entity.AttrGetter) bool {
	return entity.Is(e, KindConfigVersion)
}

func (o *ConfigVersion) ShortKind() string {
	return "config_version"
}

func (o *ConfigVersion) Kind() entity.Id {
	return KindConfigVersion
}

func (o *ConfigVersion) EntityId() entity.Id {
	return o.ID
}

func (o *ConfigVersion) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.App) {
		attrs = append(attrs, entity.Ref(ConfigVersionAppId, o.App))
	}
	if !o.Spec.Empty() {
		attrs = append(attrs, entity.Component(ConfigVersionSpecId, o.Spec.Encode()))
	}
	attrs = append(attrs, entity.Ref(entity.EntityKind, KindConfigVersion))
	return
}

func (o *ConfigVersion) Empty() bool {
	if !entity.Empty(o.App) {
		return false
	}
	if !o.Spec.Empty() {
		return false
	}
	return true
}

func (o *ConfigVersion) InitSchema(sb *schema.SchemaBuilder) {
	sb.Ref("app", "dev.miren.core/config_version.app", schema.Doc("The application this config version belongs to"), schema.Indexed, schema.Tags("dev.miren.app_ref"))
	sb.Component("spec", "dev.miren.core/config_version.spec", schema.Doc("The configuration specification"))
}

const (
	DeploymentAppNameId            = entity.Id("dev.miren.core/deployment.app_name")
	DeploymentAppVersionId         = entity.Id("dev.miren.core/deployment.app_version")
	DeploymentBuildLogsId          = entity.Id("dev.miren.core/deployment.build_logs")
	DeploymentClusterIdId          = entity.Id("dev.miren.core/deployment.cluster_id")
	DeploymentCompletedAtId        = entity.Id("dev.miren.core/deployment.completed_at")
	DeploymentDeployedById         = entity.Id("dev.miren.core/deployment.deployed_by")
	DeploymentErrorMessageId       = entity.Id("dev.miren.core/deployment.error_message")
	DeploymentGitInfoId            = entity.Id("dev.miren.core/deployment.git_info")
	DeploymentPhaseId              = entity.Id("dev.miren.core/deployment.phase")
	DeploymentSourceDeploymentIdId = entity.Id("dev.miren.core/deployment.source_deployment_id")
	DeploymentStatusId             = entity.Id("dev.miren.core/deployment.status")
)

type Deployment struct {
	ID                 entity.Id  `json:"id"`
	AppName            string     `cbor:"app_name,omitempty" json:"app_name,omitempty"`
	AppVersion         string     `cbor:"app_version,omitempty" json:"app_version,omitempty"`
	BuildLogs          string     `cbor:"build_logs,omitempty" json:"build_logs,omitempty"`
	ClusterId          string     `cbor:"cluster_id,omitempty" json:"cluster_id,omitempty"`
	CompletedAt        string     `cbor:"completed_at,omitempty" json:"completed_at,omitempty"`
	DeployedBy         DeployedBy `cbor:"deployed_by,omitempty" json:"deployed_by,omitempty"`
	ErrorMessage       string     `cbor:"error_message,omitempty" json:"error_message,omitempty"`
	GitInfo            GitInfo    `cbor:"git_info,omitempty" json:"git_info,omitempty"`
	Phase              string     `cbor:"phase,omitempty" json:"phase,omitempty"`
	SourceDeploymentId string     `cbor:"source_deployment_id,omitempty" json:"source_deployment_id,omitempty"`
	Status             string     `cbor:"status,omitempty" json:"status,omitempty"`
}

func (o *Deployment) Decode(e entity.AttrGetter) {
	o.ID = entity.MustGet(e, entity.DBId).Value.Id()
	if a, ok := e.Get(DeploymentAppNameId); ok && a.Value.Kind() == entity.KindString {
		o.AppName = a.Value.String()
	}
	if a, ok := e.Get(DeploymentAppVersionId); ok && a.Value.Kind() == entity.KindString {
		o.AppVersion = a.Value.String()
	}
	if a, ok := e.Get(DeploymentBuildLogsId); ok && a.Value.Kind() == entity.KindString {
		o.BuildLogs = a.Value.String()
	}
	if a, ok := e.Get(DeploymentClusterIdId); ok && a.Value.Kind() == entity.KindString {
		o.ClusterId = a.Value.String()
	}
	if a, ok := e.Get(DeploymentCompletedAtId); ok && a.Value.Kind() == entity.KindString {
		o.CompletedAt = a.Value.String()
	}
	if a, ok := e.Get(DeploymentDeployedById); ok && a.Value.Kind() == entity.KindComponent {
		o.DeployedBy.Decode(a.Value.Component())
	}
	if a, ok := e.Get(DeploymentErrorMessageId); ok && a.Value.Kind() == entity.KindString {
		o.ErrorMessage = a.Value.String()
	}
	if a, ok := e.Get(DeploymentGitInfoId); ok && a.Value.Kind() == entity.KindComponent {
		o.GitInfo.Decode(a.Value.Component())
	}
	if a, ok := e.Get(DeploymentPhaseId); ok && a.Value.Kind() == entity.KindString {
		o.Phase = a.Value.String()
	}
	if a, ok := e.Get(DeploymentSourceDeploymentIdId); ok && a.Value.Kind() == entity.KindString {
		o.SourceDeploymentId = a.Value.String()
	}
	if a, ok := e.Get(DeploymentStatusId); ok && a.Value.Kind() == entity.KindString {
		o.Status = a.Value.String()
	}
}

func (o *Deployment) Is(e entity.AttrGetter) bool {
	return entity.Is(e, KindDeployment)
}

func (o *Deployment) ShortKind() string {
	return "deployment"
}

func (o *Deployment) Kind() entity.Id {
	return KindDeployment
}

func (o *Deployment) EntityId() entity.Id {
	return o.ID
}

func (o *Deployment) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.AppName) {
		attrs = append(attrs, entity.String(DeploymentAppNameId, o.AppName))
	}
	if !entity.Empty(o.AppVersion) {
		attrs = append(attrs, entity.String(DeploymentAppVersionId, o.AppVersion))
	}
	if !entity.Empty(o.BuildLogs) {
		attrs = append(attrs, entity.String(DeploymentBuildLogsId, o.BuildLogs))
	}
	if !entity.Empty(o.ClusterId) {
		attrs = append(attrs, entity.String(DeploymentClusterIdId, o.ClusterId))
	}
	if !entity.Empty(o.CompletedAt) {
		attrs = append(attrs, entity.String(DeploymentCompletedAtId, o.CompletedAt))
	}
	if !o.DeployedBy.Empty() {
		attrs = append(attrs, entity.Component(DeploymentDeployedById, o.DeployedBy.Encode()))
	}
	if !entity.Empty(o.ErrorMessage) {
		attrs = append(attrs, entity.String(DeploymentErrorMessageId, o.ErrorMessage))
	}
	if !o.GitInfo.Empty() {
		attrs = append(attrs, entity.Component(DeploymentGitInfoId, o.GitInfo.Encode()))
	}
	if !entity.Empty(o.Phase) {
		attrs = append(attrs, entity.String(DeploymentPhaseId, o.Phase))
	}
	if !entity.Empty(o.SourceDeploymentId) {
		attrs = append(attrs, entity.String(DeploymentSourceDeploymentIdId, o.SourceDeploymentId))
	}
	if !entity.Empty(o.Status) {
		attrs = append(attrs, entity.String(DeploymentStatusId, o.Status))
	}
	attrs = append(attrs, entity.Ref(entity.EntityKind, KindDeployment))
	return
}

func (o *Deployment) Empty() bool {
	if !entity.Empty(o.AppName) {
		return false
	}
	if !entity.Empty(o.AppVersion) {
		return false
	}
	if !entity.Empty(o.BuildLogs) {
		return false
	}
	if !entity.Empty(o.ClusterId) {
		return false
	}
	if !entity.Empty(o.CompletedAt) {
		return false
	}
	if !o.DeployedBy.Empty() {
		return false
	}
	if !entity.Empty(o.ErrorMessage) {
		return false
	}
	if !o.GitInfo.Empty() {
		return false
	}
	if !entity.Empty(o.Phase) {
		return false
	}
	if !entity.Empty(o.SourceDeploymentId) {
		return false
	}
	if !entity.Empty(o.Status) {
		return false
	}
	return true
}

func (o *Deployment) InitSchema(sb *schema.SchemaBuilder) {
	sb.String("app_name", "dev.miren.core/deployment.app_name", schema.Doc("The name of the app being deployed"), schema.Indexed)
	sb.String("app_version", "dev.miren.core/deployment.app_version", schema.Doc("The app version ID or temporary value (pending-build, failed-{id})"))
	sb.String("build_logs", "dev.miren.core/deployment.build_logs", schema.Doc("Build logs concatenated with newlines (especially useful for failed deployments)"))
	sb.String("cluster_id", "dev.miren.core/deployment.cluster_id", schema.Doc("The cluster where the deployment is happening"), schema.Indexed)
	sb.String("completed_at", "dev.miren.core/deployment.completed_at", schema.Doc("When the deployment was completed (RFC3339 format)"))
	sb.Component("deployed_by", "dev.miren.core/deployment.deployed_by", schema.Doc("Information about who initiated the deployment"))
	(&DeployedBy{}).InitSchema(sb.Builder("deployment.deployed_by"))
	sb.String("error_message", "dev.miren.core/deployment.error_message", schema.Doc("Error message if deployment failed"))
	sb.Component("git_info", "dev.miren.core/deployment.git_info", schema.Doc("Git information at time of deployment"))
	(&GitInfo{}).InitSchema(sb.Builder("deployment.git_info"))
	sb.String("phase", "dev.miren.core/deployment.phase", schema.Doc("Current phase of deployment (preparing, building, pushing, activating)"))
	sb.String("source_deployment_id", "dev.miren.core/deployment.source_deployment_id", schema.Doc("ID of the deployment this was based on (for rollback/redeploy provenance)"))
	sb.String("status", "dev.miren.core/deployment.status", schema.Doc("Deployment status (in_progress, active, failed, rolled_back)"), schema.Indexed)
}

const (
	DeployedByTimestampId = entity.Id("dev.miren.core/deployed_by.timestamp")
	DeployedByUserEmailId = entity.Id("dev.miren.core/deployed_by.user_email")
	DeployedByUserIdId    = entity.Id("dev.miren.core/deployed_by.user_id")
	DeployedByUserNameId  = entity.Id("dev.miren.core/deployed_by.user_name")
)

type DeployedBy struct {
	Timestamp string `cbor:"timestamp,omitempty" json:"timestamp,omitempty"`
	UserEmail string `cbor:"user_email,omitempty" json:"user_email,omitempty"`
	UserId    string `cbor:"user_id,omitempty" json:"user_id,omitempty"`
	UserName  string `cbor:"user_name,omitempty" json:"user_name,omitempty"`
}

func (o *DeployedBy) Decode(e entity.AttrGetter) {
	if a, ok := e.Get(DeployedByTimestampId); ok && a.Value.Kind() == entity.KindString {
		o.Timestamp = a.Value.String()
	}
	if a, ok := e.Get(DeployedByUserEmailId); ok && a.Value.Kind() == entity.KindString {
		o.UserEmail = a.Value.String()
	}
	if a, ok := e.Get(DeployedByUserIdId); ok && a.Value.Kind() == entity.KindString {
		o.UserId = a.Value.String()
	}
	if a, ok := e.Get(DeployedByUserNameId); ok && a.Value.Kind() == entity.KindString {
		o.UserName = a.Value.String()
	}
}

func (o *DeployedBy) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.Timestamp) {
		attrs = append(attrs, entity.String(DeployedByTimestampId, o.Timestamp))
	}
	if !entity.Empty(o.UserEmail) {
		attrs = append(attrs, entity.String(DeployedByUserEmailId, o.UserEmail))
	}
	if !entity.Empty(o.UserId) {
		attrs = append(attrs, entity.String(DeployedByUserIdId, o.UserId))
	}
	if !entity.Empty(o.UserName) {
		attrs = append(attrs, entity.String(DeployedByUserNameId, o.UserName))
	}
	return
}

func (o *DeployedBy) Empty() bool {
	if !entity.Empty(o.Timestamp) {
		return false
	}
	if !entity.Empty(o.UserEmail) {
		return false
	}
	if !entity.Empty(o.UserId) {
		return false
	}
	if !entity.Empty(o.UserName) {
		return false
	}
	return true
}

func (o *DeployedBy) InitSchema(sb *schema.SchemaBuilder) {
	sb.String("timestamp", "dev.miren.core/deployed_by.timestamp", schema.Doc("When the deployment was initiated (RFC3339 format)"))
	sb.String("user_email", "dev.miren.core/deployed_by.user_email", schema.Doc("The email of the user who deployed"))
	sb.String("user_id", "dev.miren.core/deployed_by.user_id", schema.Doc("The ID of the user who deployed"))
	sb.String("user_name", "dev.miren.core/deployed_by.user_name", schema.Doc("The username of the user who deployed"))
}

const (
	GitInfoAuthorId            = entity.Id("dev.miren.core/git_info.author")
	GitInfoBranchId            = entity.Id("dev.miren.core/git_info.branch")
	GitInfoCommitAuthorEmailId = entity.Id("dev.miren.core/git_info.commit_author_email")
	GitInfoCommitTimestampId   = entity.Id("dev.miren.core/git_info.commit_timestamp")
	GitInfoIsDirtyId           = entity.Id("dev.miren.core/git_info.is_dirty")
	GitInfoMessageId           = entity.Id("dev.miren.core/git_info.message")
	GitInfoRepositoryId        = entity.Id("dev.miren.core/git_info.repository")
	GitInfoShaId               = entity.Id("dev.miren.core/git_info.sha")
	GitInfoWorkingTreeHashId   = entity.Id("dev.miren.core/git_info.working_tree_hash")
)

type GitInfo struct {
	Author            string `cbor:"author,omitempty" json:"author,omitempty"`
	Branch            string `cbor:"branch,omitempty" json:"branch,omitempty"`
	CommitAuthorEmail string `cbor:"commit_author_email,omitempty" json:"commit_author_email,omitempty"`
	CommitTimestamp   string `cbor:"commit_timestamp,omitempty" json:"commit_timestamp,omitempty"`
	IsDirty           bool   `cbor:"is_dirty,omitempty" json:"is_dirty,omitempty"`
	Message           string `cbor:"message,omitempty" json:"message,omitempty"`
	Repository        string `cbor:"repository,omitempty" json:"repository,omitempty"`
	Sha               string `cbor:"sha,omitempty" json:"sha,omitempty"`
	WorkingTreeHash   string `cbor:"working_tree_hash,omitempty" json:"working_tree_hash,omitempty"`
}

func (o *GitInfo) Decode(e entity.AttrGetter) {
	if a, ok := e.Get(GitInfoAuthorId); ok && a.Value.Kind() == entity.KindString {
		o.Author = a.Value.String()
	}
	if a, ok := e.Get(GitInfoBranchId); ok && a.Value.Kind() == entity.KindString {
		o.Branch = a.Value.String()
	}
	if a, ok := e.Get(GitInfoCommitAuthorEmailId); ok && a.Value.Kind() == entity.KindString {
		o.CommitAuthorEmail = a.Value.String()
	}
	if a, ok := e.Get(GitInfoCommitTimestampId); ok && a.Value.Kind() == entity.KindString {
		o.CommitTimestamp = a.Value.String()
	}
	if a, ok := e.Get(GitInfoIsDirtyId); ok && a.Value.Kind() == entity.KindBool {
		o.IsDirty = a.Value.Bool()
	}
	if a, ok := e.Get(GitInfoMessageId); ok && a.Value.Kind() == entity.KindString {
		o.Message = a.Value.String()
	}
	if a, ok := e.Get(GitInfoRepositoryId); ok && a.Value.Kind() == entity.KindString {
		o.Repository = a.Value.String()
	}
	if a, ok := e.Get(GitInfoShaId); ok && a.Value.Kind() == entity.KindString {
		o.Sha = a.Value.String()
	}
	if a, ok := e.Get(GitInfoWorkingTreeHashId); ok && a.Value.Kind() == entity.KindString {
		o.WorkingTreeHash = a.Value.String()
	}
}

func (o *GitInfo) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.Author) {
		attrs = append(attrs, entity.String(GitInfoAuthorId, o.Author))
	}
	if !entity.Empty(o.Branch) {
		attrs = append(attrs, entity.String(GitInfoBranchId, o.Branch))
	}
	if !entity.Empty(o.CommitAuthorEmail) {
		attrs = append(attrs, entity.String(GitInfoCommitAuthorEmailId, o.CommitAuthorEmail))
	}
	if !entity.Empty(o.CommitTimestamp) {
		attrs = append(attrs, entity.String(GitInfoCommitTimestampId, o.CommitTimestamp))
	}
	attrs = append(attrs, entity.Bool(GitInfoIsDirtyId, o.IsDirty))
	if !entity.Empty(o.Message) {
		attrs = append(attrs, entity.String(GitInfoMessageId, o.Message))
	}
	if !entity.Empty(o.Repository) {
		attrs = append(attrs, entity.String(GitInfoRepositoryId, o.Repository))
	}
	if !entity.Empty(o.Sha) {
		attrs = append(attrs, entity.String(GitInfoShaId, o.Sha))
	}
	if !entity.Empty(o.WorkingTreeHash) {
		attrs = append(attrs, entity.String(GitInfoWorkingTreeHashId, o.WorkingTreeHash))
	}
	return
}

func (o *GitInfo) Empty() bool {
	if !entity.Empty(o.Author) {
		return false
	}
	if !entity.Empty(o.Branch) {
		return false
	}
	if !entity.Empty(o.CommitAuthorEmail) {
		return false
	}
	if !entity.Empty(o.CommitTimestamp) {
		return false
	}
	if !entity.Empty(o.IsDirty) {
		return false
	}
	if !entity.Empty(o.Message) {
		return false
	}
	if !entity.Empty(o.Repository) {
		return false
	}
	if !entity.Empty(o.Sha) {
		return false
	}
	if !entity.Empty(o.WorkingTreeHash) {
		return false
	}
	return true
}

func (o *GitInfo) InitSchema(sb *schema.SchemaBuilder) {
	sb.String("author", "dev.miren.core/git_info.author", schema.Doc("Git commit author"))
	sb.String("branch", "dev.miren.core/git_info.branch", schema.Doc("Git branch name"))
	sb.String("commit_author_email", "dev.miren.core/git_info.commit_author_email", schema.Doc("Git commit author email address"))
	sb.String("commit_timestamp", "dev.miren.core/git_info.commit_timestamp", schema.Doc("Git commit timestamp in RFC3339 format"))
	sb.Bool("is_dirty", "dev.miren.core/git_info.is_dirty", schema.Doc("Whether working tree had uncommitted changes"))
	sb.String("message", "dev.miren.core/git_info.message", schema.Doc("Git commit message"))
	sb.String("repository", "dev.miren.core/git_info.repository", schema.Doc("Git repository remote URL"))
	sb.String("sha", "dev.miren.core/git_info.sha", schema.Doc("Git commit SHA"))
	sb.String("working_tree_hash", "dev.miren.core/git_info.working_tree_hash", schema.Doc("Hash of working tree if dirty"))
}

const (
	MetadataLabelsId  = entity.Id("dev.miren.core/metadata.labels")
	MetadataNameId    = entity.Id("dev.miren.core/metadata.name")
	MetadataProjectId = entity.Id("dev.miren.core/metadata.project")
)

type Metadata struct {
	ID      entity.Id    `json:"id"`
	Labels  types.Labels `cbor:"labels,omitempty" json:"labels,omitempty"`
	Name    string       `cbor:"name,omitempty" json:"name,omitempty"`
	Project entity.Id    `cbor:"project,omitempty" json:"project,omitempty"`
}

func (o *Metadata) Decode(e entity.AttrGetter) {
	o.ID = entity.MustGet(e, entity.DBId).Value.Id()
	for _, a := range e.GetAll(MetadataLabelsId) {
		if a.Value.Kind() == entity.KindLabel {
			o.Labels = append(o.Labels, a.Value.Label())
		}
	}
	if a, ok := e.Get(MetadataNameId); ok && a.Value.Kind() == entity.KindString {
		o.Name = a.Value.String()
	}
	if a, ok := e.Get(MetadataProjectId); ok && a.Value.Kind() == entity.KindId {
		o.Project = a.Value.Id()
	}
}

func (o *Metadata) Is(e entity.AttrGetter) bool {
	return entity.Is(e, KindMetadata)
}

func (o *Metadata) ShortKind() string {
	return "metadata"
}

func (o *Metadata) Kind() entity.Id {
	return KindMetadata
}

func (o *Metadata) EntityId() entity.Id {
	return o.ID
}

func (o *Metadata) Encode() (attrs []entity.Attr) {
	for _, v := range o.Labels {
		attrs = append(attrs, entity.Label(MetadataLabelsId, v.Key, v.Value))
	}
	if !entity.Empty(o.Name) {
		attrs = append(attrs, entity.String(MetadataNameId, o.Name))
	}
	if !entity.Empty(o.Project) {
		attrs = append(attrs, entity.Ref(MetadataProjectId, o.Project))
	}
	attrs = append(attrs, entity.Ref(entity.EntityKind, KindMetadata))
	return
}

func (o *Metadata) Empty() bool {
	if len(o.Labels) != 0 {
		return false
	}
	if !entity.Empty(o.Name) {
		return false
	}
	if !entity.Empty(o.Project) {
		return false
	}
	return true
}

func (o *Metadata) InitSchema(sb *schema.SchemaBuilder) {
	sb.Label("labels", "dev.miren.core/metadata.labels", schema.Doc("Identifying labels for the entity"), schema.Many)
	sb.String("name", "dev.miren.core/metadata.name", schema.Doc("The name of the entity"))
	sb.Ref("project", "dev.miren.core/metadata.project", schema.Doc("A reference to the project the entity belongs to"))
}

const (
	OidcBindingAppId             = entity.Id("dev.miren.core/oidc_binding.app")
	OidcBindingClaimConditionsId = entity.Id("dev.miren.core/oidc_binding.claim_conditions")
	OidcBindingDescriptionId     = entity.Id("dev.miren.core/oidc_binding.description")
	OidcBindingIssuerId          = entity.Id("dev.miren.core/oidc_binding.issuer")
	OidcBindingProviderId        = entity.Id("dev.miren.core/oidc_binding.provider")
	OidcBindingSubjectPatternId  = entity.Id("dev.miren.core/oidc_binding.subject_pattern")
)

type OidcBinding struct {
	ID              entity.Id         `json:"id"`
	App             entity.Id         `cbor:"app,omitempty" json:"app,omitempty"`
	ClaimConditions []ClaimConditions `cbor:"claim_conditions,omitempty" json:"claim_conditions,omitempty"`
	Description     string            `cbor:"description,omitempty" json:"description,omitempty"`
	Issuer          string            `cbor:"issuer,omitempty" json:"issuer,omitempty"`
	Provider        string            `cbor:"provider,omitempty" json:"provider,omitempty"`
	SubjectPattern  string            `cbor:"subject_pattern,omitempty" json:"subject_pattern,omitempty"`
}

func (o *OidcBinding) Decode(e entity.AttrGetter) {
	o.ID = entity.MustGet(e, entity.DBId).Value.Id()
	if a, ok := e.Get(OidcBindingAppId); ok && a.Value.Kind() == entity.KindId {
		o.App = a.Value.Id()
	}
	for _, a := range e.GetAll(OidcBindingClaimConditionsId) {
		if a.Value.Kind() == entity.KindComponent {
			var v ClaimConditions
			v.Decode(a.Value.Component())
			o.ClaimConditions = append(o.ClaimConditions, v)
		}
	}
	if a, ok := e.Get(OidcBindingDescriptionId); ok && a.Value.Kind() == entity.KindString {
		o.Description = a.Value.String()
	}
	if a, ok := e.Get(OidcBindingIssuerId); ok && a.Value.Kind() == entity.KindString {
		o.Issuer = a.Value.String()
	}
	if a, ok := e.Get(OidcBindingProviderId); ok && a.Value.Kind() == entity.KindString {
		o.Provider = a.Value.String()
	}
	if a, ok := e.Get(OidcBindingSubjectPatternId); ok && a.Value.Kind() == entity.KindString {
		o.SubjectPattern = a.Value.String()
	}
}

func (o *OidcBinding) Is(e entity.AttrGetter) bool {
	return entity.Is(e, KindOidcBinding)
}

func (o *OidcBinding) ShortKind() string {
	return "oidc_binding"
}

func (o *OidcBinding) Kind() entity.Id {
	return KindOidcBinding
}

func (o *OidcBinding) EntityId() entity.Id {
	return o.ID
}

func (o *OidcBinding) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.App) {
		attrs = append(attrs, entity.Ref(OidcBindingAppId, o.App))
	}
	for _, v := range o.ClaimConditions {
		attrs = append(attrs, entity.Component(OidcBindingClaimConditionsId, v.Encode()))
	}
	if !entity.Empty(o.Description) {
		attrs = append(attrs, entity.String(OidcBindingDescriptionId, o.Description))
	}
	if !entity.Empty(o.Issuer) {
		attrs = append(attrs, entity.String(OidcBindingIssuerId, o.Issuer))
	}
	if !entity.Empty(o.Provider) {
		attrs = append(attrs, entity.String(OidcBindingProviderId, o.Provider))
	}
	if !entity.Empty(o.SubjectPattern) {
		attrs = append(attrs, entity.String(OidcBindingSubjectPatternId, o.SubjectPattern))
	}
	attrs = append(attrs, entity.Ref(entity.EntityKind, KindOidcBinding))
	return
}

func (o *OidcBinding) Empty() bool {
	if !entity.Empty(o.App) {
		return false
	}
	if len(o.ClaimConditions) != 0 {
		return false
	}
	if !entity.Empty(o.Description) {
		return false
	}
	if !entity.Empty(o.Issuer) {
		return false
	}
	if !entity.Empty(o.Provider) {
		return false
	}
	if !entity.Empty(o.SubjectPattern) {
		return false
	}
	return true
}

func (o *OidcBinding) InitSchema(sb *schema.SchemaBuilder) {
	sb.Ref("app", "dev.miren.core/oidc_binding.app", schema.Doc("The application this OIDC binding is for"), schema.Indexed, schema.Tags("dev.miren.app_ref"))
	sb.Component("claim_conditions", "dev.miren.core/oidc_binding.claim_conditions", schema.Doc("Additional claim conditions that must all match"), schema.Many)
	(&ClaimConditions{}).InitSchema(sb.Builder("oidc_binding.claim_conditions"))
	sb.String("description", "dev.miren.core/oidc_binding.description", schema.Doc("Human-readable description of this binding"))
	sb.String("issuer", "dev.miren.core/oidc_binding.issuer", schema.Doc("The OIDC issuer URL (e.g. https://token.actions.githubusercontent.com)"), schema.Indexed)
	sb.String("provider", "dev.miren.core/oidc_binding.provider", schema.Doc("The OIDC provider type (github, gitlab, generic)"))
	sb.String("subject_pattern", "dev.miren.core/oidc_binding.subject_pattern", schema.Doc("Glob pattern to match the token subject claim (e.g. repo:acme/web-app:*)"))
}

const (
	ClaimConditionsKeyId     = entity.Id("dev.miren.core/claim_conditions.key")
	ClaimConditionsPatternId = entity.Id("dev.miren.core/claim_conditions.pattern")
)

type ClaimConditions struct {
	Key     string `cbor:"key,omitempty" json:"key,omitempty"`
	Pattern string `cbor:"pattern,omitempty" json:"pattern,omitempty"`
}

func (o *ClaimConditions) Decode(e entity.AttrGetter) {
	if a, ok := e.Get(ClaimConditionsKeyId); ok && a.Value.Kind() == entity.KindString {
		o.Key = a.Value.String()
	}
	if a, ok := e.Get(ClaimConditionsPatternId); ok && a.Value.Kind() == entity.KindString {
		o.Pattern = a.Value.String()
	}
}

func (o *ClaimConditions) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.Key) {
		attrs = append(attrs, entity.String(ClaimConditionsKeyId, o.Key))
	}
	if !entity.Empty(o.Pattern) {
		attrs = append(attrs, entity.String(ClaimConditionsPatternId, o.Pattern))
	}
	return
}

func (o *ClaimConditions) Empty() bool {
	if !entity.Empty(o.Key) {
		return false
	}
	if !entity.Empty(o.Pattern) {
		return false
	}
	return true
}

func (o *ClaimConditions) InitSchema(sb *schema.SchemaBuilder) {
	sb.String("key", "dev.miren.core/claim_conditions.key", schema.Doc("The claim name to match (e.g. event_name)"))
	sb.String("pattern", "dev.miren.core/claim_conditions.pattern", schema.Doc("Glob pattern for the claim value (e.g. push,workflow_dispatch)"))
}

const (
	ProjectOwnerId = entity.Id("dev.miren.core/project.owner")
)

type Project struct {
	ID    entity.Id `json:"id"`
	Owner string    `cbor:"owner,omitempty" json:"owner,omitempty"`
}

func (o *Project) Decode(e entity.AttrGetter) {
	o.ID = entity.MustGet(e, entity.DBId).Value.Id()
	if a, ok := e.Get(ProjectOwnerId); ok && a.Value.Kind() == entity.KindString {
		o.Owner = a.Value.String()
	}
}

func (o *Project) Is(e entity.AttrGetter) bool {
	return entity.Is(e, KindProject)
}

func (o *Project) ShortKind() string {
	return "project"
}

func (o *Project) Kind() entity.Id {
	return KindProject
}

func (o *Project) EntityId() entity.Id {
	return o.ID
}

func (o *Project) Encode() (attrs []entity.Attr) {
	if !entity.Empty(o.Owner) {
		attrs = append(attrs, entity.String(ProjectOwnerId, o.Owner))
	}
	attrs = append(attrs, entity.Ref(entity.EntityKind, KindProject))
	return
}

func (o *Project) Empty() bool {
	return entity.Empty(o.Owner)
}

func (o *Project) InitSchema(sb *schema.SchemaBuilder) {
	sb.String("owner", "dev.miren.core/project.owner", schema.Doc("The email address of the project owner"))
}

var (
	KindApp           = entity.Id("dev.miren.core/kind.app")
	KindAppVersion    = entity.Id("dev.miren.core/kind.app_version")
	KindArtifact      = entity.Id("dev.miren.core/kind.artifact")
	KindConfigVersion = entity.Id("dev.miren.core/kind.config_version")
	KindDeployment    = entity.Id("dev.miren.core/kind.deployment")
	KindMetadata      = entity.Id("dev.miren.core/kind.metadata")
	KindOidcBinding   = entity.Id("dev.miren.core/kind.oidc_binding")
	KindProject       = entity.Id("dev.miren.core/kind.project")
	Schema            = entity.Id("dev.miren.core/schema.v1alpha")
)

func init() {
	schema.Register("dev.miren.core", "v1alpha", func(sb *schema.SchemaBuilder) {
		(&ConfigSpec{}).InitSchema(sb)
		(&App{}).InitSchema(sb)
		(&AppVersion{}).InitSchema(sb)
		(&Artifact{}).InitSchema(sb)
		(&ConfigVersion{}).InitSchema(sb)
		(&Deployment{}).InitSchema(sb)
		(&Metadata{}).InitSchema(sb)
		(&OidcBinding{}).InitSchema(sb)
		(&Project{}).InitSchema(sb)
	})
	schema.RegisterEncodedSchema("dev.miren.core", "v1alpha", []byte("\x1f\x8b\b\x00\x00\x00\x00\x00\x00\xff\xac[I\x93\xec8\x11\xfe\x1bl\xc32\xec\x03x\xe61\x100,\x13\xc0\x85\b.\xfc\x04\x87\xca\xca*\xab˖\xfc$\xb9\xba\x9b\x1b{\x10\xf0+x\xfd\b\xfe \x9c'\xacͲ,˒\xeb]:\xa4\xb4\xf3S:7\xa5\x94]/\x98\xa2\x1e(\x86[\xd5\x13\x0e\xb4j\x18\a\xb8\x12\x8a\xc5\x7f\x1f\x97\xd4\x0f'j\x85\x86\xe1?\x8a\x87\aO\xd10h\xbe\xff\x9f1\xeb\x11\xa1\x01\xe8\xf9L\xa0\xc3\xe2\xcfoN\x04?}m\xcd\\\xa1F\x92\x1b\xd47\xe0\x820\xaa\xe5\nh\xf2y\x80\x13\xc1\x9b\x10\x84\x12IPW7\x8c\x9e\xc9EC\x044\x1f\xe2\v\x11\x88\x81\xb3\ah\xa4\xe2\xbd؉a\xba\x189.\xb7W\xa8\x1bZ\xd4\r\x9c\xf4\x88?\xd7\xd3w7h\x18\x9e\xbe\x18S\x99A\xd1j\xbb\x05o\x98\x879\xaa\xfb\xa3\x12\xfaKq\x80\x8a=R\xe0j\t\xd0\xc3I賐\x9c\xd0KRp\xfb\x95+dmo.\xc9\x19Y\xe9C\x97\xb0Os\xc4\xff\x8b\x12?ԐE\x98\x1cK-1\xe9qa\xa5\xafnq\xf4\x88\x923\bm\xab\xd6ͼ\xefV\xfc\xdf\xde\xe3\xaf1\xb9X\x18\x16\x12=\xb4\x97\t\xed\xcb[hB\"9\n\x05r6\xe3\x89\x17\x03\x1d\xfb\xeb\xf4\xa7\xbe\xa1n\x04\xf1\xef\xb3vꕺ5\x93\t\x83\x16\xf1\xa6%7X/h_3ϓ\xa6m\xadtq\xdb\xf6 \x11F\x12\xc5mk\x9ffEuT7\x16\xa1\xea\xd0\t:\x81{D\x9f\xff\xa75d(\x93\x86@\x8d\xa3\xbe\xed\x00&\x1e<\xff\tM\xfc\x95-\xbe\xc3\xd1\xdcZ\x88\xd5G)\xcda\x18:\xf6\xdc\x035q\xf1\xf4\xf9\xe0\xad\xf9\x85\x1c\xf5\xfdS}\xc5\xfb\x9b\x18Sp\xd4\xee\xf3[7\v\xf5\xf0\xcd4\x82\x9fZ\xaf>!\xc4\xf9\xc66\xcei$\x1d\xae;vѮ\xfe\xe0\xcd\vP\x9an\x14\x12xM\xb0F\xf1\xe6!ʷ\x12(\xac\x1f:\x90\x80k\xa4M\xdc-(a\xe8&\xb4\xa3\x87\x80\xebӳ֎O\x98pȄ\xcc(P9\x8f\x8c\xe9\x03\xd8*\x0e\x9b\x9f!\xe3jS \x95$=\b\x89z\x9d*\xc9<\xcd\xf3\x04\r2\n\xe05\xf4\x88tZ\xf9\xde<\x84\x89\xbb\xa4\ac\fx\xb1\x93<\x1f\xf0\x00\x9cW\x93y\x9a\xbbs=X\xb4\xd3s4\xd3{\x96\x00\xce\x19\xaf{\x10\x02]\xf4z\xfd\x92\x14:K\"\x18/Dք\x9e\x99\x0eF7\xdbq\x93\xf7\xb7\xdd\xc4B\xe4\xf8\xc8?\xde\xc42\xadE\xa8\xd0([\xa6ˀ\xb3\x19\x87&\xd9\xe4=qD\x9bV\xf3\x9aq\xc8\xfb\xbd-ކ\xf5=\x91\xb5^\xd2s.\x11{\x10\xa2~g\au\xe9\xf5Ê\x1a\xe2\x85\x15\x83\xc3#\xa2ƄK\x1d㭛\xa9}\xfa\xc4X\x17\xddL\x1c\xb7\xef=\x97\x88\xdfD#\xc6qs\x18\x98 \x92q\xbd\xfa\x837\x0f1\xc2\x1a\xc9a\x88\x16\xe9\x1ai\x1a\x84\\\xdf\xdd\xe2zd\xfcJ襖\x1c\xa0n\x91\xd0&~\xbd&gW\x8c\x17\"'䨺<\xbf\x1eZ$\xb4\xba@\x0fC\x91\xabm^\xc1F\xde@=Sl\xaa\x91\xd1'{.\xe0#\xc7\v\xb5\x82\x843\xc1\xac\xbe۞\x8e\xec\xa6j\xc2>rưo\xe4\x84\xfb\xbf\xa2{\xa0\aR!\xdc\x13ZKv\x05\xbb\xb1{\x84\xbd\xd8_\x00m\x15\xe0_O1\x99\x02\xd3\x14&vf\xd8_6\x0ej\x8e\xdd;\xa8\x9d\xbd\x03Z\"\x8d\x06h\xd5\x1a-G\xad\x7f{\x1bӆ\xe6WY\aQ\xec\u05eb\xad\xa3\xed\x88\xf7\xc1\xaex\x0e>G\xce?EC\xcc\"X(\x9d\x91\xecd\xaf8v\xdc\x02\xf8\x8d4&\x9f\xd9In(8\x8dD\xc3\xcd|*Pɟ\aF\xa8\xf6\x8f\ao\x1eJ\x19ƉA\x18\x18\u05fcX\x8d&\xae\x86P\x992\x9f\xf9\x92\x85\xf9\x1c\xed~\xf3Y\xa8\xac\xcdZ\xc9\xf9^x\x823\b\x15&\xe2\xea\x8b\t\x9a\xb0#\xe3G\xf92\xea\x15r$\xfd{<\x97O\xecՙt \x9e\x85\x84^[ћ\xef\u058b\n\xa0\x03$@\xed\xd7l\xd4\xd6엤=\x97\xd50=\x1b\xa9\xac\a$\xf5\x06\xf6\xe0\xcdC\x80\xd5qL\x01l\x9f\"_bv\xd2L\x03g7\x82\xcd\xd5J\xebf\xf1\xb3=t\xacA\xdd\n\xc9rU\xea1\xa8'\xdb/)Z4]k\x898 \\3\xda\xe9B\x82\xcc\xd3e\x1d\x13\x1e\xa65\xb3 \x7f\x80\xfar2Ao&6\xac\x92\x11\xaf\xbd\xf3m\xac@q\xfe\x06\xf4\xe6\xf9s3Mw\xbc\xb9*\xf0f\xa0\xb7|_\x0eu\a\xf4Va\x10\r'\x83t\x87`\x9f\x10\xf8PxQ7\xf1_A뼙\x06{>710N.D\xafu6\xe3\xbdbob\xe3\xf0z$\x1ctbo\xdd,m߉Q\x00\x15D\x92\x9b9R\xcd\xd3%kLT]V\x99\xb2H\x8fCQ?\x17aS\x8e\xaf\xb3\x97\x1e\xe6n \x93s\xbcI\xa6G\xd2\xdbj\x1b\xf40\x94gu\x83f9w\xae\x8b6\xf96v\x9b\xe8}\xf3\x82\xc9;\xc6\xce\xd3p\xd94\x82\xb2Ќ\xa0\xa6\x1eBz'\x99X\x16;\x89&\xbcÝD\x01\xe6D\xdf_\xa3\x1e\xa6\xd8\xf7\xec\x12\x86\xacab\x18jg\x192O\x17\xe6\x89/\xb8a\xd0h\xba7\x1c\x9cIְΥ{=\x8b\xa7\xfbF6\xeb;\x7f\xcbS\xc9fhF\x9cxa\xc4CBv\xe7\x108\xf4\x85t\x9aV\xdc/\xb1Ӻ3\xa6\x19\xd4\r\xa3\xcd\xc89\xd0F\xe75\x11{\xb0\xe3D\x9f\x168Q\x04>ߥ\xc2\u06dd\bX\xd53lt\xa6F\xa1\x83}\x94\x011\x99\x97P!\x11\x9d\xeaEU\xac,I\v\xb7\xfbi\x06┿AHQ\x0f\xc0\x1d\x8eB\x1e\xe3\x8f\x16+|\x9c\xb1\x82\x98\xaa\x8a\x1a\xb3GZc\xe8\x906氢\x86\xeaȂnG\xa9 \xfc\xf2mXQs\xbd\x93\x9b5\xbc%\xd2\xc7\f\xeb;\xd1[M\xeb_\x12qYc¡q\xf7+,$\x86\xb9t\xe3\xf4pC\x9c\xa0S\a\xfe\xe9\xc1\xd1\xee?=X\xa8\xfc:&<}[\x84\xa2bf\x95\x83\x1cJ\xb2\xa4\tu\xe4\xb82\xea\x9a\xf0H\xe8x\xb3\x8a\x9bp\xaftܙ\x15Φ\xe4\x19eN\xb818\xdeõ\x8e\xf3\xa0\xe4[\xe6\x06$z\x99\xb7v\xa5e\xa3<\xa0\xf9\xd77\x1f&\xa0`h\xa1\a\x8e\xba\x1a\x9e\x06\xc2A\xd8N\x8a\x8c>\xd1\x1b\x11\xe9A\x01\x7f\x90\x05\xac\xba{:$C\xe2\xde=p\x1cP\xca\xce\\\xe0/H{\x9d\a\x1fL\x95\x93\xf5\xc85\x10\x99\xa7!H\xea\xea+\xb3\xfd\x9c\xd2Ri\a:z\xc6\xf6\x01}\x9f\xb8D\xba{I\xff\xf3[\x82\xab\xf0Uכ\x8c\xe0\xa6>\x11\x8a\t\xbd\x98\x9c\x17&\x16\xff\x95\xac\x9b\xb8\xe8\x81\xdfG\x89^L\xaa\x14\xfe\xfd\x14W\xd3!\xd2O[\x19&SF\xf4\xab\xe3a\xf5l'\xb1\a\vUɅ\xf2\xef\xf5B\xef\n\x91\xd2\xc99\xac\x87V\xdc\x03\x92\x12\xb8q\x06;\xc9u\x06\xa6\xe0f\xb4\xe8\x92\v=\x14\xedG\xa1\x13/\x90\x88\x10\xa3\xb9p9\x9b\xf1^t/\xf8\x13W6\xa9f\xd6\x02C\x8c\xa7\ah\xd4\xf5\x92S\"\v\x89\xb9\xca\xec|\xe8շ\xab\xd0Z&p\xe3w\xe1N\xb4|)\xdf\xd1\xc2\xddt\x89\x13\r\xb0h\x134\xe0\x13\x034\xba\xdaV\xa3\x9d \nM\xe6\x9e\xdbo\x9f@\xf2\xdb\xe4\xe1\xb6\x16\x85˼\x81V\xd9\xe4\aY\x80\xf7\xdc.\a+T\xe9\x15\xf2\xffq\xe4\xc7E\x92\xefv\r\x94\xe9?)\xc5\\\x9e$\xaf\x05'\xc8O\x8a\xd4R\x1d:<~z\xf8s\xf6Δ\xbf=\x8e\\v\xd4\xfc\xfd\xf1\x85\xee;\x81\xfe\xee\xf8\xc2\a\x0f\xa6\xf7\xac\xf8nϫO\xef\xe9\x15\xa7\x05\xedz\xderoc\xc7\xe8\x1di\x8fu}>.\v\x92\xc2\xc6Oax\x14\xf7\x85~}\x04\xbf\xb8mt\xe8+\n\xbaJ\xe1eO\x16\xfeN\xd3\xe9\x17G0\x8f\xf4\xa4~s\xcfB\x8b\xc6\xd5}Hsw\xeb\x97Gp2\x9b_??\x82}\xbc7\xf6\xb8\xce\x12s\xb7\xecU\x99,\xe5=\xb4We\xb9\xa1\xa8\x8dVh\xa4\xd2.[i\xed\xb2ۅ+\x8c\xd1\xcc&\xdd\xcf\xcaQ\xb3\xae\xb9\nݴ\xa0\xc5w@\x0f\x19Wc?)G=|i6\xaecʶ\f\vw\xdc\xddF\xe2\x0f\xcb\xf0v\xdaX\x85h\xa9\xa6c\xa1\xbe\xb3[\x91\x85\xfe\xac;\x92\xdeF\xdc-(wzIv\xfb\xb3\xd0\xecǚ\xa2\x85\x85Va_\xb40,\xb3ڦ\x85)\xba\xa4\xabzH\xdcTӵ\xb0\xdc9ړ\xfd\xd5=˸\xc6\xed}(\xb6\xbb{H\x87\a\x9b\xbf\x91:D\xe1\xa5{rk&%\xf8\x8f\xf2\x04?\xd2|\v\xff\xf17\x0em\xbb&~\x10\x93\x99X\xf8\x9fE;k\xe4\x97Dy\xc9\xd3\x01\x97\xd5Cy\x9b\xc7\f\x9e,\x86\xf2\x8a\xab\x19,\xa3\x12\xca\xcb\xed3dV\x19\x94\x17\"3hf\rT\xfa\xf9\x19\x05P^L̐\x87\xab\x1f1\a\xa5CKwo<\x01\x92/\x06\xbd\xc2\xf0ݫh\xa7\rX\xff\x04\xb8Að\xf53`\xf7\xbb\xd1ԏ^w~\x81h\x9f\xce?\xb7K\xfeP\xd1\xff\xff\xfb\x9d\xdf\xe5-\x9aY{\xff\xab\xbf\xbc\x9f\xdfm}\x05\x1a̹\xd0\xff\f\x00\x00\xff\xff\x01\x00\x00\xff\xff'\xb6\xd29h=\x00\x00"))
}
