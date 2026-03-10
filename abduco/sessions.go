// Package abduco provides session listing, creation, attachment, and management
// for abduco terminal sessions. Unlike tmux, abduco provides transparent PTY
// passthrough with automatic client detachment.
package abduco

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/mark-jaeger/ccc/internal/shellutil"
)

// sessionLineRe matches abduco session output lines.
// Format: status space timestamp\tPID\tname
// Status: * = attached, + = dead, space = detached
// The timestamp format varies by locale, so we use tab delimiters as primary separators.
var sessionLineRe = regexp.MustCompile(`^([*+ ])\s+\S+\s+\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}\t(\d+)\t(.+)$`)

// Session represents an abduco session with parsed metadata.
type Session struct {
	Name     string // full session name, e.g., "ccc.rt1.main"
	Project  string // extracted project key, empty if External
	Suffix   string // extracted suffix, empty if External
	External bool   // true if not prefixed with "ccc."
	Dead     bool   // true if status is "+"
	PID      int    // process ID for kill command
}

// BuildCreateCommand returns a shell command that creates a new abduco session.
// The session starts the user's default shell ($SHELL) in the specified path.
// If detachKey is non-empty, it's passed as the -e flag (e.g., "^a" for Ctrl+a).
func BuildCreateCommand(name, path, detachKey string) string {
	if detachKey != "" {
		return fmt.Sprintf("cd %s && abduco -e %s -n %s $SHELL",
			shellutil.Quote(path), shellutil.Quote(detachKey), shellutil.Quote(name))
	}
	return fmt.Sprintf("cd %s && abduco -n %s $SHELL",
		shellutil.Quote(path), shellutil.Quote(name))
}

// BuildAttachCommand returns a shell command to attach to a named session.
// If detachKey is non-empty, it's passed as the -e flag (e.g., "^a" for Ctrl+a).
func BuildAttachCommand(name, detachKey string) string {
	if detachKey != "" {
		return fmt.Sprintf("abduco -e %s -a %s", shellutil.Quote(detachKey), shellutil.Quote(name))
	}
	return fmt.Sprintf("abduco -a %s", shellutil.Quote(name))
}

// BuildListCommand returns a shell command that lists all abduco sessions.
// Output is redirected from stderr to stdout since abduco writes to stderr.
func BuildListCommand() string {
	return "abduco 2>&1 || true"
}

// BuildKillCommand returns a shell command to kill a session by PID.
// Using PID is safer than pkill which can match too broadly.
func BuildKillCommand(pid int) string {
	return fmt.Sprintf("kill %d", pid)
}

// BuildCheckCommand returns a shell command to check if abduco is installed.
func BuildCheckCommand() string {
	return "command -v abduco"
}

// ParseSessionList parses the output of abduco (BuildListCommand) into
// a slice of Session values. Returns nil for empty input.
func ParseSessionList(output string) []Session {
	if strings.TrimSpace(output) == "" {
		return nil
	}

	lines := strings.Split(output, "\n")
	var sessions []Session

	for _, line := range lines {
		// Don't trim leading whitespace - the status character may be a space
		line = strings.TrimRight(line, "\r\n")
		matches := sessionLineRe.FindStringSubmatch(line)
		if len(matches) != 4 {
			continue
		}

		status := matches[1]
		pid, _ := strconv.Atoi(matches[2])
		name := matches[3]

		s := parseSessionName(name)
		s.Dead = status == "+"
		s.PID = pid
		sessions = append(sessions, s)
	}

	return sessions
}

// parseSessionName extracts project and suffix from a session name.
// Format: ccc.{project}.{suffix}
// Non-ccc sessions are marked as External.
func parseSessionName(name string) Session {
	if !strings.HasPrefix(name, "ccc.") {
		return Session{Name: name, External: true}
	}

	parts := strings.SplitN(name[4:], ".", 2) // skip "ccc."
	if len(parts) != 2 {
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
// (for visibility into all abduco sessions on the machine).
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
