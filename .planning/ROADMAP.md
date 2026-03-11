# Roadmap: ccc v2.0 — zmx + Bubbletea

**Milestone:** v2.0
**Created:** 2026-03-11
**Phases:** 2

---

## Phase Overview

| Phase | Name | Goal | Key Deliverables |
|-------|------|------|------------------|
| 1 | zmx-backend | Replace abduco with zmx | zmx package, flow migration, TERM handling |
| 2 | bubbletea-tui | Replace basic menus with Bubbletea | tui package, state machine, fuzzy filtering, styling |

---

## Phase 1: zmx-backend

**Goal:** Replace abduco with zmx as session persistence backend

**Why first:** zmx fixes the terminal passthrough issues. Can use existing menu UI while developing.

### Requirements Covered

- FR-1.1 through FR-1.8 (zmx backend)
- FR-4.1 through FR-4.6 (terminal passthrough)
- FR-3.7 (hand terminal to zmx)

### Key Tasks

1. Create `zmx/` package with:
   - `zmx.Session` struct (name, pid, clients, startedIn)
   - `BuildListCommand()` → `zmx list`
   - `BuildAttachCommand(name)` → `TERM=$TERM zmx attach <name>`
   - `BuildKillCommand(name)` → `zmx kill <name>`
   - `ParseListOutput(string)` → `[]Session`
   - `FilterSessionsForProject(sessions, project)` → `[]Session`

2. Update `flow/` to use zmx instead of abduco:
   - Replace abduco imports with zmx
   - Update `SessionFlow` to use zmx commands
   - Handle TERM passthrough in SSH execution
   - Update session naming (can simplify from `ccc.{project}.{suffix}` if zmx handles it better)

3. Delete `abduco/` package

4. Update tests

### Success Criteria

- `ccc` connects to host, lists zmx sessions, can attach/create/kill
- Terminal passthrough works (notifications, Shift+Enter, scrolling, URLs)
- State restoration works on reattach

### Estimated Complexity

Similar to v1.0 Phase 1 (abduco package creation): ~2-3 plans

---

## Phase 2: bubbletea-tui

**Goal:** Replace basic menu UI with modern Bubbletea TUI

**Why second:** Once zmx works, we can iterate on UX without worrying about backend.

### Requirements Covered

- FR-2.1 through FR-2.10 (Bubbletea TUI)
- FR-3.1 through FR-3.6 (orchestration via TUI)

### Key Tasks

1. Create `tui/` package with:
   - Main model with state machine (host → project → session)
   - `hostList` component (bubbles/list with fuzzy filter)
   - `projectList` component
   - `sessionList` component with create/attach/kill actions
   - lipgloss styling
   - Custom keymap (vim j/k, g/G, /)

2. Integrate with existing flow:
   - Use `Runner` interface for SSH commands
   - Use `tea.ExecProcess` to hand terminal to zmx
   - Handle async operations (SSH connect, project scan, session list)

3. Update `main.go`:
   - Replace `flow.RunRemoteMode` with `tea.NewProgram(tui.New(...))`
   - Keep `--no-tui` flag for scripting compatibility

4. Deprecate/remove old `ui/` package

5. Tests for TUI state transitions

### Success Criteria

- Beautiful TUI with fuzzy filtering
- Vim-style navigation works
- Back navigation with Esc
- Loading states for async operations
- Clean handoff to zmx sessions

### Estimated Complexity

Larger than Phase 1: ~3-4 plans (new TUI framework, multiple components)

---

## Phase Dependencies

```
Phase 1 (zmx-backend)
    │
    └──> Phase 2 (bubbletea-tui)
```

Phase 2 depends on Phase 1 because the TUI needs zmx commands to work.

---

## Validation Strategy

### Phase 1 Validation
- Unit tests for zmx command building and parsing
- Integration test: SSH → zmx list → parse
- Manual test: UAT-2 (terminal features)

### Phase 2 Validation
- Unit tests for TUI state transitions
- Integration test: full flow with mock runner
- Manual test: UAT-1 (basic flow), UAT-3 (error handling)

---

## Risk Mitigation

| Risk | Phase | Mitigation |
|------|-------|------------|
| zmx TERM issues | 1 | Already tested workaround: `TERM=$TERM` wrapper |
| zmx parsing changes | 1 | Pin to known output format, add version check |
| Bubbletea v2 instability | 2 | Can use v1.x import path as fallback |
| tea.ExecProcess handoff | 2 | Study SSHM project for pattern |

---

## Post-Milestone

After v2.0:
- Monitor zmx updates for IPC-breaking changes
- Consider `zmx run`/`zmx wait`/`zmx history` integration for agent workflows
- Consider session templates or quick-attach shortcuts
