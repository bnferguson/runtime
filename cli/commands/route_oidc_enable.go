package commands

import (
	"fmt"
	"strings"

	"miren.dev/runtime/api/ingress"
	"miren.dev/runtime/api/ingress/ingress_v1alpha"
	"miren.dev/runtime/pkg/labs"
	"miren.dev/runtime/pkg/ui"
)

func RouteOidcEnable(ctx *Context, opts struct {
	Host         string   `position:"0" usage:"Hostname for the route (e.g., example.com)"`
	Default      bool     `long:"default" description:"Apply to the default route"`
	Provider     string   `long:"provider" description:"Name of existing OIDC provider (use --provider-url for inline creation)"`
	ProviderURL  string   `long:"provider-url" description:"OIDC provider URL (e.g., https://accounts.google.com) - creates provider if not exists"`
	ClientID     string   `long:"client-id" description:"OAuth2 client ID (required with --provider-url)"`
	ClientSecret string   `long:"client-secret" description:"OAuth2 client secret (required with --provider-url)"`
	Scopes       []string `long:"scope" description:"OAuth2 scopes (can be specified multiple times)"`
	ClaimHeader  []string `long:"claim-header" description:"Claim to header mapping in format 'claim:header' (e.g., 'email:X-User-Email')"`
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

	// Determine provider name
	providerName := opts.Provider
	if providerName == "" {
		// If no provider name given and provider-url specified, generate a name
		if opts.ProviderURL != "" {
			// Include the route host to avoid collisions when multiple routes
			// use the same OIDC provider URL with different credentials.
			providerHost := strings.ReplaceAll(opts.ProviderURL, "https://", "")
			providerHost = strings.ReplaceAll(providerHost, "http://", "")
			providerHost = strings.Split(providerHost, "/")[0]
			providerName = routeLabel + "/" + providerHost
		} else {
			return fmt.Errorf("either --provider or --provider-url must be specified")
		}
	}

	// If provider-url is specified, create or update the provider
	if opts.ProviderURL != "" {
		if opts.ClientID == "" || opts.ClientSecret == "" {
			return fmt.Errorf("--client-id and --client-secret are required with --provider-url")
		}

		// Build scopes, ensuring "openid" is always included
		hasOpenID := false
		for _, s := range opts.Scopes {
			if s == "openid" {
				hasOpenID = true
				break
			}
		}
		scopeList := opts.Scopes
		if !hasOpenID {
			scopeList = append([]string{"openid"}, scopeList...)
		}
		scopes := strings.Join(scopeList, " ")

		// Create or update provider
		provider := &ingress_v1alpha.OidcProvider{
			Name:         providerName,
			ProviderUrl:  opts.ProviderURL,
			ClientId:     opts.ClientID,
			ClientSecret: opts.ClientSecret,
			Scopes:       scopes,
		}

		_, err = ic.CreateOrUpdateOIDCProvider(ctx, provider)
		if err != nil {
			return fmt.Errorf("failed to create/update OIDC provider: %w", err)
		}

		ctx.Printf("Created/updated OIDC provider: %s\n", providerName)
	}

	// Parse claim mappings
	var claimMappings []ingress_v1alpha.ClaimMappings
	for _, mapping := range opts.ClaimHeader {
		parts := strings.SplitN(mapping, ":", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid claim-header mapping format: %s (expected 'claim:header')", mapping)
		}
		claimMappings = append(claimMappings, ingress_v1alpha.ClaimMappings{
			Claim:  strings.TrimSpace(parts[0]),
			Header: strings.TrimSpace(parts[1]),
		})
	}

	// Attach provider to route
	_, err = ic.AttachOIDCProviderToRoute(ctx, route, providerName, claimMappings)
	if err != nil {
		return fmt.Errorf("failed to attach OIDC provider to route: %w", err)
	}

	items := []ui.NamedValue{
		ui.NewNamedValue("Route", routeLabel),
		ui.NewNamedValue("OIDC", true),
		ui.NewNamedValue("Provider", providerName),
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
