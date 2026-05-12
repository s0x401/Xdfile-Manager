//go:build !windows

package cmd

func xdfileWindowsCmdPath() string {
	return "cmd.exe"
}
