//go:build !windows

package cmd

import (
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

type xdfilePTYMouseProxy struct{}

func xdfileStartPTYMouseProxy(_ *os.Process) *xdfilePTYMouseProxy {
	return nil
}

func (p *xdfilePTYMouseProxy) Send(_ tea.MouseMsg, _ int, _ int) bool {
	return false
}

func (p *xdfilePTYMouseProxy) Close() {}

func xdfileMaybeRunPTYMouseProxy() bool {
	return false
}
