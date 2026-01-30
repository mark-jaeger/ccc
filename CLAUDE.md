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

No Makefile or linter config exists. CI runs via GitHub Actions on tag pushes (`.github/workflows/release.yml`), executing tests and GoReleaser. The binary is built with `go build -o ccc .` for local development (gitignored as `ccc` and `ccc-*`). Release builds use GoReleaser (`.goreleaser.yaml`), which injects the version string via `-ldflags '-X main.version=...'`.

## Architecture

ccc is a CLI tool for managing tmux sessions on local and remote machines over SSH. It has two modes: **remote** (default, connects via SSH) and **local** (operates directly on the current machine, auto-detected when running inside an SSH session).

### Package Dependency Graph

```
main → flow → config, ssh, tmux, scan, ui, tailscale
                ssh → config, internal/shellutil
               scan → internal/shellutil
               tmux → internal/shellutil
```

### Core Abstraction: Runner Interface

The `Runner` interface (`flow/common.go`) decouples all workflow logic from the execution transport:

```go
type Runner interface {
    Run(cmd string) (string, error)        // non-interactive, returns stdout
    RunInteractive(cmd string) error       // PTY passthrough
}
```

Implementations: `ssh.Connection` (remote) and `flow.LocalRunner` (local). All tmux and scan commands are built as shell strings and executed through this interface.

### Control Flow

`main.go` → version flag check → mode detection → `RunRemoteMode` or `RunLocalMode` → `ProjectFlow` (project menu loop) → `SessionFlow` (session menu loop) → tmux attach/create.

Remote mode adds: host config loading → host selection loop → SSH connection → project config read from host.

### Key Design Patterns

- **Command building**: Packages (`tmux`, `scan`) export `Build*Command()` functions returning shell strings. All user-controlled values are quoted via `shellutil.Quote()`. Commands are executed through `Runner`, not `os/exec` directly (except in `Runner` implementations).

- **Scan fallback chain**: `scan.BuildScanChainCommand()` tries mdfind → plocate → locate → fd → find, guarded by `command -v`, using the first tool that produces non-empty output.

- **Session metadata**: tmux sessions are tagged with `@ccc_project` and `@ccc_path` user options. `FilterSessionsForProject` matches by metadata first (Verified=true), then by name prefix (Verified=false, shown with warning).

- **Config split**: Client-side config (`~/.ccc/config.toml`) stores host definitions locally. Project config (`~/.ccc/projects.toml`) lives on the target machine and is read via SSH in remote mode.

- **Callback parameters**: `ProjectFlow` takes `onScan` and `onSave` callbacks so the same flow code works for both remote (SSH-based persistence) and local (file-based persistence) modes.

### SSH Modes

Non-interactive (`Connection.Run`): `BatchMode=yes`, `StrictHostKeyChecking=accept-new` (TOFU), 10s timeout, commands wrapped in `bash -lc`.

Interactive (`Connection.RunInteractive`): PTY allocation (`-t`), stdin/stdout/stderr passthrough.

### Testing

Tests use a `mockRunner` in `flow/common_test.go` that maps command substrings to responses/errors. UI tests use `strings.NewReader` for input and `bytes.Buffer` for output. No external test dependencies. Note: `bufio.Scanner` in `ShowMenu` buffers the full reader, so `ui.Confirm`/`ui.Prompt` called from within a `ShowMenu`-driven flow cannot read from the same `io.Reader` — they receive EOF instead.
