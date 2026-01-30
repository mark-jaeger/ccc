package flow

import (
	"errors"
	"fmt"
	"io"

	"github.com/markjd/ccc/config"
	"github.com/markjd/ccc/internal/shellutil"
	sshpkg "github.com/markjd/ccc/ssh"
	"github.com/markjd/ccc/ui"
)

// RunRemoteMode runs ccc in remote mode (SSH to host).
func RunRemoteMode(in io.Reader, out io.Writer, args []string) error {
	cfgPath, err := config.DefaultClientConfigPath()
	if err != nil {
		return err
	}
	cfg, err := config.LoadClientConfig(cfgPath)
	if err != nil {
		if errors.Is(err, config.ErrNoConfig) {
			return runFirstTimeSetup(in, out, cfgPath)
		}
		return fmt.Errorf("config error: %w", err)
	}

	// Shortcut: ccc <project> — bypass host selection when only one host exists
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
				{Key: "a", Label: "Add host", ID: "add"},
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
			if saveErr := config.SaveClientConfig(cfgPath, cfg); saveErr != nil {
				fmt.Fprintf(out, "  Warning: could not save config: %v\n", saveErr)
			}
			fmt.Fprintf(out, "  ✓ Removed %s\n", result.Selected.Key)
			continue
		case ui.ActionExtra:
			if err := AddHostFlow(in, out, cfg, cfgPath); err != nil {
				fmt.Fprintf(out, "  %v\n", err)
			}
			// Reload config — keep old cfg on failure
			if reloaded, loadErr := config.LoadClientConfig(cfgPath); loadErr != nil {
				fmt.Fprintf(out, "  Warning: could not reload config: %v\n", loadErr)
			} else {
				cfg = reloaded
			}
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

	conn := sshpkg.ConnectionFromHost(host)

	fmt.Fprintf(out, "\n  Connecting to %s...\n", hostName)

	// Read projects config from host
	projectsData, err := conn.Run("cat ~/.ccc/projects.toml")
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

	// Shortcut: project specified as arg
	if len(args) >= 1 {
		projectKey := args[0]
		if p, ok := projects.Projects[projectKey]; ok {
			if len(args) >= 2 && args[1] == "new" {
				return createSession(in, out, conn, projectKey, p.Path, nil)
			}
			return SessionFlow(in, out, conn, projectKey, p.Path)
		}
		fmt.Fprintf(out, "  Unknown project: %s\n", projectKey)
	}

	scanFn := func(in io.Reader, out io.Writer) (*config.ProjectsConfig, error) {
		return RunScanFlow(in, out, conn, hostName)
	}
	saveFn := remoteSaveFn(conn)
	return ProjectFlow(in, out, conn, projects, scanFn, saveFn)
}

func remoteSaveFn(conn *sshpkg.Connection) func(*config.ProjectsConfig) error {
	return func(projects *config.ProjectsConfig) error {
		data, err := config.SerializeProjectsConfig(projects)
		if err != nil {
			return err
		}
		writeCmd := fmt.Sprintf("mkdir -p ~/.ccc && printf '%%s' %s > ~/.ccc/projects.toml", shellutil.Quote(string(data)))
		_, err = conn.Run(writeCmd)
		return err
	}
}

func runFirstTimeSetup(in io.Reader, out io.Writer, cfgPath string) error {
	cfg, err := SetupFirstHost(in, out, cfgPath)
	if err != nil {
		return err
	}
	return hostSelectionLoop(in, out, cfg, cfgPath, nil)
}

func runRemoteScan(in io.Reader, out io.Writer, conn *sshpkg.Connection, hostName string) error {
	projects, err := RunScanFlow(in, out, conn, hostName)
	if err != nil || projects == nil {
		return err
	}
	scanFn := func(in io.Reader, out io.Writer) (*config.ProjectsConfig, error) {
		return RunScanFlow(in, out, conn, hostName)
	}
	saveFn := remoteSaveFn(conn)
	return ProjectFlow(in, out, conn, projects, scanFn, saveFn)
}
