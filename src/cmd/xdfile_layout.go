package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	variable "github.com/s0x401/xdfile-manager/src/config"
	"github.com/s0x401/xdfile-manager/src/internal/utils"
)

const (
	xdfileLayoutFileName   = "xdfile-layout.json"
	xdfileCommandsFileName = "xdfile-commands.json"
)

type xdfileSortMode string

const (
	xdfileSortModeName xdfileSortMode = "name"
	xdfileSortModeExt  xdfileSortMode = "ext"
)

type xdfileLayoutPrefs struct {
	PanelSplitPercent       int                 `json:"panel_split_percent"`
	TerminalHeightPercent   int                 `json:"terminal_height_percent"`
	PanelClickFocusTerminal bool                `json:"panel_click_focus_terminal"`
	ShowHidden              bool                `json:"show_hidden"`
	ThemeName               string              `json:"theme_name,omitempty"`
	StartupLeftPath         string              `json:"startup_left_path"`
	StartupRightPath        string              `json:"startup_right_path"`
	LeftSortMode            xdfileSortMode      `json:"left_sort_mode,omitempty"`
	RightSortMode           xdfileSortMode      `json:"right_sort_mode,omitempty"`
	QuickViewDocked         bool                `json:"quick_view_docked,omitempty"`
	Commands                []xdfileCommandItem `json:"commands,omitempty"`
}

type xdfileCommandPrefs struct {
	Commands []xdfileCommandItem `json:"commands"`
}

func xdfileDefaultLayoutPrefs() xdfileLayoutPrefs {
	return xdfileLayoutPrefs{
		PanelSplitPercent:       50,
		TerminalHeightPercent:   25,
		PanelClickFocusTerminal: false,
		ShowHidden:              true,
		ThemeName:               xdfileThemePersona3Name,
		LeftSortMode:            xdfileSortModeName,
		RightSortMode:           xdfileSortModeName,
		QuickViewDocked:         true,
	}
}

func (p xdfileLayoutPrefs) normalized() xdfileLayoutPrefs {
	defaults := xdfileDefaultLayoutPrefs()
	if p.PanelSplitPercent == 0 {
		p.PanelSplitPercent = defaults.PanelSplitPercent
	}
	if p.TerminalHeightPercent == 0 {
		p.TerminalHeightPercent = defaults.TerminalHeightPercent
	}
	p.PanelSplitPercent = xdfileClamp(p.PanelSplitPercent, 1, 99)
	p.TerminalHeightPercent = xdfileClamp(p.TerminalHeightPercent, 1, 99)
	p.ThemeName = xdfileNormalizeThemeName(p.ThemeName)
	p.LeftSortMode = xdfileNormalizeSortMode(p.LeftSortMode)
	p.RightSortMode = xdfileNormalizeSortMode(p.RightSortMode)
	p.Commands = xdfileNormalizeCommandItems(p.Commands)
	return p
}

func xdfileNormalizeSortMode(mode xdfileSortMode) xdfileSortMode {
	switch mode {
	case xdfileSortModeExt:
		return xdfileSortModeExt
	default:
		return xdfileSortModeName
	}
}

func xdfileLayoutPrefsPath() string {
	return filepath.Join(variable.XdfileMainDir, xdfileLayoutFileName)
}

func xdfileCommandsPrefsPath() string {
	return filepath.Join(variable.XdfileMainDir, xdfileCommandsFileName)
}

func xdfileLoadLayoutPrefs(path string) (xdfileLayoutPrefs, error) {
	prefs := xdfileDefaultLayoutPrefs()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return prefs, nil
		}
		return prefs, fmt.Errorf("read layout settings: %w", err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return prefs, nil
	}
	if err := json.Unmarshal(data, &prefs); err != nil {
		return xdfileDefaultLayoutPrefs(), fmt.Errorf("parse layout settings: %w", err)
	}
	return prefs.normalized(), nil
}

func xdfileSaveLayoutPrefs(path string, prefs xdfileLayoutPrefs) error {
	prefs = prefs.normalized()
	if err := os.MkdirAll(filepath.Dir(path), utils.ConfigDirPerm); err != nil {
		return fmt.Errorf("create layout config directory: %w", err)
	}

	data, err := json.MarshalIndent(prefs, "", "  ")
	if err != nil {
		return fmt.Errorf("encode layout settings: %w", err)
	}
	if err := os.WriteFile(path, data, utils.ConfigFilePerm); err != nil {
		return fmt.Errorf("write layout settings: %w", err)
	}
	return nil
}

func xdfileSaveLayoutPrefsWithoutCommands(path string, prefs xdfileLayoutPrefs) error {
	prefs.Commands = nil
	return xdfileSaveLayoutPrefs(path, prefs)
}

func xdfileLoadCommandPrefs(path string) ([]xdfileCommandItem, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("read user menu settings: %w", err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, true, nil
	}

	var prefs xdfileCommandPrefs
	if err := json.Unmarshal(data, &prefs); err != nil {
		return nil, true, fmt.Errorf("parse user menu settings: %w", err)
	}
	return xdfileNormalizeCommandItems(prefs.Commands), true, nil
}

func xdfileSaveCommandPrefs(path string, commands []xdfileCommandItem) error {
	if err := os.MkdirAll(filepath.Dir(path), utils.ConfigDirPerm); err != nil {
		return fmt.Errorf("create user menu config directory: %w", err)
	}

	data, err := json.MarshalIndent(xdfileCommandPrefs{
		Commands: xdfileNormalizeCommandItems(commands),
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode user menu settings: %w", err)
	}
	if err := os.WriteFile(path, data, utils.ConfigFilePerm); err != nil {
		return fmt.Errorf("write user menu settings: %w", err)
	}
	return nil
}

func xdfileLoadOrMigrateCommandPrefs(layoutPath string, commandsPath string, layoutPrefs *xdfileLayoutPrefs) ([]xdfileCommandItem, error) {
	commands, exists, err := xdfileLoadCommandPrefs(commandsPath)
	if err != nil {
		legacy := xdfileNormalizeCommandItems(layoutPrefs.Commands)
		if len(legacy) > 0 {
			return legacy, err
		}
		return nil, err
	}
	if exists {
		return commands, nil
	}

	legacy := xdfileNormalizeCommandItems(layoutPrefs.Commands)
	if len(legacy) == 0 {
		return nil, nil
	}
	if err := xdfileSaveCommandPrefs(commandsPath, legacy); err != nil {
		return legacy, fmt.Errorf("migrate user menu settings: %w", err)
	}
	layoutPrefs.Commands = nil
	if layoutPath != "" {
		if err := xdfileSaveLayoutPrefsWithoutCommands(layoutPath, *layoutPrefs); err != nil {
			return legacy, fmt.Errorf("cleanup legacy layout settings: %w", err)
		}
	}
	return legacy, nil
}

func xdfileClamp(value int, minValue int, maxValue int) int {
	if maxValue < minValue {
		return minValue
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func xdfilePercentFromSize(part int, total int) int {
	if total <= 0 {
		return 0
	}
	percent := int(math.Round(float64(part) * 100 / float64(total)))
	return xdfileClamp(percent, 1, 99)
}

func xdfileLeftWidthForPrefs(totalWidth int, prefs xdfileLayoutPrefs) int {
	if totalWidth <= 0 {
		return 0
	}
	if totalWidth < xdfileMinPanelWidth*2 {
		return totalWidth / 2
	}

	prefs = prefs.normalized()
	minLeft := xdfileMinPanelWidth
	maxLeft := totalWidth - xdfileMinPanelWidth
	width := int(math.Round(float64(totalWidth) * float64(prefs.PanelSplitPercent) / 100))
	return xdfileClamp(width, minLeft, maxLeft)
}

func xdfileTerminalHeightForPrefs(totalHeight int, prefs xdfileLayoutPrefs) int {
	if totalHeight <= 0 {
		return 0
	}

	minTerminal := min(xdfileMinTerminalHeight, max(1, totalHeight-1))
	maxTerminal := max(minTerminal, totalHeight-xdfileMinPanelBodyHeight)
	if maxTerminal < minTerminal {
		return minTerminal
	}

	prefs = prefs.normalized()
	height := int(math.Round(float64(totalHeight) * float64(prefs.TerminalHeightPercent) / 100))
	return xdfileClamp(height, minTerminal, maxTerminal)
}

func (m *xdfileModel) currentLayoutPercents() (int, int) {
	panelPercent := m.layoutPrefs.normalized().PanelSplitPercent
	terminalPercent := m.layoutPrefs.normalized().TerminalHeightPercent

	if m.width > 1 && m.layout.panelRects[0].w > 0 {
		panelPercent = xdfilePercentFromSize(m.layout.panelRects[0].w, m.width-1)
	}
	if m.height > xdfileHeaderHeight+xdfileFooterHeight && m.layout.terminalRect.h > 0 {
		terminalPercent = xdfilePercentFromSize(
			m.layout.terminalRect.h,
			m.height-xdfileHeaderHeight-xdfileFooterHeight,
		)
	}

	return panelPercent, terminalPercent
}

func (m *xdfileModel) layoutStatusLabel() string {
	panelPercent, terminalPercent := m.currentLayoutPercents()
	return fmt.Sprintf("L%d T%d", panelPercent, terminalPercent)
}

func (m *xdfileModel) captureCurrentLayoutPrefs() {
	panelPercent, terminalPercent := m.currentLayoutPercents()
	current := m.layoutPrefs.normalized()
	m.layoutPrefs = xdfileLayoutPrefs{
		PanelSplitPercent:       panelPercent,
		TerminalHeightPercent:   terminalPercent,
		PanelClickFocusTerminal: current.PanelClickFocusTerminal,
		ShowHidden:              m.showHidden,
		ThemeName:               current.ThemeName,
		StartupLeftPath:         m.panels[0].Cwd,
		StartupRightPath:        m.panels[1].Cwd,
		LeftSortMode:            current.LeftSortMode,
		RightSortMode:           current.RightSortMode,
		QuickViewDocked:         current.QuickViewDocked,
		Commands:                append([]xdfileCommandItem(nil), current.Commands...),
	}.normalized()
}

func (m *xdfileModel) adjustPanelSplit(deltaCols int) tea.Cmd {
	if m.width <= 1 {
		return nil
	}
	if m.layout.panelRects[0].w == 0 {
		m.computeLayout()
	}

	availableWidth := m.width - 1
	if availableWidth < xdfileMinPanelWidth*2 {
		return nil
	}

	currentLeft := m.layout.panelRects[0].w
	nextLeft := xdfileClamp(currentLeft+deltaCols, xdfileMinPanelWidth, availableWidth-xdfileMinPanelWidth)
	if nextLeft == currentLeft {
		return nil
	}

	m.layoutPrefs.PanelSplitPercent = xdfilePercentFromSize(nextLeft, availableWidth)
	m.computeLayout()
	m.syncTerminalViewport(false)

	panelPercent, terminalPercent := m.currentLayoutPercents()
	m.setStatus("Layout resized: left %d%%, terminal %d%%", panelPercent, terminalPercent)
	return nil
}

func (m *xdfileModel) adjustTerminalHeight(deltaRows int) tea.Cmd {
	if m.height <= xdfileHeaderHeight+xdfileFooterHeight {
		return nil
	}
	if m.layout.terminalRect.h == 0 {
		m.computeLayout()
	}

	availableHeight := m.height - xdfileHeaderHeight - xdfileFooterHeight
	maxTerminal := availableHeight - xdfileMinPanelBodyHeight
	if maxTerminal < xdfileMinTerminalHeight {
		return nil
	}

	currentTerminal := m.layout.terminalRect.h
	nextTerminal := xdfileClamp(currentTerminal+deltaRows, xdfileMinTerminalHeight, maxTerminal)
	if nextTerminal == currentTerminal {
		return nil
	}

	m.layoutPrefs.TerminalHeightPercent = xdfilePercentFromSize(nextTerminal, availableHeight)
	m.computeLayout()
	m.syncTerminalViewport(false)

	panelPercent, terminalPercent := m.currentLayoutPercents()
	m.setStatus("Layout resized: left %d%%, terminal %d%%", panelPercent, terminalPercent)
	return nil
}

func (m *xdfileModel) saveLayout() tea.Cmd {
	if m.layoutFile == "" {
		return nil
	}
	if m.layout.panelRects[0].w == 0 || m.layout.terminalRect.h == 0 {
		m.computeLayout()
	}

	m.captureCurrentLayoutPrefs()
	if err := xdfileSaveLayoutPrefsWithoutCommands(m.layoutFile, m.layoutPrefs); err != nil {
		m.setStatusErr(err)
		return nil
	}
	if err := m.saveCommandMenuPrefs(); err != nil {
		m.setStatusErr(err)
		return nil
	}

	panelPercent, terminalPercent := m.currentLayoutPercents()
	m.setStatus("Saved setup: left %d%%, terminal %d%%", panelPercent, terminalPercent)
	return nil
}

func (m *xdfileModel) resetSetup() tea.Cmd {
	defaults := xdfileDefaultLayoutPrefs()
	m.layoutPrefs = defaults.normalized()
	m.layoutPrefs.Commands = nil
	m.applyThemeByName(m.layoutPrefs.ThemeName)
	m.commandMenuPath = nil
	m.commandInsertPath = nil
	m.commandInsertIndex = 0
	m.commandEditPath = nil
	m.commandEditIndex = 0
	m.quickView.Open = false
	m.quickView.Path = ""
	m.quickView.Binary = false
	m.activePanel = 0
	m.terminalFocused = false
	m.terminalAutoFocused = false
	m.showHidden = m.layoutPrefs.ShowHidden

	for i := range m.panels {
		m.panels[i].clearMarked()
		if err := m.reloadPanel(i); err != nil {
			m.setStatusErr(err)
			return nil
		}
	}

	m.computeLayout()
	m.syncTerminalViewport(false)

	if m.layoutFile != "" {
		if err := xdfileSaveLayoutPrefsWithoutCommands(m.layoutFile, m.layoutPrefs); err != nil {
			m.setStatusErr(err)
			return nil
		}
	}
	if err := m.saveCommandMenuPrefs(); err != nil {
		m.setStatusErr(err)
		return nil
	}

	panelPercent, terminalPercent := m.currentLayoutPercents()
	m.setStatus("Reset setup: left %d%%, terminal %d%%", panelPercent, terminalPercent)
	return m.syncTerminalToPanel(m.activePanel)
}
