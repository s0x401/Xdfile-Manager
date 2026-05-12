package cmd

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	rw "github.com/mattn/go-runewidth"
)

const xdfileNoHoverIndex = -1

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

func (m *xdfileModel) handleMouse(msg tea.MouseMsg) tea.Cmd {
	m.updateMouseHover(msg)
	if msg.Action == tea.MouseActionPress && m.panelSearch.Active {
		m.closePanelSearch()
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
					m.panels[i].move(-1, rows)
				} else {
					m.panels[i].move(1, rows)
				}
				m.syncQuickViewViewport()
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

	for i, rect := range m.layout.panelRects {
		if m.quickViewActive() && i == m.quickViewPanelIndex() {
			continue
		}
		if !rect.contains(msg.X, msg.Y) {
			continue
		}
		_ = m.focusPanel(i)
		rows := m.panels[i].visibleRows(rect.h)
		row := msg.Y - (rect.y + 3)
		if row < 0 {
			return m.handlePanelBlankClick(i, 0, rows)
		}
		if row >= rows {
			return m.handlePanelBlankClick(i, len(m.panels[i].Entries)-1, rows)
		}
		index, ok := m.panelEntryIndexAt(i, rect, msg.Y)
		if !ok {
			return m.handlePanelBlankClick(i, len(m.panels[i].Entries)-1, rows)
		}
		m.panels[i].clearMarked()
		m.panels[i].setCursor(index, rows)
		m.syncQuickViewViewport()

		now := time.Now()
		if m.lastClick.panel == i && m.lastClick.row == index && now.Sub(m.lastClick.at) < 450*time.Millisecond {
			m.lastClick = xdfileClickState{panel: -1, row: -1}
			return m.activateSelection()
		}
		m.lastClick = xdfileClickState{panel: i, row: index, at: now}
		return nil
	}

	return nil
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

func (m *xdfileModel) handlePanelBlankClick(panelIndex int, targetIndex int, rows int) tea.Cmd {
	if panelIndex < 0 || panelIndex >= len(m.panels) || len(m.panels[panelIndex].Entries) == 0 {
		m.lastClick = xdfileClickState{panel: -1, row: -1}
		return nil
	}

	targetIndex = max(0, min(targetIndex, len(m.panels[panelIndex].Entries)-1))
	m.panels[panelIndex].clearMarked()
	m.panels[panelIndex].setCursor(targetIndex, rows)
	m.syncQuickViewViewport()

	now := time.Now()
	if m.lastClick.panel == panelIndex && m.lastClick.row == targetIndex && now.Sub(m.lastClick.at) < 450*time.Millisecond {
		m.lastClick = xdfileClickState{panel: -1, row: -1}
		return m.activateSelection()
	}
	m.lastClick = xdfileClickState{panel: panelIndex, row: targetIndex, at: now}
	return nil
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
