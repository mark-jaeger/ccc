package zmx

import (
	"strings"
	"testing"
)

// TestPathPrefixDoesNotShadowUserPath guards against a PATH-shadowing bug:
// zmx is distributed via Homebrew/tarball, NOT as a cargo crate. Prepending
// hardcoded directories (especially ~/.cargo/bin) ahead of the user's existing
// $PATH can resolve an unrelated same-named binary instead of the real zmx.
// The prefix must only APPEND fallbacks so the user's own resolution wins.
func TestPathPrefixDoesNotShadowUserPath(t *testing.T) {
	if strings.Contains(pathPrefix, ".cargo/bin") {
		t.Errorf("pathPrefix references .cargo/bin (zmx is not a cargo crate, this shadows the real zmx): %q", pathPrefix)
	}

	pathIdx := strings.Index(pathPrefix, "$PATH")
	if pathIdx == -1 {
		t.Fatalf("pathPrefix must include $PATH so the user's resolution is preserved: %q", pathPrefix)
	}
	// Any hardcoded fallback dir must appear AFTER $PATH, never before it.
	for _, dir := range []string{"/opt/homebrew/bin", "/usr/local/bin"} {
		if idx := strings.Index(pathPrefix, dir); idx != -1 && idx < pathIdx {
			t.Errorf("fallback %q appears before $PATH and shadows the user's zmx: %q", dir, pathPrefix)
		}
	}
}

// Task 1: Command builder tests

func TestBuildListCommand(t *testing.T) {
	got := BuildListCommand()
	want := envPrefix + "zmx list"
	if got != want {
		t.Errorf("BuildListCommand() = %q, want %q", got, want)
	}
}

func TestBuildAttachCommand(t *testing.T) {
	got := BuildAttachCommand("dev")
	want := envPrefix + "TERM=$TERM zmx attach 'dev'"
	if got != want {
		t.Errorf("BuildAttachCommand() = %q, want %q", got, want)
	}
}

func TestBuildAttachCommand_WithSpecialChars(t *testing.T) {
	got := BuildAttachCommand("my session")
	want := envPrefix + "TERM=$TERM zmx attach 'my session'"
	if got != want {
		t.Errorf("BuildAttachCommand() = %q, want %q", got, want)
	}
}

func TestBuildAttachCommand_WithQuotes(t *testing.T) {
	got := BuildAttachCommand("user's session")
	want := envPrefix + "TERM=$TERM zmx attach 'user'\\''s session'"
	if got != want {
		t.Errorf("BuildAttachCommand() = %q, want %q", got, want)
	}
}

func TestBuildCreateCommand(t *testing.T) {
	got := BuildCreateCommand("dev", "/home/user/project")
	want := "cd '/home/user/project' && " + envPrefix + "TERM=$TERM zmx attach 'dev'"
	if got != want {
		t.Errorf("BuildCreateCommand() = %q, want %q", got, want)
	}
}

func TestBuildCreateCommand_WithSpecialChars(t *testing.T) {
	got := BuildCreateCommand("dev", "/home/user/project's dir")
	want := "cd '/home/user/project'\\''s dir' && " + envPrefix + "TERM=$TERM zmx attach 'dev'"
	if got != want {
		t.Errorf("BuildCreateCommand() = %q, want %q", got, want)
	}
}

func TestBuildCreateCommand_WithSpaces(t *testing.T) {
	got := BuildCreateCommand("my session", "/home/user/my project")
	want := "cd '/home/user/my project' && " + envPrefix + "TERM=$TERM zmx attach 'my session'"
	if got != want {
		t.Errorf("BuildCreateCommand() = %q, want %q", got, want)
	}
}

func TestBuildKillCommand(t *testing.T) {
	got := BuildKillCommand("dev")
	want := envPrefix + "zmx kill 'dev'"
	if got != want {
		t.Errorf("BuildKillCommand() = %q, want %q", got, want)
	}
}

func TestBuildKillCommand_WithSpecialChars(t *testing.T) {
	got := BuildKillCommand("user's session")
	want := envPrefix + "zmx kill 'user'\\''s session'"
	if got != want {
		t.Errorf("BuildKillCommand() = %q, want %q", got, want)
	}
}

func TestBuildCheckCommand(t *testing.T) {
	got := BuildCheckCommand()
	// Check command includes fallbacks for common installation paths
	// Each fallback outputs the path on success (required by CheckZmx)
	want := "command -v zmx || { test -x ~/.cargo/bin/zmx && echo ~/.cargo/bin/zmx; } || { test -x /opt/homebrew/bin/zmx && echo /opt/homebrew/bin/zmx; } || { test -x /usr/local/bin/zmx && echo /usr/local/bin/zmx; }"
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

// TestSessionNameEncodesProjectKey verifies arbitrary project keys are encoded
// into a session name that zmx can use as a socket filename. The key guarantee
// is that a '/' in the project key never reaches the name (a '/' there makes zmx
// exit 1 trying to create a socket under a non-existent subdirectory), and that
// the structural '.' delimiter is likewise escaped.
func TestSessionNameEncodesProjectKey(t *testing.T) {
	tests := []struct {
		name    string
		project string
		suffix  string
		want    string
	}{
		{"plain key unchanged", "rt1", "main", "ccc.rt1.main"},
		{"slash encoded", "projects/tmp", "main", "ccc.projects%2Ftmp.main"},
		{"dot encoded", "a.b", "main", "ccc.a%2Eb.main"},
		{"percent encoded", "50%", "main", "ccc.50%25.main"},
		{"combined", "a/b.c%d", "2", "ccc.a%2Fb%2Ec%25d.2"},
		{"hyphen workaround stays distinct", "projects-tmp", "main", "ccc.projects-tmp.main"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SessionName(tt.project, tt.suffix)
			if got != tt.want {
				t.Errorf("SessionName(%q, %q) = %q, want %q", tt.project, tt.suffix, got, tt.want)
			}
			if strings.Contains(got, "/") {
				t.Errorf("SessionName(%q, %q) = %q contains '/', which breaks zmx socket creation", tt.project, tt.suffix, got)
			}
		})
	}
}

// TestSessionNameRoundTrip verifies that SessionName and parseSessionName are
// inverses for arbitrary project keys: Session.Project must equal the original
// key so FilterSessionsForProject (which compares against the raw key) still
// groups sessions under the right project, and distinct keys must never collide.
func TestSessionNameRoundTrip(t *testing.T) {
	keys := []string{
		"rt1",
		"projects/tmp",
		"projects-tmp", // the user's workaround: must stay distinct from projects/tmp
		"a.b.c",
		"weird %2F literal",
		"deep/nested/path",
		"unicode-café/项目",
	}
	seen := map[string]string{} // encoded name -> original key, to catch collisions
	for _, k := range keys {
		name := SessionName(k, "main")
		if strings.Contains(name, "/") {
			t.Errorf("SessionName(%q) = %q contains '/'", k, name)
		}
		if prev, ok := seen[name]; ok && prev != k {
			t.Errorf("collision: keys %q and %q both encode to %q", prev, k, name)
		}
		seen[name] = k

		s := parseSessionName(name)
		if s.External {
			t.Errorf("parseSessionName(%q) marked External", name)
		}
		if s.Project != k {
			t.Errorf("round-trip project = %q, want %q (name %q)", s.Project, k, name)
		}
		if s.Suffix != "main" {
			t.Errorf("round-trip suffix = %q, want %q (name %q)", s.Suffix, "main", name)
		}
	}
}

// TestDisplayName verifies the UI name is the decoded form for ccc sessions and
// verbatim for external ones, and that a plain (unencoded) name is unchanged.
func TestDisplayName(t *testing.T) {
	tests := []struct {
		name string
		sess Session
		want string
	}{
		{"plain ccc", parseSessionName("ccc.rt1.main"), "ccc.rt1.main"},
		{"encoded ccc decodes", parseSessionName(SessionName("projects/tmp", "main")), "ccc.projects/tmp.main"},
		{"external verbatim", parseSessionName("some-external"), "some-external"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.sess.DisplayName(); got != tt.want {
				t.Errorf("DisplayName() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestNextAutoNameEncodesProjectKey verifies the auto-name path (used by both the
// TUI and the CLI flow) also encodes the project key.
func TestNextAutoNameEncodesProjectKey(t *testing.T) {
	got := NextAutoName("projects/tmp", nil)
	want := "ccc.projects%2Ftmp.main"
	if got != want {
		t.Errorf("NextAutoName(%q) = %q, want %q", "projects/tmp", got, want)
	}
	if strings.Contains(got, "/") {
		t.Errorf("NextAutoName(%q) = %q contains '/'", "projects/tmp", got)
	}
}

// TestEncodeDecodeProjectTokenRoundTrip exercises the token codec directly,
// including a key that already contains a literal percent-escape sequence.
func TestEncodeDecodeProjectTokenRoundTrip(t *testing.T) {
	inputs := []string{
		"",
		"plain",
		"/",
		".",
		"%",
		"%2F",   // literal, must not be mistaken for an encoded slash
		"a/b.c", // combined
		"trailing%",
	}
	for _, in := range inputs {
		enc := encodeProjectToken(in)
		if strings.ContainsAny(enc, "/.") {
			t.Errorf("encodeProjectToken(%q) = %q still contains '/' or '.'", in, enc)
		}
		if dec := decodeProjectToken(enc); dec != in {
			t.Errorf("round-trip failed: %q -> %q -> %q", in, enc, dec)
		}
	}
}
