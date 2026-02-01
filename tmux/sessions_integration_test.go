//go:build integration

package tmux_test

import (
	"testing"

	"github.com/mark-jaeger/ccc/internal/testutil"
	"github.com/mark-jaeger/ccc/tmux"
)

func TestCreateSession_SetsMetadata(t *testing.T) {
	t.Parallel()
	tt := testutil.NewTestTmux(t)

	cmd := tmux.BuildCreateCommand("myapp", "/tmp/myapp", "myapp")
	if _, err := tt.Run(cmd); err != nil {
		t.Fatalf("create command failed: %v", err)
	}

	project := tt.GetOption(t, "myapp", "@ccc_project")
	if project != "myapp" {
		t.Errorf("@ccc_project = %q, want %q", project, "myapp")
	}

	path := tt.GetOption(t, "myapp", "@ccc_path")
	if path != "/tmp/myapp" {
		t.Errorf("@ccc_path = %q, want %q", path, "/tmp/myapp")
	}
}

func TestCreateSession_SetsBellOptions(t *testing.T) {
	t.Parallel()
	tt := testutil.NewTestTmux(t)

	cmd := tmux.BuildCreateCommand("myapp", "/tmp/myapp", "myapp")
	if _, err := tt.Run(cmd); err != nil {
		t.Fatalf("create command failed: %v", err)
	}

	bellAction := tt.GetOption(t, "myapp", "bell-action")
	if bellAction != "any" {
		t.Errorf("bell-action = %q, want %q", bellAction, "any")
	}

	visualBell := tt.GetWindowOption(t, "myapp", "visual-bell")
	if visualBell != "off" {
		t.Errorf("visual-bell = %q, want %q", visualBell, "off")
	}
}

func TestSetPassthrough_EnablesOption(t *testing.T) {
	t.Parallel()
	tt := testutil.NewTestTmux(t)

	tt.CreateSession(t, "myapp")

	cmd := tmux.BuildSetPassthroughCommand("myapp")
	if _, err := tt.Run(cmd); err != nil {
		t.Fatalf("set passthrough failed: %v", err)
	}

	val := tt.GetWindowOption(t, "myapp", "allow-passthrough")
	if val != "on" {
		t.Errorf("allow-passthrough = %q, want %q", val, "on")
	}
}

func TestEnsureNotifyOptions_SetsAllOptions(t *testing.T) {
	t.Parallel()
	tt := testutil.NewTestTmux(t)

	tt.CreateSession(t, "oldapp")

	cmd := tmux.BuildEnsureNotifyOptionsCommand("oldapp")
	if _, err := tt.Run(cmd); err != nil {
		t.Fatalf("ensure notify options failed: %v", err)
	}

	bellAction := tt.GetOption(t, "oldapp", "bell-action")
	if bellAction != "any" {
		t.Errorf("bell-action = %q, want %q", bellAction, "any")
	}

	visualBell := tt.GetWindowOption(t, "oldapp", "visual-bell")
	if visualBell != "off" {
		t.Errorf("visual-bell = %q, want %q", visualBell, "off")
	}

	passthrough := tt.GetWindowOption(t, "oldapp", "allow-passthrough")
	if passthrough != "on" {
		t.Errorf("allow-passthrough = %q, want %q", passthrough, "on")
	}
}

func TestEnsureNotifyOptions_Idempotent(t *testing.T) {
	t.Parallel()
	tt := testutil.NewTestTmux(t)

	tt.CreateSession(t, "myapp")

	cmd := tmux.BuildEnsureNotifyOptionsCommand("myapp")

	if _, err := tt.Run(cmd); err != nil {
		t.Fatalf("first ensure notify options failed: %v", err)
	}
	if _, err := tt.Run(cmd); err != nil {
		t.Fatalf("second ensure notify options failed: %v", err)
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

func TestFilterSessionsForProject_RealTmux(t *testing.T) {
	t.Parallel()
	tt := testutil.NewTestTmux(t)

	cmd1 := tmux.BuildCreateCommand("proj", "/tmp/proj", "proj")
	if _, err := tt.Run(cmd1); err != nil {
		t.Fatalf("create proj failed: %v", err)
	}
	cmd2 := tmux.BuildCreateCommand("proj-2", "/tmp/proj", "proj")
	if _, err := tt.Run(cmd2); err != nil {
		t.Fatalf("create proj-2 failed: %v", err)
	}
	cmd3 := tmux.BuildCreateCommand("other", "/tmp/other", "other")
	if _, err := tt.Run(cmd3); err != nil {
		t.Fatalf("create other failed: %v", err)
	}
	tt.CreateSession(t, "proj-legacy")

	listCmd := tmux.BuildListCommand()
	output, err := tt.Run(listCmd)
	if err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	allSessions := tmux.ParseSessionList(output)
	filtered := tmux.FilterSessionsForProject(allSessions, "proj")

	if len(filtered) != 3 {
		names := make([]string, len(filtered))
		for i, s := range filtered {
			names[i] = s.Name
		}
		t.Fatalf("expected 3 filtered sessions, got %d: %v", len(filtered), names)
	}

	found := map[string]bool{}
	for _, s := range filtered {
		found[s.Name] = true
		switch s.Name {
		case "proj", "proj-2":
			if !s.Verified {
				t.Errorf("%s should be verified", s.Name)
			}
		case "proj-legacy":
			if s.Verified {
				t.Errorf("proj-legacy should NOT be verified (prefix match)")
			}
		}
	}

	for _, name := range []string{"proj", "proj-2", "proj-legacy"} {
		if !found[name] {
			t.Errorf("expected %s in filtered results", name)
		}
	}
}
