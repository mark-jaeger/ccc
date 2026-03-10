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
