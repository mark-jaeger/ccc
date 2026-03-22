package zmx

import "testing"

// Task 1: Command builder tests

func TestBuildListCommand(t *testing.T) {
	got := BuildListCommand()
	want := pathPrefix + "zmx list"
	if got != want {
		t.Errorf("BuildListCommand() = %q, want %q", got, want)
	}
}

func TestBuildAttachCommand(t *testing.T) {
	got := BuildAttachCommand("dev")
	want := pathPrefix + "TERM=$TERM zmx attach 'dev'"
	if got != want {
		t.Errorf("BuildAttachCommand() = %q, want %q", got, want)
	}
}

func TestBuildAttachCommand_WithSpecialChars(t *testing.T) {
	got := BuildAttachCommand("my session")
	want := pathPrefix + "TERM=$TERM zmx attach 'my session'"
	if got != want {
		t.Errorf("BuildAttachCommand() = %q, want %q", got, want)
	}
}

func TestBuildAttachCommand_WithQuotes(t *testing.T) {
	got := BuildAttachCommand("user's session")
	want := pathPrefix + "TERM=$TERM zmx attach 'user'\\''s session'"
	if got != want {
		t.Errorf("BuildAttachCommand() = %q, want %q", got, want)
	}
}

func TestBuildCreateCommand(t *testing.T) {
	got := BuildCreateCommand("dev", "/home/user/project")
	want := "cd '/home/user/project' && " + pathPrefix + "TERM=$TERM zmx attach 'dev'"
	if got != want {
		t.Errorf("BuildCreateCommand() = %q, want %q", got, want)
	}
}

func TestBuildCreateCommand_WithSpecialChars(t *testing.T) {
	got := BuildCreateCommand("dev", "/home/user/project's dir")
	want := "cd '/home/user/project'\\''s dir' && " + pathPrefix + "TERM=$TERM zmx attach 'dev'"
	if got != want {
		t.Errorf("BuildCreateCommand() = %q, want %q", got, want)
	}
}

func TestBuildCreateCommand_WithSpaces(t *testing.T) {
	got := BuildCreateCommand("my session", "/home/user/my project")
	want := "cd '/home/user/my project' && " + pathPrefix + "TERM=$TERM zmx attach 'my session'"
	if got != want {
		t.Errorf("BuildCreateCommand() = %q, want %q", got, want)
	}
}

func TestBuildKillCommand(t *testing.T) {
	got := BuildKillCommand("dev")
	want := pathPrefix + "zmx kill 'dev'"
	if got != want {
		t.Errorf("BuildKillCommand() = %q, want %q", got, want)
	}
}

func TestBuildKillCommand_WithSpecialChars(t *testing.T) {
	got := BuildKillCommand("user's session")
	want := pathPrefix + "zmx kill 'user'\\''s session'"
	if got != want {
		t.Errorf("BuildKillCommand() = %q, want %q", got, want)
	}
}

func TestBuildCheckCommand(t *testing.T) {
	got := BuildCheckCommand()
	// Check command includes fallbacks for common installation paths
	want := "command -v zmx || test -x ~/.cargo/bin/zmx || test -x /opt/homebrew/bin/zmx || test -x /usr/local/bin/zmx"
	if got != want {
		t.Errorf("BuildCheckCommand() = %q, want %q", got, want)
	}
}

// Task 2: Parsing tests

func TestParseListOutput_SingleSession(t *testing.T) {
	// zmx list format: tab-separated key=value pairs (current format)
	output := "name=dev\tpid=1234\tclients=1\tstart_dir=/home/user"
	sessions := ParseListOutput(output)

	if len(sessions) != 1 {
		t.Fatalf("ParseListOutput() returned %d sessions, want 1", len(sessions))
	}
	s := sessions[0]
	if s.Name != "dev" {
		t.Errorf("Name = %q, want %q", s.Name, "dev")
	}
	if s.PID != 1234 {
		t.Errorf("PID = %d, want %d", s.PID, 1234)
	}
	if s.Clients != 1 {
		t.Errorf("Clients = %d, want %d", s.Clients, 1)
	}
	if s.StartedIn != "/home/user" {
		t.Errorf("StartedIn = %q, want %q", s.StartedIn, "/home/user")
	}
	if !s.External {
		t.Error("External = false, want true (non-ccc session)")
	}
}

func TestParseListOutput_LegacyFormat(t *testing.T) {
	// Legacy zmx format with session_name and started_in
	output := "session_name=dev\tpid=1234\tclients=1\tstarted_in=/home/user"
	sessions := ParseListOutput(output)

	if len(sessions) != 1 {
		t.Fatalf("ParseListOutput() returned %d sessions, want 1", len(sessions))
	}
	s := sessions[0]
	if s.Name != "dev" {
		t.Errorf("Name = %q, want %q", s.Name, "dev")
	}
	if s.StartedIn != "/home/user" {
		t.Errorf("StartedIn = %q, want %q", s.StartedIn, "/home/user")
	}
}

func TestParseListOutput_CCCSession(t *testing.T) {
	output := "name=ccc.myproject.main\tpid=5678\tclients=0\tstart_dir=/home/user/project"
	sessions := ParseListOutput(output)

	if len(sessions) != 1 {
		t.Fatalf("ParseListOutput() returned %d sessions, want 1", len(sessions))
	}
	s := sessions[0]
	if s.Name != "ccc.myproject.main" {
		t.Errorf("Name = %q, want %q", s.Name, "ccc.myproject.main")
	}
	if s.Project != "myproject" {
		t.Errorf("Project = %q, want %q", s.Project, "myproject")
	}
	if s.Suffix != "main" {
		t.Errorf("Suffix = %q, want %q", s.Suffix, "main")
	}
	if s.External {
		t.Error("External = true, want false")
	}
}

func TestParseListOutput_MultipleSessions(t *testing.T) {
	output := `name=ccc.proj.main	pid=1000	clients=1	start_dir=/home/user/proj
name=ccc.proj.2	pid=1001	clients=0	start_dir=/home/user/proj
name=other	pid=1002	clients=2	start_dir=/tmp`
	sessions := ParseListOutput(output)

	if len(sessions) != 3 {
		t.Fatalf("ParseListOutput() returned %d sessions, want 3", len(sessions))
	}
}

func TestParseListOutput_EmptyOutput(t *testing.T) {
	sessions := ParseListOutput("")
	if sessions != nil {
		t.Errorf("ParseListOutput(\"\") = %v, want nil", sessions)
	}
}

func TestParseListOutput_WhitespaceOnly(t *testing.T) {
	sessions := ParseListOutput("   \n\t\n  ")
	if sessions != nil {
		t.Errorf("ParseListOutput(whitespace) = %v, want nil", sessions)
	}
}

func TestParseListOutput_SkipsMalformedLines(t *testing.T) {
	output := `session_name=valid	pid=1234	clients=1	started_in=/home
malformed line without proper format
session_name=also-valid	pid=5678	clients=0	started_in=/tmp`
	sessions := ParseListOutput(output)

	if len(sessions) != 2 {
		t.Fatalf("ParseListOutput() returned %d sessions, want 2", len(sessions))
	}
	if sessions[0].Name != "valid" {
		t.Errorf("First session Name = %q, want %q", sessions[0].Name, "valid")
	}
	if sessions[1].Name != "also-valid" {
		t.Errorf("Second session Name = %q, want %q", sessions[1].Name, "also-valid")
	}
}

func TestParseSessionName_CCC(t *testing.T) {
	tests := []struct {
		name    string
		want    Session
	}{
		{
			name: "ccc.myproject.main",
			want: Session{Name: "ccc.myproject.main", Project: "myproject", Suffix: "main", External: false},
		},
		{
			name: "ccc.rt1.2",
			want: Session{Name: "ccc.rt1.2", Project: "rt1", Suffix: "2", External: false},
		},
		{
			name: "ccc.my-proj.dev",
			want: Session{Name: "ccc.my-proj.dev", Project: "my-proj", Suffix: "dev", External: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseSessionName(tt.name)
			if got.Name != tt.want.Name {
				t.Errorf("Name = %q, want %q", got.Name, tt.want.Name)
			}
			if got.Project != tt.want.Project {
				t.Errorf("Project = %q, want %q", got.Project, tt.want.Project)
			}
			if got.Suffix != tt.want.Suffix {
				t.Errorf("Suffix = %q, want %q", got.Suffix, tt.want.Suffix)
			}
			if got.External != tt.want.External {
				t.Errorf("External = %v, want %v", got.External, tt.want.External)
			}
		})
	}
}

func TestParseSessionName_External(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "dev"},
		{name: "my-session"},
		{name: "ccc"}, // just "ccc" without proper format
		{name: "ccc."},
		{name: "ccc.project"}, // missing suffix
		{name: "other.project.main"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseSessionName(tt.name)
			if !got.External {
				t.Errorf("External = false, want true for %q", tt.name)
			}
			if got.Name != tt.name {
				t.Errorf("Name = %q, want %q", got.Name, tt.name)
			}
		})
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

func TestNextAutoName_EmptySlice(t *testing.T) {
	got := NextAutoName("rt1", []Session{})
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
