package cmd

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

func (m *xdfileModel) executeAction(action xdfileAction) tea.Cmd {
	if index, ok := xdfileParseCommandRunAction(action); ok {
		return m.executeCommandMenuIndex(index)
	}
	if index, ok := xdfileParseCommandOpenAction(action); ok {
		return m.openCommandSubmenuIndex(index)
	}
	if index, ok := xdfileParseNetBoxConnectAction(action); ok {
		return m.openNetBoxConnection(index)
	}
	if index, ok := xdfileParseNetBoxEditAction(action); ok {
		connection, ok := m.netBoxConnectionAt(index)
		if !ok {
			m.setStatus("SSH connection not found")
			return nil
		}
		return m.openNetBoxConnectionForm(&connection)
	}
	if index, ok := xdfileParseNetBoxDeleteIndexedAction(action); ok {
		return m.confirmDeleteNetBoxConnection(index)
	}
	if action == xdfileActionCommandsMenu {
		return m.openCommandMenu()
	}
	if isXdfileMenuAction(action) {
		return m.toggleMenu(action)
	}
	m.openMenu = ""
	m.menuCursor = 0

	switch action {
	case xdfileActionHelp:
		m.openTextModal("Xdfile Manager Help", xdfileHelpText())
	case xdfileActionOpen:
		return m.activateSelection()
	case xdfileActionClipboardCopy:
		return m.copySelectionToClipboard()
	case xdfileActionClipboardCut:
		return m.cutSelectionToClipboard()
	case xdfileActionSync:
		passive := 1 - m.activePanel
		m.panels[passive].Cwd = m.panels[m.activePanel].Cwd
		if err := m.reloadPanel(passive); err != nil {
			m.setStatusErr(err)
		} else {
			m.setStatus("Synced %s panel to %s", strings.ToLower(m.panels[passive].Label), m.panels[m.activePanel].Cwd)
		}
	case xdfileActionPreview:
		return m.openPreview()
	case xdfileActionProperties:
		return m.openProperties()
	case xdfileActionWindowsContextMenu:
		return m.openWindowsContextMenuForSelection()
	case xdfileActionPaste:
		return m.pasteClipboardToActivePanel()
	case xdfileActionPasteConflictOverwrite:
		return m.resolvePendingClipboardPasteConflict(xdfileActionPasteConflictOverwrite)
	case xdfileActionPasteConflictSkip:
		return m.resolvePendingClipboardPasteConflict(xdfileActionPasteConflictSkip)
	case xdfileActionPasteConflictRename:
		return m.resolvePendingClipboardPasteConflict(xdfileActionPasteConflictRename)
	case xdfileActionPasteConflictApplyAll:
		return m.resolvePendingClipboardPasteConflict(xdfileActionPasteConflictApplyAll)
	case xdfileActionRename:
		return m.openRename()
	case xdfileActionCopy:
		return m.openTransferConfirm(xdfileActionCopy)
	case xdfileActionMove:
		return m.openTransferConfirm(xdfileActionMove)
	case xdfileActionMkdir:
		m.openInputModal(
			xdfileActionModalMkdir,
			"Create Directory",
			"Create a new directory in the active panel.",
			m.activePanel,
			"",
			"",
		)
	case xdfileActionDelete:
		return m.openDeleteConfirm()
	case xdfileActionUndoDelete:
		return m.undoLastFileAction()
	case xdfileActionHidden:
		m.showHidden = !m.showHidden
		m.reloadAllPanels()
		if m.showHidden {
			m.setStatus("Dotfiles are now visible")
		} else {
			m.setStatus("Dotfiles are now hidden")
		}
	case xdfileActionQuit:
		return m.openQuitConfirm()
	case xdfileActionRefresh:
		m.invalidateActivePanelCache()
		m.reloadAllPanels()
		m.setStatus("Panels refreshed")
	case xdfileActionParent:
		panel := &m.panels[m.activePanel]
		childPath := panel.Cwd
		parent := xdfileParentPath(panel.Cwd)
		if parent == panel.Cwd {
			m.setStatus("Already at the filesystem root")
			return nil
		}
		if err := m.changePanelDir(m.activePanel, parent, childPath); err != nil {
			m.setStatusErr(err)
			return nil
		}
		m.setStatus("Moved to %s", parent)
		return m.syncTerminalToPanel(m.activePanel)
	case xdfileActionTerminalExpand:
		return m.toggleTerminalExpandedView()
	case xdfileActionNetBoxNew:
		return m.openNetBoxConnectionForm(nil)
	case xdfileActionNetBoxDisconnect:
		return m.disconnectNetBoxPanel()
	case xdfileActionSaveLayout:
		return m.saveLayout()
	case xdfileActionResetLayout:
		return m.resetSetup()
	case xdfileActionInsertCommand:
		m.openCommandItemForm(xdfileCommandItemTypeCommand)
		return nil
	case xdfileActionInsertMenu:
		m.openCommandItemForm(xdfileCommandItemTypeMenu)
		return nil
	case xdfileActionPanelClickCmd:
		m.setStatus("Panel clicks always keep focus on the file panels")
	case xdfileActionQuickViewMode:
		m.layoutPrefs.QuickViewDocked = !m.layoutPrefs.QuickViewDocked
		if !m.layoutPrefs.QuickViewDocked && m.quickView.Open {
			m.closeQuickView()
		}
		if m.layoutPrefs.QuickViewDocked {
			m.setStatus("Ctrl+Q now opens docked quick view on the left panel")
		} else {
			m.setStatus("Ctrl+Q now opens the floating preview window")
		}
	case xdfileActionThemePersona3:
		m.applyThemeByName(xdfileThemePersona3Name)
		m.setStatus("Theme switched to %s", xdfileThemeDisplayName(xdfileThemePersona3Name))
	case xdfileActionThemePersona3Reload:
		m.applyThemeByName(xdfileThemePersona3ReloadName)
		m.setStatus("Theme switched to %s", xdfileThemeDisplayName(xdfileThemePersona3ReloadName))
	case xdfileActionThemePersona3Kotone:
		m.applyThemeByName(xdfileThemePersona3KotoneName)
		m.setStatus("Theme switched to %s", xdfileThemeDisplayName(xdfileThemePersona3KotoneName))
	case xdfileActionThemePersona4:
		m.applyThemeByName(xdfileThemePersona4Name)
		m.setStatus("Theme switched to %s", xdfileThemeDisplayName(xdfileThemePersona4Name))
	case xdfileActionThemePersona5:
		m.applyThemeByName(xdfileThemePersona5Name)
		m.setStatus("Theme switched to %s", xdfileThemeDisplayName(xdfileThemePersona5Name))
	case xdfileActionSortName:
		m.setPanelSortMode(m.activePanel, xdfileSortModeName)
		if err := m.reloadPanel(m.activePanel); err != nil {
			m.setStatusErr(err)
			return nil
		}
		m.setStatus("Sorted %s panel by name", strings.ToLower(m.panels[m.activePanel].Label))
	case xdfileActionSortExt:
		m.setPanelSortMode(m.activePanel, xdfileSortModeExt)
		if err := m.reloadPanel(m.activePanel); err != nil {
			m.setStatusErr(err)
			return nil
		}
		m.setStatus("Sorted %s panel by extension", strings.ToLower(m.panels[m.activePanel].Label))
	}
	return nil
}

func (m *xdfileModel) activateSelection() tea.Cmd {
	panel := &m.panels[m.activePanel]
	entry, ok := panel.selected()
	if !ok {
		return nil
	}
	if entry.IsDir || entry.IsParent {
		revealPath := ""
		if entry.IsParent {
			revealPath = panel.Cwd
		}
		if err := m.changePanelDir(m.activePanel, entry.Path, revealPath); err != nil {
			m.setStatusErr(err)
			return nil
		}
		m.setStatus("Opened %s", panel.Cwd)
		return m.syncTerminalToPanel(m.activePanel)
	}
	if xdfileIsNetBoxPath(entry.Path) {
		m.setStatus("Remote file open is unavailable; copy it locally first")
		return nil
	}
	if err := xdfileOpenPathFunc(entry.Path); err != nil {
		m.setStatusErr(fmt.Errorf("open failed: %w", err))
		return nil
	}
	m.setStatus("Opened %s", entry.Path)
	return nil
}

func (m *xdfileModel) copySelectionToClipboard() tea.Cmd {
	return m.storeSelectionInClipboard(false)
}

func (m *xdfileModel) cutSelectionToClipboard() tea.Cmd {
	return m.storeSelectionInClipboard(true)
}

func (m *xdfileModel) storeSelectionInClipboard(cut bool) tea.Cmd {
	entries := m.activeClipboardEntries()
	if len(entries) == 0 {
		if cut {
			m.setStatus("Select a file or directory to cut")
		} else {
			m.setStatus("Select a file or directory to copy")
		}
		return nil
	}

	hasRemote := false
	for _, entry := range entries {
		if xdfileIsNetBoxPath(entry.Path) {
			hasRemote = true
			break
		}
	}
	if hasRemote {
		if cut {
			m.setStatus("SSH cut is unavailable; copy the item, then delete it manually")
			return nil
		}
		return m.copyRemoteSelectionToClipboard(entries)
	}

	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		paths = append(paths, entry.Path)
	}

	m.cleanupRemoteClipboardDirs()
	m.clipboardPaths = append([]string(nil), paths...)
	m.clipboardPath = paths[0]
	m.clipboardCut = cut
	if len(entries) == 1 && cut {
		m.setStatus("Cut %s to clipboard", entries[0].Name)
	} else if len(entries) == 1 {
		m.setStatus("Copied %s to clipboard", entries[0].Name)
	} else if cut {
		m.setStatus("Cut %d items to clipboard", len(entries))
	} else {
		m.setStatus("Copied %d items to clipboard", len(entries))
	}
	return func() tea.Msg {
		return xdfileClipboardWriteResultMsg{Err: xdfileWriteClipboardPathsFunc(paths, cut)}
	}
}

func (m *xdfileModel) copyRemoteSelectionToClipboard(entries []xdfileEntry) tea.Cmd {
	paths := make([]string, 0, len(entries))
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !xdfileIsNetBoxPath(entry.Path) {
			m.setStatus("Remote clipboard copy cannot mix local and SSH items")
			return nil
		}
		paths = append(paths, entry.Path)
		names = append(names, entry.Name)
	}

	if len(paths) == 1 {
		m.setStatus("Downloading remote %s to clipboard", names[0])
	} else {
		m.setStatus("Downloading %d remote items to clipboard", len(paths))
	}
	downloadCmd := func() tea.Msg {
		localPaths, cacheDir, err := xdfileNetBoxDownloadPathsFunc(paths)
		var clipboardErr error
		if err == nil {
			clipboardErr = xdfileWriteClipboardPathsFunc(localPaths, false)
		}
		return xdfileRemoteClipboardCopyResultMsg{
			Paths:        localPaths,
			CacheDir:     cacheDir,
			Names:        names,
			Err:          err,
			ClipboardErr: clipboardErr,
		}
	}
	return tea.Batch(downloadCmd, m.startBackgroundTask())
}

func (m *xdfileModel) pasteClipboardToActivePanel() tea.Cmd {
	if m.backgroundTaskBusy {
		m.setStatus("Wait for the current background task to finish")
		return nil
	}

	sources, cutMode, err := m.currentClipboardPayload()
	if err != nil {
		m.setStatusErr(err)
		return nil
	}
	if len(sources) == 0 {
		m.setStatus("Clipboard is empty")
		return nil
	}
	if xdfileIsNetBoxPath(m.panels[m.activePanel].Cwd) {
		if cutMode {
			m.setStatus("SSH paste supports copied files only")
			return nil
		}
	}

	pending := &xdfilePendingClipboardPaste{
		Sources:        append([]string(nil), sources...),
		CutMode:        cutMode,
		DestinationDir: m.panels[m.activePanel].Cwd,
	}
	if err := pending.initQueue(); err != nil {
		m.setStatusErr(err)
		return nil
	}
	return m.continuePendingClipboardPaste(pending)
}

func (m *xdfileModel) openPanelContextMenu(panelIndex int, rect xdfileRect, x int, y int) tea.Cmd {
	focusCmd := m.focusPanel(panelIndex)
	panel := &m.panels[panelIndex]
	rows := panel.visibleRows(rect.h)
	row := y - (rect.y + 3)
	index := -1
	if row >= 0 && row < rows {
		candidate := panel.Scroll + row
		if candidate >= 0 && candidate < len(panel.Entries) {
			index = candidate
			entry := panel.Entries[candidate]
			if entry.IsParent || !panel.isMarked(entry) {
				panel.clearMarked()
			}
			panel.setCursor(candidate, rows)
		}
	}
	if index < 0 {
		panel.clearMarked()
	}
	m.panelMouse = xdfilePanelMouseState{}
	m.lastClick = xdfileClickState{panel: -1, row: -1}
	m.syncQuickViewViewport()

	m.contextMenu = xdfileMenu{
		Action: xdfileActionContextMenu,
		Label:  "Context",
		Items:  m.panelContextMenuItems(panelIndex, index),
	}
	m.contextMenuAnchor = xdfileRect{x: x, y: y, w: 1, h: 1}
	m.openMenu = xdfileActionContextMenu
	m.menuCursor = xdfileFirstSelectableMenuIndex(m.contextMenu)
	m.clearMouseHover()
	m.setStatus("Opened context menu")
	return focusCmd
}

func (m *xdfileModel) activeNativeContextMenuPaths() []string {
	entries := m.activeFileSelectionEntries()
	if len(entries) == 0 {
		return nil
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsParent || xdfileIsNetBoxPath(entry.Path) {
			continue
		}
		paths = append(paths, entry.Path)
	}
	return paths
}

func (m *xdfileModel) openWindowsContextMenuForSelection() tea.Cmd {
	paths := m.activeNativeContextMenuPaths()
	if len(paths) == 0 {
		m.setStatus("Select a local file or directory first")
		return nil
	}
	if err := xdfileShowNativeContextMenuFunc(paths); err != nil {
		m.setStatusErr(err)
		return nil
	}
	m.reloadAllPanels()
	m.setStatus("Closed Windows context menu")
	return nil
}

func (m *xdfileModel) panelContextMenuItems(panelIndex int, entryIndex int) []xdfileButton {
	canPaste := m.canPasteClipboardFiles()
	items := []xdfileButton{
		{Action: xdfileActionPaste, Key: "Ctrl+Shift+V", Label: "Paste", Disabled: !canPaste},
		{Action: xdfileActionRefresh, Label: "Refresh"},
		{Action: xdfileActionMkdir, Key: "F7", Label: "New folder"},
		{Action: xdfileActionTerminalExpand, Key: "Ctrl+O", Label: "Expand terminal"},
		{Action: xdfileActionHidden, Key: "F9", Label: xdfileHiddenLabel(m.showHidden)},
	}

	if entryIndex < 0 || entryIndex >= len(m.panels[panelIndex].Entries) {
		return items
	}

	entry := m.panels[panelIndex].Entries[entryIndex]
	remoteEntry := xdfileIsNetBoxPath(entry.Path)
	remoteFile := remoteEntry && !entry.IsDir && !entry.IsParent
	return []xdfileButton{
		{Action: xdfileActionWindowsContextMenu, Label: "Windows menu", Disabled: entry.IsParent || remoteEntry},
		{Action: xdfileActionOpen, Key: "Enter", Label: "Open", Disabled: remoteFile},
		{Action: xdfileActionPreview, Key: "Ctrl+Q", Label: "Preview", Disabled: entry.IsParent || remoteEntry},
		{Action: xdfileActionProperties, Key: "R", Label: "Properties", Disabled: entry.IsParent || remoteEntry},
		{Action: xdfileActionClipboardCopy, Key: "Ctrl+Shift+C", Label: "Copy", Disabled: entry.IsParent},
		{Action: xdfileActionClipboardCut, Key: "Ctrl+X", Label: "Cut", Disabled: entry.IsParent || remoteEntry},
		{Action: xdfileActionPaste, Key: "Ctrl+Shift+V", Label: "Paste", Disabled: !canPaste},
		{Action: xdfileActionRename, Key: "F4", Label: "Rename", Disabled: entry.IsParent},
		{Action: xdfileActionDelete, Key: "F8", Label: "Delete", Disabled: entry.IsParent || remoteEntry},
		{Action: xdfileActionRefresh, Label: "Refresh"},
		{Action: xdfileActionTerminalExpand, Key: "Ctrl+O", Label: "Expand terminal"},
	}
}

func (m *xdfileModel) openPreview() tea.Cmd {
	panel := &m.panels[m.activePanel]
	entry, ok := panel.selected()
	if !ok {
		return nil
	}
	if entry.IsParent {
		m.setStatus("Parent shortcut cannot be previewed")
		return nil
	}
	if xdfileIsNetBoxPath(entry.Path) {
		m.setStatus("Remote preview is unavailable; copy it locally first")
		return nil
	}
	vp := viewport.New(0, 0)
	vp.MouseWheelEnabled = false
	m.modal = xdfileModal{
		Kind:        xdfileModalText,
		Title:       "Preview: " + entry.Name,
		Action:      xdfileActionPreview,
		Input:       m.modalInputModel(),
		Viewport:    vp,
		Text:        "",
		PreviewPath: entry.Path,
	}
	m.syncModalViewport()
	return nil
}

func (m *xdfileModel) openProperties() tea.Cmd {
	panel := &m.panels[m.activePanel]
	entry, ok := panel.selected()
	if !ok || entry.IsParent {
		m.setStatus("Select a file or directory to view properties")
		return nil
	}
	if xdfileIsNetBoxPath(entry.Path) {
		m.setStatus("Remote properties are unavailable")
		return nil
	}

	if err := xdfileShowSystemPropertiesFunc(entry.Path); err != nil {
		m.setStatusErr(err)
		return nil
	}

	m.setStatus("Opened Windows properties for %s", entry.Name)
	return nil
}

func (m *xdfileModel) openQuickView() tea.Cmd {
	if len(m.panels) < 2 {
		return nil
	}

	panel := &m.panels[m.activePanel]
	if _, ok := panel.selected(); !ok {
		return nil
	}

	m.quickView.Open = true
	m.quickView.Binary = false
	m.quickView.Path = ""
	m.quickView.ContentW = 0
	m.quickView.ContentH = 0
	m.syncQuickViewViewport()
	m.setStatus("Quick view opened")
	return nil
}

func (m *xdfileModel) closeQuickView() {
	m.quickView.Open = false
	m.quickView.Binary = false
	m.quickView.Path = ""
	m.quickView.Title = ""
	m.quickView.Description = ""
	m.quickView.Text = ""
	m.quickView.Visual = false
	m.quickView.ContentW = 0
	m.quickView.ContentH = 0
}

func (m *xdfileModel) toggleQuickView() tea.Cmd {
	if m.quickViewActive() {
		m.closeQuickView()
		m.setStatus("Closed quick view")
		return nil
	}
	return m.openQuickView()
}

func (m *xdfileModel) togglePreview() tea.Cmd {
	if m.layoutPrefs.QuickViewDocked {
		return m.toggleQuickView()
	}
	if m.modal.Kind == xdfileModalText && m.modal.Action == xdfileActionPreview {
		m.closeModal()
		m.setStatus("Closed preview")
		return nil
	}
	return m.openPreview()
}

func (m *xdfileModel) togglePreviewBinary() tea.Cmd {
	if m.quickViewActive() {
		if !xdfilePreviewCanToggleBinary(m.quickView.Path) {
			return nil
		}
		m.quickView.Binary = !m.quickView.Binary
		m.quickView.ContentW = 0
		m.quickView.ContentH = 0
		m.syncQuickViewViewport()
		if m.quickView.Binary {
			m.setStatus("Quick view switched to binary")
		} else {
			m.setStatus("Quick view switched to normal")
		}
		return nil
	}
	if m.modal.Action != xdfileActionPreview || m.modal.PreviewPath == "" {
		return nil
	}
	if !xdfilePreviewCanToggleBinary(m.modal.PreviewPath) {
		return nil
	}

	m.modal.PreviewBinary = !m.modal.PreviewBinary
	m.syncModalViewport()
	if m.modal.PreviewBinary {
		m.setStatus("Preview switched to binary")
	} else {
		m.setStatus("Preview switched to normal")
	}
	return nil
}

func (m *xdfileModel) openRename() tea.Cmd {
	panel := &m.panels[m.activePanel]
	entry, ok := panel.selected()
	if !ok || entry.IsParent {
		m.setStatus("Select a file or directory to rename")
		return nil
	}
	m.openInputModal(
		xdfileActionModalRename,
		"Rename Item",
		"Rename the selected item inside the active panel.",
		m.activePanel,
		entry.Path,
		entry.Name,
	)
	return nil
}

func (m *xdfileModel) openTransferConfirm(action xdfileAction) tea.Cmd {
	entries := m.activeFileSelectionEntries()
	if len(entries) == 0 {
		m.setStatus("Select a file or directory first")
		return nil
	}
	for _, entry := range entries {
		if xdfileIsNetBoxPath(entry.Path) || xdfileIsNetBoxPath(m.panels[1-m.activePanel].Cwd) {
			m.setStatus("Panel-to-panel SSH copy/move is unavailable; use clipboard copy/paste")
			return nil
		}
	}
	dstPanel := &m.panels[1-m.activePanel]

	sourcePaths := make([]string, 0, len(entries))
	for _, entry := range entries {
		sourcePaths = append(sourcePaths, entry.Path)
	}

	targetPath := dstPanel.Cwd
	if len(entries) == 1 {
		targetPath = xdfileJoinPath(dstPanel.Cwd, entries[0].Name)
	}

	title := "Copy Item"
	desc := fmt.Sprintf("Copy %s into %s", entries[0].Path, dstPanel.Cwd)
	if len(entries) > 1 {
		title = "Copy Items"
		desc = fmt.Sprintf("Copy %d selected items into %s", len(entries), dstPanel.Cwd)
	}
	if action == xdfileActionMove {
		title = "Move Item"
		desc = fmt.Sprintf("Move %s into %s", entries[0].Path, dstPanel.Cwd)
		if len(entries) > 1 {
			title = "Move Items"
			desc = fmt.Sprintf("Move %d selected items into %s", len(entries), dstPanel.Cwd)
		}
	}

	m.modal = xdfileModal{
		Kind:        xdfileModalConfirm,
		Title:       title,
		Description: desc,
		Action:      action,
		Input:       m.modalInputModel(),
		SourcePath:  entries[0].Path,
		SourcePaths: sourcePaths,
		TargetPath:  targetPath,
		PanelIndex:  m.activePanel,
	}
	m.setStatus("Press Enter to confirm or Esc to cancel")
	return nil
}

func (m *xdfileModel) openDeleteConfirm() tea.Cmd {
	panel := &m.panels[m.activePanel]
	entries := m.activeFileSelectionEntries()
	if len(entries) == 0 {
		m.setStatus("Select a file or directory to delete")
		return nil
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if xdfileIsNetBoxPath(entry.Path) {
			m.setStatus("Remote delete is unavailable")
			return nil
		}
		paths = append(paths, entry.Path)
	}
	title := "Delete Item"
	desc := fmt.Sprintf("Delete %s", paths[0])
	if len(paths) > 1 {
		title = "Delete Items"
		desc = fmt.Sprintf("Delete %d selected items from %s", len(paths), panel.Cwd)
	}
	m.modal = xdfileModal{
		Kind:        xdfileModalConfirm,
		Title:       title,
		Description: desc,
		Action:      xdfileActionDelete,
		Input:       m.modalInputModel(),
		SourcePath:  paths[0],
		SourcePaths: paths,
		PanelIndex:  m.activePanel,
	}
	m.setStatus("Press Enter to delete or Esc to cancel")
	return nil
}

func (m *xdfileModel) openQuitConfirm() tea.Cmd {
	m.modal = xdfileModal{
		Kind:        xdfileModalConfirm,
		Title:       "Quit Xdfile Manager",
		Description: "Exit Xdfile Manager now?",
		Action:      xdfileActionQuit,
		Input:       m.modalInputModel(),
	}
	m.setStatus("Press Enter to quit or Esc to cancel")
	return nil
}

func (m *xdfileModel) openInputModal(
	action xdfileAction,
	title string,
	description string,
	panelIndex int,
	sourcePath string,
	initial string,
) {
	m.modal = xdfileModal{
		Kind:        xdfileModalInput,
		Title:       title,
		Description: description,
		Action:      action,
		Input:       m.modalInputModel(),
		SourcePath:  sourcePath,
		PanelIndex:  panelIndex,
	}
	m.modal.Input.SetValue(initial)
	m.modal.Input.CursorEnd()
	m.modal.Input.Placeholder = "Type here"
	_ = m.modal.Input.Focus()
}

func (m *xdfileModel) newModalFormField(label string, placeholder string, initial string) xdfileModalField {
	input := m.modalInputModel()
	input.SetValue(initial)
	input.CursorEnd()
	input.Placeholder = placeholder
	input.CharLimit = 4096
	input.Blur()
	return xdfileModalField{
		Label: label,
		Input: input,
	}
}

func (m *xdfileModel) newModalPasswordField(label string, placeholder string, initial string) xdfileModalField {
	field := m.newModalFormField(label, placeholder, initial)
	field.Input.EchoMode = textinput.EchoPassword
	field.Input.EchoCharacter = '*'
	return field
}

func (m *xdfileModel) newModalFormMultilineField(label string, placeholder string, initial string) xdfileModalField {
	input := m.modalInputModel()
	input.SetValue(initial)

	area := textarea.New()
	area.Prompt = ""
	area.Placeholder = placeholder
	area.SetValue(initial)
	area.CharLimit = 8192
	area.ShowLineNumbers = false
	area.EndOfBufferCharacter = ' '
	area.SetHeight(5)
	area.SetWidth(40)

	field := xdfileModalField{
		Label:     label,
		Input:     input,
		TextArea:  area,
		Multiline: true,
	}
	return field
}

func (m *xdfileModel) openChoiceModal(title string, description string, items []xdfileModalChoiceItem) {
	m.modal = xdfileModal{
		Kind:        xdfileModalChoice,
		Title:       title,
		Description: description,
		ChoiceItems: append([]xdfileModalChoiceItem(nil), items...),
		Input:       m.modalInputModel(),
		Viewport:    m.modal.Viewport,
	}
	m.setStatus("Opened %s", title)
}

func (m *xdfileModel) openFormModal(action xdfileAction, title string, description string, fields []xdfileModalField) {
	m.modal = xdfileModal{
		Kind:        xdfileModalForm,
		Title:       title,
		Description: description,
		Action:      action,
		FormFields:  append([]xdfileModalField(nil), fields...),
		Input:       m.modalInputModel(),
		Viewport:    m.modal.Viewport,
	}
	m.syncThemeInputStyles()
	m.focusModalFormField(0)
	m.setStatus("Opened %s", title)
}

func (m *xdfileModel) focusModalFormField(index int) {
	if len(m.modal.FormFields) == 0 {
		m.modal.FormCursor = 0
		return
	}

	index = max(0, min(index, len(m.modal.FormFields)-1))
	m.modal.FormCursor = index
	for i := range m.modal.FormFields {
		if i == index {
			m.modal.FormFields[i].Focus()
			continue
		}
		m.modal.FormFields[i].Blur()
	}
}

func (m *xdfileModel) openTextModal(title string, text string) {
	m.openTextModalWithAction("", title, text, "Enter/Esc close | Up/Down/PgUp/PgDn scroll | Wheel scroll")
}

func (m *xdfileModel) openTextModalWithAction(action xdfileAction, title string, text string, description string) {
	vp := viewport.New(0, 0)
	vp.MouseWheelEnabled = false
	m.modal = xdfileModal{
		Kind:        xdfileModalText,
		Title:       title,
		Description: description,
		Action:      action,
		Viewport:    vp,
		Text:        text,
		Input:       m.modalInputModel(),
	}
	m.syncModalViewport()
	m.setStatus("Opened %s", title)
}

func (m *xdfileModel) closeModal() {
	if m.modal.Action == xdfileActionPasteConflictPrompt {
		m.pendingClipboardPaste = nil
	}
	m.modal = xdfileModal{
		Input:         m.modalInputModel(),
		Viewport:      m.modal.Viewport,
		PreviewBinary: false,
		PreviewVisual: false,
	}
	m.modal.Input.Blur()
}

func (m *xdfileModel) closeModalAndResumeFileQueue() tea.Cmd {
	action := m.modal.Action
	m.closeModal()
	if action == xdfileActionFileOperationErrors {
		return m.startNextQueuedFileOperation()
	}
	return nil
}

func (m *xdfileModel) closeXdfileResources() {
	if m.fileOperationCancel != nil {
		m.fileOperationCancel()
		m.fileOperationCancel = nil
	}
	m.fileOperationQueue = nil
	m.closeExclusiveTerminalSession()
	m.closeTerminalSession()
	m.cleanupAllCommandMenuTempFiles()
	m.cleanupRemoteClipboardDirs()
	m.cleanupClipboardMoveUndoStack()
	m.cleanupDeleteUndoStack()
	if m.thumbnailGenerator != nil {
		_ = m.thumbnailGenerator.CleanUp()
	}
}
