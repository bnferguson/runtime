package commands

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDynamicResolversRespectNoNetwork verifies that the opt-out short-circuits
// every server-backed resolver before any RPC client is built, so completion
// stays local-only and never touches the network when asked not to.
func TestDynamicResolversRespectNoNetwork(t *testing.T) {
	t.Setenv(completionNoNetworkEnv, "1")

	ctx := &Context{Context: context.Background()}
	resolvers := []struct {
		name string
		fn   valueResolver
	}{
		{"apps", resolveAppNames},
		{"routes", resolveRouteHosts},
		{"sandboxes", resolveSandboxIDs},
		{"pools", resolvePoolIDs},
		{"addons", resolveAddonNames},
		{"runners", resolveRunnerNodes},
		{"tokens", resolveTokenIDs},
	}

	for _, r := range resolvers {
		t.Run(r.name, func(t *testing.T) {
			assert.Nil(t, r.fn(ctx))
		})
	}
}
