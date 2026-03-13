package flow

import (
	"fmt"
	"io"
	"strings"

	"github.com/mark-jaeger/ccc/zmx"
)

// CheckZmx verifies zmx is available. If not, shows install instructions
// and offers a shell to install it.
func CheckZmx(in io.Reader, out io.Writer, runner Runner) error {
	result, err := runner.Run(zmx.BuildCheckCommand())
	if err == nil && strings.TrimSpace(result) != "" {
		return nil // zmx found
	}

	// Detect OS for install instructions
	osInfo, _ := runner.Run("uname -s")
	osInfo = strings.TrimSpace(strings.ToLower(osInfo))

	fmt.Fprintf(out, "\n  zmx not found.\n\n")
	fmt.Fprintf(out, "  Install zmx:\n")

	if strings.Contains(osInfo, "darwin") {
		fmt.Fprintf(out, "    brew install zmx\n")
	} else {
		fmt.Fprintf(out, "    macOS:   brew install zmx\n")
		fmt.Fprintf(out, "    Linux:   cargo install zmx (requires Rust)\n")
	}

	fmt.Fprintf(out, "\n  Opening shell so you can install it...\n")
	if err := runner.RunInteractive("$SHELL -l"); err != nil {
		return fmt.Errorf("shell failed: %w", err)
	}

	// Recheck
	fmt.Fprintf(out, "\n  Rechecking... ")
	result, err = runner.Run(zmx.BuildCheckCommand())
	if err != nil || strings.TrimSpace(result) == "" {
		fmt.Fprintf(out, "zmx still not found.\n")
		return fmt.Errorf("zmx not installed")
	}
	fmt.Fprintf(out, "\u2713 zmx found.\n")
	return nil
}
