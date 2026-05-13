package cmd

import (
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	vt "github.com/charmbracelet/x/vt"

	filepreview "github.com/s0x401/xdfile-manager/src/pkg/file_preview"
)

type xdfileTerminal struct {
	Cwd            string
	Title          string
	Busy           bool
	Input          textinput.Model
	Viewport       viewport.Model
	Lines          []string
	Events         chan tea.Msg
	Session        *xdfileTerminalPTYSession
	Emulator       *vt.SafeEmulator
	StreamEmulator *vt.SafeEmulator
	ManagedCancel  func()

	Suggestions         []string
	SuggestionCursor    int
	SuggestionInput     string
	SuggestionDismissed bool

	ScrollOffset    int
	ViewWidth       int
	ViewHeight      int
	ViewportContent string

	History          []string
	HistoryIndex     int
	HistoryDraft     string
	CommandHadOutput bool
	PendingPanel     int
	PendingCwd       string
	PendingPolls     int

	StartupSubmitPending bool
	StreamCanRewrite     bool

	Exclusive xdfileExclusiveTerminal
}

type xdfileLayout struct {
	menuButtons             []xdfileButtonRect
	footerButtons           []xdfileButtonRect
	menuItemRects           []xdfileButtonRect
	menuRect                xdfileRect
	panelRects              [2]xdfileRect
	terminalRect            xdfileRect
	terminalInputRect       xdfileRect
	terminalSuggestionRects []xdfileButtonRect
	exclusiveRect           xdfileRect
}

type xdfileClickState struct {
	panel int
	row   int
	at    time.Time
}

type xdfileHoverState struct {
	MenuAction   xdfileAction
	MenuItem     int
	FooterAction xdfileAction
	Panel        int
	PanelIndex   int
}

type xdfilePanelSearchState struct {
	Active  bool
	Panel   int
	Pattern string
}

type xdfileTerminalFocusState struct {
	Focused     bool
	AutoFocused bool
}

type xdfileTerminalResultMsg struct {
	Command         string
	Output          string
	Err             error
	Dir             string
	Clear           bool
	SyncActivePanel bool
}

type xdfileTerminalConsoleDoneMsg struct {
	Command string
	Dir     string
	Err     error
}

type xdfileFooterCtrlHintExpiredMsg struct {
	At time.Time
}

type xdfileStatusSpinnerTickMsg struct {
	At time.Time
}

type xdfileAutoRefreshMsg struct{}
type xdfileReturnFromUserScreenMsg struct{}

type xdfilePreviewContent struct {
	Text        string
	Description string
	Visual      bool
}

type xdfileTerminalLineMsg struct {
	Line     string
	Rewrite  bool
	Finalize bool
}

type xdfileTerminalCommandDoneMsg struct {
	Cwd      string
	Err      error
	Canceled bool
}

type xdfileTerminalCommandPollMsg struct{}

type xdfileTerminalExitMsg struct {
	Err error
}

type xdfileTerminalCommandStartMsg struct {
	Command  string
	Dir      string
	Events   chan tea.Msg
	Cancel   func()
	Emulator *vt.SafeEmulator
}

type xdfileTerminalStreamScreenMsg struct{}

type xdfileTerminalStartResultMsg struct {
	Session *xdfileTerminalPTYSession
	Err     error
	Dir     string
}

type xdfileExclusiveTerminal struct {
	Command string
	Cwd     string
	Title   string
	Events  chan tea.Msg
	Session *xdfileTerminalPTYSession
}

type xdfileExclusiveTerminalStartMsg struct {
	Command string
	Dir     string
	Session *xdfileTerminalPTYSession
	Err     error
}

type xdfileExclusiveTerminalScreenMsg struct{}

type xdfileExclusiveTerminalTitleMsg struct {
	Title string
}

type xdfileExclusiveTerminalExitMsg struct {
	Err error
}

type xdfileClipboardWriteResultMsg struct {
	Err error
}

type xdfileRemoteClipboardCopyResultMsg struct {
	Paths        []string
	CacheDir     string
	Names        []string
	Err          error
	ClipboardErr error
}

type xdfileRemoteClipboardPasteDoneMsg struct {
	Pending    *xdfilePendingClipboardPaste
	TargetPath string
	TopLevel   bool
	Err        error
}

type xdfileLocalClipboardPasteDoneMsg struct {
	Pending      *xdfilePendingClipboardPaste
	SourcePath   string
	TargetPath   string
	TopLevel     bool
	Action       xdfileAction
	ReplacedPath string
	Err          error
}

type xdfilePanelDirState struct {
	Path    string
	ModTime time.Time
	Exists  bool
}

type xdfileDeleteUndoItem struct {
	OriginalPath string
	StagedPath   string
}

type xdfileDeleteUndoBatch struct {
	Root  string
	Items []xdfileDeleteUndoItem
	At    time.Time
}

type xdfileClipboardMoveUndoItem struct {
	OriginalPath string
	MovedPath    string
	ReplacedPath string
}

type xdfileClipboardMoveUndoBatch struct {
	Root  string
	Items []xdfileClipboardMoveUndoItem
	At    time.Time
}

type xdfilePendingClipboardPaste struct {
	Sources          []string
	CutMode          bool
	DestinationDir   string
	Queue            []xdfilePendingClipboardPasteItem
	ConflictSource   string
	ConflictTarget   string
	ConflictTopLevel bool
	ConflictApplyAll bool
	ConflictPolicy   xdfileAction
	Targets          []string
	RemainingSources []string
	Skipped          int
	Overwritten      int
	Renamed          int
	LastTarget       string
	FocusTarget      string
	MoveUndoRoot     string
	MoveUndoItems    []xdfileClipboardMoveUndoItem
}

type xdfilePendingClipboardPasteItem struct {
	SourcePath string
	TargetPath string
	TopLevel   bool
	CleanupDir bool
}

type xdfileModel struct {
	width  int
	height int

	activePanel         int
	terminalFocused     bool
	terminalAutoFocused bool
	terminalExpanded    bool
	terminalReturnFocus xdfileTerminalFocusState
	userScreenVisible   bool
	showHidden          bool

	panels             [2]xdfilePanel
	panelDirState      [2]xdfilePanelDirState
	terminal           xdfileTerminal
	modal              xdfileModal
	imagePreviewer     *filepreview.ImagePreviewer
	thumbnailGenerator *filepreview.ThumbnailGenerator
	layout             xdfileLayout
	layoutPrefs        xdfileLayoutPrefs
	layoutFile         string
	commandsFile       string
	netboxFile         string
	netboxConnections  []xdfileNetBoxConnection
	quickView          xdfileQuickView

	statusText             string
	statusError            bool
	statusSpinnerIndex     int
	backgroundTaskBusy     bool
	lastClick              xdfileClickState
	clipboardPath          string
	clipboardPaths         []string
	clipboardCut           bool
	openMenu               xdfileAction
	menuCursor             int
	contextMenu            xdfileMenu
	contextMenuAnchor      xdfileRect
	footerCtrlHintUntil    time.Time
	commandMenuPath        []int
	commandInsertPath      []int
	commandInsertIndex     int
	commandEditPath        []int
	commandEditIndex       int
	commandPromptHistory   map[string]string
	pendingCommandMenu     *xdfilePendingCommandMenu
	commandMenuTempFiles   []string
	remoteClipboardDirs    []string
	deleteUndoStack        []xdfileDeleteUndoBatch
	clipboardMoveUndoStack []xdfileClipboardMoveUndoBatch
	pendingClipboardPaste  *xdfilePendingClipboardPaste
	terminalStarting       bool
	hover                  xdfileHoverState
	panelSearch            xdfilePanelSearchState
}

var xdfileRenderPreviewThumbnailFunc = func(m *xdfileModel, path string, width int, height int) (string, bool, error) {
	return m.renderPreviewThumbnail(path, width, height)
}
