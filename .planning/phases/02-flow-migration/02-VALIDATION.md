---
phase: 2
slug: flow-migration
status: draft
nyquist_compliant: false
wave_0_complete: false
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
| 2-01-01 | 01 | 1 | MIGR-04 | unit | `go test ./flow/ -run TestSessionFlow` | Update existing | ⬜ pending |
| 2-01-02 | 01 | 1 | MIGR-02 | unit | `go test ./flow/ -run TestAttachSession` | Update existing | ⬜ pending |
| 2-01-03 | 01 | 1 | MIGR-03 | unit | `go test ./flow/ -run TestCreateSession` | Update existing | ⬜ pending |
| 2-02-01 | 02 | 2 | MIGR-05 | manual | `test ! -d tmux/` | N/A | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

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

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 5s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
