# Testing Patterns

**Analysis Date:** 2025-02-25

## Test Framework

**Runner:**
- Go standard library `testing` package (no external test framework)
- Tests run with `go test ./...` (all packages) or `go test ./package/` (single package)
- Individual tests: `go test ./flow/ -run TestProjectFlowQuit`

**Assertion Library:**
- Manual assertions via `if` statements and `t.Error*()` methods
- No external assertion library (testify, assert, etc.)
- Comparison operator patterns: `if x != expected { t.Errorf(...) }`

**Run Commands:**
```bash
go test ./...              # Run all tests
go test ./...              # No watch mode; use external tool (e.g., entr)
go test ./... -cover       # View coverage summary
go test ./... -coverprofile=coverage.out && go tool cover -html=coverage.out  # Coverage HTML
```

**Test helpers:**
- `t.TempDir()` for filesystem isolation (standard Go 1.15+)
- No external test fixtures or factories required

## Test File Organization

**Location:**
- Co-located: test files in same directory as implementation
- Pattern: `filename.go` → `filename_test.go`
- Examples: `config/client.go` → `config/client_test.go`, `flow/common.go` → `flow/common_test.go`

**Naming:**
- Test files: `*_test.go`
- Test functions: `TestFunctionName` format (standard Go)
- Subtests: `t.Run(name, func(t *testing.T) { ... })` for grouped assertions
  - Example in `config/client_test.go` → `TestHostValidate` with subtests

**Structure:**
```
config/
├── client.go
├── client_test.go
├── projects.go
├── projects_test.go
flow/
├── common.go
├── common_test.go
├── remote.go
├── local.go
ssh/
├── connection.go
├── connection_test.go
```

## Test Structure

**Suite Organization:**

Go uses flat test organization. Tests in same package group logically by function/feature:

```go
// Testing config loading
func TestLoadClientConfig(t *testing.T) { ... }
func TestLoadClientConfigMissing(t *testing.T) { ... }
func TestLoadClientConfigInvalidTOML(t *testing.T) { ... }

// Testing config saving
func TestSaveClientConfig(t *testing.T) { ... }

// Testing operations
func TestAddHost(t *testing.T) { ... }
func TestRemoveHost(t *testing.T) { ... }
func TestSortedHostNames(t *testing.T) { ... }
```

**Patterns:**

**Setup (arrange):**
```go
func TestLoadClientConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	data := `[hosts.prod]...`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
```

**Assertion (act + assert combined):**
```go
	cfg, err := LoadClientConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Hosts) != 2 {
		t.Fatalf("expected 2 hosts, got %d", len(cfg.Hosts))
	}

	prod, ok := cfg.Hosts["prod"]
	if !ok {
		t.Fatal("missing host 'prod'")
	}
	if prod.User != "deploy" {
		t.Errorf("prod.User = %q, want %q", prod.User, "deploy")
	}
```

**Subtests for parameterized cases:**
```go
func TestHostValidate(t *testing.T) {
	tests := []struct {
		name    string
		host    Host
		wantErr bool
	}{
		{"valid", Host{User: "deploy", Address: "10.0.0.1"}, false},
		{"missing user", Host{Address: "10.0.0.1"}, true},
		{"missing address", Host{User: "deploy"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.host.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
```

## Mocking

**Framework:**
- Manual mock structs implementing required interfaces
- No external mocking library (testify/mock, gomock, etc.)

**Patterns:**

**mockRunner (flow/common_test.go):**
- Implements `Runner` interface (`Run()`, `RunInteractive()`)
- Maps command patterns to responses and errors
- Tracks interactive calls for assertion

```go
type mockRunner struct {
	responses   map[string]string
	errors      map[string]error
	interactive []string
}

func (m *mockRunner) Run(cmd string) (string, error) {
	if err, ok := m.errors[cmd]; ok {
		return "", err
	}
	// Check prefix matches (for shell-quoted commands)
	for pattern, err := range m.errors {
		if strings.Contains(cmd, pattern) {
			return "", err
		}
	}
	if resp, ok := m.responses[cmd]; ok {
		return resp, nil
	}
	// Check prefix matches for responses
	for pattern, resp := range m.responses {
		if strings.Contains(cmd, pattern) {
			return resp, nil
		}
	}
	return "", fmt.Errorf("unexpected command: %s", cmd)
}

func (m *mockRunner) RunInteractive(cmd string) error {
	m.interactive = append(m.interactive, cmd)
	return nil
}
```

**Usage in test:**
```go
runner := newMockRunner()
runner.responses["tmux list-sessions"] = ""
runner.responses["tmux new-session"] = ""
runner.errors["test -d"] = fmt.Errorf("exit 1")

err := SessionFlow(in, out, runner, "myapp", "/home/user/myapp")
// Assert:
if len(runner.interactive) == 0 {
	t.Error("expected RunInteractive to be called")
}
```

**What to Mock:**
- External commands: `ssh` execution, `tmux` operations, filesystem checks
- Pass mocks via interface: `Runner` interface abstracts execution
- Avoid mocking `io.Reader`/`io.Writer` — use real `strings.NewReader` and `bytes.Buffer`

**What NOT to Mock:**
- Standard library (use real `os.TempDir()`, real `time.Now()`)
- Parsing functions (test real parsing logic)
- Configuration structs (use real TOML marshaling/unmarshaling)

## Fixtures and Factories

**Test Data:**

**In-memory config fixtures:**
```go
projects := &config.ProjectsConfig{
	Projects: map[string]config.Project{
		"myapp": {Path: "/home/user/myapp"},
		"other": {Path: "/home/user/other"},
	},
}
```

**TOML fixtures (for file round-trip tests):**
```go
data := `[hosts.prod]
user = "deploy"
address = "10.0.0.1"
port = 2222
identity_file = "/home/deploy/.ssh/id_ed25519"
proxy_jump = "bastion"
ssh_options = ["-o", "StrictHostKeyChecking=no"]
`
```

**Session parsing fixtures:**
```go
runner.responses["tmux list-sessions"] = "myapp|||myapp|||/home/user/myapp|||2\nmyapp-2|||myapp|||/home/user/myapp|||1"
```

**Location:**
- Fixtures defined inline in test functions (not external files)
- Reused via copy-paste or by setting up mockRunner with common responses
- No factory pattern; tests create `config.*` structs directly

## Coverage

**Requirements:**
- No explicit coverage target enforced in CI/repository
- Coverage metrics available via `go test -cover ./...`

**View Coverage:**
```bash
go test ./... -cover                                    # Summary by package
go test ./... -coverprofile=coverage.out                # Generate profile
go tool cover -html=coverage.out                        # Open in browser
go tool cover -func=coverage.out                        # Per-function breakdown
```

## Test Types

**Unit Tests:**
- **Scope:** Individual functions and small workflows
- **Approach:**
  - Mock external dependencies (Runner for SSH/tmux)
  - Use real file I/O via `t.TempDir()`
  - Test happy path, error cases, edge cases
  - Examples: `TestLoadClientConfig`, `TestParseSessionList`, `TestFilterSessionsForProject`

**Integration Tests:**
- **Scope:** Multi-function workflows (e.g., project selection → session creation)
- **Approach:**
  - Mock only OS-level commands (via mockRunner)
  - Use real parsing/filtering logic
  - Simulate user input via `strings.NewReader`
  - Capture output to `bytes.Buffer` for assertion
  - Examples: `TestProjectFlowSelectProject`, `TestSessionFlowZeroSessionsCreatesNew`

**E2E Tests:**
- **Framework:** Not used in main codebase
- **Rationale:** CLI integration best tested manually or via CI system tests
- **Exception:** `e2e_test.go` exists but may be a placeholder

**Test Example (integration):**
```go
func TestProjectFlowSelectProject(t *testing.T) {
	runner := newMockRunner()
	runner.responses["test -d"] = ""
	runner.responses["command -v tmux"] = "/usr/bin/tmux"
	runner.responses["tmux list-sessions"] = ""
	runner.responses["tmux new-session"] = ""

	projects := &config.ProjectsConfig{
		Projects: map[string]config.Project{
			"myapp": {Path: "/home/user/myapp"},
		},
	}

	// Simulate user selecting project 1, then entering default session name
	in := strings.NewReader("1\n\n")
	out := &bytes.Buffer{}

	err := ProjectFlow(in, out, runner, projects, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "myapp") {
		t.Errorf("expected project 'myapp' in output, got: %s", output)
	}
}
```

## Common Patterns

**Async Testing:**

Not applicable — no goroutines or channels in codebase. All operations are synchronous.

**Error Testing:**

**Testing error returns:**
```go
func TestLoadClientConfigMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.toml")

	_, err := LoadClientConfig(path)
	if err != ErrNoConfig {
		t.Fatalf("expected ErrNoConfig, got %v", err)
	}
}
```

**Testing error wrapping:**
```go
func TestDeleteProjectSaveError(t *testing.T) {
	onSave := func(cfg *config.ProjectsConfig) error {
		return fmt.Errorf("disk full")
	}

	err := deleteProject(in, out, projects, item, onSave)
	if err == nil {
		t.Fatal("expected error when save fails")
	}
	if !strings.Contains(err.Error(), "failed to delete project") {
		t.Errorf("expected wrapped error, got: %v", err)
	}

	// Verify rollback occurred
	if _, exists := projects.Projects["myapp"]; !exists {
		t.Error("expected project to be rolled back after save failure")
	}
}
```

**Testing input handling edge cases:**
```go
func TestProjectFlowPathNotFoundProjectPersists(t *testing.T) {
	runner := newMockRunner()
	runner.errors["test -d"] = fmt.Errorf("exit 1")

	projects := &config.ProjectsConfig{
		Projects: map[string]config.Project{
			"myapp": {Path: "/home/user/myapp"},
		},
	}

	// Select project, path not found, Confirm gets EOF → decline
	in := strings.NewReader("1\n")
	out := &bytes.Buffer{}

	err := ProjectFlow(in, out, runner, projects, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Project should still exist since confirmation wasn't provided
	if _, exists := projects.Projects["myapp"]; !exists {
		t.Error("expected project to still exist when confirmation not provided")
	}
}
```

Note: Due to `bufio.Scanner` buffering in `ShowMenu`, subsequent `Confirm`/`Prompt` calls from within menu-driven flows receive EOF when input is exhausted. Tests document this behavior and work around it.

**Testing state transitions:**
```go
func TestSessionFlowDetachWithClients(t *testing.T) {
	runner := newMockRunner()
	runner.responses["command -v tmux"] = "/usr/bin/tmux"
	runner.responses["tmux list-sessions"] = "myapp|||...|||2\nmyapp-2|||...|||1"
	runner.responses["tmux list-clients"] = "/dev/ttys004: 220x56 0"
	runner.responses["tmux detach-client"] = ""

	// Detach action, select session, quit
	in := strings.NewReader("t\n1\nq\n")
	out := &bytes.Buffer{}

	err := SessionFlow(in, out, runner, "myapp", "/home/user/myapp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify state change message
	if !strings.Contains(out.String(), "Detached 1 client(s)") {
		t.Errorf("expected detach success message, got: %s", out.String())
	}
}
```

---

*Testing analysis: 2025-02-25*
