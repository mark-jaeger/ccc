# Requirements: ccc v2.0 — zmx + Bubbletea

**Milestone:** v2.0
**Created:** 2026-03-11
**Status:** Draft

## Problem Statement

v1.0 (abduco) has unresolved terminal issues. The previous tmux version had problems with notifications, Shift+Enter, scrolling, and URL handling. Testing confirms zmx solves these issues. Additionally, the basic menu UI feels dated compared to modern TUI standards.

## Goals

1. Replace abduco backend with zmx for proper terminal passthrough
2. Replace basic menus with Bubbletea TUI for better UX
3. Maintain orchestration features (host/project/session management)
4. Ensure Claude Code works seamlessly over SSH

## Non-Goals

- Window/pane management (zmx doesn't do this, use OS windows)
- SSH server mode (ccc is a client)
- Backwards compatibility with v1.x sessions (clean break)

---

## Functional Requirements

### FR-1: zmx Backend

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-1.1 | List zmx sessions via `zmx list` | Must |
| FR-1.2 | Create zmx sessions via `zmx attach <name>` | Must |
| FR-1.3 | Attach to existing sessions via `zmx attach <name>` | Must |
| FR-1.4 | Kill sessions via `zmx kill <name>` | Must |
| FR-1.5 | Handle TERM passthrough (`TERM=$TERM zmx attach`) | Must |
| FR-1.6 | Parse `zmx list` output (session_name, pid, clients, started_in) | Must |
| FR-1.7 | Support `ZMX_SESSION_PREFIX` for project namespacing | Should |
| FR-1.8 | Detect `ZMX_SESSION` to know if inside a session | Should |

### FR-2: Bubbletea TUI

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-2.1 | Host selection list with fuzzy filtering | Must |
| FR-2.2 | Project selection list with fuzzy filtering | Must |
| FR-2.3 | Session selection list with create/attach/kill options | Must |
| FR-2.4 | Vim-style navigation (j/k, g/G, /) | Must |
| FR-2.5 | Back navigation (Esc to go up one level) | Must |
| FR-2.6 | Breadcrumb display (host > project > session) | Should |
| FR-2.7 | Loading spinner for SSH operations | Should |
| FR-2.8 | Error display with recovery options | Should |
| FR-2.9 | Help view (? key) | Should |
| FR-2.10 | Alt-screen mode (clean terminal) | Should |

### FR-3: Orchestration

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-3.1 | Load hosts from `~/.ccc/config.toml` | Must |
| FR-3.2 | SSH connection to selected host | Must |
| FR-3.3 | Project scanning on remote (mdfind/locate/fd/find chain) | Must |
| FR-3.4 | Load projects from `~/.ccc/projects.toml` on remote | Must |
| FR-3.5 | Session-per-project organization | Must |
| FR-3.6 | Local mode (when running inside SSH session) | Should |
| FR-3.7 | Hand terminal to zmx via `tea.ExecProcess` | Must |

### FR-4: Terminal Passthrough

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-4.1 | Notifications (OSC 9/777) pass through to local terminal | Must |
| FR-4.2 | Shift+Enter works (Kitty keyboard protocol) | Must |
| FR-4.3 | Scrolling works natively | Must |
| FR-4.4 | cmd-click URLs work | Must |
| FR-4.5 | Clipboard (OSC 52) works | Must |
| FR-4.6 | State restoration on reattach (zmx/libghostty-vt) | Must |

---

## Non-Functional Requirements

### NFR-1: Performance

| ID | Requirement | Target |
|----|-------------|--------|
| NFR-1.1 | TUI startup time | < 100ms |
| NFR-1.2 | SSH connection + session list | < 2s |
| NFR-1.3 | Project scan on remote | < 5s |

### NFR-2: Compatibility

| ID | Requirement |
|----|-------------|
| NFR-2.1 | Works with Ghostty terminal |
| NFR-2.2 | Works over SSH (PTY allocation) |
| NFR-2.3 | zmx v0.4.1+ on remote |
| NFR-2.4 | macOS (darwin) primary, Linux secondary |

### NFR-3: Code Quality

| ID | Requirement |
|----|-------------|
| NFR-3.1 | Maintain Runner interface abstraction |
| NFR-3.2 | Unit tests for TUI state transitions |
| NFR-3.3 | Integration tests for zmx command parsing |

---

## User Acceptance Tests

### UAT-1: Basic Flow
1. Run `ccc`
2. See list of configured hosts
3. Press j/k to navigate, / to filter
4. Press Enter on host → see projects
5. Navigate to project → see sessions
6. Press n to create new session → enters zmx session
7. Press Ctrl+\ to detach → returns to ccc
8. Navigate to same session → reattaches with state restored

### UAT-2: Terminal Features
1. Inside zmx session, run `printf "\e]9;Test\a"` → notification appears on Mac
2. In Claude Code, press Shift+Enter → multi-line input works
3. Run command with long output → scrolling works, no artifacts
4. Output contains URL → cmd-click opens in browser
5. Copy text → paste works outside terminal

### UAT-3: Error Handling
1. Select host that's unreachable → error displayed, can go back
2. zmx not installed on remote → clear error message
3. Session dies while attached → returns to session list

---

## Out of Scope

- tmux/abduco compatibility mode
- Session rename (zmx doesn't support)
- Multi-select operations
- Configuration TUI (edit config.toml manually)
- Themes/customization (hardcoded lipgloss styles)

---

## Dependencies

| Dependency | Version | Purpose |
|------------|---------|---------|
| zmx | 0.4.1+ | Session persistence |
| bubbletea | v2 | TUI framework |
| bubbles | v2 | List, textinput, spinner components |
| lipgloss | v2 | Styling |

---

## Risks

| Risk | Mitigation |
|------|------------|
| zmx v0.4.1 is young (Dec 2025) | We tested it works; monitor for issues |
| Bubbletea v2 recently released | Can fall back to v1.x if needed |
| TERM passthrough complexity | Documented workaround: `TERM=$TERM` wrapper |
| IPC-breaking zmx upgrades | Document version pinning recommendation |

---

## Success Criteria

1. All UATs pass
2. Terminal passthrough issues from tmux era are resolved
3. TUI feels responsive and modern
4. No regression in orchestration features
5. Code remains maintainable (similar or fewer LOC than v1.0)
