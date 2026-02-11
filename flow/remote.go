package flow

import (
	"errors"
	"fmt"
	"io"

	"github.com/mark-jaeger/ccc/config"
	"github.com/mark-jaeger/ccc/internal/shellutil"
	sshpkg "github.com/mark-jaeger/ccc/ssh"
	"github.com/mark-jaeger/ccc/ui"
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
		return connectToHost(in, out, cfg, cfgPath, hostName, args)
	}

	// Shortcut: ccc <host> <project> [new]
	if len(args) >= 2 {
		if _, ok := cfg.Hosts[args[0]]; ok {
			return connectToHost(in, out, cfg, cfgPath, args[0], args[1:])
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
			return connectToHost(in, out, cfg, cfgPath, names[0], nil)
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
			if err := connectToHost(in, out, cfg, cfgPath, result.Selected.Key, nil); err != nil {
				fmt.Fprintf(out, "\n  Error: %v\n", err)
				continue
			}
			return nil
		}
	}
}

func connectToHost(in io.Reader, out io.Writer, cfg *config.ClientConfig, cfgPath string, hostName string, args []string) error {
	host, ok := cfg.Hosts[hostName]
	if !ok {
		return fmt.Errorf("unknown host: %s", hostName)
	}

	conn := sshpkg.ConnectionFromHost(host)

	fmt.Fprintf(out, "\n  Connecting to %s...\n", hostName)

	// Try primary address, then fallbacks if available
	if err := conn.TestConnection(); err != nil {
		fmt.Fprintf(out, "  Connection to %s failed: %v\n", host.Address, err)

		conn = tryFallbackAddresses(out, conn, host.FallbackAddresses)

		if conn == nil {
			var addErr error
			conn, addErr = offerAddFallback(in, out, cfg, cfgPath, hostName, host)
			if addErr != nil {
				return addErr
			}
		}

		if conn == nil {
			return fmt.Errorf("cannot reach %s: all addresses failed", hostName)
		}
	}

	fmt.Fprintf(out, "  ✓ Connected.\n")

	// Read projects config from host
	projectsData, err := conn.Run("cat ~/.ccc/projects.toml")
	if err != nil {
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

// tryFallbackAddresses attempts each fallback address in order and returns
// the first working connection, or nil if none are available or all fail.
func tryFallbackAddresses(out io.Writer, conn *sshpkg.Connection, addrs []string) *sshpkg.Connection {
	if len(addrs) == 0 {
		return nil
	}
	for _, addr := range addrs {
		fmt.Fprintf(out, "  Trying %s...\n", addr)
		fallback := conn.WithAddress(addr)
		if err := fallback.TestConnection(); err != nil {
			fmt.Fprintf(out, "  %s failed: %v\n", addr, err)
			continue
		}
		return fallback
	}
	return nil
}

// offerAddFallback prompts the user to enter a new fallback address, saves it
// to config, and returns a working connection if the address is reachable.
func offerAddFallback(in io.Reader, out io.Writer, cfg *config.ClientConfig, cfgPath string, hostName string, host config.Host) (*sshpkg.Connection, error) {
	addFallback, err := ui.Confirm(in, out, "Add a fallback address for next time?")
	if err != nil {
		return nil, err
	}
	if !addFallback {
		return nil, nil
	}

	addr, err := ui.Prompt(in, out, "Fallback address (IP or hostname)")
	if err != nil {
		return nil, err
	}
	if addr == "" {
		return nil, nil
	}

	// Check for duplicate before saving
	isDuplicate := addr == host.Address
	for _, existing := range host.FallbackAddresses {
		if existing == addr {
			isDuplicate = true
			break
		}
	}

	if isDuplicate {
		fmt.Fprintf(out, "  Address already configured.\n")
	} else {
		// Save the fallback regardless of whether it works (user might fix it later)
		host.FallbackAddresses = append(host.FallbackAddresses, addr)
		cfg.AddHost(hostName, host)
		if saveErr := config.SaveClientConfig(cfgPath, cfg); saveErr != nil {
			fmt.Fprintf(out, "  Warning: could not save config: %v\n", saveErr)
		} else {
			fmt.Fprintf(out, "  Fallback saved.\n")
		}
	}

	// Test the new address
	fmt.Fprintf(out, "  Testing %s...\n", addr)
	conn := sshpkg.ConnectionFromHost(host).WithAddress(addr)
	if testErr := conn.TestConnection(); testErr != nil {
		fmt.Fprintf(out, "  %s failed: %v\n", addr, testErr)
		return nil, nil
	}

	return conn, nil
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

	// Save initial scan results immediately
	if saveErr := saveFn(projects); saveErr != nil {
		fmt.Fprintf(out, "  Warning: could not save config: %v\n", saveErr)
	}

	return ProjectFlow(in, out, conn, projects, scanFn, saveFn)
}
