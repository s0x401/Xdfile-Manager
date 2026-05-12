package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *xdfileModel) applyPendingRemoteClipboardPasteItem(
	pending *xdfilePendingClipboardPaste,
	item xdfilePendingClipboardPasteItem,
) (bool, tea.Cmd, error) {
	sourcePath := filepath.Clean(item.SourcePath)
	targetPath := xdfileNormalizeClipboardPastePath(item.TargetPath)

	sourceInfo, err := os.Lstat(sourcePath)
	if err != nil {
		return false, nil, err
	}
	if sourceInfo.Mode()&os.ModeSymlink != 0 {
		return false, nil, fmt.Errorf("symlinks are not supported for remote paste yet: %s", sourcePath)
	}

	targetInfo, err := xdfileNetBoxStatPathFunc(targetPath)
	if err != nil {
		return false, nil, err
	}
	if !targetInfo.Exists {
		return false, m.remoteClipboardPasteTransferCmd(pending, sourcePath, targetPath, item.TopLevel), nil
	}

	if sourceInfo.IsDir() && targetInfo.IsDir {
		pending.recordTarget(targetPath, item.TopLevel)
		children, err := xdfilePendingRemoteClipboardPasteChildren(sourcePath, targetPath)
		if err != nil {
			return false, nil, err
		}
		pending.Queue = append(children, pending.Queue...)
		return false, nil, nil
	}

	if pending.ConflictPolicy != "" {
		cmd, err := m.applyPendingRemoteClipboardPasteConflictAction(pending, pending.ConflictPolicy, sourcePath, targetPath, item.TopLevel)
		return false, cmd, err
	}

	pending.ConflictSource = sourcePath
	pending.ConflictTarget = targetPath
	pending.ConflictTopLevel = item.TopLevel
	m.pendingClipboardPaste = pending
	m.openClipboardPasteConflict(sourcePath, targetPath, false)
	return true, nil, nil
}

func xdfilePendingRemoteClipboardPasteChildren(sourceDir string, targetDir string) ([]xdfilePendingClipboardPasteItem, error) {
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return nil, err
	}
	items := make([]xdfilePendingClipboardPasteItem, 0, len(entries))
	for _, entry := range entries {
		items = append(items, xdfilePendingClipboardPasteItem{
			SourcePath: filepath.Join(sourceDir, entry.Name()),
			TargetPath: xdfileJoinPath(targetDir, entry.Name()),
		})
	}
	return items, nil
}

func (m *xdfileModel) applyPendingRemoteClipboardPasteConflictAction(
	pending *xdfilePendingClipboardPaste,
	action xdfileAction,
	sourcePath string,
	targetPath string,
	topLevel bool,
) (tea.Cmd, error) {
	switch action {
	case xdfileActionPasteConflictOverwrite:
		pending.Overwritten++
		return m.remoteClipboardPasteReplaceCmd(pending, sourcePath, targetPath, topLevel), nil
	case xdfileActionPasteConflictSkip:
		pending.Skipped++
	case xdfileActionPasteConflictRename:
		renamedTarget, err := xdfileNetBoxUniquePasteTargetFunc(targetPath)
		if err != nil {
			return nil, err
		}
		pending.Renamed++
		return m.remoteClipboardPasteTransferCmd(pending, sourcePath, renamedTarget, topLevel), nil
	default:
		return nil, fmt.Errorf("unknown remote paste conflict action: %s", action)
	}
	return nil, nil
}

func (m *xdfileModel) remoteClipboardPasteTransferCmd(
	pending *xdfilePendingClipboardPaste,
	sourcePath string,
	targetPath string,
	topLevel bool,
) tea.Cmd {
	m.setStatus("Uploading %s to %s", filepath.Base(sourcePath), xdfileNetBoxPathLabel(targetPath))
	uploadCmd := func() tea.Msg {
		return xdfileRemoteClipboardPasteDoneMsg{
			Pending:    pending,
			TargetPath: targetPath,
			TopLevel:   topLevel,
			Err:        xdfileNetBoxUploadPathFunc(sourcePath, targetPath),
		}
	}
	return tea.Batch(uploadCmd, m.startBackgroundTask())
}

func (m *xdfileModel) remoteClipboardPasteReplaceCmd(
	pending *xdfilePendingClipboardPaste,
	sourcePath string,
	targetPath string,
	topLevel bool,
) tea.Cmd {
	m.setStatus("Replacing %s", xdfileNetBoxPathLabel(targetPath))
	replaceCmd := func() tea.Msg {
		err := xdfileNetBoxRemovePathFunc(targetPath)
		if err == nil {
			err = xdfileNetBoxUploadPathFunc(sourcePath, targetPath)
		}
		return xdfileRemoteClipboardPasteDoneMsg{
			Pending:    pending,
			TargetPath: targetPath,
			TopLevel:   topLevel,
			Err:        err,
		}
	}
	return tea.Batch(replaceCmd, m.startBackgroundTask())
}

func (m *xdfileModel) applyRemoteClipboardPasteDone(msg xdfileRemoteClipboardPasteDoneMsg) tea.Cmd {
	m.stopBackgroundTask()
	pending := msg.Pending
	if pending == nil {
		pending = m.pendingClipboardPaste
	}
	if msg.Err != nil {
		m.pendingClipboardPaste = nil
		m.setStatusErr(msg.Err)
		return nil
	}
	if pending == nil {
		return nil
	}

	pending.recordTarget(msg.TargetPath, msg.TopLevel)
	m.pendingClipboardPaste = nil
	return m.continuePendingClipboardPaste(pending)
}
