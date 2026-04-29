package commands

import (
	"errors"
	"fmt"

	"miren.dev/runtime/api/ingress"
	"miren.dev/runtime/api/ingress/ingress_v1alpha"
	"miren.dev/runtime/pkg/cond"
	"miren.dev/runtime/pkg/entity"
	"miren.dev/runtime/pkg/ui"
)

func RouteShow(ctx *Context, opts struct {
	Host    string `position:"0" usage:"Hostname of the route to show; omit and pass --default for the default route"`
	Default bool   `long:"default" description:"Show the default route (instead of a hostname)"`
	FormatOptions
	ConfigCentric
}) error {
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
			return err
		}
		if route == nil {
			return fmt.Errorf("route not found: %s", opts.Host)
		}
		routeLabel = opts.Host
	}

	protected := !entity.Empty(route.OidcProvider)
	var provider *ingress_v1alpha.OidcProvider
	if protected {
		provider = &ingress_v1alpha.OidcProvider{}
		if err := ic.GetEntityStore().GetById(ctx, route.OidcProvider, provider); err != nil {
			if !errors.Is(err, cond.ErrNotFound{}) {
				return fmt.Errorf("failed to get identity provider: %w", err)
			}
			provider = nil
		}
	}

	if opts.IsJSON() {
		type RouteJSON struct {
			Host            string              `json:"host"`
			App             string              `json:"app"`
			Default         bool                `json:"default"`
			Protected       bool                `json:"protected"`
			OIDCEnabled     bool                `json:"oidc_enabled"`
			ProviderName    string              `json:"provider_name,omitempty"`
			ProviderURL     string              `json:"provider_url,omitempty"`
			ProviderMissing bool                `json:"provider_missing,omitempty"`
			ClaimMappings   []map[string]string `json:"claim_mappings,omitempty"`
		}

		r := RouteJSON{
			Host:        routeLabel,
			App:         ui.CleanEntityID(string(route.App)),
			Default:     route.Default,
			Protected:   protected,
			OIDCEnabled: protected,
		}

		if protected {
			if provider != nil {
				r.ProviderName = provider.Name
				r.ProviderURL = provider.ProviderUrl
			} else {
				r.ProviderMissing = true
			}
			for _, m := range route.ClaimMappings {
				r.ClaimMappings = append(r.ClaimMappings, map[string]string{
					"claim":  m.Claim,
					"header": m.Header,
				})
			}
		}

		return PrintJSON(r)
	}

	ctx.Printf("Route: %s\n", routeLabel)
	ctx.Printf("  App:       %s\n", ui.CleanEntityID(string(route.App)))
	ctx.Printf("  Default:   %v\n", route.Default)
	ctx.Printf("  Protected: %v\n", protected)

	if protected {
		if provider != nil {
			ctx.Printf("  Provider:  %s (%s)\n", provider.Name, provider.ProviderUrl)
		} else {
			ctx.Printf("  Provider:  <missing — provider has been deleted>\n")
		}

		if len(route.ClaimMappings) > 0 {
			var rows []ui.Row
			for _, m := range route.ClaimMappings {
				rows = append(rows, ui.Row{m.Claim, m.Header})
			}

			headers := []string{"CLAIM", "HEADER"}
			columns := ui.AutoSizeColumns(headers, rows, ui.Columns().NoTruncate(0).NoTruncate(1))
			table := ui.NewTable(
				ui.WithTableTitle("Claim Mappings"),
				ui.WithColumns(columns),
				ui.WithRows(rows),
			)

			ctx.Printf("\n%s\n", table.Render())
		}
	}

	return nil
}
