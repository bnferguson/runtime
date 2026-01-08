package commands

import (
	"errors"

	"miren.dev/runtime/api/deployment/deployment_v1alpha"
)

func DeployCancel(ctx *Context, opts struct {
	ConfigCentric
	DeploymentID string `short:"d" long:"deployment" description:"ID of the deployment to cancel" required:"true"`
}) error {
	client, err := ctx.RPCClient("dev.miren.runtime/deployment")
	if err != nil {
		return err
	}
	defer client.Close()

	depClient := deployment_v1alpha.NewDeploymentClient(client)

	result, err := depClient.CancelDeployment(ctx, opts.DeploymentID, "")
	if err != nil {
		return err
	}

	if result.Error() != "" {
		return errors.New(result.Error())
	}

	ctx.Printf("Cancelled deployment %s\n", opts.DeploymentID)
	return nil
}
