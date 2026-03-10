# Roadmap: ccc abduco migration

**Version:** v1.0
**Created:** 2026-03-10
**Phases:** 2

## Progress

| # | Phase | Goal | Requirements | Plans | Status |
|---|-------|------|--------------|-------|--------|
| 1 | abduco-package | Create abduco package with command builders and parser | SESS-01..06, MIGR-01, ERRH-01..02 | 1/2 | ◐ In Progress |
| 2 | flow-migration | Migrate flow layer and remove tmux | MIGR-02..05 | 0/? | ○ Pending |

---

## Phase 1: abduco-package

**Goal:** Create `abduco/` package with command builders, output parser, and session management

**Requirements:** SESS-01, SESS-02, SESS-03, SESS-04, SESS-05, SESS-06, MIGR-01, ERRH-01, ERRH-02

**Plans:** 2 plans

Plans:
- [x] 01-01-PLAN.md — Core package (Session struct, command builders, parser, helpers)
- [ ] 01-02-PLAN.md — Unit tests and integration test scaffolding

**Success Criteria:**
1. `abduco/sessions.go` exists with `BuildCreateCommand`, `BuildAttachCommand`, `BuildListCommand`, `BuildKillCommand`
2. `ParseSessionList` correctly parses tab-delimited abduco output (status, timestamp, PID, name)
3. `FilterSessionsForProject` filters by `ccc.{project}.` prefix and marks external sessions
4. Kill uses PID extraction, not pkill
5. Unit tests pass with mock output
6. Integration tests pass with real abduco binary

---

## Phase 2: flow-migration

**Goal:** Update flow layer to use abduco, remove tmux package, simplify attach flow

**Requirements:** MIGR-02, MIGR-03, MIGR-04, MIGR-05

**Success Criteria:**
1. `flow/common.go` imports `abduco` instead of `tmux`
2. `attachSession` is simplified (no client negotiation, no passthrough config)
3. `flow/setup.go` checks for abduco instead of tmux
4. `tmux/` package deleted
5. All existing tests pass (updated for abduco)
6. Manual test: create session, detach, reattach over SSH

---

*Roadmap created: 2026-03-10*
