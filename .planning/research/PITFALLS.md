# Domain Pitfalls: abduco Migration

**Domain:** Terminal session manager (tmux to abduco replacement)
**Researched:** 2026-03-10
**Overall confidence:** MEDIUM-HIGH (abduco is small/stable but niche; some findings from GitHub issues and man pages, verified across multiple sources)

## Critical Pitfalls

These cause broken functionality or require significant rework if not addressed upfront.

### Pitfall 1: Session List Output Format Mismatch

**What goes wrong:** The proposed `ParseSessionList` in `docs/abduco-migration.md` uses a regex expecting `* session-name  (timestamp)` format. The actual abduco output format is completely different:

```
Active sessions (on host debbook)
* Thu 2015-03-12 12:05:20 demo-active
+ Thu 2015-03-12 12:04:50 demo-finished
  Thu 2015-03-12 12:03:30 demo
```

Format is: `[status char] [day] [date] [time] [session-name]` where status is `*` (attached), `+` (terminated), or space (detached). The session name is at the END, not the beginning.

**Why it happens:** The migration doc was drafted without testing against real abduco output.

**Consequences:** Parser returns zero sessions. The entire session management UI breaks silently -- it looks like there are no sessions when sessions exist.

**Prevention:** Write the parser against real abduco output. Test with actual `abduco` invocations on macOS and Linux. The regex should match: optional `*`/`+`/space, then a date-time block, then the session name as the last whitespace-delimited token.

**Detection:** Integration tests that create a real abduco session and parse the listing. Unit tests with real captured output samples.

**Phase:** Must be addressed in the first implementation phase (abduco package creation).

**Confidence:** HIGH -- verified via official README examples and man page.

### Pitfall 2: Session Kill via pkill is Fragile and Dangerous

**What goes wrong:** The proposed `BuildKillCommand` uses `pkill -HUP -f 'abduco.*-n.*{name}'`. This has multiple failure modes:

1. **Over-matching:** If session name is "main", `pkill -f 'abduco.*-n.*main'` could match ANY abduco process whose arguments contain "main" anywhere -- including unrelated sessions like `ccc.maintainer.debug`.
2. **Under-matching:** The pattern assumes `-n` appears in the running process args. After session creation, the abduco server process may not retain `-n` in its command line (it forks).
3. **Race condition:** Between listing and killing, the PID could change or the process could already be gone.
4. **No confirmation of success:** `pkill` returns 0 if it signaled at least one process, but doesn't confirm the target session actually died.

**Why it happens:** abduco has no built-in `kill` command (unlike tmux's `kill-session`). The migration doc acknowledges this as an open question.

**Consequences:** Killing the wrong session destroys someone's work. Failing to kill leaves zombie sessions.

**Prevention:** Use the socket file directly instead of pkill:
- Sessions are stored as Unix domain sockets in `$HOME/.abduco/` (or `$TMPDIR/abduco/$USER/`, or `/tmp/abduco/$USER/`).
- To kill: find the abduco server PID by inspecting the socket, or simply `rm` the socket file and send SIGHUP to the specific PID.
- Better approach: `ls -la $HOME/.abduco/{session_name}` to confirm existence, then use `fuser` or `lsof` to find the exact PID for that socket, then `kill -HUP {pid}`.
- Alternatively: attach to the session and send an exit command (`abduco -a {name}` + write "exit\n"), though this requires interactive handling.

**Detection:** Test killing a session named "main" when another session named "ccc.maintainer.debug" exists. If both die, the pkill approach is broken.

**Phase:** Must be solved in the first implementation phase. This is the most dangerous pitfall.

**Confidence:** HIGH -- pkill regex matching is a well-known class of bugs. The proposed pattern demonstrably over-matches.

### Pitfall 3: Terminated Sessions Show as Active (Stale Sockets)

**What goes wrong:** abduco marks terminated sessions with `+` in the listing. The proposed parser and UI treat all parsed sessions equally. If a user's shell exits (or crashes), the session appears in the list but attaching to it just prints the exit status and returns. The UX is confusing: user selects a session, gets "session terminated with exit status 0", and is dumped back without explanation.

This also manifests as [GitHub issue #5](https://github.com/martanne/abduco/issues/5): sessions show "active" but connection is refused because the socket is stale (server process died without cleanup).

**Why it happens:** abduco preserves terminated session sockets so users can retrieve exit status. Also, if the server process is killed without proper cleanup, the socket file persists as an orphan.

**Consequences:** User frustration. Session list fills with dead sessions. Stale sockets cause "connection refused" errors.

**Prevention:**
1. Parse the `+` status indicator and mark sessions as "terminated" in the Session struct.
2. Either auto-clean terminated sessions (attach briefly to consume exit status, which removes them) or show them distinctly in the UI with a "(finished)" label.
3. For stale sockets: attempt a health check before displaying (try connecting, handle "connection refused" gracefully).
4. Consider adding a "clean dead sessions" action.

**Detection:** Create a session with `abduco -n test true` (command exits immediately). Check that listing shows it with `+`. Verify the UI handles it gracefully.

**Phase:** Must be addressed in the first implementation phase alongside session parsing.

**Confidence:** HIGH -- documented in abduco's own README and man page.

### Pitfall 4: abduco Output Goes to stderr, Not stdout

**What goes wrong:** The proposed `BuildListCommand` is `abduco 2>&1 || true`. This is a workaround for the fact that abduco writes its session listing to stderr, not stdout. The `Runner.Run()` interface returns stdout only. If the `2>&1` redirect is forgotten or handled incorrectly in some code path, session listing returns empty.

**Why it happens:** abduco's session listing behavior writes to stderr because the tool considers it a diagnostic/informational output, not program output.

**Consequences:** Silent failure -- no sessions shown. Hard to debug because the command "succeeds" (exit code 0) but returns empty string.

**Prevention:**
1. Keep the `2>&1` redirect in all abduco listing commands.
2. Add a comment explaining why it's needed.
3. Integration tests should verify that parsing works end-to-end through the Runner interface.

**Detection:** If `runner.Run("abduco")` returns empty string but sessions exist, stderr redirection is missing.

**Phase:** First implementation phase.

**Confidence:** MEDIUM -- the migration doc already has the redirect, suggesting the author discovered this. Needs verification on current abduco version.

## Moderate Pitfalls

### Pitfall 5: SSH + abduco Blank Screen Problem

**What goes wrong:** When abduco is invoked via `ssh host -t abduco -A session cmd`, session creation sometimes results in a blank screen requiring the abduco process to be killed to recover. This is [GitHub issue #15](https://github.com/martanne/abduco/issues/15) and appears to be a PTY allocation race condition.

**Prevention:** ccc's architecture already avoids this: it SSHs in first (establishing the PTY), then runs abduco commands through the established connection. The `RunInteractive` path allocates the PTY at the SSH level, not the abduco level. However, ensure that:
1. `abduco -n` (detached create) is used via `Run()` (non-interactive), NOT `RunInteractive()`.
2. `abduco -a` (attach) is used via `RunInteractive()` (with PTY).
3. Never combine SSH PTY allocation with abduco's own PTY in a single command.

**Detection:** Test session creation over SSH 20+ times in quick succession. If any result in blank screens, the PTY layering is wrong.

**Phase:** Integration testing phase.

**Confidence:** MEDIUM -- ccc's architecture likely avoids this, but needs explicit testing.

### Pitfall 6: TERM Variable Mismatch on Reattach

**What goes wrong:** The shell inside an abduco session inherits the TERM variable from the terminal that created it. If a user creates a session from one terminal type (e.g., `xterm-256color`) and reattaches from another (e.g., `screen-256color` via a different SSH client), the running programs may render incorrectly -- broken colors, garbled ncurses UIs, misaligned cursor positioning.

**Why it happens:** abduco passes through bytes transparently but cannot update the TERM variable inside the already-running shell. tmux normalizes this because it has its own terminal emulation layer (`screen-256color`).

**Consequences:** Visual glitches that confuse users. Programs like vim, htop, or Claude Code may render incorrectly.

**Prevention:**
1. Document this limitation for users.
2. Consider setting `TERM=xterm-256color` explicitly when creating sessions (most modern terminals are xterm-compatible): `TERM=xterm-256color abduco -n {name} bash -l`.
3. This normalizes the TERM for all sessions, similar to how tmux always uses `screen-256color`.

**Detection:** Create a session from iTerm2 (xterm-256color), reattach from a basic Terminal.app or Linux SSH client. Check if colors and rendering are correct.

**Phase:** Session creation phase. Simple one-line fix but easy to forget.

**Confidence:** HIGH -- documented in abduco man page.

### Pitfall 7: Socket Directory Varies by Platform

**What goes wrong:** abduco stores session sockets in the first available of: `$ABDUCO_SOCKET_DIR/abduco`, `$HOME/.abduco`, `$TMPDIR/abduco/$USER`, `/tmp/abduco/$USER`. On macOS, `$TMPDIR` is a per-user randomized path like `/var/folders/xx/xxxxx/T/`. On Linux, it's typically `/tmp`. This means:
1. You cannot hardcode the socket path for kill operations.
2. On macOS with periodic cleanup, `$TMPDIR`-based sockets may be cleaned by the OS.
3. If `$HOME/.abduco` doesn't exist and `$TMPDIR` is used, sessions may not survive a reboot even though the abduco process is still running (the socket is gone but the process lives, creating an orphan).

**Why it happens:** abduco's directory fallback chain interacts differently with each OS's temp directory policies.

**Consequences:** Kill commands that look for sockets in the wrong directory fail silently. Orphan processes after temp directory cleanup.

**Prevention:**
1. For kill: use `abduco` with no args to list sessions (guaranteed to know where its own sockets are), don't try to find socket files manually.
2. Alternatively, ensure `$HOME/.abduco` exists before creating sessions so abduco always uses a stable path.
3. For the kill command: instead of socket-based approaches, consider `pgrep -a abduco` to find exact PIDs and match by session name in the full command line, with strict anchoring.

**Detection:** On macOS, check which directory abduco actually uses. Run `ls $HOME/.abduco` and `ls $TMPDIR/abduco/` to verify.

**Phase:** Session kill implementation.

**Confidence:** HIGH -- socket locations documented in man page and config.def.h.

### Pitfall 8: Detach Key Conflict (Ctrl+\)

**What goes wrong:** abduco's default detach key is `Ctrl+\`, which is also the SIGQUIT signal in most terminals. Programs that catch or use SIGQUIT (some debuggers, certain CLI tools) will conflict. Users accustomed to tmux's `Ctrl+b d` will not know how to detach and may close their terminal instead, leaving orphaned sessions.

**Why it happens:** abduco chose `Ctrl+\` as detach key. Unlike tmux's prefix+key two-step, abduco's single-key detach has a higher chance of conflict.

**Consequences:** Users accidentally quit programs instead of detaching. Or cannot detach at all because the program intercepts the key.

**Prevention:**
1. Use `-e ^q` flag to change detach key to `Ctrl+q` when attaching: `abduco -e ^q -a {name}`.
2. Document the detach key prominently in the UI (show it when attaching).
3. Note: [GitHub issue #28](https://github.com/martanne/abduco/issues/28) reports problems changing the detach key with some key combinations. Test the chosen key thoroughly.

**Detection:** Attach to a session, run a program that uses Ctrl+\, try to detach. If the program catches the key instead of abduco, the conflict exists.

**Phase:** Attach command implementation.

**Confidence:** HIGH -- well-documented default behavior.

## Minor Pitfalls

### Pitfall 9: No Native Session Rename

**What goes wrong:** The migration doc already flags this as out-of-scope, but if users request it later, implementing rename means: creating a new session, somehow migrating the running process (not possible), or renaming the socket file (breaks abduco's internal tracking).

**Prevention:** Accept this limitation. The naming convention `ccc.{project}.{suffix}` makes rename less necessary since suffixes are short labels. Do not attempt to rename socket files.

**Phase:** N/A -- explicitly out of scope.

**Confidence:** HIGH.

### Pitfall 10: Session Name Character Restrictions

**What goes wrong:** Since session names become Unix socket file names, they are subject to filesystem path length limits (typically 108 bytes for Unix socket paths). A project named with a long path or special characters could exceed this limit or create invalid socket names.

**Prevention:** Validate session names: restrict to alphanumeric plus `.` and `-`, enforce a maximum length (e.g., 50 chars). The `ccc.{project}.{suffix}` convention helps but doesn't enforce limits.

**Detection:** Create a session with a very long name (80+ chars). Check if abduco errors or truncates.

**Phase:** Session creation implementation.

**Confidence:** HIGH -- Unix socket path limits are well-documented OS behavior.

### Pitfall 11: `abduco -n` Creates and Immediately Forks

**What goes wrong:** When using `abduco -n name cmd` to create a detached session, abduco forks and the parent returns immediately. If the command (`bash -l`) fails to start (e.g., bad path, permissions), the session appears briefly and then dies. The `Run()` call returns success (abduco itself exited 0) but the session is already gone.

**Prevention:** After `Run(BuildCreateCommand(...))`, immediately verify the session exists by listing sessions. If it doesn't appear, report the error. Add a small delay or retry since the fork may not have completed the session setup instantly.

**Detection:** Create a session with an invalid command (e.g., `abduco -n test /nonexistent`). Check if the caller reports success despite the session not existing.

**Phase:** Session creation implementation.

**Confidence:** MEDIUM -- inferred from fork behavior, needs verification.

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Mitigation |
|-------------|---------------|------------|
| abduco package (parsing) | Output format mismatch (#1), stderr output (#4), terminated sessions (#3) | Test against real abduco output; handle all status indicators |
| Session kill | pkill over-matching (#2), socket directory variance (#7) | Do NOT use pkill -f with loose patterns; find exact PID or use socket-based approach |
| Session creation | Fork race (#11), TERM mismatch (#6), name length (#10) | Verify session exists after creation; set TERM explicitly; validate name length |
| Session attach | Detach key conflict (#8), SSH PTY layering (#5) | Use -e flag; keep PTY allocation at SSH level only |
| Integration testing | Stale sessions (#3), platform differences (#7) | Test on both macOS and Linux; test cleanup of terminated sessions |

## Sources

- [abduco README (GitHub)](https://github.com/martanne/abduco/blob/master/README.md) -- output format, socket locations, SIGUSR1 recovery
- [abduco man page](https://manpages.ubuntu.com/manpages/xenial/man1/abduco.1.html) -- TERM variable warning, session status indicators, detach key
- [abduco ArchWiki](https://wiki.archlinux.org/title/Abduco) -- TERM compatibility, dvtm recommendation
- [GitHub issue #5: Session still active after command terminated](https://github.com/martanne/abduco/issues/5) -- stale socket problem
- [GitHub issue #15: ssh -t abduco blank screen](https://github.com/martanne/abduco/issues/15) -- SSH PTY conflict
- [GitHub issue #28: Changing detach key not working](https://github.com/martanne/abduco/issues/28) -- detach key configuration issues
- [abduco config.def.h](https://github.com/martanne/abduco/blob/master/config.def.h) -- socket directory fallback chain, detach key default
- [abduco Homebrew formula](https://formulae.brew.sh/formula/abduco) -- macOS availability
