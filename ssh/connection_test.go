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

	// Must wrap command in bash -lc
	if !slices.Contains(args, "bash") {
		t.Errorf("expected bash in args, got %v", args)
	}
	if !slices.Contains(args, "-lc") {
		t.Errorf("expected -lc in args, got %v", args)
	}
	if !slices.Contains(args, "uptime") {
		t.Errorf("expected uptime in args, got %v", args)
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

	// Must contain the command wrapped in bash -lc
	if !slices.Contains(args, "bash") {
		t.Errorf("expected bash in args, got %v", args)
	}
	if !slices.Contains(args, "-lc") {
		t.Errorf("expected -lc in args, got %v", args)
	}
	if !slices.Contains(args, "htop") {
		t.Errorf("expected htop in args, got %v", args)
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
