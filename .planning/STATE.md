---
milestone: v1.0
phase: 1
phase_name: abduco-package
status: not_started
plans_complete: 0
plans_total: 0
---

# Project State: ccc abduco migration

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-10)

**Core value:** Transparent terminal passthrough
**Current focus:** Phase 1 - abduco package

## Current Position

- **Milestone:** v1.0
- **Phase:** 1 of 2 (abduco-package)
- **Status:** Not started

## Key Decisions

| Decision | Date | Rationale |
|----------|------|-----------|
| Replace tmux entirely | 2026-03-10 | User only needs persistence; abduco is simpler |
| Remove session rename | 2026-03-10 | abduco doesn't support; rarely used |
| Use PID-based kill | 2026-03-10 | Research found pkill is dangerous |
| Two-phase ultra-coarse | 2026-03-10 | User preference for minimum phases |

## Blockers

None.

## Session Log

| Date | Session | Outcome |
|------|---------|---------|
| 2026-03-10 | Project init | Created PROJECT.md, research, requirements, roadmap |

---
*Last updated: 2026-03-10*
