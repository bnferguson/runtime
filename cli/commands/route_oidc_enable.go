package commands

import (
	"fmt"
	"strings"

	"miren.dev/runtime/api/ingress"
	"miren.dev/runtime/api/ingress/ingress_v1alpha"
)

func RouteOidcEnable(ctx *Context, opts struct {
	Host         string   `position:"0" usage:"Hostname for the route (e.g., example.com)" required:"true"`
	Provider     string   `flag:"provider" usage:"Name of existing OIDC provider (use --provider-url for inline creation)"`
	ProviderURL  string   `flag:"provider-url" usage:"OIDC provider URL (e.g., https://accounts.google.com) - creates provider if not exists"`
	ClientID     string   `flag:"client-id" usage:"OAuth2 client ID (required with --provider-url)"`
	ClientSecret string   `flag:"client-secret" usage:"OAuth2 client secret (required with --provider-url)"`
	Scopes       []string `flag:"scope" usage:"OAuth2 scopes (can be specified multiple times)"`
	ClaimHeader  []string `flag:"claim-header" usage:"Claim to header mapping in format 'claim:header' (e.g., 'email:X-User-Email')"`
	ConfigCentric
}) error {
	client, err := ctx.RPCClient("entities")
	if err != nil {
		return err
	}

	ic := ingress.NewClient(ctx.Log, client)

	// Look up existing route
	route, err := ic.Lookup(ctx, opts.Host)
	if err != nil {
		return fmt.Errorf("failed to lookup route: %w", err)
	}

	if route == nil {
		return fmt.Errorf("route not found for host: %s", opts.Host)
	}

	// Determine provider name
	providerName := opts.Provider
	if providerName == "" {
		// If no provider name given and provider-url specified, generate a name
		if opts.ProviderURL != "" {
			// Use the host from the provider URL as the provider name
			providerName = strings.ReplaceAll(opts.ProviderURL, "https://", "")
			providerName = strings.ReplaceAll(providerName, "http://", "")
			providerName = strings.Split(providerName, "/")[0]
		} else {
			return fmt.Errorf("either --provider or --provider-url must be specified")
		}
	}

	// If provider-url is specified, create or update the provider
	if opts.ProviderURL != "" {
		if opts.ClientID == "" || opts.ClientSecret == "" {
			return fmt.Errorf("--client-id and --client-secret are required with --provider-url")
		}

		// Build scopes string
		scopes := "openid"
		if len(opts.Scopes) > 0 {
			scopes = strings.Join(opts.Scopes, " ")
			// Ensure openid is included
			if !strings.Contains(scopes, "openid") {
				scopes = "openid " + scopes
			}
		}

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
	_, err = ic.AttachOIDCProvider(ctx, opts.Host, providerName, claimMappings)
	if err != nil {
		return fmt.Errorf("failed to attach OIDC provider to route: %w", err)
	}

	ctx.Printf("OIDC enabled for route: %s\n", opts.Host)
	ctx.Printf("Provider: %s\n", providerName)
	if len(claimMappings) > 0 {
		ctx.Printf("Claim mappings:\n")
		for _, mapping := range claimMappings {
			ctx.Printf("  %s → %s\n", mapping.Claim, mapping.Header)
		}
	}
	return nil
}
