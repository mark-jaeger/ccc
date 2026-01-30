package flow

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mark-jaeger/ccc/config"
	sshpkg "github.com/mark-jaeger/ccc/ssh"
	"github.com/mark-jaeger/ccc/tailscale"
	"github.com/mark-jaeger/ccc/ui"
)

// SetupFirstHost walks through adding the first host.
func SetupFirstHost(in io.Reader, out io.Writer, cfgPath string) (*config.ClientConfig, error) {
	fmt.Fprintf(out, "\n  No config found. Let's set up your first host.\n")

	name, host, err := addHostInteractive(in, out)
	if err != nil {
		return nil, err
	}

	cfg := &config.ClientConfig{Hosts: map[string]config.Host{}}
	cfg.AddHost(name, host)

	if err := config.SaveClientConfig(cfgPath, cfg); err != nil {
		return nil, fmt.Errorf("failed to save config: %w", err)
	}
	fmt.Fprintf(out, "  ✓ Host saved.\n")
	return cfg, nil
}

// AddHostFlow adds a new host to an existing config.
func AddHostFlow(in io.Reader, out io.Writer, cfg *config.ClientConfig, cfgPath string) error {
	name, host, err := addHostInteractive(in, out)
	if err != nil {
		return err
	}
	cfg.AddHost(name, host)
	if err := config.SaveClientConfig(cfgPath, cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	fmt.Fprintf(out, "  ✓ Host saved.\n")
	return nil
}

// addHostInteractive walks the user through adding a host, either via
// Tailscale discovery or manual entry. It validates that name, user, and
// address are non-empty, looping on empty input.
func addHostInteractive(in io.Reader, out io.Writer) (string, config.Host, error) {
	var name string
	var host config.Host

	// Try Tailscale discovery
	if tailscale.IsAvailable() {
		fmt.Fprintf(out, "\n  Looking for Tailscale... found.\n")
		hosts, err := tailscale.DiscoverHosts()
		if err == nil && len(hosts) > 0 {
			items := make([]ui.MenuItem, len(hosts))
			for i, h := range hosts {
				items[i] = ui.MenuItem{
					Key:   h.Name,
					Label: h.Name,
					Extra: fmt.Sprintf("%s (%s)", h.IP, h.OS),
				}
			}
			items = append(items, ui.MenuItem{Key: "_manual", Label: "Enter manually"})

			result, err := ui.ShowMenu(in, out, ui.MenuConfig{
				Title: "Tailscale hosts",
				Items: items,
			})
			if err != nil {
				return "", host, err
			}
			if result.Action == ui.ActionQuit {
				return "", host, fmt.Errorf("cancelled")
			}
			if result.Selected.Key != "_manual" {
				// Found via Tailscale
				for _, h := range hosts {
					if h.Name == result.Selected.Key {
						name = h.Name
						host.Address = h.IP
						break
					}
				}
				user, err := ui.Prompt(in, out, fmt.Sprintf("SSH user for %s", name))
				if err != nil {
					return "", host, err
				}
				host.User = user
				return name, host, testAndSetupKeys(in, out, name, &host)
			}
		}
	} else {
		fmt.Fprintf(out, "\n  Looking for Tailscale... not found.\n")
	}

	// Manual entry
	fmt.Fprintf(out, "\n  Enter host details manually:\n")
	var promptErr error
	for {
		name, promptErr = ui.Prompt(in, out, "Name")
		if promptErr != nil {
			return "", host, promptErr
		}
		if name != "" {
			break
		}
		fmt.Fprintf(out, "  Name cannot be empty.\n")
	}
	for {
		host.User, promptErr = ui.Prompt(in, out, "User")
		if promptErr != nil {
			return "", host, promptErr
		}
		if host.User != "" {
			break
		}
		fmt.Fprintf(out, "  User cannot be empty.\n")
	}
	for {
		host.Address, promptErr = ui.Prompt(in, out, "Address")
		if promptErr != nil {
			return "", host, promptErr
		}
		if host.Address != "" {
			break
		}
		fmt.Fprintf(out, "  Address cannot be empty.\n")
	}

	return name, host, testAndSetupKeys(in, out, name, &host)
}

// testAndSetupKeys verifies the SSH connection works. On failure it offers to
// set up SSH keys or try a different user, looping until success or cancel.
func testAndSetupKeys(in io.Reader, out io.Writer, hostName string, host *config.Host) error {
	for {
		conn := sshpkg.ConnectionFromHost(*host)

		fmt.Fprintf(out, "  Testing connection...\n")
		if err := conn.TestConnection(); err != nil {
			fmt.Fprintf(out, "  Authentication failed for %s@%s.\n", host.User, host.Address)

			result, menuErr := ui.ShowMenu(in, out, ui.MenuConfig{
				Title: "Options",
				Items: []ui.MenuItem{
					{Key: "keys", Label: "Set up SSH keys now"},
					{Key: "user", Label: "Try a different user"},
					{Key: "cancel", Label: "Cancel"},
				},
			})
			if menuErr != nil {
				return menuErr
			}

			switch result.Selected.Key {
			case "keys":
				return setupKeysFlow(in, out, host)
			case "user":
				newUser, promptErr := ui.Prompt(in, out, "User")
				if promptErr != nil {
					return promptErr
				}
				host.User = newUser
				continue // retry with new user
			default:
				return fmt.Errorf("cancelled")
			}
		}

		fmt.Fprintf(out, "  ✓ Connected.\n")
		return nil
	}
}

func setupKeysFlow(in io.Reader, out io.Writer, host *config.Host) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}
	sshDir := filepath.Join(home, ".ssh")

	pubKey, err := sshpkg.FindExistingPublicKey(sshDir)
	if err != nil {
		return err
	}

	if pubKey == "" {
		fmt.Fprintf(out, "  No SSH key found. Generating...\n")
		pubKey, err = sshpkg.GenerateKey(sshDir)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "  ✓ Created key.\n")
	} else {
		fmt.Fprintf(out, "  Found %s\n", pubKey)
	}

	fmt.Fprintf(out, "  Copying public key to host...\n")
	if err := sshpkg.CopyKeyToHost(pubKey, host.User, host.Address, host.Port); err != nil {
		return fmt.Errorf("key copy failed: %w", err)
	}

	fmt.Fprintf(out, "  ✓ Key installed. Testing connection...\n")
	conn := sshpkg.ConnectionFromHost(*host)
	if err := conn.TestConnection(); err != nil {
		return fmt.Errorf("still cannot connect after key setup: %w", err)
	}
	fmt.Fprintf(out, "  ✓ Connected.\n")
	return nil
}
