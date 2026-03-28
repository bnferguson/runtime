package mysql

import (
	"context"
	"fmt"

	"miren.dev/runtime/api/addon/addon_v1alpha"
	"miren.dev/runtime/api/compute/compute_v1alpha"
	"miren.dev/runtime/api/core/core_v1alpha"
	"miren.dev/runtime/pkg/addon"
	"miren.dev/runtime/pkg/entity"
	"miren.dev/runtime/pkg/entity/types"
	"miren.dev/runtime/pkg/idgen"
	"miren.dev/runtime/pkg/saga"
)

const mysqlPort = 3306

func mysqlContainerPorts() []compute_v1alpha.SandboxSpecContainerPort {
	return []compute_v1alpha.SandboxSpecContainerPort{
		{
			Name:     "mysql",
			Port:     mysqlPort,
			Protocol: compute_v1alpha.SandboxSpecContainerPortTCP,
		},
	}
}

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
		ServiceName:  fmt.Sprintf("%s-mysql", in.AppName),
		ServerName:   fmt.Sprintf("my-%s-%s", in.AppName, idgen.Gen("s")),
	}, nil
}

func UndoGenerateCredentials(ctx context.Context, in GenerateCredentialsIn, out GenerateCredentialsOut) error {
	return nil
}

type CreateMysqlServerIn struct {
	ServerName  string
	VariantName string
	Password    string
}

type CreateMysqlServerOut struct {
	ServerID entity.Id
}

func CreateMysqlServer(ctx context.Context, in CreateMysqlServerIn) (CreateMysqlServerOut, error) {
	fw := saga.Get[*addon.ProviderFramework](ctx)

	server := &addon_v1alpha.MysqlServer{
		AddonName:        AddonName,
		Variant:          in.VariantName,
		Status:           "provisioning",
		AssociationCount: 1,
		RootPassword:     in.Password,
	}

	serverID, err := fw.EC.Create(ctx, in.ServerName, server)
	if err != nil {
		return CreateMysqlServerOut{}, fmt.Errorf("creating mysql server entity: %w", err)
	}

	return CreateMysqlServerOut{ServerID: serverID}, nil
}

func UndoCreateMysqlServer(ctx context.Context, in CreateMysqlServerIn, out CreateMysqlServerOut) error {
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
	diskName := fmt.Sprintf("my-%s-data", in.ServerName)
	mountPath := "/var/lib/mysql"

	env := []string{
		"MYSQL_ROOT_PASSWORD=" + in.Password,
		"MYSQL_DATABASE=" + in.DatabaseName,
		"MYSQL_USER=" + in.Username,
		"MYSQL_PASSWORD=" + in.Password,
	}

	poolID, err := fw.CreateSandboxPool(ctx, addon.CreateSandboxPoolSpec{
		Name:             in.ServerName,
		Image:            DefaultImage,
		Env:              env,
		Ports:            mysqlContainerPorts(),
		DesiredInstances: 1,
		Labels:           labels,
		SandboxPrefix:    fmt.Sprintf("%s-my", in.AppName),
		Mounts: []compute_v1alpha.SandboxSpecContainerMount{
			{Source: "mydata", Destination: mountPath},
		},
		Volumes: []compute_v1alpha.SandboxSpecVolume{
			{
				Name:         "mydata",
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
		return WaitForDedicatedPoolOut{}, fmt.Errorf("waiting for mysql pool: %w", err)
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

	svcID, err := fw.CreateService(ctx, in.ServiceName, labels, mysqlPort)
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

	server := &addon_v1alpha.MysqlServer{
		AddonName:        AddonName,
		Variant:          in.VariantName,
		Status:           "active",
		AssociationCount: 1,
		RootPassword:     in.Password,
		SandboxPool:      in.PoolID,
		Service:          in.ServiceID,
	}
	server.ID = in.ServerID

	if err := fw.EC.Update(ctx, server); err != nil {
		return UpdateDedicatedServerOut{}, fmt.Errorf("updating mysql server: %w", err)
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

	envVars := buildEnvVars(in.ServiceHost, mysqlPort, in.Username, in.Password, in.DatabaseName)

	dedicatedData := &addon_v1alpha.MysqlDedicatedData{
		MysqlServer: in.ServerID,
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

func RegisterDedicatedSaga(registry *saga.Registry, fw *addon.ProviderFramework, rc *resultCapture) error {
	return saga.Define("provision-dedicated-mysql").
		Using(fw).
		Using(rc).
		Action(GenerateCredentials).Undo(UndoGenerateCredentials).
		Action(CreateMysqlServer).Undo(UndoCreateMysqlServer).
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
	var data addon_v1alpha.MysqlDedicatedData
	if in.AssocEntity != nil {
		data.Decode(in.AssocEntity)
	}

	if data.MysqlServer == "" {
		return DecodeDedicatedAttrsOut{}, fmt.Errorf("no mysql server ref found")
	}

	return DecodeDedicatedAttrsOut{DedicatedServerID: data.MysqlServer}, nil
}

func UndoDecodeDedicatedAttrs(ctx context.Context, in DecodeDedicatedAttrsIn, out DecodeDedicatedAttrsOut) error {
	return nil
}

type LookupDedicatedServerIn struct {
	DedicatedServerID entity.Id
}

type LookupDedicatedServerOut struct {
	DedicatedServiceID  entity.Id
	DedicatedPoolID     entity.Id
	DedicatedServerName string
}

func LookupDedicatedServer(ctx context.Context, in LookupDedicatedServerIn) (LookupDedicatedServerOut, error) {
	fw := saga.Get[*addon.ProviderFramework](ctx)

	var server addon_v1alpha.MysqlServer
	if err := fw.EC.GetById(ctx, in.DedicatedServerID, &server); err != nil {
		return LookupDedicatedServerOut{}, fmt.Errorf("looking up mysql server: %w", err)
	}

	var meta core_v1alpha.Metadata
	if err := fw.EC.GetById(ctx, in.DedicatedServerID, &meta); err != nil {
		return LookupDedicatedServerOut{}, fmt.Errorf("looking up server metadata: %w", err)
	}

	return LookupDedicatedServerOut{
		DedicatedServiceID:  server.Service,
		DedicatedPoolID:     server.SandboxPool,
		DedicatedServerName: meta.Name,
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
	PoolDeleted bool `saga:"dedicated_pool_deleted"`
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
	DedicatedServerID   entity.Id
	DedicatedServerName string

	PoolCleanedUp saga.Edge `saga:"dedicated_pool_deleted"`
}

type DeleteDedicatedServerEntityOut struct {
	ServerDeleted bool
}

func DeleteDedicatedServerEntity(ctx context.Context, in DeleteDedicatedServerEntityIn) (DeleteDedicatedServerEntityOut, error) {
	fw := saga.Get[*addon.ProviderFramework](ctx)

	if in.DedicatedServerName != "" {
		diskName := fmt.Sprintf("my-%s-data", in.DedicatedServerName)
		if err := fw.DeleteDiskByName(ctx, diskName); err != nil {
			return DeleteDedicatedServerEntityOut{}, fmt.Errorf("deleting dedicated data disk %s: %w", diskName, err)
		}
	}

	if err := fw.EC.Delete(ctx, in.DedicatedServerID); err != nil {
		return DeleteDedicatedServerEntityOut{}, fmt.Errorf("deleting mysql server: %w", err)
	}

	return DeleteDedicatedServerEntityOut{ServerDeleted: true}, nil
}

func UndoDeleteDedicatedServerEntity(ctx context.Context, in DeleteDedicatedServerEntityIn, out DeleteDedicatedServerEntityOut) error {
	return nil
}

func RegisterDeprovisionDedicatedSaga(registry *saga.Registry, fw *addon.ProviderFramework) error {
	return saga.Define("deprovision-dedicated-mysql").
		Using(fw).
		Action(DecodeDedicatedAttrs).Undo(UndoDecodeDedicatedAttrs).
		Action(LookupDedicatedServer).Undo(UndoLookupDedicatedServer).
		Action(DeleteDedicatedService).Undo(UndoDeleteDedicatedService).
		Action(DeleteDedicatedPool).Undo(UndoDeleteDedicatedPool).
		Action(DeleteDedicatedServerEntity).Undo(UndoDeleteDedicatedServerEntity).
		RegisterTo(registry)
}

func (p *Provider) provisionDedicated(ctx context.Context, app addon.App, variant addon.Variant) (*addon.ProvisionResult, error) {
	p.log.Info("provisioning dedicated MySQL",
		"app", app.Name,
		"variant", variant.Name)

	rc := &resultCapture{}
	registry := saga.NewRegistry()

	if err := RegisterDedicatedSaga(registry, p.fw, rc); err != nil {
		return nil, fmt.Errorf("registering dedicated saga: %w", err)
	}

	storage := p.fw.Storage
	executor := saga.NewExecutor(storage, saga.WithRegistry(registry), saga.WithLogger(p.log))

	err := executor.Start("provision-dedicated-mysql").
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

	p.log.Info("dedicated MySQL provisioned", "app", app.Name)
	return rc.Result, nil
}

func (p *Provider) deprovisionDedicated(ctx context.Context, assoc addon.AddonAssociation) error {
	p.log.Info("deprovisioning dedicated MySQL", "assoc", assoc.ID)

	registry := saga.NewRegistry()

	if err := RegisterDeprovisionDedicatedSaga(registry, p.fw); err != nil {
		return fmt.Errorf("registering deprovision saga: %w", err)
	}

	storage := p.fw.Storage
	executor := saga.NewExecutor(storage, saga.WithRegistry(registry), saga.WithLogger(p.log))

	err := executor.Start("deprovision-dedicated-mysql").
		Input("assocentity", assoc.Entity).
		Execute(ctx)
	if err != nil {
		return err
	}

	p.log.Info("dedicated MySQL deprovisioned", "assoc", assoc.ID)
	return nil
}
