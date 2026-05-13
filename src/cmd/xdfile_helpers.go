package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	charmansi "github.com/charmbracelet/x/ansi"
)

func xdfileResolveStartPaths(paths []string, prefs xdfileLayoutPrefs) (string, string) {
	cleanedPaths := make([]string, 0, len(paths))
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			continue
		}
		cleanedPaths = append(cleanedPaths, path)
	}
	paths = cleanedPaths

	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	left := cwd
	right := cwd

	if len(paths) == 0 {
		if prefs.StartupLeftPath != "" {
			if p, resolveErr := xdfileNormalizePath(prefs.StartupLeftPath); resolveErr == nil {
				left = p
			}
		}
		if prefs.StartupRightPath != "" {
			if p, resolveErr := xdfileNormalizePath(prefs.StartupRightPath); resolveErr == nil {
				right = p
			} else {
				right = left
			}
		} else {
			right = left
		}
		return left, right
	}

	if p, resolveErr := xdfileNormalizePath(paths[0]); resolveErr == nil {
		left = p
	}
	if len(paths) > 1 {
		if p, resolveErr := xdfileNormalizePath(paths[1]); resolveErr == nil {
			right = p
		}
	} else {
		right = left
	}
	return left, right
}

func xdfileNormalizePath(path string) (string, error) {
	if path == "" {
		return os.Getwd()
	}
	if xdfileIsNetBoxPath(path) {
		return path, nil
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return abs, nil
	}
	return filepath.Dir(abs), nil
}

func xdfilePathsEqual(a string, b string) bool {
	if xdfileIsNetBoxPath(a) || xdfileIsNetBoxPath(b) {
		return xdfileNetBoxPathsEqual(a, b)
	}
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	if os.PathSeparator == '\\' {
		return strings.EqualFold(a, b)
	}
	return a == b
}

type xdfileStringSet map[string]struct{}

func (s xdfileStringSet) has(value string) bool {
	_, ok := s[value]
	return ok
}

func xdfileJoinLeftRight(left string, right string, width int) string {
	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	if leftWidth+rightWidth+1 >= width {
		return xdfilePadRight(charmansi.Truncate(left+" "+right, width, "..."), width)
	}
	return left + xdfileBlank(width-leftWidth-rightWidth) + right
}

func xdfileJoinLeftRightPreserveRight(left string, right string, width int, rightReserve int) string {
	if width <= 0 {
		return ""
	}

	rightWidth := lipgloss.Width(right)
	if rightWidth >= width {
		return xdfileAlignRight(xdfileTruncateLeft(right, width, "..."), width)
	}

	rightReserve = max(rightWidth, min(width-1, rightReserve))
	leftBudget := max(0, width-rightReserve-1)
	if leftBudget == 0 {
		return xdfileAlignRight(right, width)
	}

	left = charmansi.Truncate(left, leftBudget, "...")
	return xdfileJoinLeftRight(left, right, width)
}

type xdfileCompactPathKey struct {
	Path  string
	Width int
}

func xdfileCompactPath(path string, width int) string {
	if width <= 0 {
		return ""
	}
	if xdfileIsNetBoxPath(path) {
		path = xdfileNetBoxPathLabel(path)
	} else {
		path = filepath.Clean(path)
	}
	key := xdfileCompactPathKey{Path: path, Width: width}
	if compacted, ok := xdfileCompactPathCache[key]; ok {
		return compacted
	}

	compacted := path
	if lipgloss.Width(path) > width {
		compacted = xdfileTruncateLeft(path, width, "...")
	}
	if xdfileCompactPathCache == nil || len(xdfileCompactPathCache) >= xdfileCompactPathCacheMax {
		xdfileCompactPathCache = make(map[xdfileCompactPathKey]string, xdfileCompactPathCacheMax)
	}
	xdfileCompactPathCache[key] = compacted
	return compacted
}

func xdfileBlank(width int) string {
	if width <= 0 {
		return ""
	}
	if blank, ok := xdfileBlankStrings[width]; ok {
		return blank
	}
	if xdfileBlankStrings == nil {
		xdfileBlankStrings = map[int]string{0: ""}
	}
	blank := strings.Repeat(" ", width)
	xdfileBlankStrings[width] = blank
	return blank
}

func xdfilePadRight(value string, width int) string {
	value = charmansi.Truncate(value, width, "...")
	padding := max(0, width-lipgloss.Width(value))
	return value + xdfileBlank(padding)
}

func xdfileAlignRight(value string, width int) string {
	value = charmansi.Truncate(value, width, "...")
	padding := max(0, width-lipgloss.Width(value))
	return xdfileBlank(padding) + value
}

func xdfileTruncateLeft(value string, width int, prefix string) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(value) <= width {
		return value
	}
	prefixWidth := lipgloss.Width(prefix)
	if prefixWidth >= width {
		return charmansi.Truncate(prefix, width, "")
	}
	keepWidth := width - prefixWidth
	valueWidth := lipgloss.Width(value)
	return prefix + charmansi.Cut(value, max(0, valueWidth-keepWidth), valueWidth)
}

func xdfilePanelNameWidth(width int) int {
	return max(12, width-xdfilePanelSizeWidth-xdfilePanelTimeWidth-2)
}

func xdfileRoundedHeavyBorder() lipgloss.Border {
	return lipgloss.Border{
		Top:         "━",
		Bottom:      "━",
		Left:        "┃",
		Right:       "┃",
		TopLeft:     "┏",
		TopRight:    "┓",
		BottomLeft:  "┗",
		BottomRight: "┛",
	}
}

func xdfilePanelBorder(active bool) lipgloss.Style {
	if active {
		return xdfilePanelBorderActiveStyle
	}
	return xdfilePanelBorderInactiveStyle
}

func xdfileTerminalBorder(active bool) lipgloss.Style {
	if active {
		return xdfileTerminalBorderActiveStyle
	}
	return xdfileTerminalBorderInactiveStyle
}

func xdfileModalBorder() lipgloss.Style {
	return xdfileModalBorderStyle
}

func xdfileMenuBorder() lipgloss.Style {
	return xdfileMenuBorderStyle
}

func isXdfileMenuAction(action xdfileAction) bool {
	switch action {
	case xdfileActionPanelsMenu, xdfileActionViewMenu, xdfileActionTerminalMenu, xdfileActionNetBoxMenu, xdfileActionThemeMenu, xdfileActionOptionsMenu:
		return true
	default:
		return false
	}
}

func (m *xdfileModel) menuDefinitions() []xdfileMenu {
	return []xdfileMenu{
		{
			Action: xdfileActionPanelsMenu,
			Label:  "Panels",
			Items: []xdfileButton{
				{Action: xdfileActionCommandsMenu, Key: "F2", Label: "Commands"},
				{Action: xdfileActionParent, Label: "Parent"},
				{Action: xdfileActionSync, Label: "Sync"},
				{Action: xdfileActionRefresh, Key: "R", Label: "Refresh"},
			},
		},
		{
			Action: xdfileActionViewMenu,
			Label:  "View",
			Items: []xdfileButton{
				{Action: xdfileActionHidden, Key: "F9", Label: xdfileHiddenLabel(m.showHidden)},
				{Action: xdfileActionSortName, Key: "Ctrl+3", Label: "Sort by Name"},
				{Action: xdfileActionSortExt, Key: "Ctrl+4", Label: "Sort by Extens"},
			},
		},
		{
			Action: xdfileActionTerminalMenu,
			Label:  "Terminal",
			Items: []xdfileButton{
				{Action: xdfileActionTerminalExpand, Key: "Ctrl+O", Label: "Expand terminal"},
			},
		},
		m.netBoxMenuDefinition(),
		{
			Action: xdfileActionThemeMenu,
			Label:  "Theme",
			Items: []xdfileButton{
				{Action: xdfileActionThemePersona3, Label: xdfileThemeMenuLabel(xdfileThemePersona3Name, m.layoutPrefs.ThemeName)},
				{Action: xdfileActionThemePersona3Reload, Label: xdfileThemeMenuLabel(xdfileThemePersona3ReloadName, m.layoutPrefs.ThemeName)},
				{Action: xdfileActionThemePersona3Kotone, Label: xdfileThemeMenuLabel(xdfileThemePersona3KotoneName, m.layoutPrefs.ThemeName)},
				{Action: xdfileActionThemePersona4, Label: xdfileThemeMenuLabel(xdfileThemePersona4Name, m.layoutPrefs.ThemeName)},
				{Action: xdfileActionThemePersona5, Label: xdfileThemeMenuLabel(xdfileThemePersona5Name, m.layoutPrefs.ThemeName)},
			},
		},
		{
			Action: xdfileActionOptionsMenu,
			Label:  "Options",
			Items: []xdfileButton{
				{Action: xdfileActionSaveLayout, Key: "Shift+F9", Label: "Save setup"},
				{Action: xdfileActionResetLayout, Label: "Reset setup"},
				{Action: xdfileActionQuickViewMode, Label: xdfileQuickViewModeLabel(m.layoutPrefs.QuickViewDocked)},
			},
		},
	}
}

func (m *xdfileModel) currentMenu() (xdfileMenu, bool) {
	if m.openMenu == xdfileActionContextMenu {
		return m.contextMenu, len(m.contextMenu.Items) > 0
	}
	if m.openMenu == xdfileActionCommandsMenu {
		menu := m.commandMenuDefinition()
		return menu, len(menu.Items) > 0
	}
	for _, menu := range m.menuDefinitions() {
		if menu.Action == m.openMenu {
			return menu, true
		}
	}
	return xdfileMenu{}, false
}

func (m *xdfileModel) menuAnchorRect(action xdfileAction) xdfileRect {
	if action == xdfileActionContextMenu {
		return m.contextMenuAnchor
	}
	if action == xdfileActionCommandsMenu {
		return xdfileRect{
			x: max(0, (m.width-40)/2),
			y: max(3, (m.height-14)/2),
			w: 1,
			h: 1,
		}
	}
	for _, hit := range m.layout.menuButtons {
		if hit.Action == action {
			return hit.Rect
		}
	}
	return xdfileRect{}
}

func (m *xdfileModel) toggleMenu(action xdfileAction) tea.Cmd {
	label := "menu"
	for _, menu := range m.menuDefinitions() {
		if menu.Action == action {
			label = menu.Label
			break
		}
	}
	if m.openMenu == action {
		m.closeMenu("Closed %s", xdfileMenuStatusLabel(label))
		return nil
	}
	m.contextMenu = xdfileMenu{}
	m.contextMenuAnchor = xdfileRect{}
	m.openMenu = action
	m.menuCursor = 0
	m.clearMouseHover()
	if menu, ok := m.currentMenu(); ok {
		m.menuCursor = xdfileFirstSelectableMenuIndex(menu)
	}
	m.setStatus("Opened %s", xdfileMenuStatusLabel(label))
	return nil
}

func xdfileMenuStatusLabel(label string) string {
	label = strings.TrimSpace(label)
	if label == "" {
		return "menu"
	}
	if strings.HasSuffix(strings.ToLower(label), "menu") {
		return label
	}
	return label + " menu"
}

func (m *xdfileModel) openAdjacentMenu(delta int) {
	menus := m.menuDefinitions()
	if len(menus) == 0 {
		return
	}
	index := 0
	for i, menu := range menus {
		if menu.Action == m.openMenu {
			index = i
			break
		}
	}
	index = (index + delta + len(menus)) % len(menus)
	m.contextMenu = xdfileMenu{}
	m.contextMenuAnchor = xdfileRect{}
	m.openMenu = menus[index].Action
	m.menuCursor = xdfileFirstSelectableMenuIndex(menus[index])
	m.clearMouseHover()
	m.setStatus("Opened %s", xdfileMenuStatusLabel(menus[index].Label))
}

func (m *xdfileModel) closeMenu(format string, args ...any) {
	m.contextMenu = xdfileMenu{}
	m.contextMenuAnchor = xdfileRect{}
	m.commandMenuPath = nil
	m.openMenu = ""
	m.menuCursor = 0
	m.clearMouseHover()
	m.setStatus(format, args...)
}

func xdfileMenuItemSelectable(item xdfileButton) bool {
	return item.Action != "" && !item.Disabled
}

func xdfileFirstSelectableMenuIndex(menu xdfileMenu) int {
	for i, item := range menu.Items {
		if xdfileMenuItemSelectable(item) {
			return i
		}
	}
	return -1
}

func xdfileValidMenuCursor(menu xdfileMenu, current int) int {
	if current >= 0 && current < len(menu.Items) && xdfileMenuItemSelectable(menu.Items[current]) {
		return current
	}
	return xdfileFirstSelectableMenuIndex(menu)
}

func xdfileNormalizeMenuHotkey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func xdfileMenuIndexForHotkey(menu xdfileMenu, input string) (int, bool) {
	input = xdfileNormalizeMenuHotkey(input)
	if input == "" {
		return 0, false
	}
	for i, item := range menu.Items {
		if !xdfileMenuItemSelectable(item) {
			continue
		}
		if xdfileNormalizeMenuHotkey(item.Key) == input {
			return i, true
		}
	}
	return 0, false
}

func xdfileNextSelectableMenuIndex(menu xdfileMenu, current int, delta int) int {
	if len(menu.Items) == 0 {
		return -1
	}
	if delta == 0 {
		return xdfileValidMenuCursor(menu, current)
	}
	if current < 0 || current >= len(menu.Items) {
		current = xdfileFirstSelectableMenuIndex(menu)
		if current < 0 {
			return -1
		}
	}
	index := current
	for i := 0; i < len(menu.Items); i++ {
		index = (index + delta + len(menu.Items)) % len(menu.Items)
		if xdfileMenuItemSelectable(menu.Items[index]) {
			return index
		}
	}
	return xdfileValidMenuCursor(menu, current)
}

func xdfileSelectedLineStyle(active bool) lipgloss.Style {
	if active {
		return xdfileSelectedLineActiveStyle
	}
	return xdfileSelectedLineInactiveStyle
}

func xdfileInactiveCursorLineStyle() lipgloss.Style {
	return xdfileInactiveCursorStyle
}

func xdfileHoveredEntryLineStyle() lipgloss.Style {
	return xdfileHoveredEntryLineCachedStyle
}

func xdfileHoveredEntryNameStyle(entry xdfileEntry) lipgloss.Style {
	return xdfileEntryNameStyle(entry).
		Background(xdfileColorHighlight)
}

func xdfileHoveredEntryMetaStyle() lipgloss.Style {
	return xdfileHoveredEntryMetaCachedStyle
}

func xdfileHoveredMenuButtonStyle() lipgloss.Style {
	return xdfileHoveredMenuButtonCachedStyle
}

func xdfileHoveredMenuItemStyle() lipgloss.Style {
	return xdfileHoveredMenuItemCachedStyle
}

func xdfileHoveredFooterKeyStyle() lipgloss.Style {
	return xdfileHoveredFooterKeyCachedStyle
}

func xdfileHoveredFooterLabelStyle() lipgloss.Style {
	return xdfileHoveredFooterLabelCachedStyle
}

type xdfileEntryKindSpec struct {
	Label string
	Color lipgloss.Color
	Bold  bool
}

type xdfileEntryKindRenderKey struct {
	Spec       xdfileEntryKindSpec
	Background lipgloss.Color
}

func (spec xdfileEntryKindSpec) render() string {
	if rendered, ok := xdfileEntryKindRenders[spec]; ok {
		return rendered
	}
	if xdfileEntryKindRenders == nil {
		xdfileEntryKindRenders = make(map[xdfileEntryKindSpec]string, 32)
	}
	style := lipgloss.NewStyle().Foreground(spec.Color)
	if spec.Bold {
		style = style.Bold(true)
	}
	rendered := style.Render(spec.Label)
	xdfileEntryKindRenders[spec] = rendered
	return rendered
}

func (spec xdfileEntryKindSpec) renderOn(background lipgloss.Color) string {
	key := xdfileEntryKindRenderKey{Spec: spec, Background: background}
	if rendered, ok := xdfileEntryKindOnRenders[key]; ok {
		return rendered
	}
	if xdfileEntryKindOnRenders == nil {
		xdfileEntryKindOnRenders = make(map[xdfileEntryKindRenderKey]string, 8)
	}
	style := lipgloss.NewStyle().
		Foreground(spec.Color).
		Background(background)
	if spec.Bold {
		style = style.Bold(true)
	}
	rendered := style.Render(spec.Label)
	xdfileEntryKindOnRenders[key] = rendered
	return rendered
}

func xdfileRenderHoveredEntryNameColumn(entry xdfileEntry, prefix string, width int) string {
	prefixKind := xdfileHoveredEntryLineStyle().Render(prefix) +
		xdfileEntryKindSpecForEntry(entry).renderOn(xdfileColorHighlight)
	fixedWidth := lipgloss.Width(prefixKind) + 2
	nameWidth := max(0, width-fixedWidth)
	name := charmansi.Truncate(entry.Name, nameWidth, "...")

	marker := xdfileHoveredEntryLineStyle().Render(" ")
	if entry.GitMarker != "" {
		marker = xdfileGitMarkerStyle(entry.GitMarker).
			Background(xdfileColorHighlight).
			Render(entry.GitMarker)
	}

	content := prefixKind +
		marker +
		xdfileHoveredEntryLineStyle().Render(" ") +
		xdfileHoveredEntryNameStyle(entry).Render(name)
	padding := max(0, width-lipgloss.Width(content))
	if padding > 0 {
		content += xdfileHoveredEntryLineStyle().Render(xdfileBlank(padding))
	}
	return content
}

func xdfileRenderHoveredEntryMetaColumn(value string, width int) string {
	value = charmansi.Truncate(value, width, "...")
	padding := max(0, width-lipgloss.Width(value))
	if value == "" {
		return xdfileHoveredEntryLineStyle().Render(xdfileBlank(width))
	}
	return xdfileHoveredEntryLineStyle().Render(xdfileBlank(padding)) +
		xdfileHoveredEntryMetaStyle().Render(value)
}

func xdfileMarkedLineStyle(active bool) lipgloss.Style {
	if active {
		return xdfileMarkedLineActiveStyle
	}
	return xdfileMarkedLineInactiveStyle
}

func xdfileEntryKindSpecForEntry(entry xdfileEntry) xdfileEntryKindSpec {
	switch {
	case entry.IsParent:
		return xdfileEntryKindSpec{Label: "[..]", Color: xdfileColorDim}
	case entry.IsDir:
		return xdfileEntryKindSpec{Label: "[D]", Color: xdfileColorAccent2, Bold: true}
	default:
		if label, color, ok := xdfileResolveFileMarker(entry.Name); ok {
			return xdfileEntryKindSpec{Label: label, Color: color, Bold: true}
		}
		return xdfileEntryKindSpec{Label: "[ ]", Color: xdfileColorBorder}
	}
}

func xdfileEntryKindLabel(entry xdfileEntry) string {
	return xdfileEntryKindSpecForEntry(entry).Label
}

func xdfileEntrySummary(entry xdfileEntry) string {
	if entry.IsParent {
		return "parent directory"
	}
	if entry.IsDir {
		return "directory"
	}
	return xdfileHumanSize(entry.Size)
}

func xdfileHiddenLabel(show bool) string {
	if show {
		return "Hidden On"
	}
	return "Hidden Off"
}

func xdfileQuickViewModeLabel(enabled bool) string {
	if enabled {
		return "Ctrl+Q Docked On"
	}
	return "Ctrl+Q Docked Off"
}

func xdfileStartupTerminalPrompt() string {
	return "XD> "
}

func xdfileLoadTerminalHistorySeed() []string {
	if runtime.GOOS != "windows" {
		return nil
	}

	appData := os.Getenv("APPDATA")
	if appData == "" {
		return nil
	}

	paths := []string{
		filepath.Join(appData, "Microsoft", "Windows", "PowerShell", "PSReadLine", "ConsoleHost_history.txt"),
	}
	seenPaths := make(map[string]struct{}, len(paths))
	seen := make(map[string]struct{}, 400)
	history := make([]string, 0, 400)
	for _, path := range paths {
		path = filepath.Clean(path)
		if _, ok := seenPaths[path]; ok {
			continue
		}
		seenPaths[path] = struct{}{}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
		for i := len(lines) - 1; i >= 0; i-- {
			line := strings.TrimSpace(lines[i])
			if line == "" {
				continue
			}
			if _, ok := seen[line]; ok {
				continue
			}
			seen[line] = struct{}{}
			history = append(history, line)
			if len(history) >= 400 {
				break
			}
		}
		if len(history) >= 400 {
			break
		}
	}

	for i, j := 0, len(history)-1; i < j; i, j = i+1, j-1 {
		history[i], history[j] = history[j], history[i]
	}
	return history
}

func xdfileWrapText(text string, width int) []string {
	if width <= 0 {
		return nil
	}
	var lines []string
	for _, raw := range strings.Split(text, "\n") {
		if raw == "" {
			lines = append(lines, "")
			continue
		}
		current := raw
		for lipgloss.Width(current) > width {
			chunk := charmansi.Truncate(current, width, "")
			lines = append(lines, chunk)
			current = strings.TrimPrefix(current, chunk)
		}
		lines = append(lines, current)
	}
	return lines
}

func xdfileHelpText() string {
	return strings.Join([]string{
		"Xdfile Manager dual-pane workflow",
		"",
		"Tab / Shift+Tab  switch between the left and right panels",
		"Ctrl+Left/Right move the vertical split between both panels",
		"Ctrl+Up/Down    resize the bottom terminal against the file panels",
		"Top menus        Panels / View / Terminal / Theme / Options live on the top-left",
		"Shift+F9        save setup (Options -> Save setup)",
		"Options          Reset setup restores layout, theme, view options, hidden files, and the user menu defaults",
		"Enter            open directory or launch file",
		"Ctrl+Shift+C / Ctrl+X / Ctrl+Shift+V copy, cut, and paste the selected item when a file panel has focus",
		"Left/Right       move the file cursor by one page up or down",
		"Shift+Up/Down    extend the current file selection",
		"Shift+Left/Right extend selection to the first or last item",
		"Esc              clear the current multi-selection",
		"Mouse            click/double-click, right-click local items for Windows menu, Ctrl click, Shift/Alt range-click, drag-select",
		"",
		"F2  open the user menu (Enter run/open | Ins insert | F4 edit | Del delete)",
		"Ins inside F2   asks whether to insert a command or a submenu; F4 edits the current item",
		"F3  preview the current file",
		"Ctrl+Q toggle preview for the current item or the docked quick view mode",
		"Ctrl+3 sort the active panel by name",
		"Ctrl+4 sort the active panel by extension",
		"Ctrl+B inside preview toggles binary view for files",
		"F4  rename the selected item",
		"F5  copy to the other panel",
		"F6  move to the other panel",
		"F7  create a directory",
		"F8  delete the selected item",
		"Ctrl+Z undo the most recent delete in this session",
		"F9  toggle dotfiles",
		"F10 quit (confirm)",
		"Panels menu       still contains Sync for mirroring the passive panel path",
		"",
		"Command line",
		"",
		"Managed command line  input stays bound to the active panel path",
		"Type text        writes into the command line while panel arrows stay on the file list",
		"Backspace        edits the command line only; use Enter on [..] or Panels -> Parent to go up",
		"Popup arrows     when the command popup is open, Up/Down select and Right/Tab accept",
		"Ctrl+O           expand or restore the bottom terminal inside the Xdfile Manager panel area",
		"Options          Ctrl+Q Docked switches between floating preview and docked quick view",
		"Theme            switches between Persona 3, Persona 3 Reload, Persona 4, and Persona 5 visual styles",
		"PgUp/PgDn        scroll terminal output",
		"Aliases          ls / ll / la / cat are handled by the built-in Xdfile Manager shell",
		"Mouse            clicking a panel keeps panel focus and the command line follows that panel",
	}, "\n")
}
