//go:build unix

package harness

import (
	"os"
	"os/exec"
	"syscall"
)

// configureKill puts the Harness in its own process group and makes cancellation kill the whole
// group, so the sandbox-exec processes the Harness started do not outlive it.
func configureKill(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if err == syscall.ESRCH {
			return os.ErrProcessDone
		}
		return err
	}
}
