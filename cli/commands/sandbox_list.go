package commands

import (
	"fmt"
	"time"

	"miren.dev/runtime/api/compute"
	"miren.dev/runtime/api/compute/compute_v1alpha"
	"miren.dev/runtime/api/core/core_v1alpha"
	"miren.dev/runtime/api/entityserver/entityserver_v1alpha"
	"miren.dev/runtime/pkg/entity"
	"miren.dev/runtime/pkg/ui"
)

func SandboxList(ctx *Context, opts struct {
	All    bool   `short:"a" long:"all" description:"Include dead sandboxes (excluded by default)"`
	Status string `short:"s" long:"status" description:"Filter by status (pending, not_ready, running, stopped, dead)"`
	FormatOptions
	ConfigCentric
}) error {
	client, err := ctx.RPCClient("entities")
	if err != nil {
		return err
	}

	eac := entityserver_v1alpha.NewEntityAccessClient(client)

	// Get the sandbox kind attribute
	kindRes, err := eac.LookupKind(ctx, "sandbox")
	if err != nil {
		return err
	}

	// List all sandboxes
	res, err := eac.List(ctx, kindRes.Attr())
	if err != nil {
		return err
	}

	// Get all sandbox pools to map pool ID -> service
	poolKindRes, err := eac.LookupKind(ctx, "sandbox_pool")
	if err != nil {
		return err
	}
	poolsRes, err := eac.List(ctx, poolKindRes.Attr())
	if err != nil {
		return err
	}

	// Create maps of pool ID -> service and pool ID -> short ID
	poolServiceMap := make(map[string]string)
	poolShortIdMap := make(map[string]string)
	for _, e := range poolsRes.Values() {
		var pool compute_v1alpha.SandboxPool
		pool.Decode(e.Entity())
		poolServiceMap[pool.ID.String()] = pool.Service
		if sid := e.Entity().ShortId(); sid != "" {
			poolShortIdMap[pool.ID.String()] = sid
		}
	}

	// Build maps of version ID -> short ID and version ID -> app name (best-effort for display)
	versionShortIdMap := make(map[string]string)
	versionAppMap := make(map[string]string)
	if versionKindRes, err := eac.LookupKind(ctx, "app_version"); err == nil {
		if versionsRes, err := eac.List(ctx, versionKindRes.Attr()); err == nil {
			for _, e := range versionsRes.Values() {
				var v core_v1alpha.AppVersion
				v.Decode(e.Entity())
				if sid := e.Entity().ShortId(); sid != "" {
					versionShortIdMap[v.ID.String()] = sid
				}
				if !entity.Empty(v.App) {
					versionAppMap[v.ID.String()] = ui.CleanEntityID(v.App.String())
				}
			}
		}
	}

	// Build a map of node entity ID -> human-readable name
	nodeKindRes, err := eac.LookupKind(ctx, "node")
	if err != nil {
		return err
	}
	nodesRes, err := eac.List(ctx, nodeKindRes.Attr())
	if err != nil {
		return err
	}
	nodeNameMap := make(map[entity.Id]string)
	for _, e := range nodesRes.Values() {
		var node compute_v1alpha.Node
		node.Decode(e.Entity())
		name := node.Name
		if name == "" {
			name = node.RunnerId
			if len(name) > 12 {
				name = name[:12]
			}
		}
		if name != "" {
			nodeNameMap[node.ID] = name
		}
	}

	// Determine whether to exclude dead sandboxes.
	// Dead sandboxes are excluded by default unless --all is passed
	// or --status explicitly requests a dead state.
	excludeDead := !opts.All && opts.Status == ""

	// For JSON output, just filter and return the raw sandbox structs with pool info
	if opts.IsJSON() {
		var sandboxes []struct {
			compute_v1alpha.Sandbox
			App     string `json:"app,omitempty"`
			Pool    string `json:"pool,omitempty"`
			Service string `json:"service,omitempty"`
			Address string `json:"address,omitempty"`
			Runner  string `json:"runner,omitempty"`
		}

		for _, e := range res.Values() {
			var sandbox compute_v1alpha.Sandbox
			sandbox.Decode(e.Entity())

			if excludeDead && compute.SandboxDead(sandbox.Status) {
				continue
			}

			// Apply status filter if specified
			if opts.Status != "" {
				status := string(sandbox.Status)
				cleanStatus := ui.CleanStatus(status)
				if cleanStatus != opts.Status {
					continue
				}
			}

			// Extract pool label from metadata
			var md core_v1alpha.Metadata
			md.Decode(e.Entity())
			poolLabel, _ := md.Labels.Get("pool")

			// Get service from pool
			service := poolServiceMap[poolLabel]

			// Get network address
			address := ""
			if len(sandbox.Network) > 0 && sandbox.Network[0].Address != "" {
				address = sandbox.Network[0].Address
			}

			// Resolve runner from schedule
			var sch compute_v1alpha.Schedule
			sch.Decode(e.Entity())
			runner := ""
			if !entity.Empty(sch.Key.Node) {
				runner = nodeNameMap[sch.Key.Node]
			}

			// Resolve app name from version
			appName := ""
			if name, ok := versionAppMap[sandbox.Spec.Version.String()]; ok {
				appName = name
			}

			entry := struct {
				compute_v1alpha.Sandbox
				App     string `json:"app,omitempty"`
				Pool    string `json:"pool,omitempty"`
				Service string `json:"service,omitempty"`
				Address string `json:"address,omitempty"`
				Runner  string `json:"runner,omitempty"`
			}{
				Sandbox: sandbox,
				App:     appName,
				Pool:    poolLabel,
				Service: service,
				Address: address,
				Runner:  runner,
			}
			sandboxes = append(sandboxes, entry)
		}

		return PrintJSON(sandboxes)
	}

	// Table output - all the UI formatting logic
	var rows []ui.Row
	var deadCount int
	headers := []string{"ID", "APP", "VERSION", "SERVICE", "POOL", "ADDRESS", "RUNNER", "STATUS", "CREATED", "UPDATED"}

	for _, e := range res.Values() {
		// Decode the sandbox entity
		var sandbox compute_v1alpha.Sandbox
		sandbox.Decode(e.Entity())

		if excludeDead && compute.SandboxDead(sandbox.Status) {
			deadCount++
			continue
		}

		// Get status string
		status := string(sandbox.Status)
		if status == "" {
			status = "unknown"
		}

		// Clean status for filtering (removes "status." prefix)
		cleanStatus := ui.CleanStatus(status)

		// Filter by status if specified
		if opts.Status != "" && cleanStatus != opts.Status {
			continue
		}

		// Extract pool label from metadata
		var md core_v1alpha.Metadata
		md.Decode(e.Entity())
		poolLabel, _ := md.Labels.Get("pool")
		poolLabelDisplay := poolLabel
		if poolLabelDisplay == "" {
			poolLabelDisplay = "-"
		} else {
			poolLabelDisplay = ui.CleanEntityID(poolLabelDisplay)
		}

		// Get service from pool
		service := poolServiceMap[poolLabel]
		if service == "" {
			service = "-"
		}

		// Get network address
		address := "-"
		if len(sandbox.Network) > 0 && sandbox.Network[0].Address != "" {
			address = sandbox.Network[0].Address
		}

		// Resolve runner from schedule
		var sch compute_v1alpha.Schedule
		sch.Decode(e.Entity())
		runnerName := "-"
		if !entity.Empty(sch.Key.Node) {
			if name, ok := nodeNameMap[sch.Key.Node]; ok {
				runnerName = name
			}
		}

		// Apply all UI formatting for table display
		sandboxId := ui.CleanEntityID(sandbox.ID.String())
		if !ctx.Verbose() {
			sandboxId = ui.BriefId(e.Entity())
		}

		// Resolve version display: prefer short ID
		versionDisplay := ui.DisplayAppVersion(sandbox.Spec.Version.String())
		if shortId, ok := versionShortIdMap[sandbox.Spec.Version.String()]; ok {
			versionDisplay = shortId
		}

		// Resolve pool display: prefer short ID
		if shortId, ok := poolShortIdMap[poolLabel]; ok {
			poolLabelDisplay = shortId
		}

		// Resolve app name from version
		appName := "-"
		if name, ok := versionAppMap[sandbox.Spec.Version.String()]; ok {
			appName = name
		}

		rows = append(rows, ui.Row{
			sandboxId,
			appName,
			versionDisplay,
			service,
			poolLabelDisplay,
			address,
			runnerName,
			ui.DisplayStatus(status),
			humanFriendlyTimestamp(time.UnixMilli(e.CreatedAt())),
			humanFriendlyTimestamp(time.UnixMilli(e.UpdatedAt())),
		})
	}

	if len(rows) == 0 {
		ctx.Printf("No sandboxes found\n")
		return nil
	}

	// Create and render the table
	columns := ui.AutoSizeColumns(headers, rows, ui.Columns().NoTruncate(0))
	table := ui.NewTable(
		ui.WithColumns(columns),
		ui.WithRows(rows),
	)

	ctx.Printf("%s\n", table.Render())

	if deadCount > 0 {
		ctx.Printf("\n%d dead sandbox(es) hidden. Use --all to show.\n", deadCount)
	}

	return nil
}

// humanFriendlyTimestamp formats a timestamp into a human-friendly format like Docker's
func humanFriendlyTimestamp(t time.Time) string {
	if t.IsZero() || t.Unix() <= 0 {
		return "-"
	}

	since := time.Since(t)

	// Handle negative durations (timestamps in the future or invalid)
	if since < 0 {
		return "-"
	}

	if since < time.Minute {
		return fmt.Sprintf("%ds ago", int(since.Seconds()))
	} else if since < time.Hour {
		return fmt.Sprintf("%dm ago", int(since.Minutes()))
	} else if since < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(since.Hours()))
	} else if since < 7*24*time.Hour {
		return fmt.Sprintf("%dd ago", int(since.Hours()/24))
	} else if since < 30*24*time.Hour {
		return fmt.Sprintf("%dw ago", int(since.Hours()/(24*7)))
	} else if since < 365*24*time.Hour {
		return fmt.Sprintf("%dmo ago", int(since.Hours()/(24*30)))
	} else {
		return fmt.Sprintf("%dy ago", int(since.Hours()/(24*365)))
	}
}
