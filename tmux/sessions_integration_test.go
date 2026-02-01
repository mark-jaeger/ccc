//go:build integration

package tmux_test

import (
	"bytes"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/creack/pty"
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

	project := tt.GetOption(t, "myapp", "@ccc_project")
	if project != "myapp" {
		t.Errorf("@ccc_project = %q, want %q", project, "myapp")
	}

	path := tt.GetOption(t, "myapp", "@ccc_path")
	if path != "/tmp/myapp" {
		t.Errorf("@ccc_path = %q, want %q", path, "/tmp/myapp")
	}
}

func TestCreateSession_SetsNotifyOptions(t *testing.T) {
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

	silenceAction := tt.GetOption(t, "myapp", "silence-action")
	if silenceAction != "any" {
		t.Errorf("silence-action = %q, want %q", silenceAction, "any")
	}

	visualBell := tt.GetWindowOption(t, "myapp", "visual-bell")
	if visualBell != "off" {
		t.Errorf("visual-bell = %q, want %q", visualBell, "off")
	}

	monitorSilence := tt.GetWindowOption(t, "myapp", "monitor-silence")
	if monitorSilence != "5" {
		t.Errorf("monitor-silence = %q, want %q", monitorSilence, "5")
	}
}

func TestSetPassthrough_EnablesOption(t *testing.T) {
	t.Parallel()
	tt := testutil.NewTestTmux(t)

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

func TestMonitorSilence_TriggersAfterTimeout(t *testing.T) {
	t.Parallel()
	tt := testutil.NewTestTmux(t)

	// Create session with ccc's full notification setup
	cmd := tmux.BuildCreateCommand("myapp", "/tmp", "myapp")
	if _, err := tt.Run(cmd); err != nil {
		t.Fatalf("create command failed: %v", err)
	}
	if _, err := tt.Run(tmux.BuildSetPassthroughCommand("myapp")); err != nil {
		t.Fatalf("set passthrough failed: %v", err)
	}

	// Generate activity then let the window go silent
	if _, err := tt.Run("tmux send-keys -t myapp 'echo activity' Enter"); err != nil {
		t.Fatalf("send-keys failed: %v", err)
	}

	// Wait for monitor-silence timeout (5s) + margin
	time.Sleep(7 * time.Second)

	// The silence flag should be set
	flag, err := tt.Run("tmux list-windows -t myapp -F '#{window_silence_flag}'")
	if err != nil {
		t.Fatalf("list-windows failed: %v", err)
	}
	if flag != "1" {
		t.Errorf("window_silence_flag = %q, want %q (monitor-silence should have triggered)", flag, "1")
	}
}

func TestMonitorSilence_SendsBellToAttachedClient(t *testing.T) {
	t.Parallel()
	tt := testutil.NewTestTmux(t)

	// Create session with full notification setup and a short silence timeout
	cmd := tmux.BuildCreateCommand("myapp", "/tmp", "myapp")
	if _, err := tt.Run(cmd); err != nil {
		t.Fatalf("create command failed: %v", err)
	}
	// Override monitor-silence to 2s for faster test
	if _, err := tt.Run("tmux set-window-option -t myapp monitor-silence 2"); err != nil {
		t.Fatalf("set monitor-silence failed: %v", err)
	}

	// Attach via PTY so we can capture the actual terminal output
	attachCmd := exec.Command("tmux", "-L", tt.Socket, "attach", "-t", "myapp")
	ptmx, err := pty.Start(attachCmd)
	if err != nil {
		t.Fatalf("failed to start tmux attach with PTY: %v", err)
	}
	t.Cleanup(func() {
		ptmx.Close()
		attachCmd.Process.Kill()
		attachCmd.Wait()
	})

	// Generate activity then let the window go silent
	sendKeysCmd := exec.Command("tmux", "-L", tt.Socket, "send-keys", "-t", "myapp", "echo hello", "Enter")
	if out, err := sendKeysCmd.CombinedOutput(); err != nil {
		t.Fatalf("send-keys failed: %v: %s", err, out)
	}

	// Read from PTY until we see BEL (0x07) or timeout
	deadline := time.After(8 * time.Second)
	totalBytes := 0
	foundBell := false

	for !foundBell {
		ch := make(chan struct{ n int; err error }, 1)
		tmp := make([]byte, 256)
		go func() {
			n, err := ptmx.Read(tmp)
			ch <- struct{ n int; err error }{n, err}
		}()

		select {
		case <-deadline:
			t.Fatalf("timeout waiting for BEL character in PTY output.\nGot %d bytes so far (no 0x07 found)", totalBytes)
		case rc := <-ch:
			if rc.n > 0 {
				totalBytes += rc.n
				if bytes.IndexByte(tmp[:rc.n], '\a') >= 0 {
					foundBell = true
				}
			}
			if rc.err != nil {
				if !os.IsTimeout(rc.err) {
					t.Fatalf("PTY read error: %v", rc.err)
				}
			}
		}
	}

	t.Logf("BEL character received after silence timeout (read %d bytes total)", totalBytes)
}

func TestEnsureNotifyOptions_SetsAllOptions(t *testing.T) {
	t.Parallel()
	tt := testutil.NewTestTmux(t)

	tt.CreateSession(t, "oldapp")

	cmd := tmux.BuildEnsureNotifyOptionsCommand("oldapp")
	if _, err := tt.Run(cmd); err != nil {
		t.Fatalf("ensure notify options failed: %v", err)
	}

	bellAction := tt.GetOption(t, "oldapp", "bell-action")
	if bellAction != "any" {
		t.Errorf("bell-action = %q, want %q", bellAction, "any")
	}

	silenceAction := tt.GetOption(t, "oldapp", "silence-action")
	if silenceAction != "any" {
		t.Errorf("silence-action = %q, want %q", silenceAction, "any")
	}

	visualBell := tt.GetWindowOption(t, "oldapp", "visual-bell")
	if visualBell != "off" {
		t.Errorf("visual-bell = %q, want %q", visualBell, "off")
	}

	monitorSilence := tt.GetWindowOption(t, "oldapp", "monitor-silence")
	if monitorSilence != "5" {
		t.Errorf("monitor-silence = %q, want %q", monitorSilence, "5")
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

	if _, err := tt.Run(cmd); err != nil {
		t.Fatalf("first ensure notify options failed: %v", err)
	}
	if _, err := tt.Run(cmd); err != nil {
		t.Fatalf("second ensure notify options failed: %v", err)
	}

	bellAction := tt.GetOption(t, "myapp", "bell-action")
	if bellAction != "any" {
		t.Errorf("bell-action = %q, want %q", bellAction, "any")
	}

	silenceAction := tt.GetOption(t, "myapp", "silence-action")
	if silenceAction != "any" {
		t.Errorf("silence-action = %q, want %q", silenceAction, "any")
	}

	monitorSilence := tt.GetWindowOption(t, "myapp", "monitor-silence")
	if monitorSilence != "5" {
		t.Errorf("monitor-silence = %q, want %q", monitorSilence, "5")
	}

	passthrough := tt.GetWindowOption(t, "myapp", "allow-passthrough")
	if passthrough != "on" {
		t.Errorf("allow-passthrough = %q, want %q", passthrough, "on")
	}
}

func TestFilterSessionsForProject_RealTmux(t *testing.T) {
	t.Parallel()
	tt := testutil.NewTestTmux(t)

	cmd1 := tmux.BuildCreateCommand("proj", "/tmp/proj", "proj")
	if _, err := tt.Run(cmd1); err != nil {
		t.Fatalf("create proj failed: %v", err)
	}
	cmd2 := tmux.BuildCreateCommand("proj-2", "/tmp/proj", "proj")
	if _, err := tt.Run(cmd2); err != nil {
		t.Fatalf("create proj-2 failed: %v", err)
	}
	cmd3 := tmux.BuildCreateCommand("other", "/tmp/other", "other")
	if _, err := tt.Run(cmd3); err != nil {
		t.Fatalf("create other failed: %v", err)
	}
	tt.CreateSession(t, "proj-legacy")

	listCmd := tmux.BuildListCommand()
	output, err := tt.Run(listCmd)
	if err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	allSessions := tmux.ParseSessionList(output)
	filtered := tmux.FilterSessionsForProject(allSessions, "proj")

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
