package config

import (
	"testing"
)

func TestParseProjectsConfig(t *testing.T) {
	data := []byte(`[projects.myapp]
path = "/home/deploy/myapp"

[projects.api]
path = "/home/deploy/api"
`)

	cfg, err := ParseProjectsConfig(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(cfg.Projects))
	}

	myapp, ok := cfg.Projects["myapp"]
	if !ok {
		t.Fatal("missing project 'myapp'")
	}
	if myapp.Path != "/home/deploy/myapp" {
		t.Errorf("myapp.Path = %q, want %q", myapp.Path, "/home/deploy/myapp")
	}

	api, ok := cfg.Projects["api"]
	if !ok {
		t.Fatal("missing project 'api'")
	}
	if api.Path != "/home/deploy/api" {
		t.Errorf("api.Path = %q, want %q", api.Path, "/home/deploy/api")
	}
}

func TestParseProjectsConfigEmpty(t *testing.T) {
	cfg, err := ParseProjectsConfig([]byte{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Projects != nil && len(cfg.Projects) != 0 {
		t.Errorf("expected nil or empty projects map, got %v", cfg.Projects)
	}
}

func TestSerializeProjectsConfig(t *testing.T) {
	cfg := &ProjectsConfig{
		Projects: map[string]Project{
			"frontend": {Path: "/var/www/frontend"},
			"backend":  {Path: "/var/www/backend"},
		},
	}

	data, err := SerializeProjectsConfig(cfg)
	if err != nil {
		t.Fatalf("SerializeProjectsConfig: %v", err)
	}

	// Round-trip: parse back and verify.
	parsed, err := ParseProjectsConfig(data)
	if err != nil {
		t.Fatalf("ParseProjectsConfig after serialize: %v", err)
	}

	if len(parsed.Projects) != 2 {
		t.Fatalf("expected 2 projects after round-trip, got %d", len(parsed.Projects))
	}

	fe, ok := parsed.Projects["frontend"]
	if !ok {
		t.Fatal("missing project 'frontend' after round-trip")
	}
	if fe.Path != "/var/www/frontend" {
		t.Errorf("frontend.Path = %q, want %q", fe.Path, "/var/www/frontend")
	}

	be, ok := parsed.Projects["backend"]
	if !ok {
		t.Fatal("missing project 'backend' after round-trip")
	}
	if be.Path != "/var/www/backend" {
		t.Errorf("backend.Path = %q, want %q", be.Path, "/var/www/backend")
	}
}

func TestSortedProjectKeys(t *testing.T) {
	cfg := &ProjectsConfig{
		Projects: map[string]Project{
			"zulu":  {Path: "/z"},
			"alpha": {Path: "/a"},
			"mike":  {Path: "/m"},
		},
	}

	keys := cfg.SortedProjectKeys()
	expected := []string{"alpha", "mike", "zulu"}

	if len(keys) != len(expected) {
		t.Fatalf("len = %d, want %d", len(keys), len(expected))
	}
	for i, key := range keys {
		if key != expected[i] {
			t.Errorf("keys[%d] = %q, want %q", i, key, expected[i])
		}
	}
}
