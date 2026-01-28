package outboard

import (
	"encoding/json"
	"fmt"
	"os"
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
func WriteConfig(path string, cfg *Config) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling outboard config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("writing outboard config: %w", err)
	}

	return nil
}
