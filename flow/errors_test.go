package flow

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestCheckAbducoFound(t *testing.T) {
	runner := newMockRunner()
	runner.responses["command -v abduco"] = "/usr/bin/abduco"

	in := strings.NewReader("")
	out := &bytes.Buffer{}

	err := CheckAbduco(in, out, runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckAbducoNotFoundThenInstalled(t *testing.T) {
	runner := newMockRunner()

	// First call fails, second succeeds (after shell)
	callCount := 0
	origRun := runner.Run
	_ = origRun
	// Override Run to track call count for "command -v abduco"
	runner.responses["uname -s"] = "Linux"

	customRunner := &checkAbducoRunner{
		mockRunner:   runner,
		abducoCalls: 0,
	}

	in := strings.NewReader("")
	out := &bytes.Buffer{}

	err := CheckAbduco(in, out, customRunner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "abduco found") {
		t.Errorf("expected 'abduco found' after recheck, got: %s", out.String())
	}
	_ = callCount
}

// checkAbducoRunner fails the first "command -v abduco" call and succeeds on the second.
type checkAbducoRunner struct {
	*mockRunner
	abducoCalls int
}

func (r *checkAbducoRunner) Run(cmd string) (string, error) {
	if strings.Contains(cmd, "command -v abduco") {
		r.abducoCalls++
		if r.abducoCalls == 1 {
			return "", fmt.Errorf("not found")
		}
		return "/usr/bin/abduco", nil
	}
	return r.mockRunner.Run(cmd)
}

func (r *checkAbducoRunner) RunInteractive(cmd string) error {
	return r.mockRunner.RunInteractive(cmd)
}

func TestCheckAbducoNotFoundAfterRetry(t *testing.T) {
	runner := &alwaysFailAbducoRunner{
		mockRunner: newMockRunner(),
	}
	runner.mockRunner.responses["uname -s"] = "Linux"

	in := strings.NewReader("")
	out := &bytes.Buffer{}

	err := CheckAbduco(in, out, runner)
	if err == nil {
		t.Fatal("expected error when abduco is never found")
	}
	if !strings.Contains(err.Error(), "abduco not installed") {
		t.Errorf("expected 'abduco not installed' error, got: %v", err)
	}
}

type alwaysFailAbducoRunner struct {
	*mockRunner
}

func (r *alwaysFailAbducoRunner) Run(cmd string) (string, error) {
	if strings.Contains(cmd, "command -v abduco") {
		return "", fmt.Errorf("not found")
	}
	return r.mockRunner.Run(cmd)
}

func (r *alwaysFailAbducoRunner) RunInteractive(cmd string) error {
	return r.mockRunner.RunInteractive(cmd)
}

func TestCheckAbducoDarwinHint(t *testing.T) {
	runner := &alwaysFailAbducoRunner{
		mockRunner: newMockRunner(),
	}
	runner.mockRunner.responses["uname -s"] = "Darwin"

	in := strings.NewReader("")
	out := &bytes.Buffer{}

	_ = CheckAbduco(in, out, runner)

	output := out.String()
	if !strings.Contains(output, "brew install abduco") {
		t.Errorf("expected brew hint for Darwin, got: %s", output)
	}
	// On Darwin, should NOT show Ubuntu/Fedora hints
	if strings.Contains(output, "apt install") {
		t.Errorf("should not show apt hint on Darwin, got: %s", output)
	}
}
