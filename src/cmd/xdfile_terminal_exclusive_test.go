package cmd

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	vt "github.com/charmbracelet/x/vt"
)

func TestXdfileResolveExclusiveTUICommandRecognizesDirectExecutable(t *testing.T) {
	dir := t.TempDir()
	vimPath := filepath.Join(dir, "vim.exe")
	if err := os.WriteFile(vimPath, []byte("fake"), 0o600); err != nil {
		t.Fatalf("write fake vim executable: %v", err)
	}

	candidate, ok := xdfileResolveExclusiveTUICommand(dir, `vim.exe notes.txt`)
	if !ok {
		t.Fatal("expected vim.exe command to be recognized as an exclusive TUI candidate")
	}
	if !strings.EqualFold(candidate.Path, vimPath) {
		t.Fatalf("expected resolved vim path %q, got %q", vimPath, candidate.Path)
	}
	wantArgs := []string{"-c", "silent! if exists('&mouse') | set mouse=a | endif", "notes.txt"}
	if strings.Join(candidate.Args, "\x00") != strings.Join(wantArgs, "\x00") {
		t.Fatalf("expected vim arguments %+v, got %+v", wantArgs, candidate.Args)
	}
	if candidate.MouseInput != xdfilePTYMouseInputNative {
		t.Fatalf("expected vim.exe to use native Windows mouse input, got %v", candidate.MouseInput)
	}
}

func TestXdfileResolveExclusiveTUICommandKeepsExplicitVimTerminalMode(t *testing.T) {
	dir := t.TempDir()
	vimPath := filepath.Join(dir, "vim.exe")
	if err := os.WriteFile(vimPath, []byte("fake"), 0o600); err != nil {
		t.Fatalf("write fake vim executable: %v", err)
	}

	candidate, ok := xdfileResolveExclusiveTUICommand(dir, `vim.exe -T win32 notes.txt`)
	if !ok {
		t.Fatal("expected vim.exe command to be recognized as an exclusive TUI candidate")
	}
	if !strings.EqualFold(candidate.Path, vimPath) {
		t.Fatalf("expected resolved vim path %q, got %q", vimPath, candidate.Path)
	}
	wantArgs := []string{"-c", "silent! if exists('&mouse') | set mouse=a | endif", "-T", "win32", "notes.txt"}
	if strings.Join(candidate.Args, "\x00") != strings.Join(wantArgs, "\x00") {
		t.Fatalf("expected explicit vim terminal mode to be preserved with mouse setup, got %+v", candidate.Args)
	}
}

func TestXdfileResolveExclusiveTUICommandDoesNotForceUnknownVimTerminal(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "vim.exe"), []byte("fake"), 0o600); err != nil {
		t.Fatalf("write fake vim executable: %v", err)
	}

	candidate, ok := xdfileResolveExclusiveTUICommand(dir, `vim.exe notes.txt`)
	if !ok {
		t.Fatal("expected vim.exe command to be recognized as an exclusive TUI candidate")
	}
	for _, arg := range candidate.Args {
		if arg == "xterm-256color" {
			t.Fatalf("expected vim startup not to force xterm-256color, got %+v", candidate.Args)
		}
	}
}

func TestXdfileResolveExclusiveTUICommandUnwrapsCallCmdWrapper(t *testing.T) {
	dir := t.TempDir()
	vxdbgPath := filepath.Join(dir, "vxdbg.exe")
	if err := os.WriteFile(vxdbgPath, []byte("fake"), 0o600); err != nil {
		t.Fatalf("write fake vxdbg executable: %v", err)
	}

	candidate, ok := xdfileResolveExclusiveTUICommand(dir, `call cmd /c vxdbg.exe -p -eV "sample.bin"`)
	if !ok {
		t.Fatal("expected wrapped vxdbg command to be recognized as an exclusive TUI candidate")
	}
	if !strings.EqualFold(candidate.Path, vxdbgPath) {
		t.Fatalf("expected resolved vxdbg path %q, got %q", vxdbgPath, candidate.Path)
	}
	if got := strings.Join(candidate.Args, " "); got != `-p -eV sample.bin` {
		t.Fatalf("expected wrapped vxdbg arguments to survive cmd unwrapping, got %q", got)
	}
	if candidate.MouseInput != xdfilePTYMouseInputBoth {
		t.Fatalf("expected wrapped exclusive TUI to use dual mouse input, got %v", candidate.MouseInput)
	}
}

func TestSubmitTerminalCommandStartsExclusiveTUI(t *testing.T) {
	dir := t.TempDir()
	vimPath := filepath.Join(dir, "vim.exe")
	if err := os.WriteFile(vimPath, []byte("fake"), 0o600); err != nil {
		t.Fatalf("write fake vim executable: %v", err)
	}

	originalStartExclusive := xdfileStartExclusiveTerminalFunc
	defer func() { xdfileStartExclusiveTerminalFunc = originalStartExclusive }()

	events := make(chan tea.Msg, 1)
	session := &xdfileTerminalPTYSession{
		emulator: vt.NewSafeEmulator(80, 24),
		events:   events,
		mode:     xdfileTerminalPTYModeExclusive,
	}
	started := false
	xdfileStartExclusiveTerminalFunc = func(gotDir string, gotCommand string, width int, height int) tea.Cmd {
		started = true
		if gotDir != dir {
			t.Fatalf("expected exclusive TUI dir %q, got %q", dir, gotDir)
		}
		if gotCommand != `vim.exe notes.txt` {
			t.Fatalf("expected exclusive TUI command to preserve input, got %q", gotCommand)
		}
		if width <= 0 || height <= 0 {
			t.Fatalf("expected exclusive TUI viewport size to be positive, got %dx%d", width, height)
		}
		return func() tea.Msg {
			return xdfileExclusiveTerminalStartMsg{
				Command: gotCommand,
				Dir:     gotDir,
				Session: session,
			}
		}
	}

	m := &xdfileModel{
		width:       120,
		height:      40,
		layoutPrefs: xdfileDefaultLayoutPrefs(),
		terminal: xdfileTerminal{
			Cwd:   dir,
			Input: textinput.New(),
		},
	}
	m.terminal.Input.SetValue(`vim.exe notes.txt`)

	cmd := m.submitTerminalCommand(`vim.exe notes.txt`)
	if cmd == nil {
		t.Fatal("expected exclusive TUI submit to return a start command")
	}
	if !started {
		t.Fatal("expected submitTerminalCommand to start exclusive TUI mode")
	}
	if m.terminal.Busy {
		t.Fatal("expected exclusive TUI launch not to mark the managed terminal busy")
	}
	if got := m.terminal.Input.Value(); got != "" {
		t.Fatalf("expected managed terminal input to clear after launch, got %q", got)
	}

	updated, waitCmd := m.Update(cmd())
	got := updated.(*xdfileModel)
	if !got.exclusiveTerminalActive() {
		t.Fatal("expected exclusive TUI session to become active after the start message")
	}
	if got.terminal.Exclusive.Command != `vim.exe notes.txt` {
		t.Fatalf("expected exclusive command metadata to be stored, got %q", got.terminal.Exclusive.Command)
	}
	if waitCmd == nil {
		t.Fatal("expected exclusive TUI start to wait for PTY events")
	}
}

func TestExclusiveTerminalExitMsgClearsExclusiveState(t *testing.T) {
	left := t.TempDir()
	right := t.TempDir()
	m := &xdfileModel{
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: left},
			{Label: "RIGHT", Cwd: right},
		},
		terminal: xdfileTerminal{
			Input: textinput.New(),
			Exclusive: xdfileExclusiveTerminal{
				Command: `vim.exe notes.txt`,
				Cwd:     left,
				Session: &xdfileTerminalPTYSession{emulator: vt.NewSafeEmulator(80, 24)},
			},
		},
	}

	updated, cmd := m.Update(xdfileExclusiveTerminalExitMsg{})
	got := updated.(*xdfileModel)
	if cmd == nil {
		t.Fatalf("expected exclusive exit handling to restore normal mouse mode")
	}
	if got.exclusiveTerminalActive() {
		t.Fatal("expected exclusive TUI state to clear after exit")
	}
}

func TestExclusiveTerminalPassesQuitChordToProgram(t *testing.T) {
	emulator := vt.NewSafeEmulator(80, 24)
	defer emulator.Close()

	m := &xdfileModel{
		terminal: xdfileTerminal{
			Exclusive: xdfileExclusiveTerminal{
				Command: `vim.exe notes.txt`,
				Session: &xdfileTerminalPTYSession{
					emulator: emulator,
				},
			},
		},
	}

	done := make(chan string, 1)
	go func() {
		buf := make([]byte, 32)
		n, err := emulator.Read(buf)
		if err != nil && err != io.EOF {
			done <- ""
			return
		}
		done <- string(buf[:n])
	}()

	if cmd := m.handleExclusiveTerminalKey(tea.KeyMsg{Type: tea.KeyCtrlQ, Alt: true}); cmd != nil {
		t.Fatalf("expected exclusive key forwarding to complete immediately")
	}
	if !m.exclusiveTerminalActive() {
		t.Fatal("expected host quit chord to remain attached to the exclusive program")
	}

	select {
	case got := <-done:
		if got == "" {
			t.Fatal("expected host quit chord to be forwarded into the TUI input stream")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for forwarded exclusive key input")
	}
}

func TestExclusiveTerminalViewportUsesFullEmbeddedPanelContent(t *testing.T) {
	m := &xdfileModel{
		layout: xdfileLayout{
			exclusiveRect: xdfileRect{x: 5, y: 3, w: 20, h: 8},
		},
	}

	width, height := m.exclusiveTerminalViewportSize()
	if width != 18 || height != 6 {
		t.Fatalf("expected exclusive viewport to use full panel content 18x6, got %dx%d", width, height)
	}
}

func TestExclusiveTerminalMouseForwardsClickToPTY(t *testing.T) {
	backend := &testXdfilePTYBackend{}
	session := &xdfileTerminalPTYSession{
		backend:  backend,
		emulator: vt.NewSafeEmulator(18, 6),
	}
	go session.runInputLoop()
	defer session.Close()

	m := &xdfileModel{
		layout: xdfileLayout{
			exclusiveRect: xdfileRect{x: 5, y: 3, w: 20, h: 8},
		},
		terminal: xdfileTerminal{
			Exclusive: xdfileExclusiveTerminal{
				Session: session,
			},
		},
	}

	updated, cmd := m.Update(tea.MouseMsg{
		X:      6,
		Y:      4,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	})
	if updated != m {
		t.Fatal("expected exclusive mouse message to keep the current model")
	}
	if cmd != nil {
		t.Fatal("expected exclusive mouse message to complete immediately")
	}

	time.Sleep(100 * time.Millisecond)
	if got := backend.writes.String(); got != "\x1b[<0;1;1M" {
		t.Fatalf("expected exclusive mouse input to be forwarded through the PTY, got %q", got)
	}
}

func TestExclusiveTerminalPlainMouseMotionDoesNotWriteToPTY(t *testing.T) {
	backend := &testXdfilePTYBackend{}
	session := &xdfileTerminalPTYSession{
		backend:  backend,
		emulator: vt.NewSafeEmulator(18, 6),
	}
	go session.runInputLoop()
	defer session.Close()

	m := &xdfileModel{
		layout: xdfileLayout{
			exclusiveRect: xdfileRect{x: 5, y: 3, w: 20, h: 8},
		},
		terminal: xdfileTerminal{
			Exclusive: xdfileExclusiveTerminal{
				Session: session,
			},
		},
	}

	_, cmd := m.Update(tea.MouseMsg{
		X:      6,
		Y:      4,
		Action: tea.MouseActionMotion,
		Button: tea.MouseButtonNone,
	})
	if cmd != nil {
		t.Fatal("expected exclusive mouse message to complete immediately")
	}

	time.Sleep(100 * time.Millisecond)
	if got := backend.writes.String(); got != "" {
		t.Fatalf("expected plain mouse motion to stay out of the PTY input stream, got %q", got)
	}
}

func TestExclusiveTerminalViewDoesNotAdvertiseHostExitKey(t *testing.T) {
	emulator := vt.NewSafeEmulator(80, 24)
	defer emulator.Close()

	m := &xdfileModel{
		width:       120,
		height:      40,
		layoutPrefs: xdfileDefaultLayoutPrefs(),
		terminal: xdfileTerminal{
			Exclusive: xdfileExclusiveTerminal{
				Command: `vim.exe notes.txt`,
				Session: &xdfileTerminalPTYSession{
					emulator: emulator,
				},
			},
		},
	}

	rendered := stripXdfileANSI(m.View())
	if strings.Contains(rendered, "Ctrl+Alt+Q") || strings.Contains(rendered, "force quit") {
		t.Fatalf("expected exclusive view not to advertise a host exit key, got %q", rendered)
	}
	if !strings.Contains(rendered, "embedded TUI") {
		t.Fatalf("expected exclusive view to be rendered as an embedded TUI, got %q", rendered)
	}
}
