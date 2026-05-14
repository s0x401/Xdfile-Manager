package cmd

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	charmansi "github.com/charmbracelet/x/ansi"
)

const xdfileTerminalLineLimit = 1200

func (m *xdfileModel) applyModal() tea.Cmd {
	switch m.modal.Action {
	case xdfileActionModalRename, xdfileActionModalMkdir, xdfileActionCopy, xdfileActionMove, xdfileActionDelete:
		if m.backgroundTaskBusy && !m.fileOperationActive() {
			m.setStatus("Wait for the current background task to finish")
			return nil
		}
	}

	switch m.modal.Action {
	case xdfileActionModalRename:
		panelIndex := m.modal.PanelIndex
		if !m.validPanelIndex(panelIndex) {
			m.setStatus("Invalid panel")
			return nil
		}
		newName := strings.TrimSpace(m.modal.Input.Value())
		if newName == "" {
			m.setStatus("Name cannot be empty")
			return nil
		}
		if strings.ContainsAny(newName, `/\`) {
			m.setStatus("Rename only accepts a single file or directory name")
			return nil
		}
		dst := xdfileJoinPath(xdfileParentPath(m.modal.SourcePath), newName)
		sourcePath := m.modal.SourcePath
		m.closeModal()
		return m.startFileOperation(xdfileFileOperation{
			Kind:       xdfileFileOperationRename,
			SourcePath: sourcePath,
			TargetPath: dst,
			PanelIndex: panelIndex,
		})
	case xdfileActionModalMkdir:
		panelIndex := m.modal.PanelIndex
		if !m.validPanelIndex(panelIndex) {
			m.setStatus("Invalid panel")
			return nil
		}
		name := strings.TrimSpace(m.modal.Input.Value())
		if name == "" {
			m.setStatus("Directory name cannot be empty")
			return nil
		}
		dst := xdfileJoinPath(m.panels[panelIndex].Cwd, name)
		m.closeModal()
		return m.startFileOperation(xdfileFileOperation{
			Kind:       xdfileFileOperationMkdir,
			TargetPath: dst,
			PanelIndex: panelIndex,
		})
	case xdfileActionInsertCommand:
		if len(m.modal.FormFields) < 3 {
			m.setStatus("Command form is incomplete")
			return nil
		}
		item := xdfileCommandItem{
			Type:    xdfileCommandItemTypeCommand,
			Hotkey:  m.modal.FormFields[0].Value(),
			Label:   strings.TrimSpace(m.modal.FormFields[1].Value()),
			Command: strings.TrimSpace(m.modal.FormFields[2].Value()),
		}.normalized()
		if item.Label == "" {
			m.setStatus("Command label cannot be empty")
			return nil
		}
		if item.Command == "" {
			m.setStatus("Command line cannot be empty")
			return nil
		}
		return m.insertCommandMenuItem(item)
	case xdfileActionEditCommand:
		if len(m.modal.FormFields) < 3 {
			m.setStatus("Command form is incomplete")
			return nil
		}
		item := xdfileCommandItem{
			Type:    xdfileCommandItemTypeCommand,
			Hotkey:  m.modal.FormFields[0].Value(),
			Label:   strings.TrimSpace(m.modal.FormFields[1].Value()),
			Command: strings.TrimSpace(m.modal.FormFields[2].Value()),
		}.normalized()
		if item.Label == "" {
			m.setStatus("Command label cannot be empty")
			return nil
		}
		if item.Command == "" {
			m.setStatus("Command line cannot be empty")
			return nil
		}
		return m.updateCommandMenuItem(item)
	case xdfileActionInsertMenu:
		if len(m.modal.FormFields) < 2 {
			m.setStatus("Menu form is incomplete")
			return nil
		}
		item := xdfileCommandItem{
			Type:   xdfileCommandItemTypeMenu,
			Hotkey: m.modal.FormFields[0].Value(),
			Label:  strings.TrimSpace(m.modal.FormFields[1].Value()),
		}.normalized()
		if item.Label == "" {
			m.setStatus("Menu label cannot be empty")
			return nil
		}
		return m.insertCommandMenuItem(item)
	case xdfileActionEditMenu:
		if len(m.modal.FormFields) < 2 {
			m.setStatus("Menu form is incomplete")
			return nil
		}
		item := xdfileCommandItem{
			Type:   xdfileCommandItemTypeMenu,
			Hotkey: m.modal.FormFields[0].Value(),
			Label:  strings.TrimSpace(m.modal.FormFields[1].Value()),
		}.normalized()
		if item.Label == "" {
			m.setStatus("Menu label cannot be empty")
			return nil
		}
		return m.updateCommandMenuItem(item)
	case xdfileActionCommandPrompt:
		return m.applyCommandMenuPrompt()
	case xdfileActionNetBoxSave:
		return m.saveNetBoxConnectionFromModal()
	case xdfileActionNetBoxDelete:
		return m.deleteNetBoxConnectionFromModal()
	case xdfileActionCopy:
		sourcePaths := append([]string(nil), m.modal.SourcePaths...)
		sourcePath := m.modal.SourcePath
		targetPath := m.modal.TargetPath
		if len(sourcePaths) <= 1 {
			target, err := xdfileUniqueCopyTarget(m.modal.TargetPath)
			if err != nil {
				m.setStatusErr(err)
				return nil
			}
			targetPath = target
		}
		panelIndex := m.modal.PanelIndex
		m.closeModal()
		return m.startFileOperation(xdfileFileOperation{
			Kind:        xdfileFileOperationCopy,
			SourcePath:  sourcePath,
			SourcePaths: sourcePaths,
			TargetPath:  targetPath,
			PanelIndex:  panelIndex,
		})
	case xdfileActionMove:
		targetPath := m.modal.TargetPath
		sourcePath := m.modal.SourcePath
		sourcePaths := append([]string(nil), m.modal.SourcePaths...)
		panelIndex := m.modal.PanelIndex
		m.closeModal()
		return m.startFileOperation(xdfileFileOperation{
			Kind:        xdfileFileOperationMove,
			SourcePath:  sourcePath,
			SourcePaths: sourcePaths,
			TargetPath:  targetPath,
			PanelIndex:  panelIndex,
		})
	case xdfileActionDelete:
		sourcePaths := append([]string(nil), m.modal.SourcePaths...)
		if len(sourcePaths) == 0 && m.modal.SourcePath != "" {
			sourcePaths = []string{m.modal.SourcePath}
		}
		sourcePath := m.modal.SourcePath
		panelIndex := m.modal.PanelIndex
		m.closeModal()
		return m.startFileOperation(xdfileFileOperation{
			Kind:        xdfileFileOperationDelete,
			SourcePath:  sourcePath,
			SourcePaths: sourcePaths,
			PanelIndex:  panelIndex,
		})
	case xdfileActionQuit:
		m.closeXdfileResources()
		return tea.Quit
	default:
		m.closeModal()
	}
	return nil
}

func (m *xdfileModel) reloadPanel(index int) error {
	panel := &m.panels[index]
	entries, err := xdfileReadEntries(panel.Cwd, m.showHidden, m.panelSortMode(index))
	if err != nil {
		parent := xdfileParentPath(panel.Cwd)
		if parent != panel.Cwd {
			panel.Cwd = parent
			entries, err = xdfileReadEntries(panel.Cwd, m.showHidden, m.panelSortMode(index))
		}
		if err != nil {
			return err
		}
	}
	if xdfileIsNetBoxPath(panel.Cwd) {
		panel.Git = xdfileGitPanelInfo{}
	} else {
		panel.Git = xdfileReadGitStatusFunc(panel.Cwd)
	}
	entries = xdfileApplyGitStatus(entries, panel.Git)
	panel.Entries = entries
	panel.syncMarkedEntries()
	m.panelDirState[index] = xdfileCapturePanelDirState(panel.Cwd)
	panel.ensureVisible(panel.visibleRows(m.layout.panelRects[index].h))
	m.syncQuickViewViewport()
	return nil
}

func (m *xdfileModel) reloadAllPanels() {
	for i := range m.panels {
		if err := m.reloadPanel(i); err != nil {
			m.setStatusErr(err)
		}
	}
}

func (m *xdfileModel) reloadPanelsAfterTerminalCommand(commandCwd string) {
	if xdfileIsNetBoxPath(commandCwd) {
		return
	}
	m.reloadAllPanels()
}

func (m *xdfileModel) invalidateActivePanelCache() {
	if m.activePanel < 0 || m.activePanel >= len(m.panels) {
		return
	}
	xdfileInvalidateNetBoxEntryCache(m.panels[m.activePanel].Cwd)
}

func (m *xdfileModel) changePanelDir(index int, dir string, revealPath string) error {
	panel := &m.panels[index]
	previous := panel.Cwd
	panel.Cwd = dir
	if err := m.reloadPanel(index); err != nil {
		panel.Cwd = previous
		_ = m.reloadPanel(index)
		return err
	}
	rows := panel.visibleRows(m.layout.panelRects[index].h)
	if revealPath == "" {
		panel.setCursor(0, rows)
		return nil
	}
	panel.focusPath(revealPath, rows)
	return nil
}

func (m *xdfileModel) appendTerminalOutput(msg xdfileTerminalResultMsg) {
	m.appendTerminalPrompt(msg.Dir, msg.Command)

	output := strings.TrimSpace(msg.Output)
	if output != "" {
		m.terminal.CommandHadOutput = true
		m.terminal.Lines = append(m.terminal.Lines, strings.Split(output, "\n")...)
	}
	if msg.Err != nil {
		if !xdfileSuppressTerminalExitLine(msg.Err, m.terminal.CommandHadOutput) {
			m.terminal.Lines = append(m.terminal.Lines, xdfileStatusErrStyle.Render(msg.Err.Error()))
		}
		m.setStatusErr(msg.Err)
	} else if output == "" {
		m.terminal.Lines = append(m.terminal.Lines, xdfileDimStyle.Render("(no output)"))
		m.setStatus("Command completed")
	} else {
		m.setStatus("Command completed")
	}
	m.trimTerminalLines()
	m.syncTerminalViewport(true)
}

func (m *xdfileModel) appendTerminalPrompt(dir string, command string) {
	if strings.TrimSpace(command) == "" {
		return
	}
	m.terminal.StreamCanRewrite = false
	rendered := xdfileTerminalPromptLabelStyle.Render("xd") +
		xdfileDimStyle.Render(" ") +
		xdfileTerminalPromptPathStyle.Render(dir) +
		xdfileDimStyle.Render(" :: ") +
		xdfileTerminalPromptCommandStyle.Render(command)
	m.terminal.Lines = append(m.terminal.Lines, rendered)
	m.trimTerminalLines()
}

func (m *xdfileModel) appendTerminalLine(line string, rewrite bool, finalize bool) {
	if strings.TrimSpace(charmansi.Strip(line)) != "" {
		m.terminal.CommandHadOutput = true
	}
	if m.terminal.StreamCanRewrite && len(m.terminal.Lines) > 0 {
		m.terminal.Lines[len(m.terminal.Lines)-1] = line
	} else {
		m.terminal.Lines = append(m.terminal.Lines, line)
	}
	m.terminal.StreamCanRewrite = rewrite && !finalize
	m.trimTerminalLines()
	m.syncTerminalViewport(true)
}

func (m *xdfileModel) trimTerminalLines() {
	if len(m.terminal.Lines) > xdfileTerminalLineLimit {
		m.terminal.Lines = m.terminal.Lines[len(m.terminal.Lines)-xdfileTerminalLineLimit:]
	}
}

func (m *xdfileModel) submitTerminalCommand(command string) tea.Cmd {
	rawCommand := strings.TrimSpace(command)
	expansion, err := m.prepareManagedTerminalCommand(rawCommand)
	if err != nil {
		m.setStatusErr(err)
		return nil
	}
	command = expansion.Command
	m.registerCommandMenuTempFiles(expansion.TempFiles)

	historyCommand := rawCommand
	if inputValue := strings.TrimSpace(m.terminal.Input.Value()); inputValue != "" {
		historyCommand = inputValue
	}
	m.pushTerminalHistory(historyCommand)
	m.terminal.Input.SetValue("")
	m.refreshManagedTerminalSuggestions()

	if strings.EqualFold(command, "clear") || strings.EqualFold(command, "cls") {
		m.terminal.Lines = nil
		m.syncTerminalViewport(true)
		m.clearPendingTerminalHistory(command)
		m.setStatus("Terminal cleared")
		return nil
	}

	m.restorePanelFocusAfterManagedCommand()
	if xdfileIsNetBoxPath(m.terminal.Cwd) {
		m.terminal.Busy = true
		width, height := m.streamingCommandTerminalSize()
		return xdfileExecuteCommandCmd(m.terminal.Cwd, command, width, height)
	}
	if detached, handled := xdfileStartDetachedExternalCommand(m.terminal.Cwd, command); handled {
		return func() tea.Msg { return detached }
	}

	if _, ok := xdfileResolveExclusiveTUICommand(m.terminal.Cwd, command); ok {
		width, height := m.exclusiveTerminalViewportSize()
		m.setStatus("Launching exclusive terminal...")
		return xdfileStartExclusiveTerminalFunc(m.terminal.Cwd, command, width, height)
	}

	m.terminal.Busy = true
	width, height := m.streamingCommandTerminalSize()
	return xdfileExecuteCommandCmd(m.terminal.Cwd, command, width, height)
}

func xdfileSuppressTerminalExitLine(err error, commandHadOutput bool) bool {
	if err == nil || !commandHadOutput {
		return false
	}
	return strings.HasPrefix(err.Error(), "command exited with code ")
}

func (m *xdfileModel) terminalHistoryPrevious() bool {
	if len(m.terminal.History) == 0 {
		return false
	}
	if m.terminal.HistoryIndex == -1 {
		m.terminal.HistoryDraft = m.terminal.Input.Value()
		m.terminal.HistoryIndex = len(m.terminal.History) - 1
	} else if m.terminal.HistoryIndex > 0 {
		m.terminal.HistoryIndex--
	} else {
		return true
	}
	m.terminal.Input.SetValue(m.terminal.History[m.terminal.HistoryIndex])
	m.terminal.Input.CursorEnd()
	m.refreshManagedTerminalSuggestions()
	return true
}

func (m *xdfileModel) terminalHistoryNext() bool {
	if m.terminal.HistoryIndex == -1 {
		return false
	}
	if m.terminal.HistoryIndex < len(m.terminal.History)-1 {
		m.terminal.HistoryIndex++
		m.terminal.Input.SetValue(m.terminal.History[m.terminal.HistoryIndex])
	} else {
		m.terminal.HistoryIndex = -1
		m.terminal.Input.SetValue(m.terminal.HistoryDraft)
	}
	m.terminal.Input.CursorEnd()
	m.refreshManagedTerminalSuggestions()
	return true
}

func xdfileManagedTerminalInputKey(msg tea.KeyMsg, current string) bool {
	switch msg.Type {
	case tea.KeyRunes, tea.KeySpace:
		return true
	case tea.KeyBackspace, tea.KeyDelete:
		return current != ""
	case tea.KeyLeft, tea.KeyRight:
		return current != ""
	}

	switch msg.String() {
	case "ctrl+u", "ctrl+w":
		return current != ""
	default:
		return msg.Paste && len(msg.Runes) > 0
	}
}

func (m *xdfileModel) refreshManagedTerminalSuggestions() {
	if m == nil || m.terminalUsesPTY() {
		return
	}
	input := m.terminal.Input.Value()
	inputChanged := m.terminal.SuggestionInput != input
	if inputChanged {
		m.terminal.SuggestionDismissed = false
	}
	m.terminal.SuggestionInput = input
	m.terminal.Input.ShowSuggestions = false
	m.terminal.Input.SetSuggestions(nil)

	raw := m.terminalSuggestionValues(input, 20)
	filtered := make([]string, 0, len(raw))
	trimmedInput := strings.TrimSpace(input)
	for _, suggestion := range raw {
		if strings.EqualFold(strings.TrimSpace(suggestion), trimmedInput) {
			continue
		}
		filtered = append(filtered, suggestion)
	}
	m.terminal.Suggestions = filtered
	if len(filtered) == 0 {
		m.terminal.SuggestionCursor = -1
		return
	}
	if inputChanged || m.terminal.SuggestionCursor < 0 || m.terminal.SuggestionCursor > len(filtered) {
		m.terminal.SuggestionCursor = 0
	}
}

func (m *xdfileModel) acceptManagedTerminalSuggestion() bool {
	if m == nil || m.terminalUsesPTY() {
		return false
	}
	suggestion := strings.TrimSpace(m.selectedManagedTerminalSuggestion())
	if suggestion == "" {
		return false
	}
	m.terminal.Input.SetValue(suggestion)
	m.terminal.Input.CursorEnd()
	m.terminal.SuggestionDismissed = false
	m.refreshManagedTerminalSuggestions()
	return true
}

func (m *xdfileModel) managedTerminalSubmitCommandValue() string {
	if m == nil {
		return ""
	}
	if m.managedTerminalPopupVisible() {
		if suggestion := strings.TrimSpace(m.selectedManagedTerminalSuggestion()); suggestion != "" {
			return suggestion
		}
	}
	return strings.TrimSpace(m.terminal.Input.Value())
}

func (m *xdfileModel) syncManagedTerminalSuggestionPreview() {
	if m == nil || m.terminalUsesPTY() {
		return
	}

	value := m.terminal.SuggestionInput
	if suggestion := strings.TrimSpace(m.selectedManagedTerminalSuggestion()); suggestion != "" {
		if m.terminal.SuggestionInput == "" {
			m.terminal.SuggestionInput = m.terminal.Input.Value()
		}
		value = suggestion
	} else if value == "" {
		value = m.terminal.Input.Value()
	}
	if m.terminal.Input.Value() == value {
		return
	}
	m.terminal.Input.SetValue(value)
	m.terminal.Input.CursorEnd()
}

func (m *xdfileModel) restoreManagedTerminalSuggestionInput() {
	if m == nil || m.terminalUsesPTY() {
		return
	}
	m.terminal.SuggestionCursor = 0
	m.syncManagedTerminalSuggestionPreview()
}

func (m *xdfileModel) dismissManagedTerminalPopup() tea.Cmd {
	if m == nil {
		return nil
	}
	m.restoreManagedTerminalSuggestionInput()
	m.terminal.SuggestionDismissed = true
	return nil
}

func (m *xdfileModel) updateManagedTerminalInputWithoutRefreshing(msg tea.KeyMsg) tea.Cmd {
	if m == nil {
		return nil
	}
	var cmd tea.Cmd
	m.terminal.Input, cmd = m.terminal.Input.Update(msg)
	return cmd
}

func (m *xdfileModel) managedTerminalPopupVisible() bool {
	return m != nil &&
		!m.terminalUsesPTY() &&
		!m.terminal.Busy &&
		!m.terminal.SuggestionDismissed &&
		len(m.terminal.Suggestions) > 0 &&
		strings.TrimSpace(m.terminal.Input.Value()) != ""
}

func (m *xdfileModel) selectedManagedTerminalSuggestion() string {
	if m == nil || len(m.terminal.Suggestions) == 0 {
		return ""
	}
	index := m.terminal.SuggestionCursor
	if index <= 0 || index > len(m.terminal.Suggestions) {
		index = 0
	}
	if index == 0 {
		return ""
	}
	return m.terminal.Suggestions[index-1]
}

func (m *xdfileModel) moveManagedTerminalSuggestion(delta int) {
	if m == nil || len(m.terminal.Suggestions) == 0 {
		return
	}
	total := len(m.terminal.Suggestions) + 1
	index := m.terminal.SuggestionCursor
	if index < 0 || index >= total {
		index = 0
	}
	index = (index + delta + total) % total
	m.terminal.SuggestionCursor = index
	m.syncManagedTerminalSuggestionPreview()
}

func (m *xdfileModel) closeTerminalSession() {
	if m.terminal.Session != nil {
		m.terminal.Session.Close()
	}
	m.terminal.Session = nil
	m.terminal.Emulator = nil
	m.terminal.Events = nil
	m.terminalStarting = false
}

func (m *xdfileModel) setStatus(format string, args ...any) {
	m.statusText = fmt.Sprintf(format, args...)
	m.statusError = false
}

func (m *xdfileModel) setStatusErr(err error) {
	if err == nil {
		return
	}
	m.statusText = err.Error()
	m.statusError = true
}
