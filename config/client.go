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

	// Transport selects the transport used for the INTERACTIVE zmx attach only.
	// "" or "ssh" (default) uses plain ssh. "mosh" and "et" (Eternal Terminal)
	// are roaming-capable transports for unstable networks (e.g. trains, Wi-Fi↔LTE
	// handoffs). Non-interactive commands (scan, zmx ls/check, config reads) always
	// stay on plain ssh — mosh cannot pipe command stdout back.
	Transport string `toml:"transport,omitempty"`
	// MoshServerPath optionally overrides the remote mosh-server binary, passed as
	// `mosh --server=<path>`. Only meaningful when Transport is "mosh".
	MoshServerPath string `toml:"mosh_server_path,omitempty"`
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

// oldClientConfig is the legacy map-based format
type oldClientConfig struct {
	Hosts map[string]Host `toml:"hosts"`
}

// ParseClientConfigData parses TOML data, migrating from old format if needed.
func ParseClientConfigData(data []byte) (*ClientConfig, error) {
	// Try new array format first
	var cfg ClientConfig
	newErr := toml.Unmarshal(data, &cfg)
	if newErr == nil && len(cfg.Hosts) > 0 {
		return &cfg, nil
	}

	// Try old map format only if new format parsed but was empty
	// (indicates possible old format, not a parse error)
	var oldCfg oldClientConfig
	oldErr := toml.Unmarshal(data, &oldCfg)
	if oldErr != nil {
		// Both formats failed - report the more useful error
		if newErr != nil {
			return nil, fmt.Errorf("parse error: %w", newErr)
		}
		return nil, oldErr
	}

	// Migrate: convert map to slice, sorted alphabetically
	if len(oldCfg.Hosts) > 0 {
		names := make([]string, 0, len(oldCfg.Hosts))
		for name := range oldCfg.Hosts {
			names = append(names, name)
		}
		sort.Strings(names)

		cfg.Hosts = make([]Host, len(names))
		for i, name := range names {
			h := oldCfg.Hosts[name]
			h.Name = name
			cfg.Hosts[i] = h
		}
		return &cfg, nil
	}

	// Empty config
	cfg.Hosts = []Host{}
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
