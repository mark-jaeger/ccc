package ui

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Action represents the type of action returned from a menu interaction.
type Action int

const (
	ActionSelect Action = iota
	ActionQuit
	ActionBack
	ActionRemove
	ActionExtra
)

// MenuItem represents a selectable item in a menu.
type MenuItem struct {
	Key   string
	Label string
	Extra string // optional extra info shown after label
}

// ExtraAction represents an additional action available in the menu
// beyond standard select/quit/back/remove.
type ExtraAction struct {
	Key    string // single char like "a", "s", "n"
	Label  string // display text
	Action string // returned in result
}

// MenuConfig holds the configuration for displaying a menu.
type MenuConfig struct {
	Title        string
	Items        []MenuItem
	Prompt       string // defaults to "Select"
	ShowBack     bool
	ShowRemove   bool
	ExtraActions []ExtraAction
}

// MenuResult holds the outcome of a menu interaction.
type MenuResult struct {
	Action   Action
	Selected MenuItem
	ExtraKey string // which extra action was chosen
}

// ShowMenu displays a numbered menu and handles user input.
// It loops until the user makes a valid selection.
// All I/O goes through in/out for testability.
func ShowMenu(in io.Reader, out io.Writer, cfg MenuConfig) (MenuResult, error) {
	scanner := bufio.NewScanner(in)
	prompt := cfg.Prompt
	if prompt == "" {
		prompt = "Select"
	}

	for {
		// Display title
		fmt.Fprintf(out, "\n  %s\n", cfg.Title)

		// Display numbered items
		for i, item := range cfg.Items {
			if item.Extra != "" {
				fmt.Fprintf(out, "  [%d] %s %s\n", i+1, item.Label, item.Extra)
			} else {
				fmt.Fprintf(out, "  [%d] %s\n", i+1, item.Label)
			}
		}

		// Display extra actions
		for _, ea := range cfg.ExtraActions {
			fmt.Fprintf(out, "  [%s] %s\n", ea.Key, ea.Label)
		}

		// Display back option
		if cfg.ShowBack {
			fmt.Fprintf(out, "  [b] Back\n")
		}

		// Always display quit
		fmt.Fprintf(out, "  [q] Quit\n")

		// Display prompt
		if cfg.ShowRemove {
			fmt.Fprintf(out, "\n  %s (or 'r' to remove): ", prompt)
		} else {
			fmt.Fprintf(out, "\n  %s: ", prompt)
		}

		// Read input
		if !scanner.Scan() {
			return MenuResult{Action: ActionQuit}, scanner.Err()
		}
		input := strings.TrimSpace(scanner.Text())

		// Handle quit
		if input == "q" {
			return MenuResult{Action: ActionQuit}, nil
		}

		// Handle back
		if input == "b" && cfg.ShowBack {
			return MenuResult{Action: ActionBack}, nil
		}

		// Handle remove
		if input == "r" && cfg.ShowRemove {
			return handleRemove(scanner, out, cfg)
		}

		// Handle extra actions
		for _, ea := range cfg.ExtraActions {
			if input == ea.Key {
				return MenuResult{Action: ActionExtra, ExtraKey: ea.Action}, nil
			}
		}

		// Handle numeric selection
		num, err := strconv.Atoi(input)
		if err != nil || num < 1 || num > len(cfg.Items) {
			fmt.Fprintf(out, "  Invalid selection. Try again.\n")
			continue
		}
		return MenuResult{Action: ActionSelect, Selected: cfg.Items[num-1]}, nil
	}
}

// handleRemove manages the remove flow: pick item number, then confirm y/n.
func handleRemove(scanner *bufio.Scanner, out io.Writer, cfg MenuConfig) (MenuResult, error) {
	fmt.Fprintf(out, "  Select item to remove: ")
	if !scanner.Scan() {
		return MenuResult{Action: ActionQuit}, scanner.Err()
	}
	input := strings.TrimSpace(scanner.Text())
	num, err := strconv.Atoi(input)
	if err != nil || num < 1 || num > len(cfg.Items) {
		fmt.Fprintf(out, "  Invalid selection.\n")
		return MenuResult{Action: ActionQuit}, nil
	}
	item := cfg.Items[num-1]

	fmt.Fprintf(out, "  Remove %s? (y/n): ", item.Label)
	if !scanner.Scan() {
		return MenuResult{Action: ActionQuit}, scanner.Err()
	}
	confirm := strings.TrimSpace(scanner.Text())
	if confirm != "y" && confirm != "Y" {
		return MenuResult{Action: ActionQuit}, nil
	}
	return MenuResult{Action: ActionRemove, Selected: item}, nil
}

// Prompt asks a simple question and returns the trimmed answer.
func Prompt(in io.Reader, out io.Writer, question string) (string, error) {
	scanner := bufio.NewScanner(in)
	fmt.Fprintf(out, "  %s: ", question)
	if !scanner.Scan() {
		return "", scanner.Err()
	}
	return strings.TrimSpace(scanner.Text()), nil
}

// Confirm asks a y/n question and returns true for yes.
func Confirm(in io.Reader, out io.Writer, question string) (bool, error) {
	scanner := bufio.NewScanner(in)
	fmt.Fprintf(out, "  %s (y/n): ", question)
	if !scanner.Scan() {
		return false, scanner.Err()
	}
	answer := strings.TrimSpace(scanner.Text())
	return answer == "y" || answer == "Y", nil
}
