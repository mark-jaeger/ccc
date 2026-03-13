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

	l := list.New(items, newDelegate(), width, height)
	l.Title = "Select Project"
	configureList(&l)

	return l
}
