package cmd

import (
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
)

func TestXdfilePersona3ThemeIsDefault(t *testing.T) {
	if xdfileCurrentTheme.Name != "persona3" {
		t.Fatalf("expected persona3 theme to be the active default, got %q", xdfileCurrentTheme.Name)
	}
	if string(xdfileColorAccent) != "#59D5FF" {
		t.Fatalf("expected persona3 accent color to be applied, got %q", xdfileColorAccent)
	}
	if string(xdfileColorAccent2) != "#F2FCFF" {
		t.Fatalf("expected persona3 title accent color to be applied, got %q", xdfileColorAccent2)
	}
	if string(xdfileColorSelectionActiveBG) != "#58CFFF" {
		t.Fatalf("expected persona3 selected-line color to be applied, got %q", xdfileColorSelectionActiveBG)
	}
	if string(xdfileColorMarkedActiveBG) != "#2A628E" {
		t.Fatalf("expected persona3 marked-line color to be applied, got %q", xdfileColorMarkedActiveBG)
	}
}

func TestXdfileThemeActionSwitchesToPersona3Reload(t *testing.T) {
	original := xdfileCurrentTheme.Name
	defer xdfileApplyTheme(xdfileThemeByName(original))

	m := &xdfileModel{
		layoutPrefs: xdfileLayoutPrefs{ThemeName: xdfileThemePersona3Name},
		terminal: xdfileTerminal{
			Input: textinput.New(),
		},
		modal: xdfileModal{
			Input: textinput.New(),
		},
	}

	if cmd := m.executeAction(xdfileActionThemePersona3Reload); cmd != nil {
		t.Fatalf("expected theme switch action to complete synchronously")
	}
	if m.layoutPrefs.ThemeName != xdfileThemePersona3ReloadName {
		t.Fatalf("expected theme setting to switch to %q, got %q", xdfileThemePersona3ReloadName, m.layoutPrefs.ThemeName)
	}
	if xdfileCurrentTheme.Name != xdfileThemePersona3ReloadName {
		t.Fatalf("expected active theme to switch to %q, got %q", xdfileThemePersona3ReloadName, xdfileCurrentTheme.Name)
	}
	if got := m.terminal.Input.Prompt; got != "XD> " {
		t.Fatalf("expected terminal prompt to be resynced after theme switch, got %q", got)
	}
}

func TestXdfileThemeActionSwitchesToPersona5(t *testing.T) {
	original := xdfileCurrentTheme.Name
	defer xdfileApplyTheme(xdfileThemeByName(original))

	m := &xdfileModel{
		layoutPrefs: xdfileLayoutPrefs{ThemeName: xdfileThemePersona3Name},
		terminal: xdfileTerminal{
			Input: textinput.New(),
		},
		modal: xdfileModal{
			Input: textinput.New(),
		},
	}

	if cmd := m.executeAction(xdfileActionThemePersona5); cmd != nil {
		t.Fatalf("expected theme switch action to complete synchronously")
	}
	if m.layoutPrefs.ThemeName != xdfileThemePersona5Name {
		t.Fatalf("expected theme setting to switch to %q, got %q", xdfileThemePersona5Name, m.layoutPrefs.ThemeName)
	}
	if xdfileCurrentTheme.Name != xdfileThemePersona5Name {
		t.Fatalf("expected active theme to switch to %q, got %q", xdfileThemePersona5Name, xdfileCurrentTheme.Name)
	}
	if string(xdfileColorAccent) != "#E63B45" {
		t.Fatalf("expected persona5 accent color to be applied, got %q", xdfileColorAccent)
	}
	if string(xdfileColorSuccess) != "#FFB8BE" {
		t.Fatalf("expected persona5 status info color to stay in red-white palette, got %q", xdfileColorSuccess)
	}
}

func TestXdfileThemeActionSwitchesToPersona3Kotone(t *testing.T) {
	original := xdfileCurrentTheme.Name
	defer xdfileApplyTheme(xdfileThemeByName(original))

	m := &xdfileModel{
		layoutPrefs: xdfileLayoutPrefs{ThemeName: xdfileThemePersona3Name},
		terminal: xdfileTerminal{
			Input: textinput.New(),
		},
		modal: xdfileModal{
			Input: textinput.New(),
		},
	}

	if cmd := m.executeAction(xdfileActionThemePersona3Kotone); cmd != nil {
		t.Fatalf("expected theme switch action to complete synchronously")
	}
	if m.layoutPrefs.ThemeName != xdfileThemePersona3KotoneName {
		t.Fatalf("expected theme setting to switch to %q, got %q", xdfileThemePersona3KotoneName, m.layoutPrefs.ThemeName)
	}
	if xdfileCurrentTheme.Name != xdfileThemePersona3KotoneName {
		t.Fatalf("expected active theme to switch to %q, got %q", xdfileThemePersona3KotoneName, xdfileCurrentTheme.Name)
	}
	if string(xdfileColorAccent) != "#FF6FAE" {
		t.Fatalf("expected persona3 kotone accent color to be applied, got %q", xdfileColorAccent)
	}
	if string(xdfileColorSelectionActiveBG) != "#FF7AB8" {
		t.Fatalf("expected persona3 kotone selected-line color to be applied, got %q", xdfileColorSelectionActiveBG)
	}
	if string(xdfileColorSuccess) != "#FFC6E0" {
		t.Fatalf("expected persona3 kotone status info color to stay in pink palette, got %q", xdfileColorSuccess)
	}
}

func TestXdfileThemeActionSwitchesToPersona4(t *testing.T) {
	original := xdfileCurrentTheme.Name
	defer xdfileApplyTheme(xdfileThemeByName(original))

	m := &xdfileModel{
		layoutPrefs: xdfileLayoutPrefs{ThemeName: xdfileThemePersona3Name},
		terminal: xdfileTerminal{
			Input: textinput.New(),
		},
		modal: xdfileModal{
			Input: textinput.New(),
		},
	}

	if cmd := m.executeAction(xdfileActionThemePersona4); cmd != nil {
		t.Fatalf("expected theme switch action to complete synchronously")
	}
	if m.layoutPrefs.ThemeName != xdfileThemePersona4Name {
		t.Fatalf("expected theme setting to switch to %q, got %q", xdfileThemePersona4Name, m.layoutPrefs.ThemeName)
	}
	if xdfileCurrentTheme.Name != xdfileThemePersona4Name {
		t.Fatalf("expected active theme to switch to %q, got %q", xdfileThemePersona4Name, xdfileCurrentTheme.Name)
	}
	if string(xdfileColorAccent) != "#FFD84A" {
		t.Fatalf("expected persona4 accent color to be applied, got %q", xdfileColorAccent)
	}
	if string(xdfileColorSelectionActiveFG) != "#181407" {
		t.Fatalf("expected persona4 selected text color to be applied, got %q", xdfileColorSelectionActiveFG)
	}
}

func TestXdfileThemeMenuLabelMarksCurrentTheme(t *testing.T) {
	label := xdfileThemeMenuLabel(xdfileThemePersona3ReloadName, xdfileThemePersona3ReloadName)
	if label != "Persona 3 Reload (Current)" {
		t.Fatalf("expected current theme label, got %q", label)
	}
}

func TestXdfileNormalizeThemeNameSupportsPersona5Aliases(t *testing.T) {
	for _, input := range []string{"persona5", "persona-5", "p5"} {
		if got := xdfileNormalizeThemeName(input); got != xdfileThemePersona5Name {
			t.Fatalf("expected alias %q to normalize to %q, got %q", input, xdfileThemePersona5Name, got)
		}
	}
}

func TestXdfileNormalizeThemeNameSupportsPersona4Aliases(t *testing.T) {
	for _, input := range []string{"persona4", "persona-4", "p4"} {
		if got := xdfileNormalizeThemeName(input); got != xdfileThemePersona4Name {
			t.Fatalf("expected alias %q to normalize to %q, got %q", input, xdfileThemePersona4Name, got)
		}
	}
}

func TestXdfileNormalizeThemeNameSupportsPersona3KotoneAliases(t *testing.T) {
	for _, input := range []string{"persona3kotone", "persona3-kotone", "p3p", "kotone", "shiomi"} {
		if got := xdfileNormalizeThemeName(input); got != xdfileThemePersona3KotoneName {
			t.Fatalf("expected alias %q to normalize to %q, got %q", input, xdfileThemePersona3KotoneName, got)
		}
	}
}

func TestXdfileMenuDefinitionsExposeStandaloneThemeMenu(t *testing.T) {
	m := &xdfileModel{
		layoutPrefs: xdfileLayoutPrefs{ThemeName: xdfileThemePersona3Name},
	}

	menus := m.menuDefinitions()
	themeIndex := -1
	optionsIndex := -1
	for i, menu := range menus {
		if menu.Action == xdfileActionThemeMenu {
			themeIndex = i
			if len(menu.Items) != 5 {
				t.Fatalf("expected standalone theme menu to expose five theme choices, got %d", len(menu.Items))
			}
		}
		if menu.Action == xdfileActionOptionsMenu {
			optionsIndex = i
			for _, item := range menu.Items {
				if item.Action == xdfileActionThemePersona3 ||
					item.Action == xdfileActionThemePersona3Reload ||
					item.Action == xdfileActionThemePersona3Kotone ||
					item.Action == xdfileActionThemePersona4 ||
					item.Action == xdfileActionThemePersona5 {
					t.Fatalf("expected theme actions to move out of Options, got %+v", item)
				}
			}
		}
	}

	if themeIndex < 0 {
		t.Fatal("expected a standalone Theme menu")
	}
	if optionsIndex < 0 {
		t.Fatal("expected an Options menu")
	}
	if themeIndex >= optionsIndex {
		t.Fatalf("expected Theme menu to appear before Options, got theme=%d options=%d", themeIndex, optionsIndex)
	}
}

func TestXdfileBuiltInThemeContrastPairs(t *testing.T) {
	themes := []xdfileTheme{
		xdfilePersona3Theme(),
		xdfilePersona3ReloadTheme(),
		xdfilePersona3KotoneTheme(),
		xdfilePersona4Theme(),
		xdfilePersona5Theme(),
	}
	checks := []struct {
		name string
		fg   func(xdfileTheme) string
		bg   func(xdfileTheme) string
	}{
		{name: "text on background", fg: func(t xdfileTheme) string { return t.Text }, bg: func(t xdfileTheme) string { return t.BG }},
		{name: "dim on background", fg: func(t xdfileTheme) string { return t.Dim }, bg: func(t xdfileTheme) string { return t.BG }},
		{name: "accent on background", fg: func(t xdfileTheme) string { return t.Accent }, bg: func(t xdfileTheme) string { return t.BG }},
		{name: "active selection", fg: func(t xdfileTheme) string { return t.SelectionActiveFG }, bg: func(t xdfileTheme) string { return t.SelectionActiveBG }},
		{name: "inactive selection", fg: func(t xdfileTheme) string { return t.SelectionInactiveFG }, bg: func(t xdfileTheme) string { return t.SelectionInactiveBG }},
		{name: "active mark", fg: func(t xdfileTheme) string { return t.MarkedActiveFG }, bg: func(t xdfileTheme) string { return t.MarkedActiveBG }},
		{name: "inactive mark", fg: func(t xdfileTheme) string { return t.MarkedInactiveFG }, bg: func(t xdfileTheme) string { return t.MarkedInactiveBG }},
		{name: "terminal cursor", fg: func(t xdfileTheme) string { return t.TerminalCursorForeground }, bg: func(t xdfileTheme) string { return t.TerminalCursorBackground }},
	}

	for _, theme := range themes {
		for _, check := range checks {
			if ratio := xdfileThemeContrastRatio(check.fg(theme), check.bg(theme)); ratio < 4.5 {
				t.Fatalf("%s %s contrast %.2f is too low", theme.Name, check.name, ratio)
			}
		}
	}
}

func xdfileThemeContrastRatio(foreground string, background string) float64 {
	fg := xdfileThemeRelativeLuminance(foreground)
	bg := xdfileThemeRelativeLuminance(background)
	light := math.Max(fg, bg)
	dark := math.Min(fg, bg)
	return (light + 0.05) / (dark + 0.05)
}

func xdfileThemeRelativeLuminance(value string) float64 {
	value = strings.TrimPrefix(strings.TrimSpace(value), "#")
	if len(value) != 6 {
		return 0
	}
	r := xdfileThemeLinearChannel(value[0:2])
	g := xdfileThemeLinearChannel(value[2:4])
	b := xdfileThemeLinearChannel(value[4:6])
	return 0.2126*r + 0.7152*g + 0.0722*b
}

func xdfileThemeLinearChannel(value string) float64 {
	raw, err := strconv.ParseUint(value, 16, 8)
	if err != nil {
		return 0
	}
	channel := float64(raw) / 255
	if channel <= 0.03928 {
		return channel / 12.92
	}
	return math.Pow((channel+0.055)/1.055, 2.4)
}
