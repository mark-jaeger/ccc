package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mark-jaeger/ccc/config"
	"github.com/mark-jaeger/ccc/scan"
	"github.com/mark-jaeger/ccc/ssh"
	"github.com/mark-jaeger/ccc/transport"
	"github.com/mark-jaeger/ccc/zmx"
)

// saveHostsCmd saves the host list to the local config file.
func saveHostsCmd(hosts []config.Host) tea.Cmd {
	return func() tea.Msg {
		path, err := config.DefaultClientConfigPath()
		if err != nil {
			return errMsg{err: err}
		}
		cfg := &config.ClientConfig{Hosts: hosts}
		if err := config.SaveClientConfig(path, cfg); err != nil {
			return errMsg{err: err}
		}
		return nil // success, no message needed
	}
}

// loadHostsCmd loads hosts from the local config file.
func loadHostsCmd() tea.Cmd {
	return func() tea.Msg {
		path, err := config.DefaultClientConfigPath()
		if err != nil {
			return errMsg{err: err}
		}
		cfg, err := config.LoadClientConfig(path)
		if err != nil {
			return errMsg{err: err}
		}
		return hostsLoadedMsg{
			hosts: cfg.Hosts,
		}
	}
}

// connectionTester probes whether a candidate connection is reachable. It is a
// package-level seam (defaulting to (*ssh.Connection).TestConnectionContext) so
// connectHostCmd's fallback wiring can be unit-tested without real SSH. It takes
// the connect's context so a probe can be aborted mid-flight (esc) or bounded
// (dead link) instead of hanging on the kernel TCP timeout.
var connectionTester = func(ctx context.Context, c *ssh.Connection) error {
	return c.TestConnectionContext(ctx)
}

// connectHostCmd establishes an SSH connection to a host, trying the primary
// address first and then each entry in host.FallbackAddresses in order. This
// mirrors the non-TUI flow (flow.tryFallbackAddresses) so the live TUI can
// recover when the primary address becomes unreachable (e.g. when a laptop
// roams between Wi-Fi, LTE, and Tailscale and the host's IP changes).
//
// ctx bounds and cancels the whole primary+fallback probe so an esc (which
// cancels ctx) returns promptly instead of walking every dead address; gen tags
// the result so the model can drop it if the user has since canceled or started
// a different connect.
func connectHostCmd(ctx context.Context, gen int, name string, host config.Host) tea.Cmd {
	return func() tea.Msg {
		conn, err := selectWorkingConnection(ctx, host, connectionTester)
		if err != nil {
			return errMsg{err: err, gen: gen}
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
			gen:      gen,
		}
	}
}

// loadProjectsCmd loads projects from remote ~/.ccc/projects.toml. It runs
// through RunContext so a dead link self-aborts (or esc cancels) instead of
// hanging; gen tags the result for the model's stale-result guard.
func loadProjectsCmd(ctx context.Context, gen int, runner Runner) tea.Cmd {
	return func() tea.Msg {
		// Read projects.toml via runner
		output, err := runner.RunContext(ctx, "cat ~/.ccc/projects.toml 2>/dev/null || echo ''")
		if err != nil {
			return errMsg{err: err, gen: gen}
		}

		if output == "" {
			// Return empty projects if file doesn't exist
			return projectsLoadedMsg{projects: &config.ProjectsConfig{
				Projects: []config.Project{},
			}, gen: gen}
		}

		projects, err := config.ParseProjectsConfig([]byte(output))
		if err != nil {
			return errMsg{err: fmt.Errorf("failed to parse projects.toml: %w", err), gen: gen}
		}
		return projectsLoadedMsg{projects: projects, gen: gen}
	}
}

// loadProjectsLocalCmd loads projects from local ~/.ccc/projects.toml. Local
// reads cannot hang on the network so there is no context; gen is echoed for
// uniform stale-result handling (it is 0 — unguarded — at startup).
func loadProjectsLocalCmd(gen int) tea.Cmd {
	return func() tea.Msg {
		home, err := os.UserHomeDir()
		if err != nil {
			return errMsg{err: err, gen: gen}
		}
		path := home + "/.ccc/projects.toml"
		data, err := os.ReadFile(path)
		if err != nil {
			// Return empty projects if file doesn't exist
			return projectsLoadedMsg{projects: &config.ProjectsConfig{
				Projects: []config.Project{},
			}, gen: gen}
		}
		projects, err := config.ParseProjectsConfig(data)
		if err != nil {
			return errMsg{err: err, gen: gen}
		}
		return projectsLoadedMsg{projects: projects, gen: gen}
	}
}

// loadSessionsCmd lists zmx sessions for the current project. It runs through
// RunContext so a dead link self-aborts (or esc cancels); gen tags the result.
func loadSessionsCmd(ctx context.Context, gen int, runner Runner, projectKey string) tea.Cmd {
	return func() tea.Msg {
		output, err := runner.RunContext(ctx, zmx.BuildListCommand())
		if err != nil {
			// Treat as no sessions
			return sessionsLoadedMsg{sessions: nil, gen: gen}
		}

		allSessions := zmx.ParseListOutput(output)
		sessions := zmx.FilterSessionsForProject(allSessions, projectKey)
		return sessionsLoadedMsg{sessions: sessions, gen: gen}
	}
}

// loadSessionsLocalCmd lists zmx sessions locally. gen is echoed for uniform
// stale-result handling.
func loadSessionsLocalCmd(gen int, projectKey string) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("sh", "-c", zmx.BuildListCommand())
		out, err := cmd.Output()
		if err != nil {
			// Treat as no sessions
			return sessionsLoadedMsg{sessions: nil, gen: gen}
		}

		allSessions := zmx.ParseListOutput(string(out))
		sessions := zmx.FilterSessionsForProject(allSessions, projectKey)
		return sessionsLoadedMsg{sessions: sessions, gen: gen}
	}
}

// transportParamsFromHost maps a config.Host onto transport.Params so the
// interactive attach can be built for the host's opt-in transport (mosh/et).
func transportParamsFromHost(host config.Host) transport.Params {
	return transport.Params{
		Transport:      host.Transport,
		User:           host.User,
		Address:        host.Address,
		Port:           host.Port,
		IdentityFile:   host.IdentityFile,
		ProxyJump:      host.ProxyJump,
		SSHOptions:     host.SSHOptions,
		MoshServerPath: host.MoshServerPath,
	}
}

// interactiveAttachCmd builds the *exec.Cmd that hands the terminal to zmx for
// the remote command. It honors the host's opt-in roaming transport (mosh/et)
// when one is configured and locally available; otherwise — including the
// default ssh transport and any capability fallback — it uses the existing
// ssh.Connection.InteractiveCommand, which remains the single source of truth
// for the ssh wiring (login shell, identity/proxy/options).
func interactiveAttachCmd(host config.Host, remoteCmd string) *exec.Cmd {
	if cmd, err := transport.BuildInteractiveCmd(transportParamsFromHost(host), remoteCmd); err == nil {
		return cmd
	}
	return ssh.ConnectionFromHost(host).InteractiveCommand(remoteCmd)
}

// attachSessionCmd attaches to an existing zmx session using tea.ExecProcess.
// This hands the terminal over to zmx for full passthrough (FR-4.x).
//
// The remote command is built once (via zmx) and dispatched through
// interactiveAttachCmd, which picks the host's opt-in roaming transport
// (mosh/et) when configured and available, falling back to ssh.Connection
// otherwise. The ssh path still goes through a login shell ($SHELL -lc) and
// carries every connection option (identity file, proxy jump, SSH options);
// building the args by hand previously dropped both, making zmx fail to resolve
// (~/.local/bin not on the non-login PATH) → exit 127.
func attachSessionCmd(host config.Host, sessionName string) tea.Cmd {
	cmd := interactiveAttachCmd(host, zmx.BuildAttachCommand(sessionName))

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
// Like attachSessionCmd, it dispatches through interactiveAttachCmd so the
// host's opt-in roaming transport (mosh/et) is honored, falling back to
// ssh.Connection (login shell, full connection options) otherwise.
func createSessionWithNameCmd(host config.Host, name, projectPath string) tea.Cmd {
	cmd := interactiveAttachCmd(host, zmx.BuildCreateCommand(name, projectPath))

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
			return errMsg{err: err}
		}
		return sessionKilledMsg{name: sessionName}
	}
}

// killSessionLocalCmd kills a zmx session in local mode.
func killSessionLocalCmd(sessionName string) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("sh", "-c", zmx.BuildKillCommand(sessionName))
		if err := cmd.Run(); err != nil {
			return errMsg{err: err}
		}
		return sessionKilledMsg{name: sessionName}
	}
}

// scanProjectsCmd runs project scanning on remote. Both remote reads run through
// RunContext so a dead link self-aborts (or esc cancels); gen tags the result.
func scanProjectsCmd(ctx context.Context, gen int, runner Runner) tea.Cmd {
	return func() tea.Msg {
		// Get home directory on remote
		homeDir, err := runner.RunContext(ctx, "echo $HOME")
		if err != nil {
			return errMsg{err: err, gen: gen}
		}

		output, err := runner.RunContext(ctx, scan.BuildScanChainCommand(homeDir))
		if err != nil {
			return errMsg{err: err, gen: gen}
		}

		results := scan.ParseScanResults(output)
		var scanResults []scanResult
		for _, r := range results {
			scanResults = append(scanResults, scanResult{
				key:  scan.DeriveProjectKey(r.Path),
				path: r.Path,
			})
		}
		return scanCompleteMsg{results: scanResults, gen: gen}
	}
}

// scanProjectsLocalCmd runs project scanning locally. gen is echoed for uniform
// stale-result handling.
func scanProjectsLocalCmd(gen int) tea.Cmd {
	return func() tea.Msg {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return errMsg{err: err, gen: gen}
		}

		cmd := exec.Command("sh", "-c", scan.BuildScanChainCommand(homeDir))
		out, err := cmd.Output()
		if err != nil {
			return errMsg{err: err, gen: gen}
		}

		results := scan.ParseScanResults(string(out))
		var scanResults []scanResult
		for _, r := range results {
			scanResults = append(scanResults, scanResult{
				key:  scan.DeriveProjectKey(r.Path),
				path: r.Path,
			})
		}
		return scanCompleteMsg{results: scanResults, gen: gen}
	}
}

// saveProjectsCmd saves projects config to remote. The reload it emits is
// unguarded (gen 0): it follows a user-initiated scan/reorder/delete, not a
// cancelable request, so it should always apply.
func saveProjectsCmd(runner Runner, projects *config.ProjectsConfig) tea.Cmd {
	return func() tea.Msg {
		toml, err := config.SerializeProjectsConfig(projects)
		if err != nil {
			return errMsg{err: err}
		}
		cmd := fmt.Sprintf("mkdir -p ~/.ccc && cat > ~/.ccc/projects.toml << 'EOF'\n%s\nEOF", string(toml))
		if _, err := runner.Run(cmd); err != nil {
			return errMsg{err: err}
		}
		return projectsLoadedMsg{projects: projects}
	}
}

// saveProjectsLocalCmd saves projects config locally.
func saveProjectsLocalCmd(projects *config.ProjectsConfig) tea.Cmd {
	return func() tea.Msg {
		home, err := os.UserHomeDir()
		if err != nil {
			return errMsg{err: err}
		}
		toml, err := config.SerializeProjectsConfig(projects)
		if err != nil {
			return errMsg{err: err}
		}
		dir := home + "/.ccc"
		if err := os.MkdirAll(dir, 0700); err != nil {
			return errMsg{err: err}
		}
		if err := os.WriteFile(dir+"/projects.toml", toml, 0600); err != nil {
			return errMsg{err: err}
		}
		return projectsLoadedMsg{projects: projects}
	}
}

// checkZmxCmd checks if zmx is installed on the target. It runs through
// RunContext so a dead link self-aborts (or esc cancels) instead of hanging in
// the StateLoading window between connect and project load; gen tags the result.
func checkZmxCmd(ctx context.Context, gen int, runner Runner) tea.Cmd {
	return func() tea.Msg {
		if _, err := runner.RunContext(ctx, zmx.BuildCheckCommand()); err != nil {
			return errMsg{err: errors.New(zmx.InstallMessage), gen: gen}
		}
		return zmxAvailableMsg{gen: gen}
	}
}

// checkZmxLocalCmd checks if zmx is installed locally. gen is echoed for uniform
// stale-result handling (0 — unguarded — at startup).
func checkZmxLocalCmd(gen int) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("sh", "-c", zmx.BuildCheckCommand())
		if err := cmd.Run(); err != nil {
			return errMsg{err: errors.New(zmx.InstallMessage), gen: gen}
		}
		return zmxAvailableMsg{gen: gen}
	}
}

// selectWorkingConnection returns the first connection — the primary built from
// host, then each host.FallbackAddresses entry in order — for which test
// succeeds. If every candidate fails it returns an error pairing each attempted
// address with its underlying failure, so the user can tell whether the problem
// is reachability (add/reorder fallbacks) or trust/config (bad key, host-key
// mismatch, timeout). This preserves the diagnostic detail the pre-fallback TUI
// surfaced by returning TestConnection's error directly. test is injected so the
// selection logic stays unit-testable without real SSH; connectHostCmd passes
// connectionTester (TestConnectionContext).
//
// ctx is threaded into each probe and checked between fallbacks: once the caller
// cancels (esc) or the overall deadline passes, the walk short-circuits instead
// of dialing every remaining dead address — the difference between a prompt
// abort and a minutes-long freeze.
func selectWorkingConnection(ctx context.Context, host config.Host, test func(context.Context, *ssh.Connection) error) (*ssh.Connection, error) {
	primary := ssh.ConnectionFromHost(host)
	primErr := test(ctx, primary)
	if primErr == nil {
		return primary, nil
	}

	if len(host.FallbackAddresses) == 0 {
		return nil, fmt.Errorf("cannot reach host at %s: %w", host.Address, primErr)
	}

	failures := []string{fmt.Sprintf("%s: %v", host.Address, primErr)}
	for _, addr := range host.FallbackAddresses {
		// Stop probing once the connect has been canceled (esc) or its deadline
		// has passed, so an aborted connect returns promptly.
		if err := ctx.Err(); err != nil {
			failures = append(failures, fmt.Sprintf("aborted: %v", err))
			return nil, fmt.Errorf("cannot reach host: connect aborted [%s]", strings.Join(failures, "; "))
		}
		fallback := primary.WithAddress(addr)
		err := test(ctx, fallback)
		if err == nil {
			return fallback, nil
		}
		failures = append(failures, fmt.Sprintf("%s: %v", addr, err))
	}

	return nil, fmt.Errorf("cannot reach host: all addresses failed [%s]", strings.Join(failures, "; "))
}
