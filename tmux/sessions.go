// Package tmux provides session listing, creation, attachment, and metadata
// management for tmux sessions tagged with ccc project information.
package tmux

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mark-jaeger/ccc/internal/shellutil"
)

// SocketOverride, when non-empty, adds -L <socket> to all tmux commands.
// Used for test isolation. Set from CCC_TMUX_SOCKET environment variable.
var SocketOverride string

// tmuxCmd returns "tmux" or "tmux -L <socket>" depending on SocketOverride.
func tmuxCmd() string {
	if SocketOverride != "" {
		return fmt.Sprintf("tmux -L %s", shellutil.Quote(SocketOverride))
	}
	return "tmux"
}

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
	return fmt.Sprintf("%s list-sessions -F '%s' 2>/dev/null || true", tmuxCmd(), listFormat)
}

// BuildListClientsCommand returns a shell command that lists clients for a session.
func BuildListClientsCommand(session string) string {
	return fmt.Sprintf("%s list-clients -t %s -F '%s'", tmuxCmd(), shellutil.Quote(session), clientFormat)
}

// BuildCreateCommand returns a shell command that creates a new detached tmux
// session with ccc metadata tags and notification options.
//
// Session options: @ccc_project, @ccc_path, bell-action, silence-action,
// activity-action, visual-activity.
// Window options: visual-bell, monitor-silence, monitor-activity.
//
// monitor-silence causes tmux to generate an alert when the window has no
// output for 5 seconds. With silence-action any, the alert propagates, and
// with visual-bell off the alert becomes a real BEL character sent to the
// attached terminal.
//
// activity-action any + visual-activity on allow the alert-activity hook to
// fire (for re-arming monitor-silence) without generating an unwanted bell.
// The one-shot hooks are set separately via BuildSetNotifyHooksCommand.
//
// Note: allow-passthrough is set separately via BuildSetPassthroughCommand
// because it requires tmux >= 3.3 and must not break session creation on
// older versions.
func BuildCreateCommand(name, path, projectKey string) string {
	qn := shellutil.Quote(name)
	return fmt.Sprintf(
		"%s new-session -d -s %s -c %s"+
			" \\; set-option -t %s @ccc_project %s"+
			" \\; set-option -t %s @ccc_path %s"+
			" \\; set-option -t %s bell-action any"+
			" \\; set-option -t %s silence-action any"+
			" \\; set-option -t %s activity-action any"+
			" \\; set-option -t %s visual-activity on"+
			" \\; set-window-option -t %s visual-bell off"+
			" \\; set-window-option -t %s monitor-silence 5"+
			" \\; set-window-option -t %s monitor-activity off",
		tmuxCmd(),
		qn, shellutil.Quote(path),
		qn, shellutil.Quote(projectKey),
		qn, shellutil.Quote(path),
		qn,
		qn,
		qn,
		qn,
		qn,
		qn,
		qn,
	)
}

// BuildSetNotifyHooksCommand returns a shell command that installs tmux hooks
// to make monitor-silence fire exactly once per silence period.
//
// Without hooks, monitor-silence fires repeatedly every N seconds while the
// window is silent. The hooks create a state machine:
//
//	silence detected → bell fires → disable monitor-silence, enable monitor-activity
//	activity detected → disable monitor-activity, re-enable monitor-silence
//
// This requires activity-action any + visual-activity on to be set on the
// session so that the alert-activity hook fires without generating a bell.
func BuildSetNotifyHooksCommand(name string) string {
	return buildNotifyHooks(shellutil.Quote(name))
}

// buildNotifyHooks returns the shell fragment that installs one-shot
// alert-silence / alert-activity hooks on the given quoted session name.
// The alert-silence hook also posts a native macOS notification via osascript
// (fails silently on non-macOS systems).
func buildNotifyHooks(quotedName string) string {
	tc := tmuxCmd()
	// osascript notification piped via echo to avoid single-quote nesting.
	// Escaping: Go raw string → shell (single-quoted) → tmux (double-quoted) → sh → osascript.
	notify := `run-shell -b "echo \"display notification \\\"#{session_name} idle\\\" with title \\\"ccc\\\"\" | osascript 2>/dev/null || true"`
	return fmt.Sprintf(
		"%s set-hook -t %s alert-silence '%s ; set-window-option monitor-silence 0 ; set-window-option monitor-activity on'"+
			" ; %s set-hook -t %s alert-activity 'set-window-option monitor-activity off ; set-window-option monitor-silence 5'",
		tc, quotedName, notify, tc, quotedName,
	)
}

// BuildSetPassthroughCommand returns a shell command to enable escape sequence
// passthrough on a session. This requires tmux >= 3.3; callers should ignore
// errors for backward compatibility with older tmux versions.
func BuildSetPassthroughCommand(name string) string {
	return fmt.Sprintf("%s set-window-option -t %s allow-passthrough on", tmuxCmd(), shellutil.Quote(name))
}

// BuildEnsureNotifyOptionsCommand returns a shell command that sets bell,
// silence monitoring, activity, passthrough options, and one-shot hooks on
// an existing session. This is idempotent and safe to call on every attach
// so that sessions created by older ccc versions get the correct options.
// The allow-passthrough part is appended with "2>/dev/null" so it silently
// fails on tmux < 3.3.
func BuildEnsureNotifyOptionsCommand(name string) string {
	qn := shellutil.Quote(name)
	tc := tmuxCmd()
	return fmt.Sprintf(
		"%s set-option -t %s bell-action any"+
			" \\; set-option -t %s silence-action any"+
			" \\; set-option -t %s activity-action any"+
			" \\; set-option -t %s visual-activity on"+
			" \\; set-window-option -t %s visual-bell off"+
			" \\; set-window-option -t %s monitor-silence 5"+
			" \\; set-window-option -t %s monitor-activity off"+
			" 2>/dev/null;",
		tc, qn, qn, qn, qn, qn, qn, qn,
	) + " " + buildNotifyHooks(qn) + fmt.Sprintf(
		" ; %s set-window-option -t %s allow-passthrough on 2>/dev/null; true",
		tc, qn,
	)
}

// BuildAttachCommand returns a shell command to attach to a named session.
func BuildAttachCommand(name string) string {
	return fmt.Sprintf("%s attach -t %s", tmuxCmd(), shellutil.Quote(name))
}

// BuildKillCommand returns a shell command to kill a named session.
func BuildKillCommand(name string) string {
	return fmt.Sprintf("%s kill-session -t %s", tmuxCmd(), shellutil.Quote(name))
}

// BuildDetachClientsCommand returns a shell command to detach all clients
// attached to the named session.
func BuildDetachClientsCommand(name string) string {
	return fmt.Sprintf("%s detach-client -s %s", tmuxCmd(), shellutil.Quote(name))
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
