package cmd

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	filepreview "github.com/s0x401/xdfile-manager/src/pkg/file_preview"
)

func runXdfileApp(paths []string) error {
	m := newXdfileModel(paths)
	defer m.closeXdfileResources()

	p := tea.NewProgram(
		m,
		tea.WithAltScreen(),
		tea.WithMouseAllMotion(),
	)
	_, err := p.Run()
	return err
}

func newXdfileModel(paths []string) *xdfileModel {
	deleteUndoCleanupCount, deleteUndoCleanupErr := xdfileCleanupDeleteUndoState(xdfileDeleteUndoStatePath())
	layoutFile := xdfileLayoutPrefsPath()
	layoutPrefs, layoutErr := xdfileLoadLayoutPrefs(layoutFile)
	commandsFile := xdfileCommandsPrefsPath()
	commands, commandsErr := xdfileLoadOrMigrateCommandPrefs(layoutFile, commandsFile, &layoutPrefs)
	layoutPrefs.Commands = append([]xdfileCommandItem(nil), commands...)
	netboxFile := xdfileNetBoxPrefsPath()
	netboxConnections, netboxErr := xdfileLoadNetBoxPrefs(netboxFile)
	xdfileSetNetBoxConnectionsCache(netboxConnections)
	layoutPrefs.ThemeName = xdfileNormalizeThemeName(layoutPrefs.ThemeName)
	xdfileApplyTheme(xdfileThemeByName(layoutPrefs.ThemeName))
	leftPath, rightPath := xdfileResolveStartPaths(paths, layoutPrefs)
	terminalHistoryItems, terminalHistoryDeleted, terminalHistoryErr := xdfileLoadTerminalHistoryState(xdfileTerminalHistoryPath())
	terminalHistoryItems = xdfileMergeTerminalHistorySeed(terminalHistoryItems, xdfileLoadTerminalHistorySeed(), terminalHistoryDeleted)
	terminalHistory := xdfileTerminalHistoryCommands(terminalHistoryItems, terminalHistoryDeleted)

	termInput := xdfileNewManagedTerminalInput()

	modalInput := xdfileNewModalInput()

	vp := viewport.New(10, 5)
	vp.MouseWheelEnabled = false
	quickViewVP := viewport.New(10, 5)
	quickViewVP.MouseWheelEnabled = false

	var thumbnailGenerator *filepreview.ThumbnailGenerator
	if generator, generatorErr := filepreview.NewThumbnailGenerator(); generatorErr == nil {
		thumbnailGenerator = generator
	}

	terminalLines := []string{}

	m := &xdfileModel{
		activePanel: 0,
		statusText:  "Type to build a command. Arrows stay on panels unless the command popup is open.",
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: leftPath, RangeAnchor: -1},
			{Label: "RIGHT", Cwd: rightPath, RangeAnchor: -1},
		},
		showHidden:           layoutPrefs.ShowHidden,
		layoutPrefs:          layoutPrefs,
		layoutFile:           layoutFile,
		commandsFile:         commandsFile,
		netboxFile:           netboxFile,
		netboxConnections:    netboxConnections,
		commandPromptHistory: make(map[string]string),
		quickView: xdfileQuickView{
			Viewport: quickViewVP,
		},
		terminal: xdfileTerminal{
			Cwd:              leftPath,
			Input:            termInput,
			Viewport:         vp,
			Lines:            terminalLines,
			Events:           nil,
			Session:          nil,
			Emulator:         nil,
			History:          terminalHistory,
			HistoryItems:     terminalHistoryItems,
			HistoryDeleted:   terminalHistoryDeleted,
			HistoryIndex:     -1,
			PendingPanel:     -1,
			SuggestionCursor: -1,
		},
		modal: xdfileModal{
			Input: modalInput,
		},
		imagePreviewer:     filepreview.NewImagePreviewer(),
		thumbnailGenerator: thumbnailGenerator,
		lastClick:          xdfileClickState{panel: -1, row: -1},
		terminalStarting:   false,
		hover:              xdfileNoHoverState(),
		panelSearch: xdfilePanelSearchState{
			Panel: -1,
		},
	}
	if layoutErr != nil {
		m.setStatusErr(fmt.Errorf("layout settings ignored: %w", layoutErr))
	}
	if commandsErr != nil {
		m.setStatusErr(fmt.Errorf("user menu settings ignored: %w", commandsErr))
	}
	if netboxErr != nil {
		m.setStatusErr(fmt.Errorf("SSH connection settings ignored: %w", netboxErr))
	}
	if terminalHistoryErr != nil {
		m.setStatusErr(fmt.Errorf("terminal history ignored: %w", terminalHistoryErr))
	}
	if deleteUndoCleanupErr != nil {
		m.setStatusErr(fmt.Errorf("stale delete undo cleanup ignored: %w", deleteUndoCleanupErr))
	} else if deleteUndoCleanupCount > 0 {
		label := "directories"
		if deleteUndoCleanupCount == 1 {
			label = "directory"
		}
		m.setStatus("Released %d stale delete undo %s from the previous session", deleteUndoCleanupCount, label)
	}
	m.reloadAllPanels()
	m.syncThemeInputStyles()
	m.refreshManagedTerminalSuggestions()
	m.syncTerminalViewport(true)
	return m
}

func (m *xdfileModel) syncThemeInputStyles() {
	m.terminal.Input.Prompt = m.managedTerminalPrompt()
	m.terminal.Input.TextStyle = xdfileTerminalInputTextStyle
	m.terminal.Input.PromptStyle = xdfileTerminalInputPromptStyle
	m.terminal.Input.PlaceholderStyle = xdfileDimStyle
	m.terminal.Input.CompletionStyle = xdfileTerminalSuggestionStyle
	m.terminal.Input.ShowSuggestions = false
	m.terminal.Input.Cursor.Style = xdfileTerminalInputCursorStyle

	m.ensureModalInputInitialized()
	m.modal.Input.TextStyle = xdfilePathStyle
	m.modal.Input.PromptStyle = xdfileTagStyle
	m.modal.Input.PlaceholderStyle = xdfileDimStyle
	m.modal.Input.Cursor.Style = xdfileTagStyle

	for i := range m.modal.FormFields {
		if m.modal.FormFields[i].Multiline {
			m.modal.FormFields[i].TextArea.FocusedStyle.Base = lipgloss.NewStyle()
			m.modal.FormFields[i].TextArea.FocusedStyle.Text = xdfilePathStyle
			m.modal.FormFields[i].TextArea.FocusedStyle.Prompt = xdfileTagStyle
			m.modal.FormFields[i].TextArea.FocusedStyle.Placeholder = xdfileDimStyle
			m.modal.FormFields[i].TextArea.FocusedStyle.CursorLine = lipgloss.NewStyle()
			m.modal.FormFields[i].TextArea.FocusedStyle.CursorLineNumber = xdfileDimStyle
			m.modal.FormFields[i].TextArea.FocusedStyle.EndOfBuffer = xdfileDimStyle
			m.modal.FormFields[i].TextArea.BlurredStyle = m.modal.FormFields[i].TextArea.FocusedStyle
			m.modal.FormFields[i].TextArea.Cursor.Style = xdfileTagStyle
			m.modal.FormFields[i].syncMirrorInput()
			continue
		}
		m.modal.FormFields[i].Input.TextStyle = xdfilePathStyle
		m.modal.FormFields[i].Input.PromptStyle = xdfileTagStyle
		m.modal.FormFields[i].Input.PlaceholderStyle = xdfileDimStyle
		m.modal.FormFields[i].Input.Cursor.Style = xdfileTagStyle
	}
}

func xdfileNewModalInput() textinput.Model {
	input := textinput.New()
	input.Prompt = "> "
	input.CharLimit = 2048
	input.TextStyle = xdfilePathStyle
	input.PromptStyle = xdfileTagStyle
	input.PlaceholderStyle = xdfileDimStyle
	input.Cursor.Style = xdfileTagStyle
	return input
}

func (m *xdfileModel) ensureModalInputInitialized() {
	if m == nil {
		return
	}
	if m.modal.Input.Cursor.BlinkSpeed <= 0 {
		m.modal.Input = xdfileNewModalInput()
	}
}

func (m *xdfileModel) modalInputModel() textinput.Model {
	if m == nil {
		return xdfileNewModalInput()
	}
	m.ensureModalInputInitialized()
	return m.modal.Input
}

func xdfileNewManagedTerminalInput() textinput.Model {
	input := textinput.New()
	input.Prompt = xdfileStartupTerminalPrompt()
	input.Placeholder = "Run commands in the Xdfile Manager shell"
	input.CharLimit = 4096
	input.TextStyle = xdfileTerminalInputTextStyle
	input.PromptStyle = xdfileTerminalInputPromptStyle
	input.PlaceholderStyle = xdfileDimStyle
	input.CompletionStyle = xdfileTerminalSuggestionStyle
	input.ShowSuggestions = false
	input.Cursor.Style = xdfileTerminalInputCursorStyle
	_ = input.Focus()
	return input
}

func (m *xdfileModel) managedTerminalPrompt() string {
	if m == nil {
		return xdfileStartupTerminalPrompt()
	}
	remote, ok := xdfileParseNetBoxPath(m.terminal.Cwd)
	if !ok {
		return xdfileStartupTerminalPrompt()
	}
	label := remote.Profile
	if connection, ok := m.netBoxConnectionByName(remote.Profile); ok {
		if user := connection.sshUser(); user != "" {
			label = user + "@" + remote.Profile
		}
	}
	return label + "> "
}

func (m *xdfileModel) syncManagedTerminalPrompt() {
	if m == nil || m.terminalUsesPTY() {
		return
	}
	m.terminal.Input.Prompt = m.managedTerminalPrompt()
}

func (m *xdfileModel) focusManagedTerminalInput() {
	if m == nil || m.terminalUsesPTY() {
		return
	}

	defer func() {
		if recover() != nil {
			value := m.terminal.Input.Value()
			input := xdfileNewManagedTerminalInput()
			input.SetValue(value)
			input.CursorEnd()
			m.terminal.Input = input
		}
	}()

	_ = m.terminal.Input.Focus()
	m.terminal.Input.CursorEnd()
}

func (m *xdfileModel) applyThemeByName(name string) {
	theme := xdfileThemeByName(name)
	xdfileApplyTheme(theme)
	m.layoutPrefs.ThemeName = theme.Name
	m.syncThemeInputStyles()
}

func (m *xdfileModel) Init() tea.Cmd {
	cmds := []tea.Cmd{
		tea.SetWindowTitle(xdfileProductName),
		textinput.Blink,
		xdfileScheduleAutoRefresh(),
	}
	if m.terminal.Events != nil {
		cmds = append(cmds, xdfileWaitTerminalMsg(m.terminal.Events))
	}
	return tea.Batch(cmds...)
}
