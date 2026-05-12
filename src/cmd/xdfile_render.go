package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	charmansi "github.com/charmbracelet/x/ansi"
	iconconfig "github.com/s0x401/xdfile-manager/src/config/icon"
	filepreview "github.com/s0x401/xdfile-manager/src/pkg/file_preview"
	stringfunction "github.com/s0x401/xdfile-manager/src/pkg/string_function"
)

func (m *xdfileModel) renderHeader() string {
	menuBar, menuHits := m.renderMenuBar(0)
	m.layout.menuButtons = menuHits

	leftInfo := menuBar
	rightReserve := max(24, m.width/3)
	panelLabel := xdfileTagStyle.Render(m.panels[m.activePanel].Label)
	pathWidth := max(12, rightReserve-lipgloss.Width(panelLabel)-1)
	rightInfo := panelLabel + " " +
		xdfilePathStyle.Render(xdfileCompactPath(m.panels[m.activePanel].Cwd, pathWidth))
	line0 := xdfileJoinLeftRightPreserveRight(leftInfo, rightInfo, m.width, rightReserve)

	headerTitle := xdfileTitleStyle.Render(xdfileProductName)
	statusText := m.renderStatusText(max(0, m.width/2))
	line1 := xdfileJoinLeftRight(headerTitle, statusText, m.width)

	return xdfileWrapANSIRender(lipgloss.JoinVertical(
		lipgloss.Left,
		xdfileHeaderLineStyle.Width(m.width).Render(line0),
		xdfileHeaderLineStyle.Width(m.width).Render(line1),
	))
}

func (m *xdfileModel) renderMenuBar(y int) (string, []xdfileButtonRect) {
	menus := m.menuDefinitions()
	x := 0
	hits := make([]xdfileButtonRect, 0, len(menus))
	var builder strings.Builder

	for i, menu := range menus {
		if i > 0 {
			builder.WriteByte(' ')
			x++
		}
		label := " " + menu.Label + " "
		rendered := xdfileMenuButton.Render(label)
		if m.openMenu == menu.Action {
			rendered = xdfileMenuButtonHot.Render(label)
		} else if m.hover.MenuAction == menu.Action {
			rendered = xdfileHoveredMenuButtonStyle().Render(label)
		}
		width := lipgloss.Width(rendered)
		hits = append(hits, xdfileButtonRect{
			Action: menu.Action,
			Rect: xdfileRect{
				x: x,
				y: y,
				w: width,
				h: 1,
			},
		})
		builder.WriteString(rendered)
		x += width
	}

	return builder.String(), hits
}

func (m *xdfileModel) renderOpenMenu() string {
	menu, ok := m.currentMenu()
	if !ok || len(menu.Items) == 0 {
		m.layout.menuRect = xdfileRect{}
		m.layout.menuItemRects = nil
		return ""
	}

	anchor := m.menuAnchorRect(menu.Action)
	contentWidth := 18
	for _, item := range menu.Items {
		itemWidth := lipgloss.Width(item.Label)
		if item.Key != "" {
			itemWidth += lipgloss.Width(item.Key) + 2
		}
		contentWidth = max(contentWidth, itemWidth+4)
	}

	titleLines := 0
	if menu.Action == xdfileActionCommandsMenu {
		titleLines = 2
	}
	menuHeight := len(menu.Items) + 2 + titleLines
	originX := anchor.x
	originY := anchor.y + 1
	if menu.Action == xdfileActionContextMenu || menu.Action == xdfileActionCommandsMenu {
		originY = anchor.y
	}
	originX = max(0, min(originX, m.width-(contentWidth+2)))
	originY = max(0, min(originY, m.height-menuHeight))

	lines := make([]string, 0, len(menu.Items)+titleLines)
	if menu.Action == xdfileActionCommandsMenu {
		lines = append(lines, xdfileRenderCommandsMenuTitle(contentWidth, menu.Label))
		lines = append(lines, xdfileMetaStyle.Render(strings.Repeat("-", contentWidth)))
	}
	hits := make([]xdfileButtonRect, 0, len(menu.Items))
	for i, item := range menu.Items {
		lines = append(lines, xdfileRenderMenuItem(item, contentWidth, i == m.menuCursor && m.hover.MenuItem < 0, i == m.hover.MenuItem))
		hits = append(hits, xdfileButtonRect{
			Action:   item.Action,
			Disabled: item.Disabled,
			Rect: xdfileRect{
				x: originX + 1,
				y: originY + 1 + titleLines + i,
				w: contentWidth,
				h: 1,
			},
		})
	}

	m.layout.menuRect = xdfileRect{
		x: originX,
		y: originY,
		w: contentWidth + 2,
		h: menuHeight,
	}
	m.layout.menuItemRects = hits

	return xdfileWrapANSIRender(xdfileMenuBorder().Width(contentWidth).Render(strings.Join(lines, "\n")))
}

func xdfileRenderCommandsMenuTitle(width int, label string) string {
	label = strings.TrimSpace(label)
	if label == "" || strings.EqualFold(label, "User menu") {
		return xdfilePadRight(xdfileTitleStyle.Bold(true).Render("F2 User Menu"), width)
	}
	return xdfileJoinLeftRight(
		xdfileTitleStyle.Bold(true).Render("F2 User Menu"),
		xdfileDimStyle.Render(charmansi.Truncate(label, max(8, width/2), "...")),
		width,
	)
}

func xdfileRenderMenuItem(item xdfileButton, width int, selected bool, hovered bool) string {
	content := xdfileJoinLeftRight(item.Label, item.Key, width)
	switch {
	case item.Disabled:
		return xdfileMenuItemDisabledStyle.Width(width).Render(content)
	case selected:
		return xdfileMenuItemHot.Width(width).Render(content)
	case hovered:
		return xdfileHoveredMenuItemStyle().Width(width).Render(content)
	case item.Key != "":
		return xdfileMenuItemStyle.Width(width).Render(
			xdfileJoinLeftRight(item.Label, xdfileMenuItemKeyStyle.Render(item.Key), width),
		)
	default:
		return xdfileMenuItemStyle.Width(width).Render(content)
	}
}

func (m *xdfileModel) renderFooter() string {
	line0 := xdfileJoinLeftRight(
		m.renderFooterSelection(),
		m.renderFooterState(),
		m.width,
	)

	previewLabel := "Preview"
	if m.layoutPrefs.QuickViewDocked {
		previewLabel = "Quick view"
	}

	buttons := []xdfileButton{
		{Action: xdfileActionHelp, Key: "F1", Label: "Help"},
		{Action: xdfileActionCommandsMenu, Key: "F2", Label: "Commands"},
		{Action: xdfileActionPreview, Key: "F3", Label: previewLabel},
		{Action: xdfileActionRename, Key: "F4", Label: "Rename"},
		{Action: xdfileActionCopy, Key: "F5", Label: "Copy"},
		{Action: xdfileActionMove, Key: "F6", Label: "Move"},
		{Action: xdfileActionMkdir, Key: "F7", Label: "Mkdir"},
		{Action: xdfileActionDelete, Key: "F8", Label: "Delete"},
		{Action: xdfileActionHidden, Key: "F9", Label: "Hidden"},
		{Action: xdfileActionQuit, Key: "F10", Label: "Quit"},
	}
	if m.footerShowsCtrlHints() {
		buttons = []xdfileButton{
			{Action: xdfileActionPreview, Key: "Ctrl+Q", Label: previewLabel},
			{Action: xdfileActionSortName, Key: "Ctrl+3", Label: "Name"},
			{Action: xdfileActionSortExt, Key: "Ctrl+4", Label: "Extens"},
			{Action: xdfileActionClipboardCopy, Key: "Ctrl+Shift+C", Label: "Copy"},
			{Action: xdfileActionClipboardCut, Key: "Ctrl+X", Label: "Cut"},
			{Action: xdfileActionPaste, Key: "Ctrl+Shift+V", Label: "Paste"},
			{Action: xdfileActionUndoDelete, Key: "Ctrl+Z", Label: "Undo"},
		}
	}
	line1, hits := m.renderFunctionButtons(buttons, m.height-1)
	m.layout.footerButtons = hits

	return xdfileWrapANSIRender(lipgloss.JoinVertical(
		lipgloss.Left,
		xdfileFooterLineStyle.Width(m.width).Render(line0),
		xdfileFooterLineStyle.Width(m.width).Render(line1),
	))
}

func (m *xdfileModel) renderFooterSelection() string {
	sep := xdfileFooterSeparator()
	if marked := m.panels[m.activePanel].markedEntries(); len(marked) > 0 {
		return xdfileTagStyle.Render(fmt.Sprintf("%d selected", len(marked))) +
			sep +
			xdfileDimStyle.Render(m.panels[m.activePanel].Cwd)
	}
	if entry, ok := m.panels[m.activePanel].selected(); ok {
		return xdfileEntryKindSpecForEntry(entry).render() +
			sep +
			xdfileEntryNameStyle(entry).Render(entry.Name) +
			sep +
			xdfileDimStyle.Render(xdfileEntrySummary(entry))
	}
	return xdfileDimStyle.Render("No selection")
}

func (m *xdfileModel) renderFooterState() string {
	parts := []string{
		xdfileDimStyle.Render("PANE ") + xdfileTitleStyle.Render(m.panels[m.activePanel].Label),
	}
	switch {
	case m.terminalUsesPTY():
		termPart := xdfileDimStyle.Render("TERM ") + xdfileTagStyle.Render("conpty")
		if m.terminalFocused {
			termPart += " " + xdfileTitleStyle.Render("focus")
		}
		if m.terminal.ScrollOffset > 0 {
			termPart += " " + xdfileDimStyle.Render(fmt.Sprintf("scroll +%d", m.terminal.ScrollOffset))
		}
		parts = append(parts, termPart)
	case m.terminal.Busy:
		parts = append(parts, xdfileDimStyle.Render("CMD ")+xdfileTitleStyle.Render(m.commandStateLabel()))
	case m.managedTerminalPopupVisible():
		parts = append(parts, xdfileDimStyle.Render("CMD ")+xdfileTagStyle.Render("popup"))
	case m.terminalFocused:
		parts = append(parts, xdfileDimStyle.Render("TERM ")+xdfileTitleStyle.Render("focus"))
	default:
		parts = append(parts, xdfileDimStyle.Render("CMD bound"))
	}
	parts = append(parts,
		xdfileDimStyle.Render("SORT ")+xdfileTagStyle.Render(xdfileSortModeLabel(m.panelSortMode(m.activePanel))),
		xdfileDimStyle.Render(m.layoutStatusLabel()),
	)
	return strings.Join(parts, xdfileFooterSeparator())
}

func xdfileFooterSeparator() string {
	return xdfileDimStyle.Render(" | ")
}

func (m *xdfileModel) renderPanel(index int) string {
	if index == m.quickViewPanelIndex() {
		return m.renderQuickViewPanel()
	}

	panel := &m.panels[index]
	rect := m.layout.panelRects[index]
	innerW := max(10, rect.w-2)
	innerH := max(4, rect.h-2)
	rows := panel.visibleRows(rect.h)
	panel.ensureVisible(rows)

	titlePrefix := xdfileTagStyle.Render(panel.Label)
	if index == m.activePanel {
		titlePrefix = xdfileTitleStyle.Render(panel.Label)
	}
	cursorDisplay := 0
	if len(panel.Entries) > 0 {
		cursorDisplay = min(panel.Cursor+1, len(panel.Entries))
	}
	titleRightParts := []string{fmt.Sprintf("%d/%d", cursorDisplay, len(panel.Entries))}
	if marked := panel.markedCount(); marked > 0 {
		titleRightParts = append(titleRightParts, fmt.Sprintf("%d sel", marked))
	}
	if gitLabel := panel.Git.TitleLabel(); gitLabel != "" {
		titleRightParts = append(titleRightParts, gitLabel)
	}
	titleRightParts = append(titleRightParts, xdfileSortModeLabel(m.panelSortMode(index)))
	titleRight := xdfileDimStyle.Render(strings.Join(titleRightParts, " | "))
	pathStyle := xdfilePathStyle
	if index == m.activePanel {
		pathStyle = xdfileTerminalPromptPathStyle
	}
	titleLine := xdfileJoinLeftRight(
		titlePrefix+" "+pathStyle.Render(xdfileCompactPath(panel.Cwd, max(12, innerW-14))),
		titleRight,
		innerW,
	)

	nameWidth := xdfilePanelNameWidth(innerW)
	columns := xdfilePadRight(xdfileTagStyle.Render("Name"), nameWidth) + " " +
		xdfileAlignRight(xdfileMetaStyle.Render("Size"), xdfilePanelSizeWidth) + " " +
		xdfileAlignRight(xdfileMetaStyle.Render("Time"), xdfilePanelTimeWidth)

	lines := make([]string, 0, innerH)
	lines = append(lines, titleLine, columns)
	end := min(len(panel.Entries), panel.Scroll+rows)
	for i := panel.Scroll; i < end; i++ {
		entry := panel.Entries[i]
		lines = append(lines, m.renderEntry(
			entry,
			i == panel.Cursor,
			panel.isMarked(entry),
			index == m.activePanel,
			m.hover.Panel == index && m.hover.PanelIndex == i,
			innerW,
		))
	}
	blankLine := xdfileBlank(innerW)
	for len(lines) < innerH {
		lines = append(lines, blankLine)
	}

	borderStyle := xdfilePanelBorder(index == m.activePanel)

	rendered := borderStyle.Width(rect.w - 2).Height(rect.h - 2).Render(strings.Join(lines[:innerH], "\n"))
	rendered = m.renderPanelSearchOverlay(index, rendered, rect)
	return xdfileWrapANSIRender(rendered)
}

func (m *xdfileModel) renderQuickViewPanel() string {
	quickViewIndex := m.quickViewPanelIndex()
	if quickViewIndex < 0 {
		return ""
	}
	rect := m.layout.panelRects[quickViewIndex]
	innerW := max(10, rect.w-2)
	innerH := max(4, rect.h-2)
	bodyHeight := max(1, innerH-2)

	m.syncQuickViewViewport()

	titleLeft := xdfileTitleStyle.Render("QUICK VIEW")
	if m.quickView.Title != "" {
		titleLeft += " " + xdfilePathStyle.Render(xdfileCompactPath(m.quickView.Title, max(12, innerW-16)))
	}

	titleRight := xdfileDimStyle.Render("auto")
	if m.quickView.Binary {
		titleRight = xdfileTagStyle.Render("binary")
	} else if m.quickView.Visual {
		titleRight = xdfileTagStyle.Render("visual")
	}
	titleLine := xdfileJoinLeftRight(titleLeft, titleRight, innerW)

	lines := make([]string, 0, innerH)
	lines = append(lines, titleLine)
	if m.quickView.Visual {
		lines = append(lines, strings.Split(m.quickView.Text, "\n")...)
	} else {
		lines = append(lines, strings.Split(m.quickView.Viewport.View(), "\n")...)
	}
	for len(lines) < 1+bodyHeight {
		lines = append(lines, "")
	}
	lines = append(lines, xdfileDimStyle.Render(charmansi.Truncate(m.quickView.Description, innerW, "...")))
	for len(lines) < innerH {
		lines = append(lines, "")
	}

	return xdfileWrapANSIRender(xdfilePanelBorder(false).BorderForeground(xdfileColorAccent2).
		Width(rect.w - 2).
		Height(rect.h - 2).
		Render(strings.Join(lines[:innerH], "\n")))
}

func (m *xdfileModel) renderEntry(entry xdfileEntry, selected bool, marked bool, active bool, hovered bool, width int) string {
	nameWidth := xdfilePanelNameWidth(width)
	kind := xdfileEntryKindSpecForEntry(entry)
	prefix := " "
	switch {
	case selected && marked:
		prefix = "*"
	case selected:
		prefix = ">"
	case marked:
		prefix = "+"
	}

	name := prefix + kind.Label + xdfileGitMarkerPrefix(entry.GitMarker) + " " + entry.Name

	size := ""
	modified := ""
	if !entry.IsParent {
		modified = entry.Modified.Format("01-02 15:04")
	}
	if !entry.IsDir && !entry.IsParent {
		size = xdfileHumanSize(entry.Size)
	}

	if selected || marked {
		plainLine := xdfilePadRight(name, nameWidth) + " " +
			xdfileAlignRight(size, xdfilePanelSizeWidth) + " " +
			xdfileAlignRight(modified, xdfilePanelTimeWidth)
		if selected {
			switch {
			case active:
				return xdfileSelectedLineStyle(true).Render(xdfilePadRight(plainLine, width))
			case marked:
				return xdfileMarkedLineStyle(false).Render(xdfilePadRight(plainLine, width))
			default:
				return xdfileInactiveCursorLineStyle().Render(xdfilePadRight(plainLine, width))
			}
		}
		if hovered {
			return xdfileMarkedLineStyle(active).Underline(true).Render(xdfilePadRight(plainLine, width))
		}
		return xdfileMarkedLineStyle(active).Render(xdfilePadRight(plainLine, width))
	}

	nameStyle := xdfileEntryNameStyle(entry)
	line := xdfilePadRight(xdfileRenderEntryName(nameStyle, entry.GitMarker, prefix+kind.render(), entry.Name), nameWidth) + " " +
		xdfileAlignRight(xdfileMetaStyle.Render(size), xdfilePanelSizeWidth) + " " +
		xdfileAlignRight(xdfileMetaStyle.Render(modified), xdfilePanelTimeWidth)
	if hovered {
		hoveredLine := xdfileRenderHoveredEntryNameColumn(entry, prefix, nameWidth) +
			xdfileHoveredEntryLineStyle().Render(" ") +
			xdfileRenderHoveredEntryMetaColumn(size, xdfilePanelSizeWidth) +
			xdfileHoveredEntryLineStyle().Render(" ") +
			xdfileRenderHoveredEntryMetaColumn(modified, xdfilePanelTimeWidth)
		return xdfilePadRight(hoveredLine, width)
	}
	return xdfilePadRight(line, width)
}

func xdfileEntryNameStyle(entry xdfileEntry) lipgloss.Style {
	switch {
	case entry.IsParent:
		return xdfileMetaStyle
	case entry.IsDir:
		return xdfileDirStyle
	default:
		if color, ok := xdfileResolveFileColor(entry.Name); ok {
			return xdfileCachedFileColorStyle(color)
		}
		return xdfileFileStyle
	}
}

func xdfileCachedFileColorStyle(color lipgloss.Color) lipgloss.Style {
	if style, ok := xdfileFileColorStyles[color]; ok {
		return style
	}
	style := lipgloss.NewStyle().Foreground(color)
	xdfileFileColorStyles[color] = style
	return style
}

func xdfileResolveFileColor(name string) (lipgloss.Color, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return "", false
	}

	if color, ok := xdfileLookupIconColor(name); ok {
		return color, true
	}

	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
	if ext == "" {
		return "", false
	}
	return xdfileLookupIconColor(ext)
}

var xdfileExecutableMarkerExtensions = xdfileStringSet{
	"appinstaller": {},
	"appx":         {},
	"appxbundle":   {},
	"com":          {},
	"exe":          {},
	"msi":          {},
	"msp":          {},
	"msix":         {},
	"msixbundle":   {},
	"msu":          {},
	"scr":          {},
	"vsix":         {},
}

var xdfileScriptMarkerExtensions = xdfileStringSet{
	"awk":    {},
	"bash":   {},
	"bat":    {},
	"cmd":    {},
	"fish":   {},
	"hta":    {},
	"js":     {},
	"ksh":    {},
	"lua":    {},
	"php":    {},
	"pl":     {},
	"ps1":    {},
	"ps1xml": {},
	"psd1":   {},
	"psm1":   {},
	"py":     {},
	"pyw":    {},
	"rb":     {},
	"sh":     {},
	"vbs":    {},
	"wsf":    {},
	"zsh":    {},
}

var xdfileArchiveMarkerExtensions = xdfileStringSet{
	"7z":  {},
	"bz2": {},
	"cab": {},
	"esd": {},
	"gz":  {},
	"iso": {},
	"lz":  {},
	"rar": {},
	"tar": {},
	"tgz": {},
	"txz": {},
	"wim": {},
	"swm": {},
	"xz":  {},
	"zip": {},
}

var xdfileScriptMarkerIconKeys = xdfileStringSet{
	"lua":   {},
	"php":   {},
	"py":    {},
	"rb":    {},
	"shell": {},
}

func xdfileResolveFileMarker(name string) (string, lipgloss.Color, bool) {
	trimmed := strings.TrimSpace(name)
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(trimmed)), ".")
	key := xdfileResolveIconKey(trimmed)
	switch {
	case xdfileScriptMarkerExtensions.has(ext) || xdfileScriptMarkerIconKeys.has(key):
		return xdfileBuildFileMarker("[S]", trimmed)
	case xdfileArchiveMarkerExtensions.has(ext) || key == "zip":
		return xdfileBuildFileMarker("[Z]", trimmed)
	case xdfileExecutableMarkerExtensions.has(ext):
		return xdfileBuildFileMarker("[E]", trimmed)
	default:
		return "", "", false
	}
}

func xdfileBuildFileMarker(label string, name string) (string, lipgloss.Color, bool) {
	color := xdfileColorAccent
	if resolved, ok := xdfileResolveFileColor(name); ok {
		color = resolved
	}
	return label, color, true
}

func xdfileResolveIconKey(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return ""
	}
	if alias, ok := iconconfig.Aliases[name]; ok {
		return alias
	}
	if _, ok := iconconfig.Icons[name]; ok {
		return name
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
	if ext == "" {
		return ""
	}
	if alias, ok := iconconfig.Aliases[ext]; ok {
		return alias
	}
	if _, ok := iconconfig.Icons[ext]; ok {
		return ext
	}
	return ""
}

func xdfileLookupIconColor(key string) (lipgloss.Color, bool) {
	if alias, ok := iconconfig.Aliases[key]; ok {
		key = alias
	}
	style, ok := iconconfig.Icons[key]
	if !ok || style.Color == "" || strings.EqualFold(style.Color, "NONE") {
		return "", false
	}
	return lipgloss.Color(style.Color), true
}

func (m *xdfileModel) renderTerminal() string {
	rect := m.terminalRenderRect()
	innerW := max(10, rect.w-2)
	innerH := max(4, rect.h-2)
	managedPopupVisible := m.managedTerminalPopupVisible()
	terminalActive := m.terminalFocused || m.terminalExpandedViewActive()
	m.layout.terminalInputRect = xdfileRect{}
	m.layout.terminalSuggestionRects = nil

	titleLabel := m.terminal.Cwd
	if m.terminal.Title != "" {
		titleLabel = m.terminal.Title
	}
	titleLeft := xdfileTagStyle.Render("TERMINAL") + " " +
		xdfileTerminalPromptPathStyle.Render(xdfileCompactPath(titleLabel, max(12, innerW-18)))
	titleRight := xdfileTagStyle.Render("xd shell | bound")
	if m.terminalStarting {
		titleRight = xdfileDimStyle.Render("starting")
	}
	if m.terminalUsesPTY() {
		titleRight = xdfileStatusOKStyle.Render("conpty")
		if m.terminal.ScrollOffset > 0 {
			titleRight = xdfileDimStyle.Render(fmt.Sprintf("scroll +%d", m.terminal.ScrollOffset))
		}
		if terminalActive {
			titleRight = xdfileTagStyle.Render("focus")
			if m.terminal.ScrollOffset > 0 {
				titleRight = xdfileTagStyle.Render(fmt.Sprintf("focus +%d", m.terminal.ScrollOffset))
			}
		}
	} else if m.terminal.Busy {
		titleRight = xdfileStatusOKStyle.Render(m.commandStateLabel())
	} else if managedPopupVisible {
		titleRight = xdfileTitleStyle.Render("popup")
	}
	titleLine := xdfileJoinLeftRight(titleLeft, titleRight, innerW)
	blankLine := xdfileBlank(innerW)

	if m.terminalUsesPTY() {
		screenHeight := max(1, innerH-1)
		screen := m.renderPTYTerminalScreen(innerW, screenHeight)
		lines := make([]string, 0, innerH)
		lines = append(lines, titleLine)
		if screen != "" {
			lines = append(lines, strings.Split(screen, "\n")...)
		}
		for len(lines) < innerH {
			lines = append(lines, blankLine)
		}
		return xdfileWrapANSIRender(xdfileTerminalBorder(terminalActive).Width(rect.w - 2).Height(rect.h - 2).
			Render(strings.Join(lines[:innerH], "\n")))
	}
	if m.terminal.StreamEmulator != nil {
		screenHeight := max(1, innerH-2)
		screen := xdfileRenderStreamingTerminalScreen(m.terminal.StreamEmulator, innerW, screenHeight)
		lines := make([]string, 0, innerH)
		lines = append(lines, titleLine)
		if screen != "" {
			lines = append(lines, strings.Split(screen, "\n")...)
		}
		lines = append(lines, xdfilePadRight(m.terminal.Input.View(), innerW))
		for len(lines) < innerH {
			lines = append(lines, blankLine)
		}
		return xdfileWrapANSIRender(xdfileTerminalBorder(terminalActive || managedPopupVisible).Width(rect.w - 2).Height(rect.h - 2).
			Render(strings.Join(lines[:innerH], "\n")))
	}

	viewportLines := xdfileManagedTerminalViewportLines(
		m.terminal.ViewportContent,
		m.terminal.Viewport.YOffset,
		m.terminal.Viewport.Height,
	)
	inputLine := xdfilePadRight(m.terminal.Input.View(), innerW)

	lines := make([]string, 0, innerH)
	lines = append(lines, titleLine)
	bodyHeight := max(0, innerH-2)
	if bodyHeight > 0 && len(viewportLines) > bodyHeight {
		viewportLines = viewportLines[len(viewportLines)-bodyHeight:]
	} else if bodyHeight == 0 {
		viewportLines = nil
	}
	inputRow := innerH - 1
	m.layout.terminalInputRect = xdfileRect{
		x: rect.x + 1,
		y: rect.y + 1 + inputRow,
		w: innerW,
		h: 1,
	}
	lines = append(lines, viewportLines...)
	for len(lines) < inputRow {
		lines = append(lines, blankLine)
	}
	lines = append(lines, inputLine)
	for len(lines) < innerH {
		lines = append(lines, blankLine)
	}

	rendered := xdfileTerminalBorder(terminalActive || managedPopupVisible).Width(rect.w - 2).Height(rect.h - 2).
		Render(strings.Join(lines[:innerH], "\n"))
	if managedPopupVisible {
		popup := m.renderManagedTerminalSuggestionPopup(innerW, max(0, innerH-2))
		if popup != "" {
			popupHeight := lipgloss.Height(popup)
			overlayY := max(1, 1+inputRow-popupHeight)
			rendered = stringfunction.PlaceOverlay(1, overlayY, popup, rendered)
		}
	}

	return xdfileWrapANSIRender(rendered)
}

func (m *xdfileModel) renderManagedTerminalSuggestionPopup(width int, maxHeight int) string {
	if !m.managedTerminalPopupVisible() || width <= 0 || maxHeight < 3 {
		return ""
	}
	m.layout.terminalSuggestionRects = nil

	items := m.terminal.Suggestions
	maxItems := max(1, min(8, maxHeight-2))
	maxSuggestions := max(0, maxItems-1)
	if len(items) > maxSuggestions {
		items = items[:maxSuggestions]
	}

	contentWidth := 18
	for _, item := range items {
		contentWidth = max(contentWidth, lipgloss.Width(item)+2)
	}
	contentWidth = min(contentWidth, max(12, width-2))
	lines := make([]string, 0, len(items)+1)
	blankStyle := xdfileMenuItemStyle
	if m.terminal.SuggestionCursor == 0 {
		blankStyle = xdfileMenuItemHot
	}
	lines = append(lines, blankStyle.Width(contentWidth).Render(""))
	for i, item := range items {
		style := xdfileMenuItemStyle
		if i+1 == m.terminal.SuggestionCursor {
			style = xdfileMenuItemHot
		}
		lines = append(lines, style.Width(contentWidth).Render(charmansi.Truncate(item, contentWidth, "...")))
	}

	popup := xdfileMenuBorder().Width(contentWidth).Render(strings.Join(lines, "\n"))
	m.layout.terminalSuggestionRects = m.managedTerminalSuggestionHitRects(contentWidth, len(items), lipgloss.Height(popup))
	return popup
}

func (m *xdfileModel) managedTerminalSuggestionHitRects(contentWidth int, itemCount int, popupHeight int) []xdfileButtonRect {
	rect := m.terminalRenderRect()
	innerH := max(4, rect.h-2)
	inputRow := innerH - 1
	if popupHeight <= 0 {
		popupHeight = itemCount + 3
	}
	popupY := rect.y + max(1, 1+inputRow-popupHeight)
	hits := make([]xdfileButtonRect, 0, itemCount+1)
	for index := 0; index <= itemCount; index++ {
		hits = append(hits, xdfileButtonRect{
			Action: xdfileAction(fmt.Sprintf("terminal_suggestion:%d", index)),
			Rect: xdfileRect{
				x: rect.x + 2,
				y: popupY + 1 + index,
				w: contentWidth,
				h: 1,
			},
		})
	}
	return hits
}

func (m *xdfileModel) renderModal() string {
	rect := m.modalRect()
	width := rect.w
	height := rect.h
	innerW := width - 2
	innerH := height - 2

	lines := []string{
		xdfileModalTitleStyle.Render(m.modal.Title),
		"",
	}
	bodyHeight := xdfileModalBodyHeight(innerH)

	switch m.modal.Kind {
	case xdfileModalText:
		m.syncModalViewport()
		if m.modal.PreviewVisual {
			lines = append(lines, strings.Split(m.modal.Text, "\n")...)
		} else {
			lines = append(lines, strings.Split(m.modal.Viewport.View(), "\n")...)
		}
		for len(lines) < 2+bodyHeight {
			lines = append(lines, "")
		}
		lines = append(lines, xdfileDimStyle.Render(charmansi.Truncate(m.modal.Description, innerW, "...")))
	case xdfileModalConfirm:
		for _, line := range xdfileWrapText(m.modal.Description, innerW) {
			lines = append(lines, line)
		}
		lines = append(lines, "")
		choices := []string{"Confirm", "Cancel"}
		cursor := max(0, min(m.modal.ChoiceCursor, len(choices)-1))
		lines = append(lines, xdfileRenderHorizontalModalChoices(choices, cursor, innerW))
		lines = append(lines, "", xdfileDimStyle.Render("Arrow/Tab choose | Enter apply | Esc cancel"))
	case xdfileModalInput:
		for _, line := range xdfileWrapText(m.modal.Description, innerW) {
			lines = append(lines, line)
		}
		m.modal.Input.Width = max(10, innerW-4)
		lines = append(lines, "", m.modal.Input.View(), "", xdfileDimStyle.Render("Enter apply | Esc cancel"))
	case xdfileModalChoice:
		for _, line := range xdfileWrapText(m.modal.Description, innerW) {
			lines = append(lines, line)
		}
		lines = append(lines, "")
		choiceLabels := make([]string, 0, len(m.modal.ChoiceItems))
		for _, item := range m.modal.ChoiceItems {
			choiceLabels = append(choiceLabels, item.Label)
		}
		cursor := max(0, min(m.modal.ChoiceCursor, len(choiceLabels)-1))
		lines = append(lines, xdfileRenderHorizontalModalChoices(choiceLabels, cursor, innerW))
		if cursor >= 0 && cursor < len(m.modal.ChoiceItems) && m.modal.ChoiceItems[cursor].Description != "" {
			lines = append(lines, xdfileDimStyle.Render(charmansi.Truncate(m.modal.ChoiceItems[cursor].Description, innerW, "...")))
		}
		lines = append(lines, "", xdfileDimStyle.Render("Enter choose | Esc cancel"))
	case xdfileModalForm:
		for _, line := range xdfileWrapText(m.modal.Description, innerW) {
			lines = append(lines, line)
		}
		lines = append(lines, "")
		for i := range m.modal.FormFields {
			m.modal.FormFields[i].SetWidth(max(10, innerW-4))
			lines = append(lines, xdfileTagStyle.Render(m.modal.FormFields[i].Label+":"))
			lines = append(lines, m.modal.FormFields[i].ViewLines()...)
			lines = append(lines, "")
		}
		hint := "Tab move | Enter apply | Esc cancel"
		if m.modalFormHasMultilineField() {
			hint = "Tab move | Enter apply | Shift+Enter newline | Esc cancel"
		}
		lines = append(lines, xdfileDimStyle.Render(hint))
	}

	for len(lines) < innerH {
		lines = append(lines, "")
	}

	return xdfileModalBorder().Width(width - 2).Height(height - 2).Render(strings.Join(lines[:innerH], "\n"))
}

func xdfileRenderHorizontalModalChoices(labels []string, cursor int, width int) string {
	if len(labels) == 0 || width <= 0 {
		return ""
	}
	segmentWidth, gap := xdfileHorizontalModalChoiceGeometry(width, len(labels))
	segments := make([]string, 0, len(labels))
	for i, label := range labels {
		prefix := "  "
		style := xdfileMenuItemStyle
		text := prefix + label
		if i == cursor {
			prefix = "> "
			style = xdfileMenuItemHot
			text = prefix + label + " "
		}
		rendered := style.Render(charmansi.Truncate(text, segmentWidth, "..."))
		segments = append(segments, xdfilePadRight(rendered, segmentWidth))
	}
	return strings.Join(segments, xdfileBlank(gap))
}

func xdfileHorizontalModalChoiceGeometry(width int, count int) (int, int) {
	if width <= 0 || count <= 0 {
		return 0, 0
	}
	gap := 2
	available := width - gap*(count-1)
	if available < count {
		gap = 1
		available = width - gap*(count-1)
	}
	if available < count {
		gap = 0
		available = width
	}
	return max(1, available/count), gap
}

func xdfileHorizontalModalChoiceIndexAt(innerX int, width int, count int) (int, bool) {
	if innerX < 0 || width <= 0 || count <= 0 {
		return 0, false
	}
	segmentWidth, gap := xdfileHorizontalModalChoiceGeometry(width, count)
	stride := segmentWidth + gap
	for i := 0; i < count; i++ {
		start := i * stride
		end := start + segmentWidth
		if innerX >= start && innerX < end {
			return i, true
		}
	}
	return 0, false
}

func (m *xdfileModel) modalChoiceIndexAt(x int, y int) (int, bool) {
	rect := m.modalRect()
	innerX := x - (rect.x + 1)
	innerY := y - (rect.y + 1)
	if innerY < 0 || innerY >= max(0, rect.h-2) {
		return 0, false
	}

	innerW := max(1, rect.w-2)
	choiceLine := 3 + len(xdfileWrapText(m.modal.Description, innerW))
	if innerY != choiceLine {
		return 0, false
	}
	return xdfileHorizontalModalChoiceIndexAt(innerX, innerW, len(m.modal.ChoiceItems))
}

func (m *xdfileModel) modalConfirmChoiceIndexAt(x int, y int) (int, bool) {
	rect := m.modalRect()
	innerX := x - (rect.x + 1)
	innerY := y - (rect.y + 1)
	if innerY < 0 || innerY >= max(0, rect.h-2) {
		return 0, false
	}
	innerW := max(1, rect.w-2)
	firstChoiceLine := 3 + len(xdfileWrapText(m.modal.Description, innerW))
	if innerY != firstChoiceLine {
		return 0, false
	}
	return xdfileHorizontalModalChoiceIndexAt(innerX, innerW, 2)
}

func (m *xdfileModel) modalFormFieldIndexAt(y int) (int, bool) {
	rect := m.modalRect()
	innerY := y - (rect.y + 1)
	if innerY < 0 || innerY >= max(0, rect.h-2) {
		return 0, false
	}

	currentLine := 3 + len(xdfileWrapText(m.modal.Description, max(1, rect.w-2)))
	for i := range m.modal.FormFields {
		if innerY == currentLine {
			return i, true
		}
		currentLine++
		fieldHeight := m.modal.FormFields[i].displayHeight()
		if innerY >= currentLine && innerY < currentLine+fieldHeight {
			return i, true
		}
		currentLine += fieldHeight + 1
	}
	return 0, false
}

func (m *xdfileModel) modalFormHasMultilineField() bool {
	for i := range m.modal.FormFields {
		if m.modal.FormFields[i].Multiline {
			return true
		}
	}
	return false
}

func (m *xdfileModel) modalRect() xdfileRect {
	if m.modal.Action == xdfileActionPreview {
		return xdfileRect{
			x: max(0, (m.width-min(96, max(50, m.width-6)))/2),
			y: max(0, (m.height-min(24, max(12, m.height-5)))/2),
			w: min(96, max(50, m.width-6)),
			h: min(24, max(12, m.height-5)),
		}
	}
	return xdfileRect{
		x: max(0, (m.width-min(88, max(44, m.width-10)))/2),
		y: max(0, (m.height-min(22, max(10, m.height-8)))/2),
		w: min(88, max(44, m.width-10)),
		h: min(22, max(10, m.height-8)),
	}
}

func (m *xdfileModel) syncModalViewport() {
	if m.modal.Kind != xdfileModalText {
		return
	}

	rect := m.modalRect()
	innerW := rect.w - 2
	innerH := rect.h - 2
	bodyHeight := xdfileModalBodyHeight(innerH)

	if m.modal.Action == xdfileActionPreview {
		m.refreshPreviewModalContent(max(1, innerW), bodyHeight)
		if m.modal.PreviewVisual {
			return
		}
	}

	m.modal.Viewport.Width = max(1, innerW)
	m.modal.Viewport.Height = bodyHeight

	xdfileSetTruncatedViewportContent(&m.modal.Viewport, m.modal.Text, m.modal.Viewport.Width)
}

func xdfileModalBodyHeight(innerH int) int {
	return max(1, innerH-3)
}

func (m *xdfileModel) buildPreviewContent(path string, width int, height int, binary bool, docked bool) xdfilePreviewContent {
	if path == "" {
		return xdfilePreviewContent{
			Text:        xdfileDimStyle.Render("No preview target"),
			Description: "Ctrl+Q close",
		}
	}

	description := "Ctrl+Q close | Up/Down/PgUp/PgDn scroll | Wheel scroll"
	if docked {
		description = "Auto preview | Ctrl+Q close | Wheel scroll"
	}
	if xdfileIsNetBoxPath(path) {
		return xdfilePreviewContent{
			Text:        "Remote preview is unavailable. Copy the file locally first.",
			Description: description,
			Visual:      false,
		}
	}
	showBinaryToggle := xdfilePreviewCanToggleBinary(path)

	if binary {
		preview, err := xdfileReadBinaryPreviewPath(path)
		if err != nil {
			preview = err.Error()
		}
		if showBinaryToggle {
			if docked {
				description = "Auto preview | B normal | Ctrl+Q close | Wheel scroll"
			} else {
				description = "B normal | Ctrl+Q close | Up/Down/PgUp/PgDn scroll | Wheel scroll"
			}
		}
		return xdfilePreviewContent{
			Text:        preview,
			Description: description,
			Visual:      false,
		}
	}

	if xdfilePreviewCanUseThumbnailForContext(path, docked) {
		rendered, ok, err := xdfileRenderPreviewThumbnailFunc(m, path, width, height)
		if err == nil && ok && rendered != "" {
			if showBinaryToggle {
				if docked {
					description = "Auto preview | Ctrl+B binary | Ctrl+Q close"
				} else {
					description = "Ctrl+B binary | Ctrl+Q close"
				}
			} else if docked {
				description = "Auto preview | Ctrl+Q close"
			} else {
				description = "Ctrl+Q close"
			}
			return xdfilePreviewContent{
				Text:        rendered,
				Description: description,
				Visual:      true,
			}
		}
	}

	preview, err := xdfileReadPreview(path)
	if err != nil {
		preview = err.Error()
	}
	if showBinaryToggle {
		if docked {
			description = "Auto preview | Ctrl+B binary | Ctrl+Q close | Wheel scroll"
		} else {
			description = "Ctrl+B binary | Ctrl+Q close | Up/Down/PgUp/PgDn scroll | Wheel scroll"
		}
	}
	return xdfilePreviewContent{
		Text:        preview,
		Description: description,
		Visual:      false,
	}
}

func xdfilePreviewCanUseThumbnailForContext(path string, docked bool) bool {
	if docked && strings.EqualFold(filepath.Ext(path), ".pdf") {
		return false
	}
	return xdfilePreviewCanUseThumbnail(path)
}

func (m *xdfileModel) refreshPreviewModalContent(width int, height int) {
	if m.modal.Action != xdfileActionPreview || m.modal.PreviewPath == "" {
		return
	}

	content := m.buildPreviewContent(m.modal.PreviewPath, width, height, m.modal.PreviewBinary, false)
	m.modal.Text = content.Text
	m.modal.PreviewVisual = content.Visual
	m.modal.Description = content.Description
}

func (m *xdfileModel) syncQuickViewViewport() {
	if !m.quickViewActive() {
		return
	}

	quickViewIndex := m.quickViewPanelIndex()
	if quickViewIndex < 0 {
		return
	}
	rect := m.layout.panelRects[quickViewIndex]
	if rect.w == 0 || rect.h == 0 {
		return
	}

	innerW := max(10, rect.w-2)
	innerH := max(4, rect.h-2)
	bodyHeight := max(1, innerH-2)

	panel := &m.panels[m.activePanel]
	entry, ok := panel.selected()
	if !ok {
		m.quickView.Title = "Quick View"
		m.quickView.Path = ""
		m.quickView.Text = xdfileDimStyle.Render("No selection")
		m.quickView.Description = "Ctrl+Q close"
		m.quickView.Visual = false
		m.quickView.Viewport.Width = innerW
		m.quickView.Viewport.Height = bodyHeight
		m.quickView.Viewport.SetContent(m.quickView.Text)
		return
	}

	title := entry.Name
	path := entry.Path
	if entry.IsParent {
		title = ".."
	}

	if m.quickView.Path != path || m.quickView.ContentW != innerW || m.quickView.ContentH != bodyHeight || m.quickView.Title != title {
		content := m.buildPreviewContent(path, innerW, bodyHeight, m.quickView.Binary, true)
		m.quickView.Path = path
		m.quickView.Title = title
		m.quickView.Text = content.Text
		m.quickView.Description = content.Description
		m.quickView.Visual = content.Visual
		m.quickView.ContentW = innerW
		m.quickView.ContentH = bodyHeight
		m.quickView.Viewport.GotoTop()
	}

	if m.quickView.Visual {
		return
	}

	m.quickView.Viewport.Width = innerW
	m.quickView.Viewport.Height = bodyHeight
	xdfileSetTruncatedViewportContent(&m.quickView.Viewport, m.quickView.Text, m.quickView.Viewport.Width)
}

func (m *xdfileModel) renderPreviewThumbnail(path string, width int, height int) (string, bool, error) {
	if m.imagePreviewer == nil || width <= 0 || height <= 0 {
		return "", false, nil
	}

	previewPath := path
	ext := strings.ToLower(filepath.Ext(path))
	switch {
	case xdfilePreviewCanUseImageThumbnail(path):
	case ext == ".pdf":
		if m.thumbnailGenerator == nil || !m.thumbnailGenerator.SupportsExt(ext) {
			return "", false, nil
		}
		thumbnailPath, err := m.thumbnailGenerator.GetThumbnailOrGenerate(path)
		if err != nil {
			return "", false, err
		}
		previewPath = thumbnailPath
	default:
		return "", false, nil
	}

	rendered, err := m.imagePreviewer.ImagePreviewWithRenderer(
		previewPath,
		width,
		height,
		string(xdfileColorSurface),
		filepreview.RendererANSI,
		0,
	)
	if err != nil {
		return "", false, err
	}
	return rendered, true, nil
}

func (m *xdfileModel) renderFunctionButtons(buttons []xdfileButton, y int) (string, []xdfileButtonRect) {
	x := 0
	hits := make([]xdfileButtonRect, 0, len(buttons))
	var builder strings.Builder

	for i, button := range buttons {
		if i > 0 {
			builder.WriteByte(' ')
			x++
		}
		label := " " + button.Label + "  "
		rendered := xdfileButtonKeyStyle.Render(button.Key) +
			xdfileDimStyle.Render(label)
		if m.hover.FooterAction == button.Action {
			rendered = xdfileHoveredFooterKeyStyle().Render(button.Key) +
				xdfileHoveredFooterLabelStyle().Render(label)
		}
		width := lipgloss.Width(rendered)
		hits = append(hits, xdfileButtonRect{
			Action: button.Action,
			Rect:   xdfileRect{x: x, y: y, w: width, h: 1},
		})
		builder.WriteString(rendered)
		x += width
	}
	return xdfilePadRight(builder.String(), m.width), hits
}
