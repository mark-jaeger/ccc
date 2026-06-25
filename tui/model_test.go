package tui

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mark-jaeger/ccc/config"
	"github.com/mark-jaeger/ccc/zmx"
)

// exitError255 returns a real *exec.ExitError with code 255, used to simulate
// an ssh transport failure on interactive attach.
func exitError255(t *testing.T) error {
	t.Helper()
	err := exec.Command("sh", "-c", "exit 255").Run()
	var ee *exec.ExitError
	if !errors.As(err, &ee) || ee.ExitCode() != 255 {
		t.Fatalf("failed to construct exit-255 error, got %v", err)
	}
	return err
}

func TestNewModel(t *testing.T) {
	t.Run("remote mode", func(t *testing.T) {
		m := New(false)
		if m.isLocal {
			t.Error("expected isLocal=false")
		}
		if m.state != StateLoading {
			t.Errorf("expected StateLoading, got %v", m.state)
		}
	})

	t.Run("local mode", func(t *testing.T) {
		m := New(true)
		if !m.isLocal {
			t.Error("expected isLocal=true")
		}
	})
}

func TestWindowSizeMsg(t *testing.T) {
	m := New(false)
	// Initialize lists by setting hosts (simulates what happens when hostsLoadedMsg is received)
	m.width = 80
	m.height = 24
	hosts := []config.Host{{Name: "server1", Address: "192.168.1.1"}}
	m.SetHosts(hosts)

	msg := tea.WindowSizeMsg{Width: 100, Height: 30}

	newModel, _ := m.Update(msg)
	m = newModel.(Model)

	if m.width != 100 || m.height != 30 {
		t.Errorf("expected 100x30, got %dx%d", m.width, m.height)
	}
}

func TestBackNavigation(t *testing.T) {
	tests := []struct {
		name       string
		initial    State
		isLocal    bool
		expected   State
		shouldQuit bool
	}{
		{"host->quit", StateHostSelect, false, StateHostSelect, true},
		{"project->host", StateProjectSelect, false, StateHostSelect, false},
		{"session->project", StateSessionSelect, false, StateProjectSelect, false},
		{"project->quit (local)", StateProjectSelect, true, StateProjectSelect, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(tt.isLocal)
			m.state = tt.initial

			newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
			m = newModel.(Model)

			if tt.shouldQuit {
				if cmd == nil {
					t.Error("expected quit command")
				}
			} else {
				if m.state != tt.expected {
					t.Errorf("expected %v, got %v", tt.expected, m.state)
				}
			}
		})
	}
}

func TestHostsLoadedMsg(t *testing.T) {
	m := New(false)
	m.width = 80
	m.height = 24

	hosts := []config.Host{
		{Name: "server1", Address: "192.168.1.1"},
		{Name: "server2", Address: "192.168.1.2"},
	}

	newModel, _ := m.Update(hostsLoadedMsg{hosts: hosts})
	m = newModel.(Model)

	if m.state != StateHostSelect {
		t.Errorf("expected StateHostSelect, got %v", m.state)
	}
}

func TestProjectsLoadedMsg(t *testing.T) {
	m := New(false)
	m.width = 80
	m.height = 24

	projects := &config.ProjectsConfig{
		Projects: []config.Project{
			{Name: "proj1", Path: "/home/user/proj1"},
		},
	}

	newModel, _ := m.Update(projectsLoadedMsg{projects: projects})
	m = newModel.(Model)

	if m.state != StateProjectSelect {
		t.Errorf("expected StateProjectSelect, got %v", m.state)
	}
}

func TestSessionsLoadedMsg(t *testing.T) {
	m := New(false)
	m.width = 80
	m.height = 24
	m.currentProjectKey = "proj1"

	sessions := []zmx.Session{
		{Name: "ccc.proj1.main", Project: "proj1", Suffix: "main"},
	}

	newModel, _ := m.Update(sessionsLoadedMsg{sessions: sessions})
	m = newModel.(Model)

	if m.state != StateSessionSelect {
		t.Errorf("expected StateSessionSelect, got %v", m.state)
	}
}

func TestErrorMsg(t *testing.T) {
	m := New(false)
	testErr := fmt.Errorf("test error")

	newModel, _ := m.Update(errMsg{err: testErr})
	m = newModel.(Model)

	if m.state != StateError {
		t.Errorf("expected StateError, got %v", m.state)
	}
	if m.err != testErr {
		t.Error("error not set correctly")
	}
}

func TestBreadcrumb(t *testing.T) {
	m := New(false)

	// No selections
	if b := m.breadcrumb(); b != "" {
		t.Errorf("expected empty breadcrumb, got %q", b)
	}

	// Host selected
	m.selectedHost = "server1"
	if b := m.breadcrumb(); !strings.Contains(b, "server1") {
		t.Errorf("breadcrumb should contain host: %q", b)
	}

	// Host and project selected
	m.selectedProject = "myproj"
	if b := m.breadcrumb(); !strings.Contains(b, "server1") || !strings.Contains(b, "myproj") {
		t.Errorf("breadcrumb should contain both: %q", b)
	}
}

func TestNewSessionKeyTransition(t *testing.T) {
	m := New(true) // local mode
	m.state = StateSessionSelect
	m.currentProjectKey = "proj"
	m.width = 80
	m.height = 24

	// Initialize session list
	m.SetSessions([]zmx.Session{}, "proj")

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = newModel.(Model)

	if m.state != StateSessionNameInput {
		t.Errorf("expected StateSessionNameInput, got %v", m.state)
	}
	if cmd == nil {
		t.Error("expected textinput.Blink command")
	}
}

func TestSessionNameInputEscape(t *testing.T) {
	m := New(true)
	m.state = StateSessionNameInput
	m.currentProjectKey = "proj"

	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = newModel.(Model)

	if m.state != StateSessionSelect {
		t.Errorf("expected StateSessionSelect, got %v", m.state)
	}
	if m.sessionNameInputErr != "" {
		t.Errorf("expected no error on escape, got %q", m.sessionNameInputErr)
	}
}

func TestSessionNameInputValidation(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		existing    []zmx.Session
		wantState   State
		wantErrText string
	}{
		{
			name:      "valid suffix",
			input:     "dev",
			existing:  nil,
			wantState: StateCreatingSession,
		},
		{
			name:        "dots rejected",
			input:       "my.suffix",
			existing:    nil,
			wantState:   StateSessionNameInput,
			wantErrText: "dots",
		},
		{
			name:        "space rejected",
			input:       "my suffix",
			existing:    nil,
			wantState:   StateSessionNameInput,
			wantErrText: "whitespace",
		},
		{
			name:        "tab rejected",
			input:       "my\tsuffix",
			existing:    nil,
			wantState:   StateSessionNameInput,
			wantErrText: "whitespace",
		},
		{
			name:  "conflict rejected",
			input: "main",
			existing: []zmx.Session{
				{Name: "ccc.proj.main", Project: "proj", Suffix: "main"},
			},
			wantState:   StateSessionNameInput,
			wantErrText: "already exists",
		},
		{
			name:      "empty uses auto",
			input:     "",
			existing:  nil,
			wantState: StateCreatingSession,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(true) // local mode
			m.state = StateSessionNameInput
			m.currentProjectKey = "proj"
			m.currentProjectPath = "/home/user/proj"
			m.sessions = tt.existing

			// Set the input value
			m.sessionNameInput.SetValue(tt.input)

			// Press Enter
			newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			m = newModel.(Model)

			if m.state != tt.wantState {
				t.Errorf("expected state %v, got %v", tt.wantState, m.state)
			}

			if tt.wantErrText != "" {
				if !strings.Contains(m.sessionNameInputErr, tt.wantErrText) {
					t.Errorf("expected error containing %q, got %q", tt.wantErrText, m.sessionNameInputErr)
				}
			} else {
				if m.sessionNameInputErr != "" {
					t.Errorf("expected no error, got %q", m.sessionNameInputErr)
				}
			}
		})
	}
}

func TestReorderHostBoundaries(t *testing.T) {
	m := New(false)
	m.width = 80
	m.height = 24
	m.hosts = []config.Host{
		{Name: "a", Address: "1.1.1.1"},
		{Name: "b", Address: "2.2.2.2"},
	}
	m.SetHosts(m.hosts)
	m.state = StateHostSelect

	// Select first item and try to move up (boundary)
	m.hostList.Select(0)
	newModel, _ := m.reorderHost(-1)
	m = newModel.(Model)

	// Order should be unchanged
	if m.hosts[0].Name != "a" {
		t.Errorf("expected first host to be 'a', got %q", m.hosts[0].Name)
	}

	// Select last item and try to move down (boundary)
	m.hostList.Select(1)
	newModel, _ = m.reorderHost(1)
	m = newModel.(Model)

	// Order should be unchanged
	if m.hosts[1].Name != "b" {
		t.Errorf("expected second host to be 'b', got %q", m.hosts[1].Name)
	}
}

func TestReorderHostSuccess(t *testing.T) {
	m := New(false)
	m.width = 80
	m.height = 24
	m.hosts = []config.Host{
		{Name: "a", Address: "1.1.1.1"},
		{Name: "b", Address: "2.2.2.2"},
		{Name: "c", Address: "3.3.3.3"},
	}
	m.SetHosts(m.hosts)
	m.state = StateHostSelect

	// Select second item and move up
	m.hostList.Select(1)
	newModel, cmd := m.reorderHost(-1)
	m = newModel.(Model)

	// "b" should now be first
	if m.hosts[0].Name != "b" {
		t.Errorf("expected first host to be 'b', got %q", m.hosts[0].Name)
	}
	if m.hosts[1].Name != "a" {
		t.Errorf("expected second host to be 'a', got %q", m.hosts[1].Name)
	}

	// Cursor should follow the moved item
	if m.hostList.Index() != 0 {
		t.Errorf("expected cursor at 0, got %d", m.hostList.Index())
	}

	// Should trigger save command
	if cmd == nil {
		t.Error("expected save command")
	}
}

func TestReorderProjectBoundaries(t *testing.T) {
	m := New(true) // local mode
	m.width = 80
	m.height = 24
	m.projects = &config.ProjectsConfig{
		Projects: []config.Project{
			{Name: "a", Path: "/a"},
			{Name: "b", Path: "/b"},
		},
	}
	m.SetProjects(m.projects.Projects)
	m.state = StateProjectSelect

	// Select first item and try to move up (boundary)
	m.projectList.Select(0)
	newModel, _ := m.reorderProject(-1)
	m = newModel.(Model)

	// Order should be unchanged
	if m.projects.Projects[0].Name != "a" {
		t.Errorf("expected first project to be 'a', got %q", m.projects.Projects[0].Name)
	}

	// Select last item and try to move down (boundary)
	m.projectList.Select(1)
	newModel, _ = m.reorderProject(1)
	m = newModel.(Model)

	// Order should be unchanged
	if m.projects.Projects[1].Name != "b" {
		t.Errorf("expected second project to be 'b', got %q", m.projects.Projects[1].Name)
	}
}

func TestReorderProjectNilSafe(t *testing.T) {
	m := New(true)
	m.state = StateProjectSelect
	m.projects = nil

	// Should not panic
	newModel, cmd := m.reorderProject(-1)
	m = newModel.(Model)

	if cmd != nil {
		t.Error("expected nil command when projects is nil")
	}
}

// TestErrorRecovery verifies that pressing esc in StateError clears the error
// and returns to the first usable screen (1B).
func TestErrorRecovery(t *testing.T) {
	tests := []struct {
		name     string
		isLocal  bool
		expected State
	}{
		{"remote returns to host select", false, StateHostSelect},
		{"local returns to project select", true, StateProjectSelect},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(tt.isLocal)
			m.state = StateError
			m.err = fmt.Errorf("boom")

			newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
			m = newModel.(Model)

			if m.state != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, m.state)
			}
			if m.err != nil {
				t.Errorf("expected err cleared, got %v", m.err)
			}
		})
	}
}

// TestSessionExited255Reconnects verifies a 255 exit moves to StateReconnecting
// and increments the attempt counter rather than dumping to StateError (3A).
func TestSessionExited255Reconnects(t *testing.T) {
	m := New(true) // local mode
	m.state = StateSessionSelect
	m.currentProjectKey = "proj"
	m.lastSession = "ccc.proj.main"

	newModel, cmd := m.Update(sessionExitedMsg{err: exitError255(t)})
	m = newModel.(Model)

	if m.state != StateReconnecting {
		t.Errorf("expected StateReconnecting, got %v", m.state)
	}
	if m.reconnectAttempts != 1 {
		t.Errorf("expected reconnectAttempts=1, got %d", m.reconnectAttempts)
	}
	if cmd == nil {
		t.Error("expected a backoff tick command")
	}
}

// TestReconnectCancelAndExhaust verifies the bounded-reconnect lifecycle:
// reaching the cap transitions to StateConnectionLost, where r re-initiates an
// attach and esc returns to the session list (3A).
func TestReconnectCancelAndExhaust(t *testing.T) {
	m := New(true) // local mode
	m.state = StateSessionSelect
	m.currentProjectKey = "proj"
	m.lastSession = "ccc.proj.main"

	// First 255 exit -> reconnecting (attempt 1)
	nm, _ := m.Update(sessionExitedMsg{err: exitError255(t)})
	m = nm.(Model)
	if m.state != StateReconnecting || m.reconnectAttempts != 1 {
		t.Fatalf("after 1st exit: state=%v attempts=%d", m.state, m.reconnectAttempts)
	}

	// Second 255 exit -> reconnecting (attempt 2)
	nm, _ = m.Update(sessionExitedMsg{err: exitError255(t)})
	m = nm.(Model)
	if m.state != StateReconnecting || m.reconnectAttempts != 2 {
		t.Fatalf("after 2nd exit: state=%v attempts=%d", m.state, m.reconnectAttempts)
	}

	// Third 255 exit -> cap exceeded -> connection lost
	nm, _ = m.Update(sessionExitedMsg{err: exitError255(t)})
	m = nm.(Model)
	if m.state != StateConnectionLost {
		t.Fatalf("after cap exceeded: expected StateConnectionLost, got %v", m.state)
	}

	// Press r -> re-initiate attach (resets attempts, returns a cmd)
	nm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	rm := nm.(Model)
	if cmd == nil {
		t.Error("expected attach cmd on reconnect")
	}
	if rm.reconnectAttempts != 0 {
		t.Errorf("expected attempts reset to 0 on manual reconnect, got %d", rm.reconnectAttempts)
	}

	// From the connection-lost screen, esc returns to the session list.
	nm, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	em := nm.(Model)
	if em.state != StateSessionSelect {
		t.Errorf("expected esc to return to StateSessionSelect, got %v", em.state)
	}
}

// TestReconnectCanceledTickIgnored verifies that a backoff tick scheduled before
// the user cancels (esc) is ignored once it finally fires, instead of seizing
// the terminal with a re-attach.
func TestReconnectCanceledTickIgnored(t *testing.T) {
	m := New(true) // local mode
	m.state = StateSessionSelect
	m.currentProjectKey = "proj"
	m.lastSession = "ccc.proj.main"

	// Drop -> reconnecting; a backoff tick is scheduled for this epoch.
	nm, _ := m.Update(sessionExitedMsg{err: exitError255(t)})
	m = nm.(Model)
	if m.state != StateReconnecting {
		t.Fatalf("expected StateReconnecting, got %v", m.state)
	}
	staleGen := m.reconnectGen

	// User cancels with esc before the tick fires.
	nm, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = nm.(Model)
	if m.state != StateSessionSelect {
		t.Fatalf("expected esc to return to StateSessionSelect, got %v", m.state)
	}

	// The previously scheduled tick now arrives; it must be ignored.
	nm, cmd := m.Update(reconnectMsg{gen: staleGen})
	m = nm.(Model)
	if cmd != nil {
		t.Error("expected canceled reconnect tick to be ignored (nil cmd)")
	}
	if m.state != StateSessionSelect {
		t.Errorf("expected to remain in StateSessionSelect, got %v", m.state)
	}
}

// TestReconnectTickFires verifies that an in-epoch tick delivered while still
// reconnecting does re-fire the attach.
func TestReconnectTickFires(t *testing.T) {
	m := New(true) // local mode
	m.state = StateSessionSelect
	m.currentProjectKey = "proj"
	m.lastSession = "ccc.proj.main"

	nm, _ := m.Update(sessionExitedMsg{err: exitError255(t)})
	m = nm.(Model)
	if m.state != StateReconnecting {
		t.Fatalf("expected StateReconnecting, got %v", m.state)
	}

	nm, cmd := m.Update(reconnectMsg{gen: m.reconnectGen})
	m = nm.(Model)
	if cmd == nil {
		t.Error("expected matching in-epoch reconnect tick to fire an attach cmd")
	}
}

// TestCreateSessionTracksLastSession verifies the create-and-attach path records
// the new session as the reconnect target (and resets the attempt budget), so a
// transport drop on the first attach retries the right session rather than a
// stale one.
func TestCreateSessionTracksLastSession(t *testing.T) {
	m := New(true) // local mode
	m.state = StateSessionNameInput
	m.currentProjectKey = "proj"
	m.currentProjectPath = "/home/user/proj"
	// Simulate stale reconnect context from a prior attach.
	m.lastSession = "ccc.other.stale"
	m.reconnectAttempts = 1

	m.sessionNameInput.SetValue("dev")
	nm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm.(Model)

	if m.state != StateCreatingSession {
		t.Fatalf("expected StateCreatingSession, got %v", m.state)
	}
	if m.lastSession != "ccc.proj.dev" {
		t.Errorf("expected lastSession=ccc.proj.dev, got %q", m.lastSession)
	}
	if m.lastProjectPath != "/home/user/proj" {
		t.Errorf("expected lastProjectPath captured for reconnect, got %q", m.lastProjectPath)
	}
	if m.reconnectAttempts != 0 {
		t.Errorf("expected reconnectAttempts reset to 0, got %d", m.reconnectAttempts)
	}
	if cmd == nil {
		t.Error("expected a create-session command")
	}
}

// TestCreateDropRetainsProjectPath verifies that when a freshly created
// session's first attach drops with exit 255, the model enters reconnecting
// with the project directory retained, so the reconnect can recreate the
// session in the correct cwd instead of the login dir.
func TestCreateDropRetainsProjectPath(t *testing.T) {
	m := New(true) // local mode
	m.state = StateSessionNameInput
	m.currentProjectKey = "proj"
	m.currentProjectPath = "/home/user/proj"

	// Create the session via the name-input Enter path.
	m.sessionNameInput.SetValue("dev")
	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm.(Model)

	// The first attach drops before the session is established.
	nm, cmd := m.Update(sessionExitedMsg{err: exitError255(t)})
	m = nm.(Model)

	if m.state != StateReconnecting {
		t.Fatalf("expected StateReconnecting, got %v", m.state)
	}
	if m.lastSession != "ccc.proj.dev" {
		t.Errorf("expected lastSession=ccc.proj.dev, got %q", m.lastSession)
	}
	if m.lastProjectPath != "/home/user/proj" {
		t.Errorf("expected project dir retained for reconnect, got %q", m.lastProjectPath)
	}
	if cmd == nil {
		t.Error("expected a backoff tick command")
	}
}

// TestSessionExitedCleanResets verifies a clean exit refreshes sessions and
// resets the reconnect counter (3A).
func TestSessionExitedCleanResets(t *testing.T) {
	m := New(true) // local mode
	m.state = StateReconnecting
	m.currentProjectKey = "proj"
	m.reconnectAttempts = 2

	newModel, cmd := m.Update(sessionExitedMsg{err: nil})
	m = newModel.(Model)

	if m.reconnectAttempts != 0 {
		t.Errorf("expected reconnectAttempts reset to 0, got %d", m.reconnectAttempts)
	}
	if cmd == nil {
		t.Error("expected a session-refresh command")
	}
}

// fakeRunner is a Runner stub for exercising remote-mode reconnect probes and
// the cancelable load path. When blockOnCtx is set, RunContext blocks until the
// context is done and then returns its error, emulating an ssh child that only
// unblocks once its bounded context is cancelled or times out.
type fakeRunner struct {
	runErr     error
	blockOnCtx bool
}

func (f fakeRunner) Run(string) (string, error) { return "", f.runErr }
func (f fakeRunner) RunContext(ctx context.Context, _ string) (string, error) {
	if f.blockOnCtx {
		<-ctx.Done()
		return "", ctx.Err()
	}
	return "", f.runErr
}
func (f fakeRunner) RunInteractive(string) error { return nil }

// TestErrorRecoveryInitializesList verifies that recovering from StateError when
// the destination list was never built (e.g. a startup load failure) lands on a
// usable, non-panicking screen rather than a zero-value list.Model.
func TestErrorRecoveryInitializesList(t *testing.T) {
	tests := []struct {
		name    string
		isLocal bool
		want    State
	}{
		{"remote", false, StateHostSelect},
		{"local", true, StateProjectSelect},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(tt.isLocal)
			m.state = StateError
			m.err = fmt.Errorf("startup load failed")
			// Lists are deliberately left at their zero value: the load failed
			// before SetHosts/SetProjects ran.

			nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
			m = nm.(Model)

			if m.state != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, m.state)
			}
			// Rendering and a follow-up key update must not panic on an
			// uninitialized list (nil delegate).
			_ = m.View()
			nm, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
			_ = nm.(Model).View()
		})
	}
}

// TestReconnectRemoteProbesBeforeAttach verifies that an in-epoch reconnect tick
// in remote mode kicks off a non-interactive reachability probe rather than
// firing the interactive attach directly.
func TestReconnectRemoteProbesBeforeAttach(t *testing.T) {
	m := New(false) // remote mode
	m.state = StateReconnecting
	m.runner = fakeRunner{}
	m.lastSession = "ccc.proj.main"

	nm, cmd := m.Update(reconnectMsg{gen: m.reconnectGen})
	m = nm.(Model)

	if cmd == nil {
		t.Fatal("expected a probe command before the interactive attach")
	}
	if m.state != StateReconnecting {
		t.Errorf("expected to remain StateReconnecting during probe, got %v", m.state)
	}
}

// TestReconnectProbeUnreachable verifies that a failed reachability probe hands
// off to manual recovery instead of launching a blocking interactive ssh.
func TestReconnectProbeUnreachable(t *testing.T) {
	m := New(false) // remote mode
	m.state = StateReconnecting
	m.runner = fakeRunner{runErr: fmt.Errorf("ssh: connect to host failed")}
	m.lastSession = "ccc.proj.main"

	nm, _ := m.Update(reconnectProbeMsg{gen: m.reconnectGen, ok: false})
	m = nm.(Model)

	if m.state != StateConnectionLost {
		t.Errorf("expected StateConnectionLost after failed probe, got %v", m.state)
	}
}

// TestReconnectProbeReachable verifies a successful probe proceeds to the
// interactive attach while staying in StateReconnecting.
func TestReconnectProbeReachable(t *testing.T) {
	m := New(false) // remote mode
	m.state = StateReconnecting
	m.runner = fakeRunner{}
	m.lastSession = "ccc.proj.main"
	m.currentHost = &config.Host{Name: "h", Address: "1.2.3.4"}

	nm, cmd := m.Update(reconnectProbeMsg{gen: m.reconnectGen, ok: true})
	m = nm.(Model)

	if cmd == nil {
		t.Error("expected an attach command after a successful probe")
	}
	if m.state != StateReconnecting {
		t.Errorf("expected to remain StateReconnecting until attach, got %v", m.state)
	}
}

// TestReconnectProbeStaleIgnored verifies a probe result from a superseded epoch
// (e.g. after the user canceled) is ignored.
func TestReconnectProbeStaleIgnored(t *testing.T) {
	m := New(false) // remote mode
	m.state = StateReconnecting
	m.runner = fakeRunner{}
	staleGen := m.reconnectGen
	m.reconnectGen++ // a cancel/navigation bumped the epoch

	nm, cmd := m.Update(reconnectProbeMsg{gen: staleGen, ok: true})
	m = nm.(Model)

	if cmd != nil {
		t.Error("expected a stale probe result to be ignored (nil cmd)")
	}
}

// TestEscCancelsConnecting verifies that esc on the connecting screen cancels the
// in-flight connect (so a dead network unblocks instead of hanging), bumps the
// request generation, and returns to a usable host-selection screen.
func TestEscCancelsConnecting(t *testing.T) {
	m := New(false) // remote mode
	m.width, m.height = 80, 24
	m.hosts = []config.Host{{Name: "h", Address: "1.2.3.4"}}
	m.state = StateConnecting
	canceled := false
	m.cancel = func() { canceled = true }
	prevGen := m.reqGen

	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = nm.(Model)

	if !canceled {
		t.Error("expected the in-flight connect context to be canceled")
	}
	if m.state != StateHostSelect {
		t.Errorf("expected StateHostSelect, got %v", m.state)
	}
	if m.reqGen != prevGen+1 {
		t.Errorf("expected reqGen bumped to %d, got %d", prevGen+1, m.reqGen)
	}
	// The destination list must be (re)initialized so rendering/updates don't
	// panic on a zero-value list.Model.
	_ = m.View()
}

// TestEscCancelsLoading verifies that esc on the loading screen cancels the
// in-flight load, bumps the request generation, and steps back to project
// selection (projects already loaded).
func TestEscCancelsLoading(t *testing.T) {
	m := New(false) // remote mode
	m.width, m.height = 80, 24
	m.projects = &config.ProjectsConfig{Projects: []config.Project{{Name: "p", Path: "/p"}}}
	m.state = StateLoading
	canceled := false
	m.cancel = func() { canceled = true }
	prevGen := m.reqGen

	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = nm.(Model)

	if !canceled {
		t.Error("expected the in-flight load context to be canceled")
	}
	if m.state != StateProjectSelect {
		t.Errorf("expected StateProjectSelect, got %v", m.state)
	}
	if m.reqGen != prevGen+1 {
		t.Errorf("expected reqGen bumped to %d, got %d", prevGen+1, m.reqGen)
	}
	_ = m.View()
}

// TestStaleHostConnectedIgnored verifies a hostConnectedMsg from a superseded
// request generation is dropped (runner/state untouched), while a current-gen
// one is applied.
func TestStaleHostConnectedIgnored(t *testing.T) {
	m := New(false)
	m.width, m.height = 80, 24
	m.reqGen = 3
	m.state = StateConnecting

	// Stale: gen does not match reqGen.
	nm, _ := m.Update(hostConnectedMsg{hostName: "h", host: config.Host{Name: "h"}, runner: fakeRunner{}, gen: 2})
	m = nm.(Model)
	if m.runner != nil {
		t.Error("stale hostConnectedMsg must not set the runner")
	}
	if m.state != StateConnecting {
		t.Errorf("stale hostConnectedMsg must not change state, got %v", m.state)
	}

	// Current: gen matches reqGen.
	nm, _ = m.Update(hostConnectedMsg{hostName: "h", host: config.Host{Name: "h"}, runner: fakeRunner{}, gen: 3})
	m = nm.(Model)
	if m.runner == nil {
		t.Error("current hostConnectedMsg should set the runner")
	}
	if m.state != StateLoading {
		t.Errorf("current hostConnectedMsg should advance to StateLoading, got %v", m.state)
	}
}

// TestStaleProjectsLoadedIgnored verifies a projectsLoadedMsg from a superseded
// generation is dropped while a current-gen one is applied.
func TestStaleProjectsLoadedIgnored(t *testing.T) {
	m := New(false)
	m.width, m.height = 80, 24
	m.reqGen = 5
	m.state = StateLoading

	stale := &config.ProjectsConfig{Projects: []config.Project{{Name: "stale"}}}
	nm, _ := m.Update(projectsLoadedMsg{projects: stale, gen: 4})
	m = nm.(Model)
	if m.state != StateLoading {
		t.Errorf("stale projectsLoadedMsg must not change state, got %v", m.state)
	}
	if m.projects != nil {
		t.Error("stale projectsLoadedMsg must not set projects")
	}

	current := &config.ProjectsConfig{Projects: []config.Project{{Name: "p"}}}
	nm, _ = m.Update(projectsLoadedMsg{projects: current, gen: 5})
	m = nm.(Model)
	if m.state != StateProjectSelect {
		t.Errorf("current projectsLoadedMsg should advance to StateProjectSelect, got %v", m.state)
	}
	if m.projects != current {
		t.Error("current projectsLoadedMsg should set projects")
	}
}

// TestStaleSessionsLoadedIgnored verifies a sessionsLoadedMsg from a superseded
// generation is dropped while a current-gen one is applied.
func TestStaleSessionsLoadedIgnored(t *testing.T) {
	m := New(false)
	m.width, m.height = 80, 24
	m.currentProjectKey = "proj"
	m.reqGen = 9
	m.state = StateLoading

	nm, _ := m.Update(sessionsLoadedMsg{sessions: []zmx.Session{{Name: "ccc.proj.x"}}, gen: 8})
	m = nm.(Model)
	if m.state != StateLoading {
		t.Errorf("stale sessionsLoadedMsg must not change state, got %v", m.state)
	}
	if m.sessions != nil {
		t.Error("stale sessionsLoadedMsg must not set sessions")
	}

	nm, _ = m.Update(sessionsLoadedMsg{sessions: []zmx.Session{{Name: "ccc.proj.main"}}, gen: 9})
	m = nm.(Model)
	if m.state != StateSessionSelect {
		t.Errorf("current sessionsLoadedMsg should advance to StateSessionSelect, got %v", m.state)
	}
}

// TestStaleErrIgnored verifies an errMsg from a superseded request generation is
// dropped instead of clobbering the screen with StateError.
func TestStaleErrIgnored(t *testing.T) {
	m := New(false)
	m.width, m.height = 80, 24
	m.reqGen = 4
	m.state = StateLoading

	nm, _ := m.Update(errMsg{err: fmt.Errorf("late failure"), gen: 3})
	m = nm.(Model)
	if m.state == StateError {
		t.Error("stale errMsg must not switch to StateError")
	}
	if m.err != nil {
		t.Errorf("stale errMsg must not set err, got %v", m.err)
	}
}
