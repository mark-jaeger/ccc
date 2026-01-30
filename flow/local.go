package flow

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/markjd/ccc/config"
)

// LocalRunner executes commands directly on the local machine.
type LocalRunner struct{}

func (r *LocalRunner) Run(cmd string) (string, error) {
	out, err := exec.Command("bash", "-lc", cmd).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("command failed: %s\nstderr: %s", cmd, string(exitErr.Stderr))
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (r *LocalRunner) RunInteractive(cmd string) error {
	proc := exec.Command("bash", "-lc", cmd)
	proc.Stdin = os.Stdin
	proc.Stdout = os.Stdout
	proc.Stderr = os.Stderr
	return proc.Run()
}

// RunLocalMode runs ccc in local mode (no SSH).
func RunLocalMode(in io.Reader, out io.Writer) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}
	projectsPath := home + "/.ccc/projects.toml"

	data, err := os.ReadFile(projectsPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(out, "\n  No projects configured. Run scan or create %s\n", projectsPath)
			return nil
		}
		return err
	}

	projects, err := config.ParseProjectsConfig(data)
	if err != nil {
		return fmt.Errorf("config error in %s: %w", projectsPath, err)
	}

	runner := &LocalRunner{}
	saveFn := func(p *config.ProjectsConfig) error {
		data, serErr := config.SerializeProjectsConfig(p)
		if serErr != nil {
			return serErr
		}
		return os.WriteFile(projectsPath, data, 0600)
	}
	return ProjectFlow(in, out, runner, projects, nil, saveFn)
}

// IsSSHSession checks if we're running inside an SSH session.
func IsSSHSession() bool {
	return os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_CLIENT") != ""
}
