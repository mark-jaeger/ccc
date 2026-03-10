# Requirements: ccc abduco migration

**Defined:** 2026-03-10
**Core Value:** Transparent terminal passthrough — all escape sequences, scrolling, and copy/paste work natively

## v1 Requirements

### Session Management

- [ ] **SESS-01**: Create abduco session with `ccc.{project}.{suffix}` naming
- [ ] **SESS-02**: List sessions filtering by project prefix
- [ ] **SESS-03**: Attach to session (auto-detaches previous client)
- [ ] **SESS-04**: Kill session using PID from list output
- [ ] **SESS-05**: Show external (non-ccc) abduco sessions marked as "(external)"
- [ ] **SESS-06**: Handle dead sessions (`+` status) in list output

### Migration

- [ ] **MIGR-01**: Replace tmux package with abduco package
- [ ] **MIGR-02**: Remove client negotiation from attach flow
- [ ] **MIGR-03**: Remove passthrough/bell configuration
- [ ] **MIGR-04**: Update flow layer to use abduco commands
- [ ] **MIGR-05**: Delete tmux package after migration complete

### Error Handling

- [ ] **ERRH-01**: Clear error when abduco not installed
- [ ] **ERRH-02**: Handle stderr output from abduco (2>&1 redirect)

## v2 Requirements

### Enhancements

- **ENH-01**: Read-only attach mode (`-r` flag)
- **ENH-02**: Custom detach key configuration
- **ENH-03**: Auto-clean dead sessions on list

## Out of Scope

| Feature | Reason |
|---------|--------|
| Session rename | abduco doesn't support natively; not worth complexity |
| Dual tmux/abduco backend | Clean break, simpler code |
| tmux compatibility mode | User only needs persistence, not multiplexing |
| Multiple windows per session | abduco is single-window by design |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| SESS-01 | Phase 1 | Pending |
| SESS-02 | Phase 1 | Pending |
| SESS-03 | Phase 1 | Pending |
| SESS-04 | Phase 1 | Pending |
| SESS-05 | Phase 1 | Pending |
| SESS-06 | Phase 1 | Pending |
| MIGR-01 | Phase 1 | Pending |
| MIGR-02 | Phase 2 | Pending |
| MIGR-03 | Phase 2 | Pending |
| MIGR-04 | Phase 2 | Pending |
| MIGR-05 | Phase 2 | Pending |
| ERRH-01 | Phase 1 | Pending |
| ERRH-02 | Phase 1 | Pending |

**Coverage:**
- v1 requirements: 13 total
- Mapped to phases: 13
- Unmapped: 0 ✓

---
*Requirements defined: 2026-03-10*
*Last updated: 2026-03-10 after initial definition*
