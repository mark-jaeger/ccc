// Package config handles reading, writing, and validating TOML configuration
// files for client hosts and project tracking.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	toml "github.com/pelletier/go-toml/v2"
)

// ErrNoConfig is returned when the config file does not exist.
var ErrNoConfig = errors.New("config file not found")

// Host represents connection details for a remote host.
type Host struct {
	User              string   `toml:"user"`
	Address           string   `toml:"address"`
	Port              int      `toml:"port,omitempty"`
	IdentityFile      string   `toml:"identity_file,omitempty"`
	ProxyJump         string   `toml:"proxy_jump,omitempty"`
	SSHOptions        []string `toml:"ssh_options,omitempty"`
	FallbackAddresses []string `toml:"fallback_addresses,omitempty"`
}

// ClientConfig holds the client-side configuration including known hosts.
type ClientConfig struct {
	Hosts map[string]Host `toml:"hosts"`
}

// DefaultClientConfigPath returns the default path for the client config file.
// Returns an error if the home directory cannot be determined.
func DefaultClientConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".ccc", "config.toml"), nil
}

// Validate checks that the host has the minimum required fields.
func (h Host) Validate() error {
	if h.User == "" {
		return fmt.Errorf("host user is required")
	}
	if h.Address == "" {
		return fmt.Errorf("host address is required")
	}
	return nil
}

// LoadClientConfig reads and parses a TOML client config from the given path.
// Returns ErrNoConfig if the file does not exist.
func LoadClientConfig(path string) (*ClientConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNoConfig
		}
		return nil, err
	}

	var cfg ClientConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.Hosts == nil {
		cfg.Hosts = make(map[string]Host)
	}
	return &cfg, nil
}

// SaveClientConfig serializes the config to TOML and writes it to the given path.
// Parent directories are created with mode 0700; the file itself is written with
// mode 0600.
func SaveClientConfig(path string, cfg *ClientConfig) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	data, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

// AddHost adds or replaces a host entry in the config.
func (c *ClientConfig) AddHost(name string, host Host) {
	if c.Hosts == nil {
		c.Hosts = make(map[string]Host)
	}
	c.Hosts[name] = host
}

// RemoveHost removes a host entry from the config.
func (c *ClientConfig) RemoveHost(name string) {
	delete(c.Hosts, name)
}

// SortedHostNames returns the host names in alphabetical order.
func (c *ClientConfig) SortedHostNames() []string {
	names := make([]string, 0, len(c.Hosts))
	for name := range c.Hosts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
