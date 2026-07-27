// Package flow orchestrates the user-facing workflows: host selection,
// project browsing, session management, and first-time setup.
package flow

import (
	"fmt"
	"io"

	"github.com/mark-jaeger/ccc/v2/config"
	"github.com/mark-jaeger/ccc/v2/internal/shellutil"
	"github.com/mark-jaeger/ccc/v2/ui"
	"github.com/mark-jaeger/ccc/v2/zmx"
)

// Runner abstracts command execution (SSH or local).
type Runner interface {
	// Run executes a command non-interactively and returns trimmed stdout.
	Run(cmd string) (string, error)
	// RunInteractive executes a command with full stdin/stdout/stderr passthrough.
	RunInteractive(cmd string) error
}

// removeProject removes the project with the given name from the config in-place.
func removeProject(projects *config.ProjectsConfig, name string) {
	for i, p := range projects.Projects {
		if p.Name == name {
			projects.Projects = append(projects.Projects[:i], projects.Projects[i+1:]...)
			return
		}
	}
}

// Deprecated: ProjectFlow is replaced by the TUI package.
// Kept for backwards compatibility and testing.
//
// ProjectFlow handles project selection -> session selection -> attach/create.
// onScan is an optional callback invoked when the user selects [s] Scan.
// onSave is an optional callback invoked after a project is removed, allowing
// the caller to persist the updated config. If nil, removals are in-memory only.
func ProjectFlow(in io.Reader, out io.Writer, runner Runner, projects *config.ProjectsConfig, onScan func(io.Reader, io.Writer) (*config.ProjectsConfig, error), onSave func(*config.ProjectsConfig) error) error {
	for {
		if len(projects.Projects) == 0 {
			fmt.Fprintf(out, "\n  No projects configured.\n")
			return nil
		}

		items := make([]ui.MenuItem, len(projects.Projects))
		for i, p := range projects.Projects {
			items[i] = ui.MenuItem{Key: p.Name, Label: p.Name, Extra: p.Path}
		}

		result, err := ui.ShowMenu(in, out, ui.MenuConfig{
			Title:    "Projects",
			Items:    items,
			ShowBack: true,
			ExtraActions: []ui.ExtraAction{
				{Key: "s", Label: "Scan for projects", ID: "scan"},
				{Key: "d", Label: "Delete project", ID: "delete", ItemAction: true},
			},
		})
		if err != nil {
			return err
		}

		switch result.Action {
		case ui.ActionQuit, ui.ActionBack:
			return nil
		case ui.ActionExtra:
			switch result.ExtraKey {
			case "scan":
				if onScan == nil {
					fmt.Fprintf(out, "\n  Scan not available in this mode.\n")
					continue
				}
				newProjects, err := onScan(in, out)
				if err != nil {
					return err
				}
				if newProjects != nil {
					for _, p := range newProjects.Projects {
						// Add or update project by name
						found := false
						for i, existing := range projects.Projects {
							if existing.Name == p.Name {
								projects.Projects[i] = p
								found = true
								break
							}
						}
						if !found {
							projects.Projects = append(projects.Projects, p)
						}
					}
					if onSave != nil {
						if saveErr := onSave(projects); saveErr != nil {
							fmt.Fprintf(out, "  Warning: could not save config: %v\n", saveErr)
						}
					}
				}
				continue
			case "delete":
				if err := deleteProject(in, out, projects, result.Selected, onSave); err != nil {
					return err
				}
				continue
			}
		case ui.ActionSelect:
			projectKey := result.Selected.Key
			p, ok := projects.ProjectByName(projectKey)
			if !ok {
				continue
			}
			projectPath := p.Path

			// Validate project path exists
			checkCmd := fmt.Sprintf("test -d %s", shellutil.Quote(projectPath))
			if _, err := runner.Run(checkCmd); err != nil {
				fmt.Fprintf(out, "\n  Path %s not found.\n", projectPath)
				answer, confirmErr := ui.Confirm(in, out, "Remove from projects?")
				if confirmErr != nil {
					return confirmErr
				}
				if answer {
					removeProject(projects, projectKey)
					fmt.Fprintf(out, "  Removed %s.\n", projectKey)
					if onSave != nil {
						if saveErr := onSave(projects); saveErr != nil {
							fmt.Fprintf(out, "  Warning: could not save config: %v\n", saveErr)
						}
					}
				}
				continue
			}

			return SessionFlow(in, out, runner, projectKey, projectPath)
		}
	}
}

// Deprecated: SessionFlow is replaced by the TUI package.
// Kept for backwards compatibility and testing.
//
// SessionFlow handles session listing -> attach or create.
func SessionFlow(in io.Reader, out io.Writer, runner Runner, projectKey, projectPath string) error {
	if err := CheckZmx(in, out, runner); err != nil {
		return err
	}

	for {
		listCmd := zmx.BuildListCommand()
		listOutput, err := runner.Run(listCmd)
		if err != nil {
			// Could be "no server" — treat as zero sessions
			listOutput = ""
		}

		allSessions := zmx.ParseListOutput(listOutput)
		sessions := zmx.FilterSessionsForProject(allSessions, projectKey)

		// Auto-skip: zero sessions -> create
		if len(sessions) == 0 {
			return createSession(in, out, runner, projectKey, projectPath, sessions)
		}

		// Show session menu
		items := make([]ui.MenuItem, len(sessions))
		for i, s := range sessions {
			extra := ""
			if s.External {
				extra = "(external)"
			}
			items[i] = ui.MenuItem{Key: s.Name, Label: s.Name, Extra: extra}
		}

		result, err := ui.ShowMenu(in, out, ui.MenuConfig{
			Title:      fmt.Sprintf("Sessions for %s", projectKey),
			Items:      items,
			ShowBack:   true,
			ShowRemove: false,
			ExtraActions: []ui.ExtraAction{
				{Key: "n", Label: "New session", ID: "new"},
				{Key: "x", Label: "Kill session", ID: "kill", ItemAction: true},
			},
		})
		if err != nil {
			return err
		}

		switch result.Action {
		case ui.ActionQuit, ui.ActionBack:
			return nil
		case ui.ActionExtra:
			switch result.ExtraKey {
			case "kill":
				if err := killSession(in, out, runner, result.Selected, sessions); err != nil {
					return err
				}
				continue
			case "new":
				return createSession(in, out, runner, projectKey, projectPath, sessions)
			}
		case ui.ActionSelect:
			for _, s := range sessions {
				if s.Name == result.Selected.Key {
					return attachSession(in, out, runner, s)
				}
			}
		}
		return nil
	}
}

func attachSession(in io.Reader, out io.Writer, runner Runner, session zmx.Session) error {
	if session.External {
		fmt.Fprintf(out, "\n  Session %q is an external session (not created by ccc).\n", session.Name)
		answer, err := ui.Confirm(in, out, "Proceed?")
		if err != nil || !answer {
			return err
		}
	}

	fmt.Fprintf(out, "\n  Attaching to %s...\n", session.Name)
	return runner.RunInteractive(zmx.BuildAttachCommand(session.Name))
}

func createSession(in io.Reader, out io.Writer, runner Runner, projectKey, projectPath string, existing []zmx.Session) error {
	name := zmx.NextAutoName(projectKey, existing)

	fmt.Fprintf(out, "  Created session %s\n", name)
	return runner.RunInteractive(zmx.BuildCreateCommand(name, projectPath))
}

func killSession(in io.Reader, out io.Writer, runner Runner, item ui.MenuItem, sessions []zmx.Session) error {
	for _, s := range sessions {
		if s.Name == item.Key && s.External {
			fmt.Fprintf(out, "\n  Warning: session %q wasn't created by ccc.\n", s.Name)
			break
		}
	}

	answer, err := ui.Confirm(in, out, fmt.Sprintf("Kill %s?", item.Key))
	if err != nil {
		return err
	}
	if !answer {
		return nil
	}

	killCmd := zmx.BuildKillCommand(item.Key)
	if _, err := runner.Run(killCmd); err != nil {
		return fmt.Errorf("failed to kill session: %w", err)
	}
	fmt.Fprintf(out, "  Killed session %s\n", item.Key)
	return nil
}

func deleteProject(in io.Reader, out io.Writer, projects *config.ProjectsConfig, item ui.MenuItem, onSave func(*config.ProjectsConfig) error) error {
	answer, err := ui.Confirm(in, out, fmt.Sprintf("Delete %s?", item.Key))
	if err != nil {
		return err
	}
	if !answer {
		return nil
	}

	// Store for potential rollback
	oldProject, found := projects.ProjectByName(item.Key)
	removeProject(projects, item.Key)

	if onSave != nil {
		if saveErr := onSave(projects); saveErr != nil {
			// Rollback the in-memory deletion
			if found {
				projects.Projects = append(projects.Projects, oldProject)
			}
			return fmt.Errorf("failed to delete project: %w", saveErr)
		}
	}

	fmt.Fprintf(out, "  \u2713 Deleted %s\n", item.Key)
	return nil
}
