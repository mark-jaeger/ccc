# TUI Testing Frameworks: The State of the Art (February 2026)

The terminal UI testing landscape has matured significantly but unevenly over the past year. The original promise of Microsoft's tui-test as "Playwright for terminals" has not fully materialized — it remains pre-release — while **Textual's built-in testing** continues to set the gold standard with rapid iteration (now at v7.5.0). Meanwhile, Bubbletea v2 nears stable release, Ratatui hit a major milestone with v0.30.0 and no_std support, and the emergence of **OpenTUI** signals a new wave of TypeScript-native TUI frameworks.

The dominant paradigm remains **snapshot/golden file testing**, with framework-specific solutions consistently outperforming generic tools. Real PTY-based testing is gaining traction for integration testing, and the ecosystem continues to fragment along language lines rather than converging on a single cross-platform solution.

---

## Microsoft tui-test: Still Waiting for 1.0

**@microsoft/tui-test** remains pre-release at **v0.0.1-rc.5** (released March 11, 2025), with 74 GitHub stars (up from ~48 in late 2024). While Microsoft continues to maintain the project — adding xonsh shell support, migrating to @xterm/headless, introducing terminal trace support, configurable worker counts, and snapshot locking fixes — there has been no significant release activity since December 2024 (dependency updates only).

```typescript
import { test, expect } from "@microsoft/tui-test";

test.use({ program: { file: "git" } });
test("git shows usage message", async ({ terminal }) => {
  await expect(terminal.getByText("usage: git", { full: true })).toBeVisible();
  await expect(terminal).toMatchSnapshot();
});
```

The Playwright-like API design (auto-wait, tracing, test isolation via separate PTYs, regex assertions) remains compelling, but adoption has been minimal. The framework has not achieved the "Playwright for terminals" status anticipated in 2024–2025. Teams evaluating it should treat it as experimental and plan for potential API changes.

**Status**: Pre-release, experimental. Not recommended for production testing pipelines yet.

---

## Python/Textual: The Gold Standard

Textual by Textualize has had an extraordinary development velocity, progressing from v3.x through **v7.5.0** (January 30, 2026) with major version bumps in rapid succession: v4.0.0 (July 2025), v5.0.0 (July 2025), v6.0.0 (August 2025), and v7.0.0 (late 2025). It now carries a **Production/Stable** (PyPI status 5) designation and supports Python 3.9–3.14.

The **Pilot API** remains the most complete built-in TUI testing solution across any ecosystem:

```python
async def test_calculator(snap_compare):
    app = CalculatorApp()
    async with app.run_test() as pilot:
        await pilot.press("5", "+", "3", "=")
        await pilot.click("#clear-button")  # CSS selector-based
        await pilot.hover("#number-5")
        assert app.query_one("#display").value == "8"
```

Key testing capabilities include CSS selector-based element selection, headless mode execution, keyboard and mouse simulation, configurable terminal dimensions, and the companion **pytest-textual-snapshot** plugin for SVG-based visual regression testing with HTML diff reports.

Recent additions like the **pointer rule** (v7.4.0, January 25, 2026) for mouse cursor styling reflect continued investment in UI fidelity that directly benefits testing accuracy.

**Status**: Production-ready, most complete testing story. The clear recommendation for Python TUI projects.

---

## Go/Bubbletea: v2 Approaching Stable

Bubbletea has reached the **v2 RC phase** with v2.0.0-rc.2, representing a major architectural evolution. The module path has changed from `github.com/charmbracelet/bubbletea/v2` to `charm.land/bubbletea/v2`, and several message types (`CursorPositionMsg`, `KeyboardEnhancementsMsg`, `PasteMsg`, `CapabilityMsg`) are now structs rather than type aliases — breaking changes to be aware of. The v2.0.0 milestone is **77% complete** (6 open issues, 21 closed), and rc.2 includes **Mode 2026 (synchronized output)** support.

Bubbletea v1 remains at ~38,900 stars, firmly established as the dominant Go TUI framework.

**teatest** remains in `github.com/charmbracelet/x/exp/teatest` — still experimental, last published October 15, 2025, with no signs of graduation to stable status:

```go
func TestFullOutput(t *testing.T) {
    m := initialModel()
    tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(300, 100))
    tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
    out, _ := io.ReadAll(tm.FinalOutput(t))
    teatest.RequireEqualOutput(t, out)  // Compares against testdata/*.golden
}
```

Complementary tools in the Charm ecosystem include **catwalk** (556 stars, data-driven testing), **VHS** (still excellent for documentation-as-testing), and **sequin** (human-readable ANSI sequence inspection, useful for debugging golden file diffs).

**Status**: teatest experimental. Bubbletea v2 nearing stable. Golden file testing remains the standard approach.

---

## Rust/Ratatui: v0.30.0 and Modular Architecture

Ratatui achieved a major milestone with **v0.30.0** — described by maintainers as "the biggest release of ratatui so far" — with **11.9M+ total downloads**. The headline features are **no_std support** (enabling use on embedded targets like Cortex-M microcontrollers) and a reorganization into a modular workspace architecture with separate crates: `ratatui` (main), `ratatui-core` (for widget library authors), and others.

**TestBackend** remains the primary unit testing tool, paired with the **insta** snapshot testing crate:

```rust
#[test]
fn test_render_app() {
    let mut terminal = Terminal::new(TestBackend::new(80, 20)).unwrap();
    terminal.draw(|frame| frame.render_widget(&app, frame.area())).unwrap();
    assert_snapshot!(terminal.backend());
}
```

For integration testing, **ratatui-testlib** (available on crates.io) provides PTY-based testing with real terminal emulation, Sixel graphics testing support, Bevy ECS integration, Tokio async/await support, and insta snapshot integration. The recommended approach is now clear: **TestBackend for unit tests, ratatui-testlib for integration tests**.

Additional ecosystem improvements include Crossterm version feature flags (`crossterm_0_28`, `crossterm_0_29`) and the **ratatui-wasm** project enabling browser-based Ratatui rendering via WebAssembly — opening new testing possibilities through DOM/Canvas/WebGL backends.

**Status**: Production-ready. Mature unit testing with TestBackend + insta. Growing integration testing story with ratatui-testlib.

---

## JavaScript/Ink: Stable but Stagnant

**ink-testing-library** sits at v4.0.0 (May 2024) with **3,300+ dependent projects**, making it the most widely adopted JavaScript TUI testing solution by usage. However, there have been no significant updates through 2025 or early 2026. The library provides React Testing Library–style patterns (`lastFrame()` assertions, stdin simulation, snapshot support) that remain effective for Ink-based applications.

**Status**: Stable, production-ready, but no active development. Adequate for existing Ink projects.

---

## Emerging: OpenTUI

**@opentui/core** is a new TypeScript TUI framework by the creators of OpenCode (94k stars) and terminal.shop, currently at **v0.1.75** (January 25, 2026) with **8,000 GitHub stars** and 330 forks. Development is extremely active with releases every few days.

Key technical differentiators include React and SolidJS reconcilers, a native **Zig rendering backend**, Go bindings (`packages/go`), and a built-in console overlay for debugging. OpenTUI serves as the foundational framework for OpenCode, one of the most popular terminal-based AI coding tools.

There is no dedicated testing framework yet, though the project includes test infrastructure. Some Windows compatibility issues remain (reported January 2026). Given the pace of development and the backing of a well-funded team, a testing story is likely to emerge, but it's too early to evaluate.

**Status**: Development phase. Not production-ready. Worth watching as a potential future platform.

---

## Cross-Platform and Legacy Approaches

For teams working outside these framework ecosystems — or testing arbitrary terminal applications — the established approaches remain viable:

**expect/pexpect** continues as the most portable option for spawning and interacting with any terminal application. **tmux-based testing** (using `send-keys` / `capture-pane`) provides a middle ground with no framework dependency. **Architecture-level separation** — extracting business logic from terminal I/O — remains the most reliable testing strategy regardless of framework choice.

**Microsoft's tui-test** was supposed to fill the "test any terminal app" niche but hasn't reached the adoption or stability needed to serve as a reliable cross-platform solution.

---

## Ecosystem Comparison (February 2026)

| Language | Framework | Testing Solution | Maturity | Latest Version |
|----------|-----------|-----------------|----------|----------------|
| Python | Textual | Pilot API + pytest-textual-snapshot | **Production** | v7.5.0 (Jan 2026) |
| Python | Any | pexpect | Mature | Stable |
| Go | Bubbletea | teatest + golden files | Experimental | v2.0.0-rc.2 |
| Go | tcell/tview | SimulationScreen | Mature | Stable |
| Rust | Ratatui | TestBackend + insta (unit), ratatui-testlib (integration) | **Production** | v0.30.0 |
| Node.js | Ink | ink-testing-library | Stable (stagnant) | v4.0.0 (May 2024) |
| TypeScript | OpenTUI | (none yet) | Pre-alpha | v0.1.75 (Jan 2026) |
| Any | Any | @microsoft/tui-test | Pre-release | v0.0.1-rc.5 (Mar 2025) |
| Any | Any | tmux + expect | Mature | Stable |

---

## Key Trends and Takeaways

**Snapshot testing dominance**: Golden file / snapshot testing has won as the primary TUI testing paradigm across all ecosystems. The complexity of styled terminal output makes assertion-based approaches impractical at scale.

**Framework-specific > generic**: Despite hopes for a universal "Playwright for terminals," framework-specific testing tools (Textual Pilot, teatest, TestBackend) consistently provide better developer experience than cross-platform solutions. This mirrors web testing where framework test utilities (React Testing Library, Vue Test Utils) complement rather than replace Playwright.

**Real PTY testing maturing**: ratatui-testlib and tui-test both demonstrate that testing against real terminal emulation (rather than mocked backends) is becoming table stakes for integration tests. Expect this pattern to spread.

**The Playwright gap persists**: No tool has achieved Playwright's combination of cross-framework support, rich element selection, reliable auto-waiting, and massive ecosystem. Microsoft's tui-test is the closest conceptually but hasn't shipped a stable release in over a year. The TUI ecosystem may never converge on a single tool the way web testing has, given the deeper language-level fragmentation.

**Recommendations by use case**:
- **Python TUI app**: Textual + Pilot API (clear winner, no contest)
- **Go TUI app**: teatest for golden file tests, but expect to supplement with custom helpers
- **Rust TUI app**: TestBackend + insta for unit tests, ratatui-testlib for integration tests
- **JavaScript TUI app**: ink-testing-library if using Ink; tui-test if language-agnostic approach needed
- **Testing any CLI/TUI**: pexpect (Python) or tmux-based scripting remain the most reliable cross-platform options
- **New project choosing a stack**: Textual offers the best overall testing DX; Ratatui offers the most mature Rust experience
