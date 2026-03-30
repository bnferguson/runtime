package ipdiscovery

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"
)

// Discovery holds information about discovered IP addresses
type Discovery struct {
	Addresses []Address `json:"addresses"`
}

// Address represents an IP address associated with a network interface
type Address struct {
	Interface string `json:"interface"`
	IP        string `json:"ip"`
	Network   string `json:"network"`
	IsIPv6    bool   `json:"is_ipv6"`
}

// Discover gathers all local interface addresses.
func Discover(ctx context.Context, log *slog.Logger) (*Discovery, error) {
	discovery := &Discovery{
		Addresses: []Address{},
	}

	// Get local interface addresses
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to get interfaces: %w", err)
	}

	for _, iface := range interfaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			var network string

			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
				network = v.String()
			case *net.IPAddr:
				ip = v.IP
				network = v.String()
			default:
				continue
			}

			if ip.IsLoopback() {
				continue
			}

			address := Address{
				Interface: iface.Name,
				IP:        ip.String(),
				Network:   network,
				IsIPv6:    ip.To4() == nil,
			}

			discovery.Addresses = append(discovery.Addresses, address)
		}
	}

	return discovery, nil
}

// DiscoverWithTimeout is a convenience function that adds a timeout to Discover
func DiscoverWithTimeout(timeout time.Duration, log *slog.Logger) (*Discovery, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return Discover(ctx, log)
}
