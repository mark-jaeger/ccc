package flow

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/mark-jaeger/ccc/config"
)

// mockRunner implements the Runner interface for testing.
type mockRunner struct {
	responses map[string]string
	errors    map[string]error
	interactive []string
}

func newMockRunner() *mockRunner {
	return &mockRunner{
		responses: make(map[string]string),
		errors:    make(map[string]error),
	}
}

func (m *mockRunner) Run(cmd string) (string, error) {
	if err, ok := m.errors[cmd]; ok {
		return "", err
	}
	// Check prefix matches for errors
	for pattern, err := range m.errors {
		if strings.Contains(cmd, pattern) {
			return "", err
		}
	}
	if resp, ok := m.responses[cmd]; ok {
		return resp, nil
	}
	// Check prefix matches for commands that include shell quoting
	for pattern, resp := range m.responses {
		if strings.Contains(cmd, pattern) {
			return resp, nil
		}
	}
	return "", fmt.Errorf("unexpected command: %s", cmd)
}

func (m *mockRunner) RunInteractive(cmd string) error {
	m.interactive = append(m.interactive, cmd)
	return nil
}

func TestProjectFlowSelectProject(t *testing.T) {
	runner := newMockRunner()
	// "test -d" for path validation
	runner.responses["test -d"] = ""
	// tmux check
	runner.responses["command -v tmux"] = "/usr/bin/tmux"
	// tmux list-sessions returns empty (no sessions)
	runner.responses["tmux list-sessions"] = ""
	// tmux create session
	runner.responses["tmux new-session"] = ""

	projects := &config.ProjectsConfig{
		Projects: map[string]config.Project{
			"myapp": {Path: "/home/user/myapp"},
		},
	}

	// Input: select project 1, then enter for default session name
	in := strings.NewReader("1\n\n")
	out := &bytes.Buffer{}

	err := ProjectFlow(in, out, runner, projects, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "myapp") {
		t.Errorf("expected project 'myapp' in output, got: %s", output)
	}
}

func TestProjectFlowQuit(t *testing.T) {
	runner := newMockRunner()
	projects := &config.ProjectsConfig{
		Projects: map[string]config.Project{
			"myapp": {Path: "/home/user/myapp"},
		},
	}

	in := strings.NewReader("q\n")
	out := &bytes.Buffer{}

	err := ProjectFlow(in, out, runner, projects, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProjectFlowBack(t *testing.T) {
	runner := newMockRunner()
	projects := &config.ProjectsConfig{
		Projects: map[string]config.Project{
			"myapp": {Path: "/home/user/myapp"},
		},
	}

	in := strings.NewReader("b\n")
	out := &bytes.Buffer{}

	err := ProjectFlow(in, out, runner, projects, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProjectFlowNoProjects(t *testing.T) {
	runner := newMockRunner()
	projects := &config.ProjectsConfig{
		Projects: map[string]config.Project{},
	}

	in := strings.NewReader("")
	out := &bytes.Buffer{}

	err := ProjectFlow(in, out, runner, projects, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "No projects configured") {
		t.Errorf("expected 'No projects configured' message, got: %s", out.String())
	}
}

func TestProjectFlowScanNotAvailable(t *testing.T) {
	runner := newMockRunner()
	projects := &config.ProjectsConfig{
		Projects: map[string]config.Project{
			"myapp": {Path: "/home/user/myapp"},
		},
	}

	// Select scan, then quit
	in := strings.NewReader("s\nq\n")
	out := &bytes.Buffer{}

	err := ProjectFlow(in, out, runner, projects, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "Scan not available") {
		t.Errorf("expected 'Scan not available' message, got: %s", out.String())
	}
}

func TestSessionFlowZeroSessionsCreatesNew(t *testing.T) {
	runner := newMockRunner()
	runner.responses["command -v tmux"] = "/usr/bin/tmux"
	runner.responses["tmux list-sessions"] = ""
	runner.responses["tmux new-session"] = ""

	// Enter key accepts default session name
	in := strings.NewReader("\n")
	out := &bytes.Buffer{}

	err := SessionFlow(in, out, runner, "myapp", "/home/user/myapp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "Created session") {
		t.Errorf("expected session creation message, got: %s", out.String())
	}

	// Should have called RunInteractive for attach
	if len(runner.interactive) == 0 {
		t.Error("expected RunInteractive to be called for attach")
	}
}

func TestSessionFlowOneSessionAutoAttaches(t *testing.T) {
	runner := newMockRunner()
	runner.responses["command -v tmux"] = "/usr/bin/tmux"
	// One verified session
	runner.responses["tmux list-sessions"] = "myapp|||myapp|||/home/user/myapp|||2"
	// No other clients
	runner.responses["tmux list-clients"] = ""

	in := strings.NewReader("")
	out := &bytes.Buffer{}

	err := SessionFlow(in, out, runner, "myapp", "/home/user/myapp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "Attaching") {
		t.Errorf("expected attach message, got: %s", out.String())
	}

	if len(runner.interactive) == 0 {
		t.Error("expected RunInteractive to be called")
	}
}

func TestCreateSessionDefaultName(t *testing.T) {
	runner := newMockRunner()
	runner.responses["command -v tmux"] = "/usr/bin/tmux"
	runner.responses["tmux list-sessions"] = ""
	runner.responses["tmux new-session"] = ""

	// Empty input → accept default name
	in := strings.NewReader("\n")
	out := &bytes.Buffer{}

	err := SessionFlow(in, out, runner, "myapp", "/home/user/myapp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Created session myapp") {
		t.Errorf("expected 'Created session myapp', got: %s", output)
	}
}

func TestCreateSessionCustomName(t *testing.T) {
	runner := newMockRunner()
	runner.responses["command -v tmux"] = "/usr/bin/tmux"
	runner.responses["tmux list-sessions"] = ""
	runner.responses["tmux new-session"] = ""

	// Type "dev" → becomes "myapp-dev"
	in := strings.NewReader("dev\n")
	out := &bytes.Buffer{}

	err := SessionFlow(in, out, runner, "myapp", "/home/user/myapp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Created session myapp-dev") {
		t.Errorf("expected 'Created session myapp-dev', got: %s", output)
	}
}

func TestCreateSessionAlreadyPrefixed(t *testing.T) {
	runner := newMockRunner()
	runner.responses["command -v tmux"] = "/usr/bin/tmux"
	runner.responses["tmux list-sessions"] = ""
	runner.responses["tmux new-session"] = ""

	// Type "myapp-staging" → stays as-is (already has prefix)
	in := strings.NewReader("myapp-staging\n")
	out := &bytes.Buffer{}

	err := SessionFlow(in, out, runner, "myapp", "/home/user/myapp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Created session myapp-staging") {
		t.Errorf("expected 'Created session myapp-staging', got: %s", output)
	}
}

func TestCreateSessionNameMatchesProject(t *testing.T) {
	runner := newMockRunner()
	runner.responses["command -v tmux"] = "/usr/bin/tmux"
	runner.responses["tmux list-sessions"] = ""
	runner.responses["tmux new-session"] = ""

	// Type "myapp" → stays as "myapp"
	in := strings.NewReader("myapp\n")
	out := &bytes.Buffer{}

	err := SessionFlow(in, out, runner, "myapp", "/home/user/myapp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Created session myapp") {
		t.Errorf("expected 'Created session myapp', got: %s", output)
	}
	// Should NOT be "myapp-myapp"
	if strings.Contains(output, "myapp-myapp") {
		t.Errorf("should not double-prefix, got: %s", output)
	}
}

func TestRemoveSessionSuccess(t *testing.T) {
	runner := newMockRunner()
	runner.responses["command -v tmux"] = "/usr/bin/tmux"
	// Two verified sessions
	runner.responses["tmux list-sessions"] = "myapp|||myapp|||/home/user/myapp|||2\nmyapp-2|||myapp|||/home/user/myapp|||1"
	runner.responses["tmux list-clients"] = ""
	runner.responses["tmux kill-session"] = ""

	// Select remove, pick session 1, confirm
	in := strings.NewReader("r\n1\ny\n")
	out := &bytes.Buffer{}

	err := SessionFlow(in, out, runner, "myapp", "/home/user/myapp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "Killed session") {
		t.Errorf("expected kill message, got: %s", out.String())
	}
}

func TestRemoveSessionUnverifiedWarning(t *testing.T) {
	runner := newMockRunner()
	runner.responses["command -v tmux"] = "/usr/bin/tmux"
	// Two sessions: one verified, one unverified (prefix match)
	runner.responses["tmux list-sessions"] = "myapp|||myapp|||/home/user/myapp|||2\nmyapp-extra||||||/tmp|||1"
	runner.responses["tmux list-clients"] = ""
	runner.responses["tmux kill-session"] = ""

	// Select remove, pick session 2 (unverified), confirm
	in := strings.NewReader("r\n2\ny\n")
	out := &bytes.Buffer{}

	err := SessionFlow(in, out, runner, "myapp", "/home/user/myapp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "wasn't created by ccc") {
		t.Errorf("expected unverified warning, got: %s", output)
	}
}

func TestAttachSessionVerifiedNoClients(t *testing.T) {
	runner := newMockRunner()
	runner.responses["command -v tmux"] = "/usr/bin/tmux"
	runner.responses["tmux list-sessions"] = "myapp|||myapp|||/home/user/myapp|||2"
	runner.responses["tmux list-clients"] = ""

	in := strings.NewReader("")
	out := &bytes.Buffer{}

	err := SessionFlow(in, out, runner, "myapp", "/home/user/myapp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.interactive) == 0 {
		t.Error("expected RunInteractive to be called for attach")
	}
	found := false
	for _, cmd := range runner.interactive {
		if strings.Contains(cmd, "tmux attach") {
			found = true
		}
	}
	if !found {
		t.Error("expected tmux attach in interactive commands")
	}
}

func TestAttachSessionUnverifiedDeclined(t *testing.T) {
	runner := newMockRunner()
	runner.responses["command -v tmux"] = "/usr/bin/tmux"
	// One unverified session (no project tag, name matches by prefix)
	runner.responses["tmux list-sessions"] = "myapp||||||/tmp|||1"
	runner.responses["tmux list-clients"] = ""

	// User declines unverified session
	in := strings.NewReader("n\n")
	out := &bytes.Buffer{}

	err := SessionFlow(in, out, runner, "myapp", "/home/user/myapp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "wasn't created by ccc") {
		t.Errorf("expected unverified warning, got: %s", out.String())
	}
	if len(runner.interactive) != 0 {
		t.Error("expected no RunInteractive when user declines")
	}
}

func TestProjectFlowPathNotFoundShowsMessage(t *testing.T) {
	runner := newMockRunner()
	runner.errors["test -d"] = fmt.Errorf("exit 1")

	projects := &config.ProjectsConfig{
		Projects: map[string]config.Project{
			"myapp": {Path: "/home/user/myapp"},
		},
	}

	// Select project 1 — path not found message should appear
	// Note: due to bufio.Scanner buffering in ShowMenu, subsequent
	// Confirm input must come from a separate scan cycle
	in := strings.NewReader("1\nq\n")
	out := &bytes.Buffer{}

	err := ProjectFlow(in, out, runner, projects, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Path /home/user/myapp not found") {
		t.Errorf("expected path not found message, got: %s", output)
	}
	if !strings.Contains(output, "Remove from projects?") {
		t.Errorf("expected removal prompt, got: %s", output)
	}
}

func TestProjectFlowPathNotFoundProjectPersists(t *testing.T) {
	runner := newMockRunner()
	runner.errors["test -d"] = fmt.Errorf("exit 1")

	projects := &config.ProjectsConfig{
		Projects: map[string]config.Project{
			"myapp": {Path: "/home/user/myapp"},
		},
	}

	// Select project 1, path not found, Confirm gets EOF → decline
	in := strings.NewReader("1\n")
	out := &bytes.Buffer{}

	err := ProjectFlow(in, out, runner, projects, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Project should still exist since confirmation wasn't provided
	if _, exists := projects.Projects["myapp"]; !exists {
		t.Error("expected project to still exist when confirmation not provided")
	}
}
