package cmd

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *xdfileModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case xdfileReturnFromUserScreenMsg:
		m.userScreenVisible = false
		_ = m.syncTerminalToPanel(m.activePanel)
		return m, tea.ClearScreen
	case tea.WindowSizeMsg:
		m.width = xdfileWindowRenderWidth(msg.Width)
		m.height = msg.Height
		m.computeLayout()
		if m.exclusiveTerminalActive() {
			m.syncExclusiveTerminalViewport()
			return m, nil
		}
		m.syncTerminalViewport(false)
		m.syncModalViewport()
		m.syncQuickViewViewport()
		return m, nil
	case xdfileFooterCtrlHintExpiredMsg:
		if !msg.At.Before(m.footerCtrlHintUntil) {
			m.footerCtrlHintUntil = time.Time{}
		}
		return m, nil
	case xdfileStatusSpinnerTickMsg:
		if !m.statusSpinnerActive() {
			return m, nil
		}
		m.statusSpinnerIndex = (m.statusSpinnerIndex + 1) % len(xdfileStatusSpinnerFrames)
		return m, xdfileScheduleStatusSpinner()
	case xdfileAutoRefreshMsg:
		m.refreshPanelsIfChanged()
		return m, xdfileScheduleAutoRefresh()
	case tea.MouseMsg:
		if m.userScreenVisible {
			return m, nil
		}
		if m.exclusiveTerminalActive() {
			return m, m.handleExclusiveTerminalMouse(msg)
		}
		if m.modal.Kind != xdfileModalNone {
			return m, m.handleModalMouse(msg)
		}
		if m.terminalExpandedViewActive() {
			return m, m.handleTerminalExpandedMouse(msg)
		}
		return m, m.handleMouse(msg)
	case tea.KeyMsg:
		if m.userScreenVisible {
			switch msg.String() {
			case "ctrl+o":
				return m, m.toggleUserScreen()
			default:
				return m, nil
			}
		}
		if m.exclusiveTerminalActive() {
			return m, m.handleExclusiveTerminalKey(msg)
		}
		if m.modal.Kind != xdfileModalNone {
			return m, m.handleModalKey(msg)
		}
		if m.terminalExpandedViewActive() {
			return m, m.handleTerminalExpandedKey(msg)
		}
		if cmd, handled := m.handlePanelSearchKey(msg); handled {
			return m, cmd
		}
		if cmd, handled := m.handleFilePasteEvent(msg); handled {
			return m, cmd
		}
		if cmd, handled := m.handleMenuKey(msg); handled {
			return m, cmd
		}
		if cmd, handled := m.handleGlobalKey(msg); handled {
			return m, cmd
		}
		if cmd, handled := m.handleManagedTerminalBoundKey(msg); handled {
			return m, cmd
		}
		if m.terminalFocused {
			return m, m.handleTerminalKey(msg)
		}
		return m, m.handlePanelKey(msg)
	case xdfileTerminalLineMsg:
		m.appendTerminalLine(msg.Line, msg.Rewrite, msg.Finalize)
		return m, xdfileWaitTerminalMsg(m.terminal.Events)
	case xdfileTerminalCommandStartMsg:
		m.terminal.Events = msg.Events
		m.terminal.ManagedCancel = msg.Cancel
		m.terminal.StreamEmulator = msg.Emulator
		m.terminal.StreamCanRewrite = false
		m.terminal.CommandHadOutput = false
		m.appendTerminalPrompt(msg.Dir, msg.Command)
		m.syncTerminalViewport(true)
		return m, tea.Batch(xdfileWaitTerminalMsg(m.terminal.Events), xdfileScheduleStatusSpinner())
	case xdfileExclusiveTerminalStartMsg:
		if msg.Err != nil {
			m.appendTerminalPrompt(msg.Dir, msg.Command)
			if !xdfileSuppressTerminalExitLine(msg.Err, false) {
				m.appendTerminalLine(xdfileStatusErrStyle.Render(msg.Err.Error()), false, true)
			}
			m.setStatusErr(msg.Err)
			return m, nil
		}
		m.appendTerminalPrompt(msg.Dir, msg.Command)
		m.terminal.Exclusive = xdfileExclusiveTerminal{
			Command: msg.Command,
			Cwd:     msg.Dir,
			Events:  msg.Session.events,
			Session: msg.Session,
		}
		m.clearMouseHover()
		m.openMenu = ""
		m.setStatus("Exclusive terminal started")
		m.syncExclusiveTerminalViewport()
		return m, tea.Batch(xdfileWaitTerminalMsg(m.terminal.Exclusive.Events), tea.EnableMouseAllMotion)
	case xdfileTerminalStreamScreenMsg:
		return m, xdfileWaitTerminalMsg(m.terminal.Events)
	case xdfileExclusiveTerminalScreenMsg:
		if !m.exclusiveTerminalActive() {
			return m, nil
		}
		return m, xdfileWaitTerminalMsg(m.terminal.Exclusive.Events)
	case xdfileTerminalScreenMsg:
		m.syncPanelFromPTYPrompt()
		if m.terminal.ScrollOffset > 0 {
			m.setTerminalScrollOffset(m.terminal.ScrollOffset)
		}
		return m, xdfileWaitTerminalMsg(m.terminal.Events)
	case xdfileExclusiveTerminalTitleMsg:
		if !m.exclusiveTerminalActive() {
			return m, nil
		}
		m.terminal.Exclusive.Title = msg.Title
		return m, xdfileWaitTerminalMsg(m.terminal.Exclusive.Events)
	case xdfileTerminalStartResultMsg:
		m.terminalStarting = false
		if msg.Err != nil {
			m.terminal.Lines = []string{
				"PTY terminal failed to start.",
				"Falling back to one-shot command execution in the active panel path.",
			}
			m.terminal.Events = nil
			m.terminal.Session = nil
			m.terminal.Emulator = nil
			m.setStatusErr(fmt.Errorf("PTY terminal unavailable: %w", msg.Err))
			return m, nil
		}

		m.terminal.Lines = []string{
			"PTY terminal is ready.",
			"Shift+PgUp/PgDn scroll local terminal history. Ctrl+O expands the terminal view.",
		}
		m.terminal.Events = msg.Session.events
		m.terminal.Session = msg.Session
		m.terminal.Emulator = msg.Session.emulator
		m.terminal.Cwd = m.panels[m.activePanel].Cwd
		m.syncManagedTerminalPrompt()

		waitCmd := xdfileWaitTerminalMsg(m.terminal.Events)
		var replayCmd tea.Cmd
		if draft := m.terminal.Input.Value(); draft != "" {
			m.terminal.Emulator.SendText(draft)
			if m.terminal.StartupSubmitPending {
				command := strings.TrimSpace(draft)
				if command != "" {
					replayCmd = m.beginPTYCommandTracking(command)
				}
				m.terminal.Emulator.SendText("\r")
			}
			m.terminal.Input.SetValue("")
			m.terminal.Input.CursorStart()
		}
		m.terminal.StartupSubmitPending = false
		if !xdfilePathsEqual(msg.Dir, m.panels[m.activePanel].Cwd) {
			m.terminal.PendingCwd = m.panels[m.activePanel].Cwd
			if err := m.requestPTYTerminalCwdSync(m.panels[m.activePanel].Cwd); err != nil {
				m.terminal.PendingCwd = ""
				m.setStatusErr(err)
				if replayCmd != nil {
					return m, tea.Batch(waitCmd, replayCmd)
				}
				return m, waitCmd
			}
		}
		if replayCmd != nil {
			return m, tea.Batch(waitCmd, replayCmd)
		}
		return m, waitCmd
	case xdfileTerminalCwdMsg:
		m.applyTerminalCwd(msg.Cwd)
		return m, xdfileWaitTerminalMsg(m.terminal.Events)
	case xdfileTerminalTitleMsg:
		m.terminal.Title = msg.Title
		return m, xdfileWaitTerminalMsg(m.terminal.Events)
	case xdfileTerminalCommandDoneMsg:
		m.terminal.Busy = false
		m.terminal.ManagedCancel = nil
		m.terminal.PendingPolls = 0
		m.terminal.StreamCanRewrite = false
		m.restorePanelFocusAfterManagedCommand()
		m.terminal.Events = nil
		if streamed := xdfileCollectTerminalEmulatorLines(m.terminal.StreamEmulator); len(streamed) > 0 {
			for _, line := range streamed {
				m.appendTerminalLine(line, false, true)
			}
		}
		m.terminal.StreamEmulator = nil
		if msg.Cwd != "" {
			m.terminal.Cwd = msg.Cwd
			m.syncManagedTerminalPrompt()
			if m.terminal.PendingPanel >= 0 && m.terminal.PendingPanel < len(m.panels) {
				m.panels[m.terminal.PendingPanel].Cwd = msg.Cwd
			}
		}
		m.terminal.PendingPanel = -1
		m.reloadPanelsAfterTerminalCommand(msg.Cwd)
		_ = m.syncTerminalToPanel(m.activePanel)
		if msg.Canceled {
			m.setStatus("Command canceled")
		} else if msg.Err != nil {
			if !xdfileSuppressTerminalExitLine(msg.Err, m.terminal.CommandHadOutput) {
				m.appendTerminalLine(xdfileStatusErrStyle.Render(msg.Err.Error()), false, true)
			}
			m.setStatusErr(msg.Err)
		} else {
			m.setStatus("Command completed")
		}
		m.terminal.CommandHadOutput = false
		m.syncTerminalViewport(true)
		m.terminal.Input.CursorEnd()
		m.refreshManagedTerminalSuggestions()
		return m, nil
	case xdfileTerminalCommandPollMsg:
		if !m.terminal.Busy || !m.terminalUsesPTY() || m.terminal.Emulator == nil {
			return m, nil
		}
		m.reloadAllPanels()
		m.syncPanelFromPTYPrompt()
		width, height := m.terminalViewportSize()
		if _, _, _, ok := m.currentPTYPromptStateForCompletion(width, height); ok {
			m.terminal.Busy = false
			m.terminal.PendingPolls = 0
			m.setStatus("Command completed")
			return m, nil
		}
		if m.terminal.PendingPolls > 0 {
			m.terminal.PendingPolls--
			if m.terminal.PendingPolls == 0 {
				m.terminal.Busy = false
				return m, nil
			}
		}
		return m, xdfileWaitPTYCommandPoll()
	case xdfileTerminalExitMsg:
		m.terminal.Busy = false
		m.terminal.ManagedCancel = nil
		m.terminal.PendingPolls = 0
		m.terminal.StreamCanRewrite = false
		m.terminal.Session = nil
		m.terminal.Events = nil
		m.terminal.Emulator = nil
		m.terminal.ScrollOffset = 0
		if msg.Err != nil {
			m.appendTerminalLine(xdfileStatusErrStyle.Render(msg.Err.Error()), false, true)
			m.setStatusErr(msg.Err)
		} else {
			m.setStatus("PTY terminal closed")
		}
		return m, nil
	case xdfileExclusiveTerminalExitMsg:
		if !m.exclusiveTerminalActive() {
			return m, nil
		}
		m.finishExclusiveTerminal(msg.Err)
		return m, tea.EnableMouseAllMotion
	case xdfileTerminalResultMsg:
		m.terminal.Busy = false
		m.terminal.ManagedCancel = nil
		m.terminal.PendingPolls = 0
		m.terminal.StreamCanRewrite = false
		m.restorePanelFocusAfterManagedCommand()
		if msg.Clear {
			m.terminal.Lines = nil
		}
		m.appendTerminalOutput(msg)
		m.terminal.CommandHadOutput = false
		if msg.Dir != "" {
			m.terminal.Cwd = msg.Dir
			m.syncManagedTerminalPrompt()
		}
		if msg.SyncActivePanel && msg.Dir != "" {
			m.panels[m.activePanel].Cwd = msg.Dir
			if err := m.reloadPanel(m.activePanel); err != nil {
				m.setStatusErr(err)
			}
		}
		if !msg.SyncActivePanel {
			m.reloadPanelsAfterTerminalCommand(msg.Dir)
		}
		m.refreshManagedTerminalSuggestions()
		return m, nil
	case xdfileTerminalConsoleDoneMsg:
		m.terminal.Busy = false
		m.terminal.ManagedCancel = nil
		m.terminal.PendingPolls = 0
		m.terminal.StreamCanRewrite = false
		m.restorePanelFocusAfterManagedCommand()
		m.reloadAllPanels()
		_ = m.syncTerminalToPanel(m.activePanel)
		m.appendTerminalPrompt(m.terminal.Cwd, msg.Command)
		if msg.Err != nil {
			m.setStatusErr(msg.Err)
		} else {
			m.setStatus("Command completed")
		}
		m.syncTerminalViewport(true)
		m.refreshManagedTerminalSuggestions()
		return m, nil
	case xdfileClipboardWriteResultMsg:
		if msg.Err != nil {
			m.setStatusErr(fmt.Errorf("copied internally but system clipboard failed: %w", msg.Err))
		}
		return m, nil
	case xdfileRemoteClipboardCopyResultMsg:
		m.stopBackgroundTask()
		if msg.Err != nil {
			if msg.CacheDir != "" {
				_ = xdfileRemoveAllFunc(msg.CacheDir)
			}
			m.setStatusErr(msg.Err)
			return m, nil
		}
		if len(msg.Paths) == 0 {
			if msg.CacheDir != "" {
				_ = xdfileRemoveAllFunc(msg.CacheDir)
			}
			m.setStatus("Remote clipboard copy produced no files")
			return m, nil
		}
		m.cleanupRemoteClipboardDirs()
		m.registerRemoteClipboardDir(msg.CacheDir)
		m.clipboardPaths = append([]string(nil), msg.Paths...)
		m.clipboardPath = msg.Paths[0]
		m.clipboardCut = false
		if msg.ClipboardErr != nil {
			m.setStatusErr(fmt.Errorf("remote copy is ready internally but system clipboard failed: %w", msg.ClipboardErr))
			return m, nil
		}
		if len(msg.Paths) == 1 && len(msg.Names) == 1 {
			m.setStatus("Copied remote %s to clipboard", msg.Names[0])
		} else {
			m.setStatus("Copied %d remote items to clipboard", len(msg.Paths))
		}
		return m, nil
	case xdfileRemoteClipboardPasteDoneMsg:
		return m, m.applyRemoteClipboardPasteDone(msg)
	case xdfileLocalClipboardPasteDoneMsg:
		return m, m.applyLocalClipboardPasteDone(msg)
	default:
		if m.modal.Kind == xdfileModalInput {
			var cmd tea.Cmd
			m.modal.Input, cmd = m.modal.Input.Update(msg)
			return m, cmd
		}
		if m.terminalFocused && !m.terminalUsesPTY() {
			var cmd tea.Cmd
			m.terminal.Input, cmd = m.terminal.Input.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}
