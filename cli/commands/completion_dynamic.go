package commands

import (
	"os"
	"strings"
	"time"

	"miren.dev/runtime/api/addon/addon_v1alpha"
	"miren.dev/runtime/api/app/app_v1alpha"
	"miren.dev/runtime/api/compute"
	"miren.dev/runtime/api/compute/compute_v1alpha"
	"miren.dev/runtime/api/entityserver/entityserver_v1alpha"
	"miren.dev/runtime/api/ingress"
	"miren.dev/runtime/api/runner/runner_v1alpha"
	"miren.dev/runtime/clientconfig"
	"miren.dev/runtime/pkg/rpc"
)

// completionTimeout bounds the server round-trips a single completion may make.
const completionTimeout = 1500 * time.Millisecond

// defaultCompletionServerAddress mirrors the GlobalFlags default and is used
// when no cluster is configured but a local server may still be running.
const defaultCompletionServerAddress = "127.0.0.1:8443"

// completionNoNetworkEnv disables every server-backed resolver when set, for
// users who would rather not pay for round-trips while completing.
const completionNoNetworkEnv = "MIREN_COMPLETION_NO_NETWORK"

// loadCompletionConfig populates the Context with client and cluster config so
// the dynamic resolvers can reach the server. It is best-effort: when nothing
// is configured the resolvers simply return no suggestions.
func loadCompletionConfig(ctx *Context) {
	cfg, err := clientconfig.LoadConfig()
	if err != nil {
		return
	}
	ctx.ClientConfig = cfg

	name := cfg.ActiveCluster()
	if name == "" {
		return
	}
	if cluster, err := cfg.GetCluster(name); err == nil {
		ctx.ClusterConfig = cluster
		ctx.ClusterName = name
	}
}

// completeViaClient runs fn with an RPC client for the given service. Server-backed
// completion is best-effort: tab completion must feel instant, so it returns nil
// (no suggestions) when completion is disabled, the server is unreachable, or the
// deadline fires. The work runs in a goroutine selected against ctx so a hung dial
// can never block the shell; this relies on ctx carrying a deadline, which
// newCompletionContext always sets. The short-lived process exits right after
// printing, so the goroutine is not a meaningful leak.
func completeViaClient(ctx *Context, service string, fn func(*rpc.NetworkClient) []string) []string {
	if os.Getenv(completionNoNetworkEnv) != "" {
		return nil
	}

	result := make(chan []string, 1)
	go func() {
		client, err := ctx.RPCClient(service)
		if err != nil {
			result <- nil
			return
		}
		defer client.Close()
		result <- fn(client)
	}()

	select {
	case <-ctx.Done():
		return nil
	case values := <-result:
		return values
	}
}

func resolveAppNames(ctx *Context) []string {
	return completeViaClient(ctx, "dev.miren.runtime/app", func(client *rpc.NetworkClient) []string {
		res, err := app_v1alpha.NewCrudClient(client).List(ctx)
		if err != nil {
			return nil
		}
		apps := res.Apps()
		names := make([]string, 0, len(apps))
		for _, a := range apps {
			names = append(names, a.Name())
		}
		return names
	})
}

func resolveRouteHosts(ctx *Context) []string {
	return completeViaClient(ctx, "entities", func(client *rpc.NetworkClient) []string {
		routes, err := ingress.NewClient(ctx.Log, client).List(ctx)
		if err != nil {
			return nil
		}
		var hosts []string
		for _, r := range routes {
			if r.Route.Host != "" { // the default route has no host
				hosts = append(hosts, r.Route.Host)
			}
		}
		return hosts
	})
}

func resolveSandboxIDs(ctx *Context) []string {
	return completeViaClient(ctx, "entities", func(client *rpc.NetworkClient) []string {
		eac := entityserver_v1alpha.NewEntityAccessClient(client)
		kind, err := eac.LookupKind(ctx, "sandbox")
		if err != nil {
			return nil
		}
		res, err := eac.List(ctx, kind.Attr())
		if err != nil {
			return nil
		}
		var ids []string
		for _, e := range res.Values() {
			var sandbox compute_v1alpha.Sandbox
			sandbox.Decode(e.Entity())
			if compute.SandboxDead(sandbox.Status) {
				continue
			}
			ids = append(ids, sandbox.ID.String())
		}
		return ids
	})
}

func resolvePoolIDs(ctx *Context) []string {
	return completeViaClient(ctx, "entities", func(client *rpc.NetworkClient) []string {
		eac := entityserver_v1alpha.NewEntityAccessClient(client)
		kind, err := eac.LookupKind(ctx, "sandbox_pool")
		if err != nil {
			return nil
		}
		res, err := eac.List(ctx, kind.Attr())
		if err != nil {
			return nil
		}
		var ids []string
		for _, e := range res.Values() {
			var pool compute_v1alpha.SandboxPool
			pool.Decode(e.Entity())
			ids = append(ids, pool.ID.String())
		}
		return ids
	})
}

func resolveAddonNames(ctx *Context) []string {
	return completeViaClient(ctx, "entities", func(client *rpc.NetworkClient) []string {
		eac := entityserver_v1alpha.NewEntityAccessClient(client)
		kind, err := eac.LookupKind(ctx, "addon")
		if err != nil {
			return nil
		}
		res, err := eac.List(ctx, kind.Attr())
		if err != nil {
			return nil
		}
		var names []string
		for _, e := range res.Values() {
			var addon addon_v1alpha.Addon
			addon.Decode(e.Entity())
			names = append(names, addon.Name)
		}
		return names
	})
}

func resolveRunnerNodes(ctx *Context) []string {
	return completeViaClient(ctx, rpc.ServiceRunner, func(client *rpc.NetworkClient) []string {
		res, err := runner_v1alpha.NewRunnerRegistrationClient(client).ListRunners(ctx)
		if err != nil {
			return nil
		}
		var names []string
		for _, r := range res.Runners() {
			if name := r.Name(); name != "" {
				names = append(names, name)
			} else {
				names = append(names, r.RunnerId())
			}
		}
		return names
	})
}

func resolveTokenIDs(ctx *Context) []string {
	return completeViaClient(ctx, rpc.ServiceRunner, func(client *rpc.NetworkClient) []string {
		res, err := runner_v1alpha.NewRunnerRegistrationClient(client).ListInvites(ctx)
		if err != nil {
			return nil
		}
		var ids []string
		for _, inv := range res.Invites() {
			// Already-revoked or expired tokens can't be revoked again.
			switch strings.TrimPrefix(inv.Status(), "status.") {
			case "revoked", "expired":
				continue
			}
			ids = append(ids, inv.Id())
		}
		return ids
	})
}
