# Project Retrospective

*A living document updated after each milestone. Lessons feed forward into future planning.*

## Milestone: v1.0 — abduco migration

**Shipped:** 2026-03-10
**Phases:** 2 | **Plans:** 4 | **Sessions:** 5

### What Was Built
- abduco package with Session struct, 5 command builders, regex parser
- Flow layer migration from tmux to abduco
- Removed 884 lines of tmux workaround code

### What Worked
- Ultra-coarse 2-phase approach — kept context tight, minimized overhead
- TDD flow with atomic commits — caught parser bug early (TrimSpace stripping status char)
- Research phase identified PID-based kill as safer than pkill before implementation started

### What Was Inefficient
- Integration test infrastructure not updated — skipped tests, tech debt carried forward
- Phase 01 Nyquist validation started but not completed — draft left behind

### Patterns Established
- `ccc.{project}.{suffix}` session naming — first gets "main", subsequent get 2, 3, 4
- Build*Command functions return shell strings, never execute directly
- External sessions included for visibility, marked with "(external)"

### Key Lessons
1. When replacing a subsystem, delete the old code immediately after wiring the new — reduces confusion and ensures clean break
2. abduco's auto-detach of previous client eliminated entire client negotiation flow — simpler tools enable simpler code

### Cost Observations
- Model mix: 60% sonnet (execution), 40% opus (planning, verification)
- Sessions: 5 total
- Notable: Entire migration completed in single day with zero manual intervention

---

## Post-v1.0 Testing Notes (2026-03-11)

### What We Tried
- Tested abduco on mj-dev (Ubuntu) with SSH from Mac
- Added `CCC_DETACH_KEY` env var for custom detach key (e.g., `^a` for Ctrl+a)
- Tried different shell configurations (bash, zsh, $SHELL)

### Issues Found
1. **Reattach behavior**: Blank screen on reattach, requires typing + Enter to refresh
2. **No persistent scrollback**: Unlike tmux, abduco has no scrollback buffer — history lost on detach
3. **Color issues**: TUI apps (Claude Code) don't render colors correctly
4. **Scroll behavior**: Mouse scroll triggers command history navigation, not terminal scrollback

### Root Cause Analysis
abduco is a minimal PTY passthrough — it doesn't maintain terminal state like tmux does. While this means native terminal features work *while attached*, the tradeoff is:
- No scrollback persistence across detach/reattach
- No automatic screen redraw on reattach
- TUI apps may not handle the attach/detach lifecycle gracefully

### Conclusion
abduco works well for simple shell sessions but may not be ideal for complex TUI applications like Claude Code. The v1.0 migration is technically complete, but real-world testing revealed usability gaps for the primary use case.

### Options for Next Steps
1. **Return to tmux** with better passthrough config
2. **Investigate dtach** (another alternative)
3. **Hybrid approach** — abduco for simple sessions, tmux for Claude Code
4. **Accept limitations** — use abduco with workarounds (Ctrl+L to refresh)

---

## Cross-Milestone Trends

### Process Evolution

| Milestone | Sessions | Phases | Key Change |
|-----------|----------|--------|------------|
| v1.0 | 5 | 2 | First milestone — established baseline |

### Cumulative Quality

| Milestone | Tests | Coverage | Zero-Dep Additions |
|-----------|-------|----------|-------------------|
| v1.0 | 22+ | ~70% | abduco package |

### Top Lessons (Verified Across Milestones)

1. (Pending — need more milestones for cross-validation)
