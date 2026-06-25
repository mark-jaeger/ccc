// Package transport builds the interactive zmx-attach command for an opt-in,
// roaming-capable transport selected per host.
//
// ccc normally attaches to a zmx session with plain `ssh -t … zmx attach`. On
// an unstable network (a train switching Wi-Fi↔LTE, a tunnel) plain ssh cannot
// roam across the IP change and the session stalls. mosh (UDP, roams, predictive
// local echo) and Eternal Terminal (et) are purpose-built to survive that. zmx
// already persists the session server-side, so only the client transport needs
// to roam.
//
// SCOPE: this package covers the INTERACTIVE ATTACH ONLY. Non-interactive
// commands (scan, `zmx ls`, `zmx check`, config reads) MUST stay on plain ssh —
// mosh cannot pipe a command's stdout back to the caller. That is a hard
// constraint, so this package deliberately only produces the attach command.
package transport

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/mark-jaeger/ccc/internal/shellutil"
)

// ErrUseSSH signals that the caller should fall back to the existing
// ssh.Connection.InteractiveCommand path rather than build a custom transport.
// It is returned for the default ("" / "ssh") transport and whenever the
// selected transport's binary is not installed locally (graceful fallback).
//
// The ssh path stays the single source of truth: this package never duplicates
// it, it only signals "use ssh".
var ErrUseSSH = errors.New("transport: use ssh")

// lookPath is a seam over exec.LookPath so capability detection can be stubbed
// in tests.
var lookPath = exec.LookPath

// Params carries the connection parameters and the selected transport. The
// fields mirror ssh.Connection / config.Host so the caller can populate them
// directly from a config.Host.
type Params struct {
	Transport      string // "", "ssh", "mosh", or "et"
	User           string
	Address        string
	Port           int
	IdentityFile   string
	ProxyJump      string
	SSHOptions     []string
	MoshServerPath string // optional override for `mosh --server=<path>`
}

// target returns user@address.
func (p Params) target() string {
	return p.User + "@" + p.Address
}

// remoteShellCmd wraps the remote command in a login shell, matching
// ssh.Connection.appendRemoteCmd: PATH additions from login profiles (where
// zmx commonly lives, e.g. ~/.local/bin) must be honored, so the command runs
// under `$SHELL -lc <quoted>`.
func remoteShellCmd(remoteCmd string) string {
	return "$SHELL -lc " + shellutil.Quote(remoteCmd)
}

// sshBootstrap builds the `ssh ...` string mosh uses only to bootstrap the
// connection (it carries port, identity, proxy jump, and any extra ssh options).
// mosh then takes over with UDP; ssh is not used for the session itself.
func (p Params) sshBootstrap() string {
	parts := []string{"ssh"}
	if p.Port != 0 {
		parts = append(parts, "-p", strconv.Itoa(p.Port))
	}
	if p.IdentityFile != "" {
		parts = append(parts, "-i", p.IdentityFile)
	}
	if p.ProxyJump != "" {
		parts = append(parts, "-J", p.ProxyJump)
	}
	parts = append(parts, p.SSHOptions...)
	return strings.Join(parts, " ")
}

// BuildArgs is the pure, unit-testable core: given the transport params and the
// already-built remote command (from zmx.BuildAttachCommand / BuildCreateCommand),
// it returns the executable name and argument list for the interactive attach.
//
// For the default ("" / "ssh") transport it returns ErrUseSSH so the caller
// uses the existing ssh path. It does NOT probe whether the transport binary is
// installed — that capability check lives in BuildInteractiveCmd, keeping BuildArgs
// free of side effects.
func BuildArgs(p Params, remoteCmd string) (name string, args []string, err error) {
	switch p.Transport {
	case "", "ssh":
		return "", nil, ErrUseSSH

	case "mosh":
		// mosh [--server=<path>] [--ssh="ssh -p <port> -i <id> ..."] user@host -- $SHELL -lc <cmd>
		//
		// IMPORTANT: no -t. mosh provides the PTY for the `--` command itself;
		// passing -t would be wrong. The `--` separator is required so mosh runs
		// our command rather than a login shell. Port/identity/proxy flow through
		// the --ssh= bootstrap option, since mosh only uses ssh to bootstrap.
		args = []string{}
		if p.MoshServerPath != "" {
			args = append(args, "--server="+p.MoshServerPath)
		}
		if boot := p.sshBootstrap(); boot != "ssh" {
			// Only pass --ssh= when there is something beyond bare "ssh" to carry.
			args = append(args, "--ssh="+boot)
		}
		args = append(args, p.target(), "--", remoteShellCmd(remoteCmd))
		return "mosh", args, nil

	case "et":
		// et [--jport <port>] user@host -c <$SHELL -lc <cmd>>
		// et runs the command via -c.
		args = []string{}
		if p.Port != 0 {
			args = append(args, "--jport", strconv.Itoa(p.Port))
		}
		args = append(args, p.target(), "-c", remoteShellCmd(remoteCmd))
		return "et", args, nil

	default:
		// Unknown transport: be safe and fall back to ssh.
		return "", nil, ErrUseSSH
	}
}

// BuildInteractiveCmd returns a ready-to-run *exec.Cmd for the interactive
// attach, with stdio and env wired exactly like ssh.Connection.InteractiveCommand
// (os.Std* passthrough, os.Environ()).
//
// It returns ErrUseSSH (and a nil cmd) when:
//   - the transport is the default "" / "ssh", or
//   - the selected transport's binary ("mosh" / "et") is not installed locally
//     (graceful capability fallback).
//
// Capability detection here is LOCAL only. Probing the REMOTE mosh-server's
// presence and auto-falling-back when the network blocks UDP is a deliberate
// follow-up and is out of scope for this change.
func BuildInteractiveCmd(p Params, remoteCmd string) (*exec.Cmd, error) {
	name, args, err := BuildArgs(p, remoteCmd)
	if err != nil {
		return nil, err
	}

	// Local capability probe: if the transport binary is missing, fall back to ssh.
	if _, lookErr := lookPath(name); lookErr != nil {
		return nil, ErrUseSSH
	}

	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	return cmd, nil
}
