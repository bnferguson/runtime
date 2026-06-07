package commands

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
