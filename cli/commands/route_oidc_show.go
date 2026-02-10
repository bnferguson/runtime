package commands

import (
	"fmt"

	"miren.dev/runtime/api/ingress"
	"miren.dev/runtime/api/ingress/ingress_v1alpha"
	"miren.dev/runtime/pkg/entity"
	"miren.dev/runtime/pkg/labs"
	"miren.dev/runtime/pkg/ui"
)

func RouteOidcShow(ctx *Context, opts struct {
	Host    string `position:"0" usage:"Hostname for the route (e.g., example.com)"`
	Default bool   `long:"default" description:"Show OIDC config for the default route"`
	FormatOptions
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
		if opts.IsJSON() {
			return PrintJSON(map[string]interface{}{
				"host":         routeLabel,
				"oidc_enabled": false,
				"provider":     nil,
			})
		}
		ctx.Printf("OIDC is not configured for route: %s\n", routeLabel)
		return nil
	}

	// Look up the OIDC provider
	var provider ingress_v1alpha.OidcProvider
	err = ic.GetEntityStore().GetById(ctx, route.OidcProvider, &provider)
	if err != nil {
		return fmt.Errorf("failed to get OIDC provider: %w", err)
	}

	// Display OIDC config
	if opts.IsJSON() {
		type OIDCConfigJSON struct {
			Host          string              `json:"host"`
			OIDCEnabled   bool                `json:"oidc_enabled"`
			ProviderName  string              `json:"provider_name"`
			ProviderURL   string              `json:"provider_url"`
			ClientID      string              `json:"client_id"`
			Scopes        string              `json:"scopes"`
			ClaimMappings []map[string]string `json:"claim_mappings,omitempty"`
		}

		var mappings []map[string]string
		for _, m := range route.ClaimMappings {
			mappings = append(mappings, map[string]string{
				"claim":  m.Claim,
				"header": m.Header,
			})
		}

		return PrintJSON(OIDCConfigJSON{
			Host:          routeLabel,
			OIDCEnabled:   true,
			ProviderName:  provider.Name,
			ProviderURL:   provider.ProviderUrl,
			ClientID:      provider.ClientId,
			Scopes:        provider.Scopes,
			ClaimMappings: mappings,
		})
	}

	items := []ui.NamedValue{
		ui.NewNamedValue("Route", routeLabel),
		ui.NewNamedValue("Enabled", true),
		ui.NewNamedValue("Provider", provider.Name),
		ui.NewNamedValue("Provider URL", provider.ProviderUrl),
		ui.NewNamedValue("Client ID", provider.ClientId),
		ui.NewNamedValue("Scopes", provider.Scopes),
	}

	ctx.Printf("%s\n", ui.NewNamedValueList(items).Render())

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

	return nil
}
