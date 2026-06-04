//go:build linux

package sandbox

import (
	"fmt"
	"os/exec"
	"strconv"

	compute "miren.dev/runtime/api/compute/compute_v1alpha"
	"miren.dev/runtime/network"
	"miren.dev/runtime/pkg/netutil"
)

func (c *SandboxController) configureFirewall(sb *compute.Sandbox, ep *network.EndpointConfig) error {
	for _, co := range sb.Spec.Container {
		c.Log.Info("configuring firewall", "sandbox", sb.ID.String(), "ports", len(co.Port))

		for _, p := range co.Port {
			if err := c.configurePort(p, ep); err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *SandboxController) configurePort(p compute.SandboxSpecContainerPort, ep *network.EndpointConfig) error {
	// NodePort DNAT is handled by the service controller via nftables.
	// No iptables rules are needed here.
	return nil
}

func (c *SandboxController) UnconfigureFirewall(sb *compute.Sandbox) {
	if len(sb.Network) == 0 {
		c.Log.Warn("no network info on sandbox, skipping firewall cleanup", "sandbox", sb.ID.String())
		return
	}

	ip, err := netutil.ParseNetworkAddress(sb.Network[0].Address)
	if err != nil {
		c.Log.Warn("failed to parse sandbox network address for firewall cleanup", "sandbox", sb.ID.String(), "address", sb.Network[0].Address, "err", err)
		return
	}

	for _, co := range sb.Spec.Container {
		for _, p := range co.Port {
			c.unconfigurePort(p, ip)
		}
	}
}

func (c *SandboxController) unconfigurePort(p compute.SandboxSpecContainerPort, ip string) {
	if p.NodePort == 0 {
		return
	}

	c.Log.Info("removing firewall rules", "nodePort", p.NodePort, "targetPort", p.Port, "ip", ip)

	// Loop each delete to remove all duplicate rules accumulated by prior
	// versions that inserted without deduplication.
	for {
		if err := exec.Command("iptables",
			"-t", "nat",
			"-D", "PREROUTING",
			"!", "-i", c.Bridge,
			"-p", "tcp",
			"-m", "tcp",
			"--dport", strconv.Itoa(int(p.NodePort)),
			"-j", "DNAT", "--to-destination", fmt.Sprintf("%s:%d", ip, p.Port),
		).Run(); err != nil {
			break
		}
	}

	for {
		if err := exec.Command("iptables",
			"-t", "nat",
			"-D", "OUTPUT",
			"-p", "tcp",
			"-m", "tcp",
			"-d", "127.0.0.1",
			"--dport", strconv.Itoa(int(p.NodePort)),
			"-j", "DNAT", "--to-destination", fmt.Sprintf("%s:%d", ip, p.Port),
		).Run(); err != nil {
			break
		}
	}

	for {
		if err := exec.Command("iptables",
			"-t", "nat",
			"-D", "POSTROUTING",
			"-s", "127.0.0.1",
			"-p", "tcp",
			"-d", ip,
			"--dport", strconv.Itoa(int(p.Port)),
			"-j", "MASQUERADE",
		).Run(); err != nil {
			break
		}
	}
}
