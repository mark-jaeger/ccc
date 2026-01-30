// ccc is a CLI tool for managing tmux sessions on local and remote machines.
package main

import (
	"fmt"
	"os"

	"github.com/markjd/ccc/flow"
)

func main() {
	args := os.Args[1:]

	isLocal := len(args) > 0 && args[0] == "local"
	if isLocal {
		args = args[1:]
	}

	// Auto-detect: if running over SSH, use local mode
	if !isLocal && flow.IsSSHSession() {
		fmt.Println("\n  You're already on this machine via SSH.")
		fmt.Println("  Switching to local mode (no SSH hop).")
		isLocal = true
	}

	var err error
	if isLocal {
		err = flow.RunLocalMode(os.Stdin, os.Stdout)
	} else {
		err = flow.RunRemoteMode(os.Stdin, os.Stdout, args)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
