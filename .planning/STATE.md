---
gsd_state_version: 1.0
milestone: v2.0
milestone_name: zmx + Bubbletea
current_plan: Not started
status: planning
last_updated: "2026-03-11T00:00:00Z"
progress:
  total_phases: 1
  completed_phases: 0
  total_plans: 0
  completed_plans: 0
---

# Project State: ccc v2.0

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-11)

**Core value:** Transparent terminal passthrough with beautiful TUI
**Current focus:** Planning v2.0 — zmx backend + Bubbletea UI

## Current Position

- **Milestone:** v2.0
- **Phase:** 1 of 1 (zmx-bubbletea)
- **Current Plan:** Not started
- **Status:** Ready to plan Phase 1

## Key Decisions

| Decision | Date | Rationale |
|----------|------|-----------|
| zmx instead of abduco | 2026-03-11 | Tested: fixes notifications, Shift+Enter, scrolling, URLs |
| Bubbletea TUI | 2026-03-11 | User wants nicer UI than basic menus |
| Single milestone for both | 2026-03-11 | Cohesive rewrite, clean break from v1.x |
| TERM passthrough via wrapper | 2026-03-11 | `TERM=$TERM zmx attach` needed for SSH |

## Blockers

None.

## Session Log

| Date | Session | Outcome |
|------|---------|---------|
| 2026-03-11 | Milestone planning | Tested zmx, confirmed it works, defined v2.0 scope |

---
*Last updated: 2026-03-11*
