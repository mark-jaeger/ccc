# Coding Conventions

**Analysis Date:** 2025-02-25

## Naming Patterns

**Files:**
- Go source files follow standard `package_name.go` pattern
- Test files: `filename_test.go` (co-located with implementation)
- Packages named after directory (e.g., `flow`, `config`, `ssh`, `tmux`)
- Example: `flow/common.go`, `flow/common_test.go`

**Functions:**
- Exported functions: PascalCase (e.g., `LoadClientConfig`, `BuildListCommand`, `FilterSessionsForProject`)
- Unexported functions: camelCase (e.g., `tmuxCmd()`, `connectToHost()`, `targetAddress()`)
- Command builders: `Build*Command()` pattern for functions that return shell commands
  - Example: `BuildListCommand()`, `BuildCreateCommand()`, `BuildAttachCommand()`
- Parse/filter functions: `Parse*()` and `Filter*()`
  - Example: `ParseSessionList()`, `FilterSessionsForProject()`

**Variables:**
- Receiver variables: short names (e.g., `c` for Connection, `m` for mockRunner, `h` for Host)
- Loop variables: `i`, `s` for sessions, `c` for config, etc.
- Error variables: `err` consistently, with suffix patterns for multiple errors
  - Example: `saveErr`, `loadErr`, `confirmErr`, `addErr`
- Map key iteration: named variables (`name`, `key`, `addr`) rather than underscore when value is used

**Types:**
- Struct types: PascalCase (e.g., `Connection`, `Session`, `Host`, `MenuConfig`)
- Interface types: PascalCase (e.g., `Runner` - single method `Run()`)
- Fields use snake_case in TOML tags: `@toml:"identity_file"` (example in Host struct)

## Code Style

**Formatting:**
- Standard Go formatting (gofmt) — no explicit linter config needed
- Indentation: tabs (Go standard)
- Line length: fits reasonably on screen; no hard limit enforced
- Consistent spacing: single blank line between functions, double blank line between major sections

**Linting:**
- `go vet ./...` used for static analysis per CLAUDE.md
- No external linter config (eslint, golangci-lint) in repository
- Errors are formatted with `fmt.Errorf("context: %w", err)` for error wrapping

## Import Organization

**Order:**
1. Standard library imports (e.g., `fmt`, `io`, `os`, `strings`)
2. External dependencies (e.g., `github.com/pelletier/go-toml/v2`)
3. Local package imports (e.g., `github.com/mark-jaeger/ccc/...`)

**Example from `flow/remote.go`:**
```go
import (
	"errors"
	"fmt"
	"io"

	"github.com/mark-jaeger/ccc/config"
	"github.com/mark-jaeger/ccc/internal/shellutil"
	sshpkg "github.com/mark-jaeger/ccc/ssh"
	"github.com/mark-jaeger/ccc/ui"
)
```

**Aliases:**
- Used when needed to avoid conflicts: `sshpkg "github.com/mark-jaeger/ccc/ssh"` (because `ssh` is reserved)
- Not used for standard practice; most packages use unaliased names

## Error Handling

**Patterns:**
- Explicit error returns in function signatures: `(T, error)` or `error`
- Errors wrapped with context: `fmt.Errorf("action: %w", err)` provides call-stack context
- Sentinel errors: Package-level vars like `ErrNoConfig` in `config/client.go` for domain-specific errors
- Error checks consistently placed immediately after operation:
  ```go
  out, err := exec.Command("ssh", args...).Output()
  if err != nil {
  	return "", fmt.Errorf("ssh command failed: %w", err)
  }
  ```

**Silent failure patterns (warnings only):**
- Non-critical operations report via `fmt.Fprintf(out, "Warning: %v\n", err)` to user output
  - Example: SSH connection save failures, config reload failures
- Interactive sessions continue after non-fatal errors (e.g., failed client detach)
  ```go
  if _, err := runner.Run(cmd); err != nil {
  	fmt.Fprintf(out, "Warning: could not detach clients: %v\n", err)
  } else {
  	fmt.Fprintf(out, "Detached %d client(s)\n", len(clients))
  }
  ```

**Rollback on error:**
- When mutation can fail: preserve original state for rollback
- Example in `deleteProject()`: store original before deletion, restore if save fails
  ```go
  oldProject := projects.Projects[item.Key]
  delete(projects.Projects, item.Key)
  if saveErr := onSave(projects); saveErr != nil {
  	projects.Projects[item.Key] = oldProject
  	return fmt.Errorf("failed to delete project: %w", saveErr)
  }
  ```

## Logging

**Framework:**
- `fmt.Fprintf(out io.Writer, format string, args...)` for all user-facing output
- No structured logging or external logging framework
- Output is always passed as `io.Writer` parameter (enables test capture via `bytes.Buffer`)

**Patterns:**
- Status messages: `fmt.Fprintf(out, "  ✓ Message\n", ...)`  - leading spaces for indentation
- Informational: `fmt.Fprintf(out, "  Information\n", ...)`
- Warnings: `fmt.Fprintf(out, "  Warning: %v\n", err)`
- Errors: `fmt.Fprintf(out, "  Error: %v\n", err)` OR return error up call stack
- Menu output: delegated to `ui` package which manages its own output

**Example from `flow/common.go`:**
```go
fmt.Fprintf(out, "  ✓ Created session %s\n", name)
fmt.Fprintf(out, "  ✓ Renamed %s → %s\n", item.Key, newName)
fmt.Fprintf(out, "  Attaching to %s...\n", session.Name)
```

## Comments

**When to Comment:**
- Function: Always document exported functions with comment starting with function name
  - Example: `// LoadClientConfig reads and parses a TOML client config...`
- Unexported functions: Comment if logic is non-obvious or cross-package relevant
- Complex logic: Explain WHY (not WHAT), especially for shell command construction or special cases
- Gotchas: Document known limitations or surprising behaviors

**JSDoc/TSDoc:**
- Not used in Go codebase
- Standard Go doc comments (no special format)
- Example from `config/client.go`:
  ```go
  // DefaultClientConfigPath returns the default path for the client config file.
  // Returns an error if the home directory cannot be determined.
  func DefaultClientConfigPath() (string, error) {
  ```

**Special comment patterns:**
- `// TOFU` comment pattern: explains trust-on-first-use semantics in SSH connection setup
- Block comments for detailed explanations (e.g., tmux command building in `tmux/sessions.go`)
  ```go
  // bell-action any forwards BEL characters from any window to the attached
  // terminal. visual-bell off ensures tmux sends the real BEL byte (0x07)
  // rather than flashing the status bar.
  ```

## Function Design

**Size:**
- Prefer functions under 50 lines
- Longer flows (like `SessionFlow`) document complexity with inline comments and clear variable names
- Helper functions extracted for clarity: `attachSession()`, `createSession()`, `killSession()` separate concerns

**Parameters:**
- Use receiver pattern for methods: `(c *Connection)` rather than `connection *Connection` parameter
- Multiple return values: `(result, error)` standard; no tuples or custom return types
- Context passed as parameters: `(in io.Reader, out io.Writer)` for I/O, `(runner Runner)` for execution
- Callbacks: passed as function parameters to decouple flows
  - Example: `ProjectFlow(..., onScan func(...) (*config.ProjectsConfig, error), onSave func(...) error)`

**Return Values:**
- Single value types preferred: return `[]Session` not `*[]Session`
- Error always last: `(T, error)`, `(T1, T2, error)`, never `(error, T)`
- Blank returns when only error: simple `return` or `return nil` when successful

## Module Design

**Exports:**
- All exported types documented at top of package
- Runner interface in `flow/common.go` is core abstraction used across packages
- Packages export builder functions that return strings (commands), not execute them
  - Forces composition in `Runner` implementations, enabling testability
  - Example: `tmux.BuildListCommand()` returns string; `Connection.Run()` executes

**Barrel Files:**
- Not used; each package is minimal and focused
- `flow/common.go` contains shared types/interfaces, flow functions in separate files (`flow/remote.go`, `flow/local.go`)

**Interface Segregation:**
- Small interfaces preferred: `Runner` has only 2 methods (`Run`, `RunInteractive`)
- Implementations (`Connection`, `LocalRunner`) are simple adapters
- Easier to mock and test (see `mockRunner` in tests)

---

*Convention analysis: 2025-02-25*
