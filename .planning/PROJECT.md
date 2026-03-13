# ccc: Persistent Terminal Sessions

## What This Is

ccc is a CLI tool for managing persistent terminal sessions on local and remote machines over SSH. Sessions use zmx for transparent terminal passthrough with state restoration — all escape sequences, scrolling, clipboard, and notifications work natively.

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

- Replace abduco with zmx for session persistence — v2.0
- Bubbletea TUI for host/project/session selection — v2.0
- Terminal passthrough: notifications, Shift+Enter, scrolling, URLs — v2.0
- State restoration on reattach via zmx/libghostty-vt — v2.0
- Proper TERM handling through SSH+zmx chain — v2.0

### Out of Scope

- Session rename — abduco doesn't support natively, not worth complexity
- Client detach menu — abduco auto-detaches previous client
- Passthrough/bell configuration — not needed with abduco
- Verified/unverified session warnings — simplified with naming convention
- tmux compatibility mode — clean break, no dual backend

## Context

Shipped v1.0 with 5,624 LOC Go. Migration removed 505 net lines (eliminated tmux workaround code).

Tech stack: Go, zmx (session persistence, v2.0+), Bubbletea (TUI, v2.0+), SSH (remote mode).

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

- **Dependency**: zmx must be installed on target machines (v2.0+), abduco for v1.x
- **Breaking change**: Existing abduco sessions from v1.x are not managed by v2.0
- **Naming convention**: Sessions can use zmx's native naming or `ccc.{project}.{suffix}` pattern
- **TERM handling**: SSH must pass TERM correctly; may need `TERM=$TERM` wrapper

---
*Last updated: 2026-03-10 after v1.0 milestone*
