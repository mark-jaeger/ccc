# ccc: Abduco Migration

## What This Is

ccc is a CLI tool for managing persistent terminal sessions on local and remote machines over SSH. This milestone replaces tmux with abduco as the session backend to enable native terminal features (copy, scroll, OSC notifications).

## Core Value

Transparent terminal passthrough — all escape sequences, scrolling, and copy/paste work natively without configuration.

## Requirements

### Validated

- Session persistence across disconnects — existing
- SSH remote mode with host management — existing
- Local mode (auto-detected in SSH sessions) — existing
- Project-based session organization — existing
- Multiple sessions per project — existing
- Session listing, creation, attachment — existing
- Session kill — existing
- Project scanning (mdfind/locate/fd/find fallback chain) — existing

### Active

- [ ] Replace tmux backend with abduco
- [ ] Native terminal passthrough (OSC sequences, clipboard, notifications)
- [ ] Native scrolling (use terminal scrollback, not session scrollback)
- [ ] Simplified attach flow (no client negotiation)
- [ ] Session naming convention: `ccc.{project}.{suffix}`
- [ ] External session visibility (non-ccc abduco sessions shown)

### Out of Scope

- Session rename — abduco doesn't support natively, not worth complexity
- Client detach menu — abduco auto-detaches previous client
- Passthrough/bell configuration — not needed with abduco
- Verified/unverified session warnings — simplified with naming convention
- tmux compatibility mode — clean break, no dual backend

## Context

The current tmux implementation has ~125 lines of workaround code for:
- Client negotiation (tmux allows multiple clients at different sizes)
- Passthrough configuration (tmux blocks OSC sequences by default)
- Bell/visual-bell configuration (tmux intercepts bells)
- Metadata tags (@ccc_project, @ccc_path) for session ownership

abduco is a transparent PTY wrapper that eliminates all of these issues. User only uses tmux for persistence, not multiplexing, making abduco a better fit.

## Constraints

- **Dependency**: abduco must be installed on target machines
- **Breaking change**: Existing tmux sessions won't be managed after migration
- **Naming convention**: Sessions must follow `ccc.{project}.{suffix}` pattern for filtering

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Replace tmux entirely (no dual backend) | Simpler code, user doesn't need both | — Pending |
| Remove session rename feature | abduco doesn't support, rarely used | — Pending |
| Encode project in session name | abduco has no metadata tags | — Pending |
| Show external abduco sessions | User requested visibility | — Pending |

---
*Last updated: 2026-03-10 after initialization*
