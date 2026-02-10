package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"miren.dev/runtime/api/app/app_v1alpha"
	"miren.dev/runtime/api/deployment/deployment_v1alpha"
)

const rpcAppStatus = "dev.miren.runtime/app-status"

// appInfoGetter is an interface for getting app info status.
// The real implementation is app_v1alpha.AppStatusClient.
type appInfoGetter interface {
	AppInfo(ctx context.Context, application string) (*app_v1alpha.AppStatusClientAppInfoResults, error)
}

// waitForActivation polls AppInfo to wait for a specific version to become active.
// This is non-fatal — the deploy/rollback already succeeded server-side, so a
// timeout just means we can't confirm activation yet.
func waitForActivation(ctx *Context, getter appInfoGetter, appName, versionID string) {
	const (
		pollInterval = 2 * time.Second
		timeout      = 60 * time.Second
	)

	ctx.Printf("Waiting for version to become active...\n")

	deadline := time.After(timeout)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-deadline:
			ctx.Printf("⚠ Timed out waiting for version %s to activate. Activation is still in progress.\n", versionID)
			return
		case <-ticker.C:
			ready, summary := checkVersionActive(ctx, getter, appName, versionID)
			if ready {
				ctx.Printf("✓ Version %s is active%s\n", versionID, summary)
				return
			}
		}
	}
}

// checkVersionActive polls AppInfo once and checks if the target version is active.
// It checks ActiveVersion first, then looks at pool status for extra detail.
func checkVersionActive(ctx context.Context, getter appInfoGetter, appName, versionID string) (ready bool, summary string) {
	pollCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result, err := getter.AppInfo(pollCtx, appName)
	if err != nil {
		return false, ""
	}

	status := result.Status()
	if status == nil {
		return false, ""
	}

	// Normalize versionID: the CLI may pass an entity ID (e.g.
	// "app_version/go-server-vXXX") while AppInfo returns the short
	// version string ("go-server-vXXX"). Strip the kind prefix if present.
	shortVersion := versionID
	if idx := strings.LastIndex(shortVersion, "/"); idx >= 0 {
		shortVersion = shortVersion[idx+1:]
	}

	if status.ActiveVersion() != shortVersion {
		return false, ""
	}

	// Version is active. Build a summary of pool status.
	pools := status.Pools()
	if len(pools) == 0 {
		return true, ""
	}

	// Count total idle sandboxes across pools
	var totalIdle int32
	for _, pool := range pools {
		totalIdle += pool.Idle()
	}

	if totalIdle > 0 {
		return true, fmt.Sprintf(" — %d idle sandbox(es) ready", totalIdle)
	}

	return true, fmt.Sprintf(" — %d pool(s) assigned", len(pools))
}

// displayDeployVersionAccessInfo shows route/access information from the
// DeployVersion RPC response, mirroring the displayAccessInfo function
// used by the full deploy path.
func displayDeployVersionAccessInfo(ctx *Context, appName string, accessInfo *deployment_v1alpha.AccessInfo) {
	var hostnames []string
	if accessInfo.HasHostnames() && accessInfo.Hostnames() != nil {
		hostnames = *accessInfo.Hostnames()
	}
	hasDefaultRoute := accessInfo.DefaultRoute()

	var clusterAddr string
	if accessInfo.ClusterHostname() != "" {
		clusterAddr = accessInfo.ClusterHostname()
	} else if ctx.ClusterConfig != nil && ctx.ClusterConfig.Hostname != "" {
		clusterAddr = stripPort(ctx.ClusterConfig.Hostname)
	}

	if len(hostnames) > 0 {
		ctx.Printf("\nYour app is available at:\n")
		for _, host := range hostnames {
			ctx.Printf("  https://%s\n", host)
		}
		if hasDefaultRoute {
			ctx.Printf("  (also the default route)\n")
		}
	} else if hasDefaultRoute {
		if clusterAddr != "" {
			ctx.Printf("\nYour app is the default route, available at:\n")
			ctx.Printf("  https://%s\n", clusterAddr)
		} else {
			ctx.Printf("\nYour app is the default route and will receive all unmatched traffic.\n")
		}
		suggestRoute(ctx, appName, clusterAddr)
	} else {
		ctx.Printf("\nNo routes configured for this app.\n")
		suggestRoute(ctx, appName, clusterAddr)
		ctx.Printf("To make it the default route: miren route set-default %s\n", appName)
	}
}
