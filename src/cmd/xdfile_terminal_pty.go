package cmd

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	uv "github.com/charmbracelet/ultraviolet"
	charmansi "github.com/charmbracelet/x/ansi"
	vt "github.com/charmbracelet/x/vt"
)

var (
	xdfileTerminalSuggestionUVStyle       uv.Style
	xdfileTerminalSuggestionCursorUVStyle uv.Style
	xdfileTerminalCursorUVStyle           uv.Style
)

type xdfileTerminalPTYBackend interface {
	io.ReadWriteCloser
	Resize(width int, height int) error
}

type xdfileTerminalShellKind string

const (
	xdfileTerminalShellUnknown    xdfileTerminalShellKind = ""
	xdfileTerminalShellPowerShell xdfileTerminalShellKind = "powershell"
	xdfileTerminalShellCmd        xdfileTerminalShellKind = "cmd"
)

type xdfileTerminalPTYMode int

const (
	xdfileTerminalPTYModeShell xdfileTerminalPTYMode = iota
	xdfileTerminalPTYModeExclusive
)

type xdfilePTYMouseInputMode int

const (
	xdfilePTYMouseInputVT xdfilePTYMouseInputMode = iota
	xdfilePTYMouseInputNative
	xdfilePTYMouseInputBoth
)

type xdfileTerminalPTYSession struct {
	backend  xdfileTerminalPTYBackend
	process  *os.Process
	emulator *vt.SafeEmulator
	events   chan tea.Msg
	mouse    *xdfilePTYMouseProxy
	mouseIn  xdfilePTYMouseInputMode
	syncFile string
	shell    xdfileTerminalShellKind
	mode     xdfileTerminalPTYMode

	closeOnce sync.Once
	mu        sync.Mutex
	closing   bool
}

type xdfileTerminalScreenMsg struct{}

type xdfileTerminalCwdMsg struct {
	Cwd string
}

type xdfileTerminalTitleMsg struct {
	Title string
}

type xdfilePTYPromptState struct {
	CursorX int
	CursorY int
	Input   string
	Cwd     string
	Ok      bool
}

const (
	xdfilePTYCommandPollInterval  = 250 * time.Millisecond
	xdfilePTYCommandPollMaxTicks  = 1200
	xdfileTerminalEventBufferSize = 256
	xdfileTerminalScrollbackLimit = 5000
)

func xdfileTerminalSyncKeyEvent() uv.KeyEvent {
	return uv.KeyPressEvent{Code: '_', Mod: uv.ModCtrl}
}

func xdfileWaitPTYCommandPoll() tea.Cmd {
	return tea.Tick(xdfilePTYCommandPollInterval, func(time.Time) tea.Msg {
		return xdfileTerminalCommandPollMsg{}
	})
}

func xdfileStartExclusiveCommandPTYSession(
	dir string,
	path string,
	args []string,
	events chan tea.Msg,
	width int,
	height int,
	mouseInput xdfilePTYMouseInputMode,
) (*xdfileTerminalPTYSession, error) {
	backend, process, err := xdfileStartCommandPTYBackend(dir, path, args, width, height)
	if err != nil {
		return nil, err
	}

	return xdfileNewTerminalPTYSession(backend, process, xdfileTerminalShellUnknown, events, width, height, xdfileTerminalPTYModeExclusive, mouseInput), nil
}

func xdfileNewTerminalPTYSession(
	backend xdfileTerminalPTYBackend,
	process *os.Process,
	shellKind xdfileTerminalShellKind,
	events chan tea.Msg,
	width int,
	height int,
	mode xdfileTerminalPTYMode,
	mouseInput xdfilePTYMouseInputMode,
) *xdfileTerminalPTYSession {
	session := &xdfileTerminalPTYSession{
		backend:  backend,
		process:  process,
		emulator: vt.NewSafeEmulator(max(1, width), max(1, height)),
		events:   events,
		mouseIn:  mouseInput,
		shell:    shellKind,
		mode:     mode,
	}
	session.emulator.SetScrollbackSize(xdfileTerminalScrollbackLimit)
	session.emulator.SetCallbacks(vt.Callbacks{
		WorkingDirectory: func(cwd string) {
			session.sendWorkingDirectory(xdfileNormalizeTerminalWorkingDirectory(cwd))
		},
		Title: func(title string) {
			session.sendTitle(title)
		},
	})

	go session.runOutputLoop()
	go session.runInputLoop()
	go session.runWaitLoop()

	return session
}

func (s *xdfileTerminalPTYSession) Resize(width int, height int) error {
	if s == nil {
		return nil
	}

	width = max(1, width)
	height = max(1, height)
	s.emulator.Resize(width, height)
	if s.backend == nil {
		return nil
	}
	return s.backend.Resize(width, height)
}

func (s *xdfileTerminalPTYSession) Close() {
	s.shutdown(true)
}

func (s *xdfileTerminalPTYSession) shutdown(killProcess bool) {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.closing = true
		process := s.process
		backend := s.backend
		emulator := s.emulator
		mouse := s.mouse
		syncFile := s.syncFile
		s.mu.Unlock()

		if mouse != nil {
			mouse.Close()
		}
		if killProcess && process != nil {
			_ = process.Kill()
		}
		if backend != nil {
			_ = backend.Close()
		}
		if emulator != nil {
			_ = emulator.Close()
		}
		if syncFile != "" {
			_ = os.Remove(syncFile)
		}
	})
}

func (s *xdfileTerminalPTYSession) SendMouse(msg tea.MouseMsg, x int, y int) bool {
	if s == nil || !s.usesNativeMouseInput() || !xdfileTerminalMouseShouldForward(msg) {
		return false
	}
	mouse := s.nativeMouseProxy()
	if mouse == nil {
		return false
	}
	return mouse.Send(msg, x, y)
}

func (s *xdfileTerminalPTYSession) nativeMouseProxy() *xdfilePTYMouseProxy {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closing {
		return nil
	}
	if s.mouse == nil {
		s.mouse = xdfileStartPTYMouseProxy(s.process)
	}
	return s.mouse
}

func (s *xdfileTerminalPTYSession) usesNativeMouseInput() bool {
	return s != nil && (s.mouseIn == xdfilePTYMouseInputNative || s.mouseIn == xdfilePTYMouseInputBoth)
}

func (s *xdfileTerminalPTYSession) usesVTMouseInput() bool {
	return s == nil || s.mouseIn == xdfilePTYMouseInputVT || s.mouseIn == xdfilePTYMouseInputBoth
}

func (s *xdfileTerminalPTYSession) runInputLoop() {
	if s == nil || s.backend == nil || s.emulator == nil {
		return
	}

	if _, err := io.Copy(s.backend, s.emulator); err != nil && !s.isClosing() {
		s.sendExit(fmt.Errorf("write PTY input: %w", err))
	}
}

func (s *xdfileTerminalPTYSession) runOutputLoop() {
	if s == nil || s.backend == nil || s.emulator == nil {
		return
	}

	buf := make([]byte, 32*1024)
	for {
		n, err := s.backend.Read(buf)
		if n > 0 {
			if _, writeErr := s.emulator.Write(buf[:n]); writeErr != nil {
				if !s.isClosing() {
					s.sendExit(fmt.Errorf("render PTY output: %w", writeErr))
				}
				return
			}
			s.sendScreen()
		}
		if err != nil {
			if err != io.EOF && !s.isClosing() {
				s.sendExit(fmt.Errorf("read PTY output: %w", err))
			}
			return
		}
	}
}

func (s *xdfileTerminalPTYSession) runWaitLoop() {
	if s == nil || s.process == nil {
		return
	}

	_, err := s.process.Wait()
	if err != nil {
		if !s.isClosing() {
			s.sendExit(fmt.Errorf("PTY process exited: %w", err))
		}
		s.shutdown(false)
		return
	}

	if !s.isClosing() {
		s.sendExit(nil)
	}
	s.shutdown(false)
}

func (s *xdfileTerminalPTYSession) send(msg tea.Msg) {
	if s == nil || s.events == nil || msg == nil {
		return
	}
	s.events <- msg
}

func (s *xdfileTerminalPTYSession) isClosing() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closing
}

func (s *xdfileTerminalPTYSession) sendScreen() {
	if s == nil {
		return
	}
	if s.mode == xdfileTerminalPTYModeExclusive {
		s.send(xdfileExclusiveTerminalScreenMsg{})
		return
	}
	s.send(xdfileTerminalScreenMsg{})
}

func (s *xdfileTerminalPTYSession) sendExit(err error) {
	if s == nil {
		return
	}
	if s.mode == xdfileTerminalPTYModeExclusive {
		s.send(xdfileExclusiveTerminalExitMsg{Err: err})
		return
	}
	s.send(xdfileTerminalExitMsg{Err: err})
}

func (s *xdfileTerminalPTYSession) sendTitle(title string) {
	if s == nil {
		return
	}
	if s.mode == xdfileTerminalPTYModeExclusive {
		s.send(xdfileExclusiveTerminalTitleMsg{Title: title})
		return
	}
	s.send(xdfileTerminalTitleMsg{Title: title})
}

func (s *xdfileTerminalPTYSession) sendWorkingDirectory(cwd string) {
	if s == nil || s.mode == xdfileTerminalPTYModeExclusive {
		return
	}
	s.send(xdfileTerminalCwdMsg{Cwd: cwd})
}

func xdfileNormalizeTerminalWorkingDirectory(cwd string) string {
	cwd = strings.TrimSpace(cwd)
	if cwd == "" || !strings.HasPrefix(strings.ToLower(cwd), "file://") {
		return cwd
	}

	u, err := url.Parse(cwd)
	if err != nil || u.Scheme != "file" {
		return cwd
	}

	path := u.Path
	if path == "" {
		return cwd
	}

	if runtime.GOOS == "windows" {
		if u.Host != "" && !strings.EqualFold(u.Host, "localhost") {
			unc := `\\` + u.Host + filepath.FromSlash(path)
			return filepath.Clean(unc)
		}

		if len(path) >= 3 && path[0] == '/' && path[2] == ':' {
			path = path[1:]
		}
		return filepath.Clean(filepath.FromSlash(path))
	}

	if u.Host != "" && !strings.EqualFold(u.Host, "localhost") {
		return filepath.Clean("//" + u.Host + path)
	}
	return filepath.Clean(path)
}

func (m *xdfileModel) terminalUsesPTY() bool {
	return m.terminal.Session != nil && m.terminal.Emulator != nil
}

func (m *xdfileModel) terminalViewportSize() (int, int) {
	rect := m.terminalRenderRect()
	if rect.w == 0 || rect.h == 0 {
		return 80, 24
	}
	return max(10, rect.w-2), max(1, rect.h-3)
}

func (m *xdfileModel) streamingCommandTerminalSize() (int, int) {
	rect := m.terminalRenderRect()
	if rect.w == 0 || rect.h == 0 {
		return 80, 24
	}
	innerWidth := max(10, rect.w-2)
	innerHeight := max(4, rect.h-2)
	return innerWidth, max(1, innerHeight-2)
}

func (m *xdfileModel) terminalMaxScrollOffset() int {
	if !m.terminalUsesPTY() || m.terminal.Emulator == nil || m.terminal.Emulator.IsAltScreen() {
		return 0
	}
	return max(0, m.terminal.Emulator.ScrollbackLen())
}

func (m *xdfileModel) setTerminalScrollOffset(offset int) {
	m.terminal.ScrollOffset = max(0, min(offset, m.terminalMaxScrollOffset()))
}

func (m *xdfileModel) scrollTerminal(delta int) {
	m.setTerminalScrollOffset(m.terminal.ScrollOffset + delta)
}

func (m *xdfileModel) requestPTYTerminalCwdSync(target string) error {
	if !m.terminalUsesPTY() || m.terminal.Session == nil || m.terminal.Emulator == nil {
		return nil
	}
	if m.terminal.Emulator.IsAltScreen() {
		return nil
	}
	if m.terminal.Session.shell == xdfileTerminalShellCmd {
		m.terminal.Emulator.SendText(xdfileCmdCDCommand(target) + "\r")
		m.setTerminalScrollOffset(0)
		return nil
	}

	if m.terminal.Session.syncFile == "" {
		return fmt.Errorf("PTY cwd sync file is unavailable")
	}
	if err := os.WriteFile(m.terminal.Session.syncFile, []byte(target), 0o600); err != nil {
		return fmt.Errorf("write PTY cwd sync file: %w", err)
	}
	m.terminal.Emulator.SendKey(xdfileTerminalSyncKeyEvent())
	m.setTerminalScrollOffset(0)
	return nil
}

func xdfileCmdCDCommand(target string) string {
	return `cd /d "` + strings.ReplaceAll(target, `"`, `""`) + `"`
}

func (m *xdfileModel) terminalScrollPage() int {
	return max(1, m.terminal.ViewHeight-1)
}

func (m *xdfileModel) capturePTYPromptCommand() string {
	width, height := m.terminalViewportSize()
	_, _, input, ok := m.currentPTYPromptState(width, height)
	if !ok {
		return ""
	}
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}
	m.pushTerminalHistory(input)
	return input
}

func (m *xdfileModel) beginPTYCommandTracking(command string) tea.Cmd {
	command = strings.TrimSpace(command)
	if command == "" || !m.terminalUsesPTY() || m.terminal.Emulator == nil {
		return nil
	}
	wasActive := m.statusSpinnerActive()
	m.terminal.Busy = true
	m.terminal.PendingPolls = xdfilePTYCommandPollMaxTicks
	m.setTerminalScrollOffset(0)
	if wasActive {
		return xdfileWaitPTYCommandPoll()
	}
	return tea.Batch(xdfileWaitPTYCommandPoll(), xdfileScheduleStatusSpinner())
}

func (m *xdfileModel) currentPTYPromptState(width int, height int) (int, int, string, bool) {
	state := m.currentPTYPromptStateEx(width, height, true)
	return state.CursorX, state.CursorY, state.Input, state.Ok
}

func (m *xdfileModel) currentPTYPromptStateForCompletion(width int, height int) (int, int, string, bool) {
	state := m.currentPTYPromptStateEx(width, height, false)
	return state.CursorX, state.CursorY, state.Input, state.Ok
}

func (m *xdfileModel) currentPTYPromptStateEx(width int, height int, requireBottom bool) xdfilePTYPromptState {
	if !m.terminalUsesPTY() || m.terminal.Emulator == nil || m.terminal.Emulator.IsAltScreen() || (requireBottom && m.terminal.ScrollOffset > 0) {
		return xdfilePTYPromptState{}
	}

	cursor := m.terminal.Emulator.CursorPosition()
	if cursor.X < 0 || cursor.X > width || cursor.Y < 0 || cursor.Y >= height {
		return xdfilePTYPromptState{}
	}

	rawBeforeCursor := xdfilePTYLineString(m.terminal.Emulator, cursor.Y, 0, cursor.X)
	afterCursor := strings.TrimSpace(xdfilePTYLineString(m.terminal.Emulator, cursor.Y, cursor.X, width))
	if afterCursor != "" {
		return xdfilePTYPromptState{}
	}

	cwd, input, ok := xdfileParsePTYPromptLine(rawBeforeCursor, m.currentPTYTerminalShell())
	if !ok {
		return xdfilePTYPromptState{}
	}
	return xdfilePTYPromptState{
		CursorX: cursor.X,
		CursorY: cursor.Y,
		Input:   input,
		Cwd:     cwd,
		Ok:      true,
	}
}

func (m *xdfileModel) currentPTYTerminalShell() xdfileTerminalShellKind {
	if m == nil || m.terminal.Session == nil {
		return xdfileTerminalShellUnknown
	}
	return m.terminal.Session.shell
}

func xdfileParsePTYPromptLine(raw string, shell xdfileTerminalShellKind) (string, string, bool) {
	line := strings.TrimLeft(raw, " ")
	switch shell {
	case xdfileTerminalShellCmd:
		return xdfileParseCMDPromptLine(line)
	case xdfileTerminalShellPowerShell:
		return xdfileParsePowerShellPromptLine(line)
	}

	if cwd, input, ok := xdfileParsePowerShellPromptLine(line); ok {
		return cwd, input, true
	}
	return xdfileParseCMDPromptLine(line)
}

func xdfileParsePowerShellPromptLine(line string) (string, string, bool) {
	if !strings.HasPrefix(line, "PS ") {
		return "", "", false
	}

	rest := line[len("PS "):]
	promptIndex := strings.IndexRune(rest, '>')
	if promptIndex < 0 {
		return "", "", false
	}

	cwd := strings.TrimSpace(rest[:promptIndex])
	if cwd == "" {
		return "", "", false
	}

	input := strings.TrimLeft(rest[promptIndex+1:], " ")
	return cwd, input, true
}

func xdfileParseCMDPromptLine(line string) (string, string, bool) {
	promptIndex := strings.IndexRune(line, '>')
	if promptIndex < 0 {
		return "", "", false
	}

	cwd := strings.TrimSpace(line[:promptIndex])
	if !xdfileLooksLikeWindowsPromptPath(cwd) {
		return "", "", false
	}

	input := strings.TrimLeft(line[promptIndex+1:], " ")
	return cwd, input, true
}

func xdfileLooksLikeWindowsPromptPath(path string) bool {
	if len(path) >= 2 && path[1] == ':' {
		first := path[0]
		if (first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z') {
			return true
		}
	}
	return strings.HasPrefix(path, `\\`)
}

func (m *xdfileModel) syncPanelFromPTYPrompt() {
	width, height := m.terminalViewportSize()
	state := m.currentPTYPromptStateEx(width, height, false)
	if !state.Ok || state.Cwd == "" {
		return
	}
	m.applyTerminalCwd(state.Cwd)
}

func (m *xdfileModel) applyTerminalCwd(cwd string) {
	if cwd == "" {
		return
	}

	previous := m.terminal.Cwd
	m.terminal.Cwd = cwd
	m.syncManagedTerminalPrompt()
	if m.currentPTYTerminalShell() == xdfileTerminalShellCmd && (m.terminal.Title == "" || xdfilePathsEqual(m.terminal.Title, previous)) {
		m.terminal.Title = cwd
	}
	if m.terminal.PendingCwd != "" {
		if xdfilePathsEqual(m.terminal.PendingCwd, cwd) {
			m.terminal.PendingCwd = ""
		}
		return
	}

	if !xdfilePathsEqual(m.panels[m.activePanel].Cwd, cwd) {
		m.panels[m.activePanel].Cwd = cwd
		if err := m.reloadPanel(m.activePanel); err != nil {
			m.setStatusErr(err)
		}
	}
}

func xdfilePTYLineString(emulator *vt.SafeEmulator, y int, startX int, endX int) string {
	if emulator == nil || startX >= endX {
		return ""
	}

	var builder strings.Builder
	for x := startX; x < endX; x++ {
		cell := emulator.CellAt(x, y)
		if cell == nil || cell.Content == "" {
			builder.WriteByte(' ')
			continue
		}
		builder.WriteString(cell.Content)
	}
	return builder.String()
}

func (m *xdfileModel) ptyInlineSuggestion(width int, height int) (int, int, string, bool) {
	cursorX, cursorY, input, ok := m.currentPTYPromptState(width, height)
	if !ok || cursorX >= width {
		return 0, 0, "", false
	}

	suggestion := m.bestTerminalSuggestion(input)
	if suggestion == "" {
		return 0, 0, "", false
	}

	suffix := xdfileSuggestionSuffix(suggestion, input)
	if suffix == "" {
		return 0, 0, "", false
	}
	return cursorX, cursorY, suffix, true
}

func (m *xdfileModel) acceptPTYInlineSuggestion() bool {
	if !m.terminalUsesPTY() || m.terminal.Emulator == nil {
		return false
	}

	width, height := m.terminalViewportSize()
	_, _, suffix, ok := m.ptyInlineSuggestion(width, height)
	if !ok || suffix == "" {
		return false
	}

	m.terminal.Emulator.SendText(suffix)
	m.setTerminalScrollOffset(0)
	return true
}

func xdfileSuggestionSuffix(candidate string, prefix string) string {
	candidateRunes := []rune(candidate)
	prefixRunes := []rune(prefix)
	if len(prefixRunes) >= len(candidateRunes) {
		return ""
	}
	if !strings.HasPrefix(strings.ToLower(candidate), strings.ToLower(prefix)) {
		return ""
	}
	return string(candidateRunes[len(prefixRunes):])
}

func xdfileApplyPTYInlineSuggestion(buffer *uv.Buffer, width int, height int, cursorX int, cursorY int, suffix string) {
	if buffer == nil || suffix == "" || cursorY < 0 || cursorY >= height || cursorX < 0 || cursorX >= width {
		return
	}
	for i, r := range []rune(suffix) {
		x := cursorX + i
		if x >= width {
			break
		}
		style := xdfileTerminalSuggestionUVStyle
		if i == 0 {
			style = xdfileTerminalSuggestionCursorUVStyle
		}
		buffer.SetCell(x, cursorY, &uv.Cell{
			Content: string(r),
			Width:   1,
			Style:   style,
		})
	}
}

func xdfileApplyPTYCursor(buffer *uv.Buffer, width int, height int, cursorX int, cursorY int) {
	if buffer == nil || cursorX < 0 || cursorX >= width || cursorY < 0 || cursorY >= height {
		return
	}

	cell := buffer.CellAt(cursorX, cursorY)
	if cell == nil {
		cell = uv.EmptyCell.Clone()
	} else {
		cell = cell.Clone()
	}
	if cell.Content == "" {
		cell.Content = " "
		cell.Width = 1
	}
	cell.Style = xdfileTerminalCursorUVStyle
	buffer.SetCell(cursorX, cursorY, cell)
}

func xdfileCopyTerminalEmulatorLineToBuffer(emulator *vt.SafeEmulator, buffer *uv.Buffer, targetY int, lineIndex int, width int) {
	if emulator == nil || buffer == nil || targetY < 0 || width <= 0 {
		return
	}
	for x := 0; x < width; {
		cell := xdfileTerminalEmulatorCellAt(emulator, x, lineIndex)
		if cell != nil && !cell.IsZero() {
			buffer.SetCell(x, targetY, cell.Clone())
			x += max(cell.Width, 1)
			continue
		}
		x++
	}
}

func xdfileRenderTerminalBuffer(buffer *uv.Buffer, height int) string {
	if buffer == nil || height <= 0 {
		return ""
	}
	lines := make([]string, 0, height)
	for y := 0; y < height; y++ {
		lines = append(lines, buffer.Line(y).Render())
	}
	return strings.Join(lines, "\n")
}

func xdfileRenderStreamingTerminalScreen(emulator *vt.SafeEmulator, width int, height int) string {
	if emulator == nil || width <= 0 || height <= 0 {
		return ""
	}
	return strings.Join(xdfileRenderTerminalEmulatorLines(emulator, width, 0, height), "\n")
}

func xdfileCollectTerminalEmulatorLines(emulator *vt.SafeEmulator) []string {
	if emulator == nil {
		return nil
	}
	lines := xdfileRenderTerminalEmulatorLines(
		emulator,
		max(1, emulator.Width()),
		0,
		emulator.ScrollbackLen()+max(1, emulator.Height()),
	)
	last := len(lines)
	for last > 0 && strings.TrimSpace(charmansi.Strip(lines[last-1])) == "" {
		last--
	}
	return lines[:last]
}

func xdfileRenderTerminalEmulatorLines(emulator *vt.SafeEmulator, width int, scrollOffset int, height int) []string {
	if emulator == nil || width <= 0 || height <= 0 {
		return nil
	}

	width = max(1, width)
	height = max(1, height)
	screenHeight := max(1, emulator.Height())
	totalScrollback := emulator.ScrollbackLen()
	totalLines := totalScrollback + screenHeight
	start := max(0, totalLines-height-scrollOffset)
	if start > totalLines {
		start = totalLines
	}
	end := min(totalLines, start+height)

	lines := make([]string, 0, max(height, end-start))
	for lineIndex := start; lineIndex < end; lineIndex++ {
		lines = append(lines, xdfileRenderTerminalEmulatorLine(emulator, lineIndex, width))
	}
	blankLine := xdfileBlank(width)
	for len(lines) < height {
		lines = append(lines, blankLine)
	}
	return lines
}

func xdfileRenderTerminalEmulatorLine(emulator *vt.SafeEmulator, lineIndex int, width int) string {
	width = max(1, width)
	lastUsedX := 0
	for x := 0; x < width; {
		cell := xdfileTerminalEmulatorCellAt(emulator, x, lineIndex)
		if cell != nil && !cell.IsZero() {
			lastUsedX = max(lastUsedX, x+max(cell.Width, 1))
			x += max(cell.Width, 1)
			continue
		}
		x++
	}
	if lastUsedX == 0 {
		return ""
	}

	buffer := uv.NewBuffer(lastUsedX, 1)
	xdfileCopyTerminalEmulatorLineToBuffer(emulator, buffer, 0, lineIndex, lastUsedX)
	return buffer.Line(0).Render()
}

func xdfileTerminalEmulatorCellAt(emulator *vt.SafeEmulator, x int, lineIndex int) *uv.Cell {
	if emulator == nil {
		return nil
	}
	scrollbackLen := emulator.ScrollbackLen()
	if lineIndex < scrollbackLen {
		return emulator.ScrollbackCellAt(x, lineIndex)
	}
	return emulator.CellAt(x, lineIndex-scrollbackLen)
}

func (m *xdfileModel) renderPTYTerminalScreen(width int, height int) string {
	if !m.terminalUsesPTY() || m.terminal.Emulator == nil {
		return ""
	}

	width = max(1, width)
	height = max(1, height)

	buffer := uv.NewBuffer(width, height)
	totalScrollback := m.terminal.Emulator.ScrollbackLen()
	screenHeight := max(1, m.terminal.Emulator.Height())
	offset := m.terminal.ScrollOffset
	if m.terminal.Emulator.IsAltScreen() {
		offset = 0
	}
	totalLines := totalScrollback + screenHeight
	start := max(0, totalLines-height-offset)

	for y := 0; y < height; y++ {
		xdfileCopyTerminalEmulatorLineToBuffer(m.terminal.Emulator, buffer, y, start+y, width)
	}

	suggestionX, suggestionY, suggestionSuffix, hasSuggestion := 0, 0, "", false
	if m.terminalFocused && offset == 0 {
		suggestionX, suggestionY, suggestionSuffix, hasSuggestion = m.ptyInlineSuggestion(width, height)
	}

	if m.terminalFocused && offset == 0 && !hasSuggestion {
		cursor := m.terminal.Emulator.CursorPosition()
		if cursor.X >= 0 && cursor.X < width && cursor.Y >= 0 && cursor.Y < height {
			xdfileApplyPTYCursor(buffer, width, height, cursor.X, cursor.Y)
		}
	}

	if hasSuggestion {
		xdfileApplyPTYInlineSuggestion(buffer, width, height, suggestionX, suggestionY, suggestionSuffix)
	}
	return xdfileRenderTerminalBuffer(buffer, height)
}
