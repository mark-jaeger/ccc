package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Adaptive colors for light/dark terminals
	subtle    = lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#383838"}
	highlight = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}
	special   = lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#73F59F"}

	// TitleStyle for screen headers.
	TitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFDF5")).
			Background(highlight).
			Padding(0, 1)

	// ItemStyle for unselected menu items.
	ItemStyle = lipgloss.NewStyle().PaddingLeft(4)

	// SelectedItemStyle for the currently highlighted item.
	SelectedItemStyle = lipgloss.NewStyle().
				PaddingLeft(2).
				Foreground(highlight)

	// StatusBarStyle for the bottom status bar.
	StatusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#343433", Dark: "#C1C6B2"}).
			Background(subtle)

	// ErrorStyle for error messages.
	ErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000"))

	// BreadcrumbStyle for navigation breadcrumbs.
	BreadcrumbStyle = lipgloss.NewStyle().
			Foreground(subtle)

	// SpecialStyle for special items (e.g., external sessions).
	SpecialStyle = lipgloss.NewStyle().
			Foreground(special)
)

// Unused variable to satisfy compiler
var _ = special
