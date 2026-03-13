package tui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
	"github.com/mark-jaeger/ccc/zmx"
)

// sessionKeys returns additional keybindings to show in session list help.
func sessionKeys() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new session")),
		key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "kill")),
	}
}

// NewSessionList creates a list model for session selection.
// The title includes the project name for context.
func NewSessionList(sessions []zmx.Session, projectKey string, width, height int) list.Model {
	items := make([]list.Item, len(sessions))
	for i, s := range sessions {
		items[i] = NewSessionItem(s)
	}

	l := list.New(items, newDelegate(), width, height)
	l.Title = "Sessions: " + projectKey
	configureList(&l)
	l.AdditionalShortHelpKeys = sessionKeys

	return l
}

// EmptySessionList creates a list with a "no sessions" message.
func EmptySessionList(projectKey string, width, height int) list.Model {
	l := list.New([]list.Item{}, newDelegate(), width, height)
	l.Title = "Sessions: " + projectKey
	configureList(&l)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.AdditionalShortHelpKeys = sessionKeys

	// High-contrast empty state message with hint
	l.Styles.NoItems = lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#555555", Dark: "#AAAAAA"}).
		Padding(1, 2)

	return l
}
