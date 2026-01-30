package flow

import (
	"fmt"
	"io"
	"strings"

	"github.com/markjd/ccc/config"
	"github.com/markjd/ccc/tmux"
	"github.com/markjd/ccc/ui"
)

// Runner abstracts command execution (SSH or local).
type Runner interface {
	Run(cmd string) (string, error)
	RunInteractive(cmd string) error
}

// ProjectFlow handles project selection -> session selection -> attach/create.
func ProjectFlow(in io.Reader, out io.Writer, runner Runner, projects *config.ProjectsConfig) error {
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
			{Key: "s", Label: "Scan for projects", Action: "scan"},
		},
	})
	if err != nil {
		return err
	}

	switch result.Action {
	case ui.ActionQuit:
		return nil
	case ui.ActionBack:
		return nil
	case ui.ActionExtra:
		if result.ExtraKey == "scan" {
			fmt.Fprintf(out, "\n  Scan not yet implemented in this flow.\n")
			return nil
		}
	case ui.ActionSelect:
		return SessionFlow(in, out, runner, result.Selected.Key, projects.Projects[result.Selected.Key].Path)
	}
	return nil
}

// SessionFlow handles session listing -> attach or create.
func SessionFlow(in io.Reader, out io.Writer, runner Runner, projectKey, projectPath string) error {
	// Check tmux is available
	if err := CheckTmux(in, out, runner); err != nil {
		return err
	}

	// List sessions
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

	// Auto-skip: one session -> attach
	if len(sessions) == 1 {
		return attachSession(in, out, runner, sessions[0])
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
			{Key: "n", Label: "New session", Action: "new"},
		},
	})
	if err != nil {
		return err
	}

	switch result.Action {
	case ui.ActionQuit:
		return nil
	case ui.ActionBack:
		return nil
	case ui.ActionExtra:
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

func attachSession(in io.Reader, out io.Writer, runner Runner, session tmux.Session) error {
	if !session.Verified {
		fmt.Fprintf(out, "\n  Session %q matches by name but wasn't created by ccc.\n", session.Name)
		answer, err := ui.Confirm(in, out, "Proceed?")
		if err != nil || !answer {
			return err
		}
	}

	// Check for other clients
	clientOutput, _ := runner.Run(tmux.BuildListClientsCommand(session.Name))
	clients := tmux.ParseClientList(clientOutput)
	if len(clients) > 0 {
		c := clients[0]
		fmt.Fprintf(out, "\n  This session is attached from another client (%dx%d).\n", c.Width, c.Height)

		detachResult, err := ui.ShowMenu(in, out, ui.MenuConfig{
			Title: "Options",
			Items: []ui.MenuItem{
				{Key: "attach", Label: fmt.Sprintf("Attach anyway (layout constrained to %dx%d)", c.Width, c.Height)},
				{Key: "detach", Label: "Detach other client and attach (full resolution)"},
			},
		})
		if err != nil {
			return err
		}
		if detachResult.Action == ui.ActionQuit {
			return nil
		}
		if detachResult.Selected.Key == "detach" {
			runner.Run(tmux.BuildDetachClientsCommand(session.Name))
		}
	}

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

	fmt.Fprintf(out, "  \u2713 Created session %s\n", name)
	return runner.RunInteractive(tmux.BuildAttachCommand(name))
}

func removeSession(in io.Reader, out io.Writer, runner Runner, item ui.MenuItem, sessions []tmux.Session) error {
	for _, s := range sessions {
		if s.Name == item.Key && !s.Verified {
			fmt.Fprintf(out, "\n  Warning: session %q wasn't created by ccc.\n", s.Name)
		}
	}

	killCmd := tmux.BuildKillCommand(item.Key)
	if _, err := runner.Run(killCmd); err != nil {
		return fmt.Errorf("failed to kill session: %w", err)
	}
	fmt.Fprintf(out, "  \u2713 Killed session %s\n", item.Key)
	return nil
}
