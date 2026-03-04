package commands

import (
	"fmt"
	"strings"
	"time"

	"miren.dev/runtime/api/app/app_v1alpha"
	"miren.dev/runtime/appconfig"
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

// resolveAppName resolves the application name from explicit flag, directory
// context, or returns an error if it can't be determined.
func resolveAppName(app, dir string) (string, error) {
	if app != "" {
		return app, nil
	}

	var ac *appconfig.AppConfig
	var err error

	if dir != "." {
		ac, err = appconfig.LoadAppConfigUnder(dir)
	} else {
		ac, err = appconfig.LoadAppConfig()
	}

	if err == nil && ac != nil && ac.Name != "" {
		return ac.Name, nil
	}
	return "", fmt.Errorf("must specify --app or run from an app directory")
}

// LogsApp shows application logs. This is the default subcommand for `miren logs`.
func LogsApp(ctx *Context, opts struct {
	ConfigCentric

	App     string         `short:"a" long:"app" description:"Application to get logs for" env:"MIREN_APP"`
	Dir     string         `short:"d" long:"dir" description:"Directory to run from" default:"."`
	Last    *time.Duration `short:"l" long:"last" description:"Show logs from the last duration"`
	Follow  bool           `short:"f" long:"follow" description:"Follow log output (live tail)"`
	Filter  string         `short:"g" long:"grep" description:"Filter logs (e.g., 'error', '\"exact phrase\"', 'error -debug', '/regex/')"`
	Service string         `long:"service" description:"Filter logs by service name (e.g., 'web', 'worker')"`
}) error {
	app, err := resolveAppName(opts.App, opts.Dir)
	if err != nil {
		return err
	}

	cl, err := ctx.RPCClient("dev.miren.runtime/logs")
	if err != nil {
		return err
	}

	combinedFilter := buildFilterWithService(opts.Filter, opts.Service)
	return dispatchLogs(ctx, cl, app, "", opts.Last, opts.Follow, opts.Filter, combinedFilter)
}

// LogsSandbox shows logs for a specific sandbox.
func LogsSandbox(ctx *Context, opts struct {
	ConfigCentric

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

	return dispatchLogs(ctx, cl, "", sandbox, opts.Last, opts.Follow, opts.Filter, opts.Filter)
}

// LogsBuild shows build logs for a specific version.
func LogsBuild(ctx *Context, opts struct {
	ConfigCentric

	App     string         `short:"a" long:"app" description:"Application to get logs for" env:"MIREN_APP"`
	Dir     string         `short:"d" long:"dir" description:"Directory to run from" default:"."`
	Version string         `position:"0" usage:"Build version (e.g., v3)" required:"true"`
	Last    *time.Duration `short:"l" long:"last" description:"Show logs from the last duration"`
	Follow  bool           `short:"f" long:"follow" description:"Follow log output (live tail)"`
	Filter  string         `short:"g" long:"grep" description:"Filter logs (e.g., 'error', '\"exact phrase\"', 'error -debug', '/regex/')"`
}) error {
	app, err := resolveAppName(opts.App, opts.Dir)
	if err != nil {
		return err
	}

	cl, err := ctx.RPCClient("dev.miren.runtime/logs")
	if err != nil {
		return err
	}

	combinedFilter := buildBuildFilter(opts.Version, opts.Filter)
	return dispatchLogs(ctx, cl, app, "", opts.Last, opts.Follow, opts.Filter, combinedFilter)
}

// LogsSystem shows system/server logs, optionally filtered by component.
func LogsSystem(ctx *Context, opts struct {
	ConfigCentric

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

	ac := app_v1alpha.LogsClient{Client: cl}
	callback := stream.Callback(func(chunk *app_v1alpha.LogChunk) error {
		for _, l := range chunk.Entries() {
			printLogEntry(ctx, l)
		}
		return nil
	})

	_, err = ac.StreamLogChunks(ctx, target, ts, opts.Follow, combinedFilter, callback)
	return err
}

// dispatchLogs handles protocol negotiation and dispatches to the appropriate
// log streaming method based on server capabilities.
func dispatchLogs(ctx *Context, cl *rpc.NetworkClient, app, sandbox string, last *time.Duration, follow bool, rawFilter, combinedFilter string) error {
	// Check if server supports streaming (prefer chunked for efficiency)
	if cl.HasMethod(ctx, "streamLogChunks") {
		return streamLogChunks(ctx, cl, app, sandbox, last, follow, combinedFilter)
	}

	// Older server - warn about upgrade and limited functionality
	ctx.Printf("Warning: server does not support optimized log streaming. Upgrade your server for better performance and --service/--build filtering.\n")

	// Parse filter for client-side filtering on older protocol
	var filter *logfilter.Filter
	if rawFilter != "" {
		var err error
		filter, err = logfilter.Parse(rawFilter)
		if err != nil {
			return fmt.Errorf("invalid filter: %w", err)
		}
	}

	if cl.HasMethod(ctx, "streamLogs") {
		return streamLogs(ctx, cl, app, sandbox, last, follow, filter)
	}

	// Warn if --follow requested but not supported
	if follow {
		ctx.Printf("Warning: server does not support --follow, showing recent logs only\n")
	}

	// Fall back to legacy pagination
	return legacyLogs(ctx, cl, app, sandbox, last, filter)
}

var streamTypePrefixes = map[string]string{
	"stdout":   "S",
	"stderr":   "E",
	"error":    "ERR",
	"user-oob": "U",
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
	ctx.Printf("%s %s: %s%s\n",
		streamTypePrefixes[l.Stream()],
		standard.FromTimestamp(l.Timestamp()).Format("2006-01-02 15:04:05"),
		prefix,
		l.Line())
}

func streamLogs(ctx *Context, cl *rpc.NetworkClient, app, sandbox string, last *time.Duration, follow bool, filter *logfilter.Filter) error {
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

		printLogEntry(ctx, l)
		return nil
	})

	_, err := ac.StreamLogs(ctx, target, ts, follow, callback)
	return err
}

func streamLogChunks(ctx *Context, cl *rpc.NetworkClient, app, sandbox string, last *time.Duration, follow bool, filter string) error {
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
			printLogEntry(ctx, l)
		}
		return nil
	})

	_, err := ac.StreamLogChunks(ctx, target, ts, follow, filter, callback)
	return err
}

func legacyLogs(ctx *Context, cl *rpc.NetworkClient, app, sandbox string, last *time.Duration, filter *logfilter.Filter) error {
	ac := app_v1alpha.LogsClient{Client: cl}

	var ts *standard.Timestamp

	if last != nil {
		ts = standard.ToTimestamp(time.Now().Add(-*last))
	} else {
		start := time.Now()
		start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
		ts = standard.ToTimestamp(start)
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

			printLogEntry(ctx, l)
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
