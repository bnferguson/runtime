package app

import (
	"context"
	"fmt"
	"log/slog"

	"miren.dev/runtime/api/addon/addon_v1alpha"
	"miren.dev/runtime/api/app/app_v1alpha"
	"miren.dev/runtime/api/core/core_v1alpha"
	"miren.dev/runtime/api/entityserver"
	"miren.dev/runtime/pkg/addon"
	"miren.dev/runtime/pkg/entity"
	"miren.dev/runtime/pkg/idgen"
)

// AddonsServer implements the app_v1alpha.Addons RPC interface.
type AddonsServer struct {
	log      *slog.Logger
	ec       *entityserver.Client
	registry *addon.Registry
}

var _ app_v1alpha.Addons = &AddonsServer{}

func NewAddonsServer(log *slog.Logger, ec *entityserver.Client, registry *addon.Registry) *AddonsServer {
	return &AddonsServer{
		log:      log.With("module", "addons-rpc"),
		ec:       ec,
		registry: registry,
	}
}

func (s *AddonsServer) CreateInstance(ctx context.Context, state *app_v1alpha.AddonsCreateInstance) error {
	args := state.Args()
	appName := args.App()
	addonSpec := args.Addon()
	variantOverride := args.Variant()

	if appName == "" {
		return fmt.Errorf("app name is required")
	}
	if addonSpec == "" {
		return fmt.Errorf("addon name is required")
	}

	// Resolve addon and variant
	addonName, variantName, err := s.registry.ResolveAddonAndVariant(addonSpec)
	if err != nil {
		return err
	}
	if variantOverride != "" {
		variantName = variantOverride
	}

	// Look up the app entity
	var app core_v1alpha.App
	if err := s.ec.Get(ctx, appName, &app); err != nil {
		return fmt.Errorf("app %q not found: %w", appName, err)
	}

	// Look up the addon entity
	var addonEntity addon_v1alpha.Addon
	if err := s.ec.Get(ctx, addonName, &addonEntity); err != nil {
		return fmt.Errorf("addon %q not found: %w", addonName, err)
	}

	// Check for existing association (prevent duplicates)
	existing, err := s.ec.List(ctx, entity.Ref(addon_v1alpha.AddonAssociationAppId, app.ID))
	if err != nil {
		return fmt.Errorf("listing existing associations: %w", err)
	}
	for existing.Next() {
		var assoc addon_v1alpha.AddonAssociation
		existing.Read(&assoc)
		if assoc.Addon == addonEntity.ID {
			return fmt.Errorf("addon %q is already attached to app %q", addonName, appName)
		}
	}

	// Create AddonAssociation entity with status="pending"
	assoc := &addon_v1alpha.AddonAssociation{
		App:    app.ID,
		Addon:  addonEntity.ID,
		Variant: variantName,
		Status: "pending",
	}

	name := idgen.GenNS("addon-assoc")
	id, err := s.ec.Create(ctx, name, assoc)
	if err != nil {
		return fmt.Errorf("creating addon association: %w", err)
	}

	state.Results().SetId(string(id))
	s.log.Info("addon association created",
		"id", id,
		"app", appName,
		"addon", addonName,
		"variant", variantName,
	)

	return nil
}

func (s *AddonsServer) ListInstances(ctx context.Context, state *app_v1alpha.AddonsListInstances) error {
	appName := state.Args().App()
	if appName == "" {
		return fmt.Errorf("app name is required")
	}

	var app core_v1alpha.App
	if err := s.ec.Get(ctx, appName, &app); err != nil {
		return fmt.Errorf("app %q not found: %w", appName, err)
	}

	results, err := s.ec.List(ctx, entity.Ref(addon_v1alpha.AddonAssociationAppId, app.ID))
	if err != nil {
		return fmt.Errorf("listing addon associations: %w", err)
	}

	var addons []*app_v1alpha.AddonInstance
	for results.Next() {
		var assoc addon_v1alpha.AddonAssociation
		results.Read(&assoc)

		instance := &app_v1alpha.AddonInstance{}
		instance.SetId(string(assoc.ID))
		instance.SetName(addonNameFromRef(assoc.Addon))
		instance.SetAddon(string(assoc.Addon))
		instance.SetVariant(assoc.Variant)
		addons = append(addons, instance)
	}

	state.Results().SetAddons(addons)
	return nil
}

func (s *AddonsServer) DeleteInstance(ctx context.Context, state *app_v1alpha.AddonsDeleteInstance) error {
	appName := state.Args().App()
	addonName := state.Args().Name()

	if appName == "" {
		return fmt.Errorf("app name is required")
	}
	if addonName == "" {
		return fmt.Errorf("addon name is required")
	}

	var app core_v1alpha.App
	if err := s.ec.Get(ctx, appName, &app); err != nil {
		return fmt.Errorf("app %q not found: %w", appName, err)
	}

	// Find the association for this addon
	results, err := s.ec.List(ctx, entity.Ref(addon_v1alpha.AddonAssociationAppId, app.ID))
	if err != nil {
		return fmt.Errorf("listing addon associations: %w", err)
	}

	for results.Next() {
		var assoc addon_v1alpha.AddonAssociation
		results.Read(&assoc)

		if addonNameFromRef(assoc.Addon) == addonName {
			// Set status to deprovisioning so the controller handles cleanup
			if err := s.ec.Patch(ctx, assoc.ID, 0,
				entity.String(addon_v1alpha.AddonAssociationStatusId, "deprovisioning"),
			); err != nil {
				return fmt.Errorf("updating association status: %w", err)
			}

			s.log.Info("addon marked for deprovisioning",
				"association", assoc.ID,
				"app", appName,
				"addon", addonName,
			)
			return nil
		}
	}

	return fmt.Errorf("addon %q is not attached to app %q", addonName, appName)
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
