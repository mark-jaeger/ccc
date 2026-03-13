package tui

// Runner interface decouples TUI from the execution transport.
// Implemented by ssh.Connection (remote) and LocalRunner (local).
// Same interface as flow.Runner but defined here to avoid import cycles.
type Runner interface {
	Run(cmd string) (string, error)
	RunInteractive(cmd string) error
}
