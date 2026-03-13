# Session Naming & Host/Project Ordering Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add session naming prompts when creating sessions and enable custom ordering of hosts/projects via config arrays and TUI reordering.

**Architecture:** Two independent features sharing config format changes. Config migrates from map-based TOML to array-based TOML on first load. TUI adds text input state for session naming and Ctrl+arrow handling for reordering.

**Tech Stack:** Go, Bubbletea (TUI), bubbles/textinput, TOML (pelletier/go-toml/v2)

**Spec:** `docs/superpowers/specs/2026-03-13-session-naming-and-ordering-design.md`

---

## File Structure

### Config Layer
| File | Responsibility |
|------|----------------|
| `config/client.go` | Host struct with Name field, slice-based ClientConfig, migration from old format |
| `config/projects.go` | Project struct with Name field, slice-based ProjectsConfig, migration |
| `config/client_test.go` | Tests for new format parsing, migration, round-trip |
| `config/projects_test.go` | Tests for new format parsing, migration, round-trip |

### TUI Layer
| File | Responsibility |
|------|----------------|
| `tui/state.go` | Add StateSessionNameInput constant |
| `tui/keys.go` | Add MoveUp, MoveDown key bindings |
| `tui/model.go` | Text input field, change `hosts` field to `[]config.Host`, session naming flow, reorder handling |
| `tui/items.go` | Update HostItem/ProjectItem to work with embedded Name field |
| `tui/hostlist.go` | Accept []Host, update help keys |
| `tui/projectlist.go` | Accept []Project, update help keys |
| `tui/commands.go` | Update createSessionCmd signature, save commands, update scanCompleteMsg handler |
| `tui/model_test.go` | Tests for new flows |

---

## Chunk 1: Config Format Migration (Hosts)

### Task 1.1: Add Host.Name field and slice-based config struct

**Files:**
- Modify: `config/client.go`
- Modify: `config/client_test.go`

- [ ] **Step 1: Write test for new array format parsing**

```go
// In config/client_test.go

func TestParseArrayFormat(t *testing.T) {
	data := `
[[hosts]]
name = "server1"
user = "deploy"
address = "10.0.0.1"

[[hosts]]
name = "server2"
user = "admin"
address = "10.0.0.2"
port = 2222
`
	cfg, err := ParseClientConfigData([]byte(data))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(cfg.Hosts) != 2 {
		t.Fatalf("expected 2 hosts, got %d", len(cfg.Hosts))
	}
	if cfg.Hosts[0].Name != "server1" {
		t.Errorf("expected server1, got %s", cfg.Hosts[0].Name)
	}
	if cfg.Hosts[1].Port != 2222 {
		t.Errorf("expected port 2222, got %d", cfg.Hosts[1].Port)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./config/ -run TestParseArrayFormat -v`
Expected: FAIL - ParseClientConfigData doesn't exist or Hosts is wrong type

- [ ] **Step 3: Update Host struct and ClientConfig**

```go
// In config/client.go - update Host struct

type Host struct {
	Name              string   `toml:"name"`
	User              string   `toml:"user"`
	Address           string   `toml:"address"`
	Port              int      `toml:"port,omitempty"`
	IdentityFile      string   `toml:"identity_file,omitempty"`
	ProxyJump         string   `toml:"proxy_jump,omitempty"`
	SSHOptions        []string `toml:"ssh_options,omitempty"`
	FallbackAddresses []string `toml:"fallback_addresses,omitempty"`
}

// Update ClientConfig
type ClientConfig struct {
	Hosts []Host `toml:"hosts"`
}

// Add ParseClientConfigData for testability
func ParseClientConfigData(data []byte) (*ClientConfig, error) {
	var cfg ClientConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.Hosts == nil {
		cfg.Hosts = []Host{}
	}
	return &cfg, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./config/ -run TestParseArrayFormat -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add config/client.go config/client_test.go
git commit -m "feat(config): add array-based host config format"
```

---

### Task 1.2: Add HostByName helper

**Files:**
- Modify: `config/client.go`
- Modify: `config/client_test.go`

- [ ] **Step 1: Write test for HostByName**

```go
func TestHostByName(t *testing.T) {
	cfg := &ClientConfig{
		Hosts: []Host{
			{Name: "server1", User: "deploy", Address: "10.0.0.1"},
			{Name: "server2", User: "admin", Address: "10.0.0.2"},
		},
	}

	host, found := cfg.HostByName("server2")
	if !found {
		t.Fatal("expected to find server2")
	}
	if host.User != "admin" {
		t.Errorf("expected admin, got %s", host.User)
	}

	_, found = cfg.HostByName("nonexistent")
	if found {
		t.Error("expected not found for nonexistent")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./config/ -run TestHostByName -v`
Expected: FAIL - HostByName undefined

- [ ] **Step 3: Implement HostByName**

```go
// In config/client.go

// HostByName finds a host by name. Returns the host and true if found.
func (c *ClientConfig) HostByName(name string) (Host, bool) {
	for _, h := range c.Hosts {
		if h.Name == name {
			return h, true
		}
	}
	return Host{}, false
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./config/ -run TestHostByName -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add config/client.go config/client_test.go
git commit -m "feat(config): add HostByName lookup helper"
```

---

### Task 1.3: Add migration from old map format

**Files:**
- Modify: `config/client.go`
- Modify: `config/client_test.go`

- [ ] **Step 1: Write test for migration**

```go
func TestMigrateOldFormat(t *testing.T) {
	oldData := `
[hosts.server1]
user = "deploy"
address = "10.0.0.1"

[hosts.server2]
user = "admin"
address = "10.0.0.2"
port = 2222
`
	cfg, err := ParseClientConfigData([]byte(oldData))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(cfg.Hosts) != 2 {
		t.Fatalf("expected 2 hosts, got %d", len(cfg.Hosts))
	}
	// After migration, hosts should have names and be sorted alphabetically
	names := make([]string, len(cfg.Hosts))
	for i, h := range cfg.Hosts {
		names[i] = h.Name
	}
	if names[0] != "server1" || names[1] != "server2" {
		t.Errorf("expected [server1, server2], got %v", names)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./config/ -run TestMigrateOldFormat -v`
Expected: FAIL - old format not parsed into Hosts slice

- [ ] **Step 3: Implement migration logic**

```go
// In config/client.go

// oldClientConfig is the legacy map-based format
type oldClientConfig struct {
	Hosts map[string]Host `toml:"hosts"`
}

// ParseClientConfigData parses TOML data, migrating from old format if needed.
func ParseClientConfigData(data []byte) (*ClientConfig, error) {
	// Try new array format first
	var cfg ClientConfig
	if err := toml.Unmarshal(data, &cfg); err == nil && len(cfg.Hosts) > 0 {
		return &cfg, nil
	}

	// Try old map format
	var oldCfg oldClientConfig
	if err := toml.Unmarshal(data, &oldCfg); err != nil {
		return nil, err
	}

	// Migrate: convert map to slice, sorted alphabetically
	if len(oldCfg.Hosts) > 0 {
		names := make([]string, 0, len(oldCfg.Hosts))
		for name := range oldCfg.Hosts {
			names = append(names, name)
		}
		sort.Strings(names)

		cfg.Hosts = make([]Host, len(names))
		for i, name := range names {
			h := oldCfg.Hosts[name]
			h.Name = name
			cfg.Hosts[i] = h
		}
		return &cfg, nil
	}

	// Empty config
	cfg.Hosts = []Host{}
	return &cfg, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./config/ -run TestMigrateOldFormat -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add config/client.go config/client_test.go
git commit -m "feat(config): migrate old map-based host format to array"
```

---

### Task 1.4: Update LoadClientConfig to use new parsing

**Files:**
- Modify: `config/client.go`
- Modify: `config/client_test.go`

- [ ] **Step 1: Write integration test for LoadClientConfig**

```go
func TestLoadClientConfigMigration(t *testing.T) {
	// Create temp file with old format
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	oldData := `
[hosts.myhost]
user = "testuser"
address = "192.168.1.1"
`
	if err := os.WriteFile(path, []byte(oldData), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadClientConfig(path)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	if len(cfg.Hosts) != 1 {
		t.Fatalf("expected 1 host, got %d", len(cfg.Hosts))
	}
	if cfg.Hosts[0].Name != "myhost" {
		t.Errorf("expected myhost, got %s", cfg.Hosts[0].Name)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./config/ -run TestLoadClientConfigMigration -v`
Expected: FAIL - LoadClientConfig doesn't use new parsing

- [ ] **Step 3: Update LoadClientConfig**

```go
// In config/client.go - update LoadClientConfig

func LoadClientConfig(path string) (*ClientConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNoConfig
		}
		return nil, err
	}

	return ParseClientConfigData(data)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./config/ -run TestLoadClientConfigMigration -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add config/client.go config/client_test.go
git commit -m "feat(config): LoadClientConfig uses new parsing with migration"
```

---

### Task 1.5: Update SaveClientConfig for new format

**Files:**
- Modify: `config/client.go`
- Modify: `config/client_test.go`

- [ ] **Step 1: Write round-trip test**

```go
func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	original := &ClientConfig{
		Hosts: []Host{
			{Name: "host1", User: "user1", Address: "1.1.1.1"},
			{Name: "host2", User: "user2", Address: "2.2.2.2", Port: 22},
		},
	}

	if err := SaveClientConfig(path, original); err != nil {
		t.Fatalf("save error: %v", err)
	}

	loaded, err := LoadClientConfig(path)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}

	if len(loaded.Hosts) != 2 {
		t.Fatalf("expected 2 hosts, got %d", len(loaded.Hosts))
	}
	if loaded.Hosts[0].Name != "host1" || loaded.Hosts[1].Name != "host2" {
		t.Error("hosts order not preserved")
	}
}
```

- [ ] **Step 2: Run test to verify it passes** (SaveClientConfig should already work with new struct)

Run: `go test ./config/ -run TestSaveLoadRoundTrip -v`
Expected: PASS (toml.Marshal handles slices correctly)

- [ ] **Step 3: Remove SortedHostNames (no longer needed)**

```go
// In config/client.go - REMOVE this function:
// func (c *ClientConfig) SortedHostNames() []string { ... }
```

- [ ] **Step 4: Run all config tests**

Run: `go test ./config/ -v`
Expected: Some tests may fail due to SortedHostNames removal - fix in next step

- [ ] **Step 5: Fix any failing tests that used SortedHostNames**

Update test code that called `SortedHostNames()` to iterate over `cfg.Hosts` directly.

- [ ] **Step 6: Run all config tests again**

Run: `go test ./config/ -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add config/client.go config/client_test.go
git commit -m "refactor(config): remove SortedHostNames, slice order is display order"
```

---

## Chunk 2: Config Format Migration (Projects)

### Task 2.1: Add Project.Name field and slice-based config

**Files:**
- Modify: `config/projects.go`
- Modify: `config/projects_test.go`

- [ ] **Step 1: Write test for new array format**

```go
func TestParseProjectsArrayFormat(t *testing.T) {
	data := `
[[projects]]
name = "project1"
path = "/home/user/project1"

[[projects]]
name = "project2"
path = "/home/user/project2"
`
	cfg, err := ParseProjectsConfig([]byte(data))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(cfg.Projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(cfg.Projects))
	}
	if cfg.Projects[0].Name != "project1" {
		t.Errorf("expected project1, got %s", cfg.Projects[0].Name)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./config/ -run TestParseProjectsArrayFormat -v`
Expected: FAIL

- [ ] **Step 3: Update Project struct and ProjectsConfig**

```go
// In config/projects.go

type Project struct {
	Name string `toml:"name"`
	Path string `toml:"path"`
}

type ProjectsConfig struct {
	Projects []Project `toml:"projects"`
}

// oldProjectsConfig is the legacy map-based format
type oldProjectsConfig struct {
	Projects map[string]struct {
		Path string `toml:"path"`
	} `toml:"projects"`
}

// ParseProjectsConfig parses TOML data, migrating from old format if needed.
func ParseProjectsConfig(data []byte) (*ProjectsConfig, error) {
	// Try new array format first
	var cfg ProjectsConfig
	if err := toml.Unmarshal(data, &cfg); err == nil && len(cfg.Projects) > 0 {
		return &cfg, nil
	}

	// Try old map format
	var oldCfg oldProjectsConfig
	if err := toml.Unmarshal(data, &oldCfg); err != nil {
		return nil, err
	}

	// Migrate: convert map to slice, sorted alphabetically
	if len(oldCfg.Projects) > 0 {
		names := make([]string, 0, len(oldCfg.Projects))
		for name := range oldCfg.Projects {
			names = append(names, name)
		}
		sort.Strings(names)

		cfg.Projects = make([]Project, len(names))
		for i, name := range names {
			cfg.Projects[i] = Project{
				Name: name,
				Path: oldCfg.Projects[name].Path,
			}
		}
		return &cfg, nil
	}

	cfg.Projects = []Project{}
	return &cfg, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./config/ -run TestParseProjectsArrayFormat -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add config/projects.go config/projects_test.go
git commit -m "feat(config): add array-based project config with migration"
```

---

### Task 2.2: Add ProjectByName helper and remove SortedProjectKeys

**Files:**
- Modify: `config/projects.go`
- Modify: `config/projects_test.go`

- [ ] **Step 1: Write test for ProjectByName**

```go
func TestProjectByName(t *testing.T) {
	cfg := &ProjectsConfig{
		Projects: []Project{
			{Name: "proj1", Path: "/path/to/proj1"},
			{Name: "proj2", Path: "/path/to/proj2"},
		},
	}

	proj, found := cfg.ProjectByName("proj2")
	if !found {
		t.Fatal("expected to find proj2")
	}
	if proj.Path != "/path/to/proj2" {
		t.Errorf("expected /path/to/proj2, got %s", proj.Path)
	}

	_, found = cfg.ProjectByName("nonexistent")
	if found {
		t.Error("expected not found")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./config/ -run TestProjectByName -v`
Expected: FAIL

- [ ] **Step 3: Implement ProjectByName and remove SortedProjectKeys**

```go
// In config/projects.go

// ProjectByName finds a project by name. Returns the project and true if found.
func (c *ProjectsConfig) ProjectByName(name string) (Project, bool) {
	for _, p := range c.Projects {
		if p.Name == name {
			return p, true
		}
	}
	return Project{}, false
}

// REMOVE: SortedProjectKeys - no longer needed
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./config/ -run TestProjectByName -v`
Expected: PASS

- [ ] **Step 5: Fix any tests using SortedProjectKeys**

Update tests to iterate over `cfg.Projects` directly.

- [ ] **Step 6: Run all config tests**

Run: `go test ./config/ -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add config/projects.go config/projects_test.go
git commit -m "feat(config): add ProjectByName, remove SortedProjectKeys"
```

---

## Chunk 3: TUI Session Name Input

### Task 3.1: Add StateSessionNameInput

**Files:**
- Modify: `tui/state.go`

- [ ] **Step 1: Add new state constant**

```go
// In tui/state.go - add to const block after existing states

const (
	StateLoading State = iota
	StateHostSelect
	StateProjectSelect
	StateSessionSelect
	StateSessionNameInput  // NEW
	StateCreatingSession
	StateConnecting
	StateError
	StateHelp
)

// Update String() method
func (s State) String() string {
	switch s {
	// ... existing cases ...
	case StateSessionNameInput:
		return "SessionNameInput"
	// ... rest ...
	}
}
```

- [ ] **Step 2: Run build to verify no errors**

Run: `go build ./...`
Expected: SUCCESS

- [ ] **Step 3: Commit**

```bash
git add tui/state.go
git commit -m "feat(tui): add StateSessionNameInput state"
```

---

### Task 3.2: Add text input field to Model

**Files:**
- Modify: `tui/model.go`

- [ ] **Step 1: Add textinput import and field**

```go
// In tui/model.go - add import
import (
	// ... existing imports ...
	"github.com/charmbracelet/bubbles/textinput"
)

// Add field to Model struct
type Model struct {
	// ... existing fields ...

	// Session name input
	sessionNameInput textinput.Model
}

// Update New() to initialize the textinput
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
		keys:             Keys(),
		isLocal:          isLocal,
		sessionNameInput: ti,
	}
}
```

- [ ] **Step 2: Run build to verify no errors**

Run: `go build ./...`
Expected: SUCCESS

- [ ] **Step 3: Commit**

```bash
git add tui/model.go
git commit -m "feat(tui): add textinput field for session naming"
```

---

### Task 3.3: Handle 'n' key to enter name input state

**Files:**
- Modify: `tui/model.go`

- [ ] **Step 1: Update updateSessionSelect to transition on 'n'**

```go
// In tui/model.go - modify updateSessionSelect

func (m Model) updateSessionSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.sessionList, cmd = m.sessionList.Update(msg)

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMsg, m.keys.Select):
			// ... existing select logic ...
		case key.Matches(keyMsg, m.keys.New):
			// Transition to name input instead of creating directly
			m.sessionNameInput.Reset()
			m.sessionNameInput.Focus()
			m.state = StateSessionNameInput
			return m, textinput.Blink
		case key.Matches(keyMsg, m.keys.Kill):
			// ... existing kill logic ...
		}
	}

	return m, cmd
}
```

- [ ] **Step 2: Run build to verify no errors**

Run: `go build ./...`
Expected: SUCCESS

- [ ] **Step 3: Commit**

```bash
git add tui/model.go
git commit -m "feat(tui): 'n' key transitions to session name input"
```

---

### Task 3.4: Add createSessionWithNameCmd commands

**Files:**
- Modify: `tui/commands.go`

- [ ] **Step 1: Add new command functions that take explicit name**

```go
// In tui/commands.go

// createSessionWithNameCmd creates and attaches to a session with explicit name.
func createSessionWithNameCmd(host config.Host, name, projectPath string) tea.Cmd {
	args := []string{"-t", host.Address}
	if host.User != "" {
		args = []string{"-t", host.User + "@" + host.Address}
	}
	if host.Port != 0 {
		args = append([]string{"-p", fmt.Sprintf("%d", host.Port)}, args...)
	}
	args = append(args, zmx.BuildCreateCommand(name, projectPath))

	cmd := exec.Command("ssh", args...)
	cmd.Env = os.Environ()

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return sessionExitedMsg{err: err}
	})
}

// createSessionWithNameLocalCmd creates a session with explicit name locally.
func createSessionWithNameLocalCmd(name, projectPath string) tea.Cmd {
	cmd := exec.Command("sh", "-c", zmx.BuildCreateCommand(name, projectPath))
	cmd.Env = os.Environ()

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return sessionExitedMsg{err: err}
	})
}
```

- [ ] **Step 2: Run build to verify no errors**

Run: `go build ./...`
Expected: SUCCESS

- [ ] **Step 3: Commit**

```bash
git add tui/commands.go
git commit -m "feat(tui): add createSessionWithNameCmd for explicit naming"
```

---

### Task 3.5: Add updateSessionNameInput handler

**Files:**
- Modify: `tui/model.go`

- [ ] **Step 1: Add handler for session name input state**

```go
// In tui/model.go - add new function

func (m Model) updateSessionNameInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			suffix := strings.TrimSpace(m.sessionNameInput.Value())

			// Validate suffix
			if strings.Contains(suffix, ".") {
				// Show error - suffix can't contain dots
				// For now, just ignore invalid input
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
					// Conflict - stay in input
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
```

- [ ] **Step 2: Wire up handler in updateState**

```go
// In tui/model.go - update updateState

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
```

- [ ] **Step 3: Run build to verify no errors**

Run: `go build ./...`
Expected: SUCCESS

- [ ] **Step 4: Commit**

```bash
git add tui/model.go
git commit -m "feat(tui): add session name input handler"
```

---

### Task 3.6: Add View for session name input state

**Files:**
- Modify: `tui/model.go`

- [ ] **Step 1: Update View() to render input state**

```go
// In tui/model.go - update View()

func (m Model) View() string {
	// ... existing code ...

	switch m.state {
	// ... existing cases ...
	case StateSessionNameInput:
		return pad + m.sessionNameInputView()
	// ... rest ...
	}
	return ""
}

// Add new helper
func (m Model) sessionNameInputView() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Creating session for: %s\n\n", m.currentProjectKey))
	b.WriteString("Session suffix (empty for auto): ")
	b.WriteString(m.sessionNameInput.View())
	b.WriteString("\n\n")
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("[Enter] Create  [Esc] Cancel"))
	return b.String()
}
```

- [ ] **Step 2: Run build and manual test**

Run: `go build ./...`
Expected: SUCCESS

- [ ] **Step 3: Commit**

```bash
git add tui/model.go
git commit -m "feat(tui): add view for session name input state"
```

---

## Chunk 4: TUI Reordering

### Task 4.1: Add MoveUp/MoveDown key bindings

**Files:**
- Modify: `tui/keys.go`

- [ ] **Step 1: Add new key bindings**

```go
// In tui/keys.go - add to keyMap struct

type keyMap struct {
	// ... existing fields ...
	MoveUp   key.Binding
	MoveDown key.Binding
}

// Update Keys() function
func Keys() keyMap {
	return keyMap{
		// ... existing bindings ...
		MoveUp: key.NewBinding(
			key.WithKeys("ctrl+up"),
			key.WithHelp("ctrl+↑", "move up"),
		),
		MoveDown: key.NewBinding(
			key.WithKeys("ctrl+down"),
			key.WithHelp("ctrl+↓", "move down"),
		),
	}
}
```

- [ ] **Step 2: Run build to verify no errors**

Run: `go build ./...`
Expected: SUCCESS

- [ ] **Step 3: Commit**

```bash
git add tui/keys.go
git commit -m "feat(tui): add MoveUp/MoveDown key bindings"
```

---

### Task 4.2: Update hostlist to accept []Host

**Files:**
- Modify: `tui/hostlist.go`
- Modify: `tui/items.go`

- [ ] **Step 1: Update NewHostList signature**

```go
// In tui/hostlist.go

func NewHostList(hosts []config.Host, width, height int) list.Model {
	items := make([]list.Item, len(hosts))
	for i, h := range hosts {
		items[i] = NewHostItem(h.Name, h)
	}

	l := list.New(items, newDelegate(), width, height)
	l.Title = "Hosts"
	configureList(&l)

	return l
}
```

- [ ] **Step 2: Run build to see what breaks**

Run: `go build ./...`
Expected: Errors in model.go where NewHostList is called

- [ ] **Step 3: Update Model struct and callers in model.go**

```go
// In tui/model.go - CHANGE hosts field type from map to slice

type Model struct {
	// ... other fields ...
	hosts       []config.Host      // CHANGED from map[string]config.Host
	// ... rest ...
}

// Update SetHosts
func (m *Model) SetHosts(hosts []config.Host) {
	m.hosts = hosts  // Store the slice
	m.hostList = NewHostList(hosts, m.width, m.height-2)
	m.state = StateHostSelect
}
```

Also update the hostsLoadedMsg handler to assign the slice directly.

- [ ] **Step 4: Run build to verify no errors**

Run: `go build ./...`
Expected: SUCCESS

- [ ] **Step 5: Commit**

```bash
git add tui/hostlist.go tui/model.go
git commit -m "refactor(tui): hostlist accepts []Host directly"
```

---

### Task 4.3: Update projectlist to accept []Project

**Files:**
- Modify: `tui/projectlist.go`
- Modify: `tui/model.go`

- [ ] **Step 1: Update NewProjectList signature**

```go
// In tui/projectlist.go

func NewProjectList(projects []config.Project, width, height int) list.Model {
	items := make([]list.Item, len(projects))
	for i, p := range projects {
		items[i] = NewProjectItem(p.Name, p)
	}

	l := list.New(items, newDelegate(), width, height)
	l.Title = "Projects"
	configureList(&l)
	l.AdditionalShortHelpKeys = projectKeys

	return l
}
```

- [ ] **Step 2: Update callers in model.go**

```go
// In tui/model.go - update SetProjects

func (m *Model) SetProjects(projects []config.Project) {
	m.projectList = NewProjectList(projects, m.width, m.height-4)
}
```

- [ ] **Step 3: Run build to verify no errors**

Run: `go build ./...`
Expected: SUCCESS

- [ ] **Step 4: Commit**

```bash
git add tui/projectlist.go tui/model.go
git commit -m "refactor(tui): projectlist accepts []Project directly"
```

---

### Task 4.4: Handle Ctrl+arrow for host reordering

**Files:**
- Modify: `tui/model.go`

- [ ] **Step 1: Add reorder handling to updateHostSelect**

```go
// In tui/model.go - update updateHostSelect

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
			// ... existing select logic ...
		}
	}

	return m, cmd
}

// reorderHost moves the selected host up (-1) or down (+1)
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
```

- [ ] **Step 2: Add saveHostsCmd**

```go
// In tui/commands.go

func saveHostsCmd(hosts []config.Host) tea.Cmd {
	return func() tea.Msg {
		path, err := config.DefaultClientConfigPath()
		if err != nil {
			return errMsg{err}
		}
		cfg := &config.ClientConfig{Hosts: hosts}
		if err := config.SaveClientConfig(path, cfg); err != nil {
			return errMsg{err}
		}
		return nil // success, no message needed
	}
}
```

- [ ] **Step 3: Run build to verify no errors**

Run: `go build ./...`
Expected: SUCCESS

- [ ] **Step 4: Commit**

```bash
git add tui/model.go tui/commands.go
git commit -m "feat(tui): Ctrl+arrow reorders hosts"
```

---

### Task 4.5: Handle Ctrl+arrow for project reordering

**Files:**
- Modify: `tui/model.go`
- Modify: `tui/commands.go`

- [ ] **Step 1: Add reorder handling to updateProjectSelect**

```go
// In tui/model.go - update updateProjectSelect

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
			// ... existing select logic ...
		// ... rest of existing cases ...
		}
	}

	return m, cmd
}

// reorderProject moves the selected project up (-1) or down (+1)
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
```

- [ ] **Step 2: Run build to verify no errors**

Run: `go build ./...`
Expected: SUCCESS

- [ ] **Step 3: Commit**

```bash
git add tui/model.go
git commit -m "feat(tui): Ctrl+arrow reorders projects"
```

---

## Chunk 5: Update TUI Commands and Messages

### Task 5.1: Update loadHostsCmd for new format

**Files:**
- Modify: `tui/commands.go`
- Modify: `tui/messages.go`

- [ ] **Step 1: Update hostsLoadedMsg to use slice**

```go
// In tui/messages.go - update message type

type hostsLoadedMsg struct {
	hosts []config.Host  // Changed from map[string]Host
}
```

- [ ] **Step 2: Update loadHostsCmd**

```go
// In tui/commands.go

func loadHostsCmd() tea.Cmd {
	return func() tea.Msg {
		path, err := config.DefaultClientConfigPath()
		if err != nil {
			return errMsg{err}
		}
		cfg, err := config.LoadClientConfig(path)
		if err != nil {
			return errMsg{err}
		}
		return hostsLoadedMsg{
			hosts: cfg.Hosts,  // Now a slice
		}
	}
}
```

- [ ] **Step 3: Update handler in model.go**

```go
// In tui/model.go - update hostsLoadedMsg handler

case hostsLoadedMsg:
	m.hosts = msg.hosts
	m.SetHosts(msg.hosts)
	return m, nil
```

- [ ] **Step 4: Run build to verify no errors**

Run: `go build ./...`
Expected: SUCCESS

- [ ] **Step 5: Commit**

```bash
git add tui/commands.go tui/messages.go tui/model.go
git commit -m "refactor(tui): loadHostsCmd uses slice-based config"
```

---

### Task 5.2: Update projectsLoadedMsg for new format

**Files:**
- Modify: `tui/commands.go`
- Modify: `tui/model.go`

- [ ] **Step 1: Verify projectsLoadedMsg already uses ProjectsConfig**

The existing code passes `*config.ProjectsConfig` which now contains `[]Project`.

- [ ] **Step 2: Update SetProjects call**

```go
// In tui/model.go - update projectsLoadedMsg handler

case projectsLoadedMsg:
	m.projects = msg.projects
	m.SetProjects(msg.projects.Projects)  // Pass the slice
	m.state = StateProjectSelect
	return m, nil
```

- [ ] **Step 3: Run build to verify no errors**

Run: `go build ./...`
Expected: SUCCESS

- [ ] **Step 4: Commit**

```bash
git add tui/model.go
git commit -m "refactor(tui): projectsLoadedMsg uses slice-based config"
```

---

### Task 5.3: Update scanCompleteMsg handler for slice-based projects

**Files:**
- Modify: `tui/model.go`

- [ ] **Step 1: Update scanCompleteMsg handler**

The current handler uses map-based assignment:
```go
m.projects.Projects[r.key] = config.Project{Path: r.path}
```

Update to append to slice:
```go
// In tui/model.go - update scanCompleteMsg handler

case scanCompleteMsg:
	if m.projects == nil {
		m.projects = &config.ProjectsConfig{
			Projects: []config.Project{},
		}
	}
	for _, r := range msg.results {
		// Check if project already exists
		found := false
		for i, p := range m.projects.Projects {
			if p.Name == r.key {
				m.projects.Projects[i].Path = r.path
				found = true
				break
			}
		}
		if !found {
			m.projects.Projects = append(m.projects.Projects, config.Project{
				Name: r.key,
				Path: r.path,
			})
		}
	}
	// Save and reload
	if m.isLocal {
		return m, saveProjectsLocalCmd(m.projects)
	}
	return m, saveProjectsCmd(m.runner, m.projects)
```

- [ ] **Step 2: Run build to verify no errors**

Run: `go build ./...`
Expected: SUCCESS

- [ ] **Step 3: Commit**

```bash
git add tui/model.go
git commit -m "fix(tui): scanCompleteMsg uses slice-based projects"
```

---

## Chunk 6: Fix Remaining Compilation and Run Tests

### Task 6.1: Fix any remaining compilation errors

**Files:**
- Various

- [ ] **Step 1: Full build**

Run: `go build ./...`
Expected: Fix any errors that appear

- [ ] **Step 2: Fix errors iteratively**

Common fixes needed:
- Update `flow/common.go` if it still uses old config types (deprecated but may need compile)
- Update any test files using old signatures

- [ ] **Step 3: Commit fixes**

```bash
git add -A
git commit -m "fix: resolve compilation errors from config migration"
```

---

### Task 6.2: Run all tests

**Files:**
- All test files

- [ ] **Step 1: Run all tests**

Run: `go test ./... -v`
Expected: Note failures

- [ ] **Step 2: Fix failing tests**

Update tests to use new slice-based APIs.

- [ ] **Step 3: Run tests again**

Run: `go test ./... -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "test: update tests for new config format"
```

---

### Task 6.3: Manual testing

- [ ] **Step 1: Build and run**

```bash
go build -o ccc . && ./ccc
```

- [ ] **Step 2: Test session naming**

1. Navigate to a project
2. Press 'n' - should see input prompt
3. Type a suffix, press Enter - session created with that name
4. Press 'n' again, just press Enter - auto-name used

- [ ] **Step 3: Test host reordering**

1. On host list, press Ctrl+Down - host moves down
2. Press Ctrl+Up - host moves up
3. Quit and restart - order persisted

- [ ] **Step 4: Test project reordering**

1. Connect to a host
2. On project list, Ctrl+Down/Up
3. Reconnect - order persisted

- [ ] **Step 5: Test migration**

1. Create old-format config manually
2. Run ccc - should migrate and work
3. Check config file - now in new format

---

## Summary

**Total Tasks:** 22
**Estimated Time:** 2-3 hours

**Key Commits:**
1. Array-based host config
2. HostByName helper
3. Host format migration
4. Array-based project config
5. ProjectByName helper
6. Session name input state
7. Session name input handler
8. MoveUp/MoveDown keys
9. Host reordering
10. Project reordering
