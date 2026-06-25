// Package ssh provides SSH connection management, key discovery, and remote
// command execution.
package ssh

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mark-jaeger/ccc/config"
	"github.com/mark-jaeger/ccc/internal/shellutil"
)

// execCommandContext is a seam over exec.CommandContext so tests can substitute
// a harmless local command (e.g. sleep) for ssh and exercise cancellation
// without touching the network.
var execCommandContext = exec.CommandContext

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

// WithAddress returns a copy of the connection with a different address.
func (c *Connection) WithAddress(addr string) *Connection {
	copy := *c
	copy.Address = addr
	return &copy
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
//
// ConnectTimeout only bounds the initial handshake, so it adds the
// ServerAliveInterval/CountMax probes to also detect an established-then-dead
// link (a flaky train Wi-Fi): with 5s probes and a count of 3, a wedged
// connection self-terminates in ~15s instead of hanging for minutes.
// TCPKeepAlive=no avoids spoofable kernel-level keepalives in favor of these
// encrypted application-level ones.
func (c *Connection) buildNonInteractiveArgs(cmd string) []string {
	args := c.commonArgs()
	args = append(args,
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "ConnectTimeout=10",
		"-o", "ServerAliveInterval=5",
		"-o", "ServerAliveCountMax=3",
		"-o", "TCPKeepAlive=no",
	)
	args = c.appendControlMaster(args)
	return c.appendRemoteCmd(args, cmd)
}

// buildInteractiveArgs constructs the ssh argument list for interactive command
// execution with PTY allocation.
//
// Keepalives are gentler here than for non-interactive commands (10s vs 5s
// interval, same count of 3, so ~30s vs ~15s): an attach is a long-lived PTY
// that should ride out short tunnel hiccups without dropping the user, yet still
// tear down a genuinely dead link rather than freeze the terminal. The -o
// options must precede -t (and thus the target host) or ssh ignores them.
//
// ControlMaster multiplexing is deliberately NOT applied to interactive
// attaches: a persistent master buys nothing for a single long-lived session
// and only complicates teardown.
func (c *Connection) buildInteractiveArgs(cmd string) []string {
	args := c.commonArgs()
	args = append(args,
		"-o", "ServerAliveInterval=10",
		"-o", "ServerAliveCountMax=3",
		"-o", "TCPKeepAlive=no",
		"-t",
	)
	return c.appendRemoteCmd(args, cmd)
}

// appendRemoteCmd adds the target and shell-quoted remote command to args.
func (c *Connection) appendRemoteCmd(args []string, cmd string) []string {
	args = append(args, c.target())
	args = append(args, "$SHELL -lc "+shellutil.Quote(cmd))
	return args
}

// appendControlMaster enables SSH connection multiplexing so a burst of
// non-interactive commands shares one transport instead of paying a fresh
// handshake each time. ControlPersist keeps the master alive 60s past the last
// client for follow-up commands.
//
// The keepalive probes added in buildNonInteractiveArgs are a prerequisite:
// without them a half-dead master would leave a stale socket that hangs the
// *next* command indefinitely (defeating the whole point of this PR). With
// them, a wedged master self-terminates and the next command transparently
// spins up a fresh one.
//
// The socket lives in a tool-owned, 0700, absolute dir under ~/.ccc/cm, named by
// controlPathToken (a hash of the full effective connection) which keeps the
// path well under the ~104-char unix-socket limit. If the home dir can't be
// resolved or the dir can't be created, multiplexing is skipped gracefully — a
// missing optimization, not a hard failure.
func (c *Connection) appendControlMaster(args []string) []string {
	dir, err := controlMasterDir()
	if err != nil {
		return args
	}
	return append(args,
		"-o", "ControlMaster=auto",
		"-o", "ControlPath="+filepath.Join(dir, c.controlPathToken()),
		"-o", "ControlPersist=60",
	)
}

// controlPathToken derives a unique, filesystem-safe socket name from every
// connection-identifying field. OpenSSH's own %C token hashes only
// local-host/remote-host/port/user (and jump host), NOT IdentityFile,
// ProxyJump, or arbitrary SSHOptions such as ProxyCommand — verified with
// `ssh -G`, two configs differing only in IdentityFile or ProxyCommand expand to
// the same %C. Since ccc lets each host carry its own key and ssh_options, %C
// would let a command silently multiplex over a master built for a *different*
// auth/routing config, crossing a trust boundary. Hashing the full effective
// Connection instead keeps each distinct config on its own master while the
// 32-hex-char result stays comfortably under the unix-socket path limit.
func (c *Connection) controlPathToken() string {
	h := sha256.New()
	// Length-prefix each field so distinct field boundaries can't collide
	// (e.g. user "ab"+addr "c" must not hash the same as user "a"+addr "bc").
	writeField := func(s string) {
		fmt.Fprintf(h, "%d\x00%s", len(s), s)
	}
	writeField(c.User)
	writeField(c.Address)
	writeField(strconv.Itoa(c.Port))
	writeField(c.IdentityFile)
	writeField(c.ProxyJump)
	for _, opt := range c.SSHOptions {
		writeField(opt)
	}
	return hex.EncodeToString(h.Sum(nil)[:16])
}

// controlMasterDir returns the absolute, 0700, tool-owned directory that holds
// ControlPath sockets, creating it lazily. It errors if the home directory
// can't be determined or the dir can't be created.
func controlMasterDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".ccc", "cm")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// Run executes a remote command non-interactively and returns the
// trimmed stdout output. Connection implements the flow.Runner interface.
//
// It is a thin wrapper over RunContext with a background context so existing
// callers (and the Runner interface) are unchanged.
func (c *Connection) Run(cmd string) (string, error) {
	return c.RunContext(context.Background(), cmd)
}

// RunContext is the cancellable form of Run: when ctx is cancelled or its
// deadline expires, the ssh process group is killed (see setProcAttrs) so
// ProxyJump hops and the remote "$SHELL -lc" child are reaped rather than left
// hanging. It returns trimmed stdout on success.
//
// stderr is deliberately left nil so exec wires the child's fd 2 straight to
// /dev/null (an *os.File, not a pipe). This is load-bearing with ControlPersist:
// the backgrounded master inherits and holds the child's stderr open for the
// whole persist window, so routing stderr through an os.Pipe — as Cmd.Output and
// Cmd.CombinedOutput both do — would never reach EOF. Wait would then block until
// WaitDelay (set in proc_unix.go) and return ErrWaitDelay, discarding valid
// stdout and failing core flows (TestConnection/list/scan) even when the remote
// command succeeded. The master does not hold stdout, so capturing it through a
// buffer is safe and EOFs promptly when the command's channel closes.
func (c *Connection) RunContext(ctx context.Context, cmd string) (string, error) {
	args := c.buildNonInteractiveArgs(cmd)
	proc := execCommandContext(ctx, "ssh", args...)
	setProcAttrs(proc)

	var stdout bytes.Buffer
	proc.Stdout = &stdout

	if err := proc.Run(); err != nil {
		switch ctx.Err() {
		case context.DeadlineExceeded:
			return "", fmt.Errorf("connection timed out: %w", err)
		case context.Canceled:
			return "", fmt.Errorf("aborted: %w", err)
		default:
			return "", classifyRunError(err)
		}
	}
	return strings.TrimSpace(stdout.String()), nil
}

// classifyRunError turns a failed ssh invocation into a human-readable error.
// Exit status 255 is ssh's sentinel for a transport-level failure (handshake or
// established-link breakage), distinct from 127 (remote command not found) or
// any other code the remote command itself may return. Surfacing it as a clear
// "lost connection" message beats the cryptic bare "exit status 255".
func classifyRunError(err error) error {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 255 {
		return fmt.Errorf("lost connection to host (link down or unreachable): %w", err)
	}
	return fmt.Errorf("ssh command failed: %w", err)
}

// InteractiveCommand builds (but does not start) an *exec.Cmd that runs cmd on
// the remote host over SSH with PTY allocation and full stdio passthrough.
//
// It exists so callers that need the *exec.Cmd directly — such as the TUI, which
// hands it to tea.ExecProcess to release and reacquire the terminal — get the
// exact same SSH wiring as RunInteractive. Critically, the remote command is
// wrapped in "$SHELL -lc" (a login shell) via buildInteractiveArgs, so PATH
// additions from login profiles (e.g. ~/.local/bin, where zmx is commonly
// installed) are honored. Building the ssh args by hand instead drops that
// wrapping and makes zmx fail to resolve under non-login shells (exit 127).
func (c *Connection) InteractiveCommand(cmd string) *exec.Cmd {
	proc := exec.Command("ssh", c.buildInteractiveArgs(cmd)...)
	proc.Stdin = os.Stdin
	proc.Stdout = os.Stdout
	proc.Stderr = os.Stderr
	proc.Env = os.Environ()
	return proc
}

// RunInteractive executes a remote command interactively, handing over
// stdin, stdout, and stderr to the calling process.
func (c *Connection) RunInteractive(cmd string) error {
	return c.InteractiveCommand(cmd).Run()
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
