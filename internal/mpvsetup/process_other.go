//go:build !linux

package mpvsetup

import "os/exec"

func prepareBuildCommand(_ *exec.Cmd) {}
