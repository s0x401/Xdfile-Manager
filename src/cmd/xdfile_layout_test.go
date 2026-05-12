package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func assertXdfileCommandItemsEqual(t *testing.T, got []xdfileCommandItem, want []xdfileCommandItem) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("expected %d command items, got %d", len(want), len(got))
	}
	for i := range want {
		gotItem := got[i].normalized()
		wantItem := want[i].normalized()
		if gotItem.Type != wantItem.Type || gotItem.Hotkey != wantItem.Hotkey || gotItem.Label != wantItem.Label || gotItem.Command != wantItem.Command {
			t.Fatalf("expected command item %d %+v, got %+v", i, wantItem, gotItem)
		}
		assertXdfileCommandItemsEqual(t, gotItem.Items, wantItem.Items)
	}
}

func TestXdfileLoadLayoutPrefsDefaultsPanelClickFocusTerminal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "xdfile-layout.json")

	if err := os.WriteFile(path, []byte(`{"panel_split_percent":55,"terminal_height_percent":30}`), 0o644); err != nil {
		t.Fatalf("write layout file: %v", err)
	}

	prefs, err := xdfileLoadLayoutPrefs(path)
	if err != nil {
		t.Fatalf("load layout prefs: %v", err)
	}
	if prefs.PanelClickFocusTerminal {
		t.Fatalf("expected panel click terminal focus to default to disabled")
	}
	if !prefs.ShowHidden {
		t.Fatalf("expected hidden files to default to visible")
	}
	if !prefs.QuickViewDocked {
		t.Fatalf("expected quick view docked to default to enabled")
	}
}

func TestXdfileSaveAndLoadLayoutPrefsRoundTripPanelClickFocusTerminal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "xdfile-layout.json")
	leftDir := filepath.Join(dir, "left")
	rightDir := filepath.Join(dir, "right")

	if err := os.MkdirAll(leftDir, 0o755); err != nil {
		t.Fatalf("create left dir: %v", err)
	}
	if err := os.MkdirAll(rightDir, 0o755); err != nil {
		t.Fatalf("create right dir: %v", err)
	}

	want := xdfileLayoutPrefs{
		PanelSplitPercent:       48,
		TerminalHeightPercent:   28,
		PanelClickFocusTerminal: false,
		ShowHidden:              true,
		StartupLeftPath:         leftDir,
		StartupRightPath:        rightDir,
		LeftSortMode:            xdfileSortModeExt,
		RightSortMode:           xdfileSortModeName,
		QuickViewDocked:         true,
		Commands: []xdfileCommandItem{
			{Type: xdfileCommandItemTypeCommand, Hotkey: "f3", Label: "View with Python", Command: `python "!.!"`},
			{
				Type:   xdfileCommandItemTypeMenu,
				Hotkey: "m",
				Label:  "Tools",
				Items: []xdfileCommandItem{
					{Type: xdfileCommandItemTypeCommand, Hotkey: "ctrl+alt+r", Label: "Run tool", Command: `tool.exe "!.!"`},
				},
			},
		},
	}

	if err := xdfileSaveLayoutPrefs(path, want); err != nil {
		t.Fatalf("save layout prefs: %v", err)
	}

	got, err := xdfileLoadLayoutPrefs(path)
	if err != nil {
		t.Fatalf("load layout prefs: %v", err)
	}

	if got.PanelSplitPercent != want.PanelSplitPercent {
		t.Fatalf("expected panel split %d, got %d", want.PanelSplitPercent, got.PanelSplitPercent)
	}
	if got.TerminalHeightPercent != want.TerminalHeightPercent {
		t.Fatalf("expected terminal height %d, got %d", want.TerminalHeightPercent, got.TerminalHeightPercent)
	}
	if got.PanelClickFocusTerminal != want.PanelClickFocusTerminal {
		t.Fatalf("expected panel click terminal focus %v, got %v", want.PanelClickFocusTerminal, got.PanelClickFocusTerminal)
	}
	if got.ShowHidden != want.ShowHidden {
		t.Fatalf("expected hidden files visibility %v, got %v", want.ShowHidden, got.ShowHidden)
	}
	if got.StartupLeftPath != want.StartupLeftPath {
		t.Fatalf("expected startup left path %q, got %q", want.StartupLeftPath, got.StartupLeftPath)
	}
	if got.StartupRightPath != want.StartupRightPath {
		t.Fatalf("expected startup right path %q, got %q", want.StartupRightPath, got.StartupRightPath)
	}
	if got.LeftSortMode != want.LeftSortMode {
		t.Fatalf("expected left sort mode %q, got %q", want.LeftSortMode, got.LeftSortMode)
	}
	if got.RightSortMode != want.RightSortMode {
		t.Fatalf("expected right sort mode %q, got %q", want.RightSortMode, got.RightSortMode)
	}
	if got.QuickViewDocked != want.QuickViewDocked {
		t.Fatalf("expected quick view docked %v, got %v", want.QuickViewDocked, got.QuickViewDocked)
	}
	assertXdfileCommandItemsEqual(t, got.Commands, want.Commands)
}

func TestXdfileSaveLayoutPrefsWithoutCommandsOmitsLegacyCommandTree(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "xdfile-layout.json")

	want := xdfileLayoutPrefs{
		PanelSplitPercent:       48,
		TerminalHeightPercent:   28,
		PanelClickFocusTerminal: false,
		ShowHidden:              true,
		Commands: []xdfileCommandItem{
			{Type: xdfileCommandItemTypeCommand, Hotkey: "f3", Label: "Viewer", Command: `python "!.!"`},
		},
	}

	if err := xdfileSaveLayoutPrefsWithoutCommands(path, want); err != nil {
		t.Fatalf("save layout prefs without commands: %v", err)
	}

	got, err := xdfileLoadLayoutPrefs(path)
	if err != nil {
		t.Fatalf("load layout prefs: %v", err)
	}
	if len(got.Commands) != 0 {
		t.Fatalf("expected legacy command tree to be omitted from saved layout, got %+v", got.Commands)
	}
}

func TestXdfileLoadOrMigrateCommandPrefsMovesLegacyLayoutCommandsToCommandsFile(t *testing.T) {
	dir := t.TempDir()
	layoutFile := filepath.Join(dir, "xdfile-layout.json")
	commandsFile := filepath.Join(dir, "xdfile-commands.json")

	layoutPrefs := xdfileLayoutPrefs{
		PanelSplitPercent:       48,
		TerminalHeightPercent:   28,
		PanelClickFocusTerminal: false,
		ShowHidden:              true,
		Commands: []xdfileCommandItem{
			{
				Type:   xdfileCommandItemTypeMenu,
				Hotkey: "m",
				Label:  "Tools",
				Items: []xdfileCommandItem{
					{Type: xdfileCommandItemTypeCommand, Hotkey: "f3", Label: "Viewer", Command: `python "!.!"`},
				},
			},
		},
	}
	if err := xdfileSaveLayoutPrefs(layoutFile, layoutPrefs); err != nil {
		t.Fatalf("save legacy layout prefs: %v", err)
	}

	loadedLayout, err := xdfileLoadLayoutPrefs(layoutFile)
	if err != nil {
		t.Fatalf("load legacy layout prefs: %v", err)
	}
	commands, err := xdfileLoadOrMigrateCommandPrefs(layoutFile, commandsFile, &loadedLayout)
	if err != nil {
		t.Fatalf("migrate legacy command prefs: %v", err)
	}
	assertXdfileCommandItemsEqual(t, commands, layoutPrefs.Commands)

	savedCommands, exists, err := xdfileLoadCommandPrefs(commandsFile)
	if err != nil {
		t.Fatalf("load migrated commands file: %v", err)
	}
	if !exists {
		t.Fatalf("expected migrated commands file to exist")
	}
	assertXdfileCommandItemsEqual(t, savedCommands, layoutPrefs.Commands)

	cleanedLayout, err := xdfileLoadLayoutPrefs(layoutFile)
	if err != nil {
		t.Fatalf("reload cleaned layout prefs: %v", err)
	}
	if len(cleanedLayout.Commands) != 0 {
		t.Fatalf("expected cleaned layout prefs to drop legacy command tree, got %+v", cleanedLayout.Commands)
	}
}

func TestXdfileResolveStartPathsUsesSavedStartupPaths(t *testing.T) {
	dir := t.TempDir()
	leftDir := filepath.Join(dir, "left")
	rightDir := filepath.Join(dir, "right")

	if err := os.MkdirAll(leftDir, 0o755); err != nil {
		t.Fatalf("create left dir: %v", err)
	}
	if err := os.MkdirAll(rightDir, 0o755); err != nil {
		t.Fatalf("create right dir: %v", err)
	}

	left, right := xdfileResolveStartPaths(nil, xdfileLayoutPrefs{
		StartupLeftPath:  leftDir,
		StartupRightPath: rightDir,
	})

	if left != leftDir {
		t.Fatalf("expected left path %q, got %q", leftDir, left)
	}
	if right != rightDir {
		t.Fatalf("expected right path %q, got %q", rightDir, right)
	}
}

func TestXdfileResolveStartPathsCLIOverridesSavedStartupPaths(t *testing.T) {
	dir := t.TempDir()
	savedLeftDir := filepath.Join(dir, "saved-left")
	savedRightDir := filepath.Join(dir, "saved-right")
	cliLeftDir := filepath.Join(dir, "cli-left")
	cliRightDir := filepath.Join(dir, "cli-right")

	for _, path := range []string{savedLeftDir, savedRightDir, cliLeftDir, cliRightDir} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatalf("create dir %q: %v", path, err)
		}
	}

	left, right := xdfileResolveStartPaths(
		[]string{cliLeftDir, cliRightDir},
		xdfileLayoutPrefs{
			StartupLeftPath:  savedLeftDir,
			StartupRightPath: savedRightDir,
		},
	)

	if left != cliLeftDir {
		t.Fatalf("expected left path %q, got %q", cliLeftDir, left)
	}
	if right != cliRightDir {
		t.Fatalf("expected right path %q, got %q", cliRightDir, right)
	}
}

func TestXdfileResolveStartPathsIgnoresEmptyCLIArgsAndUsesSavedStartupPaths(t *testing.T) {
	dir := t.TempDir()
	leftDir := filepath.Join(dir, "saved-left")
	rightDir := filepath.Join(dir, "saved-right")

	for _, path := range []string{leftDir, rightDir} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatalf("create dir %q: %v", path, err)
		}
	}

	left, right := xdfileResolveStartPaths(
		[]string{"", "   "},
		xdfileLayoutPrefs{
			StartupLeftPath:  leftDir,
			StartupRightPath: rightDir,
		},
	)

	if left != leftDir {
		t.Fatalf("expected empty CLI args to fall back to saved left path %q, got %q", leftDir, left)
	}
	if right != rightDir {
		t.Fatalf("expected empty CLI args to fall back to saved right path %q, got %q", rightDir, right)
	}
}

func TestXdfileTruncateLeftKeepsRightSideWithinDisplayWidth(t *testing.T) {
	got := xdfileTruncateLeft(`C:\very\long\path\file.txt`, 12, "...")
	if got != `...\file.txt` {
		t.Fatalf("expected compact right side %q, got %q", `...\file.txt`, got)
	}
	if lipgloss.Width(got) != 12 {
		t.Fatalf("expected compacted path width 12, got %d", lipgloss.Width(got))
	}
}

func TestCaptureCurrentLayoutPrefsPreservesCommandsAndViewOptions(t *testing.T) {
	leftDir := t.TempDir()
	rightDir := t.TempDir()

	m := &xdfileModel{
		width:      120,
		height:     40,
		showHidden: true,
		layout: xdfileLayout{
			panelRects: [2]xdfileRect{
				{w: 60, h: 24},
				{w: 59, h: 24},
			},
			terminalRect: xdfileRect{h: 12},
		},
		panels: [2]xdfilePanel{
			{Cwd: leftDir},
			{Cwd: rightDir},
		},
		layoutPrefs: xdfileLayoutPrefs{
			PanelClickFocusTerminal: false,
			ShowHidden:              true,
			ThemeName:               xdfileThemePersona3ReloadName,
			LeftSortMode:            xdfileSortModeExt,
			RightSortMode:           xdfileSortModeName,
			QuickViewDocked:         true,
			Commands: []xdfileCommandItem{
				{
					Type:   xdfileCommandItemTypeMenu,
					Hotkey: "m",
					Label:  "Tools",
					Items: []xdfileCommandItem{
						{Type: xdfileCommandItemTypeCommand, Hotkey: "f3", Label: "Viewer", Command: `python "!.!"`},
					},
				},
			},
		},
	}

	m.captureCurrentLayoutPrefs()

	if m.layoutPrefs.LeftSortMode != xdfileSortModeExt {
		t.Fatalf("expected left sort mode to be preserved, got %q", m.layoutPrefs.LeftSortMode)
	}
	if m.layoutPrefs.RightSortMode != xdfileSortModeName {
		t.Fatalf("expected right sort mode to be preserved, got %q", m.layoutPrefs.RightSortMode)
	}
	if !m.layoutPrefs.QuickViewDocked {
		t.Fatalf("expected quick view docked preference to be preserved")
	}
	if !m.layoutPrefs.ShowHidden {
		t.Fatalf("expected hidden files preference to be preserved")
	}
	if m.layoutPrefs.ThemeName != xdfileThemePersona3ReloadName {
		t.Fatalf("expected theme preference to be preserved, got %q", m.layoutPrefs.ThemeName)
	}
	if len(m.layoutPrefs.Commands) != 1 {
		t.Fatalf("expected commands to be preserved, got %d", len(m.layoutPrefs.Commands))
	}
	if m.layoutPrefs.Commands[0].Label != "Tools" || len(m.layoutPrefs.Commands[0].Items) != 1 || m.layoutPrefs.Commands[0].Items[0].Label != "Viewer" {
		t.Fatalf("expected command label to be preserved, got %+v", m.layoutPrefs.Commands[0])
	}
}

func TestSaveLayoutAlsoPersistsUserMenu(t *testing.T) {
	dir := t.TempDir()
	leftDir := filepath.Join(dir, "left")
	rightDir := filepath.Join(dir, "right")
	layoutFile := filepath.Join(dir, "xdfile-layout.json")
	commandsFile := filepath.Join(dir, "xdfile-commands.json")

	for _, path := range []string{leftDir, rightDir} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatalf("create dir %q: %v", path, err)
		}
	}

	m := &xdfileModel{
		width:        120,
		height:       40,
		layoutFile:   layoutFile,
		commandsFile: commandsFile,
		layout: xdfileLayout{
			panelRects: [2]xdfileRect{
				{w: 60, h: 24},
				{w: 59, h: 24},
			},
			terminalRect: xdfileRect{h: 12},
		},
		panels: [2]xdfilePanel{
			{Cwd: leftDir},
			{Cwd: rightDir},
		},
		layoutPrefs: xdfileLayoutPrefs{
			PanelClickFocusTerminal: false,
			ShowHidden:              false,
			ThemeName:               xdfileThemePersona3ReloadName,
			QuickViewDocked:         true,
			Commands: []xdfileCommandItem{
				{Type: xdfileCommandItemTypeCommand, Hotkey: "f3", Label: "Viewer", Command: `python "!.!"`},
			},
		},
	}

	if cmd := m.saveLayout(); cmd != nil {
		t.Fatalf("expected save layout not to return async command")
	}

	loadedLayout, err := xdfileLoadLayoutPrefs(layoutFile)
	if err != nil {
		t.Fatalf("load saved layout prefs: %v", err)
	}
	if loadedLayout.PanelClickFocusTerminal {
		t.Fatalf("expected saved panel click setting to remain disabled")
	}
	if loadedLayout.ShowHidden {
		t.Fatalf("expected saved hidden files setting to remain disabled")
	}
	if loadedLayout.ThemeName != xdfileThemePersona3ReloadName {
		t.Fatalf("expected saved theme to remain %q, got %q", xdfileThemePersona3ReloadName, loadedLayout.ThemeName)
	}
	if !loadedLayout.QuickViewDocked {
		t.Fatalf("expected saved quick view docked setting to remain enabled")
	}
	if loadedLayout.StartupLeftPath != leftDir {
		t.Fatalf("expected save setup to persist left panel path %q, got %q", leftDir, loadedLayout.StartupLeftPath)
	}
	if loadedLayout.StartupRightPath != rightDir {
		t.Fatalf("expected save setup to persist right panel path %q, got %q", rightDir, loadedLayout.StartupRightPath)
	}

	loadedCommands, exists, err := xdfileLoadCommandPrefs(commandsFile)
	if err != nil {
		t.Fatalf("load saved command prefs: %v", err)
	}
	if !exists {
		t.Fatalf("expected commands file to be written by save setup")
	}
	assertXdfileCommandItemsEqual(t, loadedCommands, m.layoutPrefs.Commands)
}

func TestResetSetupRestoresDefaultsAndClearsUserMenu(t *testing.T) {
	dir := t.TempDir()
	leftDir := filepath.Join(dir, "left")
	rightDir := filepath.Join(dir, "right")
	layoutFile := filepath.Join(dir, "xdfile-layout.json")
	commandsFile := filepath.Join(dir, "xdfile-commands.json")

	for _, path := range []string{leftDir, rightDir} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatalf("create dir %q: %v", path, err)
		}
	}

	m := &xdfileModel{
		width:        120,
		height:       40,
		layoutFile:   layoutFile,
		commandsFile: commandsFile,
		layout: xdfileLayout{
			panelRects: [2]xdfileRect{
				{w: 72, h: 24},
				{w: 47, h: 24},
			},
			terminalRect: xdfileRect{h: 10},
		},
		panels: [2]xdfilePanel{
			{Cwd: leftDir},
			{Cwd: rightDir},
		},
		layoutPrefs: xdfileLayoutPrefs{
			PanelSplitPercent:       60,
			TerminalHeightPercent:   30,
			PanelClickFocusTerminal: true,
			ShowHidden:              false,
			ThemeName:               xdfileThemePersona3ReloadName,
			LeftSortMode:            xdfileSortModeExt,
			RightSortMode:           xdfileSortModeExt,
			QuickViewDocked:         false,
			Commands: []xdfileCommandItem{
				{Type: xdfileCommandItemTypeCommand, Hotkey: "f3", Label: "Viewer", Command: `python "!.!"`},
			},
		},
		quickView: xdfileQuickView{
			Open: true,
			Path: filepath.Join(leftDir, "preview.txt"),
		},
		activePanel:     1,
		terminalFocused: true,
	}

	if cmd := m.resetSetup(); cmd != nil {
		if msg := cmd(); msg != nil {
			t.Fatalf("expected reset setup follow-up sync to complete silently, got %#v", msg)
		}
	}

	if m.layoutPrefs.PanelClickFocusTerminal {
		t.Fatalf("expected panel click terminal focus to reset to disabled")
	}
	if !m.layoutPrefs.ShowHidden {
		t.Fatalf("expected hidden files visibility to reset to enabled")
	}
	if m.layoutPrefs.ThemeName != xdfileThemePersona3Name {
		t.Fatalf("expected theme to reset to %q, got %q", xdfileThemePersona3Name, m.layoutPrefs.ThemeName)
	}
	if !m.layoutPrefs.QuickViewDocked {
		t.Fatalf("expected quick view docked to reset to enabled")
	}
	if len(m.layoutPrefs.Commands) != 0 {
		t.Fatalf("expected reset setup to clear user menu, got %+v", m.layoutPrefs.Commands)
	}
	if m.quickView.Open {
		t.Fatalf("expected reset setup to close quick view")
	}

	loadedLayout, err := xdfileLoadLayoutPrefs(layoutFile)
	if err != nil {
		t.Fatalf("load reset layout prefs: %v", err)
	}
	if loadedLayout.PanelClickFocusTerminal {
		t.Fatalf("expected saved reset layout to disable panel click terminal focus")
	}
	if !loadedLayout.ShowHidden {
		t.Fatalf("expected saved reset layout to enable hidden files visibility")
	}
	if loadedLayout.ThemeName != xdfileThemePersona3Name {
		t.Fatalf("expected saved reset layout to restore default theme %q, got %q", xdfileThemePersona3Name, loadedLayout.ThemeName)
	}
	if !loadedLayout.QuickViewDocked {
		t.Fatalf("expected saved reset layout to enable quick view docked")
	}

	loadedCommands, exists, err := xdfileLoadCommandPrefs(commandsFile)
	if err != nil {
		t.Fatalf("load reset command prefs: %v", err)
	}
	if !exists {
		t.Fatalf("expected reset setup to keep commands file in the exe directory")
	}
	if len(loadedCommands) != 0 {
		t.Fatalf("expected reset setup to save an empty user menu, got %+v", loadedCommands)
	}
}
