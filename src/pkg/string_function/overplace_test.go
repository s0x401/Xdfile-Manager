package stringfunction

import (
	"strings"
	"testing"

	charmansi "github.com/charmbracelet/x/ansi"
)

func TestPlaceOverlayKeepsAnsiEscapePrefixIntact(t *testing.T) {
	bg := "\x1b[38;2;229;231;235m Panels View Terminal Options \x1b[0m"
	fg := "\x1b[38;2;56;189;248m Commands \x1b[0m"

	rendered := PlaceOverlay(1, 0, fg, bg)
	if idx := strings.Index(rendered, "[38;2;229;231;235m"); idx >= 0 && (idx == 0 || rendered[idx-1] != '\x1b') {
		t.Fatalf("expected overlay to preserve ANSI escape prefixes, got %q", rendered)
	}
	if !strings.Contains(rendered, "\x1b[38;2;56;189;248m") {
		t.Fatalf("expected overlay output to retain foreground ANSI escape sequence, got %q", rendered)
	}
}

func TestPlaceOverlayClipsForegroundWiderThanBackground(t *testing.T) {
	bg := strings.Join([]string{
		"0123456789",
		"abcdefghij",
		"ABCDEFGHIJ",
	}, "\n")
	fg := strings.Join([]string{
		"long foreground line",
		"second foreground line",
		"third foreground line",
		"fourth foreground line",
	}, "\n")

	rendered := PlaceOverlay(8, 1, fg, bg)
	lines := strings.Split(rendered, "\n")
	if len(lines) != 3 {
		t.Fatalf("expected overlay to preserve background height, got %d lines: %q", len(lines), rendered)
	}
	for i, line := range lines {
		if width := charmansi.StringWidth(line); width != 10 {
			t.Fatalf("expected line %d to preserve width 10, got %d in %q", i, width, line)
		}
	}
	if strings.Contains(rendered, "fourth") {
		t.Fatalf("expected overlay to clip foreground height, got %q", rendered)
	}
}

func TestPlaceOverlayKeepsWideCharacterColumnsStable(t *testing.T) {
	bg := "中文文件ABCD\n0123456789"
	fg := "弹窗"

	rendered := PlaceOverlay(4, 0, fg, bg)
	lines := strings.Split(rendered, "\n")
	wantWidth := charmansi.StringWidth("中文文件ABCD")
	for i, line := range lines {
		if width := charmansi.StringWidth(line); width != wantWidth {
			t.Fatalf("expected line %d width %d, got %d in %q", i, wantWidth, width, line)
		}
	}
}
