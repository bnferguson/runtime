package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"miren.dev/mflags"
	"miren.dev/runtime/cli/commands"
	"miren.dev/runtime/pkg/labs"
	"miren.dev/runtime/version"
)

func Run(args []string) int {
	helpJSON := os.Getenv("MIREN_HELP_JSON") != ""

	// When generating help JSON, enable all labs features so every command
	// is registered and included in the output.
	if helpJSON {
		labs.EnableAll()
	}

	// Initialize labs feature flags from environment before registering commands
	if labsEnv := os.Getenv("MIREN_LABS"); labsEnv != "" {
		labs.Init(nil, strings.Split(labsEnv, ","))
	}

	d := mflags.NewDispatcher("miren")

	commands.RegisterAll(d)

	if helpJSON {
		data, err := d.HelpJSON()
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
			return 1
		}
		fmt.Println(string(data))
		return 0
	}

	err := d.Execute(args[1:])
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return 0
		}

		// Check for ErrExitCode
		if exitErr, ok := err.(commands.ErrExitCode); ok {
			return int(exitErr)
		}

		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		return 1
	}

	return 0
}

// Version returns the version string
func Version() string {
	return version.Version
}
