package commands

import (
	"fmt"

	"miren.dev/runtime/api/runner/runner_v1alpha"
	"miren.dev/runtime/pkg/rpc"
)

func RunnerDrain(ctx *Context, opts struct {
	ConfigCentric

	Timeout int64  `long:"timeout" description:"Max seconds to wait for the node to empty (0 uses the server default)"`
	Reason  string `long:"reason" description:"Optional reason for draining the runner"`
	Node    string `position:"0" usage:"Runner to drain (name, ID, or short ID)" required:"true"`
}) error {
	client, err := ctx.RPCClient(rpc.ServiceRunner)
	if err != nil {
		return err
	}
	defer client.Close()

	rc := runner_v1alpha.NewRunnerRegistrationClient(client)

	ctx.Printf("Draining runner %q...\n", opts.Node)

	res, err := rc.DrainRunner(ctx, opts.Node, opts.Reason, opts.Timeout)
	if err != nil {
		return err
	}

	if res.Error() != "" {
		return fmt.Errorf("%s", res.Error())
	}

	ctx.Printf("Cordoned and evicted %d sandbox(es) from runner %q; they will be rescheduled onto other ready nodes\n",
		res.EvictedCount(), res.Name())
	if res.TimedOut() {
		ctx.Printf("warning: timed out waiting for the node to fully empty; some sandboxes may still be draining\n")
	}
	ctx.Printf("Run 'miren runner uncordon %s' when the runner is ready to accept work again\n", opts.Node)

	return nil
}
