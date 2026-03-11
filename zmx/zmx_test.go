package zmx

import "testing"

// Task 1: Command builder tests

func TestBuildListCommand(t *testing.T) {
	got := BuildListCommand()
	want := "zmx list"
	if got != want {
		t.Errorf("BuildListCommand() = %q, want %q", got, want)
	}
}

func TestBuildAttachCommand(t *testing.T) {
	got := BuildAttachCommand("dev")
	want := "TERM=$TERM zmx attach 'dev'"
	if got != want {
		t.Errorf("BuildAttachCommand() = %q, want %q", got, want)
	}
}

func TestBuildAttachCommand_WithSpecialChars(t *testing.T) {
	got := BuildAttachCommand("my session")
	want := "TERM=$TERM zmx attach 'my session'"
	if got != want {
		t.Errorf("BuildAttachCommand() = %q, want %q", got, want)
	}
}

func TestBuildAttachCommand_WithQuotes(t *testing.T) {
	got := BuildAttachCommand("user's session")
	want := "TERM=$TERM zmx attach 'user'\\''s session'"
	if got != want {
		t.Errorf("BuildAttachCommand() = %q, want %q", got, want)
	}
}

func TestBuildCreateCommand(t *testing.T) {
	got := BuildCreateCommand("dev", "/home/user/project")
	want := "cd '/home/user/project' && TERM=$TERM zmx attach 'dev'"
	if got != want {
		t.Errorf("BuildCreateCommand() = %q, want %q", got, want)
	}
}

func TestBuildCreateCommand_WithSpecialChars(t *testing.T) {
	got := BuildCreateCommand("dev", "/home/user/project's dir")
	want := "cd '/home/user/project'\\''s dir' && TERM=$TERM zmx attach 'dev'"
	if got != want {
		t.Errorf("BuildCreateCommand() = %q, want %q", got, want)
	}
}

func TestBuildCreateCommand_WithSpaces(t *testing.T) {
	got := BuildCreateCommand("my session", "/home/user/my project")
	want := "cd '/home/user/my project' && TERM=$TERM zmx attach 'my session'"
	if got != want {
		t.Errorf("BuildCreateCommand() = %q, want %q", got, want)
	}
}

func TestBuildKillCommand(t *testing.T) {
	got := BuildKillCommand("dev")
	want := "zmx kill 'dev'"
	if got != want {
		t.Errorf("BuildKillCommand() = %q, want %q", got, want)
	}
}

func TestBuildKillCommand_WithSpecialChars(t *testing.T) {
	got := BuildKillCommand("user's session")
	want := "zmx kill 'user'\\''s session'"
	if got != want {
		t.Errorf("BuildKillCommand() = %q, want %q", got, want)
	}
}

func TestBuildCheckCommand(t *testing.T) {
	got := BuildCheckCommand()
	want := "command -v zmx"
	if got != want {
		t.Errorf("BuildCheckCommand() = %q, want %q", got, want)
	}
}
