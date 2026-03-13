# Codebase Concerns

**Analysis Date:** 2025-02-25

## Error Handling Issues

**Silent Error Suppression:**
- Issue: Errors from `RunInteractive()` are not checked in scan workflows
- Files: `flow/scan.go` (line 57)
- Impact: If shell exits with error, the rescan still proceeds; user may not realize the shell failed
- Fix approach: Check `RunInteractive()` return value and propagate error or user-visible message

**Ignored Tmux Configuration Errors:**
- Issue: `runner.Run(tmux.BuildSetPassthroughCommand(name))` at `flow/common.go:261` and `runner.Run(tmux.BuildEnsureNotifyOptionsCommand(session.Name))` at `flow/common.go:236` silently ignore errors
- Files: `flow/common.go` (lines 236, 261)
- Impact: For tmux < 3.3, passthrough setup fails silently; sessions created by older ccc versions don't get the correct options on attach
- Current mitigation: Commands wrapped in 2>/dev/null already; errors are acceptable for older tmux versions
- Recommendations: Add debug-level logging to distinguish "expected failures" (old tmux) from actual problems

**Optional Uname Call:**
- Issue: `osInfo, _ := runner.Run("uname -s")` at `flow/errors.go:20` discards error; empty string treated as non-darwin
- Files: `flow/errors.go` (line 20)
- Impact: If `uname` fails (unlikely on Unix), install instructions default to multi-OS format; minor UX degradation
- Fix approach: Check error or provide fallback instructions

**Unhandled Shell Open Error:**
- Issue: `conn.RunInteractive("$SHELL -l")` at `flow/scan.go:57` return value is ignored
- Files: `flow/scan.go` (line 57)
- Impact: If shell fails to open (e.g., connection drops), no error feedback before "Rescanning..." message
- Fix approach: Check error and provide user feedback

**Client List Error Ignored:**
- Issue: `clientOutput, _ := runner.Run(tmux.BuildListClientsCommand(item.Key))` at `flow/common.go:268` discards error
- Files: `flow/common.go` (line 268)
- Impact: If tmux client list fails, empty list is treated as "no clients"; warning/error condition becomes silent
- Fix approach: Check error and log or warn

## Boundary Conditions & Array Access

**Unsafe Array Indexing:**
- Issue: `c := clients[0]` at `flow/common.go:211` accesses clients without bounds checking; previously validated with `if len(clients) > 0` which is correct, but pattern is fragile
- Files: `flow/common.go` (line 211)
- Impact: Currently safe due to length check, but fragile to future refactoring
- Safe modification: Already guarded by `if len(clients) > 0`; ensure this guard stays with the access

**Session Rename with Empty Suffix:**
- Issue: User can enter empty string for rename suffix at `flow/common.go:305-306`; defaults to "main" if empty, which is intentional
- Files: `flow/common.go` (lines 305-312)
- Impact: No risk; empty input is handled
- Current state: Correct behavior

## Input Validation Issues

**Scan Selection Parsing:**
- Issue: In `flow/scan.go:115-122`, comma-separated scan selection accepts malformed input; any non-integer is silently skipped
- Files: `flow/scan.go` (lines 115-122)
- Impact: User enters "1, 3, bad, 5" and silently omits "bad"; no feedback about invalid entries
- Fix approach: Validate and provide feedback on invalid entries before committing selection

**Project Name Collision Risk:**
- Issue: `scan.DeriveProjectKey(path)` at `flow/scan.go:111, 119` can produce collisions (e.g., "my-proj" and "my_proj" both become "my-proj")
- Files: `flow/scan.go` (lines 111, 119), `scan/projects.go` (lines 123-131)
- Impact: Silent deduplication; later project overwrites earlier one without warning
- Fix approach: Check for collisions before adding to projects map; warn user and offer rename

**Duplicate Fallback Address:**
- Issue: `offerAddFallback` at `flow/remote.go:213-218` checks for duplicates, but no early return after saving fallback
- Files: `flow/remote.go` (lines 224-231)
- Impact: Fallback is saved even if address is duplicate; check is only informational
- Fix approach: Actual implementation prevents duplicates (checks at line 213-218); UX could clarify this

## Resource Management

**Config Save Errors Reduce to Warnings:**
- Issue: Multiple `fmt.Fprintf(out, "  Warning: could not save config: %v\n", saveErr)` calls don't return errors
- Files: `flow/common.go` (lines 75, 103), `flow/remote.go` (lines 87, 228, 277), `flow/scan.go` (lines 276-278)
- Impact: User is notified but workflow continues; project/host deletions, scans may not persist
- Current behavior: In-memory state changes proceed; next session starts fresh if config wasn't saved
- Recommendations: Consider making persistence failures fatal for delete/scan operations (especially remote mode)

**Config Rollback on Failed Save:**
- Issue: `deleteProject` at `flow/common.go:338-361` implements rollback for failed saves, but other operations don't
- Files: `flow/common.go` (lines 338-361)
- Impact: Scan results and project selections may be in-memory-only if save fails; inconsistent behavior across operations
- Fix approach: Extend rollback pattern to scan operations (store original before modifying)

## Race Conditions & State Management

**Session Metadata Fallback:**
- Issue: `FilterSessionsForProject` at `tmux/sessions.go:174-201` matches untagged sessions by name prefix; migration from old ccc (untagged) to new (tagged) creates ambiguity
- Files: `tmux/sessions.go` (lines 174-201)
- Impact: If both tagged and prefix-matched sessions exist, results may include both; UI shows "(unverified)" warning
- Current state: Acceptable; verification status flags which sessions are tracked
- Recommendations: Document migration path; add FAQ about legacy session handling

**Concurrent Session Creation:**
- Issue: `NextAutoName` at `tmux/sessions.go:206-228` reads current sessions to avoid name collisions, but doesn't prevent concurrent creation
- Files: `tmux/sessions.go` (lines 206-228)
- Impact: If two ccc instances create sessions simultaneously, collision is possible
- Frequency: Very low (user would need two concurrent instances)
- Fix approach: Add collision detection at tmux-level if user reports issues; current approach acceptable for typical usage

## Security Considerations

**SSH Key Validation:**
- Risk: No validation of `IdentityFile` existence in `config/client.go` or `ssh/connection.go`
- Files: `config/client.go` (lines 44-52), `ssh/connection.go` (lines 59-60)
- Current mitigation: SSH validates at connection time; invalid keys fail with clear SSH error
- Recommendations: Validate at config load time with helpful error message

**TOFU SSH Trust:**
- Risk: `StrictHostKeyChecking=accept-new` at `ssh/connection.go:78` implements TOFU (trust-on-first-use)
- Files: `ssh/connection.go` (line 78)
- Current mitigation: SSH client validates against known_hosts; new hosts are accepted, changed keys rejected
- Acceptable: Standard TOFU model; documented in CLAUDE.md

**Fallback Address User Input:**
- Risk: User-entered fallback address at `flow/remote.go:204` is not sanitized before being passed to SSH
- Files: `flow/remote.go` (lines 204-206)
- Current mitigation: Address is validated via `TestConnection()` which uses SSH; invalid addresses fail gracefully
- Recommendations: Add basic validation (no spaces, valid hostname/IP pattern) before SSH attempt

**Home Directory Expansion:**
- Risk: `~/.ccc/projects.toml` read via `cat` at `flow/remote.go:144` relies on shell expansion
- Files: `flow/remote.go` (line 144)
- Current mitigation: Command is wrapped in `$SHELL -lc` which expands ~; remotely executed
- Acceptable: Standard shell behavior

## Test Coverage Gaps

**No E2E Testing for SSH Persistence:**
- What's not tested: Remote mode file I/O (projects.toml save/load on remote hosts)
- Files: `e2e_test.go` exists but may not cover full SSH+persistence path
- Risk: Changes to remote save logic could break silently for non-local users
- Priority: High - affects core workflow

**No Test for Concurrent User Input:**
- What's not tested: Behavior when user enters input during menu transitions
- Files: `ui/menu_test.go`, `ui/interactive_test.go`
- Risk: UI state corruption if input is buffered and replayed
- Priority: Medium - rare in practice but high impact if occurs

**Limited Scan Failure Testing:**
- What's not tested: Scan failure scenarios (timeout, no results, permission denied)
- Files: `flow/scan.go` has limited error path coverage in tests
- Risk: Unhandled error conditions in scan workflow
- Priority: Medium

**No Symlink Handling Test:**
- What's not tested: Projects that are symlinks, especially broken ones
- Files: `flow/common.go:91-108` checks path existence but may not handle symlinks
- Risk: Symlink to deleted target breaks project loading
- Priority: Low

## Fragile Areas

**UI Scanner/Reader Interaction:**
- Files: `ui/menu.go` (lines 73-107), `ui/interactive.go`
- Why fragile: `bufio.Scanner` in `ShowMenu` buffers the full reader; `Confirm`/`Prompt` called from within menu loop cannot read from same reader (get EOF)
- Safe modification: For line-based menu mode, document that subsequent UI calls must use fresh input or convert to interactive mode
- Current workaround: Existing code avoids nested reads from same scanner
- Test coverage: Sufficient for current usage patterns

**Tmux Option Compatibility:**
- Files: `tmux/sessions.go` (lines 88-111)
- Why fragile: Different tmux versions support different options; bell-action and allow-passthrough vary
- Safe modification: Add version detection before setting options; validate which options exist
- Current approach: Wrapped in 2>/dev/null; acceptable for optional features
- Test coverage: Limited; only tested with single tmux version

**Config File Format Migration:**
- Files: `config/client.go`, `config/projects.go`
- Why fragile: TOML format changes (adding fields) could break old configs
- Safe modification: Add version field to TOML; implement migration logic on load
- Current state: No version tracking; new fields silently ignored by old code
- Test coverage: Sufficient for current schema

## Performance Bottlenecks

**Initial Scan Performance:**
- Problem: Full filesystem scan (mdfind → plocate → fd → find) on first run can be slow
- Files: `scan/projects.go` (lines 25-49)
- Cause: Fallback chain tries multiple tools; find is very thorough
- Improvement path: Cache scan results; add timeout for long-running scans; allow user to specify scan paths

**Session Listing on Large Tmux Servers:**
- Problem: `BuildListCommand` at `tmux/sessions.go:49-51` lists all sessions every time user navigates menus
- Files: `tmux/sessions.go` (lines 49-51)
- Cause: No caching between menu renders
- Improvement path: Cache session list with short TTL; refresh on user action

**SSH Connection Overhead:**
- Problem: Each `runner.Run()` call establishes new SSH connection in remote mode
- Files: `ssh/connection.go` (lines 101-108)
- Cause: `exec.Command("ssh", ...)` doesn't reuse connections
- Improvement path: Implement SSH connection pooling or persistent session
- Current acceptable use: Typical workflows (scan, select project) are 5-10 commands; acceptable latency

## Dependencies at Risk

**Go-TOML Library:**
- Risk: `github.com/pelletier/go-toml/v2` is only external dependency for core functionality
- Impact: Version bump could break config parsing
- Migration plan: Library is stable; pin version in go.mod; test on updates

**golang.org/x/term for Terminal Control:**
- Risk: Terminal handling (raw mode, cursor movement) depends on x/term
- Impact: Terminal features could break with different terminal emulators
- Current mitigation: Fallback to line-based menu works on any input
- Recommendations: Test interactive mode on common terminals (iTerm, Terminal.app, Linux terminals)

## Missing Critical Features

**No Config Backup on First Run:**
- Problem: Initial config creation has no rollback; user cannot undo host setup
- Blocks: User recovery from mistyped host config
- Workaround: User can manually edit ~/.ccc/config.toml

**No Projects Sync Between Machines:**
- Problem: Projects are per-machine; no way to sync across multiple local machines using different hosts
- Blocks: Users managing projects on multiple remote machines must reconfigure each
- Complexity: Would require centralized config storage or push/pull mechanism

**No Session Recording/Logging:**
- Problem: No way to audit or record session creation/deletion
- Blocks: Teams cannot track who created/killed sessions
- Priority: Low - single-user tool

**No SSH Agent Integration:**
- Problem: Relies on SSH config file for key management; no explicit SSH agent handling
- Blocks: Subprocesses don't inherit SSH agent state in some shells
- Workaround: User configures SSH agent in shell profile
- Recommendations: Document SSH agent setup; verify agent state before SSH attempt

## Data Durability Issues

**Unconfirmed Project Writes:**
- Issue: Remote projects.toml writes use shell `printf` with `>` (overwrite)
- Files: `flow/remote.go:251`
- Risk: Partial write leaves empty/corrupted config; no atomic writes
- Fix approach: Write to temp file, then atomic rename; implement in remote save function

**Projects.toml Format Inconsistency:**
- Issue: TOML serialization may produce different formatting on resave; no canonicalization
- Files: `config/projects.go:32-34`
- Risk: Config diffs are noisy; hard to track actual changes
- Fix approach: Use TOML library with consistent formatting; test serialization roundtrip

## Scaling Limits

**Memory Usage with Large Project Sets:**
- Current capacity: Projects loaded entirely into memory; no streaming
- Limit: ~1000+ projects becomes noticeable (map lookups still O(1), UI rendering O(n))
- Scaling path: Implement project filtering/search; lazy load from disk

**Session List Size:**
- Current capacity: All sessions listed in menu; ~100+ sessions becomes unwieldy
- Limit: Terminal rendering slows; UI becomes hard to navigate
- Scaling path: Add project filtering by prefix; add search/filter mode

**Scan Time for Large Filesystems:**
- Current capacity: Scan works on typical home directories (~10k-100k files)
- Limit: Filesystems with millions of files timeout or hang
- Scaling path: Add scan timeout; limit scan depth; use incremental/background scanning

## Known Bugs

**File Descriptor Leak on TTY Detection Failure:**
- Symptoms: Repeated failures on non-TTY input may leak file descriptors
- Files: `ui/menu.go:64-66`
- Trigger: Call ShowMenu with os.File that fails term.IsTerminal() check
- Workaround: Fallback to line-based menu works correctly
- Current state: Not a practical issue; only happens in non-TTY contexts where line-based menu is fine

**Unverified Session Warning Only on Attach:**
- Symptoms: User sees unverified warning only when attaching; not in session list
- Files: `flow/common.go:196-202`
- Trigger: Create session outside ccc, then try to attach via ccc
- Impact: User may not realize session is untracked until final attach attempt
- Fix: Show verification status in session menu list (already done in `flow/common.go:140-143`)
- Current state: Actually fixed; warning shows both in menu and on attach

---

*Concerns audit: 2025-02-25*
