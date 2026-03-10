# Research Summary: ccc abduco migration

**Domain:** Terminal session persistence CLI
**Researched:** 2026-03-10
**Overall confidence:** HIGH

## Executive Summary

The abduco migration is well-scoped and technically straightforward. abduco (v0.6) is a stable, lightweight terminal session manager that provides exactly the session persistence ccc needs without the multiplexing overhead of tmux. The existing codebase architecture -- particularly the `Runner` interface and command-builder pattern -- makes this a clean package swap rather than an architecture change.

The most critical finding is that the migration doc (`docs/abduco-migration.md`) has the **wrong output format** for session parsing. The actual abduco output is tab-delimited with status char, timestamp, PID, and session name -- not the parenthesized format shown in the doc. The parser must be rewritten from scratch against real abduco output.

The second critical finding is that session kill should use **PID extraction from list output** rather than `pkill -f` pattern matching. abduco's list output includes the server PID for each session, making precise `kill <pid>` possible and safe.

The migration eliminates approximately 125 lines of tmux workaround code (client negotiation, passthrough configuration, bell configuration, metadata tagging) and gains native terminal passthrough for free. This is a net simplification: fewer features, less code, better terminal behavior.

## Key Findings

**Stack:** abduco 0.6 as sole session backend. No new Go dependencies. Shell command strings through existing Runner interface.
**Architecture:** Package swap (`tmux/` -> `abduco/`). No architectural changes needed. Command-builder pattern preserved.
**Critical pitfall:** Migration doc's parser is wrong (output format mismatch). Kill approach should use PID from list output, not pkill.

## Implications for Roadmap

Based on research, suggested phase structure:

1. **Create abduco package** - Build `abduco/sessions.go` with command builders, output parser, and comprehensive tests
   - Addresses: Session create, attach, list, kill, naming convention, dead session handling
   - Avoids: Output format mismatch (test against real abduco output), kill reliability (use PID extraction)
   - Note: Parser must handle status chars (*/+/space), tab-separated PID, and session name

2. **Migrate flow layer** - Update `flow/common.go` and `flow/setup.go` to use abduco package
   - Addresses: Simplified attach flow, remove client negotiation, remove passthrough config
   - Avoids: Stderr parsing pitfall (verify Runner captures stderr with 2>&1 redirect)
   - Avoids: SSH blank screen with -A flag (use explicit -n/-a two-step)

3. **Remove tmux package and clean up** - Delete `tmux/`, update tests, add migration notice
   - Addresses: Breaking change communication, clean codebase
   - Avoids: Breaking change pitfall (one-time notice for upgrading users)

4. **Integration testing and docs** - Test over SSH, update CLAUDE.md and README
   - Addresses: SSH PTY interaction, install instructions, TERM variable normalization
   - Avoids: Remote host install pitfall (actionable error messages)

**Phase ordering rationale:**
- Phase 1 is independent and testable in isolation (mock Runner tests + real abduco integration tests)
- Phase 2 depends on phase 1 (needs abduco package to exist)
- Phase 3 depends on phase 2 (cannot remove tmux until flow is migrated)
- Phase 4 can partially overlap with phase 3

**Research flags for phases:**
- Phase 1: Needs integration testing with real abduco binary to verify output format and PID extraction
- Phase 2: Standard refactoring, unlikely to need additional research
- Phase 3: Standard cleanup, unlikely to need additional research
- Phase 4: May need research on SSH PTY allocation specifics if issues arise

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | abduco is well-documented, stable, widely packaged. Command syntax verified against man pages and GitHub. |
| Features | HIGH | Feature set is a strict subset of current tmux features. Well-defined in migration doc. |
| Architecture | HIGH | Existing Runner interface and command-builder pattern make this a clean swap. No unknowns. |
| Pitfalls | MEDIUM-HIGH | Output format, kill approach, and SSH issues verified via GitHub issues and docs. TERM mismatch and dead session handling need hands-on testing. |

## Gaps to Address

- **Output format verification:** The ARCHITECTURE.md documents the format from abduco README, but should be verified against current abduco 0.6 on both macOS and Linux during implementation.
- **SSH + abduco -a interaction:** Needs hands-on testing. The existing `RunInteractive` uses `-t` for PTY, which should work, but should be verified with real abduco over SSH.
- **Dots in project names:** The naming convention `ccc.{project}.{suffix}` breaks if project keys contain dots. Need to decide: validate no dots in project keys, or use a different delimiter.
- **Zombie session cleanup:** When abduco sessions crash, sockets linger (status `+`). Need to decide if ccc should auto-clean on list, or leave to user.
- **TERM normalization:** Should ccc set `TERM=xterm-256color` on session creation? Needs testing across terminal emulators.
