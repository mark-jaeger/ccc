# Roadmap: ccc v2.0 — zmx + Bubbletea

**Milestone:** v2.0
**Created:** 2026-03-11
**Phases:** 1

---

## Phase 1: zmx-bubbletea

**Goal:** Replace abduco with zmx AND replace basic menus with Bubbletea TUI

### Requirements Covered

All requirements from REQUIREMENTS.md:
- FR-1.x (zmx backend)
- FR-2.x (Bubbletea TUI)
- FR-3.x (orchestration)
- FR-4.x (terminal passthrough)

### Key Tasks

**zmx Package:**
1. Create `zmx/` package with:
   - `zmx.Session` struct (name, pid, clients, startedIn)
   - `BuildListCommand()` → `zmx list`
   - `BuildAttachCommand(name)` → `TERM=$TERM zmx attach <name>`
   - `BuildKillCommand(name)` → `zmx kill <name>`
   - `ParseListOutput(string)` → `[]Session`
   - `FilterSessionsForProject(sessions, project)` → `[]Session`

**Bubbletea TUI:**
2. Create `tui/` package with:
   - Main model with state machine (host → project → session)
   - `hostList` component (bubbles/list with fuzzy filter)
   - `projectList` component
   - `sessionList` component with create/attach/kill actions
   - lipgloss styling
   - Custom keymap (vim j/k, g/G, /)
   - `tea.ExecProcess` for zmx handoff

**Integration:**
3. Update `main.go` to use TUI
4. Delete `abduco/` package
5. Deprecate/remove old `ui/` package
6. Tests

### Success Criteria

- Beautiful TUI with fuzzy filtering and vim keys
- zmx sessions work (create/attach/kill)
- Terminal passthrough works (notifications, Shift+Enter, scrolling, URLs)
- State restoration on reattach

### Estimated Complexity

~4-5 plans (zmx package, TUI core, TUI components, integration, cleanup)

---

## Validation Strategy

- Unit tests for zmx command building and parsing
- Unit tests for TUI state transitions
- Integration test: full flow with mock runner
- Manual test: all UATs from REQUIREMENTS.md

---

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| zmx TERM issues | Already tested: `TERM=$TERM` wrapper works |
| Bubbletea v2 instability | Can use v1.x import path as fallback |
| tea.ExecProcess handoff | Study SSHM project for pattern |
