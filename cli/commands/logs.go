package commands

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"miren.dev/runtime/api/app/app_v1alpha"
	"miren.dev/runtime/pkg/logfilter"
	"miren.dev/runtime/pkg/rpc"
	"miren.dev/runtime/pkg/rpc/standard"
	"miren.dev/runtime/pkg/rpc/stream"
)

// normalizeSandboxID ensures the sandbox ID has the "sandbox/" prefix
// required for log queries. Logs are stored with the full entity ID.
func normalizeSandboxID(sandboxID string) string {
	if strings.HasPrefix(sandboxID, "sandbox/") {
		return sandboxID
	}
	return "sandbox/" + sandboxID
}

// buildFilterWithService combines a user filter with a service filter for LogsQL.
// Service filter is added as a field match: service:"value"
func buildFilterWithService(userFilter, service string) string {
	if service == "" {
		return userFilter
	}
	serviceFilter := fmt.Sprintf("service:%q", service)
	if userFilter == "" {
		return serviceFilter
	}
	return serviceFilter + " " + userFilter
}

// systemExclusion is a LogsQL filter clause that excludes system logs from
// app log queries. Applied in dispatchLogs on the streamLogChunks path.
const systemExclusion = `-source:"system"`

// buildBuildFilter creates a filter for build logs of a specific version.
// Combines source:build with the version filter.
func buildBuildFilter(version, userFilter string) string {
	buildFilter := fmt.Sprintf("source:build version:%q", version)
	if userFilter == "" {
		return buildFilter
	}
	return buildFilter + " " + userFilter
}

// buildSystemFilter creates a filter for system logs, optionally scoped to a component.
func buildSystemFilter(component, userFilter string) string {
	filter := `source:"system"`
	if component != "" {
		filter += fmt.Sprintf(" module:%q", component)
	}
	if userFilter != "" {
		filter += " " + userFilter
	}
	return filter
}

// LogsApp shows application logs. This is the default subcommand for `miren logs`.
func LogsApp(ctx *Context, opts struct {
	AppCentric
	FormatOptions

	Last    *time.Duration `short:"l" long:"last" description:"Show logs from the last duration"`
	Follow  bool           `short:"f" long:"follow" description:"Follow log output (live tail)"`
	Filter  string         `short:"g" long:"grep" description:"Filter logs (e.g., 'error', '\"exact phrase\"', 'error -debug', '/regex/')"`
	Service string         `long:"service" description:"Filter logs by service name (e.g., 'web', 'worker')"`
}) error {
	cl, err := ctx.RPCClient("dev.miren.runtime/logs")
	if err != nil {
		return err
	}

	combinedFilter := buildFilterWithService(opts.Filter, opts.Service)
	return dispatchLogs(ctx, cl, logDispatchArgs{
		app:            opts.App,
		last:           opts.Last,
		follow:         opts.Follow,
		rawFilter:      opts.Filter,
		combinedFilter: combinedFilter,
		json:           opts.IsJSON(),
	})
}

// LogsSandbox shows logs for a specific sandbox.
func LogsSandbox(ctx *Context, opts struct {
	ConfigCentric
	FormatOptions

	SandboxID string         `position:"0" usage:"Sandbox ID" required:"true"`
	Last      *time.Duration `short:"l" long:"last" description:"Show logs from the last duration"`
	Follow    bool           `short:"f" long:"follow" description:"Follow log output (live tail)"`
	Filter    string         `short:"g" long:"grep" description:"Filter logs (e.g., 'error', '\"exact phrase\"', 'error -debug', '/regex/')"`
}) error {
	sandbox := normalizeSandboxID(opts.SandboxID)

	cl, err := ctx.RPCClient("dev.miren.runtime/logs")
	if err != nil {
		return err
	}

	return dispatchLogs(ctx, cl, logDispatchArgs{
		sandbox:        sandbox,
		last:           opts.Last,
		follow:         opts.Follow,
		rawFilter:      opts.Filter,
		combinedFilter: opts.Filter,
		json:           opts.IsJSON(),
	})
}

// LogsBuild shows build logs for a specific version.
func LogsBuild(ctx *Context, opts struct {
	AppCentric
	FormatOptions

	Version string         `position:"0" usage:"Build version (e.g., v3)" required:"true"`
	Last    *time.Duration `short:"l" long:"last" description:"Show logs from the last duration"`
	Follow  bool           `short:"f" long:"follow" description:"Follow log output (live tail)"`
	Filter  string         `short:"g" long:"grep" description:"Filter logs (e.g., 'error', '\"exact phrase\"', 'error -debug', '/regex/')"`
}) error {
	cl, err := ctx.RPCClient("dev.miren.runtime/logs")
	if err != nil {
		return err
	}

	combinedFilter := buildBuildFilter(opts.Version, opts.Filter)
	return dispatchLogs(ctx, cl, logDispatchArgs{
		app:            opts.App,
		last:           opts.Last,
		follow:         opts.Follow,
		rawFilter:      opts.Filter,
		combinedFilter: combinedFilter,
		json:           opts.IsJSON(),
	})
}

// LogsSystem shows system/server logs, optionally filtered by component.
func LogsSystem(ctx *Context, opts struct {
	ConfigCentric
	FormatOptions

	Component string         `position:"0" usage:"System component to filter by (e.g., 'etcd', 'scheduler')"`
	Last      *time.Duration `short:"l" long:"last" description:"Show logs from the last duration"`
	Follow    bool           `short:"f" long:"follow" description:"Follow log output (live tail)"`
	Filter    string         `short:"g" long:"grep" description:"Filter logs (e.g., 'error', '\"exact phrase\"', 'error -debug', '/regex/')"`
}) error {
	cl, err := ctx.RPCClient("dev.miren.runtime/logs")
	if err != nil {
		return err
	}

	if !cl.HasMethod(ctx, "streamLogChunks") {
		return fmt.Errorf("system logs require a newer server version")
	}

	target := &app_v1alpha.LogTarget{}
	target.SetSystem(true)

	combinedFilter := buildSystemFilter(opts.Component, opts.Filter)

	var ts *standard.Timestamp
	if opts.Last != nil {
		ts = standard.ToTimestamp(time.Now().Add(-*opts.Last))
	}
	// When no --last and no --follow, ts is nil → server returns last 100 lines

	printer := logPrinter(ctx, opts.IsJSON())

	ac := app_v1alpha.LogsClient{Client: cl}
	callback := stream.Callback(func(chunk *app_v1alpha.LogChunk) error {
		for _, l := range chunk.Entries() {
			printer(l)
		}
		return nil
	})

	_, err = ac.StreamLogChunks(ctx, target, ts, opts.Follow, combinedFilter, callback)
	return err
}

// logDispatchArgs holds the parameters for dispatching log requests across
// different server protocol versions.
type logDispatchArgs struct {
	app            string
	sandbox        string
	last           *time.Duration
	follow         bool
	rawFilter      string
	combinedFilter string
	json           bool
}

// dispatchLogs handles protocol negotiation and dispatches to the appropriate
// log streaming method based on server capabilities.
func dispatchLogs(ctx *Context, cl *rpc.NetworkClient, args logDispatchArgs) error {
	printer := logPrinter(ctx, args.json)

	// For app queries, wrap the printer to skip system logs that may have
	// leaked into app log storage due to entity field collisions.
	if args.app != "" {
		inner := printer
		printer = func(l *app_v1alpha.LogEntry) {
			if l.HasSource() && l.Source() == "system" {
				return
			}
			inner(l)
		}
	}

	// Check if server supports streaming (prefer chunked for efficiency)
	if cl.HasMethod(ctx, "streamLogChunks") {
		// Append system exclusion for server-side filtering on app queries
		filter := args.combinedFilter
		if args.app != "" {
			if filter == "" {
				filter = systemExclusion
			} else {
				filter = systemExclusion + " " + filter
			}
		}
		return streamLogChunks(ctx, cl, args.app, args.sandbox, args.last, args.follow, filter, printer)
	}

	// Older server - warn about upgrade and limited functionality
	ctx.Printf("Warning: server does not support optimized log streaming. Upgrade your server for better performance and --service/--build filtering.\n")

	// Server-side filtering (--service, --build) requires streamLogChunks.
	// If the combined filter differs from the raw user filter, one of these
	// was applied and we must error rather than silently dropping it.
	if args.rawFilter != args.combinedFilter {
		return fmt.Errorf("--service and --build filtering require a newer server version")
	}

	// Parse filter for client-side filtering on older protocol
	var filter *logfilter.Filter
	if args.rawFilter != "" {
		var err error
		filter, err = logfilter.Parse(args.rawFilter)
		if err != nil {
			return fmt.Errorf("invalid filter: %w", err)
		}
	}

	if cl.HasMethod(ctx, "streamLogs") {
		return streamLogs(ctx, cl, args.app, args.sandbox, args.last, args.follow, filter, printer)
	}

	// Warn if --follow requested but not supported
	if args.follow {
		ctx.Printf("Warning: server does not support --follow, showing recent logs only\n")
	}

	// Fall back to legacy pagination
	return legacyLogs(ctx, cl, args.app, args.sandbox, args.last, filter, printer)
}

var streamTypePrefixes = map[string]string{
	"stdout":   "S",
	"stderr":   "E",
	"error":    "ERR",
	"user-oob": "U",
}

// logPrinter returns a function that prints a log entry in either text or JSON format.
func logPrinter(ctx *Context, jsonOutput bool) func(*app_v1alpha.LogEntry) {
	if jsonOutput {
		return func(l *app_v1alpha.LogEntry) {
			printLogEntryJSON(ctx, l)
		}
	}
	return func(l *app_v1alpha.LogEntry) {
		printLogEntry(ctx, l)
	}
}

// logEntryJSON is the JSON representation of a log entry.
type logEntryJSON struct {
	Timestamp  string            `json:"timestamp"`
	Stream     string            `json:"stream"`
	Source     string            `json:"source,omitempty"`
	Message    string            `json:"message"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

func printLogEntryJSON(ctx *Context, l *app_v1alpha.LogEntry) {
	entry := logEntryJSON{
		Timestamp: standard.FromTimestamp(l.Timestamp()).Format(time.RFC3339Nano),
		Stream:    l.Stream(),
		Message:   l.Line(),
	}
	if l.HasSource() && l.Source() != "" {
		entry.Source = l.Source()
	}
	if l.HasAttributes() {
		attrs := l.Attributes()
		if len(attrs) > 0 {
			entry.Attributes = attrs
		}
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	ctx.Printf("%s\n", data)
}

func printLogEntry(ctx *Context, l *app_v1alpha.LogEntry) {
	prefix := ""
	if l.HasSource() && l.Source() != "" {
		source := l.Source()
		if len(source) > 12 {
			source = source[:3] + "…" + source[len(source)-8:]
		}
		prefix = "[" + source + "] "
	}
	attrs := ""
	if l.HasAttributes() {
		attrs = formatAttributes(l.Attributes())
	}
	ctx.Printf("%s %s: %s%s%s\n",
		streamTypePrefixes[l.Stream()],
		standard.FromTimestamp(l.Timestamp()).Format("2006-01-02 15:04:05"),
		prefix,
		l.Line(),
		attrs)
}

func formatAttributes(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	var b strings.Builder
	for _, k := range keys {
		b.WriteByte(' ')
		b.WriteString(k)
		b.WriteByte('=')
		v := m[k]
		if strings.ContainsAny(v, " \t\"\n\r") {
			fmt.Fprintf(&b, "%q", v)
		} else {
			b.WriteString(v)
		}
	}
	return b.String()
}

func streamLogs(ctx *Context, cl *rpc.NetworkClient, app, sandbox string, last *time.Duration, follow bool, filter *logfilter.Filter, printer func(*app_v1alpha.LogEntry)) error {
	ac := app_v1alpha.LogsClient{Client: cl}

	// Build target
	target := &app_v1alpha.LogTarget{}
	if sandbox != "" {
		target.SetSandbox(sandbox)
	} else {
		target.SetApp(app)
	}

	// Determine start time
	var ts *standard.Timestamp
	if last != nil {
		ts = standard.ToTimestamp(time.Now().Add(-*last))
	}
	// When no --last: ts is nil
	// - follow mode: start from now
	// - non-follow: server returns last 100 lines

	// Create callback to print logs as they arrive
	callback := stream.Callback(func(l *app_v1alpha.LogEntry) error {
		// Apply local filter if provided
		if filter != nil && !filter.Match(l.Line()) {
			return nil
		}

		printer(l)
		return nil
	})

	_, err := ac.StreamLogs(ctx, target, ts, follow, callback)
	return err
}

func streamLogChunks(ctx *Context, cl *rpc.NetworkClient, app, sandbox string, last *time.Duration, follow bool, filter string, printer func(*app_v1alpha.LogEntry)) error {
	ac := app_v1alpha.LogsClient{Client: cl}

	// Build target
	target := &app_v1alpha.LogTarget{}
	if sandbox != "" {
		target.SetSandbox(sandbox)
	} else {
		target.SetApp(app)
	}

	// Determine start time
	var ts *standard.Timestamp
	if last != nil {
		ts = standard.ToTimestamp(time.Now().Add(-*last))
	}
	// When no --last: ts is nil
	// - follow mode: start from now
	// - non-follow: server returns last 100 lines

	// Create callback to print logs as they arrive in chunks
	callback := stream.Callback(func(chunk *app_v1alpha.LogChunk) error {
		for _, l := range chunk.Entries() {
			printer(l)
		}
		return nil
	})

	_, err := ac.StreamLogChunks(ctx, target, ts, follow, filter, callback)
	return err
}

func legacyLogs(ctx *Context, cl *rpc.NetworkClient, app, sandbox string, last *time.Duration, filter *logfilter.Filter, printer func(*app_v1alpha.LogEntry)) error {
	ac := app_v1alpha.LogsClient{Client: cl}

	var ts *standard.Timestamp

	if last != nil {
		ts = standard.ToTimestamp(time.Now().Add(-*last))
	} else {
		// Legacy protocol can't do server-side limit=100, so default to
		// last 1 hour as a reasonable bounded window of recent logs.
		ts = standard.ToTimestamp(time.Now().Add(-1 * time.Hour))
	}

	for {
		var (
			res interface {
				Logs() []*app_v1alpha.LogEntry
			}
			err error
		)

		if sandbox != "" {
			res, err = ac.SandboxLogs(ctx, sandbox, ts, false)
		} else {
			res, err = ac.AppLogs(ctx, app, ts, false)
		}

		if err != nil {
			return err
		}

		logs := res.Logs()

		for _, l := range logs {
			// Apply local filter if provided
			if filter != nil && !filter.Match(l.Line()) {
				continue
			}

			printer(l)
		}

		if len(logs) != 100 {
			break
		}

		// For pagination, use the last log's timestamp + 1 microsecond to avoid duplicates
		lastTime := standard.FromTimestamp(logs[len(logs)-1].Timestamp())
		nextTime := lastTime.Add(time.Microsecond)
		ts = standard.ToTimestamp(nextTime)
	}

	return nil
}
