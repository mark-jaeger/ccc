---
phase: 01-abduco-package
plan: 01
subsystem: session-management
tags: [abduco, terminal, session, parser, regex]

# Dependency graph
requires: []
provides:
  - abduco package with Session struct and command builders
  - ParseSessionList parser for abduco output
  - FilterSessionsForProject for project-based filtering
  - NextAutoName for session naming convention
affects: [02-flow-integration]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Command builders return shell strings (Build*Command pattern)
    - Output parsers return typed structs (Parse* pattern)
    - Filter functions for project-based filtering (Filter*ForProject pattern)

key-files:
  created:
    - abduco/sessions.go
    - abduco/sessions_test.go
  modified: []

key-decisions:
  - "Session naming: ccc.{project}.{suffix} format"
  - "First session gets 'main' suffix, subsequent get 2, 3, 4..."
  - "PID-based kill using parsed PID from list output (not pkill)"
  - "Preserve leading whitespace when parsing (status char may be space)"

patterns-established:
  - "Build*Command: functions that return shell command strings, never execute directly"
  - "shellutil.Quote: all user values must be quoted for shell safety"
  - "Parse*: functions that parse command output into typed structs"
  - "TDD flow: failing test -> implementation -> verification"

requirements-completed: [SESS-01, SESS-02, SESS-03, SESS-04, SESS-05, SESS-06, MIGR-01, ERRH-01, ERRH-02]

# Metrics
duration: 8min
completed: 2026-03-10
---

# Phase 1 Plan 01: abduco Package Summary

**Complete abduco package with Session struct, 5 command builders, regex parser, and project filtering/naming helpers**

## Performance

- **Duration:** 7m 32s
- **Started:** 2026-03-10T13:55:17Z
- **Completed:** 2026-03-10T14:02:49Z
- **Tasks:** 3
- **Files created:** 2

## Accomplishments
- Session struct with Name, Project, Suffix, External, Dead, and PID fields
- Command builders: BuildCreateCommand, BuildAttachCommand, BuildListCommand, BuildKillCommand, BuildCheckCommand
- ParseSessionList with regex parsing for attached/dead/detached sessions
- FilterSessionsForProject includes project sessions plus external sessions
- NextAutoName generates "main" first, then 2, 3, 4 for subsequent sessions

## Task Commits

Each task was committed atomically (TDD: test -> feat):

1. **Task 1: Session struct and command builders**
   - `b162e5b` (test: add failing tests for command builders)
   - `3ab17b5` (feat: implement command builders)
2. **Task 2: ParseSessionList parser**
   - `3494944` (test: add failing tests for ParseSessionList)
   - `03a8ffc` (feat: implement ParseSessionList with regex parser)
3. **Task 3: FilterSessionsForProject and NextAutoName**
   - `4abbee0` (test: add failing tests for filter and naming)
   - `12b91de` (feat: implement FilterSessionsForProject and NextAutoName)

## Files Created/Modified
- `abduco/sessions.go` - Package with Session struct, command builders, parser, and helpers (151 lines)
- `abduco/sessions_test.go` - 15 unit tests covering all exported functions (249 lines)

## Decisions Made
- Used `\s+` after status character to handle tab delimiter (not fixed column positions)
- Preserved leading whitespace when parsing lines (TrimRight only) - status char may be space for detached sessions
- External sessions included in FilterSessionsForProject for visibility
- NextAutoName checks if "main" exists before returning it

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed TrimSpace stripping status character**
- **Found during:** Task 2 (ParseSessionList implementation)
- **Issue:** `strings.TrimSpace(output)` was stripping leading space from detached sessions, which meant status character (space) was lost
- **Fix:** Changed to only check for empty output with TrimSpace, then TrimRight on each line to preserve leading status character
- **Files modified:** abduco/sessions.go
- **Verification:** TestParseSessionList_DetachedExternalSession now passes
- **Committed in:** 03a8ffc (Task 2 feat commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Bug fix essential for correct parsing of detached sessions. No scope creep.

## Issues Encountered
- Discovered during debugging that TrimSpace was removing the status character for detached sessions - traced through hex dump of test input and debug output

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- abduco package complete with all required exports
- Ready for Phase 2 integration with flow package
- Package follows established tmux package patterns

---
*Phase: 01-abduco-package*
*Completed: 2026-03-10*

## Self-Check: PASSED

- [x] abduco/sessions.go exists
- [x] abduco/sessions_test.go exists
- [x] All 6 task commits found (b162e5b, 3ab17b5, 3494944, 03a8ffc, 4abbee0, 12b91de)
