package tui

import (
	"fmt"

	"github.com/mark-jaeger/ccc/config"
	"github.com/mark-jaeger/ccc/zmx"
)

// HostItem wraps a host name and config.Host for the list component.
type HostItem struct {
	name string
	host config.Host
}

// Title returns the host name.
func (i HostItem) Title() string { return i.name }

// Description returns the host address.
func (i HostItem) Description() string { return i.host.Address }

// FilterValue returns the host name for fuzzy filtering.
func (i HostItem) FilterValue() string { return i.name }

// ProjectItem wraps a project key and config.Project.
type ProjectItem struct {
	key     string
	project config.Project
}

// Title returns the project key.
func (i ProjectItem) Title() string { return i.key }

// Description returns the project path.
func (i ProjectItem) Description() string { return i.project.Path }

// FilterValue returns the project key for fuzzy filtering.
func (i ProjectItem) FilterValue() string { return i.key }

// SessionItem wraps a zmx.Session for the list component.
type SessionItem struct {
	session zmx.Session
}

// Title returns the session name.
func (i SessionItem) Title() string { return i.session.Name }

// Description returns session status information.
func (i SessionItem) Description() string {
	if i.session.External {
		return "(external)"
	}
	if i.session.Clients > 0 {
		return fmt.Sprintf("%d client(s) attached", i.session.Clients)
	}
	return "detached"
}

// FilterValue returns the session name for fuzzy filtering.
func (i SessionItem) FilterValue() string { return i.session.Name }

// Helper constructors

// NewHostItem creates a HostItem from a name and host config.
func NewHostItem(name string, h config.Host) HostItem {
	return HostItem{name: name, host: h}
}

// NewProjectItem creates a ProjectItem from a key and project config.
func NewProjectItem(key string, p config.Project) ProjectItem {
	return ProjectItem{key: key, project: p}
}

// NewSessionItem creates a SessionItem from a zmx.Session.
func NewSessionItem(s zmx.Session) SessionItem {
	return SessionItem{session: s}
}

// Getters for accessing wrapped data

// Name returns the host name.
func (i HostItem) Name() string { return i.name }

// Host returns the underlying config.Host.
func (i HostItem) Host() config.Host { return i.host }

// Key returns the project key.
func (i ProjectItem) Key() string { return i.key }

// Project returns the underlying config.Project.
func (i ProjectItem) Project() config.Project { return i.project }

// Session returns the underlying zmx.Session.
func (i SessionItem) Session() zmx.Session { return i.session }
