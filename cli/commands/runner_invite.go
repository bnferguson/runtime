package commands

import (
	"time"

	"miren.dev/runtime/api/runner/runner_v1alpha"
	"miren.dev/runtime/pkg/rpc"
	"miren.dev/runtime/pkg/rpc/standard"
)

func RunnerInvite(ctx *Context, opts struct {
	ConfigCentric

	Labels  []string `short:"l" long:"labels" description:"Labels to apply to the runner (key=value format)"`
	Expires int      `short:"e" long:"expires" default:"1" description:"Hours until the invite expires"`
}) error {
	client, err := ctx.RPCClient(rpc.ServiceRunner)
	if err != nil {
		return err
	}
	defer client.Close()

	rc := runner_v1alpha.NewRunnerRegistrationClient(client)

	res, err := rc.CreateInvite(ctx, opts.Labels, int32(opts.Expires))
	if err != nil {
		return err
	}

	code := res.Code()
	expiresAt := standard.FromTimestamp(res.ExpiresAt())

	ctx.Printf("Join code: %s\n", code)
	ctx.Printf("Expires: %s (%s)\n", expiresAt.Format(time.RFC3339), formatDuration(time.Until(expiresAt)))

	if len(opts.Labels) > 0 {
		ctx.Printf("Labels: %v\n", opts.Labels)
	}

	ctx.Printf("\nTo join a runner to this coordinator, run on the runner machine:\n")
	ctx.Printf("  miren runner join <coordinator-address>\n")
	ctx.Printf("\nThen enter the join code when prompted.\n")

	return nil
}
