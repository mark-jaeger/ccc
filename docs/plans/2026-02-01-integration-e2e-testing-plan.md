# Integration & E2E Testing Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add real-tmux integration tests and thin PTY e2e tests so features like notification passthrough can be verified against actual tmux behavior.

**Architecture:** Two-layer test infrastructure: (1) `internal/testutil/` provides an isolated tmux server harness (`TestTmux`) and lightweight PTY expect helpers, (2) integration tests in `tmux/` and `flow/` packages use `TestTmux` as a real `Runner`, (3) e2e tests at the repo root launch the full ccc binary via PTY. All gated behind `//go:build integration`.

**Tech Stack:** Go stdlib `os/exec` for tmux harness, `github.com/creack/pty` for PTY allocation, tmux `-L` socket flag for test isolation.

---

### Task 1: Add `creack/pty` dependency

**Files:**
- Modify: `go.mod`

**Step 1: Add the dependency**

Run:
```bash
cd /Users/mark/Projects/jd/ccc/.worktrees/integration-testing && go get github.com/creack/pty@latest
```

**Step 2: Verify it installed**

Run:
```bash
cd /Users/mark/Projects/jd/ccc/.worktrees/integration-testing && grep creack go.mod
```
Expected: Line containing `github.com/creack/pty`

**Step 3: Verify existing tests still pass**

Run:
```bash
cd /Users/mark/Projects/jd/ccc/.worktrees/integration-testing && go test ./...
```
Expected: All packages PASS

**Step 4: Commit**

```bash
cd /Users/mark/Projects/jd/ccc/.worktrees/integration-testing && git add go.mod go.sum && git commit -m "Add creack/pty dependency for integration tests"
```

---

### Task 2: Create TestTmux harness

**Files:**
- Create: `internal/testutil/tmuxtest.go`

This is the foundational test helper. Every integration test will use it.

**Step 1: Write the harness**

Create `internal/testutil/tmuxtest.go`:

```go
//go:build integration

package testutil

import (
	"fmt"
	"math/rand"
	"os/exec"
	"strings"
	"testing"
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

	// Generate unique socket name
	socket := fmt.Sprintf("ccc-test-%s-%d", sanitize(t.Name()), rand.Int63())

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
	// Inject socket into tmux commands
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
		// No sessions = expected in some cases
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
```

**Step 2: Verify it compiles**

Run:
```bash
cd /Users/mark/Projects/jd/ccc/.worktrees/integration-testing && go build -tags=integration ./internal/testutil/
```
Expected: No errors

**Step 3: Commit**

```bash
cd /Users/mark/Projects/jd/ccc/.worktrees/integration-testing && git add internal/testutil/tmuxtest.go && git commit -m "Add TestTmux harness for integration tests"
```

---

### Task 3: Tmux integration tests — session creation and metadata

**Files:**
- Create: `tmux/sessions_integration_test.go`

**Step 1: Write the first two integration tests**

Create `tmux/sessions_integration_test.go`:

```go
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

	// Verify ccc metadata tags are set on the real session
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
```

**Step 2: Run the tests**

Run:
```bash
cd /Users/mark/Projects/jd/ccc/.worktrees/integration-testing && go test -tags=integration -run 'TestCreateSession' -v ./tmux/
```
Expected: Both tests PASS

**Step 3: Commit**

```bash
cd /Users/mark/Projects/jd/ccc/.worktrees/integration-testing && git add tmux/sessions_integration_test.go && git commit -m "Add integration tests for session creation metadata and bell options"
```

---

### Task 4: Tmux integration tests — passthrough and EnsureNotifyOptions

**Files:**
- Modify: `tmux/sessions_integration_test.go`

**Step 1: Add passthrough and EnsureNotifyOptions tests**

Append to `tmux/sessions_integration_test.go`:

```go
func TestSetPassthrough_EnablesOption(t *testing.T) {
	t.Parallel()
	tt := testutil.NewTestTmux(t)

	// Create a bare session (no ccc options)
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

	// Create a bare session — simulates a session from an older ccc version
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

	// Run twice — should not error
	if _, err := tt.Run(cmd); err != nil {
		t.Fatalf("first ensure notify options failed: %v", err)
	}
	if _, err := tt.Run(cmd); err != nil {
		t.Fatalf("second ensure notify options failed: %v", err)
	}

	// Options should still be correct after second run
	bellAction := tt.GetOption(t, "myapp", "bell-action")
	if bellAction != "any" {
		t.Errorf("bell-action = %q, want %q", bellAction, "any")
	}

	passthrough := tt.GetWindowOption(t, "myapp", "allow-passthrough")
	if passthrough != "on" {
		t.Errorf("allow-passthrough = %q, want %q", passthrough, "on")
	}
}
```

**Step 2: Run the new tests**

Run:
```bash
cd /Users/mark/Projects/jd/ccc/.worktrees/integration-testing && go test -tags=integration -run 'TestSetPassthrough|TestEnsureNotifyOptions' -v ./tmux/
```
Expected: All 3 tests PASS

**Step 3: Commit**

```bash
cd /Users/mark/Projects/jd/ccc/.worktrees/integration-testing && git add tmux/sessions_integration_test.go && git commit -m "Add integration tests for passthrough and EnsureNotifyOptions"
```

---

### Task 5: Tmux integration test — FilterSessionsForProject with real tmux

**Files:**
- Modify: `tmux/sessions_integration_test.go`

**Step 1: Add real-tmux session filtering test**

Append to `tmux/sessions_integration_test.go`:

```go
func TestFilterSessionsForProject_RealTmux(t *testing.T) {
	t.Parallel()
	tt := testutil.NewTestTmux(t)

	// Create sessions with different metadata via ccc's BuildCreateCommand
	cmd1 := tmux.BuildCreateCommand("proj", "/tmp/proj", "proj")
	if _, err := tt.Run(cmd1); err != nil {
		t.Fatalf("create proj failed: %v", err)
	}
	cmd2 := tmux.BuildCreateCommand("proj-2", "/tmp/proj", "proj")
	if _, err := tt.Run(cmd2); err != nil {
		t.Fatalf("create proj-2 failed: %v", err)
	}
	// Create a session for a different project
	cmd3 := tmux.BuildCreateCommand("other", "/tmp/other", "other")
	if _, err := tt.Run(cmd3); err != nil {
		t.Fatalf("create other failed: %v", err)
	}
	// Create an untagged session that matches by prefix
	tt.CreateSession(t, "proj-legacy")

	// List sessions using ccc's own list command
	listCmd := tmux.BuildListCommand()
	output, err := tt.Run(listCmd)
	if err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	allSessions := tmux.ParseSessionList(output)
	filtered := tmux.FilterSessionsForProject(allSessions, "proj")

	// Should include: proj (verified), proj-2 (verified), proj-legacy (unverified)
	// Should NOT include: other, _init (the harness init session)
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
```

**Step 2: Run the test**

Run:
```bash
cd /Users/mark/Projects/jd/ccc/.worktrees/integration-testing && go test -tags=integration -run 'TestFilterSessionsForProject_RealTmux' -v ./tmux/
```
Expected: PASS

**Step 3: Run ALL tmux integration tests together**

Run:
```bash
cd /Users/mark/Projects/jd/ccc/.worktrees/integration-testing && go test -tags=integration -v ./tmux/
```
Expected: All tests PASS (both unit and integration)

**Step 4: Commit**

```bash
cd /Users/mark/Projects/jd/ccc/.worktrees/integration-testing && git add tmux/sessions_integration_test.go && git commit -m "Add integration test for session filtering with real tmux"
```

---

### Task 6: Flow integration tests

**Files:**
- Create: `flow/flow_integration_test.go`

These tests verify the orchestration in `flow/common.go` — that `SessionFlow` calls the right tmux commands in the right order and leaves tmux in the correct state.

Important context for implementor: `flow.SessionFlow` takes `io.Reader` for menu input and `io.Writer` for output. Menu items are selected by typing their number. The session name prompt accepts enter for default. The `Runner` interface's `Run` method is used for all non-interactive tmux commands; `RunInteractive` is called for `tmux attach`. The `TestTmux.RunInteractive` is a no-op so these tests don't block.

**Step 1: Write flow integration tests**

Create `flow/flow_integration_test.go`:

```go
//go:build integration

package flow_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mark-jaeger/ccc/config"
	"github.com/mark-jaeger/ccc/flow"
	"github.com/mark-jaeger/ccc/internal/testutil"
	"github.com/mark-jaeger/ccc/tmux"
)

func TestProjectFlow_CreateNewSession(t *testing.T) {
	t.Parallel()
	tt := testutil.NewTestTmux(t)

	projects := &config.ProjectsConfig{
		Projects: map[string]config.Project{
			"myapp": {Path: "/tmp"},
		},
	}

	// Input: select project 1, enter for default session name
	in := strings.NewReader("1\n\n")
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
	in := strings.NewReader("1\n1\n")
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
	in := strings.NewReader("1\n1\n")
	out := &bytes.Buffer{}

	err := flow.ProjectFlow(in, out, tt, projects, nil, nil)
	if err != nil {
		t.Fatalf("ProjectFlow error: %v", err)
	}

	output := out.String()
	// The session menu should show "myapp" but NOT "other"
	if !strings.Contains(output, "myapp") {
		t.Errorf("expected myapp in output, got: %s", output)
	}
	// "other" should not appear in the sessions list section
	// (it appears in project list, but not in session list)
	if strings.Contains(output, "Sessions for other") {
		t.Errorf("should not show sessions for 'other' project, got: %s", output)
	}
}
```

**Step 2: Run the flow integration tests**

Run:
```bash
cd /Users/mark/Projects/jd/ccc/.worktrees/integration-testing && go test -tags=integration -run 'TestProjectFlow' -v ./flow/
```
Expected: All 3 tests PASS

**Step 3: Run all flow tests (unit + integration)**

Run:
```bash
cd /Users/mark/Projects/jd/ccc/.worktrees/integration-testing && go test -tags=integration -v ./flow/
```
Expected: All tests PASS

**Step 4: Commit**

```bash
cd /Users/mark/Projects/jd/ccc/.worktrees/integration-testing && git add flow/flow_integration_test.go && git commit -m "Add flow integration tests with real tmux"
```

---

### Task 7: Expect helpers

**Files:**
- Create: `internal/testutil/expect.go`

**Step 1: Write the expect helpers**

Create `internal/testutil/expect.go`:

```go
//go:build integration

package testutil

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/creack/pty"
)

// Process represents a PTY-attached child process for e2e testing.
type Process struct {
	PTY *os.File
	Cmd *exec.Cmd
}

// StartCCC builds the ccc binary and launches it with a PTY.
// The tmuxSocket parameter sets CCC_TMUX_SOCKET in the child environment
// so the binary uses an isolated tmux server.
// args are passed to the ccc binary (e.g., "local").
func StartCCC(t *testing.T, repoRoot string, tmuxSocket string, args ...string) *Process {
	t.Helper()

	// Build the binary
	binPath := repoRoot + "/ccc-test"
	buildCmd := exec.Command("go", "build", "-o", binPath, ".")
	buildCmd.Dir = repoRoot
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build ccc: %v: %s", err, out)
	}
	t.Cleanup(func() { os.Remove(binPath) })

	// Launch with PTY
	cmd := exec.Command(binPath, args...)
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(), fmt.Sprintf("CCC_TMUX_SOCKET=%s", tmuxSocket))

	ptmx, err := pty.Start(cmd)
	if err != nil {
		t.Fatalf("failed to start ccc with PTY: %v", err)
	}

	t.Cleanup(func() {
		cmd.Process.Kill()
		cmd.Wait()
		ptmx.Close()
	})

	return &Process{PTY: ptmx, Cmd: cmd}
}

// ReadUntil reads from the PTY until the match string appears or timeout fires.
// Returns all bytes read up to and including the match.
// Fails the test on timeout.
func (p *Process) ReadUntil(t *testing.T, match string, timeout time.Duration) string {
	t.Helper()

	deadline := time.After(timeout)
	var buf bytes.Buffer
	tmp := make([]byte, 256)

	for {
		select {
		case <-deadline:
			t.Fatalf("timeout waiting for %q in PTY output.\nGot so far:\n%s", match, buf.String())
			return ""
		default:
		}

		// Set a short read deadline so we can check the timeout channel
		p.PTY.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		n, err := p.PTY.Read(tmp)
		if n > 0 {
			buf.Write(tmp[:n])
			if bytes.Contains(buf.Bytes(), []byte(match)) {
				return buf.String()
			}
		}
		if err != nil && err != io.EOF {
			// Deadline exceeded is expected — just retry
			if !os.IsTimeout(err) {
				t.Fatalf("PTY read error: %v\nGot so far:\n%s", err, buf.String())
			}
		}
	}
}

// Send writes raw bytes to the PTY (keystrokes, arrow keys, etc.).
func (p *Process) Send(t *testing.T, input string) {
	t.Helper()
	if _, err := p.PTY.Write([]byte(input)); err != nil {
		t.Fatalf("failed to send to PTY: %v", err)
	}
}

// WaitForExit waits for the process to exit within the timeout.
// Returns the exit error (nil if exited cleanly).
func (p *Process) WaitForExit(t *testing.T, timeout time.Duration) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- p.Cmd.Wait() }()
	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		t.Fatalf("process did not exit within %v", timeout)
		return nil
	}
}
```

**Step 2: Verify it compiles**

Run:
```bash
cd /Users/mark/Projects/jd/ccc/.worktrees/integration-testing && go build -tags=integration ./internal/testutil/
```
Expected: No errors

**Step 3: Commit**

```bash
cd /Users/mark/Projects/jd/ccc/.worktrees/integration-testing && git add internal/testutil/expect.go && git commit -m "Add PTY expect helpers for e2e tests"
```

---

### Task 8: Add CCC_TMUX_SOCKET support to tmux package

**Files:**
- Modify: `tmux/sessions.go`

The e2e tests need the ccc binary to use an isolated tmux socket. We add a package-level `SocketOverride` variable that, when non-empty, causes all `Build*Command` functions to include `-L <socket>`. On startup, `main.go` (or an init function) reads `CCC_TMUX_SOCKET` from the environment.

**Step 1: Write the failing test**

Add to `tmux/sessions_test.go`:

```go
func TestBuildListCommand_WithSocketOverride(t *testing.T) {
	tmux.SocketOverride = "test-socket"
	defer func() { tmux.SocketOverride = "" }()

	cmd := tmux.BuildListCommand()
	if !strings.Contains(cmd, "-L test-socket") {
		t.Errorf("expected -L test-socket in command, got: %s", cmd)
	}
}

func TestBuildCreateCommand_WithSocketOverride(t *testing.T) {
	tmux.SocketOverride = "test-socket"
	defer func() { tmux.SocketOverride = "" }()

	cmd := tmux.BuildCreateCommand("rt1", "/tmp", "rt1")
	if !strings.Contains(cmd, "-L test-socket") {
		t.Errorf("expected -L test-socket in command, got: %s", cmd)
	}
}
```

**Step 2: Run to verify they fail**

Run:
```bash
cd /Users/mark/Projects/jd/ccc/.worktrees/integration-testing && go test -run 'TestBuild.*WithSocketOverride' -v ./tmux/
```
Expected: FAIL (SocketOverride doesn't exist yet)

**Step 3: Implement SocketOverride**

In `tmux/sessions.go`, add:

```go
// SocketOverride, when non-empty, adds -L <socket> to all tmux commands.
// Used for test isolation. Set from CCC_TMUX_SOCKET environment variable.
var SocketOverride string

// tmuxCmd returns "tmux" or "tmux -L <socket>" depending on SocketOverride.
func tmuxCmd() string {
	if SocketOverride != "" {
		return fmt.Sprintf("tmux -L %s", SocketOverride)
	}
	return "tmux"
}
```

Then replace all `"tmux "` or `"tmux list-sessions"` etc. in the `Build*` functions to use `tmuxCmd()`. Specifically update these functions:
- `BuildListCommand`: `fmt.Sprintf("%s list-sessions ...", tmuxCmd())`
- `BuildListClientsCommand`: `fmt.Sprintf("%s list-clients ...", tmuxCmd())`
- `BuildCreateCommand`: `fmt.Sprintf("%s new-session ...", tmuxCmd())`
- `BuildSetPassthroughCommand`: `fmt.Sprintf("%s set-window-option ...", tmuxCmd())`
- `BuildEnsureNotifyOptionsCommand`: both `tmux` invocations → `tmuxCmd()`
- `BuildAttachCommand`: `fmt.Sprintf("%s attach ...", tmuxCmd())`
- `BuildKillCommand`: `fmt.Sprintf("%s kill-session ...", tmuxCmd())`
- `BuildDetachClientsCommand`: `fmt.Sprintf("%s detach-client ...", tmuxCmd())`
- `BuildCheckTmuxCommand`: leave as `"command -v tmux"` (checks binary existence, not socket)

**Step 4: Run the new tests**

Run:
```bash
cd /Users/mark/Projects/jd/ccc/.worktrees/integration-testing && go test -run 'TestBuild.*WithSocketOverride' -v ./tmux/
```
Expected: PASS

**Step 5: Run ALL existing tests to verify no regressions**

Run:
```bash
cd /Users/mark/Projects/jd/ccc/.worktrees/integration-testing && go test ./...
```
Expected: All packages PASS (existing tests have SocketOverride="" so behavior unchanged)

**Step 6: Add init in main.go to read the env var**

In `main.go`, add to the `main()` function before the mode detection:

```go
if socket := os.Getenv("CCC_TMUX_SOCKET"); socket != "" {
	tmux.SocketOverride = socket
}
```

And add `"github.com/mark-jaeger/ccc/tmux"` to imports.

**Step 7: Commit**

```bash
cd /Users/mark/Projects/jd/ccc/.worktrees/integration-testing && git add tmux/sessions.go tmux/sessions_test.go main.go && git commit -m "Add CCC_TMUX_SOCKET support for test isolation"
```

---

### Task 9: Update TestTmux harness to use SocketOverride instead of string replacement

**Files:**
- Modify: `internal/testutil/tmuxtest.go`

Now that `tmux.SocketOverride` exists, the `TestTmux.Run` method can set it instead of doing string replacement. But since tests run in parallel, string replacement in `injectSocket` is actually safer (SocketOverride is a global). Keep `injectSocket` for the harness's own `Run` method. But add a helper to set/restore `SocketOverride` for e2e tests.

**Step 1: Add SetSocketOverride helper**

Add to `internal/testutil/tmuxtest.go`:

```go
// SetSocketOverride sets tmux.SocketOverride for the duration of a test.
// This is used by e2e tests where the ccc binary reads the override.
// For integration tests that call Build*Command directly, prefer using
// TestTmux.Run which injects the socket via string replacement.
func (tt *TestTmux) SetSocketOverride(t *testing.T) {
	t.Helper()
	tmux.SocketOverride = tt.Socket
	t.Cleanup(func() { tmux.SocketOverride = "" })
}
```

Add the import: `"github.com/mark-jaeger/ccc/tmux"`

**Step 2: Verify it compiles**

Run:
```bash
cd /Users/mark/Projects/jd/ccc/.worktrees/integration-testing && go build -tags=integration ./internal/testutil/
```
Expected: No errors

**Step 3: Commit**

```bash
cd /Users/mark/Projects/jd/ccc/.worktrees/integration-testing && git add internal/testutil/tmuxtest.go && git commit -m "Add SetSocketOverride helper to TestTmux"
```

---

### Task 10: E2E smoke tests

**Files:**
- Create: `e2e_test.go`

These tests launch the real ccc binary via PTY and verify the interactive experience. They require a projects config file in a temp directory to simulate local mode.

**Step 1: Write e2e tests**

Create `e2e_test.go` at the repo root:

```go
//go:build integration

package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mark-jaeger/ccc/internal/testutil"
)

func TestE2E_LocalMode_ProjectMenu(t *testing.T) {
	t.Parallel()
	tt := testutil.NewTestTmux(t)

	// Create a temp home with projects config
	home := t.TempDir()
	cccDir := filepath.Join(home, ".ccc")
	os.MkdirAll(cccDir, 0755)
	os.WriteFile(filepath.Join(cccDir, "projects.toml"), []byte(`
[projects.myapp]
path = "/tmp"
`), 0600)

	// Build and launch ccc in local mode
	repoRoot, _ := os.Getwd()
	proc := testutil.StartCCC(t, repoRoot, tt.Socket, "local")

	// Override HOME so ccc reads our temp config
	// Note: StartCCC uses os.Environ() so we need to set HOME before starting.
	// We'll adjust StartCCC to accept env overrides, or set HOME in the process env.
	// For now, we'll need to set it via the process's Env.

	// Actually, StartCCC sets env from os.Environ(). We need to customize.
	// Let's kill this process and relaunch with HOME override.
	proc.Cmd.Process.Kill()
	proc.Cmd.Wait()
	proc.PTY.Close()

	// Relaunch with HOME set
	proc = startCCCWithHome(t, repoRoot, tt.Socket, home, "local")

	// Wait for project menu to render
	output := proc.ReadUntil(t, "myapp", 10*time.Second)
	if output == "" {
		t.Fatal("project menu did not render")
	}
}

func TestE2E_LocalMode_QuitFromMenu(t *testing.T) {
	t.Parallel()
	tt := testutil.NewTestTmux(t)

	// Create a temp home with projects config
	home := t.TempDir()
	cccDir := filepath.Join(home, ".ccc")
	os.MkdirAll(cccDir, 0755)
	os.WriteFile(filepath.Join(cccDir, "projects.toml"), []byte(`
[projects.myapp]
path = "/tmp"
`), 0600)

	repoRoot, _ := os.Getwd()
	proc := startCCCWithHome(t, repoRoot, tt.Socket, home, "local")

	// Wait for menu
	proc.ReadUntil(t, "myapp", 10*time.Second)

	// Send 'q' to quit
	proc.Send(t, "q\n")

	// Process should exit cleanly
	err := proc.WaitForExit(t, 5*time.Second)
	if err != nil {
		t.Errorf("expected clean exit, got: %v", err)
	}
}

func TestE2E_LocalMode_CreateAndAttachSession(t *testing.T) {
	t.Parallel()
	tt := testutil.NewTestTmux(t)

	home := t.TempDir()
	cccDir := filepath.Join(home, ".ccc")
	os.MkdirAll(cccDir, 0755)
	os.WriteFile(filepath.Join(cccDir, "projects.toml"), []byte(`
[projects.myapp]
path = "/tmp"
`), 0600)

	repoRoot, _ := os.Getwd()
	proc := startCCCWithHome(t, repoRoot, tt.Socket, home, "local")

	// Wait for project menu
	proc.ReadUntil(t, "myapp", 10*time.Second)

	// Select project 1
	proc.Send(t, "1\n")

	// Wait for session name prompt (zero sessions → auto-create)
	proc.ReadUntil(t, "Session name", 5*time.Second)

	// Accept default name
	proc.Send(t, "\n")

	// Wait for attach (the process will be attached to tmux)
	// Give it a moment then check tmux state
	time.Sleep(1 * time.Second)

	// Verify session exists in tmux with correct options
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

// startCCCWithHome builds and launches ccc with a custom HOME directory.
func startCCCWithHome(t *testing.T, repoRoot, tmuxSocket, home string, args ...string) *testutil.Process {
	t.Helper()

	binPath := repoRoot + "/ccc-test"

	// Build if not already built
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		buildCmd := testutil.BuildCCC(t, repoRoot)
		_ = buildCmd
	}

	return testutil.StartCCCWithEnv(t, repoRoot, tmuxSocket, map[string]string{
		"HOME": home,
	}, args...)
}
```

Wait — this needs adjustments to the expect helpers. We need `StartCCCWithEnv` and `BuildCCC`. Let me revise.

Actually, the cleaner approach: modify `StartCCC` in `internal/testutil/expect.go` to accept an `env` map parameter. Let me update both files.

**Revised Step 1: Update expect.go to support env overrides**

In `internal/testutil/expect.go`, change `StartCCC` to accept env overrides:

```go
// StartCCC builds the ccc binary and launches it with a PTY.
// envOverrides are added to the child process environment (overriding existing values).
// The CCC_TMUX_SOCKET env var is always set to tmuxSocket.
func StartCCC(t *testing.T, repoRoot string, tmuxSocket string, envOverrides map[string]string, args ...string) *Process {
	t.Helper()

	// Build the binary
	binPath := repoRoot + "/ccc-test"
	buildCmd := exec.Command("go", "build", "-o", binPath, ".")
	buildCmd.Dir = repoRoot
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build ccc: %v: %s", err, out)
	}
	t.Cleanup(func() { os.Remove(binPath) })

	// Build environment
	env := os.Environ()
	env = append(env, fmt.Sprintf("CCC_TMUX_SOCKET=%s", tmuxSocket))
	for k, v := range envOverrides {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	// Launch with PTY
	cmd := exec.Command(binPath, args...)
	cmd.Dir = repoRoot
	cmd.Env = env

	ptmx, err := pty.Start(cmd)
	if err != nil {
		t.Fatalf("failed to start ccc with PTY: %v", err)
	}

	t.Cleanup(func() {
		cmd.Process.Kill()
		cmd.Wait()
		ptmx.Close()
	})

	return &Process{PTY: ptmx, Cmd: cmd}
}
```

**Revised e2e_test.go** (simpler, using the updated StartCCC):

```go
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
	proc.Send(t, "q\n")

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

	// Select project
	proc.ReadUntil(t, "myapp", 10*time.Second)
	proc.Send(t, "1\n")

	// Accept default session name
	proc.ReadUntil(t, "Session name", 5*time.Second)
	proc.Send(t, "\n")

	// Give tmux a moment to create the session
	time.Sleep(1 * time.Second)

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
```

**Step 2: Run the e2e tests**

Run:
```bash
cd /Users/mark/Projects/jd/ccc/.worktrees/integration-testing && go test -tags=integration -run 'TestE2E' -v -timeout 60s .
```
Expected: All 3 tests PASS

**Step 3: Commit**

```bash
cd /Users/mark/Projects/jd/ccc/.worktrees/integration-testing && git add e2e_test.go internal/testutil/expect.go && git commit -m "Add e2e smoke tests with PTY expect helpers"
```

---

### Task 11: Add CI integration test step

**Files:**
- Modify: `.github/workflows/release.yml`

**Step 1: Add integration test step**

After the existing "Run tests" step (line 38-39), add:

```yaml
      - name: Run integration tests
        run: go test -tags=integration -timeout 60s ./...
```

**Step 2: Verify YAML is valid**

Run:
```bash
cd /Users/mark/Projects/jd/ccc/.worktrees/integration-testing && python3 -c "import yaml; yaml.safe_load(open('.github/workflows/release.yml'))" && echo "valid"
```
Expected: "valid"

**Step 3: Commit**

```bash
cd /Users/mark/Projects/jd/ccc/.worktrees/integration-testing && git add .github/workflows/release.yml && git commit -m "Add integration tests to CI pipeline"
```

---

### Task 12: Final verification

**Step 1: Run all unit tests**

Run:
```bash
cd /Users/mark/Projects/jd/ccc/.worktrees/integration-testing && go test ./...
```
Expected: All packages PASS (no integration tests run)

**Step 2: Run all tests including integration**

Run:
```bash
cd /Users/mark/Projects/jd/ccc/.worktrees/integration-testing && go test -tags=integration -timeout 60s ./...
```
Expected: All packages PASS including integration tests

**Step 3: Run vet**

Run:
```bash
cd /Users/mark/Projects/jd/ccc/.worktrees/integration-testing && go vet ./...
```
Expected: No issues

**Step 4: Verify clean git state**

Run:
```bash
cd /Users/mark/Projects/jd/ccc/.worktrees/integration-testing && git status && git log --oneline -15
```
Expected: Clean working tree, all commits present
