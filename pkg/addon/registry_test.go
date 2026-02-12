package addon

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testDefinition() AddonDefinition {
	return AddonDefinition{
		Name:           "test-addon",
		DisplayName:    "Test Addon",
		Description:    "A test addon",
		DefaultVariant: "small",
		Variants: []VariantDefinition{
			{
				Name:        "small",
				Description: "Small variant",
				Details:     map[string]string{"CPU": "0.5"},
				Config:      map[string]string{"cpu": "500m"},
			},
			{
				Name:        "large",
				Description: "Large variant",
				Details:     map[string]string{"CPU": "2"},
				Config:      map[string]string{"cpu": "2000m"},
			},
		},
	}
}

type mockProvider struct{}

func (m *mockProvider) Provision(ctx context.Context, app App, variant Variant) (*ProvisionResult, error) {
	return &ProvisionResult{}, nil
}
func (m *mockProvider) AdjustEnvVars(ctx context.Context, result *ProvisionResult, assoc AddonAssociation, collisions []string) ([]Variable, error) {
	return nil, nil
}
func (m *mockProvider) Deprovision(ctx context.Context, assoc AddonAssociation) error {
	return nil
}

func TestRegistryRegisterAndGet(t *testing.T) {
	r := NewRegistry()
	def := testDefinition()
	provider := &mockProvider{}

	r.Register("test-addon", provider, def)

	p, d, ok := r.Get("test-addon")
	require.True(t, ok)
	assert.Equal(t, def.Name, d.Name)
	assert.Equal(t, def.DisplayName, d.DisplayName)
	assert.NotNil(t, p)
}

func TestRegistryGetNotFound(t *testing.T) {
	r := NewRegistry()

	_, _, ok := r.Get("nonexistent")
	assert.False(t, ok)
}

func TestRegistryListAddons(t *testing.T) {
	r := NewRegistry()
	r.Register("addon-a", &mockProvider{}, AddonDefinition{Name: "addon-a"})
	r.Register("addon-b", &mockProvider{}, AddonDefinition{Name: "addon-b"})

	defs := r.ListAddons()
	assert.Len(t, defs, 2)

	names := make(map[string]bool)
	for _, d := range defs {
		names[d.Name] = true
	}
	assert.True(t, names["addon-a"])
	assert.True(t, names["addon-b"])
}

func TestResolveAddonAndVariantExplicit(t *testing.T) {
	r := NewRegistry()
	r.Register("test-addon", &mockProvider{}, testDefinition())

	name, variant, err := r.ResolveAddonAndVariant("test-addon:large")
	require.NoError(t, err)
	assert.Equal(t, "test-addon", name)
	assert.Equal(t, "large", variant)
}

func TestResolveAddonAndVariantDefault(t *testing.T) {
	r := NewRegistry()
	r.Register("test-addon", &mockProvider{}, testDefinition())

	name, variant, err := r.ResolveAddonAndVariant("test-addon")
	require.NoError(t, err)
	assert.Equal(t, "test-addon", name)
	assert.Equal(t, "small", variant) // default variant
}

func TestResolveAddonAndVariantUnknownAddon(t *testing.T) {
	r := NewRegistry()

	_, _, err := r.ResolveAddonAndVariant("unknown")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown addon")
}

func TestResolveAddonAndVariantUnknownVariant(t *testing.T) {
	r := NewRegistry()
	r.Register("test-addon", &mockProvider{}, testDefinition())

	_, _, err := r.ResolveAddonAndVariant("test-addon:nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown variant")
}

func TestGetVariantConfig(t *testing.T) {
	r := NewRegistry()
	r.Register("test-addon", &mockProvider{}, testDefinition())

	config, err := r.GetVariantConfig("test-addon", "small")
	require.NoError(t, err)
	assert.Equal(t, "500m", config["cpu"])

	config, err = r.GetVariantConfig("test-addon", "large")
	require.NoError(t, err)
	assert.Equal(t, "2000m", config["cpu"])
}

func TestGetVariantConfigUnknownAddon(t *testing.T) {
	r := NewRegistry()

	_, err := r.GetVariantConfig("unknown", "small")
	assert.Error(t, err)
}

func TestGetVariantConfigUnknownVariant(t *testing.T) {
	r := NewRegistry()
	r.Register("test-addon", &mockProvider{}, testDefinition())

	_, err := r.GetVariantConfig("test-addon", "nonexistent")
	assert.Error(t, err)
}
