package cmd

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	uv "github.com/charmbracelet/ultraviolet"
	charmansi "github.com/charmbracelet/x/ansi"
)

func (m *xdfileModel) handlePTYTerminalKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "shift+pgup":
		m.scrollTerminal(m.terminalScrollPage())
		return nil
	case "shift+pgdown":
		m.scrollTerminal(-m.terminalScrollPage())
		return nil
	case "shift+home":
		m.setTerminalScrollOffset(m.terminalMaxScrollOffset())
		return nil
	case "shift+end":
		m.setTerminalScrollOffset(0)
		return nil
	}

	if !m.terminalUsesPTY() || m.terminal.Emulator == nil {
		return nil
	}

	return m.forwardPTYTerminalKey(msg, true)
}

func (m *xdfileModel) forwardPTYTerminalKey(msg tea.KeyMsg, trackCommand bool) tea.Cmd {
	if !m.terminalUsesPTY() || m.terminal.Emulator == nil {
		return nil
	}

	if msg.Paste && len(msg.Runes) > 0 {
		m.terminal.Emulator.Paste(string(msg.Runes))
		m.setTerminalScrollOffset(0)
		return nil
	}

	if trackCommand && msg.Type == tea.KeyRight && !msg.Alt && m.acceptPTYInlineSuggestion() {
		return nil
	}

	if event, ok := xdfileTerminalKeyEvent(msg); ok {
		var pollCmd tea.Cmd
		if trackCommand && msg.Type == tea.KeyEnter {
			command := m.capturePTYPromptCommand()
			pollCmd = m.beginPTYCommandTracking(command)
		}
		m.terminal.Emulator.SendKey(event)
		m.setTerminalScrollOffset(0)
		return pollCmd
	}

	if msg.Type == tea.KeyRunes && len(msg.Runes) > 0 {
		m.terminal.Emulator.SendText(string(msg.Runes))
		m.setTerminalScrollOffset(0)
	}

	return nil
}

func xdfileTerminalKeyEvent(msg tea.KeyMsg) (uv.KeyEvent, bool) {
	mod := uv.KeyMod(0)
	if msg.Alt {
		mod |= uv.ModAlt
	}

	switch msg.Type {
	case tea.KeyRunes:
		if len(msg.Runes) == 1 {
			return uv.KeyPressEvent{
				Code: msg.Runes[0],
				Text: string(msg.Runes),
				Mod:  mod,
			}, true
		}
		return nil, false
	case tea.KeyEnter:
		return uv.KeyPressEvent{Code: uv.KeyEnter, Mod: mod}, true
	case tea.KeyTab:
		return uv.KeyPressEvent{Code: uv.KeyTab, Mod: mod}, true
	case tea.KeyBackspace:
		return uv.KeyPressEvent{Code: uv.KeyBackspace, Mod: mod}, true
	case tea.KeyEsc:
		return uv.KeyPressEvent{Code: uv.KeyEscape, Mod: mod}, true
	case tea.KeySpace:
		return uv.KeyPressEvent{Code: uv.KeySpace, Mod: mod}, true
	case tea.KeyUp:
		return uv.KeyPressEvent{Code: uv.KeyUp, Mod: mod}, true
	case tea.KeyDown:
		return uv.KeyPressEvent{Code: uv.KeyDown, Mod: mod}, true
	case tea.KeyLeft:
		return uv.KeyPressEvent{Code: uv.KeyLeft, Mod: mod}, true
	case tea.KeyRight:
		return uv.KeyPressEvent{Code: uv.KeyRight, Mod: mod}, true
	case tea.KeyHome:
		return uv.KeyPressEvent{Code: uv.KeyHome, Mod: mod}, true
	case tea.KeyEnd:
		return uv.KeyPressEvent{Code: uv.KeyEnd, Mod: mod}, true
	case tea.KeyPgUp:
		return uv.KeyPressEvent{Code: uv.KeyPgUp, Mod: mod}, true
	case tea.KeyPgDown:
		return uv.KeyPressEvent{Code: uv.KeyPgDown, Mod: mod}, true
	case tea.KeyDelete:
		return uv.KeyPressEvent{Code: uv.KeyDelete, Mod: mod}, true
	case tea.KeyInsert:
		return uv.KeyPressEvent{Code: uv.KeyInsert, Mod: mod}, true
	case tea.KeyShiftTab:
		return uv.KeyPressEvent{Code: uv.KeyTab, Mod: mod | uv.ModShift}, true
	case tea.KeyCtrlUp:
		return uv.KeyPressEvent{Code: uv.KeyUp, Mod: mod | uv.ModCtrl}, true
	case tea.KeyCtrlDown:
		return uv.KeyPressEvent{Code: uv.KeyDown, Mod: mod | uv.ModCtrl}, true
	case tea.KeyCtrlLeft:
		return uv.KeyPressEvent{Code: uv.KeyLeft, Mod: mod | uv.ModCtrl}, true
	case tea.KeyCtrlRight:
		return uv.KeyPressEvent{Code: uv.KeyRight, Mod: mod | uv.ModCtrl}, true
	case tea.KeyCtrlHome:
		return uv.KeyPressEvent{Code: uv.KeyHome, Mod: mod | uv.ModCtrl}, true
	case tea.KeyCtrlEnd:
		return uv.KeyPressEvent{Code: uv.KeyEnd, Mod: mod | uv.ModCtrl}, true
	case tea.KeyCtrlPgUp:
		return uv.KeyPressEvent{Code: uv.KeyPgUp, Mod: mod | uv.ModCtrl}, true
	case tea.KeyCtrlPgDown:
		return uv.KeyPressEvent{Code: uv.KeyPgDown, Mod: mod | uv.ModCtrl}, true
	case tea.KeyShiftUp:
		return uv.KeyPressEvent{Code: uv.KeyUp, Mod: mod | uv.ModShift}, true
	case tea.KeyShiftDown:
		return uv.KeyPressEvent{Code: uv.KeyDown, Mod: mod | uv.ModShift}, true
	case tea.KeyShiftLeft:
		return uv.KeyPressEvent{Code: uv.KeyLeft, Mod: mod | uv.ModShift}, true
	case tea.KeyShiftRight:
		return uv.KeyPressEvent{Code: uv.KeyRight, Mod: mod | uv.ModShift}, true
	case tea.KeyShiftHome:
		return uv.KeyPressEvent{Code: uv.KeyHome, Mod: mod | uv.ModShift}, true
	case tea.KeyShiftEnd:
		return uv.KeyPressEvent{Code: uv.KeyEnd, Mod: mod | uv.ModShift}, true
	case tea.KeyCtrlShiftUp:
		return uv.KeyPressEvent{Code: uv.KeyUp, Mod: mod | uv.ModCtrl | uv.ModShift}, true
	case tea.KeyCtrlShiftDown:
		return uv.KeyPressEvent{Code: uv.KeyDown, Mod: mod | uv.ModCtrl | uv.ModShift}, true
	case tea.KeyCtrlShiftLeft:
		return uv.KeyPressEvent{Code: uv.KeyLeft, Mod: mod | uv.ModCtrl | uv.ModShift}, true
	case tea.KeyCtrlShiftRight:
		return uv.KeyPressEvent{Code: uv.KeyRight, Mod: mod | uv.ModCtrl | uv.ModShift}, true
	case tea.KeyCtrlShiftHome:
		return uv.KeyPressEvent{Code: uv.KeyHome, Mod: mod | uv.ModCtrl | uv.ModShift}, true
	case tea.KeyCtrlShiftEnd:
		return uv.KeyPressEvent{Code: uv.KeyEnd, Mod: mod | uv.ModCtrl | uv.ModShift}, true
	case tea.KeyCtrlShiftC:
		return uv.KeyPressEvent{Code: 'c', Mod: mod | uv.ModCtrl | uv.ModShift}, true
	case tea.KeyCtrlShiftV:
		return uv.KeyPressEvent{Code: 'v', Mod: mod | uv.ModCtrl | uv.ModShift}, true
	case tea.KeyF1:
		return uv.KeyPressEvent{Code: uv.KeyF1, Mod: mod}, true
	case tea.KeyF2:
		return uv.KeyPressEvent{Code: uv.KeyF2, Mod: mod}, true
	case tea.KeyF3:
		return uv.KeyPressEvent{Code: uv.KeyF3, Mod: mod}, true
	case tea.KeyF4:
		return uv.KeyPressEvent{Code: uv.KeyF4, Mod: mod}, true
	case tea.KeyF5:
		return uv.KeyPressEvent{Code: uv.KeyF5, Mod: mod}, true
	case tea.KeyF6:
		return uv.KeyPressEvent{Code: uv.KeyF6, Mod: mod}, true
	case tea.KeyF7:
		return uv.KeyPressEvent{Code: uv.KeyF7, Mod: mod}, true
	case tea.KeyF8:
		return uv.KeyPressEvent{Code: uv.KeyF8, Mod: mod}, true
	case tea.KeyF9:
		return uv.KeyPressEvent{Code: uv.KeyF9, Mod: mod}, true
	case tea.KeyF10:
		return uv.KeyPressEvent{Code: uv.KeyF10, Mod: mod}, true
	case tea.KeyF11:
		return uv.KeyPressEvent{Code: uv.KeyF11, Mod: mod}, true
	case tea.KeyF12:
		return uv.KeyPressEvent{Code: uv.KeyF12, Mod: mod}, true
	}

	base := msg.String()
	if msg.Alt {
		base = strings.TrimPrefix(base, "alt+")
	}
	if strings.HasPrefix(base, "ctrl+") {
		suffix := strings.TrimPrefix(base, "ctrl+")
		ctrlMod := mod | uv.ModCtrl
		switch suffix {
		case "@":
			return uv.KeyPressEvent{Code: uv.KeySpace, Mod: ctrlMod}, true
		case "[":
			return uv.KeyPressEvent{Code: '[', Mod: ctrlMod}, true
		case `\`:
			return uv.KeyPressEvent{Code: '\\', Mod: ctrlMod}, true
		case "]":
			return uv.KeyPressEvent{Code: ']', Mod: ctrlMod}, true
		case "^":
			return uv.KeyPressEvent{Code: '^', Mod: ctrlMod}, true
		case "_":
			return uv.KeyPressEvent{Code: '_', Mod: ctrlMod}, true
		}
		if len([]rune(suffix)) == 1 {
			return uv.KeyPressEvent{Code: []rune(suffix)[0], Mod: ctrlMod}, true
		}
	}

	return nil, false
}

func (m *xdfileModel) sendTerminalMouse(msg tea.MouseMsg) bool {
	if !m.terminalUsesPTY() || m.terminal.Emulator == nil {
		return false
	}
	if !xdfileTerminalMouseShouldForward(msg) {
		return false
	}

	rect := m.terminalRenderRect()
	x := msg.X - rect.x - 1
	y := msg.Y - rect.y - 2
	if x < 0 || y < 0 || x >= m.terminal.ViewWidth || y >= m.terminal.ViewHeight {
		return false
	}

	if !xdfileSendPTYSessionMouse(m.terminal.Session, m.terminal.Emulator, msg, x, y) {
		return false
	}
	if xdfileTerminalMouseResetsScroll(msg) {
		m.setTerminalScrollOffset(0)
	}
	return true
}

func xdfileTerminalMouseShouldForward(msg tea.MouseMsg) bool {
	return msg.Action != tea.MouseActionMotion || msg.Button != tea.MouseButtonNone
}

func xdfileSendPTYSessionMouse(session *xdfileTerminalPTYSession, emulator interface {
	SendText(string)
}, msg tea.MouseMsg, x int, y int) bool {
	nativeOK := false
	if session != nil && session.usesNativeMouseInput() {
		nativeOK = session.SendMouse(msg, x, y)
	}
	vtOK := false
	if session == nil || session.usesVTMouseInput() {
		vtOK = xdfileSendPTYMouse(emulator, msg, x, y)
	}
	return nativeOK || vtOK
}

func (m *xdfileModel) shouldForwardTerminalMouse() bool {
	if !m.terminalUsesPTY() || m.terminal.Emulator == nil {
		return false
	}
	return m.terminal.Emulator.IsAltScreen() || m.terminal.Busy
}

func xdfileSendPTYMouse(emulator interface {
	SendText(string)
}, msg tea.MouseMsg, x int, y int) bool {
	if emulator == nil {
		return false
	}
	if !xdfileTerminalMouseShouldForward(msg) {
		return false
	}

	sequence, ok := xdfileTerminalMouseSGR(msg, x, y)
	if !ok {
		return false
	}
	emulator.SendText(sequence)
	return true
}

func xdfileTerminalMouseSGR(msg tea.MouseMsg, x int, y int) (string, bool) {
	if msg.Action == tea.MouseActionMotion && msg.Button == tea.MouseButtonNone {
		return "", false
	}

	button, ok := xdfileTerminalMouseButton(msg.Button)
	if !ok {
		return "", false
	}

	motion := msg.Action == tea.MouseActionMotion
	release := msg.Action == tea.MouseActionRelease
	encoded := charmansi.EncodeMouseButton(button, motion, msg.Shift, msg.Alt, msg.Ctrl)
	if encoded == 0xff {
		return "", false
	}
	return charmansi.MouseSgr(encoded, x, y, release), true
}

func xdfileTerminalMouseResetsScroll(msg tea.MouseMsg) bool {
	if msg.Button == tea.MouseButtonWheelUp ||
		msg.Button == tea.MouseButtonWheelDown ||
		msg.Button == tea.MouseButtonWheelLeft ||
		msg.Button == tea.MouseButtonWheelRight {
		return true
	}
	return msg.Action == tea.MouseActionPress
}

func xdfileTerminalMouseButton(button tea.MouseButton) (uv.MouseButton, bool) {
	switch button {
	case tea.MouseButtonLeft:
		return uv.MouseLeft, true
	case tea.MouseButtonMiddle:
		return uv.MouseMiddle, true
	case tea.MouseButtonRight:
		return uv.MouseRight, true
	case tea.MouseButtonWheelUp:
		return uv.MouseWheelUp, true
	case tea.MouseButtonWheelDown:
		return uv.MouseWheelDown, true
	case tea.MouseButtonWheelLeft:
		return uv.MouseWheelLeft, true
	case tea.MouseButtonWheelRight:
		return uv.MouseWheelRight, true
	case tea.MouseButtonBackward:
		return uv.MouseBackward, true
	case tea.MouseButtonForward:
		return uv.MouseForward, true
	case tea.MouseButtonNone:
		return uv.MouseNone, true
	default:
		return uv.MouseNone, false
	}
}
