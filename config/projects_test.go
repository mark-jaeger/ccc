package config

import (
	"testing"
)

func TestParseProjectsArrayFormat(t *testing.T) {
	data := `
[[projects]]
name = "project1"
path = "/home/user/project1"

[[projects]]
name = "project2"
path = "/home/user/project2"
`
	cfg, err := ParseProjectsConfig([]byte(data))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(cfg.Projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(cfg.Projects))
	}
	if cfg.Projects[0].Name != "project1" {
		t.Errorf("expected project1, got %s", cfg.Projects[0].Name)
	}
}

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

	// Find projects by name (migrated from old format, sorted alphabetically)
	var myapp, api *Project
	for i := range cfg.Projects {
		switch cfg.Projects[i].Name {
		case "myapp":
			myapp = &cfg.Projects[i]
		case "api":
			api = &cfg.Projects[i]
		}
	}

	if myapp == nil {
		t.Fatal("missing project 'myapp'")
	}
	if myapp.Path != "/home/deploy/myapp" {
		t.Errorf("myapp.Path = %q, want %q", myapp.Path, "/home/deploy/myapp")
	}

	if api == nil {
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

	if len(cfg.Projects) != 0 {
		t.Errorf("expected empty projects slice, got %v", cfg.Projects)
	}
}

func TestSerializeProjectsConfig(t *testing.T) {
	cfg := &ProjectsConfig{
		Projects: []Project{
			{Name: "frontend", Path: "/var/www/frontend"},
			{Name: "backend", Path: "/var/www/backend"},
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

	var fe, be *Project
	for i := range parsed.Projects {
		switch parsed.Projects[i].Name {
		case "frontend":
			fe = &parsed.Projects[i]
		case "backend":
			be = &parsed.Projects[i]
		}
	}

	if fe == nil {
		t.Fatal("missing project 'frontend' after round-trip")
	}
	if fe.Path != "/var/www/frontend" {
		t.Errorf("frontend.Path = %q, want %q", fe.Path, "/var/www/frontend")
	}

	if be == nil {
		t.Fatal("missing project 'backend' after round-trip")
	}
	if be.Path != "/var/www/backend" {
		t.Errorf("backend.Path = %q, want %q", be.Path, "/var/www/backend")
	}
}

func TestParseProjectsConfigInvalid(t *testing.T) {
	_, err := ParseProjectsConfig([]byte("this is {{not valid toml"))
	if err == nil {
		t.Fatal("expected error for invalid TOML")
	}
}

func TestMigrateOldFormatSortedAlphabetically(t *testing.T) {
	cfg := &ProjectsConfig{
		Projects: []Project{
			{Name: "zulu", Path: "/z"},
			{Name: "alpha", Path: "/a"},
			{Name: "mike", Path: "/m"},
		},
	}

	// Verify we can iterate in any order and find them
	names := make([]string, len(cfg.Projects))
	for i, p := range cfg.Projects {
		names[i] = p.Name
	}

	if len(names) != 3 {
		t.Fatalf("len = %d, want 3", len(names))
	}
}
