package server

import (
	"encoding/json"
	"os"
)

// ServiceConfig holds credentials for connecting to the entity server.
// This is written by the main server process for lsvd-server to use.
type ServiceConfig struct {
	// ClientCert is the PEM-encoded client certificate
	ClientCert []byte `json:"client_cert,omitempty"`
	// ClientKey is the PEM-encoded client private key
	ClientKey []byte `json:"client_key,omitempty"`
	// CloudURL is the miren.cloud API URL for disk replication
	CloudURL string `json:"cloud_url,omitempty"`
	// PrivateKey is the PEM-encoded private key for cloud authentication
	PrivateKey string `json:"private_key,omitempty"`
}

// LoadServiceConfig loads a service config from the given path.
func LoadServiceConfig(path string) (*ServiceConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg ServiceConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// SaveServiceConfig saves a service config to the given path.
func SaveServiceConfig(path string, cfg *ServiceConfig) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}
