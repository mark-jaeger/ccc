// Package tui provides a Bubbletea-based terminal user interface for ccc.
package tui

// State represents the current screen in the application.
type State int

const (
	StateLoading State = iota
	StateHostSelect
	StateProjectSelect
	StateSessionSelect
	StateSessionNameInput
	StateCreatingSession
	StateConnecting
	StateReconnecting
	StateConnectionLost
	StateError
	StateHelp
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
	case StateSessionNameInput:
		return "SessionNameInput"
	case StateCreatingSession:
		return "CreatingSession"
	case StateConnecting:
		return "Connecting"
	case StateReconnecting:
		return "Reconnecting"
	case StateConnectionLost:
		return "ConnectionLost"
	case StateError:
		return "Error"
	case StateHelp:
		return "Help"
	default:
		return "Unknown"
	}
}
