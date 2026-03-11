// Package tui provides a Bubbletea-based terminal user interface for ccc.
package tui

// State represents the current screen in the application.
type State int

const (
	StateLoading State = iota
	StateHostSelect
	StateProjectSelect
	StateSessionSelect
	StateCreatingSession
	StateConnecting
	StateError
)

// String returns the state name for debugging.
func (s State) String() string {
	switch s {
	case StateLoading:
		return "Loading"
	case StateHostSelect:
		return "HostSelect"
	case StateProjectSelect:
		return "ProjectSelect"
	case StateSessionSelect:
		return "SessionSelect"
	case StateCreatingSession:
		return "CreatingSession"
	case StateConnecting:
		return "Connecting"
	case StateError:
		return "Error"
	default:
		return "Unknown"
	}
}
