package tui

import (
	"github.com/mark-jaeger/ccc/config"
	"github.com/mark-jaeger/ccc/zmx"
)

// hostsLoadedMsg is sent when hosts are loaded from config.
type hostsLoadedMsg struct {
	hosts map[string]config.Host
	names []string // sorted host names
}

// hostConnectedMsg is sent when SSH connection is established.
type hostConnectedMsg struct {
	hostName string
	host     config.Host
	runner   Runner // the SSH connection as a Runner
}

// projectsLoadedMsg is sent when projects are loaded from remote.
type projectsLoadedMsg struct {
	projects *config.ProjectsConfig
}

// sessionsLoadedMsg is sent when zmx sessions are listed.
type sessionsLoadedMsg struct {
	sessions []zmx.Session
}

// sessionExitedMsg is sent when zmx attach exits.
type sessionExitedMsg struct {
	err error
}

// sessionCreatedMsg is sent when a new session is created.
type sessionCreatedMsg struct {
	name string
}

// sessionKilledMsg is sent when a session is killed.
type sessionKilledMsg struct {
	name string
}

// scanCompleteMsg is sent when project scanning completes.
type scanCompleteMsg struct {
	results []scanResult
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

// zmxAvailableMsg is sent when zmx is confirmed installed.
type zmxAvailableMsg struct{}

// errMsg wraps errors for display.
type errMsg struct {
	err error
}

func (e errMsg) Error() string { return e.err.Error() }
