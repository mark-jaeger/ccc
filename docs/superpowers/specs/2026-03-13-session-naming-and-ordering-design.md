# Session Naming & Host/Project Ordering

## Overview

Two enhancements to ccc:

1. **Session naming** — Prompt for session suffix when creating new sessions
2. **Host/project ordering** — Custom display order via TOML arrays and TUI reordering

## Feature 1: Session Naming

### Requirements

- Always prompt when pressing `n` to create a session
- User enters suffix only (full name is `ccc.{project}.{suffix}`)
- Empty input uses auto-generated name (`main`, `2`, `3`, etc.)
- Esc cancels and returns to session list

### New State

Add `StateSessionNameInput` to the TUI state machine. This works alongside existing `StateCreatingSession`:

```
SessionSelect → press 'n' → SessionNameInput → Enter → StateCreatingSession → attach
                                             → Esc   → SessionSelect
```

Note: `StateCreatingSession` already exists and handles the "creating..." spinner. `SessionNameInput` is the new input prompt that precedes it.

### UI

```
Creating session for: myproject

Session suffix (empty for auto): █

[Enter] Create  [Esc] Cancel
```

### Implementation

| File | Changes |
|------|---------|
| `tui/state.go` | Add `StateSessionNameInput` constant |
| `tui/model.go` | Add `sessionNameInput textinput.Model` field (import `github.com/charmbracelet/bubbles/textinput`) |
| `tui/model.go` | In `updateSessionSelect`, `n` key transitions to input state |
| `tui/model.go` | Add `updateSessionNameInput` handler |
| `tui/commands.go` | `createSessionCmd` takes explicit name parameter; TUI layer calls `zmx.NextAutoName` when input is empty, then passes result to command |

Note: `zmx/zmx.go` is unchanged. The conditional `NextAutoName` call happens in TUI before dispatching the command.

### Validation

- If suffix conflicts with existing session, show error and stay in input state
- Suffix must not contain `.` (would break `ccc.{project}.{suffix}` parsing)
- Suffix must not contain whitespace (spaces, tabs, newlines)
- Suffix is trimmed before validation

## Feature 2: Host/Project Ordering

### Requirements

- Display order matches config file order
- Ctrl+Up / Ctrl+Down to reorder in TUI
- Changes persist immediately to config
- Both hosts and projects support ordering

### Config Format Change

**New format** (array-based, order preserved):

```toml
[[hosts]]
name = "prod-server"
user = "deploy"
address = "10.0.0.1"
port = 22

[[hosts]]
name = "dev-box"
user = "mark"
address = "192.168.1.50"
```

**Old format** (map-based, will be migrated):

```toml
[hosts.prod-server]
user = "deploy"
address = "10.0.0.1"

[hosts.dev-box]
user = "mark"
address = "192.168.1.50"
```

Same pattern applies to `projects.toml`.

### Migration

On `LoadClientConfig`:

1. Try parsing as new array format
2. If empty/fails and file has content, try old map format
3. If old format detected, convert to array (alphabetical order) and save
4. Migration is silent and instant

### Struct Changes

```go
// config/client.go

type Host struct {
    Name              string   `toml:"name"` // NEW - was map key
    User              string   `toml:"user"`
    Address           string   `toml:"address"`
    Port              int      `toml:"port,omitempty"`
    IdentityFile      string   `toml:"identity_file,omitempty"`
    ProxyJump         string   `toml:"proxy_jump,omitempty"`
    SSHOptions        []string `toml:"ssh_options,omitempty"`
    FallbackAddresses []string `toml:"fallback_addresses,omitempty"`
}

type ClientConfig struct {
    Hosts []Host `toml:"hosts"` // Changed from map[string]Host
}

// Remove SortedHostNames() - slice order IS the order
// Add HostByName(name string) (Host, bool) helper - returns value + found flag, consistent with existing patterns
```

```go
// config/projects.go

type Project struct {
    Name string `toml:"name"` // NEW - was map key
    Path string `toml:"path"`
}

type ProjectsConfig struct {
    Projects []Project `toml:"projects"` // Changed from map[string]Project
}
```

### TUI Reordering

**Keys**: `ctrl+up`, `ctrl+down`

**Behavior**:

- Works on host list and project list only (sessions are transient)
- Disabled when list is filtered (would be confusing)
- Cursor follows moved item
- At top/bottom edge: no-op
- Config saved immediately after each move

### Implementation

| File | Changes |
|------|---------|
| `config/client.go` | Change `Hosts` to slice, add `Name` field, add migration |
| `config/projects.go` | Change `Projects` to slice, add `Name` field, add migration |
| `tui/keys.go` | Add `MoveUp`, `MoveDown` bindings |
| `tui/model.go` | Handle Ctrl+arrow in `updateHostSelect`, `updateProjectSelect` |
| `tui/model.go` | Add `reorderHosts()`, `reorderProjects()` helpers |
| `tui/hostlist.go` | Change `NewHostList(hosts map[string]Host, names []string, ...)` to `NewHostList(hosts []Host, ...)` — name now embedded in struct |
| `tui/projectlist.go` | Change `NewProjectList(projects map[string]Project, keys []string, ...)` to `NewProjectList(projects []Project, ...)` — name now embedded |
| `tui/commands.go` | Update save commands for new format |

### Save Behavior

- **Hosts**: Always saved to local `~/.ccc/config.toml` (even in remote mode, hosts are client-side)
- **Projects**: Saved to local `~/.ccc/projects.toml` in local mode, or via SSH to remote `~/.ccc/projects.toml` in remote mode

### Helper Functions

```go
// Swap items in a slice (generic or duplicated per type)
func moveUp(slice []T, index int) []T
func moveDown(slice []T, index int) []T

// Find by name - returns value + found flag (not pointer, consistent with existing patterns)
func (c *ClientConfig) HostByName(name string) (Host, bool)
func (c *ProjectsConfig) ProjectByName(name string) (Project, bool)
```

## Files Summary

### Modified

| File | Reason |
|------|--------|
| `config/client.go` | Slice-based hosts, migration |
| `config/projects.go` | Slice-based projects, migration |
| `config/client_test.go` | Migration tests, new format |
| `config/projects_test.go` | Migration tests, new format |
| `tui/state.go` | Add `StateSessionNameInput` |
| `tui/model.go` | Text input, reorder handling |
| `tui/keys.go` | Move up/down bindings |
| `tui/hostlist.go` | Slice input, help update |
| `tui/projectlist.go` | Slice input, help update |
| `tui/commands.go` | Explicit session name, updated saves |
| `tui/model_test.go` | New state tests |

### Unchanged

| File | Reason |
|------|--------|
| `zmx/zmx.go` | `NextAutoName` still used as fallback |
| `ssh/*` | Unaffected |
| `scan/*` | Unaffected |
| `flow/*` | Deprecated, leave as-is |

## Edge Cases

### Session Naming

- Empty input → use `NextAutoName` result
- Suffix contains `.` → reject with error message
- Suffix matches existing → reject with error message
- Esc during input → return to session list, no session created

### Ordering

- Single item in list → reorder keys do nothing
- Filtered list → reorder disabled (only full list)
- Migration with duplicate host names → keep first, warn in logs
- Empty config file → initialize empty slice (no migration needed)

## Testing

### Unit Tests

- `config/client_test.go`: Parse new format, migrate old format, round-trip
- `config/projects_test.go`: Same
- `tui/model_test.go`: Session name input flow, reorder behavior

### Manual Testing

1. Create session with custom name
2. Create session with empty input (auto-name)
3. Try invalid suffix (with `.`)
4. Reorder hosts with Ctrl+arrows
5. Verify order persists after restart
6. Verify old config migrates correctly
