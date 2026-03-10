# ccc: Persistent Terminal Sessions

## What This Is

ccc is a CLI tool for managing persistent terminal sessions on local and remote machines over SSH. Sessions use abduco for transparent terminal passthrough — all escape sequences, scrolling, and copy/paste work natively.

## Core Value

Transparent terminal passthrough — all escape sequences, scrolling, and copy/paste work natively without configuration.

## Requirements

### Validated

- ✓ Session persistence across disconnects — v1.0
- ✓ SSH remote mode with host management — existing
- ✓ Local mode (auto-detected in SSH sessions) — existing
- ✓ Project-based session organization — v1.0
- ✓ Multiple sessions per project — v1.0
- ✓ Session listing, creation, attachment — v1.0
- ✓ Session kill using PID extraction — v1.0
- ✓ Project scanning (mdfind/locate/fd/find fallback chain) — existing
- ✓ Replace tmux backend with abduco — v1.0
- ✓ Native terminal passthrough (OSC sequences, clipboard, notifications) — v1.0
- ✓ Native scrolling (use terminal scrollback, not session scrollback) — v1.0
- ✓ Simplified attach flow (no client negotiation) — v1.0
- ✓ Session naming convention: `ccc.{project}.{suffix}` — v1.0
- ✓ External session visibility (non-ccc abduco sessions shown) — v1.0

### Active

(None — define in next milestone)

### Out of Scope

- Session rename — abduco doesn't support natively, not worth complexity
- Client detach menu — abduco auto-detaches previous client
- Passthrough/bell configuration — not needed with abduco
- Verified/unverified session warnings — simplified with naming convention
- tmux compatibility mode — clean break, no dual backend

## Context

Shipped v1.0 with 5,624 LOC Go. Migration removed 505 net lines (eliminated tmux workaround code).

Tech stack: Go, abduco (session persistence), SSH (remote mode).

Known tech debt:
- Integration tests in `flow/flow_integration_test.go` skipped pending testutil update
- `internal/testutil/tmuxtest.go` still references tmux (needs abduco support)

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Replace tmux entirely (no dual backend) | Simpler code, user doesn't need both | ✓ Good — 505 LOC reduction |
| Remove session rename feature | abduco doesn't support, rarely used | ✓ Good — no complaints |
| Encode project in session name | abduco has no metadata tags | ✓ Good — `ccc.{project}.{suffix}` works well |
| Show external abduco sessions | User requested visibility | ✓ Good — marked as "(external)" |
| Use PID-based kill | pkill is dangerous (matches partial names) | ✓ Good — safer, explicit |
| Session naming: main, 2, 3, 4... | First session special, subsequent numbered | ✓ Good — clear convention |
| Preserve leading whitespace in parser | Detached session status is space character | ✓ Good — critical for correctness |

## Constraints

- **Dependency**: abduco must be installed on target machines
- **Breaking change**: Existing tmux sessions from pre-v1.0 are not managed
- **Naming convention**: Sessions must follow `ccc.{project}.{suffix}` pattern

---
*Last updated: 2026-03-10 after v1.0 milestone*
