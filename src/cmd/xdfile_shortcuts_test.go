package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	vt "github.com/charmbracelet/x/vt"
)

func TestHandleGlobalKeyBareDigitsPassThrough(t *testing.T) {
	root := t.TempDir()
	left := filepath.Join(root, "left")
	right := filepath.Join(root, "right")
	if err := os.MkdirAll(left, 0o755); err != nil {
		t.Fatalf("create left dir: %v", err)
	}
	if err := os.MkdirAll(right, 0o755); err != nil {
		t.Fatalf("create right dir: %v", err)
	}

	for _, name := range []string{"b.txt", "a.go"} {
		if err := os.WriteFile(filepath.Join(left, name), []byte(name), 0o644); err != nil {
			t.Fatalf("write file %q: %v", name, err)
		}
	}

	m := &xdfileModel{
		activePanel: 0,
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: left},
			{Label: "RIGHT", Cwd: right},
		},
	}
	for i := range m.panels {
		if err := m.reloadPanel(i); err != nil {
			t.Fatalf("reload panel %d: %v", i, err)
		}
	}

	if cmd, handled := m.handleGlobalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'4'}}); handled || cmd != nil {
		t.Fatalf("expected bare 4 to pass through for text input, handled=%v cmd=%v", handled, cmd)
	}

	if cmd, handled := m.handleGlobalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}}); handled || cmd != nil {
		t.Fatalf("expected bare 3 to pass through for text input, handled=%v cmd=%v", handled, cmd)
	}
}

func TestHandleGlobalKeyCtrlBackslashSortsEvenWhenPTYFocused(t *testing.T) {
	root := t.TempDir()
	left := filepath.Join(root, "left")
	right := filepath.Join(root, "right")
	if err := os.MkdirAll(left, 0o755); err != nil {
		t.Fatalf("create left dir: %v", err)
	}
	if err := os.MkdirAll(right, 0o755); err != nil {
		t.Fatalf("create right dir: %v", err)
	}

	m := &xdfileModel{
		activePanel:     0,
		terminalFocused: true,
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: left},
			{Label: "RIGHT", Cwd: right},
		},
		terminal: xdfileTerminal{
			Session:  &xdfileTerminalPTYSession{},
			Emulator: vt.NewSafeEmulator(80, 24),
		},
	}
	for i := range m.panels {
		if err := m.reloadPanel(i); err != nil {
			t.Fatalf("reload panel %d: %v", i, err)
		}
	}

	cmd, handled := m.handleGlobalKey(tea.KeyMsg{Type: tea.KeyCtrlBackslash})
	if !handled {
		t.Fatalf("expected ctrl+backslash sort shortcut to be handled while PTY is focused")
	}
	if cmd == nil {
		t.Fatalf("expected ctrl+backslash sort shortcut to return a command")
	}
	_ = cmd()
	if m.panelSortMode(0) != xdfileSortModeExt {
		t.Fatalf("expected ctrl+backslash to switch panel sort mode to extension, got %q", m.panelSortMode(0))
	}
}

func TestHandleGlobalKeyCtrl4SortsByExtension(t *testing.T) {
	root := t.TempDir()
	left := filepath.Join(root, "left")
	right := filepath.Join(root, "right")
	if err := os.MkdirAll(left, 0o755); err != nil {
		t.Fatalf("create left dir: %v", err)
	}
	if err := os.MkdirAll(right, 0o755); err != nil {
		t.Fatalf("create right dir: %v", err)
	}

	m := &xdfileModel{
		activePanel: 0,
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: left},
			{Label: "RIGHT", Cwd: right},
		},
	}
	for i := range m.panels {
		if err := m.reloadPanel(i); err != nil {
			t.Fatalf("reload panel %d: %v", i, err)
		}
	}

	cmd, handled := m.handleGlobalKey(tea.KeyMsg{Type: tea.KeyCtrl4})
	if !handled {
		t.Fatalf("expected ctrl+4 sort shortcut to be handled")
	}
	if cmd == nil {
		t.Fatalf("expected ctrl+4 sort shortcut to return a command")
	}
	_ = cmd()
	if m.panelSortMode(0) != xdfileSortModeExt {
		t.Fatalf("expected ctrl+4 to switch panel sort mode to extension, got %q", m.panelSortMode(0))
	}
}

func TestHandleGlobalKeyCtrl3SortsByNameWhenPanelFocused(t *testing.T) {
	root := t.TempDir()
	left := filepath.Join(root, "left")
	right := filepath.Join(root, "right")
	if err := os.MkdirAll(left, 0o755); err != nil {
		t.Fatalf("create left dir: %v", err)
	}
	if err := os.MkdirAll(right, 0o755); err != nil {
		t.Fatalf("create right dir: %v", err)
	}

	m := &xdfileModel{
		activePanel: 0,
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: left},
			{Label: "RIGHT", Cwd: right},
		},
	}
	for i := range m.panels {
		if err := m.reloadPanel(i); err != nil {
			t.Fatalf("reload panel %d: %v", i, err)
		}
	}

	cmd, handled := m.handleGlobalKey(tea.KeyMsg{Type: tea.KeyCtrl3})
	if !handled {
		t.Fatalf("expected ctrl+3 sort shortcut to be handled")
	}
	if cmd == nil {
		t.Fatalf("expected ctrl+3 sort shortcut to return a command")
	}
	_ = cmd()
	if m.panelSortMode(0) != xdfileSortModeName {
		t.Fatalf("expected ctrl+3 to switch panel sort mode to name, got %q", m.panelSortMode(0))
	}
}

func TestHandleGlobalKeyEscDoesNotResetPanelSortMode(t *testing.T) {
	root := t.TempDir()
	left := filepath.Join(root, "left")
	right := filepath.Join(root, "right")
	if err := os.MkdirAll(left, 0o755); err != nil {
		t.Fatalf("create left dir: %v", err)
	}
	if err := os.MkdirAll(right, 0o755); err != nil {
		t.Fatalf("create right dir: %v", err)
	}
	for _, name := range []string{"b.txt", "a.go"} {
		if err := os.WriteFile(filepath.Join(left, name), []byte(name), 0o644); err != nil {
			t.Fatalf("write file %q: %v", name, err)
		}
	}

	m := &xdfileModel{
		activePanel: 0,
		layout: xdfileLayout{
			panelRects: [2]xdfileRect{{h: 12}, {h: 12}},
		},
		layoutPrefs: xdfileLayoutPrefs{
			LeftSortMode:  xdfileSortModeExt,
			RightSortMode: xdfileSortModeName,
		},
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: left, RangeAnchor: -1},
			{Label: "RIGHT", Cwd: right, RangeAnchor: -1},
		},
	}
	for i := range m.panels {
		if err := m.reloadPanel(i); err != nil {
			t.Fatalf("reload panel %d: %v", i, err)
		}
	}
	m.panels[0].toggleMarkedAt(findXdfileEntryIndex(t, m.panels[0].Entries, "a.go"))

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil {
		t.Fatalf("expected esc update to complete immediately")
	}
	got := updated.(*xdfileModel)
	if got.panelSortMode(0) != xdfileSortModeExt {
		t.Fatalf("expected esc to preserve extension sort mode, got %q", got.panelSortMode(0))
	}
	if got.panels[0].markedCount() != 0 {
		t.Fatalf("expected esc to keep clearing marked entries, got %d", got.panels[0].markedCount())
	}
}

func TestHandleGlobalKeyCtrlOpenBracketPassesThroughWhenPTYFocused(t *testing.T) {
	root := t.TempDir()
	left := filepath.Join(root, "left")
	right := filepath.Join(root, "right")
	if err := os.MkdirAll(left, 0o755); err != nil {
		t.Fatalf("create left dir: %v", err)
	}
	if err := os.MkdirAll(right, 0o755); err != nil {
		t.Fatalf("create right dir: %v", err)
	}

	m := &xdfileModel{
		activePanel:     0,
		terminalFocused: true,
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: left},
			{Label: "RIGHT", Cwd: right},
		},
		terminal: xdfileTerminal{
			Session:  &xdfileTerminalPTYSession{},
			Emulator: vt.NewSafeEmulator(80, 24),
		},
	}
	for i := range m.panels {
		if err := m.reloadPanel(i); err != nil {
			t.Fatalf("reload panel %d: %v", i, err)
		}
	}

	if cmd, handled := m.handleGlobalKey(tea.KeyMsg{Type: tea.KeyCtrlOpenBracket}); handled || cmd != nil {
		t.Fatalf("expected ctrl+open-bracket to pass through while PTY is focused")
	}
}

func TestHandlePanelKeyShiftDownTogglesCurrentAndAdvances(t *testing.T) {
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
			t.Fatalf("write file %q: %v", name, err)
		}
	}

	m := &xdfileModel{
		activePanel: 0,
		layout: xdfileLayout{
			panelRects: [2]xdfileRect{{h: 12}, {h: 12}},
		},
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: left, RangeAnchor: -1},
			{Label: "RIGHT", Cwd: right, RangeAnchor: -1},
		},
	}
	for i := range m.panels {
		if err := m.reloadPanel(i); err != nil {
			t.Fatalf("reload panel %d: %v", i, err)
		}
	}

	m.panels[0].setCursor(findXdfileEntryIndex(t, m.panels[0].Entries, "a.txt"), 8)
	_ = m.handlePanelKey(tea.KeyMsg{Type: tea.KeyShiftDown})
	_ = m.handlePanelKey(tea.KeyMsg{Type: tea.KeyShiftDown})

	if got := m.panels[0].markedCount(); got != 2 {
		t.Fatalf("expected shift+down to toggle two entries, got %d", got)
	}
	if !m.panels[0].isMarked(xdfileEntry{Name: "a.txt", Path: filepath.Join(left, "a.txt")}) {
		t.Fatalf("expected a.txt to be selected after the first shift+down")
	}
	if !m.panels[0].isMarked(xdfileEntry{Name: "b.txt", Path: filepath.Join(left, "b.txt")}) {
		t.Fatalf("expected b.txt to be selected after the second shift+down")
	}
	if entry, ok := m.panels[0].selected(); !ok || entry.Name != "c.txt" {
		t.Fatalf("expected cursor to land on c.txt, got %+v", entry)
	}
}

func TestHandlePanelKeyNavigationPreservesMarksForJumpSelection(t *testing.T) {
	root := t.TempDir()
	left := filepath.Join(root, "left")
	right := filepath.Join(root, "right")
	if err := os.MkdirAll(left, 0o755); err != nil {
		t.Fatalf("create left dir: %v", err)
	}
	if err := os.MkdirAll(right, 0o755); err != nil {
		t.Fatalf("create right dir: %v", err)
	}
	for _, name := range []string{"a.txt", "b.txt", "c.txt", "d.txt"} {
		if err := os.WriteFile(filepath.Join(left, name), []byte(name), 0o644); err != nil {
			t.Fatalf("write file %q: %v", name, err)
		}
	}

	m := &xdfileModel{
		activePanel: 0,
		layout: xdfileLayout{
			panelRects: [2]xdfileRect{{h: 12}, {h: 12}},
		},
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: left, RangeAnchor: -1},
			{Label: "RIGHT", Cwd: right, RangeAnchor: -1},
		},
	}
	for i := range m.panels {
		if err := m.reloadPanel(i); err != nil {
			t.Fatalf("reload panel %d: %v", i, err)
		}
	}

	m.panels[0].setCursor(findXdfileEntryIndex(t, m.panels[0].Entries, "a.txt"), 8)
	_ = m.handlePanelKey(tea.KeyMsg{Type: tea.KeyShiftDown})
	_ = m.handlePanelKey(tea.KeyMsg{Type: tea.KeyDown})
	_ = m.handlePanelKey(tea.KeyMsg{Type: tea.KeyDown})
	_ = m.handlePanelKey(tea.KeyMsg{Type: tea.KeyShiftUp})

	if got := m.panels[0].markedCount(); got != 2 {
		t.Fatalf("expected jump selection to preserve earlier marks, got %d", got)
	}
	if !m.panels[0].isMarked(xdfileEntry{Name: "a.txt", Path: filepath.Join(left, "a.txt")}) {
		t.Fatalf("expected a.txt to remain selected after jumping")
	}
	if !m.panels[0].isMarked(xdfileEntry{Name: "d.txt", Path: filepath.Join(left, "d.txt")}) {
		t.Fatalf("expected d.txt to be selected after jump selection")
	}
}

func TestHandlePanelKeyShiftUpCanInvertMarkedEntry(t *testing.T) {
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
			t.Fatalf("write file %q: %v", name, err)
		}
	}

	m := &xdfileModel{
		activePanel: 0,
		layout: xdfileLayout{
			panelRects: [2]xdfileRect{{h: 12}, {h: 12}},
		},
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: left, RangeAnchor: -1},
			{Label: "RIGHT", Cwd: right, RangeAnchor: -1},
		},
	}
	for i := range m.panels {
		if err := m.reloadPanel(i); err != nil {
			t.Fatalf("reload panel %d: %v", i, err)
		}
	}

	m.panels[0].setCursor(findXdfileEntryIndex(t, m.panels[0].Entries, "a.txt"), 8)
	_ = m.handlePanelKey(tea.KeyMsg{Type: tea.KeyShiftDown})
	_ = m.handlePanelKey(tea.KeyMsg{Type: tea.KeyShiftDown})
	_ = m.handlePanelKey(tea.KeyMsg{Type: tea.KeyUp})
	_ = m.handlePanelKey(tea.KeyMsg{Type: tea.KeyShiftUp})

	if got := m.panels[0].markedCount(); got != 1 {
		t.Fatalf("expected shift+up to invert the current marked entry, got %d", got)
	}
	if !m.panels[0].isMarked(xdfileEntry{Name: "a.txt", Path: filepath.Join(left, "a.txt")}) {
		t.Fatalf("expected a.txt to remain selected")
	}
	if m.panels[0].isMarked(xdfileEntry{Name: "b.txt", Path: filepath.Join(left, "b.txt")}) {
		t.Fatalf("expected b.txt to be deselected by shift+up inversion")
	}
}

func TestHandlePanelKeyEscClearsMarkedEntries(t *testing.T) {
	root := t.TempDir()
	left := filepath.Join(root, "left")
	right := filepath.Join(root, "right")
	if err := os.MkdirAll(left, 0o755); err != nil {
		t.Fatalf("create left dir: %v", err)
	}
	if err := os.MkdirAll(right, 0o755); err != nil {
		t.Fatalf("create right dir: %v", err)
	}
	for _, name := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(filepath.Join(left, name), []byte(name), 0o644); err != nil {
			t.Fatalf("write file %q: %v", name, err)
		}
	}

	m := &xdfileModel{
		activePanel: 0,
		layout: xdfileLayout{
			panelRects: [2]xdfileRect{{h: 12}, {h: 12}},
		},
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: left, RangeAnchor: -1},
			{Label: "RIGHT", Cwd: right, RangeAnchor: -1},
		},
	}
	for i := range m.panels {
		if err := m.reloadPanel(i); err != nil {
			t.Fatalf("reload panel %d: %v", i, err)
		}
	}

	m.panels[0].setCursor(findXdfileEntryIndex(t, m.panels[0].Entries, "a.txt"), 8)
	_ = m.handlePanelKey(tea.KeyMsg{Type: tea.KeyShiftDown})
	_ = m.handlePanelKey(tea.KeyMsg{Type: tea.KeyEsc})

	if got := m.panels[0].markedCount(); got != 0 {
		t.Fatalf("expected esc to clear marked entries, got %d", got)
	}
}

func TestHandlePanelKeyShiftLeftAndRightToggleBoundaries(t *testing.T) {
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
			t.Fatalf("write file %q: %v", name, err)
		}
	}

	m := &xdfileModel{
		activePanel: 0,
		layout: xdfileLayout{
			panelRects: [2]xdfileRect{{h: 12}, {h: 12}},
		},
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: left, RangeAnchor: -1},
			{Label: "RIGHT", Cwd: right, RangeAnchor: -1},
		},
	}
	for i := range m.panels {
		if err := m.reloadPanel(i); err != nil {
			t.Fatalf("reload panel %d: %v", i, err)
		}
	}

	m.panels[0].setCursor(findXdfileEntryIndex(t, m.panels[0].Entries, "b.txt"), 8)
	_ = m.handlePanelKey(tea.KeyMsg{Type: tea.KeyShiftLeft})
	if got := m.panels[0].markedCount(); got != 2 {
		t.Fatalf("expected shift+left to toggle current and above entries on, got %d", got)
	}
	if !m.panels[0].isMarked(xdfileEntry{Name: "a.txt", Path: filepath.Join(left, "a.txt")}) {
		t.Fatalf("expected a.txt to be selected by shift+left")
	}
	if entry, ok := m.panels[0].selected(); !ok || entry.Name != "a.txt" {
		t.Fatalf("expected cursor to move to a.txt after shift+left, got %+v", entry)
	}
	_ = m.handlePanelKey(tea.KeyMsg{Type: tea.KeyShiftLeft})
	if got := m.panels[0].markedCount(); got != 0 {
		t.Fatalf("expected second shift+left to invert the same range off, got %d", got)
	}
	if entry, ok := m.panels[0].selected(); !ok || entry.Name != "a.txt" {
		t.Fatalf("expected cursor to remain on a.txt after repeated shift+left, got %+v", entry)
	}

	m.panels[0].clearMarked()
	m.panels[0].setCursor(findXdfileEntryIndex(t, m.panels[0].Entries, "b.txt"), 8)
	_ = m.handlePanelKey(tea.KeyMsg{Type: tea.KeyShiftRight})
	if got := m.panels[0].markedCount(); got != 2 {
		t.Fatalf("expected shift+right to toggle current and below entries on, got %d", got)
	}
	if !m.panels[0].isMarked(xdfileEntry{Name: "c.txt", Path: filepath.Join(left, "c.txt")}) {
		t.Fatalf("expected c.txt to be selected by shift+right")
	}
	if entry, ok := m.panels[0].selected(); !ok || entry.Name != "c.txt" {
		t.Fatalf("expected cursor to move to c.txt after shift+right, got %+v", entry)
	}
	_ = m.handlePanelKey(tea.KeyMsg{Type: tea.KeyShiftRight})
	if got := m.panels[0].markedCount(); got != 0 {
		t.Fatalf("expected second shift+right to invert the same range off, got %d", got)
	}
	if entry, ok := m.panels[0].selected(); !ok || entry.Name != "c.txt" {
		t.Fatalf("expected cursor to remain on c.txt after repeated shift+right, got %+v", entry)
	}
}

func TestHandleGlobalKeyCtrlOTogglesTerminalExpandedView(t *testing.T) {
	m := &xdfileModel{
		width:       120,
		height:      36,
		activePanel: 1,
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: `C:\left`},
			{Label: "RIGHT", Cwd: `D:\right`},
		},
		terminal: xdfileTerminal{
			Cwd: `C:\left`,
		},
	}
	m.computeLayout()

	cmd, handled := m.handleGlobalKey(tea.KeyMsg{Type: tea.KeyCtrlO})
	if !handled {
		t.Fatal("expected ctrl+o to be handled")
	}
	if cmd == nil {
		t.Fatal("expected ctrl+o to return a terminal expanded view command")
	}
	if m.userScreenVisible {
		t.Fatal("expected ctrl+o to keep the real user screen hidden")
	}
	if !m.terminalExpandedViewActive() {
		t.Fatal("expected ctrl+o to expand the terminal inside the panel area")
	}
	if got, want := m.terminalRenderRect(), m.layout.exclusiveRect; got != want {
		t.Fatalf("expected expanded terminal to use the exclusive panel rect, got %+v want %+v", got, want)
	}

	cmd, handled = m.handleGlobalKey(tea.KeyMsg{Type: tea.KeyCtrlO})
	if !handled {
		t.Fatal("expected second ctrl+o to be handled")
	}
	if cmd != nil {
		t.Fatal("expected second ctrl+o to restore inline without a command")
	}
	if m.terminalExpandedViewActive() {
		t.Fatal("expected second ctrl+o to restore the normal panel layout")
	}
}

func TestHandleGlobalKeyCtrlTIsUnused(t *testing.T) {
	m := &xdfileModel{
		activePanel: 0,
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: `C:\left`},
			{Label: "RIGHT", Cwd: `D:\right`},
		},
	}

	cmd, handled := m.handleGlobalKey(tea.KeyMsg{Type: tea.KeyCtrlT})
	if handled || cmd != nil {
		t.Fatalf("expected ctrl+t to be unused, handled=%v cmd=%v", handled, cmd)
	}

	m.terminalFocused = true
	m.terminal = xdfileTerminal{
		Session:  &xdfileTerminalPTYSession{},
		Emulator: vt.NewSafeEmulator(80, 24),
	}
	cmd, handled = m.handleGlobalKey(tea.KeyMsg{Type: tea.KeyCtrlT})
	if handled || cmd != nil {
		t.Fatalf("expected ctrl+t to pass through while PTY is focused, handled=%v cmd=%v", handled, cmd)
	}
}

func TestUpdateCtrlORestoresUserScreen(t *testing.T) {
	m := &xdfileModel{
		activePanel:       1,
		userScreenVisible: true,
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: `C:\left`},
			{Label: "RIGHT", Cwd: `D:\right`},
		},
		terminal: xdfileTerminal{
			Cwd: `C:\left`,
		},
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	got := updated.(*xdfileModel)
	if cmd == nil {
		t.Fatal("expected ctrl+o while user screen is visible to return a restore command")
	}
	if !got.userScreenVisible {
		t.Fatal("expected user screen mode to stay visible until restore completes")
	}

	updated, cmd = got.Update(xdfileReturnFromUserScreenMsg{})
	got = updated.(*xdfileModel)
	if got.userScreenVisible {
		t.Fatal("expected restore message to leave user screen mode")
	}
	if got.terminal.Cwd != `D:\right` {
		t.Fatalf("expected restore message to resync terminal cwd to active panel, got %q", got.terminal.Cwd)
	}
	if cmd == nil {
		t.Fatal("expected restore message to request a screen clear")
	}
}

func TestHandlePanelKeyLeftAndRightMoveByPageWithoutSwitchingPanels(t *testing.T) {
	root := t.TempDir()
	left := filepath.Join(root, "left")
	right := filepath.Join(root, "right")
	if err := os.MkdirAll(left, 0o755); err != nil {
		t.Fatalf("create left dir: %v", err)
	}
	if err := os.MkdirAll(right, 0o755); err != nil {
		t.Fatalf("create right dir: %v", err)
	}
	for i := 1; i <= 10; i++ {
		name := filepath.Join(left, fmt.Sprintf("%02d.txt", i))
		if err := os.WriteFile(name, []byte(name), 0o644); err != nil {
			t.Fatalf("write file %q: %v", name, err)
		}
	}

	m := &xdfileModel{
		activePanel: 0,
		layout: xdfileLayout{
			panelRects: [2]xdfileRect{{h: 12}, {h: 12}},
		},
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: left, RangeAnchor: -1},
			{Label: "RIGHT", Cwd: right, RangeAnchor: -1},
		},
	}
	for i := range m.panels {
		if err := m.reloadPanel(i); err != nil {
			t.Fatalf("reload panel %d: %v", i, err)
		}
	}

	rows := m.panels[0].visibleRows(m.layout.panelRects[0].h)
	m.panels[0].setCursor(findXdfileEntryIndex(t, m.panels[0].Entries, "01.txt"), rows)

	if cmd := m.handlePanelKey(tea.KeyMsg{Type: tea.KeyRight}); cmd != nil {
		t.Fatalf("expected right key paging to stay inside the active panel")
	}
	if m.activePanel != 0 {
		t.Fatalf("expected right key to keep the left panel active, got %d", m.activePanel)
	}
	if entry, ok := m.panels[0].selected(); !ok || entry.Name != "09.txt" {
		t.Fatalf("expected right key to page down to 09.txt, got %+v", entry)
	}

	if cmd := m.handlePanelKey(tea.KeyMsg{Type: tea.KeyLeft}); cmd != nil {
		t.Fatalf("expected left key paging to stay inside the active panel")
	}
	if m.activePanel != 0 {
		t.Fatalf("expected left key to keep the left panel active, got %d", m.activePanel)
	}
	if entry, ok := m.panels[0].selected(); !ok || entry.Name != "01.txt" {
		t.Fatalf("expected left key to page up back to 01.txt, got %+v", entry)
	}
}
