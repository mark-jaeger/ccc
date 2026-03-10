// Package abduco provides session listing, creation, attachment, and management
// for abduco terminal sessions. Unlike tmux, abduco provides transparent PTY
// passthrough with automatic client detachment.
package abduco

import (
	"fmt"

	"github.com/mark-jaeger/ccc/internal/shellutil"
)

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
// The session starts bash -l in the specified path.
func BuildCreateCommand(name, path string) string {
	return fmt.Sprintf("cd %s && abduco -n %s bash -l",
		shellutil.Quote(path), shellutil.Quote(name))
}

// BuildAttachCommand returns a shell command to attach to a named session.
func BuildAttachCommand(name string) string {
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
