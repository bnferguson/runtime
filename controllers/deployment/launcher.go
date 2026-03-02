package deployment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"miren.dev/runtime/api/addon/addon_v1alpha"
	"miren.dev/runtime/api/compute/compute_v1alpha"
	coreutil "miren.dev/runtime/api/core"
	"miren.dev/runtime/api/core/core_v1alpha"
	"miren.dev/runtime/api/entityserver/entityserver_v1alpha"
	"miren.dev/runtime/pkg/cond"
	"miren.dev/runtime/pkg/containerdx"
	"miren.dev/runtime/pkg/controller"
	"miren.dev/runtime/pkg/entity"
	"miren.dev/runtime/pkg/entity/types"
	"miren.dev/runtime/pkg/idgen"
	"miren.dev/runtime/pkg/labs"
)

var launcherTracer = otel.Tracer("miren.dev/runtime/deployment/launcher")

// Launcher watches App entities and proactively creates SandboxPools when ActiveVersion changes.
// This enables immediate startup for fixed-mode services and pool reuse across deployments.
type Launcher struct {
	Log *slog.Logger
	EAC *entityserver_v1alpha.EntityAccessClient

	// PoolReadyTimeout is how long to wait for new pools to have ready instances
	// before proceeding with old pool cleanup. Defaults to 60s.
	PoolReadyTimeout time.Duration

	appMu sync.Map // per-app mutexes: app ID -> *sync.Mutex
}

// PoolWithEntity wraps a SandboxPool with its entity, allowing updates without re-fetching
type PoolWithEntity struct {
	Pool   *compute_v1alpha.SandboxPool
	Entity entity.Entity
}

func NewLauncher(log *slog.Logger, eac *entityserver_v1alpha.EntityAccessClient) *Launcher {
	return &Launcher{
		Log:              log.With("module", "deployment"),
		EAC:              eac,
		PoolReadyTimeout: 60 * time.Second,
	}
}

func (l *Launcher) Init(ctx context.Context) error {
	l.Log.Info("deployment launcher initialized")
	return nil
}

func (l *Launcher) Reconcile(ctx context.Context, app *core_v1alpha.App, meta *entity.Meta) error {
	ctx, span := launcherTracer.Start(ctx, "launcher.reconcile",
		trace.WithAttributes(attribute.String("miren.app.id", app.ID.String())))
	defer span.End()

	// Serialize reconciles for the same app to avoid races between rapid deploys.
	val, _ := l.appMu.LoadOrStore(app.ID, &sync.Mutex{})
	mu := val.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()

	// Re-read from store to get latest state, coalescing rapid updates.
	// The controller framework embeds the entity snapshot at dispatch time,
	// so the app passed here may have a stale ActiveVersion if multiple
	// versions were created in quick succession.
	appResp, err := l.EAC.Get(ctx, app.ID.String())
	if err != nil {
		if errors.Is(err, cond.ErrNotFound{}) {
			l.Log.Debug("app not found in store, skipping", "app", app.ID)
			return nil
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("failed to get app from store: %w", err)
	}

	var current core_v1alpha.App
	current.Decode(appResp.Entity().Entity())

	if current.ActiveVersion == "" {
		l.Log.Debug("app has no active version, skipping", "app", current.ID)
		return nil
	}

	span.SetAttributes(attribute.String("miren.app.active_version", current.ActiveVersion.String()))

	l.Log.Info("reconciling app", "app", current.ID, "version", current.ActiveVersion)
	ready, err := l.addonsReady(ctx, current.ID)
	if err != nil {
		l.Log.Error("failed to check addon readiness", "app", current.ID, "error", err)
	} else if !ready {
		l.Log.Info("deferring pool creation, addons not yet ready", "app", current.ID)
		return nil
	}

	err = l.reconcileAppVersion(ctx, &current)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return err
}

// addonsReady returns true if the app has no pending or provisioning addon associations.
// Apps without any addons are always considered ready.
func (l *Launcher) addonsReady(ctx context.Context, appID entity.Id) (bool, error) {
	results, err := l.EAC.List(ctx, entity.Ref(addon_v1alpha.AddonAssociationAppId, appID))
	if err != nil {
		return false, fmt.Errorf("listing addon associations: %w", err)
	}

	for _, ent := range results.Values() {
		var assoc addon_v1alpha.AddonAssociation
		assoc.Decode(ent.Entity())

		if assoc.Status == "pending" || assoc.Status == "provisioning" {
			l.Log.Info("addon not ready", "association", assoc.ID, "status", assoc.Status)
			return false, nil
		}
	}

	return true, nil
}

// AddonAssociationHandler returns a controller.HandlerFunc that watches AddonAssociation
// changes and re-triggers app reconciliation so the launcher can re-evaluate addon readiness.
func (l *Launcher) AddonAssociationHandler() controller.HandlerFunc {
	return func(ctx context.Context, event controller.Event) ([]entity.Attr, error) {
		if event.Type == controller.EventDeleted || event.Entity == nil {
			return nil, nil
		}

		var assoc addon_v1alpha.AddonAssociation
		assoc.Decode(event.Entity)

		if assoc.App == "" {
			return nil, nil
		}

		l.Log.Info("addon association changed, reconciling app",
			"association", assoc.ID, "status", assoc.Status, "app", assoc.App)

		// Fetch the app and run the same reconcile logic
		appResp, err := l.EAC.Get(ctx, assoc.App.String())
		if err != nil {
			if errors.Is(err, cond.ErrNotFound{}) {
				return nil, nil
			}
			return nil, fmt.Errorf("failed to get app: %w", err)
		}

		var app core_v1alpha.App
		app.Decode(appResp.Entity().Entity())

		if app.ActiveVersion == "" {
			return nil, nil
		}

		ready, err := l.addonsReady(ctx, app.ID)
		if err != nil {
			l.Log.Error("failed to check addon readiness", "app", app.ID, "error", err)
			return nil, nil
		}
		if !ready {
			l.Log.Info("addons still not ready, deferring", "app", app.ID)
			return nil, nil
		}

		l.Log.Info("addons ready, triggering app reconciliation", "app", app.ID)
		return nil, l.reconcileAppVersion(ctx, &app)
	}
}

// reconcileAppVersion ensures pools exist for all services in the active version
func (l *Launcher) reconcileAppVersion(ctx context.Context, app *core_v1alpha.App) error {
	// Fetch the AppVersion entity
	verResp, err := l.EAC.Get(ctx, app.ActiveVersion.String())
	if err != nil {
		return fmt.Errorf("failed to get app version: %w", err)
	}

	var ver core_v1alpha.AppVersion
	ver.Decode(verResp.Entity().Entity())

	// Resolve config from ConfigVersion if available, otherwise use inline
	spec, err := coreutil.ResolveConfig(ctx, l.EAC, &ver)
	if err != nil {
		return fmt.Errorf("failed to resolve config: %w", err)
	}

	l.Log.Info("reconciling app version",
		"app", app.ID,
		"version", ver.Version,
		"services", len(spec.Services))

	// For each service, ensure a pool exists. Collect IDs of newly created pools.
	var newPoolIDs []entity.Id
	for _, svc := range spec.Services {
		poolID, err := l.ensurePoolForService(ctx, app, &ver, spec, svc.Name)
		if err != nil {
			l.Log.Error("failed to ensure pool for service",
				"app", app.ID,
				"service", svc.Name,
				"error", err)
			// Continue with other services even if one fails
			continue
		}
		if poolID != "" {
			newPoolIDs = append(newPoolIDs, poolID)
		}
	}

	// Wait for new pools to have ready instances before killing old ones.
	// This prevents a gap where the old sandbox is dead but the new one
	// isn't serving yet, which would cause 502s.
	for _, poolID := range newPoolIDs {
		if err := l.waitForPoolReady(ctx, poolID, l.PoolReadyTimeout); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				l.Log.Warn("timed out waiting for new pool to become ready, proceeding with cleanup",
					"pool", poolID,
					"error", err)
				continue
			}
			return fmt.Errorf("failed waiting for new pool readiness %s: %w", poolID, err)
		}
	}

	// Clean up old version pools (pools not referenced by current version)
	if err := l.cleanupOldVersionPools(ctx, app, ver.ID); err != nil {
		l.Log.Error("failed to cleanup old version pools",
			"app", app.ID,
			"version", ver.ID,
			"error", err)
		// Don't fail the entire reconciliation if cleanup fails
	}

	return nil
}

// ensurePoolForService creates or reuses a pool for the given service.
// Returns the pool ID if a new pool was created (caller should wait for it
// to become ready), or an empty ID if an existing pool was reused.
func (l *Launcher) ensurePoolForService(ctx context.Context, app *core_v1alpha.App, ver *core_v1alpha.AppVersion, spec *core_v1alpha.ConfigSpec, serviceName string) (entity.Id, error) {
	// Get service config
	svcConcurrency, err := coreutil.GetServiceConcurrency(spec, serviceName)
	if err != nil {
		return "", fmt.Errorf("failed to get service concurrency: %w", err)
	}

	// Determine which image to use
	image := ver.ImageUrl
	for _, svc := range spec.Services {
		if svc.Name == serviceName && svc.Image != "" {
			image = containerdx.NormalizeImageReference(svc.Image)
			l.Log.Info("using custom image for service",
				"service", serviceName,
				"image", image,
				"original", svc.Image)
			break
		}
	}

	// Get app metadata for label
	appResp, err := l.EAC.Get(ctx, app.ID.String())
	if err != nil {
		return "", fmt.Errorf("failed to get app metadata: %w", err)
	}

	var appMD core_v1alpha.Metadata
	appMD.Decode(appResp.Entity().Entity())

	// Build the desired sandbox spec
	sbSpec, err := l.buildSandboxSpec(ctx, app, ver, spec, serviceName, image)
	if err != nil {
		return "", fmt.Errorf("failed to build sandbox spec: %w", err)
	}

	// Calculate desired instances based on concurrency mode
	// Auto mode starts with 1 to boot immediately after deploy (can scale down later)
	// Fixed mode uses the configured number of instances
	desiredInstances := int64(1)
	fixedMode := svcConcurrency.Mode == "fixed" && svcConcurrency.NumInstances > 0
	if fixedMode {
		desiredInstances = svcConcurrency.NumInstances
	}

	// Try to find existing pool with matching spec
	poolWithEntity, err := l.findMatchingPool(ctx, app.ID, serviceName, sbSpec)
	if err != nil {
		return "", fmt.Errorf("failed to find matching pool: %w", err)
	}

	if poolWithEntity != nil {
		// Reuse existing pool — sandboxes already running, no wait needed
		l.Log.Info("reusing existing pool",
			"pool", poolWithEntity.Pool.ID,
			"service", serviceName,
			"app", app.ID)

		needsUpdate := false

		// Update the pool's sandbox spec version to track the current AppVersion
		if poolWithEntity.Pool.SandboxSpec.Version != ver.ID {
			poolWithEntity.Pool.SandboxSpec.Version = ver.ID
			needsUpdate = true
		}

		// Add this version to referenced_by_versions if not already present
		if !containsRef(poolWithEntity.Pool.ReferencedByVersions, ver.ID) {
			poolWithEntity.Pool.ReferencedByVersions = append(poolWithEntity.Pool.ReferencedByVersions, ver.ID)
			needsUpdate = true
		}

		// For fixed mode services, update desired instances if they've changed
		// For auto mode, the activator manages desired instances - don't touch it
		if fixedMode && poolWithEntity.Pool.DesiredInstances != desiredInstances {
			poolWithEntity.Pool.DesiredInstances = desiredInstances
			l.Log.Info("fixed mode service, updating desired instances",
				"service", serviceName,
				"desired_instances", desiredInstances)
			needsUpdate = true
		}

		if needsUpdate {
			if err := l.updatePool(ctx, poolWithEntity); err != nil {
				return "", fmt.Errorf("failed to update pool: %w", err)
			}
		}

		// Return empty ID — existing pool already has running sandboxes
		return "", nil
	}

	// No matching pool found, create a new one
	l.Log.Info("creating new pool",
		"service", serviceName,
		"app", app.ID,
		"version", ver.Version)

	if fixedMode {
		l.Log.Info("fixed mode service, starting with desired instances",
			"service", serviceName,
			"desired_instances", desiredInstances)
	}

	// Use app metadata (already fetched earlier) for sandbox labels
	pool := &compute_v1alpha.SandboxPool{
		App:                  app.ID,
		Service:              serviceName,
		SandboxSpec:          *sbSpec,
		DesiredInstances:     desiredInstances,
		ReferencedByVersions: []entity.Id{ver.ID},
		SandboxLabels: types.LabelSet(
			"app", appMD.Name,
		),
		SandboxPrefix: fmt.Sprintf("%s-%s", appMD.Name, serviceName),
	}

	name := idgen.GenNS("pool")
	id := entity.Id("pool/" + name)

	pr, err := l.EAC.Create(ctx, entity.New(
		(&core_v1alpha.Metadata{
			Name: name,
			Labels: types.LabelSet(
				"app", app.ID.String(),
				"version", ver.Version,
				"service", serviceName,
			),
		}).Encode,
		entity.DBId, entity.Id("pool/"+name),
		pool.Encode,
	).Attrs())
	if err != nil {
		return "", fmt.Errorf("failed to create pool entity: %w", err)
	}

	pool.ID = id
	l.Log.Info("created new pool",
		"pool", pool.ID,
		"pr-id", pr.Id(),
		"service", serviceName,
		"desired_instances", desiredInstances)

	// Return the new pool ID — caller should wait for it to become ready
	return id, nil
}

// buildSandboxSpec creates a SandboxSpec for the given service
func (l *Launcher) buildSandboxSpec(
	ctx context.Context,
	app *core_v1alpha.App,
	ver *core_v1alpha.AppVersion,
	cfgSpec *core_v1alpha.ConfigSpec,
	serviceName string,
	image string,
) (
	*compute_v1alpha.SandboxSpec,
	error,
) {
	// Get app metadata
	appResp, err := l.EAC.Get(ctx, app.ID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get app: %w", err)
	}

	var appMD core_v1alpha.Metadata
	appMD.Decode(appResp.Entity().Entity())

	sbSpec := &compute_v1alpha.SandboxSpec{
		Version:      ver.ID,
		LogEntity:    app.ID.String(),
		LogAttribute: types.LabelSet("stage", "app-run", "service", serviceName),
	}

	startDir := cfgSpec.StartDirectory
	if startDir == "" {
		startDir = "/app"
	}

	appCont := compute_v1alpha.SandboxSpecContainer{
		Name:  "app",
		Image: image,
		Env: []string{
			"MIREN_APP=" + appMD.Name,
			"MIREN_VERSION=" + ver.Version,
		},
		Directory: startDir,
	}

	// Determine port configuration from service config
	port := int64(0)
	portName := ""
	portType := ""
	shutdownTimeout := ""

	for _, svc := range cfgSpec.Services {
		if svc.Name == serviceName {
			if svc.Port > 0 {
				port = svc.Port
			}
			if svc.PortName != "" {
				portName = svc.PortName
			}
			if svc.PortType != "" {
				portType = svc.PortType
			}
			if svc.Concurrency.ShutdownTimeout != "" {
				shutdownTimeout = svc.Concurrency.ShutdownTimeout
			}
			break
		}
	}

	// Default to 3000 for web service if still no port configured
	if port == 0 && serviceName == "web" {
		port = 3000
	}

	// Add port configuration if a port was determined
	if port > 0 {
		if portName == "" {
			portName = "http"
		}
		if portType == "" {
			portType = "http"
		}

		appCont.Port = []compute_v1alpha.SandboxSpecContainerPort{
			{
				Port: port,
				Name: portName,
				Type: portType,
			},
		}
	}

	// Add user-supplied config env vars, stripping any system-managed keys
	envMap := make(map[string]string)
	for _, x := range cfgSpec.Variables {
		if !isSystemEnvVar(x.Key) {
			envMap[x.Key] = x.Value
		}
	}

	// Find and merge per-service env vars (these override global vars)
	for _, svc := range cfgSpec.Services {
		if svc.Name == serviceName {
			for _, x := range svc.Env {
				if !isSystemEnvVar(x.Key) {
					envMap[x.Key] = x.Value
				}
			}
			break
		}
	}

	// Convert map to env var slice
	for k, v := range envMap {
		appCont.Env = append(appCont.Env, k+"="+v)
	}

	// Append system-managed env vars last so they cannot be overridden
	if port > 0 {
		appCont.Env = append(appCont.Env, fmt.Sprintf("PORT=%d", port))
	}
	if labs.AdminAPI() && ver.AdminToken != "" {
		appCont.Env = append(appCont.Env, "ADMIN_TOKEN="+ver.AdminToken)
	}

	// Find service command
	for _, svc := range cfgSpec.Services {
		if svc.Name == serviceName && svc.Command != "" {
			if cfgSpec.Entrypoint != "" {
				appCont.Command = cfgSpec.Entrypoint + " " + svc.Command
			} else {
				appCont.Command = svc.Command
			}
			break
		}
	}

	// Add disk volumes and mounts for this service
	for _, svc := range cfgSpec.Services {
		if svc.Name == serviceName {
			if len(svc.Disks) > 0 {
				svcConcurrency, err := coreutil.GetServiceConcurrency(cfgSpec, serviceName)
				if err != nil {
					return nil, fmt.Errorf("failed to get service concurrency: %w", err)
				}

				if svcConcurrency.Mode != "fixed" {
					l.Log.Warn("skipping disk attachment for non-fixed service",
						"service", serviceName,
						"mode", svcConcurrency.Mode)
					break
				}
			}

			for _, disk := range svc.Disks {
				sbSpec.Volume = append(sbSpec.Volume, compute_v1alpha.SandboxSpecVolume{
					Name:         disk.Name,
					Provider:     "miren",
					DiskName:     disk.Name,
					MountPath:    disk.MountPath,
					ReadOnly:     disk.ReadOnly,
					SizeGb:       disk.SizeGb,
					Filesystem:   disk.Filesystem,
					LeaseTimeout: disk.LeaseTimeout,
				})

				appCont.Mount = append(appCont.Mount, compute_v1alpha.SandboxSpecContainerMount{
					Source:      disk.Name,
					Destination: disk.MountPath,
				})
			}
			break
		}
	}

	if shutdownTimeout != "" {
		appCont.ShutdownTimeout = shutdownTimeout
	}

	sbSpec.Container = []compute_v1alpha.SandboxSpecContainer{appCont}

	return sbSpec, nil
}

// findMatchingPool searches for an existing pool with matching spec
func (l *Launcher) findMatchingPool(ctx context.Context, appID entity.Id, serviceName string, desiredSpec *compute_v1alpha.SandboxSpec) (*PoolWithEntity, error) {
	// List all sandbox pools for this app
	poolsResp, err := l.EAC.List(ctx, entity.Ref(entity.EntityKind, compute_v1alpha.KindSandboxPool))
	if err != nil {
		return nil, fmt.Errorf("failed to list pools: %w", err)
	}

	// Scan for matching pool
	for _, ent := range poolsResp.Values() {
		var pool compute_v1alpha.SandboxPool
		pool.Decode(ent.Entity())

		// Check if this pool belongs to our app and service
		if pool.Service != serviceName {
			continue
		}

		// Get pool metadata to check app label
		var poolMeta core_v1alpha.Metadata
		poolMeta.Decode(ent.Entity())

		// Check if pool belongs to this app
		appLabel, _ := poolMeta.Labels.Get("app")
		if appLabel != appID.String() {
			continue
		}

		// Check if specs match
		reason, matches := l.specsMatch(&pool.SandboxSpec, desiredSpec)
		if matches {
			return &PoolWithEntity{
				Pool:   &pool,
				Entity: *ent.Entity(),
			}, nil
		}
		l.Log.Debug("Pool spec mismatch", "pool", pool.ID, "reason", reason)
	}

	return nil, nil
}

// specsMatch compares two SandboxSpecs, ignoring the version field
// Returns (reason, matches) where reason explains why specs don't match (empty if they do match)
func (l *Launcher) specsMatch(spec1, spec2 *compute_v1alpha.SandboxSpec) (string, bool) {
	// Quick checks first
	if len(spec1.Container) != len(spec2.Container) {
		return fmt.Sprintf("container count mismatch: %d vs %d", len(spec1.Container), len(spec2.Container)), false
	}

	// Compare containers
	for i := range spec1.Container {
		c1 := &spec1.Container[i]
		c2 := &spec2.Container[i]

		if c1.Name != c2.Name {
			return fmt.Sprintf("container[%d] name mismatch: %s vs %s", i, c1.Name, c2.Name), false
		}
		if c1.Image != c2.Image {
			return fmt.Sprintf("container[%d] image mismatch: %s vs %s", i, c1.Image, c2.Image), false
		}
		if c1.Command != c2.Command {
			return fmt.Sprintf("container[%d] command mismatch: %s vs %s", i, c1.Command, c2.Command), false
		}
		if c1.Directory != c2.Directory {
			return fmt.Sprintf("container[%d] directory mismatch: %s vs %s", i, c1.Directory, c2.Directory), false
		}

		// Compare env vars (order-independent)
		if !envVarsEqual(c1.Env, c2.Env) {
			return fmt.Sprintf("container[%d] environment variables mismatch", i), false
		}

		// Compare ports
		if !portsEqual(c1.Port, c2.Port) {
			return fmt.Sprintf("container[%d] ports mismatch", i), false
		}
	}

	// All fields match (excluding version)
	return "", true
}

// envVarsEqual compares two env var slices in an order-independent way,
// ignoring version-specific system env vars (MIREN_VERSION, MIREN_APP)
func envVarsEqual(env1, env2 []string) bool {
	// Filter out system env vars
	filtered1 := filterSystemEnvVars(env1)
	filtered2 := filterSystemEnvVars(env2)

	if len(filtered1) != len(filtered2) {
		return false
	}

	// Build map for O(n) comparison
	envMap := make(map[string]bool)
	for _, e := range filtered1 {
		envMap[e] = true
	}

	for _, e := range filtered2 {
		if !envMap[e] {
			return false
		}
	}

	return true
}

// isSystemEnvVar returns true if the given key is a system-managed env var
// that user config must not override.
func isSystemEnvVar(key string) bool {
	switch key {
	case "MIREN_VERSION", "MIREN_APP", "MIREN_INSTANCE_NUM", "PORT", "ADMIN_TOKEN":
		return true
	}
	return strings.HasPrefix(key, "MIREN_")
}

// filterSystemEnvVars filters out system-managed env vars that shouldn't affect pool reuse
func filterSystemEnvVars(envVars []string) []string {
	filtered := []string{}
	for _, e := range envVars {
		// Skip MIREN_VERSION, MIREN_APP, MIREN_INSTANCE_NUM, PORT, and ADMIN_TOKEN - these are set automatically
		if strings.HasPrefix(e, "MIREN_VERSION=") {
			continue
		}
		if strings.HasPrefix(e, "MIREN_APP=") {
			continue
		}
		if strings.HasPrefix(e, "MIREN_INSTANCE_NUM=") {
			continue
		}
		if strings.HasPrefix(e, "PORT=") {
			continue
		}
		if strings.HasPrefix(e, "ADMIN_TOKEN=") {
			continue
		}
		filtered = append(filtered, e)
	}
	return filtered
}

// portsEqual compares two port slices
func portsEqual(ports1, ports2 []compute_v1alpha.SandboxSpecContainerPort) bool {
	if len(ports1) != len(ports2) {
		return false
	}

	for i := range ports1 {
		p1 := &ports1[i]
		p2 := &ports2[i]

		if p1.Port != p2.Port ||
			p1.Name != p2.Name ||
			p1.Type != p2.Type {
			return false
		}
	}

	return true
}

// containsRef checks if a slice of refs contains a specific ref
func containsRef(refs []entity.Id, ref entity.Id) bool {
	for _, r := range refs {
		if r == ref {
			return true
		}
	}
	return false
}

// updatePool updates a pool entity in the store
func (l *Launcher) updatePool(ctx context.Context, poolWithEntity *PoolWithEntity) error {
	pool := poolWithEntity.Pool
	ent := poolWithEntity.Entity

	l.Log.Info("updating pool",
		"pool", pool.ID,
		"desired_instances", pool.DesiredInstances,
		"references", pool.ReferencedByVersions,
		"num_refs", len(pool.ReferencedByVersions))

	// Build new attributes from the pool
	newAttrs := pool.Encode()

	// Filter out ReferencedByVersions from encoded attrs - we'll add them separately
	// (pool.Encode() includes them, but we need explicit control to handle empty arrays)
	filteredAttrs := make([]entity.Attr, 0, len(newAttrs))
	for _, attr := range newAttrs {
		if attr.ID != compute_v1alpha.SandboxPoolReferencedByVersionsId {
			filteredAttrs = append(filteredAttrs, attr)
		}
	}
	newAttrs = filteredAttrs

	// Add critical fields that Encode() filters out
	// (Encode() uses entity.Empty() which filters out zero/empty values)

	// Always include DesiredInstances, even when 0 (for scale-down)
	if pool.DesiredInstances == 0 {
		newAttrs = append(newAttrs, entity.Int64(compute_v1alpha.SandboxPoolDesiredInstancesId, 0))
	}

	// Always include CurrentInstances, even when 0
	if pool.CurrentInstances == 0 {
		newAttrs = append(newAttrs, entity.Int64(compute_v1alpha.SandboxPoolCurrentInstancesId, 0))
	}

	// Always include ReadyInstances, even when 0
	if pool.ReadyInstances == 0 {
		newAttrs = append(newAttrs, entity.Int64(compute_v1alpha.SandboxPoolReadyInstancesId, 0))
	}

	// Build the final attribute list: metadata from existing + new pool attrs
	finalAttrs := make([]entity.Attr, 0, len(ent.Attrs())+len(newAttrs))

	// Collect IDs we're replacing (including multi-valued attrs we'll handle separately)
	replacingIDs := make(map[entity.Id]bool)
	for _, attr := range newAttrs {
		replacingIDs[attr.ID] = true
	}
	// Always replace ReferencedByVersions (even if empty) since we're explicitly setting them
	replacingIDs[compute_v1alpha.SandboxPoolReferencedByVersionsId] = true

	// Add existing attrs except those we're replacing
	for _, attr := range ent.Attrs() {
		if !replacingIDs[attr.ID] {
			finalAttrs = append(finalAttrs, attr)
		}
	}

	// Add all new attrs
	finalAttrs = append(finalAttrs, newAttrs...)

	// Now add ALL the references from the pool (multi-valued attribute)
	// NOTE: We can't use entity.Update() for multi-valued attributes because
	// entity.Set() replaces the first matching attribute instead of adding a new one.
	// This is why we manually append each reference.
	for _, ref := range pool.ReferencedByVersions {
		finalAttrs = append(finalAttrs, entity.Ref(compute_v1alpha.SandboxPoolReferencedByVersionsId, ref))
	}

	// Use Replace with the combined attributes (preserves metadata)
	_, err := l.EAC.Replace(ctx, finalAttrs, 0)
	if err != nil {
		return fmt.Errorf("failed to update pool: %w", err)
	}

	l.Log.Info("pool update successful", "pool", pool.ID)

	return nil
}

// waitForPoolReady polls the pool entity until ReadyInstances > 0 or the timeout expires.
// On timeout it returns an error, but the caller should proceed with cleanup anyway —
// the httpingress retry logic handles any remaining gap.
func (l *Launcher) waitForPoolReady(ctx context.Context, poolID entity.Id, timeout time.Duration) error {
	pollInterval := 2 * time.Second
	if timeout < pollInterval {
		pollInterval = timeout / 2
	}
	deadline := time.Now().Add(timeout)

	for {
		resp, err := l.EAC.Get(ctx, poolID.String())
		if err != nil {
			return fmt.Errorf("failed to get pool %s: %w", poolID, err)
		}

		var pool compute_v1alpha.SandboxPool
		pool.Decode(resp.Entity().Entity())

		if pool.ReadyInstances > 0 {
			l.Log.Info("new pool has ready instances",
				"pool", poolID,
				"ready_instances", pool.ReadyInstances)
			return nil
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("pool %s not ready after %s (ready_instances=%d, current_instances=%d): %w",
				poolID, timeout, pool.ReadyInstances, pool.CurrentInstances, context.DeadlineExceeded)
		}

		l.Log.Debug("waiting for pool to become ready",
			"pool", poolID,
			"ready_instances", pool.ReadyInstances,
			"current_instances", pool.CurrentInstances)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

// cleanupOldVersionPools removes old version references from pools and scales down unreferenced pools
func (l *Launcher) cleanupOldVersionPools(ctx context.Context, app *core_v1alpha.App, currentVersionID entity.Id) error {
	l.Log.Info("cleaning up old version pools",
		"app", app.ID,
		"current_version", currentVersionID)

	// List all pools
	poolsResp, err := l.EAC.List(ctx, entity.Ref(entity.EntityKind, compute_v1alpha.KindSandboxPool))
	if err != nil {
		return fmt.Errorf("failed to list pools: %w", err)
	}

	poolCount := len(poolsResp.Values())
	l.Log.Info("found pools to check", "count", poolCount)

	for _, ent := range poolsResp.Values() {
		var pool compute_v1alpha.SandboxPool
		pool.Decode(ent.Entity())

		// Get pool metadata to check app label
		var poolMeta core_v1alpha.Metadata
		poolMeta.Decode(ent.Entity())

		// Check if pool belongs to this app
		appLabel, _ := poolMeta.Labels.Get("app")
		if appLabel != app.ID.String() {
			l.Log.Debug("skipping pool for different app",
				"pool", pool.ID,
				"pool_app", appLabel,
				"our_app", app.ID)
			continue
		}

		l.Log.Info("checking pool for cleanup",
			"pool", pool.ID,
			"service", pool.Service,
			"references", pool.ReferencedByVersions)

		// Check if this pool is being used by the current version
		isUsedByCurrentVersion := containsRef(pool.ReferencedByVersions, currentVersionID)

		if isUsedByCurrentVersion {
			// Pool is being reused by current version - keep ALL references
			// Multiple versions may reference the same pool during rolling deployments
			l.Log.Info("pool is being reused by current version, keeping all references",
				"pool", pool.ID,
				"service", pool.Service)
			continue
		}

		// Pool is NOT being used by current version - remove old references and scale down
		updated := false

		for _, ref := range pool.ReferencedByVersions {
			// Remove any version references (they're all old versions since current version isn't using this pool)
			updated = true
			l.Log.Info("removing old version reference from unused pool",
				"pool", pool.ID,
				"service", pool.Service,
				"old_version", ref)
		}

		if !updated {
			continue
		}

		// Update pool with nil slice to ensure zero value is properly encoded
		// (empty slice []entity.Id{} might be filtered out by entity encoder)
		pool.ReferencedByVersions = nil

		// Scale to 0 since no versions reference this pool
		l.Log.Info("scaling down unreferenced pool",
			"pool", pool.ID,
			"service", pool.Service,
			"app", app.ID)
		pool.DesiredInstances = 0

		// Persist changes
		poolWithEntity := &PoolWithEntity{
			Pool:   &pool,
			Entity: *ent.Entity(),
		}
		err := l.updatePool(ctx, poolWithEntity)
		if err != nil {
			l.Log.Error("failed to update pool", "error", err, "pool", pool.ID)
			continue
		}
	}

	return nil
}
