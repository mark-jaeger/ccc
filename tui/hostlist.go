package tui

import (
	"github.com/charmbracelet/bubbles/list"
	"github.com/mark-jaeger/ccc/config"
)

// NewHostList creates a list model for host selection.
func NewHostList(hosts map[string]config.Host, names []string, width, height int) list.Model {
	items := make([]list.Item, len(names))
	for i, name := range names {
		items[i] = NewHostItem(name, hosts[name])
	}

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = SelectedItemStyle
	delegate.Styles.SelectedDesc = SelectedItemStyle.Foreground(subtle)

	l := list.New(items, delegate, width, height)
	l.Title = "Select Host"
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
