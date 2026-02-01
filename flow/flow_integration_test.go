//go:build integration

package flow_test

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/mark-jaeger/ccc/config"
	"github.com/mark-jaeger/ccc/flow"
	"github.com/mark-jaeger/ccc/internal/testutil"
	"github.com/mark-jaeger/ccc/tmux"
)

// slowReader wraps an io.Reader and returns at most one byte per Read call.
// This prevents bufio.Scanner from buffering the entire input, allowing
// multiple sequential scanners to read from the same underlying reader.
type slowReader struct {
	r io.Reader
}

func (s *slowReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	return s.r.Read(p[:1])
}

func newSlowReader(s string) *slowReader {
	return &slowReader{strings.NewReader(s)}
}

func TestProjectFlow_CreateNewSession(t *testing.T) {
	t.Parallel()
	tt := testutil.NewTestTmux(t)

	projects := &config.ProjectsConfig{
		Projects: map[string]config.Project{
			"myapp": {Path: "/tmp"},
		},
	}

	// Input: select project 1, enter for default session name
	in := &slowReader{strings.NewReader("1\n\n")}
	out := &bytes.Buffer{}

	err := flow.ProjectFlow(in, out, tt, projects, nil, nil)
	if err != nil {
		t.Fatalf("ProjectFlow error: %v", err)
	}

	// Verify session was actually created in tmux
	if !tt.SessionExists(t, "myapp") {
		t.Error("expected session 'myapp' to exist in tmux")
	}

	// Verify notification options were set
	bellAction := tt.GetOption(t, "myapp", "bell-action")
	if bellAction != "any" {
		t.Errorf("bell-action = %q, want %q", bellAction, "any")
	}

	passthrough := tt.GetWindowOption(t, "myapp", "allow-passthrough")
	if passthrough != "on" {
		t.Errorf("allow-passthrough = %q, want %q", passthrough, "on")
	}
}

func TestProjectFlow_AttachExistingSession_SetsNotifyOptions(t *testing.T) {
	t.Parallel()
	tt := testutil.NewTestTmux(t)

	// Pre-create a bare session (simulates older ccc version)
	tt.CreateSession(t, "myapp")

	// Tag it with metadata so SessionFlow finds it as verified
	tagCmd := "tmux set-option -t myapp @ccc_project myapp \\; set-option -t myapp @ccc_path /tmp"
	if _, err := tt.Run(tagCmd); err != nil {
		t.Fatalf("failed to tag session: %v", err)
	}

	projects := &config.ProjectsConfig{
		Projects: map[string]config.Project{
			"myapp": {Path: "/tmp"},
		},
	}

	// Input: select project 1, then select session 1
	in := &slowReader{strings.NewReader("1\n1\n")}
	out := &bytes.Buffer{}

	err := flow.ProjectFlow(in, out, tt, projects, nil, nil)
	if err != nil {
		t.Fatalf("ProjectFlow error: %v", err)
	}

	// Verify notification options were retroactively applied
	bellAction := tt.GetOption(t, "myapp", "bell-action")
	if bellAction != "any" {
		t.Errorf("bell-action = %q, want %q (should be set on attach)", bellAction, "any")
	}

	visualBell := tt.GetWindowOption(t, "myapp", "visual-bell")
	if visualBell != "off" {
		t.Errorf("visual-bell = %q, want %q", visualBell, "off")
	}

	passthrough := tt.GetWindowOption(t, "myapp", "allow-passthrough")
	if passthrough != "on" {
		t.Errorf("allow-passthrough = %q, want %q", passthrough, "on")
	}
}

func TestProjectFlow_MultipleSessionsFiltered(t *testing.T) {
	t.Parallel()
	tt := testutil.NewTestTmux(t)

	// Create sessions for two different projects
	cmd1 := tmux.BuildCreateCommand("myapp", "/tmp", "myapp")
	if _, err := tt.Run(cmd1); err != nil {
		t.Fatalf("create myapp failed: %v", err)
	}
	cmd2 := tmux.BuildCreateCommand("other", "/tmp", "other")
	if _, err := tt.Run(cmd2); err != nil {
		t.Fatalf("create other failed: %v", err)
	}

	projects := &config.ProjectsConfig{
		Projects: map[string]config.Project{
			"myapp": {Path: "/tmp"},
		},
	}

	// Input: select project 1, then select session 1 from session menu
	in := &slowReader{strings.NewReader("1\n1\n")}
	out := &bytes.Buffer{}

	err := flow.ProjectFlow(in, out, tt, projects, nil, nil)
	if err != nil {
		t.Fatalf("ProjectFlow error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "myapp") {
		t.Errorf("expected myapp in output, got: %s", output)
	}
	if strings.Contains(output, "Sessions for other") {
		t.Errorf("should not show sessions for 'other' project, got: %s", output)
	}
}

func TestSessionFlow_DetachSession(t *testing.T) {
	t.Parallel()
	tt := testutil.NewTestTmux(t)

	// Create a session with metadata
	cmd := tmux.BuildCreateCommand("myapp", "/tmp", "myapp")
	if _, err := tt.Run(cmd); err != nil {
		t.Fatalf("create session failed: %v", err)
	}

	// Input: select session 1 for detach action, then quit
	// "d" triggers detach, "1" selects which session, then "q" quits the menu
	in := newSlowReader("d\n1\nq\n")
	out := &bytes.Buffer{}

	err := flow.SessionFlow(in, out, tt, "myapp", "/tmp")
	if err != nil {
		t.Fatalf("SessionFlow error: %v", err)
	}

	// Session should still exist after detach
	if !tt.SessionExists(t, "myapp") {
		t.Error("expected session 'myapp' to still exist after detach")
	}

	output := out.String()
	if !strings.Contains(output, "No clients attached") {
		t.Errorf("expected 'No clients attached' message (no real clients in test), got: %s", output)
	}
}
