// Package tmux provides session listing, creation, attachment, and metadata
// management for tmux sessions tagged with ccc project information.
package tmux

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mark-jaeger/ccc/internal/shellutil"
)

// Session represents a tmux session with optional ccc metadata.
type Session struct {
	Name     string
	Project  string // from @ccc_project, empty if untagged
	Path     string // from @ccc_path
	Windows  int
	Verified bool // true if matched by metadata, false if by prefix
}

// Client represents a tmux client attached to a session.
type Client struct {
	TTY    string
	Width  int
	Height int
}

// listFormat is the tmux -F format string for session listing.
// Fields are |||-delimited: name, ccc_project tag, ccc_path tag, window count.
const listFormat = "#{session_name}|||#{@ccc_project}|||#{@ccc_path}|||#{session_windows}"

// clientFormat is the tmux -F format string for client listing.
const clientFormat = "#{client_tty}: #{client_width}x#{client_height} #{client_flags}"

// BuildListCommand returns a shell command that lists all tmux sessions.
func BuildListCommand() string {
	return fmt.Sprintf("tmux list-sessions -F '%s' 2>/dev/null || true", listFormat)
}

// BuildListClientsCommand returns a shell command that lists clients for a session.
func BuildListClientsCommand(session string) string {
	return fmt.Sprintf("tmux list-clients -t %s -F '%s'", shellutil.Quote(session), clientFormat)
}

// BuildCreateCommand returns a shell command that creates a new detached tmux
// session with ccc metadata tags.
func BuildCreateCommand(name, path, projectKey string) string {
	qn := shellutil.Quote(name)
	return fmt.Sprintf(
		"tmux new-session -d -s %s -c %s \\; set-option -t %s @ccc_project %s \\; set-option -t %s @ccc_path %s \\; set-option -t %s visual-bell off \\; set-option -t %s bell-action any",
		qn, shellutil.Quote(path), qn, shellutil.Quote(projectKey), qn, shellutil.Quote(path), qn, qn,
	)
}

// BuildSetPassthroughCommand returns a shell command to enable escape sequence
// passthrough for a session. Requires tmux 3.3+; fails silently on older versions.
func BuildSetPassthroughCommand(name string) string {
	return fmt.Sprintf("tmux set-option -t %s allow-passthrough on", shellutil.Quote(name))
}

// BuildAttachCommand returns a shell command to attach to a named session.
func BuildAttachCommand(name string) string {
	return fmt.Sprintf("tmux attach -t %s", shellutil.Quote(name))
}

// BuildKillCommand returns a shell command to kill a named session.
func BuildKillCommand(name string) string {
	return fmt.Sprintf("tmux kill-session -t %s", shellutil.Quote(name))
}

// BuildDetachClientsCommand returns a shell command to detach all clients
// from a named session.
func BuildDetachClientsCommand(name string) string {
	return fmt.Sprintf("tmux detach-client -t %s -a", shellutil.Quote(name))
}

// BuildCheckTmuxCommand returns a shell command to check if tmux is installed.
func BuildCheckTmuxCommand() string {
	return "command -v tmux"
}

// ParseSessionList parses the |||-delimited output of BuildListCommand into
// a slice of Session values. Returns nil for empty input.
func ParseSessionList(output string) []Session {
	output = strings.TrimSpace(output)
	if output == "" {
		return nil
	}

	lines := strings.Split(output, "\n")
	var sessions []Session

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "|||", 4)
		if len(parts) != 4 {
			continue
		}

		windows, _ := strconv.Atoi(parts[3])

		sessions = append(sessions, Session{
			Name:    parts[0],
			Project: parts[1],
			Path:    parts[2],
			Windows: windows,
		})
	}

	return sessions
}

// FilterSessionsForProject returns sessions belonging to a project.
// Sessions tagged with @ccc_project matching projectKey are returned with
// Verified=true. Untagged sessions whose name equals projectKey or starts
// with projectKey+"-" are returned with Verified=false.
func FilterSessionsForProject(sessions []Session, projectKey string) []Session {
	var result []Session

	// First pass: metadata matches.
	for _, s := range sessions {
		if s.Project == projectKey {
			s.Verified = true
			result = append(result, s)
		}
	}

	// Second pass: prefix fallback for untagged sessions.
	prefix := projectKey + "-"
	for _, s := range sessions {
		if s.Project != "" {
			continue // already tagged — skip
		}
		if s.Name == projectKey || strings.HasPrefix(s.Name, prefix) {
			result = append(result, s)
		}
	}

	return result
}

// NextAutoName returns the next session name for a project. The first session
// gets the bare projectKey (e.g. "rt1"), subsequent sessions get
// projectKey-N where N increments from 2 (e.g. "rt1-2", "rt1-3").
func NextAutoName(projectKey string, existing []Session) string {
	if len(existing) == 0 {
		return projectKey
	}

	maxNum := 1 // the bare name counts as 1
	prefix := projectKey + "-"

	for _, s := range existing {
		if s.Name == projectKey {
			// bare name exists, counts as 1
			continue
		}
		if strings.HasPrefix(s.Name, prefix) {
			suffix := s.Name[len(prefix):]
			if n, err := strconv.Atoi(suffix); err == nil && n > maxNum {
				maxNum = n
			}
		}
	}

	return fmt.Sprintf("%s-%d", projectKey, maxNum+1)
}

// ParseClientList parses the output of BuildListClientsCommand into Client values.
func ParseClientList(output string) []Client {
	output = strings.TrimSpace(output)
	if output == "" {
		return nil
	}

	lines := strings.Split(output, "\n")
	var clients []Client

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Format: "/dev/ttys004: 220x56 0"
		colonIdx := strings.Index(line, ":")
		if colonIdx < 0 {
			continue
		}

		tty := line[:colonIdx]
		rest := strings.TrimSpace(line[colonIdx+1:])

		// rest is "220x56 0" — we need WxH
		fields := strings.Fields(rest)
		if len(fields) < 1 {
			continue
		}

		dims := strings.SplitN(fields[0], "x", 2)
		if len(dims) != 2 {
			continue
		}

		width, err := strconv.Atoi(dims[0])
		if err != nil {
			continue
		}
		height, err := strconv.Atoi(dims[1])
		if err != nil {
			continue
		}

		clients = append(clients, Client{
			TTY:    tty,
			Width:  width,
			Height: height,
		})
	}

	return clients
}
