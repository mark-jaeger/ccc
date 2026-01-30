package scan

import (
	"strings"
	"testing"
)

func TestBuildMdfindCommand(t *testing.T) {
	cmd := buildMdfindCommand("/Users/mark")

	if !strings.Contains(cmd, "mdfind") {
		t.Errorf("expected command to contain 'mdfind', got %q", cmd)
	}
	if !strings.Contains(cmd, "/Users/mark") {
		t.Errorf("expected command to contain homeDir '/Users/mark', got %q", cmd)
	}
}

func TestBuildLocateCommand(t *testing.T) {
	cmd := buildLocateCommand("/Users/mark")

	if !strings.Contains(cmd, "locate") && !strings.Contains(cmd, "plocate") {
		t.Errorf("expected command to contain 'locate' or 'plocate', got %q", cmd)
	}
}

func TestBuildFdCommand(t *testing.T) {
	cmd := buildFdCommand("/Users/mark")

	if !strings.Contains(cmd, "fd") {
		t.Errorf("expected command to contain 'fd', got %q", cmd)
	}
}

func TestBuildFindCommand(t *testing.T) {
	cmd := buildFindCommand("/Users/mark")

	if !strings.Contains(cmd, "find") {
		t.Errorf("expected command to contain 'find', got %q", cmd)
	}
	if !strings.Contains(cmd, "maxdepth") {
		t.Errorf("expected command to contain 'maxdepth', got %q", cmd)
	}
}

func TestBuildScanChainCommand(t *testing.T) {
	cmd := BuildScanChainCommand("/Users/mark")

	if !strings.Contains(cmd, "mdfind") {
		t.Errorf("expected chain to contain 'mdfind' (first choice), got %q", cmd)
	}
	if !strings.Contains(cmd, "find") {
		t.Errorf("expected chain to contain 'find' (fallback), got %q", cmd)
	}
}

func TestParseScanResults(t *testing.T) {
	input := "/Users/mark/Projects/jd/ccc/.git\n" +
		"/Users/mark/Projects/foo/.git\n" +
		"/Users/mark/Projects/bar/.git\n"

	results := ParseScanResults(input)

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	expected := []struct {
		path string
		name string
	}{
		{"/Users/mark/Projects/jd/ccc", "ccc"},
		{"/Users/mark/Projects/foo", "foo"},
		{"/Users/mark/Projects/bar", "bar"},
	}

	for i, exp := range expected {
		if results[i].Path != exp.path {
			t.Errorf("results[%d].Path = %q, want %q", i, results[i].Path, exp.path)
		}
		if results[i].Name != exp.name {
			t.Errorf("results[%d].Name = %q, want %q", i, results[i].Name, exp.name)
		}
	}
}

func TestParseScanResultsEmpty(t *testing.T) {
	results := ParseScanResults("")
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty input, got %d", len(results))
	}
}

func TestParseScanResultsDedup(t *testing.T) {
	input := "/Users/mark/Projects/foo/.git\n" +
		"/Users/mark/Projects/foo/.git\n"

	results := ParseScanResults(input)

	if len(results) != 1 {
		t.Fatalf("expected 1 deduplicated result, got %d", len(results))
	}
	if results[0].Path != "/Users/mark/Projects/foo" {
		t.Errorf("Path = %q, want %q", results[0].Path, "/Users/mark/Projects/foo")
	}
}

func TestDeriveProjectKey(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/Users/mark/Projects/jd/rt1", "rt1"},
		{"/Users/mark/Projects/death_and_taxes", "death-and-taxes"},
		{"/home/user/My Project", "my-project"},
	}

	for _, tc := range tests {
		got := DeriveProjectKey(tc.path)
		if got != tc.want {
			t.Errorf("DeriveProjectKey(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

func TestDeriveProjectKeySpecialChars(t *testing.T) {
	got := DeriveProjectKey("/home/user/@project.name!")
	if got != "projectname" {
		t.Errorf("DeriveProjectKey with special chars = %q, want %q", got, "projectname")
	}
}

func TestDeriveProjectKeyLeadingHyphens(t *testing.T) {
	got := DeriveProjectKey("/home/user/---my-project---")
	if got != "my-project" {
		t.Errorf("DeriveProjectKey with leading/trailing hyphens = %q, want %q", got, "my-project")
	}
}

func TestBuildScanCommandsQuoteSpaces(t *testing.T) {
	cmd := BuildScanChainCommand("/Users/mark/My Projects")

	// The path with spaces should be properly quoted
	if !strings.Contains(cmd, "'/Users/mark/My Projects'") {
		t.Errorf("expected quoted path with spaces in command: %s", cmd)
	}
}
