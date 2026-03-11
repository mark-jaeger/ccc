---
gsd_state_version: 1.0
milestone: v2.0
milestone_name: zmx + Bubbletea
current_plan: 01-05
status: executing
last_updated: "2026-03-11T12:11:08Z"
progress:
  total_phases: 1
  completed_phases: 0
  total_plans: 6
  completed_plans: 4
---

# Project State: ccc v2.0

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-11)

**Core value:** Transparent terminal passthrough with beautiful TUI
**Current focus:** Executing v2.0 - zmx backend + Bubbletea UI

## Current Position

- **Milestone:** v2.0
- **Phase:** 1 of 1 (zmx-bubbletea)
- **Current Plan:** 01-05
- **Status:** Executing (01-01, 01-02, 01-03, 01-04 complete)

## Key Decisions

| Decision | Date | Rationale |
|----------|------|-----------|
| zmx instead of abduco | 2026-03-11 | Tested: fixes notifications, Shift+Enter, scrolling, URLs |
| Bubbletea TUI | 2026-03-11 | User wants nicer UI than basic menus |
| Single milestone for both | 2026-03-11 | Cohesive rewrite, clean break from v1.x |
| TERM passthrough via wrapper | 2026-03-11 | `TERM=$TERM zmx attach` needed for SSH |
| zmx kill uses name not PID | 2026-03-11 | zmx CLI uses session names directly for kill |
| Session.Clients/StartedIn fields | 2026-03-11 | zmx list provides more info than abduco |
| Bubbletea v1.x (github.com/charmbracelet) | 2026-03-11 | v2 charm.land path not yet available |
| Breadcrumb navigation pattern | 2026-03-11 | Shows host > project context during navigation |
| HostItem takes name+Host | 2026-03-11 | config.Host lacks Name field (map key is name) |
| Help overlay with prevState | 2026-03-11 | Return to previous screen when closing help |
| Local Runner interface in tui | 2026-03-11 | Avoid import cycle with flow package |
| tea.ExecProcess for zmx attach | 2026-03-11 | Full terminal passthrough with TERM inheritance |

## Blockers

None.

## Session Log

| Date | Session | Outcome |
|------|---------|---------|
| 2026-03-11 | Milestone planning | Tested zmx, confirmed it works, defined v2.0 scope |
| 2026-03-11 | 01-01 execution | Created zmx package with command builders and parsing |
| 2026-03-11 | 01-02 execution | Created tui package with Bubbletea model, state machine, vim keys |
| 2026-03-11 | 01-03 execution | Created list components with fuzzy filtering, vim keys, help overlay |
| 2026-03-11 | 01-04 execution | Created tea.Cmd functions and wired orchestration in model |

---
*Last updated: 2026-03-11T12:11:08Z*
