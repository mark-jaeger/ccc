# ccc

A CLI tool for managing tmux sessions on remote machines over SSH. Navigate to a project, attach to a session, and start working — all through simple numbered menus that work even from a phone.

## Install

```bash
go install github.com/mark-jaeger/ccc@latest
```

Or build from source:

```bash
go build -o ccc .
```

## Usage

```bash
ccc                          # Interactive: host → project → session
ccc rt1                      # Jump to project (single-host shortcut)
ccc macbook-m1 rt1           # Explicit host + project
ccc macbook-m1 rt1 new       # Create a new session directly
ccc local                    # Local mode (no SSH hop)
```

When run without arguments, ccc presents interactive menus:

```
  Hosts
  [1] macbook-m1 (mark@100.64.0.1)
  [2] server-lab (mark@100.64.0.5)
  [a] Add host
  [q] Quit

  Select (or 'r' to remove): 1

  Projects
  [1] rt1          /Users/mark/Projects/jd/rt1
  [2] pro-rag      /Users/mark/Projects/jd/pro-rag
  [s] Scan for projects
  [b] Back
  [q] Quit

  Select: 1

  Session name (enter for "rt1"):
  ✓ Created session rt1
  Attaching to rt1...
```

### Auto-skip rules

- Single host configured → skips host menu
- Zero sessions for a project → creates one
- One session → attaches directly

### Local mode

If ccc detects it's running inside an SSH session (`$SSH_CONNECTION` set), it automatically switches to local mode — executing tmux commands directly instead of over SSH. You can also force it with `ccc local`.

## Configuration

### Client-side: `~/.ccc/config.toml`

Stores your remote host definitions (on the machine where you run ccc):

```toml
[hosts.macbook-m1]
user = "mark"
address = "100.64.0.1"

[hosts.server-lab]
user = "mark"
address = "100.64.0.5"
port = 2222
identity_file = "/home/mark/.ssh/id_ed25519"
proxy_jump = "bastion"
ssh_options = ["-o", "ServerAliveInterval=60"]
```

### Host-side: `~/.ccc/projects.toml`

Stores tracked projects (on the remote host, managed by ccc's scan flow):

```toml
[projects.rt1]
path = "/Users/mark/Projects/jd/rt1"

[projects.pro-rag]
path = "/Users/mark/Projects/jd/pro-rag"
```

## First-run setup

On first run with no config, ccc walks you through:

1. **Host discovery** — auto-detects Tailscale peers if available, or prompts for manual entry
2. **SSH key setup** — tests connectivity and offers to generate/copy keys if auth fails
3. **Project scan** — discovers git repositories on the host using mdfind, locate, fd, or find

## Requirements

- Go 1.25+ (build only)
- SSH client on the local machine
- tmux on the remote host (ccc will prompt you to install it if missing)
- No installation needed on the remote host
