//go:build unix

package ssh

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

// TestRunContextDoesNotHangOnLingeringStderr is a regression test for the
// ControlPersist stderr-pipe trap. With ControlMaster, ssh forks a background
// master that holds the child's stderr (fd 2) open for the whole persist
// window. If RunContext captured stderr through an os.Pipe (as Cmd.Output and
// Cmd.CombinedOutput both do), that pipe would never reach EOF: Wait would block
// until WaitDelay (set in proc_unix.go) and return ErrWaitDelay, discarding the
// valid stdout and reporting the command as failed even though the remote
// succeeded.
//
// The execCommandContext seam stands in a shell that emulates the master: it
// prints to stdout, then forks a child that keeps stderr open for several
// seconds while redirecting its own stdout away (just as the real master
// lingers on stderr but releases stdout). The foreground exits 0 immediately.
// RunContext must return the stdout promptly and without error.
func TestRunContextDoesNotHangOnLingeringStderr(t *testing.T) {
	orig := execCommandContext
	defer func() { execCommandContext = orig }()

	// `sleep 5 >/dev/null &` keeps fd 2 (stderr) open but not fd 1 (stdout), so
	// only a stderr *pipe* would be held past the foreground's exit.
	const script = `printf 'session-output\n'; sleep 5 >/dev/null & exit 0`
	execCommandContext = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sh", "-c", script)
	}

	c := Connection{User: "deploy", Address: "10.0.0.1"}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	start := time.Now()
	out, err := c.RunContext(ctx, "whoami")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("RunContext returned error despite a successful command: %v", err)
	}
	if out != "session-output" {
		t.Errorf("expected stdout %q, got %q", "session-output", out)
	}
	// With the bug, WaitDelay would elapse before returning. Allow generous slack
	// for slow CI but stay well under the 5s stderr-holder lifetime.
	if elapsed > 3*time.Second {
		t.Errorf("RunContext took %v; a lingering stderr holder must not delay a successful run", elapsed)
	}
}
