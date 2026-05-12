//go:build windows

package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestXdfileWindowsCmdPathReturnsAbsoluteExistingCommand(t *testing.T) {
	path := xdfileWindowsCmdPath()
	if !filepath.IsAbs(path) {
		t.Fatalf("expected absolute cmd.exe path, got %q", path)
	}
	if !strings.HasSuffix(strings.ToLower(path), "cmd.exe") {
		t.Fatalf("expected cmd.exe path, got %q", path)
	}
	if info, err := os.Stat(path); err != nil || info.IsDir() {
		t.Fatalf("expected cmd.exe path to point to an existing file, path=%q err=%v", path, err)
	}
}

func TestXdfileFirstExistingAbsoluteWindowsFileSkipsRelativePaths(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tool.exe")
	if err := os.WriteFile(path, []byte("fake"), 0o600); err != nil {
		t.Fatalf("write fake executable: %v", err)
	}

	got, ok := xdfileFirstExistingAbsoluteWindowsFile("tool.exe", `"`+path+`"`)
	if !ok {
		t.Fatal("expected absolute existing candidate to be selected")
	}
	if got != filepath.Clean(path) {
		t.Fatalf("expected %q, got %q", filepath.Clean(path), got)
	}
}

func TestXdfileNormalizeTerminalWorkingDirectoryWindowsFileURI(t *testing.T) {
	got := xdfileNormalizeTerminalWorkingDirectory(`file:///E:/work/xdfile`)
	if got != `E:\work\xdfile` {
		t.Fatalf("expected file URI to normalize to a Windows path, got %q", got)
	}
}
