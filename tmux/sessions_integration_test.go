//go:build integration

package tmux_test

import (
	"bytes"
	"os/exec"
	"strings"
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

	activityAction := tt.GetOption(t, "myapp", "activity-action")
	if activityAction != "any" {
		t.Errorf("activity-action = %q, want %q", activityAction, "any")
	}

	visualActivity := tt.GetOption(t, "myapp", "visual-activity")
	if visualActivity != "on" {
		t.Errorf("visual-activity = %q, want %q", visualActivity, "on")
	}

	visualBell := tt.GetWindowOption(t, "myapp", "visual-bell")
	if visualBell != "off" {
		t.Errorf("visual-bell = %q, want %q", visualBell, "off")
	}

	monitorSilence := tt.GetWindowOption(t, "myapp", "monitor-silence")
	if monitorSilence != "5" {
		t.Errorf("monitor-silence = %q, want %q", monitorSilence, "5")
	}

	monitorActivity := tt.GetWindowOption(t, "myapp", "monitor-activity")
	if monitorActivity != "off" {
		t.Errorf("monitor-activity = %q, want %q", monitorActivity, "off")
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

func TestNotifyHooks_OneShotBell(t *testing.T) {
	t.Parallel()
	tt := testutil.NewTestTmux(t)

	// Create session with full notification setup
	cmd := tmux.BuildCreateCommand("myapp", "/tmp", "myapp")
	if _, err := tt.Run(cmd); err != nil {
		t.Fatalf("create command failed: %v", err)
	}
	// Install one-shot hooks
	if _, err := tt.Run(tmux.BuildSetNotifyHooksCommand("myapp")); err != nil {
		t.Fatalf("set notify hooks failed: %v", err)
	}
	// Override monitor-silence to 2s for faster test
	if _, err := tt.Run("tmux set-window-option -t myapp monitor-silence 2"); err != nil {
		t.Fatalf("set monitor-silence failed: %v", err)
	}

	// Attach via PTY so we can capture actual terminal output
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
	sendKeys := exec.Command("tmux", "-L", tt.Socket, "send-keys", "-t", "myapp", "echo hello", "Enter")
	if out, err := sendKeys.CombinedOutput(); err != nil {
		t.Fatalf("send-keys failed: %v: %s", err, out)
	}

	// Count BELs over 10 seconds (monitor-silence=2s, so without hooks we'd
	// get ~4 bells; with hooks we should get exactly 1)
	bellCount := 0
	deadline := time.After(10 * time.Second)
	bellCh := make(chan struct{}, 100)
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				for i := 0; i < bytes.Count(buf[:n], []byte{'\a'}); i++ {
					bellCh <- struct{}{}
				}
			}
			if err != nil {
				return
			}
		}
	}()

	for {
		select {
		case <-bellCh:
			bellCount++
		case <-deadline:
			if bellCount == 0 {
				t.Fatal("no BEL received (expected exactly 1)")
			}
			if bellCount != 1 {
				t.Errorf("got %d BELs, want exactly 1 (hooks should prevent repeating)", bellCount)
			} else {
				t.Logf("one-shot bell verified: exactly 1 BEL in 10s")
			}
			return
		}
	}
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

	activityAction := tt.GetOption(t, "oldapp", "activity-action")
	if activityAction != "any" {
		t.Errorf("activity-action = %q, want %q", activityAction, "any")
	}

	visualActivity := tt.GetOption(t, "oldapp", "visual-activity")
	if visualActivity != "on" {
		t.Errorf("visual-activity = %q, want %q", visualActivity, "on")
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

	// Verify hooks are installed
	hooks, err := tt.Run("tmux show-hooks -t oldapp")
	if err != nil {
		t.Fatalf("show-hooks failed: %v", err)
	}
	if !strings.Contains(hooks, "alert-silence") {
		t.Errorf("expected alert-silence hook in: %s", hooks)
	}
	if !strings.Contains(hooks, "alert-activity") {
		t.Errorf("expected alert-activity hook in: %s", hooks)
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

	activityAction := tt.GetOption(t, "myapp", "activity-action")
	if activityAction != "any" {
		t.Errorf("activity-action = %q, want %q", activityAction, "any")
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
