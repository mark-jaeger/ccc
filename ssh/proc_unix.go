//go:build unix

package ssh

import (
	"os/exec"
	"syscall"
	"time"
)

// setProcAttrs configures cmd so that, when its context is cancelled, the whole
// process group is killed rather than just the ssh process. SSH spawns children
// for ProxyJump hops and the remote "$SHELL -lc" wrapper; without a group kill
// those can survive and keep the transport (or stdio pipes) open, defeating the
// point of a deadline. Setpgid puts ssh in its own group, the Cancel hook sends
// SIGKILL to the negated PID (the whole group), and WaitDelay bounds how long
// Wait blocks on still-draining pipes after the kill.
func setProcAttrs(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = 2 * time.Second
}
