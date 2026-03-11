// Package zmx provides session listing, creation, attachment, and management
// for zmx terminal sessions. zmx provides transparent PTY passthrough with
// automatic terminal state restoration on reattach.
package zmx

import (
	"fmt"

	"github.com/mark-jaeger/ccc/internal/shellutil"
)

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

// BuildListCommand returns a shell command that lists all zmx sessions.
func BuildListCommand() string {
	return "zmx list"
}

// BuildAttachCommand returns a shell command to attach to a named session.
// Uses TERM=$TERM prefix to ensure terminal type passthrough over SSH.
func BuildAttachCommand(name string) string {
	return fmt.Sprintf("TERM=$TERM zmx attach %s", shellutil.Quote(name))
}

// BuildCreateCommand returns a shell command that creates a new zmx session.
// Since zmx attach creates if not exists, this is the same as attach but
// with a cd to the specified path first.
// Uses TERM=$TERM prefix to ensure terminal type passthrough over SSH.
func BuildCreateCommand(name, path string) string {
	return fmt.Sprintf("cd %s && TERM=$TERM zmx attach %s",
		shellutil.Quote(path), shellutil.Quote(name))
}

// BuildKillCommand returns a shell command to kill a session by name.
// Unlike abduco which uses PID, zmx uses the session name directly.
func BuildKillCommand(name string) string {
	return fmt.Sprintf("zmx kill %s", shellutil.Quote(name))
}

// BuildCheckCommand returns a shell command to check if zmx is installed.
func BuildCheckCommand() string {
	return "command -v zmx"
}
