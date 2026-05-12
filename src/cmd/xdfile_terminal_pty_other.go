//go:build !windows && !linux

package cmd

import (
	"errors"
	"os"
)

func xdfileStartCommandPTYBackend(
	dir string,
	path string,
	args []string,
	width int,
	height int,
) (xdfileTerminalPTYBackend, *os.Process, error) {
	return nil, nil, errors.New("PTY command execution is only implemented for Windows and Linux in Xdfile Manager")
}
