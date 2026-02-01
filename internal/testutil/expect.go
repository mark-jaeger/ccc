//go:build integration

package testutil

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
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

	// Build environment: filter out keys we're overriding to avoid
	// relying on "last value wins" semantics for duplicate env keys.
	overrides := map[string]string{"CCC_TMUX_SOCKET": tmuxSocket}
	for k, v := range envOverrides {
		overrides[k] = v
	}
	var env []string
	for _, e := range os.Environ() {
		if k, _, ok := strings.Cut(e, "="); ok && overrides[k] != "" {
			continue
		}
		env = append(env, e)
	}
	for k, v := range overrides {
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
		// Close the PTY first — this sends SIGHUP to the child's session,
		// which terminates tmux attach and any other children.
		ptmx.Close()
		cmd.Process.Kill()
		// Wait with a timeout to avoid hanging on cleanup.
		done := make(chan struct{})
		go func() {
			cmd.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
		}
	})

	return &Process{PTY: ptmx, Cmd: cmd}
}

// readChunk holds data from a single PTY read.
type readChunk struct {
	data []byte
	err  error
}

// ReadUntil reads from the PTY until the match string appears or timeout fires.
// Returns all bytes read up to and including the match.
// Fails the test on timeout.
//
// Reads are performed in a goroutine because PTY file descriptors on macOS
// do not support SetReadDeadline; a direct Read can block indefinitely.
func (p *Process) ReadUntil(t *testing.T, match string, timeout time.Duration) string {
	t.Helper()

	deadline := time.After(timeout)
	var buf bytes.Buffer

	for {
		ch := make(chan readChunk, 1)
		go func() {
			tmp := make([]byte, 256)
			n, err := p.PTY.Read(tmp)
			ch <- readChunk{data: tmp[:n], err: err}
		}()

		select {
		case <-deadline:
			t.Fatalf("timeout waiting for %q in PTY output.\nGot so far:\n%s", match, buf.String())
			return ""
		case rc := <-ch:
			if len(rc.data) > 0 {
				buf.Write(rc.data)
				if bytes.Contains(buf.Bytes(), []byte(match)) {
					return buf.String()
				}
			}
			if rc.err != nil {
				if rc.err == io.EOF {
					t.Fatalf("PTY closed (process exited) before finding %q.\nGot so far:\n%s",
						match, buf.String())
					return ""
				}
				if !os.IsTimeout(rc.err) {
					t.Fatalf("PTY read error: %v\nGot so far:\n%s", rc.err, buf.String())
				}
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
