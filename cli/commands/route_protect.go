package commands

import (
	"fmt"
	"strings"

	"miren.dev/runtime/api/ingress"
	"miren.dev/runtime/api/ingress/ingress_v1alpha"
	"miren.dev/runtime/pkg/ui"
)

func RouteProtect(ctx *Context, opts struct {
	Host        string   `position:"0" usage:"Hostname for the route (e.g., example.com); omit and pass --default for the default route"`
	Default     bool     `long:"default" description:"Protect the default route (instead of a hostname)"`
	Provider    string   `long:"provider" description:"Name of the identity provider" required:"true"`
	ClaimHeader []string `long:"claim-header" description:"Claim to header mapping in format 'claim:header' (e.g., 'email:X-User-Email')"`
	FormatOptions
	ConfigCentric
}) error {
	if opts.Host == "" && !opts.Default {
		return fmt.Errorf("either a hostname or --default must be specified")
	}

	if opts.Host != "" && opts.Default {
		return fmt.Errorf("--default cannot be used with a hostname")
	}

	if opts.Provider == "" {
		return fmt.Errorf("--provider is required")
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

	// Auto-detect provider type: try OIDC first, then password
	oidcProvider, err := ic.GetOIDCProvider(ctx, opts.Provider)
	if err != nil {
		return fmt.Errorf("failed to lookup provider: %w", err)
	}

	if oidcProvider != nil {
		// OIDC provider found — attach with claim mappings
		var claimMappings []ingress_v1alpha.ClaimMappings
		for _, mapping := range opts.ClaimHeader {
			parts := strings.SplitN(mapping, ":", 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid claim-header mapping format: %s (expected 'claim:header')", mapping)
			}
			claim := strings.TrimSpace(parts[0])
			header := strings.TrimSpace(parts[1])
			if claim == "" || header == "" {
				return fmt.Errorf("invalid claim-header mapping format: %q (expected non-empty 'claim:header')", mapping)
			}
			claimMappings = append(claimMappings, ingress_v1alpha.ClaimMappings{
				Claim:  claim,
				Header: header,
			})
		}

		_, err = ic.AttachAuthProviderToRoute(ctx, route, oidcProvider.ID, claimMappings)
		if err != nil {
			return fmt.Errorf("failed to protect route: %w", err)
		}

		providerType := "oidc"
		if isConnector(oidcProvider) {
			providerType = "connector"
		}

		if opts.IsJSON() {
			type RouteProtectJSON struct {
				Route         string              `json:"route"`
				Protected     bool                `json:"protected"`
				Provider      string              `json:"provider"`
				ProviderType  string              `json:"provider_type"`
				ConnectorType string              `json:"connector_type,omitempty"`
				ClaimMappings []map[string]string `json:"claim_mappings,omitempty"`
			}

			var mappings []map[string]string
			for _, m := range claimMappings {
				mappings = append(mappings, map[string]string{
					"claim":  m.Claim,
					"header": m.Header,
				})
			}

			out := RouteProtectJSON{
				Route:         routeLabel,
				Protected:     true,
				Provider:      opts.Provider,
				ProviderType:  providerType,
				ClaimMappings: mappings,
			}
			if providerType == "connector" {
				out.ConnectorType = oidcProvider.ConnectorType
			}
			return PrintJSON(out)
		}

		items := []ui.NamedValue{
			ui.NewNamedValue("Route", routeLabel),
			ui.NewNamedValue("Protected", true),
			ui.NewNamedValue("Provider", opts.Provider),
			ui.NewNamedValue("Type", providerType),
		}
		if providerType == "connector" {
			items = append(items, ui.NewNamedValue("Connector", oidcProvider.ConnectorType))
		}

		ctx.Printf("%s\n", ui.NewNamedValueList(items).Render())

		if len(claimMappings) > 0 {
			var rows []ui.Row
			for _, m := range claimMappings {
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

	// Try password provider
	pwProvider, err := ic.GetPasswordProvider(ctx, opts.Provider)
	if err != nil {
		return fmt.Errorf("failed to lookup provider: %w", err)
	}

	if pwProvider == nil {
		return fmt.Errorf("provider not found: %s", opts.Provider)
	}

	if len(opts.ClaimHeader) > 0 {
		if opts.IsJSON() {
			return fmt.Errorf("--claim-header is not supported for password providers")
		}
		ctx.Printf("Warning: --claim-header is ignored for password providers\n")
	}

	_, err = ic.AttachAuthProviderToRoute(ctx, route, pwProvider.ID, nil)
	if err != nil {
		return fmt.Errorf("failed to protect route: %w", err)
	}

	if opts.IsJSON() {
		type RouteProtectJSON struct {
			Route        string `json:"route"`
			Protected    bool   `json:"protected"`
			Provider     string `json:"provider"`
			ProviderType string `json:"provider_type"`
		}

		return PrintJSON(RouteProtectJSON{
			Route:        routeLabel,
			Protected:    true,
			Provider:     opts.Provider,
			ProviderType: "password",
		})
	}

	items := []ui.NamedValue{
		ui.NewNamedValue("Route", routeLabel),
		ui.NewNamedValue("Protected", true),
		ui.NewNamedValue("Provider", opts.Provider),
		ui.NewNamedValue("Type", "password"),
	}

	ctx.Printf("%s\n", ui.NewNamedValueList(items).Render())
	return nil
}
