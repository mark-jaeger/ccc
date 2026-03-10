# Phase 2: flow-migration - Research

**Researched:** 2026-03-10
**Domain:** Go flow package refactoring, tmux-to-abduco migration
**Confidence:** HIGH

## Summary

This phase migrates the `flow/` package from using `tmux/` to using `abduco/` for session management. The migration involves replacing imports, updating function calls to match abduco's simpler API, and removing tmux-specific concepts like client negotiation, passthrough configuration, and metadata verification.

The abduco package (created in Phase 1) already provides all required functionality: `BuildCreateCommand`, `BuildAttachCommand`, `BuildListCommand`, `BuildKillCommand`, `BuildCheckCommand`, `ParseSessionList`, `FilterSessionsForProject`, and `NextAutoName`. The key simplifications are:
- **No client negotiation**: abduco auto-detaches previous clients
- **No passthrough config**: abduco has transparent PTY passthrough by default
- **No metadata verification**: session names encode project info directly (`ccc.{project}.{suffix}`)
- **PID-based kill**: `BuildKillCommand(pid int)` instead of `BuildKillCommand(name string)`

**Primary recommendation:** Systematic file-by-file migration following the pattern: change import, update function signatures, remove unused tmux-specific code, update tests.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| MIGR-02 | Remove client negotiation from attach flow | Delete `attachSession`'s client list/detach logic (lines 205-233 in flow/common.go) |
| MIGR-03 | Remove passthrough/bell configuration | Delete `BuildSetPassthroughCommand` call (line 261) and `BuildEnsureNotifyOptionsCommand` call (line 236) |
| MIGR-04 | Update flow layer to use abduco commands | Change import from `tmux` to `abduco`, update all function calls |
| MIGR-05 | Delete tmux package after migration complete | `rm -rf tmux/` after flow migration verified |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `abduco` package | local | Session management commands | Created in Phase 1, replaces tmux package |
| Go stdlib | N/A | All other functionality | Existing code patterns |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `testing` | N/A | Unit tests | Update existing flow tests |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Full rewrite | Incremental migration | Incremental is lower risk, easier to verify |

**Installation:**
```bash
# No new dependencies - using existing abduco package
```

## Architecture Patterns

### Recommended Migration Structure
```
flow/
    common.go        # Update import tmux -> abduco
    common_test.go   # Update mock responses, remove tmux-specific tests
    errors.go        # Rename CheckTmux -> CheckAbduco, change import
    errors_test.go   # Update test name and mock responses
    setup.go         # No changes needed (doesn't use tmux)
    local.go         # No changes needed (doesn't use tmux)
    remote.go        # No changes needed (doesn't use tmux directly)
    scan.go          # No changes needed (doesn't use tmux)

tmux/              # DELETE entire directory after migration
```

### Pattern 1: Import Replacement
**What:** Change import statement from tmux to abduco
**When to use:** Every file that uses tmux package
**Example:**
```go
// Before
import (
    "github.com/mark-jaeger/ccc/tmux"
)

// After
import (
    "github.com/mark-jaeger/ccc/abduco"
)
```

### Pattern 2: Simplified Attach Flow
**What:** Remove client negotiation, direct attach
**When to use:** `attachSession` function
**Example:**
```go
// Before (flow/common.go lines 195-240)
func attachSession(in io.Reader, out io.Writer, runner Runner, session tmux.Session) error {
    if !session.Verified {
        // ... unverified warning prompt
    }
    // Check for other clients
    clientOutput, clientErr := runner.Run(tmux.BuildListClientsCommand(session.Name))
    // ... 30+ lines of client negotiation
    // Ensure bell/passthrough options
    runner.Run(tmux.BuildEnsureNotifyOptionsCommand(session.Name))
    // ...
}

// After
func attachSession(in io.Reader, out io.Writer, runner Runner, session abduco.Session) error {
    if session.External {
        fmt.Fprintf(out, "\n  Session %q is an external session (not created by ccc).\n", session.Name)
        answer, err := ui.Confirm(in, out, "Proceed?")
        if err != nil || !answer {
            return err
        }
    }
    if session.Dead {
        fmt.Fprintf(out, "\n  Session %q is dead.\n", session.Name)
        return nil // or offer to kill/remove
    }
    fmt.Fprintf(out, "\n  Attaching to %s...\n", session.Name)
    return runner.RunInteractive(abduco.BuildAttachCommand(session.Name))
}
```

### Pattern 3: Session Struct Field Mapping
**What:** Map tmux.Session fields to abduco.Session fields
**Example:**
```go
// tmux.Session         -> abduco.Session
// .Name                -> .Name
// .Project             -> .Project
// .Path                -> (removed - not stored in abduco)
// .Windows             -> (removed - abduco is single-window)
// .Verified            -> .External (inverted semantics)
//                      -> .Suffix (new)
//                      -> .Dead (new)
//                      -> .PID (new)
```

### Pattern 4: Kill Session with PID
**What:** Use PID from session struct instead of name
**When to use:** `killSession` function
**Example:**
```go
// Before
killCmd := tmux.BuildKillCommand(item.Key)

// After - need to find session to get PID
var sessionPID int
for _, s := range sessions {
    if s.Name == item.Key {
        sessionPID = s.PID
        break
    }
}
killCmd := abduco.BuildKillCommand(sessionPID)
```

### Anti-Patterns to Avoid
- **Keeping tmux imports**: Remove ALL tmux imports, don't mix packages
- **Preserving unused concepts**: Don't keep Windows, Verified, Path fields
- **Half-migration**: Complete the migration in one phase; don't leave hybrid state

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Session parsing | Custom abduco parser | `abduco.ParseSessionList` | Already implemented in Phase 1 |
| Shell quoting | Custom escape logic | `shellutil.Quote()` | Already used by abduco package |
| Session naming | Custom naming logic | `abduco.NextAutoName` | Already implemented in Phase 1 |

**Key insight:** The abduco package already handles everything the tmux package did, just with a simpler API. This phase is about wiring, not logic.

## Common Pitfalls

### Pitfall 1: Forgetting to Update Test Mocks
**What goes wrong:** Tests fail because mock responses still use tmux format
**Why it happens:** Mock responses in flow/common_test.go are keyed by command substrings
**How to avoid:** Update all mock patterns: "tmux" -> "abduco", adjust expected output format
**Warning signs:** "unexpected command" errors in tests

### Pitfall 2: Missing PID Lookup for Kill
**What goes wrong:** Can't call BuildKillCommand without PID
**Why it happens:** tmux uses name-based kill, abduco uses PID-based kill
**How to avoid:** Pass full session list to killSession, lookup PID by name
**Warning signs:** Compile errors about wrong argument type

### Pitfall 3: Verified vs External Logic Inversion
**What goes wrong:** Session warnings appear when they shouldn't
**Why it happens:** tmux uses Verified=true for known sessions, abduco uses External=false
**How to avoid:** Replace `!session.Verified` with `session.External`
**Warning signs:** Every ccc session triggers "not created by ccc" warning

### Pitfall 4: Session Name Format Change
**What goes wrong:** Session filtering stops working
**Why it happens:** tmux uses "projectKey" / "projectKey-suffix", abduco uses "ccc.project.suffix"
**How to avoid:** Use abduco.NextAutoName, don't modify session names in flow layer
**Warning signs:** Sessions not appearing in filtered list

### Pitfall 5: Rename Session Feature
**What goes wrong:** Trying to keep rename feature that abduco doesn't support
**Why it happens:** tmux supports rename-session, abduco doesn't
**How to avoid:** Remove "r" rename menu option from SessionFlow entirely
**Warning signs:** User decision already marked rename as out of scope

## Code Examples

### Updated SessionFlow Session Display
```go
// Source: flow/common.go adaptation
items := make([]ui.MenuItem, len(sessions))
for i, s := range sessions {
    extra := ""
    if s.Dead {
        extra = "(dead)"
    }
    if s.External {
        extra = "(external)"
    }
    items[i] = ui.MenuItem{Key: s.Name, Label: s.Name, Extra: extra}
}
```

### Updated createSession Function
```go
// Source: flow/common.go adaptation
func createSession(in io.Reader, out io.Writer, runner Runner, projectKey, projectPath string, existing []abduco.Session) error {
    autoName := abduco.NextAutoName(projectKey, existing)
    // Note: autoName is already "ccc.{project}.{suffix}", no more prompt needed
    // User decided: first session gets "main", subsequent get 2, 3, 4...

    createCmd := abduco.BuildCreateCommand(autoName, projectPath)
    if _, err := runner.Run(createCmd); err != nil {
        return fmt.Errorf("failed to create session: %w", err)
    }
    // No passthrough command needed - abduco has transparent PTY

    fmt.Fprintf(out, "  Created session %s\n", autoName)
    return runner.RunInteractive(abduco.BuildAttachCommand(autoName))
}
```

### Updated CheckAbduco Function
```go
// Source: flow/errors.go adaptation
func CheckAbduco(in io.Reader, out io.Writer, runner Runner) error {
    result, err := runner.Run(abduco.BuildCheckCommand())
    if err == nil && strings.TrimSpace(result) != "" {
        return nil // abduco found
    }

    // Detect OS for install instructions
    osInfo, _ := runner.Run("uname -s")
    osInfo = strings.TrimSpace(strings.ToLower(osInfo))

    fmt.Fprintf(out, "\n  abduco not found.\n\n")
    fmt.Fprintf(out, "  Install abduco:\n")

    if strings.Contains(osInfo, "darwin") {
        fmt.Fprintf(out, "    brew install abduco\n")
    } else {
        fmt.Fprintf(out, "    macOS:   brew install abduco\n")
        fmt.Fprintf(out, "    Ubuntu:  sudo apt install abduco\n")
        fmt.Fprintf(out, "    Fedora:  sudo dnf install abduco\n")
        fmt.Fprintf(out, "    Arch:    sudo pacman -S abduco\n")
    }

    fmt.Fprintf(out, "\n  Opening shell so you can install it...\n")
    if err := runner.RunInteractive("$SHELL -l"); err != nil {
        return fmt.Errorf("shell failed: %w", err)
    }

    // Recheck
    fmt.Fprintf(out, "\n  Rechecking... ")
    result, err = runner.Run(abduco.BuildCheckCommand())
    if err != nil || strings.TrimSpace(result) == "" {
        fmt.Fprintf(out, "abduco still not found.\n")
        return fmt.Errorf("abduco not installed")
    }
    fmt.Fprintf(out, "abduco found.\n")
    return nil
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| tmux metadata tags | Session name encoding | This migration | No @ccc_project/@ccc_path |
| Multiple clients + negotiation | Auto-detach | This migration | No client listing/detaching |
| Passthrough config | Transparent PTY | This migration | No SetPassthrough/EnsureNotify |
| Name-based kill | PID-based kill | This migration | Need to track PID |
| Session rename | Not supported | This migration | Remove rename menu option |

**Deprecated/outdated:**
- `tmux/` package: Delete entirely after migration
- Client handling: `ParseClientList`, `BuildListClientsCommand`, `BuildDetachClientsCommand`
- Passthrough: `BuildSetPassthroughCommand`, `BuildEnsureNotifyOptionsCommand`
- Rename: `BuildRenameCommand`, `renameSession` function

## Open Questions

1. **Session name customization**
   - What we know: User decision says first session gets "main", subsequent get 2, 3, 4...
   - What's unclear: Should we still prompt for custom names?
   - Recommendation: Remove custom name prompt, use auto-naming only (simpler UX)

2. **Dead session handling**
   - What we know: abduco marks dead sessions with "+" status
   - What's unclear: Should we auto-clean dead sessions or just show them?
   - Recommendation: Show with "(dead)" marker per user decision (SESS-06), no auto-clean

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing (stdlib) |
| Config file | None needed |
| Quick run command | `go test ./flow/` |
| Full suite command | `go test ./...` |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| MIGR-02 | No client negotiation in attach | unit | `go test ./flow/ -run TestAttachSession` | Update existing |
| MIGR-03 | No passthrough config | unit | `go test ./flow/ -run TestCreateSession` | Update existing |
| MIGR-04 | Flow uses abduco commands | unit | `go test ./flow/ -run TestSessionFlow` | Update existing |
| MIGR-05 | tmux package deleted | manual | `test ! -d tmux/` | N/A |

### Sampling Rate
- **Per task commit:** `go test ./flow/`
- **Per wave merge:** `go test ./...`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
None - existing test infrastructure covers all phase requirements. Tests will be updated in place.

## Sources

### Primary (HIGH confidence)
- `abduco/sessions.go` - Phase 1 implementation, verified API
- `flow/common.go` - Current tmux-based implementation to migrate
- `flow/common_test.go` - Current tests to update
- `flow/errors.go` - CheckTmux function to rename
- `.planning/REQUIREMENTS.md` - User decisions on session naming and features

### Secondary (MEDIUM confidence)
- `.planning/phases/01-abduco-package/01-RESEARCH.md` - Phase 1 research for abduco patterns

### Tertiary (LOW confidence)
- None

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - Using existing abduco package from Phase 1
- Architecture: HIGH - Clear mapping from tmux to abduco functions
- Pitfalls: HIGH - Derived from direct code analysis of differences

**Research date:** 2026-03-10
**Valid until:** 2026-04-10 (migration is one-time, patterns stable)
