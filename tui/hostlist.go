package tui

import (
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
	"github.com/mark-jaeger/ccc/v2/config"
)

// NewHostList creates a list model for host selection.
func NewHostList(hosts []config.Host, width, height int) list.Model {
	items := make([]list.Item, len(hosts))
	for i, h := range hosts {
		items[i] = NewHostItem(h.Name, h)
	}

	l := list.New(items, newDelegate(), width, height)
	l.Title = "Select Host"
	configureList(&l)

	return l
}

// newDelegate creates a consistently styled list delegate.
func newDelegate() list.DefaultDelegate {
	delegate := list.NewDefaultDelegate()

	// Normal items - padding to align with selected items' border
	delegate.Styles.NormalTitle = lipgloss.NewStyle().
		PaddingLeft(2)
	delegate.Styles.NormalDesc = lipgloss.NewStyle().
		PaddingLeft(2).
		Foreground(subtle)

	// Selected items - vertical bar indicator on the left
	selectedBorder := lipgloss.Border{Left: "│"}
	// Dimmed version of highlight for description
	highlightDim := lipgloss.AdaptiveColor{Light: "#CC8E50", Dark: "#B8956A"}
	delegate.Styles.SelectedTitle = lipgloss.NewStyle().
		Foreground(highlight).
		BorderLeft(true).
		BorderStyle(selectedBorder).
		BorderForeground(highlight).
		PaddingLeft(1)
	delegate.Styles.SelectedDesc = lipgloss.NewStyle().
		Foreground(highlightDim).
		BorderLeft(true).
		BorderStyle(selectedBorder).
		BorderForeground(highlight).
		PaddingLeft(1)

	return delegate
}

// configureList applies common configuration to all lists.
func configureList(l *list.Model) {
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(true)
	l.SetShowPagination(false) // Hide the "..." pagination

	// Title style - orange background with dark text
	l.Styles.Title = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#000000")).
		Background(lipgloss.Color("#FFB270")).
		Padding(0, 1)

	// Hide the spinner/divider elements
	l.Styles.Spinner = lipgloss.NewStyle()
	l.Styles.DividerDot = lipgloss.NewStyle()
	l.Styles.TitleBar = lipgloss.NewStyle()

	// Apply vim-style keybindings
	keys := Keys()
	l.KeyMap.CursorUp = keys.Up
	l.KeyMap.CursorDown = keys.Down
	l.KeyMap.GoToStart = keys.Top
	l.KeyMap.GoToEnd = keys.Bottom
	l.KeyMap.Filter = keys.Filter
	l.KeyMap.ShowFullHelp = keys.Help
	l.KeyMap.CloseFullHelp = keys.Help
}
