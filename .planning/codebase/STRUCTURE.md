# Codebase Structure

**Analysis Date:** 2026-02-25

## Directory Layout

```
/Users/mark/Projects/jd/ccc/
├── config/                     # TOML config structs and I/O
│   ├── client.go              # ClientConfig, Host types; load/save to ~/.ccc/config.toml
│   ├── projects.go            # ProjectsConfig type; parse/serialize TOML
│   ├── client_test.go
│   └── projects_test.go
├── flow/                       # Orchestration: workflows for remote/local/scan/setup
│   ├── common.go              # ProjectFlow, SessionFlow (core workflow loops)
│   ├── remote.go              # RunRemoteMode, host selection, SSH connection
│   ├── local.go               # RunLocalMode, LocalRunner implementation
│   ├── scan.go                # RunScanFlow, project discovery
│   ├── setup.go               # SetupFirstHost, AddHostFlow, SSH key setup
│   ├── errors.go              # CheckTmux (tmux install verification)
│   ├── common_test.go
│   ├── remote_test.go
│   ├── scan_test.go
│   └── setup_test.go
├── ssh/                        # SSH connection and key management
│   ├── connection.go          # Connection struct, Run(), RunInteractive()
│   ├── keys.go                # FindExistingPublicKey, GenerateKey, CopyKeyToHost
│   ├── connection_test.go
│   └── keys_test.go
├── tmux/                       # tmux session and command building
│   ├── sessions.go            # Session parsing, command builders (list, create, attach, kill, rename)
│   ├── sessions_test.go
│   └── sessions_integration_test.go
├── scan/                       # Project discovery via shell commands
│   ├── projects.go            # BuildScanChainCommand (mdfind→plocate→locate→fd→find)
│   ├── projects_test.go
│   └── projects_integration_test.go
├── ui/                         # Terminal UI: menus, prompts, confirmations
│   ├── menu.go                # ShowMenu, MenuConfig, MenuResult
│   ├── interactive.go         # showInteractiveMenu (arrow-key navigation)
│   ├── keys.go                # Key constants (up, down, enter, q, etc.)
│   ├── menu_test.go
│   └── interactive_test.go
├── internal/
│   ├── shellutil/
│   │   ├── quote.go           # Quote() for safe shell quoting
│   │   └── quote_test.go
│   └── testutil/
│       ├── tmuxtest.go        # Test helpers for tmux
│       └── expect.go          # Test assertions
├── tailscale/                  # Tailscale host discovery (optional)
│   ├── discovery.go           # IsAvailable(), DiscoverHosts()
│   └── discovery_test.go
├── main.go                     # Entry point: mode detection, dispatch
├── e2e_test.go                 # End-to-end tests
├── go.mod                      # Module definition, dependencies
├── go.sum                       # Dependency checksums
├── CLAUDE.md                   # Project instructions for Claude Code
├── .goreleaser.yaml            # GoReleaser config for CI/CD
└── .github/
    └── workflows/
        └── release.yml         # GitHub Actions: test + release on tag
```

## Directory Purposes

**config/:**
- Purpose: Define and persist configuration state (hosts, projects)
- Contains: TOML serialization, Host/Project/Config types
- Key files: `client.go` (hosts), `projects.go` (tracked projects)

**flow/:**
- Purpose: Orchestrate multi-step workflows (selection loops, state transitions)
- Contains: All workflow functions (ProjectFlow, SessionFlow, RunLocalMode, RunRemoteMode, etc.)
- Key files: `common.go` (core loops), `remote.go` (SSH mode), `local.go` (local mode), `setup.go` (first-time setup)

**ssh/:**
- Purpose: SSH connection management and key setup
- Contains: Connection struct, command execution (Run, RunInteractive), SSH key discovery/generation
- Key files: `connection.go` (SSH Runner implementation), `keys.go` (key management)

**tmux/:**
- Purpose: Build and parse tmux shell commands
- Contains: BuildListCommand, BuildCreateCommand, BuildAttachCommand, etc.; session/client parsing
- Key files: `sessions.go` (all tmux operations)

**scan/:**
- Purpose: Discover projects by finding .git directories using shell commands
- Contains: Multi-tool fallback chain (mdfind → plocate → locate → fd → find)
- Key files: `projects.go` (scan command builders and parsing)

**ui/:**
- Purpose: Terminal interface (menus, prompts, confirmations)
- Contains: ShowMenu with TTY/non-TTY adaptation, interactive cursor navigation, line-based fallback
- Key files: `menu.go` (menu logic), `interactive.go` (arrow-key navigation)

**internal/shellutil/:**
- Purpose: Safe shell quoting utility
- Contains: Quote() function protecting all user input in shell commands
- Key files: `quote.go`

**internal/testutil/:**
- Purpose: Shared test helpers
- Contains: tmux test utilities, assertion helpers
- Key files: `tmuxtest.go`, `expect.go`

**tailscale/:**
- Purpose: Optional Tailscale integration for host discovery
- Contains: IsAvailable() check, DiscoverHosts() to list Tailscale network peers
- Key files: `discovery.go`

## Key File Locations

**Entry Points:**
- `main.go`: CLI entry point (version flag, mode detection, dispatch)

**Configuration:**
- `config/client.go`: Host definitions (~/.ccc/config.toml)
- `config/projects.go`: Project tracking (remote ~/.ccc/projects.toml)

**Core Logic:**
- `flow/common.go`: ProjectFlow (project selection loop) and SessionFlow (session selection/creation loop)
- `flow/remote.go`: Remote mode entry, host selection, SSH connection, project config loading
- `flow/local.go`: Local mode entry, local file I/O, LocalRunner implementation
- `flow/scan.go`: Project discovery workflow
- `flow/setup.go`: First-time setup, host addition, SSH key setup

**SSH:**
- `ssh/connection.go`: Connection struct, Run (non-interactive), RunInteractive (PTY passthrough)
- `ssh/keys.go`: SSH key discovery/generation, ssh-copy-id wrapper

**Tmux:**
- `tmux/sessions.go`: Session listing, parsing, command builders (create, list, attach, kill, rename, detach clients)

**Scanning:**
- `scan/projects.go`: Multi-tool scan command builder and .git result parsing

**UI:**
- `ui/menu.go`: ShowMenu dispatcher (TTY vs non-TTY)
- `ui/interactive.go`: Interactive menu with arrow-key navigation

## Naming Conventions

**Files:**
- `*_test.go`: Unit tests (run with go test)
- `*_integration_test.go`: Integration tests (may require external tools like tmux)
- `.go` files in same package: lowercase, underscores (e.g., `common.go`, `remote.go`)

**Directories:**
- `internal/`: Go convention for unexported packages
- `config/`, `flow/`, `ssh/`, `ui/`, etc.: Lowercase, no hyphens, semantic names

**Functions:**
- Exported: PascalCase (e.g., `RunRemoteMode`, `ProjectFlow`, `BuildListCommand`)
- Unexported: camelCase (e.g., `attachSession`, `createSession`, `buildNonInteractiveArgs`)

**Variables:**
- Constants: PascalCase or UPPER_SNAKE_CASE (e.g., `ActionSelect`, `SocketOverride`)
- Interfaces: Capitalized, ending in "er" (e.g., `Runner`)
- Structs: PascalCase (e.g., `Connection`, `Session`, `MenuResult`)

**Types:**
- Config types: PascalCase ending in "Config" (e.g., `ClientConfig`, `ProjectsConfig`)
- Domain types: PascalCase (e.g., `Host`, `Project`, `Session`, `MenuItem`)

## Where to Add New Code

**New Feature (Session Management):**
- Primary code: `flow/common.go` (SessionFlow logic), `tmux/sessions.go` (tmux commands)
- Tests: `flow/common_test.go`, `tmux/sessions_test.go`
- Config support: `config/projects.go` or `config/client.go` (if persisted)

**New Command/Action (e.g., [e] Edit Project):**
- Handler: Add to ProjectFlow or SessionFlow menu ExtraActions, implement helper function
- Location: `flow/common.go`
- Tests: `flow/common_test.go`

**New Utility (e.g., shell command escaping):**
- Location: `internal/` (if shared), or within relevant package
- Example: New Git command builders could go in `scan/` or a new `git/` package

**SSH/Networking Enhancement:**
- Primary code: `ssh/connection.go`, `ssh/keys.go`
- Example: ProxyJump, IdentityFile support already in place
- Tests: `ssh/connection_test.go`, `ssh/keys_test.go`

**Discovery Integration (e.g., new discovery backend):**
- Pattern: Mirror `tailscale/discovery.go`
- Location: New package e.g., `consul/discovery.go` or extend `tailscale/discovery.go`
- Export: IsAvailable(), DiscoverHosts() for use in `flow/setup.go`

**UI Enhancement (e.g., new menu type):**
- Location: `ui/` package
- Pattern: Add new MenuConfig variants or MenuResult actions
- Don't break: ui.ShowMenu signature (core abstraction)

## Special Directories

**dist/:**
- Purpose: GoReleaser build output (binaries for macOS, Linux)
- Generated: Yes (via CI/CD)
- Committed: No (added to .gitignore)

**docs/:**
- Purpose: Documentation and research notes
- Generated: No
- Committed: Yes

**.github/workflows/:**
- Purpose: GitHub Actions CI/CD
- Generated: No
- Committed: Yes

**.planning/:**
- Purpose: GSD planning documents
- Generated: Yes (by /gsd:map-codebase)
- Committed: Yes

**internal/:**
- Purpose: Go convention for package-local code
- Generated: No
- Committed: Yes

## Import Patterns

**Standard patterns by layer:**

```
// flow/ imports
import (
  "io"                          // io.Reader/io.Writer (testable I/O)
  "github.com/mark-jaeger/ccc/config"  // config structures
  "github.com/mark-jaeger/ccc/ssh"     // Runner (remote)
  "github.com/mark-jaeger/ccc/tmux"    // command builders
  "github.com/mark-jaeger/ccc/ui"      // menu/prompt functions
)

// ssh/ imports
import (
  "github.com/mark-jaeger/ccc/config"  // Host type
  "github.com/mark-jaeger/ccc/internal/shellutil"  // Quote()
)

// tmux/ imports
import (
  "github.com/mark-jaeger/ccc/internal/shellutil"  // Quote()
)

// scan/ imports
import (
  "github.com/mark-jaeger/ccc/internal/shellutil"  // Quote()
)
```

**No cross-imports (avoids cycles):**
- config → ssh ✓ (config defines Host type)
- ssh → config ✓ (ssh reads config.Host)
- flow → ssh, config, tmux, ui ✓ (orchestration layer uses all)
- tmux ← flow ✓ (tmux is utility)
- ui ← flow ✓ (ui is utility)

---

*Structure analysis: 2026-02-25*
