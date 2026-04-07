package commands

import (
	"fmt"

	"miren.dev/runtime/api/runner/runner_v1alpha"
	"miren.dev/runtime/pkg/rpc"
)

func RunnerRemove(ctx *Context, opts struct {
	ConfigCentric

	Force bool `long:"force" short:"f" description:"Force removal even if the runner has active sandboxes"`

	Args struct {
		Node string `positional-arg-name:"node" description:"Runner to remove (name, ID, or short ID)" required:"true"`
	} `positional-args:"yes" required:"true"`
}) error {
	client, err := ctx.RPCClient(rpc.ServiceRunner)
	if err != nil {
		return err
	}
	defer client.Close()

	rc := runner_v1alpha.NewRunnerRegistrationClient(client)

	res, err := rc.RemoveRunner(ctx, opts.Args.Node, opts.Force)
	if err != nil {
		return err
	}

	if res.Error() != "" {
		return fmt.Errorf("%s", res.Error())
	}

	ctx.Printf("Removed runner %q", res.Name())
	if res.RemovedResources() > 0 {
		ctx.Printf(" (%d associated resources cleaned up)", res.RemovedResources())
	}
	ctx.Printf("\n")

	return nil
}
