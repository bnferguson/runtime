package addon

import (
	"context"

	"miren.dev/runtime/pkg/entity"
)

// AddonProvider defines the interface that addon implementations must satisfy.
type AddonProvider interface {
	// Provision creates the backing resources for an addon and returns the
	// environment variables and entity attributes to store.
	Provision(ctx context.Context, app App, variant Variant) (*ProvisionResult, error)

	// AdjustEnvVars is called when provisioned env vars collide with existing
	// app env vars. The provider can rename or adjust variables.
	AdjustEnvVars(ctx context.Context, result *ProvisionResult, assoc AddonAssociation, collisions []string) ([]Variable, error)

	// Deprovision tears down the backing resources for an addon.
	Deprovision(ctx context.Context, assoc AddonAssociation) error
}

// App identifies the application an addon is being attached to.
type App struct {
	ID   entity.Id
	Name string
}

// Variant describes the variant selected for provisioning.
type Variant struct {
	Name   string
	Config map[string]string
}

// Variable represents an environment variable contributed by an addon.
type Variable struct {
	Key       string
	Value     string
	Sensitive bool
}

// ProvisionResult is returned by a provider after successful provisioning.
type ProvisionResult struct {
	EnvVars []Variable
	Attrs   []entity.Attr
}

// AddonAssociation holds the state needed for deprovisioning.
type AddonAssociation struct {
	ID      entity.Id
	App     entity.Id
	Addon   entity.Id
	Variant string
	Entity  *entity.Entity
}

// AddonDefinition describes an addon's metadata and available variants.
type AddonDefinition struct {
	Name           string
	DisplayName    string
	Description    string
	DefaultVariant string
	Variants       []VariantDefinition
}

// VariantDefinition describes a single variant within an addon.
type VariantDefinition struct {
	Name        string
	Description string
	Details     map[string]string // display key-value pairs shown to users
	Config      map[string]string // provider-internal configuration
}
