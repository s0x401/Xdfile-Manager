package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	vt "github.com/charmbracelet/x/vt"
)

func TestExecuteCommandMenuIndexExpandsCurrentFileName(t *testing.T) {
	originalExecuteCommandMenu := xdfileExecuteCommandMenuFunc
	defer func() {
		xdfileExecuteCommandMenuFunc = originalExecuteCommandMenu
	}()

	dir := t.TempDir()
	filePath := filepath.Join(dir, "demo.py")
	if err := os.WriteFile(filePath, []byte("print('hi')"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	var gotCommand string
	xdfileExecuteCommandMenuFunc = func(m *xdfileModel, command string) tea.Cmd {
		gotCommand = command
		return nil
	}

	m := &xdfileModel{
		activePanel: 0,
		openMenu:    xdfileActionCommandsMenu,
		panels: [2]xdfilePanel{
			{
				Label: "LEFT",
				Cwd:   dir,
				Entries: []xdfileEntry{
					{Name: "demo.py", Path: filePath},
				},
			},
		},
		layoutPrefs: xdfileLayoutPrefs{
			Commands: []xdfileCommandItem{
				{Type: xdfileCommandItemTypeCommand, Hotkey: "f3", Label: "Run Python", Command: `xxx\python.exe "!.!"`},
			},
		},
	}

	if cmd := m.executeCommandMenuIndex(0); cmd != nil {
		t.Fatalf("expected command execution to complete immediately")
	}
	if gotCommand != `xxx\python.exe "demo.py"` {
		t.Fatalf("expected expanded command, got %q", gotCommand)
	}
	if m.openMenu != "" {
		t.Fatalf("expected command menu to close after running a command, got %q", m.openMenu)
	}
}

func TestDeleteSelectedCommandMenuItemRemovesNestedItemAndPersists(t *testing.T) {
	dir := t.TempDir()
	commandsFile := filepath.Join(dir, "xdfile-commands.json")

	initial := []xdfileCommandItem{
		{
			Type:   xdfileCommandItemTypeMenu,
			Hotkey: "m",
			Label:  "Tools",
			Items: []xdfileCommandItem{
				{Type: xdfileCommandItemTypeCommand, Hotkey: "f3", Label: "Run Python", Command: `python "!.!"`},
				{Type: xdfileCommandItemTypeCommand, Hotkey: "f4", Label: "Open Tool", Command: `tool.exe "!.!"`},
			},
		},
	}
	if err := xdfileSaveCommandPrefs(commandsFile, initial); err != nil {
		t.Fatalf("save initial user menu prefs: %v", err)
	}

	m := &xdfileModel{
		commandsFile:      commandsFile,
		layoutPrefs:       xdfileLayoutPrefs{Commands: initial},
		openMenu:          xdfileActionCommandsMenu,
		commandMenuPath:   []int{0},
		commandInsertPath: nil,
		menuCursor:        0,
	}

	if cmd := m.deleteSelectedCommandMenuItem(); cmd != nil {
		t.Fatalf("expected delete to complete immediately")
	}
	if len(m.layoutPrefs.Commands[0].Items) != 1 {
		t.Fatalf("expected one nested command after delete, got %d", len(m.layoutPrefs.Commands[0].Items))
	}
	if m.layoutPrefs.Commands[0].Items[0].Label != "Open Tool" {
		t.Fatalf("expected remaining nested command to be Open Tool, got %+v", m.layoutPrefs.Commands[0].Items[0])
	}

	saved, exists, err := xdfileLoadCommandPrefs(commandsFile)
	if err != nil {
		t.Fatalf("load saved user menu prefs: %v", err)
	}
	if !exists {
		t.Fatalf("expected user menu config file to exist")
	}
	if len(saved) != 1 || len(saved[0].Items) != 1 || saved[0].Items[0].Label != "Open Tool" {
		t.Fatalf("expected saved nested commands to match deletion, got %+v", saved)
	}
}

func TestInsertCommandMenuItemInsertsNestedItemAndPersists(t *testing.T) {
	dir := t.TempDir()
	commandsFile := filepath.Join(dir, "xdfile-commands.json")

	initial := []xdfileCommandItem{
		{
			Type:   xdfileCommandItemTypeMenu,
			Hotkey: "m",
			Label:  "Tools",
			Items: []xdfileCommandItem{
				{Type: xdfileCommandItemTypeCommand, Hotkey: "f3", Label: "Run Python", Command: `python "!.!"`},
				{Type: xdfileCommandItemTypeCommand, Hotkey: "f4", Label: "Open Tool", Command: `tool.exe "!.!"`},
			},
		},
	}
	if err := xdfileSaveCommandPrefs(commandsFile, initial); err != nil {
		t.Fatalf("save initial user menu prefs: %v", err)
	}

	m := &xdfileModel{
		commandsFile:       commandsFile,
		layoutPrefs:        xdfileLayoutPrefs{Commands: initial},
		openMenu:           xdfileActionCommandsMenu,
		commandInsertPath:  []int{0},
		commandInsertIndex: 1,
	}

	item := xdfileCommandItem{
		Type:    xdfileCommandItemTypeCommand,
		Hotkey:  "f5",
		Label:   "New Tool",
		Command: `newtool.exe "!.!"`,
	}
	if cmd := m.insertCommandMenuItem(item); cmd != nil {
		t.Fatalf("expected insert to complete immediately")
	}
	if got := len(m.layoutPrefs.Commands[0].Items); got != 3 {
		t.Fatalf("expected three nested commands after insert, got %d", got)
	}
	labels := []string{
		m.layoutPrefs.Commands[0].Items[0].Label,
		m.layoutPrefs.Commands[0].Items[1].Label,
		m.layoutPrefs.Commands[0].Items[2].Label,
	}
	want := []string{"Run Python", "New Tool", "Open Tool"}
	for i := range want {
		if labels[i] != want[i] {
			t.Fatalf("expected inserted item order %v, got %v", want, labels)
		}
	}

	saved, exists, err := xdfileLoadCommandPrefs(commandsFile)
	if err != nil {
		t.Fatalf("load saved user menu prefs: %v", err)
	}
	if !exists {
		t.Fatalf("expected user menu config file to exist")
	}
	if len(saved) != 1 || len(saved[0].Items) != 3 || saved[0].Items[1].Label != "New Tool" {
		t.Fatalf("expected saved nested commands to match insertion, got %+v", saved)
	}
}

func TestHandleGlobalKeyF2OpensCommandMenuWhilePTYFocused(t *testing.T) {
	m := &xdfileModel{
		terminalFocused: true,
		terminal: xdfileTerminal{
			Session:  &xdfileTerminalPTYSession{},
			Emulator: vt.NewSafeEmulator(80, 24),
		},
		layoutPrefs: xdfileLayoutPrefs{
			Commands: []xdfileCommandItem{
				{Type: xdfileCommandItemTypeCommand, Hotkey: "f3", Label: "Run Python", Command: `python "!.!"`},
			},
		},
	}

	cmd, handled := m.handleGlobalKey(tea.KeyMsg{Type: tea.KeyF2})
	if !handled {
		t.Fatalf("expected F2 to be handled while PTY terminal is focused")
	}
	if cmd != nil {
		if msg := cmd(); msg != nil {
			t.Fatalf("expected F2 command menu open to complete immediately, got %T", msg)
		}
	}
	if m.openMenu != xdfileActionCommandsMenu {
		t.Fatalf("expected command menu to open, got %q", m.openMenu)
	}
	if len(m.commandMenuPath) != 0 {
		t.Fatalf("expected F2 to reset nested menu path, got %+v", m.commandMenuPath)
	}
}

func TestHandleMenuKeyInsertBeginsChoiceModal(t *testing.T) {
	m := &xdfileModel{
		openMenu: xdfileActionCommandsMenu,
		modal: xdfileModal{
			Input: textinput.New(),
		},
		layoutPrefs: xdfileLayoutPrefs{
			Commands: []xdfileCommandItem{
				{Type: xdfileCommandItemTypeCommand, Hotkey: "f3", Label: "Run Python", Command: `python "!.!"`},
			},
		},
	}

	cmd, handled := m.handleMenuKey(tea.KeyMsg{Type: tea.KeyInsert})
	if !handled {
		t.Fatalf("expected Insert to be handled in command menu")
	}
	if cmd != nil {
		t.Fatalf("expected Insert to open the choice modal immediately")
	}
	if m.modal.Kind != xdfileModalChoice {
		t.Fatalf("expected Insert to open choice modal, got kind %v", m.modal.Kind)
	}
	if len(m.modal.ChoiceItems) != 2 {
		t.Fatalf("expected Insert choice modal to contain command and menu options, got %d", len(m.modal.ChoiceItems))
	}
	if m.modal.ChoiceItems[0].Action != xdfileActionInsertCommand || m.modal.ChoiceItems[1].Action != xdfileActionInsertMenu {
		t.Fatalf("expected choice modal to offer insert command/menu actions, got %+v", m.modal.ChoiceItems)
	}
}

func TestHandleMenuKeyInsertUsesHoveredCommandMenuItem(t *testing.T) {
	m := &xdfileModel{
		openMenu:   xdfileActionCommandsMenu,
		menuCursor: 0,
		hover: xdfileHoverState{
			MenuItem: 1,
		},
		modal: xdfileModal{
			Input: textinput.New(),
		},
		layoutPrefs: xdfileLayoutPrefs{
			Commands: []xdfileCommandItem{
				{Type: xdfileCommandItemTypeCommand, Hotkey: "a", Label: "First", Command: "one"},
				{Type: xdfileCommandItemTypeCommand, Hotkey: "b", Label: "Second", Command: "two"},
			},
		},
	}

	cmd, handled := m.handleMenuKey(tea.KeyMsg{Type: tea.KeyInsert})
	if !handled {
		t.Fatalf("expected Insert to be handled in command menu")
	}
	if cmd != nil {
		t.Fatalf("expected Insert to open the choice modal immediately")
	}
	if m.commandInsertIndex != 2 {
		t.Fatalf("expected Insert to target below hovered item index 1, got %d", m.commandInsertIndex)
	}
}

func TestExecuteActionInsertCommandOpensOneShotForm(t *testing.T) {
	m := &xdfileModel{
		modal: xdfileModal{
			Input: textinput.New(),
		},
	}

	if cmd := m.executeAction(xdfileActionInsertCommand); cmd != nil {
		t.Fatalf("expected insert-command action to open modal immediately")
	}
	if m.modal.Kind != xdfileModalForm {
		t.Fatalf("expected insert-command action to open form modal, got kind %v", m.modal.Kind)
	}
	if m.modal.Action != xdfileActionInsertCommand {
		t.Fatalf("expected insert-command form action, got %q", m.modal.Action)
	}
	if len(m.modal.FormFields) != 3 {
		t.Fatalf("expected insert-command form to have 3 fields, got %d", len(m.modal.FormFields))
	}
	if !m.modal.FormFields[2].Multiline {
		t.Fatal("expected command field to use multiline editing")
	}
}

func TestHandleMenuKeyF4BeginsEditForm(t *testing.T) {
	m := &xdfileModel{
		openMenu: xdfileActionCommandsMenu,
		modal: xdfileModal{
			Input: textinput.New(),
		},
		layoutPrefs: xdfileLayoutPrefs{
			Commands: []xdfileCommandItem{
				{Type: xdfileCommandItemTypeCommand, Hotkey: "f3", Label: "Run Python", Command: `python "!.!"`},
			},
		},
	}

	cmd, handled := m.handleMenuKey(tea.KeyMsg{Type: tea.KeyF4})
	if !handled {
		t.Fatalf("expected F4 to be handled in command menu")
	}
	if cmd != nil {
		t.Fatalf("expected F4 edit to open the form immediately")
	}
	if m.modal.Kind != xdfileModalForm {
		t.Fatalf("expected F4 to open edit form, got kind %v", m.modal.Kind)
	}
	if m.modal.Action != xdfileActionEditCommand {
		t.Fatalf("expected edit-command form action, got %q", m.modal.Action)
	}
	if got := m.modal.FormFields[0].Input.Value(); got != "f3" {
		t.Fatalf("expected hot key to be prefilled, got %q", got)
	}
	if got := m.modal.FormFields[1].Input.Value(); got != "Run Python" {
		t.Fatalf("expected label to be prefilled, got %q", got)
	}
	if got := m.modal.FormFields[2].Input.Value(); got != `python "!.!"` {
		t.Fatalf("expected command to be prefilled, got %q", got)
	}
	if !m.modal.FormFields[2].Multiline {
		t.Fatal("expected edit-command form to use multiline command editing")
	}
}

func TestHandleMenuKeyF4UsesHoveredCommandMenuItem(t *testing.T) {
	m := &xdfileModel{
		openMenu:   xdfileActionCommandsMenu,
		menuCursor: 0,
		hover: xdfileHoverState{
			MenuItem: 1,
		},
		modal: xdfileModal{
			Input: textinput.New(),
		},
		layoutPrefs: xdfileLayoutPrefs{
			Commands: []xdfileCommandItem{
				{Type: xdfileCommandItemTypeCommand, Hotkey: "f3", Label: "Run Python", Command: `python "!.!"`},
				{Type: xdfileCommandItemTypeCommand, Hotkey: "c", Label: "Check", Command: `cobra "!.!"`},
			},
		},
	}

	cmd, handled := m.handleMenuKey(tea.KeyMsg{Type: tea.KeyF4})
	if !handled {
		t.Fatalf("expected F4 to be handled in command menu")
	}
	if cmd != nil {
		t.Fatalf("expected F4 edit to open the form immediately")
	}
	if m.modal.Kind != xdfileModalForm {
		t.Fatalf("expected F4 to open edit form, got kind %v", m.modal.Kind)
	}
	if got := m.modal.FormFields[0].Input.Value(); got != "c" {
		t.Fatalf("expected hovered item hot key to be prefilled, got %q", got)
	}
	if got := m.modal.FormFields[1].Input.Value(); got != "Check" {
		t.Fatalf("expected hovered item label to be prefilled, got %q", got)
	}
	if got := m.modal.FormFields[2].Input.Value(); got != `cobra "!.!"` {
		t.Fatalf("expected hovered item command to be prefilled, got %q", got)
	}
}

func TestHandleModalFormShiftEnterAddsNewlineToCommandField(t *testing.T) {
	m := &xdfileModel{
		modal: xdfileModal{
			Input: textinput.New(),
		},
	}
	if cmd := m.executeAction(xdfileActionInsertCommand); cmd != nil {
		t.Fatalf("expected insert-command action to open modal immediately")
	}
	m.focusModalFormField(2)
	m.modal.FormFields[2].TextArea.SetValue("echo first")
	m.modal.FormFields[2].TextArea.CursorEnd()
	m.modal.FormFields[2].syncMirrorInput()

	if cmd := m.handleModalKey(tea.KeyMsg{Type: tea.KeyShiftEnter}); cmd != nil {
		t.Fatalf("expected shift+enter to insert a newline without closing the form")
	}
	if got := m.modal.FormFields[2].Value(); got != "echo first\n" {
		t.Fatalf("expected shift+enter to append a newline, got %q", got)
	}
	if m.modal.Kind != xdfileModalForm {
		t.Fatalf("expected form modal to stay open after shift+enter, got kind %v", m.modal.Kind)
	}
}

func TestApplyModalInsertMenuAndNestedCommand(t *testing.T) {
	dir := t.TempDir()
	commandsFile := filepath.Join(dir, "xdfile-commands.json")
	m := &xdfileModel{
		commandsFile: commandsFile,
		modal: xdfileModal{
			Input: textinput.New(),
		},
		layoutPrefs: xdfileLayoutPrefs{},
	}

	m.commandInsertPath = nil
	m.commandInsertIndex = 0
	m.openFormModal(
		xdfileActionInsertMenu,
		"Insert User Menu",
		"",
		[]xdfileModalField{
			m.newModalFormField("Hot key", "", "m"),
			m.newModalFormField("Label", "", "Tools"),
		},
	)
	if cmd := m.applyModal(); cmd != nil {
		t.Fatalf("expected submenu insert to complete immediately")
	}
	if len(m.layoutPrefs.Commands) != 1 || !m.layoutPrefs.Commands[0].isMenu() || m.layoutPrefs.Commands[0].Label != "Tools" {
		t.Fatalf("expected submenu to be inserted at root, got %+v", m.layoutPrefs.Commands)
	}

	m.commandInsertPath = []int{0}
	m.commandInsertIndex = 0
	m.openFormModal(
		xdfileActionInsertCommand,
		"Insert User Command",
		"",
		[]xdfileModalField{
			m.newModalFormField("Hot key", "", "r"),
			m.newModalFormField("Label", "", "Run tool"),
			m.newModalFormField("Command", "", `tool.exe "!.!"`),
		},
	)
	if cmd := m.applyModal(); cmd != nil {
		t.Fatalf("expected nested command insert to complete immediately")
	}
	if len(m.layoutPrefs.Commands[0].Items) != 1 {
		t.Fatalf("expected nested command to be inserted, got %+v", m.layoutPrefs.Commands[0].Items)
	}
	if got := m.layoutPrefs.Commands[0].Items[0]; got.Label != "Run tool" || got.Command != `tool.exe "!.!"` {
		t.Fatalf("expected inserted nested command, got %+v", got)
	}

	saved, exists, err := xdfileLoadCommandPrefs(commandsFile)
	if err != nil {
		t.Fatalf("load saved user menu prefs: %v", err)
	}
	if !exists {
		t.Fatalf("expected user menu config file to exist")
	}
	if len(saved) != 1 || len(saved[0].Items) != 1 {
		t.Fatalf("expected saved tree structure, got %+v", saved)
	}
}

func TestApplyModalEditCommandPersists(t *testing.T) {
	dir := t.TempDir()
	commandsFile := filepath.Join(dir, "xdfile-commands.json")
	initial := []xdfileCommandItem{
		{Type: xdfileCommandItemTypeCommand, Hotkey: "f3", Label: "Run Python", Command: `python "!.!"`},
	}
	if err := xdfileSaveCommandPrefs(commandsFile, initial); err != nil {
		t.Fatalf("save initial user menu prefs: %v", err)
	}

	m := &xdfileModel{
		commandsFile: commandsFile,
		modal: xdfileModal{
			Input: textinput.New(),
		},
		layoutPrefs: xdfileLayoutPrefs{
			Commands: initial,
		},
		openMenu: xdfileActionCommandsMenu,
	}

	m.commandEditPath = nil
	m.commandEditIndex = 0
	m.openFormModal(
		xdfileActionEditCommand,
		"Edit User Command",
		"",
		[]xdfileModalField{
			m.newModalFormField("Hot key", "", "f4"),
			m.newModalFormField("Label", "", "Run Tool"),
			m.newModalFormField("Command", "", `tool.exe "!.!"`),
		},
	)
	if cmd := m.applyModal(); cmd != nil {
		t.Fatalf("expected command edit to complete immediately")
	}

	if got := m.layoutPrefs.Commands[0]; got.Hotkey != "f4" || got.Label != "Run Tool" || got.Command != `tool.exe "!.!"` {
		t.Fatalf("expected edited command to be persisted in memory, got %+v", got)
	}

	saved, exists, err := xdfileLoadCommandPrefs(commandsFile)
	if err != nil {
		t.Fatalf("load saved user menu prefs: %v", err)
	}
	if !exists {
		t.Fatalf("expected user menu config file to exist")
	}
	if got := saved[0]; got.Hotkey != "f4" || got.Label != "Run Tool" || got.Command != `tool.exe "!.!"` {
		t.Fatalf("expected edited command to be saved, got %+v", got)
	}
}

func TestApplyModalInsertCommandPreservesMultilineCommand(t *testing.T) {
	dir := t.TempDir()
	commandsFile := filepath.Join(dir, "xdfile-commands.json")
	m := &xdfileModel{
		commandsFile: commandsFile,
		modal: xdfileModal{
			Input: textinput.New(),
		},
	}

	m.openFormModal(
		xdfileActionInsertCommand,
		"Insert User Command",
		"",
		[]xdfileModalField{
			m.newModalFormField("Hot key", "", "r"),
			m.newModalFormField("Label", "", "Run tool"),
			m.newModalFormMultilineField("Command", "", "echo first\ncall second"),
		},
	)

	if cmd := m.applyModal(); cmd != nil {
		t.Fatalf("expected insert-command apply to complete immediately")
	}
	if len(m.layoutPrefs.Commands) != 1 {
		t.Fatalf("expected one inserted command, got %+v", m.layoutPrefs.Commands)
	}
	if got := m.layoutPrefs.Commands[0].Command; got != "echo first\ncall second" {
		t.Fatalf("expected multiline command to be preserved, got %q", got)
	}
}

func TestHandleModalMouseChoiceClickOpensSelectedInsertForm(t *testing.T) {
	m := &xdfileModel{
		width:  120,
		height: 40,
		modal: xdfileModal{
			Input: textinput.New(),
		},
	}

	m.openChoiceModal(
		"Insert Into User Menu",
		"Choose what to insert below the current item.",
		[]xdfileModalChoiceItem{
			{Action: xdfileActionInsertCommand, Label: "Command", Description: "Insert a runnable command."},
			{Action: xdfileActionInsertMenu, Label: "Menu", Description: "Insert a nested submenu."},
		},
	)

	rect := m.modalRect()
	innerW := max(1, rect.w-2)
	segmentWidth, gap := xdfileHorizontalModalChoiceGeometry(innerW, len(m.modal.ChoiceItems))
	cmd := m.handleModalMouse(tea.MouseMsg{
		X:      rect.x + 1 + segmentWidth + gap + 1,
		Y:      rect.y + 1 + 3 + len(xdfileWrapText(m.modal.Description, innerW)),
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	if cmd != nil {
		if msg := cmd(); msg != nil {
			t.Fatalf("expected choice click to complete immediately, got %T", msg)
		}
	}
	if m.modal.Kind != xdfileModalForm {
		t.Fatalf("expected choice click to open a form modal, got kind %v", m.modal.Kind)
	}
	if m.modal.Action != xdfileActionInsertMenu {
		t.Fatalf("expected second choice click to open the menu insert form, got %q", m.modal.Action)
	}
}

func TestHandleModalMouseFormClickFocusesRequestedField(t *testing.T) {
	m := &xdfileModel{
		width:  120,
		height: 40,
		modal: xdfileModal{
			Input: textinput.New(),
		},
	}

	m.openFormModal(
		xdfileActionInsertCommand,
		"Insert User Command",
		"Fill in the command properties.",
		[]xdfileModalField{
			m.newModalFormField("Hot key", "", ""),
			m.newModalFormField("Label", "", ""),
			m.newModalFormField("Command", "", ""),
		},
	)

	rect := m.modalRect()
	m.handleModalMouse(tea.MouseMsg{
		X:      rect.x + 4,
		Y:      rect.y + 9,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})

	if m.modal.FormCursor != 1 {
		t.Fatalf("expected mouse click to focus the second form field, got %d", m.modal.FormCursor)
	}
	if !m.modal.FormFields[1].Input.Focused() {
		t.Fatalf("expected second form field to receive focus")
	}
	if m.modal.FormFields[0].Input.Focused() {
		t.Fatalf("expected previous form field to lose focus")
	}
}

func TestHandleMenuKeyHotkeyOpensNestedSubmenu(t *testing.T) {
	m := &xdfileModel{
		openMenu: xdfileActionCommandsMenu,
		layoutPrefs: xdfileLayoutPrefs{
			Commands: []xdfileCommandItem{
				{
					Type:   xdfileCommandItemTypeMenu,
					Hotkey: "m",
					Label:  "Tools",
					Items: []xdfileCommandItem{
						{Type: xdfileCommandItemTypeCommand, Hotkey: "r", Label: "Run", Command: "run.cmd"},
					},
				},
			},
		},
	}

	cmd, handled := m.handleMenuKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	if !handled {
		t.Fatalf("expected submenu hotkey to be handled")
	}
	if cmd != nil {
		if msg := cmd(); msg != nil {
			t.Fatalf("expected submenu open to complete immediately, got %T", msg)
		}
	}
	if len(m.commandMenuPath) != 1 || m.commandMenuPath[0] != 0 {
		t.Fatalf("expected submenu hotkey to open nested path, got %+v", m.commandMenuPath)
	}
	if got := m.commandMenuPathLabel(); got != "Tools" {
		t.Fatalf("expected nested submenu label, got %q", got)
	}
}

func TestOpenCommandMenuSwallowsUnmatchedTextInput(t *testing.T) {
	m := &xdfileModel{
		width:       120,
		height:      40,
		activePanel: 0,
		openMenu:    xdfileActionCommandsMenu,
		layoutPrefs: xdfileLayoutPrefs{
			Commands: []xdfileCommandItem{
				{Type: xdfileCommandItemTypeCommand, Hotkey: "r", Label: "Run", Command: "run.cmd"},
			},
		}.normalized(),
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: `C:\left`},
			{Label: "RIGHT", Cwd: `D:\right`},
		},
		terminal: xdfileTerminal{
			Input: xdfileNewManagedTerminalInput(),
		},
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	got := updated.(*xdfileModel)
	if cmd != nil {
		t.Fatalf("expected unmatched menu key to complete immediately")
	}
	if got.openMenu != xdfileActionCommandsMenu {
		t.Fatalf("expected F2 menu to stay open, got %q", got.openMenu)
	}
	if value := got.terminal.Input.Value(); value != "" {
		t.Fatalf("expected unmatched menu key not to enter the command line, got %q", value)
	}
	if !strings.Contains(got.statusText, "No menu shortcut: q") {
		t.Fatalf("expected top status to explain unmatched menu key, got %q", got.statusText)
	}
}

func TestExecuteCommandMenuCommandPTYStartsBusyTracking(t *testing.T) {
	emulator := vt.NewSafeEmulator(80, 24)
	if _, err := emulator.Write([]byte("PS C:\\work> ")); err != nil {
		t.Fatalf("seed emulator prompt: %v", err)
	}
	backend := &testXdfilePTYBackend{}
	session := &xdfileTerminalPTYSession{
		backend:  backend,
		emulator: emulator,
	}
	go session.runInputLoop()
	defer session.Close()

	m := &xdfileModel{
		activePanel: 0,
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: `C:\work`},
			{Label: "RIGHT", Cwd: `C:\right`},
		},
		terminal: xdfileTerminal{
			Cwd:      `C:\work`,
			Session:  session,
			Emulator: emulator,
		},
	}

	command := "Invoke-WebRequest https://example.invalid/file.zip"
	cmd := m.executeCommandMenuCommand(command)
	if cmd == nil {
		t.Fatalf("expected PTY command menu execution to start polling")
	}
	if !m.terminal.Busy {
		t.Fatalf("expected PTY command menu execution to mark terminal busy")
	}
	if len(m.terminal.History) != 1 || m.terminal.History[0] != command {
		t.Fatalf("expected command menu PTY execution to push terminal history, got %+v", m.terminal.History)
	}
}

func TestTerminalCommandDoneKeepsPanelsVisibleAfterCommandMenuRun(t *testing.T) {
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
		width:       120,
		height:      40,
		layoutPrefs: xdfileDefaultLayoutPrefs(),
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: left},
			{Label: "RIGHT", Cwd: right},
		},
		terminal: xdfileTerminal{
			Busy:         true,
			PendingPanel: -1,
		},
	}

	model, cmd := m.Update(xdfileTerminalCommandDoneMsg{Cwd: left})
	if cmd != nil {
		t.Fatalf("expected command completion update to finish immediately")
	}

	got, ok := model.(*xdfileModel)
	if !ok {
		t.Fatalf("expected updated model type *xdfileModel, got %T", model)
	}
	if got.terminal.Busy {
		t.Fatalf("expected terminal busy state to clear after command completion")
	}
	rendered := stripXdfileANSI(got.View())
	if !strings.Contains(rendered, "LEFT") || !strings.Contains(rendered, "RIGHT") {
		t.Fatalf("expected command completion to keep both panels visible, got %q", rendered)
	}
}

func TestStreamingPTYCommandCompletionAppendsEmulatorOutput(t *testing.T) {
	root := t.TempDir()
	left := filepath.Join(root, "left")
	right := filepath.Join(root, "right")
	if err := os.MkdirAll(left, 0o755); err != nil {
		t.Fatalf("create left dir: %v", err)
	}
	if err := os.MkdirAll(right, 0o755); err != nil {
		t.Fatalf("create right dir: %v", err)
	}

	emulator := vt.NewSafeEmulator(40, 4)
	emulator.SetScrollbackSize(32)
	if _, err := emulator.Write([]byte("\x1b[34mread data from clipboard!\x1b[0m\r\n\x1b[32mSuccess!\x1b[0m")); err != nil {
		t.Fatalf("seed streaming emulator output: %v", err)
	}

	m := &xdfileModel{
		width:       120,
		height:      40,
		layoutPrefs: xdfileDefaultLayoutPrefs(),
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: left},
			{Label: "RIGHT", Cwd: right},
		},
		terminal: xdfileTerminal{
			Busy:           true,
			Input:          textinput.New(),
			StreamEmulator: emulator,
			PendingPanel:   -1,
		},
	}

	model, cmd := m.Update(xdfileTerminalCommandDoneMsg{Cwd: left})
	if cmd != nil {
		t.Fatalf("expected streaming PTY command completion update to finish immediately")
	}

	got, ok := model.(*xdfileModel)
	if !ok {
		t.Fatalf("expected updated model type *xdfileModel, got %T", model)
	}
	if got.terminal.StreamEmulator != nil {
		t.Fatal("expected streaming PTY emulator to clear after command completion")
	}
	joined := strings.Join(got.terminal.Lines, "\n")
	if !strings.Contains(joined, "\x1b[34mread data from clipboard!") || !strings.Contains(joined, "\x1b[32mSuccess!") {
		t.Fatalf("expected streaming PTY output to be appended with ANSI styles preserved, got %q", joined)
	}
}

func TestExecuteCommandMenuCommandRunsInActivePanelDir(t *testing.T) {
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
		activePanel: 1,
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: left},
			{Label: "RIGHT", Cwd: right},
		},
		terminal: xdfileTerminal{
			Cwd:   left,
			Input: textinput.New(),
		},
	}

	cmd := m.executeCommandMenuCommand(`cmd /c echo hi`)
	if cmd == nil {
		t.Fatal("expected command menu execution to return a command")
	}
	startMsg := cmd()
	start, ok := startMsg.(xdfileTerminalCommandStartMsg)
	if !ok {
		t.Fatalf("expected command start message, got %T", startMsg)
	}
	defer start.Cancel()

	if start.Dir != right {
		t.Fatalf("expected command menu execution dir %q, got %q", right, start.Dir)
	}
	if m.terminal.Cwd != right {
		t.Fatalf("expected terminal cwd to sync to active panel %q, got %q", right, m.terminal.Cwd)
	}
}

func TestExecuteCommandMenuCommandAppliesSideEffectsInActivePanelDir(t *testing.T) {
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
		activePanel: 1,
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: left},
			{Label: "RIGHT", Cwd: right},
		},
		terminal: xdfileTerminal{
			Cwd:   left,
			Input: textinput.New(),
		},
	}

	cmd := m.executeCommandMenuCommand(`cmd /c echo hi> out.txt`)
	if cmd == nil {
		t.Fatal("expected command menu execution to return a command")
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

	for {
		select {
		case msg, ok := <-start.Events:
			if !ok {
				t.Fatal("expected command events before channel close")
			}
			updated, _ = got.Update(msg)
			got = updated.(*xdfileModel)
			if _, done := msg.(xdfileTerminalCommandDoneMsg); done {
				target := filepath.Join(right, "out.txt")
				data, err := os.ReadFile(target)
				if err != nil {
					t.Fatalf("expected side-effect file in active panel dir: %v", err)
				}
				if strings.TrimSpace(string(data)) != "hi" {
					t.Fatalf("expected side-effect file content %q, got %q", "hi", string(data))
				}
				if _, err := os.Stat(filepath.Join(left, "out.txt")); !os.IsNotExist(err) {
					t.Fatalf("expected no side-effect file in stale terminal dir, got err=%v", err)
				}
				return
			}
		case <-time.After(3 * time.Second):
			t.Fatal("timed out waiting for command side effects")
		}
	}
}
