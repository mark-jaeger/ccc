package flow

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/mark-jaeger/ccc/abduco"
	"github.com/mark-jaeger/ccc/config"
	"github.com/mark-jaeger/ccc/ui"
)

// mockRunner implements the Runner interface for testing.
type mockRunner struct {
	responses   map[string]string
	errors      map[string]error
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
	// abduco check
	runner.responses["command -v abduco"] = "/usr/bin/abduco"
	// abduco list returns empty (no sessions)
	runner.responses["abduco 2>&1"] = ""
	// abduco create session
	runner.responses["abduco -n"] = ""

	projects := &config.ProjectsConfig{
		Projects: map[string]config.Project{
			"myapp": {Path: "/home/user/myapp"},
		},
	}

	// Input: select project 1 (no session name prompt - auto-naming)
	in := strings.NewReader("1\n")
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
	runner.responses["command -v abduco"] = "/usr/bin/abduco"
	runner.responses["abduco 2>&1"] = ""
	runner.responses["abduco -n"] = ""

	// No user input needed - auto-naming
	in := strings.NewReader("")
	out := &bytes.Buffer{}

	err := SessionFlow(in, out, runner, "myapp", "/home/user/myapp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "Created session ccc.myapp.main") {
		t.Errorf("expected session creation message with ccc.myapp.main, got: %s", out.String())
	}

	// Should have called RunInteractive for attach
	if len(runner.interactive) == 0 {
		t.Error("expected RunInteractive to be called for attach")
	}
}

func TestSessionFlowOneSessionShowsMenu(t *testing.T) {
	runner := newMockRunner()
	runner.responses["command -v abduco"] = "/usr/bin/abduco"
	// One session in abduco format (status char + tab + day + date + time + tab + PID + tab + name)
	runner.responses["abduco 2>&1"] = " \tThu 2024-01-01 12:00:00\t12345\tccc.myapp.main"

	// Select session 1 from menu
	in := strings.NewReader("1\n")
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

func TestKillSessionConfirmed(t *testing.T) {
	runner := newMockRunner()
	runner.responses["kill "] = ""

	sessions := []abduco.Session{
		{Name: "ccc.myapp.main", Project: "myapp", Suffix: "main", PID: 12345},
	}
	item := ui.MenuItem{Key: "ccc.myapp.main", Label: "ccc.myapp.main"}

	// Confirm kill
	in := strings.NewReader("y\n")
	out := &bytes.Buffer{}

	err := killSession(in, out, runner, item, sessions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Killed session ccc.myapp.main") {
		t.Errorf("expected kill message, got: %s", output)
	}
}

func TestKillSessionExternalWarning(t *testing.T) {
	runner := newMockRunner()
	runner.responses["kill "] = ""

	sessions := []abduco.Session{
		{Name: "myapp-extra", External: true, PID: 12346},
	}
	item := ui.MenuItem{Key: "myapp-extra", Label: "myapp-extra"}

	// Confirm kill of external session
	in := strings.NewReader("y\n")
	out := &bytes.Buffer{}

	err := killSession(in, out, runner, item, sessions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "wasn't created by ccc") {
		t.Errorf("expected external warning, got: %s", output)
	}
	if !strings.Contains(output, "Killed session") {
		t.Errorf("expected kill message, got: %s", output)
	}
}

func TestKillSessionDeclined(t *testing.T) {
	runner := newMockRunner()
	// Note: no kill response needed - command shouldn't be called

	sessions := []abduco.Session{
		{Name: "ccc.myapp.main", Project: "myapp", Suffix: "main", PID: 12345},
	}
	item := ui.MenuItem{Key: "ccc.myapp.main", Label: "ccc.myapp.main"}

	// Decline kill
	in := strings.NewReader("n\n")
	out := &bytes.Buffer{}

	err := killSession(in, out, runner, item, sessions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := out.String()
	if strings.Contains(output, "Killed") {
		t.Errorf("should not show kill message when declined, got: %s", output)
	}
}

func TestAttachSessionNoClients(t *testing.T) {
	runner := newMockRunner()
	runner.responses["command -v abduco"] = "/usr/bin/abduco"
	runner.responses["abduco 2>&1"] = " \tThu 2024-01-01 12:00:00\t12345\tccc.myapp.main"

	// Select session 1 from menu
	in := strings.NewReader("1\n")
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
		if strings.Contains(cmd, "abduco -a") {
			found = true
		}
	}
	if !found {
		t.Error("expected abduco -a in interactive commands")
	}
}

func TestAttachSessionExternalDeclined(t *testing.T) {
	runner := newMockRunner()
	runner.responses["command -v abduco"] = "/usr/bin/abduco"
	// One external session (no ccc. prefix)
	runner.responses["abduco 2>&1"] = " \tThu 2024-01-01 12:00:00\t12347\tmyapp"

	// Select session 1, then decline external prompt
	in := strings.NewReader("1\nn\n")
	out := &bytes.Buffer{}

	err := SessionFlow(in, out, runner, "myapp", "/home/user/myapp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "external session") {
		t.Errorf("expected external warning, got: %s", out.String())
	}
	if len(runner.interactive) != 0 {
		t.Error("expected no RunInteractive when user declines")
	}
}

func TestAttachSessionDead(t *testing.T) {
	runner := newMockRunner()
	runner.responses["command -v abduco"] = "/usr/bin/abduco"
	// One dead session (+ status)
	runner.responses["abduco 2>&1"] = "+\tThu 2024-01-01 12:00:00\t12345\tccc.myapp.main"

	// Select session 1
	in := strings.NewReader("1\n")
	out := &bytes.Buffer{}

	err := SessionFlow(in, out, runner, "myapp", "/home/user/myapp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "is dead") {
		t.Errorf("expected dead session message, got: %s", out.String())
	}
	if len(runner.interactive) != 0 {
		t.Error("expected no RunInteractive for dead session")
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

	// Select project 1, path not found, Confirm gets EOF -> decline
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

func TestDeleteProjectConfirmed(t *testing.T) {
	saveCalled := false
	var savedConfig *config.ProjectsConfig

	projects := &config.ProjectsConfig{
		Projects: map[string]config.Project{
			"myapp": {Path: "/home/user/myapp"},
			"other": {Path: "/home/user/other"},
		},
	}

	onSave := func(cfg *config.ProjectsConfig) error {
		saveCalled = true
		savedConfig = cfg
		return nil
	}

	item := ui.MenuItem{Key: "myapp", Label: "myapp"}

	// Confirm deletion
	in := strings.NewReader("y\n")
	out := &bytes.Buffer{}

	err := deleteProject(in, out, projects, item, onSave)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Deleted myapp") {
		t.Errorf("expected delete success message, got: %s", output)
	}

	if !saveCalled {
		t.Error("expected onSave to be called")
	}

	if len(savedConfig.Projects) != 1 {
		t.Errorf("expected 1 project remaining, got %d", len(savedConfig.Projects))
	}

	if _, exists := savedConfig.Projects["myapp"]; exists {
		t.Error("expected myapp to be deleted")
	}
}

func TestDeleteProjectDeclined(t *testing.T) {
	projects := &config.ProjectsConfig{
		Projects: map[string]config.Project{
			"myapp": {Path: "/home/user/myapp"},
		},
	}

	item := ui.MenuItem{Key: "myapp", Label: "myapp"}

	// Decline deletion
	in := strings.NewReader("n\n")
	out := &bytes.Buffer{}

	err := deleteProject(in, out, projects, item, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Project should still exist
	if _, exists := projects.Projects["myapp"]; !exists {
		t.Error("expected project to still exist after declining delete")
	}

	// Should not show success message
	output := out.String()
	if strings.Contains(output, "Deleted") {
		t.Errorf("should not show delete message when declined, got: %s", output)
	}
}

func TestDeleteProjectNoOnSave(t *testing.T) {
	projects := &config.ProjectsConfig{
		Projects: map[string]config.Project{
			"myapp": {Path: "/home/user/myapp"},
		},
	}

	item := ui.MenuItem{Key: "myapp", Label: "myapp"}

	// Confirm deletion with nil onSave
	in := strings.NewReader("y\n")
	out := &bytes.Buffer{}

	err := deleteProject(in, out, projects, item, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Project should be deleted from map
	if _, exists := projects.Projects["myapp"]; exists {
		t.Error("expected project to be deleted")
	}

	output := out.String()
	if !strings.Contains(output, "Deleted myapp") {
		t.Errorf("expected delete success message, got: %s", output)
	}
}

func TestDeleteProjectSaveError(t *testing.T) {
	projects := &config.ProjectsConfig{
		Projects: map[string]config.Project{
			"myapp": {Path: "/home/user/myapp"},
		},
	}

	onSave := func(cfg *config.ProjectsConfig) error {
		return fmt.Errorf("disk full")
	}

	item := ui.MenuItem{Key: "myapp", Label: "myapp"}

	// Confirm deletion but save fails
	in := strings.NewReader("y\n")
	out := &bytes.Buffer{}

	err := deleteProject(in, out, projects, item, onSave)
	if err == nil {
		t.Fatal("expected error when save fails")
	}
	if !strings.Contains(err.Error(), "failed to delete project") {
		t.Errorf("expected wrapped error, got: %v", err)
	}

	// Project should be rolled back (still exists)
	if _, exists := projects.Projects["myapp"]; !exists {
		t.Error("expected project to be rolled back after save failure")
	}

	// Should not show success message
	output := out.String()
	if strings.Contains(output, "Deleted") {
		t.Errorf("should not show delete message on save failure, got: %s", output)
	}
}

func TestSessionFlowMultipleSessions(t *testing.T) {
	runner := newMockRunner()
	runner.responses["command -v abduco"] = "/usr/bin/abduco"
	// Two sessions for the project
	runner.responses["abduco 2>&1"] = " \tThu 2024-01-01 12:00:00\t12345\tccc.myapp.main\n \tThu 2024-01-01 12:01:00\t12346\tccc.myapp.2"

	// Select session 2 from menu
	in := strings.NewReader("2\n")
	out := &bytes.Buffer{}

	err := SessionFlow(in, out, runner, "myapp", "/home/user/myapp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "Attaching") {
		t.Errorf("expected attach message, got: %s", out.String())
	}

	// Verify correct session was attached
	found := false
	for _, cmd := range runner.interactive {
		if strings.Contains(cmd, "ccc.myapp.2") {
			found = true
		}
	}
	if !found {
		t.Error("expected to attach to ccc.myapp.2")
	}
}

func TestSessionFlowNewSession(t *testing.T) {
	runner := newMockRunner()
	runner.responses["command -v abduco"] = "/usr/bin/abduco"
	// One existing session
	runner.responses["abduco 2>&1"] = " \tThu 2024-01-01 12:00:00\t12345\tccc.myapp.main"
	runner.responses["abduco -n"] = ""

	// Select 'n' for new session
	in := strings.NewReader("n\n")
	out := &bytes.Buffer{}

	err := SessionFlow(in, out, runner, "myapp", "/home/user/myapp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Since main exists, next should be "2"
	if !strings.Contains(out.String(), "Created session ccc.myapp.2") {
		t.Errorf("expected session creation message with ccc.myapp.2, got: %s", out.String())
	}
}
