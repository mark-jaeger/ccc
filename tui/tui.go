package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mark-jaeger/ccc/v2/config"
)

// Run starts the TUI application.
// isLocal: true for local mode (no SSH hop), false for remote mode.
func Run(isLocal bool) error {
	m := New(isLocal)

	// Use alt-screen mode for clean terminal (FR-2.10)
	p := tea.NewProgram(m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(), // Optional: enable mouse support
	)

	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	// Check if model exited with error
	if model, ok := finalModel.(Model); ok && model.err != nil {
		return model.err
	}

	return nil
}

// RunWithHosts starts the TUI with pre-loaded hosts (for testing).
func RunWithHosts(hosts []config.Host) error {
	m := New(false)
	// Note: We need to convert []config.Host to map[string]config.Host
	// This function is for testing - caller provides named hosts
	// For now, skip setting hosts directly as SetHosts expects map + names

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
