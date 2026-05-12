//go:build windows

package cmd

import (
	"errors"
	"strings"

	"golang.org/x/sys/windows"
)

func xdfileIsBenignPTYReadError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, windows.ERROR_BROKEN_PIPE) || errors.Is(err, windows.ERROR_PIPE_NOT_CONNECTED) {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "pipe has been ended") ||
		strings.Contains(message, "broken pipe") ||
		strings.Contains(message, "pipe is being closed")
}
