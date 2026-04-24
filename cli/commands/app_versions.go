package commands

import (
	"fmt"
	"sort"
	"time"

	"miren.dev/runtime/api/app"
	"miren.dev/runtime/api/core/core_v1alpha"
	"miren.dev/runtime/api/entityserver"
	"miren.dev/runtime/api/entityserver/entityserver_v1alpha"
	"miren.dev/runtime/pkg/entity"
	"miren.dev/runtime/pkg/ui"
)

type appVersionInfo struct {
	version   core_v1alpha.AppVersion
	createdAt time.Time
}

func AppVersions(ctx *Context, opts struct {
	AppCentric
	FormatOptions

	Ephemeral bool `long:"ephemeral" description:"Show only ephemeral versions"`
	Limit     int  `short:"n" long:"limit" description:"Max versions to show" default:"20"`
}) error {
	client, err := ctx.RPCClient("entities")
	if err != nil {
		return err
	}

	eac := entityserver_v1alpha.NewEntityAccessClient(client)
	ec := entityserver.NewClient(ctx.Log, eac)

	appClient := app.NewClient(ctx.Log, client)
	appEntity, err := appClient.GetByName(ctx, opts.App)
	if err != nil {
		return fmt.Errorf("failed to get app %q: %w", opts.App, err)
	}

	lr, err := ec.List(ctx, entity.Ref(core_v1alpha.AppVersionAppId, appEntity.ID))
	if err != nil {
		return fmt.Errorf("failed to list app versions: %w", err)
	}

	var versions []appVersionInfo
	for lr.Next() {
		var av core_v1alpha.AppVersion
		if err := lr.Read(&av); err != nil {
			continue
		}

		if opts.Ephemeral && av.EphemeralLabel == "" {
			continue
		}

		createdAt := lr.Entity().GetCreatedAt()
		versions = append(versions, appVersionInfo{
			version:   av,
			createdAt: createdAt,
		})
	}

	sort.Slice(versions, func(i, j int) bool {
		return versions[i].createdAt.After(versions[j].createdAt)
	})

	if opts.Limit > 0 && len(versions) > opts.Limit {
		versions = versions[:opts.Limit]
	}

	if opts.IsJSON() {
		return printAppVersionsJSON(versions, appEntity)
	}

	if opts.Ephemeral {
		return printEphemeralVersionsTable(ctx, versions)
	}
	return printAllVersionsTable(ctx, versions, appEntity)
}

func versionStatus(av core_v1alpha.AppVersion, appEntity *core_v1alpha.App) string {
	if av.ID == appEntity.ActiveVersion {
		return "active"
	}
	if av.EphemeralLabel != "" {
		return "ephemeral"
	}
	return "replaced"
}

func printAppVersionsJSON(versions []appVersionInfo, appEntity *core_v1alpha.App) error {
	type versionJSON struct {
		ID             string `json:"id"`
		Version        string `json:"version"`
		Status         string `json:"status"`
		CreatedAt      string `json:"created_at"`
		EphemeralLabel string `json:"ephemeral_label,omitempty"`
		EphemeralTTL   string `json:"ephemeral_ttl,omitempty"`
		ExpiresAt      string `json:"expires_at,omitempty"`
	}

	var items []versionJSON
	for _, v := range versions {
		item := versionJSON{
			ID:        string(v.version.ID),
			Version:   v.version.Version,
			Status:    versionStatus(v.version, appEntity),
			CreatedAt: v.createdAt.UTC().Format(time.RFC3339),
		}
		if v.version.EphemeralLabel != "" {
			item.EphemeralLabel = v.version.EphemeralLabel
			item.EphemeralTTL = v.version.EphemeralTtl
			if !v.version.EphemeralExpiresAt.IsZero() {
				item.ExpiresAt = v.version.EphemeralExpiresAt.UTC().Format(time.RFC3339)
			}
		}
		items = append(items, item)
	}
	return PrintJSON(items)
}

func printAllVersionsTable(ctx *Context, versions []appVersionInfo, appEntity *core_v1alpha.App) error {
	headers := []string{"VERSION", "STATUS", "CREATED", "LABEL"}
	var rows []ui.Row

	for _, v := range versions {
		status := versionStatus(v.version, appEntity)

		label := ""
		if v.version.EphemeralLabel != "" {
			remaining := time.Until(v.version.EphemeralExpiresAt)
			if remaining > 0 {
				label = fmt.Sprintf("%s (expires in %s)", v.version.EphemeralLabel, formatVersionDuration(remaining))
			} else {
				label = fmt.Sprintf("%s (expired)", v.version.EphemeralLabel)
			}
		}

		rows = append(rows, ui.Row{
			v.version.Version,
			status,
			humanFriendlyTimestamp(v.createdAt),
			label,
		})
	}

	if len(rows) == 0 {
		ctx.Printf("No versions found.\n")
		return nil
	}

	columns := ui.AutoSizeColumns(headers, rows, ui.Columns().NoTruncate(0))
	table := ui.NewTable(
		ui.WithColumns(columns),
		ui.WithRows(rows),
	)
	ctx.Printf("%s\n", table.Render())
	return nil
}

func printEphemeralVersionsTable(ctx *Context, versions []appVersionInfo) error {
	headers := []string{"VERSION", "LABEL", "CREATED", "EXPIRES"}
	var rows []ui.Row

	for _, v := range versions {
		expires := "-"
		if !v.version.EphemeralExpiresAt.IsZero() {
			expires = v.version.EphemeralExpiresAt.Format("2006-01-02 15:04:05")
		}

		rows = append(rows, ui.Row{
			v.version.Version,
			v.version.EphemeralLabel,
			humanFriendlyTimestamp(v.createdAt),
			expires,
		})
	}

	if len(rows) == 0 {
		ctx.Printf("No ephemeral versions found.\n")
		return nil
	}

	columns := ui.AutoSizeColumns(headers, rows, ui.Columns().NoTruncate(0))
	table := ui.NewTable(
		ui.WithColumns(columns),
		ui.WithRows(rows),
	)
	ctx.Printf("%s\n", table.Render())
	return nil
}

func formatVersionDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	} else if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	} else if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}
