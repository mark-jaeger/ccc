# Roadmap: ccc v2.0 — zmx + Bubbletea

**Milestone:** v2.0
**Created:** 2026-03-11
**Phases:** 1

---

## Phase 1: zmx-bubbletea

**Goal:** Replace abduco with zmx AND replace basic menus with Bubbletea TUI

**Requirements:** [FR-1.1, FR-1.2, FR-1.3, FR-1.4, FR-1.5, FR-1.6, FR-1.7, FR-1.8, FR-2.1, FR-2.2, FR-2.3, FR-2.4, FR-2.5, FR-2.6, FR-2.7, FR-2.8, FR-2.9, FR-2.10, FR-3.1, FR-3.2, FR-3.3, FR-3.4, FR-3.5, FR-3.6, FR-3.7, FR-4.1, FR-4.2, FR-4.3, FR-4.4, FR-4.5, FR-4.6]

**Plans:** 6 plans

Plans:
- [x] 01-01-PLAN.md — zmx package with command builders and session parsing
- [x] 01-02-PLAN.md — TUI core with state machine, styles, and keybindings
- [x] 01-03-PLAN.md — TUI list components for hosts, projects, sessions
- [x] 01-04-PLAN.md — Async commands and orchestration wiring
- [ ] 01-05-PLAN.md — Main.go integration and flow deprecation
- [ ] 01-06-PLAN.md — Tests and abduco cleanup

### Wave Structure

| Wave | Plans | Description |
|------|-------|-------------|
| 1 | 01-01, 01-02 | zmx package + TUI core (parallel) |
| 2 | 01-03, 01-04 | TUI components + async commands |
| 3 | 01-05, 01-06 | Integration + cleanup |

### Success Criteria

- Beautiful TUI with fuzzy filtering and vim keys
- zmx sessions work (create/attach/kill)
- Terminal passthrough works (notifications, Shift+Enter, scrolling, URLs)
- State restoration on reattach

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
