package postgresql

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"miren.dev/runtime/pkg/addon"
	"miren.dev/runtime/pkg/saga"
)

func TestRegisterDedicatedSaga(t *testing.T) {
	registry := saga.NewRegistry()
	fw := &addon.ProviderFramework{}
	rc := &resultCapture{}

	err := RegisterDedicatedSaga(registry, fw, rc)
	require.NoError(t, err)

	def, ok := registry.Get("provision-dedicated-postgresql")
	require.True(t, ok)
	assert.Equal(t, "provision-dedicated-postgresql", def.Name)
	assert.Len(t, def.Actions, 8)
}

func TestRegisterDeprovisionDedicatedSaga(t *testing.T) {
	registry := saga.NewRegistry()
	fw := &addon.ProviderFramework{}

	err := RegisterDeprovisionDedicatedSaga(registry, fw)
	require.NoError(t, err)

	def, ok := registry.Get("deprovision-dedicated-postgresql")
	require.True(t, ok)
	assert.Equal(t, "deprovision-dedicated-postgresql", def.Name)
	assert.Len(t, def.Actions, 5)
}

func TestDedicatedSagaActionOrder(t *testing.T) {
	registry := saga.NewRegistry()
	fw := &addon.ProviderFramework{}
	rc := &resultCapture{}

	err := RegisterDedicatedSaga(registry, fw, rc)
	require.NoError(t, err)

	def, ok := registry.Get("provision-dedicated-postgresql")
	require.True(t, ok)

	// Verify that all expected actions are present
	expectedActions := []string{
		"generate-credentials",
		"create-postgres-server",
		"create-dedicated-pool",
		"wait-for-dedicated-pool",
		"create-dedicated-service",
		"wait-for-dedicated-service",
		"update-dedicated-server",
		"build-dedicated-result",
	}

	for _, name := range expectedActions {
		_, exists := def.Actions[name]
		assert.True(t, exists, "expected action %q to exist", name)
	}
}
