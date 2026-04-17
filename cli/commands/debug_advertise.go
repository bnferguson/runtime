package commands

import (
	"fmt"
	"net"
	"sort"
	"time"

	"miren.dev/runtime/components/coordinate"
	"miren.dev/runtime/pkg/cloudauth"
	"miren.dev/runtime/pkg/ipdiscovery"
)

// DebugAdvertise runs the same advertise-address computation the server uses
// (coordinate.ComputeAdvertise) and prints a per-candidate explanation plus
// the final advertised list, so we can debug cases where the server
// advertises addresses that aren't actually reachable from clients.
func DebugAdvertise(ctx *Context, opts struct {
	CloudURL      string   `long:"cloud-url" description:"Cloud URL to use for netcheck (default: https://api.miren.cloud)"`
	SkipNetcheck  bool     `long:"skip-netcheck" description:"Skip the netcheck call and only report interface scan"`
	AdditionalIPs []string `long:"additional-ip" description:"Simulate a server-configured AdditionalIP (repeatable)"`
	ListenAddr    string   `long:"listen" description:"Simulate the server's listen address (default: 0.0.0.0:8443)"`
}) error {
	cloudURL := opts.CloudURL
	if cloudURL == "" {
		cloudURL = coordinate.DefaultCloudURL
	}
	listenAddr := opts.ListenAddr
	if listenAddr == "" {
		listenAddr = "0.0.0.0:8443"
	}

	ctx.Info("debug advertise — reproducing server advertisement logic")
	ctx.Info("  cloud URL:    %s", cloudURL)
	ctx.Info("  listen:       %s", listenAddr)
	ctx.Info("  netcheck:     %s", boolWord(!opts.SkipNetcheck, "enabled", "skipped"))
	ctx.Info("")

	discoveryOpts := ipdiscovery.Options{}
	if !opts.SkipNetcheck {
		discoveryOpts.NetcheckURL = cloudURL
	}

	ctx.Info("Step 1: interface scan")
	discovery, err := ipdiscovery.DiscoverWithTimeout(15*time.Second, ctx.Log, discoveryOpts)
	if err != nil {
		ctx.Warn("ipdiscovery.Discover failed: %v", err)
		return err
	}

	ipSet := coordinate.NewIPSet()
	for _, a := range discovery.Addresses {
		ip := net.ParseIP(a.IP)
		if ip == nil {
			continue
		}
		// server.go drops link-local addresses before handing the list
		// to the coordinator — mirror that here.
		if ip.IsLinkLocalUnicast() {
			ctx.Info("  %-15s %-40s [skipped: link-local]", a.Interface, a.IP)
			continue
		}
		ctx.Info("  %-15s %-40s (discovered)", a.Interface, a.IP)
		ipSet.AddDiscovered(ip)
	}

	for _, s := range opts.AdditionalIPs {
		ip := net.ParseIP(s)
		if ip == nil {
			ctx.Warn("--additional-ip %q is not a valid IP, skipping", s)
			continue
		}
		ctx.Info("  %-15s %-40s (explicit)", "user", ip.String())
		ipSet.AddExplicit(ip)
	}
	ctx.Info("")

	ctx.Info("Step 2: dual-stack netcheck")
	var netcheckResult *cloudauth.NetcheckDualStackResult
	if opts.SkipNetcheck {
		ctx.Info("  skipped (--skip-netcheck)")
	} else {
		ports := []cloudauth.NetcheckPort{
			{Port: 8443, Protocol: "https"},
			{Port: 8443, Protocol: "http3"},
		}
		netcheckResult, err = cloudauth.NetcheckDualStack(ctx, cloudURL, ports)
		if err != nil {
			ctx.Warn("netcheck failed: %v", err)
			netcheckResult = nil
		} else {
			printNetcheckResponse(ctx, "IPv4", netcheckResult.IPv4)
			printNetcheckResponse(ctx, "IPv6", netcheckResult.IPv6)
		}
	}
	ctx.Info("")

	candidates, final := coordinate.ComputeAdvertise(coordinate.AdvertiseInput{
		ListenAddr: listenAddr,
		IPs:        ipSet.All(),
		Netcheck:   netcheckResult,
	})

	ctx.Info("Step 3: per-candidate classification and inclusion decision")
	ctx.Info("")
	ctx.Info("  %-12s %-40s %-16s %-10s %s", "SOURCE", "IP:PORT", "CLASS", "DECISION", "REASON")
	ctx.Info("  %s", "-------------------------------------------------------------------------------------------------------------")
	for _, c := range candidates {
		decision := "SKIPPED"
		if c.Included {
			decision = "ADVERTISED"
		}
		ctx.Info("  %-12s %-40s %-16s %-10s %s",
			c.Source, c.HostPort, c.Classification, decision, c.Reason)
	}

	ctx.Info("")
	ctx.Info("Final advertised list (%d entries):", len(final))
	for _, a := range final {
		ctx.Info("  %s", a)
	}

	return nil
}

func printNetcheckResponse(ctx *Context, family string, resp *cloudauth.NetcheckResponse) {
	if resp == nil {
		ctx.Info("  %s: no response", family)
		return
	}
	var reachable []string
	var unreachable []string
	for _, r := range resp.Results {
		entry := fmt.Sprintf("%s/%d", r.Protocol, r.Port)
		if r.Reachable {
			reachable = append(reachable, entry)
		} else {
			unreachable = append(unreachable, entry)
		}
	}
	sort.Strings(reachable)
	sort.Strings(unreachable)
	ctx.Info("  %s source=%s reachable=%v unreachable=%v duration=%dms",
		family, resp.SourceAddress, reachable, unreachable, resp.DurationMs)
}

func boolWord(b bool, yes, no string) string {
	if b {
		return yes
	}
	return no
}
