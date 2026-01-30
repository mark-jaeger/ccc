package flow

import (
	"fmt"
	"io"

	"github.com/markjd/ccc/config"
	sshpkg "github.com/markjd/ccc/ssh"
	"github.com/markjd/ccc/ui"
)

// SSHRunner executes commands over SSH.
type SSHRunner struct {
	Conn *sshpkg.Connection
}

func (r *SSHRunner) Run(cmd string) (string, error) {
	return r.Conn.RunCommand(cmd)
}

func (r *SSHRunner) RunInteractive(cmd string) error {
	return r.Conn.RunInteractive(cmd)
}

// RunRemoteMode runs ccc in remote mode (SSH to host).
func RunRemoteMode(in io.Reader, out io.Writer, args []string) error {
	cfgPath := config.DefaultClientConfigPath()
	cfg, err := config.LoadClientConfig(cfgPath)
	if err != nil {
		if err == config.ErrNoConfig {
			return runFirstTimeSetup(in, out, cfgPath)
		}
		return fmt.Errorf("config error: %w", err)
	}

	// Shortcut: ccc <project> — skip host if single host
	if len(args) >= 1 && len(cfg.Hosts) == 1 {
		var hostName string
		for name := range cfg.Hosts {
			hostName = name
		}
		return connectToHost(in, out, cfg, hostName, args)
	}

	// Shortcut: ccc <host> <project> [new]
	if len(args) >= 2 {
		if _, ok := cfg.Hosts[args[0]]; ok {
			return connectToHost(in, out, cfg, args[0], args[1:])
		}
	}

	// Interactive host selection
	return hostSelectionLoop(in, out, cfg, cfgPath, args)
}

func hostSelectionLoop(in io.Reader, out io.Writer, cfg *config.ClientConfig, cfgPath string, args []string) error {
	for {
		names := cfg.SortedHostNames()
		if len(names) == 0 {
			return runFirstTimeSetup(in, out, cfgPath)
		}

		// Auto-skip single host
		if len(names) == 1 && len(args) == 0 {
			return connectToHost(in, out, cfg, names[0], nil)
		}

		items := make([]ui.MenuItem, len(names))
		for i, name := range names {
			h := cfg.Hosts[name]
			items[i] = ui.MenuItem{
				Key:   name,
				Label: name,
				Extra: fmt.Sprintf("(%s@%s)", h.User, h.Address),
			}
		}

		result, err := ui.ShowMenu(in, out, ui.MenuConfig{
			Title:      "Hosts",
			Items:      items,
			ShowRemove: true,
			ExtraActions: []ui.ExtraAction{
				{Key: "a", Label: "Add host", Action: "add"},
			},
		})
		if err != nil {
			return err
		}

		switch result.Action {
		case ui.ActionQuit:
			return nil
		case ui.ActionRemove:
			cfg.RemoveHost(result.Selected.Key)
			config.SaveClientConfig(cfgPath, cfg)
			fmt.Fprintf(out, "  ✓ Removed %s\n", result.Selected.Key)
			continue
		case ui.ActionExtra:
			// Add host flow — stub for now, Task 12 will implement
			fmt.Fprintf(out, "\n  Add host: not yet fully implemented.\n")
			continue
		case ui.ActionSelect:
			if err := connectToHost(in, out, cfg, result.Selected.Key, nil); err != nil {
				fmt.Fprintf(out, "\n  Error: %v\n", err)
				continue
			}
			return nil
		}
	}
}

func connectToHost(in io.Reader, out io.Writer, cfg *config.ClientConfig, hostName string, args []string) error {
	host, ok := cfg.Hosts[hostName]
	if !ok {
		return fmt.Errorf("unknown host: %s", hostName)
	}

	conn := &sshpkg.Connection{
		User:         host.User,
		Address:      host.Address,
		Port:         host.Port,
		IdentityFile: host.IdentityFile,
		ProxyJump:    host.ProxyJump,
		SSHOptions:   host.SSHOptions,
	}

	fmt.Fprintf(out, "\n  Connecting to %s...\n", hostName)

	// Read projects config from host
	projectsData, err := conn.RunCommand("cat ~/.ccc/projects.toml")
	if err != nil {
		// Check if it's a connection issue or missing file
		if testErr := conn.TestConnection(); testErr != nil {
			return fmt.Errorf("cannot reach %s: %w", hostName, testErr)
		}
		// File missing → trigger scan
		fmt.Fprintf(out, "  No projects configured on %s.\n", hostName)
		return runRemoteScan(in, out, conn, hostName)
	}

	projects, err := config.ParseProjectsConfig([]byte(projectsData))
	if err != nil {
		return fmt.Errorf("projects config error on %s: %w", hostName, err)
	}

	runner := &SSHRunner{Conn: conn}

	// Shortcut: project specified as arg
	if len(args) >= 1 {
		projectKey := args[0]
		if p, ok := projects.Projects[projectKey]; ok {
			if len(args) >= 2 && args[1] == "new" {
				return createSession(in, out, runner, projectKey, p.Path, nil)
			}
			return SessionFlow(in, out, runner, projectKey, p.Path)
		}
		fmt.Fprintf(out, "  Unknown project: %s\n", projectKey)
	}

	return ProjectFlow(in, out, runner, projects)
}

func runFirstTimeSetup(in io.Reader, out io.Writer, cfgPath string) error {
	fmt.Fprintf(out, "\n  No config found. Let's set up your first host.\n")
	// TODO: Tailscale discovery, manual entry, save config (Task 12)
	fmt.Fprintf(out, "  First-time setup not yet fully implemented.\n")
	return nil
}

func runRemoteScan(in io.Reader, out io.Writer, conn *sshpkg.Connection, hostName string) error {
	// TODO: run scan chain, present results, save projects.toml (Task 13)
	fmt.Fprintf(out, "  Remote scan not yet fully implemented.\n")
	return nil
}
