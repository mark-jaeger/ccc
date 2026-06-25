//go:build !unix

package ssh

import "os/exec"

// setProcAttrs is a no-op on platforms without POSIX process groups. The
// default exec.CommandContext behavior (SIGKILL to the lone process) still
// applies; we just can't reap an entire process group there.
func setProcAttrs(cmd *exec.Cmd) {}
