package addon

import (
	"context"
	"fmt"
	"strings"

	"miren.dev/runtime/api/addon/addon_v1alpha"
	"miren.dev/runtime/api/entityserver"
)

// Registry holds registered addon providers and their definitions.
type Registry struct {
	providers map[string]*registeredAddon
}

type registeredAddon struct {
	provider   AddonProvider
	definition AddonDefinition
}

// NewRegistry creates a new addon registry.
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]*registeredAddon),
	}
}

// Register adds an addon provider with its definition to the registry.
func (r *Registry) Register(name string, provider AddonProvider, def AddonDefinition) {
	r.providers[name] = &registeredAddon{
		provider:   provider,
		definition: def,
	}
}

// Get returns the provider and definition for the named addon.
func (r *Registry) Get(name string) (AddonProvider, AddonDefinition, bool) {
	ra, ok := r.providers[name]
	if !ok {
		return nil, AddonDefinition{}, false
	}
	return ra.provider, ra.definition, true
}

// ListAddons returns all registered addon definitions.
func (r *Registry) ListAddons() []AddonDefinition {
	var defs []AddonDefinition
	for _, ra := range r.providers {
		defs = append(defs, ra.definition)
	}
	return defs
}

// EnsureEntities creates or updates Addon entities in the entity store
// for each registered addon, so they can be discovered by the CLI and API.
func (r *Registry) EnsureEntities(ctx context.Context, ec *entityserver.Client) error {
	for name, ra := range r.providers {
		def := ra.definition

		var plans []addon_v1alpha.Plans
		for _, pd := range def.Plans {
			var details []addon_v1alpha.Details
			for k, v := range pd.Details {
				details = append(details, addon_v1alpha.Details{
					Key:   k,
					Value: v,
				})
			}
			plans = append(plans, addon_v1alpha.Plans{
				Name:        pd.Name,
				Description: pd.Description,
				Details:     details,
			})
		}

		addonEntity := &addon_v1alpha.Addon{
			Name:        name,
			DisplayName: def.DisplayName,
			Description: def.Description,
			DefaultPlan: def.DefaultPlan,
			Plans:       plans,
		}

		_, err := ec.CreateOrUpdate(ctx, name, addonEntity)
		if err != nil {
			return fmt.Errorf("ensuring addon entity %q: %w", name, err)
		}
	}
	return nil
}

// ResolveAddonAndPlan parses a spec like "miren-postgresql:small-local" into
// addon name and plan name. If no plan is specified, the default plan is used.
func (r *Registry) ResolveAddonAndPlan(spec string) (addonName, planName string, err error) {
	parts := strings.SplitN(spec, ":", 2)
	addonName = parts[0]

	_, def, ok := r.Get(addonName)
	if !ok {
		return "", "", fmt.Errorf("unknown addon %q", addonName)
	}

	if len(parts) == 2 {
		planName = parts[1]
	} else {
		planName = def.DefaultPlan
	}

	// Validate the plan exists
	for _, p := range def.Plans {
		if p.Name == planName {
			return addonName, planName, nil
		}
	}

	return "", "", fmt.Errorf("unknown plan %q for addon %q", planName, addonName)
}

// GetPlanConfig returns the provider-internal config for a specific plan.
func (r *Registry) GetPlanConfig(addonName, planName string) (map[string]string, error) {
	_, def, ok := r.Get(addonName)
	if !ok {
		return nil, fmt.Errorf("unknown addon %q", addonName)
	}

	for _, p := range def.Plans {
		if p.Name == planName {
			return p.Config, nil
		}
	}

	return nil, fmt.Errorf("unknown plan %q for addon %q", planName, addonName)
}
