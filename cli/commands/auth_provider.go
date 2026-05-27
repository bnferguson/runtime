package commands

import (
	"fmt"
	"strings"

	"miren.dev/runtime/api/ingress"
	"miren.dev/runtime/pkg/ui"
)

// providerType returns the user-facing type label for an OIDC provider entity.
// Connector-backed providers (e.g. github) surface as their connector type
// directly so the CLI exposes a flat three-type model: oidc, github, password.
func providerType(connectorType string) string {
	if connectorType == "" || connectorType == "oidc" {
		return "oidc"
	}
	return connectorType
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
			ConfigJSON  string   `json:"config_json,omitempty"`
		}

		items := make([]ProviderJSON, 0, len(oidcProviders)+len(pwProviders))
		for _, p := range oidcProviders {
			if isConnector(p) {
				items = append(items, ProviderJSON{
					Name:       p.Name,
					Type:       providerType(p.ConnectorType),
					ClientID:   p.ClientId,
					ConfigJSON: p.ConfigJson,
				})
			} else {
				items = append(items, ProviderJSON{
					Name:        p.Name,
					Type:        "oidc",
					ProviderURL: p.ProviderUrl,
					ClientID:    p.ClientId,
					Scopes:      strings.Fields(p.Scopes),
				})
			}
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
	headers := []string{"NAME", "TYPE", "DETAILS"}

	for _, p := range oidcProviders {
		if isConnector(p) {
			rows = append(rows, ui.Row{p.Name, providerType(p.ConnectorType), summarizeConnectorConfig(p.ConfigJson)})
		} else {
			rows = append(rows, ui.Row{p.Name, "oidc", p.ProviderUrl})
		}
	}
	for _, p := range pwProviders {
		rows = append(rows, ui.Row{p.Name, "password", ""})
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

	provider, err := ic.GetOIDCProvider(ctx, opts.Name)
	if err != nil {
		return fmt.Errorf("failed to get identity provider: %w", err)
	}

	if provider != nil {
		if isConnector(provider) {
			if opts.IsJSON() {
				type ProviderJSON struct {
					Name       string `json:"name"`
					Type       string `json:"type"`
					ClientID   string `json:"client_id"`
					ConfigJSON string `json:"config_json,omitempty"`
				}

				return PrintJSON(ProviderJSON{
					Name:       provider.Name,
					Type:       providerType(provider.ConnectorType),
					ClientID:   provider.ClientId,
					ConfigJSON: provider.ConfigJson,
				})
			}

			items := []ui.NamedValue{
				ui.NewNamedValue("Name", provider.Name),
				ui.NewNamedValue("Type", providerType(provider.ConnectorType)),
				ui.NewNamedValue("Client ID", provider.ClientId),
			}
			if summary := summarizeConnectorConfig(provider.ConfigJson); summary != "" {
				items = append(items, ui.NewNamedValue("Config", summary))
			}

			ctx.Printf("%s\n", ui.NewNamedValueList(items).Render())
			return nil
		}

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

	oidcProvider, err := ic.GetOIDCProvider(ctx, opts.Name)
	if err != nil {
		return fmt.Errorf("failed to get identity provider: %w", err)
	}

	if oidcProvider != nil {
		label := providerType(oidcProvider.ConnectorType) + " provider"

		if !opts.Force {
			routes, err := ic.List(ctx)
			if err != nil {
				return fmt.Errorf("failed to list routes: %w", err)
			}

			var inUse []string
			for _, rm := range routes {
				if rm.Route.AuthProvider == oidcProvider.ID {
					host := rm.Route.Host
					if rm.Route.Default {
						host = "(default)"
					}
					inUse = append(inUse, host)
				}
			}

			if len(inUse) > 0 {
				return fmt.Errorf("%s %q is attached to %d route(s): %s. Detach with `miren route unprotect`, or pass --force to remove anyway", label, opts.Name, len(inUse), strings.Join(inUse, ", "))
			}
		}

		err = ic.DeleteOIDCProvider(ctx, opts.Name)
		if err != nil {
			return fmt.Errorf("failed to remove %s: %w", label, err)
		}

		ctx.Printf("Removed %s: %s\n", label, opts.Name)
		return nil
	}

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
				if rm.Route.AuthProvider == pwProvider.ID {
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
