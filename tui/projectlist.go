package tui

import (
	"github.com/charmbracelet/bubbles/list"
	"github.com/mark-jaeger/ccc/config"
)

// NewProjectList creates a list model for project selection.
func NewProjectList(projects map[string]config.Project, keys []string, width, height int) list.Model {
	items := make([]list.Item, len(keys))
	for i, k := range keys {
		items[i] = NewProjectItem(k, projects[k])
	}

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = SelectedItemStyle
	delegate.Styles.SelectedDesc = SelectedItemStyle.Foreground(subtle)

	l := list.New(items, delegate, width, height)
	l.Title = "Select Project"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(true)

	// Apply vim-style keybindings
	k := Keys()
	l.KeyMap.CursorUp = k.Up
	l.KeyMap.CursorDown = k.Down
	l.KeyMap.GoToStart = k.Top
	l.KeyMap.GoToEnd = k.Bottom
	l.KeyMap.Filter = k.Filter
	l.KeyMap.ShowFullHelp = k.Help
	l.KeyMap.CloseFullHelp = k.Help

	return l
}
