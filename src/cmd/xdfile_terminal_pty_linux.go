//go:build linux

package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"

	"golang.org/x/sys/unix"
)

type xdfileLinuxPTYBackend struct {
	master *os.File
}

func (b *xdfileLinuxPTYBackend) Read(p []byte) (int, error) {
	return b.master.Read(p)
}

func (b *xdfileLinuxPTYBackend) Write(p []byte) (int, error) {
	return b.master.Write(p)
}

func (b *xdfileLinuxPTYBackend) Close() error {
	if b == nil || b.master == nil {
		return nil
	}
	return b.master.Close()
}

func (b *xdfileLinuxPTYBackend) Resize(width int, height int) error {
	if b == nil || b.master == nil {
		return nil
	}
	return xdfileLinuxSetPTYSize(int(b.master.Fd()), width, height)
}

func xdfileStartCommandPTYBackend(
	dir string,
	path string,
	args []string,
	width int,
	height int,
) (xdfileTerminalPTYBackend, *os.Process, error) {
	master, slave, err := xdfileLinuxOpenPTY(width, height)
	if err != nil {
		return nil, nil, err
	}

	cmd := exec.Command(path, args...)
	cmd.Dir = dir
	cmd.Env = xdfileCommandExecutionEnvironment(os.Environ())
	cmd.Stdin = slave
	cmd.Stdout = slave
	cmd.Stderr = slave
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid:  true,
		Setctty: true,
		Ctty:    0,
	}

	if err := cmd.Start(); err != nil {
		_ = slave.Close()
		_ = master.Close()
		return nil, nil, fmt.Errorf("spawn PTY command: %w", err)
	}
	_ = slave.Close()

	return &xdfileLinuxPTYBackend{master: master}, cmd.Process, nil
}

func xdfileLinuxOpenPTY(width int, height int) (*os.File, *os.File, error) {
	masterFD, err := unix.Open("/dev/ptmx", unix.O_RDWR|unix.O_NOCTTY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("open PTY master: %w", err)
	}
	master := os.NewFile(uintptr(masterFD), "/dev/ptmx")
	cleanupMaster := true
	defer func() {
		if cleanupMaster {
			_ = master.Close()
		}
	}()

	if err := unix.IoctlSetPointerInt(masterFD, unix.TIOCSPTLCK, 0); err != nil {
		return nil, nil, fmt.Errorf("unlock PTY slave: %w", err)
	}
	ptyNumber, err := unix.IoctlGetInt(masterFD, unix.TIOCGPTN)
	if err != nil {
		return nil, nil, fmt.Errorf("query PTY slave: %w", err)
	}
	slavePath := "/dev/pts/" + strconv.Itoa(ptyNumber)
	slave, err := os.OpenFile(slavePath, os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("open PTY slave %s: %w", slavePath, err)
	}
	cleanupSlave := true
	defer func() {
		if cleanupSlave {
			_ = slave.Close()
		}
	}()

	if err := xdfileLinuxSetPTYSize(masterFD, width, height); err != nil {
		return nil, nil, err
	}

	cleanupMaster = false
	cleanupSlave = false
	return master, slave, nil
}

func xdfileLinuxSetPTYSize(fd int, width int, height int) error {
	size := &unix.Winsize{
		Col: uint16(max(1, width)),
		Row: uint16(max(1, height)),
	}
	if err := unix.IoctlSetWinsize(fd, unix.TIOCSWINSZ, size); err != nil {
		return fmt.Errorf("resize PTY: %w", err)
	}
	return nil
}
