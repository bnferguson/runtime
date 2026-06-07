package commands

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"slices"
	"strings"
	"time"

	"miren.dev/mflags"
	"miren.dev/runtime/api/addon/addon_v1alpha"
	"miren.dev/runtime/api/app/app_v1alpha"
	"miren.dev/runtime/api/compute"
	"miren.dev/runtime/api/compute/compute_v1alpha"
	"miren.dev/runtime/api/entityserver/entityserver_v1alpha"
	"miren.dev/runtime/api/ingress"
	"miren.dev/runtime/api/runner/runner_v1alpha"
	"miren.dev/runtime/clientconfig"
	"miren.dev/runtime/pkg/rpc"
)

// posKey identifies a positional argument by its command path and zero-based
// index (e.g. {"route set", 1} is the app name in "route set <host> <app>").
type posKey struct {
	path  string
	index int
}

// valueResolver produces candidate values for a positional argument. It returns
// nil when it has nothing to offer (e.g. an unreachable server); the engine then
// suggests nothing for that argument rather than guessing. Resource-name
// positionals never fall back to file names.
type valueResolver func(ctx *Context) []string

// candidate is a single suggestion. desc is shown by shells that support
// described completions (zsh, fish) and ignored by bash.
type candidate struct {
	value string
	desc  string
}

// directive tells the shell how to treat the results. The values match the
// subset of cobra's ShellCompDirective that the generated scripts understand.
type directive int

const (
	dirDefault    directive = 0 // allow the shell's default (file) completion
	dirNoFileComp directive = 4 // results are complete; do not add files
)

// completionTimeout bounds the server round-trips a single completion may make.
const completionTimeout = 1500 * time.Millisecond

// defaultCompletionServerAddress mirrors the GlobalFlags default and is used
// when no cluster is configured but a local server may still be running.
const defaultCompletionServerAddress = "127.0.0.1:8443"

// completionNoNetworkEnv disables every server-backed resolver when set, for
// users who would rather not pay for round-trips while completing.
const completionNoNetworkEnv = "MIREN_COMPLETION_NO_NETWORK"

// Complete is the entry point for the hidden "__complete" command that the
// generated completion scripts (see CompletionBash/Zsh/Fish) call to ask the
// binary what to suggest. words are the tokens typed after the program name, the
// last of which is the (possibly empty) word under the cursor. Resolving against
// the live dispatcher keeps completion in sync with the real command tree,
// including feature-gated commands only registered when enabled. It always
// succeeds so the shell never sees an error.
func Complete(d *mflags.Dispatcher, words []string) int {
	ctx, cancel := newCompletionContext()
	defer cancel()

	cands, dir := resolveCompletions(d, ctx, words, positionalResolvers())
	printCompletions(os.Stdout, cands, dir)
	return 0
}

// newCompletionContext builds a Context for resolvers to use. It loads client
// config so server-backed resolvers can reach the cluster, and bounds the work
// with a deadline so tab completion never hangs on a slow or unreachable server.
func newCompletionContext() (*Context, context.CancelFunc) {
	base, cancel := context.WithTimeout(context.Background(), completionTimeout)
	ctx := &Context{
		Context: base,
		Stdout:  io.Discard,
		Stderr:  io.Discard,
		Log:     slog.New(slog.DiscardHandler),
	}
	ctx.Config.ServerAddress = defaultCompletionServerAddress
	loadCompletionConfig(ctx)
	return ctx, cancel
}

// loadCompletionConfig populates the Context with client and cluster config so
// the server-backed resolvers can reach the cluster. It is best-effort: when
// nothing is configured the resolvers simply return no suggestions.
func loadCompletionConfig(ctx *Context) {
	cfg, err := clientconfig.LoadConfig()
	if err != nil {
		return
	}
	ctx.ClientConfig = cfg

	name := cfg.ActiveCluster()
	if name == "" {
		return
	}
	if cluster, err := cfg.GetCluster(name); err == nil {
		ctx.ClusterConfig = cluster
		ctx.ClusterName = name
	}
}

// positionalResolvers maps a command's positional argument to the resolver that
// supplies its candidate values. Local resolvers read from client config;
// server-backed ones reach the cluster (see completeViaClient).
func positionalResolvers() map[posKey]valueResolver {
	return map[posKey]valueResolver{
		// Local: read from client config, no server round-trip.
		{"cluster switch", 0}: resolveClusterNames,
		{"cluster remove", 0}: resolveClusterNames,

		// Server-backed. "route set" takes a NEW host at position 0, so only its
		// app argument is completed.
		{"app delete", 0}:        resolveAppNames,
		{"route set", 1}:         resolveAppNames,
		{"route set-default", 0}: resolveAppNames,
		{"route show", 0}:        resolveRouteHosts,
		{"route remove", 0}:      resolveRouteHosts,
		{"route protect", 0}:     resolveRouteHosts,
		{"route unprotect", 0}:   resolveRouteHosts,
		{"route waf", 0}:         resolveRouteHosts,
		{"sandbox stop", 0}:      resolveSandboxIDs,
		{"sandbox delete", 0}:    resolveSandboxIDs,
		{"logs sandbox", 0}:      resolveSandboxIDs,

		{"sandbox-pool set-desired", 0}: resolvePoolIDs,
		{"addon variants", 0}:           resolveAddonNames,
		{"runner remove", 0}:            resolveRunnerNodes,
		{"runner token revoke", 0}:      resolveTokenIDs,
	}
}

// resolveCompletions returns the suggestions and shell directive for the tokens
// typed so far. It is pure aside from the injected resolvers, which keeps it
// unit-testable without a dispatcher mutation or a server.
func resolveCompletions(d *mflags.Dispatcher, ctx *Context, words []string, resolvers map[posKey]valueResolver) ([]candidate, directive) {
	cur := ""
	if n := len(words); n > 0 {
		cur = words[n-1]
		words = words[:n-1]
	}

	// After "--" everything is passed through verbatim (e.g. the command for
	// "app run -- ..."), so leave it to the shell's default file completion.
	if slices.Contains(words, "--") {
		return nil, dirDefault
	}

	cmdPath, posIndex := walkCommand(d, words)

	// Completing a flag name.
	if strings.HasPrefix(cur, "-") {
		return completeFlags(d, cmdPath, cur), dirNoFileComp
	}

	// Completing the value of the immediately preceding flag.
	if len(words) > 0 {
		if cands, dir, ok := completeFlagValue(d, cmdPath, words[len(words)-1], cur); ok {
			return cands, dir
		}
	}

	// Completing a subcommand of the resolved command or namespace.
	if children := d.GetDirectChildren(cmdPath); len(children) > 0 {
		return completeChildren(children, cur), dirNoFileComp
	}

	// Completing a positional argument value.
	if resolve, ok := resolvers[posKey{cmdPath, posIndex}]; ok {
		return filterCandidates(resolve(ctx), cur), dirNoFileComp
	}

	// Nothing left to complete. miren positionals are names and IDs, never file
	// paths, so suppress the shell's file fallback instead of offering nonsense.
	return nil, dirNoFileComp
}

// walkCommand consumes the typed tokens to find the deepest matching command
// path and how many positional values have already been supplied to it. Flags
// and the values they take are skipped so they never count as positionals.
func walkCommand(d *mflags.Dispatcher, words []string) (cmdPath string, posIndex int) {
	for i := 0; i < len(words); i++ {
		tok := words[i]
		if strings.HasPrefix(tok, "-") {
			if !strings.Contains(tok, "=") && flagTakesValue(d, cmdPath, tok) {
				i++ // the next token is this flag's value
			}
			continue
		}

		next := tok
		if cmdPath != "" {
			next = cmdPath + " " + tok
		}
		if isCommandOrNamespace(d, next) {
			cmdPath = next
		} else {
			posIndex++
		}
	}
	return cmdPath, posIndex
}

// isCommandOrNamespace reports whether path is a registered command or a parent
// of one (an implicit namespace).
func isCommandOrNamespace(d *mflags.Dispatcher, path string) bool {
	return d.GetCommandEntry(path) != nil || len(d.GetDirectChildren(path)) > 0
}

func completeFlags(d *mflags.Dispatcher, cmdPath, cur string) []candidate {
	fs := commandFlagSet(d, cmdPath)
	if fs == nil {
		return nil
	}

	var cands []candidate
	fs.VisitAll(func(f *mflags.Flag) {
		if f.Hidden {
			return
		}
		if f.Name != "" {
			if long := "--" + f.Name; strings.HasPrefix(long, cur) {
				cands = append(cands, candidate{value: long, desc: f.Usage})
			}
		}
		if f.Short != 0 {
			if short := "-" + string(f.Short); strings.HasPrefix(short, cur) {
				cands = append(cands, candidate{value: short, desc: f.Usage})
			}
		}
	})
	return cands
}

// completeFlagValue handles the value position of a value-taking flag. It
// returns ok=false when prevTok is not such a flag so the caller can continue.
func completeFlagValue(d *mflags.Dispatcher, cmdPath, prevTok, cur string) ([]candidate, directive, bool) {
	if !strings.HasPrefix(prevTok, "-") || strings.Contains(prevTok, "=") {
		return nil, 0, false
	}

	f := lookupFlag(d, cmdPath, prevTok)
	if f == nil || f.Value.IsBool() {
		return nil, 0, false
	}

	if cp, ok := f.Value.(mflags.ChoiceProvider); ok {
		return filterCandidates(cp.Choices(), cur), dirNoFileComp, true
	}

	// A value is expected but we can't enumerate it. Offer files only for flags
	// that take a path; other values (app names, addresses, ...) have no useful
	// file fallback, so suppress it.
	if flagTakesFile(f.Name) {
		return nil, dirDefault, true
	}
	return nil, dirNoFileComp, true
}

// flagTakesFile reports whether a flag conventionally takes a filesystem path,
// so the shell should offer file completion for its value.
func flagTakesFile(name string) bool {
	switch name {
	case "file", "files", "config", "options", "input", "output", "path", "dir", "directory":
		return true
	default:
		return false
	}
}

func completeChildren(children []mflags.ChildEntry, cur string) []candidate {
	var cands []candidate
	for _, ch := range children {
		if ch.Group == GroupHidden {
			continue // hidden from help; hide from completion too (e.g. "internal")
		}
		if strings.HasPrefix(ch.Name, cur) {
			cands = append(cands, candidate{value: ch.Name, desc: ch.Usage})
		}
	}
	return cands
}

func filterCandidates(values []string, cur string) []candidate {
	var cands []candidate
	for _, v := range values {
		if strings.HasPrefix(v, cur) {
			cands = append(cands, candidate{value: v})
		}
	}
	return cands
}

// flagTakesValue reports whether tok names a flag on the command at cmdPath that
// consumes the following token as its value.
func flagTakesValue(d *mflags.Dispatcher, cmdPath, tok string) bool {
	f := lookupFlag(d, cmdPath, tok)
	return f != nil && !f.Value.IsBool()
}

// lookupFlag finds the flag named by tok (long or short form) on the command at
// cmdPath, or nil if there is no such flag.
func lookupFlag(d *mflags.Dispatcher, cmdPath, tok string) *mflags.Flag {
	fs := commandFlagSet(d, cmdPath)
	if fs == nil {
		return nil
	}

	name, _, _ := strings.Cut(strings.TrimLeft(tok, "-"), "=")
	if name == "" {
		return nil
	}
	if f := fs.Lookup(name); f != nil {
		return f
	}

	if r := []rune(name); len(r) == 1 {
		var found *mflags.Flag
		fs.VisitAll(func(f *mflags.Flag) {
			if found == nil && f.Short == r[0] {
				found = f
			}
		})
		return found
	}
	return nil
}

func commandFlagSet(d *mflags.Dispatcher, cmdPath string) *mflags.FlagSet {
	entry := d.GetCommandEntry(cmdPath)
	if entry == nil {
		return nil
	}
	return entry.Command.FlagSet()
}

// printCompletions writes one candidate per line as "value\tdescription" (the
// description is omitted when empty), followed by a ":<directive>" trailer that
// the shell scripts parse.
func printCompletions(w io.Writer, cands []candidate, dir directive) {
	for _, c := range cands {
		if c.desc != "" {
			fmt.Fprintf(w, "%s\t%s\n", c.value, c.desc)
		} else {
			fmt.Fprintln(w, c.value)
		}
	}
	fmt.Fprintf(w, ":%d\n", int(dir))
}

// resolveClusterNames lists the locally configured cluster names. It reads the
// client config directly, so completion stays instant with no server round-trip.
func resolveClusterNames(*Context) []string {
	cfg, err := clientconfig.LoadConfig()
	if err != nil {
		return nil
	}
	return cfg.GetClusterNames()
}

// completeViaClient runs fn with an RPC client for the given service. Server-backed
// completion is best-effort: tab completion must feel instant, so it returns nil
// (no suggestions) when completion is disabled, the server is unreachable, or the
// deadline fires. The work runs in a goroutine selected against ctx so a hung dial
// can never block the shell; this relies on ctx carrying a deadline, which
// newCompletionContext always sets. The short-lived process exits right after
// printing, so the goroutine is not a meaningful leak.
func completeViaClient(ctx *Context, service string, fn func(*rpc.NetworkClient) []string) []string {
	if os.Getenv(completionNoNetworkEnv) != "" {
		return nil
	}

	result := make(chan []string, 1)
	go func() {
		client, err := ctx.RPCClient(service)
		if err != nil {
			result <- nil
			return
		}
		defer client.Close()
		result <- fn(client)
	}()

	select {
	case <-ctx.Done():
		return nil
	case values := <-result:
		return values
	}
}

func resolveAppNames(ctx *Context) []string {
	return completeViaClient(ctx, "dev.miren.runtime/app", func(client *rpc.NetworkClient) []string {
		res, err := app_v1alpha.NewCrudClient(client).List(ctx)
		if err != nil {
			return nil
		}
		apps := res.Apps()
		names := make([]string, 0, len(apps))
		for _, a := range apps {
			names = append(names, a.Name())
		}
		return names
	})
}

func resolveRouteHosts(ctx *Context) []string {
	return completeViaClient(ctx, "entities", func(client *rpc.NetworkClient) []string {
		routes, err := ingress.NewClient(ctx.Log, client).List(ctx)
		if err != nil {
			return nil
		}
		var hosts []string
		for _, r := range routes {
			if r.Route.Host != "" { // the default route has no host
				hosts = append(hosts, r.Route.Host)
			}
		}
		return hosts
	})
}

func resolveSandboxIDs(ctx *Context) []string {
	return completeViaClient(ctx, "entities", func(client *rpc.NetworkClient) []string {
		eac := entityserver_v1alpha.NewEntityAccessClient(client)
		kind, err := eac.LookupKind(ctx, "sandbox")
		if err != nil {
			return nil
		}
		res, err := eac.List(ctx, kind.Attr())
		if err != nil {
			return nil
		}
		var ids []string
		for _, e := range res.Values() {
			var sandbox compute_v1alpha.Sandbox
			sandbox.Decode(e.Entity())
			if compute.SandboxDead(sandbox.Status) {
				continue
			}
			ids = append(ids, sandbox.ID.String())
		}
		return ids
	})
}

func resolvePoolIDs(ctx *Context) []string {
	return completeViaClient(ctx, "entities", func(client *rpc.NetworkClient) []string {
		eac := entityserver_v1alpha.NewEntityAccessClient(client)
		kind, err := eac.LookupKind(ctx, "sandbox_pool")
		if err != nil {
			return nil
		}
		res, err := eac.List(ctx, kind.Attr())
		if err != nil {
			return nil
		}
		var ids []string
		for _, e := range res.Values() {
			var pool compute_v1alpha.SandboxPool
			pool.Decode(e.Entity())
			ids = append(ids, pool.ID.String())
		}
		return ids
	})
}

func resolveAddonNames(ctx *Context) []string {
	return completeViaClient(ctx, "entities", func(client *rpc.NetworkClient) []string {
		eac := entityserver_v1alpha.NewEntityAccessClient(client)
		kind, err := eac.LookupKind(ctx, "addon")
		if err != nil {
			return nil
		}
		res, err := eac.List(ctx, kind.Attr())
		if err != nil {
			return nil
		}
		var names []string
		for _, e := range res.Values() {
			var addon addon_v1alpha.Addon
			addon.Decode(e.Entity())
			names = append(names, addon.Name)
		}
		return names
	})
}

func resolveRunnerNodes(ctx *Context) []string {
	return completeViaClient(ctx, rpc.ServiceRunner, func(client *rpc.NetworkClient) []string {
		res, err := runner_v1alpha.NewRunnerRegistrationClient(client).ListRunners(ctx)
		if err != nil {
			return nil
		}
		var names []string
		for _, r := range res.Runners() {
			if name := r.Name(); name != "" {
				names = append(names, name)
			} else {
				names = append(names, r.RunnerId())
			}
		}
		return names
	})
}

func resolveTokenIDs(ctx *Context) []string {
	return completeViaClient(ctx, rpc.ServiceRunner, func(client *rpc.NetworkClient) []string {
		res, err := runner_v1alpha.NewRunnerRegistrationClient(client).ListInvites(ctx)
		if err != nil {
			return nil
		}
		var ids []string
		for _, inv := range res.Invites() {
			// Already-revoked or expired tokens can't be revoked again.
			switch strings.TrimPrefix(inv.Status(), "status.") {
			case "revoked", "expired":
				continue
			}
			ids = append(ids, inv.Id())
		}
		return ids
	})
}
