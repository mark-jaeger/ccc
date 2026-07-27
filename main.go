// ccc is a CLI tool for managing zmx sessions on local and remote machines.
package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/mark-jaeger/ccc/v2/flow"
	"github.com/mark-jaeger/ccc/v2/tui"
	"github.com/mark-jaeger/ccc/v2/zmx"
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

	// Check for zmx before starting TUI (local mode only - remote checks happen after SSH)
	if isLocal {
		if err := exec.Command("sh", "-c", "command -v zmx").Run(); err != nil {
			fmt.Fprintln(os.Stderr, zmx.InstallMessage)
			os.Exit(1)
		}
	}

	// Run the TUI
	if err := tui.Run(isLocal); err != nil {
		// Print a blank line after alt-screen exit for better visibility
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
