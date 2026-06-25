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
// under `$SHELL -lc <quoted>`. The result is a SINGLE shell-command string,
// suitable for a transport that hands its argument to a shell (et types -c into
// the login shell it opens). It is NOT suitable where the command is execvp'd
// directly — see remoteShellArgv.
func remoteShellCmd(remoteCmd string) string {
	return "$SHELL -lc " + shellutil.Quote(remoteCmd)
}

// remoteShellArgv is the argv-token form of remoteShellCmd, for transports that
// execvp the remote command directly with NO intervening shell. mosh-server does
// exactly this: it execs command_argv[0] (everything after `--`) verbatim. So the
// shell, its flags, and the command MUST be separate argv tokens — a single
// "$SHELL -lc <cmd>" string would be execvp'd as one literal program name and
// fail with ENOENT. Likewise "$SHELL" cannot be argv[0]: nothing would expand it.
// We therefore bootstrap a concrete /bin/sh that expands $SHELL and exec's it as
// a login shell, preserving ssh.Connection's "$SHELL -lc" semantics.
func remoteShellArgv(remoteCmd string) []string {
	return []string{"/bin/sh", "-c", `exec "$SHELL" -lc ` + shellutil.Quote(remoteCmd)}
}

// sshBootstrap builds the `ssh ...` string mosh uses only to bootstrap the
// connection (it carries port, identity, proxy jump, and any extra ssh options).
// mosh then takes over with UDP; ssh is not used for the session itself.
//
// The value is a single string handed to `mosh --ssh=`. mosh re-splits it with
// Perl's shellwords() (Text::ParseWords), which honors shell quoting, so each
// value that could contain spaces or shell metacharacters (identity file, proxy
// jump, ssh options) is shell-quoted to preserve its argv boundary — matching the
// boundaries the plain `exec.Command("ssh", args...)` path keeps. The numeric port
// needs no quoting.
func (p Params) sshBootstrap() string {
	parts := []string{"ssh"}
	if p.Port != 0 {
		parts = append(parts, "-p", strconv.Itoa(p.Port))
	}
	if p.IdentityFile != "" {
		parts = append(parts, "-i", shellutil.Quote(p.IdentityFile))
	}
	if p.ProxyJump != "" {
		parts = append(parts, "-J", shellutil.Quote(p.ProxyJump))
	}
	for _, opt := range p.SSHOptions {
		parts = append(parts, shellutil.Quote(opt))
	}
	return strings.Join(parts, " ")
}

// etSSHOptions maps ccc's structured connection fields onto et's --ssh-option
// values. et bootstraps with ssh and forwards each --ssh-option to `ssh -o`, so
// Port/IdentityFile/ProxyJump map cleanly to the standard ssh config keywords.
//
// Freeform Host.SSHOptions are intentionally NOT forwarded here: they are stored
// as raw ssh argv tokens (e.g. ["-o", "K=V"]), which do not map onto et's
// -o-prefixed --ssh-option form without producing malformed args. Forwarding them
// is left as a follow-up rather than emitted as broken commands.
func (p Params) etSSHOptions() []string {
	var opts []string
	if p.Port != 0 {
		opts = append(opts, "Port="+strconv.Itoa(p.Port))
	}
	if p.IdentityFile != "" {
		opts = append(opts, "IdentityFile="+p.IdentityFile)
	}
	if p.ProxyJump != "" {
		opts = append(opts, "ProxyJump="+p.ProxyJump)
	}
	return opts
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
		// mosh [--server=<path>] [--ssh="ssh -p <port> -i <id> ..."] user@host -- /bin/sh -c '...'
		//
		// IMPORTANT: no -t. mosh provides the PTY for the `--` command itself;
		// passing -t would be wrong. The `--` separator is required so mosh runs
		// our command rather than a login shell. The remote command is passed as a
		// real argv (remoteShellArgv) because mosh-server execvp's it directly with
		// no shell. Port/identity/proxy flow through the --ssh= bootstrap option,
		// since mosh only uses ssh to bootstrap.
		args = []string{}
		if p.MoshServerPath != "" {
			args = append(args, "--server="+p.MoshServerPath)
		}
		if boot := p.sshBootstrap(); boot != "ssh" {
			// Only pass --ssh= when there is something beyond bare "ssh" to carry.
			args = append(args, "--ssh="+boot)
		}
		args = append(args, p.target(), "--")
		args = append(args, remoteShellArgv(remoteCmd)...)
		return "mosh", args, nil

	case "et":
		// et [--ssh-option K=V ...] user@host -c <$SHELL -lc <cmd>>
		//
		// et types the -c command into the login shell it opens on the server, so
		// remoteShellCmd is interpreted there. et bootstraps the connection with
		// ssh and forwards each --ssh-option to `ssh -o`, so per-host port,
		// identity, and proxy settings flow through there. (et's --jport is the
		// *jumphost's* et port, NOT the sshd port, so Host.Port must NOT be routed
		// to it — doing so silently drops the real ssh port.)
		args = []string{}
		for _, opt := range p.etSSHOptions() {
			args = append(args, "--ssh-option", opt)
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
