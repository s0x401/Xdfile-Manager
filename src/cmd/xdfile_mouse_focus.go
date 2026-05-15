package cmd

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	rw "github.com/mattn/go-runewidth"
)

const xdfileNoHoverIndex = -1

type xdfilePanelMouseBlank int

const (
	xdfilePanelMouseBlankBottom xdfilePanelMouseBlank = iota
	xdfilePanelMouseBlankTop
)

type xdfilePanelMouseHit struct {
	Panel      int
	Rows       int
	EntryIndex int
	OnEntry    bool
	Blank      xdfilePanelMouseBlank
}

func xdfileNoHoverState() xdfileHoverState {
	return xdfileHoverState{
		MenuItem:   xdfileNoHoverIndex,
		Panel:      xdfileNoHoverIndex,
		PanelIndex: xdfileNoHoverIndex,
	}
}

func (m *xdfileModel) clearMouseHover() {
	m.hover = xdfileNoHoverState()
}

func (m *xdfileModel) updateMouseHover(msg tea.MouseMsg) {
	m.setMouseHover(m.mouseHoverAt(msg.X, msg.Y))
}

func (m *xdfileModel) setMouseHover(next xdfileHoverState) {
	if m.hover == next {
		return
	}
	m.hover = next
}

func (m *xdfileModel) mouseHoverAt(x int, y int) xdfileHoverState {
	for _, hit := range m.layout.menuButtons {
		if hit.Rect.contains(x, y) {
			hover := xdfileNoHoverState()
			hover.MenuAction = hit.Action
			return hover
		}
	}

	if m.openMenu != "" {
		for index, hit := range m.layout.menuItemRects {
			if hit.Rect.contains(x, y) {
				hover := xdfileNoHoverState()
				hover.MenuItem = index
				return hover
			}
		}
		return xdfileNoHoverState()
	}

	for _, hit := range m.layout.footerButtons {
		if hit.Rect.contains(x, y) {
			hover := xdfileNoHoverState()
			hover.FooterAction = hit.Action
			return hover
		}
	}

	if m.layout.terminalRect.contains(x, y) {
		return xdfileNoHoverState()
	}

	for i, rect := range m.layout.panelRects {
		if m.quickViewActive() && i == m.quickViewPanelIndex() {
			continue
		}
		if !rect.contains(x, y) {
			continue
		}
		if index, ok := m.panelEntryIndexAt(i, rect, y); ok {
			hover := xdfileNoHoverState()
			hover.Panel = i
			hover.PanelIndex = index
			return hover
		}
		return xdfileNoHoverState()
	}

	return xdfileNoHoverState()
}

func (m *xdfileModel) panelEntryIndexAt(panelIndex int, rect xdfileRect, y int) (int, bool) {
	if panelIndex < 0 || panelIndex >= len(m.panels) {
		return 0, false
	}
	row := y - (rect.y + 3)
	rows := m.panels[panelIndex].visibleRows(rect.h)
	if row < 0 || row >= rows {
		return 0, false
	}
	index := m.panels[panelIndex].Scroll + row
	if index < 0 || index >= len(m.panels[panelIndex].Entries) {
		return 0, false
	}
	return index, true
}

func (m *xdfileModel) panelMouseHitAt(x int, y int) (xdfilePanelMouseHit, bool) {
	for i, rect := range m.layout.panelRects {
		if m.quickViewActive() && i == m.quickViewPanelIndex() {
			continue
		}
		if !rect.contains(x, y) {
			continue
		}
		panel := &m.panels[i]
		rows := panel.visibleRows(rect.h)
		row := y - (rect.y + 3)
		hit := xdfilePanelMouseHit{
			Panel:      i,
			Rows:       rows,
			EntryIndex: -1,
		}
		switch {
		case row < 0:
			hit.Blank = xdfilePanelMouseBlankTop
		case row >= rows:
			hit.Blank = xdfilePanelMouseBlankBottom
		default:
			index := panel.Scroll + row
			if index >= 0 && index < len(m.panels[i].Entries) {
				hit.EntryIndex = index
				hit.OnEntry = true
			} else {
				hit.Blank = xdfilePanelMouseBlankBottom
			}
		}
		return hit, true
	}
	return xdfilePanelMouseHit{}, false
}

func (m *xdfileModel) handleMouse(msg tea.MouseMsg) tea.Cmd {
	m.updateMouseHover(msg)
	if msg.Action == tea.MouseActionPress && m.panelSearch.Active {
		m.closePanelSearch()
	}

	if m.panelMouse.Active {
		if cmd, handled := m.handlePanelMouseContinuation(msg); handled {
			return cmd
		}
	}

	if quickViewIndex := m.quickViewPanelIndex(); quickViewIndex >= 0 && m.layout.panelRects[quickViewIndex].contains(msg.X, msg.Y) {
		if msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown {
			if !m.quickView.Visual {
				if msg.Button == tea.MouseButtonWheelUp {
					m.quickView.Viewport.LineUp(3)
				} else {
					m.quickView.Viewport.LineDown(3)
				}
			}
			return nil
		}
		if msg.Action == tea.MouseActionPress {
			if msg.Button == tea.MouseButtonLeft && !m.quickView.Visual {
				m.quickView.Viewport.LineUp(3)
			}
			if msg.Button == tea.MouseButtonRight && !m.quickView.Visual {
				m.quickView.Viewport.LineDown(3)
			}
			return nil
		}
	}

	if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonRight {
		for i, rect := range m.layout.panelRects {
			if m.quickViewActive() && i == m.quickViewPanelIndex() {
				continue
			}
			if !rect.contains(msg.X, msg.Y) {
				continue
			}
			return m.openPanelContextMenu(i, rect, msg.X, msg.Y)
		}
		if m.openMenu != "" {
			m.closeMenu("Closed menu")
		}
		if m.shouldForwardTerminalMouse() && m.layout.terminalRect.contains(msg.X, msg.Y) {
			if m.sendTerminalMouse(msg) {
				return nil
			}
		}
		return nil
	}

	if msg.Action == tea.MouseActionPress {
		for _, hit := range m.layout.menuButtons {
			if hit.Rect.contains(msg.X, msg.Y) {
				if hit.Disabled {
					return nil
				}
				return m.executeAction(hit.Action)
			}
		}
		if m.openMenu != "" {
			for _, hit := range m.layout.menuItemRects {
				if hit.Rect.contains(msg.X, msg.Y) {
					if hit.Disabled {
						return nil
					}
					return m.executeAction(hit.Action)
				}
			}
			if m.layout.menuRect.contains(msg.X, msg.Y) {
				return nil
			}
			m.closeMenu("Closed menu")
		}
		for _, hit := range m.layout.footerButtons {
			if hit.Rect.contains(msg.X, msg.Y) {
				if hit.Disabled {
					return nil
				}
				return m.executeAction(hit.Action)
			}
		}
		if cmd, handled := m.handleManagedTerminalMousePress(msg); handled {
			return cmd
		}
	}

	if msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown {
		if m.layout.terminalRect.contains(msg.X, msg.Y) {
			if m.shouldForwardTerminalMouse() {
				if m.sendTerminalMouse(msg) {
					return nil
				}
			}
			if m.terminalUsesPTY() {
				if msg.Button == tea.MouseButtonWheelUp {
					m.scrollTerminal(3)
				} else {
					m.scrollTerminal(-3)
				}
				return nil
			}
			if msg.Button == tea.MouseButtonWheelUp {
				m.terminal.Viewport.LineUp(3)
			} else {
				m.terminal.Viewport.LineDown(3)
			}
			return nil
		}

		for i, rect := range m.layout.panelRects {
			if m.quickViewActive() && i == m.quickViewPanelIndex() {
				continue
			}
			if rect.contains(msg.X, msg.Y) {
				rows := m.panels[i].visibleRows(rect.h)
				if msg.Button == tea.MouseButtonWheelUp {
					m.panels[i].scroll(-3, rows)
				} else {
					m.panels[i].scroll(3, rows)
				}
				m.updateMouseHover(msg)
				return nil
			}
		}
	}

	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		if m.shouldForwardTerminalMouse() && m.layout.terminalRect.contains(msg.X, msg.Y) {
			if m.sendTerminalMouse(msg) {
				return nil
			}
		}
		return nil
	}

	if m.layout.terminalRect.contains(msg.X, msg.Y) {
		if m.terminalUsesPTY() {
			cmd := m.focusTerminal()
			if m.shouldForwardTerminalMouse() {
				_ = m.sendTerminalMouse(msg)
			}
			return cmd
		}
		m.focusManagedTerminalInput()
		return nil
	}

	if hit, ok := m.panelMouseHitAt(msg.X, msg.Y); ok {
		return m.handlePanelMousePress(msg, hit)
	}

	return nil
}

func (m *xdfileModel) handlePanelMousePress(msg tea.MouseMsg, hit xdfilePanelMouseHit) tea.Cmd {
	focusCmd := m.focusPanel(hit.Panel)
	panel := &m.panels[hit.Panel]

	if !hit.OnEntry {
		return m.handlePanelBlankMousePress(msg, hit, focusCmd)
	}

	baseMarked := panel.cloneMarkedPaths()
	anchor := hit.EntryIndex
	rangeSelect := msg.Shift || msg.Alt
	if rangeSelect {
		anchor = panel.rangeSelectionAnchor()
	}
	m.applyPanelMouseSelection(hit.Panel, anchor, hit.EntryIndex, hit.Rows, msg.Ctrl, rangeSelect, baseMarked)
	m.panelMouse = xdfilePanelMouseState{
		Active:     true,
		Panel:      hit.Panel,
		StartIndex: anchor,
		LastIndex:  hit.EntryIndex,
		Ctrl:       msg.Ctrl,
		BaseMarked: baseMarked,
	}

	if cmd, activated := m.finishPanelMouseClick(msg, hit.Panel, hit.EntryIndex, focusCmd); activated {
		return cmd
	}
	return focusCmd
}

func (m *xdfileModel) handlePanelBlankMousePress(msg tea.MouseMsg, hit xdfilePanelMouseHit, focusCmd tea.Cmd) tea.Cmd {
	panel := &m.panels[hit.Panel]
	m.panelMouse = xdfilePanelMouseState{}

	if len(panel.Entries) == 0 {
		panel.clearMarked()
		m.lastClick = xdfileClickState{panel: -1, row: -1}
		m.syncQuickViewViewport()
		return focusCmd
	}

	target := panel.lastSelectableIndex()
	if hit.Blank == xdfilePanelMouseBlankTop {
		target = 0
	}

	baseMarked := panel.cloneMarkedPaths()
	anchor := target
	rangeSelect := msg.Shift || msg.Alt
	if rangeSelect {
		anchor = panel.rangeSelectionAnchor()
	}
	m.applyPanelMouseSelection(hit.Panel, anchor, target, hit.Rows, msg.Ctrl, rangeSelect, baseMarked)
	if cmd, activated := m.finishPanelMouseClick(msg, hit.Panel, target, focusCmd); activated {
		return cmd
	}
	return focusCmd
}

func (m *xdfileModel) finishPanelMouseClick(msg tea.MouseMsg, panelIndex int, entryIndex int, focusCmd tea.Cmd) (tea.Cmd, bool) {
	now := time.Now()
	if !msg.Ctrl && !msg.Shift && m.lastClick.panel == panelIndex && m.lastClick.row == entryIndex && now.Sub(m.lastClick.at) < 450*time.Millisecond {
		m.cancelPanelMouseInteraction()
		return tea.Batch(focusCmd, m.activateSelection()), true
	}
	m.lastClick = xdfileClickState{panel: panelIndex, row: entryIndex, at: now}
	return nil, false
}

func (m *xdfileModel) cancelPanelMouseInteraction() {
	m.panelMouse = xdfilePanelMouseState{}
	m.lastClick = xdfileClickState{panel: -1, row: -1}
}

func (m *xdfileModel) handlePanelMouseContinuation(msg tea.MouseMsg) (tea.Cmd, bool) {
	if msg.Action == tea.MouseActionRelease {
		if m.panelMouse.Dragging {
			m.lastClick = xdfileClickState{panel: -1, row: -1}
		}
		m.panelMouse = xdfilePanelMouseState{}
		return nil, true
	}
	if msg.Action != tea.MouseActionMotion {
		return nil, false
	}

	state := m.panelMouse
	index, rows, ok := m.panelMouseDragIndexAt(state.Panel, msg.X, msg.Y)
	if !ok {
		return nil, true
	}
	if index == state.LastIndex {
		return nil, true
	}

	state.LastIndex = index
	state.Dragging = true
	m.panelMouse = state
	m.applyPanelMouseDragSelection(state.Panel, state.StartIndex, index, rows, state.Ctrl, state.BaseMarked)
	return nil, true
}

func (m *xdfileModel) panelMouseDragIndexAt(panelIndex int, x int, y int) (int, int, bool) {
	if panelIndex < 0 || panelIndex >= len(m.panels) {
		return 0, 0, false
	}
	rect := m.layout.panelRects[panelIndex]
	if !rect.contains(x, y) || len(m.panels[panelIndex].Entries) == 0 {
		return 0, 0, false
	}

	rows := m.panels[panelIndex].visibleRows(rect.h)
	row := y - (rect.y + 3)
	index := m.panels[panelIndex].Scroll + row
	if row < 0 {
		index = m.panels[panelIndex].Scroll
	}
	if row >= rows {
		index = m.panels[panelIndex].Scroll + rows - 1
	}
	index = max(0, min(index, len(m.panels[panelIndex].Entries)-1))
	return index, rows, true
}

func (m *xdfileModel) applyPanelMouseSelection(panelIndex int, anchor int, target int, rows int, ctrl bool, shift bool, baseMarked map[string]struct{}) {
	panel := &m.panels[panelIndex]
	switch {
	case ctrl && shift:
		panel.selectRangeWithBase(anchor, target, rows, baseMarked)
	case shift:
		panel.selectRange(anchor, target, rows)
	case ctrl:
		panel.setCursor(target, rows)
		panel.toggleMarkedAt(target)
	default:
		panel.clearMarked()
		panel.setCursor(target, rows)
	}
	m.syncQuickViewViewport()
}

func (m *xdfileModel) applyPanelMouseDragSelection(panelIndex int, anchor int, target int, rows int, ctrl bool, baseMarked map[string]struct{}) {
	panel := &m.panels[panelIndex]
	if ctrl {
		panel.selectRangeWithBase(anchor, target, rows, baseMarked)
	} else {
		panel.selectRange(anchor, target, rows)
	}
	m.syncQuickViewViewport()
}

func (m *xdfileModel) handleManagedTerminalMousePress(msg tea.MouseMsg) (tea.Cmd, bool) {
	if m == nil || m.terminalUsesPTY() || msg.Button != tea.MouseButtonLeft {
		return nil, false
	}

	for index, hit := range m.layout.terminalSuggestionRects {
		if hit.Rect.contains(msg.X, msg.Y) {
			m.selectManagedTerminalSuggestionIndex(index)
			m.focusManagedTerminalInput()
			return nil, true
		}
	}

	if m.layout.terminalInputRect.contains(msg.X, msg.Y) {
		m.focusManagedTerminalInput()
		m.moveManagedTerminalCursorToMouseX(msg.X)
		m.refreshManagedTerminalSuggestions()
		return nil, true
	}

	return nil, false
}

func (m *xdfileModel) selectManagedTerminalSuggestionIndex(index int) {
	if m == nil || len(m.terminal.Suggestions) == 0 {
		return
	}
	index = max(0, min(index, len(m.terminal.Suggestions)))
	m.terminal.SuggestionCursor = index
	m.terminal.SuggestionDismissed = false
	m.syncManagedTerminalSuggestionPreview()
}

func (m *xdfileModel) moveManagedTerminalCursorToMouseX(x int) {
	if m == nil {
		return
	}

	promptWidth := rw.StringWidth(m.terminal.Input.Prompt)
	targetCell := max(0, x-m.layout.terminalInputRect.x-promptWidth)
	value := []rune(m.terminal.Input.Value())
	visibleStart, visibleEnd := xdfileManagedTerminalInputVisibleRange(value, m.terminal.Input.Position(), m.terminal.Input.Width)
	if visibleStart >= len(value) {
		m.terminal.Input.SetCursor(len(value))
		return
	}
	value = value[visibleStart:visibleEnd]

	pos := visibleStart
	cell := 0
	for i, r := range value {
		width := max(1, rw.RuneWidth(r))
		if targetCell < cell+width {
			pos = visibleStart + i
			if targetCell-cell >= (width+1)/2 {
				pos = visibleStart + i + 1
			}
			m.terminal.Input.SetCursor(pos)
			return
		}
		cell += width
		pos = visibleStart + i + 1
	}
	m.terminal.Input.SetCursor(pos)
}

func xdfileManagedTerminalInputVisibleRange(value []rune, cursor int, width int) (int, int) {
	if len(value) == 0 {
		return 0, 0
	}
	if width <= 0 || xdfileRuneWidth(value) <= width {
		return 0, len(value)
	}

	cursor = max(0, min(cursor, len(value)))
	if cursor == 0 || xdfileRuneWidth(value[:cursor]) <= width {
		return 0, xdfileManagedTerminalInputVisibleEnd(value, 0, width)
	}

	w := 0
	i := cursor - 1
	for i > 0 && w < width {
		w += max(1, rw.RuneWidth(value[i]))
		if w <= width {
			i--
		}
	}
	start := cursor - (cursor - 1 - i)
	return start, min(len(value), cursor)
}

func xdfileManagedTerminalInputVisibleEnd(value []rune, start int, width int) int {
	w := 0
	i := start
	for i < len(value) && w <= width {
		w += max(1, rw.RuneWidth(value[i]))
		if w <= width+1 {
			i++
		}
	}
	return i
}

func xdfileRuneWidth(value []rune) int {
	width := 0
	for _, r := range value {
		width += max(1, rw.RuneWidth(r))
	}
	return width
}

func (m *xdfileModel) focusPanel(index int) tea.Cmd {
	return m.selectPanel(index, false)
}

func (m *xdfileModel) selectPanel(index int, keepTerminalFocus bool) tea.Cmd {
	if m.quickViewActive() && index == m.quickViewPanelIndex() {
		index = m.activePanel
	}
	if index != m.activePanel {
		m.closePanelSearch()
	}
	m.activePanel = index
	syncCmd := m.syncTerminalToPanel(index)
	if keepTerminalFocus && m.terminalUsesPTY() {
		m.terminalFocused = true
		m.terminalAutoFocused = true
		m.setStatus("Terminal focused, selected %s panel", strings.ToLower(m.panels[index].Label))
		return syncCmd
	}

	m.terminalFocused = false
	m.terminalAutoFocused = false
	if !m.terminalUsesPTY() {
		m.focusManagedTerminalInput()
	}
	m.setStatus("Focused %s panel", strings.ToLower(m.panels[index].Label))
	return syncCmd
}

func (m *xdfileModel) focusTerminal() tea.Cmd {
	return m.setTerminalFocus(false)
}

func (m *xdfileModel) setTerminalFocus(auto bool) tea.Cmd {
	m.closePanelSearch()
	if !m.terminalUsesPTY() {
		m.terminalFocused = false
		m.terminalAutoFocused = false
		m.focusManagedTerminalInput()
		m.refreshManagedTerminalSuggestions()
		m.setStatus("Command line follows the active panel")
		return m.syncTerminalToPanel(m.activePanel)
	}

	m.terminalFocused = true
	m.terminalAutoFocused = auto
	syncCmd := m.syncTerminalToPanel(m.activePanel)
	m.terminal.ScrollOffset = 0
	m.setStatus("Focused terminal")
	return syncCmd
}

func (m *xdfileModel) syncTerminalToPanel(index int) tea.Cmd {
	if index < 0 || index >= len(m.panels) {
		return nil
	}

	target := m.panels[index].Cwd
	if target == "" {
		return nil
	}
	if xdfileIsNetBoxPath(target) {
		previous := m.terminal.Cwd
		m.terminal.Cwd = target
		m.syncManagedTerminalPrompt()
		m.terminal.PendingCwd = ""
		if !xdfilePathsEqual(previous, target) {
			m.terminal.SuggestionDismissed = false
		}
		if m.terminalUsesPTY() && (m.terminal.Title == "" || xdfilePathsEqual(m.terminal.Title, previous)) {
			m.terminal.Title = target
		}
		m.refreshManagedTerminalSuggestions()
		return nil
	}

	previous := m.terminal.Cwd
	m.terminal.Cwd = target
	m.syncManagedTerminalPrompt()
	if !xdfilePathsEqual(previous, target) {
		m.terminal.SuggestionDismissed = false
	}
	if m.terminalUsesPTY() && (m.terminal.Title == "" || xdfilePathsEqual(m.terminal.Title, previous)) {
		m.terminal.Title = target
	}

	if !m.terminalUsesPTY() || xdfilePathsEqual(previous, target) {
		m.terminal.PendingCwd = ""
		m.refreshManagedTerminalSuggestions()
		return nil
	}
	if m.terminal.Emulator == nil || m.terminal.Emulator.IsAltScreen() {
		m.terminal.PendingCwd = ""
		return nil
	}

	m.terminal.PendingCwd = target
	if err := m.requestPTYTerminalCwdSync(target); err != nil {
		m.terminal.PendingCwd = ""
		m.setStatusErr(err)
	}
	return nil
}

func (m *xdfileModel) cycleFocusForward() tea.Cmd {
	if m.quickViewActive() {
		if m.terminalFocused {
			return m.selectPanel(m.activePanel, true)
		}
		return nil
	}
	if m.terminalFocused {
		if m.activePanel == 0 {
			return m.selectPanel(1, true)
		}
		return m.selectPanel(0, true)
	}
	if m.activePanel == 0 {
		return m.focusPanel(1)
	}
	return m.focusPanel(0)
}

func (m *xdfileModel) cycleFocusBackward() tea.Cmd {
	if m.quickViewActive() {
		if m.terminalFocused {
			return m.selectPanel(m.activePanel, true)
		}
		return nil
	}
	if m.terminalFocused {
		if m.activePanel == 1 {
			return m.selectPanel(0, true)
		}
		return m.selectPanel(1, true)
	}
	if m.activePanel == 1 {
		return m.focusPanel(0)
	}
	return m.focusPanel(1)
}
