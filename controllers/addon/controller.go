package addon

import (
	"context"
	"fmt"
	"log/slog"

	"miren.dev/runtime/api/addon/addon_v1alpha"
	"miren.dev/runtime/api/core/core_v1alpha"
	"miren.dev/runtime/api/entityserver"
	"miren.dev/runtime/api/entityserver/entityserver_v1alpha"
	"miren.dev/runtime/pkg/addon"
	"miren.dev/runtime/pkg/entity"
)

// Controller reconciles AddonAssociation entities, driving provisioning
// and deprovisioning of addons through their providers.
//
// Implements controller.ReconcileControllerI[*addon_v1alpha.AddonAssociation]
type Controller struct {
	log      *slog.Logger
	ec       *entityserver.Client
	eac      *entityserver_v1alpha.EntityAccessClient
	registry *addon.Registry
}

// NewController creates a new addon controller.
func NewController(
	log *slog.Logger,
	ec *entityserver.Client,
	eac *entityserver_v1alpha.EntityAccessClient,
	registry *addon.Registry,
) *Controller {
	return &Controller{
		log:      log.With("module", "addon"),
		ec:       ec,
		eac:      eac,
		registry: registry,
	}
}

func (c *Controller) Init(ctx context.Context) error {
	c.log.Info("initializing addon controller")
	return nil
}

func (c *Controller) Reconcile(ctx context.Context, assoc *addon_v1alpha.AddonAssociation, meta *entity.Meta) error {
	switch assoc.Status {
	case "pending":
		return c.provision(ctx, assoc, meta)
	case "deprovisioning":
		return c.deprovision(ctx, assoc, meta)
	case "active", "error":
		return nil
	default:
		return nil
	}
}

func (c *Controller) provision(ctx context.Context, assoc *addon_v1alpha.AddonAssociation, meta *entity.Meta) error {
	c.log.Info("provisioning addon", "association", assoc.ID, "addon", assoc.Addon, "plan", assoc.Plan)

	// Step 1: Set status to provisioning
	if err := meta.Update((&addon_v1alpha.AddonAssociation{Status: "provisioning"}).Encode()); err != nil {
		return fmt.Errorf("setting status to provisioning: %w", err)
	}

	// Resolve provider
	addonName := addonNameFromRef(assoc.Addon)
	provider, _, ok := c.registry.Get(addonName)
	if !ok {
		return c.setError(meta, fmt.Errorf("unknown addon %q", addonName))
	}

	// Resolve plan config
	planConfig, err := c.registry.GetPlanConfig(addonName, assoc.Plan)
	if err != nil {
		return c.setError(meta, fmt.Errorf("resolving plan config: %w", err))
	}

	// Look up the app to get its name
	appName, err := c.resolveAppName(ctx, assoc.App)
	if err != nil {
		return c.setError(meta, fmt.Errorf("resolving app name: %w", err))
	}

	// Step 2: Call provider.Provision
	result, err := provider.Provision(ctx, addon.App{
		ID:   assoc.App,
		Name: appName,
	}, addon.Plan{
		Name:   assoc.Plan,
		Config: planConfig,
	})
	if err != nil {
		return c.setError(meta, fmt.Errorf("provisioning: %w", err))
	}

	// Step 3: Append provider attrs to association entity
	if len(result.Attrs) > 0 {
		if err := meta.Update(result.Attrs); err != nil {
			return c.setError(meta, fmt.Errorf("appending provider attrs: %w", err))
		}
	}

	// Step 4: Check for env var collisions and adjust if needed
	existingVars, err := c.getAppVariables(ctx, assoc.App)
	if err != nil {
		return c.setError(meta, fmt.Errorf("getting existing app variables: %w", err))
	}

	envVars := result.EnvVars
	collisions := findCollisions(existingVars, envVars)
	if len(collisions) > 0 {
		adjusted, err := provider.AdjustEnvVars(ctx, result, addon.AddonAssociation{
			ID:     assoc.ID,
			App:    assoc.App,
			Addon:  assoc.Addon,
			Plan:   assoc.Plan,
			Entity: meta.Entity,
		}, collisions)
		if err != nil {
			return c.setError(meta, fmt.Errorf("adjusting env vars: %w", err))
		}
		envVars = adjusted
	}

	// Step 5: Inject env vars into app ConfigVersion and store on association
	if err := c.injectEnvVars(ctx, assoc.App, envVars); err != nil {
		return c.setError(meta, fmt.Errorf("injecting env vars: %w", err))
	}

	// Store variables on the association for later removal
	variables := make([]addon_v1alpha.Variables, len(envVars))
	for i, v := range envVars {
		variables[i] = addon_v1alpha.Variables{
			Key:       v.Key,
			Value:     v.Value,
			Sensitive: v.Sensitive,
		}
	}

	// Step 6: Set status to active
	if err := meta.Update((&addon_v1alpha.AddonAssociation{
		Status:    "active",
		Variables: variables,
	}).Encode()); err != nil {
		return fmt.Errorf("setting status to active: %w", err)
	}

	c.log.Info("addon provisioned", "association", assoc.ID)
	return nil
}

func (c *Controller) deprovision(ctx context.Context, assoc *addon_v1alpha.AddonAssociation, meta *entity.Meta) error {
	c.log.Info("deprovisioning addon", "association", assoc.ID, "addon", assoc.Addon)

	// Resolve provider
	addonName := addonNameFromRef(assoc.Addon)
	provider, _, ok := c.registry.Get(addonName)
	if !ok {
		return c.setError(meta, fmt.Errorf("unknown addon %q", addonName))
	}

	// Step 1: Call provider.Deprovision
	err := provider.Deprovision(ctx, addon.AddonAssociation{
		ID:     assoc.ID,
		App:    assoc.App,
		Addon:  assoc.Addon,
		Plan:   assoc.Plan,
		Entity: meta.Entity,
	})
	if err != nil {
		return c.setError(meta, fmt.Errorf("deprovisioning: %w", err))
	}

	// Step 2: Remove addon env vars from app ConfigVersion
	if err := c.removeEnvVars(ctx, assoc.App, assoc.Variables); err != nil {
		c.log.Warn("failed to remove addon env vars", "error", err)
	}

	// Step 3: Delete the association entity
	if err := c.ec.Delete(ctx, assoc.ID); err != nil {
		return fmt.Errorf("deleting association: %w", err)
	}

	c.log.Info("addon deprovisioned", "association", assoc.ID)
	return nil
}

func (c *Controller) setError(meta *entity.Meta, err error) error {
	c.log.Error("addon error", "error", err)

	updateErr := meta.Update((&addon_v1alpha.AddonAssociation{
		Status:       "error",
		ErrorMessage: err.Error(),
	}).Encode())
	if updateErr != nil {
		return fmt.Errorf("setting error status: %w (original: %w)", updateErr, err)
	}

	return nil
}

// resolveAppName looks up an App entity and returns its metadata name.
func (c *Controller) resolveAppName(ctx context.Context, appID entity.Id) (string, error) {
	var meta core_v1alpha.Metadata
	if err := c.ec.GetById(ctx, appID, &meta); err != nil {
		return "", fmt.Errorf("getting app entity: %w", err)
	}
	if meta.Name == "" {
		return string(appID), nil
	}
	return meta.Name, nil
}

// getAppVariables fetches the current variables from the app's active version.
func (c *Controller) getAppVariables(ctx context.Context, appID entity.Id) ([]core_v1alpha.Variable, error) {
	var app core_v1alpha.App
	if err := c.ec.GetById(ctx, appID, &app); err != nil {
		return nil, fmt.Errorf("getting app: %w", err)
	}
	if app.ActiveVersion == "" {
		return nil, nil
	}

	var version core_v1alpha.AppVersion
	if err := c.ec.GetById(ctx, app.ActiveVersion, &version); err != nil {
		return nil, fmt.Errorf("getting app version: %w", err)
	}
	return version.Config.Variable, nil
}

// injectEnvVars adds addon environment variables to the app's active version config.
func (c *Controller) injectEnvVars(ctx context.Context, appID entity.Id, envVars []addon.Variable) error {
	if len(envVars) == 0 {
		return nil
	}

	var app core_v1alpha.App
	if err := c.ec.GetById(ctx, appID, &app); err != nil {
		return fmt.Errorf("getting app: %w", err)
	}
	if app.ActiveVersion == "" {
		return fmt.Errorf("app has no active version")
	}

	var version core_v1alpha.AppVersion
	if err := c.ec.GetById(ctx, app.ActiveVersion, &version); err != nil {
		return fmt.Errorf("getting app version: %w", err)
	}

	// Merge addon vars into existing variables.
	// Addon vars use source="addon" and never override manual vars.
	merged := mergeAddonVars(version.Config.Variable, envVars)
	version.Config.Variable = merged

	// Patch the AppVersion with the updated config
	return c.ec.Patch(ctx, app.ActiveVersion, 0,
		entity.Component(core_v1alpha.AppVersionConfigId, version.Config.Encode()),
	)
}

// removeEnvVars removes addon-sourced variables from the app's active version config.
func (c *Controller) removeEnvVars(ctx context.Context, appID entity.Id, variables []addon_v1alpha.Variables) error {
	if len(variables) == 0 {
		return nil
	}

	var app core_v1alpha.App
	if err := c.ec.GetById(ctx, appID, &app); err != nil {
		return fmt.Errorf("getting app: %w", err)
	}
	if app.ActiveVersion == "" {
		return nil
	}

	var version core_v1alpha.AppVersion
	if err := c.ec.GetById(ctx, app.ActiveVersion, &version); err != nil {
		return fmt.Errorf("getting app version: %w", err)
	}

	// Build set of keys to remove
	removeKeys := make(map[string]bool, len(variables))
	for _, v := range variables {
		removeKeys[v.Key] = true
	}

	// Filter out addon vars that match the keys
	var filtered []core_v1alpha.Variable
	for _, v := range version.Config.Variable {
		if removeKeys[v.Key] && v.Source == "addon" {
			continue
		}
		filtered = append(filtered, v)
	}

	version.Config.Variable = filtered

	return c.ec.Patch(ctx, app.ActiveVersion, 0,
		entity.Component(core_v1alpha.AppVersionConfigId, version.Config.Encode()),
	)
}

// mergeAddonVars merges addon-contributed variables into the existing variable set.
// Addon vars (source="addon") never override manual vars.
func mergeAddonVars(existing []core_v1alpha.Variable, addonVars []addon.Variable) []core_v1alpha.Variable {
	varMap := make(map[string]core_v1alpha.Variable, len(existing))

	for _, v := range existing {
		varMap[v.Key] = v
	}

	for _, v := range addonVars {
		source := ""
		if ev, ok := varMap[v.Key]; ok {
			source = ev.Source
			if source == "" {
				source = "manual"
			}
		}

		// Never override manual vars
		if source == "manual" {
			continue
		}

		varMap[v.Key] = core_v1alpha.Variable{
			Key:       v.Key,
			Value:     v.Value,
			Sensitive: v.Sensitive,
			Source:    "addon",
		}
	}

	result := make([]core_v1alpha.Variable, 0, len(varMap))
	for _, v := range varMap {
		result = append(result, v)
	}
	return result
}

// findCollisions returns keys that exist in both existing vars and addon vars.
func findCollisions(existing []core_v1alpha.Variable, addonVars []addon.Variable) []string {
	existingKeys := make(map[string]bool, len(existing))
	for _, v := range existing {
		existingKeys[v.Key] = true
	}

	var collisions []string
	for _, v := range addonVars {
		if existingKeys[v.Key] {
			collisions = append(collisions, v.Key)
		}
	}
	return collisions
}

// addonNameFromRef extracts the addon name from an entity ref like "addon/miren-postgresql".
func addonNameFromRef(ref entity.Id) string {
	s := string(ref)
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '/' {
			return s[i+1:]
		}
	}
	return s
}
