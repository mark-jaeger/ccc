# Phase 1: abduco-package - Context

**Gathered:** 2026-03-10
**Status:** Ready for planning

<domain>
## Phase Boundary

Create `abduco/` package with command builders, output parser, and session management. This is a Go package that mirrors the existing tmux package patterns but simplified for abduco's transparent PTY model.

</domain>

<decisions>
## Implementation Decisions

### Session naming
- Pattern: `ccc.{project}.{suffix}`
- First session gets suffix "main" (e.g., `ccc.rt1.main`)
- Subsequent sessions get numbered suffixes: 2, 3, 4...
- Prompt user for custom suffix, default to auto-generated

### Dead session handling
- Show dead sessions (abduco `+` status) in the list with `(dead)` marker
- User can see them and manually kill if desired
- No auto-cleaning — transparent, no surprises

### External session display
- Non-ccc abduco sessions shown with `(external)` marker
- Allows user visibility into all abduco sessions on the machine

### Kill implementation
- Extract PID from abduco list output
- Kill via PID, not pkill (pkill deemed dangerous — can match too broadly)

### Error messaging
- Short error with platform-specific install hints
- macOS: `brew install abduco`
- Ubuntu/Debian: `apt install abduco`
- Example: `abduco not found. Install: brew install abduco (macOS) or apt install abduco (Debian/Ubuntu)`

### Claude's Discretion
- Exact regex pattern for parsing abduco output
- Test structure and mock data format
- Integration test skip logic when abduco not installed

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/shellutil.Quote()`: Use for all user-controlled values in shell commands
- `tmux/sessions.go`: Pattern to follow for `Build*Command()` and `Parse*()` functions

### Established Patterns
- Command builders return shell strings, not execute commands
- Parsers take string output, return typed structs
- `Filter*ForProject()` functions handle project-based filtering
- Unit tests use mock output strings; integration tests use real binary

### Integration Points
- `flow/common.go` will import `abduco` instead of `tmux` (Phase 2)
- `Runner` interface executes commands — package builds commands only

</code_context>

<specifics>
## Specific Ideas

- Follow the tmux package structure closely — same function naming patterns
- abduco output goes to stderr, so commands need `2>&1` redirect
- Session struct should have: Name, Project, Suffix, External, Dead fields

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 01-abduco-package*
*Context gathered: 2026-03-10*
