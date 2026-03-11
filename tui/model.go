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

	// List models (will be populated by components in Plan 03)
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
	return m.spinner.Tick
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Resize lists to fit new dimensions
		h := msg.Height - 2 // room for padding
		m.hostList.SetSize(msg.Width, h)
		m.projectList.SetSize(msg.Width, h-2) // room for breadcrumb
		m.sessionList.SetSize(msg.Width, h-2)
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
	return m, cmd
}

// updateProjectSelect handles updates when in project selection state.
func (m Model) updateProjectSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.projectList, cmd = m.projectList.Update(msg)
	return m, cmd
}

// updateSessionSelect handles updates when in session selection state.
func (m Model) updateSessionSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.sessionList, cmd = m.sessionList.Update(msg)
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
	case StateProjectSelect:
		if m.isLocal {
			return m, tea.Quit
		}
		m.state = StateHostSelect
		m.selectedHost = ""
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
