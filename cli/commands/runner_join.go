package commands

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"

	"miren.dev/runtime/api/runner/runner_v1alpha"
	"miren.dev/runtime/pkg/joincode"
	"miren.dev/runtime/pkg/rpc"
	"miren.dev/runtime/pkg/runnerconfig"
	"miren.dev/runtime/version"
)

func RunnerJoin(ctx *Context, opts struct {
	Coordinator string   `short:"c" long:"coordinator" description:"Coordinator address (host:port)"`
	ListenAddr  string   `short:"l" long:"listen" description:"Address this runner will listen on"`
	Labels      []string `long:"labels" description:"Additional labels for the runner (key=value)"`
	ConfigPath  string   `long:"config" description:"Path to save runner config" default:"/var/lib/miren/runner/config.yaml"`
	RunnerID    string   `long:"runner-id" description:"Specific runner ID to use (for reconnecting)"`

	Args struct {
		Coordinator string `positional-arg-name:"coordinator" description:"Coordinator address (host:port)"`
	} `positional-args:"yes"`
}) error {
	coordinator := opts.Coordinator
	if coordinator == "" {
		coordinator = opts.Args.Coordinator
	}
	if coordinator == "" {
		return fmt.Errorf("coordinator address is required")
	}

	if _, _, err := net.SplitHostPort(coordinator); err != nil {
		coordinator = net.JoinHostPort(coordinator, "8443")
	}

	if runnerconfig.Exists(opts.ConfigPath) {
		ctx.Printf("Runner config already exists at %s\n", opts.ConfigPath)
		ctx.Printf("Remove it first if you want to re-register this runner.\n")
		return nil
	}

	ctx.Printf("Joining coordinator at %s\n", coordinator)
	ctx.Printf("\nEnter join code: ")

	reader := bufio.NewReader(os.Stdin)
	code, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read code: %w", err)
	}
	code = strings.TrimSpace(code)

	if !joincode.Validate(code) {
		return fmt.Errorf("invalid join code format")
	}

	cs, err := rpc.NewState(ctx, rpc.WithSkipVerify, rpc.WithLogger(ctx.Log))
	if err != nil {
		return fmt.Errorf("failed to create RPC state: %w", err)
	}

	client, err := cs.Connect(coordinator, "dev.miren.runtime/runner")
	if err != nil {
		return fmt.Errorf("failed to connect to coordinator: %w", err)
	}
	defer client.Close()

	rc := runner_v1alpha.NewRunnerRegistrationClient(client)

	versionInfo := version.GetInfo()
	res, err := rc.Join(ctx, code, opts.RunnerID, opts.ListenAddr, versionInfo.Version, opts.Labels)
	if err != nil {
		return fmt.Errorf("join request failed: %w", err)
	}

	if res.Error() != "" {
		return fmt.Errorf("join failed: %s", res.Error())
	}

	runnerID := res.RunnerId()
	ctx.Printf("\n✓ Joined as runner '%s'\n", runnerID)

	labels := make(map[string]string)
	for _, l := range opts.Labels {
		parts := strings.SplitN(l, "=", 2)
		if len(parts) == 2 {
			labels[parts[0]] = parts[1]
		}
	}

	cfg := &runnerconfig.Config{
		RunnerID:           runnerID,
		CoordinatorAddress: res.CoordinatorAddr(),
		CACert:             string(res.CaPem()),
		ClientCert:         string(res.CertPem()),
		ClientKey:          string(res.KeyPem()),
		Labels:             labels,
	}

	if err := cfg.Save(opts.ConfigPath); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	ctx.Printf("Config saved to %s\n", opts.ConfigPath)
	ctx.Printf("\nTo start this runner, run:\n")
	ctx.Printf("  miren server --runner\n")

	return nil
}
