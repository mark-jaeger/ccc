# Bubbletea TUI Patterns Research

**Project:** ccc (CLI for managing remote terminal sessions)
**Researched:** 2026-03-11
**Confidence:** HIGH (multiple official sources, active ecosystem)

## Executive Summary

Bubbletea is a mature, well-documented TUI framework for Go based on The Elm Architecture. It provides a declarative Model-View-Update pattern that maps well to ccc's multi-screen navigation needs (host -> project -> session). The Charmbracelet ecosystem includes battle-tested components (bubbles) for lists with fuzzy filtering, styling (lipgloss), forms (huh), and even SSH server capabilities (wish). The framework has strong community adoption and recently released v2.0 with significant rendering improvements.

**Recommendation:** Use Bubbletea v2 with bubbles/list for the selection menus. The list component has built-in fuzzy filtering and easily customizable keybindings. The state machine pattern with nested models handles the host -> project -> session flow cleanly.

---

## Architecture Overview

### The Elm Architecture (TEA)

Bubbletea implements The Elm Architecture with three core components:

```
┌─────────────────────────────────────────────────────────────┐
│                         tea.Program                         │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────┐    ┌──────────┐    ┌─────────┐                │
│  │  Init() │───>│  Model   │───>│  View() │───> Terminal   │
│  └─────────┘    └────┬─────┘    └─────────┘                │
│                      │                                      │
│                      v                                      │
│               ┌──────────────┐                              │
│               │   Update()   │<─── Messages (keys, resize,  │
│               └──────────────┘      custom events)          │
└─────────────────────────────────────────────────────────────┘
```

**Model**: Application state (a Go struct)
**Init()**: Returns initial command to run
**Update(msg)**: Handles events, returns updated model + commands
**View()**: Renders model to string (pure function, no side effects)

### Core Pattern

```go
type model struct {
    state     appState  // Current screen
    hosts     []Host
    projects  []Project
    sessions  []Session
    // Sub-models for components
    hostList    list.Model
    projectList list.Model
    sessionList list.Model
}

type appState int

const (
    stateHostSelect appState = iota
    stateProjectSelect
    stateSessionSelect
)

func (m model) Init() tea.Cmd {
    return nil // or initial data fetch
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch m.state {
        case stateHostSelect:
            return m.updateHostSelect(msg)
        case stateProjectSelect:
            return m.updateProjectSelect(msg)
        case stateSessionSelect:
            return m.updateSessionSelect(msg)
        }
    }
    return m, nil
}

func (m model) View() string {
    switch m.state {
    case stateHostSelect:
        return m.hostList.View()
    case stateProjectSelect:
        return m.projectList.View()
    case stateSessionSelect:
        return m.sessionList.View()
    }
    return ""
}
```

---

## Component Recommendations for ccc

### 1. bubbles/list - Primary Selection Component

The list component is the core building block for ccc. It provides:

- **Fuzzy filtering** (built-in, uses sahilm/fuzzy)
- **Pagination** for long lists
- **Customizable keybindings**
- **Status messages and help**
- **Spinner for loading states**

```go
import "github.com/charmbracelet/bubbles/list"

// Items must implement list.Item interface
type hostItem struct {
    name string
    addr string
}

func (i hostItem) FilterValue() string { return i.name }
func (i hostItem) Title() string       { return i.name }
func (i hostItem) Description() string { return i.addr }

// Create list with default delegate
func newHostList(hosts []Host, width, height int) list.Model {
    items := make([]list.Item, len(hosts))
    for i, h := range hosts {
        items[i] = hostItem{name: h.Name, addr: h.Address}
    }

    delegate := list.NewDefaultDelegate()

    l := list.New(items, delegate, width, height)
    l.Title = "Select Host"
    l.SetShowStatusBar(true)
    l.SetFilteringEnabled(true)  // Fuzzy search with /

    return l
}
```

### 2. Vim-Style Keybindings

Override the default keybindings to use j/k:

```go
func customKeyMap() list.KeyMap {
    return list.KeyMap{
        // Cursor movement
        CursorUp: key.NewBinding(
            key.WithKeys("up", "k"),
            key.WithHelp("k/up", "up"),
        ),
        CursorDown: key.NewBinding(
            key.WithKeys("down", "j"),
            key.WithHelp("j/down", "down"),
        ),
        // Page navigation
        PrevPage: key.NewBinding(
            key.WithKeys("left", "h", "pgup", "b", "u"),
            key.WithHelp("h/pgup", "prev page"),
        ),
        NextPage: key.NewBinding(
            key.WithKeys("right", "l", "pgdown", "f", "d"),
            key.WithHelp("l/pgdown", "next page"),
        ),
        // Selection
        GoToStart: key.NewBinding(
            key.WithKeys("home", "g"),
            key.WithHelp("g/home", "go to start"),
        ),
        GoToEnd: key.NewBinding(
            key.WithKeys("end", "G"),
            key.WithHelp("G/end", "go to end"),
        ),
        // Filter
        Filter: key.NewBinding(
            key.WithKeys("/"),
            key.WithHelp("/", "filter"),
        ),
        ClearFilter: key.NewBinding(
            key.WithKeys("esc"),
            key.WithHelp("esc", "clear filter"),
        ),
        // Cancel/quit
        CancelWhileFiltering: key.NewBinding(
            key.WithKeys("esc"),
            key.WithHelp("esc", "cancel"),
        ),
        AcceptWhileFiltering: key.NewBinding(
            key.WithKeys("enter", "tab"),
            key.WithHelp("enter", "apply filter"),
        ),
        Quit: key.NewBinding(
            key.WithKeys("q", "esc"),
            key.WithHelp("q", "quit"),
        ),
        ShowFullHelp: key.NewBinding(
            key.WithKeys("?"),
            key.WithHelp("?", "more"),
        ),
        CloseFullHelp: key.NewBinding(
            key.WithKeys("?"),
            key.WithHelp("?", "close help"),
        ),
    }
}

// Apply to list
l := list.New(items, delegate, width, height)
l.KeyMap = customKeyMap()
```

### 3. lipgloss Styling

```go
import "github.com/charmbracelet/lipgloss"

var (
    // Colors
    subtle    = lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#383838"}
    highlight = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}
    special   = lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#73F59F"}

    // Styles
    titleStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color("#FFFDF5")).
        Background(highlight).
        Padding(0, 1)

    itemStyle = lipgloss.NewStyle().
        PaddingLeft(4)

    selectedItemStyle = lipgloss.NewStyle().
        PaddingLeft(2).
        Foreground(highlight)

    statusBarStyle = lipgloss.NewStyle().
        Foreground(lipgloss.AdaptiveColor{Light: "#343433", Dark: "#C1C6B2"}).
        Background(subtle)
)

// Apply to delegate
delegate := list.NewDefaultDelegate()
delegate.Styles.SelectedTitle = selectedItemStyle
delegate.Styles.SelectedDesc = selectedItemStyle.Foreground(subtle)
```

### 4. textinput - For Session Name Input

When creating a new session:

```go
import "github.com/charmbracelet/bubbles/textinput"

func newSessionInput() textinput.Model {
    ti := textinput.New()
    ti.Placeholder = "session-name"
    ti.Focus()
    ti.CharLimit = 32
    ti.Width = 30
    return ti
}
```

### 5. spinner - For Loading States

When connecting to SSH or fetching data:

```go
import "github.com/charmbracelet/bubbles/spinner"

func newSpinner() spinner.Model {
    s := spinner.New()
    s.Spinner = spinner.Dot
    s.Style = lipgloss.NewStyle().Foreground(highlight)
    return s
}
```

---

## Multi-Screen Navigation Pattern

### State Machine Approach

For ccc's host -> project -> session flow:

```go
package tui

import (
    "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/bubbles/list"
)

// Application states
type appState int

const (
    stateLoading appState = iota
    stateHostSelect
    stateProjectSelect
    stateSessionSelect
    stateCreatingSession
    stateConnecting
)

// Custom messages for state transitions
type (
    hostsLoadedMsg    []Host
    projectsLoadedMsg []Project
    sessionsLoadedMsg []Session
    hostSelectedMsg   Host
    projectSelectedMsg Project
    sessionSelectedMsg Session
    errorMsg          error
)

type model struct {
    state       appState
    err         error

    // Data
    hosts       []Host
    projects    []Project
    sessions    []Session

    // Current selections
    selectedHost    *Host
    selectedProject *Project

    // Sub-models
    hostList    list.Model
    projectList list.Model
    sessionList list.Model
    spinner     spinner.Model
    input       textinput.Model

    // Dimensions
    width  int
    height int
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {

    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        // Resize all lists
        m.hostList.SetSize(msg.Width, msg.Height-4)
        m.projectList.SetSize(msg.Width, msg.Height-4)
        m.sessionList.SetSize(msg.Width, msg.Height-4)
        return m, nil

    case tea.KeyMsg:
        // Global quit
        if msg.String() == "ctrl+c" {
            return m, tea.Quit
        }

        // Handle back navigation
        if msg.String() == "esc" && !m.isFiltering() {
            return m.handleBack()
        }

    case hostsLoadedMsg:
        m.hosts = msg
        m.hostList = newHostList(msg, m.width, m.height-4)
        m.state = stateHostSelect
        return m, nil

    case projectsLoadedMsg:
        m.projects = msg
        m.projectList = newProjectList(msg, m.width, m.height-4)
        m.state = stateProjectSelect
        return m, nil

    case sessionsLoadedMsg:
        m.sessions = msg
        m.sessionList = newSessionList(msg, m.width, m.height-4)
        m.state = stateSessionSelect
        return m, nil

    case errorMsg:
        m.err = msg
        return m, nil
    }

    // Delegate to current screen
    return m.updateCurrentScreen(msg)
}

func (m model) updateCurrentScreen(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmd tea.Cmd

    switch m.state {
    case stateHostSelect:
        m.hostList, cmd = m.hostList.Update(msg)

        // Check for selection
        if key, ok := msg.(tea.KeyMsg); ok && key.String() == "enter" {
            if item := m.hostList.SelectedItem(); item != nil {
                host := item.(hostItem).host
                m.selectedHost = &host
                return m, m.loadProjects(host)
            }
        }

    case stateProjectSelect:
        m.projectList, cmd = m.projectList.Update(msg)

        if key, ok := msg.(tea.KeyMsg); ok && key.String() == "enter" {
            if item := m.projectList.SelectedItem(); item != nil {
                project := item.(projectItem).project
                m.selectedProject = &project
                return m, m.loadSessions(project)
            }
        }

    case stateSessionSelect:
        m.sessionList, cmd = m.sessionList.Update(msg)

        if key, ok := msg.(tea.KeyMsg); ok {
            switch key.String() {
            case "enter":
                if item := m.sessionList.SelectedItem(); item != nil {
                    session := item.(sessionItem).session
                    return m, m.attachSession(session)
                }
            case "n":
                m.state = stateCreatingSession
                m.input = newSessionInput()
                return m, m.input.Focus()
            }
        }
    }

    return m, cmd
}

func (m model) handleBack() (tea.Model, tea.Cmd) {
    switch m.state {
    case stateSessionSelect:
        m.state = stateProjectSelect
        m.selectedProject = nil
    case stateProjectSelect:
        m.state = stateHostSelect
        m.selectedHost = nil
    case stateHostSelect:
        return m, tea.Quit
    }
    return m, nil
}

func (m model) View() string {
    if m.err != nil {
        return errorView(m.err)
    }

    switch m.state {
    case stateLoading:
        return m.spinner.View() + " Loading..."
    case stateHostSelect:
        return m.hostList.View()
    case stateProjectSelect:
        return m.breadcrumb() + "\n" + m.projectList.View()
    case stateSessionSelect:
        return m.breadcrumb() + "\n" + m.sessionList.View()
    case stateCreatingSession:
        return m.createSessionView()
    case stateConnecting:
        return m.spinner.View() + " Connecting..."
    }
    return ""
}

func (m model) breadcrumb() string {
    var parts []string
    if m.selectedHost != nil {
        parts = append(parts, m.selectedHost.Name)
    }
    if m.selectedProject != nil {
        parts = append(parts, m.selectedProject.Name)
    }
    return lipgloss.NewStyle().
        Foreground(subtle).
        Render(strings.Join(parts, " > "))
}
```

### Command Pattern for Async Operations

SSH connections and data fetching should use commands:

```go
// Commands return messages
func (m model) loadHosts() tea.Cmd {
    return func() tea.Msg {
        hosts, err := m.config.LoadHosts()
        if err != nil {
            return errorMsg(err)
        }
        return hostsLoadedMsg(hosts)
    }
}

func (m model) loadProjects(host Host) tea.Cmd {
    return func() tea.Msg {
        // This runs in a goroutine managed by tea.Program
        conn, err := ssh.Connect(host)
        if err != nil {
            return errorMsg(err)
        }
        defer conn.Close()

        projects, err := conn.ListProjects()
        if err != nil {
            return errorMsg(err)
        }
        return projectsLoadedMsg(projects)
    }
}

// For interactive SSH (attach session), use tea.ExecProcess
func (m model) attachSession(session Session) tea.Cmd {
    c := exec.Command("ssh", "-t", m.selectedHost.Address,
        "abduco", "-a", session.Name)
    return tea.ExecProcess(c, func(err error) tea.Msg {
        // Called when process exits
        return sessionExitedMsg{err: err}
    })
}
```

---

## Integration with ccc's Existing Architecture

### Bridging TUI and Runner Interface

ccc already has a `Runner` interface. The TUI can use it via commands:

```go
type tuiModel struct {
    runner flow.Runner  // Injected SSH connection or LocalRunner
    // ... other fields
}

func (m tuiModel) fetchProjects() tea.Cmd {
    return func() tea.Msg {
        output, err := m.runner.Run(scan.BuildScanChainCommand())
        if err != nil {
            return errorMsg(err)
        }
        projects := parseProjects(output)
        return projectsLoadedMsg(projects)
    }
}

func (m tuiModel) attachSession(session string) tea.Cmd {
    // For interactive commands, need to handle differently
    // Option 1: Exit TUI temporarily with tea.ExecProcess
    // Option 2: Use Wish for SSH-over-SSH TUI
    cmd := abduco.BuildAttachCommand(session)
    return tea.ExecProcess(
        exec.Command("bash", "-c", m.runner.BuildFullCommand(cmd)),
        func(err error) tea.Msg {
            return sessionExitedMsg{err}
        },
    )
}
```

### Wish Integration (Optional)

Wish is NOT needed for ccc's use case. Wish is for building SSH *servers* that serve TUIs. ccc is an SSH *client*.

However, if you wanted to run the TUI *on the remote host* and serve it via SSH:

```go
// This is NOT what ccc needs, but shown for completeness
import "github.com/charmbracelet/wish"

func main() {
    s, err := wish.NewServer(
        wish.WithAddress(":2222"),
        wish.WithMiddleware(
            bubbletea.Middleware(teaHandler),
        ),
    )
    s.ListenAndServe()
}
```

**For ccc's architecture:** The TUI runs locally and uses SSH to execute remote commands. No Wish needed.

---

## Testing Patterns

### Unit Testing Models

Test Update() and View() directly:

```go
func TestHostListNavigation(t *testing.T) {
    hosts := []Host{{Name: "server1"}, {Name: "server2"}}
    m := initialModel(hosts)

    // Simulate key press
    newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
    m = newModel.(model)

    // Assert selection moved
    selected := m.hostList.SelectedItem().(hostItem)
    if selected.name != "server2" {
        t.Errorf("expected server2, got %s", selected.name)
    }
}

func TestViewRendering(t *testing.T) {
    m := initialModel([]Host{{Name: "test-host"}})
    m.state = stateHostSelect

    view := m.View()

    if !strings.Contains(view, "test-host") {
        t.Error("view should contain host name")
    }
}
```

### Integration Testing with teatest

```go
import "github.com/charmbracelet/x/exp/teatest"

func TestFullFlow(t *testing.T) {
    m := initialModel(testHosts)
    tm := teatest.NewTestModel(t, m)

    // Wait for initial render
    teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
        return strings.Contains(string(out), "Select Host")
    })

    // Navigate and select
    tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
    tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

    // Verify state change
    teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
        return strings.Contains(string(out), "Select Project")
    })
}
```

### Golden File Testing

Capture and compare rendered output:

```go
func TestViewGolden(t *testing.T) {
    m := initialModel(testData)
    m.state = stateHostSelect

    golden := filepath.Join("testdata", "host_select.golden")

    if *update {
        os.WriteFile(golden, []byte(m.View()), 0644)
        return
    }

    expected, _ := os.ReadFile(golden)
    if m.View() != string(expected) {
        t.Errorf("view mismatch")
    }
}
```

---

## Recommended Project Structure

```
ccc/
├── main.go
├── flow/
│   ├── common.go      # Runner interface (existing)
│   ├── remote.go      # Remote mode (existing)
│   └── local.go       # Local mode (existing)
├── tui/
│   ├── tui.go         # Main model, Init/Update/View, tea.Program setup
│   ├── state.go       # State constants and transition logic
│   ├── styles.go      # lipgloss styles
│   ├── keys.go        # KeyMap definitions
│   ├── commands.go    # tea.Cmd functions (async operations)
│   ├── components/
│   │   ├── hostlist.go     # Host list model wrapper
│   │   ├── projectlist.go  # Project list model wrapper
│   │   └── sessionlist.go  # Session list model wrapper
│   └── tui_test.go    # Tests
├── ssh/               # (existing)
├── abduco/            # (existing)
├── scan/              # (existing)
├── config/            # (existing)
└── ui/                # (existing, can be deprecated or kept for non-TUI mode)
```

### Alternative: Single-File Approach

For simpler integration, keep all TUI code in one file initially:

```
ccc/
├── main.go
├── tui.go             # All TUI code in one file
├── flow/              # (existing)
└── ...
```

---

## Migration Strategy for ccc

### Phase 1: Parallel Implementation

1. Create `tui/` package alongside existing `ui/`
2. Implement host selection with bubbletea
3. Add `--tui` flag to switch between old and new UI

### Phase 2: Feature Parity

1. Implement project selection
2. Implement session selection with create/attach
3. Add fuzzy filtering
4. Add vim keybindings

### Phase 3: Full Migration

1. Make TUI the default
2. Keep `--no-tui` flag for scripting/compatibility
3. Deprecate old `ui/` package

### Key Integration Points

```go
// main.go
func main() {
    if *useTUI {
        // New TUI mode
        p := tea.NewProgram(tui.New(config), tea.WithAltScreen())
        if _, err := p.Run(); err != nil {
            log.Fatal(err)
        }
    } else {
        // Existing flow
        flow.RunRemoteMode(config)
    }
}
```

---

## Example Projects to Study

### Highly Relevant

1. **SSHM** - [github.com/Gu1llaum-3/sshm](https://github.com/Gu1llaum-3/sshm)
   - SSH connection manager with Bubbletea TUI
   - Host selection, connection management
   - Very similar use case to ccc

2. **TUIOS** - [github.com/Gaurav-Gosain/tuios](https://github.com/Gaurav-Gosain/tuios)
   - Terminal multiplexer with Bubbletea v2
   - Session management, vim-like interface
   - Uses Lipgloss v2 styling

3. **Official Examples** - [github.com/charmbracelet/bubbletea/examples](https://github.com/charmbracelet/bubbletea/tree/main/examples)
   - `list-fancy/` - Custom delegate styling
   - `views/` - Multi-view navigation pattern
   - `exec/` - Running external processes

### Component Examples

4. **Bubbles List Example** - [bubbletea/examples/list-simple](https://github.com/charmbracelet/bubbletea/blob/main/examples/list-simple/main.go)
   - Basic list with filtering

5. **Huh Forms** - [github.com/charmbracelet/huh](https://github.com/charmbracelet/huh)
   - For creating sessions (text input with validation)

---

## Version Recommendations

### Use Bubbletea v2 (if stable)

As of early 2026, v2.0 is released with:
- New "Cursed Renderer" with better performance
- Mode 2026 synchronized output (reduces flicker)
- Native clipboard support
- Progressive keyboard enhancements
- Pure lipgloss integration

```go
import tea "charm.land/bubbletea/v2"
import "charm.land/bubbles/v2/list"
import "charm.land/lipgloss/v2"
```

### Or Use Bubbletea v1.x (stable fallback)

If v2 has issues:
```go
import tea "github.com/charmbracelet/bubbletea"
import "github.com/charmbracelet/bubbles/list"
import "github.com/charmbracelet/lipgloss"
```

---

## Potential Pitfalls

### 1. PTY Handling with SSH

When attaching to a remote session via SSH, the TUI must exit cleanly to hand over the terminal:

```go
// Use tea.ExecProcess, NOT runner.RunInteractive from within Update()
func (m model) attachSession(session Session) tea.Cmd {
    return tea.ExecProcess(exec.Command("ssh", "-t", host, "abduco", "-a", session),
        func(err error) tea.Msg {
            return sessionExitedMsg{err}
        })
}
```

### 2. Window Resize Handling

Always handle `tea.WindowSizeMsg`:

```go
case tea.WindowSizeMsg:
    m.width = msg.Width
    m.height = msg.Height
    m.hostList.SetSize(msg.Width, msg.Height-2)
```

### 3. Filtering State

The list component has internal filtering state. Check before handling `esc`:

```go
func (m model) isFiltering() bool {
    switch m.state {
    case stateHostSelect:
        return m.hostList.FilterState() == list.Filtering
    // ... etc
    }
    return false
}
```

### 4. Alt Screen Mode

For full-screen TUI, use alt screen:

```go
p := tea.NewProgram(model, tea.WithAltScreen())
```

But when running external process, it handles its own screen.

---

## Sources

### Official Documentation
- [Bubbletea GitHub](https://github.com/charmbracelet/bubbletea)
- [Bubbletea v2 Release Notes](https://github.com/charmbracelet/bubbletea/discussions/1374)
- [Bubbles Components](https://github.com/charmbracelet/bubbles)
- [Lipgloss Styling](https://github.com/charmbracelet/lipgloss)
- [Huh Forms](https://github.com/charmbracelet/huh)
- [Wish SSH Server](https://github.com/charmbracelet/wish)

### Tutorials and Patterns
- [The Bubbletea State Machine Pattern](https://zackproser.com/blog/bubbletea-state-machine)
- [Tips for Building Bubbletea Programs](https://leg100.github.io/en/posts/building-bubbletea-programs/)
- [Managing Nested Models](https://donderom.com/posts/managing-nested-models-with-bubble-tea/)
- [Multi-model Message Routing Discussion](https://github.com/charmbracelet/bubbletea/discussions/751)
- [Nested Components Discussion](https://github.com/charmbracelet/bubbletea/discussions/176)

### Testing
- [Writing Bubbletea Tests](https://carlosbecker.com/posts/teatest/)
- [Catwalk Test Library](https://github.com/knz/catwalk)

### Related Projects
- [SSHM - SSH Manager](https://github.com/Gu1llaum-3/sshm)
- [TUIOS - Terminal Multiplexer](https://github.com/Gaurav-Gosain/tuios)
- [Soft Serve - Git Server](https://github.com/charmbracelet/soft-serve)
