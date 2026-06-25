package tui

import (
	"github.com/mark-jaeger/ccc/config"
	"github.com/mark-jaeger/ccc/zmx"
)

// hostsLoadedMsg is sent when hosts are loaded from config.
type hostsLoadedMsg struct {
	hosts []config.Host
}

// hostConnectedMsg is sent when SSH connection is established. gen is the
// request epoch (reqGen) the connect was fired in; the model drops a message
// whose gen no longer matches reqGen so a late result from a canceled or
// superseded connect cannot clobber the screen. A gen of 0 is treated as
// unguarded (legacy/startup paths) and always applies.
type hostConnectedMsg struct {
	hostName string
	host     config.Host
	runner   Runner // the SSH connection as a Runner
	gen      int
}

// projectsLoadedMsg is sent when projects are loaded. gen ties the result to its
// request epoch for the stale-result guard (see hostConnectedMsg); gen 0 is
// unguarded (e.g. the post-scan save reload and the local startup load).
type projectsLoadedMsg struct {
	projects *config.ProjectsConfig
	gen      int
}

// sessionsLoadedMsg is sent when zmx sessions are listed. gen ties the result to
// its request epoch for the stale-result guard.
type sessionsLoadedMsg struct {
	sessions []zmx.Session
	gen      int
}

// sessionExitedMsg is sent when zmx attach exits.
type sessionExitedMsg struct {
	err error
}

// reconnectMsg is sent after the auto-reattach backoff elapses, signalling the
// model to re-fire the interactive attach for lastSession. gen identifies the
// reconnect epoch the tick was scheduled in; a tick whose gen no longer matches
// the model (because the user canceled, navigated away, or started a fresh
// attach) is stale and must be ignored rather than hijacking the terminal.
type reconnectMsg struct {
	gen int
}

// reconnectProbeMsg carries the result of a bounded, non-interactive
// reachability check run before an automatic interactive reattach. It guards
// against a still-down or auth-broken host seizing the terminal with a blocking
// ssh prompt. gen ties the probe to its reconnect epoch so a stale result is
// ignored.
type reconnectProbeMsg struct {
	gen int
	ok  bool
}

// sessionCreatedMsg is sent when a new session is created.
type sessionCreatedMsg struct {
	name string
}

// sessionKilledMsg is sent when a session is killed.
type sessionKilledMsg struct {
	name string
}

// scanCompleteMsg is sent when project scanning completes. gen ties the result
// to its request epoch for the stale-result guard.
type scanCompleteMsg struct {
	results []scanResult
	gen     int
}

// scanResult represents a discovered project from scanning.
type scanResult struct {
	key  string
	path string
}

// projectDeletedMsg is sent when a project is deleted.
type projectDeletedMsg struct {
	key string
}

// zmxAvailableMsg is sent when zmx is confirmed installed. gen ties the result
// to its request epoch so a late check from a canceled connect is dropped rather
// than driving a project load against a host the user already navigated away
// from. gen 0 is unguarded (local startup check).
type zmxAvailableMsg struct {
	gen int
}

// errMsg wraps errors for display. gen ties an error to the cancelable
// connect/load/scan request that produced it, so a failure from a canceled or
// superseded operation is dropped instead of clobbering the screen with
// StateError. gen 0 marks an unguarded error (saves, kills, the zmx-not-found
// install message, startup) that always applies.
type errMsg struct {
	err error
	gen int
}

func (e errMsg) Error() string { return e.err.Error() }
