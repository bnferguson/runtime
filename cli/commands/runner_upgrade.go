//go:build linux

package commands

import (
	"fmt"
	"os"
	"time"

	"miren.dev/runtime/pkg/release"
)

// RunnerUpgrade upgrades the miren runner to the latest or specified version
func RunnerUpgrade(ctx *Context, opts struct {
	Version        string `short:"V" long:"version" description:"Specific version to upgrade to (e.g., v0.2.0)"`
	Channel        string `long:"channel" description:"Channel to use: 'latest' (stable releases, default) or 'main' (bleeding edge)"`
	Check          bool   `short:"c" long:"check" description:"Check for available updates only"`
	Force          bool   `short:"f" long:"force" description:"Force upgrade even if already up to date"`
	SkipHealth     bool   `long:"skip-health" description:"Skip health check after upgrade"`
	NoAutoRollback bool   `long:"no-auto-rollback" description:"Disable automatic rollback on failure"`
	HealthTimeout  int    `long:"health-timeout" default:"60" description:"Health check timeout in seconds"`
}) error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("runner upgrade requires root privileges (use sudo)")
	}

	if !release.IsRunnerRunning() {
		return fmt.Errorf("miren runner is not running. Use 'miren upgrade' to upgrade the CLI binary instead")
	}

	version := opts.Version
	channel := opts.Channel

	if version != "" && channel != "" {
		return fmt.Errorf("--version and --channel are mutually exclusive")
	}

	if version == "" && channel == "" {
		channel = "latest"
	}
	if channel != "" {
		version = channel
	}

	mgrOpts := release.RunnerManagerOptions()
	mgrOpts.SkipHealthCheck = opts.SkipHealth
	mgrOpts.AutoRollback = !opts.NoAutoRollback
	if opts.HealthTimeout > 0 {
		mgrOpts.HealthTimeout = time.Duration(opts.HealthTimeout) * time.Second
	}

	if opts.Check {
		current, latest, err := CheckVersionStatus(ctx, version, &mgrOpts)
		if err != nil {
			return err
		}

		PrintVersionComparison(current, latest)

		if latest.IsNewer(current) {
			fmt.Println("\nAn update is available! Run 'sudo miren runner upgrade' to install it.")
		} else {
			fmt.Println("\nYour runner is already on the latest version.")
		}
		return nil
	}

	needsUpgrade, err := CheckIfUpgradeNeeded(ctx, version, opts.Force, &mgrOpts)
	if err != nil {
		ctx.Log.Warn("could not check version status", "error", err)
	} else if !needsUpgrade {
		return nil
	}

	mgr := release.NewManager(mgrOpts)

	current, err := mgr.GetCurrentVersion(ctx)
	if err != nil {
		ctx.Log.Warn("could not determine current version", "error", err)
	}

	artifact := release.NewArtifact(release.ArtifactTypeBase, version)

	fmt.Printf("Upgrading runner to %s version %s...\n", artifact.Type, version)
	if err := mgr.UpgradeServer(ctx, artifact); err != nil {
		return err
	}

	PrintUpgradeSuccess(ctx, current, "Runner", &mgrOpts)

	return nil
}

// RunnerUpgradeRollback rolls back the runner to the previous version
func RunnerUpgradeRollback(ctx *Context, opts struct {
	SkipHealth bool `long:"skip-health" description:"Skip health check after rollback"`
}) error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("runner rollback requires root privileges (use sudo)")
	}

	mgrOpts := release.RunnerManagerOptions()
	mgrOpts.SkipHealthCheck = opts.SkipHealth
	mgr := release.NewManager(mgrOpts)

	fmt.Println("Rolling back runner to previous version...")
	if err := mgr.Rollback(ctx); err != nil {
		return err
	}

	fmt.Println("\nRunner rollback successful!")
	return nil
}
