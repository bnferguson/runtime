//go:build !linux

package commands

import "fmt"

// RunnerInstall is not supported on non-Linux platforms
func RunnerInstall(ctx *Context, opts struct {
	Token           string   `short:"t" long:"token" description:"Enrollment token from 'miren runner token create'"`
	Coordinator     string   `short:"c" long:"coordinator" description:"Override coordinator address from the token"`
	Name            string   `long:"name" description:"Human-readable name for this runner (defaults to hostname)"`
	ListenAddr      string   `short:"l" long:"listen" description:"Address this runner will listen on"`
	Labels          []string `long:"labels" description:"Runner labels (key=value)"`
	Branch          string   `short:"b" long:"branch" description:"Branch to download"`
	Force           bool     `short:"f" long:"force" description:"Overwrite existing service file"`
	NoStart         bool     `long:"no-start" description:"Do not start the service after installation"`
	SkipSystemCheck bool     `long:"skip-system-check" description:"Skip minimum system requirements check"`
	ConfigPath      string   `long:"config" description:"Path to runner config" default:"/var/lib/miren/runner/config.yaml"`
	DataPath        string   `long:"data-path" description:"Path to store runner data" default:"/var/lib/miren/runner"`
}) error {
	return fmt.Errorf("runner install is only available on Linux")
}

// RunnerUninstall is not supported on non-Linux platforms
func RunnerUninstall(ctx *Context, opts struct {
	RemoveData bool   `long:"remove-data" description:"Remove runner data directory"`
	DataPath   string `long:"data-path" description:"Path to runner data" default:"/var/lib/miren/runner"`
}) error {
	return fmt.Errorf("runner uninstall is only available on Linux")
}

// RunnerServiceStatus is not supported on non-Linux platforms
func RunnerServiceStatus(ctx *Context, opts struct {
	Follow bool `short:"f" long:"follow" description:"Follow logs in real-time"`
}) error {
	return fmt.Errorf("runner service-status is only available on Linux")
}
