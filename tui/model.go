package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
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

	// Session name input
	sessionNameInput textinput.Model

	// Mode (remote vs local)
	isLocal bool

	// Help state
	showHelp  bool
	prevState State // state before showing help

	// Connection state
	runner      Runner            // current Runner (SSH or local)
	currentHost *config.Host      // selected host (nil in local mode)
	hosts       []config.Host
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

	ti := textinput.New()
	ti.Placeholder = "main"
	ti.CharLimit = 50
	ti.Width = 30

	return Model{
		state:            StateLoading,
		spinner:          s,
		sessionNameInput: ti,
		keys:             Keys(),
		isLocal:          isLocal,
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
		m.SetHosts(msg.hosts)
		return m, nil

	case hostConnectedMsg:
		m.currentHost = &msg.host
		m.selectedHost = msg.hostName
		m.runner = msg.runner
		m.state = StateLoading
		// Check zmx first, only load projects if available
		return m, tea.Batch(
			m.spinner.Tick,
			checkZmxCmd(m.runner),
		)

	case zmxAvailableMsg:
		// zmx is installed, now load projects
		return m, loadProjectsCmd(m.runner)

	case projectsLoadedMsg:
		m.projects = msg.projects
		m.SetProjects(msg.projects.Projects)
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
				Projects: []config.Project{},
			}
		}
		for _, r := range msg.results {
			// Add or update project by name
			found := false
			for i, existing := range m.projects.Projects {
				if existing.Name == r.key {
					m.projects.Projects[i] = config.Project{Name: r.key, Path: r.path}
					found = true
					break
				}
			}
			if !found {
				m.projects.Projects = append(m.projects.Projects, config.Project{Name: r.key, Path: r.path})
			}
		}
		// Save and reload
		if m.isLocal {
			return m, saveProjectsLocalCmd(m.projects)
		}
		return m, saveProjectsCmd(m.runner, m.projects)

	case projectDeletedMsg:
		if m.projects != nil {
			for i, p := range m.projects.Projects {
				if p.Name == msg.key {
					m.projects.Projects = append(m.projects.Projects[:i], m.projects.Projects[i+1:]...)
					break
				}
			}
			if m.isLocal {
				return m, saveProjectsLocalCmd(m.projects)
			}
			return m, saveProjectsCmd(m.runner, m.projects)
		}
		return m, nil

	case errMsg:
		m.err = msg.err
		// Fatal errors: quit TUI so error prints to normal terminal (selectable)
		if strings.Contains(msg.err.Error(), "zmx not found") {
			return m, tea.Quit
		}
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
	case StateSessionNameInput:
		return m.updateSessionNameInput(msg)
	default:
		return m, nil
	}
}

// updateHostSelect handles updates when in host selection state.
func (m Model) updateHostSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.hostList, cmd = m.hostList.Update(msg)

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		// Don't allow reorder while filtering
		if m.hostList.FilterState() != list.Filtering {
			switch {
			case key.Matches(keyMsg, m.keys.MoveUp):
				return m.reorderHost(-1)
			case key.Matches(keyMsg, m.keys.MoveDown):
				return m.reorderHost(1)
			}
		}

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

// reorderHost moves the selected host up (-1) or down (+1).
func (m Model) reorderHost(direction int) (tea.Model, tea.Cmd) {
	idx := m.hostList.Index()
	newIdx := idx + direction

	if newIdx < 0 || newIdx >= len(m.hosts) {
		return m, nil // at boundary, do nothing
	}

	// Swap in the slice
	m.hosts[idx], m.hosts[newIdx] = m.hosts[newIdx], m.hosts[idx]

	// Rebuild list and restore cursor
	m.hostList = NewHostList(m.hosts, m.width, m.height-2)
	m.hostList.Select(newIdx)

	// Save config
	return m, saveHostsCmd(m.hosts)
}

// updateProjectSelect handles updates when in project selection state.
func (m Model) updateProjectSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.projectList, cmd = m.projectList.Update(msg)

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		// Don't allow reorder while filtering
		if m.projectList.FilterState() != list.Filtering {
			switch {
			case key.Matches(keyMsg, m.keys.MoveUp):
				return m.reorderProject(-1)
			case key.Matches(keyMsg, m.keys.MoveDown):
				return m.reorderProject(1)
			}
		}

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

// reorderProject moves the selected project up (-1) or down (+1).
func (m Model) reorderProject(direction int) (tea.Model, tea.Cmd) {
	idx := m.projectList.Index()
	newIdx := idx + direction

	if newIdx < 0 || newIdx >= len(m.projects.Projects) {
		return m, nil
	}

	// Swap
	m.projects.Projects[idx], m.projects.Projects[newIdx] = m.projects.Projects[newIdx], m.projects.Projects[idx]

	// Rebuild list
	m.projectList = NewProjectList(m.projects.Projects, m.width, m.height-4)
	m.projectList.Select(newIdx)

	// Save
	if m.isLocal {
		return m, saveProjectsLocalCmd(m.projects)
	}
	return m, saveProjectsCmd(m.runner, m.projects)
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
			// Transition to name input instead of creating directly
			m.sessionNameInput.Reset()
			m.sessionNameInput.Focus()
			m.state = StateSessionNameInput
			return m, textinput.Blink
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

// updateSessionNameInput handles updates when in session name input state.
func (m Model) updateSessionNameInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			suffix := strings.TrimSpace(m.sessionNameInput.Value())

			// Validate suffix
			if strings.Contains(suffix, ".") {
				return m, nil
			}
			if strings.ContainsAny(suffix, " \t\n\r") {
				return m, nil
			}

			// Build session name
			var name string
			if suffix == "" {
				name = zmx.NextAutoName(m.currentProjectKey, m.sessions)
			} else {
				name = "ccc." + m.currentProjectKey + "." + suffix
			}

			// Check for conflicts
			for _, s := range m.sessions {
				if s.Name == name {
					return m, nil
				}
			}

			// Create the session
			m.state = StateCreatingSession
			if m.isLocal {
				return m, createSessionWithNameLocalCmd(name, m.currentProjectPath)
			}
			return m, createSessionWithNameCmd(*m.currentHost, name, m.currentProjectPath)

		case tea.KeyEsc:
			m.sessionNameInput.Blur()
			m.state = StateSessionSelect
			return m, nil
		}
	}

	m.sessionNameInput, cmd = m.sessionNameInput.Update(msg)
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

	// Top padding for generous whitespace
	pad := "\n\n"

	switch m.state {
	case StateLoading:
		return pad + m.spinner.View() + " Loading..."
	case StateHostSelect:
		return pad + m.hostList.View()
	case StateProjectSelect:
		return pad + m.breadcrumb() + "\n" + m.projectList.View()
	case StateSessionSelect:
		return pad + m.breadcrumb() + "\n" + m.sessionList.View()
	case StateSessionNameInput:
		return pad + m.sessionNameInputView()
	case StateConnecting:
		return pad + m.spinner.View() + " Connecting..."
	case StateError:
		return pad + ErrorStyle.Render("Error: " + m.err.Error())
	case StateHelp:
		return m.helpView()
	}
	return ""
}

// sessionNameInputView renders the session name input screen.
func (m Model) sessionNameInputView() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Creating session for: %s\n\n", m.currentProjectKey))
	b.WriteString("Session suffix (empty for auto): ")
	b.WriteString(m.sessionNameInput.View())
	b.WriteString("\n\n")
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("[Enter] Create  [Esc] Cancel"))
	return b.String()
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
func (m *Model) SetHosts(hosts []config.Host) {
	m.hostList = NewHostList(hosts, m.width, m.height-2)
	m.state = StateHostSelect
}

// SetProjects initializes the project list with data.
func (m *Model) SetProjects(projects []config.Project) {
	m.projectList = NewProjectList(projects, m.width, m.height-4)
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

Inside zmx session:
  Ctrl+\    Detach (return to ccc)

Press ? to close this help.
`
	return lipgloss.NewStyle().Padding(1, 2).Render(help)
}
