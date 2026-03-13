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
	Name              string   `toml:"name"`
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
	Hosts []Host `toml:"hosts"`
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

// ParseClientConfigData parses TOML config data from a byte slice.
func ParseClientConfigData(data []byte) (*ClientConfig, error) {
	var cfg ClientConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.Hosts == nil {
		cfg.Hosts = []Host{}
	}
	return &cfg, nil
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
	return ParseClientConfigData(data)
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

// HostByName returns the host with the given name and true, or zero value and
// false if not found.
func (c *ClientConfig) HostByName(name string) (Host, bool) {
	for _, h := range c.Hosts {
		if h.Name == name {
			return h, true
		}
	}
	return Host{}, false
}

// AddHost adds or replaces a host entry in the config by name.
func (c *ClientConfig) AddHost(name string, host Host) {
	host.Name = name
	for i, h := range c.Hosts {
		if h.Name == name {
			c.Hosts[i] = host
			return
		}
	}
	c.Hosts = append(c.Hosts, host)
}

// RemoveHost removes a host entry from the config by name.
func (c *ClientConfig) RemoveHost(name string) {
	hosts := make([]Host, 0, len(c.Hosts))
	for _, h := range c.Hosts {
		if h.Name != name {
			hosts = append(hosts, h)
		}
	}
	c.Hosts = hosts
}

// SortedHostNames returns the host names in alphabetical order.
func (c *ClientConfig) SortedHostNames() []string {
	names := make([]string, 0, len(c.Hosts))
	for _, h := range c.Hosts {
		names = append(names, h.Name)
	}
	sort.Strings(names)
	return names
}
