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

		p.PTY.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		n, err := p.PTY.Read(tmp)
		if n > 0 {
			buf.Write(tmp[:n])
			if bytes.Contains(buf.Bytes(), []byte(match)) {
				return buf.String()
			}
		}
		if err != nil && err != io.EOF {
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
