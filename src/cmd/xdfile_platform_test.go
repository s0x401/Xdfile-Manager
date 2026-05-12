package cmd

import (
	"testing"

	platformterminal "github.com/s0x401/xdfile-manager/src/internal/platform/terminal"
)

func TestWindowRenderWidthLeavesRightEdgeForLegacyWindowsConsole(t *testing.T) {
	got := xdfileWindowRenderWidthForHost(120, platformterminal.HostInfo{RuntimeOS: "windows"})
	if got != 119 {
		t.Fatalf("expected legacy Windows console width guard, got %d", got)
	}
}

func TestWindowRenderWidthKeepsModernTerminalWidth(t *testing.T) {
	info := platformterminal.HostInfo{
		RuntimeOS:     "windows",
		WTSessionID:   "65e1",
		IsWindowsTerm: true,
	}
	got := xdfileWindowRenderWidthForHost(120, info)
	if got != 120 {
		t.Fatalf("expected Windows Terminal width to stay unchanged, got %d", got)
	}
}
