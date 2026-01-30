package flow

import (
	"fmt"
	"io"
	"strings"

	"github.com/markjd/ccc/config"
	"github.com/markjd/ccc/scan"
	sshpkg "github.com/markjd/ccc/ssh"
	"github.com/markjd/ccc/ui"
)

// RunScanFlow discovers projects on a host and saves projects.toml.
func RunScanFlow(in io.Reader, out io.Writer, conn *sshpkg.Connection, hostName string) (*config.ProjectsConfig, error) {
	// Get home directory
	homeDir, err := conn.RunCommand("echo $HOME")
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}

	fmt.Fprintf(out, "\n  Scanning for git repositories in %s...\n", homeDir)
	scanCmd := scan.BuildScanChainCommand(homeDir)
	scanOutput, err := conn.RunCommand(scanCmd)
	if err != nil || strings.TrimSpace(scanOutput) == "" {
		return handleNoResults(in, out, conn, hostName)
	}

	results := scan.ParseScanResults(scanOutput)
	if len(results) == 0 {
		return handleNoResults(in, out, conn, hostName)
	}

	return selectProjects(in, out, conn, results)
}

func handleNoResults(in io.Reader, out io.Writer, conn *sshpkg.Connection, hostName string) (*config.ProjectsConfig, error) {
	fmt.Fprintf(out, "\n  No git repositories found.\n")

	result, err := ui.ShowMenu(in, out, ui.MenuConfig{
		Title: "Options",
		Items: []ui.MenuItem{
			{Key: "path", Label: "Enter a path to scan"},
			{Key: "shell", Label: fmt.Sprintf("Open a shell on %s", hostName)},
		},
	})
	if err != nil || result.Action == ui.ActionQuit {
		return nil, err
	}

	if result.Selected.Key == "shell" {
		fmt.Fprintf(out, "\n  Opening shell. Type 'exit' when done.\n")
		conn.RunInteractive("bash -l")

		// Rescan after shell exit
		fmt.Fprintf(out, "\n  Rescanning...\n")
		homeDir, _ := conn.RunCommand("echo $HOME")
		shellScanCmd := scan.BuildScanChainCommand(homeDir)
		shellScanOutput, _ := conn.RunCommand(shellScanCmd)
		results := scan.ParseScanResults(shellScanOutput)
		if len(results) == 0 {
			fmt.Fprintf(out, "  Still no projects found.\n")
			return nil, nil
		}
		return selectProjects(in, out, conn, results)
	}

	// Manual path
	path, err := ui.Prompt(in, out, "Path to scan")
	if err != nil {
		return nil, err
	}
	pathScanCmd := scan.BuildScanChainCommand(path)
	pathScanOutput, _ := conn.RunCommand(pathScanCmd)
	results := scan.ParseScanResults(pathScanOutput)
	if len(results) == 0 {
		fmt.Fprintf(out, "  No projects found at %s.\n", path)
		return nil, nil
	}
	return selectProjects(in, out, conn, results)
}

func selectProjects(in io.Reader, out io.Writer, conn *sshpkg.Connection, results []scan.ScanResult) (*config.ProjectsConfig, error) {
	fmt.Fprintf(out, "\n  Found %d projects:\n", len(results))
	for i, r := range results {
		fmt.Fprintf(out, "  [%d] %-20s %s\n", i+1, r.Name, r.Path)
	}

	answer, err := ui.Prompt(in, out, "Select projects to add (comma-separated, or 'a' for all)")
	if err != nil {
		return nil, err
	}

	projects := &config.ProjectsConfig{Projects: map[string]config.Project{}}

	if answer == "a" || answer == "A" {
		for _, r := range results {
			key := scan.DeriveProjectKey(r.Path)
			projects.Projects[key] = config.Project{Path: r.Path}
		}
	} else {
		for _, part := range strings.Split(answer, ",") {
			part = strings.TrimSpace(part)
			idx := 0
			fmt.Sscanf(part, "%d", &idx)
			if idx >= 1 && idx <= len(results) {
				r := results[idx-1]
				key := scan.DeriveProjectKey(r.Path)
				projects.Projects[key] = config.Project{Path: r.Path}
			}
		}
	}

	if len(projects.Projects) == 0 {
		fmt.Fprintf(out, "  No projects selected.\n")
		return nil, nil
	}

	// Save to host
	data, err := config.SerializeProjectsConfig(projects)
	if err != nil {
		return nil, err
	}
	writeCmd := fmt.Sprintf("mkdir -p ~/.ccc && cat > ~/.ccc/projects.toml << 'CCCEOF'\n%s\nCCCEOF", string(data))
	if _, err := conn.RunCommand(writeCmd); err != nil {
		return nil, fmt.Errorf("failed to write projects.toml: %w", err)
	}
	fmt.Fprintf(out, "  Saved %d projects to ~/.ccc/projects.toml\n", len(projects.Projects))

	return projects, nil
}
