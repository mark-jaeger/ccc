package tui

import (
	"github.com/charmbracelet/bubbles/list"
	"github.com/mark-jaeger/ccc/zmx"
)

// NewSessionList creates a list model for session selection.
// The title includes the project name for context.
func NewSessionList(sessions []zmx.Session, projectKey string, width, height int) list.Model {
	items := make([]list.Item, len(sessions))
	for i, s := range sessions {
		items[i] = NewSessionItem(s)
	}

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = SelectedItemStyle
	delegate.Styles.SelectedDesc = SelectedItemStyle.Foreground(subtle)

	l := list.New(items, delegate, width, height)
	l.Title = "Sessions: " + projectKey
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(true)

	// Apply vim-style keybindings
	keys := Keys()
	l.KeyMap.CursorUp = keys.Up
	l.KeyMap.CursorDown = keys.Down
	l.KeyMap.GoToStart = keys.Top
	l.KeyMap.GoToEnd = keys.Bottom
	l.KeyMap.Filter = keys.Filter
	l.KeyMap.ShowFullHelp = keys.Help
	l.KeyMap.CloseFullHelp = keys.Help

	return l
}

// EmptySessionList creates a list with a "no sessions" message.
func EmptySessionList(projectKey string, width, height int) list.Model {
	l := list.New([]list.Item{}, list.NewDefaultDelegate(), width, height)
	l.Title = "Sessions: " + projectKey
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(true)
	return l
}
