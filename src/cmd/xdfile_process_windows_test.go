//go:build windows

package cmd

import (
	"os/exec"
	"testing"
)

func TestXdfileConfigureManagedExternalCommandIsolatesWindowsConsole(t *testing.T) {
	cmd := exec.Command("cmd.exe", "/d", "/c", "echo ok")
	xdfileConfigureManagedExternalCommand(cmd)
	if cmd.SysProcAttr == nil || cmd.SysProcAttr.CreationFlags&xdfileCreateNoWindow == 0 {
		t.Fatalf("expected managed external command to use CREATE_NO_WINDOW, got %#v", cmd.SysProcAttr)
	}
}
