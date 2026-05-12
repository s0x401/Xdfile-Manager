package cmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	vt "github.com/charmbracelet/x/vt"
)

type testXdfilePTYBackend struct {
	writes bytes.Buffer
}

func (b *testXdfilePTYBackend) Read(_ []byte) (int, error) {
	return 0, io.EOF
}

func (b *testXdfilePTYBackend) Write(p []byte) (int, error) {
	return b.writes.Write(p)
}

func (b *testXdfilePTYBackend) Close() error {
	return nil
}

func (b *testXdfilePTYBackend) Resize(_ int, _ int) error {
	return nil
}

func TestHandleGlobalKeyTabWhileManagedCommandLineFocusedReturnsToPanelFocus(t *testing.T) {
	m := &xdfileModel{
		activePanel:     0,
		terminalFocused: true,
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: `C:\left`},
			{Label: "RIGHT", Cwd: `C:\right`},
		},
		terminal: xdfileTerminal{},
	}

	cmd, handled := m.handleGlobalKey(tea.KeyMsg{Type: tea.KeyTab})
	if !handled {
		t.Fatalf("expected tab to be handled while terminal is focused")
	}
	if cmd != nil {
		if msg := cmd(); msg != nil {
			t.Fatalf("expected focus-switch command to be immediate, got message %T", msg)
		}
	}
	if m.activePanel != 1 {
		t.Fatalf("expected active panel to switch to right, got %d", m.activePanel)
	}
	if m.terminalFocused {
		t.Fatalf("expected non-PTY tab cycle to return to panel focus")
	}
}

func TestSelectPanelSyncsTerminalCwdToActivePanel(t *testing.T) {
	m := &xdfileModel{
		activePanel: 0,
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: `C:\left`},
			{Label: "RIGHT", Cwd: `C:\right`},
		},
		terminal: xdfileTerminal{
			Cwd: `C:\left`,
		},
	}

	cmd := m.selectPanel(1, false)
	if cmd != nil {
		if msg := cmd(); msg != nil {
			t.Fatalf("expected immediate panel selection command, got message %T", msg)
		}
	}
	if m.activePanel != 1 {
		t.Fatalf("expected active panel to switch to right, got %d", m.activePanel)
	}
	if m.terminal.Cwd != `C:\right` {
		t.Fatalf("expected terminal cwd to follow right panel, got %q", m.terminal.Cwd)
	}
	if m.terminalFocused {
		t.Fatalf("expected terminal focus to remain off")
	}
}

func TestSelectPanelWithPTYQueuesTerminalCwdSync(t *testing.T) {
	dir := t.TempDir()
	syncFile := filepath.Join(dir, "pty-sync.txt")
	if err := os.WriteFile(syncFile, nil, 0o600); err != nil {
		t.Fatalf("create PTY sync file: %v", err)
	}

	backend := &testXdfilePTYBackend{}
	session := &xdfileTerminalPTYSession{
		backend:  backend,
		emulator: vt.NewSafeEmulator(20, 5),
		syncFile: syncFile,
	}
	go session.runInputLoop()
	defer session.Close()

	m := &xdfileModel{
		activePanel:     0,
		terminalFocused: true,
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: `C:\left`},
			{Label: "RIGHT", Cwd: `C:\right`},
		},
		terminal: xdfileTerminal{
			Cwd:      `C:\left`,
			Session:  session,
			Emulator: session.emulator,
		},
	}

	cmd := m.selectPanel(1, true)
	if cmd != nil {
		if msg := cmd(); msg != nil {
			t.Fatalf("expected PTY panel switch to complete immediately, got message %T", msg)
		}
	}
	if m.terminal.Cwd != `C:\right` {
		t.Fatalf("expected terminal cwd to follow right panel, got %q", m.terminal.Cwd)
	}
	if m.terminal.PendingCwd != `C:\right` {
		t.Fatalf("expected pending PTY cwd sync for right panel, got %q", m.terminal.PendingCwd)
	}
	if !m.terminalFocused {
		t.Fatalf("expected terminal focus to remain on")
	}

	data, err := os.ReadFile(syncFile)
	if err != nil {
		t.Fatalf("read PTY sync file: %v", err)
	}
	if string(data) != `C:\right` {
		t.Fatalf("expected PTY sync file to contain right panel path, got %q", string(data))
	}
}

func TestSelectPanelWithCmdPTYWritesCDCommand(t *testing.T) {
	backend := &testXdfilePTYBackend{}
	session := &xdfileTerminalPTYSession{
		backend:  backend,
		emulator: vt.NewSafeEmulator(20, 5),
		shell:    xdfileTerminalShellCmd,
	}
	go session.runInputLoop()
	defer session.Close()

	m := &xdfileModel{
		activePanel:     0,
		terminalFocused: true,
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: `C:\left`},
			{Label: "RIGHT", Cwd: `D:\right dir`},
		},
		terminal: xdfileTerminal{
			Cwd:      `C:\left`,
			Session:  session,
			Emulator: session.emulator,
		},
	}

	cmd := m.selectPanel(1, true)
	if cmd != nil {
		if msg := cmd(); msg != nil {
			t.Fatalf("expected PTY panel switch to complete immediately, got message %T", msg)
		}
	}
	time.Sleep(50 * time.Millisecond)
	if got := backend.writes.String(); got != "cd /d \"D:\\right dir\"\r" {
		t.Fatalf("expected cmd PTY sync command, got %q", got)
	}
}

func TestXdfileTerminalCwdMsgSkipsPanelRewriteWhilePanelSyncPending(t *testing.T) {
	m := &xdfileModel{
		activePanel: 0,
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: `C:\left`},
			{Label: "RIGHT", Cwd: `C:\right`},
		},
		terminal: xdfileTerminal{
			Cwd:        `C:\left`,
			PendingCwd: `C:\right`,
		},
	}

	updated, cmd := m.Update(xdfileTerminalCwdMsg{Cwd: `C:\right`})
	got := updated.(*xdfileModel)
	if cmd != nil {
		if msg := cmd(); msg != nil {
			t.Fatalf("expected no async terminal message while events are nil, got %T", msg)
		}
	}
	if got.terminal.Cwd != `C:\right` {
		t.Fatalf("expected terminal cwd to update to pending path, got %q", got.terminal.Cwd)
	}
	if got.terminal.PendingCwd != "" {
		t.Fatalf("expected pending cwd sync to clear after matching update, got %q", got.terminal.PendingCwd)
	}
	if got.panels[0].Cwd != `C:\left` {
		t.Fatalf("expected left panel cwd to stay unchanged, got %q", got.panels[0].Cwd)
	}
	if got.panels[1].Cwd != `C:\right` {
		t.Fatalf("expected right panel cwd to stay unchanged, got %q", got.panels[1].Cwd)
	}
}

func TestHandleGlobalKeyF10OpensQuitConfirm(t *testing.T) {
	m := &xdfileModel{}

	cmd, handled := m.handleGlobalKey(tea.KeyMsg{Type: tea.KeyF10})
	if !handled {
		t.Fatalf("expected F10 to be handled")
	}
	if cmd != nil {
		t.Fatalf("expected F10 quit confirmation to open immediately without async command")
	}
	if m.modal.Kind != xdfileModalConfirm {
		t.Fatalf("expected F10 to open a confirm modal, got kind %v", m.modal.Kind)
	}
	if m.modal.Action != xdfileActionQuit {
		t.Fatalf("expected F10 modal action to be quit, got %q", m.modal.Action)
	}
}

func TestApplyModalQuitReturnsTeaQuit(t *testing.T) {
	m := &xdfileModel{
		modal: xdfileModal{
			Kind:   xdfileModalConfirm,
			Action: xdfileActionQuit,
		},
	}

	cmd := m.applyModal()
	if cmd == nil {
		t.Fatalf("expected quit confirmation to return a quit command")
	}
	if msg := cmd(); msg != tea.Quit() {
		t.Fatalf("expected quit confirmation to trigger tea.Quit, got %#v", msg)
	}
}

func TestQuitConfirmSupportsKeyboardChoiceCursor(t *testing.T) {
	m := &xdfileModel{}
	if cmd := m.openQuitConfirm(); cmd != nil {
		t.Fatalf("expected quit confirm to be immediate")
	}
	if m.modal.Kind != xdfileModalConfirm {
		t.Fatalf("expected confirm modal, got %v", m.modal.Kind)
	}
	if m.modal.ChoiceCursor != 0 {
		t.Fatalf("expected confirm choice to start on Confirm, got %d", m.modal.ChoiceCursor)
	}

	if cmd := m.handleModalKey(tea.KeyMsg{Type: tea.KeyDown}); cmd != nil {
		t.Fatalf("expected choice movement not to return command")
	}
	if m.modal.ChoiceCursor != 1 {
		t.Fatalf("expected Down to move cursor to Cancel, got %d", m.modal.ChoiceCursor)
	}
	if cmd := m.handleModalKey(tea.KeyMsg{Type: tea.KeyEnter}); cmd != nil {
		t.Fatalf("expected Enter on Cancel to close without quitting")
	}
	if m.modal.Kind != xdfileModalNone {
		t.Fatalf("expected Enter on Cancel to close modal, got %v", m.modal.Kind)
	}
}
