# Rename Sessions & Delete Projects

## Overview

Add two user-initiated actions:
1. **Rename session** — Change an existing tmux session's name from the session menu
2. **Delete project** — Remove a project from ccc config from the project menu

## Key Mapping

### Project Menu

| Key | Action | Description |
|-----|--------|-------------|
| `s` | Scan | Scan for projects (existing) |
| `d` | Delete | Remove project from config (NEW) |
| `b` | Back | Return to previous menu |
| `q` | Quit | Exit ccc |

### Session Menu

| Key | Action | Description |
|-----|--------|-------------|
| `n` | New | Create new session (existing) |
| `r` | Rename | Rename selected session (NEW) |
| `x` | Kill | Kill session (was `r` Remove) |
| `t` | Detach | Detach other clients (was `d`) |
| `b` | Back | Return to project menu |
| `q` | Quit | Exit ccc |

## Rename Session Flow

1. User presses `r` in session menu
2. Prompt: `New suffix (enter for 'main'): `
3. Input handling:
   - Empty → rename to `{projectKey}-main`
   - User types `foo` → rename to `{projectKey}-foo`
4. Validate new name doesn't conflict with existing session
5. Execute `tmux rename-session -t {oldName} {newName}`
6. Show: `✓ Renamed {oldName} → {newName}`
7. Return to session menu with refreshed list

### Constraints

- Suffix is always required (no bare project names)
- New name must start with project key prefix
- Metadata (`@ccc_project`, `@ccc_path`) preserved automatically

### Edge Cases

- New name matches existing session → error, return to menu
- New name equals current name → no-op, return silently

## Delete Project Flow

1. User presses `d` in project menu
2. Prompt: `Select item to delete: ` (number input)
3. Confirm: `Delete {projectKey}? (y/n)`
4. On confirm:
   - Remove from `projects.Projects` map
   - Persist via `onSave` callback
   - Show: `✓ Deleted {projectKey}`
5. Return to project menu with refreshed list

### What It Does NOT Do

- Does not delete files on disk
- Does not kill associated tmux sessions

## Implementation

### Files to Modify

| File | Changes |
|------|---------|
| `tmux/sessions.go` | Add `BuildRenameCommand(oldName, newName string) string` |
| `flow/common.go` | Update `SessionFlow` ExtraActions, add `renameSession()`, update `ProjectFlow` for delete |
| `tmux/sessions_test.go` | Test for `BuildRenameCommand` |
| `flow/common_test.go` | Tests for rename flow, update tests for new keys |

### New Function

```go
// BuildRenameCommand returns a shell command to rename a tmux session.
func BuildRenameCommand(oldName, newName string) string {
    return fmt.Sprintf("%s rename-session -t %s %s",
        tmuxCmd(), shellutil.Quote(oldName), shellutil.Quote(newName))
}
```

### No Changes Needed

- `ui/menu.go` — existing patterns sufficient
- Config files — uses existing `onSave` callback
