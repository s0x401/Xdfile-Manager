package cmd

import (
	"image/color"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	uv "github.com/charmbracelet/ultraviolet"
)

type xdfileTheme struct {
	Name                     string
	BG                       string
	Surface                  string
	Surface2                 string
	Border                   string
	Accent                   string
	Accent2                  string
	Text                     string
	Dim                      string
	Success                  string
	Danger                   string
	Highlight                string
	SelectionActiveBG        string
	SelectionActiveFG        string
	SelectionInactiveBG      string
	SelectionInactiveFG      string
	MarkedActiveBG           string
	MarkedActiveFG           string
	MarkedInactiveBG         string
	MarkedInactiveFG         string
	TerminalPromptPath       string
	TerminalInputCursor      string
	TerminalSuggestion       string
	TerminalSuggestionCursor string
	TerminalCursorForeground string
	TerminalCursorBackground string
}

const (
	xdfileThemePersona3Name       = "persona3"
	xdfileThemePersona3ReloadName = "persona3reload"
	xdfileThemePersona3KotoneName = "persona3kotone"
	xdfileThemePersona4Name       = "persona4"
	xdfileThemePersona5Name       = "persona5"
)

var xdfileCurrentTheme = xdfilePersona3Theme()

func init() {
	xdfileApplyTheme(xdfileCurrentTheme)
}

func xdfilePersona3Theme() xdfileTheme {
	return xdfileTheme{
		Name:                     xdfileThemePersona3Name,
		BG:                       "#071827",
		Surface:                  "#0D2236",
		Surface2:                 "#14314A",
		Border:                   "#377FB2",
		Accent:                   "#59D5FF",
		Accent2:                  "#F2FCFF",
		Text:                     "#F7FCFF",
		Dim:                      "#A7CBE6",
		Success:                  "#8CFAFF",
		Danger:                   "#FF9DB5",
		Highlight:                "#245A85",
		SelectionActiveBG:        "#58CFFF",
		SelectionActiveFG:        "#04131F",
		SelectionInactiveBG:      "#234B6E",
		SelectionInactiveFG:      "#F2FBFF",
		MarkedActiveBG:           "#2A628E",
		MarkedActiveFG:           "#F2FCFF",
		MarkedInactiveBG:         "#1D4567",
		MarkedInactiveFG:         "#B5E9FF",
		TerminalPromptPath:       "#DDF3FF",
		TerminalInputCursor:      "#B9F3FF",
		TerminalSuggestion:       "#6C95B2",
		TerminalSuggestionCursor: "#F2FCFF",
		TerminalCursorForeground: "#071827",
		TerminalCursorBackground: "#B9F3FF",
	}
}

func xdfilePersona3ReloadTheme() xdfileTheme {
	return xdfileTheme{
		Name:                     xdfileThemePersona3ReloadName,
		BG:                       "#0A1924",
		Surface:                  "#122838",
		Surface2:                 "#1B3D55",
		Border:                   "#62AEDD",
		Accent:                   "#92E7FF",
		Accent2:                  "#FFFFFF",
		Text:                     "#FBFDFF",
		Dim:                      "#B4D3E6",
		Success:                  "#A7F8FF",
		Danger:                   "#FFADC2",
		Highlight:                "#2E6C97",
		SelectionActiveBG:        "#9AE8FF",
		SelectionActiveFG:        "#071722",
		SelectionInactiveBG:      "#34698C",
		SelectionInactiveFG:      "#FBFDFF",
		MarkedActiveBG:           "#34719B",
		MarkedActiveFG:           "#FFFFFF",
		MarkedInactiveBG:         "#255573",
		MarkedInactiveFG:         "#C3EEFF",
		TerminalPromptPath:       "#E7F7FF",
		TerminalInputCursor:      "#C8F7FF",
		TerminalSuggestion:       "#7DA3BC",
		TerminalSuggestionCursor: "#FFFFFF",
		TerminalCursorForeground: "#08151F",
		TerminalCursorBackground: "#C8F7FF",
	}
}

func xdfilePersona3KotoneTheme() xdfileTheme {
	return xdfileTheme{
		Name:                     xdfileThemePersona3KotoneName,
		BG:                       "#12101D",
		Surface:                  "#1D1528",
		Surface2:                 "#2D1D37",
		Border:                   "#7661A7",
		Accent:                   "#FF6FAE",
		Accent2:                  "#FFE8F4",
		Text:                     "#FFF7FB",
		Dim:                      "#D8B7CB",
		Success:                  "#FFC6E0",
		Danger:                   "#FF8C8C",
		Highlight:                "#4B274A",
		SelectionActiveBG:        "#FF7AB8",
		SelectionActiveFG:        "#1B0C17",
		SelectionInactiveBG:      "#6F3A5F",
		SelectionInactiveFG:      "#FFEAF4",
		MarkedActiveBG:           "#9D3F73",
		MarkedActiveFG:           "#FFF4FA",
		MarkedInactiveBG:         "#5B2B50",
		MarkedInactiveFG:         "#F4C7DC",
		TerminalPromptPath:       "#FFD2E5",
		TerminalInputCursor:      "#FF9BCB",
		TerminalSuggestion:       "#C28BAA",
		TerminalSuggestionCursor: "#FFF1F7",
		TerminalCursorForeground: "#1A0C16",
		TerminalCursorBackground: "#FF9BCB",
	}
}

func xdfilePersona4Theme() xdfileTheme {
	return xdfileTheme{
		Name:                     xdfileThemePersona4Name,
		BG:                       "#11110A",
		Surface:                  "#1D1B0F",
		Surface2:                 "#2F2A12",
		Border:                   "#C8A728",
		Accent:                   "#FFD84A",
		Accent2:                  "#FFF7B8",
		Text:                     "#FFF9D8",
		Dim:                      "#D7C776",
		Success:                  "#F5F06A",
		Danger:                   "#FF8A6A",
		Highlight:                "#4C4117",
		SelectionActiveBG:        "#FFD94D",
		SelectionActiveFG:        "#181407",
		SelectionInactiveBG:      "#705A18",
		SelectionInactiveFG:      "#FFF6C8",
		MarkedActiveBG:           "#765A12",
		MarkedActiveFG:           "#FFF7D0",
		MarkedInactiveBG:         "#574815",
		MarkedInactiveFG:         "#F2DD7A",
		TerminalPromptPath:       "#FFF3A6",
		TerminalInputCursor:      "#FFE66D",
		TerminalSuggestion:       "#C8A94F",
		TerminalSuggestionCursor: "#FFF7B8",
		TerminalCursorForeground: "#171307",
		TerminalCursorBackground: "#FFE66D",
	}
}

func xdfilePersona5Theme() xdfileTheme {
	return xdfileTheme{
		Name:                     xdfileThemePersona5Name,
		BG:                       "#08090D",
		Surface:                  "#121419",
		Surface2:                 "#1B1F26",
		Border:                   "#C52731",
		Accent:                   "#E63B45",
		Accent2:                  "#FFF4F1",
		Text:                     "#FFF7F3",
		Dim:                      "#C8B8B2",
		Success:                  "#FFB8BE",
		Danger:                   "#FF7A86",
		Highlight:                "#6E111A",
		SelectionActiveBG:        "#F5EDE8",
		SelectionActiveFG:        "#111216",
		SelectionInactiveBG:      "#7B1A24",
		SelectionInactiveFG:      "#FFF7F3",
		MarkedActiveBG:           "#A51F2A",
		MarkedActiveFG:           "#FFF8F5",
		MarkedInactiveBG:         "#521118",
		MarkedInactiveFG:         "#F6DFD9",
		TerminalPromptPath:       "#FFE3DD",
		TerminalInputCursor:      "#FFF4F1",
		TerminalSuggestion:       "#C48D92",
		TerminalSuggestionCursor: "#FFF7F3",
		TerminalCursorForeground: "#090A0D",
		TerminalCursorBackground: "#FFF4F1",
	}
}

func xdfileThemeByName(name string) xdfileTheme {
	switch xdfileNormalizeThemeName(name) {
	case xdfileThemePersona5Name:
		return xdfilePersona5Theme()
	case xdfileThemePersona4Name:
		return xdfilePersona4Theme()
	case xdfileThemePersona3KotoneName:
		return xdfilePersona3KotoneTheme()
	case xdfileThemePersona3ReloadName:
		return xdfilePersona3ReloadTheme()
	default:
		return xdfilePersona3Theme()
	}
}

func xdfileNormalizeThemeName(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case xdfileThemePersona5Name, "persona-5", "p5":
		return xdfileThemePersona5Name
	case xdfileThemePersona4Name, "persona-4", "p4":
		return xdfileThemePersona4Name
	case xdfileThemePersona3ReloadName, "persona3-reload", "p3r":
		return xdfileThemePersona3ReloadName
	case xdfileThemePersona3KotoneName, "persona3-kotone", "p3p", "kotone", "shiomi":
		return xdfileThemePersona3KotoneName
	default:
		return xdfileThemePersona3Name
	}
}

func xdfileThemeDisplayName(name string) string {
	switch xdfileNormalizeThemeName(name) {
	case xdfileThemePersona5Name:
		return "Persona 5"
	case xdfileThemePersona4Name:
		return "Persona 4"
	case xdfileThemePersona3KotoneName:
		return "Persona 3 Kotone"
	case xdfileThemePersona3ReloadName:
		return "Persona 3 Reload"
	default:
		return "Persona 3"
	}
}

func xdfileThemeMenuLabel(name string, current string) string {
	label := xdfileThemeDisplayName(name)
	if xdfileNormalizeThemeName(name) == xdfileNormalizeThemeName(current) {
		return label + " (Current)"
	}
	return label
}

func xdfileApplyTheme(theme xdfileTheme) {
	xdfileCurrentTheme = theme

	xdfileColorBG = lipgloss.Color(theme.BG)
	xdfileColorSurface = lipgloss.Color(theme.Surface)
	xdfileColorSurface2 = lipgloss.Color(theme.Surface2)
	xdfileColorBorder = lipgloss.Color(theme.Border)
	xdfileColorAccent = lipgloss.Color(theme.Accent)
	xdfileColorAccent2 = lipgloss.Color(theme.Accent2)
	xdfileColorText = lipgloss.Color(theme.Text)
	xdfileColorDim = lipgloss.Color(theme.Dim)
	xdfileColorSuccess = lipgloss.Color(theme.Success)
	xdfileColorDanger = lipgloss.Color(theme.Danger)
	xdfileColorHighlight = lipgloss.Color(theme.Highlight)
	xdfileColorSelectionActiveBG = lipgloss.Color(theme.SelectionActiveBG)
	xdfileColorSelectionActiveFG = lipgloss.Color(theme.SelectionActiveFG)
	xdfileColorSelectionInactiveBG = lipgloss.Color(theme.SelectionInactiveBG)
	xdfileColorSelectionInactiveFG = lipgloss.Color(theme.SelectionInactiveFG)
	xdfileColorMarkedActiveBG = lipgloss.Color(theme.MarkedActiveBG)
	xdfileColorMarkedActiveFG = lipgloss.Color(theme.MarkedActiveFG)
	xdfileColorMarkedInactiveBG = lipgloss.Color(theme.MarkedInactiveBG)
	xdfileColorMarkedInactiveFG = lipgloss.Color(theme.MarkedInactiveFG)

	xdfileHeaderLineStyle = lipgloss.NewStyle().Foreground(xdfileColorText)
	xdfileFooterLineStyle = lipgloss.NewStyle().Foreground(xdfileColorText)
	xdfileStatusOKStyle = lipgloss.NewStyle().Foreground(xdfileColorSuccess)
	xdfileStatusErrStyle = lipgloss.NewStyle().Foreground(xdfileColorDanger)
	xdfileTagStyle = lipgloss.NewStyle().Foreground(xdfileColorAccent)
	xdfileTitleStyle = lipgloss.NewStyle().Foreground(xdfileColorAccent2)
	xdfileDimStyle = lipgloss.NewStyle().Foreground(xdfileColorDim)
	xdfilePathStyle = lipgloss.NewStyle().Foreground(xdfileColorText)
	xdfileTerminalPromptLabelStyle = lipgloss.NewStyle().Foreground(xdfileColorAccent).Bold(true)
	xdfileTerminalPromptPathStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.TerminalPromptPath))
	xdfileTerminalPromptCommandStyle = lipgloss.NewStyle().Foreground(xdfileColorAccent2)
	xdfileTerminalInputPromptStyle = lipgloss.NewStyle().Foreground(xdfileColorAccent2).Bold(true)
	xdfileTerminalInputTextStyle = lipgloss.NewStyle().Foreground(xdfileColorText)
	xdfileTerminalInputCursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.TerminalInputCursor)).Bold(true)
	xdfileTerminalSuggestionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.TerminalSuggestion)).Italic(true)
	xdfileTerminalSuggestionCursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.TerminalSuggestionCursor)).Bold(true).Underline(true)
	xdfileTerminalSuggestionUVStyle = uv.Style{
		Fg:    xdfileHexRGBA(theme.TerminalSuggestion),
		Attrs: uv.AttrItalic,
	}
	xdfileTerminalSuggestionCursorUVStyle = uv.Style{
		Fg:        xdfileHexRGBA(theme.TerminalSuggestionCursor),
		Underline: uv.UnderlineSingle,
		Attrs:     uv.AttrBold,
	}
	xdfileTerminalCursorUVStyle = uv.Style{
		Fg:        xdfileHexRGBA(theme.TerminalCursorForeground),
		Bg:        xdfileHexRGBA(theme.TerminalCursorBackground),
		Underline: uv.UnderlineSingle,
		Attrs:     uv.AttrBold,
	}
	xdfileDirStyle = lipgloss.NewStyle().Foreground(xdfileColorAccent)
	xdfileFileStyle = lipgloss.NewStyle().Foreground(xdfileColorText)
	xdfileMetaStyle = lipgloss.NewStyle().Foreground(xdfileColorDim)
	xdfileButtonKeyStyle = lipgloss.NewStyle().Foreground(xdfileColorAccent2)
	xdfileMenuButton = lipgloss.NewStyle().Foreground(xdfileColorAccent2).Padding(0, 1)
	xdfileMenuButtonHot = lipgloss.NewStyle().
		Foreground(xdfileColorSelectionActiveFG).
		Background(xdfileColorSelectionActiveBG).
		Padding(0, 1).
		Bold(true)
	xdfileMenuItemStyle = lipgloss.NewStyle().
		Foreground(xdfileColorText)
	xdfileMenuItemHot = lipgloss.NewStyle().
		Foreground(xdfileColorSelectionActiveFG).
		Background(xdfileColorSelectionActiveBG).
		Bold(true)
	xdfileMenuItemKeyStyle = lipgloss.NewStyle().
		Foreground(xdfileColorAccent)
	xdfileMenuItemDisabledStyle = lipgloss.NewStyle().
		Foreground(xdfileColorDim)
	xdfileModalTitleStyle = lipgloss.NewStyle().Foreground(xdfileColorAccent)

	xdfileSelectedLineActiveStyle = lipgloss.NewStyle().
		Foreground(xdfileColorSelectionActiveFG).
		Background(xdfileColorSelectionActiveBG).
		Bold(true)
	xdfileSelectedLineInactiveStyle = lipgloss.NewStyle().
		Foreground(xdfileColorSelectionInactiveFG).
		Background(xdfileColorSelectionInactiveBG)
	xdfileInactiveCursorStyle = lipgloss.NewStyle().
		Foreground(xdfileColorSelectionInactiveFG).
		Bold(true)
	xdfileHoveredEntryLineCachedStyle = lipgloss.NewStyle().
		Foreground(xdfileColorText).
		Background(xdfileColorHighlight)
	xdfileHoveredEntryMetaCachedStyle = lipgloss.NewStyle().
		Foreground(xdfileColorDim).
		Background(xdfileColorHighlight)
	xdfileHoveredMenuButtonCachedStyle = lipgloss.NewStyle().
		Foreground(xdfileColorText).
		Background(xdfileColorHighlight).
		Padding(0, 1)
	xdfileHoveredMenuItemCachedStyle = lipgloss.NewStyle().
		Foreground(xdfileColorText).
		Background(xdfileColorHighlight)
	xdfileHoveredFooterKeyCachedStyle = lipgloss.NewStyle().
		Foreground(xdfileColorAccent2).
		Background(xdfileColorHighlight)
	xdfileHoveredFooterLabelCachedStyle = lipgloss.NewStyle().
		Foreground(xdfileColorDim).
		Background(xdfileColorHighlight)
	xdfileMarkedLineActiveStyle = lipgloss.NewStyle().
		Foreground(xdfileColorMarkedActiveFG).
		Background(xdfileColorMarkedActiveBG).
		Bold(true)
	xdfileMarkedLineInactiveStyle = lipgloss.NewStyle().
		Foreground(xdfileColorMarkedInactiveFG).
		Background(xdfileColorMarkedInactiveBG)
	heavyBorder := xdfileRoundedHeavyBorder()
	xdfilePanelBorderActiveStyle = lipgloss.NewStyle().
		Foreground(xdfileColorText).
		Border(heavyBorder).
		BorderForeground(xdfileColorAccent).
		Padding(0, 0)
	xdfilePanelBorderInactiveStyle = lipgloss.NewStyle().
		Foreground(xdfileColorText).
		Border(heavyBorder).
		BorderForeground(xdfileColorBorder).
		Padding(0, 0)
	xdfileTerminalBorderActiveStyle = lipgloss.NewStyle().
		Foreground(xdfileColorText).
		Border(heavyBorder).
		BorderForeground(xdfileColorAccent2).
		Padding(0, 0)
	xdfileTerminalBorderInactiveStyle = lipgloss.NewStyle().
		Foreground(xdfileColorText).
		Border(heavyBorder).
		BorderForeground(xdfileColorBorder).
		Padding(0, 0)
	xdfileModalBorderStyle = lipgloss.NewStyle().
		Foreground(xdfileColorText).
		Border(lipgloss.ThickBorder()).
		BorderForeground(xdfileColorAccent2)
	xdfileMenuBorderStyle = lipgloss.NewStyle().
		Foreground(xdfileColorText).
		Border(lipgloss.NormalBorder()).
		BorderForeground(xdfileColorBorder)
	xdfileFileColorStyles = make(map[lipgloss.Color]lipgloss.Style, 64)
	xdfileBlankStrings = map[int]string{0: ""}
	xdfileCompactPathCache = make(map[xdfileCompactPathKey]string, xdfileCompactPathCacheMax)
	xdfileEntryKindRenders = make(map[xdfileEntryKindSpec]string, 32)
	xdfileEntryKindOnRenders = make(map[xdfileEntryKindRenderKey]string, 8)
}

func xdfileHexRGBA(value string) color.RGBA {
	value = strings.TrimPrefix(strings.TrimSpace(value), "#")
	if len(value) != 6 {
		return color.RGBA{A: 0xff}
	}

	r, errR := strconv.ParseUint(value[0:2], 16, 8)
	g, errG := strconv.ParseUint(value[2:4], 16, 8)
	b, errB := strconv.ParseUint(value[4:6], 16, 8)
	if errR != nil || errG != nil || errB != nil {
		return color.RGBA{A: 0xff}
	}
	return color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 0xff}
}
