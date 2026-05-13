//go:build windows

package system

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	coinitApartmentThreaded = 0x2
	sFalse                  = syscall.Errno(1)

	cmfNormal  = 0x00000000
	cmfExplore = 0x00000004

	tpmRightButton = 0x0002
	tpmReturnCmd   = 0x0100

	cmicMaskUnicode  = 0x00004000
	cmicMaskPtInvoke = 0x20000000

	wmNull          = 0x0000
	wmDrawItem      = 0x002B
	wmMeasureItem   = 0x002C
	wmInitMenuPopup = 0x0117
	wmMenuChar      = 0x0120

	wsPopup = 0x80000000

	shellContextMenuCommandFirst = 1
	shellContextMenuCommandLast  = 0x7FFF
)

var (
	procSHParseDisplayName = shell32.NewProc("SHParseDisplayName")
	procSHBindToParent     = shell32.NewProc("SHBindToParent")

	procRegisterClassExW = user32.NewProc("RegisterClassExW")
	procCreateWindowExW  = user32.NewProc("CreateWindowExW")
	procDestroyWindow    = user32.NewProc("DestroyWindow")
	procDefWindowProcW   = user32.NewProc("DefWindowProcW")
	procCreatePopupMenu  = user32.NewProc("CreatePopupMenu")
	procDestroyMenu      = user32.NewProc("DestroyMenu")
	procTrackPopupMenuEx = user32.NewProc("TrackPopupMenuEx")
	procGetCursorPos     = user32.NewProc("GetCursorPos")
	procSetForegroundWnd = user32.NewProc("SetForegroundWindow")
	procPostMessageW     = user32.NewProc("PostMessageW")

	shellContextMenuWndProc = syscall.NewCallback(shellContextMenuWindowProc)
)

var (
	iidIShellFolder  = windows.GUID{Data1: 0x000214e6, Data2: 0x0000, Data3: 0x0000, Data4: [8]byte{0xc0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
	iidIContextMenu  = windows.GUID{Data1: 0x000214e4, Data2: 0x0000, Data3: 0x0000, Data4: [8]byte{0xc0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
	iidIContextMenu2 = windows.GUID{Data1: 0x000214f4, Data2: 0x0000, Data3: 0x0000, Data4: [8]byte{0xc0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
	iidIContextMenu3 = windows.GUID{Data1: 0xbcfce0a0, Data2: 0xec17, Data3: 0x11d0, Data4: [8]byte{0x8d, 0x10, 0x00, 0xa0, 0xc9, 0x0f, 0x27, 0x19}}
)

type shellContextMenuPoint struct {
	X int32
	Y int32
}

type shellContextMenuWindowClass struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     windows.Handle
	HIcon         windows.Handle
	HCursor       windows.Handle
	HbrBackground windows.Handle
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       windows.Handle
}

type shellFolder struct {
	Vtbl *shellFolderVtbl
}

type shellFolderVtbl struct {
	QueryInterface   uintptr
	AddRef           uintptr
	Release          uintptr
	ParseDisplayName uintptr
	EnumObjects      uintptr
	BindToObject     uintptr
	BindToStorage    uintptr
	CompareIDs       uintptr
	CreateViewObject uintptr
	GetAttributesOf  uintptr
	GetUIObjectOf    uintptr
}

type shellContextMenu struct {
	Vtbl *shellContextMenuVtbl
}

type shellContextMenuVtbl struct {
	QueryInterface   uintptr
	AddRef           uintptr
	Release          uintptr
	QueryContextMenu uintptr
	InvokeCommand    uintptr
	GetCommandString uintptr
}

type shellContextMenu2 struct {
	Vtbl *shellContextMenu2Vtbl
}

type shellContextMenu2Vtbl struct {
	QueryInterface   uintptr
	AddRef           uintptr
	Release          uintptr
	QueryContextMenu uintptr
	InvokeCommand    uintptr
	GetCommandString uintptr
	HandleMenuMsg    uintptr
}

type shellContextMenu3 struct {
	Vtbl *shellContextMenu3Vtbl
}

type shellContextMenu3Vtbl struct {
	QueryInterface   uintptr
	AddRef           uintptr
	Release          uintptr
	QueryContextMenu uintptr
	InvokeCommand    uintptr
	GetCommandString uintptr
	HandleMenuMsg    uintptr
	HandleMenuMsg2   uintptr
}

type shellContextMenuInvokeInfoEx struct {
	CbSize        uint32
	FMask         uint32
	Hwnd          windows.Handle
	LpVerb        uintptr
	LpParameters  uintptr
	LpDirectory   uintptr
	NShow         int32
	DwHotKey      uint32
	HIcon         windows.Handle
	LpTitle       uintptr
	LpVerbW       uintptr
	LpParametersW uintptr
	LpDirectoryW  uintptr
	LpTitleW      uintptr
	PtInvoke      shellContextMenuPoint
}

type shellContextMenuWindowState struct {
	Menu2 *shellContextMenu2
	Menu3 *shellContextMenu3
}

type shellContextMenuSelection struct {
	Parent        *shellFolder
	AbsolutePIDLs []unsafe.Pointer
	ChildPIDLs    []unsafe.Pointer
}

var activeShellContextMenuWindow shellContextMenuWindowState

func showContextMenu(paths []string) error {
	cleaned := cleanShellContextMenuPaths(paths)
	if len(cleaned) == 0 {
		return fmt.Errorf("empty context menu selection")
	}
	if !sameShellContextMenuParent(cleaned) {
		return fmt.Errorf("Windows context menu selection must be in one directory")
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	comInitialized, err := initializeShellContextMenuCOM()
	if err != nil {
		return err
	}
	if comInitialized {
		defer windows.CoUninitialize()
	}

	hwnd, err := createShellContextMenuWindow()
	if err != nil {
		return err
	}
	defer procDestroyWindow.Call(uintptr(hwnd))

	selection, err := bindShellContextMenuSelection(cleaned)
	if err != nil {
		return err
	}
	defer selection.release()

	menu, err := selection.Parent.contextMenu(hwnd, selection.ChildPIDLs)
	if err != nil {
		return err
	}
	defer menu.release()

	menu2 := menu.queryContextMenu2()
	if menu2 != nil {
		defer menu2.release()
	}
	menu3 := menu.queryContextMenu3()
	if menu3 != nil {
		defer menu3.release()
	}

	hmenu, err := createShellPopupMenu()
	if err != nil {
		return err
	}
	defer procDestroyMenu.Call(uintptr(hmenu))

	if err := menu.queryContextMenu(hmenu); err != nil {
		return err
	}

	point, err := currentCursorPoint()
	if err != nil {
		return err
	}

	activeShellContextMenuWindow = shellContextMenuWindowState{Menu2: menu2, Menu3: menu3}
	defer func() {
		activeShellContextMenuWindow = shellContextMenuWindowState{}
	}()

	procSetForegroundWnd.Call(uintptr(hwnd))
	command, err := trackShellPopupMenu(hmenu, hwnd, point)
	procPostMessageW.Call(uintptr(hwnd), wmNull, 0, 0)
	if err != nil || command == 0 {
		return err
	}

	return menu.invokeCommand(hwnd, command-shellContextMenuCommandFirst, point)
}

func cleanShellContextMenuPaths(paths []string) []string {
	cleaned := make([]string, 0, len(paths))
	for _, path := range paths {
		path = filepath.Clean(path)
		if path != "" && path != "." {
			cleaned = append(cleaned, path)
		}
	}
	return cleaned
}

func sameShellContextMenuParent(paths []string) bool {
	if len(paths) <= 1 {
		return true
	}
	parent := filepath.Clean(filepath.Dir(paths[0]))
	for _, path := range paths[1:] {
		if !strings.EqualFold(parent, filepath.Clean(filepath.Dir(path))) {
			return false
		}
	}
	return true
}

func initializeShellContextMenuCOM() (bool, error) {
	err := windows.CoInitializeEx(0, coinitApartmentThreaded)
	if err == nil || err == sFalse {
		return true, nil
	}
	return false, fmt.Errorf("initialize Windows COM for context menu: %w", err)
}

func createShellContextMenuWindow() (windows.Handle, error) {
	className, err := windows.UTF16PtrFromString("XdfileShellContextMenuHelper")
	if err != nil {
		return 0, err
	}

	wc := shellContextMenuWindowClass{
		CbSize:        uint32(unsafe.Sizeof(shellContextMenuWindowClass{})),
		LpfnWndProc:   shellContextMenuWndProc,
		LpszClassName: className,
	}
	if atom, _, callErr := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); atom == 0 {
		if errno, ok := callErr.(syscall.Errno); !ok || errno != windows.ERROR_CLASS_ALREADY_EXISTS {
			return 0, fmt.Errorf("register Windows context menu helper window: %w", callErr)
		}
	}

	hwnd, _, callErr := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(className)),
		wsPopup,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
	)
	if hwnd == 0 {
		return 0, fmt.Errorf("create Windows context menu helper window: %w", callErr)
	}
	return windows.Handle(hwnd), nil
}

func shellContextMenuWindowProc(hwnd uintptr, msg uint32, wParam uintptr, lParam uintptr) uintptr {
	switch msg {
	case wmInitMenuPopup, wmDrawItem, wmMeasureItem, wmMenuChar:
		if menu3 := activeShellContextMenuWindow.Menu3; menu3 != nil {
			result, handled := menu3.handleMenuMsg2(msg, wParam, lParam)
			if handled {
				return result
			}
		}
		if menu2 := activeShellContextMenuWindow.Menu2; menu2 != nil && menu2.handleMenuMsg(msg, wParam, lParam) {
			return 0
		}
	}
	result, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return result
}

func (f *shellFolder) release() {
	if f != nil {
		syscall.SyscallN(f.Vtbl.Release, uintptr(unsafe.Pointer(f)))
	}
}

func bindShellContextMenuSelection(paths []string) (*shellContextMenuSelection, error) {
	selection := &shellContextMenuSelection{
		AbsolutePIDLs: make([]unsafe.Pointer, 0, len(paths)),
		ChildPIDLs:    make([]unsafe.Pointer, 0, len(paths)),
	}

	for _, path := range paths {
		pidl, err := parseShellDisplayName(path)
		if err != nil {
			selection.release()
			return nil, err
		}
		selection.AbsolutePIDLs = append(selection.AbsolutePIDLs, pidl)

		parent, child, err := bindShellPIDLToParent(pidl)
		if err != nil {
			selection.release()
			return nil, err
		}
		if selection.Parent == nil {
			selection.Parent = parent
		} else {
			parent.release()
		}
		selection.ChildPIDLs = append(selection.ChildPIDLs, child)
	}

	if selection.Parent == nil || len(selection.ChildPIDLs) == 0 {
		selection.release()
		return nil, fmt.Errorf("empty Windows context menu shell selection")
	}
	return selection, nil
}

func parseShellDisplayName(path string) (unsafe.Pointer, error) {
	displayName, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, fmt.Errorf("encode Windows context menu path: %w", err)
	}

	var pidl unsafe.Pointer
	var attrs uint32
	hr, _, _ := procSHParseDisplayName.Call(
		uintptr(unsafe.Pointer(displayName)),
		0,
		uintptr(unsafe.Pointer(&pidl)),
		0,
		uintptr(unsafe.Pointer(&attrs)),
	)
	if shellHRESULTFailed(hr) {
		return nil, shellHRESULTError("SHParseDisplayName", hr)
	}
	if pidl == nil {
		return nil, fmt.Errorf("SHParseDisplayName returned nil for %s", path)
	}
	return pidl, nil
}

func bindShellPIDLToParent(pidl unsafe.Pointer) (*shellFolder, unsafe.Pointer, error) {
	var parent *shellFolder
	var child unsafe.Pointer
	hr, _, _ := procSHBindToParent.Call(
		uintptr(pidl),
		uintptr(unsafe.Pointer(&iidIShellFolder)),
		uintptr(unsafe.Pointer(&parent)),
		uintptr(unsafe.Pointer(&child)),
	)
	if shellHRESULTFailed(hr) {
		return nil, nil, shellHRESULTError("SHBindToParent", hr)
	}
	if parent == nil || child == nil {
		if parent != nil {
			parent.release()
		}
		return nil, nil, fmt.Errorf("SHBindToParent returned an empty shell selection")
	}
	return parent, child, nil
}

func (f *shellFolder) contextMenu(hwnd windows.Handle, pidls []unsafe.Pointer) (*shellContextMenu, error) {
	if len(pidls) == 0 {
		return nil, fmt.Errorf("empty context menu PIDL selection")
	}

	var menu *shellContextMenu
	hr, _, _ := syscall.SyscallN(
		f.Vtbl.GetUIObjectOf,
		uintptr(unsafe.Pointer(f)),
		uintptr(hwnd),
		uintptr(len(pidls)),
		uintptr(unsafe.Pointer(&pidls[0])),
		uintptr(unsafe.Pointer(&iidIContextMenu)),
		0,
		uintptr(unsafe.Pointer(&menu)),
	)
	if shellHRESULTFailed(hr) {
		return nil, shellHRESULTError("IShellFolder.GetUIObjectOf(IContextMenu)", hr)
	}
	if menu == nil {
		return nil, fmt.Errorf("IShellFolder.GetUIObjectOf returned nil IContextMenu")
	}
	return menu, nil
}

func (s *shellContextMenuSelection) release() {
	if s == nil {
		return
	}
	if s.Parent != nil {
		s.Parent.release()
		s.Parent = nil
	}
	for _, pidl := range s.AbsolutePIDLs {
		if pidl != nil {
			windows.CoTaskMemFree(pidl)
		}
	}
	s.AbsolutePIDLs = nil
	s.ChildPIDLs = nil
}

func (m *shellContextMenu) release() {
	if m != nil {
		syscall.SyscallN(m.Vtbl.Release, uintptr(unsafe.Pointer(m)))
	}
}

func (m *shellContextMenu) queryContextMenu(hmenu windows.Handle) error {
	hr, _, _ := syscall.SyscallN(
		m.Vtbl.QueryContextMenu,
		uintptr(unsafe.Pointer(m)),
		uintptr(hmenu),
		0,
		shellContextMenuCommandFirst,
		shellContextMenuCommandLast,
		cmfNormal|cmfExplore,
	)
	if shellHRESULTFailed(hr) {
		return shellHRESULTError("IContextMenu.QueryContextMenu", hr)
	}
	return nil
}

func (m *shellContextMenu) invokeCommand(hwnd windows.Handle, command uintptr, point shellContextMenuPoint) error {
	info := shellContextMenuInvokeInfoEx{
		CbSize:   uint32(unsafe.Sizeof(shellContextMenuInvokeInfoEx{})),
		FMask:    cmicMaskUnicode | cmicMaskPtInvoke,
		Hwnd:     hwnd,
		LpVerb:   command,
		NShow:    shellShowNormal,
		LpVerbW:  command,
		PtInvoke: point,
	}
	hr, _, _ := syscall.SyscallN(
		m.Vtbl.InvokeCommand,
		uintptr(unsafe.Pointer(m)),
		uintptr(unsafe.Pointer(&info)),
	)
	if shellHRESULTFailed(hr) {
		return shellHRESULTError("IContextMenu.InvokeCommand", hr)
	}
	return nil
}

func (m *shellContextMenu) queryContextMenu2() *shellContextMenu2 {
	ptr := shellQueryInterface(unsafe.Pointer(m), &iidIContextMenu2)
	return (*shellContextMenu2)(ptr)
}

func (m *shellContextMenu) queryContextMenu3() *shellContextMenu3 {
	ptr := shellQueryInterface(unsafe.Pointer(m), &iidIContextMenu3)
	return (*shellContextMenu3)(ptr)
}

func (m *shellContextMenu2) release() {
	if m != nil {
		syscall.SyscallN(m.Vtbl.Release, uintptr(unsafe.Pointer(m)))
	}
}

func (m *shellContextMenu2) handleMenuMsg(msg uint32, wParam uintptr, lParam uintptr) bool {
	hr, _, _ := syscall.SyscallN(
		m.Vtbl.HandleMenuMsg,
		uintptr(unsafe.Pointer(m)),
		uintptr(msg),
		wParam,
		lParam,
	)
	return !shellHRESULTFailed(hr)
}

func (m *shellContextMenu3) release() {
	if m != nil {
		syscall.SyscallN(m.Vtbl.Release, uintptr(unsafe.Pointer(m)))
	}
}

func (m *shellContextMenu3) handleMenuMsg2(msg uint32, wParam uintptr, lParam uintptr) (uintptr, bool) {
	var result uintptr
	hr, _, _ := syscall.SyscallN(
		m.Vtbl.HandleMenuMsg2,
		uintptr(unsafe.Pointer(m)),
		uintptr(msg),
		wParam,
		lParam,
		uintptr(unsafe.Pointer(&result)),
	)
	return result, !shellHRESULTFailed(hr)
}

func shellQueryInterface(object unsafe.Pointer, iid *windows.GUID) unsafe.Pointer {
	if object == nil {
		return nil
	}
	vtbl := *(**shellContextMenuVtbl)(object)
	var out unsafe.Pointer
	hr, _, _ := syscall.SyscallN(
		vtbl.QueryInterface,
		uintptr(object),
		uintptr(unsafe.Pointer(iid)),
		uintptr(unsafe.Pointer(&out)),
	)
	if shellHRESULTFailed(hr) {
		return nil
	}
	return out
}

func createShellPopupMenu() (windows.Handle, error) {
	hmenu, _, callErr := procCreatePopupMenu.Call()
	if hmenu == 0 {
		return 0, fmt.Errorf("create Windows shell popup menu: %w", callErr)
	}
	return windows.Handle(hmenu), nil
}

func currentCursorPoint() (shellContextMenuPoint, error) {
	var point shellContextMenuPoint
	result, _, callErr := procGetCursorPos.Call(uintptr(unsafe.Pointer(&point)))
	if result == 0 {
		return shellContextMenuPoint{}, fmt.Errorf("get Windows cursor position: %w", callErr)
	}
	return point, nil
}

func trackShellPopupMenu(hmenu windows.Handle, hwnd windows.Handle, point shellContextMenuPoint) (uintptr, error) {
	command, _, callErr := procTrackPopupMenuEx.Call(
		uintptr(hmenu),
		tpmReturnCmd|tpmRightButton,
		uintptr(point.X),
		uintptr(point.Y),
		uintptr(hwnd),
		0,
	)
	if command == 0 {
		if errno, ok := callErr.(syscall.Errno); ok && errno != 0 {
			return 0, fmt.Errorf("track Windows shell popup menu: %w", callErr)
		}
		return 0, nil
	}
	return command, nil
}

func shellHRESULTFailed(hr uintptr) bool {
	return hr&0x80000000 != 0
}

func shellHRESULTError(operation string, hr uintptr) error {
	return fmt.Errorf("%s failed with HRESULT 0x%08X", operation, uint32(hr))
}
