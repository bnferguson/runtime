package commands

import (
	"fmt"
	"net"

	"miren.dev/runtime/api/runner/runner_v1alpha"
	"miren.dev/runtime/pkg/rpc"
	"miren.dev/runtime/pkg/runnerconfig"
)

// RunnerReissue rotates the runner's client certificate in place. It authenticates
// to the coordinator with the runner's existing certificate and asks for a fresh
// one (new key, same identity and CommonName), then writes only the credential
// material back to the config. Use it to rotate credentials on demand, for hygiene
// or in response to a suspected key compromise, without a full re-join: the runner
// keeps its identity and its sandbox schedules stay bound to the same node.
//
// The on-disk certificate must still be valid, because it is the authentication.
// A runner whose certificate has expired or is otherwise unusable can't be rotated
// this way and should be re-provisioned with `runner remove` followed by
// `runner join --runner-id <id>`, which re-establishes the same identity from a
// fresh join token.
func RunnerReissue(ctx *Context, opts struct {
	Coordinator string `short:"c" long:"coordinator" description:"Override coordinator address (defaults to the runner config)"`
	ListenAddr  string `short:"l" long:"listen" description:"Address this runner listens on (covered by the new cert; auto-discovered if unset)"`
	ConfigPath  string `long:"config" description:"Path to the runner config" default:"/var/lib/miren/runner/config.yaml"`
}) error {
	if !runnerconfig.Exists(opts.ConfigPath) {
		return fmt.Errorf("no runner config at %s; run 'miren runner join' first", opts.ConfigPath)
	}

	cfg, err := runnerconfig.Load(opts.ConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load runner config: %w", err)
	}
	if cfg.ClientCert == "" || cfg.ClientKey == "" || cfg.CACert == "" {
		return fmt.Errorf("runner config at %s has no complete certificate material to rotate; re-provision with 'runner remove' + 'runner join'", opts.ConfigPath)
	}

	coordinator := cfg.CoordinatorAddress
	if opts.Coordinator != "" {
		coordinator = opts.Coordinator
	}
	if coordinator == "" {
		return fmt.Errorf("no coordinator address in config; pass --coordinator")
	}

	listenAddr := opts.ListenAddr
	if listenAddr == "" {
		ip, err := discoverOutboundIP(coordinator)
		if err != nil {
			return fmt.Errorf("could not discover listen address (use --listen to set manually): %w", err)
		}
		listenAddr = net.JoinHostPort(ip.String(), "8444")
	}

	ctx.Printf("Rotating certificate for runner '%s' (%s) via %s\n", cfg.Name, cfg.RunnerID, coordinator)

	// Authenticate with the runner's existing certificate. This is what proves we
	// are this runner, so the coordinator will only ever re-issue for the identity
	// the presented certificate already carries.
	cs, err := rpc.NewState(ctx,
		rpc.WithLogger(ctx.Log),
		rpc.WithBindAddr("[::]:0"),
		rpc.WithCertPEMs([]byte(cfg.ClientCert), []byte(cfg.ClientKey)),
		rpc.WithCertificateVerification([]byte(cfg.CACert)),
	)
	if err != nil {
		return fmt.Errorf("failed to create RPC state: %w", err)
	}
	defer cs.Close()

	client, err := cs.Connect(coordinator, rpc.ServiceRunner)
	if err != nil {
		return fmt.Errorf("failed to connect to coordinator: %w", err)
	}
	defer client.Close()

	rc := runner_v1alpha.NewRunnerRegistrationClient(client)
	res, err := rc.RefreshCertificate(ctx, listenAddr)
	if err != nil {
		return fmt.Errorf("reissue request failed: %w", err)
	}
	if res.Error() != "" {
		return fmt.Errorf("reissue rejected by coordinator: %s", res.Error())
	}
	if len(res.CertPem()) == 0 || len(res.KeyPem()) == 0 || len(res.CaPem()) == 0 {
		return fmt.Errorf("coordinator returned incomplete certificate material; leaving config unchanged")
	}

	// Update only the credential material; every other field (labels, disk_mode,
	// endpoints, name) is preserved as-is.
	cfg.ClientCert = string(res.CertPem())
	cfg.ClientKey = string(res.KeyPem())
	cfg.CACert = string(res.CaPem())

	if err := cfg.Save(opts.ConfigPath); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	ctx.Printf("Certificate rotated; config updated at %s\n", opts.ConfigPath)
	ctx.Printf("\nRestart the runner to load the new certificate:\n")
	ctx.Printf("  systemctl restart miren-runner\n")

	return nil
}
