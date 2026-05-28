package commands

import (
	"io"
	"os"
	"os/signal"

	"github.com/containerd/console"
	"golang.org/x/sys/unix"
	"miren.dev/runtime/api/exec/exec_v1alpha"
)

// setupExecIO wires stdin/stdout for an interactive exec call.
//
// When both stdin and stdout are TTYs, it puts stdin in raw mode, fills in
// opt.WinSize, and starts a SIGWINCH goroutine that pushes resize events onto
// the returned channel. Otherwise it returns os.Stdin/os.Stdout untouched and
// leaves opt.WinSize unset, which signals the server not to allocate a PTY
// (preserving binary output — see MIR-1001).
//
// The returned cleanup func resets the console and stops the signal handler
// and must always be deferred by the caller.
func setupExecIO(ctx *Context, opt *exec_v1alpha.ShellOptions) (
	io.Reader,
	io.Writer,
	<-chan *exec_v1alpha.WindowSize,
	func(),
) {
	winUpdates := make(chan *exec_v1alpha.WindowSize, 1)

	stdinCon, stdinErr := console.ConsoleFromFile(os.Stdin)
	stdoutCon, stdoutErr := console.ConsoleFromFile(os.Stdout)
	if stdinErr != nil || stdoutErr != nil {
		return os.Stdin, os.Stdout, winUpdates, func() {}
	}

	if csz, err := stdinCon.Size(); err == nil {
		ws := new(exec_v1alpha.WindowSize)
		ws.SetHeight(int32(csz.Height))
		ws.SetWidth(int32(csz.Width))
		opt.SetWinSize(ws)
	}

	if err := stdinCon.SetRaw(); err != nil {
		ctx.Log.Error("failed to set raw mode on stdin", "error", err)
	}

	winCh := make(chan os.Signal, 1)
	signal.Notify(winCh, unix.SIGWINCH)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-winCh:
				csz, err := stdinCon.Size()
				if err != nil {
					ctx.Log.Error("failed to get console size", "error", err)
					continue
				}

				ws := new(exec_v1alpha.WindowSize)
				ws.SetHeight(int32(csz.Height))
				ws.SetWidth(int32(csz.Width))

				winUpdates <- ws
			}
		}
	}()

	cleanup := func() {
		signal.Stop(winCh)
		stdinCon.Reset()
	}

	return stdinCon, stdoutCon, winUpdates, cleanup
}
