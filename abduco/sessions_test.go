package abduco

import "testing"

func TestBuildCreateCommand(t *testing.T) {
	got := BuildCreateCommand("ccc.rt1.main", "/home/user/proj", "")
	want := "cd '/home/user/proj' && abduco -n 'ccc.rt1.main' bash -l"
	if got != want {
		t.Errorf("BuildCreateCommand() = %q, want %q", got, want)
	}
}

func TestBuildCreateCommand_WithSpecialChars(t *testing.T) {
	got := BuildCreateCommand("ccc.rt1.main", "/home/user/project's dir", "")
	want := "cd '/home/user/project'\\''s dir' && abduco -n 'ccc.rt1.main' bash -l"
	if got != want {
		t.Errorf("BuildCreateCommand() = %q, want %q", got, want)
	}
}

func TestBuildCreateCommand_WithDetachKey(t *testing.T) {
	got := BuildCreateCommand("ccc.rt1.main", "/home/user/proj", "^a")
	want := "cd '/home/user/proj' && abduco -e '^a' -n 'ccc.rt1.main' bash -l"
	if got != want {
		t.Errorf("BuildCreateCommand() = %q, want %q", got, want)
	}
}

func TestBuildAttachCommand(t *testing.T) {
	got := BuildAttachCommand("ccc.rt1.main", "")
	want := "abduco -a 'ccc.rt1.main'"
	if got != want {
		t.Errorf("BuildAttachCommand() = %q, want %q", got, want)
	}
}

func TestBuildAttachCommand_WithDetachKey(t *testing.T) {
	got := BuildAttachCommand("ccc.rt1.main", "^a")
	want := "abduco -e '^a' -a 'ccc.rt1.main'"
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

func TestFilterSessionsForProject(t *testing.T) {
	sessions := []Session{
		{Name: "ccc.rt1.main", Project: "rt1", Suffix: "main"},
		{Name: "ccc.rt1.2", Project: "rt1", Suffix: "2"},
		{Name: "ccc.other.main", Project: "other", Suffix: "main"},
		{Name: "external-session", External: true},
	}

	filtered := FilterSessionsForProject(sessions, "rt1")

	// Should include rt1 sessions + external
	if len(filtered) != 3 {
		t.Fatalf("FilterSessionsForProject() returned %d sessions, want 3", len(filtered))
	}

	// Verify rt1 sessions are included
	hasRt1Main := false
	hasRt12 := false
	hasExternal := false
	for _, s := range filtered {
		if s.Name == "ccc.rt1.main" {
			hasRt1Main = true
		}
		if s.Name == "ccc.rt1.2" {
			hasRt12 = true
		}
		if s.Name == "external-session" {
			hasExternal = true
		}
	}
	if !hasRt1Main {
		t.Error("FilterSessionsForProject() missing ccc.rt1.main")
	}
	if !hasRt12 {
		t.Error("FilterSessionsForProject() missing ccc.rt1.2")
	}
	if !hasExternal {
		t.Error("FilterSessionsForProject() missing external-session")
	}
}

func TestFilterSessionsForProject_ExcludesOtherProjects(t *testing.T) {
	sessions := []Session{
		{Name: "ccc.rt1.main", Project: "rt1", Suffix: "main"},
		{Name: "ccc.other.main", Project: "other", Suffix: "main"},
	}

	filtered := FilterSessionsForProject(sessions, "rt1")

	if len(filtered) != 1 {
		t.Fatalf("FilterSessionsForProject() returned %d sessions, want 1", len(filtered))
	}
	if filtered[0].Name != "ccc.rt1.main" {
		t.Errorf("FilterSessionsForProject() returned %q, want ccc.rt1.main", filtered[0].Name)
	}
}

func TestNextAutoName_NoExisting(t *testing.T) {
	got := NextAutoName("rt1", nil)
	want := "ccc.rt1.main"
	if got != want {
		t.Errorf("NextAutoName() = %q, want %q", got, want)
	}
}

func TestNextAutoName_MainExists(t *testing.T) {
	existing := []Session{
		{Name: "ccc.rt1.main", Project: "rt1", Suffix: "main"},
	}
	got := NextAutoName("rt1", existing)
	want := "ccc.rt1.2"
	if got != want {
		t.Errorf("NextAutoName() = %q, want %q", got, want)
	}
}

func TestNextAutoName_WithGap(t *testing.T) {
	// main and 3 exist, should return 4
	existing := []Session{
		{Name: "ccc.rt1.main", Project: "rt1", Suffix: "main"},
		{Name: "ccc.rt1.3", Project: "rt1", Suffix: "3"},
	}
	got := NextAutoName("rt1", existing)
	want := "ccc.rt1.4"
	if got != want {
		t.Errorf("NextAutoName() = %q, want %q", got, want)
	}
}

func TestNextAutoName_NoMain(t *testing.T) {
	// Only numbered sessions exist, should return main
	existing := []Session{
		{Name: "ccc.rt1.2", Project: "rt1", Suffix: "2"},
	}
	got := NextAutoName("rt1", existing)
	want := "ccc.rt1.main"
	if got != want {
		t.Errorf("NextAutoName() = %q, want %q", got, want)
	}
}
