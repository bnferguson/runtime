package app

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"miren.dev/runtime/api/addon/addon_v1alpha"
	"miren.dev/runtime/api/app/app_v1alpha"
	"miren.dev/runtime/api/core/core_v1alpha"
	"miren.dev/runtime/api/entityserver"
	"miren.dev/runtime/pkg/addon"
	"miren.dev/runtime/pkg/entity"
	"miren.dev/runtime/pkg/entity/testutils"
	"miren.dev/runtime/pkg/rpc"
)

type mockProvider struct{}

func (m *mockProvider) LocalityMode() addon.LocalityMode {
	return addon.OnCluster
}

func (m *mockProvider) Provision(ctx context.Context, app addon.App, variant addon.Variant) (*addon.ProvisionResult, error) {
	return &addon.ProvisionResult{}, nil
}

func (m *mockProvider) AdjustEnvVars(ctx context.Context, result *addon.ProvisionResult, assoc addon.AddonAssociation, collisions []string) ([]addon.Variable, error) {
	return nil, nil
}

func (m *mockProvider) Deprovision(ctx context.Context, assoc addon.AddonAssociation) error {
	return nil
}

func setupAddonsTest(t *testing.T) (context.Context, *app_v1alpha.AddonsClient, *entityserver.Client) {
	t.Helper()

	ctx := context.Background()

	inmem, cleanup := testutils.NewInMemEntityServer(t)
	t.Cleanup(cleanup)

	ec := entityserver.NewClient(slog.Default(), inmem.EAC)

	registry := addon.NewRegistry()
	registry.Register("miren-postgresql", &mockProvider{}, addon.AddonDefinition{
		Name:           "miren-postgresql",
		DisplayName:    "PostgreSQL",
		DefaultVariant: "small",
		Variants: []addon.VariantDefinition{
			{Name: "small", Description: "Small"},
			{Name: "shared", Description: "Shared"},
		},
	})

	// Ensure addon entities exist
	require.NoError(t, registry.EnsureEntities(ctx, ec))

	server := NewAddonsServer(slog.Default(), ec, registry, nil)

	client := &app_v1alpha.AddonsClient{
		Client: rpc.LocalClient(app_v1alpha.AdaptAddons(server)),
	}

	return ctx, client, ec
}

func TestAddonsCreateInstance(t *testing.T) {
	ctx, client, ec := setupAddonsTest(t)

	// Create test app
	app := &core_v1alpha.App{}
	_, err := ec.Create(ctx, "myapp", app)
	require.NoError(t, err)

	// Create addon instance
	result, err := client.CreateInstance(ctx, "test", "miren-postgresql", "small", "myapp", "")
	require.NoError(t, err)
	assert.NotEmpty(t, result.Id())
}

func TestAddonsCreateInstanceDefaultVariant(t *testing.T) {
	ctx, client, ec := setupAddonsTest(t)

	app := &core_v1alpha.App{}
	_, err := ec.Create(ctx, "myapp", app)
	require.NoError(t, err)

	// Create with empty variant — should use default
	result, err := client.CreateInstance(ctx, "test", "miren-postgresql", "", "myapp", "")
	require.NoError(t, err)
	assert.NotEmpty(t, result.Id())
}

func TestAddonsCreateInstanceDuplicatePrevented(t *testing.T) {
	ctx, client, ec := setupAddonsTest(t)

	app := &core_v1alpha.App{}
	_, err := ec.Create(ctx, "myapp", app)
	require.NoError(t, err)

	// Create first instance
	_, err = client.CreateInstance(ctx, "test", "miren-postgresql", "small", "myapp", "")
	require.NoError(t, err)

	// Attempt duplicate
	_, err = client.CreateInstance(ctx, "test2", "miren-postgresql", "small", "myapp", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already attached")
}

func TestAddonsCreateInstanceUnknownAddon(t *testing.T) {
	ctx, client, ec := setupAddonsTest(t)

	app := &core_v1alpha.App{}
	_, err := ec.Create(ctx, "myapp", app)
	require.NoError(t, err)

	_, err = client.CreateInstance(ctx, "test", "miren-redis", "small", "myapp", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown addon")
}

func TestAddonsListInstances(t *testing.T) {
	ctx, client, ec := setupAddonsTest(t)

	app := &core_v1alpha.App{}
	_, err := ec.Create(ctx, "myapp", app)
	require.NoError(t, err)

	// List with no addons
	result, err := client.ListInstances(ctx, "myapp")
	require.NoError(t, err)
	assert.Empty(t, result.Addons())

	// Create an addon instance
	_, err = client.CreateInstance(ctx, "test", "miren-postgresql", "small", "myapp", "")
	require.NoError(t, err)

	// List again
	result, err = client.ListInstances(ctx, "myapp")
	require.NoError(t, err)
	assert.Len(t, result.Addons(), 1)
	assert.Equal(t, "miren-postgresql", result.Addons()[0].Name())
	assert.Equal(t, "small", result.Addons()[0].Variant())
}

func TestAddonsDeleteInstance(t *testing.T) {
	ctx, client, ec := setupAddonsTest(t)

	app := &core_v1alpha.App{}
	appID, err := ec.Create(ctx, "myapp", app)
	require.NoError(t, err)

	// Create an addon instance
	createResult, err := client.CreateInstance(ctx, "test", "miren-postgresql", "small", "myapp", "")
	require.NoError(t, err)

	// Delete it
	_, err = client.DeleteInstance(ctx, "myapp", "miren-postgresql")
	require.NoError(t, err)

	// Verify status changed to deprovisioning
	var assoc addon_v1alpha.AddonAssociation
	err = ec.GetById(ctx, entity.Id(createResult.Id()), &assoc)
	require.NoError(t, err)
	assert.Equal(t, "deprovisioning", assoc.Status)

	// Verify it's still associated with the app
	assert.Equal(t, appID, assoc.App)
}

func TestAddonsDeleteInstanceNotFound(t *testing.T) {
	ctx, client, ec := setupAddonsTest(t)

	app := &core_v1alpha.App{}
	_, err := ec.Create(ctx, "myapp", app)
	require.NoError(t, err)

	_, err = client.DeleteInstance(ctx, "myapp", "miren-postgresql")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not attached")
}
