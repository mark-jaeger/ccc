package flow

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/markjd/ccc/config"
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

	err := ProjectFlow(in, out, runner, projects, nil)
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

	err := ProjectFlow(in, out, runner, projects, nil)
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

	err := ProjectFlow(in, out, runner, projects, nil)
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

	err := ProjectFlow(in, out, runner, projects, nil)
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

	err := ProjectFlow(in, out, runner, projects, nil)
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
