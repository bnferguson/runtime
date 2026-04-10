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
	"miren.dev/runtime/pkg/ui"
	"miren.dev/runtime/version"
)

func RunnerJoin(ctx *Context, opts struct {
	Coordinator string   `short:"c" long:"coordinator" description:"Coordinator address (host:port)"`
	Code        string   `long:"code" description:"Join code (or pass via stdin)"`
	ListenAddr  string   `short:"l" long:"listen" description:"Address this runner will listen on"`
	Name        string   `long:"name" description:"Human-readable name for this runner (defaults to hostname)"`
	Labels      []string `long:"labels" description:"Additional labels for the runner (key=value)"`
	ConfigPath  string   `long:"config" description:"Path to save runner config" default:"/var/lib/miren/runner/config.yaml"`
	RunnerID    string   `long:"runner-id" description:"Specific runner ID to use (for reconnecting)"`

	CoordinatorAddr string `position:"0" usage:"Coordinator address (host:port)"`
	JoinCode        string `position:"1" usage:"Join code from 'miren runner invite'"`
}) error {
	coordinator := opts.Coordinator
	if coordinator == "" {
		coordinator = opts.CoordinatorAddr
	}
	if coordinator == "" {
		return fmt.Errorf("coordinator address is required")
	}

	if _, _, err := net.SplitHostPort(coordinator); err != nil {
		coordinator = net.JoinHostPort(coordinator, "8443")
	}

	if runnerconfig.Exists(opts.ConfigPath) {
		return fmt.Errorf("runner config already exists at %s; remove it first to re-register", opts.ConfigPath)
	}

	ctx.Printf("Joining coordinator at %s\n", coordinator)

	// Resolve join code: --code flag > positional arg > stdin pipe > TTY prompt
	code := opts.Code
	if code == "" {
		code = opts.JoinCode
	}
	if code == "" {
		if stat, _ := os.Stdin.Stat(); stat.Mode()&os.ModeCharDevice == 0 {
			// stdin is a pipe, read the code from it
			scanner := bufio.NewScanner(os.Stdin)
			if scanner.Scan() {
				code = strings.TrimSpace(scanner.Text())
			}
		}
	}
	if code == "" {
		var err error
		code, err = ui.PromptForInput(
			ui.WithLabel("Enter join code"),
			ui.WithPlaceholder("word-word-word-abc123"),
		)
		if err != nil {
			return fmt.Errorf("failed to read code: %w", err)
		}
	}

	if !joincode.Validate(code) {
		return fmt.Errorf("invalid join code format")
	}

	name := opts.Name
	if name == "" {
		name, _ = os.Hostname()
	}

	cs, err := rpc.NewState(ctx, rpc.WithSkipVerify, rpc.WithLogger(ctx.Log), rpc.WithBindAddr("[::]:0"))
	if err != nil {
		return fmt.Errorf("failed to create RPC state: %w", err)
	}

	client, err := cs.Connect(coordinator, rpc.ServiceRunner)
	if err != nil {
		return fmt.Errorf("failed to connect to coordinator: %w", err)
	}
	defer client.Close()

	rc := runner_v1alpha.NewRunnerRegistrationClient(client)

	listenAddr := opts.ListenAddr
	if listenAddr == "" {
		ip, err := discoverOutboundIP(coordinator)
		if err != nil {
			return fmt.Errorf("could not discover outbound IP for listen address (use --listen to set manually): %w", err)
		}
		listenAddr = net.JoinHostPort(ip.String(), "8444")
		ctx.Log.Info("discovered listen address", "addr", listenAddr)
	}

	versionInfo := version.GetInfo()
	res, err := rc.Join(ctx, code, opts.RunnerID, listenAddr, versionInfo.Version, opts.Labels, name)
	if err != nil {
		return fmt.Errorf("join request failed: %w", err)
	}

	if res.Error() != "" {
		return fmt.Errorf("join failed: %s", res.Error())
	}

	runnerID := res.RunnerId()
	ctx.Printf("\n✓ Joined as runner '%s' (%s)\n", name, runnerID)

	labels := make(map[string]string)
	for _, l := range opts.Labels {
		parts := strings.SplitN(l, "=", 2)
		if len(parts) == 2 {
			labels[parts[0]] = parts[1]
		}
	}

	// Use the address the runner actually connected to rather than the
	// coordinator's bind address (which may be 0.0.0.0 or localhost).
	coordinatorHost, _, _ := net.SplitHostPort(coordinator)

	// Rewrite loopback/unspecified hosts in etcd endpoints with the
	// coordinator host. For embedded etcd (the common case with distributed
	// runners), etcd is colocated with the coordinator.
	etcdEndpoints := res.EtcdEndpoints()
	for i, ep := range etcdEndpoints {
		etcdEndpoints[i] = rewriteLoopbackEndpoint(ep, coordinatorHost)
	}

	// Rewrite loopback hosts in observability endpoints the same way we do for etcd.
	vmAddress := res.VictoriametricsAddress()
	if vmAddress != "" {
		vmAddress = rewriteLoopbackEndpoint(vmAddress, coordinatorHost)
	}
	vlAddress := res.VictorialogsAddress()
	if vlAddress != "" {
		vlAddress = rewriteLoopbackEndpoint(vlAddress, coordinatorHost)
	}

	cfg := &runnerconfig.Config{
		RunnerID:               runnerID,
		Name:                   name,
		CoordinatorAddress:     coordinator,
		CACert:                 string(res.CaPem()),
		ClientCert:             string(res.CertPem()),
		ClientKey:              string(res.KeyPem()),
		Labels:                 labels,
		EtcdEndpoints:          etcdEndpoints,
		EtcdPrefix:             res.EtcdPrefix(),
		NetworkBackend:         res.NetworkBackend(),
		VictoriametricsAddress: vmAddress,
		VictorialogsAddress:    vlAddress,
	}

	if err := cfg.Save(opts.ConfigPath); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	ctx.Printf("Config saved to %s\n", opts.ConfigPath)
	ctx.Printf("\nTo start this runner, run:\n")
	ctx.Printf("  miren runner start\n")

	return nil
}

// rewriteLoopbackEndpoint replaces loopback or unspecified hosts in an
// endpoint URL with the given replacement host. Endpoints may be bare
// host:port or have a scheme (e.g. "https://localhost:12379").
func rewriteLoopbackEndpoint(endpoint, replaceHost string) string {
	host := endpoint
	scheme := ""

	// Strip scheme if present
	if idx := strings.Index(endpoint, "://"); idx != -1 {
		scheme = endpoint[:idx+3]
		host = endpoint[idx+3:]
	}

	h, port, err := net.SplitHostPort(host)
	if err != nil {
		return endpoint
	}

	if isLoopbackOrUnspecified(h) {
		return scheme + net.JoinHostPort(replaceHost, port)
	}

	return endpoint
}

func isLoopbackOrUnspecified(host string) bool {
	switch host {
	case "localhost", "127.0.0.1", "::1", "0.0.0.0", "::":
		return true
	}
	return false
}
