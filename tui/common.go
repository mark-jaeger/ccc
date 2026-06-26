package tui

import "context"

// Runner interface decouples TUI from the execution transport.
// Implemented by ssh.Connection (remote) and LocalRunner (local).
// Same interface as flow.Runner but defined here to avoid import cycles.
//
// RunContext is the cancellable form of Run: the TUI connect/load path threads a
// bounded context so a dead network self-aborts (and esc can cancel) instead of
// hanging on the kernel TCP timeout. ssh.Connection already satisfies it.
type Runner interface {
	Run(cmd string) (string, error)
	RunContext(ctx context.Context, cmd string) (string, error)
	RunInteractive(cmd string) error
}
