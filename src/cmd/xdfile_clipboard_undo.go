package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const xdfileClipboardMoveUndoDirPrefix = "xdfile-move-undo-"

func (m *xdfileModel) undoLastFileAction() tea.Cmd {
	if cmd, ok := m.cancelClipboardCutSelection(); ok {
		return cmd
	}
	if len(m.clipboardMoveUndoStack) > 0 && len(m.deleteUndoStack) > 0 {
		moveAt := m.clipboardMoveUndoStack[len(m.clipboardMoveUndoStack)-1].At
		deleteAt := m.deleteUndoStack[len(m.deleteUndoStack)-1].At
		if moveAt.After(deleteAt) {
			return m.undoLastClipboardMove()
		}
		return m.undoLastDelete()
	}
	if len(m.clipboardMoveUndoStack) > 0 {
		return m.undoLastClipboardMove()
	}
	return m.undoLastDelete()
}

func (m *xdfileModel) cancelClipboardCutSelection() (tea.Cmd, bool) {
	if !m.clipboardCut || len(m.clipboardPaths) == 0 {
		return nil, false
	}

	paths := append([]string(nil), m.clipboardPaths...)
	m.clipboardCut = false
	m.clipboardPath = paths[0]
	m.setStatus("Canceled cut selection")
	return func() tea.Msg {
		return xdfileClipboardWriteResultMsg{Err: xdfileWriteClipboardPathsFunc(paths, false)}
	}, true
}

func (m *xdfileModel) pushClipboardMoveUndo(pending *xdfilePendingClipboardPaste) {
	if m == nil || pending == nil || len(pending.MoveUndoItems) == 0 {
		return
	}

	batch := xdfileClipboardMoveUndoBatch{
		Root:  pending.MoveUndoRoot,
		Items: append([]xdfileClipboardMoveUndoItem(nil), pending.MoveUndoItems...),
		At:    time.Now(),
	}
	m.clipboardMoveUndoStack = append(m.clipboardMoveUndoStack, batch)
	if len(m.clipboardMoveUndoStack) <= xdfileDeleteUndoStackMax {
		return
	}

	dropped := m.clipboardMoveUndoStack[0]
	if dropped.Root != "" {
		_ = os.RemoveAll(dropped.Root)
	}
	copy(m.clipboardMoveUndoStack, m.clipboardMoveUndoStack[1:])
	m.clipboardMoveUndoStack[len(m.clipboardMoveUndoStack)-1] = xdfileClipboardMoveUndoBatch{}
	m.clipboardMoveUndoStack = m.clipboardMoveUndoStack[:len(m.clipboardMoveUndoStack)-1]
}

func (pending *xdfilePendingClipboardPaste) recordMoveUndo(originalPath string, movedPath string, replacedPath string) {
	if pending == nil || !pending.CutMode {
		return
	}
	pending.MoveUndoItems = append(pending.MoveUndoItems, xdfileClipboardMoveUndoItem{
		OriginalPath: filepath.Clean(originalPath),
		MovedPath:    filepath.Clean(movedPath),
		ReplacedPath: filepath.Clean(replacedPath),
	})
	if replacedPath == "" {
		pending.MoveUndoItems[len(pending.MoveUndoItems)-1].ReplacedPath = ""
	}
}

func (pending *xdfilePendingClipboardPaste) moveUndoBackupPath(targetPath string) (string, error) {
	if pending == nil {
		return "", fmt.Errorf("missing clipboard paste state")
	}
	if pending.MoveUndoRoot == "" {
		parent, err := xdfileDeleteUndoStageParent([]string{targetPath})
		if err != nil {
			return "", err
		}
		if err := os.MkdirAll(parent, 0o700); err != nil {
			return "", err
		}
		root, err := os.MkdirTemp(parent, xdfileClipboardMoveUndoDirPrefix)
		if err != nil {
			return "", err
		}
		pending.MoveUndoRoot = root
	}
	return filepath.Join(
		pending.MoveUndoRoot,
		xdfileDeleteStageName(len(pending.MoveUndoItems), targetPath),
	), nil
}

func (m *xdfileModel) applyPendingClipboardPasteMoveReplace(
	pending *xdfilePendingClipboardPaste,
	sourcePath string,
	targetPath string,
	topLevel bool,
) (tea.Cmd, error) {
	backupPath, err := pending.moveUndoBackupPath(targetPath)
	if err != nil {
		return nil, err
	}
	m.setStatus("Replacing %s", xdfileClipboardPasteBase(targetPath))
	return m.localClipboardPasteCmd(
		pending,
		sourcePath,
		targetPath,
		backupPath,
		topLevel,
		xdfileActionPasteConflictOverwrite,
		func() error {
			if err := xdfileMovePath(targetPath, backupPath); err != nil {
				return err
			}
			if err := xdfileMovePath(sourcePath, targetPath); err != nil {
				_ = xdfileMovePath(backupPath, targetPath)
				return err
			}
			return nil
		},
	), nil
}

func (m *xdfileModel) undoLastClipboardMove() tea.Cmd {
	index := len(m.clipboardMoveUndoStack) - 1
	batch := m.clipboardMoveUndoStack[index]
	if err := xdfileRestoreClipboardMoveUndoBatch(batch); err != nil {
		m.setStatusErr(err)
		return nil
	}

	m.clipboardMoveUndoStack = m.clipboardMoveUndoStack[:index]
	if batch.Root != "" {
		_ = os.RemoveAll(batch.Root)
	}

	restoredPaths := xdfileClipboardMoveUndoOriginalPaths(batch)
	if len(restoredPaths) > 0 {
		m.clipboardCut = false
		m.clipboardPaths = restoredPaths
		m.clipboardPath = restoredPaths[0]
	}

	m.reloadAllPanels()
	if len(batch.Items) == 1 {
		m.setStatus("Restored moved item to %s", batch.Items[0].OriginalPath)
	} else {
		m.setStatus("Restored %d moved items", len(batch.Items))
	}
	if len(restoredPaths) == 0 {
		return nil
	}
	paths := append([]string(nil), restoredPaths...)
	return func() tea.Msg {
		return xdfileClipboardWriteResultMsg{Err: xdfileWriteClipboardPathsFunc(paths, false)}
	}
}

func xdfileRestoreClipboardMoveUndoBatch(batch xdfileClipboardMoveUndoBatch) error {
	if len(batch.Items) == 0 {
		return nil
	}

	for _, item := range batch.Items {
		if item.OriginalPath == "" || item.MovedPath == "" {
			return fmt.Errorf("invalid move undo item")
		}
		if _, err := os.Lstat(item.MovedPath); err != nil {
			return fmt.Errorf("undo source is missing: %s: %w", item.MovedPath, err)
		}
		if _, err := os.Lstat(item.OriginalPath); err == nil {
			return fmt.Errorf("cannot undo move; original path already exists: %s", item.OriginalPath)
		} else if !os.IsNotExist(err) {
			return err
		}
		if item.ReplacedPath != "" {
			if _, err := os.Lstat(item.ReplacedPath); err != nil {
				return fmt.Errorf("undo replaced target is missing: %s: %w", item.ReplacedPath, err)
			}
		}
	}

	for i := len(batch.Items) - 1; i >= 0; i-- {
		item := batch.Items[i]
		if err := os.MkdirAll(filepath.Dir(item.OriginalPath), 0o755); err != nil {
			return err
		}
		if err := xdfileMovePath(item.MovedPath, item.OriginalPath); err != nil {
			return err
		}
		if item.ReplacedPath != "" {
			if err := xdfileMovePath(item.ReplacedPath, item.MovedPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func xdfileClipboardMoveUndoOriginalPaths(batch xdfileClipboardMoveUndoBatch) []string {
	paths := make([]string, 0, len(batch.Items))
	seen := make(map[string]struct{}, len(batch.Items))
	for _, item := range batch.Items {
		path := filepath.Clean(item.OriginalPath)
		key := xdfilePathIdentity(path)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		paths = append(paths, path)
	}
	return paths
}

func (m *xdfileModel) cleanupClipboardMoveUndoStack() {
	for _, batch := range m.clipboardMoveUndoStack {
		if batch.Root != "" {
			_ = os.RemoveAll(batch.Root)
		}
	}
	m.clipboardMoveUndoStack = nil
}

func (m *xdfileModel) cleanupEmptyClipboardMoveUndoRoot(pending *xdfilePendingClipboardPaste) {
	if pending == nil || pending.MoveUndoRoot == "" || len(pending.MoveUndoItems) > 0 {
		return
	}
	_ = os.RemoveAll(pending.MoveUndoRoot)
	pending.MoveUndoRoot = ""
}
