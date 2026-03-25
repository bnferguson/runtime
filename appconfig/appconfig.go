package appconfig

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	toml "github.com/pelletier/go-toml/v2"
	tomlast "github.com/pelletier/go-toml/v2/unstable"
)

var aliasWordRegexp = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

type AppEnvVar struct {
	Key         string `json:"key" toml:"key"`
	Value       string `json:"value" toml:"value"`
	Required    bool   `json:"required,omitempty" toml:"required"`
	Sensitive   bool   `json:"sensitive,omitempty" toml:"sensitive"`
	Description string `json:"description,omitempty" toml:"description"`
}

type BuildConfig struct {
	Dockerfile string   `toml:"dockerfile"`
	OnBuild    []string `toml:"onbuild"`
	Version    string   `toml:"version"`

	AlpineImage string `toml:"alpine_image"`
}

// ServiceConcurrencyConfig represents per-service concurrency configuration
type ServiceConcurrencyConfig struct {
	Mode                string `toml:"mode"` // "auto" or "fixed"
	RequestsPerInstance int    `toml:"requests_per_instance"`
	ScaleDownDelay      string `toml:"scale_down_delay"` // e.g. "2m", "15m"
	NumInstances        int    `toml:"num_instances"`
	ShutdownTimeout     string `toml:"shutdown_timeout"` // e.g. "10s", "30s" - time to wait for graceful shutdown
}

// DiskConfig represents a disk attachment for a service
type DiskConfig struct {
	Name         string `toml:"name"`
	MountPath    string `toml:"mount_path"`
	ReadOnly     bool   `toml:"read_only"`
	SizeGB       int    `toml:"size_gb"`
	Filesystem   string `toml:"filesystem"`
	LeaseTimeout string `toml:"lease_timeout"`
}

// PortConfig represents a network port for a service
type PortConfig struct {
	Port     int    `toml:"port"`
	Name     string `toml:"name"`
	Type     string `toml:"type"`
	NodePort int    `toml:"node_port"`
}

// ServiceConfig represents configuration for a specific service
type ServiceConfig struct {
	Command     string                    `toml:"command"`
	Port        int                       `toml:"port"`
	PortName    string                    `toml:"port_name"`
	PortType    string                    `toml:"port_type"`
	Ports       []PortConfig              `toml:"ports"`
	Image       string                    `toml:"image"`
	EnvVars     []AppEnvVar               `toml:"env"`
	Concurrency *ServiceConcurrencyConfig `toml:"concurrency"`
	Disks       []DiskConfig              `toml:"disks"`
}

// AddonConfig represents configuration for an addon in app.toml.
type AddonConfig struct {
	Variant string `toml:"variant"`
}

type AppConfig struct {
	Name        string                    `toml:"name"`
	PostImport  string                    `toml:"post_import"`
	EnvVars     []AppEnvVar               `toml:"env"`
	Concurrency *int                      `toml:"concurrency"`
	Services    map[string]*ServiceConfig `toml:"services"`
	Build       *BuildConfig              `toml:"build"`
	Include     []string                  `toml:"include"`
	Addons      map[string]*AddonConfig   `toml:"addons"`
	Aliases     map[string]string         `toml:"aliases"`
}

const AppConfigPath = ".miren/app.toml"

func LoadAppConfig() (*AppConfig, error) {
	ac, _, err := LoadAppConfigWithPath()
	return ac, err
}

// LoadAppConfigWithPath loads the app config and returns the file path it was loaded from.
// Returns (nil, "", nil) if no config file is found.
func LoadAppConfigWithPath() (*AppConfig, string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, "", err
	}

	for dir != "/" {
		path := filepath.Join(dir, AppConfigPath)
		fi, err := os.Open(path)
		if err == nil {
			defer fi.Close()

			var ac AppConfig
			dec := toml.NewDecoder(fi)
			dec.DisallowUnknownFields()
			err = dec.Decode(&ac)
			if err != nil {
				return nil, "", err
			}

			// Validate the configuration
			if err := ac.Validate(); err != nil {
				return nil, "", err
			}

			return &ac, path, nil
		}

		dir = filepath.Dir(dir)
	}

	return nil, "", nil
}

func LoadAppConfigUnder(dir string) (*AppConfig, error) {
	path := filepath.Join(dir, AppConfigPath)
	fi, err := os.Open(path)
	if err == nil {
		defer fi.Close()

		var ac AppConfig
		dec := toml.NewDecoder(fi)
		dec.DisallowUnknownFields()
		err = dec.Decode(&ac)
		if err != nil {
			return nil, err
		}

		// Validate the configuration
		if err := ac.Validate(); err != nil {
			return nil, err
		}

		return &ac, nil
	}

	return nil, nil
}

func Parse(data []byte) (*AppConfig, error) {
	var ac AppConfig
	dec := toml.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	err := dec.Decode(&ac)
	if err != nil {
		return nil, err
	}

	// Validate the configuration
	if err := ac.Validate(); err != nil {
		return nil, err
	}

	return &ac, nil
}

// Validate checks that the AppConfig has valid values
func (ac *AppConfig) Validate() error {
	// Validate global environment variables
	for i, ev := range ac.EnvVars {
		if ev.Key == "" {
			return fmt.Errorf("env[%d]: key is required", i)
		}
	}

	// Validate service configurations
	for serviceName, svcConfig := range ac.Services {
		if svcConfig == nil {
			continue
		}

		// Validate concurrency if present
		if svcConfig.Concurrency != nil {
			concurrency := svcConfig.Concurrency

			// Validate mode
			if concurrency.Mode != "" && concurrency.Mode != "auto" && concurrency.Mode != "fixed" {
				return fmt.Errorf("service %s: invalid concurrency mode %q, must be \"auto\" or \"fixed\"", serviceName, concurrency.Mode)
			}

			// Validate auto mode settings
			if concurrency.Mode == "auto" || concurrency.Mode == "" {
				if concurrency.RequestsPerInstance < 0 {
					return fmt.Errorf("service %s: requests_per_instance must be non-negative", serviceName)
				}
				if concurrency.ScaleDownDelay != "" {
					if _, err := time.ParseDuration(concurrency.ScaleDownDelay); err != nil {
						return fmt.Errorf("service %s: invalid scale_down_delay %q: %v", serviceName, concurrency.ScaleDownDelay, err)
					}
				}
				if concurrency.NumInstances > 0 {
					return fmt.Errorf("service %s: num_instances cannot be set in auto mode", serviceName)
				}
			}

			// Validate fixed mode settings
			if concurrency.Mode == "fixed" {
				if concurrency.NumInstances <= 0 {
					return fmt.Errorf("service %s: num_instances must be at least 1 for fixed mode", serviceName)
				}
				if concurrency.RequestsPerInstance > 0 {
					return fmt.Errorf("service %s: requests_per_instance cannot be set in fixed mode", serviceName)
				}
				if concurrency.ScaleDownDelay != "" {
					return fmt.Errorf("service %s: scale_down_delay cannot be set in fixed mode", serviceName)
				}
			}

			// Validate shutdown_timeout (applies to both modes)
			if concurrency.ShutdownTimeout != "" {
				if _, err := time.ParseDuration(concurrency.ShutdownTimeout); err != nil {
					return fmt.Errorf("service %s: invalid shutdown_timeout %q: %v", serviceName, concurrency.ShutdownTimeout, err)
				}
			}
		}

		// Validate service environment variables
		for i, ev := range svcConfig.EnvVars {
			if ev.Key == "" {
				return fmt.Errorf("service %s: env[%d] key is required", serviceName, i)
			}
		}

		// Validate ports configuration
		if len(svcConfig.Ports) > 0 {
			// Mutual exclusion: cannot use both ports[] and scalar port fields
			if svcConfig.Port != 0 || svcConfig.PortName != "" || svcConfig.PortType != "" {
				return fmt.Errorf("service %s: cannot use both 'ports' array and scalar port/port_name/port_type fields", serviceName)
			}

			seenNames := make(map[string]bool)
			type portProto struct {
				port     int
				protocol string
			}
			seenPortProto := make(map[portProto]bool)
			for i, p := range svcConfig.Ports {
				if p.Port <= 0 || p.Port > 65535 {
					return fmt.Errorf("service %s: ports[%d] port must be between 1 and 65535", serviceName, i)
				}
				if p.Name == "" {
					return fmt.Errorf("service %s: ports[%d] name is required", serviceName, i)
				}
				if p.Type != "" && p.Type != "http" && p.Type != "tcp" && p.Type != "udp" {
					return fmt.Errorf("service %s: ports[%d] type must be \"http\", \"tcp\", or \"udp\"", serviceName, i)
				}
				proto := "tcp"
				if p.Type == "udp" {
					proto = "udp"
				}
				if p.NodePort < 0 || p.NodePort > 65535 {
					return fmt.Errorf("service %s: ports[%d] node_port must be between 0 and 65535", serviceName, i)
				}
				if seenNames[p.Name] {
					return fmt.Errorf("service %s: ports[%d] duplicate port name %q", serviceName, i, p.Name)
				}
				seenNames[p.Name] = true
				pp := portProto{p.Port, proto}
				if seenPortProto[pp] {
					return fmt.Errorf("service %s: ports[%d] duplicate port number %d (protocol %q)", serviceName, i, p.Port, proto)
				}
				seenPortProto[pp] = true
			}
		}

		// Validate disk configurations
		if len(svcConfig.Disks) > 0 {
			// Services with disks must use fixed concurrency mode
			if svcConfig.Concurrency == nil || svcConfig.Concurrency.Mode != "fixed" {
				return fmt.Errorf("service %s: disks can only be attached to services with fixed concurrency mode", serviceName)
			}

			// TODO: It's too unpredictable to allow multiple instances with disks for now
			if svcConfig.Concurrency.NumInstances != 1 {
				return fmt.Errorf("service %s: disks can only be attached to services with fixed concurrency mode and num_instances=1", serviceName)
			}

			for i, disk := range svcConfig.Disks {
				if disk.Name == "" {
					return fmt.Errorf("service %s: disk[%d] must have a name", serviceName, i)
				}
				if disk.MountPath == "" {
					return fmt.Errorf("service %s: disk[%d] (%s) must have a mount_path", serviceName, i, disk.Name)
				}
				if disk.Filesystem != "" && disk.Filesystem != "ext4" && disk.Filesystem != "xfs" && disk.Filesystem != "btrfs" {
					return fmt.Errorf("service %s: disk[%d] (%s) has invalid filesystem %q, must be ext4, xfs, or btrfs", serviceName, i, disk.Name, disk.Filesystem)
				}
				if disk.SizeGB < 0 {
					return fmt.Errorf("service %s: disk[%d] (%s) size_gb must be non-negative", serviceName, i, disk.Name)
				}
				if disk.LeaseTimeout != "" {
					if _, err := time.ParseDuration(disk.LeaseTimeout); err != nil {
						return fmt.Errorf("service %s: disk[%d] (%s) invalid lease_timeout %q: %v", serviceName, i, disk.Name, disk.LeaseTimeout, err)
					}
				}
			}
		}
	}

	for name, target := range ac.Aliases {
		words := strings.Fields(name)
		if len(words) == 0 {
			return fmt.Errorf("alias %q: name must not be empty", name)
		}
		for _, word := range words {
			if !aliasWordRegexp.MatchString(word) {
				return fmt.Errorf("alias %q: each word must start with a lowercase letter and contain only lowercase letters, numbers, dashes, and underscores", name)
			}
		}
		if strings.TrimSpace(target) == "" {
			return fmt.Errorf("alias %q: command must not be empty", name)
		}
	}

	return nil
}

// ResolveDefaults populates Services map for all service names with fully-resolved defaults.
// If a service already has explicit config in app.toml, it is preserved.
// Otherwise, defaults are applied based on service name:
//   - "web": auto mode, requests_per_instance=10, scale_down_delay=15m
//   - others: fixed mode, num_instances=1
func (ac *AppConfig) ResolveDefaults(services []string) {
	if ac.Services == nil {
		ac.Services = make(map[string]*ServiceConfig)
	}

	for _, serviceName := range services {
		// Skip if service already has config
		if _, exists := ac.Services[serviceName]; exists {
			continue
		}

		// Apply defaults based on service name
		if serviceName == "web" {
			ac.Services[serviceName] = &ServiceConfig{
				Concurrency: &ServiceConcurrencyConfig{
					Mode:                "auto",
					RequestsPerInstance: 10,
					ScaleDownDelay:      "15m",
					ShutdownTimeout:     "10s",
				},
			}
		} else {
			ac.Services[serviceName] = &ServiceConfig{
				Concurrency: &ServiceConcurrencyConfig{
					Mode:            "fixed",
					NumInstances:    1,
					ShutdownTimeout: "10s",
				},
			}
		}
	}
}

// GetDefaultsForServices returns an AppConfig with defaults resolved for given service names.
// This is useful for migration - it provides the same defaults used at build time.
func GetDefaultsForServices(serviceNames []string) *AppConfig {
	ac := &AppConfig{}
	ac.ResolveDefaults(serviceNames)
	return ac
}

// AliasLineNumbers parses the TOML file at configPath and returns a map from
// alias name to the line number where it is defined. Uses the go-toml/v2 AST
// parser for accurate source locations.
func AliasLineNumbers(configPath string) map[string]int {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil
	}

	var p tomlast.Parser
	p.Reset(data)

	result := make(map[string]int)

	for p.NextExpression() {
		node := p.Expression()
		if node.Kind != tomlast.Table {
			continue
		}

		// Check if this is the [aliases] table
		keyIter := node.Key()
		if !keyIter.Next() {
			continue
		}
		if string(keyIter.Node().Data) != "aliases" {
			continue
		}

		// Iterate subsequent KeyValue expressions under [aliases]
		for p.NextExpression() {
			kv := p.Expression()
			if kv.Kind != tomlast.KeyValue {
				break
			}
			keyIter := kv.Key()
			if !keyIter.Next() {
				continue
			}
			keyNode := keyIter.Node()
			name := string(keyNode.Data)
			shape := p.Shape(keyNode.Raw)
			result[name] = shape.Start.Line
		}
		break
	}

	return result
}
