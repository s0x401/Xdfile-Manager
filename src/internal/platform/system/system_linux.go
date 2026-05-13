//go:build linux

package system

import (
	"fmt"
	"net/url"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/s0x401/xdfile-manager/src/internal/utils"
)

const linuxFileClipboardMIME = "x-special/gnome-copied-files"

type linuxClipboardCommand struct {
	name string
	args []string
}

type linuxClipboardPayload struct {
	paths []string
	cut   bool
}

func readClipboardPaths() ([]string, error) {
	payload, ok, err := readLinuxClipboardPayload()
	if err != nil || !ok {
		return nil, err
	}
	return payload.paths, nil
}

func readClipboardCut() (bool, error) {
	payload, ok, err := readLinuxClipboardPayload()
	if err != nil || !ok {
		return false, err
	}
	return payload.cut, nil
}

func writeClipboardPaths(paths []string, cut bool) error {
	cleaned := cleanLinuxClipboardPaths(paths)
	if len(cleaned) == 0 {
		return nil
	}

	payload := linuxClipboardPayloadText(cleaned, cut)
	if err := runLinuxClipboardWriteCommands(linuxClipboardFileWriteCommands(), payload); err == nil {
		return nil
	}
	return runLinuxClipboardWriteCommands(linuxClipboardTextWriteCommands(), payload)
}

func readLinuxClipboardPayload() (linuxClipboardPayload, bool, error) {
	if !linuxClipboardReadToolAvailable() {
		return linuxClipboardPayload{}, false, fmt.Errorf("Linux clipboard tool not found: install wl-clipboard, xclip, or xsel")
	}

	if text, ok := runLinuxClipboardReadCommands(linuxClipboardFileReadCommands()); ok {
		if payload, parsed := parseLinuxClipboardPayload(text); parsed {
			return payload, true, nil
		}
	}
	if text, ok := runLinuxClipboardReadCommands(linuxClipboardTextReadCommands()); ok {
		if payload, parsed := parseLinuxClipboardPayload(text); parsed {
			return payload, true, nil
		}
	}
	return linuxClipboardPayload{}, false, nil
}

func cleanLinuxClipboardPaths(paths []string) []string {
	cleaned := make([]string, 0, len(paths))
	for _, path := range paths {
		path = filepath.Clean(strings.TrimSpace(path))
		if path == "." || path == "" {
			continue
		}
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
		cleaned = append(cleaned, path)
	}
	return cleaned
}

func linuxClipboardPayloadText(paths []string, cut bool) string {
	mode := "copy"
	if cut {
		mode = "cut"
	}

	lines := make([]string, 0, len(paths)+1)
	lines = append(lines, mode)
	for _, path := range paths {
		lines = append(lines, linuxFileURI(path))
	}
	return strings.Join(lines, "\n")
}

func linuxFileURI(path string) string {
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(filepath.Clean(path))}).String()
}

func parseLinuxClipboardPayload(text string) (linuxClipboardPayload, bool) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.Trim(text, "\x00\n\t ")
	if text == "" {
		return linuxClipboardPayload{}, false
	}

	lines := strings.Split(text, "\n")
	start := 0
	cut := false
	switch strings.ToLower(strings.TrimSpace(lines[0])) {
	case "copy":
		start = 1
	case "cut", "move":
		start = 1
		cut = true
	}

	paths := make([]string, 0, len(lines)-start)
	for _, line := range lines[start:] {
		path, ok := parseLinuxClipboardPathLine(line)
		if ok {
			paths = append(paths, path)
		}
	}
	if len(paths) == 0 {
		return linuxClipboardPayload{}, false
	}
	return linuxClipboardPayload{paths: paths, cut: cut}, true
}

func parseLinuxClipboardPathLine(line string) (string, bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", false
	}

	if strings.HasPrefix(strings.ToLower(line), "file://") {
		parsed, err := url.Parse(line)
		if err != nil || parsed.Scheme != "file" {
			return "", false
		}
		path := filepath.FromSlash(parsed.Path)
		if parsed.Host != "" && !strings.EqualFold(parsed.Host, "localhost") {
			path = filepath.Clean("//" + parsed.Host + path)
		}
		if filepath.IsAbs(path) {
			return filepath.Clean(path), true
		}
		return "", false
	}

	path := filepath.Clean(line)
	if filepath.IsAbs(path) {
		return path, true
	}
	return "", false
}

func linuxClipboardReadToolAvailable() bool {
	for _, name := range []string{"wl-paste", "xclip", "xsel"} {
		if _, err := exec.LookPath(name); err == nil {
			return true
		}
	}
	return false
}

func runLinuxClipboardReadCommands(commands []linuxClipboardCommand) (string, bool) {
	for _, candidate := range commands {
		path, err := exec.LookPath(candidate.name)
		if err != nil {
			continue
		}
		cmd := exec.Command(path, candidate.args...)
		output, err := cmd.Output()
		if err != nil {
			continue
		}
		text := strings.TrimRight(string(output), "\x00\r\n")
		if strings.TrimSpace(text) != "" {
			return text, true
		}
	}
	return "", false
}

func runLinuxClipboardWriteCommands(commands []linuxClipboardCommand, payload string) error {
	found := false
	var lastErr error
	for _, candidate := range commands {
		path, err := exec.LookPath(candidate.name)
		if err != nil {
			continue
		}
		found = true
		cmd := exec.Command(path, candidate.args...)
		cmd.Stdin = strings.NewReader(payload)
		if err := cmd.Run(); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	if !found {
		return fmt.Errorf("Linux clipboard tool not found: install wl-clipboard, xclip, or xsel")
	}
	return fmt.Errorf("write Linux clipboard: %w", lastErr)
}

func linuxClipboardFileReadCommands() []linuxClipboardCommand {
	return []linuxClipboardCommand{
		{name: "wl-paste", args: []string{"--no-newline", "--type", linuxFileClipboardMIME}},
		{name: "xclip", args: []string{"-selection", "clipboard", "-t", linuxFileClipboardMIME, "-o"}},
		{name: "wl-paste", args: []string{"--no-newline", "--type", "text/uri-list"}},
		{name: "xclip", args: []string{"-selection", "clipboard", "-t", "text/uri-list", "-o"}},
	}
}

func linuxClipboardTextReadCommands() []linuxClipboardCommand {
	return []linuxClipboardCommand{
		{name: "wl-paste", args: []string{"--no-newline"}},
		{name: "xclip", args: []string{"-selection", "clipboard", "-o"}},
		{name: "xsel", args: []string{"--clipboard", "--output"}},
	}
}

func linuxClipboardFileWriteCommands() []linuxClipboardCommand {
	return []linuxClipboardCommand{
		{name: "wl-copy", args: []string{"--type", linuxFileClipboardMIME}},
		{name: "xclip", args: []string{"-selection", "clipboard", "-t", linuxFileClipboardMIME, "-i"}},
	}
}

func linuxClipboardTextWriteCommands() []linuxClipboardCommand {
	return []linuxClipboardCommand{
		{name: "wl-copy"},
		{name: "xclip", args: []string{"-selection", "clipboard", "-i"}},
		{name: "xsel", args: []string{"--clipboard", "--input"}},
	}
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
	return fmt.Errorf("system properties dialog is unavailable on Linux")
}

func showContextMenu(_ []string) error {
	return fmt.Errorf("native Windows context menu is unavailable on Linux")
}

func configureManagedExternalCommand(cmd *exec.Cmd) {
}

func commandMenuShortPath(path string) (string, error) {
	return path, nil
}

func commandMenuEncodeWithCodePage(text string, encoding CommandMenuListEncoding) ([]byte, error) {
	return []byte(text), nil
}
