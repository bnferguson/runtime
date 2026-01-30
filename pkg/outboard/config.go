package outboard

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config is the shared configuration file format between the connector (parent)
// and the outboard process (child). The connector writes it before starting the
// process; the process reads it on startup and writes back its RPC address, PID,
// and ready flag.
type Config struct {
	Token      string `json:"token"`
	FIFOStdout string `json:"fifo_stdout"`
	FIFOStderr string `json:"fifo_stderr"`
	PID        int    `json:"pid,omitempty"`
	RPCAddr    string `json:"rpc_addr,omitempty"`
	Ready      bool   `json:"ready,omitempty"`
}

// ReadConfig reads an outboard config from the given path.
func ReadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading outboard config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing outboard config: %w", err)
	}

	return &cfg, nil
}

// WriteConfig writes an outboard config to the given path atomically.
// It uses a temp file and rename to ensure readers never see partial data.
func WriteConfig(path string, cfg *Config) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling outboard config: %w", err)
	}

	// Write to temp file in the same directory for atomic rename
	dir := filepath.Dir(path)
	tempFile, err := os.CreateTemp(dir, "outboard-config-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp config file: %w", err)
	}
	tempPath := tempFile.Name()

	if _, err := tempFile.Write(data); err != nil {
		tempFile.Close()
		os.Remove(tempPath)
		return fmt.Errorf("writing temp config file: %w", err)
	}

	if err := tempFile.Sync(); err != nil {
		tempFile.Close()
		os.Remove(tempPath)
		return fmt.Errorf("syncing temp config file: %w", err)
	}

	if err := tempFile.Close(); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("closing temp config file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempPath, path); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("renaming temp config file: %w", err)
	}

	return nil
}
