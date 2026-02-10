package commands

import (
	"fmt"

	"miren.dev/runtime/api/ingress"
	"miren.dev/runtime/api/ingress/ingress_v1alpha"
	"miren.dev/runtime/pkg/entity"
	"miren.dev/runtime/pkg/labs"
)

func RouteOidcDisable(ctx *Context, opts struct {
	Host    string `position:"0" usage:"Hostname for the route (e.g., example.com)"`
	Default bool   `long:"default" description:"Disable OIDC on the default route"`
	ConfigCentric
}) error {
	if !labs.RouteOIDC() {
		return fmt.Errorf("OIDC authentication for routes is disabled. Enable with MIREN_LABS=routeoidc")
	}

	if opts.Host == "" && !opts.Default {
		return fmt.Errorf("either a hostname or --default must be specified")
	}

	if opts.Host != "" && opts.Default {
		return fmt.Errorf("--default cannot be used with a hostname")
	}

	client, err := ctx.RPCClient("entities")
	if err != nil {
		return err
	}

	ic := ingress.NewClient(ctx.Log, client)

	var route *ingress_v1alpha.HttpRoute
	var routeLabel string

	if opts.Default {
		route, err = ic.LookupDefault(ctx)
		if err != nil {
			return fmt.Errorf("failed to lookup default route: %w", err)
		}
		if route == nil {
			return fmt.Errorf("no default route configured")
		}
		routeLabel = "default"
	} else {
		route, err = ic.Lookup(ctx, opts.Host)
		if err != nil {
			return fmt.Errorf("failed to lookup route: %w", err)
		}
		if route == nil {
			return fmt.Errorf("route not found for host: %s", opts.Host)
		}
		routeLabel = opts.Host
	}

	// Check if OIDC is configured
	if entity.Empty(route.OidcProvider) {
		ctx.Printf("OIDC is not configured for route: %s\n", routeLabel)
		return nil
	}

	// Detach OIDC provider
	_, err = ic.DetachOIDCProviderFromRoute(ctx, route)
	if err != nil {
		return fmt.Errorf("failed to disable OIDC: %w", err)
	}

	ctx.Printf("OIDC disabled for route: %s\n", routeLabel)
	return nil
}
