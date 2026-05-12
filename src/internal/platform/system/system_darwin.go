//go:build darwin

package system

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/s0x401/xdfile-manager/src/internal/utils"
)

const darwinClipboardHeader = "XDFILE_PATHS_V1"

func readClipboardPaths() ([]string, error) {
	text, err := readDarwinClipboardText()
	if err != nil || text == "" {
		return nil, err
	}

	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	start := 0
	if len(lines) >= 2 && strings.TrimSpace(lines[0]) == darwinClipboardHeader {
		start = 2
	}

	paths := make([]string, 0, len(lines)-start)
	for _, line := range lines[start:] {
		path := filepath.Clean(strings.TrimSpace(line))
		if path == "" {
			continue
		}
		paths = append(paths, path)
	}
	return paths, nil
}

func readClipboardCut() (bool, error) {
	text, err := readDarwinClipboardText()
	if err != nil || text == "" {
		return false, err
	}

	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if len(lines) < 2 || strings.TrimSpace(lines[0]) != darwinClipboardHeader {
		return false, nil
	}
	return strings.EqualFold(strings.TrimSpace(lines[1]), "move"), nil
}

func writeClipboardPaths(paths []string, cut bool) error {
	if len(paths) == 0 {
		return nil
	}

	cleaned := make([]string, 0, len(paths))
	for _, path := range paths {
		path = filepath.Clean(strings.TrimSpace(path))
		if path != "" {
			cleaned = append(cleaned, path)
		}
	}
	if len(cleaned) == 0 {
		return nil
	}

	mode := "copy"
	if cut {
		mode = "move"
	}
	payload := darwinClipboardHeader + "\n" + mode + "\n" + strings.Join(cleaned, "\n")

	cmd := exec.Command("pbcopy")
	cmd.Stdin = strings.NewReader(payload)
	return cmd.Run()
}

func openPath(path string) error {
	if path == "" {
		return fmt.Errorf("empty path")
	}
	cmd := exec.Command("open", path)
	utils.DetachFromTerminal(cmd)
	return cmd.Start()
}

func showProperties(path string) error {
	if path == "" {
		return fmt.Errorf("empty path")
	}
	return fmt.Errorf("system properties dialog is unavailable on macOS")
}

func configureManagedExternalCommand(cmd *exec.Cmd) {
}

func commandMenuShortPath(path string) (string, error) {
	return path, nil
}

func commandMenuEncodeWithCodePage(text string, encoding CommandMenuListEncoding) ([]byte, error) {
	return []byte(text), nil
}

func readDarwinClipboardText() (string, error) {
	cmd := exec.Command("pbpaste")
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) == 0 {
			return "", nil
		}
		return "", err
	}
	return strings.TrimRight(string(output), "\n"), nil
}
