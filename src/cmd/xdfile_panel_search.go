package cmd

import (
	"path"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	charmansi "github.com/charmbracelet/x/ansi"
	stringfunction "github.com/s0x401/xdfile-manager/src/pkg/string_function"
)

const (
	xdfilePanelSearchContentWidth = 20
	xdfilePanelSearchOverlayXPad  = 9
)

type xdfilePanelSearchPattern struct {
	Raw           string
	Match         string
	DirectoryOnly bool
	Wildcard      bool
}

func (m *xdfileModel) handlePanelSearchShortcut(msg tea.KeyMsg) (tea.Cmd, bool) {
	if m == nil || m.terminalFocused || msg.Paste {
		return nil, false
	}
	r, ok := xdfilePanelSearchAltRune(msg)
	if !ok {
		return nil, false
	}
	return m.startPanelSearch(string(r)), true
}

func (m *xdfileModel) handlePanelSearchKey(msg tea.KeyMsg) (tea.Cmd, bool) {
	if m == nil || !m.panelSearch.Active {
		return nil, false
	}
	if m.panelSearch.Panel < 0 || m.panelSearch.Panel >= len(m.panels) {
		m.closePanelSearch()
		return nil, false
	}

	switch msg.String() {
	case "esc", "f10":
		m.closePanelSearch()
		return nil, true
	case "backspace", "ctrl+h":
		m.trimPanelSearchRune()
		return nil, true
	case "ctrl+u", "ctrl+y":
		m.panelSearch.Pattern = ""
		m.setStatus("Search cleared")
		return nil, true
	case "ctrl+n", "ctrl+enter":
		m.findPanelSearchNext(1)
		return nil, true
	case "ctrl+p", "ctrl+shift+enter":
		m.findPanelSearchNext(-1)
		return nil, true
	case "enter":
		m.closePanelSearch()
		return m.activateSelection(), true
	}

	if msg.Paste && len(msg.Runes) > 0 {
		m.extendPanelSearchText(strings.TrimSpace(string(msg.Runes)))
		return nil, true
	}
	if msg.Type == tea.KeySpace {
		m.extendPanelSearchText(" ")
		return nil, true
	}
	if msg.Type == tea.KeyRunes && len(msg.Runes) > 0 {
		m.extendPanelSearchText(string(msg.Runes))
		return nil, true
	}

	m.closePanelSearch()
	return nil, false
}

func xdfilePanelSearchAltRune(msg tea.KeyMsg) (rune, bool) {
	if !msg.Alt || msg.Type != tea.KeyRunes || len(msg.Runes) != 1 {
		return 0, false
	}
	r := msg.Runes[0]
	if !unicode.IsPrint(r) || unicode.IsControl(r) {
		return 0, false
	}
	return unicode.ToLower(r), true
}

func (m *xdfileModel) startPanelSearch(text string) tea.Cmd {
	panelIndex := m.activePanel
	if panelIndex < 0 || panelIndex >= len(m.panels) {
		return nil
	}
	m.panelSearch = xdfilePanelSearchState{
		Active: true,
		Panel:  panelIndex,
	}
	m.extendPanelSearchText(text)
	return nil
}

func (m *xdfileModel) closePanelSearch() {
	m.panelSearch = xdfilePanelSearchState{Panel: -1}
}

func (m *xdfileModel) trimPanelSearchRune() {
	pattern := []rune(m.panelSearch.Pattern)
	if len(pattern) == 0 {
		m.setStatus("Search")
		return
	}
	m.setPanelSearchPattern(string(pattern[:len(pattern)-1]), false, 1)
	if m.panelSearch.Pattern == "" {
		m.setStatus("Search cleared")
	}
}

func (m *xdfileModel) extendPanelSearchText(text string) {
	if text == "" {
		m.setStatus("Search")
		return
	}

	previous := m.panelSearch.Pattern
	candidate := []rune(previous + text)
	previousLen := len([]rune(previous))
	for len(candidate) > previousLen {
		if m.setPanelSearchPattern(string(candidate), false, 1) {
			return
		}
		candidate = candidate[:len(candidate)-1]
	}

	m.setStatus("Search not found: %s", xdfileNormalizePanelSearchPattern(previous+text))
}

func (m *xdfileModel) setPanelSearchPattern(pattern string, next bool, direction int) bool {
	pattern = xdfileNormalizePanelSearchPattern(pattern)
	if m.panelSearch.Panel < 0 || m.panelSearch.Panel >= len(m.panels) {
		return false
	}
	if pattern == "" {
		m.panelSearch.Pattern = ""
		m.setStatus("Search")
		return true
	}

	panel := &m.panels[m.panelSearch.Panel]
	parsedPattern := xdfileParsePanelSearchPattern(pattern)
	start := panel.Cursor
	if next {
		start += direction
	}
	index := xdfilePanelSearchFindIndex(panel, parsedPattern, start, direction)
	if index < 0 {
		return false
	}
	m.panelSearch.Pattern = pattern
	m.focusPanelSearchMatch(m.panelSearch.Panel, index)
	m.setStatus("Search: %s", pattern)
	return true
}

func (m *xdfileModel) findPanelSearchNext(direction int) {
	if m.panelSearch.Pattern == "" {
		return
	}
	if direction == 0 {
		direction = 1
	}
	if !m.setPanelSearchPattern(m.panelSearch.Pattern, true, direction) {
		m.setStatus("Search not found: %s", m.panelSearch.Pattern)
	}
}

func (m *xdfileModel) focusPanelSearchMatch(panelIndex int, matchIndex int) {
	if panelIndex < 0 || panelIndex >= len(m.panels) {
		return
	}
	panel := &m.panels[panelIndex]
	rows := panel.visibleRows(m.layout.panelRects[panelIndex].h)
	panel.Cursor = matchIndex
	if rows > 0 {
		panel.Scroll = matchIndex - rows/2
	}
	panel.ensureVisible(rows)
	panel.resetRangeAnchor()
	m.syncQuickViewViewport()
}

func xdfileNormalizePanelSearchPattern(pattern string) string {
	pattern = strings.TrimPrefix(pattern, `"`)
	for strings.Contains(pattern, "**") {
		pattern = strings.ReplaceAll(pattern, "**", "*")
	}
	return pattern
}

func xdfileParsePanelSearchPattern(pattern string) xdfilePanelSearchPattern {
	parsed := xdfilePanelSearchPattern{
		Raw:           pattern,
		Match:         strings.ToLower(pattern),
		DirectoryOnly: strings.HasSuffix(pattern, `/`) || strings.HasSuffix(pattern, `\`),
	}
	if parsed.DirectoryOnly {
		parsed.Match = strings.ToLower(strings.TrimRight(pattern, `/\`))
	}
	parsed.Wildcard = strings.ContainsAny(parsed.Match, "*?")
	return parsed
}

func xdfilePanelSearchFindIndex(panel *xdfilePanel, pattern xdfilePanelSearchPattern, start int, direction int) int {
	count := len(panel.Entries)
	if count == 0 || pattern.Raw == "" {
		return -1
	}
	if direction < 0 {
		for offset := 0; offset < count; offset++ {
			index := (start - offset) % count
			if index < 0 {
				index += count
			}
			if xdfilePanelSearchEntryMatches(panel.Entries[index], pattern) {
				return index
			}
		}
		return -1
	}
	for offset := 0; offset < count; offset++ {
		index := (start + offset) % count
		if index < 0 {
			index += count
		}
		if xdfilePanelSearchEntryMatches(panel.Entries[index], pattern) {
			return index
		}
	}
	return -1
}

func xdfilePanelSearchEntryMatches(entry xdfileEntry, pattern xdfilePanelSearchPattern) bool {
	if entry.IsParent {
		return false
	}
	if pattern.DirectoryOnly {
		if !entry.IsDir {
			return false
		}
		if pattern.Match == "" {
			return true
		}
	}
	return xdfilePanelSearchNameMatches(entry.Name, pattern)
}

func xdfilePanelSearchNameMatches(name string, pattern xdfilePanelSearchPattern) bool {
	name = strings.ToLower(name)
	if pattern.Wildcard {
		matchPattern := pattern.Match
		if !strings.HasSuffix(matchPattern, "*") {
			matchPattern += "*"
		}
		matched, err := path.Match(matchPattern, name)
		return err == nil && matched
	}
	return strings.HasPrefix(name, pattern.Match)
}

func (m *xdfileModel) renderPanelSearchOverlay(index int, rendered string, rect xdfileRect) string {
	if m == nil || !m.panelSearch.Active || m.panelSearch.Panel != index || rect.w < 12 || rect.h < 6 {
		return rendered
	}

	contentWidth := min(xdfilePanelSearchContentWidth, max(8, rect.w-4))
	pattern := m.panelSearch.Pattern
	if pattern == "" {
		pattern = " "
	}
	inputLine := xdfileMenuItemHot.Width(contentWidth).Render(charmansi.Truncate(pattern, contentWidth, "..."))
	titleLine := xdfileTitleStyle.Width(contentWidth).Render("Search")
	popup := xdfileMenuBorder().Width(contentWidth).Render(titleLine + "\n" + inputLine)
	popupW := lipgloss.Width(popup)
	popupH := lipgloss.Height(popup)
	x := min(xdfilePanelSearchOverlayXPad, max(1, rect.w-popupW-1))
	y := max(1, rect.h-popupH-1)
	return stringfunction.PlaceOverlay(x, y, popup, rendered)
}
