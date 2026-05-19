package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	platformsystem "github.com/s0x401/xdfile-manager/src/internal/platform/system"
)

const (
	xdfileShellOpenHelperArg = "__xdfile-shell-open-helper"
	xdfileShellOpenHelperEnv = "XDFILE_SHELL_OPEN_HELPER"
)

func xdfileMaybeRunShellOpenHelper() bool {
	if len(os.Args) != 3 || os.Args[1] != xdfileShellOpenHelperArg || os.Getenv(xdfileShellOpenHelperEnv) != "1" {
		return false
	}
	if err := platformsystem.OpenPathDirect(filepath.Clean(os.Args[2])); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
	return true
}

func xdfileOpenPathViaHelper(path string) error {
	path = strings.TrimSpace(filepath.Clean(path))
	if path == "" || path == "." {
		return fmt.Errorf("empty path")
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve shell-open helper executable: %w", err)
	}

	cmd := exec.Command(exe, xdfileShellOpenHelperArg, path)
	cmd.Env = append(os.Environ(), xdfileShellOpenHelperEnv+"=1")
	platformsystem.ConfigureManagedExternalCommand(cmd)

	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	if message := strings.TrimSpace(string(output)); message != "" {
		return fmt.Errorf("%s", message)
	}
	return fmt.Errorf("shell-open helper failed: %w", err)
}
