package ssh

import (
	"slices"
	"testing"
)

func TestBuildNonInteractiveArgs(t *testing.T) {
	c := Connection{
		User:    "deploy",
		Address: "10.0.0.1",
	}

	args := c.buildNonInteractiveArgs("uptime")

	// Must contain BatchMode=yes
	if !containsOption(args, "-o", "BatchMode=yes") {
		t.Errorf("expected -o BatchMode=yes in args, got %v", args)
	}

	// Must contain StrictHostKeyChecking=accept-new
	if !containsOption(args, "-o", "StrictHostKeyChecking=accept-new") {
		t.Errorf("expected -o StrictHostKeyChecking=accept-new in args, got %v", args)
	}

	// Must contain ConnectTimeout=10
	if !containsOption(args, "-o", "ConnectTimeout=10") {
		t.Errorf("expected -o ConnectTimeout=10 in args, got %v", args)
	}

	// Must contain user@address
	if !slices.Contains(args, "deploy@10.0.0.1") {
		t.Errorf("expected deploy@10.0.0.1 in args, got %v", args)
	}

	// Must wrap command in bash -lc as a single quoted argument
	lastArg := args[len(args)-1]
	if lastArg != "bash -lc 'uptime'" {
		t.Errorf("expected last arg to be %q, got %q", "bash -lc 'uptime'", lastArg)
	}

	// Must NOT contain -t (that's interactive only)
	if slices.Contains(args, "-t") {
		t.Errorf("did not expect -t in non-interactive args, got %v", args)
	}
}

func TestBuildInteractiveArgs(t *testing.T) {
	c := Connection{
		User:    "deploy",
		Address: "10.0.0.1",
	}

	args := c.buildInteractiveArgs("htop")

	// Must contain -t for PTY allocation
	if !slices.Contains(args, "-t") {
		t.Errorf("expected -t in interactive args, got %v", args)
	}

	// Must NOT contain BatchMode=yes
	if containsOption(args, "-o", "BatchMode=yes") {
		t.Errorf("did not expect -o BatchMode=yes in interactive args, got %v", args)
	}

	// Must contain user@address
	if !slices.Contains(args, "deploy@10.0.0.1") {
		t.Errorf("expected deploy@10.0.0.1 in args, got %v", args)
	}

	// Must contain the command wrapped in bash -lc as single arg
	lastArg := args[len(args)-1]
	if lastArg != "bash -lc 'htop'" {
		t.Errorf("expected last arg to be %q, got %q", "bash -lc 'htop'", lastArg)
	}
}

func TestBuildArgsWithSSHOptions(t *testing.T) {
	c := Connection{
		User:       "deploy",
		Address:    "10.0.0.1",
		SSHOptions: []string{"-o", "ServerAliveInterval=60"},
	}

	args := c.buildNonInteractiveArgs("ls")

	// SSHOptions should be passed as raw args, not wrapped with extra -o
	if !containsOption(args, "-o", "ServerAliveInterval=60") {
		t.Errorf("expected -o ServerAliveInterval=60 in args, got %v", args)
	}
}

func TestBuildArgsWithPort(t *testing.T) {
	c := Connection{
		User:    "deploy",
		Address: "10.0.0.1",
		Port:    2222,
	}

	args := c.buildNonInteractiveArgs("ls")

	if !containsOption(args, "-p", "2222") {
		t.Errorf("expected -p 2222 in args, got %v", args)
	}
}

func TestBuildArgsWithIdentityFile(t *testing.T) {
	c := Connection{
		User:         "deploy",
		Address:      "10.0.0.1",
		IdentityFile: "/home/deploy/.ssh/id_ed25519",
	}

	args := c.buildNonInteractiveArgs("ls")

	if !containsOption(args, "-i", "/home/deploy/.ssh/id_ed25519") {
		t.Errorf("expected -i /home/deploy/.ssh/id_ed25519 in args, got %v", args)
	}
}

// containsOption checks that args contains a consecutive pair of flag and value.
func containsOption(args []string, flag, value string) bool {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == flag && args[i+1] == value {
			return true
		}
	}
	return false
}
