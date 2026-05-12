//go:build windows

package cmd

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"syscall"
	"unsafe"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/sys/windows"
)

const (
	xdfilePTYMouseProxyEnv    = "XDFILE_PTY_MOUSE_PROXY"
	xdfilePTYMouseProxyPIDEnv = "XDFILE_PTY_MOUSE_PROXY_PID"
)

const (
	xdfileConsoleMouseEventType = 0x0002

	xdfileConsoleMouseClick    = 0x0000
	xdfileConsoleMouseMoved    = 0x0001
	xdfileConsoleMouseWheeled  = 0x0004
	xdfileConsoleMouseHWheeled = 0x0008

	xdfileConsoleMouseLeft     = 0x0001
	xdfileConsoleMouseRight    = 0x0002
	xdfileConsoleMouseMiddle   = 0x0004
	xdfileConsoleMouseBackward = 0x0008
	xdfileConsoleMouseForward  = 0x0010
	xdfileConsoleWheelDelta    = 120

	xdfileConsoleLeftAlt  = 0x0002
	xdfileConsoleLeftCtrl = 0x0008
	xdfileConsoleShift    = 0x0010
)

type xdfilePTYMouseProxy struct {
	stdin   io.WriteCloser
	process *os.Process
	mu      sync.Mutex
	closed  bool
}

type xdfilePTYMouseProxyEvent struct {
	X      int
	Y      int
	Action tea.MouseAction
	Button tea.MouseButton
	Shift  bool
	Alt    bool
	Ctrl   bool
}

type xdfileConsoleInputRecord struct {
	EventType uint16
	_         [2]byte
	Event     [16]byte
}

var (
	xdfileMouseProxyKernel32              = windows.NewLazySystemDLL("kernel32.dll")
	xdfileMouseProxyAttachConsoleProc     = xdfileMouseProxyKernel32.NewProc("AttachConsole")
	xdfileMouseProxyFreeConsoleProc       = xdfileMouseProxyKernel32.NewProc("FreeConsole")
	xdfileMouseProxyWriteConsoleInputProc = xdfileMouseProxyKernel32.NewProc("WriteConsoleInputW")
)

func xdfileStartPTYMouseProxy(process *os.Process) *xdfilePTYMouseProxy {
	if process == nil || process.Pid <= 0 {
		return nil
	}

	exe, err := os.Executable()
	if err != nil || exe == "" {
		return nil
	}

	cmd := exec.Command(exe)
	cmd.Env = append(os.Environ(),
		xdfilePTYMouseProxyEnv+"=1",
		xdfilePTYMouseProxyPIDEnv+"="+strconv.Itoa(process.Pid),
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.DETACHED_PROCESS,
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil
	}
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return nil
	}

	return &xdfilePTYMouseProxy{
		stdin:   stdin,
		process: cmd.Process,
	}
}

func (p *xdfilePTYMouseProxy) Send(msg tea.MouseMsg, x int, y int) bool {
	if p == nil {
		return false
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed || p.stdin == nil {
		return false
	}

	_, err := fmt.Fprintf(
		p.stdin,
		"%d %d %d %d %t %t %t\n",
		x,
		y,
		msg.Action,
		msg.Button,
		msg.Shift,
		msg.Alt,
		msg.Ctrl,
	)
	if err != nil {
		p.closeLocked()
		return false
	}
	return true
}

func (p *xdfilePTYMouseProxy) Close() {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closeLocked()
}

func (p *xdfilePTYMouseProxy) closeLocked() {
	if p.closed {
		return
	}
	p.closed = true
	if p.stdin != nil {
		_ = p.stdin.Close()
		p.stdin = nil
	}
	if p.process != nil {
		_ = p.process.Kill()
		_ = p.process.Release()
		p.process = nil
	}
}

func xdfileMaybeRunPTYMouseProxy() bool {
	if os.Getenv(xdfilePTYMouseProxyEnv) != "1" {
		return false
	}

	code := 0
	if err := xdfileRunPTYMouseProxy(); err != nil {
		code = 1
	}
	os.Exit(code)
	return true
}

func xdfileRunPTYMouseProxy() error {
	pidText := os.Getenv(xdfilePTYMouseProxyPIDEnv)
	pid, err := strconv.ParseUint(pidText, 10, 32)
	if err != nil || pid == 0 {
		return fmt.Errorf("invalid PTY mouse proxy pid %q", pidText)
	}

	if err := xdfileAttachConsole(uint32(pid)); err != nil {
		return err
	}
	defer xdfileFreeConsole()

	conin, err := windows.CreateFile(
		windows.StringToUTF16Ptr("CONIN$"),
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_EXISTING,
		0,
		0,
	)
	if err != nil {
		return fmt.Errorf("open child console input: %w", err)
	}
	defer windows.CloseHandle(conin)

	reader := bufio.NewReader(os.Stdin)
	for {
		var ev xdfilePTYMouseProxyEvent
		_, err := fmt.Fscan(reader, &ev.X, &ev.Y, &ev.Action, &ev.Button, &ev.Shift, &ev.Alt, &ev.Ctrl)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		record, ok := xdfileMouseProxyInputRecord(ev)
		if !ok {
			continue
		}
		if err := xdfileWriteConsoleInput(conin, &record, 1); err != nil {
			return err
		}
	}
}

func xdfileMouseProxyInputRecord(ev xdfilePTYMouseProxyEvent) (xdfileConsoleInputRecord, bool) {
	var record xdfileConsoleInputRecord
	record.EventType = xdfileConsoleMouseEventType

	buttonState, eventFlags, ok := xdfileMouseProxyButtonState(ev)
	if !ok {
		return record, false
	}

	binary.LittleEndian.PutUint16(record.Event[0:2], uint16(max(0, ev.X)))
	binary.LittleEndian.PutUint16(record.Event[2:4], uint16(max(0, ev.Y)))
	binary.LittleEndian.PutUint32(record.Event[4:8], buttonState)
	binary.LittleEndian.PutUint32(record.Event[8:12], xdfileMouseProxyControlKeyState(ev))
	binary.LittleEndian.PutUint32(record.Event[12:16], eventFlags)
	return record, true
}

func xdfileMouseProxyButtonState(ev xdfilePTYMouseProxyEvent) (uint32, uint32, bool) {
	switch ev.Button {
	case tea.MouseButtonWheelUp:
		return xdfileMouseProxyWheelState(xdfileConsoleWheelDelta), xdfileConsoleMouseWheeled, true
	case tea.MouseButtonWheelDown:
		return xdfileMouseProxyWheelState(-xdfileConsoleWheelDelta), xdfileConsoleMouseWheeled, true
	case tea.MouseButtonWheelRight:
		return xdfileMouseProxyWheelState(xdfileConsoleWheelDelta), xdfileConsoleMouseHWheeled, true
	case tea.MouseButtonWheelLeft:
		return xdfileMouseProxyWheelState(-xdfileConsoleWheelDelta), xdfileConsoleMouseHWheeled, true
	}

	buttonState, ok := xdfileMouseProxyPressedButtonState(ev.Button)
	if !ok {
		return 0, 0, false
	}

	eventFlags := uint32(xdfileConsoleMouseClick)
	if ev.Action == tea.MouseActionMotion {
		eventFlags = xdfileConsoleMouseMoved
	}
	if ev.Action == tea.MouseActionRelease {
		buttonState = 0
	}
	return buttonState, eventFlags, true
}

func xdfileMouseProxyPressedButtonState(button tea.MouseButton) (uint32, bool) {
	switch button {
	case tea.MouseButtonLeft:
		return xdfileConsoleMouseLeft, true
	case tea.MouseButtonRight:
		return xdfileConsoleMouseRight, true
	case tea.MouseButtonMiddle:
		return xdfileConsoleMouseMiddle, true
	case tea.MouseButtonBackward:
		return xdfileConsoleMouseBackward, true
	case tea.MouseButtonForward:
		return xdfileConsoleMouseForward, true
	case tea.MouseButtonNone:
		return 0, true
	default:
		return 0, false
	}
}

func xdfileMouseProxyWheelState(delta int) uint32 {
	return uint32(uint16(int16(delta))) << 16
}

func xdfileMouseProxyControlKeyState(ev xdfilePTYMouseProxyEvent) uint32 {
	state := uint32(0)
	if ev.Shift {
		state |= xdfileConsoleShift
	}
	if ev.Alt {
		state |= xdfileConsoleLeftAlt
	}
	if ev.Ctrl {
		state |= xdfileConsoleLeftCtrl
	}
	return state
}

func xdfileAttachConsole(pid uint32) error {
	result, _, err := xdfileMouseProxyAttachConsoleProc.Call(uintptr(pid))
	if result == 0 {
		return fmt.Errorf("attach child console: %w", err)
	}
	return nil
}

func xdfileFreeConsole() {
	_, _, _ = xdfileMouseProxyFreeConsoleProc.Call()
}

func xdfileWriteConsoleInput(console windows.Handle, record *xdfileConsoleInputRecord, count uint32) error {
	var written uint32
	result, _, err := xdfileMouseProxyWriteConsoleInputProc.Call(
		uintptr(console),
		uintptr(unsafe.Pointer(record)),
		uintptr(count),
		uintptr(unsafe.Pointer(&written)),
	)
	if result == 0 {
		return fmt.Errorf("write child console input: %w", err)
	}
	return nil
}
