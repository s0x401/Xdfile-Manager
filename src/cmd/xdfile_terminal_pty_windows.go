//go:build windows

package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/charmbracelet/x/conpty"
	"golang.org/x/sys/windows"
)

func xdfileStartCommandPTYBackend(
	dir string,
	path string,
	args []string,
	width int,
	height int,
) (xdfileTerminalPTYBackend, *os.Process, error) {
	backend, err := conpty.New(max(1, width), max(1, height), 0)
	if err != nil {
		return nil, nil, fmt.Errorf("create ConPTY session: %w", err)
	}

	pid, handle, err := backend.Spawn(path, append([]string{path}, args...), &syscall.ProcAttr{
		Dir: dir,
		Env: xdfileCommandExecutionEnvironment(os.Environ()),
	})
	if err != nil {
		_ = backend.Close()
		return nil, nil, fmt.Errorf("spawn PTY command: %w", err)
	}

	process, processErr := os.FindProcess(pid)
	if handle != 0 {
		_ = windows.CloseHandle(windows.Handle(handle))
	}
	if processErr != nil {
		_ = backend.Close()
		return nil, nil, fmt.Errorf("track PTY command: %w", processErr)
	}

	return backend, process, nil
}

func xdfileWindowsCmdPath() string {
	candidates := []string{}

	for _, key := range []string{"ComSpec", "COMSPEC"} {
		if path := os.Getenv(key); path != "" {
			candidates = append(candidates, path)
		}
	}

	for _, key := range []string{"SystemRoot", "WINDIR"} {
		if root := os.Getenv(key); root != "" {
			candidates = append(candidates, filepath.Join(root, "System32", "cmd.exe"))
		}
	}

	if path, err := exec.LookPath("cmd.exe"); err == nil {
		candidates = append(candidates, path)
	}

	if drive := os.Getenv("SystemDrive"); drive != "" {
		candidates = append(candidates, filepath.Join(drive+`\`, "Windows", "System32", "cmd.exe"))
	}
	candidates = append(candidates, `C:\Windows\System32\cmd.exe`)

	if path, ok := xdfileFirstExistingAbsoluteWindowsFile(candidates...); ok {
		return path
	}
	return candidates[len(candidates)-1]
}

func xdfileFirstExistingAbsoluteWindowsFile(candidates ...string) (string, bool) {
	for _, candidate := range candidates {
		path := strings.Trim(strings.TrimSpace(candidate), `"'`)
		if path == "" {
			continue
		}
		path = filepath.Clean(path)
		if !filepath.IsAbs(path) {
			continue
		}
		info, err := os.Stat(path)
		if err == nil && !info.IsDir() {
			return path, true
		}
	}
	return "", false
}
