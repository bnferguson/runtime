package commands

import (
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"miren.dev/runtime/appconfig"
	"miren.dev/runtime/pkg/color"
)

type helpAliasOpts struct{}

func HelpAlias(ctx *Context, opts helpAliasOpts) error {
	bold := color.New(color.Bold)
	faint := color.New(color.Faint)
	cyan := color.New(color.FgCyan)
	green := color.New(color.FgGreen)

	bold.Println("CLI Aliases")
	fmt.Println()
	fmt.Println("Aliases let you define custom shortcuts for frequently-used commands.")
	fmt.Printf("Define them in %s:\n", cyan.Sprint(appconfig.AppConfigPath))
	fmt.Println()

	faint.Println("  [aliases]")
	faint.Println(`  console = "app run bin/rails console"`)
	faint.Println(`  tail = "logs app -f"`)
	fmt.Println()

	bold.Println("Usage")
	fmt.Println()
	fmt.Printf("  miren %s     %s  miren app run bin/rails console\n",
		green.Sprint("console"), faint.Sprint("→"))
	fmt.Printf("  miren %s -n 50  %s  miren logs app -f -n 50\n",
		green.Sprint("tail"), faint.Sprint("→"))
	fmt.Println()
	fmt.Println("  Extra arguments are appended to the expanded command.")
	fmt.Println()

	bold.Println("Multi-word aliases")
	fmt.Println()
	fmt.Println("  Alias names can contain multiple words to create namespaces:")
	fmt.Println()
	faint.Println(`  "x tail" = "logs app -f"`)
	fmt.Println()
	fmt.Printf("  miren %s  %s  miren logs app -f\n",
		green.Sprint("x tail"), faint.Sprint("→"))
	fmt.Println()

	bold.Println("Rules")
	fmt.Println()
	fmt.Println("  - Alias names must not shadow built-in commands.")
	fmt.Println("  - Aliases expand once — an alias cannot reference another alias.")
	fmt.Println("  - Names use lowercase letters, numbers, dashes, and underscores.")
	fmt.Println()

	ac, err := appconfig.LoadAppConfig()
	if err != nil {
		printConfigWarning(err)
		fmt.Println()
		return nil
	}

	if ac == nil || len(ac.Aliases) == 0 {
		faint.Println("No aliases configured.")
		fmt.Printf("Add an %s section to %s to get started.\n",
			cyan.Sprint("[aliases]"), cyan.Sprint(appconfig.AppConfigPath))
		return nil
	}

	bold.Println("Configured aliases")
	fmt.Println()

	names := make([]string, 0, len(ac.Aliases))
	for name := range ac.Aliases {
		names = append(names, name)
	}
	sort.Strings(names)

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	for _, name := range names {
		fmt.Fprintf(w, "  %s\t%s %s\n",
			green.Sprint(name), faint.Sprint("→"), ac.Aliases[name])
	}
	return w.Flush()
}
