//go:build !windows

package cmd

func xdfileIsBenignPTYReadError(err error) bool {
	return false
}
