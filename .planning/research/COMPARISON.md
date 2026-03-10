# Comparison: abduco vs dtach vs tmux vs screen

**Context:** Choosing a session persistence backend for ccc CLI tool
**Recommendation:** abduco because it provides session listing + transparent PTY passthrough, which is exactly what ccc needs and nothing more.

## Quick Comparison

| Criterion | abduco | dtach | tmux | screen |
|-----------|--------|-------|------|--------|
| Session persistence | Yes | Yes | Yes | Yes |
| Transparent PTY | Yes | Yes | No (intercepts) | No (intercepts) |
| Session listing | Built-in | None | Built-in | Built-in |
| Kill command | None | None | Built-in | Built-in |
| Multi-client | No (auto-detach) | Yes | Yes | Yes |
| Multiplexing/splits | No | No | Yes | Yes |
| Metadata/tags | None | None | User options | None |
| Preinstalled | Never | Never | Often | Often |
| Package availability | Good | Good | Excellent | Excellent |
| Binary size | ~30KB | ~20KB | ~1MB | ~1MB |
| Maintenance | Stable (low activity) | Stable (low activity) | Very active | Minimal |
| OSC passthrough | Native | Native | Requires config (3.3+) | Requires config |
| Scrollback | Terminal native | Terminal native | tmux managed | screen managed |
| License | ISC | GPL-2.0 | ISC | GPL-3.0 |

## Detailed Analysis

### abduco

**Strengths:**
- Transparent PTY passthrough -- all terminal features work natively
- Built-in session listing (dtach lacks this)
- Auto-detaches previous client -- eliminates client negotiation entirely
- Clean, small codebase (~2000 lines of C)
- Does exactly one thing: session persistence

**Weaknesses:**
- No native kill command (must use pkill workaround)
- No session metadata (must encode in name)
- No rename command
- Not preinstalled anywhere
- Low maintenance activity (but also stable, feature-complete)

**Best for:** Tools that need session persistence without multiplexing. Exactly ccc's use case.

### dtach

**Strengths:**
- Even smaller than abduco
- Transparent PTY passthrough
- Established (older project)

**Weaknesses:**
- No session listing -- must scan socket directory manually
- Socket directory location not standardized
- Less actively maintained than abduco
- GPL-2.0 license (vs abduco's permissive ISC)

**Best for:** Embedding in scripts where you manage session tracking yourself.

### tmux

**Strengths:**
- Rich feature set (multiplexing, splits, scripting)
- Very actively maintained
- Often preinstalled
- Native kill, rename, metadata, client management
- Huge ecosystem of plugins and configurations

**Weaknesses:**
- Intercepts terminal I/O (blocks OSC sequences without config)
- Multi-client sizing negotiation adds complexity
- Requires passthrough/bell configuration for notification forwarding
- ~125 lines of workaround code in current ccc implementation

**Best for:** Power users who want terminal multiplexing. Not ideal when you only need persistence.

### screen

**Strengths:**
- Venerable, widely available
- Feature-rich

**Weaknesses:**
- Security concerns (historically required SUID)
- Poor terminal passthrough
- Effectively deprecated in favor of tmux
- Complex configuration
- Minimal active development

**Best for:** Legacy systems where nothing else is available. Not recommended for new projects.

## Recommendation

**Choose abduco** for ccc because:

1. ccc uses tmux solely for session persistence, not multiplexing. abduco provides exactly this.
2. Transparent PTY eliminates ~125 lines of workaround code and gives users native terminal behavior.
3. Auto-client-detach eliminates the client negotiation flow entirely.
4. Built-in session listing (absent from dtach) means less custom code.

The two weaknesses (no kill command, no metadata) are manageable: kill via pkill with anchored regex, metadata via naming convention. These are simpler than the tmux workarounds they replace.

**Choose tmux when:** You need terminal multiplexing (splits, windows) or session scripting. Not the case for ccc.
**Choose dtach when:** You need absolute minimal dependency and will manage session tracking yourself.
**Choose screen when:** Never for new projects.

## Sources

- [abduco GitHub](https://github.com/martanne/abduco)
- [dtach GitHub](https://github.com/crigler/dtach)
- [abduco man page](https://man.archlinux.org/man/abduco.1.en)
- [abduco vs tmux - SaaSHub](https://www.saashub.com/compare-tmux-vs-abduco-plus-dvtm)
