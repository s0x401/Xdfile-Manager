package cmd

import (
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *xdfileModel) handleSortShortcut(msg tea.KeyMsg) (tea.Cmd, bool) {
	trigger := func(action xdfileAction) (tea.Cmd, bool) {
		return tea.Batch(m.noteFooterCtrlHint(), m.executeAction(action)), true
	}

	switch msg.Type {
	case tea.KeyCtrl3:
		return trigger(xdfileActionSortName)
	case tea.KeyCtrl4, tea.KeyCtrlBackslash:
		return trigger(xdfileActionSortExt)
	}

	switch msg.String() {
	case "ctrl+4", "ctrl+\\":
		return trigger(xdfileActionSortExt)
	case "ctrl+3":
		return trigger(xdfileActionSortName)
	default:
		return nil, false
	}
}

func (m *xdfileModel) handleGlobalKey(msg tea.KeyMsg) (tea.Cmd, bool) {
	switch msg.Type {
	case tea.KeyCtrlC:
		if m.cancelFileOperationIfBusy() {
			return nil, true
		}
		if m.cancelManagedCommandIfBusy() {
			return nil, true
		}
	}
	switch msg.String() {
	case "ctrl+c":
		if m.cancelFileOperationIfBusy() {
			return nil, true
		}
		if m.cancelManagedCommandIfBusy() {
			return nil, true
		}
	}

	if cmd, handled := m.handleFileClipboardShortcut(msg); handled {
		return cmd, true
	}
	if cmd, handled := m.handleSortShortcut(msg); handled {
		return cmd, true
	}
	if cmd, handled := m.handlePanelSearchShortcut(msg); handled {
		return cmd, true
	}

	if m.terminalFocused && m.terminalUsesPTY() {
		switch msg.String() {
		case "ctrl+o":
			return m.toggleTerminalExpandedView(), true
		case "f10":
			return m.openQuitConfirm(), true
		case "tab":
			return m.cycleFocusForward(), true
		case "shift+tab":
			return m.cycleFocusBackward(), true
		case "ctrl+left":
			return m.adjustPanelSplit(-2), true
		case "ctrl+right":
			return m.adjustPanelSplit(2), true
		case "ctrl+up":
			return m.adjustTerminalHeight(1), true
		case "ctrl+down":
			return m.adjustTerminalHeight(-1), true
		case "f2":
			if m.terminal.Emulator != nil && m.terminal.Emulator.IsAltScreen() {
				return nil, false
			}
			return m.executeAction(xdfileActionCommandsMenu), true
		}
		return nil, false
	}

	switch msg.String() {
	case "ctrl+o":
		return m.toggleTerminalExpandedView(), true
	case "f10":
		return m.openQuitConfirm(), true
	case "ctrl+left":
		return m.adjustPanelSplit(-2), true
	case "ctrl+right":
		return m.adjustPanelSplit(2), true
	case "ctrl+up":
		return m.adjustTerminalHeight(1), true
	case "ctrl+down":
		return m.adjustTerminalHeight(-1), true
	case "f1":
		m.openTextModal("Xdfile Manager Help", xdfileHelpText())
		return nil, true
	case "f2":
		return m.executeAction(xdfileActionCommandsMenu), true
	case "f3":
		return m.executeAction(xdfileActionPreview), true
	case "ctrl+q":
		if m.terminalFocused {
			return nil, false
		}
		return tea.Batch(m.noteFooterCtrlHint(), m.togglePreview()), true
	case "f4":
		return m.executeAction(xdfileActionRename), true
	case "f5":
		return m.executeAction(xdfileActionCopy), true
	case "f6":
		return m.executeAction(xdfileActionMove), true
	case "f7":
		return m.executeAction(xdfileActionMkdir), true
	case "f8":
		return m.executeAction(xdfileActionDelete), true
	case "f9":
		return m.executeAction(xdfileActionHidden), true
	case "ctrl+z":
		return m.executeAction(xdfileActionUndoDelete), true
	case "tab":
		return m.cycleFocusForward(), true
	case "shift+tab":
		return m.cycleFocusBackward(), true
	case "esc":
		if m.terminalFocused {
			return m.focusPanel(m.activePanel), true
		}
		return nil, false
	}
	return nil, false
}

func (m *xdfileModel) handleFileClipboardShortcut(msg tea.KeyMsg) (tea.Cmd, bool) {
	if msg.Paste && len(msg.Runes) > 0 {
		return nil, false
	}
	if m.terminalFocused && !m.terminalAutoFocused {
		if xdfileIsFilePasteShortcut(msg) && m.hasClipboardFilePayload() {
			return m.executeAction(xdfileActionPaste), true
		}
		return nil, false
	}

	switch msg.Type {
	case tea.KeyCtrlShiftC:
		return m.executeAction(xdfileActionClipboardCopy), true
	case tea.KeyCtrlX:
		return m.executeAction(xdfileActionClipboardCut), true
	case tea.KeyCtrlShiftV:
		return m.executeAction(xdfileActionPaste), true
	}

	switch msg.String() {
	case "ctrl+shift+c":
		return m.executeAction(xdfileActionClipboardCopy), true
	case "ctrl+x":
		return m.executeAction(xdfileActionClipboardCut), true
	case "ctrl+shift+v":
		return m.executeAction(xdfileActionPaste), true
	default:
		return nil, false
	}
}

func xdfileIsFilePasteShortcut(msg tea.KeyMsg) bool {
	if msg.Type == tea.KeyCtrlShiftV {
		return true
	}
	if msg.Paste && msg.PasteShortcut == tea.KeyCtrlShiftV {
		return true
	}
	return msg.String() == "ctrl+shift+v"
}

func (m *xdfileModel) hasClipboardFilePayload() bool {
	paths, err := m.currentClipboardFilePaths()
	return err == nil && len(paths) > 0
}

func (m *xdfileModel) handleFilePasteEvent(msg tea.KeyMsg) (tea.Cmd, bool) {
	if !msg.Paste {
		return nil, false
	}
	if !xdfileIsFilePasteShortcut(msg) {
		return nil, false
	}
	paths, err := m.currentClipboardFilePaths()
	if err != nil || len(paths) == 0 {
		return nil, false
	}
	if len(msg.Runes) > 0 && !xdfilePastedTextMatchesClipboardFiles(string(msg.Runes), paths) {
		return nil, false
	}
	return m.executeAction(xdfileActionPaste), true
}

func xdfilePastedTextMatchesClipboardFiles(text string, paths []string) bool {
	pasted := xdfileClipboardTextPaths(text)
	if len(pasted) == 0 {
		return false
	}
	available := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		available[xdfileClipboardPathKey(path)] = struct{}{}
	}
	for _, pastedPath := range pasted {
		if _, ok := available[xdfileClipboardPathKey(pastedPath)]; !ok {
			return false
		}
	}
	return true
}

func xdfileClipboardPathKey(path string) string {
	path = filepath.Clean(path)
	if os.PathSeparator == '\\' {
		return strings.ToLower(path)
	}
	return path
}

func xdfileClipboardTextPaths(text string) []string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	paths := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(strings.Trim(line, `"`))
		if line == "" {
			continue
		}
		if !filepath.IsAbs(line) {
			return nil
		}
		paths = append(paths, filepath.Clean(line))
	}
	return paths
}

func (m *xdfileModel) canCopySelection() bool {
	return len(m.activeFileSelectionEntries()) > 0
}

func (m *xdfileModel) activeFileSelectionEntries() []xdfileEntry {
	panel := &m.panels[m.activePanel]
	if marked := panel.markedEntries(); len(marked) > 0 {
		return marked
	}
	entry, ok := panel.selected()
	if !ok || entry.IsParent {
		return nil
	}
	return []xdfileEntry{entry}
}

func (m *xdfileModel) activeClipboardEntries() []xdfileEntry {
	return m.activeFileSelectionEntries()
}

func (m *xdfileModel) canPasteClipboardFiles() bool {
	paths, _, err := m.currentClipboardPayload()
	return err == nil && len(paths) > 0
}

func (m *xdfileModel) currentClipboardFilePaths() ([]string, error) {
	paths, _, err := m.currentClipboardPayload()
	return paths, err
}

func (m *xdfileModel) currentClipboardPayload() ([]string, bool, error) {
	paths, err := xdfileReadClipboardPathsFunc()
	if err == nil && len(paths) > 0 {
		cut, cutErr := xdfileReadClipboardCutFunc()
		if cutErr != nil {
			cut = false
		}
		if len(m.clipboardPaths) == 0 || !xdfileClipboardPathsEqual(m.clipboardPaths, paths) || m.clipboardCut != cut {
			return paths, cut, nil
		}
		return append([]string(nil), m.clipboardPaths...), m.clipboardCut, nil
	}
	if len(m.clipboardPaths) > 0 {
		return append([]string(nil), m.clipboardPaths...), m.clipboardCut, nil
	}
	if m.clipboardPath != "" {
		return []string{m.clipboardPath}, m.clipboardCut, nil
	}
	return nil, false, err
}

func xdfileClipboardPathsEqual(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if !xdfilePathsEqual(left[i], right[i]) {
			return false
		}
	}
	return true
}

func xdfileCapturePanelDirState(path string) xdfilePanelDirState {
	if xdfileIsNetBoxPath(path) {
		return xdfilePanelDirState{Path: path, Exists: true}
	}
	path = filepath.Clean(path)
	info, err := os.Stat(path)
	if err != nil {
		return xdfilePanelDirState{Path: path}
	}
	return xdfilePanelDirState{
		Path:    path,
		ModTime: info.ModTime(),
		Exists:  true,
	}
}

func xdfilePanelDirStateChanged(before xdfilePanelDirState, after xdfilePanelDirState) bool {
	if before.Path != after.Path || before.Exists != after.Exists {
		return true
	}
	if !before.Exists {
		return false
	}
	return !before.ModTime.Equal(after.ModTime)
}

func (m *xdfileModel) refreshPanelsIfChanged() {
	for i := range m.panels {
		if xdfileIsNetBoxPath(m.panels[i].Cwd) {
			continue
		}
		current := xdfileCapturePanelDirState(m.panels[i].Cwd)
		if !xdfilePanelDirStateChanged(m.panelDirState[i], current) {
			continue
		}
		if err := m.reloadPanel(i); err != nil {
			m.panelDirState[i] = current
			m.setStatusErr(err)
		}
	}
}

func (m *xdfileModel) handleMenuKey(msg tea.KeyMsg) (tea.Cmd, bool) {
	if m.openMenu == "" {
		return nil, false
	}

	menu, ok := m.currentMenu()
	if !ok || len(menu.Items) == 0 {
		m.openMenu = ""
		m.menuCursor = 0
		m.setStatus("Closed unavailable menu")
		return nil, true
	}
	m.menuCursor = xdfileValidMenuCursor(menu, m.menuCursor)

	if m.openMenu == xdfileActionCommandsMenu {
		if msg.Type == tea.KeyInsert || msg.String() == "insert" || msg.String() == "ins" {
			m.beginInsertCommandMenuItem()
			return nil, true
		}
		if msg.Type == tea.KeyF4 || msg.String() == "f4" {
			return m.beginEditCommandMenuItem(), true
		}
		if msg.Type == tea.KeyDelete || msg.String() == "delete" || msg.String() == "del" {
			return m.deleteSelectedCommandMenuItem(), true
		}
		if index, matched := m.commandMenuIndexForHotkey(msg.String()); matched {
			m.menuCursor = index
			if item, ok := m.selectedCommandMenuItem(); ok && item.isMenu() {
				return m.openCommandSubmenuIndex(index), true
			}
			return m.executeAction(xdfileCommandRunAction(index)), true
		}
	}

	if index, matched := xdfileMenuIndexForHotkey(menu, msg.String()); matched {
		m.menuCursor = index
		item := menu.Items[index]
		if xdfileMenuItemSelectable(item) {
			return m.executeAction(item.Action), true
		}
		return nil, true
	}

	switch msg.String() {
	case "esc":
		if m.openMenu == xdfileActionCommandsMenu && m.closeCommandSubmenu() {
			return nil, true
		}
		m.closeMenu("Closed %s", xdfileMenuStatusLabel(menu.Label))
		return nil, true
	case "left":
		if m.openMenu == xdfileActionCommandsMenu && m.closeCommandSubmenu() {
			return nil, true
		}
		if m.openMenu == xdfileActionContextMenu || m.openMenu == xdfileActionCommandsMenu {
			return nil, true
		}
		m.openAdjacentMenu(-1)
		return nil, true
	case "backspace":
		if m.openMenu == xdfileActionCommandsMenu && m.closeCommandSubmenu() {
			return nil, true
		}
		return m.rejectUnhandledMenuKey(msg)
	case "right":
		if m.openMenu == xdfileActionCommandsMenu {
			if item, ok := m.selectedCommandMenuItem(); ok && item.isMenu() {
				return m.openCommandSubmenuIndex(m.menuCursor), true
			}
		}
		if m.openMenu == xdfileActionContextMenu || m.openMenu == xdfileActionCommandsMenu {
			return nil, true
		}
		m.openAdjacentMenu(1)
		return nil, true
	case "up":
		m.menuCursor = xdfileNextSelectableMenuIndex(menu, m.menuCursor, -1)
		return nil, true
	case "down":
		m.menuCursor = xdfileNextSelectableMenuIndex(menu, m.menuCursor, 1)
		return nil, true
	case "enter":
		if m.menuCursor < 0 || m.menuCursor >= len(menu.Items) {
			return nil, true
		}
		item := menu.Items[m.menuCursor]
		if !xdfileMenuItemSelectable(item) {
			return nil, true
		}
		return m.executeAction(item.Action), true
	}

	return m.rejectUnhandledMenuKey(msg)
}

func (m *xdfileModel) rejectUnhandledMenuKey(msg tea.KeyMsg) (tea.Cmd, bool) {
	label := xdfileMenuKeyLabel(msg)
	if label == "" {
		m.setStatus("Menu is active; use menu shortcuts or Esc")
		return nil, true
	}
	m.setStatus("No menu shortcut: %s", label)
	return nil, true
}

func xdfileMenuKeyLabel(msg tea.KeyMsg) string {
	if msg.Paste {
		return "pasted text"
	}
	label := strings.TrimSpace(msg.String())
	if label != "" {
		return label
	}
	if len(msg.Runes) > 0 {
		return string(msg.Runes)
	}
	return ""
}

func (m *xdfileModel) handlePanelKey(msg tea.KeyMsg) tea.Cmd {
	if cmd, handled := m.handlePanelSearchKey(msg); handled {
		return cmd
	}
	if cmd, handled := m.handlePanelSearchShortcut(msg); handled {
		return cmd
	}

	panel := &m.panels[m.activePanel]
	rows := panel.visibleRows(m.layout.panelRects[m.activePanel].h)
	selectionChanged := false

	switch msg.Type {
	case tea.KeyShiftUp:
		if panel.toggleMarkedStep(-1, rows) {
			selectionChanged = true
		}
	case tea.KeyShiftDown:
		if panel.toggleMarkedStep(1, rows) {
			selectionChanged = true
		}
	case tea.KeyShiftLeft:
		if panel.toggleRangeStep(-rows, rows) > 0 {
			selectionChanged = true
		}
	case tea.KeyShiftRight:
		if panel.toggleRangeStep(rows, rows) > 0 {
			selectionChanged = true
		}
	}
	if selectionChanged {
		m.syncQuickViewViewport()
		return nil
	}

	switch msg.String() {
	case "up":
		panel.resetRangeAnchor()
		panel.move(-1, rows)
		selectionChanged = true
	case "down":
		panel.resetRangeAnchor()
		panel.move(1, rows)
		selectionChanged = true
	case "pgup":
		panel.resetRangeAnchor()
		panel.move(-rows, rows)
		selectionChanged = true
	case "pgdown":
		panel.resetRangeAnchor()
		panel.move(rows, rows)
		selectionChanged = true
	case "left":
		panel.resetRangeAnchor()
		panel.move(-rows, rows)
		selectionChanged = true
	case "right":
		panel.resetRangeAnchor()
		panel.move(rows, rows)
		selectionChanged = true
	case "home":
		panel.resetRangeAnchor()
		panel.setCursor(0, rows)
		selectionChanged = true
	case "end":
		panel.resetRangeAnchor()
		panel.setCursor(len(panel.Entries)-1, rows)
		selectionChanged = true
	case "esc":
		if panel.markedCount() > 0 {
			panel.clearMarked()
			selectionChanged = true
		}
	case "enter":
		return m.activateSelection()
	case "r":
		return m.executeAction(xdfileActionRefresh)
	case "ctrl+b":
		if m.quickViewActive() {
			return m.togglePreviewBinary()
		}
	}
	if selectionChanged {
		m.syncQuickViewViewport()
	}
	return nil
}

func (m *xdfileModel) handleManagedTerminalBoundKey(msg tea.KeyMsg) (tea.Cmd, bool) {
	if m == nil || m.terminalUsesPTY() || m.terminalFocused {
		return nil, false
	}

	if m.terminal.Busy {
		return nil, false
	}

	if m.managedTerminalPopupVisible() {
		switch msg.String() {
		case "up":
			m.moveManagedTerminalSuggestion(-1)
			return nil, true
		case "down":
			m.moveManagedTerminalSuggestion(1)
			return nil, true
		case "esc":
			return m.dismissManagedTerminalPopup(), true
		case "left", "right":
			cmd := m.updateManagedTerminalInputWithoutRefreshing(msg)
			return cmd, true
		case "delete", "del":
			if m.deleteSelectedManagedTerminalHistorySuggestion() {
				return nil, true
			}
		case "tab":
			return nil, m.acceptManagedTerminalSuggestion()
		}
		if msg.Type == tea.KeyDelete && m.deleteSelectedManagedTerminalHistorySuggestion() {
			return nil, true
		}
	}

	switch msg.String() {
	case "enter":
		command := m.managedTerminalSubmitCommandValue()
		if command == "" {
			return nil, false
		}
		return m.submitTerminalCommand(command), true
	}

	if !xdfileManagedTerminalInputKey(msg, m.terminal.Input.Value()) {
		return nil, false
	}

	var cmd tea.Cmd
	m.terminal.Input, cmd = m.terminal.Input.Update(msg)
	m.refreshManagedTerminalSuggestions()
	return cmd, true
}

func (m *xdfileModel) handleTerminalKey(msg tea.KeyMsg) tea.Cmd {
	m.terminalAutoFocused = false

	if m.terminalUsesPTY() {
		return m.handlePTYTerminalKey(msg)
	}

	if m.terminal.Busy {
		switch msg.Type {
		case tea.KeyCtrlC:
			if m.terminal.ManagedCancel != nil {
				cancel := m.terminal.ManagedCancel
				m.terminal.ManagedCancel = nil
				cancel()
				m.setStatus("Stopping command...")
				return nil
			}
		}
		switch msg.String() {
		case "ctrl+c":
			if m.terminal.ManagedCancel != nil {
				cancel := m.terminal.ManagedCancel
				m.terminal.ManagedCancel = nil
				cancel()
				m.setStatus("Stopping command...")
				return nil
			}
		}
	}

	if m.terminalStarting {
		switch msg.String() {
		case "up":
			if m.terminalHistoryPrevious() {
				return nil
			}
			m.terminal.Viewport.LineUp(1)
			return nil
		case "down":
			if m.terminalHistoryNext() {
				return nil
			}
			m.terminal.Viewport.LineDown(1)
			return nil
		case "pgup":
			m.terminal.Viewport.PageUp()
			return nil
		case "pgdown":
			m.terminal.Viewport.PageDown()
			return nil
		case "ctrl+l":
			m.terminal.Lines = nil
			m.syncTerminalViewport(true)
			m.setStatus("Terminal cleared")
			return nil
		case "enter":
			command := m.managedTerminalSubmitCommandValue()
			if command == "" {
				return nil
			}
			m.terminal.Input.SetValue(command)
			m.terminal.Input.CursorEnd()
			m.terminal.StartupSubmitPending = true
			m.pushTerminalHistory(command)
			m.setStatus("PTY terminal is still starting; command queued")
			return nil
		}

		var cmd tea.Cmd
		m.terminal.Input, cmd = m.terminal.Input.Update(msg)
		m.refreshManagedTerminalSuggestions()
		return cmd
	}

	if m.managedTerminalPopupVisible() {
		switch msg.String() {
		case "esc":
			return m.dismissManagedTerminalPopup()
		case "left", "right":
			return m.updateManagedTerminalInputWithoutRefreshing(msg)
		case "delete", "del":
			if m.deleteSelectedManagedTerminalHistorySuggestion() {
				return nil
			}
		}
		if msg.Type == tea.KeyDelete && m.deleteSelectedManagedTerminalHistorySuggestion() {
			return nil
		}
	}

	switch msg.String() {
	case "up":
		if !m.terminal.Busy && m.terminalHistoryPrevious() {
			return nil
		}
		m.terminal.Viewport.LineUp(1)
		return nil
	case "down":
		if !m.terminal.Busy && m.terminalHistoryNext() {
			return nil
		}
		m.terminal.Viewport.LineDown(1)
		return nil
	case "pgup":
		m.terminal.Viewport.PageUp()
		return nil
	case "pgdown":
		m.terminal.Viewport.PageDown()
		return nil
	case "ctrl+l":
		m.terminal.Lines = nil
		m.syncTerminalViewport(true)
		m.setStatus("Terminal cleared")
		return nil
	case "enter":
		command := m.managedTerminalSubmitCommandValue()
		if command == "" || m.terminal.Busy {
			return nil
		}
		return m.submitTerminalCommand(command)
	}

	var cmd tea.Cmd
	m.terminal.Input, cmd = m.terminal.Input.Update(msg)
	m.refreshManagedTerminalSuggestions()
	return cmd
}

func (m *xdfileModel) handleModalKey(msg tea.KeyMsg) tea.Cmd {
	switch m.modal.Kind {
	case xdfileModalText:
		switch msg.String() {
		case "up":
			if !m.modal.PreviewVisual {
				m.modal.Viewport.LineUp(1)
			}
		case "down":
			if !m.modal.PreviewVisual {
				m.modal.Viewport.LineDown(1)
			}
		case "pgup":
			if !m.modal.PreviewVisual {
				m.modal.Viewport.PageUp()
			}
		case "pgdown":
			if !m.modal.PreviewVisual {
				m.modal.Viewport.PageDown()
			}
		case "home":
			if !m.modal.PreviewVisual {
				m.modal.Viewport.GotoTop()
			}
		case "end":
			if !m.modal.PreviewVisual {
				m.modal.Viewport.GotoBottom()
			}
		case "ctrl+b":
			if m.modal.Action == xdfileActionPreview {
				return m.togglePreviewBinary()
			}
		case "ctrl+q":
			if m.modal.Action == xdfileActionPreview {
				m.closeModal()
			}
		case "enter", "esc", "q":
			return m.closeModalAndResumeFileQueue()
		}
		return nil
	case xdfileModalConfirm:
		switch msg.String() {
		case "left", "up", "shift+tab":
			m.modal.ChoiceCursor = (m.modal.ChoiceCursor - 1 + 2) % 2
		case "right", "down", "tab":
			m.modal.ChoiceCursor = (m.modal.ChoiceCursor + 1) % 2
		case "enter":
			if m.modal.ChoiceCursor == 1 {
				m.closeModal()
				return nil
			}
			return m.applyModal()
		case "y":
			m.modal.ChoiceCursor = 0
			return m.applyModal()
		case "esc", "n":
			m.closeModal()
		}
		return nil
	case xdfileModalChoice:
		switch msg.String() {
		case "left", "up", "shift+tab":
			if len(m.modal.ChoiceItems) > 0 {
				m.modal.ChoiceCursor = (m.modal.ChoiceCursor - 1 + len(m.modal.ChoiceItems)) % len(m.modal.ChoiceItems)
			}
		case "right", "down", "tab":
			if len(m.modal.ChoiceItems) > 0 {
				m.modal.ChoiceCursor = (m.modal.ChoiceCursor + 1) % len(m.modal.ChoiceItems)
			}
		case "enter":
			if len(m.modal.ChoiceItems) == 0 {
				return nil
			}
			return m.executeAction(m.modal.ChoiceItems[m.modal.ChoiceCursor].Action)
		case "esc":
			if m.modal.Action == xdfileActionPasteConflictPrompt {
				m.pendingClipboardPaste = nil
				m.setStatus("Paste canceled")
			}
			m.closeModal()
		}
		return nil
	case xdfileModalForm:
		if len(m.modal.FormFields) == 0 {
			return nil
		}
		index := max(0, min(m.modal.FormCursor, len(m.modal.FormFields)-1))
		activeField := &m.modal.FormFields[index]
		switch msg.String() {
		case "enter":
			return m.applyModal()
		case "shift+enter":
			if activeField.Multiline {
				activeField.InsertNewline()
			}
			return nil
		case "esc":
			if m.modal.Action == xdfileActionCommandPrompt {
				m.discardPendingCommandMenuPrompt()
			} else {
				m.closeModal()
			}
			return nil
		case "tab":
			m.focusModalFormField(m.modal.FormCursor + 1)
			return nil
		case "shift+tab":
			m.focusModalFormField(m.modal.FormCursor - 1)
			return nil
		case "down":
			if !activeField.Multiline {
				m.focusModalFormField(m.modal.FormCursor + 1)
				return nil
			}
		case "up":
			if !activeField.Multiline {
				m.focusModalFormField(m.modal.FormCursor - 1)
				return nil
			}
		}
		var cmd tea.Cmd
		if activeField.Multiline {
			activeField.TextArea, cmd = activeField.TextArea.Update(msg)
			activeField.syncMirrorInput()
			return cmd
		}
		activeField.Input, cmd = activeField.Input.Update(msg)
		return cmd
	case xdfileModalInput:
		switch msg.String() {
		case "enter":
			return m.applyModal()
		case "esc":
			m.closeModal()
			return nil
		}
		var cmd tea.Cmd
		m.modal.Input, cmd = m.modal.Input.Update(msg)
		return cmd
	}
	return nil
}

func (m *xdfileModel) handleModalMouse(msg tea.MouseMsg) tea.Cmd {
	rect := m.modalRect()
	if !rect.contains(msg.X, msg.Y) {
		return nil
	}

	switch m.modal.Kind {
	case xdfileModalText:
		if m.modal.PreviewVisual {
			return nil
		}
		switch {
		case msg.Button == tea.MouseButtonWheelUp:
			m.modal.Viewport.LineUp(3)
		case msg.Button == tea.MouseButtonWheelDown:
			m.modal.Viewport.LineDown(3)
		case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft:
			m.modal.Viewport.LineUp(3)
		case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonRight:
			m.modal.Viewport.LineDown(3)
		}
	case xdfileModalConfirm:
		if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
			return nil
		}
		index, ok := m.modalConfirmChoiceIndexAt(msg.X, msg.Y)
		if !ok {
			return nil
		}
		m.modal.ChoiceCursor = index
		if index == 1 {
			m.closeModal()
			return nil
		}
		return m.applyModal()
	case xdfileModalChoice:
		if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
			return nil
		}
		index, ok := m.modalChoiceIndexAt(msg.X, msg.Y)
		if !ok {
			return nil
		}
		m.modal.ChoiceCursor = index
		if index < 0 || index >= len(m.modal.ChoiceItems) {
			return nil
		}
		return m.executeAction(m.modal.ChoiceItems[index].Action)
	case xdfileModalForm:
		if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
			return nil
		}
		index, ok := m.modalFormFieldIndexAt(msg.Y)
		if !ok {
			return nil
		}
		m.focusModalFormField(index)
	}

	return nil
}
