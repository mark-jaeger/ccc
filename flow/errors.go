package flow

import (
	"fmt"
	"io"
	"strings"

	"github.com/mark-jaeger/ccc/abduco"
)

// CheckAbduco verifies abduco is available. If not, shows install instructions
// and offers a shell to install it.
func CheckAbduco(in io.Reader, out io.Writer, runner Runner) error {
	result, err := runner.Run(abduco.BuildCheckCommand())
	if err == nil && strings.TrimSpace(result) != "" {
		return nil // abduco found
	}

	// Detect OS for install instructions
	osInfo, _ := runner.Run("uname -s")
	osInfo = strings.TrimSpace(strings.ToLower(osInfo))

	fmt.Fprintf(out, "\n  abduco not found.\n\n")
	fmt.Fprintf(out, "  Install abduco:\n")

	if strings.Contains(osInfo, "darwin") {
		fmt.Fprintf(out, "    brew install abduco\n")
	} else {
		fmt.Fprintf(out, "    macOS:   brew install abduco\n")
		fmt.Fprintf(out, "    Ubuntu:  sudo apt install abduco\n")
		fmt.Fprintf(out, "    Fedora:  sudo dnf install abduco\n")
		fmt.Fprintf(out, "    Arch:    sudo pacman -S abduco\n")
	}

	fmt.Fprintf(out, "\n  Opening shell so you can install it...\n")
	if err := runner.RunInteractive("$SHELL -l"); err != nil {
		return fmt.Errorf("shell failed: %w", err)
	}

	// Recheck
	fmt.Fprintf(out, "\n  Rechecking... ")
	result, err = runner.Run(abduco.BuildCheckCommand())
	if err != nil || strings.TrimSpace(result) == "" {
		fmt.Fprintf(out, "abduco still not found.\n")
		return fmt.Errorf("abduco not installed")
	}
	fmt.Fprintf(out, "\u2713 abduco found.\n")
	return nil
}
