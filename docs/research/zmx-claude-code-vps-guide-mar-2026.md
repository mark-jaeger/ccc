# zmx: The Complete Guide for Claude Code on a Remote VPS

> **Research compiled March 2026**

**zmx (v0.4.1) is a ~1k-line Zig tool that extracts session persistence from tmux into a standalone Unix socket daemon, using Ghostty's own VT parser as a shadow observer to restore terminal state on reattach — without ever sitting in the escape code data path.** This makes it architecturally ideal for Claude Code over SSH: no `$TERM` override, no escape sequence translation, no keyboard shortcut conflicts, and zero detection as a terminal multiplexer. The tool was created by Eric Bower (neurosnap, also behind pico.sh), first released December 2025, and is already confirmed working with Claude Code by multiple users.

---

## Full CLI Reference and Session Lifecycle

zmx has **nine commands**, each with a short alias (the bracketed letters). There is no `new` subcommand — sessions are created implicitly by `attach` or `run`.

```
zmx [a]ttach <name> [command...]       Create or reattach to a session
zmx [r]un <name> [command...]          Fire-and-forget command to a session (creates if needed)
zmx [d]etach                           Detach ALL clients from current session
zmx [l]ist [--short]                   List active sessions
zmx [c]ompletions <shell>              Shell completion scripts (bash, zsh, fish)
zmx [k]ill <name>                      Kill session and all attached clients
zmx [hi]story <name> [--vt|--html]     Output session scrollback
zmx [w]ait <name>...                   Block until session task(s) complete
zmx [v]ersion                          Print version (-v, --version also work)
```

**`zmx attach`** is the primary entry point. `zmx attach dev` starts a shell session named "dev"; `zmx a dev nvim .` starts nvim in a persistent session. On reattach, libghostty-vt sends a terminal state snapshot to the new client. Multiple clients can attach simultaneously.

**`zmx run`** sends a command without attaching — the killer feature for agent workflows. It accepts stdin: `echo "ls -lah" | zmx r dev`. Combined with **`zmx wait`**, this enables fire-and-forget patterns: `zmx r tests go test ./...` then `zmx wait tests`.

**`zmx history`** dumps session scrollback. Plain text by default; `--vt` preserves escape sequences, `--html` outputs formatted HTML. Pipe to fzf for session preview: `--preview='zmx history {1}'`.

**Detach methods**: Close the terminal window (recommended), press **`ctrl+\`** (detaches current client only), or run `zmx detach` (detaches ALL clients). The `zmx detach` vs `ctrl+\` distinction matters — the former is a broadcast.

### Environment Variables

**Variables zmx reads:**

| Variable | Purpose |
|---|---|
| `ZMX_SESSION_PREFIX` | Auto-prefixes all session names (e.g., `export ZMX_SESSION_PREFIX="proj."` makes `zmx a dev` create `proj.dev`) |
| `ZMX_DIR` | Override socket directory (highest priority) |
| `XDG_RUNTIME_DIR` | Sockets at `{XDG_RUNTIME_DIR}/zmx` (2nd priority) |
| `TMPDIR` | Sockets at `{TMPDIR}/zmx-{uid}` (3rd priority) |

**Variable zmx sets:** `ZMX_SESSION` — contains the current session name inside any zmx session. This is the prompt indicator hook.

### Socket/Daemon Architecture

Each session spawns its own daemon process with its own Unix domain socket. If one session crashes, others are unaffected. This is a fundamental difference from tmux's single-server model.

Socket location fallback: `ZMX_DIR` → `XDG_RUNTIME_DIR/zmx` → `TMPDIR/zmx-{uid}` → `/tmp/zmx-{uid}`.

Logging goes to `{socket_dir}/logs/zmx.log` (global) and `{socket_dir}/logs/{session_name}.log` (per-session), rotating at **5MB**. Logging is always on and cannot be disabled.

### Installation on Ubuntu VPS (Hetzner)

```bash
curl -L https://zmx.sh/a/zmx-0.4.1-linux-x86_64.tar.gz | tar xz
sudo mv zmx /usr/local/bin/
```

Other install methods: `brew install neurosnap/tap/zmx` (Homebrew), Alpine edge/testing package, Arch AUR (`zmx` or `zmx-git`), or build from source with Zig v0.15 (`zig build -Doptimize=ReleaseSafe --prefix ~/.local`).

---

## The Architectural Split: What zmx Does vs What Ghostty Does

This is the single most important concept. zmx's entire value proposition hinges on **not being a terminal emulator**.

**During an active session**, the data path is: `Ghostty ↔ SSH ↔ Unix socket ↔ zmx daemon ↔ PTY`. The daemon simultaneously sends a copy of all PTY output to libghostty-vt, which maintains a shadow terminal state in memory. Critically, **ghostty-vt is a passive observer** — it receives data but does not sit between the client and PTY.

**On reattach**, ghostty-vt serializes the terminal state (cursor position, SGR styles, screen content, alternate buffer, scrollback, pwd, keyboard mode) into a snapshot of escape codes and sends it to the new client's stdout.

**What zmx handles**: Session lifecycle, daemon-per-session process management, `ZMX_SESSION` environment variable, state snapshots on reattach via libghostty-vt, multi-client attach, `zmx run` command injection, `zmx history` scrollback dump, `zmx wait` task completion.

**What Ghostty handles**: Color scheme rendering, font rendering, mouse reporting protocol interpretation, cursor shape/blink display, Kitty keyboard protocol, sixel/graphics rendering, scrollback UI outside session, OSC desktop notifications, clipboard via OSC 52, window/tab/split management.

The author (Eric Bower) explains: *"ghostty-vt doesn't sit in the middle of an active terminal session, it simply receives all the same data the client receives so it can re-hydrate clients that connect to the session."* Mitchell Hashimoto (Ghostty creator) commented: *"This rocks and I honestly think this workflow is the future."*

---

## Color, Mouse, and Cursor Passthrough

Because zmx passes raw bytes through a Unix socket without interpretation, **every terminal capability your outer terminal supports works natively inside zmx**.

**24-bit truecolor** works without configuration. zmx does not change `$TERM` — if Ghostty sets `TERM=xterm-ghostty`, that value persists inside the zmx session. No need for tmux's `terminal-overrides` hacks.

**Mouse reporting sequences** pass through unmodified. All mouse protocols (SGR 1006, urxvt 1015, SGR-Pixels 1016, X10 1000, button-event 1002, any-event 1003) work because zmx never parses mouse escape codes.

**Cursor shape sequences** (DECSCUSR) pass through during active sessions. On reattach, libghostty-vt's state snapshot includes cursor position and keyboard mode. **Note:** tabstop restoration is explicitly disabled in the code because it was found to corrupt cursor position after CUP sequences.

**Kitty keyboard protocol** passes through transparently during active sessions. On reattach, zmx re-sends a CSI query to re-enable Kitty keyboard mode if it was previously active.

---

## Notifications, Bell, and OSC Passthrough

zmx has **no built-in notification system** and no hooks/events API. However, because it doesn't interpret escape codes, **all OSC sequences pass through unchanged**.

- **Terminal bell** (`\a` / BEL): Passes through directly
- **OSC 9/777 desktop notifications**: Pass through to Ghostty
- **OSC 52 clipboard**: Passes through with no racing conditions

For monitoring session completion without attaching, `zmx wait <name>` blocks until the session's command exits.

---

## Claude Code Compatibility

**zmx is confirmed working with Claude Code** by multiple users. On Hacker News (February 2026): *"Been testing it today with Claude Code and it seems to work quite well switching between my laptop and phone."*

### Why tmux Breaks Claude Code

1. Shift+Enter fails (tmux doesn't support Kitty keyboard protocol)
2. Streaming output causes 4,000–6,700 scroll events/second overwhelming tmux
3. OSC 9/777 notifications require DCS wrapping
4. Claude Code explicitly refuses to run `/terminal-setup` inside tmux

### Why Zellij Has Issues

- Default keybindings intercept Ctrl+T, Ctrl+O, Ctrl+G (used by Claude Code)
- Clipboard via OSC 52 causes racing conditions

### Why zmx Works

zmx doesn't intercept escape sequences, doesn't modify `$TERM`, doesn't intercept keyboard shortcuts. Claude Code checks for `$TMUX` and `$ZELLIJ` but not `$ZMX_SESSION`. **Claude Code sees itself as running directly in Ghostty.**

**Caveat:** Claude Code's **Agent Teams** feature (split-pane multi-agent mode) only supports tmux and iTerm2. zmx provides no window management.

---

## zmx run and zmx history for Agent Workflows

These commands make zmx especially powerful for automated workflows.

```bash
zmx run tests go test ./...           # kick off tests in background
zmx run build make -j8                # start a build
echo "cat /var/log/syslog" | zmx r logs  # pipe commands via stdin
zmx wait tests build                  # block until both finish
zmx history tests                     # read output without attaching
```

When `ZMX_SESSION_PREFIX` is set and no names are given to `zmx wait`, it waits for all prefixed sessions.

**Scrollback limitation:** Community testing showed approximately **641 lines** retained from large outputs. For agent workflows with large output, pipe to a file within the session.

---

## Session Naming for Multi-Project Workflows

**`zmx list`** outputs tab-separated fields: `session_name=<name>`, `pid=<pid>`, `clients=<count>`, `started_in=<directory>`. Use `--short` for scripting.

### ZMX_SESSION_PREFIX Pattern

```bash
# Project A context
export ZMX_SESSION_PREFIX="projA."
zmx a claude        # creates "projA.claude"
zmx a tests         # creates "projA.tests"

# Project B context
export ZMX_SESSION_PREFIX="projB."
zmx a claude        # creates "projB.claude"
```

### SSH Config for Named Sessions

```
Host = d.*
    HostName <your-hetzner-ip>
    RemoteCommand zmx attach %k
    RequestTTY yes
    ControlPath ~/.ssh/cm-%r@%h:%p
    ControlMaster auto
    ControlPersist 10m
```

Then `ssh d.claude`, `ssh d.tests` each create or reattach to named sessions. Combined with `autossh -M 0 -q d.claude` for automatic reconnection.

---

## Shell and Prompt Configuration

**zmx spawns whatever `$SHELL` is set to** when no command is specified. No shell-specific behavior.

### Prompt Integration (bash)

```bash
if [[ -n $ZMX_SESSION ]]; then
    export PS1="[$ZMX_SESSION] ${PS1}"
fi
```

### Configuration

There is **no config file**. Configuration is via environment variables only. The **detach key (`ctrl+\`) is not configurable**. The recommended detach method is closing the terminal window.

---

## Known Bugs and Limitations (March 2026)

| Issue | Description | Workaround |
|-------|-------------|------------|
| **Kitty keyboard on reattach** | Programs like psql echo escape sequences after reattach | Exit psql before detaching |
| **IPC-breaking upgrades** | Version upgrades kill existing sessions | Plan upgrades when no critical sessions running |
| **Cursor corruption in nested SSH** | zmx → SSH → zmx causes cursor position issues | Avoid nested zmx sessions |
| **In-memory only state** | Daemon crash = lost history | No workaround; accept the risk |
| **Limited scrollback** | ~641 lines retained | Redirect large output to files |
| **Non-configurable detach key** | `ctrl+\` is hardcoded | Use `zmx detach` or close window |
| **Logging always on** | Cannot disable per-session logs | Accept ~5MB rotation |

---

## Comparison with abduco, dtach, and tmux

### What abduco Lacks That zmx Adds

- No terminal state restoration (deliberate design decision)
- No `run` equivalent (no command injection)
- No `history` command (no scrollback dump)
- No `wait` command
- No session name variable (only `ABDUCO_CMD` and `ABDUCO_SOCKET_DIR`)

### What dtach Provides Differently

- Zero VT assumptions — raw byte streams only
- On reattach: none, send `^L`, or send `SIGWINCH`
- No session listing, no fire-and-forget commands

### What tmux Provides That zmx Excludes

- Window/pane/split management
- Configurable key bindings
- Status bar
- Copy mode
- Plugin ecosystem
- 30-year stability track record

**If you only use tmux for session persistence, zmx is a strict upgrade. If you use tmux's multiplexing features, zmx requires delegating that to your OS window manager or dvtm.**

---

## Conclusion

zmx occupies a precise architectural niche: session persistence without terminal-within-terminal overhead. For Claude Code on a Hetzner VPS via Ghostty over SSH, zmx is the strongest available option:

- `run`/`wait`/`history` pipeline for agent workflows
- Transparent escape code passthrough (truecolor, Kitty keyboard, OSC notifications, mouse)
- `ZMX_SESSION` variable and `ZMX_SESSION_PREFIX` for multi-project management

**Primary risks:** Youth (v0.4.1, ~941 stars), non-configurable detach key, IPC-breaking upgrades, in-memory-only state. For session-persistence-only workflows, these are acceptable trade-offs.
