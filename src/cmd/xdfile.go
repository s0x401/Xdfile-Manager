package cmd

import (
	"time"

	"github.com/charmbracelet/lipgloss"
)

const (
	xdfileProductName = "Xdfile Manager"
	xdfileCopyright   = "Copyright (c) 2026 s0x401"

	xdfileMinWidth           = 92
	xdfileMinHeight          = 28
	xdfileMinPanelWidth      = 30
	xdfileMinPanelBodyHeight = 9
	xdfileMinTerminalHeight  = 6
	xdfilePanelSizeWidth     = 7
	xdfilePanelTimeWidth     = 11

	xdfileHeaderHeight = 2
	xdfileFooterHeight = 2

	xdfileANSIReset = "\x1b[0m"

	xdfileCompactPathCacheMax = 2048

	xdfileAutoRefreshInterval = 1500 * time.Millisecond

	xdfileActionHelp                   xdfileAction = "help"
	xdfileActionOpen                   xdfileAction = "open"
	xdfileActionCommandsMenu           xdfileAction = "commands_menu"
	xdfileActionSync                   xdfileAction = "sync"
	xdfileActionPreview                xdfileAction = "preview"
	xdfileActionProperties             xdfileAction = "properties"
	xdfileActionClipboardCopy          xdfileAction = "clipboard_copy"
	xdfileActionClipboardCut           xdfileAction = "clipboard_cut"
	xdfileActionPaste                  xdfileAction = "paste"
	xdfileActionPasteConflictPrompt    xdfileAction = "paste_conflict_prompt"
	xdfileActionPasteConflictOverwrite xdfileAction = "paste_conflict_overwrite"
	xdfileActionPasteConflictSkip      xdfileAction = "paste_conflict_skip"
	xdfileActionPasteConflictRename    xdfileAction = "paste_conflict_rename"
	xdfileActionPasteConflictApplyAll  xdfileAction = "paste_conflict_apply_all"
	xdfileActionRename                 xdfileAction = "rename"
	xdfileActionCopy                   xdfileAction = "copy"
	xdfileActionMove                   xdfileAction = "move"
	xdfileActionMkdir                  xdfileAction = "mkdir"
	xdfileActionDelete                 xdfileAction = "delete"
	xdfileActionUndoDelete             xdfileAction = "undo_delete"
	xdfileActionHidden                 xdfileAction = "hidden"
	xdfileActionQuit                   xdfileAction = "quit"
	xdfileActionRefresh                xdfileAction = "refresh"
	xdfileActionParent                 xdfileAction = "parent"
	xdfileActionTerminalExpand         xdfileAction = "terminal_expand"
	xdfileActionPanelsMenu             xdfileAction = "panels_menu"
	xdfileActionViewMenu               xdfileAction = "view_menu"
	xdfileActionTerminalMenu           xdfileAction = "terminal_menu"
	xdfileActionNetBoxMenu             xdfileAction = "netbox_menu"
	xdfileActionThemeMenu              xdfileAction = "theme_menu"
	xdfileActionOptionsMenu            xdfileAction = "options_menu"
	xdfileActionContextMenu            xdfileAction = "context_menu"
	xdfileActionSaveLayout             xdfileAction = "save_layout"
	xdfileActionResetLayout            xdfileAction = "reset_layout"
	xdfileActionPanelClickCmd          xdfileAction = "panel_click_cmd"
	xdfileActionQuickViewMode          xdfileAction = "quick_view_mode"
	xdfileActionThemePersona3          xdfileAction = "theme_persona3"
	xdfileActionThemePersona3Reload    xdfileAction = "theme_persona3_reload"
	xdfileActionThemePersona3Kotone    xdfileAction = "theme_persona3_kotone"
	xdfileActionThemePersona4          xdfileAction = "theme_persona4"
	xdfileActionThemePersona5          xdfileAction = "theme_persona5"
	xdfileActionSortName               xdfileAction = "sort_name"
	xdfileActionSortExt                xdfileAction = "sort_ext"
	xdfileActionModalRename            xdfileAction = "modal_rename"
	xdfileActionModalMkdir             xdfileAction = "modal_mkdir"
	xdfileActionInsertCommand          xdfileAction = "insert_command"
	xdfileActionInsertMenu             xdfileAction = "insert_menu"
	xdfileActionEditCommand            xdfileAction = "edit_command"
	xdfileActionEditMenu               xdfileAction = "edit_menu"
	xdfileActionCommandPrompt          xdfileAction = "command_prompt"
	xdfileActionNetBoxNew              xdfileAction = "netbox_new"
	xdfileActionNetBoxSave             xdfileAction = "netbox_save"
	xdfileActionNetBoxDelete           xdfileAction = "netbox_delete"
	xdfileActionNetBoxDisconnect       xdfileAction = "netbox_disconnect"
)

var (
	xdfileColorBG                  lipgloss.Color
	xdfileColorSurface             lipgloss.Color
	xdfileColorSurface2            lipgloss.Color
	xdfileColorBorder              lipgloss.Color
	xdfileColorAccent              lipgloss.Color
	xdfileColorAccent2             lipgloss.Color
	xdfileColorText                lipgloss.Color
	xdfileColorDim                 lipgloss.Color
	xdfileColorSuccess             lipgloss.Color
	xdfileColorDanger              lipgloss.Color
	xdfileColorHighlight           lipgloss.Color
	xdfileColorSelectionActiveBG   lipgloss.Color
	xdfileColorSelectionActiveFG   lipgloss.Color
	xdfileColorSelectionInactiveBG lipgloss.Color
	xdfileColorSelectionInactiveFG lipgloss.Color
	xdfileColorMarkedActiveBG      lipgloss.Color
	xdfileColorMarkedActiveFG      lipgloss.Color
	xdfileColorMarkedInactiveBG    lipgloss.Color
	xdfileColorMarkedInactiveFG    lipgloss.Color

	xdfileHeaderLineStyle               lipgloss.Style
	xdfileFooterLineStyle               lipgloss.Style
	xdfileStatusOKStyle                 lipgloss.Style
	xdfileStatusErrStyle                lipgloss.Style
	xdfileTagStyle                      lipgloss.Style
	xdfileTitleStyle                    lipgloss.Style
	xdfileDimStyle                      lipgloss.Style
	xdfilePathStyle                     lipgloss.Style
	xdfileTerminalPromptLabelStyle      lipgloss.Style
	xdfileTerminalPromptPathStyle       lipgloss.Style
	xdfileTerminalPromptCommandStyle    lipgloss.Style
	xdfileTerminalInputPromptStyle      lipgloss.Style
	xdfileTerminalInputTextStyle        lipgloss.Style
	xdfileTerminalInputCursorStyle      lipgloss.Style
	xdfileTerminalSuggestionStyle       lipgloss.Style
	xdfileTerminalSuggestionCursorStyle lipgloss.Style
	xdfileDirStyle                      lipgloss.Style
	xdfileFileStyle                     lipgloss.Style
	xdfileMetaStyle                     lipgloss.Style
	xdfileButtonKeyStyle                lipgloss.Style
	xdfileMenuButton                    lipgloss.Style
	xdfileMenuButtonHot                 lipgloss.Style
	xdfileMenuItemStyle                 lipgloss.Style
	xdfileMenuItemHot                   lipgloss.Style
	xdfileMenuItemKeyStyle              lipgloss.Style
	xdfileMenuItemDisabledStyle         lipgloss.Style
	xdfileModalTitleStyle               lipgloss.Style
	xdfileSelectedLineActiveStyle       lipgloss.Style
	xdfileSelectedLineInactiveStyle     lipgloss.Style
	xdfileInactiveCursorStyle           lipgloss.Style
	xdfileHoveredEntryLineCachedStyle   lipgloss.Style
	xdfileHoveredEntryMetaCachedStyle   lipgloss.Style
	xdfileHoveredMenuButtonCachedStyle  lipgloss.Style
	xdfileHoveredMenuItemCachedStyle    lipgloss.Style
	xdfileHoveredFooterKeyCachedStyle   lipgloss.Style
	xdfileHoveredFooterLabelCachedStyle lipgloss.Style
	xdfileMarkedLineActiveStyle         lipgloss.Style
	xdfileMarkedLineInactiveStyle       lipgloss.Style
	xdfilePanelBorderActiveStyle        lipgloss.Style
	xdfilePanelBorderInactiveStyle      lipgloss.Style
	xdfileTerminalBorderActiveStyle     lipgloss.Style
	xdfileTerminalBorderInactiveStyle   lipgloss.Style
	xdfileModalBorderStyle              lipgloss.Style
	xdfileMenuBorderStyle               lipgloss.Style

	xdfileFileColorStyles    map[lipgloss.Color]lipgloss.Style
	xdfileBlankStrings       map[int]string
	xdfileCompactPathCache   map[xdfileCompactPathKey]string
	xdfileEntryKindRenders   map[xdfileEntryKindSpec]string
	xdfileEntryKindOnRenders map[xdfileEntryKindRenderKey]string
)

type xdfileAction string

type xdfileButton struct {
	Action   xdfileAction
	Key      string
	Label    string
	Disabled bool
}

type xdfileMenu struct {
	Action xdfileAction
	Label  string
	Items  []xdfileButton
}

type xdfileRect struct {
	x int
	y int
	w int
	h int
}

func (r xdfileRect) contains(x int, y int) bool {
	return x >= r.x && x < r.x+r.w && y >= r.y && y < r.y+r.h
}

type xdfileButtonRect struct {
	Action   xdfileAction
	Rect     xdfileRect
	Disabled bool
}
