package mpvsetup

import (
	"os/exec"
	"syscall"
	"time"
)

func prepareBuildCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	// Meson spawns Ninja and compilers; cancel the entire build group so they
	// cannot keep writing into a prefix that is being cleaned up.
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = time.Second
}
