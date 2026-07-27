package flow

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/mark-jaeger/ccc/v2/config"
	"github.com/mark-jaeger/ccc/v2/scan"
	sshpkg "github.com/mark-jaeger/ccc/v2/ssh"
	"github.com/mark-jaeger/ccc/v2/ui"
)

// RunScanFlow discovers projects on a host and saves projects.toml.
func RunScanFlow(in io.Reader, out io.Writer, conn *sshpkg.Connection, hostName string) (*config.ProjectsConfig, error) {
	// Get home directory
	homeDir, err := conn.Run("echo $HOME")
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}

	fmt.Fprintf(out, "\n  Scanning for git repositories in %s...\n", homeDir)
	scanCmd := scan.BuildScanChainCommand(homeDir)
	scanOutput, err := conn.Run(scanCmd)
	if err != nil {
		return nil, fmt.Errorf("scan failed: %w", err)
	}
	if strings.TrimSpace(scanOutput) == "" {
		return handleNoResults(in, out, conn, hostName)
	}

	results := scan.ParseScanResults(scanOutput)
	if len(results) == 0 {
		return handleNoResults(in, out, conn, hostName)
	}

	return selectProjects(in, out, results)
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
		conn.RunInteractive("$SHELL -l")

		// Rescan after shell exit
		fmt.Fprintf(out, "\n  Rescanning...\n")
		homeDir, homeErr := conn.Run("echo $HOME")
		if homeErr != nil {
			return nil, fmt.Errorf("cannot determine home directory: %w", homeErr)
		}
		shellScanCmd := scan.BuildScanChainCommand(homeDir)
		shellScanOutput, shellErr := conn.Run(shellScanCmd)
		if shellErr != nil {
			return nil, fmt.Errorf("rescan failed: %w", shellErr)
		}
		results := scan.ParseScanResults(shellScanOutput)
		if len(results) == 0 {
			fmt.Fprintf(out, "  Still no projects found.\n")
			return nil, nil
		}
		return selectProjects(in, out, results)
	}

	// Manual path
	path, err := ui.Prompt(in, out, "Path to scan")
	if err != nil {
		return nil, err
	}
	pathScanCmd := scan.BuildScanChainCommand(path)
	pathScanOutput, pathErr := conn.Run(pathScanCmd)
	if pathErr != nil {
		return nil, fmt.Errorf("scan of %s failed: %w", path, pathErr)
	}
	results := scan.ParseScanResults(pathScanOutput)
	if len(results) == 0 {
		fmt.Fprintf(out, "  No projects found at %s.\n", path)
		return nil, nil
	}
	return selectProjects(in, out, results)
}

func selectProjects(in io.Reader, out io.Writer, results []scan.ScanResult) (*config.ProjectsConfig, error) {
	fmt.Fprintf(out, "\n  Found %d projects:\n", len(results))
	for i, r := range results {
		fmt.Fprintf(out, "  [%d] %-20s %s\n", i+1, r.Name, r.Path)
	}

	answer, err := ui.Prompt(in, out, "Select projects to add (comma-separated, or 'a' for all)")
	if err != nil {
		return nil, err
	}

	projects := &config.ProjectsConfig{Projects: []config.Project{}}

	if answer == "a" || answer == "A" {
		for _, r := range results {
			key := scan.DeriveProjectKey(r.Path)
			projects.Projects = append(projects.Projects, config.Project{Name: key, Path: r.Path})
		}
	} else {
		for _, part := range strings.Split(answer, ",") {
			idx, err := strconv.Atoi(strings.TrimSpace(part))
			if err == nil && idx >= 1 && idx <= len(results) {
				r := results[idx-1]
				key := scan.DeriveProjectKey(r.Path)
				projects.Projects = append(projects.Projects, config.Project{Name: key, Path: r.Path})
			}
		}
	}

	if len(projects.Projects) == 0 {
		fmt.Fprintf(out, "  No projects selected.\n")
		return nil, nil
	}

	fmt.Fprintf(out, "  Selected %d project(s).\n", len(projects.Projects))
	return projects, nil
}
