package cmd

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	charmansi "github.com/charmbracelet/x/ansi"
	vt "github.com/charmbracelet/x/vt"
)

func TestXdfileViewDoesNotExposeRawAnsiFragmentsInTopMenus(t *testing.T) {
	rawAnsiFragment := regexp.MustCompile(`(^|[^\x1b])\[(0m|38;2;)`)

	for _, openMenu := range []xdfileAction{
		xdfileActionPanelsMenu,
		xdfileActionViewMenu,
		xdfileActionTerminalMenu,
		xdfileActionThemeMenu,
		xdfileActionOptionsMenu,
	} {
		m := &xdfileModel{
			width:       120,
			height:      40,
			activePanel: 0,
			openMenu:    openMenu,
			layoutPrefs: xdfileDefaultLayoutPrefs(),
			panels: [2]xdfilePanel{
				{Label: "LEFT", Cwd: `D:\left`},
				{Label: "RIGHT", Cwd: `D:\right`},
			},
		}

		rendered := m.View()
		if rawAnsiFragment.MatchString(rendered) {
			t.Fatalf("expected rendered view for %q not to contain raw ANSI fragments, got %q", openMenu, rendered)
		}
	}
}

func TestXdfileViewDoesNotExposeRawAnsiFragmentsForHoveredPanelEntry(t *testing.T) {
	rawAnsiFragment := regexp.MustCompile(`(^|[^\x1b])\[(0m|38;2;)`)
	m := &xdfileModel{
		width:       120,
		height:      40,
		activePanel: 0,
		layoutPrefs: xdfileDefaultLayoutPrefs(),
		panels: [2]xdfilePanel{
			{
				Label: "LEFT",
				Cwd:   `C:\left`,
				Entries: []xdfileEntry{
					{Name: "xdfile-toggle-footer", Path: `C:\left\xdfile-toggle-footer`},
				},
			},
			{Label: "RIGHT", Cwd: `D:\right`},
		},
		hover: xdfileHoverState{
			MenuItem:   -1,
			Panel:      0,
			PanelIndex: 0,
		},
	}
	m.computeLayout()

	rendered := m.View()
	if rawAnsiFragment.MatchString(rendered) {
		t.Fatalf("expected hovered panel render not to contain raw ANSI fragments, got %q", rendered)
	}
}

func TestXdfileViewDoesNotExposeRawAnsiFragmentsForHoveredFooterAction(t *testing.T) {
	rawAnsiFragment := regexp.MustCompile(`(^|[^\x1b])\[(0m|38;2;)`)
	m := &xdfileModel{
		width:       120,
		height:      40,
		activePanel: 0,
		layoutPrefs: xdfileDefaultLayoutPrefs(),
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: `C:\left`},
			{Label: "RIGHT", Cwd: `D:\right`},
		},
		hover: xdfileHoverState{
			MenuItem:     -1,
			Panel:        -1,
			PanelIndex:   -1,
			FooterAction: xdfileActionCommandsMenu,
		},
	}
	m.computeLayout()

	rendered := m.View()
	if rawAnsiFragment.MatchString(rendered) {
		t.Fatalf("expected hovered footer render not to contain raw ANSI fragments, got %q", rendered)
	}
}

func TestXdfileViewWrapsFrameWithAnsiReset(t *testing.T) {
	m := &xdfileModel{
		width:       120,
		height:      40,
		activePanel: 0,
		layoutPrefs: xdfileDefaultLayoutPrefs(),
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: `D:\left`},
			{Label: "RIGHT", Cwd: `D:\right`},
		},
	}

	rendered := m.View()
	if !strings.HasPrefix(rendered, xdfileANSIReset) {
		t.Fatalf("expected rendered frame to start with ANSI reset, got %q", rendered[:min(16, len(rendered))])
	}
	if !strings.HasSuffix(rendered, xdfileANSIReset) {
		t.Fatalf("expected rendered frame to end with ANSI reset")
	}
}

func TestXdfileConstrainFrameKeepsOutputInsideTerminalBounds(t *testing.T) {
	rendered := xdfileConstrainFrame(
		xdfileStatusOKStyle.Render(strings.Repeat("W", 80))+"\nsecond\nthird",
		20,
		2,
	)

	lines := strings.Split(stripXdfileANSI(rendered), "\n")
	if got := len(lines); got != 2 {
		t.Fatalf("expected frame to be clipped to 2 rows, got %d: %q", got, rendered)
	}
	for i, line := range strings.Split(rendered, "\n") {
		line = strings.TrimSuffix(line, xdfileANSIReset)
		if width := lipgloss.Width(line); width > 20 {
			t.Fatalf("expected line %d to stay within 20 cells, got %d: %q", i, width, line)
		}
	}
}

func TestViewFrameNeverExceedsModelBounds(t *testing.T) {
	longPath := `E:\tesk\P26050700009\96302\94242\sfwewaK9HC\` + strings.Repeat("nested", 20)
	m := &xdfileModel{
		width:       120,
		height:      36,
		activePanel: 1,
		statusText:  strings.Repeat("status-", 40),
		layoutPrefs: xdfileLayoutPrefs{
			PanelSplitPercent:     33,
			TerminalHeightPercent: 70,
			RightSortMode:         xdfileSortModeExt,
		}.normalized(),
		panels: [2]xdfilePanel{
			{
				Label: "LEFT",
				Cwd:   longPath,
				Entries: []xdfileEntry{
					{Name: strings.Repeat("left-file-", 30) + ".txt", Path: longPath},
				},
			},
			{
				Label: "RIGHT",
				Cwd:   longPath,
				Entries: []xdfileEntry{
					{Name: "SW.r", Path: longPath + `\SW.r`, Size: 4_200_000},
				},
			},
		},
		terminal: xdfileTerminal{
			Cwd:   longPath,
			Input: xdfileNewManagedTerminalInput(),
			Lines: []string{
				strings.Repeat("terminal-output-", 30),
			},
		},
	}
	m.syncThemeInputStyles()
	m.syncTerminalViewport(true)

	rendered := m.View()
	lines := strings.Split(stripXdfileANSI(rendered), "\n")
	if got := len(lines); got != m.height {
		t.Fatalf("expected view to render exactly %d rows, got %d", m.height, got)
	}
	for i, line := range strings.Split(rendered, "\n") {
		line = strings.TrimSuffix(line, xdfileANSIReset)
		if width := lipgloss.Width(line); width > m.width {
			t.Fatalf("expected line %d to stay within %d cells, got %d: %q", i, m.width, width, line)
		}
	}
}

func TestViewReturnsBlankWhileUserScreenIsVisible(t *testing.T) {
	m := &xdfileModel{
		width:             120,
		height:            40,
		userScreenVisible: true,
	}

	if got := m.View(); got != "" {
		t.Fatalf("expected user screen mode to suppress Xdfile Manager redraws, got %q", got)
	}
}

func TestXdfileViewKeepsPanelsVisibleAfterRiskyTerminalSGR(t *testing.T) {
	m := &xdfileModel{
		width:       120,
		height:      40,
		activePanel: 0,
		layoutPrefs: xdfileDefaultLayoutPrefs(),
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: `D:\left`, Entries: []xdfileEntry{{Name: "alpha.txt", Path: `D:\left\alpha.txt`}}},
			{Label: "RIGHT", Cwd: `D:\right`, Entries: []xdfileEntry{{Name: "beta.txt", Path: `D:\right\beta.txt`}}},
		},
		terminal: xdfileTerminal{
			Lines: []string{"\x1b[8mconcealed"},
		},
	}

	rendered := stripXdfileANSI(m.View())
	if !strings.Contains(rendered, "LEFT") || !strings.Contains(rendered, "RIGHT") {
		t.Fatalf("expected panel labels to stay visible after risky terminal SGR, got %q", rendered)
	}
}

func TestRenderHeaderPreservesRightInfoWhenWidthIsTight(t *testing.T) {
	m := &xdfileModel{
		width:       52,
		height:      20,
		activePanel: 0,
		layoutPrefs: xdfileDefaultLayoutPrefs(),
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: `C:\very\long\folder\__TAIL__`},
			{Label: "RIGHT", Cwd: `D:\right`},
		},
	}

	header := stripXdfileANSI(m.renderHeader())
	lines := strings.Split(header, "\n")
	if len(lines) < 1 {
		t.Fatalf("expected header to render at least one line")
	}
	if !strings.Contains(lines[0], "__TAIL__") {
		t.Fatalf("expected top-right header info to preserve the path tail, got %q", lines[0])
	}
}

func TestRenderHeaderUsesXdfileManagerTitle(t *testing.T) {
	m := &xdfileModel{
		width:       80,
		height:      20,
		activePanel: 0,
		layoutPrefs: xdfileDefaultLayoutPrefs(),
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: `C:\left`},
			{Label: "RIGHT", Cwd: `D:\right`},
		},
	}

	header := stripXdfileANSI(m.renderHeader())
	if !strings.Contains(header, "Xdfile Manager") {
		t.Fatalf("expected header to use Xdfile Manager title, got %q", header)
	}
	if strings.Contains(header, "dual-pane file manager") {
		t.Fatalf("expected old subtitle to be removed, got %q", header)
	}
}

func TestRenderEntryInactiveCursorDoesNotKeepBackground(t *testing.T) {
	m := &xdfileModel{}
	rendered := m.renderEntry(xdfileEntry{Name: "alpha.txt"}, true, false, false, false, 40)
	if regexp.MustCompile(`\x1b\[[0-9;]*48;`).MatchString(rendered) {
		t.Fatalf("expected inactive cursor line not to keep a background highlight, got %q", rendered)
	}
	if !strings.Contains(stripXdfileANSI(rendered), "alpha.txt") {
		t.Fatalf("expected inactive cursor line to keep entry text, got %q", rendered)
	}
}

func TestRenderEntryHoverAddsBackgroundHighlight(t *testing.T) {
	style := xdfileHoveredEntryLineStyle()
	if style.GetUnderline() {
		t.Fatal("expected hovered entry style not to force underline")
	}
	if style.GetBackground() == (lipgloss.TerminalColor)(nil) {
		t.Fatal("expected hovered entry style to set a background color")
	}
	if got := style.GetBackground(); got != xdfileColorHighlight {
		t.Fatalf("expected hovered entry style to use theme highlight color, got %v want %v", got, xdfileColorHighlight)
	}
}

func TestRenderHoveredEntryPreservesFullRowWidths(t *testing.T) {
	m := &xdfileModel{}
	rendered := m.renderEntry(xdfileEntry{Name: "a.txt", Path: `C:\left\a.txt`}, false, false, false, true, 40)
	if got := lipgloss.Width(rendered); got != 40 {
		t.Fatalf("expected hovered row to preserve full width, got %d", got)
	}
	if got := lipgloss.Width(xdfileRenderHoveredEntryNameColumn(xdfileEntry{Name: "a.txt"}, " ", 24)); got != 24 {
		t.Fatalf("expected hovered name column to preserve requested width, got %d", got)
	}
	if got := lipgloss.Width(xdfileRenderHoveredEntryMetaColumn("", 8)); got != 8 {
		t.Fatalf("expected empty hovered meta column to preserve requested width, got %d", got)
	}
}

func TestRenderEntryUsesStructuralPrefixMarkers(t *testing.T) {
	m := &xdfileModel{}

	exeRendered := stripXdfileANSI(m.renderEntry(xdfileEntry{Name: "tool.exe", Path: `C:\left\tool.exe`}, false, false, true, false, 48))
	if !strings.Contains(exeRendered, "[E]") || !strings.Contains(exeRendered, "tool.exe") {
		t.Fatalf("expected executable entry to render [E] marker, got %q", exeRendered)
	}

	scriptRendered := stripXdfileANSI(m.renderEntry(xdfileEntry{Name: "scan.ps1", Path: `C:\left\scan.ps1`}, false, false, true, false, 48))
	if !strings.Contains(scriptRendered, "[S]") || !strings.Contains(scriptRendered, "scan.ps1") {
		t.Fatalf("expected script entry to render [S] marker, got %q", scriptRendered)
	}

	archiveRendered := stripXdfileANSI(m.renderEntry(xdfileEntry{Name: "sample.7z", Path: `C:\left\sample.7z`}, false, false, true, false, 48))
	if !strings.Contains(archiveRendered, "[Z]") || !strings.Contains(archiveRendered, "sample.7z") {
		t.Fatalf("expected archive entry to render [Z] marker, got %q", archiveRendered)
	}

	dllRendered := stripXdfileANSI(m.renderEntry(xdfileEntry{Name: "libcurl.dll", Path: `C:\left\libcurl.dll`}, false, false, true, false, 48))
	if !strings.Contains(dllRendered, "[ ]") || !strings.Contains(dllRendered, "libcurl.dll") {
		t.Fatalf("expected other file entry to render neutral marker, got %q", dllRendered)
	}

	dirRendered := stripXdfileANSI(m.renderEntry(xdfileEntry{Name: "assets", Path: `C:\left\assets`, IsDir: true}, false, false, true, false, 48))
	if !strings.Contains(dirRendered, "[D]") || !strings.Contains(dirRendered, "assets") {
		t.Fatalf("expected directory entry to render [D] marker, got %q", dirRendered)
	}
}

func TestRenderMenuItemsUseTransparentBaseBackground(t *testing.T) {
	if got := xdfileMenuItemStyle.GetBackground(); got != (lipgloss.NoColor{}) {
		t.Fatalf("expected menu item base style background to be transparent, got %v", got)
	}
	if got := xdfileMenuButton.GetBackground(); got != (lipgloss.NoColor{}) {
		t.Fatalf("expected menu bar text background to be transparent, got %v", got)
	}
	if got := xdfileMenuItemKeyStyle.GetBackground(); got != (lipgloss.NoColor{}) {
		t.Fatalf("expected menu item key style background to be transparent, got %v", got)
	}
	if got := xdfileMenuItemDisabledStyle.GetBackground(); got != (lipgloss.NoColor{}) {
		t.Fatalf("expected disabled menu item background to be transparent, got %v", got)
	}
	if got, want := xdfileMenuItemHot.GetBackground(), xdfileSelectedLineStyle(true).GetBackground(); got != want {
		t.Fatalf("expected selected menu item background to match selected file rows, got %v want %v", got, want)
	}
	if got, want := xdfileMenuButtonHot.GetBackground(), xdfileSelectedLineStyle(true).GetBackground(); got != want {
		t.Fatalf("expected active menu bar background to match selected file rows, got %v want %v", got, want)
	}
	if got, want := xdfileHoveredMenuItemStyle().GetBackground(), xdfileHoveredEntryLineStyle().GetBackground(); got != want {
		t.Fatalf("expected hovered menu item background to match hovered file rows, got %v want %v", got, want)
	}
	if got, want := xdfileHoveredMenuButtonStyle().GetBackground(), xdfileHoveredEntryLineStyle().GetBackground(); got != want {
		t.Fatalf("expected hovered menu bar background to match hovered file rows, got %v want %v", got, want)
	}
	if got := xdfileMenuBorder().GetBackground(); got != (lipgloss.NoColor{}) {
		t.Fatalf("expected menu popup background to be transparent, got %v", got)
	}
}

func TestRenderOpenMenuShowsCommandsTitle(t *testing.T) {
	m := &xdfileModel{
		width:       120,
		height:      40,
		openMenu:    xdfileActionCommandsMenu,
		layoutPrefs: xdfileDefaultLayoutPrefs(),
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: `C:\left`},
			{Label: "RIGHT", Cwd: `D:\right`},
		},
		hover: xdfileHoverState{
			MenuItem:   -1,
			Panel:      -1,
			PanelIndex: -1,
		},
	}
	m.computeLayout()

	rendered := stripXdfileANSI(m.renderOpenMenu())
	if !strings.Contains(rendered, "F2 User Menu") {
		t.Fatalf("expected commands menu to render a title, got %q", rendered)
	}
	if strings.Contains(rendered, "F2 User Menu User menu") || strings.Contains(rendered, "F2 User Menu Us...") {
		t.Fatalf("expected root commands menu title not to append the root label, got %q", rendered)
	}
}

func TestRenderPanelKeepsActiveBorderWhileTerminalFocused(t *testing.T) {
	panel := xdfilePanel{
		Label:   "LEFT",
		Cwd:     `C:\left`,
		Entries: []xdfileEntry{{Name: "alpha.txt", Path: `C:\left\alpha.txt`}},
	}
	panel.setCursor(0, 8)

	base := &xdfileModel{
		width:       120,
		height:      40,
		activePanel: 0,
		layoutPrefs: xdfileDefaultLayoutPrefs(),
		panels:      [2]xdfilePanel{panel, {Label: "RIGHT", Cwd: `D:\right`}},
	}
	base.computeLayout()

	unfocused := *base
	unfocused.terminalFocused = false
	focused := *base
	focused.terminalFocused = true

	leftWhenPanelFocused := unfocused.renderPanel(0)
	leftWhenTerminalFocused := focused.renderPanel(0)
	if leftWhenPanelFocused != leftWhenTerminalFocused {
		t.Fatalf("expected active panel render to stay stable while terminal is focused")
	}
}

func TestComputeLayoutKeepsBottomTerminalRect(t *testing.T) {
	m := &xdfileModel{
		width:       120,
		height:      40,
		layoutPrefs: xdfileDefaultLayoutPrefs(),
	}

	m.computeLayout()

	if m.layout.panelRects[0].h == 0 || m.layout.panelRects[1].h == 0 {
		t.Fatalf("expected panel layout to stay intact, got %+v %+v", m.layout.panelRects[0], m.layout.panelRects[1])
	}
	rect := m.terminalRenderRect()
	if rect != m.layout.terminalRect {
		t.Fatalf("expected terminal render rect to match the bottom terminal layout, got %+v want %+v", rect, m.layout.terminalRect)
	}
	if got, want := rect.y, xdfileHeaderHeight+m.layout.panelRects[0].h; got != want {
		t.Fatalf("expected terminal to remain below the file panels, got y=%d want %d", got, want)
	}
}

func TestViewKeepsPanelsVisibleWhileTerminalShowsOutput(t *testing.T) {
	m := &xdfileModel{
		width:       120,
		height:      40,
		layoutPrefs: xdfileDefaultLayoutPrefs(),
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: `C:\left`},
			{Label: "RIGHT", Cwd: `D:\right`},
		},
		terminal: xdfileTerminal{
			Lines: []string{"streamed output"},
		},
	}
	m.computeLayout()
	m.syncTerminalViewport(true)

	rendered := stripXdfileANSI(m.View())
	if !strings.Contains(rendered, "LEFT") || !strings.Contains(rendered, "RIGHT") {
		t.Fatalf("expected terminal output to keep both file panels visible, got %q", rendered)
	}
	if !strings.Contains(rendered, "streamed output") {
		t.Fatalf("expected terminal view to keep terminal output, got %q", rendered)
	}
}

func TestXdfileManagedTerminalViewportContentWrapsAnsiLongLines(t *testing.T) {
	line := xdfileStatusErrStyle.Render("command exited with code -22 (0xffffffea) and a very long trailing path C:\\Users\\HR\\Desktop\\code\\xdfile\\legacy-app\\some\\very\\very\\very\\long\\path")
	content := xdfileManagedTerminalViewportContent([]string{line}, 32)
	lines := strings.Split(content, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected long ANSI line to wrap into multiple lines, got %q", content)
	}
	for _, got := range lines {
		if charmansi.StringWidth(got) > 32 {
			t.Fatalf("expected wrapped terminal line width <= 32, got %d for %q", charmansi.StringWidth(got), got)
		}
	}
}

func TestXdfileManagedTerminalViewportLinesUsesRequestedWindow(t *testing.T) {
	lines := xdfileManagedTerminalViewportLines("a\nb\nc\nd", 1, 2)
	if got := strings.Join(lines, "|"); got != "b|c" {
		t.Fatalf("expected viewport lines to return the requested visible window, got %q", got)
	}
}

func TestXdfileManagedTerminalViewportLinesPadsShortContent(t *testing.T) {
	lines := xdfileManagedTerminalViewportLines("a\nb", 0, 4)
	if got := len(lines); got != 4 {
		t.Fatalf("expected padded viewport height 4, got %d", got)
	}
	if got := strings.Join(lines, "|"); got != "a|b||" {
		t.Fatalf("expected viewport lines to pad short content with blanks, got %q", got)
	}
}

func TestAppendTerminalOutputSuppressesGenericExitLineWhenCommandAlreadyPrintedOutput(t *testing.T) {
	m := &xdfileModel{
		width:       120,
		height:      40,
		layoutPrefs: xdfileDefaultLayoutPrefs(),
		terminal:    xdfileTerminal{},
	}
	m.computeLayout()

	m.appendTerminalOutput(xdfileTerminalResultMsg{
		Command: "git st",
		Dir:     `C:\repo`,
		Output:  "fatal: not a git repository",
		Err:     errors.New("command exited with code 128"),
	})

	joined := strings.Join(m.terminal.Lines, "\n")
	if strings.Contains(joined, "command exited with code 128") {
		t.Fatalf("expected generic exit line to stay out of terminal output, got %q", joined)
	}
	if !strings.Contains(joined, "fatal: not a git repository") {
		t.Fatalf("expected command output to remain visible, got %q", joined)
	}
}

func TestRenderTerminalShowsManagedAnsiOutput(t *testing.T) {
	m := &xdfileModel{
		width:       80,
		height:      20,
		layoutPrefs: xdfileDefaultLayoutPrefs(),
		terminal: xdfileTerminal{
			Cwd:   `C:\repo`,
			Input: textinput.New(),
			Lines: []string{
				xdfileStatusErrStyle.Render("fatal: not a git repository"),
			},
		},
	}

	m.computeLayout()
	m.syncTerminalViewport(true)

	rendered := stripXdfileANSI(m.renderTerminal())
	if !strings.Contains(rendered, "fatal: not a git repository") {
		t.Fatalf("expected managed terminal render to keep ansi output visible, got %q", rendered)
	}
}

func TestRenderPTYTerminalScreenPreservesAnsiStyles(t *testing.T) {
	emulator := vt.NewSafeEmulator(20, 4)
	if _, err := emulator.Write([]byte("\x1b[31mred\x1b[0m")); err != nil {
		t.Fatalf("seed emulator ansi text: %v", err)
	}

	m := &xdfileModel{
		terminal: xdfileTerminal{
			Session:  &xdfileTerminalPTYSession{},
			Emulator: emulator,
		},
	}

	rendered := m.renderPTYTerminalScreen(20, 4)
	if !strings.Contains(rendered, "\x1b[31m") {
		t.Fatalf("expected PTY render output to preserve ANSI color sequences, got %q", rendered)
	}
}

func TestRenderPTYTerminalScreenPreservesUnicodeText(t *testing.T) {
	emulator := vt.NewSafeEmulator(20, 4)
	if _, err := emulator.Write([]byte("\u4e2d\u6587\u8f93\u51fa")); err != nil {
		t.Fatalf("seed emulator unicode text: %v", err)
	}

	m := &xdfileModel{
		terminal: xdfileTerminal{
			Session:  &xdfileTerminalPTYSession{},
			Emulator: emulator,
		},
	}

	rendered := m.renderPTYTerminalScreen(20, 4)
	if !strings.Contains(rendered, "\u4e2d\u6587\u8f93\u51fa") {
		t.Fatalf("expected PTY render output to preserve Unicode text, got %q", rendered)
	}
}

func TestRenderStreamingTerminalScreenPreservesAnsiStyles(t *testing.T) {
	emulator := vt.NewSafeEmulator(40, 4)
	emulator.SetScrollbackSize(32)
	if _, err := emulator.Write([]byte("\x1b[34mread data from clipboard!\x1b[0m\r\n\x1b[32mSuccess!\x1b[0m")); err != nil {
		t.Fatalf("seed streaming emulator ansi text: %v", err)
	}

	rendered := xdfileRenderStreamingTerminalScreen(emulator, 40, 4)
	if !strings.Contains(rendered, "\x1b[34m") || !strings.Contains(rendered, "\x1b[32m") {
		t.Fatalf("expected streaming terminal render to preserve ANSI color sequences, got %q", rendered)
	}
}

func TestXdfileCollectTerminalEmulatorLinesPreservesAnsiStyles(t *testing.T) {
	emulator := vt.NewSafeEmulator(40, 4)
	emulator.SetScrollbackSize(32)
	if _, err := emulator.Write([]byte("plain\r\n\x1b[32mDownloaded -- Total: 1\x1b[0m")); err != nil {
		t.Fatalf("seed terminal emulator history: %v", err)
	}

	lines := xdfileCollectTerminalEmulatorLines(emulator)
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "\x1b[32mDownloaded -- Total: 1") {
		t.Fatalf("expected collected emulator history to preserve ANSI color sequences, got %q", joined)
	}
}

func TestRenderPTYTerminalScreenKeepsCursorVisibleOnOccupiedCell(t *testing.T) {
	emulator := vt.NewSafeEmulator(20, 4)
	if _, err := emulator.Write([]byte("abcd\x1b[D")); err != nil {
		t.Fatalf("seed emulator cursor movement: %v", err)
	}

	m := &xdfileModel{
		terminalFocused: true,
		terminal: xdfileTerminal{
			Session:  &xdfileTerminalPTYSession{},
			Emulator: emulator,
		},
	}

	rendered := m.renderPTYTerminalScreen(20, 4)
	if !strings.Contains(rendered, "d") {
		t.Fatalf("expected PTY render output to keep the underlying character visible, got %q", rendered)
	}
	cursorBG := xdfileHexRGBA(xdfileCurrentTheme.TerminalCursorBackground)
	pattern := fmt.Sprintf(`\x1b\[[0-9;]*48;2;%d;%d;%d[0-9;]*m`, cursorBG.R, cursorBG.G, cursorBG.B)
	if !regexp.MustCompile(pattern).MatchString(rendered) {
		t.Fatalf("expected PTY render output to use the active theme cursor background %q, got %q", xdfileCurrentTheme.TerminalCursorBackground, rendered)
	}
}

func stripXdfileANSI(value string) string {
	ansiEscape := regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)
	return ansiEscape.ReplaceAllString(value, "")
}
