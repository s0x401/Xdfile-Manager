package cmd

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	uv "github.com/charmbracelet/ultraviolet"
	vt "github.com/charmbracelet/x/vt"
)

func TestXdfileTerminalKeyEventArrowKey(t *testing.T) {
	event, ok := xdfileTerminalKeyEvent(tea.KeyMsg{Type: tea.KeyUp})
	if !ok {
		t.Fatalf("expected arrow key to map into a terminal key event")
	}

	key, ok := event.(uv.KeyPressEvent)
	if !ok {
		t.Fatalf("expected uv.KeyPressEvent, got %T", event)
	}
	if key.Code != uv.KeyUp {
		t.Fatalf("expected key code %v, got %v", uv.KeyUp, key.Code)
	}
}

func TestXdfileTerminalKeyEventCtrlRune(t *testing.T) {
	event, ok := xdfileTerminalKeyEvent(tea.KeyMsg{Type: tea.KeyCtrlC})
	if !ok {
		t.Fatalf("expected ctrl key to map into a terminal key event")
	}

	key, ok := event.(uv.KeyPressEvent)
	if !ok {
		t.Fatalf("expected uv.KeyPressEvent, got %T", event)
	}
	if key.Code != 'c' {
		t.Fatalf("expected ctrl+c to preserve rune c, got %q", key.Code)
	}
	if key.Mod != uv.ModCtrl {
		t.Fatalf("expected ctrl modifier, got %v", key.Mod)
	}
}

func TestHandleMouseClickForwardsToPTYAltScreen(t *testing.T) {
	backend := &testXdfilePTYBackend{}
	session := &xdfileTerminalPTYSession{
		backend:  backend,
		emulator: vt.NewSafeEmulator(98, 7),
	}
	go session.runInputLoop()
	defer session.Close()
	if _, err := session.emulator.Write([]byte("\x1b[?1049h")); err != nil {
		t.Fatalf("enable terminal alt screen: %v", err)
	}

	m := &xdfileModel{
		layout: xdfileLayout{
			terminalRect: xdfileRect{x: 0, y: 2, w: 100, h: 10},
		},
		terminal: xdfileTerminal{
			Session:    session,
			Emulator:   session.emulator,
			ViewWidth:  98,
			ViewHeight: 7,
		},
	}

	if cmd := m.handleMouse(tea.MouseMsg{
		X:      1,
		Y:      4,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}); cmd != nil {
		if msg := cmd(); msg != nil {
			t.Fatalf("expected terminal mouse click to complete immediately, got %T", msg)
		}
	}

	time.Sleep(100 * time.Millisecond)
	if got := backend.writes.String(); got != "\x1b[<0;1;1M" {
		t.Fatalf("expected terminal click to be forwarded through the PTY, got %q", got)
	}
}

func TestHandleMouseClickDoesNotWriteToPlainPTYShell(t *testing.T) {
	backend := &testXdfilePTYBackend{}
	session := &xdfileTerminalPTYSession{
		backend:  backend,
		emulator: vt.NewSafeEmulator(98, 7),
	}
	go session.runInputLoop()
	defer session.Close()

	m := &xdfileModel{
		layout: xdfileLayout{
			terminalRect: xdfileRect{x: 0, y: 2, w: 100, h: 10},
		},
		terminal: xdfileTerminal{
			Session:    session,
			Emulator:   session.emulator,
			ViewWidth:  98,
			ViewHeight: 7,
		},
	}

	if cmd := m.handleMouse(tea.MouseMsg{
		X:      1,
		Y:      4,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}); cmd != nil {
		if msg := cmd(); msg != nil {
			t.Fatalf("expected terminal mouse click to complete immediately, got %T", msg)
		}
	}

	time.Sleep(100 * time.Millisecond)
	if got := backend.writes.String(); got != "" {
		t.Fatalf("expected plain PTY shell click not to write mouse input, got %q", got)
	}
}

func TestXdfileTerminalMouseShouldForwardDropsPlainHover(t *testing.T) {
	if xdfileTerminalMouseShouldForward(tea.MouseMsg{
		Action: tea.MouseActionMotion,
		Button: tea.MouseButtonNone,
	}) {
		t.Fatal("expected plain hover motion to stay out of PTY input")
	}
	if !xdfileTerminalMouseShouldForward(tea.MouseMsg{
		Action: tea.MouseActionMotion,
		Button: tea.MouseButtonLeft,
	}) {
		t.Fatal("expected drag motion to be forwarded")
	}
}

func TestXdfileSendPTYSessionMouseDualModeAlsoWritesSGR(t *testing.T) {
	backend := &testXdfilePTYBackend{}
	session := &xdfileTerminalPTYSession{
		backend:  backend,
		emulator: vt.NewSafeEmulator(18, 6),
		mouseIn:  xdfilePTYMouseInputBoth,
	}
	go session.runInputLoop()
	defer session.Close()

	if !xdfileSendPTYSessionMouse(session, session.emulator, tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}, 0, 0) {
		t.Fatal("expected dual mouse input to write at least one mouse event")
	}

	time.Sleep(100 * time.Millisecond)
	if got := backend.writes.String(); got != "\x1b[<0;1;1M" {
		t.Fatalf("expected dual mouse input to preserve SGR fallback, got %q", got)
	}
}

func TestXdfileNewTerminalPTYSessionStartsNativeMouseProxyLazily(t *testing.T) {
	session := xdfileNewTerminalPTYSession(
		&testXdfilePTYBackend{},
		nil,
		xdfileTerminalShellUnknown,
		make(chan tea.Msg, 1),
		18,
		6,
		xdfileTerminalPTYModeExclusive,
		xdfilePTYMouseInputNative,
	)
	defer session.Close()

	if session.mouse != nil {
		t.Fatal("expected native mouse proxy not to start until the first mouse event")
	}
}

func TestHandleMouseClickForwardsToRunningPTYWithoutAltScreen(t *testing.T) {
	backend := &testXdfilePTYBackend{}
	session := &xdfileTerminalPTYSession{
		backend:  backend,
		emulator: vt.NewSafeEmulator(98, 7),
	}
	go session.runInputLoop()
	defer session.Close()

	m := &xdfileModel{
		layout: xdfileLayout{
			terminalRect: xdfileRect{x: 0, y: 2, w: 100, h: 10},
		},
		terminal: xdfileTerminal{
			Busy:       true,
			Session:    session,
			Emulator:   session.emulator,
			ViewWidth:  98,
			ViewHeight: 7,
		},
	}

	if cmd := m.handleMouse(tea.MouseMsg{
		X:      1,
		Y:      4,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}); cmd != nil {
		if msg := cmd(); msg != nil {
			t.Fatalf("expected running terminal mouse click to complete immediately, got %T", msg)
		}
	}

	time.Sleep(100 * time.Millisecond)
	if got := backend.writes.String(); got != "\x1b[<0;1;1M" {
		t.Fatalf("expected running terminal click to be forwarded through the PTY, got %q", got)
	}
}

func TestHandleMouseWheelForwardsToPTYAltScreen(t *testing.T) {
	backend := &testXdfilePTYBackend{}
	session := &xdfileTerminalPTYSession{
		backend:  backend,
		emulator: vt.NewSafeEmulator(98, 7),
	}
	go session.runInputLoop()
	defer session.Close()
	if _, err := session.emulator.Write([]byte("\x1b[?1049h")); err != nil {
		t.Fatalf("enable terminal alt screen: %v", err)
	}

	m := &xdfileModel{
		layout: xdfileLayout{
			terminalRect: xdfileRect{x: 0, y: 2, w: 100, h: 10},
		},
		terminal: xdfileTerminal{
			Session:    session,
			Emulator:   session.emulator,
			ViewWidth:  98,
			ViewHeight: 7,
		},
	}

	if cmd := m.handleMouse(tea.MouseMsg{
		X:      1,
		Y:      4,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonWheelDown,
	}); cmd != nil {
		t.Fatalf("expected terminal mouse wheel to complete immediately")
	}

	time.Sleep(100 * time.Millisecond)
	if got := backend.writes.String(); got != "\x1b[<65;1;1M" {
		t.Fatalf("expected terminal wheel to be forwarded through the PTY, got %q", got)
	}
}

func TestHandleMouseClickSelectsManagedTerminalSuggestion(t *testing.T) {
	input := xdfileNewManagedTerminalInput()
	input.SetValue("git s")
	input.CursorEnd()
	m := &xdfileModel{
		layout: xdfileLayout{
			terminalRect: xdfileRect{x: 0, y: 2, w: 80, h: 10},
		},
		terminal: xdfileTerminal{
			Cwd:              `C:\work`,
			Input:            input,
			Suggestions:      []string{"git status", "git stash"},
			SuggestionCursor: 0,
		},
	}

	_ = m.renderTerminal()
	if len(m.layout.terminalSuggestionRects) < 2 {
		t.Fatalf("expected suggestion hit rects to be populated, got %+v", m.layout.terminalSuggestionRects)
	}
	target := m.layout.terminalSuggestionRects[1].Rect
	if cmd := m.handleMouse(tea.MouseMsg{
		X:      target.x + 1,
		Y:      target.y,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}); cmd != nil {
		t.Fatalf("expected suggestion click to complete immediately")
	}

	if got := m.terminal.Input.Value(); got != "git status" {
		t.Fatalf("expected click to select git status suggestion, got %q", got)
	}
	if got := m.terminal.Input.Position(); got != len("git status") {
		t.Fatalf("expected cursor at end of selected suggestion, got %d", got)
	}
}

func TestHandleMouseClickMovesManagedTerminalCursor(t *testing.T) {
	input := xdfileNewManagedTerminalInput()
	input.SetValue("abcdef")
	input.CursorEnd()
	m := &xdfileModel{
		layout: xdfileLayout{
			terminalRect: xdfileRect{x: 0, y: 2, w: 80, h: 10},
		},
		terminal: xdfileTerminal{
			Cwd:   `C:\work`,
			Input: input,
		},
	}

	_ = m.renderTerminal()
	target := m.layout.terminalInputRect
	if target.w == 0 {
		t.Fatalf("expected terminal input hit rect to be populated")
	}
	x := target.x + lipgloss.Width(m.terminal.Input.Prompt) + 3
	if cmd := m.handleMouse(tea.MouseMsg{
		X:      x,
		Y:      target.y,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}); cmd != nil {
		t.Fatalf("expected input click to complete immediately")
	}

	if got := m.terminal.Input.Position(); got != 3 {
		t.Fatalf("expected cursor to move to position 3, got %d", got)
	}
}

func TestManagedTerminalPromptColumnStaysStableWithSuggestions(t *testing.T) {
	input := xdfileNewManagedTerminalInput()
	input.SetValue("git s")
	input.CursorEnd()

	base := &xdfileModel{
		layout: xdfileLayout{
			terminalRect: xdfileRect{x: 0, y: 2, w: 80, h: 10},
		},
		terminal: xdfileTerminal{
			Cwd:   `C:\work`,
			Input: input,
		},
	}
	withoutPopup := stripXdfileANSI(base.renderTerminal())
	withoutRow, withoutColumn, ok := xdfileRenderedPromptPosition(withoutPopup)
	if !ok {
		t.Fatalf("expected terminal prompt in base render, got %q", withoutPopup)
	}

	withPopup := &xdfileModel{
		layout: xdfileLayout{
			terminalRect: xdfileRect{x: 0, y: 2, w: 80, h: 10},
		},
		terminal: xdfileTerminal{
			Cwd:              `C:\work`,
			Input:            input,
			Suggestions:      []string{"git status", "git stash"},
			SuggestionCursor: 0,
		},
	}
	withPopupRendered := stripXdfileANSI(withPopup.renderTerminal())
	withRow, withColumn, ok := xdfileRenderedPromptPosition(withPopupRendered)
	if !ok {
		t.Fatalf("expected terminal prompt in popup render, got %q", withPopupRendered)
	}

	if withRow != withoutRow {
		t.Fatalf("expected XD> row to stay %d with suggestions, got %d\nwithout:\n%s\nwith:\n%s", withoutRow, withRow, withoutPopup, withPopupRendered)
	}
	if withColumn != withoutColumn {
		t.Fatalf("expected XD> column to stay %d with suggestions, got %d\nwithout:\n%s\nwith:\n%s", withoutColumn, withColumn, withoutPopup, withPopupRendered)
	}
	if withPopup.layout.terminalInputRect.x != base.layout.terminalInputRect.x {
		t.Fatalf("expected input hit rect x to stay %d with suggestions, got %d", base.layout.terminalInputRect.x, withPopup.layout.terminalInputRect.x)
	}
	if withPopup.layout.terminalInputRect.y != base.layout.terminalInputRect.y {
		t.Fatalf("expected input hit rect y to stay %d with suggestions, got %d", base.layout.terminalInputRect.y, withPopup.layout.terminalInputRect.y)
	}
}

func xdfileRenderedPromptPosition(rendered string) (int, int, bool) {
	for row, line := range strings.Split(rendered, "\n") {
		if column := strings.Index(line, "XD>"); column >= 0 {
			return row, column, true
		}
	}
	return 0, 0, false
}

func TestHandleMouseClickMovesManagedTerminalCursorInScrolledInput(t *testing.T) {
	input := xdfileNewManagedTerminalInput()
	input.Width = 6
	input.SetValue("abcdefghijklmnop")
	input.CursorEnd()
	m := &xdfileModel{
		layout: xdfileLayout{
			terminalRect: xdfileRect{x: 0, y: 2, w: 80, h: 10},
		},
		terminal: xdfileTerminal{
			Cwd:   `C:\work`,
			Input: input,
		},
	}

	_ = m.renderTerminal()
	target := m.layout.terminalInputRect
	x := target.x + lipgloss.Width(m.terminal.Input.Prompt)
	if cmd := m.handleMouse(tea.MouseMsg{
		X:      x,
		Y:      target.y,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}); cmd != nil {
		t.Fatalf("expected input click to complete immediately")
	}

	if got := m.terminal.Input.Position(); got != 10 {
		t.Fatalf("expected click at first visible cell to move to scrolled position 10, got %d", got)
	}
}

func TestHandleMouseClickMovesManagedTerminalCursorAcrossWideCharacters(t *testing.T) {
	input := xdfileNewManagedTerminalInput()
	input.SetValue("ab你好cd")
	input.CursorEnd()
	m := &xdfileModel{
		layout: xdfileLayout{
			terminalRect: xdfileRect{x: 0, y: 2, w: 80, h: 10},
		},
		terminal: xdfileTerminal{
			Cwd:   `C:\work`,
			Input: input,
		},
	}

	_ = m.renderTerminal()
	target := m.layout.terminalInputRect
	x := target.x + lipgloss.Width(m.terminal.Input.Prompt) + 3
	if cmd := m.handleMouse(tea.MouseMsg{
		X:      x,
		Y:      target.y,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}); cmd != nil {
		t.Fatalf("expected input click to complete immediately")
	}

	if got := m.terminal.Input.Position(); got != 3 {
		t.Fatalf("expected click on the second cell of a wide rune to move after it, got %d", got)
	}
}

func TestXdfileBestTerminalSuggestionPrefersRecentHistory(t *testing.T) {
	history := []string{"git status", "go test ./src/cmd", "git commit"}
	got := xdfileBestTerminalSuggestion("git st", history)
	if got != "git status" {
		t.Fatalf("expected recent history suggestion %q, got %q", "git status", got)
	}
}

func TestXdfileBestTerminalSuggestionFallsBackToDefaults(t *testing.T) {
	got := xdfileBestTerminalSuggestion("cle", nil)
	if got != "clear" {
		t.Fatalf("expected default suggestion %q, got %q", "clear", got)
	}
	if suffix := xdfileSuggestionSuffix(got, "cle"); suffix != "ar" {
		t.Fatalf("expected suffix %q, got %q", "ar", suffix)
	}
}

func TestXdfileBestTerminalSuggestionWaitsBeforeSuggestingMultiwordHistory(t *testing.T) {
	history := []string{"cd C:\\work\\demo"}
	if got := xdfileBestTerminalSuggestion("c", history); got != "" {
		t.Fatalf("expected no suggestion for single-letter prefix, got %q", got)
	}
	if got := xdfileBestTerminalSuggestion("cd", history); got != "" {
		t.Fatalf("expected no multiword suggestion before a space, got %q", got)
	}
	if got := xdfileBestTerminalSuggestion("cd ", history); got != "cd C:\\work\\demo" {
		t.Fatalf("expected suggestion after typing command and space, got %q", got)
	}
}

func TestXdfileManagedShellSuggestionsIncludeAliases(t *testing.T) {
	suggestions := xdfileManagedShellSuggestions("l", "", nil)
	joined := strings.Join(suggestions, "\n")
	for _, expected := range []string{"ls", "ll", "la"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("expected suggestions to include %q, got %q", expected, joined)
		}
	}
}

func TestXdfileManagedShellSuggestionsCompletePaths(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "demo.txt"), []byte("demo"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	suggestions := xdfileManagedShellSuggestions("cat de", dir, nil)
	joined := strings.Join(suggestions, "\n")
	if !strings.Contains(joined, "cat demo.txt") {
		t.Fatalf("expected path completion suggestion, got %q", joined)
	}
}

func TestXdfileParseCMDPromptLinePreservesRedirectCommand(t *testing.T) {
	cwd, input, ok := xdfileParseCMDPromptLine(`C:\work> echo a > b.txt`)
	if !ok {
		t.Fatal("expected cmd prompt line to parse")
	}
	if cwd != `C:\work` {
		t.Fatalf("expected cmd prompt cwd %q, got %q", `C:\work`, cwd)
	}
	if input != `echo a > b.txt` {
		t.Fatalf("expected full command after prompt, got %q", input)
	}
}

func TestXdfileParsePowerShellPromptLinePreservesRedirectCommand(t *testing.T) {
	cwd, input, ok := xdfileParsePowerShellPromptLine(`PS C:\work> echo a > b.txt`)
	if !ok {
		t.Fatal("expected PowerShell prompt line to parse")
	}
	if cwd != `C:\work` {
		t.Fatalf("expected PowerShell prompt cwd %q, got %q", `C:\work`, cwd)
	}
	if input != `echo a > b.txt` {
		t.Fatalf("expected full command after prompt, got %q", input)
	}
}

func TestHandlePTYTerminalKeyRightAcceptsInlineSuggestion(t *testing.T) {
	emulator := vt.NewSafeEmulator(80, 24)
	if _, err := emulator.Write([]byte("PS C:\\work> git st")); err != nil {
		t.Fatalf("seed emulator prompt: %v", err)
	}

	m := &xdfileModel{
		terminal: xdfileTerminal{
			Session:  &xdfileTerminalPTYSession{},
			Emulator: emulator,
			History:  []string{"git status"},
		},
	}

	type readResult struct {
		text string
		err  error
	}
	done := make(chan readResult, 1)
	go func() {
		buf := make([]byte, 32)
		n, err := emulator.Read(buf)
		if err != nil && err != io.EOF {
			done <- readResult{err: err}
			return
		}
		done <- readResult{text: string(buf[:n])}
	}()

	_ = m.handlePTYTerminalKey(tea.KeyMsg{Type: tea.KeyRight})

	select {
	case result := <-done:
		if result.err != nil {
			t.Fatalf("read emulator input stream: %v", result.err)
		}
		if result.text != "atus" {
			t.Fatalf("expected right arrow to accept suggestion suffix %q, got %q", "atus", result.text)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for accepted suggestion text")
	}
}

func TestApplyTerminalCwdUpdatesActivePanelForCmd(t *testing.T) {
	dir := t.TempDir()

	m := &xdfileModel{
		activePanel: 0,
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: `C:\work`},
			{Label: "RIGHT", Cwd: `C:\right`},
		},
		terminal: xdfileTerminal{
			Session: &xdfileTerminalPTYSession{shell: xdfileTerminalShellCmd},
		},
	}

	m.applyTerminalCwd(dir)

	if got := m.terminal.Cwd; got != dir {
		t.Fatalf("expected terminal cwd to follow cmd prompt, got %q", got)
	}
	if got := m.panels[0].Cwd; got != dir {
		t.Fatalf("expected active panel cwd to follow cmd prompt, got %q", got)
	}
}

func TestHandleTerminalKeyRightMovesManagedInputCursor(t *testing.T) {
	input := textinput.New()
	input.SetValue("git status")
	input.SetCursor(3)
	input.Focus()

	m := &xdfileModel{
		terminalFocused: true,
		terminal: xdfileTerminal{
			Input: input,
		},
	}
	m.refreshManagedTerminalSuggestions()

	cmd := m.handleTerminalKey(tea.KeyMsg{Type: tea.KeyRight})
	if got := m.terminal.Input.Position(); got != 4 {
		t.Fatalf("expected right arrow to move cursor to %d, got %d", 4, got)
	}
	if cmd == nil {
		t.Fatalf("expected textinput cursor update command to be returned")
	}
}

func TestHandleManagedTerminalBoundKeyRoutesTypingWithoutTakingPanelArrows(t *testing.T) {
	input := xdfileNewManagedTerminalInput()

	m := &xdfileModel{
		activePanel: 0,
		panels: [2]xdfilePanel{
			{
				Label:   "LEFT",
				Cwd:     `C:\work`,
				Entries: []xdfileEntry{{Name: "a.txt"}, {Name: "b.txt"}, {Name: "c.txt"}},
			},
		},
		layout: xdfileLayout{
			panelRects: [2]xdfileRect{{h: 12}},
		},
		terminal: xdfileTerminal{
			Cwd:   `C:\work`,
			Input: input,
		},
	}

	if cmd, handled := m.handleManagedTerminalBoundKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}}); !handled {
		t.Fatalf("expected typed runes to go into the managed command line, handled=%v cmd=%v", handled, cmd)
	}
	if got := m.terminal.Input.Value(); got != "z" {
		t.Fatalf("expected command line to capture typed text, got %q", got)
	}

	if cmd, handled := m.handleManagedTerminalBoundKey(tea.KeyMsg{Type: tea.KeyUp}); handled || cmd != nil {
		t.Fatalf("expected arrow keys to stay on the file panel while the popup is closed, handled=%v cmd=%v", handled, cmd)
	}
}

func TestUpdateManagedTerminalAllowsDigitsReservedForCtrlSortShortcuts(t *testing.T) {
	input := xdfileNewManagedTerminalInput()
	input.Focus()

	m := &xdfileModel{
		terminalFocused: true,
		terminal: xdfileTerminal{
			Cwd:   `C:\work`,
			Input: input,
		},
	}

	gotModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	got, ok := gotModel.(*xdfileModel)
	if !ok {
		t.Fatalf("expected xdfileModel after typing 3, got %T", gotModel)
	}
	if got.terminal.Input.Value() != "3" {
		t.Fatalf("expected managed terminal to accept digit 3, got %q", got.terminal.Input.Value())
	}
	if cmd == nil {
		t.Fatalf("expected typing 3 to return text input update command")
	}

	gotModel, cmd = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'4'}})
	got, ok = gotModel.(*xdfileModel)
	if !ok {
		t.Fatalf("expected xdfileModel after typing 4, got %T", gotModel)
	}
	if got.terminal.Input.Value() != "34" {
		t.Fatalf("expected managed terminal to accept digit 4, got %q", got.terminal.Input.Value())
	}
	if cmd == nil {
		t.Fatalf("expected typing 4 to return text input update command")
	}
}

func TestHandleManagedTerminalBoundKeyPopupUsesArrowsAndAcceptsSuggestion(t *testing.T) {
	input := xdfileNewManagedTerminalInput()
	input.SetValue("git s")
	input.CursorEnd()

	m := &xdfileModel{
		terminal: xdfileTerminal{
			Cwd:              `C:\work`,
			Input:            input,
			Suggestions:      []string{"git status", "git stash"},
			SuggestionCursor: 0,
		},
	}
	if !m.managedTerminalPopupVisible() {
		t.Fatal("expected managed command line popup to be visible")
	}

	if cmd, handled := m.handleManagedTerminalBoundKey(tea.KeyMsg{Type: tea.KeyDown}); !handled || cmd != nil {
		t.Fatalf("expected popup down arrow to be handled, handled=%v cmd=%v", handled, cmd)
	}
	if got := m.selectedManagedTerminalSuggestion(); got != "git status" {
		t.Fatalf("expected down arrow to move popup selection to %q, got %q", "git status", got)
	}
	if got := m.terminal.Input.Value(); got != "git status" {
		t.Fatalf("expected selected suggestion to auto-complete input to %q, got %q", "git status", got)
	}

	if cmd, handled := m.handleManagedTerminalBoundKey(tea.KeyMsg{Type: tea.KeyUp}); !handled || cmd != nil {
		t.Fatalf("expected popup up arrow to be handled, handled=%v cmd=%v", handled, cmd)
	}
	if got := m.selectedManagedTerminalSuggestion(); got != "" {
		t.Fatalf("expected up arrow to return to blank suggestion, got %q", got)
	}
	if got := m.terminal.Input.Value(); got != "git s" {
		t.Fatalf("expected blank suggestion to restore typed input %q, got %q", "git s", got)
	}

	before := m.terminal.Input.Position()
	if cmd, handled := m.handleManagedTerminalBoundKey(tea.KeyMsg{Type: tea.KeyLeft}); !handled {
		t.Fatalf("expected popup left arrow to move the input cursor, handled=%v cmd=%v", handled, cmd)
	}
	if got := m.terminal.Input.Position(); got != before-1 {
		t.Fatalf("expected popup left arrow to move input cursor from %d to %d, got %d", before, before-1, got)
	}
	if !m.managedTerminalPopupVisible() {
		t.Fatal("expected left arrow to keep popup visible")
	}
	if got := m.selectedManagedTerminalSuggestion(); got != "" {
		t.Fatalf("expected left arrow to keep current popup selection, got %q", got)
	}
}

func TestHandleManagedTerminalBoundKeyEnterExecutesSelectedSuggestion(t *testing.T) {
	input := xdfileNewManagedTerminalInput()
	input.SetValue("git s")
	input.CursorEnd()

	m := &xdfileModel{
		terminal: xdfileTerminal{
			Cwd:              t.TempDir(),
			Input:            input,
			Suggestions:      []string{"git status", "git stash"},
			SuggestionCursor: 2,
		},
	}

	cmd, handled := m.handleManagedTerminalBoundKey(tea.KeyMsg{Type: tea.KeyEnter})
	if !handled {
		t.Fatal("expected enter to be handled when a suggestion is selected")
	}
	if cmd == nil {
		t.Fatal("expected enter to submit the selected suggestion")
	}

	startMsg := cmd()
	start, ok := startMsg.(xdfileTerminalCommandStartMsg)
	if !ok {
		t.Fatalf("expected command start message, got %T", startMsg)
	}
	defer start.Cancel()
	if start.Command != "git stash" {
		t.Fatalf("expected enter to execute selected suggestion %q, got %q", "git stash", start.Command)
	}
}

func TestHandleTerminalKeyEnterExecutesSelectedSuggestion(t *testing.T) {
	input := textinput.New()
	input.SetValue("git s")
	input.CursorEnd()
	input.Focus()

	m := &xdfileModel{
		terminalFocused: true,
		terminal: xdfileTerminal{
			Cwd:              t.TempDir(),
			Input:            input,
			Suggestions:      []string{"git status", "git stash"},
			SuggestionCursor: 0,
		},
	}

	cmd := m.handleTerminalKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected enter to submit the typed command")
	}

	startMsg := cmd()
	start, ok := startMsg.(xdfileTerminalCommandStartMsg)
	if !ok {
		t.Fatalf("expected command start message, got %T", startMsg)
	}
	defer start.Cancel()
	if start.Command != "git s" {
		t.Fatalf("expected enter to execute typed command %q, got %q", "git s", start.Command)
	}
}

func TestSelectedManagedTerminalSuggestionDefaultBlankItem(t *testing.T) {
	m := &xdfileModel{
		terminal: xdfileTerminal{
			Suggestions:      []string{"git status", "git stash"},
			SuggestionCursor: 0,
		},
	}

	if got := m.selectedManagedTerminalSuggestion(); got != "" {
		t.Fatalf("expected default popup item to be blank, got %q", got)
	}

	m.moveManagedTerminalSuggestion(1)
	if got := m.selectedManagedTerminalSuggestion(); got != "git status" {
		t.Fatalf("expected first real suggestion after one move, got %q", got)
	}
	if got := m.terminal.Input.Value(); got != "git status" {
		t.Fatalf("expected moving to first real suggestion to auto-complete input, got %q", got)
	}
}

func TestRefreshManagedTerminalSuggestionsStartsAtBlankItem(t *testing.T) {
	input := textinput.New()
	input.SetValue("git s")

	m := &xdfileModel{
		terminal: xdfileTerminal{
			Cwd:   t.TempDir(),
			Input: input,
			History: []string{
				"git status",
				"git stash",
			},
		},
	}

	m.refreshManagedTerminalSuggestions()

	if got := m.terminal.SuggestionCursor; got != 0 {
		t.Fatalf("expected refreshed suggestions to default to blank item, got %d", got)
	}
	if got := m.selectedManagedTerminalSuggestion(); got != "" {
		t.Fatalf("expected refreshed default selection to be blank, got %q", got)
	}
}

func TestHandleManagedTerminalBoundKeyEscRestoresTypedInput(t *testing.T) {
	input := xdfileNewManagedTerminalInput()
	input.SetValue("git s")
	input.CursorEnd()

	m := &xdfileModel{
		terminal: xdfileTerminal{
			Cwd:              `C:\work`,
			Input:            input,
			Suggestions:      []string{"git status", "git stash"},
			SuggestionCursor: 1,
			SuggestionInput:  "git s",
		},
	}

	if cmd, handled := m.handleManagedTerminalBoundKey(tea.KeyMsg{Type: tea.KeyEsc}); !handled || cmd != nil {
		t.Fatalf("expected esc to dismiss popup, handled=%v cmd=%v", handled, cmd)
	}
	if got := m.terminal.Input.Value(); got != "git s" {
		t.Fatalf("expected esc to restore typed input %q, got %q", "git s", got)
	}
	if !m.terminal.SuggestionDismissed {
		t.Fatal("expected esc to dismiss popup suggestions")
	}
}

func TestHandleTerminalKeyEscDismissesPopupAndRestoresTypedInput(t *testing.T) {
	input := textinput.New()
	input.SetValue("git status")
	input.CursorEnd()
	input.Focus()

	m := &xdfileModel{
		terminalFocused: true,
		terminal: xdfileTerminal{
			Cwd:                 `C:\work`,
			Input:               input,
			Suggestions:         []string{"git status", "git stash"},
			SuggestionCursor:    1,
			SuggestionInput:     "git s",
			SuggestionDismissed: false,
		},
	}

	cmd := m.handleTerminalKey(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil {
		t.Fatalf("expected esc dismissal to complete immediately")
	}
	if got := m.terminal.Input.Value(); got != "git s" {
		t.Fatalf("expected esc to restore typed input %q, got %q", "git s", got)
	}
	if !m.terminal.SuggestionDismissed {
		t.Fatal("expected esc to dismiss popup suggestions")
	}
	if m.managedTerminalPopupVisible() {
		t.Fatal("expected popup to close after esc dismissal")
	}
}

func TestSubmitTerminalCommandStreamsExternalOutput(t *testing.T) {
	input := textinput.New()
	leftDir := t.TempDir()
	rightDir := t.TempDir()
	m := &xdfileModel{
		terminalFocused: true,
		activePanel:     1,
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: leftDir},
			{Label: "RIGHT", Cwd: rightDir},
		},
		terminal: xdfileTerminal{
			Cwd:   leftDir,
			Input: input,
		},
	}

	cmd := m.submitTerminalCommand(`cmd /c echo streamed`)
	if cmd == nil {
		t.Fatal("expected external command submission to return a command")
	}
	if !m.terminal.Busy {
		t.Fatal("expected external command submission to mark terminal busy")
	}
	if m.terminalFocused {
		t.Fatal("expected external command submission to restore panel focus while the command runs")
	}

	startMsg := cmd()
	start, ok := startMsg.(xdfileTerminalCommandStartMsg)
	if !ok {
		t.Fatalf("expected command start message, got %T", startMsg)
	}

	updated, waitCmd := m.Update(start)
	got := updated.(*xdfileModel)
	if waitCmd == nil {
		t.Fatal("expected command start update to wait for streaming events")
	}
	if len(got.terminal.Lines) == 0 || !strings.Contains(got.terminal.Lines[len(got.terminal.Lines)-1], `cmd /c echo streamed`) {
		t.Fatalf("expected terminal prompt line to be appended immediately, got %+v", got.terminal.Lines)
	}

	var sawOutput bool
	for i := 0; i < 4; i++ {
		select {
		case msg, ok := <-start.Events:
			if !ok {
				t.Fatal("expected command events before channel close")
			}
			updated, _ = got.Update(msg)
			got = updated.(*xdfileModel)
			switch typed := msg.(type) {
			case xdfileTerminalLineMsg:
				if strings.Contains(typed.Line, "streamed") {
					sawOutput = true
				}
			case xdfileTerminalStreamScreenMsg:
				if got.terminal.StreamEmulator != nil {
					screen := xdfileRenderStreamingTerminalScreen(
						got.terminal.StreamEmulator,
						got.terminal.StreamEmulator.Width(),
						got.terminal.StreamEmulator.Height(),
					)
					if strings.Contains(screen, "streamed") {
						sawOutput = true
					}
				}
			case xdfileTerminalCommandDoneMsg:
				if !sawOutput {
					for _, line := range got.terminal.Lines {
						if strings.Contains(line, "streamed") {
							sawOutput = true
							break
						}
					}
				}
				if !sawOutput {
					t.Fatalf("expected streamed output before done message, got terminal lines %+v", got.terminal.Lines)
				}
				if got.terminal.Busy {
					t.Fatal("expected command completion to clear busy state")
				}
				if got.terminal.Cwd != rightDir {
					t.Fatalf("expected command completion to resync terminal cwd to active panel, got %q", got.terminal.Cwd)
				}
				return
			}
		case <-time.After(3 * time.Second):
			t.Fatal("timed out waiting for streamed command output")
		}
	}
	t.Fatalf("expected streamed command to finish, got terminal lines %+v", got.terminal.Lines)
}

func TestSubmitTerminalCommandDetachesGUIExeWithoutRunningState(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("detached GUI command handling only applies on Windows")
	}

	original := xdfileIsDetachedGUIExecutableFunc
	defer func() { xdfileIsDetachedGUIExecutableFunc = original }()

	dir := t.TempDir()
	xdfileIsDetachedGUIExecutableFunc = func(path string) bool {
		return strings.EqualFold(filepath.Base(path), "cmd.exe")
	}

	m := &xdfileModel{
		terminalFocused: true,
		activePanel:     0,
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: dir},
			{Label: "RIGHT", Cwd: dir},
		},
		terminal: xdfileTerminal{
			Cwd:   dir,
			Input: textinput.New(),
		},
	}

	cmd := m.submitTerminalCommand(`cmd.exe /c exit`)
	if cmd == nil {
		t.Fatal("expected detached GUI command submission to return a result command")
	}
	if m.terminal.Busy {
		t.Fatal("expected detached GUI command not to mark terminal busy")
	}
	if m.terminalFocused {
		t.Fatal("expected detached GUI command to restore panel focus")
	}
	if _, ok := cmd().(xdfileTerminalResultMsg); !ok {
		t.Fatalf("expected detached GUI command to produce terminal result message")
	}
}

func TestSubmitTerminalCommandExpandsManagedMetasymbolsAndKeepsTemplateHistory(t *testing.T) {
	input := textinput.New()
	input.SetValue(`rehash.cmd !& | clip`)
	input.CursorEnd()

	dir := t.TempDir()
	filePath := filepath.Join(dir, "xdfile.exe")
	m := &xdfileModel{
		activePanel: 0,
		panels: [2]xdfilePanel{
			{
				Label: "LEFT",
				Cwd:   dir,
				Entries: []xdfileEntry{
					{Name: "xdfile.exe", Path: filePath},
				},
			},
		},
		terminal: xdfileTerminal{
			Cwd:   dir,
			Input: input,
		},
	}

	cmd := m.submitTerminalCommand(`rehash.cmd !& | clip`)
	if cmd == nil {
		t.Fatal("expected managed metasymbol command submission to return a command")
	}
	if len(m.terminal.History) != 1 || m.terminal.History[0] != `rehash.cmd !& | clip` {
		t.Fatalf("expected command history to keep the typed template, got %+v", m.terminal.History)
	}

	msg := cmd()
	start, ok := msg.(xdfileTerminalCommandStartMsg)
	if !ok {
		t.Fatalf("expected command start message, got %T", msg)
	}
	defer start.Cancel()
	if start.Command != `rehash.cmd "xdfile.exe" | clip` {
		t.Fatalf("expected managed metasymbol command to expand before execution, got %q", start.Command)
	}
}

func TestStreamingCommandTerminalSizeUsesOutputBody(t *testing.T) {
	m := &xdfileModel{
		layout: xdfileLayout{
			terminalRect: xdfileRect{w: 166, h: 14},
		},
	}

	width, height := m.streamingCommandTerminalSize()
	if width != 164 {
		t.Fatalf("expected streaming command width to use terminal inner width 164, got %d", width)
	}
	if height != 10 {
		t.Fatalf("expected streaming command height to exclude title and input rows, got %d", height)
	}
}

func TestSubmitTerminalCommandQuotedBatchArgumentWithPipeWorksOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("cmd quoting regression only applies on Windows")
	}

	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "showarg.cmd")
	if err := os.WriteFile(scriptPath, []byte("@echo [%1]\r\n"), 0o644); err != nil {
		t.Fatalf("write helper batch: %v", err)
	}

	input := textinput.New()
	command := `call "` + scriptPath + `" "xdfile.log" | findstr /c:"xdfile.log"`
	input.SetValue(command)
	input.CursorEnd()

	m := &xdfileModel{
		terminalFocused: true,
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: dir},
			{Label: "RIGHT", Cwd: dir},
		},
		terminal: xdfileTerminal{
			Cwd:   dir,
			Input: input,
		},
	}

	cmd := m.submitTerminalCommand(command)
	if cmd == nil {
		t.Fatal("expected quoted batch command submission to return a command")
	}

	startMsg := cmd()
	start, ok := startMsg.(xdfileTerminalCommandStartMsg)
	if !ok {
		t.Fatalf("expected command start message, got %T", startMsg)
	}
	defer start.Cancel()

	updated, waitCmd := m.Update(start)
	got := updated.(*xdfileModel)
	if waitCmd == nil {
		t.Fatal("expected streaming command start to wait for events")
	}

	var sawOutput bool
	for i := 0; i < 8; i++ {
		select {
		case msg, ok := <-start.Events:
			if !ok {
				t.Fatal("expected quoted batch command events before channel close")
			}
			updated, _ = got.Update(msg)
			got = updated.(*xdfileModel)
			switch typed := msg.(type) {
			case xdfileTerminalLineMsg:
				if strings.Contains(typed.Line, "xdfile.log") {
					sawOutput = true
				}
			case xdfileTerminalStreamScreenMsg:
				if got.terminal.StreamEmulator != nil {
					screen := xdfileRenderStreamingTerminalScreen(
						got.terminal.StreamEmulator,
						got.terminal.StreamEmulator.Width(),
						got.terminal.StreamEmulator.Height(),
					)
					if strings.Contains(screen, "xdfile.log") {
						sawOutput = true
					}
				}
			case xdfileTerminalCommandDoneMsg:
				if typed.Err != nil {
					t.Fatalf("expected quoted batch command to succeed, got %v", typed.Err)
				}
				if !sawOutput {
					for _, line := range got.terminal.Lines {
						if strings.Contains(line, "xdfile.log") {
							sawOutput = true
							break
						}
					}
				}
				if !sawOutput {
					t.Fatalf("expected quoted batch command output before done, got %+v", got.terminal.Lines)
				}
				return
			}
		case <-time.After(3 * time.Second):
			t.Fatal("timed out waiting for quoted batch command output")
		}
	}

	t.Fatalf("expected quoted batch command to finish, got terminal lines %+v", got.terminal.Lines)
}

func TestCommandLineExternalCommandKeepsPanelsVisible(t *testing.T) {
	input := textinput.New()
	input.SetValue("git status")

	leftDir := t.TempDir()
	rightDir := t.TempDir()

	m := &xdfileModel{
		width:       120,
		height:      40,
		activePanel: 0,
		layoutPrefs: xdfileDefaultLayoutPrefs(),
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: leftDir, Entries: []xdfileEntry{{Name: "a.txt", Path: filepath.Join(leftDir, "a.txt")}}},
			{Label: "RIGHT", Cwd: rightDir, Entries: []xdfileEntry{{Name: "b.txt", Path: filepath.Join(rightDir, "b.txt")}}},
		},
		terminal: xdfileTerminal{
			Cwd:   leftDir,
			Input: input,
		},
	}
	m.computeLayout()

	cmd, handled := m.handleManagedTerminalBoundKey(tea.KeyMsg{Type: tea.KeyEnter})
	if !handled {
		t.Fatal("expected command-line enter to be handled")
	}
	if cmd == nil {
		t.Fatal("expected external command submission to return a command")
	}

	rendered := stripXdfileANSI(m.View())
	if !strings.Contains(rendered, "LEFT") || !strings.Contains(rendered, "RIGHT") {
		t.Fatalf("expected both file panels to remain visible during command execution, got %q", rendered)
	}
}

func TestXdfileTerminalLineRewriteKeepsSingleProgressLine(t *testing.T) {
	m := &xdfileModel{
		width:  120,
		height: 40,
		terminal: xdfileTerminal{
			Lines: []string{`C:\work> demo`},
			Input: textinput.New(),
		},
	}
	m.computeLayout()

	updated, _ := m.Update(xdfileTerminalLineMsg{Line: "progress 1", Rewrite: true})
	got := updated.(*xdfileModel)
	updated, _ = got.Update(xdfileTerminalLineMsg{Line: "progress 2", Rewrite: true})
	got = updated.(*xdfileModel)
	updated, _ = got.Update(xdfileTerminalLineMsg{Line: "progress done", Rewrite: true, Finalize: true})
	got = updated.(*xdfileModel)

	if len(got.terminal.Lines) != 2 {
		t.Fatalf("expected prompt plus one progress line, got %+v", got.terminal.Lines)
	}
	if got.terminal.Lines[1] != "progress done" {
		t.Fatalf("expected final progress line to be replaced in place, got %+v", got.terminal.Lines)
	}
	if got.terminal.StreamCanRewrite {
		t.Fatal("expected finalized progress line to stop rewrite mode")
	}
}

func TestXdfileTerminalCommandDoneKeepsBottomTerminalLayout(t *testing.T) {
	leftDir := t.TempDir()
	rightDir := t.TempDir()

	m := &xdfileModel{
		width:       120,
		height:      40,
		layoutPrefs: xdfileDefaultLayoutPrefs(),
		panels: [2]xdfilePanel{
			{Label: "Left", Cwd: leftDir},
			{Label: "Right", Cwd: rightDir},
		},
		terminal: xdfileTerminal{
			Cwd:   leftDir,
			Input: textinput.New(),
		},
	}

	m.computeLayout()
	if got := m.layout.panelRects[0].h; got == 0 {
		t.Fatalf("expected panel layout to remain intact, got left panel height %d", got)
	}

	updated, _ := m.Update(xdfileTerminalCommandDoneMsg{Cwd: leftDir})
	got := updated.(*xdfileModel)

	if got.terminalFocused {
		t.Fatal("expected command completion to return focus to the file panel")
	}
	if got.layout.panelRects[0].h == 0 || got.layout.panelRects[1].h == 0 {
		t.Fatalf("expected panel rects to remain available after restore, got %+v", got.layout.panelRects)
	}
	if got.terminalRenderRect() != got.layout.terminalRect {
		t.Fatalf("expected terminal render rect to stay anchored to the bottom layout, got %+v want %+v", got.terminalRenderRect(), got.layout.terminalRect)
	}
	expectedViewportHeight := max(1, got.layout.terminalRect.h-5)
	if got.terminal.Viewport.Height != expectedViewportHeight {
		t.Fatalf("expected terminal viewport height %d after restore, got %d", expectedViewportHeight, got.terminal.Viewport.Height)
	}
}

func TestXdfileTerminalResultReturnsFocusToPanel(t *testing.T) {
	m := &xdfileModel{
		width:           120,
		height:          40,
		terminalFocused: true,
		layoutPrefs:     xdfileDefaultLayoutPrefs(),
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: t.TempDir()},
			{Label: "RIGHT", Cwd: t.TempDir()},
		},
		terminal: xdfileTerminal{
			Input: textinput.New(),
			Busy:  true,
		},
	}

	m.computeLayout()
	updated, cmd := m.Update(xdfileTerminalResultMsg{
		Command: "git status",
		Dir:     m.panels[0].Cwd,
		Output:  "ok",
	})
	got := updated.(*xdfileModel)
	if cmd != nil {
		t.Fatalf("expected one-shot command completion to finish immediately")
	}
	if got.terminalFocused {
		t.Fatal("expected one-shot command completion to return focus to the file panel")
	}
}

func TestHandleTerminalKeyCtrlCCancelsManagedCommand(t *testing.T) {
	input := textinput.New()
	canceled := false
	m := &xdfileModel{
		terminalFocused: true,
		terminal: xdfileTerminal{
			Cwd:           t.TempDir(),
			Input:         input,
			Busy:          true,
			ManagedCancel: func() { canceled = true },
		},
	}

	cmd := m.handleTerminalKey(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd != nil {
		t.Fatalf("expected ctrl+c cancellation not to schedule another command")
	}
	if !canceled {
		t.Fatal("expected ctrl+c to cancel the managed external command")
	}
	if m.terminal.ManagedCancel != nil {
		t.Fatal("expected managed cancel hook to be cleared after ctrl+c")
	}
	if got := m.statusText; got != "Stopping command..." {
		t.Fatalf("expected ctrl+c to update status, got %q", got)
	}
}

func TestHandleGlobalKeyCtrlCCancelsManagedCommandWhilePanelFocused(t *testing.T) {
	canceled := false
	m := &xdfileModel{
		terminal: xdfileTerminal{
			Busy:          true,
			ManagedCancel: func() { canceled = true },
		},
	}

	cmd, handled := m.handleGlobalKey(tea.KeyMsg{Type: tea.KeyCtrlC})
	if !handled {
		t.Fatal("expected ctrl+c to be handled while a managed command is running")
	}
	if cmd != nil {
		t.Fatal("expected ctrl+c cancellation not to enqueue another command")
	}
	if !canceled {
		t.Fatal("expected ctrl+c to cancel the running managed command")
	}
	if got := m.statusText; got != "Stopping command..." {
		t.Fatalf("expected status to announce managed command cancellation, got %q", got)
	}
}

func TestXdfileTerminalCommandPollReloadsPanelsWhenPromptReturns(t *testing.T) {
	dir := t.TempDir()
	initialPath := filepath.Join(dir, "before.txt")
	if err := os.WriteFile(initialPath, []byte("before"), 0o644); err != nil {
		t.Fatalf("write initial file: %v", err)
	}

	emulator := vt.NewSafeEmulator(80, 24)
	if _, err := emulator.Write([]byte("PS " + dir + "> ")); err != nil {
		t.Fatalf("seed emulator prompt: %v", err)
	}

	m := &xdfileModel{
		activePanel: 0,
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: dir},
			{Label: "RIGHT", Cwd: dir},
		},
		terminal: xdfileTerminal{
			Busy:     true,
			Session:  &xdfileTerminalPTYSession{},
			Emulator: emulator,
		},
	}
	if err := m.reloadPanel(0); err != nil {
		t.Fatalf("load left panel: %v", err)
	}
	if err := m.reloadPanel(1); err != nil {
		t.Fatalf("load right panel: %v", err)
	}

	addedPath := filepath.Join(dir, "download.zip")
	if err := os.WriteFile(addedPath, []byte("payload"), 0o644); err != nil {
		t.Fatalf("write downloaded file: %v", err)
	}

	updated, cmd := m.Update(xdfileTerminalCommandPollMsg{})
	got := updated.(*xdfileModel)
	if !got.terminal.Busy && cmd != nil {
		t.Fatalf("expected polling command to stop once busy flag clears")
	}
	if got.terminal.PendingPolls >= xdfilePTYCommandPollMaxTicks {
		t.Fatalf("expected PTY command poll to consume at least one polling tick")
	}

	found := false
	for _, entry := range got.panels[0].Entries {
		if entry.Name == "download.zip" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected new file to appear after PTY command poll refresh, got %+v", got.panels[0].Entries)
	}
}

func TestHandleMouseWheelScrollsPTYScrollbackWhenNotAltScreen(t *testing.T) {
	emulator := vt.NewSafeEmulator(32, 5)
	for i := 0; i < 16; i++ {
		if _, err := emulator.Write([]byte("line\n")); err != nil {
			t.Fatalf("seed emulator scrollback: %v", err)
		}
	}

	m := &xdfileModel{
		width:           100,
		height:          30,
		terminalFocused: true,
		layout: xdfileLayout{
			terminalRect: xdfileRect{x: 0, y: 2, w: 100, h: 10},
		},
		terminal: xdfileTerminal{
			Session:  &xdfileTerminalPTYSession{},
			Emulator: emulator,
		},
	}

	if cmd := m.handleMouse(tea.MouseMsg{
		X:      5,
		Y:      5,
		Button: tea.MouseButtonWheelUp,
	}); cmd != nil {
		t.Fatalf("expected terminal wheel scroll to complete immediately")
	}
	if m.terminal.ScrollOffset == 0 {
		t.Fatalf("expected wheel-up inside PTY terminal to scroll local history")
	}
}

func TestHandleMouseMotionKeepsPTYScrollbackOffset(t *testing.T) {
	emulator := vt.NewSafeEmulator(32, 5)
	m := &xdfileModel{
		width:           100,
		height:          30,
		terminalFocused: true,
		layout: xdfileLayout{
			terminalRect: xdfileRect{x: 0, y: 2, w: 100, h: 10},
		},
		terminal: xdfileTerminal{
			Session:      &xdfileTerminalPTYSession{},
			Emulator:     emulator,
			ViewWidth:    98,
			ViewHeight:   7,
			ScrollOffset: 9,
		},
	}

	if cmd := m.handleMouse(tea.MouseMsg{
		X:      5,
		Y:      5,
		Action: tea.MouseActionMotion,
		Button: tea.MouseButtonNone,
	}); cmd != nil {
		t.Fatalf("expected terminal mouse motion to complete immediately")
	}
	if got, want := m.terminal.ScrollOffset, 9; got != want {
		t.Fatalf("expected terminal scrollback offset to remain %d while moving the mouse, got %d", want, got)
	}
}

func TestHandleMouseMotionTracksHoveredPanelEntry(t *testing.T) {
	root := t.TempDir()
	left := filepath.Join(root, "left")
	right := filepath.Join(root, "right")
	if err := os.MkdirAll(left, 0o755); err != nil {
		t.Fatalf("create left dir: %v", err)
	}
	if err := os.MkdirAll(right, 0o755); err != nil {
		t.Fatalf("create right dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(left, "sample.txt"), []byte("sample"), 0o644); err != nil {
		t.Fatalf("write left file: %v", err)
	}

	m := &xdfileModel{
		width:  120,
		height: 40,
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: left},
			{Label: "RIGHT", Cwd: right},
		},
		hover: xdfileHoverState{
			MenuItem:   -1,
			Panel:      -1,
			PanelIndex: -1,
		},
	}
	for i := range m.panels {
		if err := m.reloadPanel(i); err != nil {
			t.Fatalf("reload panel %d: %v", i, err)
		}
	}
	m.computeLayout()

	index := findXdfileEntryIndex(t, m.panels[0].Entries, "sample.txt")
	rect := m.layout.panelRects[0]
	if cmd := m.handleMouse(tea.MouseMsg{
		X:      rect.x + 2,
		Y:      rect.y + 3 + index - m.panels[0].Scroll,
		Button: tea.MouseButtonNone,
		Action: tea.MouseActionMotion,
	}); cmd != nil {
		t.Fatalf("expected hover motion to complete immediately")
	}

	if m.hover.Panel != 0 || m.hover.PanelIndex != index {
		t.Fatalf("expected hovered panel entry to track sample.txt, got %+v", m.hover)
	}
}

func TestHandleMousePressTracksHoveredPanelEntry(t *testing.T) {
	m := &xdfileModel{
		width:       120,
		height:      40,
		activePanel: 0,
		panels: [2]xdfilePanel{
			{
				Label: "LEFT",
				Cwd:   `C:\left`,
				Entries: []xdfileEntry{
					{Name: "a.txt", Path: `C:\left\a.txt`},
					{Name: "b.txt", Path: `C:\left\b.txt`},
				},
			},
			{Label: "RIGHT", Cwd: `D:\right`},
		},
		hover: xdfileNoHoverState(),
	}
	m.computeLayout()

	rect := m.layout.panelRects[0]
	if cmd := m.handleMouse(tea.MouseMsg{
		X:      rect.x + 2,
		Y:      rect.y + 4,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	}); cmd != nil {
		t.Fatalf("expected panel click to complete immediately")
	}

	if m.hover.Panel != 0 || m.hover.PanelIndex != 1 {
		t.Fatalf("expected mouse press to update hovered panel entry, got %+v", m.hover)
	}
}

func TestHandleMouseMotionClearsHoverOutsideHitTargets(t *testing.T) {
	m := &xdfileModel{
		width:       120,
		height:      40,
		activePanel: 0,
		layoutPrefs: xdfileDefaultLayoutPrefs(),
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: `C:\left`},
			{Label: "RIGHT", Cwd: `D:\right`},
		},
		hover: xdfileHoverState{
			Panel:      0,
			PanelIndex: 0,
			MenuItem:   xdfileNoHoverIndex,
		},
	}
	m.computeLayout()

	if cmd := m.handleMouse(tea.MouseMsg{
		X:      m.layout.terminalRect.x + 1,
		Y:      m.layout.terminalRect.y + 1,
		Button: tea.MouseButtonNone,
		Action: tea.MouseActionMotion,
	}); cmd != nil {
		t.Fatalf("expected hover motion to complete immediately")
	}

	if m.hover != xdfileNoHoverState() {
		t.Fatalf("expected terminal hover to clear file/menu hover, got %+v", m.hover)
	}
}

func TestHandleMouseMotionTracksHoveredCommandMenuItem(t *testing.T) {
	m := &xdfileModel{
		width:    120,
		height:   40,
		openMenu: xdfileActionCommandsMenu,
		layoutPrefs: xdfileLayoutPrefs{
			Commands: []xdfileCommandItem{
				{Type: xdfileCommandItemTypeCommand, Label: "Scan file", Command: "scan.cmd"},
			},
		},
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: `C:\left`},
			{Label: "RIGHT", Cwd: `D:\right`},
		},
		hover: xdfileHoverState{
			MenuItem:   -1,
			Panel:      -1,
			PanelIndex: -1,
		},
	}
	m.computeLayout()
	_ = m.renderOpenMenu()

	if len(m.layout.menuItemRects) == 0 {
		t.Fatal("expected commands menu item hit rects to be populated")
	}
	itemRect := m.layout.menuItemRects[0].Rect
	if cmd := m.handleMouse(tea.MouseMsg{
		X:      itemRect.x + 1,
		Y:      itemRect.y,
		Button: tea.MouseButtonNone,
		Action: tea.MouseActionMotion,
	}); cmd != nil {
		t.Fatalf("expected menu hover motion to complete immediately")
	}

	if m.hover.MenuItem != 0 {
		t.Fatalf("expected hovered commands menu item index 0, got %+v", m.hover)
	}
}

func TestHandleMouseMotionTracksHoveredFooterAction(t *testing.T) {
	m := &xdfileModel{
		width:       120,
		height:      40,
		layoutPrefs: xdfileDefaultLayoutPrefs(),
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: `C:\left`},
			{Label: "RIGHT", Cwd: `D:\right`},
		},
		hover: xdfileHoverState{
			MenuItem:   -1,
			Panel:      -1,
			PanelIndex: -1,
		},
	}
	m.computeLayout()
	_ = m.renderFooter()

	var target xdfileRect
	found := false
	for _, hit := range m.layout.footerButtons {
		if hit.Action == xdfileActionCommandsMenu {
			target = hit.Rect
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected footer to expose an F2 commands hit rect")
	}

	if cmd := m.handleMouse(tea.MouseMsg{
		X:      target.x + 1,
		Y:      target.y,
		Button: tea.MouseButtonNone,
		Action: tea.MouseActionMotion,
	}); cmd != nil {
		t.Fatalf("expected footer hover motion to complete immediately")
	}

	if m.hover.FooterAction != xdfileActionCommandsMenu {
		t.Fatalf("expected hovered footer action %q, got %+v", xdfileActionCommandsMenu, m.hover)
	}
}

func TestHandleMouseClickAbovePanelBodyMovesCursorToTop(t *testing.T) {
	root := t.TempDir()
	left := filepath.Join(root, "left")
	right := filepath.Join(root, "right")
	if err := os.MkdirAll(left, 0o755); err != nil {
		t.Fatalf("create left dir: %v", err)
	}
	if err := os.MkdirAll(right, 0o755); err != nil {
		t.Fatalf("create right dir: %v", err)
	}
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		if err := os.WriteFile(filepath.Join(left, name), []byte(name), 0o644); err != nil {
			t.Fatalf("write left file %q: %v", name, err)
		}
	}

	m := &xdfileModel{
		width:  120,
		height: 40,
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: left},
			{Label: "RIGHT", Cwd: right},
		},
		lastClick: xdfileClickState{panel: 0, row: 2, at: time.Now()},
	}
	for i := range m.panels {
		if err := m.reloadPanel(i); err != nil {
			t.Fatalf("reload panel %d: %v", i, err)
		}
	}
	m.computeLayout()
	m.panels[0].setCursor(len(m.panels[0].Entries)-1, m.panels[0].visibleRows(m.layout.panelRects[0].h))

	rect := m.layout.panelRects[0]
	cmd := m.handleMouse(tea.MouseMsg{
		X:      rect.x + 2,
		Y:      rect.y + 1,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	if cmd != nil {
		if msg := cmd(); msg != nil {
			t.Fatalf("expected top blank click to complete immediately, got %T", msg)
		}
	}
	if got := m.panels[0].Cursor; got != 0 {
		t.Fatalf("expected click above panel body to move cursor to top, got %d", got)
	}
	if m.lastClick.panel != 0 || m.lastClick.row != 0 {
		t.Fatalf("expected blank-area click to track selected top entry for double-click, got %+v", m.lastClick)
	}
}

func TestHandleMouseClickBelowPanelBodyMovesCursorToBottom(t *testing.T) {
	root := t.TempDir()
	left := filepath.Join(root, "left")
	right := filepath.Join(root, "right")
	if err := os.MkdirAll(left, 0o755); err != nil {
		t.Fatalf("create left dir: %v", err)
	}
	if err := os.MkdirAll(right, 0o755); err != nil {
		t.Fatalf("create right dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(left, "sample.txt"), []byte("sample"), 0o644); err != nil {
		t.Fatalf("write left file: %v", err)
	}

	m := &xdfileModel{
		width:  120,
		height: 40,
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: left},
			{Label: "RIGHT", Cwd: right},
		},
		lastClick: xdfileClickState{panel: -1, row: -1},
	}
	for i := range m.panels {
		if err := m.reloadPanel(i); err != nil {
			t.Fatalf("reload panel %d: %v", i, err)
		}
	}
	m.computeLayout()
	m.panels[0].setCursor(0, m.panels[0].visibleRows(m.layout.panelRects[0].h))

	rect := m.layout.panelRects[0]
	cmd := m.handleMouse(tea.MouseMsg{
		X:      rect.x + 2,
		Y:      rect.y + rect.h - 2,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	if cmd != nil {
		if msg := cmd(); msg != nil {
			t.Fatalf("expected bottom blank click to complete immediately, got %T", msg)
		}
	}
	if want, got := len(m.panels[0].Entries)-1, m.panels[0].Cursor; got != want {
		t.Fatalf("expected click below panel body to move cursor to bottom %d, got %d", want, got)
	}
	if want, got := len(m.panels[0].Entries)-1, m.lastClick.row; got != want || m.lastClick.panel != 0 {
		t.Fatalf("expected blank-area click to track selected bottom entry %d for double-click, got %+v", want, m.lastClick)
	}
}

func TestHandleMouseDoubleClickAbovePanelBodyOpensTopSelection(t *testing.T) {
	root := t.TempDir()
	left := filepath.Join(root, "left")
	right := filepath.Join(root, "right")
	if err := os.MkdirAll(left, 0o755); err != nil {
		t.Fatalf("create left dir: %v", err)
	}
	if err := os.MkdirAll(right, 0o755); err != nil {
		t.Fatalf("create right dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(left, "sample.txt"), []byte("sample"), 0o644); err != nil {
		t.Fatalf("write left file: %v", err)
	}

	m := &xdfileModel{
		width:  120,
		height: 40,
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: left},
			{Label: "RIGHT", Cwd: right},
		},
		lastClick: xdfileClickState{panel: -1, row: -1},
	}
	for i := range m.panels {
		if err := m.reloadPanel(i); err != nil {
			t.Fatalf("reload panel %d: %v", i, err)
		}
	}
	m.computeLayout()

	rect := m.layout.panelRects[0]
	click := tea.MouseMsg{
		X:      rect.x + 2,
		Y:      rect.y + 1,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	}
	_ = m.handleMouse(click)
	_ = m.handleMouse(click)

	want := filepath.Dir(left)
	if !xdfilePathsEqual(m.panels[0].Cwd, want) {
		t.Fatalf("expected double-click above panel body to open top selection %q, got %q", want, m.panels[0].Cwd)
	}
	if m.lastClick.panel != -1 {
		t.Fatalf("expected double-click activation to clear tracking, got %+v", m.lastClick)
	}
}

func TestHandleMouseDoubleClickBelowPanelBodyOpensBottomSelection(t *testing.T) {
	root := t.TempDir()
	left := filepath.Join(root, "left")
	right := filepath.Join(root, "right")
	if err := os.MkdirAll(left, 0o755); err != nil {
		t.Fatalf("create left dir: %v", err)
	}
	if err := os.MkdirAll(right, 0o755); err != nil {
		t.Fatalf("create right dir: %v", err)
	}
	filePath := filepath.Join(left, "sample.txt")
	if err := os.WriteFile(filePath, []byte("sample"), 0o644); err != nil {
		t.Fatalf("write left file: %v", err)
	}

	originalOpenPath := xdfileOpenPathFunc
	defer func() {
		xdfileOpenPathFunc = originalOpenPath
	}()
	var opened string
	xdfileOpenPathFunc = func(path string) error {
		opened = path
		return nil
	}

	m := &xdfileModel{
		width:  120,
		height: 40,
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: left},
			{Label: "RIGHT", Cwd: right},
		},
		lastClick: xdfileClickState{panel: -1, row: -1},
	}
	for i := range m.panels {
		if err := m.reloadPanel(i); err != nil {
			t.Fatalf("reload panel %d: %v", i, err)
		}
	}
	m.computeLayout()

	rect := m.layout.panelRects[0]
	click := tea.MouseMsg{
		X:      rect.x + 2,
		Y:      rect.y + rect.h - 2,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	}
	_ = m.handleMouse(click)
	_ = m.handleMouse(click)

	if opened != filePath {
		t.Fatalf("expected double-click below panel body to open bottom selection %q, got %q", filePath, opened)
	}
	if m.lastClick.panel != -1 {
		t.Fatalf("expected double-click activation to clear tracking, got %+v", m.lastClick)
	}
}

func TestHandleTerminalKeyQueuesCommandWhilePTYStarts(t *testing.T) {
	input := textinput.New()
	input.SetValue("git status")

	m := &xdfileModel{
		terminalStarting: true,
		terminal: xdfileTerminal{
			Input: input,
		},
	}

	cmd := m.handleTerminalKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatalf("expected startup enter handling to complete immediately")
	}
	if !m.terminal.StartupSubmitPending {
		t.Fatalf("expected enter during startup to queue command submission")
	}
	if got := m.terminal.Input.Value(); got != "git status" {
		t.Fatalf("expected queued startup draft to be preserved, got %q", got)
	}
}

func TestTerminalStartResultReplaysQueuedStartupCommand(t *testing.T) {
	backend := &testXdfilePTYBackend{}
	session := &xdfileTerminalPTYSession{
		backend:  backend,
		emulator: vt.NewSafeEmulator(80, 24),
		events:   make(chan tea.Msg, 1),
	}
	go session.runInputLoop()
	defer session.Close()

	input := textinput.New()
	input.SetValue("git status")

	m := &xdfileModel{
		activePanel:      0,
		terminalStarting: true,
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: `C:\work`},
			{Label: "RIGHT", Cwd: `D:\else`},
		},
		terminal: xdfileTerminal{
			Input:                input,
			StartupSubmitPending: true,
		},
	}

	updated, cmd := m.Update(xdfileTerminalStartResultMsg{Session: session, Dir: `C:\work`})
	got := updated.(*xdfileModel)
	if got.terminalStarting {
		t.Fatalf("expected terminal startup flag to clear once PTY is ready")
	}
	if got.terminal.Input.Value() != "" {
		t.Fatalf("expected queued startup draft to clear after replay, got %q", got.terminal.Input.Value())
	}
	if got.terminal.StartupSubmitPending {
		t.Fatalf("expected queued startup submission flag to clear after replay")
	}
	if cmd == nil {
		t.Fatalf("expected PTY startup to return follow-up command(s)")
	}
	time.Sleep(50 * time.Millisecond)
	if got := backend.writes.String(); got != "git status\r" {
		t.Fatalf("expected queued startup command to replay into PTY, got %q", got)
	}
}
