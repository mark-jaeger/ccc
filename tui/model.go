package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mark-jaeger/ccc/config"
	"github.com/mark-jaeger/ccc/zmx"
)

// Model is the main application model.
type Model struct {
	state State
	err   error

	// Current selections
	selectedHost    string
	selectedProject string

	// List models
	hostList    list.Model
	projectList list.Model
	sessionList list.Model

	// Loading spinner
	spinner spinner.Model

	// Dimensions
	width  int
	height int

	// Keys
	keys keyMap

	// Mode (remote vs local)
	isLocal bool

	// Help state
	showHelp  bool
	prevState State // state before showing help

	// Connection state
	runner      Runner            // current Runner (SSH or local)
	currentHost *config.Host      // selected host (nil in local mode)
	hosts       map[string]config.Host
	projects    *config.ProjectsConfig
	sessions    []zmx.Session

	// Project state for session operations
	currentProjectKey  string
	currentProjectPath string
}

// New creates a new TUI model.
func New(isLocal bool) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot

	return Model{
		state:   StateLoading,
		spinner: s,
		keys:    Keys(),
		isLocal: isLocal,
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	if m.isLocal {
		// Local mode: skip host selection, load projects directly
		return tea.Batch(
			m.spinner.Tick,
			checkZmxLocalCmd(),
			loadProjectsLocalCmd(),
		)
	}
	return tea.Batch(
		m.spinner.Tick,
		loadHostsCmd(),
	)
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Resize lists to fit new dimensions (only if initialized)
		h := msg.Height - 2 // room for padding
		if len(m.hostList.Items()) > 0 || m.hostList.FilterState() != list.Unfiltered {
			m.hostList.SetSize(msg.Width, h)
		}
		if len(m.projectList.Items()) > 0 || m.projectList.FilterState() != list.Unfiltered {
			m.projectList.SetSize(msg.Width, h-2) // room for breadcrumb
		}
		if len(m.sessionList.Items()) > 0 || m.sessionList.FilterState() != list.Unfiltered {
			m.sessionList.SetSize(msg.Width, h-2)
		}
		return m, nil

	case tea.KeyMsg:
		// Global quit
		if key.Matches(msg, m.keys.Quit) {
			return m, tea.Quit
		}

		// Toggle help view
		if key.Matches(msg, m.keys.Help) {
			if m.showHelp {
				m.showHelp = false
				m.state = m.prevState
			} else {
				m.showHelp = true
				m.prevState = m.state
				m.state = StateHelp
			}
			return m, nil
		}

		// Handle back navigation (only when not filtering)
		if key.Matches(msg, m.keys.Back) && !m.isFiltering() {
			return m.handleBack()
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	// Message handlers for async operations
	case hostsLoadedMsg:
		m.hosts = msg.hosts
		m.SetHosts(msg.hosts, msg.names)
		return m, nil

	case hostConnectedMsg:
		m.currentHost = &msg.host
		m.selectedHost = msg.hostName
		m.runner = msg.runner
		m.state = StateLoading
		return m, tea.Batch(
			m.spinner.Tick,
			checkZmxCmd(m.runner),
			loadProjectsCmd(m.runner),
		)

	case projectsLoadedMsg:
		m.projects = msg.projects
		m.SetProjects(msg.projects.Projects, msg.projects.SortedProjectKeys())
		m.state = StateProjectSelect
		return m, nil

	case sessionsLoadedMsg:
		m.sessions = msg.sessions
		m.SetSessions(msg.sessions, m.currentProjectKey)
		m.state = StateSessionSelect
		return m, nil

	case sessionCreatedMsg:
		// After creating, attach to the new session
		if m.isLocal {
			return m, attachSessionLocalCmd(msg.name)
		}
		return m, attachSessionCmd(*m.currentHost, msg.name)

	case sessionExitedMsg:
		// Returned from zmx attach, refresh sessions
		if m.isLocal {
			return m, loadSessionsLocalCmd(m.currentProjectKey)
		}
		return m, loadSessionsCmd(m.runner, m.currentProjectKey)

	case sessionKilledMsg:
		// Refresh sessions after kill
		if m.isLocal {
			return m, loadSessionsLocalCmd(m.currentProjectKey)
		}
		return m, loadSessionsCmd(m.runner, m.currentProjectKey)

	case scanCompleteMsg:
		// Merge scanned projects with existing
		if m.projects == nil {
			m.projects = &config.ProjectsConfig{
				Projects: make(map[string]config.Project),
			}
		}
		for _, r := range msg.results {
			m.projects.Projects[r.key] = config.Project{Path: r.path}
		}
		// Save and reload
		if m.isLocal {
			return m, saveProjectsLocalCmd(m.projects)
		}
		return m, saveProjectsCmd(m.runner, m.projects)

	case projectDeletedMsg:
		if m.projects != nil {
			delete(m.projects.Projects, msg.key)
			if m.isLocal {
				return m, saveProjectsLocalCmd(m.projects)
			}
			return m, saveProjectsCmd(m.runner, m.projects)
		}
		return m, nil

	case errMsg:
		m.err = msg.err
		m.state = StateError
		return m, nil
	}

	// Delegate to state-specific update
	return m.updateState(msg)
}

// updateState handles state-specific updates.
func (m Model) updateState(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.state {
	case StateHostSelect:
		return m.updateHostSelect(msg)
	case StateProjectSelect:
		return m.updateProjectSelect(msg)
	case StateSessionSelect:
		return m.updateSessionSelect(msg)
	default:
		return m, nil
	}
}

// updateHostSelect handles updates when in host selection state.
func (m Model) updateHostSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.hostList, cmd = m.hostList.Update(msg)

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if key.Matches(keyMsg, m.keys.Select) {
			if item := m.hostList.SelectedItem(); item != nil {
				hi := item.(HostItem)
				m.state = StateConnecting
				return m, connectHostCmd(hi.Name(), hi.Host())
			}
		}
	}

	return m, cmd
}

// updateProjectSelect handles updates when in project selection state.
func (m Model) updateProjectSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.projectList, cmd = m.projectList.Update(msg)

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMsg, m.keys.Select):
			if item := m.projectList.SelectedItem(); item != nil {
				pi := item.(ProjectItem)
				m.currentProjectKey = pi.Key()
				m.currentProjectPath = pi.Project().Path
				m.selectedProject = pi.Key()
				m.state = StateLoading
				if m.isLocal {
					return m, loadSessionsLocalCmd(m.currentProjectKey)
				}
				return m, loadSessionsCmd(m.runner, m.currentProjectKey)
			}
		case key.Matches(keyMsg, m.keys.Scan):
			m.state = StateLoading
			if m.isLocal {
				return m, scanProjectsLocalCmd()
			}
			return m, scanProjectsCmd(m.runner)
		case key.Matches(keyMsg, m.keys.Delete):
			if item := m.projectList.SelectedItem(); item != nil {
				pi := item.(ProjectItem)
				return m, func() tea.Msg {
					return projectDeletedMsg{key: pi.Key()}
				}
			}
		}
	}

	return m, cmd
}

// updateSessionSelect handles updates when in session selection state.
func (m Model) updateSessionSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.sessionList, cmd = m.sessionList.Update(msg)

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMsg, m.keys.Select):
			if item := m.sessionList.SelectedItem(); item != nil {
				session := item.(SessionItem).Session()
				if m.isLocal {
					return m, attachSessionLocalCmd(session.Name)
				}
				return m, attachSessionCmd(*m.currentHost, session.Name)
			}
		case key.Matches(keyMsg, m.keys.New):
			if m.isLocal {
				return m, createSessionLocalCmd(m.currentProjectKey, m.currentProjectPath, m.sessions)
			}
			return m, createSessionCmd(m.runner, m.currentProjectKey, m.currentProjectPath, m.sessions)
		case key.Matches(keyMsg, m.keys.Kill):
			if item := m.sessionList.SelectedItem(); item != nil {
				session := item.(SessionItem).Session()
				if m.isLocal {
					return m, killSessionLocalCmd(session.Name)
				}
				return m, killSessionCmd(m.runner, session.Name)
			}
		}
	}

	return m, cmd
}

// handleBack navigates up one level.
func (m Model) handleBack() (tea.Model, tea.Cmd) {
	switch m.state {
	case StateHelp:
		m.showHelp = false
		m.state = m.prevState
		return m, nil
	case StateSessionSelect:
		m.state = StateProjectSelect
		m.selectedProject = ""
		m.currentProjectKey = ""
		m.currentProjectPath = ""
	case StateProjectSelect:
		if m.isLocal {
			return m, tea.Quit
		}
		m.state = StateHostSelect
		m.selectedHost = ""
		m.currentHost = nil
		m.runner = nil
	case StateHostSelect:
		return m, tea.Quit
	}
	return m, nil
}

// isFiltering returns true if any list is in filter mode.
func (m Model) isFiltering() bool {
	switch m.state {
	case StateHostSelect:
		return m.hostList.FilterState() == list.Filtering
	case StateProjectSelect:
		return m.projectList.FilterState() == list.Filtering
	case StateSessionSelect:
		return m.sessionList.FilterState() == list.Filtering
	}
	return false
}

// View implements tea.Model.
func (m Model) View() string {
	if m.showHelp {
		return m.helpView()
	}

	if m.err != nil {
		return ErrorStyle.Render("Error: " + m.err.Error())
	}

	switch m.state {
	case StateLoading:
		return m.spinner.View() + " Loading..."
	case StateHostSelect:
		return m.hostList.View()
	case StateProjectSelect:
		return m.breadcrumb() + "\n" + m.projectList.View()
	case StateSessionSelect:
		return m.breadcrumb() + "\n" + m.sessionList.View()
	case StateConnecting:
		return m.spinner.View() + " Connecting..."
	case StateError:
		return ErrorStyle.Render("Error: " + m.err.Error())
	case StateHelp:
		return m.helpView()
	}
	return ""
}

// breadcrumb returns the navigation breadcrumb.
func (m Model) breadcrumb() string {
	var parts []string
	if m.selectedHost != "" {
		parts = append(parts, m.selectedHost)
	}
	if m.selectedProject != "" {
		parts = append(parts, m.selectedProject)
	}
	if len(parts) == 0 {
		return ""
	}
	return BreadcrumbStyle.Render(strings.Join(parts, " > "))
}

// SetHosts initializes the host list with data.
func (m *Model) SetHosts(hosts map[string]config.Host, names []string) {
	m.hostList = NewHostList(hosts, names, m.width, m.height-2)
	m.state = StateHostSelect
}

// SetProjects initializes the project list with data.
func (m *Model) SetProjects(projects map[string]config.Project, keys []string) {
	m.projectList = NewProjectList(projects, keys, m.width, m.height-4)
	// -4 for breadcrumb and padding
}

// SetSessions initializes the session list with data.
func (m *Model) SetSessions(sessions []zmx.Session, projectKey string) {
	if len(sessions) == 0 {
		m.sessionList = EmptySessionList(projectKey, m.width, m.height-4)
	} else {
		m.sessionList = NewSessionList(sessions, projectKey, m.width, m.height-4)
	}
}

// helpView renders the help screen.
func (m Model) helpView() string {
	help := `
ccc - Terminal Session Manager

Navigation:
  j/k       Move down/up
  g/G       Go to top/bottom
  /         Filter list
  Enter     Select item
  Esc       Go back / Clear filter
  q         Quit

Host Screen:
  Enter     Connect to host

Project Screen:
  s         Scan for new projects
  d         Delete project
  Enter     View sessions

Session Screen:
  n         New session
  x         Kill session
  Enter     Attach to session

Press ? to close this help.
`
	return lipgloss.NewStyle().Padding(1, 2).Render(help)
}
