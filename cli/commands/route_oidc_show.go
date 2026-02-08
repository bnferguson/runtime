package commands

import (
	"fmt"

	"miren.dev/runtime/api/ingress"
	"miren.dev/runtime/api/ingress/ingress_v1alpha"
	"miren.dev/runtime/pkg/entity"
)

func RouteOidcShow(ctx *Context, opts struct {
	Host string `position:"0" usage:"Hostname for the route (e.g., example.com)" required:"true"`
	FormatOptions
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

	// Check if OIDC is configured
	if entity.Empty(route.OidcProvider) {
		if opts.IsJSON() {
			return PrintJSON(map[string]interface{}{
				"host":         opts.Host,
				"oidc_enabled": false,
				"provider":     nil,
			})
		}
		ctx.Printf("OIDC is not configured for route: %s\n", opts.Host)
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
			Host:          opts.Host,
			OIDCEnabled:   true,
			ProviderName:  provider.Name,
			ProviderURL:   provider.ProviderUrl,
			ClientID:      provider.ClientId,
			Scopes:        provider.Scopes,
			ClaimMappings: mappings,
		})
	}

	ctx.Printf("OIDC Configuration for route: %s\n\n", opts.Host)
	ctx.Printf("Enabled:      Yes\n")
	ctx.Printf("Provider:     %s\n", provider.Name)
	ctx.Printf("Provider URL: %s\n", provider.ProviderUrl)
	ctx.Printf("Client ID:    %s\n", provider.ClientId)
	ctx.Printf("Scopes:       %s\n", provider.Scopes)

	if len(route.ClaimMappings) > 0 {
		ctx.Printf("\nClaim Mappings:\n")
		for _, mapping := range route.ClaimMappings {
			ctx.Printf("  %s → %s\n", mapping.Claim, mapping.Header)
		}
	}

	return nil
}
