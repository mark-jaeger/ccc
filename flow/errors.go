package flow

import (
	"fmt"
	"io"
	"strings"

	"github.com/markjd/ccc/tmux"
)

// CheckTmux verifies tmux is available. If not, shows install instructions
// and offers a shell to install it.
func CheckTmux(in io.Reader, out io.Writer, runner Runner) error {
	result, err := runner.Run(tmux.BuildCheckTmuxCommand())
	if err == nil && strings.TrimSpace(result) != "" {
		return nil // tmux found
	}

	// Detect OS for install instructions
	osInfo, _ := runner.Run("uname -s")
	osInfo = strings.TrimSpace(strings.ToLower(osInfo))

	fmt.Fprintf(out, "\n  tmux not found.\n\n")
	fmt.Fprintf(out, "  Install tmux:\n")

	if strings.Contains(osInfo, "darwin") {
		fmt.Fprintf(out, "    brew install tmux\n")
	} else {
		fmt.Fprintf(out, "    macOS:   brew install tmux\n")
		fmt.Fprintf(out, "    Ubuntu:  sudo apt install tmux\n")
		fmt.Fprintf(out, "    Fedora:  sudo dnf install tmux\n")
		fmt.Fprintf(out, "    Arch:    sudo pacman -S tmux\n")
	}

	fmt.Fprintf(out, "\n  Opening shell so you can install it...\n")
	if err := runner.RunInteractive("bash -l"); err != nil {
		return fmt.Errorf("shell failed: %w", err)
	}

	// Recheck
	fmt.Fprintf(out, "\n  Rechecking... ")
	result, err = runner.Run(tmux.BuildCheckTmuxCommand())
	if err != nil || strings.TrimSpace(result) == "" {
		fmt.Fprintf(out, "tmux still not found.\n")
		return fmt.Errorf("tmux not installed")
	}
	fmt.Fprintf(out, "\u2713 tmux found.\n")
	return nil
}
