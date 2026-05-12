package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	charmansi "github.com/charmbracelet/x/ansi"
	stringfunction "github.com/s0x401/xdfile-manager/src/pkg/string_function"
)

func (m *xdfileModel) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}
	if m.userScreenVisible {
		return ""
	}
	if m.width < xdfileMinWidth || m.height < xdfileMinHeight {
		msg := fmt.Sprintf(
			"Xdfile Manager needs at least %dx%d. Current terminal is %dx%d.",
			xdfileMinWidth,
			xdfileMinHeight,
			m.width,
			m.height,
		)
		return m.finalizeView(lipgloss.NewStyle().
			Width(m.width).
			Height(m.height).
			Align(lipgloss.Center, lipgloss.Center).
			Foreground(xdfileColorDanger).
			Render(msg))
	}

	m.computeLayout()
	if m.exclusiveTerminalActive() {
		return m.finalizeView(m.renderExclusiveTerminalView())
	}
	if m.terminalExpandedViewActive() {
		base := m.renderTerminalExpandedView()
		if m.modal.Kind == xdfileModalNone {
			return m.finalizeView(base)
		}
		modal := m.renderModal()
		return m.finalizeView(stringfunction.PlaceOverlay(
			max(0, (m.width-lipgloss.Width(modal))/2),
			max(0, (m.height-lipgloss.Height(modal))/2),
			modal,
			base,
		))
	}
	if m.openMenu == "" {
		m.layout.menuRect = xdfileRect{}
		m.layout.menuItemRects = nil
	}
	baseParts := [4]string{
		m.renderHeader(),
		lipgloss.JoinHorizontal(
			lipgloss.Top,
			m.renderPanel(0),
			" ",
			m.renderPanel(1),
		),
		m.renderTerminal(),
		m.renderFooter(),
	}
	base := lipgloss.JoinVertical(lipgloss.Left, baseParts[:]...)

	withMenu := base
	if menu := m.renderOpenMenu(); menu != "" {
		withMenu = stringfunction.PlaceOverlay(m.layout.menuRect.x, m.layout.menuRect.y, menu, base)
	}

	if m.modal.Kind == xdfileModalNone {
		return m.finalizeView(withMenu)
	}

	modal := m.renderModal()
	return m.finalizeView(stringfunction.PlaceOverlay(
		max(0, (m.width-lipgloss.Width(modal))/2),
		max(0, (m.height-lipgloss.Height(modal))/2),
		modal,
		withMenu,
	))
}

func (m *xdfileModel) finalizeView(value string) string {
	if value == "" {
		return value
	}
	return xdfileConstrainFrame(xdfileWrapANSIRender(value), m.width, m.height)
}

func xdfileConstrainFrame(value string, width int, height int) string {
	if value == "" || width <= 0 || height <= 0 {
		return value
	}

	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	lines := strings.Split(value, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}

	for i, line := range lines {
		line = charmansi.Truncate(line, width, "")
		padding := max(0, width-lipgloss.Width(line))
		lines[i] = line + xdfileBlank(padding)
	}
	for len(lines) < height {
		lines = append(lines, xdfileBlank(width))
	}
	return strings.Join(lines, "\n") + xdfileANSIReset
}

func xdfileWrapANSIRender(value string) string {
	if value == "" {
		return value
	}
	return xdfileANSIReset + value + xdfileANSIReset
}

func (m *xdfileModel) computeLayout() {
	availableHeight := max(1, m.height-xdfileHeaderHeight-xdfileFooterHeight)
	terminalHeight := xdfileTerminalHeightForPrefs(availableHeight, m.layoutPrefs)
	bodyHeight := max(1, availableHeight-terminalHeight)

	availableWidth := max(1, m.width-1)
	leftWidth := xdfileLeftWidthForPrefs(availableWidth, m.layoutPrefs)
	rightWidth := availableWidth - leftWidth

	m.layout.panelRects[0] = xdfileRect{x: 0, y: xdfileHeaderHeight, w: leftWidth, h: bodyHeight}
	m.layout.panelRects[1] = xdfileRect{x: leftWidth + 1, y: xdfileHeaderHeight, w: rightWidth, h: bodyHeight}
	m.layout.terminalRect = xdfileRect{
		x: 0,
		y: xdfileHeaderHeight + bodyHeight,
		w: m.width,
		h: terminalHeight,
	}
	m.layout.exclusiveRect = xdfileRect{
		x: 0,
		y: xdfileHeaderHeight,
		w: m.width,
		h: max(1, m.height-xdfileHeaderHeight-xdfileFooterHeight),
	}

	m.computeTerminalDimensions()
}

func (m *xdfileModel) restorePanelFocusAfterManagedCommand() {
	m.terminalFocused = false
	m.terminalAutoFocused = false
	if !m.terminalUsesPTY() {
		m.focusManagedTerminalInput()
	}
}

func (m *xdfileModel) cancelManagedCommandIfBusy() bool {
	if m.terminalUsesPTY() || !m.terminal.Busy || m.terminal.ManagedCancel == nil {
		return false
	}
	cancel := m.terminal.ManagedCancel
	m.terminal.ManagedCancel = nil
	cancel()
	m.setStatus("Stopping command...")
	return true
}

func (m *xdfileModel) toggleUserScreen() tea.Cmd {
	if m.userScreenVisible {
		return tea.Sequence(
			tea.EnterAltScreen,
			func() tea.Msg { return xdfileReturnFromUserScreenMsg{} },
		)
	}
	m.userScreenVisible = true
	return tea.ExitAltScreen
}

func (m *xdfileModel) terminalRenderRect() xdfileRect {
	if m.terminalExpandedViewActive() {
		return m.layout.exclusiveRect
	}
	return m.layout.terminalRect
}

func (m *xdfileModel) syncTerminalViewport(stickToBottom bool) {
	if m.width == 0 || m.height == 0 {
		return
	}
	m.computeTerminalDimensions()
	if m.terminalUsesPTY() {
		if stickToBottom {
			m.terminal.ScrollOffset = 0
		}
		if err := m.terminal.Session.Resize(m.terminal.ViewWidth, m.terminal.ViewHeight); err != nil {
			m.setStatusErr(err)
		}
		return
	}
	m.terminal.ViewportContent = xdfileManagedTerminalViewportContent(m.terminal.Lines, m.terminal.Viewport.Width)
	m.terminal.Viewport.SetContent(m.terminal.ViewportContent)
	if stickToBottom {
		m.terminal.Viewport.GotoBottom()
	}
}

func xdfileManagedTerminalViewportContent(lines []string, width int) string {
	if len(lines) == 0 {
		return ""
	}
	if width <= 0 {
		return strings.Join(lines, "\n")
	}

	var builder strings.Builder
	for i, line := range lines {
		if i > 0 {
			builder.WriteByte('\n')
		}
		if line == "" {
			continue
		}
		builder.WriteString(charmansi.Hardwrap(line, width, true))
	}
	return builder.String()
}

func xdfileManagedTerminalViewportLines(content string, yOffset int, height int) []string {
	if height <= 0 {
		return nil
	}
	if content == "" {
		return make([]string, height)
	}

	lines := strings.Split(content, "\n")
	if yOffset < 0 {
		yOffset = 0
	}
	if yOffset >= len(lines) {
		yOffset = max(0, len(lines)-height)
	}

	bottom := min(len(lines), yOffset+height)
	if bottom-yOffset == height {
		return lines[yOffset:bottom]
	}
	visible := make([]string, height)
	copy(visible, lines[yOffset:bottom])
	return visible
}

func xdfileSetTruncatedViewportContent(vp *viewport.Model, text string, width int) string {
	if vp == nil {
		return ""
	}
	if width <= 0 {
		vp.SetContent("")
		return ""
	}

	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	if text == "" {
		vp.SetContent("")
		return ""
	}

	lines := strings.Split(text, "\n")
	var builder strings.Builder
	for i, line := range lines {
		if i > 0 {
			builder.WriteByte('\n')
		}
		builder.WriteString(charmansi.Truncate(line, width, "..."))
	}
	content := builder.String()
	vp.SetContent(content)
	return content
}

func (m *xdfileModel) computeTerminalDimensions() {
	rect := m.terminalRenderRect()
	if rect.w == 0 || rect.h == 0 {
		return
	}
	innerWidth := max(10, rect.w-2)
	viewportHeight := max(1, rect.h-3)
	m.terminal.ViewWidth = innerWidth
	m.terminal.ViewHeight = viewportHeight
	if m.terminalUsesPTY() {
		return
	}
	m.terminal.Viewport.Width = innerWidth
	m.terminal.Viewport.Height = max(1, rect.h-5)
	m.terminal.Input.Width = max(10, innerWidth-6)
}

func (m *xdfileModel) panelSortMode(index int) xdfileSortMode {
	prefs := m.layoutPrefs.normalized()
	if index == 1 {
		return prefs.RightSortMode
	}
	return prefs.LeftSortMode
}

func (m *xdfileModel) setPanelSortMode(index int, mode xdfileSortMode) {
	mode = xdfileNormalizeSortMode(mode)
	if index == 1 {
		m.layoutPrefs.RightSortMode = mode
		return
	}
	m.layoutPrefs.LeftSortMode = mode
}

func xdfileSortModeLabel(mode xdfileSortMode) string {
	switch xdfileNormalizeSortMode(mode) {
	case xdfileSortModeExt:
		return "Extens"
	default:
		return "Name"
	}
}

func (m *xdfileModel) quickViewActive() bool {
	return m.layoutPrefs.QuickViewDocked && m.quickView.Open
}

func (m *xdfileModel) quickViewPanelIndex() int {
	if !m.quickViewActive() {
		return -1
	}
	return 1 - m.activePanel
}

func (m *xdfileModel) validPanelIndex(index int) bool {
	return index >= 0 && index < len(m.panels)
}

func (m *xdfileModel) noteFooterCtrlHint() tea.Cmd {
	m.footerCtrlHintUntil = time.Now().Add(1500 * time.Millisecond)
	until := m.footerCtrlHintUntil
	return tea.Tick(1500*time.Millisecond, func(t time.Time) tea.Msg {
		return xdfileFooterCtrlHintExpiredMsg{At: until}
	})
}

func xdfileScheduleAutoRefresh() tea.Cmd {
	return tea.Tick(xdfileAutoRefreshInterval, func(time.Time) tea.Msg {
		return xdfileAutoRefreshMsg{}
	})
}

func (m *xdfileModel) footerShowsCtrlHints() bool {
	return !m.footerCtrlHintUntil.IsZero() && time.Now().Before(m.footerCtrlHintUntil)
}
