# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test Commands

```bash
go build ./...          # Build all packages
go test ./...           # Run all tests
go test ./flow/         # Run tests for a single package
go test ./flow/ -run TestProjectFlowQuit  # Run a single test
go vet ./...            # Static analysis
```

No Makefile or linter config exists. CI runs via GitHub Actions on tag pushes (`.github/workflows/release.yml`), executing tests and GoReleaser on a `macos-latest` runner. The binary is built with `go build -o ccc .` for local development (gitignored as `ccc` and `ccc-*`). Release builds use GoReleaser (`.goreleaser.yaml`), which injects the version string via `-ldflags '-X main.version=...'`. macOS binaries are code-signed and notarized via GoReleaser's native `notarize` block, using secrets configured in the GitHub repository; signing is skipped for local and snapshot builds via an `isEnvSet` guard.

## Architecture

ccc is a CLI tool for managing zmx sessions on local and remote machines over SSH. It has two modes: **remote** (default, connects via SSH) and **local** (operates directly on the current machine, auto-detected when running inside an SSH session).

### Package Dependency Graph

```
main → tui → config, zmx, scan, ssh
               ssh → config, internal/shellutil
              scan → internal/shellutil
               zmx → internal/shellutil
```

### Core Abstraction: Runner Interface

The `Runner` interface (`tui/common.go`) decouples TUI commands from the execution transport:

```go
type Runner interface {
    Run(cmd string) (string, error)        // non-interactive, returns stdout
    RunInteractive(cmd string) error       // PTY passthrough
}
```

Implementations: `ssh.Connection` (remote) and local `exec.Command` calls. All zmx and scan commands are built as shell strings and executed through this interface or directly via `sh -c`.

### Control Flow

`main.go` → version flag check → mode detection → `tui.Run(isLocal)` → Bubble Tea TUI with states: HostSelect → ProjectSelect → SessionSelect → zmx attach/create.

Local mode skips host selection and SSH connection. Remote mode adds: host config loading → host selection → SSH connection → project config read from host.

### Key Design Patterns

- **Command building**: Packages (`zmx`, `scan`) export `Build*Command()` functions returning shell strings. All user-controlled values are quoted via `shellutil.Quote()`. Commands are executed through `Runner` or `exec.Command("sh", "-c", ...)`.

- **ZMX_DIR consistency**: All zmx commands are prefixed with `ZMX_DIR=${ZMX_DIR:-/tmp/zmx-$(id -u)}` to ensure SSH sessions (where `XDG_RUNTIME_DIR` is unset) and local sessions (where systemd sets it) use the same socket directory.

- **Scan fallback chain**: `scan.BuildScanChainCommand()` tries mdfind → plocate → locate → fd → find, guarded by `command -v`, using the first tool that produces non-empty output.

- **Session naming**: zmx sessions use `ccc.{project}.{suffix}` naming convention. `FilterSessionsForProject` matches by Project field, with External sessions (non-ccc prefix) shown for visibility.

- **Config split**: Client-side config (`~/.ccc/config.toml`) stores host definitions locally. Project config (`~/.ccc/projects.toml`) lives on the target machine and is read via SSH in remote mode.

### SSH Modes

Non-interactive (`Connection.Run`): `BatchMode=yes`, `StrictHostKeyChecking=accept-new` (TOFU), 10s timeout, commands wrapped in `bash -lc`.

Interactive (`Connection.RunInteractive`): PTY allocation (`-t`), stdin/stdout/stderr passthrough.

### Testing

Tests use table-driven tests with expected command output strings. TUI tests verify model state transitions via `New(isLocal)`. No external test dependencies.
