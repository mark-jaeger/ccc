package tui

import (
	"github.com/charmbracelet/bubbles/list"
	"github.com/mark-jaeger/ccc/v2/config"
)

// NewProjectList creates a list model for project selection.
func NewProjectList(projects []config.Project, width, height int) list.Model {
	items := make([]list.Item, len(projects))
	for i, p := range projects {
		items[i] = NewProjectItem(p.Name, p)
	}

	l := list.New(items, newDelegate(), width, height)
	l.Title = "Select Project"
	configureList(&l)

	return l
}
