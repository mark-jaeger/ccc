// Package ui provides interactive terminal menus, prompts, and confirmations
// for user input.
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
	Key   string // single char like "a", "s", "n"
	Label string // display text
	ID    string // returned in MenuResult.ExtraKey
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
		fmt.Fprintf(out, "\n  %s\n", cfg.Title)

		for i, item := range cfg.Items {
			if item.Extra != "" {
				fmt.Fprintf(out, "  [%d] %s %s\n", i+1, item.Label, item.Extra)
			} else {
				fmt.Fprintf(out, "  [%d] %s\n", i+1, item.Label)
			}
		}

		for _, ea := range cfg.ExtraActions {
			fmt.Fprintf(out, "  [%s] %s\n", ea.Key, ea.Label)
		}

		if cfg.ShowBack {
			fmt.Fprintf(out, "  [b] Back\n")
		}
		fmt.Fprintf(out, "  [q] Quit\n")

		if cfg.ShowRemove {
			fmt.Fprintf(out, "\n  %s (or 'r' to remove): ", prompt)
		} else {
			fmt.Fprintf(out, "\n  %s: ", prompt)
		}

		if !scanner.Scan() {
			return MenuResult{Action: ActionQuit}, scanner.Err()
		}
		input := strings.TrimSpace(scanner.Text())

		if input == "q" {
			return MenuResult{Action: ActionQuit}, nil
		}

		if input == "b" && cfg.ShowBack {
			return MenuResult{Action: ActionBack}, nil
		}

		if input == "r" && cfg.ShowRemove {
			result, ok, err := handleRemove(scanner, out, cfg)
			if err != nil {
				return MenuResult{Action: ActionQuit}, err
			}
			if !ok {
				continue
			}
			return result, nil
		}

		for _, ea := range cfg.ExtraActions {
			if input == ea.Key {
				return MenuResult{Action: ActionExtra, ExtraKey: ea.ID}, nil
			}
		}

		num, err := strconv.Atoi(input)
		if err != nil || num < 1 || num > len(cfg.Items) {
			fmt.Fprintf(out, "  Invalid selection. Try again.\n")
			continue
		}
		return MenuResult{Action: ActionSelect, Selected: cfg.Items[num-1]}, nil
	}
}

// handleRemove manages the remove flow: pick item number, then confirm y/n.
// Returns (result, true, nil) on confirmed removal, or (_, false, nil) to
// return to the menu on invalid selection or declined confirmation.
func handleRemove(scanner *bufio.Scanner, out io.Writer, cfg MenuConfig) (MenuResult, bool, error) {
	fmt.Fprintf(out, "  Select item to remove: ")
	if !scanner.Scan() {
		return MenuResult{}, false, scanner.Err()
	}
	input := strings.TrimSpace(scanner.Text())
	num, err := strconv.Atoi(input)
	if err != nil || num < 1 || num > len(cfg.Items) {
		fmt.Fprintf(out, "  Invalid selection.\n")
		return MenuResult{}, false, nil
	}
	item := cfg.Items[num-1]

	fmt.Fprintf(out, "  Remove %s? (y/n): ", item.Label)
	if !scanner.Scan() {
		return MenuResult{}, false, scanner.Err()
	}
	confirm := strings.TrimSpace(scanner.Text())
	if confirm != "y" && confirm != "Y" {
		return MenuResult{}, false, nil
	}
	return MenuResult{Action: ActionRemove, Selected: item}, true, nil
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
