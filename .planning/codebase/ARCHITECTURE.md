# Architecture

**Analysis Date:** 2026-02-25

## Pattern Overview

**Overall:** Layered CLI application with abstraction-based command execution

**Key Characteristics:**
- Runner interface abstracts SSH and local command execution, allowing identical workflow logic for both remote and local modes
- Configuration split between client-side hosts and remote projects
- Event-driven callback pattern for scan/save operations supporting multiple persistence backends
- Shell command builders (not direct execution) ensure consistent quoting and security
- Two command execution modes: non-interactive (returns stdout) and interactive (PTY passthrough)

## Layers

**Presentation (UI):**
- Purpose: Interactive terminal menus and prompts with TTY/non-TTY adaptation
- Location: `ui/menu.go`, `ui/interactive.go`, `ui/keys.go`
- Contains: Menu rendering, input handling, action dispatch
- Depends on: io.Reader/io.Writer (injectable)
- Used by: flow package orchestration functions

**Orchestration (Flow):**
- Purpose: Control flow for workflows: host selection, project browsing, session management, first-time setup
- Location: `flow/` (common.go, remote.go, local.go, scan.go, setup.go)
- Contains: ProjectFlow, SessionFlow, RunRemoteMode, RunLocalMode, config callbacks
- Depends on: Runner interface (SSH or local), config, scan, tmux, ui
- Used by: main.go

**Transport (Runner Implementations):**
- Purpose: Execute commands on remote or local systems
- Location: `ssh/connection.go` (remote), `flow/local.go` (LocalRunner)
- Contains: Connection struct, command building, error handling
- Depends on: os/exec (local), SSH client command-line (remote)
- Used by: flow package

**Command Builders:**
- Purpose: Generate shell commands with safe quoting for tmux, git scanning, and remote operations
- Location: `tmux/sessions.go`, `scan/projects.go`, `internal/shellutil/quote.go`
- Contains: BuildListCommand, BuildCreateCommand, BuildScanChainCommand functions
- Depends on: internal/shellutil (quoting), tmux metadata parsing
- Used by: flow package

**Configuration:**
- Purpose: TOML-based persistence for hosts and projects
- Location: `config/client.go`, `config/projects.go`
- Contains: Host, ClientConfig, ProjectsConfig structs; TOML marshaling
- Depends on: github.com/pelletier/go-toml/v2
- Used by: flow (remote.go, local.go, setup.go)

**Utilities:**
- Purpose: Shared shell quoting, SSH key management, Tailscale discovery
- Location: `internal/shellutil/`, `ssh/keys.go`, `tailscale/discovery.go`
- Contains: Quote(), key generation, file discovery
- Depends on: os/exec, external SSH/Tailscale CLIs
- Used by: command builders, setup flow

## Data Flow

**Remote Mode (default):**

1. main.go → RunRemoteMode (args parsing)
2. Load/create client config (~/.ccc/config.toml) → hostSelectionLoop
3. User selects host → connectToHost (SSH connection test via fallback chain)
4. Read remote projects.toml via SSH Run() → ProjectFlow (callback for save = remoteSaveFn)
5. User selects project → SessionFlow (tmux list via SSH)
6. User selects session or creates new → attachSession/createSession → RunInteractive (tmux attach)

**Local Mode (auto-detected or explicit):**

1. main.go → RunLocalMode (no config)
2. Read ~/.ccc/projects.toml directly → ProjectFlow (callback for save = local file write)
3. User selects project → SessionFlow (tmux list via local runner)
4. User selects session or creates new → attachSession/createSession → RunInteractive (tmux attach)

**Scan Flow (triggered by [s] Scan action):**

1. RunScanFlow → BuildScanChainCommand (mdfind → plocate → locate → fd → find)
2. conn.Run(command) → parse output into ScanResult list
3. User selects which to add → ProjectFlow gets updated projects map
4. onSave callback invoked → config persisted (remote or local)

**State Management:**

- Projects kept in-memory during session
- ProjectFlow callbacks (onScan, onSave) allow decoupled persistence
- Config files are TOML (simple, human-editable, no schema)
- Session metadata stored as tmux user options (@ccc_project, @ccc_path, verified flag)

## Key Abstractions

**Runner Interface (`flow/common.go`):**
- Purpose: Decouple workflow logic from execution transport
- Examples: `ssh.Connection` implements Runner for remote, `LocalRunner` for local
- Pattern: Both Run() (non-interactive, returns stdout) and RunInteractive() (PTY passthrough)
- Enables: Identical ProjectFlow/SessionFlow code paths for remote and local

**Command Builders:**
- Purpose: Centralize shell command construction with safe quoting
- Examples: `tmux.BuildListCommand()`, `scan.BuildScanChainCommand()`
- Pattern: Functions return shell strings, executed by Runner, not by caller
- Benefit: Consistent quoting via `shellutil.Quote()`, easier to audit for injection

**Session Metadata:**
- Purpose: Track which sessions were created by ccc vs existing
- Pattern: Store @ccc_project and @ccc_path as tmux user options (persist in tmux sessions, not config)
- FilterSessionsForProject: Match by metadata first (Verified=true), then by name prefix (Verified=false, warning)
- Allows: Users to pre-existing sessions into ccc workflows safely

**Config Split:**
- Client config (~/.ccc/config.toml): Host definitions (user, address, identity file, proxy jump, fallback addresses)
- Project config (remote ~/.ccc/projects.toml): Tracked projects on that host
- Pattern: Client-side stored locally, project config lives on remote, read/written via SSH in remote mode

**Callback Pattern:**
- Purpose: Allow ProjectFlow to support both remote (SSH persistence) and local (file persistence)
- Functions: onScan callback invokes RunScanFlow, onSave callback invokes remote/local write
- Benefit: ProjectFlow unchanged; caller provides storage strategy

## Entry Points

**main.go:**
- Location: `/Users/mark/Projects/jd/ccc/main.go`
- Triggers: CLI invocation (ccc or ccc local)
- Responsibilities:
  - Parse version flag
  - Detect mode (local if explicit arg or SSH_CONNECTION env set, else remote)
  - Dispatch to RunRemoteMode or RunLocalMode
  - Handle errors and exit codes

**RunRemoteMode (`flow/remote.go`):**
- Location: `/Users/mark/Projects/jd/ccc/flow/remote.go`
- Triggers: main.go (no 'local' arg, not in SSH session)
- Responsibilities:
  - Load/create client config
  - Run host selection loop (or shortcut to host if single host or args provided)
  - Connect to host via SSH with fallback addresses
  - Read projects config from remote host
  - Enter ProjectFlow with remote persistence callbacks

**RunLocalMode (`flow/local.go`):**
- Location: `/Users/mark/Projects/jd/ccc/flow/local.go`
- Triggers: main.go (explicit 'local' arg or running inside SSH session)
- Responsibilities:
  - Read local ~/.ccc/projects.toml
  - Enter ProjectFlow with local file write callback
  - Manage tmux sessions on local machine

**ProjectFlow (`flow/common.go`):**
- Location: `/Users/mark/Projects/jd/ccc/flow/common.go`
- Triggers: RunRemoteMode, RunLocalMode, or recursive (from scan/delete)
- Responsibilities:
  - Loop over project selection menu
  - Handle scan ([s]) and delete ([d]) extra actions
  - Dispatch to SessionFlow on project select

**SessionFlow (`flow/common.go`):**
- Location: `/Users/mark/Projects/jd/ccc/flow/common.go`
- Triggers: ProjectFlow on project select
- Responsibilities:
  - Check tmux installed (CheckTmux)
  - List tmux sessions, filter by project
  - Show session menu with options (attach, detach, rename, kill, new)
  - Dispatch to attachSession or createSession

## Error Handling

**Strategy:** Propagate errors up to main, which prints and exits(1)

**Patterns:**
- Non-terminal errors shown to user with fmt.Fprintf(out, ...) but execution continues (e.g., "could not save config")
- Terminal errors returned as errors, stopping flow (e.g., "failed to connect", "config parse error")
- SSH errors: wrapped with context (e.g., "failed to create session: %w")
- Config validation: checked at load time (Host requires user, address)

**Fallback Chain Example (SSH connect):**
```
Try primary address (host.Address)
  ↓ fails
Try fallback addresses from config (host.FallbackAddresses)
  ↓ all fail
Offer user to add new fallback address, test it
  ↓ fails or user declines
Return error "cannot reach host: all addresses failed"
```

## Cross-Cutting Concerns

**Logging:** No structured logging. All output via fmt.Fprintf(out, ...) to support testability (injected io.Writer).

**Validation:**
- Config: Host.User and Host.Address required (Validate method)
- Projects: Project.Path required (implied by struct, no validation)
- Commands: All user input passed through shellutil.Quote() before shell execution

**Authentication:**
- SSH: Non-interactive mode uses BatchMode=yes, StrictHostKeyChecking=accept-new (TOFU)
- Key setup: SetupFirstHost offers to generate keys or guide user through ssh-copy-id
- Fallback: Multiple addresses allow connection to same host via different networks

**Configuration Initialization:**
- Remote mode: If config missing, run SetupFirstHost (add first host with auth test)
- Local mode: If projects.toml missing, silently exit (no setup needed, projects are discovered on demand)
- Both: Scan flow available to discover projects on demand

---

*Architecture analysis: 2026-02-25*
