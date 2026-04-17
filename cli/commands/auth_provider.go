package commands

import (
	"fmt"
	"strings"

	"miren.dev/runtime/api/ingress"
	"miren.dev/runtime/api/ingress/ingress_v1alpha"
	"miren.dev/runtime/pkg/labs"
	"miren.dev/runtime/pkg/ui"
)

func AuthProviderAdd(ctx *Context, opts struct {
	Name         string   `position:"0" usage:"Name for this identity provider" required:"true"`
	ProviderURL  string   `long:"provider-url" description:"OIDC provider URL (e.g., https://accounts.google.com)" required:"true"`
	ClientID     string   `long:"client-id" description:"OAuth2 client ID" required:"true"`
	ClientSecret string   `long:"client-secret" description:"OAuth2 client secret" required:"true"`
	Scopes       []string `long:"scope" description:"OAuth2 scopes (can be specified multiple times)"`
	ConfigCentric
}) error {
	if !labs.RouteOIDC() {
		return fmt.Errorf("route protection is disabled. Enable with MIREN_LABS=routeoidc")
	}

	if opts.Name == "" {
		return fmt.Errorf("provider name is required")
	}

	if opts.ProviderURL == "" {
		return fmt.Errorf("--provider-url is required")
	}

	if opts.ClientID == "" || opts.ClientSecret == "" {
		return fmt.Errorf("--client-id and --client-secret are required")
	}

	// Ensure "openid" scope is always included
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

	client, err := ctx.RPCClient("entities")
	if err != nil {
		return err
	}

	ic := ingress.NewClient(ctx.Log, client)

	provider := &ingress_v1alpha.OidcProvider{
		Name:         opts.Name,
		ProviderUrl:  opts.ProviderURL,
		ClientId:     opts.ClientID,
		ClientSecret: opts.ClientSecret,
		Scopes:       scopes,
	}

	_, err = ic.CreateOrUpdateOIDCProvider(ctx, provider)
	if err != nil {
		return fmt.Errorf("failed to create identity provider: %w", err)
	}

	items := []ui.NamedValue{
		ui.NewNamedValue("Name", opts.Name),
		ui.NewNamedValue("Provider URL", opts.ProviderURL),
		ui.NewNamedValue("Client ID", opts.ClientID),
		ui.NewNamedValue("Scopes", scopes),
	}

	ctx.Printf("%s\n", ui.NewNamedValueList(items).Render())
	return nil
}

func AuthProviderList(ctx *Context, opts struct {
	FormatOptions
	ConfigCentric
}) error {
	if !labs.RouteOIDC() {
		return fmt.Errorf("route protection is disabled. Enable with MIREN_LABS=routeoidc")
	}

	client, err := ctx.RPCClient("entities")
	if err != nil {
		return err
	}

	ic := ingress.NewClient(ctx.Log, client)

	providers, err := ic.ListOIDCProviders(ctx)
	if err != nil {
		return fmt.Errorf("failed to list identity providers: %w", err)
	}

	if opts.IsJSON() {
		type ProviderJSON struct {
			Name        string `json:"name"`
			ProviderURL string `json:"provider_url"`
			ClientID    string `json:"client_id"`
			Scopes      string `json:"scopes"`
		}

		var items []ProviderJSON
		for _, p := range providers {
			items = append(items, ProviderJSON{
				Name:        p.Name,
				ProviderURL: p.ProviderUrl,
				ClientID:    p.ClientId,
				Scopes:      p.Scopes,
			})
		}
		return PrintJSON(items)
	}

	if len(providers) == 0 {
		ctx.Printf("No identity providers found\n")
		return nil
	}

	var rows []ui.Row
	headers := []string{"NAME", "PROVIDER URL", "SCOPES"}

	for _, p := range providers {
		rows = append(rows, ui.Row{p.Name, p.ProviderUrl, p.Scopes})
	}

	columns := ui.AutoSizeColumns(headers, rows, ui.Columns().NoTruncate(0).NoTruncate(1))
	table := ui.NewTable(
		ui.WithColumns(columns),
		ui.WithRows(rows),
	)

	ctx.Printf("%s\n", table.Render())
	return nil
}

func AuthProviderShow(ctx *Context, opts struct {
	Name string `position:"0" usage:"Name of the identity provider" required:"true"`
	FormatOptions
	ConfigCentric
}) error {
	if !labs.RouteOIDC() {
		return fmt.Errorf("route protection is disabled. Enable with MIREN_LABS=routeoidc")
	}

	if opts.Name == "" {
		return fmt.Errorf("provider name is required")
	}

	client, err := ctx.RPCClient("entities")
	if err != nil {
		return err
	}

	ic := ingress.NewClient(ctx.Log, client)

	provider, err := ic.GetOIDCProvider(ctx, opts.Name)
	if err != nil {
		return fmt.Errorf("failed to get identity provider: %w", err)
	}

	if provider == nil {
		return fmt.Errorf("identity provider not found: %s", opts.Name)
	}

	if opts.IsJSON() {
		type ProviderJSON struct {
			Name        string `json:"name"`
			ProviderURL string `json:"provider_url"`
			ClientID    string `json:"client_id"`
			Scopes      string `json:"scopes"`
		}

		return PrintJSON(ProviderJSON{
			Name:        provider.Name,
			ProviderURL: provider.ProviderUrl,
			ClientID:    provider.ClientId,
			Scopes:      provider.Scopes,
		})
	}

	items := []ui.NamedValue{
		ui.NewNamedValue("Name", provider.Name),
		ui.NewNamedValue("Provider URL", provider.ProviderUrl),
		ui.NewNamedValue("Client ID", provider.ClientId),
		ui.NewNamedValue("Scopes", provider.Scopes),
	}

	ctx.Printf("%s\n", ui.NewNamedValueList(items).Render())
	return nil
}

func AuthProviderRemove(ctx *Context, opts struct {
	Name string `position:"0" usage:"Name of the identity provider to remove" required:"true"`
	ConfigCentric
}) error {
	if !labs.RouteOIDC() {
		return fmt.Errorf("route protection is disabled. Enable with MIREN_LABS=routeoidc")
	}

	if opts.Name == "" {
		return fmt.Errorf("provider name is required")
	}

	client, err := ctx.RPCClient("entities")
	if err != nil {
		return err
	}

	ic := ingress.NewClient(ctx.Log, client)

	err = ic.DeleteOIDCProvider(ctx, opts.Name)
	if err != nil {
		return fmt.Errorf("failed to remove identity provider: %w", err)
	}

	ctx.Printf("Removed identity provider: %s\n", opts.Name)
	return nil
}
