package ssh

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Connection holds SSH connection parameters.
type Connection struct {
	User         string
	Address      string
	Port         int
	IdentityFile string
	ProxyJump    string
	SSHOptions   []string
}

// target returns user@address.
func (c *Connection) target() string {
	return c.User + "@" + c.Address
}

// commonArgs builds the shared argument list: port, identity file, proxy jump,
// and any extra SSH options.
func (c *Connection) commonArgs() []string {
	var args []string

	if c.Port != 0 {
		args = append(args, "-p", strconv.Itoa(c.Port))
	}
	if c.IdentityFile != "" {
		args = append(args, "-i", c.IdentityFile)
	}
	if c.ProxyJump != "" {
		args = append(args, "-J", c.ProxyJump)
	}
	for _, opt := range c.SSHOptions {
		args = append(args, "-o", opt)
	}

	return args
}

// buildNonInteractiveArgs constructs the ssh argument list for non-interactive
// command execution. It enables BatchMode, accepts new host keys, sets a
// connect timeout, and wraps the remote command in bash -lc.
func (c *Connection) buildNonInteractiveArgs(cmd string) []string {
	args := c.commonArgs()
	args = append(args,
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "ConnectTimeout=10",
	)
	args = append(args, c.target())
	args = append(args, "bash", "-lc", cmd)
	return args
}

// buildInteractiveArgs constructs the ssh argument list for interactive command
// execution. It requests a PTY with -t and wraps the remote command in bash -lc.
func (c *Connection) buildInteractiveArgs(cmd string) []string {
	args := c.commonArgs()
	args = append(args, "-t")
	args = append(args, c.target())
	args = append(args, "bash", "-lc", cmd)
	return args
}

// RunCommand executes a remote command non-interactively and returns the
// trimmed stdout output.
func (c *Connection) RunCommand(cmd string) (string, error) {
	args := c.buildNonInteractiveArgs(cmd)
	out, err := exec.Command("ssh", args...).Output()
	if err != nil {
		return "", fmt.Errorf("ssh command failed: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// RunInteractive executes a remote command interactively, handing over
// stdin, stdout, and stderr to the calling process.
func (c *Connection) RunInteractive(cmd string) error {
	args := c.buildInteractiveArgs(cmd)
	proc := exec.Command("ssh", args...)
	proc.Stdin = os.Stdin
	proc.Stdout = os.Stdout
	proc.Stderr = os.Stderr
	return proc.Run()
}

// TestConnection verifies that the SSH connection works by running "echo ok".
func (c *Connection) TestConnection() error {
	out, err := c.RunCommand("echo ok")
	if err != nil {
		return fmt.Errorf("connection test failed: %w", err)
	}
	if out != "ok" {
		return fmt.Errorf("connection test: expected 'ok', got %q", out)
	}
	return nil
}
