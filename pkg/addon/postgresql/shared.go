package postgresql

import (
	"context"
	"errors"
	"fmt"
	"time"

	"miren.dev/runtime/api/addon/addon_v1alpha"
	"miren.dev/runtime/api/compute/compute_v1alpha"
	"miren.dev/runtime/pkg/addon"
	"miren.dev/runtime/pkg/cond"
	"miren.dev/runtime/pkg/entity"
	"miren.dev/runtime/pkg/entity/types"
	"miren.dev/runtime/pkg/idgen"
	"miren.dev/runtime/pkg/saga"
)

const (
	sharedServerName = "pg-shared"
	poolReadyTimeout = 5 * time.Minute
)

// --- EnsureSharedServerSaga Actions ---
//
// When no active shared server exists, this saga creates one:
//   step 1: Create PostgresServer entity       → compensate: Delete entity
//   step 2: Create SandboxPool                 → compensate: Delete pool
//   step 3: Wait for pool sandbox to reach RUNNING
//   step 4: Create Service                     → compensate: Delete service
//   step 5: Wait for service address
//   step 6: Activate PostgresServer entity (set refs, status: active)
//
// The saga captures its output into ensureServerCapture so the calling
// action can return ServerID, SuperuserPassword, and ServiceHost.

type CreateSharedServerEntityIn struct {
	SuperuserPassword string
}

type CreateSharedServerEntityOut struct {
	ServerID entity.Id
}

func CreateSharedServerEntity(ctx context.Context, in CreateSharedServerEntityIn) (CreateSharedServerEntityOut, error) {
	fw := saga.Get[*addon.ProviderFramework](ctx)

	server := &addon_v1alpha.PostgresServer{
		AddonName:         AddonName,
		Variant:           "shared",
		Status:            "provisioning",
		AssociationCount:  0,
		SuperuserPassword: in.SuperuserPassword,
	}

	serverID, err := fw.EC.Create(ctx, sharedServerName, server)
	if err != nil {
		return CreateSharedServerEntityOut{}, fmt.Errorf("creating shared server entity: %w", err)
	}

	return CreateSharedServerEntityOut{ServerID: serverID}, nil
}

func UndoCreateSharedServerEntity(ctx context.Context, in CreateSharedServerEntityIn, out CreateSharedServerEntityOut) error {
	fw := saga.Get[*addon.ProviderFramework](ctx)
	return fw.EC.Delete(ctx, out.ServerID)
}

type CreateSharedPoolIn struct {
	SuperuserPassword string
}

type CreateSharedPoolOut struct {
	PoolID entity.Id
}

func CreateSharedPool(ctx context.Context, in CreateSharedPoolIn) (CreateSharedPoolOut, error) {
	fw := saga.Get[*addon.ProviderFramework](ctx)

	labels := types.LabelSet(
		"addon", AddonName,
		"server", sharedServerName,
		"shared", "true",
	)

	mountPath := "/var/lib/postgresql/data"

	env := []string{
		"POSTGRES_PASSWORD=" + in.SuperuserPassword,
		"PGDATA=" + mountPath + "/pgdata",
	}

	poolID, err := fw.CreateSandboxPool(ctx, addon.CreateSandboxPoolSpec{
		Name:             sharedServerName,
		Image:            DefaultImage,
		Env:              env,
		Ports:            postgresContainerPorts(),
		DesiredInstances: 1,
		Labels:           labels,
		SandboxPrefix:    "pg-shared",
		Mounts: []compute_v1alpha.SandboxSpecContainerMount{
			{Source: "pgdata", Destination: mountPath},
		},
		Volumes: []compute_v1alpha.SandboxSpecVolume{
			{
				Name:         "pgdata",
				Provider:     "miren",
				DiskName:     "pg-shared-data",
				MountPath:    mountPath,
				SizeGb:       sharedDefaultStorageGb,
				Filesystem:   "ext4",
				LeaseTimeout: "5m",
			},
		},
	})
	if err != nil {
		return CreateSharedPoolOut{}, fmt.Errorf("creating shared pool: %w", err)
	}

	return CreateSharedPoolOut{PoolID: poolID}, nil
}

func UndoCreateSharedPool(ctx context.Context, in CreateSharedPoolIn, out CreateSharedPoolOut) error {
	fw := saga.Get[*addon.ProviderFramework](ctx)
	return fw.DeleteSandboxPool(ctx, out.PoolID)
}

type WaitForSharedPoolIn struct {
	PoolID entity.Id
}

type WaitForSharedPoolOut struct {
	PoolReady bool
}

func WaitForSharedPool(ctx context.Context, in WaitForSharedPoolIn) (WaitForSharedPoolOut, error) {
	fw := saga.Get[*addon.ProviderFramework](ctx)

	if err := fw.WaitForPool(ctx, in.PoolID, poolReadyTimeout); err != nil {
		return WaitForSharedPoolOut{}, fmt.Errorf("waiting for shared pool: %w", err)
	}

	return WaitForSharedPoolOut{PoolReady: true}, nil
}

func UndoWaitForSharedPool(ctx context.Context, in WaitForSharedPoolIn, out WaitForSharedPoolOut) error {
	return nil
}

type CreateSharedServiceIn struct{}

type CreateSharedServiceOut struct {
	ServiceID entity.Id
}

func CreateSharedService(ctx context.Context, in CreateSharedServiceIn) (CreateSharedServiceOut, error) {
	fw := saga.Get[*addon.ProviderFramework](ctx)

	labels := types.LabelSet(
		"addon", AddonName,
		"server", sharedServerName,
		"shared", "true",
	)

	serviceName := sharedServerName + "-postgresql"
	svcID, err := fw.CreateService(ctx, serviceName, labels, postgresPort)
	if err != nil {
		return CreateSharedServiceOut{}, fmt.Errorf("creating shared service: %w", err)
	}

	return CreateSharedServiceOut{ServiceID: svcID}, nil
}

func UndoCreateSharedService(ctx context.Context, in CreateSharedServiceIn, out CreateSharedServiceOut) error {
	fw := saga.Get[*addon.ProviderFramework](ctx)
	return fw.DeleteService(ctx, out.ServiceID)
}

type WaitForSharedServiceIn struct {
	ServiceID entity.Id
}

type WaitForSharedServiceOut struct {
	ServiceHost string
}

func WaitForSharedService(ctx context.Context, in WaitForSharedServiceIn) (WaitForSharedServiceOut, error) {
	fw := saga.Get[*addon.ProviderFramework](ctx)

	serviceHost, err := fw.WaitForServiceAddress(ctx, in.ServiceID, poolReadyTimeout)
	if err != nil {
		return WaitForSharedServiceOut{}, fmt.Errorf("waiting for shared service address: %w", err)
	}

	return WaitForSharedServiceOut{ServiceHost: serviceHost}, nil
}

func UndoWaitForSharedService(ctx context.Context, in WaitForSharedServiceIn, out WaitForSharedServiceOut) error {
	return nil
}

type ActivateSharedServerIn struct {
	ServerID          entity.Id
	PoolID            entity.Id
	ServiceID         entity.Id
	SuperuserPassword string
	ServiceHost       string
}

type ActivateSharedServerOut struct {
	Activated bool
}

func ActivateSharedServer(ctx context.Context, in ActivateSharedServerIn) (ActivateSharedServerOut, error) {
	fw := saga.Get[*addon.ProviderFramework](ctx)

	server := &addon_v1alpha.PostgresServer{
		AddonName:         AddonName,
		Variant:           "shared",
		Status:            "active",
		AssociationCount:  0,
		SuperuserPassword: in.SuperuserPassword,
		SandboxPool:       in.PoolID,
		Service:           in.ServiceID,
	}
	server.ID = in.ServerID

	if err := fw.EC.Update(ctx, server); err != nil {
		return ActivateSharedServerOut{}, fmt.Errorf("activating shared server: %w", err)
	}

	return ActivateSharedServerOut{Activated: true}, nil
}

func UndoActivateSharedServer(ctx context.Context, in ActivateSharedServerIn, out ActivateSharedServerOut) error {
	return nil
}

// RegisterEnsureSharedServerSaga registers the saga that creates the shared
// PostgreSQL server infrastructure (entity, pool, service).
func RegisterEnsureSharedServerSaga(registry *saga.Registry, fw *addon.ProviderFramework) error {
	return saga.Define("ensure-shared-server").
		Using(fw).
		Action(CreateSharedServerEntity).Undo(UndoCreateSharedServerEntity).
		Action(CreateSharedPool).Undo(UndoCreateSharedPool).
		Action(WaitForSharedPool).Undo(UndoWaitForSharedPool).
		Action(CreateSharedService).Undo(UndoCreateSharedService).
		Action(WaitForSharedService).Undo(UndoWaitForSharedService).
		Action(ActivateSharedServer).Undo(UndoActivateSharedServer).
		RegisterTo(registry)
}

// --- Shared Provisioning Saga Actions ---

// Step 1: Find or create the shared PostgresServer.
// If no active shared server exists, executes the EnsureSharedServerSaga
// as a nested saga to create the server entity, sandbox pool, and service.

type FindOrCreateSharedServerIn struct {
	AppName string
}

type FindOrCreateSharedServerOut struct {
	ServerID          entity.Id
	SuperuserPassword string
	ServiceHost       string
}

func FindOrCreateSharedServer(ctx context.Context, in FindOrCreateSharedServerIn) (FindOrCreateSharedServerOut, error) {
	fw := saga.Get[*addon.ProviderFramework](ctx)

	// Try to find an existing shared server
	var server addon_v1alpha.PostgresServer
	err := fw.EC.Get(ctx, sharedServerName, &server)
	if err == nil {
		switch server.Status {
		case "active":
			serviceHost, err := fw.GetServiceAddress(ctx, server.Service)
			if err != nil {
				return FindOrCreateSharedServerOut{}, fmt.Errorf("resolving existing shared service address: %w", err)
			}
			return FindOrCreateSharedServerOut{
				ServerID:          server.ID,
				SuperuserPassword: server.SuperuserPassword,
				ServiceHost:       serviceHost,
			}, nil
		default:
			return FindOrCreateSharedServerOut{}, fmt.Errorf("shared server exists but has status %q; retry later", server.Status)
		}
	}

	if !errors.Is(err, cond.ErrNotFound{}) {
		return FindOrCreateSharedServerOut{}, fmt.Errorf("looking up shared server: %w", err)
	}

	// No shared server found — run the EnsureSharedServerSaga as a nested saga.
	superuserPassword := idgen.Gen("su")

	result, err := saga.RunNested(ctx, "ensure-shared-server",
		saga.WithNestedInput("superuserpassword", superuserPassword),
	)
	if err != nil {
		return FindOrCreateSharedServerOut{}, fmt.Errorf("ensuring shared server: %w", err)
	}

	var serverID entity.Id
	if err := result.Get("serverid", &serverID); err != nil {
		return FindOrCreateSharedServerOut{}, fmt.Errorf("reading server ID from nested result: %w", err)
	}

	var serviceHost string
	if err := result.Get("servicehost", &serviceHost); err != nil {
		return FindOrCreateSharedServerOut{}, fmt.Errorf("reading service host from nested result: %w", err)
	}

	return FindOrCreateSharedServerOut{
		ServerID:          serverID,
		SuperuserPassword: superuserPassword,
		ServiceHost:       serviceHost,
	}, nil
}

func UndoFindOrCreateSharedServer(ctx context.Context, in FindOrCreateSharedServerIn, out FindOrCreateSharedServerOut) error {
	// The shared server is intentionally not torn down if a later provisioning
	// step fails — it may be serving other applications. The EnsureSharedServerSaga
	// handles its own compensations if server creation fails.
	return nil
}

// Step 2: Generate credentials for the app's database.

type GenerateSharedCredentialsIn struct {
	AppName string
}

type GenerateSharedCredentialsOut struct {
	SharedPassword          string
	SharedDatabaseName      string
	GeneratedSharedUsername string
}

func GenerateSharedCredentials(ctx context.Context, in GenerateSharedCredentialsIn) (GenerateSharedCredentialsOut, error) {
	return GenerateSharedCredentialsOut{
		SharedPassword:          idgen.Gen("pw"),
		SharedDatabaseName:      sanitizeIdentifier(in.AppName),
		GeneratedSharedUsername: sanitizeIdentifier(in.AppName),
	}, nil
}

func UndoGenerateSharedCredentials(ctx context.Context, in GenerateSharedCredentialsIn, out GenerateSharedCredentialsOut) error {
	return nil
}

// Step 3: Connect to the shared server and CREATE USER.

type CreateSharedUserIn struct {
	ServiceHost             string
	SuperuserPassword       string
	GeneratedSharedUsername string
	SharedPassword          string
}

type CreateSharedUserOut struct {
	SharedUsername string
}

func CreateSharedUser(ctx context.Context, in CreateSharedUserIn) (CreateSharedUserOut, error) {
	conn, err := connectAsSuperuser(ctx, in.ServiceHost, in.SuperuserPassword)
	if err != nil {
		return CreateSharedUserOut{}, fmt.Errorf("connecting to shared server: %w", err)
	}
	defer conn.Close(ctx)

	if err := createPostgresUser(ctx, conn, in.GeneratedSharedUsername, in.SharedPassword); err != nil {
		return CreateSharedUserOut{}, err
	}

	return CreateSharedUserOut{SharedUsername: in.GeneratedSharedUsername}, nil
}

func UndoCreateSharedUser(ctx context.Context, in CreateSharedUserIn, out CreateSharedUserOut) error {
	if out.SharedUsername == "" {
		return nil
	}

	conn, err := connectAsSuperuser(ctx, in.ServiceHost, in.SuperuserPassword)
	if err != nil {
		return fmt.Errorf("connecting for user cleanup: %w", err)
	}
	defer conn.Close(ctx)

	return dropPostgresUser(ctx, conn, in.GeneratedSharedUsername)
}

// Step 4: Connect to the shared server and CREATE DATABASE.

type CreateSharedDatabaseIn struct {
	ServiceHost        string
	SuperuserPassword  string
	SharedDatabaseName string
	SharedUsername     string
}

type CreateSharedDatabaseOut struct {
	DatabaseCreated bool
}

func CreateSharedDatabase(ctx context.Context, in CreateSharedDatabaseIn) (CreateSharedDatabaseOut, error) {
	conn, err := connectAsSuperuser(ctx, in.ServiceHost, in.SuperuserPassword)
	if err != nil {
		return CreateSharedDatabaseOut{}, fmt.Errorf("connecting to shared server: %w", err)
	}
	defer conn.Close(ctx)

	if err := createPostgresDatabase(ctx, conn, in.SharedDatabaseName, in.SharedUsername); err != nil {
		return CreateSharedDatabaseOut{}, err
	}

	return CreateSharedDatabaseOut{DatabaseCreated: true}, nil
}

func UndoCreateSharedDatabase(ctx context.Context, in CreateSharedDatabaseIn, out CreateSharedDatabaseOut) error {
	if !out.DatabaseCreated {
		return nil
	}

	conn, err := connectAsSuperuser(ctx, in.ServiceHost, in.SuperuserPassword)
	if err != nil {
		return fmt.Errorf("connecting for database cleanup: %w", err)
	}
	defer conn.Close(ctx)

	return dropPostgresDatabase(ctx, conn, in.SharedDatabaseName)
}

// Step 5: Increment association_count on the shared PostgresServer.

type IncrementAssociationCountIn struct {
	ServerID entity.Id
}

type IncrementAssociationCountOut struct {
	Incremented bool
}

func IncrementAssociationCount(ctx context.Context, in IncrementAssociationCountIn) (IncrementAssociationCountOut, error) {
	fw := saga.Get[*addon.ProviderFramework](ctx)

	var server addon_v1alpha.PostgresServer
	if err := fw.EC.GetById(ctx, in.ServerID, &server); err != nil {
		return IncrementAssociationCountOut{}, fmt.Errorf("getting server for count increment: %w", err)
	}

	server.AssociationCount++
	if err := fw.EC.Update(ctx, &server); err != nil {
		return IncrementAssociationCountOut{}, fmt.Errorf("updating association count: %w", err)
	}

	return IncrementAssociationCountOut{Incremented: true}, nil
}

func UndoIncrementAssociationCount(ctx context.Context, in IncrementAssociationCountIn, out IncrementAssociationCountOut) error {
	if !out.Incremented {
		return nil
	}

	fw := saga.Get[*addon.ProviderFramework](ctx)

	var server addon_v1alpha.PostgresServer
	if err := fw.EC.GetById(ctx, in.ServerID, &server); err != nil {
		return err
	}

	server.AssociationCount--
	return fw.EC.Update(ctx, &server)
}

// Step 6: Build the ProvisionResult.

type BuildSharedResultIn struct {
	ServerID           entity.Id
	ServiceHost        string
	SharedDatabaseName string
	SharedUsername     string
	SharedPassword     string
}

type BuildSharedResultOut struct {
	Done bool
}

func BuildSharedResult(ctx context.Context, in BuildSharedResultIn) (BuildSharedResultOut, error) {
	rc := saga.Get[*resultCapture](ctx)

	envVars := buildEnvVars(in.ServiceHost, postgresPort, in.SharedUsername, in.SharedPassword, in.SharedDatabaseName)

	sharedData := &addon_v1alpha.PostgresqlSharedData{
		PostgresServer: in.ServerID,
		DatabaseName:   in.SharedDatabaseName,
		Username:       in.SharedUsername,
	}

	rc.Result = &addon.ProvisionResult{
		EnvVars: envVars,
		Attrs:   sharedData.Encode(),
	}

	return BuildSharedResultOut{Done: true}, nil
}

func UndoBuildSharedResult(ctx context.Context, in BuildSharedResultIn, out BuildSharedResultOut) error {
	return nil
}

// RegisterSharedSaga registers the shared PostgreSQL provisioning saga.
// This also registers the nested ensure-shared-server saga in the same registry.
func RegisterSharedSaga(registry *saga.Registry, fw *addon.ProviderFramework, rc *resultCapture) error {
	if err := RegisterEnsureSharedServerSaga(registry, fw); err != nil {
		return err
	}

	return saga.Define("provision-shared-postgresql").
		Using(fw).
		Using(rc).
		Action(FindOrCreateSharedServer).Undo(UndoFindOrCreateSharedServer).
		Action(GenerateSharedCredentials).Undo(UndoGenerateSharedCredentials).
		Action(CreateSharedUser).Undo(UndoCreateSharedUser).
		Action(CreateSharedDatabase).Undo(UndoCreateSharedDatabase).
		Action(IncrementAssociationCount).Undo(UndoIncrementAssociationCount).
		Action(BuildSharedResult).Undo(UndoBuildSharedResult).
		RegisterTo(registry)
}

// --- Shared Deprovisioning Saga Actions ---

type DecodeSharedAttrsIn struct {
	AssocEntity *entity.Entity `saga:"assocentity"`
}

type DecodeSharedAttrsOut struct {
	SharedServerRef entity.Id
	SharedDbName    string
	SharedUserName  string
}

func DecodeSharedAttrs(ctx context.Context, in DecodeSharedAttrsIn) (DecodeSharedAttrsOut, error) {
	var data addon_v1alpha.PostgresqlSharedData
	if in.AssocEntity != nil {
		data.Decode(in.AssocEntity)
	}

	if data.PostgresServer == "" {
		return DecodeSharedAttrsOut{}, fmt.Errorf("no postgres server ref found")
	}

	return DecodeSharedAttrsOut{
		SharedServerRef: data.PostgresServer,
		SharedDbName:    data.DatabaseName,
		SharedUserName:  data.Username,
	}, nil
}

func UndoDecodeSharedAttrs(ctx context.Context, in DecodeSharedAttrsIn, out DecodeSharedAttrsOut) error {
	return nil
}

type LookupSharedServerIn struct {
	SharedServerRef entity.Id
}

type LookupSharedServerOut struct {
	SharedSuperuserPassword string
	SharedServiceRef        entity.Id
	SharedPoolRef           entity.Id
	SharedAssocCount        int64
	SharedServiceHost       string
}

func LookupSharedServer(ctx context.Context, in LookupSharedServerIn) (LookupSharedServerOut, error) {
	fw := saga.Get[*addon.ProviderFramework](ctx)

	var server addon_v1alpha.PostgresServer
	if err := fw.EC.GetById(ctx, in.SharedServerRef, &server); err != nil {
		return LookupSharedServerOut{}, fmt.Errorf("looking up shared server: %w", err)
	}

	serviceHost, err := fw.GetServiceAddress(ctx, server.Service)
	if err != nil {
		return LookupSharedServerOut{}, fmt.Errorf("resolving shared service address: %w", err)
	}

	return LookupSharedServerOut{
		SharedSuperuserPassword: server.SuperuserPassword,
		SharedServiceRef:        server.Service,
		SharedPoolRef:           server.SandboxPool,
		SharedAssocCount:        server.AssociationCount,
		SharedServiceHost:       serviceHost,
	}, nil
}

func UndoLookupSharedServer(ctx context.Context, in LookupSharedServerIn, out LookupSharedServerOut) error {
	return nil
}

type TerminateConnectionsIn struct {
	SharedServiceHost       string
	SharedSuperuserPassword string
	SharedDbName            string
}

type TerminateConnectionsOut struct {
	ConnectionsTerminated bool
}

func TerminateConnections(ctx context.Context, in TerminateConnectionsIn) (TerminateConnectionsOut, error) {
	conn, err := connectAsSuperuser(ctx, in.SharedServiceHost, in.SharedSuperuserPassword)
	if err != nil {
		return TerminateConnectionsOut{}, fmt.Errorf("connecting to terminate connections: %w", err)
	}
	defer conn.Close(ctx)

	if err := terminatePostgresConnections(ctx, conn, in.SharedDbName); err != nil {
		return TerminateConnectionsOut{}, err
	}

	return TerminateConnectionsOut{ConnectionsTerminated: true}, nil
}

func UndoTerminateConnections(ctx context.Context, in TerminateConnectionsIn, out TerminateConnectionsOut) error {
	return nil
}

type DropSharedDatabaseIn struct {
	SharedServiceHost       string
	SharedSuperuserPassword string
	SharedDbName            string
	ConnectionsTerminated   bool
}

type DropSharedDatabaseOut struct {
	DatabaseDropped bool
}

func DropSharedDatabase(ctx context.Context, in DropSharedDatabaseIn) (DropSharedDatabaseOut, error) {
	conn, err := connectAsSuperuser(ctx, in.SharedServiceHost, in.SharedSuperuserPassword)
	if err != nil {
		return DropSharedDatabaseOut{}, fmt.Errorf("connecting to drop database: %w", err)
	}
	defer conn.Close(ctx)

	if err := dropPostgresDatabase(ctx, conn, in.SharedDbName); err != nil {
		return DropSharedDatabaseOut{}, err
	}

	return DropSharedDatabaseOut{DatabaseDropped: true}, nil
}

func UndoDropSharedDatabase(ctx context.Context, in DropSharedDatabaseIn, out DropSharedDatabaseOut) error {
	return nil
}

type DropSharedUserIn struct {
	SharedServiceHost       string
	SharedSuperuserPassword string
	SharedUserName          string
	ConnectionsTerminated   bool
}

type DropSharedUserOut struct {
	UserDropped bool
}

func DropSharedUser(ctx context.Context, in DropSharedUserIn) (DropSharedUserOut, error) {
	conn, err := connectAsSuperuser(ctx, in.SharedServiceHost, in.SharedSuperuserPassword)
	if err != nil {
		return DropSharedUserOut{}, fmt.Errorf("connecting to drop user: %w", err)
	}
	defer conn.Close(ctx)

	if err := dropPostgresUser(ctx, conn, in.SharedUserName); err != nil {
		return DropSharedUserOut{}, err
	}

	return DropSharedUserOut{UserDropped: true}, nil
}

func UndoDropSharedUser(ctx context.Context, in DropSharedUserIn, out DropSharedUserOut) error {
	return nil
}

type DecrementAssociationCountIn struct {
	SharedServerRef entity.Id
	DatabaseDropped bool
	UserDropped     bool
}

type DecrementAssociationCountOut struct {
	RemainingCount int64
}

func DecrementAssociationCount(ctx context.Context, in DecrementAssociationCountIn) (DecrementAssociationCountOut, error) {
	fw := saga.Get[*addon.ProviderFramework](ctx)

	var server addon_v1alpha.PostgresServer
	if err := fw.EC.GetById(ctx, in.SharedServerRef, &server); err != nil {
		return DecrementAssociationCountOut{}, fmt.Errorf("getting server: %w", err)
	}

	server.AssociationCount--
	if err := fw.EC.Update(ctx, &server); err != nil {
		return DecrementAssociationCountOut{}, fmt.Errorf("updating association count: %w", err)
	}

	return DecrementAssociationCountOut{RemainingCount: server.AssociationCount}, nil
}

func UndoDecrementAssociationCount(ctx context.Context, in DecrementAssociationCountIn, out DecrementAssociationCountOut) error {
	return nil
}

type CleanupSharedServerIn struct {
	SharedServerRef  entity.Id
	SharedServiceRef entity.Id
	SharedPoolRef    entity.Id
	RemainingCount   int64
}

type CleanupSharedServerOut struct {
	CleanedUp bool
}

func CleanupSharedServer(ctx context.Context, in CleanupSharedServerIn) (CleanupSharedServerOut, error) {
	if in.RemainingCount > 0 {
		return CleanupSharedServerOut{CleanedUp: false}, nil
	}

	fw := saga.Get[*addon.ProviderFramework](ctx)

	if in.SharedServiceRef != "" {
		if err := fw.DeleteService(ctx, in.SharedServiceRef); err != nil {
			return CleanupSharedServerOut{}, fmt.Errorf("deleting shared service: %w", err)
		}
	}

	if in.SharedPoolRef != "" {
		if err := fw.DeleteSandboxPool(ctx, in.SharedPoolRef); err != nil {
			return CleanupSharedServerOut{}, fmt.Errorf("deleting shared pool: %w", err)
		}
	}

	if err := fw.EC.Delete(ctx, in.SharedServerRef); err != nil {
		return CleanupSharedServerOut{}, fmt.Errorf("deleting shared server: %w", err)
	}

	return CleanupSharedServerOut{CleanedUp: true}, nil
}

func UndoCleanupSharedServer(ctx context.Context, in CleanupSharedServerIn, out CleanupSharedServerOut) error {
	return nil
}

// RegisterDeprovisionSharedSaga registers the shared deprovisioning saga.
func RegisterDeprovisionSharedSaga(registry *saga.Registry, fw *addon.ProviderFramework) error {
	return saga.Define("deprovision-shared-postgresql").
		Using(fw).
		Action(DecodeSharedAttrs).Undo(UndoDecodeSharedAttrs).
		Action(LookupSharedServer).Undo(UndoLookupSharedServer).
		Action(TerminateConnections).Undo(UndoTerminateConnections).
		Action(DropSharedDatabase).Undo(UndoDropSharedDatabase).
		Action(DropSharedUser).Undo(UndoDropSharedUser).
		Action(DecrementAssociationCount).Undo(UndoDecrementAssociationCount).
		Action(CleanupSharedServer).Undo(UndoCleanupSharedServer).
		RegisterTo(registry)
}

func (p *Provider) provisionShared(ctx context.Context, app addon.App, variant addon.Variant) (*addon.ProvisionResult, error) {
	p.log.Info("provisioning shared PostgreSQL",
		"app", app.Name,
		"variant", variant.Name)

	rc := &resultCapture{}
	registry := saga.NewRegistry()

	if err := RegisterSharedSaga(registry, p.fw, rc); err != nil {
		return nil, fmt.Errorf("registering shared saga: %w", err)
	}

	storage := saga.NewEntityStorage(p.fw.Store, p.log)
	executor := saga.NewExecutor(storage, saga.WithRegistry(registry), saga.WithLogger(p.log))

	err := executor.Start("provision-shared-postgresql").
		Input("appname", app.Name).
		Execute(ctx)
	if err != nil {
		return nil, err
	}

	if rc.Result == nil {
		return nil, fmt.Errorf("saga completed but no result was captured")
	}

	p.log.Info("shared PostgreSQL provisioned", "app", app.Name)
	return rc.Result, nil
}

func (p *Provider) deprovisionShared(ctx context.Context, assoc addon.AddonAssociation) error {
	p.log.Info("deprovisioning shared PostgreSQL", "assoc", assoc.ID)

	registry := saga.NewRegistry()

	if err := RegisterDeprovisionSharedSaga(registry, p.fw); err != nil {
		return fmt.Errorf("registering deprovision saga: %w", err)
	}

	storage := saga.NewEntityStorage(p.fw.Store, p.log)
	executor := saga.NewExecutor(storage, saga.WithRegistry(registry), saga.WithLogger(p.log))

	err := executor.Start("deprovision-shared-postgresql").
		Input("assocentity", assoc.Entity).
		Execute(ctx)
	if err != nil {
		return err
	}

	p.log.Info("shared PostgreSQL deprovisioned", "assoc", assoc.ID)
	return nil
}
