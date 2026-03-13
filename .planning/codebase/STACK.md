# Technology Stack

**Analysis Date:** 2026-02-25

## Languages

**Primary:**
- Go 1.25.6 - Complete application codebase; entry point at `main.go`

## Runtime

**Environment:**
- Go runtime (self-contained binary)

**Package Manager:**
- Go modules (`go.mod`, `go.sum`)
- Lockfile: Present (`go.sum`)

## Frameworks

**Core:**
- None - pure Go standard library implementation

**CLI/UI:**
- Standard library (`os`, `io`, `bufio`) - Interactive terminal UI built in `ui/` package
- Terminal handling: `golang.org/x/term` v0.39.0 - ANSI terminal operations and raw mode
- PTY support: `github.com/creack/pty` v1.1.24 - Pseudo-terminal allocation for interactive SSH sessions

**Configuration:**
- `github.com/pelletier/go-toml/v2` v2.2.4 - TOML parsing for `config/client.go` and `config/projects.go`

**System/OS:**
- `golang.org/x/sys` v0.40.0 (indirect) - Low-level OS functionality

## Key Dependencies

**Critical:**
- `github.com/creack/pty` v1.1.24 - Enables PTY allocation in `ssh/connection.go` for interactive sessions (`Connection.RunInteractive`)
- `golang.org/x/term` v0.39.0 - Terminal operations in `ui/interactive.go` for raw mode input handling
- `github.com/pelletier/go-toml/v2` v2.2.4 - Configuration persistence for hosts (`~/.ccc/config.toml`) and projects (`~/.ccc/projects.toml`)

**Infrastructure:**
- None - no external service SDKs

## Configuration

**Environment:**
- `CCC_TMUX_SOCKET` - Optional tmux socket override for test isolation (read in `main.go` line 23)
- `SSH_CONNECTION` - Auto-detection of SSH session (checked in `flow/local.go` line 70)
- `SSH_CLIENT` - Alternative SSH session indicator (checked in `flow/local.go` line 70)

**Build:**
- `.goreleaser.yaml` - Release automation with version injection via `-X main.version={{.Version}}`
- Version flag: `-ldflags '-s -w -X main.version=...'` injects version at build time
- Local binary: `go build -o ccc .` (binary `.gitignored` as `ccc` and `ccc-*`)

## Platform Requirements

**Development:**
- Go 1.25.6 or compatible
- macOS, Linux (configured in `.goreleaser.yaml` for both amd64 and arm64)
- SSH client installed (used via `os/exec` in `ssh/connection.go`)
- tmux installed on target machines (used via shell commands)

**Production:**
- Deployment: GitHub Actions CI/CD (`.github/workflows/release.yml`)
- Release: GoReleaser v2 for cross-platform builds
- macOS code signing and notarization via GitHub secrets:
  - `MACOS_SIGN_P12` - Signing certificate
  - `MACOS_SIGN_PASSWORD` - Certificate password
  - `MACOS_NOTARY_ISSUER_ID`, `MACOS_NOTARY_KEY_ID`, `MACOS_NOTARY_KEY` - Notarization credentials
- Homebrew distribution via custom tap (`homebrew-tap` repository)

## Build Commands

```bash
go build ./...          # Build all packages
go test ./...           # Run all tests
go test -tags=integration -timeout 60s ./...  # Integration tests with tmux
go vet ./...            # Static analysis
go build -o ccc .       # Local binary for development
```

## File Storage

**Configuration Locations:**
- Client config: `~/.ccc/config.toml` - Stores host definitions (created with 0700 perms, file 0600)
- Project config: `~/.ccc/projects.toml` - Stored on remote machine via SSH, read/written by connection

## External Command Dependencies

**SSH:**
- `ssh` command - Executed via `os/exec` for remote connections in `ssh/connection.go`
- SSH options: BatchMode=yes, StrictHostKeyChecking=accept-new (TOFU), ConnectTimeout=10s

**tmux:**
- `tmux` command - All session operations via shell commands built in `tmux/sessions.go`
- Version requirement: >= 3.0 (optional: >= 3.3 for `allow-passthrough` notifications)

**Discovery (optional on target):**
- `mdfind` (macOS) - Git repository discovery
- `plocate` / `locate` - Database-backed repository discovery
- `fd` - Fast find alternative
- `find` - POSIX fallback (always available)
- Fallback chain in `scan/projects.go`: mdfind → plocate → locate → fd → find

## Workspace/Git

- Git-based repository with branches and tags
- Release triggers on `v*` tags (`.github/workflows/release.yml`)
- Git worktrees in `.worktrees/` for feature branches (`integration-testing`, `rename-sessions`)

---

*Stack analysis: 2026-02-25*
