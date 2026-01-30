package flow

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestCheckTmuxFound(t *testing.T) {
	runner := newMockRunner()
	runner.responses["command -v tmux"] = "/usr/bin/tmux"

	in := strings.NewReader("")
	out := &bytes.Buffer{}

	err := CheckTmux(in, out, runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckTmuxNotFoundThenInstalled(t *testing.T) {
	runner := newMockRunner()

	// First call fails, second succeeds (after shell)
	callCount := 0
	origRun := runner.Run
	_ = origRun
	// Override Run to track call count for "command -v tmux"
	runner.responses["uname -s"] = "Linux"

	customRunner := &checkTmuxRunner{
		mockRunner: runner,
		tmuxCalls:  0,
	}

	in := strings.NewReader("")
	out := &bytes.Buffer{}

	err := CheckTmux(in, out, customRunner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "tmux found") {
		t.Errorf("expected 'tmux found' after recheck, got: %s", out.String())
	}
	_ = callCount
}

// checkTmuxRunner fails the first "command -v tmux" call and succeeds on the second.
type checkTmuxRunner struct {
	*mockRunner
	tmuxCalls int
}

func (r *checkTmuxRunner) Run(cmd string) (string, error) {
	if strings.Contains(cmd, "command -v tmux") {
		r.tmuxCalls++
		if r.tmuxCalls == 1 {
			return "", fmt.Errorf("not found")
		}
		return "/usr/bin/tmux", nil
	}
	return r.mockRunner.Run(cmd)
}

func (r *checkTmuxRunner) RunInteractive(cmd string) error {
	return r.mockRunner.RunInteractive(cmd)
}

func TestCheckTmuxNotFoundAfterRetry(t *testing.T) {
	runner := &alwaysFailTmuxRunner{
		mockRunner: newMockRunner(),
	}
	runner.mockRunner.responses["uname -s"] = "Linux"

	in := strings.NewReader("")
	out := &bytes.Buffer{}

	err := CheckTmux(in, out, runner)
	if err == nil {
		t.Fatal("expected error when tmux is never found")
	}
	if !strings.Contains(err.Error(), "tmux not installed") {
		t.Errorf("expected 'tmux not installed' error, got: %v", err)
	}
}

type alwaysFailTmuxRunner struct {
	*mockRunner
}

func (r *alwaysFailTmuxRunner) Run(cmd string) (string, error) {
	if strings.Contains(cmd, "command -v tmux") {
		return "", fmt.Errorf("not found")
	}
	return r.mockRunner.Run(cmd)
}

func (r *alwaysFailTmuxRunner) RunInteractive(cmd string) error {
	return r.mockRunner.RunInteractive(cmd)
}

func TestCheckTmuxDarwinHint(t *testing.T) {
	runner := &alwaysFailTmuxRunner{
		mockRunner: newMockRunner(),
	}
	runner.mockRunner.responses["uname -s"] = "Darwin"

	in := strings.NewReader("")
	out := &bytes.Buffer{}

	_ = CheckTmux(in, out, runner)

	output := out.String()
	if !strings.Contains(output, "brew install tmux") {
		t.Errorf("expected brew hint for Darwin, got: %s", output)
	}
	// On Darwin, should NOT show Ubuntu/Fedora hints
	if strings.Contains(output, "apt install") {
		t.Errorf("should not show apt hint on Darwin, got: %s", output)
	}
}
