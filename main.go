package main

import (
	"fmt"
	"os"

	"github.com/markjd/ccc/flow"
)

func main() {
	args := os.Args[1:]

	// Check for local mode
	isLocal := len(args) > 0 && args[0] == "local"

	// Auto-detect: if running over SSH, suggest local mode
	if !isLocal && flow.IsSSHSession() {
		fmt.Println("\n  You're already on this machine via SSH.")
		fmt.Println("  Switching to local mode (no SSH hop).")
		isLocal = true
	}

	if isLocal {
		if err := flow.RunLocalMode(os.Stdin, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	fmt.Println("ccc: remote mode not yet implemented")
}
