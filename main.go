// ccc is a CLI tool for managing zmx sessions on local and remote machines.
package main

import (
	"fmt"
	"os"

	"github.com/mark-jaeger/ccc/flow"
	"github.com/mark-jaeger/ccc/tui"
)

// version is set at build time via ldflags; defaults to "dev" for local builds.
var version = "dev"

func main() {
	args := os.Args[1:]

	if len(args) > 0 && (args[0] == "-v" || args[0] == "--version") {
		fmt.Println("ccc", version)
		return
	}

	isLocal := len(args) > 0 && args[0] == "local"
	if isLocal {
		args = args[1:]
	}

	// Auto-detect: if running over SSH, use local mode
	if !isLocal && flow.IsSSHSession() {
		isLocal = true
	}

	// Run the TUI
	if err := tui.Run(isLocal); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
