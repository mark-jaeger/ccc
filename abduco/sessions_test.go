package abduco

import "testing"

func TestBuildCreateCommand(t *testing.T) {
	got := BuildCreateCommand("ccc.rt1.main", "/home/user/proj")
	want := "cd '/home/user/proj' && abduco -n 'ccc.rt1.main' bash -l"
	if got != want {
		t.Errorf("BuildCreateCommand() = %q, want %q", got, want)
	}
}

func TestBuildCreateCommand_WithSpecialChars(t *testing.T) {
	got := BuildCreateCommand("ccc.rt1.main", "/home/user/project's dir")
	want := "cd '/home/user/project'\\''s dir' && abduco -n 'ccc.rt1.main' bash -l"
	if got != want {
		t.Errorf("BuildCreateCommand() = %q, want %q", got, want)
	}
}

func TestBuildAttachCommand(t *testing.T) {
	got := BuildAttachCommand("ccc.rt1.main")
	want := "abduco -a 'ccc.rt1.main'"
	if got != want {
		t.Errorf("BuildAttachCommand() = %q, want %q", got, want)
	}
}

func TestBuildListCommand(t *testing.T) {
	got := BuildListCommand()
	want := "abduco 2>&1 || true"
	if got != want {
		t.Errorf("BuildListCommand() = %q, want %q", got, want)
	}
}

func TestBuildKillCommand(t *testing.T) {
	got := BuildKillCommand(12345)
	want := "kill 12345"
	if got != want {
		t.Errorf("BuildKillCommand() = %q, want %q", got, want)
	}
}

func TestBuildCheckCommand(t *testing.T) {
	got := BuildCheckCommand()
	want := "command -v abduco"
	if got != want {
		t.Errorf("BuildCheckCommand() = %q, want %q", got, want)
	}
}
