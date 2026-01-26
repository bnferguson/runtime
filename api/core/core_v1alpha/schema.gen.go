package core_v1alpha

import (
	entity "miren.dev/runtime/pkg/entity"
	schema "miren.dev/runtime/pkg/entity/schema"
	types "miren.dev/runtime/pkg/entity/types"
)

const (
	AppActiveVersionId = entity.Id("dev.miren.core/app.active_version")
	AppProjectId       = entity.Id("dev.miren.core/app.project")
)

type App struct {
	ID            entity.Id `json:"id"`
	ActiveVersion entity.Id `cbor:"active_version,omitempty" json:"active_version,omitempty"`
	Project       entity.Id `cbor:"project,omitempty" json:"project,omitempty"`
}

func (o *App) Decode(e entity.AttrGetter) {
	o.ID = entity.MustGet(e, entity.DBId).Value.Id()
	if a, ok := e.Get(AppActiveVersionId); ok && a.Value.Kind() == entity.KindId {
		o.ActiveVersion = a.Value.Id()
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
	if !entity.Empty(o.Project) {
		return false
	}
	return true
}

func (o *App) InitSchema(sb *schema.SchemaBuilder) {
	sb.Ref("active_version", "dev.miren.core/app.active_version", schema.Doc("The version of the project that should be used"))
	sb.Ref("project", "dev.miren.core/app.project", schema.Doc("The project that the app belongs to"))
}

const (
	AppVersionAppId            = entity.Id("dev.miren.core/app_version.app")
	AppVersionArtifactId       = entity.Id("dev.miren.core/app_version.artifact")
	AppVersionConfigId         = entity.Id("dev.miren.core/app_version.config")
	AppVersionImageUrlId       = entity.Id("dev.miren.core/app_version.image_url")
	AppVersionManifestId       = entity.Id("dev.miren.core/app_version.manifest")
	AppVersionManifestDigestId = entity.Id("dev.miren.core/app_version.manifest_digest")
	AppVersionVersionId        = entity.Id("dev.miren.core/app_version.version")
)

type AppVersion struct {
	ID             entity.Id `json:"id"`
	App            entity.Id `cbor:"app,omitempty" json:"app,omitempty"`
	Artifact       entity.Id `cbor:"artifact,omitempty" json:"artifact,omitempty"`
	Config         Config    `cbor:"config,omitempty" json:"config,omitempty"`
	ImageUrl       string    `cbor:"image_url,omitempty" json:"image_url,omitempty"`
	Manifest       string    `cbor:"manifest,omitempty" json:"manifest,omitempty"`
	ManifestDigest string    `cbor:"manifest_digest,omitempty" json:"manifest_digest,omitempty"`
	Version        string    `cbor:"version,omitempty" json:"version,omitempty"`
}

func (o *AppVersion) Decode(e entity.AttrGetter) {
	o.ID = entity.MustGet(e, entity.DBId).Value.Id()
	if a, ok := e.Get(AppVersionAppId); ok && a.Value.Kind() == entity.KindId {
		o.App = a.Value.Id()
	}
	if a, ok := e.Get(AppVersionArtifactId); ok && a.Value.Kind() == entity.KindId {
		o.Artifact = a.Value.Id()
	}
	if a, ok := e.Get(AppVersionConfigId); ok && a.Value.Kind() == entity.KindComponent {
		o.Config.Decode(a.Value.Component())
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
	if !entity.Empty(o.App) {
		attrs = append(attrs, entity.Ref(AppVersionAppId, o.App))
	}
	if !entity.Empty(o.Artifact) {
		attrs = append(attrs, entity.Ref(AppVersionArtifactId, o.Artifact))
	}
	if !o.Config.Empty() {
		attrs = append(attrs, entity.Component(AppVersionConfigId, o.Config.Encode()))
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
	if !entity.Empty(o.App) {
		return false
	}
	if !entity.Empty(o.Artifact) {
		return false
	}
	if !o.Config.Empty() {
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
	sb.Ref("app", "dev.miren.core/app_version.app", schema.Doc("The application the version is for"), schema.Indexed, schema.Tags("dev.miren.app_ref"))
	sb.Ref("artifact", "dev.miren.core/app_version.artifact", schema.Doc("The artifact to deploy for the version"))
	sb.Component("config", "dev.miren.core/app_version.config", schema.Doc("The configuration of the version"))
	(&Config{}).InitSchema(sb.Builder("app_version.config"))
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
	sb.String("start_directory", "dev.miren.core/config.start_directory", schema.Doc("Directory to start the process in (defaults to /app)"))
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
	sb.Component("service_concurrency", "dev.miren.core/services.service_concurrency", schema.Doc("Concurrency configuration for this service"))
	(&ServiceConcurrency{}).InitSchema(sb.Builder("services.service_concurrency"))
}

const (
	DisksFilesystemId   = entity.Id("dev.miren.core/disks.filesystem")
	DisksLeaseTimeoutId = entity.Id("dev.miren.core/disks.lease_timeout")
	DisksMountPathId    = entity.Id("dev.miren.core/disks.mount_path")
	DisksNameId         = entity.Id("dev.miren.core/disks.name")
	DisksReadOnlyId     = entity.Id("dev.miren.core/disks.read_only")
	DisksSizeGbId       = entity.Id("dev.miren.core/disks.size_gb")
)

type Disks struct {
	Filesystem   string `cbor:"filesystem,omitempty" json:"filesystem,omitempty"`
	LeaseTimeout string `cbor:"lease_timeout,omitempty" json:"lease_timeout,omitempty"`
	MountPath    string `cbor:"mount_path,omitempty" json:"mount_path,omitempty"`
	Name         string `cbor:"name,omitempty" json:"name,omitempty"`
	ReadOnly     bool   `cbor:"read_only,omitempty" json:"read_only,omitempty"`
	SizeGb       int64  `cbor:"size_gb,omitempty" json:"size_gb,omitempty"`
}

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
	sb.String("name", "dev.miren.core/disks.name", schema.Doc("The name of the disk entity to attach"))
	sb.Bool("read_only", "dev.miren.core/disks.read_only", schema.Doc("Whether to mount the disk as read-only"))
	sb.Int64("size_gb", "dev.miren.core/disks.size_gb", schema.Doc("Size in GB for auto-creating the disk if it doesn't exist"))
}

const (
	EnvKeyId       = entity.Id("dev.miren.core/env.key")
	EnvSensitiveId = entity.Id("dev.miren.core/env.sensitive")
	EnvSourceId    = entity.Id("dev.miren.core/env.source")
	EnvValueId     = entity.Id("dev.miren.core/env.value")
)

type Env struct {
	Key       string `cbor:"key,omitempty" json:"key,omitempty"`
	Sensitive bool   `cbor:"sensitive,omitempty" json:"sensitive,omitempty"`
	Source    string `cbor:"source,omitempty" json:"source,omitempty"`
	Value     string `cbor:"value,omitempty" json:"value,omitempty"`
}

func (o *Env) Decode(e entity.AttrGetter) {
	if a, ok := e.Get(EnvKeyId); ok && a.Value.Kind() == entity.KindString {
		o.Key = a.Value.String()
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
	if !entity.Empty(o.Key) {
		attrs = append(attrs, entity.String(EnvKeyId, o.Key))
	}
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
	if !entity.Empty(o.Key) {
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
	sb.String("key", "dev.miren.core/env.key", schema.Doc("The name of the variable"))
	sb.Bool("sensitive", "dev.miren.core/env.sensitive", schema.Doc("Whether or not the value is sensitive"))
	sb.String("source", "dev.miren.core/env.source", schema.Doc("The source of the variable (config or manual). Defaults to config for backward compatibility."))
	sb.String("value", "dev.miren.core/env.value", schema.Doc("The value of the variable"))
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
	VariableKeyId       = entity.Id("dev.miren.core/variable.key")
	VariableSensitiveId = entity.Id("dev.miren.core/variable.sensitive")
	VariableSourceId    = entity.Id("dev.miren.core/variable.source")
	VariableValueId     = entity.Id("dev.miren.core/variable.value")
)

type Variable struct {
	Key       string `cbor:"key,omitempty" json:"key,omitempty"`
	Sensitive bool   `cbor:"sensitive,omitempty" json:"sensitive,omitempty"`
	Source    string `cbor:"source,omitempty" json:"source,omitempty"`
	Value     string `cbor:"value,omitempty" json:"value,omitempty"`
}

func (o *Variable) Decode(e entity.AttrGetter) {
	if a, ok := e.Get(VariableKeyId); ok && a.Value.Kind() == entity.KindString {
		o.Key = a.Value.String()
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
	if !entity.Empty(o.Key) {
		attrs = append(attrs, entity.String(VariableKeyId, o.Key))
	}
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
	if !entity.Empty(o.Key) {
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
	sb.String("key", "dev.miren.core/variable.key", schema.Doc("The name of the variable"))
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
	DeploymentAppNameId      = entity.Id("dev.miren.core/deployment.app_name")
	DeploymentAppVersionId   = entity.Id("dev.miren.core/deployment.app_version")
	DeploymentBuildLogsId    = entity.Id("dev.miren.core/deployment.build_logs")
	DeploymentClusterIdId    = entity.Id("dev.miren.core/deployment.cluster_id")
	DeploymentCompletedAtId  = entity.Id("dev.miren.core/deployment.completed_at")
	DeploymentDeployedById   = entity.Id("dev.miren.core/deployment.deployed_by")
	DeploymentErrorMessageId = entity.Id("dev.miren.core/deployment.error_message")
	DeploymentGitInfoId      = entity.Id("dev.miren.core/deployment.git_info")
	DeploymentPhaseId        = entity.Id("dev.miren.core/deployment.phase")
	DeploymentStatusId       = entity.Id("dev.miren.core/deployment.status")
)

type Deployment struct {
	ID           entity.Id  `json:"id"`
	AppName      string     `cbor:"app_name,omitempty" json:"app_name,omitempty"`
	AppVersion   string     `cbor:"app_version,omitempty" json:"app_version,omitempty"`
	BuildLogs    string     `cbor:"build_logs,omitempty" json:"build_logs,omitempty"`
	ClusterId    string     `cbor:"cluster_id,omitempty" json:"cluster_id,omitempty"`
	CompletedAt  string     `cbor:"completed_at,omitempty" json:"completed_at,omitempty"`
	DeployedBy   DeployedBy `cbor:"deployed_by,omitempty" json:"deployed_by,omitempty"`
	ErrorMessage string     `cbor:"error_message,omitempty" json:"error_message,omitempty"`
	GitInfo      GitInfo    `cbor:"git_info,omitempty" json:"git_info,omitempty"`
	Phase        string     `cbor:"phase,omitempty" json:"phase,omitempty"`
	Status       string     `cbor:"status,omitempty" json:"status,omitempty"`
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
	if !entity.Empty(o.Owner) {
		return false
	}
	return true
}

func (o *Project) InitSchema(sb *schema.SchemaBuilder) {
	sb.String("owner", "dev.miren.core/project.owner", schema.Doc("The email address of the project owner"))
}

var (
	KindApp        = entity.Id("dev.miren.core/kind.app")
	KindAppVersion = entity.Id("dev.miren.core/kind.app_version")
	KindArtifact   = entity.Id("dev.miren.core/kind.artifact")
	KindDeployment = entity.Id("dev.miren.core/kind.deployment")
	KindMetadata   = entity.Id("dev.miren.core/kind.metadata")
	KindProject    = entity.Id("dev.miren.core/kind.project")
	Schema         = entity.Id("dev.miren.core/schema.v1alpha")
)

func init() {
	schema.Register("dev.miren.core", "v1alpha", func(sb *schema.SchemaBuilder) {
		(&App{}).InitSchema(sb)
		(&AppVersion{}).InitSchema(sb)
		(&Artifact{}).InitSchema(sb)
		(&Deployment{}).InitSchema(sb)
		(&Metadata{}).InitSchema(sb)
		(&Project{}).InitSchema(sb)
	})
	schema.RegisterEncodedSchema("dev.miren.core", "v1alpha", []byte("\x1f\x8b\b\x00\x00\x00\x00\x00\x00\xff\xa4YI\xb2\xe4&\x10\xbd\x86\xe7\xa1=\xb7#\xe4vx㕯\xa2\xa0DJ\xe2\x97\x04j@\xfa]\xdey\x1e\x8e\xe1\x1enh\xaf\x1d\x90@!\nI\xfc\xeeM\x05\x99\xd4{@\x0e@\xa2\x17\x94\x93\x118\x85\xa5\x1a\x99\x04^5B\x02\x9c\x19\xa7\xea\xd5\xfdZ\xfb\x8d\xd1Vd\x9a^Y\x8cLz\xc94!\uefd6\x8a\x910\x9e\x90\xb6-\x83\x81\xaa\x9f\x9f\x9f\x18}\xf6\xf1-\xb8\"\x8df\v\xd4\vH\xc5\x04\xc7y%:}\x99\xe0Ĩ\xa5x'C1Iq\a\x8d\xb6\xd8\xce\v\x0e\xd49\x92n\xf9\x96\fSO\x86I\xb2\x91\xc8Km&ݐiz\xf6nn\xbd\x8e\x05\u05fc$\xffp\x9d%\xeb\xfe\xc9N\xfa\xbd<A%\xee9H;\x04`\xd3L\xbaUZ2\xde\xedNܯ\xf2\x86\x19\x9d%5k\x89\x9f}\xeaO\xdf[2\xfd_\xed\xf4S\vy\x06\x13\x15v\bcǕ\x97>\xdaB\x8c\x84\xb3\x16\x14\xfa\xaa\x0fR\xb4n\x8b\xff\xe2\b_S\xd6y\x1a\x91*#\xb6\x17\x86\xed\x83-6\xa5\x89\x9e\x95%i]\xdb`)\xf0y<\x9b\x9fz!\xc3\f\xea\x9f\x16#\xf2\xc6\xdc\br1\xdc\x13\xd9\xf4l\x81\xdb\x01\xfd\xdf\\\xff\xaek{?\xbb\xbcoGЄ\x12M\xf2\xbe\xf5\xbd%\xbe\xfd%k\x1b\xcfP\r\xe4\x04\x83\xa2#\xe1\x97\x7f\xd1BNc,\x04\xb6\x9d\x8d\xed@`0\xf4\xfa\x93\xba\xf8\xc3-\xdckgs\xef)n\x16e-Ga\x1a\xc4e\x04\xee\xf2\xe2\xd9\xdbɿ\xae\x7f(1\xdf\xdfv\x15\x8f69Lr\xd4a\xf9}\x90R;|\xb6\xcf\x10\xef\x8b\xe7X\x91\xf2|\xba\xcds\x9a\xd9@\xebAt\x18\xeaw\x91\xfc\x00\x96f\x98\x95\x06Y3\x8a,\x91\x9c\xb2|\xbe\xc3\"\xc6i\x00\r\xb4&\xe8\xe2a\xa5ISw\xc7:\xd8\x04Z\x9f.h\x9dXax\x98a\x16\x1c\xb8\xbe\xb6\x9c\xeb\x13\xda*O[\xbeC\xe6\xcdfI*\xcdFP\x9a\x8c\xb8U\xb2\xabX\x16\tH2+\x905\x8c\x84\rh\xfcHNi\xf2!\x19\xd18\av^(\x8b\x81\x88 D5\xbb\x8a\xa5'םg;]\xb2;}\xe4\t\x90R\xc8z\x04\xa5H\x87\xe3\x8dkU\x1a,;\xc9\xd81]3\xde\nL\xc6 \x1d\x84ɣ\xed0\xf1\x14%1\xf2\xd7\xf3\xdcN\xeb\x19*2\xeb^\xe05\xa0u\xed\xd4%\x9bؓ$\xbc\xe9\x11\xeb\xda)\xf6\xeb-l#Ƒ\xe9\x1a\x87\x8c\x82K\xe5:R\xd6/\x0fX\xd7Q?\xddhS\xbe\xf4\xc6\x10\xf8\x98\xaa)\x93\x1as\xbc\x0f\x92=\xa7OB\f\xd9\xc3$\xa0\xe3\xe8\xe92q\x93͘\x80\x960\tŴ\x908\xfa]$\xa7\x1c\xe9\x1d)p\xa8\x9e\xe0\x1d\xc94R\xd4W[\xa8{!όw\xb5\x96\x00uO\x14\xba\xf8魺\xf8\xc6\xd81m\x98\xb3\xe6\x8a\xe2z\xea\x89Bs\x016\x8f\x1c\x15a7\xaeS\x0f\xd8\x16\f\xcd\xcd\xec|\x01\xe2\x8f>\x97\x9c\x99J\xc0\xff\xa3$)\xff\xc8&VD\xb2}\xbb\xfdd\x0f\xe4no\xee\xd4\xf7\x92\x83\xbfب\x82\x02\xbc\x11\xbce\x1d\x9aе\x0f\xf6\xa8\x84\xad\xbae+\xb1\xc6\xef/s\xd6@\xbcMi\xc2i|\x19\xec\x83\xee`z\x8f\x0f\xa7\x17\xe8\xcb\xeb\xc84B<\x83\xa7\xc2t\xf7\xc2\xd1\xcd3\xa0\x15ȅ5n\xb3\xf0Bi\x04\a\x8bd\xb3\xc4-\x15\xb8\x96\x97I0\x8e\xf1q\x17\xc9\xe9,\xd3\xf0v\f\x93\x90\x88\xa5\xb6eP\r\xe3z\xcf}n%+\xf7\x05ݛ\xbb\xcfS\x95\xb8\xefO;\xcf\xf7\xd3\xf2\xc81T\x94\xa9s<M@\xc5\xc1\x1c\x9f\x94\xcf\x11G(J\x88\xfcFi\xe0U\xcb\x06P\x17\xa5aD/F\xf2\xe1e\xcc\x12\f@\x14\xd8\xc3P\xcc\xe8\xcdq\xad:\nY\xa4\x19\xc5\xccu=\x11\x8d\xa7\xc3]$\xa7\x047\xb5\x8e%8(\xd1\xd2xB\x90\x04Bk\xc1\a<\x13\xd9U\\\x1f\xc9i]\x88`\xc5~\x84\xba;\xb9\x14s\x82\x0f\xe2\xdd\xfc\xc2Xx\x99;k\x83w\x81/Q\xf44F<\x88\x9d\xea\x01\xb1\x03|)\xae\b\xd2'#\xe0Ku\x064Yc\x1a\xa9\xadSs\x19\x80\x02\xae\x98f\x8b\xbbl_ŵ\xa5S\xdfZ\xa8\x98\xa5\xdb\xc9Z\xd7NG|+\x03\xb3\xcf\x1d\x98z\xd8,\xdd\xfd\x8c\xad\x9f\xef\xe66\x1b\xfd=\f\xb0yd\x81\x80<\x88\xd2M\xdc\xc6V\x99}\x89\\\x81\xa2\x02\xe7*\xa6\xc3\xee3X\x0f]\x19\xac\x98\x96+\xe9\xc5<0\xb8F\xdd\b\xde\xccR\x02o0pT\xae\xe3 \xc0\x7fx@\x80g\xe8K\x02\xfe\xb7l!\x97!\xabFA\x9d\x13m+5\xe9\x93\x02\n>\x8f5\xe3J\x13nN/\xbbu\xaeU+7\x7f_\xc0(\xe1\xe9\fJ\xabz2\xe5\xb0\xe3\xb1\xccs\xbek5\xc2w\x05#\xa8\x86\fPSq\xcfk\n\x03AgN7\xda\xd4\x1cE\xd4\xfd\xac-E|\x98L7\xda\xd24\x96n\x8ch\x88\xfdK\x8f\x8f\x9d\xec\x03\x86\x8f/M\xa46e\x1b4\xa1\x94\x12\xa92\x9a\xe1\xde]f!\x92\x91\xd3\x00\xf1]&\xe8\xde\xfc.\xe3\xa9^\xffm\xdc3\xec\xef\xf6\xe9\xe6\x11P\x85[~j\x9d+\xfex\xdfOw\xe8\x80}\xed\xcd?x`\xf7_\xae\x9e\xc9>1Ů\xb0\xa7C=K|\x8c`W1]\xc8^\x19V\xf8\x9d\xe1q\x01E駆\xec}/&\x8c\x9fr\xbb\xcc3\xee\xae\xf5\xe2\xb7\xdf\xf4\x8fg՛C\x06?ߙju\xeb\x13^\xf8l\xb4\xf7\xcd\xeb\xe0\x03\x84ｾ\xb6\xef~\xa7\x88\v\xfb\x83g\xf9x\x89\x87\x8f\x00\xff\x03\x00\x00\xff\xff\x01\x00\x00\xff\xff~Sװ\xc2\x1c\x00\x00"))
}
