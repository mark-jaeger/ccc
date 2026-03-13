package flow

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestCheckZmxFound(t *testing.T) {
	runner := newMockRunner()
	runner.responses["command -v zmx"] = "/usr/bin/zmx"

	in := strings.NewReader("")
	out := &bytes.Buffer{}

	err := CheckZmx(in, out, runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckZmxNotFoundThenInstalled(t *testing.T) {
	runner := newMockRunner()

	// First call fails, second succeeds (after shell)
	callCount := 0
	origRun := runner.Run
	_ = origRun
	// Override Run to track call count for "command -v zmx"
	runner.responses["uname -s"] = "Linux"

	customRunner := &checkZmxRunner{
		mockRunner: runner,
		zmxCalls:   0,
	}

	in := strings.NewReader("")
	out := &bytes.Buffer{}

	err := CheckZmx(in, out, customRunner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "zmx found") {
		t.Errorf("expected 'zmx found' after recheck, got: %s", out.String())
	}
	_ = callCount
}

// checkZmxRunner fails the first "command -v zmx" call and succeeds on the second.
type checkZmxRunner struct {
	*mockRunner
	zmxCalls int
}

func (r *checkZmxRunner) Run(cmd string) (string, error) {
	if strings.Contains(cmd, "command -v zmx") {
		r.zmxCalls++
		if r.zmxCalls == 1 {
			return "", fmt.Errorf("not found")
		}
		return "/usr/bin/zmx", nil
	}
	return r.mockRunner.Run(cmd)
}

func (r *checkZmxRunner) RunInteractive(cmd string) error {
	return r.mockRunner.RunInteractive(cmd)
}

func TestCheckZmxNotFoundAfterRetry(t *testing.T) {
	runner := &alwaysFailZmxRunner{
		mockRunner: newMockRunner(),
	}
	runner.mockRunner.responses["uname -s"] = "Linux"

	in := strings.NewReader("")
	out := &bytes.Buffer{}

	err := CheckZmx(in, out, runner)
	if err == nil {
		t.Fatal("expected error when zmx is never found")
	}
	if !strings.Contains(err.Error(), "zmx not installed") {
		t.Errorf("expected 'zmx not installed' error, got: %v", err)
	}
}

type alwaysFailZmxRunner struct {
	*mockRunner
}

func (r *alwaysFailZmxRunner) Run(cmd string) (string, error) {
	if strings.Contains(cmd, "command -v zmx") {
		return "", fmt.Errorf("not found")
	}
	return r.mockRunner.Run(cmd)
}

func (r *alwaysFailZmxRunner) RunInteractive(cmd string) error {
	return r.mockRunner.RunInteractive(cmd)
}

func TestCheckZmxDarwinHint(t *testing.T) {
	runner := &alwaysFailZmxRunner{
		mockRunner: newMockRunner(),
	}
	runner.mockRunner.responses["uname -s"] = "Darwin"

	in := strings.NewReader("")
	out := &bytes.Buffer{}

	_ = CheckZmx(in, out, runner)

	output := out.String()
	if !strings.Contains(output, "brew install zmx") {
		t.Errorf("expected brew hint for Darwin, got: %s", output)
	}
	// On Darwin, should NOT show Linux cargo hints
	if strings.Contains(output, "cargo install") {
		t.Errorf("should not show cargo hint on Darwin, got: %s", output)
	}
}
