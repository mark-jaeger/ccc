---
phase: 2
slug: flow-migration
status: ready
nyquist_compliant: true
wave_0_complete: true
created: 2026-03-10
---

# Phase 2 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing (stdlib) |
| **Config file** | none |
| **Quick run command** | `go test ./flow/` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~2 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./flow/`
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 2-01-01 | 01 | 1 | MIGR-04 | unit | `go build ./flow/ && grep -q "CheckAbduco" flow/errors.go` | Update existing | pending |
| 2-01-02 | 01 | 1 | MIGR-02, MIGR-03, MIGR-04 | unit | `go build ./flow/ && ! grep -q "tmux" flow/common.go` | Update existing | pending |
| 2-01-03 | 01 | 1 | MIGR-02, MIGR-03, MIGR-04 | unit | `go test ./flow/ -v` | Update existing | pending |
| 2-02-01 | 02 | 2 | MIGR-05 | manual | `! grep -r "mark-jaeger/ccc/tmux" --include="*.go" .` | N/A | pending |
| 2-02-02 | 02 | 2 | MIGR-05 | manual | `test ! -d tmux/ && go test ./...` | N/A | pending |
| 2-02-03 | 02 | 2 | MIGR-05 | manual | `grep -q "abduco" CLAUDE.md` | N/A | pending |

*Status: pending | green | red | flaky*

---

## Wave 0 Requirements

*Existing infrastructure covers all phase requirements — tests will be updated in place.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| tmux package deleted | MIGR-05 | Filesystem check, not unit test | `test ! -d tmux/ && echo "PASS"` |
| SSH reattach works | Success Criteria #6 | E2E network test | Create session, detach, reattach over SSH |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 5s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** ready
