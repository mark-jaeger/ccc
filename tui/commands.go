package tui

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

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

// connectionTester probes whether a candidate connection is reachable. It is a
// package-level seam (defaulting to (*ssh.Connection).TestConnection) so
// connectHostCmd's fallback wiring can be unit-tested without real SSH.
var connectionTester = func(c *ssh.Connection) error { return c.TestConnection() }

// connectHostCmd establishes an SSH connection to a host, trying the primary
// address first and then each entry in host.FallbackAddresses in order. This
// mirrors the non-TUI flow (flow.tryFallbackAddresses) so the live TUI can
// recover when the primary address becomes unreachable (e.g. when a laptop
// roams between Wi-Fi, LTE, and Tailscale and the host's IP changes).
func connectHostCmd(name string, host config.Host) tea.Cmd {
	return func() tea.Msg {
		conn, err := selectWorkingConnection(host, connectionTester)
		if err != nil {
			return errMsg{err}
		}
		// selectWorkingConnection may have fallen back to one of
		// host.FallbackAddresses. Record the address that actually worked on the
		// host handed to the model: later attach/create commands rebuild an
		// ssh.Connection from currentHost (= this host), so they must target the
		// same reachable address as runner rather than the dead primary. host is
		// a value copy, so this mutation stays local to the command.
		host.Address = conn.Address
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
//
// The SSH command is built via ssh.Connection so it goes through a login shell
// ($SHELL -lc) and carries every connection option (identity file, proxy jump,
// SSH options). Building the args by hand here previously dropped both, which
// made zmx fail to resolve (~/.local/bin not on the non-login PATH) → exit 127.
func attachSessionCmd(host config.Host, sessionName string) tea.Cmd {
	cmd := ssh.ConnectionFromHost(host).InteractiveCommand(zmx.BuildAttachCommand(sessionName))

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
// Like attachSessionCmd, it routes through ssh.Connection for a login shell and
// full connection options.
func createSessionWithNameCmd(host config.Host, name, projectPath string) tea.Cmd {
	cmd := ssh.ConnectionFromHost(host).InteractiveCommand(zmx.BuildCreateCommand(name, projectPath))

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

// selectWorkingConnection returns the first connection — the primary built from
// host, then each host.FallbackAddresses entry in order — for which test
// succeeds. If every candidate fails it returns an error naming the primary
// address and each fallback that was tried, so the user can tell which routes
// were attempted. test is injected so the selection logic stays unit-testable
// without real SSH; connectHostCmd passes (*ssh.Connection).TestConnection.
func selectWorkingConnection(host config.Host, test func(*ssh.Connection) error) (*ssh.Connection, error) {
	primary := ssh.ConnectionFromHost(host)
	if err := test(primary); err == nil {
		return primary, nil
	}

	for _, addr := range host.FallbackAddresses {
		fallback := primary.WithAddress(addr)
		if err := test(fallback); err == nil {
			return fallback, nil
		}
	}

	if len(host.FallbackAddresses) == 0 {
		return nil, fmt.Errorf("cannot reach host at %s", host.Address)
	}
	return nil, fmt.Errorf("cannot reach host: primary %s and fallbacks [%s] all failed",
		host.Address, strings.Join(host.FallbackAddresses, ", "))
}
