package commands

import (
	"miren.dev/runtime/api/app/app_v1alpha"
)

func AppRestart(ctx *Context, opts struct {
	Service string `short:"s" long:"service" description:"Restart only a specific service"`
	AppCentric
}) error {
	crudcl, err := ctx.RPCClient("dev.miren.runtime/app")
	if err != nil {
		return err
	}

	crud := app_v1alpha.NewCrudClient(crudcl)

	result, err := crud.Restart(ctx, opts.App, opts.Service)
	if err != nil {
		return err
	}

	if opts.Service != "" {
		ctx.Printf("Restarted service %s of app %s", opts.Service, opts.App)
	} else {
		ctx.Printf("Restarted app %s", opts.App)
	}

	ctx.Printf(" (stopped %d sandboxes across %d pools)\n",
		result.StoppedSandboxes(), result.RestartedPools())

	return nil
}
