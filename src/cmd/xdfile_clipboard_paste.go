package cmd

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *xdfileModel) continuePendingClipboardPaste(pending *xdfilePendingClipboardPaste) tea.Cmd {
	if pending == nil {
		return nil
	}

	for len(pending.Queue) > 0 {
		item := pending.Queue[0]
		pending.Queue = pending.Queue[1:]
		conflict, cmd, err := m.applyPendingClipboardPasteItem(pending, item)
		if err != nil {
			m.pendingClipboardPaste = nil
			if pending.CutMode {
				m.pushClipboardMoveUndo(pending)
			}
			m.setStatusErr(err)
			return nil
		}
		if conflict {
			return nil
		}
		if cmd != nil {
			m.pendingClipboardPaste = pending
			return cmd
		}
	}

	return m.finishPendingClipboardPaste(pending)
}

func (pending *xdfilePendingClipboardPaste) initQueue() error {
	if pending == nil {
		return nil
	}

	pending.Queue = pending.Queue[:0]
	for _, source := range pending.Sources {
		sourcePath := filepath.Clean(source)
		if pending.remoteDestination() && xdfileIsNetBoxPath(sourcePath) {
			return fmt.Errorf("remote-to-remote paste requires copying the remote item first")
		}
		targetPath := pending.targetPathForSource(sourcePath)

		if !pending.remoteDestination() && pending.CutMode && xdfilePathsEqual(sourcePath, targetPath) {
			pending.Skipped++
			pending.RemainingSources = append(pending.RemainingSources, sourcePath)
			continue
		}
		if !pending.remoteDestination() && !pending.CutMode && xdfilePathsEqual(sourcePath, targetPath) {
			copyTarget, err := xdfileUniqueSameFolderCopyTarget(targetPath)
			if err != nil {
				return err
			}
			targetPath = copyTarget
		}

		pending.Queue = append(pending.Queue, xdfilePendingClipboardPasteItem{
			SourcePath: sourcePath,
			TargetPath: targetPath,
			TopLevel:   true,
		})
	}
	return nil
}

func (m *xdfileModel) applyPendingClipboardPasteItem(
	pending *xdfilePendingClipboardPaste,
	item xdfilePendingClipboardPasteItem,
) (bool, tea.Cmd, error) {
	if pending == nil {
		return false, nil, nil
	}
	if item.CleanupDir {
		_ = os.Remove(item.SourcePath)
		return false, nil, nil
	}
	if pending.remoteDestination() {
		return m.applyPendingRemoteClipboardPasteItem(pending, item)
	}

	sourcePath := filepath.Clean(item.SourcePath)
	targetPath := filepath.Clean(item.TargetPath)
	if pending.CutMode && xdfilePathsEqual(sourcePath, targetPath) {
		pending.Skipped++
		pending.RemainingSources = append(pending.RemainingSources, sourcePath)
		return false, nil, nil
	}

	sourceInfo, err := os.Lstat(sourcePath)
	if err != nil {
		return false, nil, err
	}
	if sourceInfo.Mode()&os.ModeSymlink != 0 {
		return false, nil, fmt.Errorf("symlinks are not supported yet: %s", sourcePath)
	}

	targetInfo, err := os.Stat(targetPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			cmd := m.applyPendingClipboardPasteTransfer(pending, sourcePath, targetPath, item.TopLevel, xdfileActionPaste)
			return false, cmd, nil
		}
		return false, nil, err
	}

	if sourceInfo.IsDir() && targetInfo.IsDir() {
		if err := xdfileValidateTransferTarget(sourcePath, targetPath, sourceInfo); err != nil {
			return false, nil, err
		}
		pending.recordTarget(targetPath, item.TopLevel)
		children, err := xdfilePendingClipboardPasteChildren(sourcePath, targetPath)
		if err != nil {
			return false, nil, err
		}
		insert := children
		if pending.CutMode {
			insert = append(insert, xdfilePendingClipboardPasteItem{
				SourcePath: sourcePath,
				CleanupDir: true,
			})
		}
		pending.Queue = append(insert, pending.Queue...)
		return false, nil, nil
	}

	if pending.ConflictPolicy != "" {
		cmd, err := m.applyPendingClipboardPasteConflictAction(pending, pending.ConflictPolicy, sourcePath, targetPath, item.TopLevel)
		return false, cmd, err
	}

	pending.ConflictSource = sourcePath
	pending.ConflictTarget = targetPath
	pending.ConflictTopLevel = item.TopLevel
	m.pendingClipboardPaste = pending
	m.openClipboardPasteConflict(sourcePath, targetPath, pending.CutMode)
	return true, nil, nil
}

func (m *xdfileModel) applyPendingClipboardPasteTransfer(
	pending *xdfilePendingClipboardPaste,
	sourcePath string,
	targetPath string,
	topLevel bool,
	action xdfileAction,
) tea.Cmd {
	m.setStatus("Pasting %s", xdfileClipboardPasteBase(targetPath))
	work := func() error {
		if pending.CutMode {
			return xdfileMovePath(sourcePath, targetPath)
		}
		return xdfileCopyPath(sourcePath, targetPath)
	}
	return m.localClipboardPasteCmd(pending, sourcePath, targetPath, "", topLevel, action, work)
}

func (pending *xdfilePendingClipboardPaste) recordTarget(targetPath string, topLevel bool) {
	if pending == nil || targetPath == "" {
		return
	}
	targetPath = xdfileNormalizeClipboardPastePath(targetPath)
	if !topLevel {
		return
	}
	for _, existing := range pending.Targets {
		if xdfilePathsEqual(existing, targetPath) {
			pending.LastTarget = targetPath
			if pending.FocusTarget == "" {
				pending.FocusTarget = targetPath
			}
			return
		}
	}
	pending.LastTarget = targetPath
	if pending.FocusTarget == "" {
		pending.FocusTarget = targetPath
	}
	pending.Targets = append(pending.Targets, targetPath)
}

func xdfilePendingClipboardPasteChildren(sourceDir string, targetDir string) ([]xdfilePendingClipboardPasteItem, error) {
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return nil, err
	}
	items := make([]xdfilePendingClipboardPasteItem, 0, len(entries))
	for _, entry := range entries {
		items = append(items, xdfilePendingClipboardPasteItem{
			SourcePath: filepath.Join(sourceDir, entry.Name()),
			TargetPath: filepath.Join(targetDir, entry.Name()),
		})
	}
	return items, nil
}

func (m *xdfileModel) openClipboardPasteConflict(sourcePath string, targetPath string, cutMode bool) {
	actionWord := "paste"
	if cutMode {
		actionWord = "move"
	}

	items := []xdfileModalChoiceItem{
		{
			Action:      xdfileActionPasteConflictOverwrite,
			Label:       "Replace",
			Description: fmt.Sprintf("Replace the existing item with this %s.", actionWord),
		},
		{
			Action:      xdfileActionPasteConflictSkip,
			Label:       "Skip",
			Description: "Leave the existing item untouched and continue.",
		},
		{
			Action:      xdfileActionPasteConflictRename,
			Label:       "Keep both",
			Description: "Keep both items by creating a Windows-style numbered copy.",
		},
	}
	applyAllLabel := "Apply all: off"
	applyAllDescription := "Enable this, then choose Replace, Skip, or Keep both for every remaining conflict."
	if m.pendingClipboardPaste != nil && m.pendingClipboardPaste.ConflictApplyAll {
		applyAllLabel = "Apply all: on"
		applyAllDescription = "The next Replace, Skip, or Keep both choice will be reused for every remaining conflict."
	}
	items = append(items, xdfileModalChoiceItem{
		Action:      xdfileActionPasteConflictApplyAll,
		Label:       applyAllLabel,
		Description: applyAllDescription,
	})

	description := fmt.Sprintf(
		"%s already exists in %s.\n\nSource: %s\nTarget: %s",
		xdfileClipboardPasteBase(targetPath),
		xdfileClipboardPasteDir(targetPath),
		sourcePath,
		targetPath,
	)
	if !cutMode && xdfilePathsEqual(sourcePath, targetPath) {
		items = items[1:]
		description += "\n\nReplace is unavailable because the source and target are the same item."
	}

	m.openChoiceModal("Paste Conflict", description, items)
	m.modal.Action = xdfileActionPasteConflictPrompt
	m.setStatus("Choose Replace, Skip, Keep both, or Apply all")
}

func (m *xdfileModel) resolvePendingClipboardPasteConflict(action xdfileAction) tea.Cmd {
	pending := m.pendingClipboardPaste
	if pending == nil || pending.ConflictSource == "" || pending.ConflictTarget == "" {
		m.pendingClipboardPaste = nil
		m.closeModal()
		return nil
	}

	sourcePath := filepath.Clean(pending.ConflictSource)
	targetPath := xdfileNormalizeClipboardPastePath(pending.ConflictTarget)
	topLevel := pending.ConflictTopLevel

	if action == xdfileActionPasteConflictApplyAll {
		pending.ConflictApplyAll = !pending.ConflictApplyAll
		m.pendingClipboardPaste = pending
		m.openClipboardPasteConflict(sourcePath, targetPath, pending.CutMode)
		if pending.ConflictApplyAll {
			m.setStatus("Apply all is on; choose the conflict action to reuse")
		} else {
			m.setStatus("Apply all is off")
		}
		return nil
	}

	if pending.ConflictApplyAll {
		pending.ConflictPolicy = action
	}
	pending.ConflictSource = ""
	pending.ConflictTarget = ""
	pending.ConflictTopLevel = false

	m.modal.Action = ""
	m.closeModal()

	cmd, err := m.applyPendingClipboardPasteConflictAction(pending, action, sourcePath, targetPath, topLevel)
	if err != nil {
		m.pendingClipboardPaste = nil
		if pending.CutMode {
			m.pushClipboardMoveUndo(pending)
		}
		m.setStatusErr(err)
		return nil
	}
	if cmd != nil {
		m.pendingClipboardPaste = pending
		return cmd
	}

	return m.continuePendingClipboardPaste(pending)
}

func (m *xdfileModel) applyPendingClipboardPasteConflictAction(
	pending *xdfilePendingClipboardPaste,
	action xdfileAction,
	sourcePath string,
	targetPath string,
	topLevel bool,
) (tea.Cmd, error) {
	if pending == nil {
		return nil, nil
	}
	if pending.remoteDestination() {
		return m.applyPendingRemoteClipboardPasteConflictAction(pending, action, sourcePath, targetPath, topLevel)
	}
	switch action {
	case xdfileActionPasteConflictOverwrite:
		if pending.CutMode {
			return m.applyPendingClipboardPasteMoveReplace(pending, sourcePath, targetPath, topLevel)
		} else {
			m.setStatus("Replacing %s", xdfileClipboardPasteBase(targetPath))
			return m.localClipboardPasteCmd(
				pending,
				sourcePath,
				targetPath,
				"",
				topLevel,
				xdfileActionPasteConflictOverwrite,
				func() error {
					return xdfileReplacePath(sourcePath, targetPath, false)
				},
			), nil
		}
	case xdfileActionPasteConflictSkip:
		pending.Skipped++
		if pending.CutMode {
			pending.RemainingSources = append(pending.RemainingSources, sourcePath)
		}
	case xdfileActionPasteConflictRename:
		renamedTarget, err := xdfileUniquePasteCopyTarget(sourcePath, targetPath)
		if err != nil {
			return nil, err
		}
		return m.applyPendingClipboardPasteTransfer(pending, sourcePath, renamedTarget, topLevel, xdfileActionPasteConflictRename), nil
	default:
		return nil, fmt.Errorf("unknown paste conflict action: %s", action)
	}
	return nil, nil
}

func (m *xdfileModel) localClipboardPasteCmd(
	pending *xdfilePendingClipboardPaste,
	sourcePath string,
	targetPath string,
	replacedPath string,
	topLevel bool,
	action xdfileAction,
	work func() error,
) tea.Cmd {
	run := func() tea.Msg {
		var err error
		if work != nil {
			err = work()
		}
		return xdfileLocalClipboardPasteDoneMsg{
			Pending:      pending,
			SourcePath:   sourcePath,
			TargetPath:   targetPath,
			TopLevel:     topLevel,
			Action:       action,
			ReplacedPath: replacedPath,
			Err:          err,
		}
	}
	return tea.Batch(run, m.startBackgroundTask())
}

func (m *xdfileModel) applyLocalClipboardPasteDone(msg xdfileLocalClipboardPasteDoneMsg) tea.Cmd {
	m.stopBackgroundTask()
	pending := msg.Pending
	if pending == nil {
		pending = m.pendingClipboardPaste
	}
	if msg.Err != nil {
		m.pendingClipboardPaste = nil
		if pending != nil && pending.CutMode {
			m.pushClipboardMoveUndo(pending)
			m.cleanupEmptyClipboardMoveUndoRoot(pending)
		}
		m.setStatusErr(msg.Err)
		return nil
	}
	if pending == nil {
		return nil
	}

	pending.recordTarget(msg.TargetPath, msg.TopLevel)
	switch msg.Action {
	case xdfileActionPasteConflictOverwrite:
		pending.Overwritten++
		if pending.CutMode {
			pending.recordMoveUndo(msg.SourcePath, msg.TargetPath, msg.ReplacedPath)
		}
	case xdfileActionPasteConflictRename:
		pending.Renamed++
		pending.recordMoveUndo(msg.SourcePath, msg.TargetPath, "")
	default:
		pending.recordMoveUndo(msg.SourcePath, msg.TargetPath, "")
	}

	m.pendingClipboardPaste = nil
	return m.continuePendingClipboardPaste(pending)
}

func (m *xdfileModel) finishPendingClipboardPaste(pending *xdfilePendingClipboardPaste) tea.Cmd {
	if pending == nil {
		return nil
	}

	m.pendingClipboardPaste = nil
	m.reloadAllPanels()
	m.focusClipboardPasteTarget(pending)
	if pending.CutMode {
		m.pushClipboardMoveUndo(pending)
	}

	var cmd tea.Cmd
	if pending.CutMode {
		if len(pending.RemainingSources) > 0 {
			m.clipboardCut = true
			m.clipboardPaths = append([]string(nil), pending.RemainingSources...)
			m.clipboardPath = pending.RemainingSources[0]
			remaining := append([]string(nil), pending.RemainingSources...)
			cmd = func() tea.Msg {
				return xdfileClipboardWriteResultMsg{Err: xdfileWriteClipboardPathsFunc(remaining, true)}
			}
		} else {
			m.clipboardCut = false
			m.clipboardPaths = append([]string(nil), pending.Targets...)
			if len(pending.Targets) > 0 {
				m.clipboardPath = pending.Targets[0]
				targets := append([]string(nil), pending.Targets...)
				cmd = func() tea.Msg {
					return xdfileClipboardWriteResultMsg{Err: xdfileWriteClipboardPathsFunc(targets, false)}
				}
			} else {
				m.clipboardPath = ""
			}
		}
	}

	m.setStatus("%s", xdfileClipboardPasteStatus(pending))
	return cmd
}

func (m *xdfileModel) focusClipboardPasteTarget(pending *xdfilePendingClipboardPaste) {
	if pending == nil {
		return
	}
	targetPath := pending.FocusTarget
	if targetPath == "" {
		targetPath = pending.LastTarget
	}
	if targetPath == "" {
		return
	}

	panelIndex := m.activePanel
	if panelIndex < 0 || panelIndex >= len(m.panels) {
		return
	}
	if !xdfilePathsEqual(m.panels[panelIndex].Cwd, pending.DestinationDir) {
		for i := range m.panels {
			if xdfilePathsEqual(m.panels[i].Cwd, pending.DestinationDir) {
				panelIndex = i
				break
			}
		}
	}
	if !xdfilePathsEqual(m.panels[panelIndex].Cwd, pending.DestinationDir) {
		return
	}

	rows := m.panels[panelIndex].visibleRows(m.layout.panelRects[panelIndex].h)
	if m.panels[panelIndex].focusPath(targetPath, rows) {
		m.syncQuickViewViewport()
	}
}

func xdfileClipboardPasteStatus(pending *xdfilePendingClipboardPaste) string {
	if pending == nil {
		return "Paste canceled"
	}

	if len(pending.Targets) == 0 {
		if pending.CutMode && len(pending.RemainingSources) > 0 {
			return fmt.Sprintf(
				"Skipped %d item%s; clipboard still holds the remaining cut selection",
				len(pending.RemainingSources),
				xdfilePluralSuffix(len(pending.RemainingSources)),
			)
		}
		return fmt.Sprintf("Nothing to paste into %s", pending.DestinationDir)
	}

	if len(pending.Targets) == 1 && pending.Overwritten == 0 && pending.Renamed == 0 && pending.Skipped == 0 {
		if pending.CutMode {
			return fmt.Sprintf("Moved to %s", pending.LastTarget)
		}
		return fmt.Sprintf("Pasted %s", pending.LastTarget)
	}

	verb := "Pasted"
	if pending.CutMode {
		verb = "Moved"
	}

	status := fmt.Sprintf(
		"%s %d item%s into %s",
		verb,
		len(pending.Targets),
		xdfilePluralSuffix(len(pending.Targets)),
		pending.DestinationDir,
	)

	details := make([]string, 0, 4)
	if pending.Overwritten > 0 {
		details = append(details, fmt.Sprintf("%d overwritten", pending.Overwritten))
	}
	if pending.Renamed > 0 {
		details = append(details, fmt.Sprintf("%d kept both", pending.Renamed))
	}
	if pending.Skipped > 0 {
		details = append(details, fmt.Sprintf("%d skipped", pending.Skipped))
	}
	if pending.CutMode && len(pending.RemainingSources) > 0 {
		details = append(details, fmt.Sprintf("clipboard still holds %d item%s", len(pending.RemainingSources), xdfilePluralSuffix(len(pending.RemainingSources))))
	}
	if len(details) == 0 {
		return status
	}
	return status + " (" + strings.Join(details, ", ") + ")"
}

func xdfilePluralSuffix(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}

func (pending *xdfilePendingClipboardPaste) remoteDestination() bool {
	return pending != nil && xdfileIsNetBoxPath(pending.DestinationDir)
}

func (pending *xdfilePendingClipboardPaste) targetPathForSource(sourcePath string) string {
	if pending == nil {
		return ""
	}
	name := filepath.Base(sourcePath)
	if pending.remoteDestination() {
		return xdfileJoinPath(pending.DestinationDir, name)
	}
	return filepath.Join(pending.DestinationDir, name)
}

func xdfileNormalizeClipboardPastePath(value string) string {
	if remote, ok := xdfileParseNetBoxPath(value); ok {
		return xdfileNetBoxURL(remote.Profile, remote.Path)
	}
	return filepath.Clean(value)
}

func xdfileClipboardPasteBase(value string) string {
	if remote, ok := xdfileParseNetBoxPath(value); ok {
		return path.Base(remote.Path)
	}
	return filepath.Base(value)
}

func xdfileClipboardPasteDir(value string) string {
	if remote, ok := xdfileParseNetBoxPath(value); ok {
		return xdfileNetBoxPathLabel(xdfileNetBoxURL(remote.Profile, path.Dir(remote.Path)))
	}
	return filepath.Dir(value)
}
