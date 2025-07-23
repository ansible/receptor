//go:build !windows && !no_workceptor
// +build !windows,!no_workceptor

package workceptor

import (
	"os/exec"
	"syscall"
)

func cmdSetDetach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}
}
