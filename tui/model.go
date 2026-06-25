package tui

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

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
	sessionNameInput    textinput.Model
	sessionNameInputErr string // validation error message

	// Mode (remote vs local)
	isLocal bool

	// Help state
	showHelp  bool
	prevState State // state before showing help

	// Connection state
	runner      Runner       // current Runner (SSH or local)
	currentHost *config.Host // selected host (nil in local mode)
	hosts       []config.Host
	projects    *config.ProjectsConfig
	sessions    []zmx.Session

	// Project state for session operations
	currentProjectKey  string
	currentProjectPath string

	// Reconnect state for interactive-attach transport failures (exit 255).
	// lastSession is the name of the most recently attached session, so a
	// dropped connection can be re-fired. lastProjectPath is that session's
	// project directory, captured so a reconnect that has to (re)create a
	// never-established session does so in the right cwd rather than the login
	// dir. reconnectAttempts bounds the automatic retry loop. reconnectGen
	// identifies the current reconnect epoch: it is bumped whenever the attempt
	// budget is (re)set — i.e. on a fresh attach, a manual reconnect, or a
	// cancel/navigation — so a backoff tick scheduled in a prior epoch is
	// recognised as stale and ignored.
	lastSession       string
	lastProjectPath   string
	reconnectAttempts int
	reconnectGen      int
}

// maxReconnectAttempts bounds automatic re-attach attempts before the model
// gives up and offers a manual reconnect prompt.
const maxReconnectAttempts = 2

// reconnectBackoff is the delay between automatic re-attach attempts. It keeps
// the retry loop from spinning tightly on a flapping connection.
const reconnectBackoff = 750 * time.Millisecond

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
		m.ensureMinDimensions()
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
		// In local mode, projects are already loaded from Init()
		if m.isLocal {
			return m, nil
		}
		return m, loadProjectsCmd(m.runner)

	case projectsLoadedMsg:
		m.projects = msg.projects
		m.ensureMinDimensions()
		m.SetProjects(msg.projects.Projects)
		m.state = StateProjectSelect
		return m, nil

	case sessionsLoadedMsg:
		m.sessions = msg.sessions
		m.SetSessions(msg.sessions, m.currentProjectKey)
		m.state = StateSessionSelect
		return m, nil

	case sessionCreatedMsg:
		// After creating, attach to the new session. Track it (and its project
		// dir) so a dropped connection can be re-fired, and open a fresh epoch.
		m.lastSession = msg.name
		m.lastProjectPath = m.currentProjectPath
		m.reconnectAttempts = 0
		m.reconnectGen++
		if m.isLocal {
			return m, attachSessionLocalCmd(msg.name)
		}
		return m, attachSessionCmd(*m.currentHost, msg.name)

	case sessionExitedMsg:
		// Returned from zmx attach.
		// A transport failure (ssh exits 255) means the connection dropped, not
		// that zmx itself errored. Offer a bounded, cancellable auto-reattach
		// instead of dumping the user into a dead-end error screen.
		if isTransportFailure(msg.err) {
			if m.reconnectAttempts < maxReconnectAttempts {
				m.reconnectAttempts++
				m.state = StateReconnecting
				// Capture the current epoch so a cancel/navigation that bumps
				// reconnectGen before this tick fires makes the tick stale.
				gen := m.reconnectGen
				return m, tea.Tick(reconnectBackoff, func(time.Time) tea.Msg {
					return reconnectMsg{gen: gen}
				})
			}
			// Exhausted automatic attempts: hand off to manual recovery.
			m.state = StateConnectionLost
			return m, nil
		}
		// Any other error is a genuine failure: display it.
		if msg.err != nil {
			m.err = msg.err
			m.state = StateError
			return m, nil
		}
		// Clean exit: reset the retry counter and refresh sessions. Bump the
		// epoch so any tick still in flight from a prior drop is ignored.
		m.reconnectAttempts = 0
		m.reconnectGen++
		if m.isLocal {
			return m, loadSessionsLocalCmd(m.currentProjectKey)
		}
		return m, loadSessionsCmd(m.runner, m.currentProjectKey)

	case reconnectMsg:
		// Backoff elapsed: re-fire the interactive attach for lastSession, but
		// only if this tick belongs to the current epoch and we are still
		// reconnecting. Otherwise the user canceled or navigated away, and
		// firing the attach would unexpectedly seize the terminal.
		if msg.gen != m.reconnectGen || m.state != StateReconnecting {
			return m, nil
		}
		// Local attaches run over a pipe and cannot block on an ssh prompt, so
		// fire directly. For remote, first probe reachability non-interactively
		// (bounded, BatchMode, no prompt) so an automatic interactive attach
		// never seizes the terminal waiting on a password prompt or a long TCP
		// connect against a host that is still down.
		if m.isLocal {
			return m, m.attachLastSessionCmd()
		}
		return m, m.reconnectProbeCmd()

	case reconnectProbeMsg:
		// Only act on a probe from the current epoch while still reconnecting.
		if msg.gen != m.reconnectGen || m.state != StateReconnecting {
			return m, nil
		}
		if !msg.ok {
			// Host still unreachable (or auth broken): hand off to manual
			// recovery instead of launching a blocking interactive ssh.
			m.state = StateConnectionLost
			return m, nil
		}
		return m, m.attachLastSessionCmd()

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
	case StateConnectionLost:
		return m.updateConnectionLost(msg)
	default:
		return m, nil
	}
}

// updateConnectionLost handles the manual-recovery screen shown after automatic
// reattach attempts are exhausted. [r] re-initiates the attach (resetting the
// attempt counter); [esc] is handled upstream in handleBack().
func (m Model) updateConnectionLost(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if key.Matches(keyMsg, m.keys.Reconnect) {
			m.reconnectAttempts = 0
			m.reconnectGen++
			m.state = StateReconnecting
			return m, m.attachLastSessionCmd()
		}
	}
	return m, nil
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
	if m.projects == nil || len(m.projects.Projects) == 0 {
		return m, nil
	}

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
				// Fresh, user-initiated attach: remember the target (and its
				// project dir) and reset the reconnect counter so a later drop
				// gets a full retry budget. Open a new epoch so any stale tick
				// from a prior attach is dropped.
				m.lastSession = session.Name
				m.lastProjectPath = m.currentProjectPath
				m.reconnectAttempts = 0
				m.reconnectGen++
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
		// Clear error on any key press
		m.sessionNameInputErr = ""

		switch msg.Type {
		case tea.KeyEnter:
			suffix := strings.TrimSpace(m.sessionNameInput.Value())

			// Validate suffix
			if strings.Contains(suffix, ".") {
				m.sessionNameInputErr = "suffix cannot contain dots"
				return m, nil
			}
			if strings.ContainsAny(suffix, " \t\n\r") {
				m.sessionNameInputErr = "suffix cannot contain whitespace"
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
					m.sessionNameInputErr = fmt.Sprintf("session %q already exists", suffix)
					return m, nil
				}
			}

			// Defensive nil check for remote mode
			if !m.isLocal && m.currentHost == nil {
				m.sessionNameInputErr = "no host selected"
				return m, nil
			}

			// Create the session. The create command attaches directly and
			// reports its outcome via sessionExitedMsg, so track the new session
			// as the reconnect target now and open a fresh epoch; otherwise a
			// transport drop (exit 255) on the first attach would re-fire against
			// a stale or empty lastSession. Capture the project dir too so a
			// reconnect that has to recreate the session uses the right cwd.
			m.lastSession = name
			m.lastProjectPath = m.currentProjectPath
			m.reconnectAttempts = 0
			m.reconnectGen++
			m.state = StateCreatingSession
			m.sessionNameInputErr = ""
			if m.isLocal {
				return m, createSessionWithNameLocalCmd(name, m.currentProjectPath)
			}
			return m, createSessionWithNameCmd(*m.currentHost, name, m.currentProjectPath)

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
	case StateSessionNameInput:
		m.sessionNameInput.Blur()
		m.sessionNameInputErr = ""
		m.state = StateSessionSelect
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
	case StateReconnecting, StateConnectionLost:
		// Cancel reconnection and return to the session list. Bump the epoch so
		// a backoff tick already scheduled cannot re-fire the attach.
		m.reconnectAttempts = 0
		m.reconnectGen++
		m.state = StateSessionSelect
		return m, nil
	case StateError:
		// Recover from the dead-end error screen: clear the error and return to
		// the first usable screen for the current mode. The error may have fired
		// before that screen's list was ever built (e.g. a startup host- or
		// project-load failure), so (re)initialize the list from whatever data we
		// have rather than entering a zero-value list.Model — its nil delegate
		// would panic on the next update or render.
		m.err = nil
		m.reconnectAttempts = 0
		m.reconnectGen++
		m.ensureMinDimensions()
		if m.isLocal {
			var projects []config.Project
			if m.projects != nil {
				projects = m.projects.Projects
			}
			m.SetProjects(projects)
			m.state = StateProjectSelect
		} else {
			m.SetHosts(m.hosts) // also sets state = StateHostSelect
		}
		return m, nil
	}
	return m, nil
}

// isTransportFailure reports whether err is an ssh transport failure, signalled
// by exit code 255. Such failures mean the connection dropped rather than zmx
// itself erroring, so they warrant a reconnect rather than a fatal error.
func isTransportFailure(err error) bool {
	if err == nil {
		return false
	}
	var ee *exec.ExitError
	return errors.As(err, &ee) && ee.ExitCode() == 255
}

// reconnectProbeCmd runs a bounded, non-interactive reachability check against
// the current host before an automatic interactive reattach. It uses the
// runner's non-interactive path (BatchMode + ConnectTimeout), so it fails fast
// and never prompts — unlike the interactive attach it guards, which allocates
// a PTY and can block indefinitely on a password prompt. The reconnect epoch is
// captured so a result that arrives after a cancel/navigation is discarded.
func (m Model) reconnectProbeCmd() tea.Cmd {
	runner := m.runner
	gen := m.reconnectGen
	return func() tea.Msg {
		if runner == nil {
			return reconnectProbeMsg{gen: gen, ok: false}
		}
		_, err := runner.Run("true")
		return reconnectProbeMsg{gen: gen, ok: err == nil}
	}
}

// attachLastSessionCmd re-fires the interactive attach for the last-attached
// session, using the appropriate transport for the current mode.
//
// When the session's project directory is known it reconnects through the
// project-aware create command (cd <dir> && zmx attach). zmx attach reattaches
// to an existing session (the common dropped-connection case, where the cd is a
// harmless no-op) but creates a missing one in the current directory — so if the
// transport dropped before the initial create ever established the session, this
// recreates it in its project directory rather than the login/default dir. With
// no known directory it falls back to a plain attach (a bare `cd ''` would
// otherwise abort the command).
func (m Model) attachLastSessionCmd() tea.Cmd {
	if m.lastProjectPath == "" {
		if m.isLocal {
			return attachSessionLocalCmd(m.lastSession)
		}
		if m.currentHost == nil {
			return nil
		}
		return attachSessionCmd(*m.currentHost, m.lastSession)
	}
	if m.isLocal {
		return createSessionWithNameLocalCmd(m.lastSession, m.lastProjectPath)
	}
	if m.currentHost == nil {
		return nil
	}
	return createSessionWithNameCmd(*m.currentHost, m.lastSession, m.lastProjectPath)
}

// ensureMinDimensions sets default dimensions if WindowSizeMsg hasn't arrived yet.
func (m *Model) ensureMinDimensions() {
	if m.width == 0 {
		m.width = 80
	}
	if m.height == 0 {
		m.height = 24
	}
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

	// Render a bare error only outside StateError; the StateError branch below
	// renders the same error plus a recovery hint, so let the switch handle it.
	if m.err != nil && m.state != StateError {
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
	case StateReconnecting:
		hint := dimStyle.Render("[esc] cancel")
		return pad + m.spinner.View() + " Connection lost — reattaching…\n\n" + hint
	case StateConnectionLost:
		hint := dimStyle.Render("[r] reconnect   [esc] back   [q] quit")
		return pad + ErrorStyle.Render("Connection lost.") + "\n\n" + hint
	case StateError:
		hint := dimStyle.Render("[esc] back   [q] quit")
		msg := ""
		if m.err != nil {
			msg = m.err.Error()
		}
		return pad + ErrorStyle.Render("Error: "+msg) + "\n\n" + hint
	case StateHelp:
		return m.helpView()
	}
	return ""
}

// dimStyle renders hint text in a muted color, matching the session-name-input
// view's footer styling.
var dimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

// sessionNameInputView renders the session name input screen.
func (m Model) sessionNameInputView() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Creating session for: %s\n\n", m.currentProjectKey))
	b.WriteString("Session suffix (empty for auto): ")
	b.WriteString(m.sessionNameInput.View())
	b.WriteString("\n")
	if m.sessionNameInputErr != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("Error: " + m.sessionNameInputErr))
	}
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("[Enter] Create  [Esc] Cancel"))
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
  ctrl+↑/↓  Reorder hosts
  Enter     Connect to host

Project Screen:
  ctrl+↑/↓  Reorder projects
  s         Scan for new projects
  d         Delete project
  Enter     View sessions

Session Screen:
  n         New session
  x         Kill session
  Enter     Attach to session

Inside zmx session:
  Ctrl+\    Detach (return to ccc)

Connection lost:
  (auto-reattach on a dropped session)
  r         Reconnect now
  Esc       Back to session list

Error screen:
  Esc       Back   q  Quit

Press ? to close this help.
`
	return lipgloss.NewStyle().Padding(1, 2).Render(help)
}
