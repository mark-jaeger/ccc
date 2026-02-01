# Integration & E2E Testing Strategy

## Problem

ccc builds tmux command strings and executes them through a `Runner` interface. Current tests use `mockRunner` to verify command strings are correct, but cannot verify that commands actually produce the expected tmux state. Features like notification passthrough (bell-action, allow-passthrough) have no way to confirm they work against real tmux. Flow orchestration (25.3% coverage) is undertested.

## Decisions

- **Two-layer approach**: isolated tmux server tests (integration) + thin PTY/expect tests (e2e)
- **Build tag gating**: `//go:build integration` — regular `go test ./...` stays fast, `go test -tags=integration ./...` runs everything
- **Test harness location**: `internal/testutil/` — shared across `tmux`, `flow`, and root-level e2e tests
- **PTY library**: `creack/pty` with custom lightweight expect helpers (no go-expect dependency)

## Architecture

```
Layer 3: E2E (e2e_test.go)
  PTY + expect helpers, launches real ccc binary
  Tests: menu navigation, session creation, clean exit

Layer 2: Flow Integration (flow/flow_integration_test.go)
  Real tmux via TestTmux as Runner, simulated I/O
  Tests: project flow, session flow, notification option application

Layer 1: Tmux Integration (tmux/sessions_integration_test.go)
  Real tmux via TestTmux, command execution + state verification
  Tests: metadata, bell options, passthrough, idempotency, session filtering
```

All layers share `internal/testutil/` for tmux server lifecycle and expect helpers.

## Component: TestTmux Harness

`internal/testutil/tmuxtest.go`

Manages an isolated tmux server per test via unique socket names.

```go
type TestTmux struct {
    Socket  string
    BinPath string
}

func NewTestTmux(t *testing.T) *TestTmux
func (tt *TestTmux) Run(cmd string) (string, error)
func (tt *TestTmux) RunInteractive(cmd string) error
func (tt *TestTmux) CreateSession(name string) error
func (tt *TestTmux) GetOption(session, key string) string
func (tt *TestTmux) GetWindowOption(session, key string) string
func (tt *TestTmux) SessionExists(name string) bool
func (tt *TestTmux) ListSessions() []string
```

- `NewTestTmux` resolves the tmux binary, generates a unique socket name from `t.Name()` + random suffix, starts a detached server, registers `t.Cleanup()` to kill it, and skips the test if tmux is not installed.
- `Run`/`RunInteractive` satisfy the `Runner` interface. All commands get `-L <socket>` injected to target the isolated server.
- `GetOption`/`GetWindowOption` use `tmux show-options -t <session> -v <key>` to query state for assertions.
- Every test is self-contained and `t.Parallel()` safe.

## Component: Expect Helpers

`internal/testutil/expect.go`

Lightweight PTY expect helpers built on `creack/pty`.

```go
type Process struct {
    PTY  *os.File
    Cmd  *exec.Cmd
}

func StartProcess(t *testing.T, args ...string) *Process
func (p *Process) ReadUntil(t *testing.T, match string, timeout time.Duration) string
func (p *Process) Send(t *testing.T, input string)
```

- `StartProcess` builds the ccc binary via `go build`, launches it with a PTY, registers cleanup.
- `ReadUntil` reads from the PTY fd until the match string appears or the timeout fires (test fails on timeout).
- `Send` writes raw bytes to the PTY (keystrokes, arrow keys, enter).

## Layer 1: Tmux Integration Tests

`tmux/sessions_integration_test.go`

| Test | Verifies |
|------|----------|
| `TestCreateSession_SetsMetadata` | `@ccc_project` and `@ccc_path` user options set on real session |
| `TestCreateSession_SetsBellOptions` | `bell-action any` and `visual-bell off` set on creation |
| `TestSetPassthrough_EnablesOption` | `allow-passthrough on` set by separate command |
| `TestEnsureNotifyOptions_SetsAllOptions` | All three options applied to a bare pre-existing session |
| `TestEnsureNotifyOptions_Idempotent` | Running twice produces no errors, options remain correct |
| `TestFilterSessionsForProject_RealTmux` | Full chain: real tmux output -> ParseSessions -> FilterSessionsForProject |

## Layer 2: Flow Integration Tests

`flow/flow_integration_test.go`

| Test | Verifies |
|------|----------|
| `TestProjectFlow_CreateNewSession` | Session created with metadata + notification options |
| `TestProjectFlow_AttachExistingSession_SetsNotifyOptions` | Pre-existing bare session gets notification options on attach |
| `TestSessionFlow_DetachSession` | Session persists after detach, no attached clients |
| `TestProjectFlow_MultipleSessionsFiltered` | Only matching project sessions appear |

`TestTmux.RunInteractive` is a recording no-op at this layer — it logs the command for assertion but does not block on a PTY. The e2e layer covers actual attach behavior.

## Layer 3: E2E Tests

`e2e_test.go` (repo root)

| Test | Verifies |
|------|----------|
| `TestE2E_LocalMode_ProjectMenu` | Binary launches, project menu renders, arrow-key navigation works |
| `TestE2E_LocalMode_CreateAndAttachSession` | Full flow: menu -> new session -> tmux session exists with options |
| `TestE2E_LocalMode_QuitFromMenu` | `q` input produces clean zero-status exit |

Three tests only. Covers the happy path. Not trying to exhaustively test every menu permutation.

## Code Change: Socket Injection

Environment variable `CCC_TMUX_SOCKET`: when set, all tmux commands include `-L <socket>`. Change is in `tmux/sessions.go` — the `Build*Command` functions accept an optional socket parameter or read a package-level variable.

- Production path: env var absent, no behavior change.
- Test path: `TestTmux` sets the env var (or passes socket directly) before executing commands.
- E2e path: `StartProcess` launches ccc with `CCC_TMUX_SOCKET` set in the child process environment.

## New Dependency

`github.com/creack/pty` — added to `go.mod`, used only in `_test.go` files. Not compiled into the production binary.

## CI Integration

`.github/workflows/release.yml` — add step after existing tests:

```yaml
- name: Integration tests
  run: go test -tags=integration -timeout 60s ./...
```

Runs on `macos-latest` where tmux is pre-installed. 60s timeout guards against hung processes.

## File Summary

| File | Purpose |
|------|---------|
| `internal/testutil/tmuxtest.go` | Isolated tmux server harness |
| `internal/testutil/expect.go` | PTY expect helpers |
| `tmux/sessions_integration_test.go` | Tmux state verification (6 tests) |
| `flow/flow_integration_test.go` | Flow orchestration verification (4 tests) |
| `e2e_test.go` | End-to-end smoke tests (3 tests) |
| `tmux/sessions.go` | Small edit for socket injection |
| `.github/workflows/release.yml` | CI step for integration tests |
| `go.mod` | Add `creack/pty` dependency |
