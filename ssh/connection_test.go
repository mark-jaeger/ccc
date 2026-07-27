package ssh

import (
	"context"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mark-jaeger/ccc/v2/config"
)

func TestBuildNonInteractiveArgs(t *testing.T) {
	c := Connection{
		User:    "deploy",
		Address: "10.0.0.1",
	}

	args := c.buildNonInteractiveArgs("uptime")

	// Must contain BatchMode=yes
	if !containsOption(args, "-o", "BatchMode=yes") {
		t.Errorf("expected -o BatchMode=yes in args, got %v", args)
	}

	// Must contain StrictHostKeyChecking=accept-new
	if !containsOption(args, "-o", "StrictHostKeyChecking=accept-new") {
		t.Errorf("expected -o StrictHostKeyChecking=accept-new in args, got %v", args)
	}

	// Must contain ConnectTimeout=10
	if !containsOption(args, "-o", "ConnectTimeout=10") {
		t.Errorf("expected -o ConnectTimeout=10 in args, got %v", args)
	}

	// Must contain user@address
	if !slices.Contains(args, "deploy@10.0.0.1") {
		t.Errorf("expected deploy@10.0.0.1 in args, got %v", args)
	}

	// Must wrap command in $SHELL -lc as a single quoted argument
	lastArg := args[len(args)-1]
	if lastArg != "$SHELL -lc 'uptime'" {
		t.Errorf("expected last arg to be %q, got %q", "$SHELL -lc 'uptime'", lastArg)
	}

	// Must NOT contain -t (that's interactive only)
	if slices.Contains(args, "-t") {
		t.Errorf("did not expect -t in non-interactive args, got %v", args)
	}
}

func TestBuildInteractiveArgs(t *testing.T) {
	c := Connection{
		User:    "deploy",
		Address: "10.0.0.1",
	}

	args := c.buildInteractiveArgs("htop")

	// Must contain -t for PTY allocation
	if !slices.Contains(args, "-t") {
		t.Errorf("expected -t in interactive args, got %v", args)
	}

	// Must NOT contain BatchMode=yes
	if containsOption(args, "-o", "BatchMode=yes") {
		t.Errorf("did not expect -o BatchMode=yes in interactive args, got %v", args)
	}

	// Must contain user@address
	if !slices.Contains(args, "deploy@10.0.0.1") {
		t.Errorf("expected deploy@10.0.0.1 in args, got %v", args)
	}

	// Must contain the command wrapped in $SHELL -lc as single arg
	lastArg := args[len(args)-1]
	if lastArg != "$SHELL -lc 'htop'" {
		t.Errorf("expected last arg to be %q, got %q", "$SHELL -lc 'htop'", lastArg)
	}
}

func TestInteractiveCommand(t *testing.T) {
	c := Connection{
		User:         "deploy",
		Address:      "10.0.0.1",
		Port:         2222,
		IdentityFile: "/home/deploy/.ssh/id_ed25519",
	}

	proc := c.InteractiveCommand("zmx attach foo")

	if !strings.HasSuffix(proc.Path, "ssh") {
		t.Errorf("expected an ssh command, got %q", proc.Path)
	}

	// Remote command must run through a login shell ($SHELL -lc) so that PATH
	// additions from login profiles (e.g. ~/.local/bin where zmx is often
	// installed) are honored. Without this, attach/create fail with exit 127.
	lastArg := proc.Args[len(proc.Args)-1]
	if lastArg != "$SHELL -lc 'zmx attach foo'" {
		t.Errorf("expected last arg to be %q, got %q", "$SHELL -lc 'zmx attach foo'", lastArg)
	}

	// Must allocate a PTY and carry connection options (which the old hand-rolled
	// args in tui dropped).
	if !slices.Contains(proc.Args, "-t") {
		t.Errorf("expected -t in args, got %v", proc.Args)
	}
	if !containsOption(proc.Args, "-i", "/home/deploy/.ssh/id_ed25519") {
		t.Errorf("expected identity file in args, got %v", proc.Args)
	}
	if !containsOption(proc.Args, "-p", "2222") {
		t.Errorf("expected port in args, got %v", proc.Args)
	}
}

func TestBuildArgsWithSSHOptions(t *testing.T) {
	c := Connection{
		User:       "deploy",
		Address:    "10.0.0.1",
		SSHOptions: []string{"-o", "ServerAliveInterval=60"},
	}

	args := c.buildNonInteractiveArgs("ls")

	// SSHOptions should be passed as raw args, not wrapped with extra -o
	if !containsOption(args, "-o", "ServerAliveInterval=60") {
		t.Errorf("expected -o ServerAliveInterval=60 in args, got %v", args)
	}
}

// TestSSHResilienceOptionsPrecedeUserOptions verifies ccc emits its
// resilience/correctness options before the user's ssh_options, so ccc wins
// OpenSSH's first-value-wins rule: a user ServerAliveInterval/BatchMode entry is
// still present but no longer overrides ccc's fail-fast defaults.
func TestSSHResilienceOptionsPrecedeUserOptions(t *testing.T) {
	c := Connection{
		User:       "deploy",
		Address:    "10.0.0.1",
		SSHOptions: []string{"-o", "ServerAliveInterval=60", "-o", "BatchMode=no"},
	}

	args := c.buildNonInteractiveArgs("ls")
	mustPrecede := func(cccVal, userVal string) {
		t.Helper()
		ci, ui := indexOfValue(args, cccVal), indexOfValue(args, userVal)
		if ci == -1 {
			t.Fatalf("ccc option %q missing from args: %v", cccVal, args)
		}
		if ui == -1 {
			t.Fatalf("user option %q missing from args: %v", userVal, args)
		}
		if ci >= ui {
			t.Errorf("ccc %q (idx %d) must precede user %q (idx %d) to win; args: %v", cccVal, ci, userVal, ui, args)
		}
	}
	mustPrecede("ServerAliveInterval=5", "ServerAliveInterval=60")
	mustPrecede("BatchMode=yes", "BatchMode=no")

	// Interactive path: ccc's keepalive also precedes the user's, and -t follows
	// all -o options (ccc's and the user's) or ssh ignores them.
	iargs := c.buildInteractiveArgs("attach")
	if i, u := indexOfValue(iargs, "ServerAliveInterval=10"), indexOfValue(iargs, "ServerAliveInterval=60"); i == -1 || u == -1 || i >= u {
		t.Errorf("interactive: ccc ServerAliveInterval=10 must precede user =60; args: %v", iargs)
	}
	if tIdx, uIdx := indexOfValue(iargs, "-t"), indexOfValue(iargs, "ServerAliveInterval=60"); tIdx == -1 || tIdx < uIdx {
		t.Errorf("interactive: -t (idx %d) must follow the user options (idx %d); args: %v", tIdx, uIdx, iargs)
	}
}

func TestBuildArgsWithPort(t *testing.T) {
	c := Connection{
		User:    "deploy",
		Address: "10.0.0.1",
		Port:    2222,
	}

	args := c.buildNonInteractiveArgs("ls")

	if !containsOption(args, "-p", "2222") {
		t.Errorf("expected -p 2222 in args, got %v", args)
	}
}

func TestBuildArgsWithIdentityFile(t *testing.T) {
	c := Connection{
		User:         "deploy",
		Address:      "10.0.0.1",
		IdentityFile: "/home/deploy/.ssh/id_ed25519",
	}

	args := c.buildNonInteractiveArgs("ls")

	if !containsOption(args, "-i", "/home/deploy/.ssh/id_ed25519") {
		t.Errorf("expected -i /home/deploy/.ssh/id_ed25519 in args, got %v", args)
	}
}

func TestBuildArgsWithProxyJump(t *testing.T) {
	c := Connection{
		User:      "deploy",
		Address:   "10.0.0.1",
		ProxyJump: "bastion.example.com",
	}

	args := c.buildNonInteractiveArgs("ls")

	if !containsOption(args, "-J", "bastion.example.com") {
		t.Errorf("expected -J bastion.example.com in args, got %v", args)
	}
}

func TestBuildArgsAllOptions(t *testing.T) {
	c := Connection{
		User:         "deploy",
		Address:      "10.0.0.1",
		Port:         2222,
		IdentityFile: "/home/deploy/.ssh/id_ed25519",
		ProxyJump:    "bastion",
		SSHOptions:   []string{"-o", "ServerAliveInterval=60"},
	}

	args := c.buildNonInteractiveArgs("uptime")

	// Verify all fields are present
	if !containsOption(args, "-p", "2222") {
		t.Errorf("missing -p 2222, got %v", args)
	}
	if !containsOption(args, "-i", "/home/deploy/.ssh/id_ed25519") {
		t.Errorf("missing -i identity_file, got %v", args)
	}
	if !containsOption(args, "-J", "bastion") {
		t.Errorf("missing -J bastion, got %v", args)
	}
	if !containsOption(args, "-o", "ServerAliveInterval=60") {
		t.Errorf("missing -o ServerAliveInterval=60, got %v", args)
	}
	if !containsOption(args, "-o", "BatchMode=yes") {
		t.Errorf("missing -o BatchMode=yes, got %v", args)
	}
	if !slices.Contains(args, "deploy@10.0.0.1") {
		t.Errorf("missing deploy@10.0.0.1, got %v", args)
	}

	// Verify ordering: port, identity, proxy come before BatchMode options
	portIdx := -1
	batchIdx := -1
	for i, a := range args {
		if a == "-p" && portIdx == -1 {
			portIdx = i
		}
		if a == "BatchMode=yes" {
			batchIdx = i
		}
	}
	if portIdx >= batchIdx {
		t.Errorf("port args should come before BatchMode, port=%d batch=%d", portIdx, batchIdx)
	}
}

func TestConnectionFromHost(t *testing.T) {
	h := config.Host{
		User:         "deploy",
		Address:      "10.0.0.1",
		Port:         2222,
		IdentityFile: "/home/deploy/.ssh/id_ed25519",
		ProxyJump:    "bastion",
		SSHOptions:   []string{"-o", "Foo=bar"},
	}

	conn := ConnectionFromHost(h)

	if conn.User != "deploy" {
		t.Errorf("User = %q, want %q", conn.User, "deploy")
	}
	if conn.Address != "10.0.0.1" {
		t.Errorf("Address = %q, want %q", conn.Address, "10.0.0.1")
	}
	if conn.Port != 2222 {
		t.Errorf("Port = %d, want %d", conn.Port, 2222)
	}
	if conn.IdentityFile != "/home/deploy/.ssh/id_ed25519" {
		t.Errorf("IdentityFile = %q, want %q", conn.IdentityFile, "/home/deploy/.ssh/id_ed25519")
	}
	if conn.ProxyJump != "bastion" {
		t.Errorf("ProxyJump = %q, want %q", conn.ProxyJump, "bastion")
	}
	if len(conn.SSHOptions) != 2 || conn.SSHOptions[0] != "-o" {
		t.Errorf("SSHOptions = %v, want [-o Foo=bar]", conn.SSHOptions)
	}
}

// TestNonInteractiveKeepalives verifies that non-interactive args carry the
// aggressive server-alive probes (5s interval) that let a dead link self-
// terminate quickly instead of hanging for minutes (the "train" failure mode).
func TestNonInteractiveKeepalives(t *testing.T) {
	c := Connection{User: "deploy", Address: "10.0.0.1"}
	args := c.buildNonInteractiveArgs("uptime")

	if !containsOption(args, "-o", "ServerAliveInterval=5") {
		t.Errorf("expected -o ServerAliveInterval=5 in args, got %v", args)
	}
	if !containsOption(args, "-o", "ServerAliveCountMax=3") {
		t.Errorf("expected -o ServerAliveCountMax=3 in args, got %v", args)
	}
	if !containsOption(args, "-o", "TCPKeepAlive=no") {
		t.Errorf("expected -o TCPKeepAlive=no in args, got %v", args)
	}
}

// TestInteractiveKeepalives verifies that interactive args carry the gentler
// server-alive probes (10s interval) — long enough to ride out brief tunnel
// hiccups, short enough to tear down a truly dead attach — and that they come
// before the -t flag (i.e. before the target host, not after the remote cmd).
func TestInteractiveKeepalives(t *testing.T) {
	c := Connection{User: "deploy", Address: "10.0.0.1"}
	args := c.buildInteractiveArgs("htop")

	if !containsOption(args, "-o", "ServerAliveInterval=10") {
		t.Errorf("expected -o ServerAliveInterval=10 in args, got %v", args)
	}
	if !containsOption(args, "-o", "ServerAliveCountMax=3") {
		t.Errorf("expected -o ServerAliveCountMax=3 in args, got %v", args)
	}
	if !containsOption(args, "-o", "TCPKeepAlive=no") {
		t.Errorf("expected -o TCPKeepAlive=no in args, got %v", args)
	}

	// Keepalive options must precede -t (and thus the target host); options
	// after the remote command would be ignored by ssh.
	aliveIdx := slices.Index(args, "ServerAliveInterval=10")
	tIdx := slices.Index(args, "-t")
	if aliveIdx == -1 || tIdx == -1 || aliveIdx >= tIdx {
		t.Errorf("keepalive options must come before -t, alive=%d t=%d args=%v", aliveIdx, tIdx, args)
	}
}

// TestControlMasterNonInteractiveOnly verifies that connection multiplexing is
// enabled for non-interactive commands (so a burst of short commands shares one
// transport) but ABSENT from interactive attaches (a long-lived PTY gains
// nothing from a persistent master and would just complicate teardown).
func TestControlMasterNonInteractiveOnly(t *testing.T) {
	c := Connection{User: "deploy", Address: "10.0.0.1"}

	nonInteractive := c.buildNonInteractiveArgs("uptime")
	if !containsOption(nonInteractive, "-o", "ControlMaster=auto") {
		t.Errorf("expected -o ControlMaster=auto in non-interactive args, got %v", nonInteractive)
	}

	interactive := c.buildInteractiveArgs("htop")
	if containsOption(interactive, "-o", "ControlMaster=auto") {
		t.Errorf("did not expect -o ControlMaster=auto in interactive args, got %v", interactive)
	}
}

// TestControlPathDistinguishesAuthAndRouting verifies that the multiplex socket
// is keyed by the full effective connection, not just OpenSSH's %C (which
// ignores IdentityFile, ProxyJump, and arbitrary SSHOptions). Two hosts sharing
// user@address:port but differing in key or routing must NOT share a master, or
// a command could run over the wrong trust boundary; an identical config must
// stay stable so multiplexing actually reuses its master.
func TestControlPathDistinguishesAuthAndRouting(t *testing.T) {
	base := Connection{User: "deploy", Address: "10.0.0.1", Port: 22}

	withKeyA := base
	withKeyA.IdentityFile = "/keys/a"
	withKeyB := base
	withKeyB.IdentityFile = "/keys/b"

	pathA := controlPathArg(t, withKeyA.buildNonInteractiveArgs("uptime"))
	pathB := controlPathArg(t, withKeyB.buildNonInteractiveArgs("uptime"))
	if pathA == pathB {
		t.Errorf("different IdentityFile must yield different ControlPath, both = %q", pathA)
	}

	// An identical connection must produce a stable path across calls.
	pathA2 := controlPathArg(t, withKeyA.buildNonInteractiveArgs("whoami"))
	if pathA != pathA2 {
		t.Errorf("identical connection must yield identical ControlPath, %q != %q", pathA, pathA2)
	}

	// Arbitrary ssh_options (e.g. a different ProxyCommand) must also fork a
	// distinct master, since %C ignores them entirely.
	withProxyCmd := base
	withProxyCmd.SSHOptions = []string{"-o", "ProxyCommand=nc %h %p"}
	pathProxy := controlPathArg(t, withProxyCmd.buildNonInteractiveArgs("uptime"))
	pathBase := controlPathArg(t, base.buildNonInteractiveArgs("uptime"))
	if pathProxy == pathBase {
		t.Errorf("differing SSHOptions must yield different ControlPath, both = %q", pathProxy)
	}
}

// TestClassifyRunError checks that exit status 255 (ssh's transport-failure
// sentinel) is rewritten into a human-readable "lost connection" error, while
// any other non-zero exit (e.g. 127 = command not found) keeps the generic
// "ssh command failed" wording.
func TestClassifyRunError(t *testing.T) {
	err255 := exec.Command("sh", "-c", "exit 255").Run()
	if err255 == nil {
		t.Fatal("expected exit 255 to produce an error")
	}
	got := classifyRunError(err255)
	if !strings.Contains(got.Error(), "lost connection to host") {
		t.Errorf("exit 255 should map to a lost-connection error, got %q", got.Error())
	}

	err127 := exec.Command("sh", "-c", "exit 127").Run()
	if err127 == nil {
		t.Fatal("expected exit 127 to produce an error")
	}
	got = classifyRunError(err127)
	if strings.Contains(got.Error(), "lost connection to host") {
		t.Errorf("exit 127 should NOT map to a lost-connection error, got %q", got.Error())
	}
	if !strings.Contains(got.Error(), "ssh command failed") {
		t.Errorf("exit 127 should map to the generic ssh-failure error, got %q", got.Error())
	}
}

// TestRunContextDeadline verifies that a short deadline aborts a hung command
// promptly instead of waiting out the full child runtime. The execCommandContext
// seam is pointed at a long "sleep" so the test exercises cancellation, not ssh.
func TestRunContextDeadline(t *testing.T) {
	orig := execCommandContext
	defer func() { execCommandContext = orig }()
	execCommandContext = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sleep", "30")
	}

	c := Connection{User: "deploy", Address: "10.0.0.1"}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := c.RunContext(ctx, "uptime")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected an error from a deadline-exceeded RunContext")
	}
	if elapsed > 5*time.Second {
		t.Errorf("RunContext took %v; expected it to abort promptly on deadline", elapsed)
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected a timeout error, got %q", err.Error())
	}
}

// indexOfValue returns the index of the first occurrence of value in args, or -1.
func indexOfValue(args []string, value string) int {
	for i, a := range args {
		if a == value {
			return i
		}
	}
	return -1
}

// containsOption checks that args contains a consecutive pair of flag and value.
func containsOption(args []string, flag, value string) bool {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == flag && args[i+1] == value {
			return true
		}
	}
	return false
}

// controlPathArg returns the ControlPath value from an ssh argument list,
// failing the test if no ControlPath option is present.
func controlPathArg(t *testing.T, args []string) string {
	t.Helper()
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "-o" && strings.HasPrefix(args[i+1], "ControlPath=") {
			return strings.TrimPrefix(args[i+1], "ControlPath=")
		}
	}
	t.Fatalf("no ControlPath option in args: %v", args)
	return ""
}
