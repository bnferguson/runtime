package commands

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"miren.dev/mflags"
	"miren.dev/runtime/pkg/labs"
)

// fakeResolvers returns a positional-resolver map with deterministic values so
// the engine can be exercised without touching the network or local config.
func fakeResolvers() map[posKey]valueResolver {
	return map[posKey]valueResolver{
		{"cluster switch", 0}: func(*Context) []string { return []string{"prod", "dev"} },
		{"cluster remove", 0}: func(*Context) []string { return []string{"prod", "dev"} },
		// A resolver that opts out (e.g. offline): no suggestions, but still no
		// file fallback, since this is a named resource not a path.
		{"app delete", 0}: func(*Context) []string { return nil },
	}
}

func completionValues(cands []candidate) []string {
	out := make([]string, len(cands))
	for i, c := range cands {
		out[i] = c.value
	}
	return out
}

func TestResolveCompletions(t *testing.T) {
	labs.EnableAll()
	d := mflags.NewDispatcher("miren")
	RegisterAll(d)

	tests := []struct {
		name    string
		words   []string
		want    []string // full expected set when exact is true, else a subset that must be present
		exact   bool
		absent  []string
		wantDir directive
	}{
		{
			name:    "top level prefix",
			words:   []string{"ap"},
			want:    []string{"app", "apps"},
			wantDir: dirNoFileComp,
		},
		{
			name:    "subcommands of app",
			words:   []string{"app", ""},
			want:    []string{"list", "delete", "status"},
			wantDir: dirNoFileComp,
		},
		{
			name:    "deep namespace",
			words:   []string{"debug", "entity", ""},
			want:    []string{"get", "list", "put"},
			wantDir: dirNoFileComp,
		},
		{
			name:    "long flag completion",
			words:   []string{"app", "list", "--"},
			want:    []string{"--format", "--verbose"},
			wantDir: dirNoFileComp,
		},
		{
			name:    "short flag completion",
			words:   []string{"app", "list", "-"},
			want:    []string{"-v"},
			wantDir: dirNoFileComp,
		},
		{
			name:    "enum flag choices",
			words:   []string{"deploy", "--explain-format", ""},
			want:    []string{"auto", "plain", "tty", "rawjson"},
			exact:   true,
			wantDir: dirNoFileComp,
		},
		{
			name:    "dynamic positional values",
			words:   []string{"cluster", "switch", ""},
			want:    []string{"prod", "dev"},
			exact:   true,
			wantDir: dirNoFileComp,
		},
		{
			name:    "dynamic positional with prefix",
			words:   []string{"cluster", "switch", "pr"},
			want:    []string{"prod"},
			exact:   true,
			wantDir: dirNoFileComp,
		},
		{
			name:    "value-taking flag does not shift positional index",
			words:   []string{"cluster", "switch", "--server-address", "host:1", "pr"},
			want:    []string{"prod"},
			exact:   true,
			wantDir: dirNoFileComp,
		},
		{
			name:    "positional without resolver suppresses files",
			words:   []string{"addon", "destroy", ""},
			want:    nil,
			exact:   true,
			wantDir: dirNoFileComp,
		},
		{
			name:    "command with no further args suppresses files",
			words:   []string{"cluster", "list", ""},
			want:    nil,
			exact:   true,
			wantDir: dirNoFileComp,
		},
		{
			name:    "path flag value offers files",
			words:   []string{"deploy", "--options", ""},
			want:    nil,
			exact:   true,
			wantDir: dirDefault,
		},
		{
			name:    "non-path flag value suppresses files",
			words:   []string{"deploy", "--server-address", ""},
			want:    nil,
			exact:   true,
			wantDir: dirNoFileComp,
		},
		{
			name:    "opted-out resolver yields nothing and no file fallback",
			words:   []string{"app", "delete", ""},
			want:    nil,
			exact:   true,
			wantDir: dirNoFileComp,
		},
		{
			name:    "hidden namespace omitted from top level",
			words:   []string{""},
			want:    []string{"app", "deploy"},
			absent:  []string{"internal"},
			wantDir: dirNoFileComp,
		},
		{
			name:    "passthrough after double dash",
			words:   []string{"app", "run", "--", ""},
			want:    nil,
			exact:   true,
			wantDir: dirDefault,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cands, dir := resolveCompletions(d, &Context{}, tt.words, fakeResolvers())
			got := completionValues(cands)

			if tt.exact {
				assert.ElementsMatch(t, tt.want, got)
			} else {
				for _, w := range tt.want {
					assert.Contains(t, got, w)
				}
			}
			for _, a := range tt.absent {
				assert.NotContains(t, got, a)
			}
			assert.Equal(t, tt.wantDir, dir)
		})
	}
}

func TestPrintCompletions(t *testing.T) {
	var buf bytes.Buffer
	printCompletions(&buf, []candidate{
		{value: "list", desc: "List all applications"},
		{value: "delete"},
	}, dirNoFileComp)

	require.Equal(t, "list\tList all applications\ndelete\n:4\n", buf.String())
}

func TestCompletionScriptCommands(t *testing.T) {
	for _, tc := range []struct {
		name   string
		fn     any
		needle string
	}{
		{"bash", CompletionBash, "complete -F"},
		{"zsh", CompletionZsh, "#compdef miren"},
		{"fish", CompletionFish, "complete -c miren"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := RunCommand(tc.fn)
			require.NoError(t, err)
			script := out.Stdout.String()
			assert.Contains(t, script, tc.needle)
			assert.Contains(t, script, "__complete", "script must call the hidden resolver")
		})
	}
}

// TestPositionalResolversReferenceRealCommands guards against typos in the
// production resolver registry: every keyed command path must exist in the tree
// and actually declare a positional at that index.
func TestPositionalResolversReferenceRealCommands(t *testing.T) {
	labs.EnableAll()
	d := mflags.NewDispatcher("miren")
	RegisterAll(d)

	for key := range positionalResolvers() {
		entry := d.GetCommandEntry(key.path)
		require.NotNilf(t, entry, "resolver path %q is not a registered command", key.path)

		fs := entry.Command.FlagSet()
		require.NotNilf(t, fs, "command %q has no flag set", key.path)
		pos := fs.GetPositionalFields()
		assert.Truef(t, key.index < len(pos),
			"command %q has %d positionals but resolver targets index %d",
			key.path, len(pos), key.index)
	}
}

// TestResolversRespectNoNetwork verifies that the opt-out short-circuits every
// server-backed resolver before any RPC client is built, so completion stays
// local-only and never touches the network when asked not to.
func TestResolversRespectNoNetwork(t *testing.T) {
	t.Setenv(completionNoNetworkEnv, "1")

	ctx := &Context{Context: context.Background()}
	resolvers := []struct {
		name string
		fn   valueResolver
	}{
		{"apps", resolveAppNames},
		{"routes", resolveRouteHosts},
		{"sandboxes", resolveSandboxIDs},
		{"pools", resolvePoolIDs},
		{"addons", resolveAddonNames},
		{"runners", resolveRunnerNodes},
		{"tokens", resolveTokenIDs},
	}

	for _, r := range resolvers {
		t.Run(r.name, func(t *testing.T) {
			assert.Nil(t, r.fn(ctx))
		})
	}
}
