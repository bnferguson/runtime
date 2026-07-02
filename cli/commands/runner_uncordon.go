package commands

import (
	"fmt"

	"miren.dev/runtime/api/runner/runner_v1alpha"
	"miren.dev/runtime/pkg/rpc"
)

func RunnerUncordon(ctx *Context, opts struct {
	ConfigCentric

	Node string `position:"0" usage:"Runner to uncordon (name, ID, or short ID)" required:"true"`
}) error {
	client, err := ctx.RPCClient(rpc.ServiceRunner)
	if err != nil {
		return err
	}
	defer client.Close()

	rc := runner_v1alpha.NewRunnerRegistrationClient(client)

	res, err := rc.UncordonRunner(ctx, opts.Node)
	if err != nil {
		return err
	}

	if res.Error() != "" {
		return fmt.Errorf("%s", res.Error())
	}

	ctx.Printf("Uncordoned runner %q; it is eligible for scheduling again\n", res.Name())

	return nil
}
