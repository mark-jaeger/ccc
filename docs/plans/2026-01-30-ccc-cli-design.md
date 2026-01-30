# ccc — Claude Code Connector

**Date:** 2026-01-30
**Language:** Go
**Status:** Design

## Problem

Working on a remote Mac (M1 behind a router) requires a tedious ceremony every time: check Tailscale, SSH in, remember tmux syntax, list sessions, attach or create, navigate to the project folder. This friction multiplies across 2-3 active projects, each needing a Claude Code pane and a shell pane.

## Solution

A single Go binary called `ccc` that hides SSH+tmux plumbing behind interactive numbered menus. Two config files (client-side and host-side), no daemons, no host-side installation beyond tmux.

Works from any client — MacBook, iPhone (Blink), any machine with the binary.

## User Flow

```
$ ccc

  Hosts
  [1] macbook-m1 (mark@100.64.0.1)
  [2] server-lab (mark@100.64.0.2)
  [a] Add host

  Select host (or 'r' to remove): 1

  Projects on macbook-m1
  [1] rt1
  [2] death_and_taxes
  [3] pro-rag
  [s] Scan for projects

  Select project: 1

  Sessions for rt1
  [1] rt1 (3 windows, created 2h ago)
  [2] rt1-feature-x (1 window, created 2d ago)
  [n] New session

  Select session (or 'r' to remove): n
  Session name (enter for "rt1-3"): auth-refactor

  Connecting...
```

You land in an SSH session attached to tmux session `rt1-auth-refactor`, working directory `~/Projects/jd/rt1`.

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

## Config Files

### Client-side: `~/.ccc/config.toml`

Lives on whatever machine you run `ccc` from.

```toml
[hosts.macbook-m1]
user = "mark"
address = "100.64.0.1"

[hosts.server-lab]
user = "mark"
address = "100.64.0.5"
```

### Host-side: `~/.ccc/projects.toml`

Lives on the remote host.

```toml
[projects.rt1]
path = "~/Projects/jd/rt1"

[projects.death_and_taxes]
path = "~/Projects/jd/death_and_taxes"

[projects.pro-rag]
path = "~/Projects/jd/pro-rag"
```

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

The CLI doesn't care how you reach the host — Tailscale IP, local network IP, public IP, hostname. It stores `user@address` and SSHes to it.

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

Uses `ssh-keygen` and `ssh-copy-id` under the hood. Happens once per client-host pair.

### No host config (no projects)

On first connect to a host with no `~/.ccc/projects.toml`:

```
  Connected to macbook-m1.
  No projects configured on this host.

  Scanning for git repositories in ~...

  Found 4 projects:
  [1] rt1              ~/Projects/jd/rt1
  [2] death_and_taxes  ~/Projects/jd/death_and_taxes
  [3] pro-rag          ~/Projects/jd/pro-rag
  [4] ccc              ~/Projects/jd/ccc

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

Detection is for `.git` directories — a directory containing `.git` is a project.

If no git repos are found:

```
  No git repositories found under ~.

  [1] Enter a path to scan
  [2] Open a shell on macbook-m1

  Select: 2

  mark@macbook-m1 ~ $ git clone ...
  mark@macbook-m1 ~ $ exit

  Rescanning...
  Found 1 project:
  [1] my-project  ~/my-project

  Select projects to add (comma-separated, or 'a' for all):
```

The `[s] Scan for projects` option in the project list re-runs this flow anytime.

## How It Works Under the Hood

No daemon, no API, no agent on the host. Everything over SSH.

### Three SSH operations

1. **Read host config** — `ssh host "cat ~/.ccc/projects.toml"` — non-interactive, get project list.
2. **List tmux sessions** — `ssh host "tmux list-sessions -F '#{session_name}'"` — non-interactive, discover running sessions.
3. **Attach or create** — interactive, hands over the terminal:
   - Existing: `ssh -t host "tmux attach -t rt1-feature-x"`
   - New: `ssh -t host "tmux new-session -s rt1-auth-refactor -c ~/Projects/jd/rt1"`

Steps 1 and 2 can be combined into a single SSH call for speed.

### Session naming

- First session for a project: `rt1`
- Additional sessions: prompted for a suffix → `rt1-<suffix>`
- Matching: any tmux session starting with `rt1` or `rt1-` belongs to the rt1 project

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

- **Session disappeared between listing and attaching** → `Session rt1-feature-x no longer exists.` Re-shows session list.

### Config

- **Client config corrupted** → `Config error in ~/.ccc/config.toml: <parse error>. Fix it or delete to start fresh.`
- **Host config missing** → Triggers scan flow.
- **Project path no longer exists** → `Path ~/Projects/jd/rt1 not found. Remove from projects? (y/n)`

## Project Structure

```
ccc/
├── main.go              # Entry point, argument parsing, shortcut routing
├── config/
│   ├── client.go        # Read/write ~/.ccc/config.toml
│   └── host.go          # Read/write ~/.ccc/projects.toml (over SSH)
├── ssh/
│   ├── connection.go    # SSH connection, command execution
│   └── keys.go          # Key generation, ssh-copy-id flow
├── tmux/
│   └── sessions.go      # List, create, attach, kill sessions
├── scan/
│   └── projects.go      # mdfind → locate → fd → find chain
├── ui/
│   └── menu.go          # Numbered list selection, confirmation prompts
└── go.mod
```

### Dependencies

Minimal. TOML parser (e.g., `pelletier/go-toml`). Everything else is stdlib + shelling out to `ssh`, `ssh-keygen`, `ssh-copy-id`.

### UI

Simple numbered lists. Type a number, press enter. Works in every terminal including iPhone apps (Blink). No arrow-key navigation, no TUI frameworks.

## What ccc Is Not

- Not a tmux manager — it doesn't manage panes, layouts, or windows.
- Not a Claude Code launcher — it doesn't start Claude for you.
- Not a project manager — it doesn't know about branches or worktrees.
- Not a daemon — nothing runs in the background.

It's a connector. It gets you from "I want to work on rt1" to a tmux session in the right folder, with zero ceremony.
