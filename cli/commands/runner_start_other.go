//go:build !linux

package commands

import "fmt"

// RunnerStart is not supported on non-Linux platforms
func RunnerStart(ctx *Context, opts struct {
	ConfigPath       string `long:"config" description:"Path to runner config" default:"/var/lib/miren/runner/config.yaml"`
	DataPath         string `long:"data-path" description:"Path to store runner data" default:"/var/lib/miren/runner"`
	ContainerdSocket string `long:"containerd-socket" description:"Path to containerd socket"`
	ListenAddr       string `short:"l" long:"listen" description:"Address this runner will listen on (overrides config)"`
}) error {
	return fmt.Errorf("runner start is only available on Linux")
}
