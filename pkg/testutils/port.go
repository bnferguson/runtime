package testutils

import (
	"net"
	"testing"
)

// GetFreePort returns a port that is free for both TCP and UDP.
// This is important for QUIC-based servers (like the RPC server) which bind both protocols.
// The function binds both protocols to verify availability, then releases them.
// Fails the test if a suitable port cannot be obtained after several attempts.
func GetFreePort(t testing.TB) int {
	t.Helper()

	// Try a few times to find a port free for both TCP and UDP
	for range 10 {
		// First, get an available TCP port
		tcpListener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			continue
		}
		port := tcpListener.Addr().(*net.TCPAddr).Port

		// Try to bind UDP to the same port
		udpAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: port}
		udpConn, err := net.ListenUDP("udp", udpAddr)
		if err != nil {
			tcpListener.Close()
			continue
		}

		// Both succeeded - close and return the port
		tcpListener.Close()
		udpConn.Close()
		return port
	}

	t.Fatalf("failed to find a port free for both TCP and UDP after 10 attempts")
	return 0
}
