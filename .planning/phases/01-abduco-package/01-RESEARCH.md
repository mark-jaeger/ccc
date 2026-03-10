# Phase 1: abduco-package - Research

**Researched:** 2026-03-10
**Domain:** abduco terminal session management, Go package design
**Confidence:** HIGH

## Summary

This phase creates the `abduco/` package to replace the existing `tmux/` package for session management. The abduco tool provides a simpler model than tmux: transparent PTY passthrough with no multiplexing, automatic client detachment, and a straightforward tab-delimited output format.

Research confirms abduco's output format from source code analysis: `status timestamp\tPID\tname` where status is `*` (attached), `+` (dead/terminated), or space (detached). The tool has no native kill command, requiring PID-based termination via `kill` or `kill -HUP`. This aligns with the user decision to use PID extraction rather than pkill.

**Primary recommendation:** Follow the tmux package structure closely (Build*Command, Parse*, Filter*ForProject patterns) but with simplified logic. The abduco package will be ~120 lines vs tmux's ~280 lines due to removed complexity (no clients, no passthrough config, no metadata tags).

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- Session naming: Pattern `ccc.{project}.{suffix}`, first session gets "main", subsequent get 2, 3, 4...
- Dead session handling: Show with `(dead)` marker, no auto-cleaning
- External session display: Show with `(external)` marker
- Kill implementation: Extract PID from abduco list output, kill via PID (not pkill)
- Error messaging: Short error with platform-specific install hints (brew/apt)

### Claude's Discretion
- Exact regex pattern for parsing abduco output
- Test structure and mock data format
- Integration test skip logic when abduco not installed

### Deferred Ideas (OUT OF SCOPE)
None
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| SESS-01 | Create abduco session with `ccc.{project}.{suffix}` naming | `BuildCreateCommand` using `abduco -n name command` |
| SESS-02 | List sessions filtering by project prefix | `BuildListCommand` with `2>&1` redirect, `FilterSessionsForProject` |
| SESS-03 | Attach to session (auto-detaches previous client) | `BuildAttachCommand` using `abduco -a name` |
| SESS-04 | Kill session using PID from list output | `BuildKillCommand` using PID extracted from parsed output |
| SESS-05 | Show external (non-ccc) abduco sessions marked as "(external)" | Session struct with `External bool` field |
| SESS-06 | Handle dead sessions (`+` status) in list output | Session struct with `Dead bool` field, regex captures status |
| MIGR-01 | Replace tmux package with abduco package | New `abduco/` package mirrors `tmux/` API |
| ERRH-01 | Clear error when abduco not installed | `BuildCheckCommand` using `command -v abduco` |
| ERRH-02 | Handle stderr output from abduco (2>&1 redirect) | List command includes `2>&1` redirect |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `regexp` | N/A | Parse abduco output | Standard lib, no external deps |
| Go stdlib `strings` | N/A | String manipulation | Standard lib |
| Go stdlib `strconv` | N/A | Parse numbers | Standard lib |
| `internal/shellutil` | local | Quote shell arguments | Existing project pattern |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `os/exec` | N/A | LookPath for integration tests | Skip tests when abduco not installed |
| `testing` | N/A | Unit and integration tests | Test framework |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Custom regex | Split by tab | Regex handles variable whitespace in timestamp |
| Signal-based kill | Socket communication | PID kill is simpler, matches user decision |

**Installation:**
```bash
# No external dependencies needed - pure Go stdlib
```

## Architecture Patterns

### Recommended Project Structure
```
abduco/
    sessions.go              # Command builders, parsers, session struct
    sessions_test.go         # Unit tests with mock output
    sessions_integration_test.go  # Integration tests (go:build integration)
```

### Pattern 1: Command Builder Functions
**What:** Functions that return shell command strings, not execute them
**When to use:** All command generation
**Example:**
```go
// Source: existing tmux/sessions.go pattern
func BuildCreateCommand(name, path string) string {
    return fmt.Sprintf("cd %s && abduco -n %s bash -l",
        shellutil.Quote(path), shellutil.Quote(name))
}

func BuildAttachCommand(name string) string {
    return fmt.Sprintf("abduco -a %s", shellutil.Quote(name))
}

func BuildListCommand() string {
    return "abduco 2>&1 || true"
}

func BuildKillCommand(pid int) string {
    return fmt.Sprintf("kill %d", pid)
}

func BuildCheckCommand() string {
    return "command -v abduco"
}
```

### Pattern 2: Parser Functions
**What:** Functions that parse command output into typed structs
**When to use:** All output parsing
**Example:**
```go
// Source: existing tmux/sessions.go pattern
type Session struct {
    Name     string
    Project  string // extracted from name, empty if external
    Suffix   string // extracted from name
    External bool   // true if not a ccc session
    Dead     bool   // true if session has + status
    PID      int    // for kill command
}

func ParseSessionList(output string) []Session {
    // Parse each line matching: status timestamp\tPID\tname
    // status: * = attached, + = dead, space = detached
}
```

### Pattern 3: Filter Functions
**What:** Filter sessions by project prefix
**When to use:** Displaying project-specific sessions
**Example:**
```go
// Source: existing tmux/sessions.go pattern
func FilterSessionsForProject(sessions []Session, projectKey string) []Session {
    prefix := "ccc." + projectKey + "."
    var result []Session
    for _, s := range sessions {
        if s.Project == projectKey {
            result = append(result, s)
        } else if s.External {
            // Include external sessions for visibility
            result = append(result, s)
        }
    }
    return result
}
```

### Anti-Patterns to Avoid
- **Executing commands directly:** Package builds commands; Runner executes them
- **Using pkill:** Dangerous pattern that can match too broadly; use PID-based kill
- **Parsing without 2>&1:** abduco writes to stderr; must redirect to capture output

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Shell quoting | Custom escape logic | `shellutil.Quote()` | Handles edge cases (single quotes, special chars) |
| Command execution | `os/exec` in package | Runner interface | Decouples from transport (SSH vs local) |
| Session name parsing | Ad-hoc string splits | Regex with named groups | Handles varying whitespace in timestamp |

**Key insight:** The abduco package should ONLY build command strings and parse output. Command execution is handled by the Runner interface in flow/common.go.

## Common Pitfalls

### Pitfall 1: Stderr vs Stdout
**What goes wrong:** abduco writes session list to stderr, not stdout
**Why it happens:** abduco's design choice
**How to avoid:** Always include `2>&1` redirect in list command
**Warning signs:** Empty parse results when sessions exist

### Pitfall 2: Session Name Parsing
**What goes wrong:** Session names can contain dots
**Why it happens:** Only the first two dots in `ccc.{project}.{suffix}` are structural
**How to avoid:** Use `strings.SplitN(name[4:], ".", 2)` to split after "ccc."
**Warning signs:** Suffix contains unexpected dots

### Pitfall 3: PID Column Position
**What goes wrong:** Assuming fixed column positions
**Why it happens:** Timestamp format varies by locale
**How to avoid:** Parse by tab delimiters, not column positions
**Warning signs:** Incorrect PID extraction

### Pitfall 4: Empty Output vs No Sessions
**What goes wrong:** Confusing "no sessions" with command failure
**Why it happens:** abduco prints header even with no sessions
**How to avoid:** Check for empty output after parsing, not before
**Warning signs:** Nil slice when header-only output exists

### Pitfall 5: Status Character Position
**What goes wrong:** Assuming status is always first character
**Why it happens:** Status may have leading whitespace
**How to avoid:** Trim line before checking status, or use regex
**Warning signs:** All sessions appear detached

## Code Examples

### abduco Output Format (from source code)
```
// Source: https://github.com/martanne/abduco/blob/master/abduco.c
// printf("%c %s\t%jd\t%s\n", status, buf, (intmax_t)pid, namelist[n]->d_name);

Active sessions (on host localhost)
* Thu     2015-03-12 12:05:20    12345   ccc.rt1.main
+ Thu     2015-03-12 12:04:50    12346   ccc.rt1.2
  Thu     2015-03-12 12:03:30    12347   other-session
```

### Session Struct Definition
```go
// Source: project-specific design following tmux/sessions.go pattern
type Session struct {
    Name     string // full session name, e.g., "ccc.rt1.main"
    Project  string // "rt1" extracted from name; empty if External
    Suffix   string // "main" extracted from name; empty if External
    External bool   // true if not prefixed with "ccc."
    Dead     bool   // true if status is "+"
    PID      int    // process ID for kill command
}
```

### Parsing Regex
```go
// Source: derived from abduco output format analysis
// Line format: status timestamp\tPID\tname
// Example: "* Thu     2015-03-12 12:05:20\t12345\tccc.rt1.main"
var sessionLineRe = regexp.MustCompile(`^([*+ ])\s+\S+\s+\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}\t(\d+)\t(.+)$`)

func ParseSessionList(output string) []Session {
    var sessions []Session
    for _, line := range strings.Split(output, "\n") {
        matches := sessionLineRe.FindStringSubmatch(line)
        if len(matches) != 4 {
            continue
        }
        status := matches[1]
        pid, _ := strconv.Atoi(matches[2])
        name := matches[3]

        s := parseSessionName(name)
        s.Dead = status == "+"
        s.PID = pid
        sessions = append(sessions, s)
    }
    return sessions
}
```

### Session Name Parsing
```go
// Source: project-specific design following CONTEXT.md naming convention
func parseSessionName(name string) Session {
    // Format: ccc.{project}.{suffix}
    if !strings.HasPrefix(name, "ccc.") {
        return Session{Name: name, External: true}
    }

    parts := strings.SplitN(name[4:], ".", 2) // skip "ccc."
    if len(parts) != 2 {
        return Session{Name: name, External: true}
    }

    return Session{
        Name:    name,
        Project: parts[0],
        Suffix:  parts[1],
    }
}
```

### Integration Test Skip Pattern
```go
// Source: existing tmux/sessions_integration_test.go pattern
//go:build integration

package abduco_test

import (
    "os/exec"
    "testing"
)

func TestCreateSession(t *testing.T) {
    if _, err := exec.LookPath("abduco"); err != nil {
        t.Skip("abduco not installed, skipping integration test")
    }
    // ... test body
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| tmux metadata tags | Session name encoding | This migration | Simpler, no @ccc_project/@ccc_path |
| Multiple tmux clients | Auto-detach | This migration | No client negotiation needed |
| pkill for session kill | PID-based kill | User decision | Safer, more precise |

**Deprecated/outdated:**
- tmux passthrough configuration: Not needed with abduco (transparent PTY)
- Client detach logic: abduco auto-detaches previous client

## Open Questions

1. **Timestamp locale variations**
   - What we know: abduco uses strftime with locale-dependent day name
   - What's unclear: Whether all locales produce parseable output
   - Recommendation: Use tab delimiter as primary separator, ignore timestamp format

2. **Socket path customization**
   - What we know: abduco checks ABDUCO_SOCKET_DIR env var
   - What's unclear: Whether we need to support custom paths
   - Recommendation: Use defaults for now; add support if needed in v2

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing (stdlib) |
| Config file | None needed |
| Quick run command | `go test ./abduco/` |
| Full suite command | `go test ./abduco/ -tags=integration` |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SESS-01 | Create session with naming | integration | `go test ./abduco/ -tags=integration -run TestCreateSession` | Wave 0 |
| SESS-02 | List sessions with filter | unit | `go test ./abduco/ -run TestFilterSessionsForProject` | Wave 0 |
| SESS-03 | Attach to session | integration | `go test ./abduco/ -tags=integration -run TestAttach` | Wave 0 |
| SESS-04 | Kill session via PID | integration | `go test ./abduco/ -tags=integration -run TestKillSession` | Wave 0 |
| SESS-05 | External sessions marked | unit | `go test ./abduco/ -run TestParseExternalSessions` | Wave 0 |
| SESS-06 | Dead sessions handled | unit | `go test ./abduco/ -run TestParseDeadSessions` | Wave 0 |
| MIGR-01 | Replace tmux package | unit | `go test ./abduco/` | Wave 0 |
| ERRH-01 | Check command works | unit | `go test ./abduco/ -run TestBuildCheckCommand` | Wave 0 |
| ERRH-02 | Stderr redirect in list | unit | `go test ./abduco/ -run TestBuildListCommand` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./abduco/`
- **Per wave merge:** `go test ./abduco/ -tags=integration`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `abduco/sessions.go` - package file
- [ ] `abduco/sessions_test.go` - unit tests with mock output
- [ ] `abduco/sessions_integration_test.go` - integration tests with real abduco

## Sources

### Primary (HIGH confidence)
- [GitHub martanne/abduco source code](https://github.com/martanne/abduco) - abduco.c printf format line
- [GitHub martanne/abduco README](https://github.com/martanne/abduco/blob/master/README.md) - Status indicators and command syntax
- Existing `tmux/sessions.go` - Established patterns for command builders and parsers
- Existing `tmux/sessions_test.go` - Test structure patterns
- Existing `tmux/sessions_integration_test.go` - Integration test patterns

### Secondary (MEDIUM confidence)
- [Ubuntu Manpage abduco](https://manpages.ubuntu.com/manpages/xenial/man1/abduco.1.html) - Command line options
- [ManKier abduco](https://www.mankier.com/1/abduco) - Additional command documentation

### Tertiary (LOW confidence)
- None

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - Using existing project patterns and Go stdlib
- Architecture: HIGH - Following established tmux package patterns exactly
- Pitfalls: HIGH - Derived from source code analysis and project-specific decisions

**Research date:** 2026-03-10
**Valid until:** 2026-04-10 (abduco is stable, no recent changes)
