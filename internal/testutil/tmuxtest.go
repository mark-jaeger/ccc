//go:build integration

package testutil

import (
	"fmt"
	"math/rand"
	"os/exec"
	"strings"
	"testing"

	"github.com/mark-jaeger/ccc/tmux"
)

// TestTmux manages an isolated tmux server for integration testing.
// Each instance uses a unique socket so tests can run in parallel.
type TestTmux struct {
	Socket string // unique socket name for -L flag
}

// NewTestTmux creates a new isolated tmux server.
// It skips the test if tmux is not installed, starts a detached server,
// and registers cleanup to kill the server when the test finishes.
func NewTestTmux(t *testing.T) *TestTmux {
	t.Helper()

	// Skip if tmux not installed
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed, skipping integration test")
	}

	// Generate unique socket name.
	// Truncate the test name portion to avoid exceeding the Unix socket path
	// length limit (~104 chars). The socket path is /tmp/tmux-<uid>/<socket>.
	name := sanitize(t.Name())
	const maxNameLen = 30
	if len(name) > maxNameLen {
		name = name[:maxNameLen]
	}
	socket := fmt.Sprintf("ccc-test-%s-%d", name, rand.Int63())

	tt := &TestTmux{Socket: socket}

	// Start a detached session so the server is running
	// (tmux server only exists while sessions exist)
	cmd := exec.Command("tmux", "-L", socket, "new-session", "-d", "-s", "_init")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to start tmux server: %v: %s", err, out)
	}

	t.Cleanup(func() {
		exec.Command("tmux", "-L", socket, "kill-server").Run()
	})

	return tt
}

// Run executes a shell command against this tmux server.
// It injects -L <socket> into any tmux command found in the string.
// Non-tmux commands are executed as-is via bash -c.
func (tt *TestTmux) Run(cmd string) (string, error) {
	adjusted := tt.injectSocket(cmd)
	out, err := exec.Command("bash", "-c", adjusted).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("command %q failed: %v: %s", adjusted, err, out)
	}
	return strings.TrimSpace(string(out)), nil
}

// RunInteractive records the command but does not execute it.
// Flow integration tests use this to verify the right attach command
// is called without blocking on a PTY.
func (tt *TestTmux) RunInteractive(cmd string) error {
	// No-op for integration tests — e2e layer tests real attach.
	return nil
}

// CreateSession creates a bare detached session with no ccc metadata.
// Useful for testing retroactive option application on pre-existing sessions.
func (tt *TestTmux) CreateSession(t *testing.T, name string) {
	t.Helper()
	cmd := exec.Command("tmux", "-L", tt.Socket, "new-session", "-d", "-s", name)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to create session %q: %v: %s", name, err, out)
	}
}

// GetOption returns a session-level option value (e.g., "bell-action").
func (tt *TestTmux) GetOption(t *testing.T, session, key string) string {
	t.Helper()
	out, err := exec.Command("tmux", "-L", tt.Socket, "show-options", "-t", session, "-v", key).CombinedOutput()
	if err != nil {
		t.Fatalf("failed to get option %q for session %q: %v: %s", key, session, err, out)
	}
	return strings.TrimSpace(string(out))
}

// GetWindowOption returns a window-level option value (e.g., "visual-bell", "allow-passthrough").
func (tt *TestTmux) GetWindowOption(t *testing.T, session, key string) string {
	t.Helper()
	out, err := exec.Command("tmux", "-L", tt.Socket, "show-window-options", "-t", session, "-v", key).CombinedOutput()
	if err != nil {
		t.Fatalf("failed to get window option %q for session %q: %v: %s", key, session, err, out)
	}
	return strings.TrimSpace(string(out))
}

// SessionExists checks if a session with the given name exists.
func (tt *TestTmux) SessionExists(t *testing.T, name string) bool {
	t.Helper()
	err := exec.Command("tmux", "-L", tt.Socket, "has-session", "-t", name).Run()
	return err == nil
}

// ListSessions returns the names of all sessions on this server.
func (tt *TestTmux) ListSessions(t *testing.T) []string {
	t.Helper()
	out, err := exec.Command("tmux", "-L", tt.Socket, "list-sessions", "-F", "#{session_name}").CombinedOutput()
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var names []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			names = append(names, l)
		}
	}
	return names
}

// SetSocketOverride sets tmux.SocketOverride for the duration of a test.
// This is used by e2e tests where the ccc binary reads the override.
// For integration tests that call Build*Command directly, prefer using
// TestTmux.Run which injects the socket via string replacement.
func (tt *TestTmux) SetSocketOverride(t *testing.T) {
	t.Helper()
	tmux.SocketOverride = tt.Socket
	t.Cleanup(func() { tmux.SocketOverride = "" })
}

// injectSocket replaces "tmux " with "tmux -L <socket> " in the command string.
// Handles compound commands (semicolons) and multiple tmux invocations.
func (tt *TestTmux) injectSocket(cmd string) string {
	return strings.ReplaceAll(cmd, "tmux ", fmt.Sprintf("tmux -L %s ", tt.Socket))
}

// sanitize replaces characters that aren't safe in tmux socket names.
func sanitize(s string) string {
	r := strings.NewReplacer("/", "-", " ", "-", ":", "-")
	return r.Replace(s)
}
