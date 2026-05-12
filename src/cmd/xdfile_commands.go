package cmd

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	uv "github.com/charmbracelet/ultraviolet"
)

const (
	xdfileCommandRunActionPrefix  = "command_run:"
	xdfileCommandOpenActionPrefix = "command_open:"

	xdfileCommandItemTypeCommand = "command"
	xdfileCommandItemTypeMenu    = "menu"
)

type xdfileCommandItem struct {
	Type    string              `json:"type,omitempty"`
	Hotkey  string              `json:"hotkey"`
	Label   string              `json:"label"`
	Command string              `json:"command,omitempty"`
	Items   []xdfileCommandItem `json:"items,omitempty"`
}

var xdfileExecuteCommandMenuFunc = func(m *xdfileModel, command string) tea.Cmd {
	return m.executeCommandMenuCommand(command)
}

func xdfileNormalizeCommandItems(items []xdfileCommandItem) []xdfileCommandItem {
	normalized := make([]xdfileCommandItem, 0, len(items))
	for _, item := range items {
		item = item.normalized()
		if item.Label == "" {
			continue
		}
		if item.isMenu() {
			normalized = append(normalized, item)
			continue
		}
		if item.Command == "" {
			continue
		}
		normalized = append(normalized, item)
	}
	return normalized
}

func (c xdfileCommandItem) normalized() xdfileCommandItem {
	c.Type = xdfileNormalizeCommandItemType(c.Type, len(c.Items) > 0)
	c.Hotkey = xdfileNormalizeCommandHotkey(c.Hotkey)
	c.Label = strings.TrimSpace(c.Label)
	c.Items = xdfileNormalizeCommandItems(c.Items)
	if c.isMenu() {
		c.Command = ""
		return c
	}
	c.Command = strings.TrimSpace(c.Command)
	c.Items = nil
	return c
}

func xdfileNormalizeCommandItemType(value string, hasItems bool) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case xdfileCommandItemTypeMenu:
		return xdfileCommandItemTypeMenu
	case xdfileCommandItemTypeCommand:
		return xdfileCommandItemTypeCommand
	default:
		if hasItems {
			return xdfileCommandItemTypeMenu
		}
		return xdfileCommandItemTypeCommand
	}
}

func (c xdfileCommandItem) isMenu() bool {
	return xdfileNormalizeCommandItemType(c.Type, len(c.Items) > 0) == xdfileCommandItemTypeMenu
}

func xdfileNormalizeCommandHotkey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func xdfileCommandRunAction(index int) xdfileAction {
	return xdfileAction(fmt.Sprintf("%s%d", xdfileCommandRunActionPrefix, index))
}

func xdfileCommandOpenAction(index int) xdfileAction {
	return xdfileAction(fmt.Sprintf("%s%d", xdfileCommandOpenActionPrefix, index))
}

func xdfileParseCommandRunAction(action xdfileAction) (int, bool) {
	return xdfileParseCommandIndexedAction(action, xdfileCommandRunActionPrefix)
}

func xdfileParseCommandOpenAction(action xdfileAction) (int, bool) {
	return xdfileParseCommandIndexedAction(action, xdfileCommandOpenActionPrefix)
}

func xdfileParseCommandIndexedAction(action xdfileAction, prefix string) (int, bool) {
	raw := string(action)
	if !strings.HasPrefix(raw, prefix) {
		return 0, false
	}

	var index int
	if _, err := fmt.Sscanf(raw, prefix+"%d", &index); err != nil {
		return 0, false
	}
	return index, true
}

func (m *xdfileModel) openCommandMenu() tea.Cmd {
	m.commandMenuPath = nil
	m.openMenu = xdfileActionCommandsMenu
	m.menuCursor = xdfileFirstSelectableMenuIndex(m.commandMenuDefinition())
	m.clearMouseHover()
	m.setStatus("F2 user menu: Enter run/open | Ins insert | F4 edit | Del delete | Esc close")
	return nil
}

func (m *xdfileModel) commandMenuDefinition() xdfileMenu {
	current := m.currentCommandMenuItems()
	items := make([]xdfileButton, 0, len(current))
	for i, item := range current {
		action := xdfileCommandRunAction(i)
		label := item.Label
		if item.isMenu() {
			action = xdfileCommandOpenAction(i)
			label += " >"
		}
		items = append(items, xdfileButton{
			Action: action,
			Key:    item.Hotkey,
			Label:  label,
		})
	}
	if len(items) == 0 {
		items = append(items, xdfileButton{
			Label:    "No items in this menu",
			Disabled: true,
		})
	}
	return xdfileMenu{
		Action: xdfileActionCommandsMenu,
		Label:  m.commandMenuPathLabel(),
		Items:  items,
	}
}

func (m *xdfileModel) commandMenuPathLabel() string {
	if len(m.commandMenuPath) == 0 {
		return "User menu"
	}

	items := m.layoutPrefs.Commands
	label := "User menu"
	for _, index := range m.commandMenuPath {
		if index < 0 || index >= len(items) {
			break
		}
		label = items[index].Label
		items = items[index].Items
	}
	return label
}

func (m *xdfileModel) currentCommandMenuItems() []xdfileCommandItem {
	itemsPtr := m.commandItemsPtrAtPath(m.commandMenuPath)
	if itemsPtr == nil {
		return nil
	}
	return *itemsPtr
}

func (m *xdfileModel) commandItemsPtrAtPath(path []int) *[]xdfileCommandItem {
	items := &m.layoutPrefs.Commands
	for _, index := range path {
		if index < 0 || index >= len(*items) {
			return nil
		}
		entry := &(*items)[index]
		if !entry.isMenu() {
			return nil
		}
		items = &entry.Items
	}
	return items
}

func (m *xdfileModel) commandMenuIndexForHotkey(hotkey string) (int, bool) {
	hotkey = xdfileNormalizeCommandHotkey(hotkey)
	if hotkey == "" {
		return 0, false
	}
	for i, item := range m.currentCommandMenuItems() {
		if xdfileNormalizeCommandHotkey(item.Hotkey) == hotkey {
			return i, true
		}
	}
	return 0, false
}

func (m *xdfileModel) selectedCommandMenuIndex() (int, bool) {
	if m.openMenu != xdfileActionCommandsMenu {
		return 0, false
	}
	items := m.currentCommandMenuItems()
	if m.menuCursor < 0 || m.menuCursor >= len(items) {
		return 0, false
	}
	return m.menuCursor, true
}

func (m *xdfileModel) selectedCommandMenuItem() (*xdfileCommandItem, bool) {
	index, ok := m.selectedCommandMenuIndex()
	if !ok {
		return nil, false
	}
	items := m.commandItemsPtrAtPath(m.commandMenuPath)
	if items == nil || index < 0 || index >= len(*items) {
		return nil, false
	}
	return &(*items)[index], true
}

func (m *xdfileModel) hoveredCommandMenuIndex() (int, bool) {
	if m.openMenu != xdfileActionCommandsMenu {
		return 0, false
	}
	items := m.currentCommandMenuItems()
	index := m.hover.MenuItem
	if index < 0 || index >= len(items) {
		return 0, false
	}
	return index, true
}

func (m *xdfileModel) commandMenuActionIndex() (int, bool) {
	if index, ok := m.hoveredCommandMenuIndex(); ok {
		return index, true
	}
	return m.selectedCommandMenuIndex()
}

func xdfileCloneCommandPath(path []int) []int {
	return append([]int(nil), path...)
}

func xdfileInsertCommandItem(items []xdfileCommandItem, index int, item xdfileCommandItem) []xdfileCommandItem {
	index = max(0, min(index, len(items)))
	inserted := make([]xdfileCommandItem, len(items)+1)
	copy(inserted, items[:index])
	inserted[index] = item
	copy(inserted[index+1:], items[index:])
	return inserted
}

func xdfileDeleteCommandItem(items []xdfileCommandItem, index int) []xdfileCommandItem {
	if index < 0 || index >= len(items) {
		return items
	}
	copy(items[index:], items[index+1:])
	items[len(items)-1] = xdfileCommandItem{}
	return items[:len(items)-1]
}

func (m *xdfileModel) beginInsertCommandMenuItem() {
	m.commandInsertPath = xdfileCloneCommandPath(m.commandMenuPath)
	items := m.currentCommandMenuItems()
	if index, ok := m.commandMenuActionIndex(); ok && len(items) > 0 {
		m.commandInsertIndex = index + 1
	} else {
		m.commandInsertIndex = len(items)
	}
	m.openCommandInsertChoiceModal()
}

func (m *xdfileModel) openCommandInsertChoiceModal() {
	m.openChoiceModal(
		"Insert Into User Menu",
		"Choose what to insert below the current item.",
		[]xdfileModalChoiceItem{
			{Action: xdfileActionInsertCommand, Label: "Command", Description: "Insert a runnable command with hot key, label, and command line."},
			{Action: xdfileActionInsertMenu, Label: "Menu", Description: "Insert a nested submenu with hot key and label."},
		},
	)
}

func (m *xdfileModel) openCommandItemForm(itemType string) {
	m.openCommandItemFormWithValues("", itemType, "", "", "")
}

func (m *xdfileModel) openCommandItemFormWithValues(action xdfileAction, itemType string, hotkey string, label string, command string) {
	if itemType == xdfileCommandItemTypeMenu {
		if action == "" {
			action = xdfileActionInsertMenu
		}
		title := "Insert User Menu"
		description := "Fill in the submenu properties. Leave Hot key empty if you only want to open it from the menu."
		if action == xdfileActionEditMenu {
			title = "Edit User Menu"
			description = "Update the submenu properties. Leave Hot key empty if you only want to open it from the menu."
		}
		m.openFormModal(
			action,
			title,
			description,
			[]xdfileModalField{
				m.newModalFormField("Hot key", "Optional hot key", hotkey),
				m.newModalFormField("Label", "Menu label", label),
			},
		)
		return
	}

	if action == "" {
		action = xdfileActionInsertCommand
	}
	title := "Insert User Command"
	description := `Fill in the command properties. User-menu metasymbols are supported, including !.!, !&, !@!, !? and panel toggles like !# / !^. Use Shift+Enter inside Command to add a new line.`
	if action == xdfileActionEditCommand {
		title = "Edit User Command"
		description = `Update the command properties. User-menu metasymbols are supported, including !.!, !&, !@!, !? and panel toggles like !# / !^. Use Shift+Enter inside Command to add a new line.`
	}
	m.openFormModal(
		action,
		title,
		description,
		[]xdfileModalField{
			m.newModalFormField("Hot key", "Optional hot key", hotkey),
			m.newModalFormField("Label", "Menu label", label),
			m.newModalFormMultilineField("Command", "Command line", command),
		},
	)
}

func (m *xdfileModel) commandInsertTarget() (*[]xdfileCommandItem, int, bool) {
	items := m.commandItemsPtrAtPath(m.commandInsertPath)
	if items == nil {
		return nil, 0, false
	}
	index := max(0, min(m.commandInsertIndex, len(*items)))
	return items, index, true
}

func (m *xdfileModel) insertCommandMenuItem(item xdfileCommandItem) tea.Cmd {
	items, index, ok := m.commandInsertTarget()
	if !ok {
		m.setStatus("The target user menu no longer exists")
		return nil
	}

	item = item.normalized()
	*items = xdfileInsertCommandItem(*items, index, item)

	if err := m.saveCommandMenuPrefs(); err != nil {
		m.setStatusErr(err)
		return nil
	}

	m.closeModal()
	m.openMenu = xdfileActionCommandsMenu
	m.commandMenuPath = xdfileCloneCommandPath(m.commandInsertPath)
	m.menuCursor = index
	if item.isMenu() {
		m.setStatus("Inserted submenu %s", item.Label)
	} else {
		m.setStatus("Inserted command %s", item.Label)
	}
	return nil
}

func (m *xdfileModel) beginEditCommandMenuItem() tea.Cmd {
	index, ok := m.commandMenuActionIndex()
	if !ok {
		return nil
	}
	items := m.commandItemsPtrAtPath(m.commandMenuPath)
	if items == nil || index < 0 || index >= len(*items) {
		return nil
	}
	item := &(*items)[index]

	m.commandEditPath = xdfileCloneCommandPath(m.commandMenuPath)
	m.commandEditIndex = index
	if item.isMenu() {
		m.openCommandItemFormWithValues(xdfileActionEditMenu, xdfileCommandItemTypeMenu, item.Hotkey, item.Label, "")
		return nil
	}
	m.openCommandItemFormWithValues(xdfileActionEditCommand, xdfileCommandItemTypeCommand, item.Hotkey, item.Label, item.Command)
	return nil
}

func (m *xdfileModel) commandEditTarget() (*[]xdfileCommandItem, int, bool) {
	items := m.commandItemsPtrAtPath(m.commandEditPath)
	if items == nil || len(*items) == 0 {
		return nil, 0, false
	}
	index := max(0, min(m.commandEditIndex, len(*items)-1))
	return items, index, true
}

func (m *xdfileModel) updateCommandMenuItem(item xdfileCommandItem) tea.Cmd {
	items, index, ok := m.commandEditTarget()
	if !ok {
		m.setStatus("The target user menu no longer exists")
		return nil
	}

	item = item.normalized()
	(*items)[index] = item

	if err := m.saveCommandMenuPrefs(); err != nil {
		m.setStatusErr(err)
		return nil
	}

	m.closeModal()
	m.openMenu = xdfileActionCommandsMenu
	m.commandMenuPath = xdfileCloneCommandPath(m.commandEditPath)
	m.menuCursor = index
	if item.isMenu() {
		m.setStatus("Updated submenu %s", item.Label)
	} else {
		m.setStatus("Updated command %s", item.Label)
	}
	return nil
}

func (m *xdfileModel) deleteSelectedCommandMenuItem() tea.Cmd {
	index, ok := m.selectedCommandMenuIndex()
	if !ok {
		return nil
	}

	items := m.commandItemsPtrAtPath(m.commandMenuPath)
	if items == nil || index < 0 || index >= len(*items) {
		return nil
	}

	label := (*items)[index].Label
	*items = xdfileDeleteCommandItem(*items, index)
	if err := m.saveCommandMenuPrefs(); err != nil {
		m.setStatusErr(err)
		return nil
	}

	if len(*items) == 0 {
		m.menuCursor = 0
	} else {
		m.menuCursor = min(index, len(*items)-1)
	}
	m.setStatus("Deleted menu item %s", label)
	return nil
}

func (m *xdfileModel) openCommandSubmenuIndex(index int) tea.Cmd {
	items := m.currentCommandMenuItems()
	if index < 0 || index >= len(items) || !items[index].isMenu() {
		return nil
	}

	m.commandMenuPath = append(m.commandMenuPath, index)
	m.menuCursor = xdfileFirstSelectableMenuIndex(m.commandMenuDefinition())
	m.clearMouseHover()
	m.setStatus("Opened submenu %s", items[index].Label)
	return nil
}

func (m *xdfileModel) closeCommandSubmenu() bool {
	if len(m.commandMenuPath) == 0 {
		return false
	}

	parentIndex := m.commandMenuPath[len(m.commandMenuPath)-1]
	m.commandMenuPath = xdfileCloneCommandPath(m.commandMenuPath[:len(m.commandMenuPath)-1])
	m.menuCursor = parentIndex
	m.clearMouseHover()
	m.setStatus("Returned to %s", m.commandMenuPathLabel())
	return true
}

func (m *xdfileModel) executeCommandMenuIndex(index int) tea.Cmd {
	items := m.currentCommandMenuItems()
	if index < 0 || index >= len(items) {
		return nil
	}

	item := items[index]
	expansion, err := m.prepareCommandMenuCommand(item.Command)
	if err != nil {
		m.setStatusErr(err)
		return nil
	}

	m.openMenu = ""
	m.menuCursor = 0
	return m.openCommandMenuPromptForm(item.Label, expansion)
}

func (m *xdfileModel) executeCommandMenuCommand(command string) tea.Cmd {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil
	}

	_ = m.syncTerminalToPanel(m.activePanel)
	if !xdfileIsNetBoxPath(m.terminal.Cwd) {
		if detached, handled := xdfileStartDetachedExternalCommand(m.terminal.Cwd, command); handled {
			m.pushTerminalHistory(command)
			return func() tea.Msg { return detached }
		}
	}

	if m.terminalUsesPTY() {
		if m.terminal.Emulator == nil || m.terminal.Emulator.IsAltScreen() {
			m.setStatus("Leave the fullscreen terminal app before running a command menu item")
			return nil
		}
		m.pushTerminalHistory(command)
		m.terminalFocused = true
		m.terminal.ScrollOffset = 0
		m.terminal.Emulator.SendText(command)
		m.terminal.Emulator.SendKey(uv.KeyPressEvent{Code: uv.KeyEnter})
		return m.beginPTYCommandTracking(command)
	}

	submitCmd := m.submitTerminalCommand(command)
	return submitCmd
}

func (m *xdfileModel) saveCommandMenuPrefs() error {
	if m.commandsFile == "" {
		return nil
	}
	return xdfileSaveCommandPrefs(m.commandsFile, m.layoutPrefs.Commands)
}
