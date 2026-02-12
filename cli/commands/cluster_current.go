package commands

import (
	"fmt"

	"miren.dev/runtime/appconfig"
)

func ClusterCurrent(ctx *Context, opts struct {
	AppCentric
}) error {
	state, err := appconfig.LoadAppState(opts.App)
	if err != nil {
		return fmt.Errorf("failed to load app state: %w", err)
	}

	if state != nil && state.Cluster != "" {
		ctx.Printf("%s\n", state.Cluster)
		return nil
	}

	cfg, err := opts.LoadConfig()
	if err != nil {
		return fmt.Errorf("no cluster configured: %w", err)
	}

	active := cfg.ActiveCluster()
	if active == "" {
		ctx.Printf("No cluster configured\n")
		return nil
	}

	ctx.Printf("%s (global default)\n", active)
	return nil
}
