//go:build windows && !no_workceptor
// +build windows,!no_workceptor

package workceptor

import (
	"os/exec"
)

func cmdSetDetach(cmd *exec.Cmd) {
	// Do nothing
}
