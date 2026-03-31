package mysql

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

	def, ok := registry.Get("provision-dedicated-mysql")
	require.True(t, ok)
	assert.Equal(t, "provision-dedicated-mysql", def.Name)
	assert.Len(t, def.Actions, 8)
}

func TestRegisterDeprovisionDedicatedSaga(t *testing.T) {
	registry := saga.NewRegistry()
	fw := &addon.ProviderFramework{}

	err := RegisterDeprovisionDedicatedSaga(registry, fw)
	require.NoError(t, err)

	def, ok := registry.Get("deprovision-dedicated-mysql")
	require.True(t, ok)
	assert.Equal(t, "deprovision-dedicated-mysql", def.Name)
	assert.Len(t, def.Actions, 5)
}

func TestDeprovisionDedicatedSagaOrder(t *testing.T) {
	registry := saga.NewRegistry()
	fw := &addon.ProviderFramework{}

	err := RegisterDeprovisionDedicatedSaga(registry, fw)
	require.NoError(t, err)

	def, ok := registry.Get("deprovision-dedicated-mysql")
	require.True(t, ok)

	order := def.ExecutionOrder()

	indexOf := func(name string) int {
		for i, n := range order {
			if n == name {
				return i
			}
		}
		t.Fatalf("action %q not found in order %v", name, order)
		return -1
	}

	assert.Less(t, indexOf("decode-dedicated-attrs"), indexOf("lookup-dedicated-server"),
		"decode-dedicated-attrs must come before lookup-dedicated-server")
	assert.Less(t, indexOf("lookup-dedicated-server"), indexOf("delete-dedicated-service"),
		"lookup-dedicated-server must come before delete-dedicated-service")
	assert.Less(t, indexOf("lookup-dedicated-server"), indexOf("delete-dedicated-pool"),
		"lookup-dedicated-server must come before delete-dedicated-pool")
	assert.Less(t, indexOf("delete-dedicated-pool"), indexOf("delete-dedicated-server-entity"),
		"delete-dedicated-pool must come before delete-dedicated-server-entity")
}

func TestDedicatedSagaActionOrder(t *testing.T) {
	registry := saga.NewRegistry()
	fw := &addon.ProviderFramework{}
	rc := &resultCapture{}

	err := RegisterDedicatedSaga(registry, fw, rc)
	require.NoError(t, err)

	def, ok := registry.Get("provision-dedicated-mysql")
	require.True(t, ok)

	expectedActions := []string{
		"generate-credentials",
		"create-mysql-server",
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
