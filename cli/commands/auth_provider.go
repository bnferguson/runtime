package commands

import (
	"fmt"
	"strings"

	"miren.dev/runtime/api/ingress"
	"miren.dev/runtime/api/ingress/ingress_v1alpha"
	"miren.dev/runtime/pkg/ui"
)

func AuthProviderAdd(ctx *Context, opts struct {
	Name         string   `position:"0" usage:"Name for this identity provider" required:"true"`
	ProviderURL  string   `long:"provider-url" description:"OIDC provider URL (e.g., https://accounts.google.com)" required:"true"`
	ClientID     string   `long:"client-id" description:"OAuth2 client ID" required:"true"`
	ClientSecret string   `long:"client-secret" description:"OAuth2 client secret" required:"true"`
	Scopes       []string `long:"scope" description:"OAuth2 scopes (can be specified multiple times)"`
	Update       bool     `long:"update" description:"Overwrite an existing provider with the same name (rotates client secret)"`
	ConfigCentric
}) error {

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

	existing, err := ic.GetOIDCProvider(ctx, opts.Name)
	if err != nil {
		return fmt.Errorf("failed to check for existing identity provider: %w", err)
	}
	if existing != nil && !opts.Update {
		return fmt.Errorf("identity provider %q already exists. Pass --update to overwrite (rotates client secret)", opts.Name)
	}

	pwExisting, err := ic.GetPasswordProvider(ctx, opts.Name)
	if err != nil {
		return fmt.Errorf("failed to check for existing password provider: %w", err)
	}
	if pwExisting != nil {
		return fmt.Errorf("a password provider named %q already exists. Provider names must be unique across types", opts.Name)
	}

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

	client, err := ctx.RPCClient("entities")
	if err != nil {
		return err
	}

	ic := ingress.NewClient(ctx.Log, client)

	oidcProviders, err := ic.ListOIDCProviders(ctx)
	if err != nil {
		return fmt.Errorf("failed to list identity providers: %w", err)
	}

	pwProviders, err := ic.ListPasswordProviders(ctx)
	if err != nil {
		return fmt.Errorf("failed to list password providers: %w", err)
	}

	if opts.IsJSON() {
		type ProviderJSON struct {
			Name        string   `json:"name"`
			Type        string   `json:"type"`
			ProviderURL string   `json:"provider_url,omitempty"`
			ClientID    string   `json:"client_id,omitempty"`
			Scopes      []string `json:"scopes,omitempty"`
		}

		items := make([]ProviderJSON, 0, len(oidcProviders)+len(pwProviders))
		for _, p := range oidcProviders {
			items = append(items, ProviderJSON{
				Name:        p.Name,
				Type:        "oidc",
				ProviderURL: p.ProviderUrl,
				ClientID:    p.ClientId,
				Scopes:      strings.Fields(p.Scopes),
			})
		}
		for _, p := range pwProviders {
			items = append(items, ProviderJSON{
				Name: p.Name,
				Type: "password",
			})
		}
		return PrintJSON(items)
	}

	if len(oidcProviders) == 0 && len(pwProviders) == 0 {
		ctx.Printf("No identity providers found\n")
		return nil
	}

	var rows []ui.Row
	headers := []string{"NAME", "TYPE", "PROVIDER URL", "SCOPES"}

	for _, p := range oidcProviders {
		rows = append(rows, ui.Row{p.Name, "oidc", p.ProviderUrl, p.Scopes})
	}
	for _, p := range pwProviders {
		rows = append(rows, ui.Row{p.Name, "password", "", ""})
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

	if opts.Name == "" {
		return fmt.Errorf("provider name is required")
	}

	client, err := ctx.RPCClient("entities")
	if err != nil {
		return err
	}

	ic := ingress.NewClient(ctx.Log, client)

	// Try OIDC provider first
	provider, err := ic.GetOIDCProvider(ctx, opts.Name)
	if err != nil {
		return fmt.Errorf("failed to get identity provider: %w", err)
	}

	if provider != nil {
		if opts.IsJSON() {
			type ProviderJSON struct {
				Name        string   `json:"name"`
				Type        string   `json:"type"`
				ProviderURL string   `json:"provider_url"`
				ClientID    string   `json:"client_id"`
				Scopes      []string `json:"scopes"`
			}

			return PrintJSON(ProviderJSON{
				Name:        provider.Name,
				Type:        "oidc",
				ProviderURL: provider.ProviderUrl,
				ClientID:    provider.ClientId,
				Scopes:      strings.Fields(provider.Scopes),
			})
		}

		items := []ui.NamedValue{
			ui.NewNamedValue("Name", provider.Name),
			ui.NewNamedValue("Type", "oidc"),
			ui.NewNamedValue("Provider URL", provider.ProviderUrl),
			ui.NewNamedValue("Client ID", provider.ClientId),
			ui.NewNamedValue("Scopes", provider.Scopes),
		}

		ctx.Printf("%s\n", ui.NewNamedValueList(items).Render())
		return nil
	}

	// Try password provider
	pwProvider, err := ic.GetPasswordProvider(ctx, opts.Name)
	if err != nil {
		return fmt.Errorf("failed to get password provider: %w", err)
	}

	if pwProvider != nil {
		if opts.IsJSON() {
			type ProviderJSON struct {
				Name string `json:"name"`
				Type string `json:"type"`
			}

			return PrintJSON(ProviderJSON{
				Name: pwProvider.Name,
				Type: "password",
			})
		}

		items := []ui.NamedValue{
			ui.NewNamedValue("Name", pwProvider.Name),
			ui.NewNamedValue("Type", "password"),
		}

		ctx.Printf("%s\n", ui.NewNamedValueList(items).Render())
		return nil
	}

	return fmt.Errorf("identity provider not found: %s", opts.Name)
}

func AuthProviderRemove(ctx *Context, opts struct {
	Name  string `position:"0" usage:"Name of the identity provider to remove" required:"true"`
	Force bool   `long:"force" description:"Remove the provider even if it is attached to routes"`
	ConfigCentric
}) error {

	if opts.Name == "" {
		return fmt.Errorf("provider name is required")
	}

	client, err := ctx.RPCClient("entities")
	if err != nil {
		return err
	}

	ic := ingress.NewClient(ctx.Log, client)

	// Try OIDC provider first
	oidcProvider, err := ic.GetOIDCProvider(ctx, opts.Name)
	if err != nil {
		return fmt.Errorf("failed to get identity provider: %w", err)
	}

	if oidcProvider != nil {
		if !opts.Force {
			routes, err := ic.List(ctx)
			if err != nil {
				return fmt.Errorf("failed to list routes: %w", err)
			}

			var inUse []string
			for _, rm := range routes {
				if rm.Route.OidcProvider == oidcProvider.ID {
					inUse = append(inUse, rm.Route.Host)
				}
			}

			if len(inUse) > 0 {
				return fmt.Errorf("identity provider %q is attached to %d route(s): %s. Detach with `miren route unprotect`, or pass --force to remove anyway", opts.Name, len(inUse), strings.Join(inUse, ", "))
			}
		}

		err = ic.DeleteOIDCProvider(ctx, opts.Name)
		if err != nil {
			return fmt.Errorf("failed to remove identity provider: %w", err)
		}

		ctx.Printf("Removed identity provider: %s\n", opts.Name)
		return nil
	}

	// Try password provider
	pwProvider, err := ic.GetPasswordProvider(ctx, opts.Name)
	if err != nil {
		return fmt.Errorf("failed to get password provider: %w", err)
	}

	if pwProvider != nil {
		if !opts.Force {
			routes, err := ic.List(ctx)
			if err != nil {
				return fmt.Errorf("failed to list routes: %w", err)
			}

			var inUse []string
			for _, rm := range routes {
				if rm.Route.PasswordProvider == pwProvider.ID {
					host := rm.Route.Host
					if rm.Route.Default {
						host = "(default)"
					}
					inUse = append(inUse, host)
				}
			}

			if len(inUse) > 0 {
				return fmt.Errorf("password provider %q is attached to %d route(s): %s. Detach with `miren route unprotect`, or pass --force to remove anyway", opts.Name, len(inUse), strings.Join(inUse, ", "))
			}
		}

		err = ic.DeletePasswordProvider(ctx, opts.Name)
		if err != nil {
			return fmt.Errorf("failed to remove password provider: %w", err)
		}

		ctx.Printf("Removed password provider: %s\n", opts.Name)
		return nil
	}

	return fmt.Errorf("identity provider not found: %s", opts.Name)
}
