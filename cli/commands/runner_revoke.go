package commands

import (
	"fmt"

	"miren.dev/runtime/api/runner/runner_v1alpha"
	"miren.dev/runtime/pkg/rpc"
)

func RunnerTokenRevoke(ctx *Context, opts struct {
	ConfigCentric

	TokenID string `position:"0" usage:"ID of the token to revoke" required:"true"`
}) error {
	client, err := ctx.RPCClient(rpc.ServiceRunner)
	if err != nil {
		return err
	}
	defer client.Close()

	rc := runner_v1alpha.NewRunnerRegistrationClient(client)

	res, err := rc.RevokeInvite(ctx, opts.TokenID)
	if err != nil {
		return err
	}

	if res.Error() != "" {
		return fmt.Errorf("revoke failed: %s", res.Error())
	}

	if !res.Success() {
		return fmt.Errorf("revoke failed")
	}

	ctx.Printf("Token revoked.\n")
	return nil
}
