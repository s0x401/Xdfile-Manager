//go:build !windows && !darwin && !linux

package system

import (
	"fmt"
	"os/exec"

	"github.com/s0x401/xdfile-manager/src/internal/utils"
)

func readClipboardPaths() ([]string, error) {
	return nil, nil
}

func readClipboardCut() (bool, error) {
	return false, nil
}

func writeClipboardPaths(_ []string, _ bool) error {
	return nil
}

func openPath(path string) error {
	if path == "" {
		return fmt.Errorf("empty path")
	}
	cmd := exec.Command("xdg-open", path)
	utils.DetachFromTerminal(cmd)
	return cmd.Start()
}

func showProperties(path string) error {
	if path == "" {
		return fmt.Errorf("empty path")
	}
	return fmt.Errorf("system properties dialog is only available on Windows")
}

func configureManagedExternalCommand(cmd *exec.Cmd) {
}

func commandMenuShortPath(path string) (string, error) {
	return path, nil
}

func commandMenuEncodeWithCodePage(text string, encoding CommandMenuListEncoding) ([]byte, error) {
	return []byte(text), nil
}
