package cmd

import (
	"os/exec"

	platformsystem "github.com/s0x401/xdfile-manager/src/internal/platform/system"
	platformterminal "github.com/s0x401/xdfile-manager/src/internal/platform/terminal"
)

const xdfileCreateNoWindow = 0x08000000

var xdfileHostTerminal = platformterminal.DetectCurrent()
var xdfileShowSystemPropertiesFunc = platformsystem.ShowProperties
var xdfileShowNativeContextMenuFunc = platformsystem.ShowContextMenu

func xdfileWindowRenderWidth(width int) int {
	return xdfileWindowRenderWidthForHost(width, xdfileHostTerminal)
}

func xdfileWindowRenderWidthForHost(width int, host platformterminal.HostInfo) int {
	if width > 1 && host.NeedsRightEdgeGuard() {
		return width - 1
	}
	return width
}

func xdfileConfigureManagedExternalCommand(cmd *exec.Cmd) {
	platformsystem.ConfigureManagedExternalCommand(cmd)
}

func xdfileCommandMenuShortPath(path string) (string, error) {
	return platformsystem.CommandMenuShortPath(path)
}

func xdfileCommandMenuEncodeWithCodePage(text string, encoding xdfileCommandMenuListEncoding) ([]byte, error) {
	return platformsystem.CommandMenuEncodeWithCodePage(text, platformsystem.CommandMenuListEncoding(encoding))
}
