package system

import "os/exec"

type CommandMenuListEncoding int

const (
	CommandMenuListEncodingOEM CommandMenuListEncoding = iota
	CommandMenuListEncodingANSI
	CommandMenuListEncodingUTF8
	CommandMenuListEncodingUTF16LE
)

func ReadClipboardPaths() ([]string, error) {
	return readClipboardPaths()
}

func ReadClipboardCut() (bool, error) {
	return readClipboardCut()
}

func WriteClipboardPaths(paths []string, cut bool) error {
	return writeClipboardPaths(paths, cut)
}

func OpenPath(path string) error {
	return openPath(path)
}

func OpenPathDirect(path string) error {
	return openPath(path)
}

func ShowProperties(path string) error {
	return showProperties(path)
}

func ShowContextMenu(paths []string) error {
	return showContextMenu(paths)
}

func ConfigureManagedExternalCommand(cmd *exec.Cmd) {
	configureManagedExternalCommand(cmd)
}

func CommandMenuShortPath(path string) (string, error) {
	return commandMenuShortPath(path)
}

func CommandMenuEncodeWithCodePage(text string, encoding CommandMenuListEncoding) ([]byte, error) {
	return commandMenuEncodeWithCodePage(text, encoding)
}
