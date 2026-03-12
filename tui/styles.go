package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Adaptive colors for light/dark terminals
	subtle    = lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#383838"}
	highlight = lipgloss.AdaptiveColor{Light: "#CC8E50", Dark: "#FFB270"} // RGB 255,178,112
	special   = lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#73F59F"}

	// SelectionIndicator shown before selected items.
	SelectionIndicator = "▸ "

	// TitleStyle for screen headers.
	TitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#000000")).
			Background(lipgloss.Color("#FFB270")).
			Padding(0, 1)

	// ItemStyle for unselected menu items.
	ItemStyle = lipgloss.NewStyle().PaddingLeft(4)

	// SelectedItemStyle for the currently highlighted item.
	SelectedItemStyle = lipgloss.NewStyle().
				PaddingLeft(0).
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
