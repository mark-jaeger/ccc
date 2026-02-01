//go:build integration

package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mark-jaeger/ccc/internal/testutil"
)

func setupE2EHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	cccDir := filepath.Join(home, ".ccc")
	os.MkdirAll(cccDir, 0755)
	os.WriteFile(filepath.Join(cccDir, "projects.toml"), []byte(`
[projects.myapp]
path = "/tmp"
`), 0600)
	return home
}

func TestE2E_LocalMode_ProjectMenu(t *testing.T) {
	t.Parallel()
	tt := testutil.NewTestTmux(t)
	home := setupE2EHome(t)
	repoRoot, _ := os.Getwd()

	proc := testutil.StartCCC(t, repoRoot, tt.Socket, map[string]string{"HOME": home}, "local")

	output := proc.ReadUntil(t, "myapp", 10*time.Second)
	if output == "" {
		t.Fatal("project menu did not render")
	}
}

func TestE2E_LocalMode_QuitFromMenu(t *testing.T) {
	t.Parallel()
	tt := testutil.NewTestTmux(t)
	home := setupE2EHome(t)
	repoRoot, _ := os.Getwd()

	proc := testutil.StartCCC(t, repoRoot, tt.Socket, map[string]string{"HOME": home}, "local")

	proc.ReadUntil(t, "myapp", 10*time.Second)
	proc.Send(t, "q")

	err := proc.WaitForExit(t, 5*time.Second)
	if err != nil {
		t.Errorf("expected clean exit, got: %v", err)
	}
}

func TestE2E_LocalMode_CreateAndAttachSession(t *testing.T) {
	t.Parallel()
	tt := testutil.NewTestTmux(t)
	home := setupE2EHome(t)
	repoRoot, _ := os.Getwd()

	proc := testutil.StartCCC(t, repoRoot, tt.Socket, map[string]string{"HOME": home}, "local")

	// Select project (press Enter to select highlighted item)
	proc.ReadUntil(t, "myapp", 10*time.Second)
	proc.Send(t, "\n")

	// Accept default session name
	proc.ReadUntil(t, "Session name", 5*time.Second)
	proc.Send(t, "\n")

	// Poll for session creation instead of fixed sleep
	deadline := time.Now().Add(5 * time.Second)
	for !tt.SessionExists(t, "myapp") {
		if time.Now().After(deadline) {
			t.Fatal("session 'myapp' not created within timeout")
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Verify session exists with notification options
	if !tt.SessionExists(t, "myapp") {
		t.Error("expected session 'myapp' to exist")
	}

	bellAction := tt.GetOption(t, "myapp", "bell-action")
	if bellAction != "any" {
		t.Errorf("bell-action = %q, want %q", bellAction, "any")
	}

	passthrough := tt.GetWindowOption(t, "myapp", "allow-passthrough")
	if passthrough != "on" {
		t.Errorf("allow-passthrough = %q, want %q", passthrough, "on")
	}
}
