# ccc — Claude Code Connector

**Date:** 2026-01-30
**Language:** Go
**Status:** Design

## Problem

Working on a remote Mac (M1 behind a router) requires a tedious ceremony every time: check Tailscale, SSH in, remember tmux syntax, list sessions, attach or create, navigate to the project folder. This friction multiplies across 2-3 active projects, each needing a Claude Code pane and a shell pane.

## Solution

A single Go binary called `ccc` that hides SSH+tmux plumbing behind interactive numbered menus. Two config files (client-side and host-side), no daemons, no host-side installation beyond tmux.

Works from any client — MacBook, iPhone (Blink via local mode), any machine with the binary.

## User Flow

```
$ ccc

  Hosts
  [1] macbook-m1 (mark@100.64.0.1)
  [2] server-lab (mark@100.64.0.2)
  [a] Add host
  [q] Quit

  Select host (or 'r' to remove): 1

  Projects on macbook-m1
  [1] rt1
  [2] death_and_taxes
  [3] pro-rag
  [s] Scan for projects
  [b] Back
  [q] Quit

  Select project: 1

  Sessions for rt1
  [1] rt1 (3 windows)
  [2] rt1-feature-x (1 window)
  [n] New session
  [b] Back
  [q] Quit

  Select session (or 'r' to remove): n
  Session name (enter for "rt1-2"): auth-refactor

  Connecting...
```

You land in an SSH session attached to tmux session `rt1-auth-refactor`, working directory `/Users/mark/Projects/jd/rt1`.

Every menu has `[b]` back and `[q]` quit for escape hatches.

### Shortcuts

Skip interactive menus when you know what you want:

```
ccc                          # Full interactive flow
ccc rt1                      # Skip to sessions (single host)
ccc macbook-m1 rt1           # Explicit host + project
ccc macbook-m1 rt1 new       # Create new session directly
```

If any argument is ambiguous or wrong, falls back to the interactive menu at that step.

### Auto-skip Rules

- Single host configured → skip host selection.
- Project has zero sessions → skip to creating one.
- Project has exactly one session → skip to attaching.

## Modes: Remote and Local

### Remote mode (default)

`ccc` runs on a client machine, SSHes into a remote host.

### Local mode

`ccc` detects it's running over SSH (checks `$SSH_CONNECTION` env var) and suggests local mode:

```
  You're already on this machine via SSH.
  Switching to local mode (no SSH hop).
```

Can also be invoked explicitly:

```
ccc local
```

Local mode skips host selection entirely. Reads `~/.ccc/projects.toml` directly, manages tmux sessions locally. Same project/session flow, no SSH layer.

This is how iPhone/Blink users work: SSH into the host manually, then run `ccc local` (or just `ccc` and it auto-detects).

## Config Files

### Client-side: `~/.ccc/config.toml`

Lives on whatever machine you run `ccc` from.

```toml
[hosts.macbook-m1]
user = "mark"
address = "100.64.0.1"
# Optional overrides (SSH config takes precedence otherwise):
# port = 22
# identity_file = "~/.ssh/id_ed25519"
# proxy_jump = "bastion"
# ssh_options = ["-o", "ServerAliveInterval=60"]

[hosts.server-lab]
user = "mark"
address = "100.64.0.5"
```

Host target can be a raw address or an SSH config alias (e.g., `address = "my-mac"` if you have a `Host my-mac` block in `~/.ssh/config`).

**Precedence:** CLI args > ccc config overrides > SSH config.

### Host-side: `~/.ccc/projects.toml`

Lives on the remote host.

```toml
[projects]

[projects.rt1]
path = "/Users/mark/Projects/jd/rt1"

[projects.death-and-taxes]
path = "/Users/mark/Projects/jd/death_and_taxes"

[projects.pro-rag]
path = "/Users/mark/Projects/jd/pro-rag"
```

**Path handling:** All paths are stored as absolute paths. The scan process resolves `~` to absolute paths before writing. This avoids tmux `~` expansion issues (`tmux new-session -c` does not expand `~`).

**TOML key naming:** Project keys are stable identifiers (lowercase, hyphens). The key is used for tmux session naming and must be a valid bare TOML key. Display names can differ from keys (derived from the directory name if needed).

Deliberately minimal. Adding fields later (aliases, default branch, etc.) is trivial since TOML is forward-compatible.

## First-Run Setup

### No client config

`ccc` detects missing config and starts setup:

```
  No config found. Let's set up your first host.

  Looking for Tailscale... found.
  [1] macbook-m1  100.64.0.1
  [2] server-lab  100.64.0.5
  [3] Enter manually

  Select host: 1
  SSH user for macbook-m1: mark
  ✓ Connected. Host saved.
```

If Tailscale is not installed:

```
  Looking for Tailscale... not found.

  Enter host details manually:
  Name: macbook-m1
  User: mark
  Address: 192.168.1.50
```

The CLI doesn't care how you reach the host — Tailscale IP, local network IP, public IP, hostname, SSH alias. It stores `user@address` and SSHes to it.

### SSH key setup

If SSH authentication fails:

```
  Authentication failed for mark@100.64.0.1.

  [1] Set up SSH keys now
  [2] Try a different user
  [3] Cancel

  Select: 1

  Checking for existing SSH key...
  Found ~/.ssh/id_ed25519.pub

  Copying public key to macbook-m1...
  Password for mark@100.64.0.1: ********
  ✓ Key installed. Testing connection...
  ✓ Connected.
```

If no key exists:

```
  No SSH key found.

  Generating key...
  ✓ Created ~/.ssh/id_ed25519

  Copying public key to macbook-m1...
  Password for mark@100.64.0.1: ********
  ✓ Key installed. Testing connection...
  ✓ Connected.
```

Uses `ssh-keygen` for generation. For key installation, tries `ssh-copy-id` first. If not available (common on macOS), falls back to:

```
cat ~/.ssh/id_ed25519.pub | ssh target 'umask 077; mkdir -p ~/.ssh; cat >> ~/.ssh/authorized_keys'
```

Happens once per client-host pair.

### No host config (no projects)

On first connect to a host with no `~/.ccc/projects.toml`:

```
  Connected to macbook-m1.
  No projects configured on this host.

  Scanning for git repositories in ~...

  Found 4 projects:
  [1] rt1              /Users/mark/Projects/jd/rt1
  [2] death_and_taxes  /Users/mark/Projects/jd/death_and_taxes
  [3] pro-rag          /Users/mark/Projects/jd/pro-rag
  [4] ccc              /Users/mark/Projects/jd/ccc

  Select projects to add (comma-separated, or 'a' for all): a
  ✓ Saved 4 projects to ~/.ccc/projects.toml
```

Default scan path is `~` (home directory). Uses the fastest available tool:

| Priority | Tool | OS | Notes |
|---|---|---|---|
| 1 | `mdfind` | macOS | Queries Spotlight index, instant |
| 2 | `plocate`/`locate` | Linux | Pre-built file database, instant if fresh |
| 3 | `fd` | Any | Parallel Rust-based walker, very fast |
| 4 | `find` | Any Unix | Sequential walk with pruning, universal fallback |

**Scan fallback chain:** If a fast tool (mdfind, locate) returns zero results, automatically falls through to the next tool before declaring "no repos." This handles stale indexes.

**Detection:** A project is any directory containing a `.git` directory OR a `.git` file (git worktrees use a `.git` file pointing to the main repo). Both are detected.

**Path resolution:** All discovered paths are resolved to absolute paths before writing to `projects.toml`.

If no git repos are found after exhausting all scan tools:

```
  No git repositories found under ~.

  [1] Enter a path to scan
  [2] Open a shell on macbook-m1

  Select: 2

  mark@macbook-m1 ~ $ git clone ...
  mark@macbook-m1 ~ $ exit

  Rescanning...
  Found 1 project:
  [1] my-project  /Users/mark/my-project

  Select projects to add (comma-separated, or 'a' for all):
```

The `[s] Scan for projects` option in the project list re-runs this flow anytime.

## How It Works Under the Hood

No daemon, no API, no agent on the host. Everything over SSH (or local commands in local mode).

### SSH behavior

**Non-interactive commands** (reading config, listing sessions) use:
- `BatchMode=yes` — prevents hanging on password/MFA prompts.
- `StrictHostKeyChecking=accept-new` — auto-accepts first-time host keys, but halts and prompts the user explicitly if a known host key changes (potential MITM).
- Commands are wrapped with a login shell (`bash -lc "..."`) to ensure PATH includes common locations like `/opt/homebrew/bin`.

**Interactive commands** (attaching to tmux, opening a shell) use normal SSH with `-t` for PTY allocation.

### Three SSH operations

1. **Read host config** — `ssh host "cat ~/.ccc/projects.toml"` — non-interactive, get project list.
2. **List tmux sessions** — `ssh host "bash -lc 'tmux list-sessions -F ...'"` — non-interactive, discover running sessions.
3. **Attach or create** — interactive, hands over the terminal:
   - Existing: `ssh -t host "tmux attach -t rt1-feature-x"`
   - New: `ssh -t host "tmux new-session -s rt1-auth-refactor -c /Users/mark/Projects/jd/rt1"`

Steps 1 and 2 can be combined into a single SSH call for speed.

### Session identity

Sessions are tagged with tmux user options for reliable project matching:

**On create**, `ccc` sets metadata:
```
tmux new-session -s rt1-auth-refactor -c /abs/path \; \
  set-option -t rt1-auth-refactor @ccc_project rt1 \; \
  set-option -t rt1-auth-refactor @ccc_path /Users/mark/Projects/jd/rt1
```

**On list**, `ccc` reads metadata:
```
tmux list-sessions -F '#{session_name} #{@ccc_project} #{@ccc_path}'
```

**Matching priority:**
1. Sessions with `@ccc_project` matching the project key — reliable, preferred.
2. Sessions with names prefix-matching the project key — fallback for untagged sessions (e.g., manually created tmux sessions). These are labeled `(unverified)` in the list.

**On kill/attach of unverified sessions**, show a warning:
```
  Session "rt1-old" matches by name but wasn't created by ccc.
  Proceed? (y/n):
```

This prevents the collision between `rt1` and `rt1-auth` that prefix-only matching would cause.

### Session naming

- First session for a project: project key (e.g., `rt1`)
- Additional sessions: prompted for a suffix → `rt1-<suffix>`
- Default auto-name: `rt1-2`, `rt1-3`, etc. If that name already exists (race with concurrent client), retry with next number.

### tmux "no server" handling

`tmux list-sessions` returns non-zero when no tmux server is running. This is treated as "zero sessions" — not an error. Distinct from tmux not being installed (detected by `which tmux` / `command -v tmux`).

### Multiple clients on one session

tmux constrains the window to the smallest attached client. If your phone session is still attached, your laptop gets a phone-sized column.

`ccc` detects this via `tmux list-clients -t <session>`:

```
  This session is attached from another client (80x24).
  Your terminal is 220x56.

  [1] Attach anyway (layout constrained to 80x24)
  [2] Detach other client and attach (full resolution)
  [3] Cancel

  Select:
```

Option 2 runs `tmux detach-client` before attaching.

## Host Management

Built into the main `ccc` flow, not separate commands.

### Add host

Via `[a] Add host` in the host list, or automatically on first run.

### Remove host

Press `r` at the host list:

```
  Select host to remove: 1
  Remove macbook-m1? (y/n): y
  ✓ Removed.
```

## Session Management

### Remove session

Press `r` at the session list:

```
  Select session to remove: 2
  Kill tmux session rt1-feature-x? (y/n): y
  ✓ Killed.
```

## Error Handling

### Network/Connection

- **SSH fails** → `Cannot reach macbook-m1. Check your network or Tailscale.`
- **SSH auth fails** → SSH key setup flow (see above).
- **Host key changed** → `WARNING: Host key for macbook-m1 has changed. This could indicate a security issue. Update known host? (y/n)` — explicit user approval required.
- **Host sleeps mid-session** → tmux keeps the session alive. Next `ccc` call, it's still there.

### tmux

- **Not installed** → Show install commands for the detected OS, then drop into a shell:

```
  tmux not found on macbook-m1.

  Install tmux:
    macOS:   brew install tmux
    Ubuntu:  sudo apt install tmux
    Fedora:  sudo dnf install tmux
    Arch:    sudo pacman -S tmux
    Windows: Not supported (use WSL)

  Opening shell on macbook-m1 so you can install it...

  mark@macbook-m1 ~ $ brew install tmux
  ...
  mark@macbook-m1 ~ $ exit

  Rechecking... ✓ tmux found.
```

- **No tmux server running** → treated as zero sessions, not an error. Proceeds to "New session" creation.
- **Session disappeared between listing and attaching** → `Session rt1-feature-x no longer exists.` Re-shows session list.

### Config

- **Client config corrupted** → `Config error in ~/.ccc/config.toml: <parse error>. Fix it or delete to start fresh.`
- **Host config missing** → Triggers scan flow.
- **Project path no longer exists** → `Path /Users/mark/Projects/jd/rt1 not found. Remove from projects? (y/n)`

## Project Structure

```
ccc/
├── main.go              # Entry point, argument parsing, shortcut routing
├── config/
│   ├── client.go        # Read/write ~/.ccc/config.toml
│   └── host.go          # Read/write ~/.ccc/projects.toml (over SSH or local)
├── ssh/
│   ├── connection.go    # SSH connection, command execution, BatchMode/StrictHostKeyChecking
│   └── keys.go          # Key generation, ssh-copy-id with fallback
├── tmux/
│   └── sessions.go      # List, create, attach, kill; metadata tagging (@ccc_project)
├── scan/
│   └── projects.go      # mdfind → locate → fd → find chain; .git dir + file detection
├── ui/
│   └── menu.go          # Numbered list selection, confirmation prompts, back/quit
└── go.mod
```

### Dependencies

Minimal. TOML parser (e.g., `pelletier/go-toml`). Everything else is stdlib + shelling out to `ssh`, `ssh-keygen`.

### UI

Simple numbered lists. Type a number, press enter. Works in every terminal including iPhone apps (Blink). No arrow-key navigation, no TUI frameworks. Every menu supports `[b]` back and `[q]` quit.

## What ccc Is Not

- Not a tmux manager — it doesn't manage panes, layouts, or windows.
- Not a Claude Code launcher — it doesn't start Claude for you.
- Not a project manager — it doesn't know about branches or worktrees.
- Not a daemon — nothing runs in the background.

It's a connector. It gets you from "I want to work on rt1" to a tmux session in the right folder, with zero ceremony.
