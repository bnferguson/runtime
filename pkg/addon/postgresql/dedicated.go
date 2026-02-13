package postgresql

import (
	"context"
	"fmt"

	"miren.dev/runtime/api/addon/addon_v1alpha"
	"miren.dev/runtime/api/compute/compute_v1alpha"
	"miren.dev/runtime/pkg/addon"
	"miren.dev/runtime/pkg/entity"
	"miren.dev/runtime/pkg/entity/types"
	"miren.dev/runtime/pkg/idgen"
	"miren.dev/runtime/pkg/saga"
)

const postgresPort = 5432

func postgresContainerPorts() []compute_v1alpha.SandboxSpecContainerPort {
	return []compute_v1alpha.SandboxSpecContainerPort{
		{
			Name:     "postgres",
			Port:     postgresPort,
			Protocol: compute_v1alpha.SandboxSpecContainerPortTCP,
		},
	}
}

// resultCapture is injected as a saga dependency so the final action
// can pass the ProvisionResult back to the caller.
type resultCapture struct {
	Result *addon.ProvisionResult
}

// --- Dedicated Provisioning Saga Actions ---

type GenerateCredentialsIn struct {
	AppName string
}

type GenerateCredentialsOut struct {
	Password     string
	DatabaseName string
	Username     string
	ServiceName  string
	ServerName   string
}

func GenerateCredentials(ctx context.Context, in GenerateCredentialsIn) (GenerateCredentialsOut, error) {
	return GenerateCredentialsOut{
		Password:     idgen.Gen("pw"),
		DatabaseName: sanitizeIdentifier(in.AppName),
		Username:     sanitizeIdentifier(in.AppName),
		ServiceName:  fmt.Sprintf("%s-postgresql", in.AppName),
		ServerName:   fmt.Sprintf("pg-%s-%s", in.AppName, idgen.Gen("s")),
	}, nil
}

func UndoGenerateCredentials(ctx context.Context, in GenerateCredentialsIn, out GenerateCredentialsOut) error {
	return nil
}

type CreatePostgresServerIn struct {
	ServerName  string
	VariantName string
	Password    string
}

type CreatePostgresServerOut struct {
	ServerID entity.Id
}

func CreatePostgresServer(ctx context.Context, in CreatePostgresServerIn) (CreatePostgresServerOut, error) {
	fw := saga.Get[*addon.ProviderFramework](ctx)

	server := &addon_v1alpha.PostgresServer{
		AddonName:         AddonName,
		Variant:           in.VariantName,
		Status:            "provisioning",
		AssociationCount:  1,
		SuperuserPassword: in.Password,
	}

	serverID, err := fw.EC.Create(ctx, in.ServerName, server)
	if err != nil {
		return CreatePostgresServerOut{}, fmt.Errorf("creating postgres server entity: %w", err)
	}

	return CreatePostgresServerOut{ServerID: serverID}, nil
}

func UndoCreatePostgresServer(ctx context.Context, in CreatePostgresServerIn, out CreatePostgresServerOut) error {
	fw := saga.Get[*addon.ProviderFramework](ctx)
	return fw.EC.Delete(ctx, out.ServerID)
}

type CreateDedicatedPoolIn struct {
	ServerName    string
	AppName       string
	DatabaseName  string
	Username      string
	Password      string
	VariantConfig map[string]string
}

type CreateDedicatedPoolOut struct {
	PoolID entity.Id
}

func CreateDedicatedPool(ctx context.Context, in CreateDedicatedPoolIn) (CreateDedicatedPoolOut, error) {
	fw := saga.Get[*addon.ProviderFramework](ctx)

	labels := types.LabelSet(
		"addon", AddonName,
		"app", in.AppName,
		"server", in.ServerName,
	)

	sizeGb := parseStorageGb(in.VariantConfig[ConfigStorage])
	diskName := fmt.Sprintf("pg-%s-data", in.ServerName)
	mountPath := "/var/lib/postgresql/data"

	env := []string{
		"POSTGRES_DB=" + in.DatabaseName,
		"POSTGRES_USER=" + in.Username,
		"POSTGRES_PASSWORD=" + in.Password,
		"PGDATA=" + mountPath + "/pgdata",
	}

	poolID, err := fw.CreateSandboxPool(ctx, addon.CreateSandboxPoolSpec{
		Name:             in.ServerName,
		Image:            DefaultImage,
		Env:              env,
		Ports:            postgresContainerPorts(),
		DesiredInstances: 1,
		Labels:           labels,
		SandboxPrefix:    fmt.Sprintf("%s-pg", in.AppName),
		Mounts: []compute_v1alpha.SandboxSpecContainerMount{
			{Source: "pgdata", Destination: mountPath},
		},
		Volumes: []compute_v1alpha.SandboxSpecVolume{
			{
				Name:         "pgdata",
				Provider:     "miren",
				DiskName:     diskName,
				MountPath:    mountPath,
				SizeGb:       sizeGb,
				Filesystem:   "ext4",
				LeaseTimeout: "5m",
			},
		},
	})
	if err != nil {
		return CreateDedicatedPoolOut{}, fmt.Errorf("creating sandbox pool: %w", err)
	}

	return CreateDedicatedPoolOut{PoolID: poolID}, nil
}

func UndoCreateDedicatedPool(ctx context.Context, in CreateDedicatedPoolIn, out CreateDedicatedPoolOut) error {
	fw := saga.Get[*addon.ProviderFramework](ctx)
	return fw.DeleteSandboxPool(ctx, out.PoolID)
}

type WaitForDedicatedPoolIn struct {
	PoolID entity.Id
}

type WaitForDedicatedPoolOut struct {
	Ready bool
}

func WaitForDedicatedPool(ctx context.Context, in WaitForDedicatedPoolIn) (WaitForDedicatedPoolOut, error) {
	fw := saga.Get[*addon.ProviderFramework](ctx)

	if err := fw.WaitForPool(ctx, in.PoolID, poolReadyTimeout); err != nil {
		return WaitForDedicatedPoolOut{}, fmt.Errorf("waiting for postgres pool: %w", err)
	}

	return WaitForDedicatedPoolOut{Ready: true}, nil
}

func UndoWaitForDedicatedPool(ctx context.Context, in WaitForDedicatedPoolIn, out WaitForDedicatedPoolOut) error {
	return nil
}

type CreateDedicatedServiceIn struct {
	ServiceName string
	AppName     string
	ServerName  string
}

type CreateDedicatedServiceOut struct {
	ServiceID entity.Id
}

func CreateDedicatedService(ctx context.Context, in CreateDedicatedServiceIn) (CreateDedicatedServiceOut, error) {
	fw := saga.Get[*addon.ProviderFramework](ctx)

	labels := types.LabelSet(
		"addon", AddonName,
		"app", in.AppName,
		"server", in.ServerName,
	)

	svcID, err := fw.CreateService(ctx, in.ServiceName, labels, postgresPort)
	if err != nil {
		return CreateDedicatedServiceOut{}, fmt.Errorf("creating service: %w", err)
	}

	return CreateDedicatedServiceOut{ServiceID: svcID}, nil
}

func UndoCreateDedicatedService(ctx context.Context, in CreateDedicatedServiceIn, out CreateDedicatedServiceOut) error {
	fw := saga.Get[*addon.ProviderFramework](ctx)
	return fw.DeleteService(ctx, out.ServiceID)
}

type WaitForDedicatedServiceIn struct {
	ServiceID entity.Id
}

type WaitForDedicatedServiceOut struct {
	ServiceHost string
}

func WaitForDedicatedService(ctx context.Context, in WaitForDedicatedServiceIn) (WaitForDedicatedServiceOut, error) {
	fw := saga.Get[*addon.ProviderFramework](ctx)

	serviceHost, err := fw.WaitForServiceAddress(ctx, in.ServiceID, poolReadyTimeout)
	if err != nil {
		return WaitForDedicatedServiceOut{}, fmt.Errorf("waiting for dedicated service address: %w", err)
	}

	return WaitForDedicatedServiceOut{ServiceHost: serviceHost}, nil
}

func UndoWaitForDedicatedService(ctx context.Context, in WaitForDedicatedServiceIn, out WaitForDedicatedServiceOut) error {
	return nil
}

type UpdateDedicatedServerIn struct {
	ServerID    entity.Id
	PoolID      entity.Id
	ServiceID   entity.Id
	VariantName string
	Password    string
}

type UpdateDedicatedServerOut struct {
	Updated bool
}

func UpdateDedicatedServer(ctx context.Context, in UpdateDedicatedServerIn) (UpdateDedicatedServerOut, error) {
	fw := saga.Get[*addon.ProviderFramework](ctx)

	server := &addon_v1alpha.PostgresServer{
		AddonName:         AddonName,
		Variant:           in.VariantName,
		Status:            "active",
		AssociationCount:  1,
		SuperuserPassword: in.Password,
		SandboxPool:       in.PoolID,
		Service:           in.ServiceID,
	}
	server.ID = in.ServerID

	if err := fw.EC.Update(ctx, server); err != nil {
		return UpdateDedicatedServerOut{}, fmt.Errorf("updating postgres server: %w", err)
	}

	return UpdateDedicatedServerOut{Updated: true}, nil
}

func UndoUpdateDedicatedServer(ctx context.Context, in UpdateDedicatedServerIn, out UpdateDedicatedServerOut) error {
	return nil
}

type BuildDedicatedResultIn struct {
	ServiceHost  string
	Username     string
	Password     string
	DatabaseName string
	ServerID     entity.Id
}

type BuildDedicatedResultOut struct {
	Done bool
}

func BuildDedicatedResult(ctx context.Context, in BuildDedicatedResultIn) (BuildDedicatedResultOut, error) {
	rc := saga.Get[*resultCapture](ctx)

	host := in.ServiceHost
	envVars := buildEnvVars(host, postgresPort, in.Username, in.Password, in.DatabaseName)

	dedicatedData := &addon_v1alpha.PostgresqlDedicatedData{
		PostgresServer: in.ServerID,
	}

	rc.Result = &addon.ProvisionResult{
		EnvVars: envVars,
		Attrs:   dedicatedData.Encode(),
	}

	return BuildDedicatedResultOut{Done: true}, nil
}

func UndoBuildDedicatedResult(ctx context.Context, in BuildDedicatedResultIn, out BuildDedicatedResultOut) error {
	return nil
}

// RegisterDedicatedSaga registers the dedicated PostgreSQL provisioning saga.
func RegisterDedicatedSaga(registry *saga.Registry, fw *addon.ProviderFramework, rc *resultCapture) error {
	return saga.Define("provision-dedicated-postgresql").
		Using(fw).
		Using(rc).
		Action(GenerateCredentials).Undo(UndoGenerateCredentials).
		Action(CreatePostgresServer).Undo(UndoCreatePostgresServer).
		Action(CreateDedicatedPool).Undo(UndoCreateDedicatedPool).
		Action(WaitForDedicatedPool).Undo(UndoWaitForDedicatedPool).
		Action(CreateDedicatedService).Undo(UndoCreateDedicatedService).
		Action(WaitForDedicatedService).Undo(UndoWaitForDedicatedService).
		Action(UpdateDedicatedServer).Undo(UndoUpdateDedicatedServer).
		Action(BuildDedicatedResult).Undo(UndoBuildDedicatedResult).
		RegisterTo(registry)
}

// --- Dedicated Deprovisioning Saga Actions ---

type DecodeDedicatedAttrsIn struct {
	AssocEntity *entity.Entity `saga:"assocentity"`
}

type DecodeDedicatedAttrsOut struct {
	DedicatedServerID entity.Id
}

func DecodeDedicatedAttrs(ctx context.Context, in DecodeDedicatedAttrsIn) (DecodeDedicatedAttrsOut, error) {
	var data addon_v1alpha.PostgresqlDedicatedData
	if in.AssocEntity != nil {
		data.Decode(in.AssocEntity)
	}

	if data.PostgresServer == "" {
		return DecodeDedicatedAttrsOut{}, fmt.Errorf("no postgres server ref found")
	}

	return DecodeDedicatedAttrsOut{DedicatedServerID: data.PostgresServer}, nil
}

func UndoDecodeDedicatedAttrs(ctx context.Context, in DecodeDedicatedAttrsIn, out DecodeDedicatedAttrsOut) error {
	return nil
}

type LookupDedicatedServerIn struct {
	DedicatedServerID entity.Id
}

type LookupDedicatedServerOut struct {
	DedicatedServiceID entity.Id
	DedicatedPoolID    entity.Id
}

func LookupDedicatedServer(ctx context.Context, in LookupDedicatedServerIn) (LookupDedicatedServerOut, error) {
	fw := saga.Get[*addon.ProviderFramework](ctx)

	var server addon_v1alpha.PostgresServer
	if err := fw.EC.GetById(ctx, in.DedicatedServerID, &server); err != nil {
		return LookupDedicatedServerOut{}, fmt.Errorf("looking up postgres server: %w", err)
	}

	return LookupDedicatedServerOut{
		DedicatedServiceID: server.Service,
		DedicatedPoolID:    server.SandboxPool,
	}, nil
}

func UndoLookupDedicatedServer(ctx context.Context, in LookupDedicatedServerIn, out LookupDedicatedServerOut) error {
	return nil
}

type DeleteDedicatedServiceIn struct {
	DedicatedServiceID entity.Id
}

type DeleteDedicatedServiceOut struct {
	ServiceDeleted bool
}

func DeleteDedicatedService(ctx context.Context, in DeleteDedicatedServiceIn) (DeleteDedicatedServiceOut, error) {
	if in.DedicatedServiceID == "" {
		return DeleteDedicatedServiceOut{ServiceDeleted: false}, nil
	}

	fw := saga.Get[*addon.ProviderFramework](ctx)
	if err := fw.DeleteService(ctx, in.DedicatedServiceID); err != nil {
		return DeleteDedicatedServiceOut{}, fmt.Errorf("deleting service: %w", err)
	}

	return DeleteDedicatedServiceOut{ServiceDeleted: true}, nil
}

func UndoDeleteDedicatedService(ctx context.Context, in DeleteDedicatedServiceIn, out DeleteDedicatedServiceOut) error {
	return nil
}

type DeleteDedicatedPoolIn struct {
	DedicatedPoolID entity.Id
}

type DeleteDedicatedPoolOut struct {
	PoolDeleted bool
}

func DeleteDedicatedPool(ctx context.Context, in DeleteDedicatedPoolIn) (DeleteDedicatedPoolOut, error) {
	if in.DedicatedPoolID == "" {
		return DeleteDedicatedPoolOut{PoolDeleted: false}, nil
	}

	fw := saga.Get[*addon.ProviderFramework](ctx)
	if err := fw.DeleteSandboxPool(ctx, in.DedicatedPoolID); err != nil {
		return DeleteDedicatedPoolOut{}, fmt.Errorf("deleting sandbox pool: %w", err)
	}

	return DeleteDedicatedPoolOut{PoolDeleted: true}, nil
}

func UndoDeleteDedicatedPool(ctx context.Context, in DeleteDedicatedPoolIn, out DeleteDedicatedPoolOut) error {
	return nil
}

type DeleteDedicatedServerEntityIn struct {
	DedicatedServerID entity.Id
}

type DeleteDedicatedServerEntityOut struct {
	ServerDeleted bool
}

func DeleteDedicatedServerEntity(ctx context.Context, in DeleteDedicatedServerEntityIn) (DeleteDedicatedServerEntityOut, error) {
	fw := saga.Get[*addon.ProviderFramework](ctx)

	if err := fw.EC.Delete(ctx, in.DedicatedServerID); err != nil {
		return DeleteDedicatedServerEntityOut{}, fmt.Errorf("deleting postgres server: %w", err)
	}

	return DeleteDedicatedServerEntityOut{ServerDeleted: true}, nil
}

func UndoDeleteDedicatedServerEntity(ctx context.Context, in DeleteDedicatedServerEntityIn, out DeleteDedicatedServerEntityOut) error {
	return nil
}

// RegisterDeprovisionDedicatedSaga registers the dedicated deprovisioning saga.
func RegisterDeprovisionDedicatedSaga(registry *saga.Registry, fw *addon.ProviderFramework) error {
	return saga.Define("deprovision-dedicated-postgresql").
		Using(fw).
		Action(DecodeDedicatedAttrs).Undo(UndoDecodeDedicatedAttrs).
		Action(LookupDedicatedServer).Undo(UndoLookupDedicatedServer).
		Action(DeleteDedicatedService).Undo(UndoDeleteDedicatedService).
		Action(DeleteDedicatedPool).Undo(UndoDeleteDedicatedPool).
		Action(DeleteDedicatedServerEntity).Undo(UndoDeleteDedicatedServerEntity).
		RegisterTo(registry)
}

// sanitizeIdentifier ensures a name is safe for use as a PostgreSQL identifier.
func sanitizeIdentifier(name string) string {
	result := make([]byte, 0, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' {
			result = append(result, c)
		} else if c >= 'A' && c <= 'Z' {
			result = append(result, c+32) // lowercase
		} else if c == '-' {
			result = append(result, '_')
		}
	}
	if len(result) == 0 {
		return "app"
	}
	if result[0] >= '0' && result[0] <= '9' {
		result = append([]byte{'a'}, result...)
	}
	return string(result)
}

func (p *Provider) provisionDedicated(ctx context.Context, app addon.App, variant addon.Variant) (*addon.ProvisionResult, error) {
	p.log.Info("provisioning dedicated PostgreSQL",
		"app", app.Name,
		"variant", variant.Name)

	rc := &resultCapture{}
	registry := saga.NewRegistry()

	if err := RegisterDedicatedSaga(registry, p.fw, rc); err != nil {
		return nil, fmt.Errorf("registering dedicated saga: %w", err)
	}

	storage := p.fw.Storage
	executor := saga.NewExecutor(storage, saga.WithRegistry(registry), saga.WithLogger(p.log))

	err := executor.Start("provision-dedicated-postgresql").
		Input("appname", app.Name).
		Input("variantname", variant.Name).
		Input("variantconfig", variant.Config).
		Execute(ctx)
	if err != nil {
		return nil, err
	}

	if rc.Result == nil {
		return nil, fmt.Errorf("saga completed but no result was captured")
	}

	p.log.Info("dedicated PostgreSQL provisioned", "app", app.Name)
	return rc.Result, nil
}

func (p *Provider) deprovisionDedicated(ctx context.Context, assoc addon.AddonAssociation) error {
	p.log.Info("deprovisioning dedicated PostgreSQL", "assoc", assoc.ID)

	registry := saga.NewRegistry()

	if err := RegisterDeprovisionDedicatedSaga(registry, p.fw); err != nil {
		return fmt.Errorf("registering deprovision saga: %w", err)
	}

	storage := p.fw.Storage
	executor := saga.NewExecutor(storage, saga.WithRegistry(registry), saga.WithLogger(p.log))

	err := executor.Start("deprovision-dedicated-postgresql").
		Input("assocentity", assoc.Entity).
		Execute(ctx)
	if err != nil {
		return err
	}

	p.log.Info("dedicated PostgreSQL deprovisioned", "assoc", assoc.ID)
	return nil
}
