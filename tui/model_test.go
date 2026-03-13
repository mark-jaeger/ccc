package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mark-jaeger/ccc/config"
	"github.com/mark-jaeger/ccc/zmx"
)

func TestNewModel(t *testing.T) {
	t.Run("remote mode", func(t *testing.T) {
		m := New(false)
		if m.isLocal {
			t.Error("expected isLocal=false")
		}
		if m.state != StateLoading {
			t.Errorf("expected StateLoading, got %v", m.state)
		}
	})

	t.Run("local mode", func(t *testing.T) {
		m := New(true)
		if !m.isLocal {
			t.Error("expected isLocal=true")
		}
	})
}

func TestWindowSizeMsg(t *testing.T) {
	m := New(false)
	// Initialize lists by setting hosts (simulates what happens when hostsLoadedMsg is received)
	m.width = 80
	m.height = 24
	hosts := []config.Host{{Name: "server1", Address: "192.168.1.1"}}
	m.SetHosts(hosts)

	msg := tea.WindowSizeMsg{Width: 100, Height: 30}

	newModel, _ := m.Update(msg)
	m = newModel.(Model)

	if m.width != 100 || m.height != 30 {
		t.Errorf("expected 100x30, got %dx%d", m.width, m.height)
	}
}

func TestBackNavigation(t *testing.T) {
	tests := []struct {
		name       string
		initial    State
		isLocal    bool
		expected   State
		shouldQuit bool
	}{
		{"host->quit", StateHostSelect, false, StateHostSelect, true},
		{"project->host", StateProjectSelect, false, StateHostSelect, false},
		{"session->project", StateSessionSelect, false, StateProjectSelect, false},
		{"project->quit (local)", StateProjectSelect, true, StateProjectSelect, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(tt.isLocal)
			m.state = tt.initial

			newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
			m = newModel.(Model)

			if tt.shouldQuit {
				if cmd == nil {
					t.Error("expected quit command")
				}
			} else {
				if m.state != tt.expected {
					t.Errorf("expected %v, got %v", tt.expected, m.state)
				}
			}
		})
	}
}

func TestHostsLoadedMsg(t *testing.T) {
	m := New(false)
	m.width = 80
	m.height = 24

	hosts := []config.Host{
		{Name: "server1", Address: "192.168.1.1"},
		{Name: "server2", Address: "192.168.1.2"},
	}

	newModel, _ := m.Update(hostsLoadedMsg{hosts: hosts})
	m = newModel.(Model)

	if m.state != StateHostSelect {
		t.Errorf("expected StateHostSelect, got %v", m.state)
	}
}

func TestProjectsLoadedMsg(t *testing.T) {
	m := New(false)
	m.width = 80
	m.height = 24

	projects := &config.ProjectsConfig{
		Projects: []config.Project{
			{Name: "proj1", Path: "/home/user/proj1"},
		},
	}

	newModel, _ := m.Update(projectsLoadedMsg{projects: projects})
	m = newModel.(Model)

	if m.state != StateProjectSelect {
		t.Errorf("expected StateProjectSelect, got %v", m.state)
	}
}

func TestSessionsLoadedMsg(t *testing.T) {
	m := New(false)
	m.width = 80
	m.height = 24
	m.currentProjectKey = "proj1"

	sessions := []zmx.Session{
		{Name: "ccc.proj1.main", Project: "proj1", Suffix: "main"},
	}

	newModel, _ := m.Update(sessionsLoadedMsg{sessions: sessions})
	m = newModel.(Model)

	if m.state != StateSessionSelect {
		t.Errorf("expected StateSessionSelect, got %v", m.state)
	}
}

func TestErrorMsg(t *testing.T) {
	m := New(false)
	testErr := fmt.Errorf("test error")

	newModel, _ := m.Update(errMsg{err: testErr})
	m = newModel.(Model)

	if m.state != StateError {
		t.Errorf("expected StateError, got %v", m.state)
	}
	if m.err != testErr {
		t.Error("error not set correctly")
	}
}

func TestBreadcrumb(t *testing.T) {
	m := New(false)

	// No selections
	if b := m.breadcrumb(); b != "" {
		t.Errorf("expected empty breadcrumb, got %q", b)
	}

	// Host selected
	m.selectedHost = "server1"
	if b := m.breadcrumb(); !strings.Contains(b, "server1") {
		t.Errorf("breadcrumb should contain host: %q", b)
	}

	// Host and project selected
	m.selectedProject = "myproj"
	if b := m.breadcrumb(); !strings.Contains(b, "server1") || !strings.Contains(b, "myproj") {
		t.Errorf("breadcrumb should contain both: %q", b)
	}
}
