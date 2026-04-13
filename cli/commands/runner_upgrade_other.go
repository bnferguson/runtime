//go:build !linux

package commands

import "fmt"

// RunnerUpgrade is not supported on non-Linux platforms
func RunnerUpgrade(ctx *Context, opts struct {
	Version        string `short:"V" long:"version" description:"Specific version to upgrade to (e.g., v0.2.0)"`
	Channel        string `long:"channel" description:"Channel to use: 'latest' (stable releases, default) or 'main' (bleeding edge)"`
	Check          bool   `short:"c" long:"check" description:"Check for available updates only"`
	Force          bool   `short:"f" long:"force" description:"Force upgrade even if already up to date"`
	SkipHealth     bool   `long:"skip-health" description:"Skip health check after upgrade"`
	NoAutoRollback bool   `long:"no-auto-rollback" description:"Disable automatic rollback on failure"`
	HealthTimeout  int    `long:"health-timeout" default:"60" description:"Health check timeout in seconds"`
}) error {
	return fmt.Errorf("runner upgrade is only available on Linux")
}

// RunnerUpgradeRollback is not supported on non-Linux platforms
func RunnerUpgradeRollback(ctx *Context, opts struct {
	SkipHealth bool `long:"skip-health" description:"Skip health check after rollback"`
}) error {
	return fmt.Errorf("runner upgrade rollback is only available on Linux")
}
