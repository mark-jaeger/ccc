// Package zmx provides session listing, creation, attachment, and management
// for zmx terminal sessions. zmx provides transparent PTY passthrough with
// automatic terminal state restoration on reattach.
package zmx

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mark-jaeger/ccc/internal/shellutil"
)

// InstallMessage is the canonical, user-facing instruction shown when zmx is
// not found on a target. zmx is a Zig tool distributed as a prebuilt tarball
// (and via Homebrew on macOS) — it is NOT a Rust crate, so "cargo install zmx"
// does not work. The leading "zmx not found" line is matched by the TUI to
// render this as a dedicated help screen, so keep it first.
const InstallMessage = `zmx not found

Install zmx for your platform:

  macOS:  brew install neurosnap/tap/zmx
  Linux:  curl -L https://zmx.sh/a/zmx-0.4.1-linux-x86_64.tar.gz | tar xz && sudo mv zmx /usr/local/bin/

See https://zmx.sh (other versions/arches) and https://github.com/neurosnap/zmx for details.`

// pathPrefix APPENDS common zmx installation directories to PATH as fallbacks.
// This ensures zmx is found even when Homebrew/tarball paths are not in PATH
// for non-interactive SSH sessions (which only source .bash_profile/.zprofile,
// not .bashrc/.zshrc where PATH is often set).
//
// Fallbacks are appended AFTER $PATH, never prepended: prepending would let a
// same-named binary in one of these dirs shadow the user's correct zmx. zmx is
// a Zig tool distributed via Homebrew/tarball — it is NOT a cargo crate, so
// ~/.cargo/bin is deliberately excluded (an unrelated ~/.cargo/bin/zmx must
// never preempt the real one).
const pathPrefix = "PATH=\"$PATH:/opt/homebrew/bin:/usr/local/bin\" "

// Session represents a zmx session with parsed metadata.
type Session struct {
	Name      string // full session name, e.g., "ccc.rt1.main"
	PID       int    // process ID
	Clients   int    // number of attached clients
	StartedIn string // directory where session started
	Project   string // extracted project key, empty if External
	Suffix    string // extracted suffix, empty if External
	External  bool   // true if not prefixed with "ccc."
}

// zmxDirPrefix sets ZMX_DIR to a consistent path so that SSH sessions
// (where XDG_RUNTIME_DIR is unset) and local sessions (where systemd sets
// XDG_RUNTIME_DIR=/run/user/$UID) use the same socket directory.
// Respects an existing ZMX_DIR if already set.
const zmxDirPrefix = "ZMX_DIR=${ZMX_DIR:-/tmp/zmx-$(id -u)} "

// envPrefix combines ZMX_DIR and PATH settings for all zmx commands.
const envPrefix = zmxDirPrefix + pathPrefix

// BuildListCommand returns a shell command that lists all zmx sessions.
func BuildListCommand() string {
	return envPrefix + "zmx list"
}

// BuildAttachCommand returns a shell command to attach to a named session.
// Uses TERM=$TERM prefix to ensure terminal type passthrough over SSH.
func BuildAttachCommand(name string) string {
	return fmt.Sprintf(envPrefix+"TERM=$TERM zmx attach %s", shellutil.Quote(name))
}

// BuildCreateCommand returns a shell command that creates a new zmx session.
// Since zmx attach creates if not exists, this is the same as attach but
// with a cd to the specified path first.
// Uses TERM=$TERM prefix to ensure terminal type passthrough over SSH.
func BuildCreateCommand(name, path string) string {
	return fmt.Sprintf("cd %s && "+envPrefix+"TERM=$TERM zmx attach %s",
		shellutil.Quote(path), shellutil.Quote(name))
}

// BuildKillCommand returns a shell command to kill a session by name.
// Unlike abduco which uses PID, zmx uses the session name directly.
func BuildKillCommand(name string) string {
	return fmt.Sprintf(envPrefix+"zmx kill %s", shellutil.Quote(name))
}

// BuildCheckCommand returns a shell command to check if zmx is installed.
// It checks both the standard PATH and common installation locations like
// ~/.cargo/bin (for cargo install) and Homebrew paths, since non-interactive
// SSH sessions may not have these in PATH even when zmx is installed.
// The command outputs the path to zmx if found (required by CheckZmx).
func BuildCheckCommand() string {
	// Check standard PATH first, then common installation directories.
	// This handles cases where ~/.cargo/bin is only added to PATH in
	// .bashrc (not sourced by non-interactive login shells).
	// Each check outputs the path on success since CheckZmx requires non-empty output.
	return "command -v zmx || { test -x ~/.cargo/bin/zmx && echo ~/.cargo/bin/zmx; } || { test -x /opt/homebrew/bin/zmx && echo /opt/homebrew/bin/zmx; } || { test -x /usr/local/bin/zmx && echo /usr/local/bin/zmx; }"
}

// ParseListOutput parses the output of zmx list into a slice of Session values.
// zmx list output format: tab-separated key=value pairs per line
// Format: name=<name>\tpid=<pid>\tclients=<count>\tstart_dir=<dir>
// Also accepts legacy format: session_name=<name>\t...\tstarted_in=<dir>
// Returns nil for empty input.
func ParseListOutput(output string) []Session {
	if strings.TrimSpace(output) == "" {
		return nil
	}

	lines := strings.Split(output, "\n")
	var sessions []Session

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		s, ok := parseListLine(line)
		if !ok {
			continue
		}

		sessions = append(sessions, s)
	}

	return sessions
}

// parseListLine parses a single line of zmx list output.
// Returns the session and true if parsing succeeded, or zero value and false otherwise.
func parseListLine(line string) (Session, bool) {
	fields := strings.Split(line, "\t")

	var name, startedIn string
	var pid, clients int

	for _, field := range fields {
		parts := strings.SplitN(field, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, value := parts[0], parts[1]

		switch key {
		case "session_name", "name":
			name = value
		case "pid":
			pid, _ = strconv.Atoi(value)
		case "clients":
			clients, _ = strconv.Atoi(value)
		case "started_in", "start_dir":
			startedIn = value
		}
	}

	// Require at least a session name
	if name == "" {
		return Session{}, false
	}

	s := parseSessionName(name)
	s.PID = pid
	s.Clients = clients
	s.StartedIn = startedIn

	return s, true
}

// parseSessionName extracts project and suffix from a session name.
// Format: ccc.{project}.{suffix}
// Non-ccc sessions are marked as External.
func parseSessionName(name string) Session {
	if !strings.HasPrefix(name, "ccc.") {
		return Session{Name: name, External: true}
	}

	parts := strings.SplitN(name[4:], ".", 2) // skip "ccc."
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return Session{Name: name, External: true}
	}

	return Session{
		Name:    name,
		Project: parts[0],
		Suffix:  parts[1],
	}
}

// FilterSessionsForProject returns sessions belonging to a project.
// Sessions with matching Project are included, as are External sessions
// (for visibility into all zmx sessions on the machine).
func FilterSessionsForProject(sessions []Session, projectKey string) []Session {
	var result []Session
	for _, s := range sessions {
		if s.Project == projectKey || s.External {
			result = append(result, s)
		}
	}
	return result
}

// NextAutoName returns the next session name for a project.
// The first session gets suffix "main", subsequent sessions get 2, 3, 4...
// Format: ccc.{project}.{suffix}
func NextAutoName(projectKey string, existing []Session) string {
	prefix := "ccc." + projectKey + "."

	if len(existing) == 0 {
		return prefix + "main"
	}

	// Check if "main" suffix already exists
	hasMain := false
	maxNum := 1 // Start from 1 since "main" counts as first
	for _, s := range existing {
		if s.Suffix == "main" {
			hasMain = true
		}
		if n, err := strconv.Atoi(s.Suffix); err == nil && n > maxNum {
			maxNum = n
		}
	}

	if !hasMain {
		return prefix + "main"
	}

	return prefix + strconv.Itoa(maxNum+1)
}
