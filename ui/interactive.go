package ui

import (
	"fmt"
	"os"
	"strings"
	"unicode"

	"golang.org/x/term"
)

// showInteractiveMenu displays an arrow-key navigable menu with an action bar.
// It enters raw mode on inFile and renders ANSI output to outFile.
func showInteractiveMenu(inFile, outFile *os.File, cfg MenuConfig) (MenuResult, error) {
	fd := int(inFile.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return MenuResult{Action: ActionQuit}, err
	}
	defer term.Restore(fd, oldState)

	cursor := 0
	totalLines := 0
	firstDraw := true

	// Hide cursor
	fmt.Fprint(outFile, "\x1b[?25l")
	defer fmt.Fprint(outFile, "\x1b[?25h")

	actionBar := buildActionBar(cfg)

	for {
		if !firstDraw && totalLines > 0 {
			// Move cursor up to beginning of menu
			fmt.Fprintf(outFile, "\x1b[%dA", totalLines)
		}
		firstDraw = false

		totalLines = renderMenu(outFile, cfg, cursor, actionBar)

		evt, ch := readKey(inFile)

		switch evt {
		case keyUp:
			if cursor > 0 {
				cursor--
			}
		case keyDown:
			if cursor < len(cfg.Items)-1 {
				cursor++
			}
		case keyEnter:
			if len(cfg.Items) > 0 {
				clearMenu(outFile, totalLines)
				return MenuResult{Action: ActionSelect, Selected: cfg.Items[cursor]}, nil
			}
		case keyEsc, keyCtrlC:
			clearMenu(outFile, totalLines)
			return MenuResult{Action: ActionQuit}, nil
		case keyRune:
			lower := unicode.ToLower(ch)

			if lower == 'q' {
				clearMenu(outFile, totalLines)
				return MenuResult{Action: ActionQuit}, nil
			}
			if lower == 'b' && cfg.ShowBack {
				clearMenu(outFile, totalLines)
				return MenuResult{Action: ActionBack}, nil
			}
			if lower == 'r' && cfg.ShowRemove && len(cfg.Items) > 0 {
				confirmed, err := confirmRemove(inFile, outFile, cfg, cursor, totalLines, actionBar)
				if err != nil {
					clearMenu(outFile, totalLines)
					return MenuResult{Action: ActionQuit}, err
				}
				if confirmed {
					clearMenu(outFile, totalLines)
					return MenuResult{Action: ActionRemove, Selected: cfg.Items[cursor]}, nil
				}
				// Declined — redraw will happen on next loop iteration
				continue
			}

			for _, ea := range cfg.ExtraActions {
				if string(lower) == strings.ToLower(ea.Key) {
					clearMenu(outFile, totalLines)
					if ea.ItemAction && len(cfg.Items) > 0 {
						return MenuResult{Action: ActionExtra, ExtraKey: ea.ID, Selected: cfg.Items[cursor]}, nil
					}
					return MenuResult{Action: ActionExtra, ExtraKey: ea.ID}, nil
				}
			}

			// Number key: jump + select (1-9)
			if ch >= '1' && ch <= '9' {
				idx := int(ch-'0') - 1
				if idx < len(cfg.Items) {
					clearMenu(outFile, totalLines)
					return MenuResult{Action: ActionSelect, Selected: cfg.Items[idx]}, nil
				}
			}
		}
	}
}

// renderMenu draws the complete menu and returns the number of lines written.
func renderMenu(out *os.File, cfg MenuConfig, cursor int, actionBar string) int {
	lines := 0

	// Title (blank line + title)
	fmt.Fprintf(out, "\x1b[2K\r\n")
	lines++
	fmt.Fprintf(out, "\x1b[2K\r  %s\n", cfg.Title)
	lines++

	// Items
	for i, item := range cfg.Items {
		fmt.Fprint(out, "\x1b[2K\r")
		label := item.Label
		if item.Extra != "" {
			label += "  " + item.Extra
		}
		if i == cursor {
			fmt.Fprintf(out, "  \x1b[7m> %s\x1b[0m\n", label)
		} else {
			fmt.Fprintf(out, "    %s\n", label)
		}
		lines++
	}

	// Action bar (blank line + bar)
	fmt.Fprintf(out, "\x1b[2K\r\n")
	lines++
	fmt.Fprintf(out, "\x1b[2K\r  %s", actionBar)
	lines++

	return lines
}

// buildActionBar constructs the bottom action bar string from the menu config.
func buildActionBar(cfg MenuConfig) string {
	var parts []string
	parts = append(parts, "[enter] Select")
	for _, ea := range cfg.ExtraActions {
		parts = append(parts, fmt.Sprintf("[%s] %s", ea.Key, ea.Label))
	}
	if cfg.ShowRemove {
		parts = append(parts, "[r] Remove")
	}
	if cfg.ShowBack {
		parts = append(parts, "[b] Back")
	}
	parts = append(parts, "[q] Quit")
	return strings.Join(parts, "  ")
}

// clearMenu moves up to the start of the menu and clears all lines.
func clearMenu(out *os.File, totalLines int) {
	if totalLines > 0 {
		fmt.Fprintf(out, "\x1b[%dA", totalLines)
	}
	for i := 0; i < totalLines+1; i++ {
		fmt.Fprintf(out, "\x1b[2K\r\n")
	}
	// Move back up to where the menu started
	if totalLines > 0 {
		fmt.Fprintf(out, "\x1b[%dA", totalLines+1)
	}
}

// confirmRemove shows an inline confirmation prompt for removing the highlighted item.
// Returns true if confirmed, false if declined.
func confirmRemove(inFile, outFile *os.File, cfg MenuConfig, cursor, totalLines int, actionBar string) (bool, error) {
	item := cfg.Items[cursor]

	// Overwrite the action bar with the confirmation prompt
	fmt.Fprintf(outFile, "\x1b[2K\r  Remove %s? [y/n] ", item.Label)

	for {
		evt, ch := readKey(inFile)
		switch evt {
		case keyRune:
			lower := unicode.ToLower(ch)
			if lower == 'y' {
				return true, nil
			}
			if lower == 'n' {
				return false, nil
			}
		case keyEsc, keyCtrlC:
			return false, nil
		}
	}
}
