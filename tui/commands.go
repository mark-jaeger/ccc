package tui

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mark-jaeger/ccc/config"
	"github.com/mark-jaeger/ccc/scan"
	"github.com/mark-jaeger/ccc/ssh"
	"github.com/mark-jaeger/ccc/zmx"
)

// saveHostsCmd saves the host list to the local config file.
func saveHostsCmd(hosts []config.Host) tea.Cmd {
	return func() tea.Msg {
		path, err := config.DefaultClientConfigPath()
		if err != nil {
			return errMsg{err}
		}
		cfg := &config.ClientConfig{Hosts: hosts}
		if err := config.SaveClientConfig(path, cfg); err != nil {
			return errMsg{err}
		}
		return nil // success, no message needed
	}
}

// loadHostsCmd loads hosts from the local config file.
func loadHostsCmd() tea.Cmd {
	return func() tea.Msg {
		path, err := config.DefaultClientConfigPath()
		if err != nil {
			return errMsg{err}
		}
		cfg, err := config.LoadClientConfig(path)
		if err != nil {
			return errMsg{err}
		}
		return hostsLoadedMsg{
			hosts: cfg.Hosts,
		}
	}
}

// connectHostCmd establishes SSH connection to a host.
func connectHostCmd(name string, host config.Host) tea.Cmd {
	return func() tea.Msg {
		conn := ssh.ConnectionFromHost(host)
		// Test the connection
		if err := conn.TestConnection(); err != nil {
			return errMsg{err}
		}
		return hostConnectedMsg{
			hostName: name,
			host:     host,
			runner:   conn,
		}
	}
}

// loadProjectsCmd loads projects from remote ~/.ccc/projects.toml.
func loadProjectsCmd(runner Runner) tea.Cmd {
	return func() tea.Msg {
		// Read projects.toml via runner
		output, err := runner.Run("cat ~/.ccc/projects.toml 2>/dev/null || echo ''")
		if err != nil {
			return errMsg{err}
		}

		if output == "" {
			// Return empty projects if file doesn't exist
			return projectsLoadedMsg{projects: &config.ProjectsConfig{
				Projects: []config.Project{},
			}}
		}

		projects, err := config.ParseProjectsConfig([]byte(output))
		if err != nil {
			return errMsg{fmt.Errorf("failed to parse projects.toml: %w", err)}
		}
		return projectsLoadedMsg{projects: projects}
	}
}

// loadProjectsLocalCmd loads projects from local ~/.ccc/projects.toml.
func loadProjectsLocalCmd() tea.Cmd {
	return func() tea.Msg {
		home, err := os.UserHomeDir()
		if err != nil {
			return errMsg{err}
		}
		path := home + "/.ccc/projects.toml"
		data, err := os.ReadFile(path)
		if err != nil {
			// Return empty projects if file doesn't exist
			return projectsLoadedMsg{projects: &config.ProjectsConfig{
				Projects: []config.Project{},
			}}
		}
		projects, err := config.ParseProjectsConfig(data)
		if err != nil {
			return errMsg{err}
		}
		return projectsLoadedMsg{projects: projects}
	}
}

// loadSessionsCmd lists zmx sessions for the current project.
func loadSessionsCmd(runner Runner, projectKey string) tea.Cmd {
	return func() tea.Msg {
		output, err := runner.Run(zmx.BuildListCommand())
		if err != nil {
			// Treat as no sessions
			return sessionsLoadedMsg{sessions: nil}
		}

		allSessions := zmx.ParseListOutput(output)
		sessions := zmx.FilterSessionsForProject(allSessions, projectKey)
		return sessionsLoadedMsg{sessions: sessions}
	}
}

// loadSessionsLocalCmd lists zmx sessions locally.
func loadSessionsLocalCmd(projectKey string) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("sh", "-c", zmx.BuildListCommand())
		out, err := cmd.Output()
		if err != nil {
			// Treat as no sessions
			return sessionsLoadedMsg{sessions: nil}
		}

		allSessions := zmx.ParseListOutput(string(out))
		sessions := zmx.FilterSessionsForProject(allSessions, projectKey)
		return sessionsLoadedMsg{sessions: sessions}
	}
}

// attachSessionCmd attaches to an existing zmx session using tea.ExecProcess.
// This hands the terminal over to zmx for full passthrough (FR-4.x).
func attachSessionCmd(host config.Host, sessionName string) tea.Cmd {
	// Build the full SSH command with zmx attach
	// TERM passthrough is handled via ssh -t which inherits TERM
	args := []string{"-t", host.Address}
	if host.User != "" {
		args = []string{"-t", host.User + "@" + host.Address}
	}
	if host.Port != 0 {
		args = append([]string{"-p", fmt.Sprintf("%d", host.Port)}, args...)
	}
	args = append(args, zmx.BuildAttachCommand(sessionName))

	cmd := exec.Command("ssh", args...)
	cmd.Env = os.Environ() // Inherit TERM from current environment

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return sessionExitedMsg{err: err}
	})
}

// attachSessionLocalCmd attaches in local mode (no SSH).
func attachSessionLocalCmd(sessionName string) tea.Cmd {
	cmd := exec.Command("sh", "-c", zmx.BuildAttachCommand(sessionName))
	cmd.Env = os.Environ() // Inherit TERM

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return sessionExitedMsg{err: err}
	})
}

// createSessionWithNameCmd creates and attaches to a session with explicit name.
func createSessionWithNameCmd(host config.Host, name, projectPath string) tea.Cmd {
	args := []string{"-t", host.Address}
	if host.User != "" {
		args = []string{"-t", host.User + "@" + host.Address}
	}
	if host.Port != 0 {
		args = append([]string{"-p", fmt.Sprintf("%d", host.Port)}, args...)
	}
	args = append(args, zmx.BuildCreateCommand(name, projectPath))

	cmd := exec.Command("ssh", args...)
	cmd.Env = os.Environ()

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return sessionExitedMsg{err: err}
	})
}

// createSessionWithNameLocalCmd creates a session with explicit name locally.
func createSessionWithNameLocalCmd(name, projectPath string) tea.Cmd {
	cmd := exec.Command("sh", "-c", zmx.BuildCreateCommand(name, projectPath))
	cmd.Env = os.Environ()

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return sessionExitedMsg{err: err}
	})
}

// killSessionCmd kills a zmx session.
func killSessionCmd(runner Runner, sessionName string) tea.Cmd {
	return func() tea.Msg {
		if _, err := runner.Run(zmx.BuildKillCommand(sessionName)); err != nil {
			return errMsg{err}
		}
		return sessionKilledMsg{name: sessionName}
	}
}

// killSessionLocalCmd kills a zmx session in local mode.
func killSessionLocalCmd(sessionName string) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("sh", "-c", zmx.BuildKillCommand(sessionName))
		if err := cmd.Run(); err != nil {
			return errMsg{err}
		}
		return sessionKilledMsg{name: sessionName}
	}
}

// scanProjectsCmd runs project scanning on remote.
func scanProjectsCmd(runner Runner) tea.Cmd {
	return func() tea.Msg {
		// Get home directory on remote
		homeDir, err := runner.Run("echo $HOME")
		if err != nil {
			return errMsg{err}
		}

		output, err := runner.Run(scan.BuildScanChainCommand(homeDir))
		if err != nil {
			return errMsg{err}
		}

		results := scan.ParseScanResults(output)
		var scanResults []scanResult
		for _, r := range results {
			scanResults = append(scanResults, scanResult{
				key:  scan.DeriveProjectKey(r.Path),
				path: r.Path,
			})
		}
		return scanCompleteMsg{results: scanResults}
	}
}

// scanProjectsLocalCmd runs project scanning locally.
func scanProjectsLocalCmd() tea.Cmd {
	return func() tea.Msg {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return errMsg{err}
		}

		cmd := exec.Command("sh", "-c", scan.BuildScanChainCommand(homeDir))
		out, err := cmd.Output()
		if err != nil {
			return errMsg{err}
		}

		results := scan.ParseScanResults(string(out))
		var scanResults []scanResult
		for _, r := range results {
			scanResults = append(scanResults, scanResult{
				key:  scan.DeriveProjectKey(r.Path),
				path: r.Path,
			})
		}
		return scanCompleteMsg{results: scanResults}
	}
}

// saveProjectsCmd saves projects config to remote.
func saveProjectsCmd(runner Runner, projects *config.ProjectsConfig) tea.Cmd {
	return func() tea.Msg {
		toml, err := config.SerializeProjectsConfig(projects)
		if err != nil {
			return errMsg{err}
		}
		cmd := fmt.Sprintf("mkdir -p ~/.ccc && cat > ~/.ccc/projects.toml << 'EOF'\n%s\nEOF", string(toml))
		if _, err := runner.Run(cmd); err != nil {
			return errMsg{err}
		}
		return projectsLoadedMsg{projects: projects}
	}
}

// saveProjectsLocalCmd saves projects config locally.
func saveProjectsLocalCmd(projects *config.ProjectsConfig) tea.Cmd {
	return func() tea.Msg {
		home, err := os.UserHomeDir()
		if err != nil {
			return errMsg{err}
		}
		toml, err := config.SerializeProjectsConfig(projects)
		if err != nil {
			return errMsg{err}
		}
		dir := home + "/.ccc"
		if err := os.MkdirAll(dir, 0700); err != nil {
			return errMsg{err}
		}
		if err := os.WriteFile(dir+"/projects.toml", toml, 0600); err != nil {
			return errMsg{err}
		}
		return projectsLoadedMsg{projects: projects}
	}
}

// checkZmxCmd checks if zmx is installed on the target.
func checkZmxCmd(runner Runner) tea.Cmd {
	return func() tea.Msg {
		if _, err := runner.Run(zmx.BuildCheckCommand()); err != nil {
			return errMsg{errors.New(zmx.InstallMessage)}
		}
		return zmxAvailableMsg{}
	}
}

// checkZmxLocalCmd checks if zmx is installed locally.
func checkZmxLocalCmd() tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("sh", "-c", zmx.BuildCheckCommand())
		if err := cmd.Run(); err != nil {
			return errMsg{errors.New(zmx.InstallMessage)}
		}
		return zmxAvailableMsg{}
	}
}
