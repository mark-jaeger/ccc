# Feature Landscape

**Domain:** Terminal session persistence manager (CLI tool managing abduco sessions over SSH and locally)
**Researched:** 2026-03-10

## Table Stakes

Features users expect from a session persistence manager. Missing = product feels broken.

| Feature | Why Expected | Complexity | abduco Support | Notes |
|---------|-------------|------------|----------------|-------|
| Session persistence across disconnects | Core reason the tool exists | Low | Native | abduco's primary purpose |
| Create named sessions | Users need to identify sessions | Low | Native (`-c name`) | Session name is the socket name |
| Attach to existing session | Core workflow | Low | Native (`-a name`) | Auto-detaches previous client |
| Detach from session | Must be able to leave without killing | Low | Native (`Ctrl+\` default) | Configurable via `-e` flag |
| List sessions | Users need to see what's running | Low | Native (no-arg invocation) | Shows attached/detached status and exit status |
| Kill/terminate session | Clean up unwanted sessions | Med | NOT native | abduco has no kill command; must use `pkill` or socket deletion |
| Session survives SSH disconnect | Remote use case requires this | Low | Native | PTY stays alive server-side |
| Terminal transparency (escape sequences) | Modern terminal features (OSC52 clipboard, notifications) must work | Low | Native | abduco is a transparent PTY proxy -- no interception |
| Native scrollback | Users expect terminal scrollback to work | Low | Native | No virtual terminal emulation blocking scroll |
| Multiple sessions per project | Parallel workstreams | Low | Native | Each session is independent |
| Working directory per session | Sessions should start in the right place | Low | Wrapper (`cd path && abduco -c`) | abduco itself has no working directory concept |

## Differentiators

Features that set ccc apart from raw abduco usage. Not expected from abduco alone, but valued in a session manager.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Project-based session organization | Group sessions by project, not flat list | Low | Implemented via `ccc.{project}.{suffix}` naming convention |
| Project scanning (mdfind/locate/fd/find) | Auto-discover projects instead of manual config | Med | Existing feature, orthogonal to backend |
| SSH remote mode with host management | Manage sessions on remote machines without manual SSH | Med | Existing feature, abduco runs on remote host via Runner |
| External session visibility | Show non-ccc abduco sessions | Low | Parse all abduco output, mark sessions without `ccc.` prefix |
| Auto-naming (main, 2, 3...) | Reduce friction creating sessions | Low | Convention-based, no abduco support needed |
| Read-only session attachment | Observe a session without interfering | Low | abduco native (`-r` flag) -- not currently exposed in ccc but trivially addable |
| Create-or-attach (`-A` flag) | Idempotent session access | Low | abduco native -- simplifies create-if-not-exists flow |
| Custom detach key | Avoid conflicts with user keybindings | Low | abduco native (`-e` flag) -- could be a ccc config option |
| Low-priority client attach (`-l` flag) | Attach without taking resize control | Low | abduco native -- useful for monitoring |

## Anti-Features

Features to explicitly NOT build. These add complexity without matching the session persistence model.

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| Window/pane splitting | abduco is not a multiplexer; this is tmux/dvtm territory. Adding splits defeats the purpose of migrating away from tmux. | Users who want splits should run dvtm inside an abduco session, or use their terminal emulator's native tabs/splits. |
| Session rename | abduco identifies sessions by socket filename. Renaming requires creating a new session and migrating the process, which is fragile and error-prone. | Kill old session, create new one with desired name. Sessions are cheap. |
| Client negotiation / multi-client size management | abduco handles this automatically (most recent non-readonly client controls size). No user intervention needed. | Let abduco's built-in behavior handle it. |
| Passthrough configuration | abduco is transparent by design. There is nothing to configure. | Remove the ~30 lines of tmux passthrough/bell workarounds. |
| Bell/notification configuration | abduco passes BEL and OSC sequences natively. No interception occurs. | Remove bell-action and visual-bell configuration entirely. |
| Session metadata tags | abduco has no metadata API (no equivalent to tmux's `@ccc_project`). | Encode all metadata in the session naming convention `ccc.{project}.{suffix}`. |
| Send-keys / programmatic input | abduco has no API for sending keystrokes to a session without attaching. Users have requested this upstream (GitHub issue #41) but it is not supported. | Not a ccc use case anyway. |
| tmux compatibility / dual backend | Maintaining two backends doubles complexity for no user benefit. The user only uses tmux for persistence. | Clean break. Document migration path (kill old tmux sessions manually). |
| Scrollback buffer management | abduco has no scrollback buffer -- it relies on the terminal emulator's native scrollback. There is nothing to manage. | This is actually a feature, not a limitation. |

## Feature Dependencies

```
Project scanning --> Project config (projects.toml)
Project config --> Session listing (need project key to filter)
Session listing --> Session attach/create/kill
SSH remote mode --> Runner interface (all session ops go through Runner)
External session visibility --> Session listing (parse all sessions, not just ccc-prefixed)
```

## What abduco Provides That tmux Required Workarounds For

This is the key value proposition of the migration:

| Capability | tmux (current) | abduco (target) |
|------------|---------------|-----------------|
| Terminal transparency | Requires `allow-passthrough on` (tmux >= 3.3), still imperfect | Native, zero config |
| Native scrollback | Blocked by tmux's virtual terminal; requires mouse mode config | Native, just works |
| OSC52 clipboard | Requires passthrough config, version-dependent | Native passthrough |
| Bell forwarding | Requires `bell-action any` + `visual-bell off` | Native passthrough |
| Multi-client sizing | Complex 3-option menu (attach constrained, detach other, cancel) | Auto-detaches previous client OR newest client controls size |
| Session ownership | Custom `@ccc_project` / `@ccc_path` metadata tags | Naming convention only |
| Session kill | Native `kill-session` | Must use `pkill -HUP` or socket file deletion |

## What abduco Lacks vs tmux (Accepted Tradeoffs)

| Missing Feature | Impact | Mitigation |
|----------------|--------|------------|
| No native kill command | Medium | Use `pkill -HUP -f 'abduco.*session-name'` or delete socket file. Kill reliability is an open question (see PITFALLS.md). |
| No session rename | Low | Rarely used. Kill + recreate is acceptable. |
| No metadata API | Low | Naming convention `ccc.{project}.{suffix}` replaces metadata. |
| No window count | Low | Each abduco session is one shell. Window count concept does not apply. |
| No programmatic send-keys | None | Not a ccc use case. |
| No client listing | Low | abduco session list shows `*` for attached sessions. No per-client detail, but ccc does not need it since abduco auto-manages clients. |

## MVP Recommendation

### Must ship (table stakes):

1. **Session create** -- `abduco -n ccc.{project}.{suffix} bash -l` (detached) or `abduco -c` (attached)
2. **Session list** -- Parse `abduco` output, filter by `ccc.{project}.` prefix
3. **Session attach** -- `abduco -a ccc.{project}.{suffix}`
4. **Session kill** -- `pkill` based approach (validate reliability during implementation)
5. **External session visibility** -- Show non-ccc sessions marked as "(external)"
6. **Auto-naming** -- `main`, then `2`, `3`, etc.
7. **Dependency check** -- `command -v abduco` with clear error + install instructions

### Defer to later:

- **Read-only attach (`-r`)**: Nice to have, not blocking. Trivial to add later.
- **Custom detach key (`-e`)**: Config option, not blocking. Default `Ctrl+\` is reasonable.
- **Create-or-attach (`-A`)**: Could simplify flow but current create/attach separation works fine.
- **Low-priority attach (`-l`)**: Niche use case for monitoring.

### Never build:

- Window splitting, session rename, dual tmux backend, passthrough config, bell config.

## Sources

- [abduco GitHub repository](https://github.com/martanne/abduco)
- [abduco man page (ManKier)](https://www.mankier.com/1/abduco)
- [abduco man page (Ubuntu)](https://manpages.ubuntu.com/manpages/xenial/man1/abduco.1.html)
- [abduco author's project page](https://www.brain-dump.org/projects/abduco/)
- [abduco send-keys issue #41](https://github.com/martanne/abduco/issues/41)
- [shpool session manager](https://alternativeto.net/software/shpool/about/)
- [dtach GitHub](https://github.com/crigler/dtach)
- [abduco vs tmux comparison (SaaSHub)](https://www.saashub.com/compare-tmux-vs-abduco-plus-dvtm)
