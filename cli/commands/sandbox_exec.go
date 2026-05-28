package commands

import (
	"fmt"
	"strings"

	"miren.dev/runtime/api/exec/exec_v1alpha"
	"miren.dev/runtime/pkg/rpc/stream"
)

func SandboxExec(ctx *Context, opts struct {
	ConfigCentric
	Id string `short:"i" long:"id" description:"Sandbox ID"`

	Args []string `rest:"true"`
}) error {
	id := opts.Id
	args := opts.Args
	if id == "" {
		if len(args) == 0 {
			return fmt.Errorf("sandbox ID is required (pass as first positional arg or via --id)")
		}
		id, args = args[0], args[1:]
	}

	cl, err := ctx.RPCClient("dev.miren.runtime/exec")
	if err != nil {
		return err
	}

	sec := exec_v1alpha.NewSandboxExecClient(cl)

	opt := new(exec_v1alpha.ShellOptions)
	opt.SetCommand(args)

	in, out, winUpdates, cleanup := setupExecIO(ctx, opt)
	defer cleanup()

	res, err := sec.Exec(
		ctx,
		"id", id,
		strings.Join(args, " "),
		opt,
		stream.ServeReader(ctx, in),
		stream.ServeWriter(ctx, out),
		stream.ChanReader(winUpdates),
	)
	if err != nil {
		return err
	}

	ctx.SetExitCode(int(res.Code()))
	return nil
}
