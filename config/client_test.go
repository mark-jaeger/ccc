package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadClientConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	data := `
[[hosts]]
name = "prod"
user = "deploy"
address = "10.0.0.1"
port = 2222
identity_file = "/home/deploy/.ssh/id_ed25519"
proxy_jump = "bastion"
ssh_options = ["-o", "StrictHostKeyChecking=no"]

[[hosts]]
name = "staging"
user = "admin"
address = "10.0.0.2"
`

	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadClientConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Hosts) != 2 {
		t.Fatalf("expected 2 hosts, got %d", len(cfg.Hosts))
	}

	prod, ok := cfg.HostByName("prod")
	if !ok {
		t.Fatal("missing host 'prod'")
	}
	if prod.User != "deploy" {
		t.Errorf("prod.User = %q, want %q", prod.User, "deploy")
	}
	if prod.Address != "10.0.0.1" {
		t.Errorf("prod.Address = %q, want %q", prod.Address, "10.0.0.1")
	}
	if prod.Port != 2222 {
		t.Errorf("prod.Port = %d, want %d", prod.Port, 2222)
	}
	if prod.IdentityFile != "/home/deploy/.ssh/id_ed25519" {
		t.Errorf("prod.IdentityFile = %q, want %q", prod.IdentityFile, "/home/deploy/.ssh/id_ed25519")
	}
	if prod.ProxyJump != "bastion" {
		t.Errorf("prod.ProxyJump = %q, want %q", prod.ProxyJump, "bastion")
	}
	if len(prod.SSHOptions) != 2 || prod.SSHOptions[0] != "-o" || prod.SSHOptions[1] != "StrictHostKeyChecking=no" {
		t.Errorf("prod.SSHOptions = %v, want [-o StrictHostKeyChecking=no]", prod.SSHOptions)
	}

	staging, ok := cfg.HostByName("staging")
	if !ok {
		t.Fatal("missing host 'staging'")
	}
	if staging.User != "admin" {
		t.Errorf("staging.User = %q, want %q", staging.User, "admin")
	}
	if staging.Address != "10.0.0.2" {
		t.Errorf("staging.Address = %q, want %q", staging.Address, "10.0.0.2")
	}
	if staging.Port != 0 {
		t.Errorf("staging.Port = %d, want %d", staging.Port, 0)
	}
}

func TestLoadClientConfigMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.toml")

	_, err := LoadClientConfig(path)
	if err != ErrNoConfig {
		t.Fatalf("expected ErrNoConfig, got %v", err)
	}
}

func TestSaveClientConfig(t *testing.T) {
	dir := t.TempDir()
	// Use a nested path to verify directory creation.
	path := filepath.Join(dir, "sub", "dir", "config.toml")

	cfg := &ClientConfig{
		Hosts: []Host{
			{
				Name:    "web",
				User:    "www",
				Address: "192.168.1.10",
				Port:    22,
			},
		},
	}

	if err := SaveClientConfig(path, cfg); err != nil {
		t.Fatalf("SaveClientConfig: %v", err)
	}

	// Verify file permissions.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("file permissions = %o, want 0600", perm)
	}

	// Round-trip: load back and verify.
	loaded, err := LoadClientConfig(path)
	if err != nil {
		t.Fatalf("LoadClientConfig after save: %v", err)
	}
	if len(loaded.Hosts) != 1 {
		t.Fatalf("expected 1 host after round-trip, got %d", len(loaded.Hosts))
	}
	web, ok := loaded.HostByName("web")
	if !ok {
		t.Fatal("missing host 'web' after round-trip")
	}
	if web.User != "www" {
		t.Errorf("web.User = %q, want %q", web.User, "www")
	}
	if web.Address != "192.168.1.10" {
		t.Errorf("web.Address = %q, want %q", web.Address, "192.168.1.10")
	}
	if web.Port != 22 {
		t.Errorf("web.Port = %d, want %d", web.Port, 22)
	}
}

func TestAddHost(t *testing.T) {
	cfg := &ClientConfig{}
	cfg.AddHost("new-server", Host{
		User:    "root",
		Address: "10.0.0.99",
	})

	if len(cfg.Hosts) != 1 {
		t.Fatalf("expected 1 host, got %d", len(cfg.Hosts))
	}
	h, ok := cfg.HostByName("new-server")
	if !ok {
		t.Fatal("missing host 'new-server'")
	}
	if h.User != "root" {
		t.Errorf("User = %q, want %q", h.User, "root")
	}
	if h.Address != "10.0.0.99" {
		t.Errorf("Address = %q, want %q", h.Address, "10.0.0.99")
	}
}

func TestRemoveHost(t *testing.T) {
	cfg := &ClientConfig{
		Hosts: []Host{
			{Name: "alpha", User: "a", Address: "1.1.1.1"},
			{Name: "beta", User: "b", Address: "2.2.2.2"},
		},
	}

	cfg.RemoveHost("alpha")

	if len(cfg.Hosts) != 1 {
		t.Fatalf("expected 1 host after removal, got %d", len(cfg.Hosts))
	}
	if _, ok := cfg.HostByName("alpha"); ok {
		t.Error("host 'alpha' should have been removed")
	}
	if _, ok := cfg.HostByName("beta"); !ok {
		t.Error("host 'beta' should still exist")
	}
}

func TestSortedHostNames(t *testing.T) {
	cfg := &ClientConfig{
		Hosts: []Host{
			{Name: "charlie"},
			{Name: "alpha"},
			{Name: "bravo"},
		},
	}

	names := cfg.SortedHostNames()
	expected := []string{"alpha", "bravo", "charlie"}

	if len(names) != len(expected) {
		t.Fatalf("len = %d, want %d", len(names), len(expected))
	}
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("names[%d] = %q, want %q", i, name, expected[i])
		}
	}
}

func TestHostValidate(t *testing.T) {
	tests := []struct {
		name    string
		host    Host
		wantErr bool
	}{
		{"valid", Host{User: "deploy", Address: "10.0.0.1"}, false},
		{"missing user", Host{Address: "10.0.0.1"}, true},
		{"missing address", Host{User: "deploy"}, true},
		{"both missing", Host{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.host.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDefaultClientConfigPath(t *testing.T) {
	path, err := DefaultClientConfigPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path == "" {
		t.Fatal("expected non-empty path")
	}
	if filepath.Base(path) != "config.toml" {
		t.Errorf("expected config.toml, got %s", filepath.Base(path))
	}
}

func TestLoadClientConfigInvalidTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	if err := os.WriteFile(path, []byte("this is not valid toml {{{"), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadClientConfig(path)
	if err == nil {
		t.Fatal("expected error for invalid TOML")
	}
	if errors.Is(err, ErrNoConfig) {
		t.Fatal("should not be ErrNoConfig for invalid TOML")
	}
}

func TestParseArrayFormat(t *testing.T) {
	data := `
[[hosts]]
name = "server1"
user = "deploy"
address = "10.0.0.1"

[[hosts]]
name = "server2"
user = "admin"
address = "10.0.0.2"
port = 2222
`
	cfg, err := ParseClientConfigData([]byte(data))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(cfg.Hosts) != 2 {
		t.Fatalf("expected 2 hosts, got %d", len(cfg.Hosts))
	}
	if cfg.Hosts[0].Name != "server1" {
		t.Errorf("expected server1, got %s", cfg.Hosts[0].Name)
	}
	if cfg.Hosts[1].Port != 2222 {
		t.Errorf("expected port 2222, got %d", cfg.Hosts[1].Port)
	}
}
