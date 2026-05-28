package commands

import (
	"strings"

	"miren.dev/runtime/api/exec/exec_v1alpha"
	"miren.dev/runtime/pkg/rpc/stream"
)

func AppRun(ctx *Context, opts struct {
	AppCentric

	Args []string `rest:"true"`
}) error {
	opt := new(exec_v1alpha.ShellOptions)
	if len(opts.Args) > 0 {
		opt.SetCommand(opts.Args)
	}

	in, out, winUpdates, cleanup := setupExecIO(ctx, opt)
	defer cleanup()

	cl, err := ctx.RPCClient("dev.miren.runtime/exec")
	if err != nil {
		return err
	}

	sec := exec_v1alpha.NewSandboxExecClient(cl)

	results, err := sec.Exec(
		ctx,
		"app", opts.App,
		strings.Join(opts.Args, " "),
		opt,
		stream.ServeReader(ctx, in),
		stream.ServeWriter(ctx, out),
		stream.ChanReader(winUpdates),
	)
	if err != nil {
		return err
	}

	ctx.SetExitCode(int(results.Code()))
	return nil
}
