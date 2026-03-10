# Technology Stack

**Project:** ccc abduco migration
**Researched:** 2026-03-10

## Recommended Stack

### Session Backend

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| abduco | 0.6 | Session persistence (detach/reattach) | Transparent PTY passthrough eliminates ~125 lines of tmux workaround code. No client negotiation, no passthrough config, no bell config. Does one thing well. |

### Existing Stack (unchanged)

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| Go | 1.21+ | CLI implementation language | Existing codebase |
| SSH | OpenSSH | Remote execution transport | Existing codebase |
| GoReleaser | latest | Build/release pipeline | Existing CI |

### No New Dependencies Required

The migration replaces `tmux/` package with `abduco/` package. No new Go modules needed -- abduco commands are built as shell strings executed through the existing `Runner` interface, identical to how tmux commands work today.

## abduco Specifics (HIGH confidence -- verified via official docs and man pages)

### Command Reference

| Flag | Purpose | Use in ccc |
|------|---------|------------|
| `-c name cmd` | Create session and attach | Not used (ccc creates detached, then attaches separately) |
| `-n name cmd` | Create session, do NOT attach | **Primary create command** -- `abduco -n ccc.proj.main bash -l` |
| `-a name` | Attach to existing session | **Primary attach command** |
| `-A name cmd` | Attach or create if missing | **Do NOT use** -- known blank screen bug over SSH (GitHub issue #15) |
| `-e key` | Set detach key (default: Ctrl+\\) | Consider `-e ^q` to avoid conflict with SIGQUIT |
| `-r` | Readonly attach | Not needed for MVP |
| `-f` | Force create over terminated session | Useful for recovering from crashed sessions |
| `-l` | Low-priority client (last to control size) | Not needed -- abduco auto-detaches previous client |
| (no args) | List sessions | **Primary list command** -- outputs to stderr, hence `abduco 2>&1` |

### Session Listing Output Format

**IMPORTANT:** The migration doc (`docs/abduco-migration.md`) has the wrong output format. The actual abduco output is:

```
Active sessions (on host debbook)
* Thu 2015-03-12 12:05:20	12345	ccc.rt1.main
+ Thu 2015-03-12 12:04:50	12346	ccc.rt1.debug
  Thu 2015-03-12 12:03:30	12347	other-session
```

Key details:
- **Status character:** `*` = attached, `+` = terminated/dead, space = detached
- **Fields are TAB-separated** after the timestamp: PID, then session name
- **PID is included** -- this is critical for the kill command (extract PID, use `kill <pid>`)
- Output goes to **stderr** (not stdout) -- must redirect: `abduco 2>&1 || true`
- Header line: `Active sessions (on host <hostname>)` (NOT "on socket" as some docs say -- varies by version)
- No output at all if zero sessions exist (just usage text)
- The parser in the migration doc is **wrong** and must be rewritten to match this format

### Socket Location (priority order)

1. `$ABDUCO_SOCKET_DIR/abduco` (explicit override)
2. `$HOME/.abduco` (most common on macOS/Linux with home dirs)
3. `$TMPDIR/abduco/$USER` (fallback)
4. `/tmp/abduco/$USER` (last resort)

For ccc's use case, the socket location is opaque -- we never need to reference it directly. abduco manages sockets internally.

### Session Kill (no native command -- use PID from list output)

abduco has NO built-in kill command. The migration doc proposes `pkill -f` which is **fragile and dangerous** (see PITFALLS.md #2).

**Recommended approach:** Extract PID from abduco list output during parsing, store it in the Session struct, then use `kill <pid>` for precise termination.

```go
// In Session struct:
type Session struct {
    Name    string
    PID     int    // server process PID from list output
    // ...
}

// Kill command:
func BuildKillCommand(pid int) string {
    return fmt.Sprintf("kill %d", pid)
}
```

This is precise, does not risk killing wrong processes, and does not depend on socket path knowledge.

### Detach Key

Default: `Ctrl+\` (SIGQUIT on most terminals). This conflicts with:
- Shell SIGQUIT (though most shells ignore it)
- Some editor keybindings
- Some debuggers that use SIGQUIT

**Recommendation:** Keep default `Ctrl+\` for now. It is the standard abduco convention. Users who want to change it can use `-e ^q`. Do NOT hardcode a custom detach key in ccc -- let users configure it via abduco's own mechanism or document it. Note: GitHub issue #28 reports some key combinations not working with `-e`.

### Terminated Sessions (+ status)

abduco marks sessions where the child process exited with `+` in the listing. These sessions persist until a client attaches (to read exit status, which cleans up the socket) or the socket is manually removed.

**Recommendation:** Parse the `+` status, show terminated sessions with a "(dead)" marker in the UI. Optionally auto-clean by brief attachment or offer a "clean dead sessions" action.

### TERM Variable Consideration

abduco passes through bytes transparently but cannot update `$TERM` inside an already-running shell. If a session is created from one terminal type and reattached from another, rendering may break.

**Recommendation:** Set `TERM=xterm-256color` explicitly when creating sessions: `TERM=xterm-256color abduco -n name bash -l`. This normalizes the environment similar to how tmux always uses `screen-256color`.

## Alternatives Considered

| Category | Recommended | Alternative | Why Not |
|----------|-------------|-------------|---------|
| Session persistence | **abduco** | tmux | tmux is a multiplexer; ccc only uses session persistence. ~125 lines of workaround code for features ccc does not need (client negotiation, passthrough, bell config). |
| Session persistence | **abduco** | dtach | dtach (v0.9) is functionally similar but abduco is a cleaner reimplementation with: session listing built-in (with PIDs), no legacy code. dtach has no `list` equivalent -- you must scan socket directories manually. |
| Session persistence | **abduco** | screen | screen is bloated for this use case, has known security issues (SUID bit), terminal passthrough requires configuration. Deprecated in practice. |
| Session persistence | **abduco** | shpool | Newer Rust tool (Google). Not widely packaged, heavier dependency, less battle-tested on macOS. Interesting but premature for a tool that must work across SSH to arbitrary hosts. |

### Comparison Matrix

| Feature | abduco | dtach | tmux | screen |
|---------|--------|-------|------|--------|
| Session list command | Built-in (no args) | None (scan sockets) | `list-sessions` | `screen -ls` |
| Transparent PTY | Yes | Yes | No (intercepts) | No (intercepts) |
| Detach key | Ctrl+\\ | Ctrl+\\ | Ctrl+b d | Ctrl+a d |
| Auto-detach previous client | Yes | No | No (multi-client) | No (multi-client) |
| Kill session | No native cmd | No native cmd | `kill-session` | `screen -X quit` |
| Session metadata/tags | None | None | User options (@) | None |
| PID in list output | Yes (tab-separated) | No | No (use `list-clients`) | No |
| Package availability | brew, apt, pacman, pkg | brew, apt | Preinstalled most places | Preinstalled most places |
| Binary size | ~30KB | ~20KB | ~1MB | ~1MB |
| Active maintenance | Low activity (stable) | Low activity (stable) | Very active | Minimal |

## Installation

abduco must be installed on every target machine (local and remote). It is NOT preinstalled anywhere.

### macOS
```bash
brew install abduco
```

### Ubuntu/Debian
```bash
sudo apt-get install abduco
```

### Arch Linux
```bash
sudo pacman -S abduco
```

### Fedora/RHEL
```bash
sudo dnf install abduco
# Available in EPEL for RHEL/CentOS
```

### FreeBSD
```bash
pkg install abduco
```

### From source (any POSIX system)
```bash
git clone https://github.com/martanne/abduco.git
cd abduco
make
sudo make install
```

### Availability Assessment (MEDIUM confidence)

abduco is packaged in all major package managers. However:
- It is NOT preinstalled on any OS (unlike tmux on some Linux distros)
- The Debian/Ubuntu package exists but version may lag (0.6 is latest upstream)
- Homebrew formula is current
- This is the biggest adoption friction point vs tmux

## Version Requirements

- **Minimum:** abduco 0.6 (latest release, includes all features ccc needs)
- **No version-specific features needed** -- ccc uses only `-n`, `-a`, and bare listing, which exist in all versions
- abduco has had very few releases; 0.6 is effectively the only version in the wild

## What NOT to Use

| Technology | Why Not |
|------------|---------|
| **tmux** (for this project) | Entire migration is about removing tmux. Do not maintain dual backend. Clean break. |
| **screen** | Security concerns (SUID), poor terminal passthrough, effectively deprecated. |
| **dtach** | No session listing. Would require reimplementing socket scanning that abduco provides for free. |
| **shpool** | Not packaged in standard repos. Rust binary. Cannot expect it on remote SSH hosts. |
| **Custom PTY management in Go** | Tempting to avoid external dependency entirely, but reimplementing session persistence (daemon, socket, PTY forwarding) is significant effort with subtle bugs. abduco is ~2000 lines of battle-tested C. |
| **ABDUCO_SOCKET_DIR override** | Do not set this. Let abduco use its default `$HOME/.abduco`. Overriding creates confusion across SSH sessions where env vars may differ. |
| **pkill -f for session kill** | Pattern matching on process args is fragile and dangerous. Use PID extraction from list output instead. |
| **abduco -A flag** | Known blank screen bug over SSH (GitHub issue #15). Use explicit -n/-a two-step instead. |

## Sources

- [abduco GitHub - martanne/abduco](https://github.com/martanne/abduco)
- [abduco man page - Arch Wiki](https://man.archlinux.org/man/abduco.1.en)
- [abduco man page - mankier](https://www.mankier.com/1/abduco)
- [abduco config.def.h](https://github.com/martanne/abduco/blob/master/config.def.h)
- [abduco issue #15 - ssh -t blank screen](https://github.com/martanne/abduco/issues/15)
- [abduco issue #16 - kill session](https://github.com/martanne/abduco/issues/16)
- [abduco issue #28 - detach key issues](https://github.com/martanne/abduco/issues/28)
- [abduco Homebrew formula](https://formulae.brew.sh/formula/abduco)
- [Debian package tracker - abduco](https://tracker.debian.org/pkg/abduco)
- [abduco Repology - package versions](https://repology.org/project/abduco/versions)
- [dtach - crigler/dtach](https://github.com/crigler/dtach)
