package transport

import (
	"os/exec"
	"strings"
	"testing"
)

// hasArg returns true if args contains the exact value v.
func hasArg(args []string, v string) bool {
	for _, a := range args {
		if a == v {
			return true
		}
	}
	return false
}

// anyArgContains returns true if any arg contains substr.
func anyArgContains(args []string, substr string) bool {
	for _, a := range args {
		if strings.Contains(a, substr) {
			return true
		}
	}
	return false
}

// indexOf returns the index of the first arg equal to v, or -1.
func indexOf(args []string, v string) int {
	for i, a := range args {
		if a == v {
			return i
		}
	}
	return -1
}

const remoteAttach = "ZMX_DIR=${ZMX_DIR:-/tmp/zmx-$(id -u)} PATH=\"$PATH:/opt/homebrew/bin:/usr/local/bin\" TERM=$TERM zmx attach 'ccc.rt1.main'"

func TestBuildArgsMosh(t *testing.T) {
	p := Params{
		Transport:    "mosh",
		User:         "deploy",
		Address:      "10.0.0.1",
		Port:         2222,
		IdentityFile: "/home/deploy/.ssh/id_ed25519",
	}

	name, args, err := BuildArgs(p, remoteAttach)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "mosh" {
		t.Errorf("name = %q, want mosh", name)
	}

	// Must contain the "--" separator before the remote command.
	if !hasArg(args, "--") {
		t.Errorf("mosh args must contain -- separator, got %v", args)
	}
	// Must contain user@host.
	if !hasArg(args, "deploy@10.0.0.1") {
		t.Errorf("mosh args must contain user@host, got %v", args)
	}
	// Must carry the quoted remote command (zmx attach) somewhere.
	if !anyArgContains(args, "zmx attach") {
		t.Errorf("mosh args must contain remote command with zmx attach, got %v", args)
	}
	// Must NOT pass -t (mosh provides the PTY for the -- command itself).
	if hasArg(args, "-t") {
		t.Errorf("mosh args must NOT contain -t, got %v", args)
	}
	// Port and identity must flow through the --ssh= bootstrap option.
	if !anyArgContains(args, "--ssh=") {
		t.Errorf("mosh args must pass bootstrap via --ssh=, got %v", args)
	}
	if !anyArgContains(args, "-p 2222") {
		t.Errorf("mosh --ssh= must carry -p 2222, got %v", args)
	}
	if !anyArgContains(args, "-i /home/deploy/.ssh/id_ed25519") {
		t.Errorf("mosh --ssh= must carry -i identity, got %v", args)
	}
}

func TestBuildArgsMoshServerPath(t *testing.T) {
	p := Params{
		Transport:      "mosh",
		User:           "deploy",
		Address:        "10.0.0.1",
		MoshServerPath: "/opt/bin/mosh-server",
	}

	_, args, err := BuildArgs(p, remoteAttach)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !anyArgContains(args, "--server=/opt/bin/mosh-server") {
		t.Errorf("mosh args must contain --server=<path>, got %v", args)
	}
}

// TestBuildArgsMoshRemoteCommandIsArgv guards the mosh execvp contract: mosh-server
// execs the tokens after `--` directly with NO shell, so the remote command must be
// a real argv ([/bin/sh -c <script>]) rather than a single "$SHELL -lc <cmd>" string
// — that single token would be execvp'd as one literal program name (with $SHELL
// never expanded) and fail with ENOENT. This is the bug this test pins down.
func TestBuildArgsMoshRemoteCommandIsArgv(t *testing.T) {
	p := Params{Transport: "mosh", User: "deploy", Address: "10.0.0.1"}
	_, args, err := BuildArgs(p, remoteAttach)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sep := indexOf(args, "--")
	if sep < 0 || sep+1 >= len(args) {
		t.Fatalf("mosh args must have tokens after --, got %v", args)
	}
	cmd := args[sep+1:]
	// The token immediately after -- must be a real executable, never a string that
	// starts with the unexpanded $SHELL variable (the original bug).
	if strings.HasPrefix(cmd[0], "$SHELL") {
		t.Errorf("mosh command argv[0] must be a real shell binary, not a literal $SHELL string, got %q", cmd[0])
	}
	if cmd[0] != "/bin/sh" {
		t.Errorf("mosh command argv[0] = %q, want /bin/sh", cmd[0])
	}
	// The shell and its -c flag must be SEPARATE argv tokens.
	if !hasArg(cmd, "-c") {
		t.Errorf("mosh command argv must contain a separate -c flag, got %v", cmd)
	}
	// The script must invoke the login shell ($SHELL -lc) and carry the command.
	if !anyArgContains(cmd, "$SHELL") || !anyArgContains(cmd, "-lc") {
		t.Errorf("mosh command argv must invoke $SHELL -lc, got %v", cmd)
	}
	if !anyArgContains(cmd, "zmx attach") {
		t.Errorf("mosh command argv must carry the remote command, got %v", cmd)
	}
}

func TestBuildArgsET(t *testing.T) {
	p := Params{
		Transport:    "et",
		User:         "deploy",
		Address:      "10.0.0.1",
		Port:         2222,
		IdentityFile: "/home/deploy/.ssh/id_ed25519",
		ProxyJump:    "bastion.example.com",
	}

	name, args, err := BuildArgs(p, remoteAttach)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "et" {
		t.Errorf("name = %q, want et", name)
	}
	// et runs the command via -c.
	if !hasArg(args, "-c") {
		t.Errorf("et args must contain -c, got %v", args)
	}
	if !hasArg(args, "deploy@10.0.0.1") {
		t.Errorf("et args must contain user@host, got %v", args)
	}
	if !anyArgContains(args, "zmx attach") {
		t.Errorf("et args must contain remote command with zmx attach, got %v", args)
	}
	// The sshd port must NOT be routed to --jport (that flag is a jumphost ET port,
	// unrelated to the ssh bootstrap port).
	if hasArg(args, "--jport") {
		t.Errorf("et args must NOT contain --jport (it is not the sshd port), got %v", args)
	}
	// Port, identity, and proxy must flow through --ssh-option (ssh -o keywords).
	if !hasArg(args, "--ssh-option") {
		t.Errorf("et args must pass connection params via --ssh-option, got %v", args)
	}
	if !hasArg(args, "Port=2222") {
		t.Errorf("et args must carry Port=2222 via --ssh-option, got %v", args)
	}
	if !hasArg(args, "IdentityFile=/home/deploy/.ssh/id_ed25519") {
		t.Errorf("et args must carry IdentityFile via --ssh-option, got %v", args)
	}
	if !hasArg(args, "ProxyJump=bastion.example.com") {
		t.Errorf("et args must carry ProxyJump via --ssh-option, got %v", args)
	}
}

func TestBuildArgsETNoPort(t *testing.T) {
	p := Params{
		Transport: "et",
		User:      "deploy",
		Address:   "10.0.0.1",
	}

	_, args, err := BuildArgs(p, remoteAttach)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasArg(args, "--jport") {
		t.Errorf("et args must NOT contain --jport, got %v", args)
	}
	// With no port/identity/proxy there is nothing to forward.
	if hasArg(args, "--ssh-option") {
		t.Errorf("et args must NOT contain --ssh-option when no connection params set, got %v", args)
	}
}

func TestBuildArgsSSHUsesFallback(t *testing.T) {
	for _, tr := range []string{"", "ssh"} {
		p := Params{Transport: tr, User: "deploy", Address: "10.0.0.1"}
		name, args, err := BuildArgs(p, remoteAttach)
		if err != ErrUseSSH {
			t.Errorf("transport %q: err = %v, want ErrUseSSH", tr, err)
		}
		if name != "" || args != nil {
			t.Errorf("transport %q: expected empty name/args on ErrUseSSH, got %q %v", tr, name, args)
		}
	}
}

func TestBuildInteractiveCmdSSH(t *testing.T) {
	p := Params{Transport: "ssh", User: "deploy", Address: "10.0.0.1"}
	cmd, err := BuildInteractiveCmd(p, remoteAttach)
	if err != ErrUseSSH {
		t.Fatalf("err = %v, want ErrUseSSH", err)
	}
	if cmd != nil {
		t.Errorf("expected nil cmd on ErrUseSSH, got %v", cmd)
	}
}

func TestBuildInteractiveCmdMosh(t *testing.T) {
	// Force LookPath to succeed regardless of host environment.
	orig := lookPath
	lookPath = func(file string) (string, error) { return "/usr/bin/" + file, nil }
	defer func() { lookPath = orig }()

	p := Params{Transport: "mosh", User: "deploy", Address: "10.0.0.1"}
	cmd, err := BuildInteractiveCmd(p, remoteAttach)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	if cmd.Stdin == nil || cmd.Stdout == nil || cmd.Stderr == nil {
		t.Error("expected stdio to be wired")
	}
	if len(cmd.Env) == 0 {
		t.Error("expected env to be populated")
	}
}

func TestBuildInteractiveCmdMoshMissingFallsBack(t *testing.T) {
	// Simulate mosh not installed locally.
	orig := lookPath
	lookPath = func(file string) (string, error) { return "", exec.ErrNotFound }
	defer func() { lookPath = orig }()

	p := Params{Transport: "mosh", User: "deploy", Address: "10.0.0.1"}
	cmd, err := BuildInteractiveCmd(p, remoteAttach)
	if err != ErrUseSSH {
		t.Fatalf("err = %v, want ErrUseSSH when mosh binary absent", err)
	}
	if cmd != nil {
		t.Errorf("expected nil cmd when mosh absent, got %v", cmd)
	}
}

func TestBuildInteractiveCmdETMissingFallsBack(t *testing.T) {
	orig := lookPath
	lookPath = func(file string) (string, error) { return "", exec.ErrNotFound }
	defer func() { lookPath = orig }()

	p := Params{Transport: "et", User: "deploy", Address: "10.0.0.1"}
	cmd, err := BuildInteractiveCmd(p, remoteAttach)
	if err != ErrUseSSH {
		t.Fatalf("err = %v, want ErrUseSSH when et binary absent", err)
	}
	if cmd != nil {
		t.Errorf("expected nil cmd when et absent, got %v", cmd)
	}
}
