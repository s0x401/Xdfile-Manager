//go:build windows

package utils

import (
	"os/exec"
	"syscall"
)

const (
	windowsCreateNoWindow        = 0x08000000
	windowsDetachedProcess       = 0x00000008
	windowsCreateNewProcessGroup = 0x00000200
)

func DetachFromTerminal(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= windowsCreateNoWindow |
		windowsDetachedProcess |
		windowsCreateNewProcessGroup
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
}
