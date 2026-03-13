---
phase: 02-flow-migration
plan: 01
subsystem: session-management
tags: [abduco, terminal, session, flow, migration]

# Dependency graph
requires:
  - phase: 01-abduco-package
    provides: abduco package with Session struct, command builders, parser, filters
provides:
  - Abduco-based session flow in flow package
  - Simplified attachSession without client negotiation
  - Auto-naming for createSession
  - PID-based kill for killSession
  - CheckAbduco replacing CheckTmux
affects: [testutil-update, tmux-removal]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Import swap pattern: replace tmux import with abduco"
    - "Session struct migration: Verified -> External (inverted), Windows removed, PID added"
    - "Mock pattern update: abduco output format with day name and tabs"

key-files:
  created: []
  modified:
    - flow/errors.go
    - flow/errors_test.go
    - flow/common.go
    - flow/common_test.go
    - flow/flow_integration_test.go
    - main.go

key-decisions:
  - "Remove detach and rename features (abduco doesn't support)"
  - "Auto-naming only for sessions (no user prompt)"
  - "Skip integration tests until testutil updated for abduco"
  - "External sessions still included for visibility"

patterns-established:
  - "Session naming: ccc.{project}.{suffix} format"
  - "PID-based kill: safer than name-based or pkill"
  - "External check: s.External replaces !s.Verified"
  - "Dead session handling: display (dead) marker, prevent attach"

requirements-completed: [MIGR-02, MIGR-03, MIGR-04]

# Metrics
duration: 6min
completed: 2026-03-10
---

# Phase 2 Plan 01: Flow Migration Summary

**Migrate flow package from tmux to abduco with simplified session management, auto-naming, and PID-based kill**

## Performance

- **Duration:** 5m 55s
- **Started:** 2026-03-10T14:42:55Z
- **Completed:** 2026-03-10T14:48:50Z
- **Tasks:** 5
- **Files modified:** 6

## Accomplishments
- Replaced all tmux imports with abduco in flow package
- Simplified attachSession by removing client negotiation and passthrough config
- Simplified createSession to auto-naming only (no user prompt)
- Updated killSession to use PID-based kill instead of name-based
- Removed detachSessionClients and renameSession functions
- Updated all tests with correct abduco output format

## Task Commits

Each task was committed atomically:

1. **Task 1: Migrate flow/errors.go** - `aecef97` (feat)
2. **Task 2: Migrate flow/common.go** - `3160780` (feat)
3. **Task 3: Update flow/common_test.go** - `a6a9e8b` (test)
4. **Task 4: Update main.go** - `81f0e7e` (feat)
5. **Task 5: Update flow_integration_test.go** - `3822507` (test)

## Files Created/Modified
- `flow/errors.go` - CheckAbduco function replacing CheckTmux
- `flow/errors_test.go` - Tests for CheckAbduco
- `flow/common.go` - Abduco-based SessionFlow, attachSession, createSession, killSession
- `flow/common_test.go` - Updated tests with abduco mock patterns
- `flow/flow_integration_test.go` - Skipped tests pending testutil update
- `main.go` - Removed tmux import and CCC_TMUX_SOCKET handling

## Decisions Made
- Removed detach and rename menu actions (abduco doesn't support client detach or session rename)
- Simplified session creation to auto-naming only (removed name prompt and custom name handling)
- Integration tests skipped with TODO comment until testutil package is updated for abduco
- Dead sessions display "(dead)" marker and cannot be attached

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed abduco output format in tests**
- **Found during:** Task 3 (Update flow/common_test.go)
- **Issue:** Initial test mock data used wrong format. Abduco output format is `STATUS\tDAY DATE TIME\tPID\tNAME` not `STATUS DATE TIME\tPID\tNAME`
- **Fix:** Updated all mock responses to include day name (e.g., "Thu") and proper tab separators
- **Files modified:** flow/common_test.go
- **Verification:** All 21 flow tests pass
- **Committed in:** a6a9e8b (Task 3 commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Bug fix necessary for test correctness. Tests now match actual abduco output format.

## Issues Encountered
- Task 1 (errors.go) couldn't be verified with `go build` until Task 2 (common.go) was also updated, since common.go references CheckTmux. Committed Task 1 after Task 2 build succeeded.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Flow package fully migrated to abduco
- tmux package still exists (not deleted yet) - final cleanup needed
- testutil package needs abduco support for integration tests
- CLAUDE.md needs update to reflect abduco instead of tmux

---
*Phase: 02-flow-migration*
*Completed: 2026-03-10*

## Self-Check: PASSED

- [x] flow/errors.go exists and contains CheckAbduco
- [x] flow/errors_test.go exists and contains CheckAbduco tests
- [x] flow/common.go exists and imports abduco
- [x] flow/common_test.go exists and uses abduco mocks
- [x] flow/flow_integration_test.go exists and imports abduco
- [x] main.go exists and has no tmux references
- [x] All 5 task commits found (aecef97, 3160780, a6a9e8b, 81f0e7e, 3822507)
