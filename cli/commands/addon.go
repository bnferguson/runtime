package commands

import (
	"fmt"

	"miren.dev/runtime/api/addon/addon_v1alpha"
	"miren.dev/runtime/api/app/app_v1alpha"
	"miren.dev/runtime/api/entityserver/entityserver_v1alpha"
	"miren.dev/runtime/pkg/ui"
)

func AddonListAvailable(ctx *Context, opts struct {
	FormatOptions
	ConfigCentric
}) error {
	cl, err := ctx.RPCClient("entities")
	if err != nil {
		return err
	}
	defer cl.Close()

	eac := entityserver_v1alpha.NewEntityAccessClient(cl)

	kindRes, err := eac.LookupKind(ctx, "addon")
	if err != nil {
		return err
	}

	res, err := eac.List(ctx, kindRes.Attr())
	if err != nil {
		return err
	}

	if opts.IsJSON() {
		type addonInfo struct {
			Name        string `json:"name"`
			DisplayName string `json:"display_name"`
			Description string `json:"description"`
			DefaultPlan string `json:"default_plan"`
		}

		var addons []addonInfo
		for _, e := range res.Values() {
			var addon addon_v1alpha.Addon
			addon.Decode(e.Entity())
			addons = append(addons, addonInfo{
				Name:        addon.Name,
				DisplayName: addon.DisplayName,
				Description: addon.Description,
				DefaultPlan: addon.DefaultPlan,
			})
		}
		return PrintJSON(addons)
	}

	var rows []ui.Row
	headers := []string{"ADDON", "DESCRIPTION", "DEFAULT PLAN"}

	for _, e := range res.Values() {
		var addon addon_v1alpha.Addon
		addon.Decode(e.Entity())

		rows = append(rows, ui.Row{
			addon.Name,
			addon.Description,
			addon.DefaultPlan,
		})
	}

	if len(rows) == 0 {
		ctx.Printf("No addons available\n")
		return nil
	}

	columns := ui.AutoSizeColumns(headers, rows, nil)
	table := ui.NewTable(
		ui.WithColumns(columns),
		ui.WithRows(rows),
	)

	ctx.Printf("%s\n", table.Render())
	return nil
}

func AddonPlans(ctx *Context, opts struct {
	FormatOptions
	ConfigCentric
	Addon string `position:"0" usage:"Addon name (e.g., miren-postgresql)" required:"true"`
}) error {
	cl, err := ctx.RPCClient("entities")
	if err != nil {
		return err
	}
	defer cl.Close()

	eac := entityserver_v1alpha.NewEntityAccessClient(cl)

	addonRes, err := eac.Get(ctx, "addon/"+opts.Addon)
	if err != nil {
		return fmt.Errorf("addon %q not found: %w", opts.Addon, err)
	}

	var addon addon_v1alpha.Addon
	addon.Decode(addonRes.Entity().Entity())

	if opts.IsJSON() {
		type planInfo struct {
			Name        string            `json:"name"`
			Description string            `json:"description"`
			Details     map[string]string `json:"details,omitempty"`
			Default     bool              `json:"default,omitempty"`
		}

		var plans []planInfo
		for _, p := range addon.Plans {
			details := make(map[string]string)
			for _, d := range p.Details {
				details[d.Key] = d.Value
			}
			plans = append(plans, planInfo{
				Name:        p.Name,
				Description: p.Description,
				Details:     details,
				Default:     p.Name == addon.DefaultPlan,
			})
		}
		return PrintJSON(plans)
	}

	ctx.Printf("Plans for %s:\n\n", addon.DisplayName)

	var rows []ui.Row
	headers := []string{"PLAN", "DESCRIPTION", "DEFAULT"}

	for _, p := range addon.Plans {
		def := ""
		if p.Name == addon.DefaultPlan {
			def = "yes"
		}
		rows = append(rows, ui.Row{
			p.Name,
			p.Description,
			def,
		})
	}

	if len(rows) == 0 {
		ctx.Printf("No plans available\n")
		return nil
	}

	columns := ui.AutoSizeColumns(headers, rows, nil)
	table := ui.NewTable(
		ui.WithColumns(columns),
		ui.WithRows(rows),
	)

	ctx.Printf("%s\n", table.Render())

	for _, p := range addon.Plans {
		if len(p.Details) > 0 {
			ctx.Printf("\n%s:\n", p.Name)
			for _, d := range p.Details {
				ctx.Printf("  %s: %s\n", d.Key, d.Value)
			}
		}
	}

	return nil
}

func AddonCreate(ctx *Context, opts struct {
	AppCentric
	Spec string `position:"0" usage:"Addon spec (e.g., miren-postgresql:small-local)" required:"true"`
}) error {
	cl, err := ctx.RPCClient("dev.miren.runtime/addons")
	if err != nil {
		return err
	}

	addonsClient := app_v1alpha.NewAddonsClient(cl)

	result, err := addonsClient.CreateInstance(ctx, "", opts.Spec, "", opts.App)
	if err != nil {
		return err
	}

	ctx.Completed("Addon attached to %s (id: %s)", opts.App, ui.CleanEntityID(result.Id()))
	return nil
}

func AddonList(ctx *Context, opts struct {
	FormatOptions
	AppCentric
}) error {
	cl, err := ctx.RPCClient("dev.miren.runtime/addons")
	if err != nil {
		return err
	}

	addonsClient := app_v1alpha.NewAddonsClient(cl)

	res, err := addonsClient.ListInstances(ctx, opts.App)
	if err != nil {
		return err
	}

	addons := res.Addons()

	if opts.IsJSON() {
		type addonInfo struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			Plan string `json:"plan"`
		}

		var infos []addonInfo
		for _, a := range addons {
			infos = append(infos, addonInfo{
				ID:   a.Id(),
				Name: a.Name(),
				Plan: a.Plan(),
			})
		}
		return PrintJSON(infos)
	}

	var rows []ui.Row
	headers := []string{"ADDON", "PLAN"}

	for _, a := range addons {
		rows = append(rows, ui.Row{
			a.Name(),
			a.Plan(),
		})
	}

	if len(rows) == 0 {
		ctx.Printf("No addons attached to %s\n", opts.App)
		return nil
	}

	columns := ui.AutoSizeColumns(headers, rows, nil)
	table := ui.NewTable(
		ui.WithColumns(columns),
		ui.WithRows(rows),
	)

	ctx.Printf("%s\n", table.Render())
	return nil
}

func AddonDestroy(ctx *Context, opts struct {
	AppCentric
	Name  string `position:"0" usage:"Addon name (e.g., miren-postgresql)" required:"true"`
	Force bool   `short:"f" long:"force" description:"Skip confirmation prompt"`
}) error {
	if !opts.Force {
		confirmed, err := ui.Confirm(
			ui.WithMessage(fmt.Sprintf("This will destroy the %s addon and delete its data. Continue?", opts.Name)),
			ui.WithDefault(false),
		)
		if err != nil {
			return err
		}
		if !confirmed {
			ctx.Printf("Aborted\n")
			return nil
		}
	}

	cl, err := ctx.RPCClient("dev.miren.runtime/addons")
	if err != nil {
		return err
	}

	addonsClient := app_v1alpha.NewAddonsClient(cl)

	_, err = addonsClient.DeleteInstance(ctx, opts.App, opts.Name)
	if err != nil {
		return err
	}

	ctx.Completed("Addon %s removed from %s", opts.Name, opts.App)
	return nil
}
