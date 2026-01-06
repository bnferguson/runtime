package commands

import (
	"errors"

	"miren.dev/runtime/api/deployment/deployment_v1alpha"
)

func DeployCancel(ctx *Context, opts struct {
	ConfigCentric
	Args struct {
		DeploymentID string `positional-arg-name:"deployment-id" description:"ID of the deployment to cancel"`
	} `positional-args:"yes" required:"yes"`
}) error {
	// Get current user ID from JWT claims (if authenticated)
	callerUserId := ctx.GetCurrentUserID()

	client, err := ctx.RPCClient("deployment")
	if err != nil {
		return err
	}
	defer client.Close()

	depClient := &deployment_v1alpha.DeploymentClient{Client: client}

	result, err := depClient.CancelDeployment(ctx, opts.Args.DeploymentID, callerUserId)
	if err != nil {
		return err
	}

	if result.Error() != "" {
		return errors.New(result.Error())
	}

	ctx.Printf("Cancelled deployment %s\n", opts.Args.DeploymentID)
	return nil
}
