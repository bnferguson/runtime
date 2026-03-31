package dbsaga

import (
	"context"
	"log/slog"

	"miren.dev/runtime/pkg/addon"
)

// BaseProvider holds fields and methods common to all on-cluster database
// addon providers. Embed it in provider structs to avoid duplicating
// LocalityMode and AdjustEnvVars.
type BaseProvider struct {
	Fw  *addon.ProviderFramework
	Log *slog.Logger
}

func (p *BaseProvider) LocalityMode() addon.LocalityMode {
	return addon.OnCluster
}

func (p *BaseProvider) AdjustEnvVars(_ context.Context, result *addon.ProvisionResult, _ addon.AddonAssociation, _ []string) ([]addon.Variable, error) {
	return result.EnvVars, nil
}
