---
phase: 01-zmx-bubbletea
plan: 04
subsystem: orchestration
tags: [bubbletea, tea.Cmd, async, ssh, zmx, terminal-passthrough]

# Dependency graph
requires:
  - 01-01 (zmx package)
  - 01-02 (tui core model)
provides:
  - tea.Cmd functions for all async operations
  - Message types for TUI state transitions
  - Full orchestration wiring in model.go
affects: [01-05, 01-06, main.go-integration]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - tea.Cmd async command pattern
    - tea.ExecProcess for terminal handoff
    - Runner interface for transport abstraction

key-files:
  created:
    - tui/commands.go
    - tui/messages.go
    - tui/common.go
  modified:
    - tui/model.go

key-decisions:
  - "Local Runner interface in common.go to avoid import cycles with flow package"
  - "Separate *LocalCmd functions for all operations (local mode bypasses SSH)"
  - "tea.ExecProcess for zmx attach with os.Environ() for TERM passthrough"
  - "loadSessionsLocalCmd added for local mode session listing"

patterns-established:
  - "Message types use struct{} with typed fields, errMsg has Error() method"
  - "Commands return tea.Cmd closures that execute async operations"
  - "State handlers check key matches and trigger appropriate commands"

requirements-completed: [FR-3.1, FR-3.2, FR-3.3, FR-3.4, FR-3.5, FR-3.6, FR-3.7, FR-4.1, FR-4.2, FR-4.3, FR-4.4, FR-4.5, FR-4.6]

# Metrics
duration: 6min
completed: 2026-03-11
---

# Phase 01 Plan 04: Commands and Orchestration Summary

**tea.Cmd async functions for hosts/SSH/projects/sessions with terminal passthrough via tea.ExecProcess**

## Performance

- **Duration:** 6 min
- **Started:** 2026-03-11T12:05:06Z
- **Completed:** 2026-03-11T12:11:08Z
- **Tasks:** 3
- **Files modified:** 4

## Accomplishments
- Created message types for all TUI state transitions (hosts, projects, sessions, errors)
- Implemented 18 tea.Cmd functions for async operations (9 remote, 9 local variants)
- Wired Init() to load hosts (remote) or projects (local mode)
- Added full message handling in Update for orchestration flow
- tea.ExecProcess used for zmx attach terminal passthrough

## Task Commits

Each task was committed atomically:

1. **Task 1: Create message types for state transitions** - `f6edf87` (feat)
2. **Task 2: Create tea.Cmd functions for async operations** - `babfade` (feat)
3. **Task 3: Wire commands into model Update and add runner state** - `4a6f647` (feat)

## Files Created/Modified

- `tui/messages.go` (67 lines) - hostsLoadedMsg, hostConnectedMsg, projectsLoadedMsg, sessionsLoadedMsg, sessionExitedMsg, sessionCreatedMsg, sessionKilledMsg, scanCompleteMsg, projectDeletedMsg, errMsg
- `tui/commands.go` (319 lines) - loadHostsCmd, connectHostCmd, loadProjectsCmd/Local, loadSessionsCmd/Local, attachSessionCmd/Local, createSessionCmd/Local, killSessionCmd/Local, scanProjectsCmd/Local, saveProjectsCmd/Local, checkZmxCmd/Local
- `tui/common.go` (9 lines) - Runner interface definition
- `tui/model.go` (452 lines) - Added runner/hosts/projects/sessions state, Init() with mode detection, Update() message handlers, state-specific handlers with command triggers

## Decisions Made

- Runner interface duplicated in tui/common.go to avoid import cycle with flow package
- Separate *LocalCmd variants for all operations to support local mode without SSH
- tea.ExecProcess inherits os.Environ() ensuring TERM passthrough for zmx sessions
- loadSessionsLocalCmd added since model.go uses it for local mode session refresh

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing functionality] Added loadSessionsLocalCmd**
- **Found during:** Task 3
- **Issue:** model.go referenced loadSessionsLocalCmd for local mode session refresh but it wasn't defined
- **Fix:** Added loadSessionsLocalCmd function in commands.go
- **Files modified:** tui/commands.go
- **Commit:** 4a6f647

## Issues Encountered

None

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Orchestration wiring complete
- Ready for Plan 05 (main.go integration) or Plan 06 (error handling improvements)
- TUI can load hosts, connect via SSH, load projects, list/attach/create/kill sessions

---
*Phase: 01-zmx-bubbletea*
*Completed: 2026-03-11*

## Self-Check: PASSED

All claimed files and commits verified:
- FOUND: tui/messages.go
- FOUND: tui/commands.go
- FOUND: tui/common.go
- FOUND: tui/model.go
- FOUND: f6edf87
- FOUND: babfade
- FOUND: 4a6f647
