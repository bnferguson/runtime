package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	appclient "miren.dev/runtime/api/app"
	"miren.dev/runtime/api/app/app_v1alpha"
	coreutil "miren.dev/runtime/api/core"
	"miren.dev/runtime/api/core/core_v1alpha"
	"miren.dev/runtime/api/entityserver"
	"miren.dev/runtime/api/ingress/ingress_v1alpha"
	"miren.dev/runtime/metrics"
	"miren.dev/runtime/pkg/cond"
	"miren.dev/runtime/pkg/entity"
	"miren.dev/runtime/pkg/idgen"
	"miren.dev/runtime/pkg/rpc"
)

// TODO: Removed broken go:generate directive - no rpc.yml file exists in servers/app/
// If RPC generation is needed here, create rpc.yml first
// //go:generate go run ../../pkg/rpc/cmd/rpcgen -pkg app -input rpc.yml -output rpc.gen.go

type ClearVersioner interface {
	ClearOldVersions(ctx context.Context, current *core_v1alpha.AppVersion) error
}

type AppInfo struct {
	Log  *slog.Logger
	CV   ClearVersioner
	EC   *entityserver.Client
	CPU  *metrics.CPUUsage
	Mem  *metrics.MemoryUsage
	HTTP *metrics.HTTPMetrics
}

func NewAppInfo(log *slog.Logger, ec *entityserver.Client, cpu *metrics.CPUUsage, mem *metrics.MemoryUsage, http *metrics.HTTPMetrics) *AppInfo {
	return &AppInfo{
		Log:  log,
		CV:   nil,
		EC:   ec,
		CPU:  cpu,
		Mem:  mem,
		HTTP: http,
	}
}

var _ app_v1alpha.Crud = &AppInfo{}

func (r *AppInfo) New(ctx context.Context, state *app_v1alpha.CrudNew) error {
	name := state.Args().Name()

	var appRec core_v1alpha.App

	err := r.EC.Get(ctx, name, &appRec)
	if err == nil {
		state.Results().SetId(name)
		return nil
	}

	_, err = r.EC.Create(ctx, name, &appRec)
	if err != nil {
		return err
	}

	// TODO this is a bad id.
	state.Results().SetId(name)

	return nil
}

func (r *AppInfo) Destroy(ctx context.Context, state *app_v1alpha.CrudDestroy) error {
	name := state.Args().Name()

	var appRec core_v1alpha.App

	err := r.EC.Get(ctx, name, &appRec)
	if err != nil {
		if errors.Is(err, cond.ErrNotFound{}) {
			// No app, no problem.
			return nil
		}

		return err
	}

	return DeleteAppTransitive(ctx, r.EC, r.Log, appRec.ID)
}

func (r *AppInfo) List(ctx context.Context, state *app_v1alpha.CrudList) error {
	list, err := r.EC.List(ctx, entity.Ref(entity.EntityKind, core_v1alpha.KindApp))
	if err != nil {
		return err
	}

	var ai []*app_v1alpha.AppInfo

	for list.Next() {
		var app core_v1alpha.App
		list.Read(&app)

		md := list.Metadata()

		var a app_v1alpha.AppInfo

		a.SetName(md.Name)
		//a.SetCreatedAt(standard.ToTimestamp(list.Entity().CreatedAt))

		if app.ActiveVersion != "" {
			var appVer core_v1alpha.AppVersion
			err = r.EC.GetById(ctx, app.ActiveVersion, &appVer)
			if err != nil {
				return err
			}

			var vi app_v1alpha.VersionInfo
			vi.SetVersion(appVer.Version)
			a.SetCurrentVersion(&vi)
		}

		ai = append(ai, &a)
	}

	state.Results().SetApps(ai)

	return nil
}

func (r *AppInfo) SetConfiguration(ctx context.Context, state *app_v1alpha.CrudSetConfiguration) error {
	name := state.Args().App()

	var appRec core_v1alpha.App

	err := r.EC.Get(ctx, name, &appRec)
	if err != nil {
		if errors.Is(err, cond.ErrNotFound{}) {
			// No app, no problem.
			return nil
		}

		return err
	}

	var appVer core_v1alpha.AppVersion
	var spec core_v1alpha.ConfigSpec

	if appRec.ActiveVersion != "" {
		err = r.EC.GetById(ctx, appRec.ActiveVersion, &appVer)
		if err != nil {
			return err
		}
		resolvedCfg, err := coreutil.ResolveConfig(ctx, r.EC.EAC(), &appVer)
		if err != nil {
			return fmt.Errorf("failed to resolve config: %w", err)
		}
		spec = *resolvedCfg
	} else {
		appVer.App = appRec.ID
	}

	cfg := state.Args().Configuration()

	if cfg.HasEnvVars() {
		for _, nv := range cfg.EnvVars() {
			if strings.HasPrefix(nv.Key(), "MIREN_") {
				return fmt.Errorf("cannot set MIREN_ environment variables")
			}
		}
	}

	// Set commands directly on services
	for _, s := range cfg.Commands() {
		found := false
		for i := range spec.Services {
			if spec.Services[i].Name == s.Service() {
				spec.Services[i].Command = s.Command()
				found = true
				break
			}
		}
		if !found {
			spec.Services = append(spec.Services, core_v1alpha.ConfigSpecServices{
				Name:    s.Service(),
				Command: s.Command(),
			})
		}
	}

	// Replace the entire env var list with the new one from the client
	// The client is responsible for sending the complete desired state
	if cfg.HasEnvVars() {
		spec.Variables = nil
		for _, ev := range cfg.EnvVars() {
			source := ev.Source()
			nv := core_v1alpha.ConfigSpecVariables{
				Key:         ev.Key(),
				Value:       ev.Value(),
				Sensitive:   ev.Sensitive(),
				Source:      source,
				Required:    ev.Required(),
				Description: ev.Description(),
			}
			spec.Variables = append(spec.Variables, nv)
		}
	}

	// Handle per-service env vars
	if cfg.HasServices() {
		for _, svcCfg := range cfg.Services() {
			// Validate per-service env vars
			if svcCfg.HasServiceEnv() {
				for _, nv := range svcCfg.ServiceEnv() {
					if strings.HasPrefix(nv.Key(), "MIREN_") {
						return fmt.Errorf("cannot set MIREN_ environment variables")
					}
				}
			}

			// Find or create the service in spec.Services
			var found bool
			for i := range spec.Services {
				if spec.Services[i].Name == svcCfg.Service() {
					spec.Services[i].Env = nil
					if svcCfg.HasServiceEnv() {
						for _, ev := range svcCfg.ServiceEnv() {
							source := ev.Source()
							nv := core_v1alpha.ConfigSpecServicesEnv{
								Key:         ev.Key(),
								Value:       ev.Value(),
								Sensitive:   ev.Sensitive(),
								Source:      source,
								Required:    ev.Required(),
								Description: ev.Description(),
							}
							spec.Services[i].Env = append(spec.Services[i].Env, nv)
						}
					}
					found = true
					break
				}
			}

			if !found && svcCfg.HasServiceEnv() {
				svc := core_v1alpha.ConfigSpecServices{
					Name: svcCfg.Service(),
				}
				for _, ev := range svcCfg.ServiceEnv() {
					source := ev.Source()
					nv := core_v1alpha.ConfigSpecServicesEnv{
						Key:         ev.Key(),
						Value:       ev.Value(),
						Sensitive:   ev.Sensitive(),
						Source:      source,
						Required:    ev.Required(),
						Description: ev.Description(),
					}
					svc.Env = append(svc.Env, nv)
				}
				spec.Services = append(spec.Services, svc)
			}
		}
	}

	spec.Entrypoint = cfg.Entrypoint()

	appVer.Version = name + "-" + idgen.Gen("v")

	// Create ConfigVersion as the sole config store
	cvid, err := r.createConfigVersion(ctx, &spec, appVer.App, appVer.Version)
	if err != nil {
		return fmt.Errorf("error creating config version: %w", err)
	}
	appVer.ConfigVersion = cvid
	appVer.Config = core_v1alpha.Config{}

	avid, err := r.EC.Create(ctx, appVer.Version, &appVer)
	if err != nil {
		return err
	}

	appRec.ActiveVersion = avid
	err = r.EC.Update(ctx, &appRec)
	if err != nil {
		return fmt.Errorf("error updating app entity: %w", err)
	}

	state.Results().SetVersionId(appVer.Version)

	return nil
}

func (r *AppInfo) GetConfiguration(ctx context.Context, state *app_v1alpha.CrudGetConfiguration) error {
	name := state.Args().App()

	if !rpc.AllowApp(ctx, name) {
		return rpc.AppAccessError(ctx, name)
	}

	var appRec core_v1alpha.App

	err := r.EC.Get(ctx, name, &appRec)
	if err != nil {
		return err
	}

	var appVer core_v1alpha.AppVersion

	if appRec.ActiveVersion != "" {
		err = r.EC.GetById(ctx, appRec.ActiveVersion, &appVer)
		if err != nil {
			return err
		}
	} else {
		return fmt.Errorf("app has no active version")
	}

	spec, err := coreutil.ResolveConfig(ctx, r.EC.EAC(), &appVer)
	if err != nil {
		return fmt.Errorf("failed to resolve config: %w", err)
	}

	var cfg app_v1alpha.Configuration

	// Build commands from services that have commands
	var commands []*app_v1alpha.ServiceCommand
	for _, svc := range spec.Services {
		if svc.Command != "" {
			var sc app_v1alpha.ServiceCommand
			sc.SetService(svc.Name)
			sc.SetCommand(svc.Command)
			commands = append(commands, &sc)
		}
	}

	cfg.SetCommands(commands)

	var envVars []*app_v1alpha.NamedValue
	for _, ev := range spec.Variables {
		var env app_v1alpha.NamedValue
		env.SetKey(ev.Key)
		env.SetValue(ev.Value)
		env.SetSensitive(ev.Sensitive)
		env.SetSource(ev.Source)
		env.SetRequired(ev.Required)
		env.SetDescription(ev.Description)
		envVars = append(envVars, &env)
	}

	cfg.SetEnvVars(envVars)

	// Add per-service configurations
	var services []*app_v1alpha.ServiceConfig
	for _, svc := range spec.Services {
		var sc app_v1alpha.ServiceConfig
		sc.SetService(svc.Name)
		if svc.Concurrency.Mode != "" {
			sc.SetConcurrencyMode(svc.Concurrency.Mode)
		}
		if svc.Concurrency.NumInstances != 0 {
			sc.SetNumInstances(int32(svc.Concurrency.NumInstances))
		}

		// Add service env vars
		if len(svc.Env) > 0 {
			var svcEnvVars []*app_v1alpha.NamedValue
			for _, ev := range svc.Env {
				var env app_v1alpha.NamedValue
				env.SetKey(ev.Key)
				env.SetValue(ev.Value)
				env.SetSensitive(ev.Sensitive)
				env.SetSource(ev.Source)
				env.SetRequired(ev.Required)
				env.SetDescription(ev.Description)
				svcEnvVars = append(svcEnvVars, &env)
			}
			sc.SetServiceEnv(svcEnvVars)
		}

		services = append(services, &sc)
	}
	cfg.SetServices(services)

	cfg.SetEntrypoint(spec.Entrypoint)

	state.Results().SetConfiguration(&cfg)

	return nil
}

func (r *AppInfo) SetHost(ctx context.Context, state *app_v1alpha.CrudSetHost) error {
	name := state.Args().App()

	var appRec core_v1alpha.App

	err := r.EC.Get(ctx, name, &appRec)
	if err != nil {
		if errors.Is(err, cond.ErrNotFound{}) {
			// No app, no problem.
			return nil
		}

		return err
	}

	var routeRec ingress_v1alpha.HttpRoute

	routeRec.Host = strings.ToLower(state.Args().Host())
	routeRec.App = appRec.ID

	_, err = r.EC.CreateOrUpdate(ctx, routeRec.Host, &routeRec)
	if err != nil {
		return err
	}

	return nil
}

func (r *AppInfo) setEnvVars(ctx context.Context, name string, vars []appclient.EnvVarInput, service string) (string, error) {
	result, err := appclient.SetEnvVars(ctx, r.EC, name, nil, vars, service)
	if err != nil {
		return "", err
	}
	return result.VersionID, nil
}

func (r *AppInfo) SetEnvVar(ctx context.Context, state *app_v1alpha.CrudSetEnvVar) error {
	args := state.Args()

	versionId, err := r.setEnvVars(ctx, args.App(), []appclient.EnvVarInput{
		{Key: args.Key(), Value: args.Value(), Sensitive: args.Sensitive()},
	}, args.Service())
	if err != nil {
		return err
	}

	state.Results().SetVersionId(versionId)
	return nil
}

func (r *AppInfo) SetEnvVars(ctx context.Context, state *app_v1alpha.CrudSetEnvVars) error {
	args := state.Args()
	rpcVars := args.Vars()

	if len(rpcVars) == 0 {
		return fmt.Errorf("no environment variables provided")
	}

	vars := make([]appclient.EnvVarInput, len(rpcVars))
	for i, v := range rpcVars {
		vars[i] = appclient.EnvVarInput{Key: v.Key(), Value: v.Value(), Sensitive: v.Sensitive()}
	}

	versionId, err := r.setEnvVars(ctx, args.App(), vars, args.Service())
	if err != nil {
		return err
	}

	state.Results().SetVersionId(versionId)
	return nil
}

func (r *AppInfo) DeleteEnvVar(ctx context.Context, state *app_v1alpha.CrudDeleteEnvVar) error {
	args := state.Args()

	result, err := appclient.DeleteEnvVars(ctx, r.EC, args.App(), nil, []string{args.Key()}, args.Service())
	if err != nil {
		return err
	}

	state.Results().SetVersionId(result.VersionID)
	if len(result.DeletedSources) > 0 {
		state.Results().SetDeletedSource(result.DeletedSources[0])
	}
	return nil
}

// createConfigVersion creates a ConfigVersion entity from a ConfigSpec.
func (r *AppInfo) createConfigVersion(ctx context.Context, spec *core_v1alpha.ConfigSpec, appID entity.Id, versionName string) (entity.Id, error) {
	configVer := &core_v1alpha.ConfigVersion{
		App:  appID,
		Spec: *spec,
	}
	cvName := versionName + "-cfg"
	return r.EC.Create(ctx, cvName, configVer)
}
