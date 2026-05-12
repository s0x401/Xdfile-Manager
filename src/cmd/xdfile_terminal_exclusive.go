package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	uv "github.com/charmbracelet/ultraviolet"
	charmansi "github.com/charmbracelet/x/ansi"
)

type xdfileExclusiveTUICommand struct {
	Path       string
	Args       []string
	MouseInput xdfilePTYMouseInputMode
}

var xdfileExclusiveTUICommandNames = xdfileStringSet{
	"btm":     {},
	"fzf":     {},
	"helix":   {},
	"hx":      {},
	"k9s":     {},
	"less":    {},
	"lf":      {},
	"lazygit": {},
	"micro":   {},
	"nnn":     {},
	"nvim":    {},
	"tig":     {},
	"vim":     {},
	"vxdbg":   {},
	"yazi":    {},
}

var xdfileStartExclusiveTerminalFunc = xdfileStartExclusiveTerminalCmd

func xdfileResolveExclusiveTUICommand(dir string, command string) (xdfileExclusiveTUICommand, bool) {
	command = strings.TrimSpace(command)
	if command == "" || xdfileContainsShellOperators(command) {
		return xdfileExclusiveTUICommand{}, false
	}

	fields, err := xdfileSplitShellWords(command)
	if err != nil || len(fields) == 0 {
		return xdfileExclusiveTUICommand{}, false
	}
	return xdfileResolveExclusiveTUICommandFields(dir, fields, 0)
}

func xdfileResolveExclusiveTUICommandFields(dir string, fields []string, depth int) (xdfileExclusiveTUICommand, bool) {
	if depth > 4 || len(fields) == 0 {
		return xdfileExclusiveTUICommand{}, false
	}

	name := strings.TrimSpace(fields[0])
	if name == "" {
		return xdfileExclusiveTUICommand{}, false
	}
	if strings.EqualFold(name, "call") {
		return xdfileResolveExclusiveTUICommandFields(dir, fields[1:], depth+1)
	}

	path, resolved := xdfileResolveExternalExecutablePath(dir, name)
	if resolved {
		base := strings.ToLower(filepath.Base(path))
		if base == "cmd.exe" {
			if nested, ok := xdfileResolveExclusiveTUICmdWrapper(dir, fields[1:], depth+1); ok {
				return nested, true
			}
		}

		base = strings.TrimSuffix(base, filepath.Ext(base))
		if xdfileExclusiveTUICommandNames.has(base) {
			return xdfileExclusiveTUICommand{
				Path:       path,
				Args:       xdfileExclusiveTUICommandArgs(base, fields[1:]),
				MouseInput: xdfileExclusiveTUIMouseInputMode(base),
			}, true
		}
	}

	return xdfileExclusiveTUICommand{}, false
}

func xdfileExclusiveTUICommandArgs(base string, args []string) []string {
	args = append([]string(nil), args...)
	if !strings.EqualFold(base, "vim") {
		return args
	}
	if xdfileExclusiveTUICommandHasMouseArg(args) {
		return args
	}

	prefixed := make([]string, 0, len(args)+2)
	prefixed = append(prefixed, "-c", "silent! if exists('&mouse') | set mouse=a | endif")
	return append(prefixed, args...)
}

func xdfileExclusiveTUICommandHasMouseArg(args []string) bool {
	for _, arg := range args {
		if arg == "--" {
			return false
		}
		lower := strings.ToLower(arg)
		if strings.Contains(lower, "mouse=") || strings.Contains(lower, "ttymouse=") {
			return true
		}
	}
	return false
}

func xdfileExclusiveTUIMouseInputMode(base string) xdfilePTYMouseInputMode {
	if strings.EqualFold(base, "vim") {
		return xdfilePTYMouseInputNative
	}
	return xdfilePTYMouseInputBoth
}

func xdfileResolveExclusiveTUICmdWrapper(dir string, args []string, depth int) (xdfileExclusiveTUICommand, bool) {
	for i, arg := range args {
		lower := strings.ToLower(strings.TrimSpace(arg))
		if lower != "/c" && lower != "/k" {
			continue
		}
		nested := args[i+1:]
		if len(nested) == 0 {
			return xdfileExclusiveTUICommand{}, false
		}
		if len(nested) == 1 {
			parsed, err := xdfileSplitShellWords(nested[0])
			if err == nil && len(parsed) > 0 {
				nested = parsed
			}
		}
		return xdfileResolveExclusiveTUICommandFields(dir, nested, depth+1)
	}
	return xdfileExclusiveTUICommand{}, false
}

func xdfileStartExclusiveTerminalCmd(dir string, command string, width int, height int) tea.Cmd {
	return func() tea.Msg {
		candidate, ok := xdfileResolveExclusiveTUICommand(dir, command)
		if !ok {
			return xdfileExclusiveTerminalStartMsg{
				Command: command,
				Dir:     dir,
				Err:     fmt.Errorf("exclusive TUI command is unavailable"),
			}
		}

		events := make(chan tea.Msg, xdfileTerminalEventBufferSize)
		session, err := xdfileStartExclusiveCommandPTYSession(dir, candidate.Path, candidate.Args, events, width, height, candidate.MouseInput)
		if err != nil {
			return xdfileExclusiveTerminalStartMsg{
				Command: command,
				Dir:     dir,
				Err:     err,
			}
		}

		return xdfileExclusiveTerminalStartMsg{
			Command: command,
			Dir:     dir,
			Session: session,
		}
	}
}

func (m *xdfileModel) exclusiveTerminalActive() bool {
	return m != nil &&
		m.terminal.Exclusive.Session != nil &&
		m.terminal.Exclusive.Session.emulator != nil
}

func (m *xdfileModel) terminalExpandedViewActive() bool {
	return m != nil && m.terminalExpanded && !m.exclusiveTerminalActive()
}

func (m *xdfileModel) toggleTerminalExpandedView() tea.Cmd {
	if m == nil {
		return nil
	}

	if m.terminalExpanded {
		m.terminalExpanded = false
		if m.terminalUsesPTY() {
			m.terminalFocused = m.terminalReturnFocus.Focused
			m.terminalAutoFocused = m.terminalReturnFocus.AutoFocused
		} else {
			m.terminalFocused = false
			m.terminalAutoFocused = false
			m.focusManagedTerminalInput()
		}
		m.terminalReturnFocus = xdfileTerminalFocusState{}
		if m.width > 0 && m.height > 0 {
			m.computeLayout()
			m.syncTerminalViewport(false)
		}
		m.setStatus("Terminal restored")
		return nil
	}

	m.closePanelSearch()
	m.openMenu = ""
	m.clearMouseHover()
	m.terminalReturnFocus = xdfileTerminalFocusState{
		Focused:     m.terminalFocused,
		AutoFocused: m.terminalAutoFocused,
	}
	m.terminalExpanded = true
	if m.terminalUsesPTY() {
		m.terminalFocused = true
		m.terminalAutoFocused = false
		m.setTerminalScrollOffset(0)
	} else {
		m.terminalFocused = false
		m.terminalAutoFocused = false
		m.focusManagedTerminalInput()
	}
	if m.width > 0 && m.height > 0 {
		m.computeLayout()
		m.syncTerminalViewport(false)
	}
	m.setStatus("Terminal expanded")
	return tea.EnableMouseAllMotion
}

func (m *xdfileModel) exclusiveTerminalRenderRect() xdfileRect {
	return m.layout.exclusiveRect
}

func (m *xdfileModel) exclusiveTerminalViewportSize() (int, int) {
	rect := m.exclusiveTerminalRenderRect()
	if rect.w == 0 || rect.h == 0 {
		return 120, 30
	}
	innerWidth := max(10, rect.w-2)
	innerHeight := max(1, rect.h-2)
	return innerWidth, innerHeight
}

func (m *xdfileModel) syncExclusiveTerminalViewport() {
	if !m.exclusiveTerminalActive() {
		return
	}
	width, height := m.exclusiveTerminalViewportSize()
	if err := m.terminal.Exclusive.Session.Resize(width, height); err != nil {
		m.setStatusErr(err)
	}
}

func (m *xdfileModel) closeExclusiveTerminalSession() {
	if m == nil || m.terminal.Exclusive.Session == nil {
		return
	}
	m.terminal.Exclusive.Session.Close()
	m.terminal.Exclusive = xdfileExclusiveTerminal{}
}

func (m *xdfileModel) finishExclusiveTerminal(err error) {
	if m == nil {
		return
	}

	command := strings.TrimSpace(m.terminal.Exclusive.Command)
	m.terminal.Exclusive = xdfileExclusiveTerminal{}
	m.reloadAllPanels()
	m.refreshManagedTerminalSuggestions()
	m.focusManagedTerminalInput()

	switch {
	case err != nil:
		if command != "" {
			m.setStatusErr(fmt.Errorf("%s: %w", command, err))
		} else {
			m.setStatusErr(err)
		}
	case command != "":
		m.setStatus("Closed %s", filepath.Base(strings.Fields(command)[0]))
	default:
		m.setStatus("Exclusive terminal closed")
	}
}

func (m *xdfileModel) exclusiveTerminalTitle() string {
	if m == nil {
		return ""
	}
	if title := strings.TrimSpace(m.terminal.Exclusive.Title); title != "" {
		return title
	}
	if cwd := strings.TrimSpace(m.terminal.Exclusive.Cwd); cwd != "" {
		return cwd
	}
	return strings.TrimSpace(m.terminal.Exclusive.Command)
}

func (m *xdfileModel) renderExclusiveTerminalView() string {
	m.layout.menuButtons = nil
	m.layout.footerButtons = nil
	m.layout.menuItemRects = nil
	m.layout.menuRect = xdfileRect{}

	parts := [3]string{
		m.renderExclusiveTerminalHeader(),
		m.renderExclusiveTerminalHost(),
		m.renderExclusiveTerminalFooter(),
	}
	return xdfileWrapANSIRender(lipgloss.JoinVertical(lipgloss.Left, parts[:]...))
}

func (m *xdfileModel) renderExclusiveTerminalHeader() string {
	statusStyle := xdfileStatusOKStyle
	if m.statusError {
		statusStyle = xdfileStatusErrStyle
	}

	line0 := xdfileJoinLeftRight(
		xdfileTitleStyle.Render(xdfileProductName),
		xdfileDimStyle.Render("embedded TUI"),
		m.width,
	)
	line1 := xdfileJoinLeftRight(
		xdfileTagStyle.Render("EXCLUSIVE")+" "+
			xdfilePathStyle.Render(xdfileCompactPath(m.exclusiveTerminalTitle(), max(12, m.width/2))),
		statusStyle.Render(charmansi.Truncate(m.statusText, max(0, m.width/2), "...")),
		m.width,
	)

	return xdfileWrapANSIRender(lipgloss.JoinVertical(
		lipgloss.Left,
		xdfileHeaderLineStyle.Width(m.width).Render(line0),
		xdfileHeaderLineStyle.Width(m.width).Render(line1),
	))
}

func (m *xdfileModel) renderExclusiveTerminalFooter() string {
	command := strings.TrimSpace(m.terminal.Exclusive.Command)
	line0 := xdfileJoinLeftRight(
		xdfileDimStyle.Render(charmansi.Truncate(command, max(0, m.width/2), "...")),
		xdfileDimStyle.Render("keyboard and mouse are attached to the TUI"),
		m.width,
	)
	line1 := xdfileDimStyle.Render("Use the running program's own quit command to return")

	return xdfileWrapANSIRender(lipgloss.JoinVertical(
		lipgloss.Left,
		xdfileFooterLineStyle.Width(m.width).Render(line0),
		xdfileFooterLineStyle.Width(m.width).Render(xdfilePadRight(line1, m.width)),
	))
}

func (m *xdfileModel) renderTerminalExpandedView() string {
	m.layout.menuButtons = nil
	m.layout.footerButtons = nil
	m.layout.menuItemRects = nil
	m.layout.menuRect = xdfileRect{}

	parts := [3]string{
		m.renderTerminalExpandedHeader(),
		m.renderTerminal(),
		m.renderTerminalExpandedFooter(),
	}
	return xdfileWrapANSIRender(lipgloss.JoinVertical(lipgloss.Left, parts[:]...))
}

func (m *xdfileModel) renderTerminalExpandedHeader() string {
	statusStyle := xdfileStatusOKStyle
	if m.statusError {
		statusStyle = xdfileStatusErrStyle
	}

	line0 := xdfileJoinLeftRight(
		xdfileTitleStyle.Render(xdfileProductName),
		xdfileDimStyle.Render("terminal view"),
		m.width,
	)
	line1 := xdfileJoinLeftRight(
		xdfileTagStyle.Render("TERMINAL")+" "+
			xdfileTagStyle.Render("EXPANDED")+" "+
			xdfilePathStyle.Render(xdfileCompactPath(m.terminalExpandedTitle(), max(12, m.width/2))),
		statusStyle.Render(charmansi.Truncate(m.statusText, max(0, m.width/2), "...")),
		m.width,
	)

	return xdfileWrapANSIRender(lipgloss.JoinVertical(
		lipgloss.Left,
		xdfileHeaderLineStyle.Width(m.width).Render(line0),
		xdfileHeaderLineStyle.Width(m.width).Render(line1),
	))
}

func (m *xdfileModel) renderTerminalExpandedFooter() string {
	mode := "xd shell"
	if m.terminalUsesPTY() {
		mode = "conpty"
	}
	if m.terminal.StreamEmulator != nil {
		mode = "running"
	}
	line0 := xdfileJoinLeftRight(
		xdfileDimStyle.Render(mode),
		xdfileDimStyle.Render("Ctrl+O return to panels | F10 quit"),
		m.width,
	)
	line1 := xdfileDimStyle.Render("Terminal input, mouse, suggestions, scrollback, and command execution keep their normal behavior")

	return xdfileWrapANSIRender(lipgloss.JoinVertical(
		lipgloss.Left,
		xdfileFooterLineStyle.Width(m.width).Render(line0),
		xdfileFooterLineStyle.Width(m.width).Render(xdfilePadRight(line1, m.width)),
	))
}

func (m *xdfileModel) terminalExpandedTitle() string {
	if m == nil {
		return ""
	}
	if title := strings.TrimSpace(m.terminal.Title); title != "" {
		return title
	}
	return strings.TrimSpace(m.terminal.Cwd)
}

func (m *xdfileModel) renderExclusiveTerminalHost() string {
	rect := m.exclusiveTerminalRenderRect()
	innerW := max(10, rect.w-2)
	innerH := max(1, rect.h-2)

	screen := m.renderExclusiveTerminalScreen(innerW, innerH)
	lines := make([]string, 0, innerH)
	if screen != "" {
		lines = append(lines, strings.Split(screen, "\n")...)
	}
	blankLine := xdfileBlank(innerW)
	for len(lines) < innerH {
		lines = append(lines, blankLine)
	}

	return xdfileWrapANSIRender(xdfilePanelBorder(true).
		Width(rect.w - 2).
		Height(rect.h - 2).
		Render(strings.Join(lines[:innerH], "\n")))
}

func (m *xdfileModel) renderExclusiveTerminalScreen(width int, height int) string {
	if !m.exclusiveTerminalActive() || width <= 0 || height <= 0 {
		return ""
	}

	emulator := m.terminal.Exclusive.Session.emulator
	buffer := uv.NewBuffer(width, height)
	totalScrollback := emulator.ScrollbackLen()
	screenHeight := max(1, emulator.Height())
	totalLines := totalScrollback + screenHeight
	start := max(0, totalLines-height)
	for y := 0; y < height; y++ {
		xdfileCopyTerminalEmulatorLineToBuffer(emulator, buffer, y, start+y, width)
	}

	cursor := emulator.CursorPosition()
	if cursor.X >= 0 && cursor.X < width && cursor.Y >= 0 && cursor.Y < height {
		xdfileApplyPTYCursor(buffer, width, height, cursor.X, cursor.Y)
	}

	return xdfileRenderTerminalBuffer(buffer, height)
}

func (m *xdfileModel) handleExclusiveTerminalKey(msg tea.KeyMsg) tea.Cmd {
	if !m.exclusiveTerminalActive() {
		return nil
	}
	m.forwardExclusiveTerminalKey(msg)
	return nil
}

func (m *xdfileModel) forwardExclusiveTerminalKey(msg tea.KeyMsg) {
	if !m.exclusiveTerminalActive() {
		return
	}
	emulator := m.terminal.Exclusive.Session.emulator
	if emulator == nil {
		return
	}
	if msg.Paste && len(msg.Runes) > 0 {
		emulator.Paste(string(msg.Runes))
		return
	}
	if event, ok := xdfileTerminalKeyEvent(msg); ok {
		emulator.SendKey(event)
		return
	}
	if msg.Type == tea.KeyRunes && len(msg.Runes) > 0 {
		emulator.SendText(string(msg.Runes))
	}
}

func (m *xdfileModel) handleExclusiveTerminalMouse(msg tea.MouseMsg) tea.Cmd {
	if !m.exclusiveTerminalActive() {
		return nil
	}

	rect := m.exclusiveTerminalRenderRect()
	x := msg.X - rect.x - 1
	y := msg.Y - rect.y - 1
	width, height := m.exclusiveTerminalViewportSize()
	if x < 0 || y < 0 || x >= width || y >= height {
		return nil
	}

	session := m.terminal.Exclusive.Session
	_ = xdfileSendPTYSessionMouse(session, session.emulator, msg, x, y)
	return nil
}

func (m *xdfileModel) handleTerminalExpandedKey(msg tea.KeyMsg) tea.Cmd {
	if !m.terminalExpandedViewActive() {
		return nil
	}

	switch msg.String() {
	case "ctrl+o":
		return m.toggleTerminalExpandedView()
	case "f10":
		return m.openQuitConfirm()
	}

	if m.terminalUsesPTY() {
		return m.handleTerminalKey(msg)
	}
	if cmd, handled := m.handleManagedTerminalBoundKey(msg); handled {
		return cmd
	}
	return m.handleTerminalKey(msg)
}

func (m *xdfileModel) handleTerminalExpandedMouse(msg tea.MouseMsg) tea.Cmd {
	if !m.terminalExpandedViewActive() {
		return nil
	}
	m.clearMouseHover()

	rect := m.terminalRenderRect()
	if !rect.contains(msg.X, msg.Y) {
		return nil
	}

	if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonRight {
		if m.shouldForwardTerminalMouse() {
			_ = m.sendTerminalMouse(msg)
		}
		return nil
	}

	if msg.Action == tea.MouseActionPress {
		if cmd, handled := m.handleManagedTerminalMousePress(msg); handled {
			return cmd
		}
	}

	if msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown {
		if m.shouldForwardTerminalMouse() && m.sendTerminalMouse(msg) {
			return nil
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

	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		if m.shouldForwardTerminalMouse() {
			_ = m.sendTerminalMouse(msg)
		}
		return nil
	}

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
