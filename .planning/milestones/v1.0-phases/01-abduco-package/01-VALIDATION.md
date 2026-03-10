---
phase: 1
slug: abduco-package
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-10
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing (stdlib) |
| **Config file** | None needed |
| **Quick run command** | `go test ./abduco/` |
| **Full suite command** | `go test ./abduco/ -tags=integration` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./abduco/`
- **After every plan wave:** Run `go test ./abduco/ -tags=integration`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 01-01-01 | 01 | 1 | SESS-01 | integration | `go test ./abduco/ -tags=integration -run TestCreateSession` | ❌ W0 | ⬜ pending |
| 01-01-02 | 01 | 1 | SESS-02 | unit | `go test ./abduco/ -run TestFilterSessionsForProject` | ❌ W0 | ⬜ pending |
| 01-01-03 | 01 | 1 | SESS-03 | integration | `go test ./abduco/ -tags=integration -run TestAttach` | ❌ W0 | ⬜ pending |
| 01-01-04 | 01 | 1 | SESS-04 | integration | `go test ./abduco/ -tags=integration -run TestKillSession` | ❌ W0 | ⬜ pending |
| 01-01-05 | 01 | 1 | SESS-05 | unit | `go test ./abduco/ -run TestParseExternalSessions` | ❌ W0 | ⬜ pending |
| 01-01-06 | 01 | 1 | SESS-06 | unit | `go test ./abduco/ -run TestParseDeadSessions` | ❌ W0 | ⬜ pending |
| 01-01-07 | 01 | 1 | MIGR-01 | unit | `go test ./abduco/` | ❌ W0 | ⬜ pending |
| 01-01-08 | 01 | 1 | ERRH-01 | unit | `go test ./abduco/ -run TestBuildCheckCommand` | ❌ W0 | ⬜ pending |
| 01-01-09 | 01 | 1 | ERRH-02 | unit | `go test ./abduco/ -run TestBuildListCommand` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `abduco/sessions.go` — package file with command builders, parsers, Session struct
- [ ] `abduco/sessions_test.go` — unit tests with mock abduco output
- [ ] `abduco/sessions_integration_test.go` — integration tests (go:build integration)

*Wave 0 creates these files before Wave 1 tests can run.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Platform-specific install hint | ERRH-01 | Different platforms | Verify error message shows correct hint on macOS vs Linux |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 5s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
