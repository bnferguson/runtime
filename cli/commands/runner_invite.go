package commands

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"miren.dev/runtime/api/runner/runner_v1alpha"
	"miren.dev/runtime/pkg/rpc"
	"miren.dev/runtime/pkg/rpc/standard"
)

func RunnerTokenCreate(ctx *Context, opts struct {
	ConfigCentric

	Labels   []string `short:"l" long:"labels" description:"Labels to apply to the runner (key=value format)"`
	Expires  int      `short:"e" long:"expires" default:"1" description:"Hours until the invite expires"`
	Reusable bool     `short:"r" long:"reusable" description:"Create a reusable invite (not consumed on use)"`
	Name     string   `short:"n" long:"name" description:"Human-readable name for this invite"`
	TTL      string   `long:"ttl" description:"Time-to-live (e.g. 24h, 7d, 2w). Overrides --expires"`
	Addr     string   `short:"a" long:"addr" description:"Override coordinator address baked into the token"`
}) error {
	client, err := ctx.RPCClient(rpc.ServiceRunner)
	if err != nil {
		return err
	}
	defer client.Close()

	rc := runner_v1alpha.NewRunnerRegistrationClient(client)

	var ttlSeconds int64
	if opts.TTL != "" {
		d, err := parseTTL(opts.TTL)
		if err != nil {
			return fmt.Errorf("invalid --ttl: %w", err)
		}
		ttlSeconds = int64(d.Seconds())
	}

	res, err := rc.CreateInvite(ctx, opts.Labels, int32(opts.Expires), opts.Name, opts.Reusable, ttlSeconds, opts.Addr)
	if err != nil {
		return err
	}

	token := res.Code()
	expiresAt := standard.FromTimestamp(res.ExpiresAt())

	ctx.Printf("Token: %s\n", token)
	if expiresAt.IsZero() {
		ctx.Printf("Expires: never\n")
	} else {
		ctx.Printf("Expires: %s (%s)\n", expiresAt.Format(time.RFC3339), formatDuration(time.Until(expiresAt)))
	}

	if opts.Reusable {
		ctx.Printf("Type: reusable\n")
	}
	if opts.Name != "" {
		ctx.Printf("Name: %s\n", opts.Name)
	}
	if len(opts.Labels) > 0 {
		ctx.Printf("Labels: %v\n", opts.Labels)
	}

	ctx.Printf("\nTo join a runner, run on the runner machine:\n")
	ctx.Printf("  miren runner join %s\n", token)

	return nil
}

// parseTTL parses a duration string that supports "Nd" (days) and "Nw" (weeks)
// in addition to Go's standard time.ParseDuration formats.
func parseTTL(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty TTL")
	}

	suffix := s[len(s)-1]
	prefix := s[:len(s)-1]

	switch suffix {
	case 'd':
		n, err := strconv.Atoi(prefix)
		if err != nil {
			return 0, fmt.Errorf("invalid days value: %w", err)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	case 'w':
		n, err := strconv.Atoi(prefix)
		if err != nil {
			return 0, fmt.Errorf("invalid weeks value: %w", err)
		}
		return time.Duration(n) * 7 * 24 * time.Hour, nil
	default:
		return time.ParseDuration(s)
	}
}
