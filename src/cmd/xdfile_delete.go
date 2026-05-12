package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	variable "github.com/s0x401/xdfile-manager/src/config"
	"github.com/s0x401/xdfile-manager/src/internal/utils"
)

const (
	xdfileDeleteUndoDirPrefix       = "xdfile-delete-undo-"
	xdfileLegacyDeleteUndoDirPrefix = ".xdfile-delete-undo-"
	xdfileDeleteUndoParentName      = "Xdfile-Delete-Undo"
	xdfileDeleteUndoRootEnv         = "XDFILE_DELETE_UNDO_ROOT"
	xdfileDeleteUndoStackMax        = 20
	xdfileDeleteUndoStateFile       = "xdfile-delete-undo.json"
)

type xdfileDeleteUndoState struct {
	Roots []string `json:"roots"`
}

func (m *xdfileModel) deletePathsWithUndo(paths []string) (int, error) {
	batch, err := xdfileStageDeletePaths(paths)
	if err != nil {
		return 0, err
	}
	if len(batch.Items) == 0 {
		return 0, nil
	}

	m.deleteUndoStack = append(m.deleteUndoStack, batch)
	if len(m.deleteUndoStack) > xdfileDeleteUndoStackMax {
		copy(m.deleteUndoStack, m.deleteUndoStack[1:])
		m.deleteUndoStack[len(m.deleteUndoStack)-1] = xdfileDeleteUndoBatch{}
		m.deleteUndoStack = m.deleteUndoStack[:len(m.deleteUndoStack)-1]
	}
	m.syncDeleteUndoState()
	return len(batch.Items), nil
}

func (m *xdfileModel) undoLastDelete() tea.Cmd {
	if len(m.deleteUndoStack) == 0 {
		m.setStatus("Nothing to undo")
		return nil
	}

	index := len(m.deleteUndoStack) - 1
	batch := m.deleteUndoStack[index]
	if err := xdfileRestoreDeleteUndoBatch(batch); err != nil {
		m.setStatusErr(err)
		return nil
	}
	m.deleteUndoStack = m.deleteUndoStack[:index]
	_ = os.RemoveAll(batch.Root)
	m.syncDeleteUndoState()

	m.reloadAllPanels()
	if len(batch.Items) == 1 {
		m.setStatus("Restored %s", batch.Items[0].OriginalPath)
	} else {
		m.setStatus("Restored %d items", len(batch.Items))
	}
	return nil
}

func (m *xdfileModel) cleanupDeleteUndoStack() {
	m.deleteUndoStack = nil
	m.syncDeleteUndoState()
}

func xdfileDeleteUndoStatePath() string {
	return filepath.Join(variable.XdfileMainDir, xdfileDeleteUndoStateFile)
}

func xdfileNormalizeDeleteUndoRoots(roots []string) []string {
	unique := make([]string, 0, len(roots))
	seen := make(map[string]struct{}, len(roots))
	for _, root := range roots {
		root = filepath.Clean(strings.TrimSpace(root))
		if root == "." || root == "" {
			continue
		}
		key := xdfilePathIdentity(root)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, root)
	}
	return unique
}

func xdfileLoadDeleteUndoState(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read delete undo state: %w", err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, nil
	}

	var state xdfileDeleteUndoState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parse delete undo state: %w", err)
	}
	return xdfileNormalizeDeleteUndoRoots(state.Roots), nil
}

func xdfileSaveDeleteUndoState(path string, roots []string) error {
	roots = xdfileNormalizeDeleteUndoRoots(roots)
	if len(roots) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("clear delete undo state: %w", err)
		}
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), utils.ConfigDirPerm); err != nil {
		return fmt.Errorf("create delete undo state directory: %w", err)
	}

	data, err := json.MarshalIndent(xdfileDeleteUndoState{Roots: roots}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode delete undo state: %w", err)
	}
	if err := os.WriteFile(path, data, utils.ConfigFilePerm); err != nil {
		return fmt.Errorf("write delete undo state: %w", err)
	}
	return nil
}

func xdfileCleanupDeleteUndoState(path string) (int, error) {
	roots, err := xdfileLoadDeleteUndoState(path)
	if err != nil {
		return 0, err
	}
	if len(roots) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return 0, fmt.Errorf("clear empty delete undo state: %w", err)
		}
		return 0, nil
	}

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return len(roots), fmt.Errorf("clear delete undo state after cleanup: %w", err)
	}
	return len(roots), nil
}

func (m *xdfileModel) deleteUndoRoots() []string {
	if m == nil || len(m.deleteUndoStack) == 0 {
		return nil
	}
	roots := make([]string, 0, len(m.deleteUndoStack))
	for _, batch := range m.deleteUndoStack {
		if batch.Root != "" {
			roots = append(roots, batch.Root)
		}
	}
	return roots
}

func (m *xdfileModel) syncDeleteUndoState() {
	if m == nil {
		return
	}
	if err := xdfileSaveDeleteUndoState(xdfileDeleteUndoStatePath(), m.deleteUndoRoots()); err != nil {
		slog.Warn("failed to sync delete undo state", "error", err)
	}
}

func xdfileStageDeletePaths(paths []string) (xdfileDeleteUndoBatch, error) {
	paths = xdfileUniqueDeleteSourcePaths(paths)
	if len(paths) == 0 {
		return xdfileDeleteUndoBatch{}, nil
	}

	for _, path := range paths {
		if _, err := os.Lstat(path); err != nil {
			return xdfileDeleteUndoBatch{}, err
		}
	}

	stageParent, err := xdfileDeleteUndoStageParent(paths)
	if err != nil {
		return xdfileDeleteUndoBatch{}, err
	}
	if err := os.MkdirAll(stageParent, 0o700); err != nil {
		return xdfileDeleteUndoBatch{}, err
	}
	stageRoot, err := os.MkdirTemp(stageParent, xdfileDeleteUndoDirPrefix)
	if err != nil {
		return xdfileDeleteUndoBatch{}, err
	}

	batch := xdfileDeleteUndoBatch{
		Root:  stageRoot,
		Items: make([]xdfileDeleteUndoItem, 0, len(paths)),
		At:    time.Now(),
	}
	for i, sourcePath := range paths {
		stagePath := filepath.Join(stageRoot, xdfileDeleteStageName(i, sourcePath))
		if err := xdfileMovePath(sourcePath, stagePath); err != nil {
			xdfileRollbackStagedDeletes(batch.Items)
			_ = os.RemoveAll(stageRoot)
			return xdfileDeleteUndoBatch{}, err
		}
		batch.Items = append(batch.Items, xdfileDeleteUndoItem{
			OriginalPath: sourcePath,
			StagedPath:   stagePath,
		})
	}
	return batch, nil
}

func xdfileDeleteUndoStageParent(paths []string) (string, error) {
	if envRoot := strings.TrimSpace(os.Getenv(xdfileDeleteUndoRootEnv)); envRoot != "" {
		return filepath.Clean(envRoot), nil
	}
	if len(paths) == 0 {
		return "", fmt.Errorf("no delete undo target")
	}

	switch runtime.GOOS {
	case "windows":
		return xdfileWindowsDeleteUndoStageParent(paths[0])
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return xdfileFallbackDeleteUndoStageParent()
		}
		return filepath.Join(home, ".Trash", xdfileDeleteUndoParentName), nil
	case "linux":
		return filepath.Join(variable.LinuxTrashDirectoryFiles, xdfileDeleteUndoParentName), nil
	default:
		return xdfileFallbackDeleteUndoStageParent()
	}
}

func xdfileWindowsDeleteUndoStageParent(samplePath string) (string, error) {
	abs, err := filepath.Abs(samplePath)
	if err != nil {
		return xdfileFallbackDeleteUndoStageParent()
	}
	volume := filepath.VolumeName(abs)
	if volume == "" {
		return xdfileFallbackDeleteUndoStageParent()
	}

	recycleRoot := filepath.Join(volume+string(os.PathSeparator), "$Recycle.Bin")
	if current, err := user.Current(); err == nil && strings.HasPrefix(strings.ToUpper(current.Uid), "S-") {
		candidate := filepath.Join(recycleRoot, current.Uid, xdfileDeleteUndoParentName)
		if xdfileCanUseDeleteUndoParent(candidate) {
			return candidate, nil
		}
	}
	if entries, err := os.ReadDir(recycleRoot); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() || !strings.HasPrefix(strings.ToUpper(entry.Name()), "S-") {
				continue
			}
			candidate := filepath.Join(recycleRoot, entry.Name(), xdfileDeleteUndoParentName)
			if xdfileCanUseDeleteUndoParent(candidate) {
				return candidate, nil
			}
		}
	}

	candidate := filepath.Join(recycleRoot, xdfileDeleteUndoParentName)
	if xdfileCanUseDeleteUndoParent(candidate) {
		return candidate, nil
	}
	return xdfileFallbackDeleteUndoStageParent()
}

func xdfileFallbackDeleteUndoStageParent() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheDir, xdfileProductName, xdfileDeleteUndoParentName), nil
}

func xdfileCanUseDeleteUndoParent(path string) bool {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return false
	}
	probe, err := os.CreateTemp(path, ".probe-")
	if err != nil {
		return false
	}
	probePath := probe.Name()
	_ = probe.Close()
	_ = os.Remove(probePath)
	return true
}

func xdfileRestoreDeleteUndoBatch(batch xdfileDeleteUndoBatch) error {
	if len(batch.Items) == 0 {
		return nil
	}

	for _, item := range batch.Items {
		if _, err := os.Lstat(item.StagedPath); err != nil {
			return fmt.Errorf("undo source is missing: %s: %w", item.StagedPath, err)
		}
		if _, err := os.Lstat(item.OriginalPath); err == nil {
			return fmt.Errorf("cannot undo delete; target already exists: %s", item.OriginalPath)
		} else if !os.IsNotExist(err) {
			return err
		}
	}

	restored := make([]xdfileDeleteUndoItem, 0, len(batch.Items))
	for _, item := range batch.Items {
		if err := os.MkdirAll(filepath.Dir(item.OriginalPath), 0o755); err != nil {
			xdfileRollbackRestoredDeletes(restored)
			return err
		}
		if err := xdfileMovePath(item.StagedPath, item.OriginalPath); err != nil {
			xdfileRollbackRestoredDeletes(restored)
			return err
		}
		restored = append(restored, item)
	}
	return nil
}

func xdfileUniqueDeleteSourcePaths(paths []string) []string {
	unique := make([]string, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		path = filepath.Clean(strings.TrimSpace(path))
		if path == "." || path == "" {
			continue
		}
		key := xdfilePathIdentity(path)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, path)
	}
	return unique
}

func xdfilePathIdentity(path string) string {
	path = filepath.Clean(path)
	if os.PathSeparator == '\\' {
		return strings.ToLower(path)
	}
	return path
}

func xdfileDeleteStageName(index int, sourcePath string) string {
	base := filepath.Base(sourcePath)
	if base == "." || base == string(os.PathSeparator) || base == "" {
		base = "item"
	}
	return fmt.Sprintf("%03d-%s", index, base)
}

func xdfileRollbackStagedDeletes(items []xdfileDeleteUndoItem) {
	for i := len(items) - 1; i >= 0; i-- {
		item := items[i]
		if _, err := os.Lstat(item.OriginalPath); os.IsNotExist(err) {
			_ = xdfileMovePath(item.StagedPath, item.OriginalPath)
		}
	}
}

func xdfileRollbackRestoredDeletes(items []xdfileDeleteUndoItem) {
	for i := len(items) - 1; i >= 0; i-- {
		item := items[i]
		if _, err := os.Lstat(item.StagedPath); os.IsNotExist(err) {
			_ = xdfileMovePath(item.OriginalPath, item.StagedPath)
		}
	}
}
