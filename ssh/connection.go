// Package ssh provides SSH connection management, key discovery, and remote
// command execution.
package ssh

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/mark-jaeger/ccc/config"
	"github.com/mark-jaeger/ccc/internal/shellutil"
)

// ConnectionFromHost creates a Connection from a config.Host, copying all
// relevant fields.
func ConnectionFromHost(h config.Host) *Connection {
	return &Connection{
		User:         h.User,
		Address:      h.Address,
		Port:         h.Port,
		IdentityFile: h.IdentityFile,
		ProxyJump:    h.ProxyJump,
		SSHOptions:   h.SSHOptions,
	}
}

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
	args = append(args, c.SSHOptions...)

	return args
}

// buildNonInteractiveArgs constructs the ssh argument list for non-interactive
// command execution. It enables BatchMode, sets a connect timeout, and uses
// StrictHostKeyChecking=accept-new for trust-on-first-use (TOFU) semantics:
// new host keys are accepted automatically, but changed keys are rejected.
func (c *Connection) buildNonInteractiveArgs(cmd string) []string {
	args := c.commonArgs()
	args = append(args,
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "ConnectTimeout=10",
	)
	return c.appendRemoteCmd(args, cmd)
}

// buildInteractiveArgs constructs the ssh argument list for interactive command
// execution with PTY allocation.
func (c *Connection) buildInteractiveArgs(cmd string) []string {
	args := c.commonArgs()
	args = append(args, "-t")
	return c.appendRemoteCmd(args, cmd)
}

// appendRemoteCmd adds the target and shell-quoted remote command to args.
func (c *Connection) appendRemoteCmd(args []string, cmd string) []string {
	args = append(args, c.target())
	args = append(args, "bash -lc "+shellutil.Quote(cmd))
	return args
}

// Run executes a remote command non-interactively and returns the
// trimmed stdout output. Connection implements the flow.Runner interface.
func (c *Connection) Run(cmd string) (string, error) {
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
	out, err := c.Run("echo ok")
	if err != nil {
		return fmt.Errorf("connection test failed: %w", err)
	}
	if out != "ok" {
		return fmt.Errorf("connection test: expected 'ok', got %q", out)
	}
	return nil
}
