package commands

import (
	"fmt"

	"miren.dev/runtime/api/ingress"
	"miren.dev/runtime/api/ingress/ingress_v1alpha"
	"miren.dev/runtime/pkg/entity"
	"miren.dev/runtime/pkg/ui"
)

func RouteShow(ctx *Context, opts struct {
	Host string `position:"0" usage:"Hostname of the route to show" required:"true"`
	FormatOptions
	ConfigCentric
}) error {
	if opts.Host == "" {
		return fmt.Errorf("host is required")
	}

	client, err := ctx.RPCClient("entities")
	if err != nil {
		return err
	}

	ic := ingress.NewClient(ctx.Log, client)

	route, err := ic.Lookup(ctx, opts.Host)
	if err != nil {
		return err
	}

	if route == nil {
		return fmt.Errorf("route not found: %s", opts.Host)
	}

	protected := !entity.Empty(route.OidcProvider)
	var provider *ingress_v1alpha.OidcProvider
	if protected {
		provider = &ingress_v1alpha.OidcProvider{}
		if err := ic.GetEntityStore().GetById(ctx, route.OidcProvider, provider); err != nil {
			return fmt.Errorf("failed to get identity provider: %w", err)
		}
	}

	if opts.IsJSON() {
		type RouteJSON struct {
			Host          string              `json:"host"`
			App           string              `json:"app"`
			Default       bool                `json:"default"`
			Protected     bool                `json:"protected"`
			ProviderName  string              `json:"provider_name,omitempty"`
			ProviderURL   string              `json:"provider_url,omitempty"`
			ClaimMappings []map[string]string `json:"claim_mappings,omitempty"`
		}

		r := RouteJSON{
			Host:      opts.Host,
			App:       ui.CleanEntityID(string(route.App)),
			Default:   route.Default,
			Protected: protected,
		}

		if protected && provider != nil {
			r.ProviderName = provider.Name
			r.ProviderURL = provider.ProviderUrl
			for _, m := range route.ClaimMappings {
				r.ClaimMappings = append(r.ClaimMappings, map[string]string{
					"claim":  m.Claim,
					"header": m.Header,
				})
			}
		}

		return PrintJSON(r)
	}

	ctx.Printf("Route: %s\n", opts.Host)
	ctx.Printf("  App:       %s\n", ui.CleanEntityID(string(route.App)))
	ctx.Printf("  Default:   %v\n", route.Default)
	ctx.Printf("  Protected: %v\n", protected)

	if protected && provider != nil {
		ctx.Printf("  Provider:  %s (%s)\n", provider.Name, provider.ProviderUrl)

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
