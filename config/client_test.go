package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadClientConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	data := `[hosts.prod]
user = "deploy"
address = "10.0.0.1"
port = 2222
identity_file = "/home/deploy/.ssh/id_ed25519"
proxy_jump = "bastion"
ssh_options = ["-o", "StrictHostKeyChecking=no"]

[hosts.staging]
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

	prod, ok := cfg.Hosts["prod"]
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

	staging, ok := cfg.Hosts["staging"]
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
		Hosts: map[string]Host{
			"web": {
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
	web, ok := loaded.Hosts["web"]
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
	h, ok := cfg.Hosts["new-server"]
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
		Hosts: map[string]Host{
			"alpha": {User: "a", Address: "1.1.1.1"},
			"beta":  {User: "b", Address: "2.2.2.2"},
		},
	}

	cfg.RemoveHost("alpha")

	if len(cfg.Hosts) != 1 {
		t.Fatalf("expected 1 host after removal, got %d", len(cfg.Hosts))
	}
	if _, ok := cfg.Hosts["alpha"]; ok {
		t.Error("host 'alpha' should have been removed")
	}
	if _, ok := cfg.Hosts["beta"]; !ok {
		t.Error("host 'beta' should still exist")
	}
}

func TestSortedHostNames(t *testing.T) {
	cfg := &ClientConfig{
		Hosts: map[string]Host{
			"charlie": {},
			"alpha":   {},
			"bravo":   {},
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
