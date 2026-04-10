package commands

import (
	"fmt"
	"net"
	"net/netip"
)

// discoverOutboundIP finds the local IP that would be used to reach the given
// remote address. This gives us the machine's IP on the right interface without
// actually connecting.
func discoverOutboundIP(remoteAddr string) (netip.Addr, error) {
	conn, err := net.Dial("udp4", remoteAddr)
	if err != nil {
		return netip.Addr{}, err
	}
	defer conn.Close()
	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return netip.Addr{}, fmt.Errorf("unexpected local address type")
	}
	ip4 := addr.IP.To4()
	if ip4 == nil {
		return netip.Addr{}, fmt.Errorf("discovered non-IPv4 address: %s", addr.IP)
	}
	return netip.AddrFrom4([4]byte(ip4)), nil
}
