# External Integrations

**Analysis Date:** 2026-02-25

## APIs & External Services

**No external APIs directly consumed** - ccc is a standalone CLI tool with no HTTP/REST API calls.

## Remote Access & Network

**SSH (OpenSSH):**
- Purpose: Execute commands on remote machines and manage tmux sessions remotely
- Implementation: `ssh/connection.go` wraps `os/exec.Command("ssh", ...)`
- Non-interactive mode: Batch execution with `BatchMode=yes`, trust-on-first-use host key acceptance
- Interactive mode: PTY allocation with `-t` flag for tmux attach/interactive sessions
- Configuration: User, address, port, identity file, proxy jump, custom SSH options all configurable
- Credentials: SSH keys via `IdentityFile` config field (`.pem` files)
- Connection test: `echo ok` command to verify SSH connectivity (`ssh/connection.go:TestConnection`)

**Tailscale Network Discovery (Optional):**
- Service: Tailscale peer discovery
- Implementation: `tailscale/discovery.go` runs `tailscale status --peers`
- Purpose: Discover available peers on Tailscale network (not used in core workflow, optional integration)
- Availability check: `exec.LookPath("tailscale")` to detect if CLI installed
- Output parsing: Space-separated fields (IP, hostname, user, OS)

## Data Storage

**Local Configuration Files:**
- **Client Config:** `~/.ccc/config.toml`
  - Format: TOML via `github.com/pelletier/go-toml/v2`
  - Location: User home directory
  - Content: Host definitions (name, user, address, port, identity file, proxy jump, SSH options, fallback addresses)
  - Permissions: Directory 0700, file 0600
  - Read/Write: `config/client.go` functions `LoadClientConfig`, `SaveClientConfig`

- **Project Config (Remote):** `~/.ccc/projects.toml`
  - Format: TOML
  - Location: Home directory on target machine (accessed via SSH)
  - Content: Tracked projects with paths
  - Persistence: Read via `conn.Run("cat ~/.ccc/projects.toml")` in `flow/remote.go:143`
  - Write: Via SSH command execution with `mkdir -p ~/.ccc && printf '%s' <data> > ~/.ccc/projects.toml` in `flow/remote.go:251`

**No external databases** - All state is local files on client and remote machine

**No caching layer** - Each session reads fresh tmux metadata via shell commands

## Authentication & Identity

**SSH Key-Based Auth:**
- Type: Public key authentication via SSH
- Key locations: Configured in `config.Host.IdentityFile` field
- No passwords stored - relies on SSH agent or explicit key files
- TOML config supports multiple hosts with different identities

**SSH Session Detection:**
- For auto-detection of local vs remote mode
- Environment variables checked: `SSH_CONNECTION`, `SSH_CLIENT` (in `flow/local.go:70`)
- Purpose: Automatically use local mode when already inside an SSH session (avoid nested SSH)

**No custom auth provider** - Delegates to SSH/system authentication

## Monitoring & Observability

**Error Tracking:** None - errors returned as Go error values

**Logs:** Stdout/stderr only
- Interactive output to `io.Writer` passed to functions
- User-facing messages printed directly
- No structured logging framework

**Debugging:**
- Environment variables: `CCC_TMUX_SOCKET` for test isolation
- Integration tests in `e2e_test.go` and package-level integration tests

## CI/CD & Deployment

**GitHub Actions:**
- Workflow: `.github/workflows/release.yml`
- Trigger: Tag push (`v*`)
- Runner: `macos-latest`
- Steps:
  1. Checkout with full history (`fetch-depth: 0`)
  2. Go setup from `go.mod`
  3. Secret validation for required tokens
  4. `go vet ./...`
  5. `go test ./...`
  6. `go test -tags=integration -timeout 60s ./...`
  7. GoReleaser v2 release build

**GoReleaser:**
- Config: `.goreleaser.yaml`
- Platforms: macOS (amd64, arm64), Linux (amd64, arm64)
- Artifacts: tar.gz archives with version-specific names
- Checksums: SHA256 checksums.txt
- macOS signing: Code sign + notarize (via GitHub secrets)
- Distribution: Homebrew cask publication to `mark-jaeger/homebrew-tap`

**Required Secrets (GitHub):**
- `HOMEBREW_TAP_TOKEN` - GitHub token for Homebrew tap updates
- `MACOS_SIGN_P12` - Code signing certificate (base64)
- `MACOS_SIGN_PASSWORD` - Certificate password
- `MACOS_NOTARY_ISSUER_ID`, `MACOS_NOTARY_KEY_ID`, `MACOS_NOTARY_KEY` - Apple notarization credentials
- `GITHUB_TOKEN` - Automatic GitHub secret for release uploads

## Environment Configuration

**Required env vars:**
- None are strictly required for runtime

**Optional env vars:**
- `CCC_TMUX_SOCKET` - Override tmux socket name (used in tests)
- `SSH_CONNECTION` - Set automatically by SSH, detected for auto-local-mode
- `SSH_CLIENT` - Alternative SSH session indicator

**Build-time configuration:**
- Version injection via GoReleaser: `-X main.version={{.Version}}`
- Snapshot builds skip signing: `isEnvSet` guard in `.goreleaser.yaml`

## Webhooks & Callbacks

**None** - ccc is a pure CLI tool with no webhook support

## External Tool Dependencies

**Runtime Requirements on Target Machines:**
- `tmux` - Required for session management
- `ssh` - Required on client machine for SSH connections
- Shell interpreter (bash/sh) - Required on both client and target

**Optional on Target (for git discovery):**
- `mdfind` (macOS) - Fastest git repo discovery
- `plocate` / `locate` - Database-backed discovery
- `fd` - Fast alternative
- `find` - POSIX fallback (always works)

## Cross-Machine Communication

**SSH Connection Flow:**
1. Client reads config from `~/.ccc/config.toml`
2. Establishes SSH connection with configured host parameters
3. Reads projects from remote `~/.ccc/projects.toml`
4. All subsequent operations (tmux, scan) execute via SSH `Runner` interface
5. Project config persisted back to remote machine via SSH

**Session Metadata:**
- Sessions tagged with `@ccc_project` and `@ccc_path` user options
- Metadata enables verification (Verified=true) vs prefix matching fallback
- tmux session options set during creation: `set-option -t <session> @ccc_project <key>`

---

*Integration audit: 2026-02-25*
