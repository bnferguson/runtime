package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"miren.dev/mflags"
	"miren.dev/runtime/clientconfig"
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

// newCompletionContext builds a Context for resolvers to use. The static and
// local resolvers ignore it; the dynamic completion layer extends this to load
// cluster config and bound the work with a timeout.
func newCompletionContext() (*Context, context.CancelFunc) {
	base, cancel := context.WithCancel(context.Background())
	return &Context{
		Context: base,
		Stdout:  io.Discard,
		Stderr:  io.Discard,
	}, cancel
}

// positionalResolvers maps a command's positional argument to the resolver that
// supplies its candidate values. Only values available locally are completed
// here; server-backed resolvers are added by the dynamic completion layer.
func positionalResolvers() map[posKey]valueResolver {
	return map[posKey]valueResolver{
		{"cluster switch", 0}: resolveClusterNames,
		{"cluster remove", 0}: resolveClusterNames,
	}
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

// CompletionBash prints a bash completion script for miren.
func CompletionBash(ctx *Context, opts struct{}) error {
	ctx.Printf("%s", bashCompletionScript)
	return nil
}

// CompletionZsh prints a zsh completion script for miren.
func CompletionZsh(ctx *Context, opts struct{}) error {
	ctx.Printf("%s", zshCompletionScript)
	return nil
}

// CompletionFish prints a fish completion script for miren.
func CompletionFish(ctx *Context, opts struct{}) error {
	ctx.Printf("%s", fishCompletionScript)
	return nil
}

const bashCompletionScript = `# bash completion for miren
_miren_complete() {
    local cword=$COMP_CWORD
    local -a args=("${COMP_WORDS[@]:1:cword}")

    # Call the binary by the name actually typed (miren, m, a path, ...) so the
    # same function works for any command this completion is bound to.
    local out
    out="$("${COMP_WORDS[0]}" __complete "${args[@]}" 2>/dev/null)" || return

    local directive=0
    local -a comps=()
    local line
    while IFS= read -r line; do
        case "$line" in
            :*) directive="${line#:}" ;;
            *)  comps+=("${line%%$'\t'*}") ;;
        esac
    done <<< "$out"

    COMPREPLY=($(compgen -W "${comps[*]}" -- "${COMP_WORDS[cword]}"))

    if (( (directive & 4) == 0 )); then
        compopt -o default 2>/dev/null
    fi
}
complete -F _miren_complete miren
`

const zshCompletionScript = `#compdef miren

# Source this script or place it on your fpath to enable completion:
#   source <(miren completion zsh)

_miren() {
    local -a args
    # (@) preserves each word as a separate element (including a trailing empty
    # word under the cursor); a plain quoted range would join them into one.
    args=("${(@)words[2,CURRENT]}")

    # words[1] is the command as typed (miren, m, a path, ...), so the same
    # function works for any command this completion is bound to.
    local out
    out="$("${words[1]}" __complete "${args[@]}" 2>/dev/null)"

    local -a comps
    local directive=0 line
    while IFS= read -r line; do
        if [[ "$line" == :* ]]; then
            directive="${line#:}"
            continue
        fi
        if [[ "$line" == *$'\t'* ]]; then
            comps+=("${line%%$'\t'*}:${line#*$'\t'}")
        else
            comps+=("$line")
        fi
    done <<< "$out"

    _describe -V 'miren' comps

    if (( (directive & 4) == 0 )); then
        _files
    fi
}

compdef _miren miren
`

const fishCompletionScript = `# fish completion for miren
#   miren completion fish | source

function __miren_complete
    set -l tokens (commandline -opc)
    set -l current (commandline -ct)
    set -l args
    if test (count $tokens) -gt 1
        set args $tokens[2..-1]
    end
    set args $args $current

    # $tokens[1] is the command as typed (miren, m, a path, ...), so the same
    # function works for any command this completion is bound to.
    set -l directive 0
    for line in ($tokens[1] __complete $args 2>/dev/null)
        set -l d (string match -r '^:(.*)' -- $line)
        if test (count $d) -gt 0
            set directive $d[2]
        else
            echo $line # "value\tdescription"; fish renders the description
        end
    end

    # Offer files only when the directive permits it (unknown positionals,
    # path-valued flags); otherwise the suggestions above are the full set.
    if test "$directive" = 0
        __fish_complete_path $current
    end
end

complete -c miren -f -a '(__miren_complete)'
`
