package tmux

import (
	"strings"
	"testing"
)

func TestParseSessionList(t *testing.T) {
	output := "rt1|||myproject|||/home/user/proj|||3\n" +
		"rt2|||otherproj|||/home/user/other|||1\n" +
		"untagged||||||/tmp|||2\n"

	sessions := ParseSessionList(output)

	if len(sessions) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(sessions))
	}

	// First session: tagged with project
	if sessions[0].Name != "rt1" {
		t.Errorf("expected Name rt1, got %q", sessions[0].Name)
	}
	if sessions[0].Project != "myproject" {
		t.Errorf("expected Project myproject, got %q", sessions[0].Project)
	}
	if sessions[0].Path != "/home/user/proj" {
		t.Errorf("expected Path /home/user/proj, got %q", sessions[0].Path)
	}
	if sessions[0].Windows != 3 {
		t.Errorf("expected Windows 3, got %d", sessions[0].Windows)
	}

	// Second session: tagged with different project
	if sessions[1].Name != "rt2" {
		t.Errorf("expected Name rt2, got %q", sessions[1].Name)
	}
	if sessions[1].Project != "otherproj" {
		t.Errorf("expected Project otherproj, got %q", sessions[1].Project)
	}
	if sessions[1].Path != "/home/user/other" {
		t.Errorf("expected Path /home/user/other, got %q", sessions[1].Path)
	}
	if sessions[1].Windows != 1 {
		t.Errorf("expected Windows 1, got %d", sessions[1].Windows)
	}

	// Third session: untagged (empty project)
	if sessions[2].Name != "untagged" {
		t.Errorf("expected Name untagged, got %q", sessions[2].Name)
	}
	if sessions[2].Project != "" {
		t.Errorf("expected empty Project, got %q", sessions[2].Project)
	}
	if sessions[2].Path != "/tmp" {
		t.Errorf("expected Path /tmp, got %q", sessions[2].Path)
	}
	if sessions[2].Windows != 2 {
		t.Errorf("expected Windows 2, got %d", sessions[2].Windows)
	}
}

func TestFilterSessionsForProject(t *testing.T) {
	sessions := []Session{
		{Name: "rt1", Project: "rt1", Path: "/home/user/proj", Windows: 3},
		{Name: "rt1-2", Project: "rt1", Path: "/home/user/proj", Windows: 1},
		{Name: "untagged-rt1", Project: "", Path: "/tmp", Windows: 2},
		{Name: "rt1-extra", Project: "", Path: "/tmp", Windows: 1},
		{Name: "other", Project: "other", Path: "/tmp", Windows: 1},
	}

	filtered := FilterSessionsForProject(sessions, "rt1")

	// Should include:
	// - rt1 (metadata match, verified=true)
	// - rt1-2 (metadata match, verified=true)
	// - rt1-extra (prefix match: starts with "rt1-", verified=false)
	// Should NOT include:
	// - untagged-rt1 (name doesn't match prefix "rt1" or "rt1-*")
	// - other (different project)

	if len(filtered) != 3 {
		t.Fatalf("expected 3 filtered sessions, got %d: %+v", len(filtered), filtered)
	}

	// Check metadata matches are verified
	found := map[string]bool{}
	for _, s := range filtered {
		found[s.Name] = true
		switch s.Name {
		case "rt1":
			if !s.Verified {
				t.Errorf("rt1 should be verified (metadata match)")
			}
		case "rt1-2":
			if !s.Verified {
				t.Errorf("rt1-2 should be verified (metadata match)")
			}
		case "rt1-extra":
			if s.Verified {
				t.Errorf("rt1-extra should NOT be verified (prefix match only)")
			}
		default:
			t.Errorf("unexpected session %q in filtered results", s.Name)
		}
	}

	if !found["rt1"] {
		t.Error("expected rt1 in filtered results")
	}
	if !found["rt1-2"] {
		t.Error("expected rt1-2 in filtered results")
	}
	if !found["rt1-extra"] {
		t.Error("expected rt1-extra in filtered results")
	}
}

func TestParseEmptyOutput(t *testing.T) {
	sessions := ParseSessionList("")
	if sessions != nil {
		t.Errorf("expected nil for empty output, got %v", sessions)
	}

	sessions = ParseSessionList("\n")
	if sessions != nil {
		t.Errorf("expected nil for newline-only output, got %v", sessions)
	}
}

func TestBuildCreateCommand(t *testing.T) {
	cmd := BuildCreateCommand("rt1", "/home/user/proj", "rt1")

	expected := "tmux new-session -d -s 'rt1' -c '/home/user/proj' \\; set-option -t 'rt1' @ccc_project 'rt1' \\; set-option -t 'rt1' @ccc_path '/home/user/proj' \\; set-option -t 'rt1' bell-action any \\; set-window-option -t 'rt1' visual-bell off"
	if cmd != expected {
		t.Errorf("expected:\n  %s\ngot:\n  %s", expected, cmd)
	}
}

func TestBuildAttachCommand(t *testing.T) {
	cmd := BuildAttachCommand("rt1")

	expected := "tmux attach -t 'rt1'"
	if cmd != expected {
		t.Errorf("expected %q, got %q", expected, cmd)
	}
}

func TestBuildCreateCommandWithSpaces(t *testing.T) {
	cmd := BuildCreateCommand("my-project", "/Users/mark/My Project", "my-project")

	// Verify the path is properly quoted
	if !strings.Contains(cmd, "'/Users/mark/My Project'") {
		t.Errorf("path with spaces not properly quoted in: %s", cmd)
	}
}

func TestBuildSetPassthroughCommand(t *testing.T) {
	cmd := BuildSetPassthroughCommand("rt1")
	expected := "tmux set-window-option -t 'rt1' allow-passthrough on"
	if cmd != expected {
		t.Errorf("expected:\n  %s\ngot:\n  %s", expected, cmd)
	}
}

func TestBuildEnsureNotifyOptionsCommand(t *testing.T) {
	cmd := BuildEnsureNotifyOptionsCommand("rt1")
	// Should set bell-action and visual-bell in one compound command,
	// then allow-passthrough separately (tmux < 3.3 compat), ending with "true".
	if !strings.Contains(cmd, "bell-action any") {
		t.Errorf("expected bell-action any in: %s", cmd)
	}
	if !strings.Contains(cmd, "visual-bell off") {
		t.Errorf("expected visual-bell off in: %s", cmd)
	}
	if !strings.Contains(cmd, "allow-passthrough on") {
		t.Errorf("expected allow-passthrough on in: %s", cmd)
	}
	if !strings.Contains(cmd, "2>/dev/null") {
		t.Errorf("expected 2>/dev/null for backward compat in: %s", cmd)
	}
	// Must end with "true" so the overall command succeeds even if passthrough fails
	if !strings.HasSuffix(strings.TrimSpace(cmd), "true") {
		t.Errorf("expected command to end with 'true' for backward compat in: %s", cmd)
	}
}

func TestBuildListCommand(t *testing.T) {
	cmd := BuildListCommand()
	if cmd == "" {
		t.Error("expected non-empty list command")
	}
}

func TestNextSessionName(t *testing.T) {
	existing := []Session{
		{Name: "rt1", Project: "rt1"},
		{Name: "rt1-2", Project: "rt1"},
	}

	name := NextAutoName("rt1", existing)
	if name != "rt1-3" {
		t.Errorf("expected rt1-3, got %q", name)
	}
}

func TestNextSessionNameFirst(t *testing.T) {
	name := NextAutoName("rt1", nil)
	if name != "rt1" {
		t.Errorf("expected rt1, got %q", name)
	}
}

func TestParseClientList(t *testing.T) {
	output := "/dev/ttys004: 220x56 0\n"

	clients := ParseClientList(output)

	if len(clients) != 1 {
		t.Fatalf("expected 1 client, got %d", len(clients))
	}

	if clients[0].TTY != "/dev/ttys004" {
		t.Errorf("expected TTY /dev/ttys004, got %q", clients[0].TTY)
	}
	if clients[0].Width != 220 {
		t.Errorf("expected Width 220, got %d", clients[0].Width)
	}
	if clients[0].Height != 56 {
		t.Errorf("expected Height 56, got %d", clients[0].Height)
	}
}

func TestParseClientListEmpty(t *testing.T) {
	clients := ParseClientList("")
	if clients != nil {
		t.Errorf("expected nil for empty output, got %v", clients)
	}
}

func TestParseClientListMultiple(t *testing.T) {
	output := "/dev/ttys004: 220x56 0\n/dev/ttys005: 120x30 0\n"

	clients := ParseClientList(output)
	if len(clients) != 2 {
		t.Fatalf("expected 2 clients, got %d", len(clients))
	}
	if clients[0].Width != 220 || clients[0].Height != 56 {
		t.Errorf("client[0] = %dx%d, want 220x56", clients[0].Width, clients[0].Height)
	}
	if clients[1].Width != 120 || clients[1].Height != 30 {
		t.Errorf("client[1] = %dx%d, want 120x30", clients[1].Width, clients[1].Height)
	}
}

func TestParseClientListMalformed(t *testing.T) {
	output := "no-colon-line\n/dev/ttys004: 220x56 0\nbadline\n"

	clients := ParseClientList(output)
	if len(clients) != 1 {
		t.Fatalf("expected 1 valid client (malformed lines skipped), got %d", len(clients))
	}
	if clients[0].TTY != "/dev/ttys004" {
		t.Errorf("expected TTY /dev/ttys004, got %q", clients[0].TTY)
	}
}

func TestNextAutoNameWithGap(t *testing.T) {
	existing := []Session{
		{Name: "rt1", Project: "rt1"},
		{Name: "rt1-5", Project: "rt1"},
	}

	name := NextAutoName("rt1", existing)
	if name != "rt1-6" {
		t.Errorf("expected rt1-6 (maxNum=5 → next=6), got %q", name)
	}
}

func TestNextAutoNameNonNumericSuffix(t *testing.T) {
	existing := []Session{
		{Name: "rt1", Project: "rt1"},
		{Name: "rt1-beta", Project: "rt1"},
	}

	// "rt1-beta" has non-numeric suffix → ignored by NextAutoName
	name := NextAutoName("rt1", existing)
	if name != "rt1-2" {
		t.Errorf("expected rt1-2 (non-numeric suffix ignored), got %q", name)
	}
}

func TestBuildListCommand_WithSocketOverride(t *testing.T) {
	SocketOverride = "test-socket"
	defer func() { SocketOverride = "" }()

	cmd := BuildListCommand()
	if !strings.Contains(cmd, "-L test-socket") {
		t.Errorf("expected -L test-socket in command, got: %s", cmd)
	}
}

func TestBuildCreateCommand_WithSocketOverride(t *testing.T) {
	SocketOverride = "test-socket"
	defer func() { SocketOverride = "" }()

	cmd := BuildCreateCommand("rt1", "/tmp", "rt1")
	if !strings.Contains(cmd, "-L test-socket") {
		t.Errorf("expected -L test-socket in command, got: %s", cmd)
	}
}

func TestFilterSessionsExactNameUntagged(t *testing.T) {
	sessions := []Session{
		{Name: "myapp", Project: "", Path: "/tmp", Windows: 1},
	}

	filtered := FilterSessionsForProject(sessions, "myapp")
	if len(filtered) != 1 {
		t.Fatalf("expected 1 session, got %d", len(filtered))
	}
	if filtered[0].Verified {
		t.Error("untagged session matched by name should have Verified=false")
	}
}
