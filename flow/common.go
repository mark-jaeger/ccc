// Package flow orchestrates the user-facing workflows: host selection,
// project browsing, session management, and first-time setup.
package flow

import (
	"fmt"
	"io"
	"strings"

	"github.com/mark-jaeger/ccc/config"
	"github.com/mark-jaeger/ccc/internal/shellutil"
	"github.com/mark-jaeger/ccc/tmux"
	"github.com/mark-jaeger/ccc/ui"
)

// Runner abstracts command execution (SSH or local).
type Runner interface {
	// Run executes a command non-interactively and returns trimmed stdout.
	Run(cmd string) (string, error)
	// RunInteractive executes a command with full stdin/stdout/stderr passthrough.
	RunInteractive(cmd string) error
}

// ProjectFlow handles project selection -> session selection -> attach/create.
// onScan is an optional callback invoked when the user selects [s] Scan.
// onSave is an optional callback invoked after a project is removed, allowing
// the caller to persist the updated config. If nil, removals are in-memory only.
func ProjectFlow(in io.Reader, out io.Writer, runner Runner, projects *config.ProjectsConfig, onScan func(io.Reader, io.Writer) (*config.ProjectsConfig, error), onSave func(*config.ProjectsConfig) error) error {
	for {
		keys := projects.SortedProjectKeys()
		if len(keys) == 0 {
			fmt.Fprintf(out, "\n  No projects configured.\n")
			return nil
		}

		items := make([]ui.MenuItem, len(keys))
		for i, k := range keys {
			p := projects.Projects[k]
			items[i] = ui.MenuItem{Key: k, Label: k, Extra: p.Path}
		}

		result, err := ui.ShowMenu(in, out, ui.MenuConfig{
			Title:    "Projects",
			Items:    items,
			ShowBack: true,
			ExtraActions: []ui.ExtraAction{
				{Key: "s", Label: "Scan for projects", ID: "scan"},
			},
		})
		if err != nil {
			return err
		}

		switch result.Action {
		case ui.ActionQuit, ui.ActionBack:
			return nil
		case ui.ActionExtra:
			if result.ExtraKey == "scan" {
				if onScan == nil {
					fmt.Fprintf(out, "\n  Scan not available in this mode.\n")
					continue
				}
				newProjects, err := onScan(in, out)
				if err != nil {
					return err
				}
				if newProjects != nil {
					projects = newProjects
				}
				continue
			}
		case ui.ActionSelect:
			projectKey := result.Selected.Key
			projectPath := projects.Projects[projectKey].Path

			// Validate project path exists
			checkCmd := fmt.Sprintf("test -d %s", shellutil.Quote(projectPath))
			if _, err := runner.Run(checkCmd); err != nil {
				fmt.Fprintf(out, "\n  Path %s not found.\n", projectPath)
				answer, confirmErr := ui.Confirm(in, out, "Remove from projects?")
				if confirmErr != nil {
					return confirmErr
				}
				if answer {
					delete(projects.Projects, projectKey)
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

// SessionFlow handles session listing -> attach or create.
func SessionFlow(in io.Reader, out io.Writer, runner Runner, projectKey, projectPath string) error {
	if err := CheckTmux(in, out, runner); err != nil {
		return err
	}

	for {
		listCmd := tmux.BuildListCommand()
		listOutput, err := runner.Run(listCmd)
		if err != nil {
			// Could be "no server" — treat as zero sessions
			listOutput = ""
		}

		allSessions := tmux.ParseSessionList(listOutput)
		sessions := tmux.FilterSessionsForProject(allSessions, projectKey)

		// Auto-skip: zero sessions -> create
		if len(sessions) == 0 {
			return createSession(in, out, runner, projectKey, projectPath, sessions)
		}

		// Show session menu
		items := make([]ui.MenuItem, len(sessions))
		for i, s := range sessions {
			extra := fmt.Sprintf("(%d windows)", s.Windows)
			if !s.Verified {
				extra += " (unverified)"
			}
			items[i] = ui.MenuItem{Key: s.Name, Label: s.Name, Extra: extra}
		}

		result, err := ui.ShowMenu(in, out, ui.MenuConfig{
			Title:      fmt.Sprintf("Sessions for %s", projectKey),
			Items:      items,
			ShowBack:   true,
			ShowRemove: true,
			ExtraActions: []ui.ExtraAction{
				{Key: "d", Label: "Detach clients", ID: "detach", ItemAction: true},
				{Key: "n", Label: "New session", ID: "new"},
			},
		})
		if err != nil {
			return err
		}

		switch result.Action {
		case ui.ActionQuit, ui.ActionBack:
			return nil
		case ui.ActionExtra:
			if result.ExtraKey == "detach" {
				detachSessionClients(out, runner, result.Selected)
				continue
			}
			return createSession(in, out, runner, projectKey, projectPath, sessions)
		case ui.ActionRemove:
			return removeSession(in, out, runner, result.Selected, sessions)
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

func attachSession(in io.Reader, out io.Writer, runner Runner, session tmux.Session) error {
	if !session.Verified {
		fmt.Fprintf(out, "\n  Session %q matches by name but wasn't created by ccc.\n", session.Name)
		answer, err := ui.Confirm(in, out, "Proceed?")
		if err != nil || !answer {
			return err
		}
	}

	// Check for other clients
	clientOutput, clientErr := runner.Run(tmux.BuildListClientsCommand(session.Name))
	if clientErr != nil {
		fmt.Fprintf(out, "  Warning: could not list clients: %v\n", clientErr)
	}
	clients := tmux.ParseClientList(clientOutput)
	if len(clients) > 0 {
		c := clients[0]
		fmt.Fprintf(out, "\n  This session is attached from another client (%dx%d).\n", c.Width, c.Height)

		detachResult, err := ui.ShowMenu(in, out, ui.MenuConfig{
			Title: "Options",
			Items: []ui.MenuItem{
				{Key: "attach", Label: fmt.Sprintf("Attach anyway (layout constrained to %dx%d)", c.Width, c.Height)},
				{Key: "detach", Label: "Detach other client and attach (full resolution)"},
				{Key: "cancel", Label: "Cancel"},
			},
		})
		if err != nil {
			return err
		}
		if detachResult.Action == ui.ActionQuit || detachResult.Selected.Key == "cancel" {
			return nil
		}
		if detachResult.Selected.Key == "detach" {
			if _, detachErr := runner.Run(tmux.BuildDetachClientsCommand(session.Name)); detachErr != nil {
				fmt.Fprintf(out, "  Warning: could not detach clients: %v\n", detachErr)
			}
		}
	}

	// Ensure bell/passthrough options are set (fixes sessions from older ccc versions).
	runner.Run(tmux.BuildEnsureNotifyOptionsCommand(session.Name))

	fmt.Fprintf(out, "\n  Attaching to %s...\n", session.Name)
	return runner.RunInteractive(tmux.BuildAttachCommand(session.Name))
}

func createSession(in io.Reader, out io.Writer, runner Runner, projectKey, projectPath string, existing []tmux.Session) error {
	autoName := tmux.NextAutoName(projectKey, existing)
	namePrompt := fmt.Sprintf("Session name (enter for %q)", autoName)
	name, err := ui.Prompt(in, out, namePrompt)
	if err != nil {
		return err
	}
	if name == "" {
		name = autoName
	} else if name != projectKey && !strings.HasPrefix(name, projectKey+"-") {
		name = projectKey + "-" + name
	}

	createCmd := tmux.BuildCreateCommand(name, projectPath, projectKey)
	if _, err := runner.Run(createCmd); err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	// Set one-shot notification hooks and passthrough (tmux >= 3.3, ignore errors on older).
	runner.Run(tmux.BuildSetNotifyHooksCommand(name))
	runner.Run(tmux.BuildSetPassthroughCommand(name))

	fmt.Fprintf(out, "  \u2713 Created session %s\n", name)
	return runner.RunInteractive(tmux.BuildAttachCommand(name))
}

func detachSessionClients(out io.Writer, runner Runner, item ui.MenuItem) {
	clientOutput, _ := runner.Run(tmux.BuildListClientsCommand(item.Key))
	clients := tmux.ParseClientList(clientOutput)
	if len(clients) == 0 {
		fmt.Fprintf(out, "  No clients attached to %s.\n", item.Key)
		return
	}
	if _, err := runner.Run(tmux.BuildDetachClientsCommand(item.Key)); err != nil {
		fmt.Fprintf(out, "  Warning: could not detach clients: %v\n", err)
	} else {
		fmt.Fprintf(out, "  Detached %d client(s) from %s.\n", len(clients), item.Key)
	}
}

func removeSession(in io.Reader, out io.Writer, runner Runner, item ui.MenuItem, sessions []tmux.Session) error {
	for _, s := range sessions {
		if s.Name == item.Key && !s.Verified {
			fmt.Fprintf(out, "\n  Warning: session %q wasn't created by ccc.\n", s.Name)
			break
		}
	}

	killCmd := tmux.BuildKillCommand(item.Key)
	if _, err := runner.Run(killCmd); err != nil {
		return fmt.Errorf("failed to kill session: %w", err)
	}
	fmt.Fprintf(out, "  \u2713 Killed session %s\n", item.Key)
	return nil
}
