package commands

import (
	"fmt"

	"miren.dev/runtime/api/runner/runner_v1alpha"
	"miren.dev/runtime/pkg/rpc"
)

func RunnerRevoke(ctx *Context, opts struct {
	ConfigCentric

	InviteID string `position:"0" usage:"ID of the invite to revoke" required:"true"`
}) error {
	client, err := ctx.RPCClient(rpc.ServiceRunner)
	if err != nil {
		return err
	}
	defer client.Close()

	rc := runner_v1alpha.NewRunnerRegistrationClient(client)

	res, err := rc.RevokeInvite(ctx, opts.InviteID)
	if err != nil {
		return err
	}

	if res.Error() != "" {
		return fmt.Errorf("revoke failed: %s", res.Error())
	}

	if !res.Success() {
		return fmt.Errorf("revoke failed")
	}

	ctx.Printf("Invite revoked.\n")
	return nil
}
