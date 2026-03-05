package sandbox

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestPortListening(t *testing.T) {
	// Sample /proc/net/tcp content with a listener on port 6667 (0x1A0B)
	// and an established connection on port 3000 (0x0BB8)
	content := `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000:1A0B 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 12345 1 0000000000000000 100 0 0 10 0
   1: 0100007F:0BB8 0100007F:C000 01 00000000:00000000 00:00000000 00000000  1000        0 67890 1 0000000000000000 20 0 0 10 -1
`

	dir := t.TempDir()
	path := filepath.Join(dir, "tcp")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		port int
		want bool
	}{
		{6667, true},  // listening (state 0A)
		{3000, false}, // established (state 01), not listening
		{8080, false}, // not present at all
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("port_%d", tt.port), func(t *testing.T) {
			got := portListening(path, tt.port)
			if got != tt.want {
				t.Errorf("portListening(path, %d) = %v, want %v", tt.port, got, tt.want)
			}
		})
	}
}

func TestPortListeningIPv6(t *testing.T) {
	// Sample /proc/net/tcp6 content with a listener on port 3000
	content := `  sl  local_address                         remote_address                        st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000000000000000000000000000:0BB8 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 12345 1 0000000000000000 100 0 0 10 0
`

	dir := t.TempDir()
	path := filepath.Join(dir, "tcp6")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	if !portListening(path, 3000) {
		t.Error("expected port 3000 to be listening in tcp6")
	}

	if portListening(path, 8080) {
		t.Error("expected port 8080 to not be listening in tcp6")
	}
}

func TestPortListeningMissingFile(t *testing.T) {
	if portListening("/nonexistent/path/tcp", 80) {
		t.Error("expected false for missing file")
	}
}

func TestCheckPortWithCurrentProcess(t *testing.T) {
	// Listen on a random port and verify checkPort finds it via our own PID
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	pid := os.Getpid()

	if !checkPort(pid, port) {
		t.Errorf("checkPort(%d, %d) = false, want true for listening port", pid, port)
	}

	if checkPort(pid, port+1) {
		t.Errorf("checkPort(%d, %d) = true, want false for non-listening port", pid, port+1)
	}
}
