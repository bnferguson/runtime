package commands

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"miren.dev/mflags"
)

type helpOpts struct {
	FormatOptions
	ListCommands bool     `long:"commands" description:"List all commands with their synopsis"`
	Commands     []string `rest:"commands"`
}

// commandSummary is used for JSON output of --commands.
type commandSummary struct {
	Path  string `json:"path"`
	Usage string `json:"usage"`
}

// NewHelpCommand returns an Infer-based help command bound to the given dispatcher.
func NewHelpCommand(d *mflags.Dispatcher) *Cmd {
	return Infer("help", "Show help for one or more commands",
		func(ctx *Context, opts helpOpts) error {
			if opts.ListCommands {
				return runListCommands(d, opts)
			}
			if len(opts.Commands) > 0 {
				return runMultiHelp(d, opts)
			}
			return mflags.ErrShowHelp
		},
		WithExample(mflags.Example{
			Name: "List all commands",
			Body: "miren help --commands",
		}),
		WithExample(mflags.Example{
			Name: "List all commands as JSON",
			Body: "miren help --commands --format json",
		}),
		WithExample(mflags.Example{
			Name: "Show help for multiple commands",
			Body: "miren help app.list version sandbox.stop",
		}),
	)
}

func runListCommands(d *mflags.Dispatcher, opts helpOpts) error {
	doc := d.HelpDoc()

	if opts.IsJSON() {
		var summaries []commandSummary
		collectSummaries(doc.Commands, &summaries)
		return PrintJSON(summaries)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	printSummaryRows(w, doc.Commands)
	return w.Flush()
}

func collectSummaries(cmds []mflags.CommandDoc, out *[]commandSummary) {
	for _, cmd := range cmds {
		*out = append(*out, commandSummary{
			Path:  cmd.Path,
			Usage: cmd.Usage,
		})
		collectSummaries(cmd.Subcommands, out)
	}
}

func printSummaryRows(w *tabwriter.Writer, cmds []mflags.CommandDoc) {
	for _, cmd := range cmds {
		fmt.Fprintf(w, "%s\t%s\n", cmd.Path, cmd.Usage)
		printSummaryRows(w, cmd.Subcommands)
	}
}

func runMultiHelp(d *mflags.Dispatcher, opts helpOpts) error {
	if opts.IsJSON() {
		doc := d.HelpDoc()
		index := make(map[string]*mflags.CommandDoc)
		indexCommands(doc.Commands, index)

		var results []mflags.CommandDoc
		for _, name := range opts.Commands {
			path := strings.ReplaceAll(name, ".", " ")
			cmd, ok := index[path]
			if !ok {
				return fmt.Errorf("unknown command: %s", name)
			}
			results = append(results, *cmd)
		}
		return PrintJSON(results)
	}

	for i, name := range opts.Commands {
		path := strings.ReplaceAll(name, ".", " ")

		if i > 0 {
			fmt.Println()
			fmt.Println("---")
			fmt.Println()
		}

		if err := d.ShowCommandHelp(path); err != nil {
			return fmt.Errorf("unknown command: %s", name)
		}
	}
	return nil
}

func indexCommands(cmds []mflags.CommandDoc, index map[string]*mflags.CommandDoc) {
	for i := range cmds {
		index[cmds[i].Path] = &cmds[i]
		indexCommands(cmds[i].Subcommands, index)
	}
}
