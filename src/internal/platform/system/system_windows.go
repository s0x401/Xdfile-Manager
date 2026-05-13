//go:build windows

package system

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	clipboardCFHDrop       = 15
	clipboardCFUnicodeText = 13

	clipboardGMEMMoveable = 0x0002
	clipboardGMEMZeroInit = 0x0040

	clipboardDropEffectCopy = 1
	clipboardDropEffectMove = 2

	createNoWindow = 0x08000000

	shellOpenAccessDenied = 5
	shellOpenNoAssoc      = 31
	shellOpenOutOfMemory  = 8
	shellOpenFileNotFound = 2
	shellOpenPathNotFound = 3
	shellOpenBadFormat    = 11

	shellShowNormal     = 1
	seeMaskInvokeIDList = 0x0000000C
	seeMaskFlagNoUI     = 0x00000400

	windowsErrorOutOfMemory            syscall.Errno = 8
	windowsErrorBadFormat              syscall.Errno = 11
	windowsErrorNoAssociation          syscall.Errno = 1155
	windowsErrorAccessDisabledByPolicy syscall.Errno = 1260
)

type dropFiles struct {
	PFiles uint32
	X      int32
	Y      int32
	FNC    uint32
	FWide  uint32
}

type shellExecuteInfo struct {
	CbSize       uint32
	FMask        uint32
	Hwnd         windows.Handle
	LpVerb       *uint16
	LpFile       *uint16
	LpParameters *uint16
	LpDirectory  *uint16
	NShow        int32
	HInstApp     windows.Handle
	LpIDList     unsafe.Pointer
	LpClass      *uint16
	HkeyClass    windows.Handle
	DwHotKey     uint32
	HIcon        windows.Handle
	HProcess     windows.Handle
}

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	shell32  = windows.NewLazySystemDLL("shell32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procOpenClipboard            = user32.NewProc("OpenClipboard")
	procCloseClipboard           = user32.NewProc("CloseClipboard")
	procEmptyClipboard           = user32.NewProc("EmptyClipboard")
	procGetClipboardData         = user32.NewProc("GetClipboardData")
	procSetClipboardData         = user32.NewProc("SetClipboardData")
	procRegisterClipboardFormatW = user32.NewProc("RegisterClipboardFormatW")

	procGlobalAlloc  = kernel32.NewProc("GlobalAlloc")
	procGlobalLock   = kernel32.NewProc("GlobalLock")
	procGlobalSize   = kernel32.NewProc("GlobalSize")
	procGlobalUnlock = kernel32.NewProc("GlobalUnlock")
	procGlobalFree   = kernel32.NewProc("GlobalFree")
	procCopyMemory   = kernel32.NewProc("RtlMoveMemory")

	procDragQueryFileW          = shell32.NewProc("DragQueryFileW")
	procShellExecuteExW         = shell32.NewProc("ShellExecuteExW")
	kernel32ProcGetShortPathW   = kernel32.NewProc("GetShortPathNameW")
	kernel32ProcGetACP          = kernel32.NewProc("GetACP")
	kernel32ProcGetOEMCP        = kernel32.NewProc("GetOEMCP")
	kernel32ProcWideCharToMulti = kernel32.NewProc("WideCharToMultiByte")
)

func readClipboardPaths() ([]string, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := openClipboardRetry(); err != nil {
		return nil, err
	}
	defer procCloseClipboard.Call()

	handle, _, err := procGetClipboardData.Call(clipboardCFHDrop)
	if handle == 0 {
		if err != windows.ERROR_SUCCESS {
			return nil, fmt.Errorf("read Windows file clipboard: %w", err)
		}
		return readClipboardTextPathsLocked()
	}

	count, _, err := procDragQueryFileW.Call(handle, ^uintptr(0), 0, 0)
	if count == 0 {
		if err != windows.ERROR_SUCCESS {
			return nil, fmt.Errorf("query Windows file clipboard: %w", err)
		}
		return readClipboardTextPathsLocked()
	}

	paths := make([]string, 0, count)
	for i := uintptr(0); i < count; i++ {
		length, _, err := procDragQueryFileW.Call(handle, i, 0, 0)
		if length == 0 {
			if err != windows.ERROR_SUCCESS {
				return nil, fmt.Errorf("query Windows file clipboard path length: %w", err)
			}
			continue
		}

		buf := make([]uint16, length+1)
		written, _, err := procDragQueryFileW.Call(
			handle,
			i,
			uintptr(unsafe.Pointer(&buf[0])),
			uintptr(len(buf)),
		)
		if written == 0 && err != windows.ERROR_SUCCESS {
			return nil, fmt.Errorf("read Windows file clipboard path: %w", err)
		}

		path := filepath.Clean(windows.UTF16ToString(buf))
		if path != "" {
			paths = append(paths, path)
		}
	}

	return paths, nil
}

func readClipboardCut() (bool, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := openClipboardRetry(); err != nil {
		return false, err
	}
	defer procCloseClipboard.Call()

	format, err := registerClipboardFormat("Preferred DropEffect")
	if err != nil {
		return false, fmt.Errorf("register clipboard drop effect: %w", err)
	}

	value, ok, err := readClipboardUint32Locked(format)
	if err != nil {
		return false, fmt.Errorf("read Windows drop effect clipboard: %w", err)
	}
	if !ok {
		return false, nil
	}
	return value&clipboardDropEffectMove != 0, nil
}

func writeClipboardPaths(paths []string, cut bool) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if len(paths) == 0 {
		return nil
	}

	cleaned := make([]string, 0, len(paths))
	for _, path := range paths {
		path = filepath.Clean(path)
		if path != "" {
			cleaned = append(cleaned, path)
		}
	}
	if len(cleaned) == 0 {
		return nil
	}

	if err := openClipboardRetry(); err != nil {
		return err
	}
	defer procCloseClipboard.Call()

	if result, _, err := procEmptyClipboard.Call(); result == 0 {
		return fmt.Errorf("clear Windows clipboard: %w", err)
	}

	if err := setClipboardBytes(clipboardCFHDrop, buildDropFilesBytes(cleaned)); err != nil {
		return fmt.Errorf("write Windows file clipboard: %w", err)
	}
	if err := setClipboardBytes(clipboardCFUnicodeText, utf16TextBytes(strings.Join(cleaned, "\r\n"))); err != nil {
		return fmt.Errorf("write Windows text clipboard: %w", err)
	}

	format, err := registerClipboardFormat("Preferred DropEffect")
	if err != nil {
		return fmt.Errorf("register clipboard drop effect: %w", err)
	}
	dropEffect := clipboardDropEffectCopy
	if cut {
		dropEffect = clipboardDropEffectMove
	}
	effectBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(effectBytes, uint32(dropEffect))
	if err := setClipboardBytes(format, effectBytes); err != nil {
		return fmt.Errorf("write Windows drop effect clipboard: %w", err)
	}

	return nil
}

func configureManagedExternalCommand(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= createNoWindow
}

func openPath(path string) error {
	if path == "" {
		return fmt.Errorf("empty path")
	}
	return openPathWithShellExecuteEx(path)
}

func openPathWithShellExecuteEx(path string) error {
	verbPtr, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return fmt.Errorf("encode Windows open verb: %w", err)
	}
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return fmt.Errorf("encode Windows open path: %w", err)
	}

	sei := shellExecuteInfo{
		CbSize: uint32(unsafe.Sizeof(shellExecuteInfo{})),
		FMask:  seeMaskFlagNoUI,
		LpVerb: verbPtr,
		LpFile: pathPtr,
		NShow:  shellShowNormal,
	}

	result, _, callErr := procShellExecuteExW.Call(uintptr(unsafe.Pointer(&sei)))
	if result != 0 {
		return nil
	}

	return shellExecuteOpenError(callErr, sei.HInstApp)
}

func shellExecuteOpenError(callErr error, hInst windows.Handle) error {
	if errno, ok := callErr.(syscall.Errno); ok && errno != 0 {
		return shellOpenErrnoError(errno)
	}

	code := uintptr(hInst)
	if code > 0 && code <= 32 {
		return shellOpenCodeError(code)
	}

	return fmt.Errorf("open path failed")
}

func shellOpenErrnoError(errno syscall.Errno) error {
	switch errno {
	case windowsErrorAccessDisabledByPolicy:
		return fmt.Errorf("blocked by Windows policy")
	case syscall.ERROR_ACCESS_DENIED:
		return fmt.Errorf("access denied or blocked by Windows policy")
	case windowsErrorNoAssociation:
		return fmt.Errorf("no application is associated with this file type")
	case syscall.ERROR_FILE_NOT_FOUND:
		return fmt.Errorf("file not found")
	case syscall.ERROR_PATH_NOT_FOUND:
		return fmt.Errorf("path not found")
	case windowsErrorBadFormat:
		return fmt.Errorf("invalid executable format")
	case windowsErrorOutOfMemory:
		return fmt.Errorf("not enough memory to open this item")
	}
	return fmt.Errorf("open path: %w", errno)
}

func shellOpenCodeError(code uintptr) error {
	switch code {
	case shellOpenAccessDenied:
		return fmt.Errorf("access denied or blocked by Windows policy")
	case shellOpenNoAssoc:
		return fmt.Errorf("no application is associated with this file type")
	case shellOpenFileNotFound:
		return fmt.Errorf("file not found")
	case shellOpenPathNotFound:
		return fmt.Errorf("path not found")
	case shellOpenBadFormat:
		return fmt.Errorf("invalid executable format")
	case shellOpenOutOfMemory:
		return fmt.Errorf("not enough memory to open this item")
	}
	return fmt.Errorf("open path failed with ShellExecute code %d", code)
}

func showProperties(path string) error {
	if path == "" {
		return fmt.Errorf("empty path")
	}

	verbPtr, err := windows.UTF16PtrFromString("properties")
	if err != nil {
		return fmt.Errorf("encode Windows properties verb: %w", err)
	}
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return fmt.Errorf("encode Windows properties path: %w", err)
	}

	sei := shellExecuteInfo{
		CbSize: uint32(unsafe.Sizeof(shellExecuteInfo{})),
		FMask:  seeMaskInvokeIDList,
		LpVerb: verbPtr,
		LpFile: pathPtr,
		NShow:  shellShowNormal,
	}

	result, _, callErr := procShellExecuteExW.Call(uintptr(unsafe.Pointer(&sei)))
	if result == 0 {
		if callErr != nil && callErr != windows.ERROR_SUCCESS {
			return fmt.Errorf("open Windows properties dialog: %w", callErr)
		}
		return fmt.Errorf("open Windows properties dialog: ShellExecuteExW failed")
	}
	return nil
}

func commandMenuShortPath(path string) (string, error) {
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return "", err
	}

	size, _, callErr := kernel32ProcGetShortPathW.Call(
		uintptr(unsafe.Pointer(ptr)),
		0,
		0,
	)
	if size == 0 {
		if callErr != syscall.Errno(0) {
			return "", callErr
		}
		return "", fmt.Errorf("GetShortPathNameW(%s) returned 0", path)
	}

	buf := make([]uint16, size)
	size, _, callErr = kernel32ProcGetShortPathW.Call(
		uintptr(unsafe.Pointer(ptr)),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
	)
	if size == 0 {
		if callErr != syscall.Errno(0) {
			return "", callErr
		}
		return "", fmt.Errorf("GetShortPathNameW(%s) returned 0", path)
	}
	return windows.UTF16ToString(buf[:size]), nil
}

func commandMenuEncodeWithCodePage(text string, encoding CommandMenuListEncoding) ([]byte, error) {
	if text == "" {
		return []byte{}, nil
	}

	codePage := windowsOEMCodePage()
	if encoding == CommandMenuListEncodingANSI {
		codePage = windowsANSICodePage()
	}

	utf16Text, err := windows.UTF16FromString(text)
	if err != nil {
		return nil, err
	}

	size, _, callErr := kernel32ProcWideCharToMulti.Call(
		uintptr(codePage),
		0,
		uintptr(unsafe.Pointer(&utf16Text[0])),
		uintptr(len(utf16Text)-1),
		0,
		0,
		0,
		0,
	)
	if size == 0 {
		if callErr != syscall.Errno(0) {
			return nil, callErr
		}
		return nil, fmt.Errorf("WideCharToMultiByte(%d) returned 0", codePage)
	}

	buf := make([]byte, size)
	size, _, callErr = kernel32ProcWideCharToMulti.Call(
		uintptr(codePage),
		0,
		uintptr(unsafe.Pointer(&utf16Text[0])),
		uintptr(len(utf16Text)-1),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
		0,
		0,
	)
	if size == 0 {
		if callErr != syscall.Errno(0) {
			return nil, callErr
		}
		return nil, fmt.Errorf("WideCharToMultiByte(%d) returned 0", codePage)
	}
	return buf[:size], nil
}

func openClipboardRetry() error {
	var lastErr error
	for range 15 {
		if result, _, err := procOpenClipboard.Call(0); result != 0 {
			return nil
		} else if err != windows.ERROR_SUCCESS {
			lastErr = err
		}
		time.Sleep(20 * time.Millisecond)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("clipboard is busy")
	}
	return fmt.Errorf("open Windows clipboard: %w", lastErr)
}

func setClipboardBytes(format uint32, data []byte) error {
	if len(data) == 0 {
		data = []byte{0}
	}

	handle, _, err := procGlobalAlloc.Call(
		clipboardGMEMMoveable|clipboardGMEMZeroInit,
		uintptr(len(data)),
	)
	if handle == 0 {
		return fmt.Errorf("allocate clipboard memory: %w", err)
	}

	ptr, _, err := procGlobalLock.Call(handle)
	if ptr == 0 {
		procGlobalFree.Call(handle)
		return fmt.Errorf("lock clipboard memory: %w", err)
	}

	copyToClipboardMemory(ptr, data)
	procGlobalUnlock.Call(handle)

	result, _, err := procSetClipboardData.Call(uintptr(format), handle)
	if result == 0 {
		procGlobalFree.Call(handle)
		return fmt.Errorf("set clipboard data: %w", err)
	}
	return nil
}

func buildDropFilesBytes(paths []string) []byte {
	buffer := bytes.NewBuffer(make([]byte, 0, len(paths)*64))
	header := dropFiles{
		PFiles: uint32(binary.Size(dropFiles{})),
		FWide:  1,
	}
	_ = binary.Write(buffer, binary.LittleEndian, header)
	for _, path := range paths {
		utf16Path := windows.StringToUTF16(path)
		for _, value := range utf16Path[:len(utf16Path)-1] {
			_ = binary.Write(buffer, binary.LittleEndian, value)
		}
		_ = binary.Write(buffer, binary.LittleEndian, uint16(0))
	}
	_ = binary.Write(buffer, binary.LittleEndian, uint16(0))
	return buffer.Bytes()
}

func utf16TextBytes(value string) []byte {
	encoded := windows.StringToUTF16(value)
	data := make([]byte, len(encoded)*2)
	for i, wchar := range encoded {
		binary.LittleEndian.PutUint16(data[i*2:], wchar)
	}
	return data
}

func registerClipboardFormat(name string) (uint32, error) {
	ptr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return 0, err
	}
	result, _, callErr := procRegisterClipboardFormatW.Call(uintptr(unsafe.Pointer(ptr)))
	if result == 0 {
		return 0, callErr
	}
	return uint32(result), nil
}

func readClipboardUint32Locked(format uint32) (uint32, bool, error) {
	handle, _, err := procGetClipboardData.Call(uintptr(format))
	if handle == 0 {
		if err == windows.ERROR_SUCCESS {
			return 0, false, nil
		}
		return 0, false, err
	}
	ptr, size, err := clipboardMemoryBytes(handle)
	if err != nil {
		return 0, false, err
	}
	if size < 4 {
		return 0, false, nil
	}
	return binary.LittleEndian.Uint32(ptr[:4]), true, nil
}

func clipboardMemoryBytes(handle uintptr) ([]byte, uintptr, error) {
	ptr, _, err := procGlobalLock.Call(handle)
	if ptr == 0 {
		return nil, 0, err
	}
	defer procGlobalUnlock.Call(handle)

	size, _, err := procGlobalSize.Call(handle)
	if size == 0 {
		return nil, 0, err
	}
	if size > uintptr(^uint(0)>>1) {
		return nil, 0, fmt.Errorf("clipboard memory is too large: %d bytes", size)
	}
	data := make([]byte, int(size))
	copyFromClipboardMemory(data, ptr)
	return data, size, nil
}

func copyToClipboardMemory(dst uintptr, src []byte) {
	if len(src) == 0 {
		return
	}
	procCopyMemory.Call(dst, uintptr(unsafe.Pointer(&src[0])), uintptr(len(src)))
}

func copyFromClipboardMemory(dst []byte, src uintptr) {
	if len(dst) == 0 {
		return
	}
	procCopyMemory.Call(uintptr(unsafe.Pointer(&dst[0])), src, uintptr(len(dst)))
}

func readClipboardTextPathsLocked() ([]string, error) {
	text, ok, err := readClipboardUTF16TextLocked()
	if err != nil || !ok {
		return nil, err
	}

	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	paths := make([]string, 0, len(lines))
	for _, line := range lines {
		path := strings.TrimSpace(line)
		if path == "" {
			continue
		}
		if _, err := windows.GetFileAttributes(windows.StringToUTF16Ptr(path)); err != nil {
			continue
		}
		paths = append(paths, filepath.Clean(path))
	}
	return paths, nil
}

func readClipboardUTF16TextLocked() (string, bool, error) {
	handle, _, err := procGetClipboardData.Call(clipboardCFUnicodeText)
	if handle == 0 {
		if err == windows.ERROR_SUCCESS {
			return "", false, nil
		}
		return "", false, err
	}

	data, size, err := clipboardMemoryBytes(handle)
	if err != nil {
		return "", false, err
	}
	if size < 2 {
		return "", false, nil
	}

	wide := make([]uint16, int(size/2))
	for i := range wide {
		wide[i] = binary.LittleEndian.Uint16(data[i*2:])
	}
	end := 0
	for end < len(wide) && wide[end] != 0 {
		end++
	}
	return windows.UTF16ToString(wide[:end]), true, nil
}

func windowsANSICodePage() uint32 {
	result, _, _ := kernel32ProcGetACP.Call()
	return uint32(result)
}

func windowsOEMCodePage() uint32 {
	result, _, _ := kernel32ProcGetOEMCP.Call()
	return uint32(result)
}
