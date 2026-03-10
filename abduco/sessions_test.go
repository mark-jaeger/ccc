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

func TestParseSessionList_AttachedSession(t *testing.T) {
	output := "*	Thu 2015-03-12 12:05:20	12345	ccc.rt1.main"
	sessions := ParseSessionList(output)

	if len(sessions) != 1 {
		t.Fatalf("ParseSessionList() returned %d sessions, want 1", len(sessions))
	}
	s := sessions[0]
	if s.Name != "ccc.rt1.main" {
		t.Errorf("Name = %q, want %q", s.Name, "ccc.rt1.main")
	}
	if s.Project != "rt1" {
		t.Errorf("Project = %q, want %q", s.Project, "rt1")
	}
	if s.Suffix != "main" {
		t.Errorf("Suffix = %q, want %q", s.Suffix, "main")
	}
	if s.External {
		t.Error("External = true, want false")
	}
	if s.Dead {
		t.Error("Dead = true, want false")
	}
	if s.PID != 12345 {
		t.Errorf("PID = %d, want %d", s.PID, 12345)
	}
}

func TestParseSessionList_DeadSession(t *testing.T) {
	output := "+	Thu 2015-03-12 12:04:50	12346	ccc.rt1.2"
	sessions := ParseSessionList(output)

	if len(sessions) != 1 {
		t.Fatalf("ParseSessionList() returned %d sessions, want 1", len(sessions))
	}
	s := sessions[0]
	if s.Name != "ccc.rt1.2" {
		t.Errorf("Name = %q, want %q", s.Name, "ccc.rt1.2")
	}
	if !s.Dead {
		t.Error("Dead = false, want true")
	}
	if s.PID != 12346 {
		t.Errorf("PID = %d, want %d", s.PID, 12346)
	}
}

func TestParseSessionList_DetachedExternalSession(t *testing.T) {
	output := " 	Thu 2015-03-12 12:03:30	12347	other-session"
	sessions := ParseSessionList(output)

	if len(sessions) != 1 {
		t.Fatalf("ParseSessionList() returned %d sessions, want 1", len(sessions))
	}
	s := sessions[0]
	if s.Name != "other-session" {
		t.Errorf("Name = %q, want %q", s.Name, "other-session")
	}
	if !s.External {
		t.Error("External = false, want true")
	}
	if s.Dead {
		t.Error("Dead = true, want false")
	}
}

func TestParseSessionList_SkipsHeader(t *testing.T) {
	output := `Active sessions (on host localhost)
*	Thu 2015-03-12 12:05:20	12345	ccc.rt1.main`
	sessions := ParseSessionList(output)

	if len(sessions) != 1 {
		t.Fatalf("ParseSessionList() returned %d sessions, want 1", len(sessions))
	}
	if sessions[0].Name != "ccc.rt1.main" {
		t.Errorf("Name = %q, want %q", sessions[0].Name, "ccc.rt1.main")
	}
}

func TestParseSessionList_EmptyOutput(t *testing.T) {
	sessions := ParseSessionList("")
	if sessions != nil {
		t.Errorf("ParseSessionList(\"\") = %v, want nil", sessions)
	}
}

func TestParseSessionList_MultipleSessions(t *testing.T) {
	output := `Active sessions (on host localhost)
*	Thu 2015-03-12 12:05:20	12345	ccc.rt1.main
+	Thu 2015-03-12 12:04:50	12346	ccc.rt1.2
 	Thu 2015-03-12 12:03:30	12347	other-session`
	sessions := ParseSessionList(output)

	if len(sessions) != 3 {
		t.Fatalf("ParseSessionList() returned %d sessions, want 3", len(sessions))
	}
}
