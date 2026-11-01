//go:build windows

package winexec

import (
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

// Prepare runs the child without flashing a console window.
func Prepare(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= createNoWindow
}

// Command is exec.Command with Prepare applied.
func Command(name string, arg ...string) *exec.Cmd {
	cmd := exec.Command(name, arg...)
	Prepare(cmd)
	return cmd
}
