package commands

import (
	"fmt"

	"miren.dev/runtime/api/runner/runner_v1alpha"
	"miren.dev/runtime/pkg/rpc"
)

func RunnerCordon(ctx *Context, opts struct {
	ConfigCentric

	Reason string `long:"reason" description:"Optional reason for cordoning the runner"`
	Node   string `position:"0" usage:"Runner to cordon (name, ID, or short ID)" required:"true"`
}) error {
	client, err := ctx.RPCClient(rpc.ServiceRunner)
	if err != nil {
		return err
	}
	defer client.Close()

	rc := runner_v1alpha.NewRunnerRegistrationClient(client)

	res, err := rc.CordonRunner(ctx, opts.Node, opts.Reason)
	if err != nil {
		return err
	}

	if res.Error() != "" {
		return fmt.Errorf("%s", res.Error())
	}

	ctx.Printf("Cordoned runner %q; the scheduler will not place new sandboxes on it (running sandboxes are unaffected)\n", res.Name())

	return nil
}
