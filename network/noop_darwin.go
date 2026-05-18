//go:build !linux

package network

import (
	"fmt"
	"log/slog"
	"net/netip"

	"github.com/vishvananda/netlink"
)

func ConfigureNetNS(log *slog.Logger, pid int, ec *EndpointConfig) error {
	return fmt.Errorf("network namespace not supported on this platform")
}

func SetupBridge(n *BridgeConfig) (*netlink.Bridge, error) {
	return nil, fmt.Errorf("network bridge not supported on this platform")
}

func TeardownBridge(name string) error {
	return fmt.Errorf("network bridge not supported on this platform")
}

func ConfigureGW(br netlink.Link, ec *EndpointConfig) error {
	return fmt.Errorf("network gateway configuration not supported on this platform")
}

func MasqueradeEndpoint(ec *EndpointConfig) error {
	return fmt.Errorf("network masquerade not supported on this platform")
}

func ReconcileBridgeAddresses(log *slog.Logger, br netlink.Link, desired []netip.Prefix) error {
	return fmt.Errorf("bridge reconciliation not supported on this platform")
}
