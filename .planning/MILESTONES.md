# Milestones

## v1.0 abduco migration (Shipped: 2026-03-10)

**Delivered:** Replaced tmux session backend with abduco for native terminal passthrough

**Stats:**
- Phases: 2 | Plans: 4 | Tasks: 13
- Files: 12 changed | LOC: -505 net (removed tmux bloat)
- Timeline: 1 day | Commits: 25
- Git range: b9a323b..6908544

**Key accomplishments:**
- abduco package with Session struct, 5 command builders, regex parser, project filtering
- Flow migration — replaced all tmux imports with abduco, simplified attach/create/kill flows
- tmux removal — deleted 884 lines of workaround code (passthrough, bell config, client negotiation)
- Native terminal passthrough — OSC sequences, scrolling, copy/paste work natively

**Audit:** Passed (13/13 requirements, 2/2 phases verified)

---

