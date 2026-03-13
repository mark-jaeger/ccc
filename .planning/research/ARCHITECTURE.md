# Architecture Patterns: Abduco Integration

**Domain:** Terminal session management CLI (Go + abduco)
**Researched:** 2026-03-10

## Recommended Architecture

### Overview

Replace the `tmux/` package with an `abduco/` package that follows the identical command-builder pattern. The existing `Runner` interface (`Run` / `RunInteractive`) maps perfectly onto abduco's operational model: non-interactive commands for list/create/kill, interactive PTY passthrough for attach.

The migration is a 1:1 package swap. No changes to `Runner`, no changes to the callback pattern, no changes to config packages. The `flow/` package calls `abduco.Build*` instead of `tmux.Build*`, and several flow functions shrink or disappear because abduco eliminates client negotiation, passthrough config, and bell config.

### Component Boundaries

| Component | Responsibility | Communicates With |
|-----------|---------------|-------------------|
| `abduco/` (new) | Command builders, output parsing, session naming | `internal/shellutil` |
| `flow/` (modified) | Orchestration unchanged; calls abduco instead of tmux | `abduco/`, `config/`, `ui/`, `scan/` |
| `Runner` interface | Unchanged -- still `Run()` and `RunInteractive()` | `ssh/`, `flow/` |
| `tmux/` (deleted) | Removed entirely | -- |

### Data Flow

```
SessionFlow
  |-- runner.Run(abduco.BuildListCommand())     --> parse output --> []Session
  |-- runner.Run(abduco.BuildCreateCommand(...)) --> create detached session
  |-- runner.RunInteractive(abduco.BuildAttachCommand(...)) --> PTY attach
  |-- runner.Run(abduco.BuildKillCommand(...))   --> kill via PID signal
```

No changes to ProjectFlow. It still calls SessionFlow with projectKey and projectPath.

---

## How to Invoke abduco from Go

### Command Execution Model

abduco maps cleanly onto the existing Runner interface:

| Operation | Runner Method | abduco Flag | Notes |
|-----------|---------------|-------------|-------|
| List sessions | `Run()` | (no args) | Parse stdout |
| Create (detached) | `Run()` | `-n` | Non-interactive, returns immediately |
| Attach | `RunInteractive()` | `-a` | Full PTY passthrough |
| Kill | `Run()` | N/A (use `kill`) | Send signal to PID from list output |
| Check installed | `Run()` | N/A | `command -v abduco` |

### Command Builders

The `abduco/` package exports `Build*Command()` functions returning shell strings, identical to the tmux pattern. All user-controlled values go through `shellutil.Quote()`.

```go
// abduco/sessions.go
package abduco

// BuildCheckCommand returns a command to verify abduco is installed.
func BuildCheckCommand() string {
    return "command -v abduco"
}

// BuildListCommand returns a command that lists all abduco sessions.
// Output goes to stderr (abduco quirk), so redirect stderr to stdout.
// Format per line: "<status> <timestamp>\t<pid>\t<name>"
func BuildListCommand() string {
    return "abduco 2>&1 || true"
}

// BuildCreateCommand creates a detached session with the given name,
// starting a login shell in the specified directory.
// Uses -n (create without attaching) so it returns immediately.
func BuildCreateCommand(name, path string) string {
    return fmt.Sprintf("cd %s && abduco -n %s bash -l",
        shellutil.Quote(path), shellutil.Quote(name))
}

// BuildAttachCommand attaches to an existing session.
// Uses -a (attach only, fail if not found) -- NOT -A which has
// known issues over SSH (blank screen on create).
func BuildAttachCommand(name string) string {
    return fmt.Sprintf("abduco -a %s", shellutil.Quote(name))
}

// BuildKillCommand terminates a session by sending SIGHUP to its
// server process. Uses the PID extracted from list output, NOT
// pkill pattern matching (which is fragile).
func BuildKillCommand(pid int) string {
    return fmt.Sprintf("kill %d", pid)
}
```

**Critical design decisions:**

1. **Use `-a` not `-A` for attach.** The `-A` flag (create-or-attach) has documented issues over SSH where new session creation results in blank screens. Since ccc always creates sessions explicitly before attaching, `-a` is correct and avoids this bug.

2. **Use `-n` not `-c` for create.** `-c` creates and immediately attaches. The ccc flow creates first (non-interactive), then attaches (interactive) as a separate step. This separation is important because the create step needs to succeed before ccc prints the success message and transitions to interactive mode.

3. **Kill via PID, not pkill.** The abduco list output includes the server process PID. Extracting it during parsing and passing it to `kill <pid>` is precise. The `pkill -f` approach from the migration doc is fragile -- pattern matching on command-line arguments can hit the wrong process.

### Listing and Parsing

abduco's list output (when called with no arguments) goes to **stderr** and has this format:

```
Active sessions (on host <hostname>)
* Thu 2015-03-12 12:05:20	12345	ccc.rt1.main
+ Thu 2015-03-12 12:04:50	12346	ccc.rt1.debug
  Thu 2015-03-12 12:03:30	12347	other-session
```

Fields after the status+timestamp are **tab-separated**. The columns are:

| Column | Content | Notes |
|--------|---------|-------|
| 1 | Status char | `*` = client attached, `+` = terminated, space = detached |
| 2 | Timestamp | Human-readable, day-of-week + date + time |
| 3 | PID | Server process ID (tab-separated from timestamp) |
| 4 | Session name | Tab-separated from PID |

**Parser implementation:**

```go
type Session struct {
    Name     string
    Project  string // extracted from name, empty if external
    Suffix   string // extracted from name
    PID      int    // server process PID, used for kill
    Attached bool   // true if status is '*'
    Dead     bool   // true if status is '+'
    External bool   // true if not a ccc.* session
}

func ParseSessionList(output string) []Session {
    var sessions []Session
    for _, line := range strings.Split(output, "\n") {
        line = strings.TrimSpace(line)
        if line == "" || strings.HasPrefix(line, "Active sessions") {
            continue
        }

        // Status is first non-space character
        status := ' '
        trimmed := line
        if len(line) > 0 && (line[0] == '*' || line[0] == '+') {
            status = rune(line[0])
            trimmed = strings.TrimSpace(line[1:])
        }

        // Split by tabs to get: timestamp, pid, name
        // The timestamp portion may contain spaces but PID and name are tab-delimited
        parts := strings.Split(trimmed, "\t")
        if len(parts) < 3 {
            continue
        }

        pid, _ := strconv.Atoi(strings.TrimSpace(parts[len(parts)-2]))
        name := strings.TrimSpace(parts[len(parts)-1])

        s := parseSessionName(name)
        s.PID = pid
        s.Attached = status == '*'
        s.Dead = status == '+'
        sessions = append(sessions, s)
    }
    return sessions
}
```

**Key parsing concerns:**

- The header line ("Active sessions...") must be skipped.
- Tab-split from the right to avoid issues with timestamp format variations.
- Dead sessions (status `+`) should be shown but marked -- user can kill to clean up, or attach to see exit status.

---

## Session Naming Strategy

### Convention: `ccc.{project}.{suffix}`

```
ccc.rt1.main      -- project "rt1", first session (auto-named "main")
ccc.rt1.2         -- project "rt1", second session (auto-numbered)
ccc.rt1.debug     -- project "rt1", user-named suffix
ccc.myapp.main    -- project "myapp", first session
```

### Why This Convention

1. **Filtering without metadata.** tmux supports user options (`@ccc_project`). abduco does not. Encoding project identity in the name is the only option.

2. **Dot separator is safe.** abduco session names become Unix socket filenames. Dots are valid in socket names on all target platforms (macOS, Linux).

3. **`ccc.` prefix distinguishes managed sessions.** External abduco sessions (not created by ccc) are visible but clearly separated.

4. **Two-level hierarchy in a flat namespace.** `ccc.{project}.{suffix}` encodes both project membership and session identity without any metadata system.

### Name Extraction

```go
func parseSessionName(name string) Session {
    if !strings.HasPrefix(name, "ccc.") {
        return Session{Name: name, External: true}
    }
    rest := name[4:] // skip "ccc."
    dot := strings.IndexByte(rest, '.')
    if dot < 0 {
        return Session{Name: name, External: true} // malformed
    }
    return Session{
        Name:    name,
        Project: rest[:dot],
        Suffix:  rest[dot+1:],
    }
}
```

### Auto-Naming

```go
// NextAutoSuffix returns "main" for first session, then "2", "3", etc.
func NextAutoSuffix(sessions []Session) string {
    if len(sessions) == 0 {
        return "main"
    }
    hasMain := false
    maxNum := 1
    for _, s := range sessions {
        if s.Suffix == "main" { hasMain = true }
        if n, err := strconv.Atoi(s.Suffix); err == nil && n > maxNum {
            maxNum = n
        }
    }
    if !hasMain { return "main" }
    return strconv.Itoa(maxNum + 1)
}
```

---

## Error Handling Patterns

### Installation Check

Same pattern as current `CheckTmux` -- run `command -v abduco` via Runner, return descriptive error if missing.

```go
func CheckAbduco(in io.Reader, out io.Writer, runner Runner) error {
    if _, err := runner.Run(abduco.BuildCheckCommand()); err != nil {
        fmt.Fprintf(out, "\n  abduco is not installed.\n")
        fmt.Fprintf(out, "  Install: brew install abduco (macOS) or apt install abduco (Debian/Ubuntu)\n")
        return fmt.Errorf("abduco not found")
    }
    return nil
}
```

### Create Failures

The `cd <path> && abduco -n <name> bash -l` command can fail in two ways:

| Failure | Cause | Detection | User Message |
|---------|-------|-----------|--------------|
| Directory missing | Project path deleted | Non-zero exit from `cd` | "Path not found" (existing pattern handles this) |
| Name collision | Socket already exists | Non-zero exit from `abduco -n` | "Session already exists" |
| Permission denied | Socket dir not writable | Non-zero exit | "Cannot create session (check permissions)" |

Wrap errors with context: `fmt.Errorf("failed to create session %q: %w", name, err)`.

### Attach Failures

| Failure | Cause | Detection | Handling |
|---------|-------|-----------|---------|
| Session not found | Killed between list and attach | Non-zero exit from `abduco -a` | Return to session list (loop) |
| Session dead (`+` status) | Process inside session exited | Attach shows exit status then returns | Filter dead sessions in UI or mark them |

### Kill Failures

| Failure | Cause | Detection | Handling |
|---------|-------|-----------|---------|
| PID gone | Already exited | Non-zero exit from `kill` | Treat as success (session is gone) |
| Wrong PID | PID reused (rare) | Unlikely but possible | Accept the risk -- PID reuse within seconds is extremely unlikely for a long-running session manager |

### Dead Sessions

abduco marks sessions where the child process exited with `+`. These sessions persist until a client attaches (to read exit status) or the socket is removed.

**Recommended handling:** Show dead sessions in the list with a "(dead)" marker. Attaching to a dead session shows the exit status and cleans up automatically. Alternatively, offer kill which removes the socket.

---

## Patterns to Follow

### Pattern 1: Command Builder Isolation

**What:** All abduco CLI invocations are built in `abduco/sessions.go` as shell strings. The `flow/` package never constructs abduco commands directly.

**Why:** Same benefit as current tmux pattern -- centralized quoting, easy to audit, easy to test parsers in isolation.

### Pattern 2: Create Then Attach (Two-Step)

**What:** Session creation uses `Run()` with `-n` (detached). Attachment uses `RunInteractive()` with `-a`. Never combine into `-c` or `-A`.

**Why:**
- `-c` (create+attach) would require `RunInteractive()` for creation, losing the ability to detect creation errors before entering PTY mode.
- `-A` (create-or-attach) has known bugs over SSH (blank screen). Also conflates two operations, making error handling ambiguous.
- Two-step matches the existing ccc flow: create, print success, then attach.

### Pattern 3: PID-Based Kill

**What:** Parse PID from list output, store in Session struct, use `kill <pid>` for termination.

**Why:** abduco has no native kill command. The alternatives are:
- `pkill -f` pattern matching (fragile, can hit wrong process)
- Socket file deletion (leaves orphaned process)
- PID from list output (precise, reliable)

### Pattern 4: Stderr Redirect for List

**What:** `abduco 2>&1 || true` captures the session list.

**Why:** abduco writes its list output to stderr, not stdout. The `|| true` ensures zero exit code even when no sessions exist (abduco returns non-zero in that case).

---

## Anti-Patterns to Avoid

### Anti-Pattern 1: Using -A Flag Over SSH

**What:** Using `abduco -A` (create-or-attach) for session operations.
**Why bad:** Known bug causes blank screen when creating new sessions over SSH. The `-A` flag is convenient for interactive shell aliases but unreliable when invoked programmatically via `ssh -t`.
**Instead:** Explicit `-n` for create, `-a` for attach. The ccc flow always knows whether a session exists (it just listed them).

### Anti-Pattern 2: pkill Pattern Matching for Kill

**What:** `pkill -HUP -f 'abduco.*-n.*<name>'` to kill sessions.
**Why bad:** Pattern matches against command-line arguments of all processes. Can match the listing command itself, match similarly-named sessions, or miss if abduco was invoked differently.
**Instead:** Extract PID from `abduco` list output, use `kill <pid>`.

### Anti-Pattern 3: Socket File Deletion as Kill

**What:** Removing the socket file from `~/.abduco/` to "kill" a session.
**Why bad:** The abduco server process continues running as an orphan. The session is invisible but still consuming resources. If SIGUSR1 is sent to the orphan, it recreates the socket.
**Instead:** Send SIGTERM to the server PID, which cleanly terminates the session and removes the socket.

### Anti-Pattern 4: Dual Backend (tmux + abduco)

**What:** Supporting both tmux and abduco through an interface abstraction.
**Why bad:** Doubles test surface, doubles edge cases, doubles documentation. The user only uses tmux for persistence (not multiplexing), so abduco is a strict replacement.
**Instead:** Clean break. Remove tmux package entirely.

---

## Session Lifecycle

```
                      +-----------+
                      |  Listed   |  abduco (no args) shows session
                      +-----------+
                           |
              +------------+------------+
              |                         |
         [not found]              [found in list]
              |                         |
    +---------v---------+    +----------v----------+
    | Create            |    | Check status        |
    | cd <path> &&      |    | * = attached        |
    | abduco -n <name>  |    | + = dead            |
    | bash -l           |    |   = detached        |
    +-------------------+    +---------------------+
              |                    |          |
              |              [detached]   [dead]
              |                    |          |
              +--------+---------+      Show "(dead)"
                       |                marker in menu
                       v
              +-------------------+
              | Attach            |
              | abduco -a <name>  |  (RunInteractive)
              +-------------------+
                       |
                  [user detaches]
                  Ctrl+\ (default)
                       |
                       v
              +-------------------+
              | Detached          |
              | Session persists  |
              | Process continues |
              +-------------------+

    Kill path:
              +-------------------+
              | Kill              |
              | kill <pid>        |  (PID from list)
              +-------------------+
                       |
                       v
              +-------------------+
              | Terminated        |
              | Socket removed    |
              +-------------------+
```

---

## Socket Path Considerations

abduco looks for sessions in this order:
1. `$ABDUCO_SOCKET_DIR/abduco/`
2. `$HOME/.abduco/`
3. `$TMPDIR/abduco/$USER/`
4. `/tmp/abduco/$USER/`

**For ccc:** Do not override this. Let abduco use its default discovery. This means:
- Remote sessions live in the remote user's home directory (natural for SSH)
- Local sessions live in the local user's home directory
- No ccc-specific socket directory needed

The session name (not path) is what ccc controls. The `ccc.project.suffix` convention is sufficient for namespace isolation.

---

## Impact on Existing Code

### Files Modified

| File | Change | Complexity |
|------|--------|-----------|
| `flow/common.go` | Replace tmux calls with abduco calls, remove client negotiation, remove rename | Medium -- net deletion |
| `flow/setup.go` | `CheckTmux` becomes `CheckAbduco` | Trivial |
| `flow/local.go` | Update import from `tmux` to `abduco` | Trivial |
| `flow/remote.go` | Update import from `tmux` to `abduco` | Trivial |

### Files Created

| File | Content |
|------|---------|
| `abduco/sessions.go` | Command builders, parser, naming functions |
| `abduco/sessions_test.go` | Unit tests for parser, naming, filtering |

### Files Deleted

| File | Reason |
|------|--------|
| `tmux/sessions.go` | Replaced by `abduco/sessions.go` |
| `tmux/sessions_test.go` | Replaced by `abduco/sessions_test.go` |

### Runner Interface

**No changes.** The `Runner` interface (`Run` + `RunInteractive`) is transport-agnostic. abduco commands are shell strings executed through the same interface. This is the key architectural win -- the abstraction boundary was drawn correctly.

### Package Dependency Graph (After)

```
main -> flow -> config, ssh, abduco, scan, ui, tailscale
                 ssh -> config, internal/shellutil
                scan -> internal/shellutil
               abduco -> internal/shellutil
```

Identical shape to current graph with `tmux` replaced by `abduco`.

---

## Sources

- [abduco GitHub repository](https://github.com/martanne/abduco) - source code, README
- [abduco man page (mankier)](https://www.mankier.com/1/abduco) - CLI flags, socket paths
- [abduco man page (Ubuntu)](https://manpages.ubuntu.com/manpages/xenial/en/man1/abduco.1.html) - options reference
- [abduco issue #16: killing sessions from CLI](https://github.com/martanne/abduco/issues/16) - no native kill, use PID
- [abduco issue #15: ssh -t abduco](https://github.com/martanne/abduco/issues/15) - known issues with -A over SSH
- [abduco source: abduco.c](https://github.com/martanne/abduco/blob/master/abduco.c) - printf format for list output includes PID
- [abduco official site](https://www.brain-dump.org/projects/abduco/) - socket path resolution order
