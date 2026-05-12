package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	variable "github.com/s0x401/xdfile-manager/src/config"
)

func withDeleteUndoRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv(xdfileDeleteUndoRootEnv, root)
	return root
}

func TestDeleteUsesMarkedEntriesInsteadOfCursorOnly(t *testing.T) {
	root := t.TempDir()
	undoRoot := withDeleteUndoRoot(t)
	left := filepath.Join(root, "left")
	right := filepath.Join(root, "right")
	if err := os.MkdirAll(left, 0o755); err != nil {
		t.Fatalf("create left dir: %v", err)
	}
	if err := os.MkdirAll(right, 0o755); err != nil {
		t.Fatalf("create right dir: %v", err)
	}

	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		if err := os.WriteFile(filepath.Join(left, name), []byte(name), 0o644); err != nil {
			t.Fatalf("write file %q: %v", name, err)
		}
	}

	m := &xdfileModel{
		activePanel: 0,
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: left},
			{Label: "RIGHT", Cwd: right},
		},
	}
	for i := range m.panels {
		if err := m.reloadPanel(i); err != nil {
			t.Fatalf("reload panel %d: %v", i, err)
		}
	}

	aPath := filepath.Join(left, "a.txt")
	bPath := filepath.Join(left, "b.txt")
	cPath := filepath.Join(left, "c.txt")
	m.panels[0].MarkedPaths = map[string]struct{}{
		aPath: {},
		cPath: {},
	}
	m.panels[0].setCursor(findXdfileEntryIndex(t, m.panels[0].Entries, "b.txt"), 8)

	cmd, handled := m.handleGlobalKey(tea.KeyMsg{Type: tea.KeyF8})
	if !handled {
		t.Fatalf("expected F8 to open delete confirmation")
	}
	if cmd != nil {
		t.Fatalf("expected delete confirmation to open synchronously")
	}
	if got := len(m.modal.SourcePaths); got != 2 {
		t.Fatalf("expected delete modal to target two marked entries, got %d: %v", got, m.modal.SourcePaths)
	}

	if cmd := m.applyModal(); cmd != nil {
		t.Fatalf("expected delete to complete synchronously")
	}
	if len(m.deleteUndoStack) != 1 || !strings.HasPrefix(m.deleteUndoStack[0].Root, undoRoot) {
		t.Fatalf("expected delete undo root under %q, got %+v", undoRoot, m.deleteUndoStack)
	}
	for _, path := range []string{aPath, cPath} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected marked path %q to be deleted, stat err=%v", path, err)
		}
	}
	if _, err := os.Stat(bPath); err != nil {
		t.Fatalf("expected cursor-only unmarked path %q to remain: %v", bPath, err)
	}
}

func TestUndoDeleteRestoresMarkedEntries(t *testing.T) {
	root := t.TempDir()
	withDeleteUndoRoot(t)
	left := filepath.Join(root, "left")
	right := filepath.Join(root, "right")
	if err := os.MkdirAll(left, 0o755); err != nil {
		t.Fatalf("create left dir: %v", err)
	}
	if err := os.MkdirAll(right, 0o755); err != nil {
		t.Fatalf("create right dir: %v", err)
	}

	for _, name := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(filepath.Join(left, name), []byte(name), 0o644); err != nil {
			t.Fatalf("write file %q: %v", name, err)
		}
	}

	m := &xdfileModel{
		activePanel: 0,
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: left},
			{Label: "RIGHT", Cwd: right},
		},
	}
	for i := range m.panels {
		if err := m.reloadPanel(i); err != nil {
			t.Fatalf("reload panel %d: %v", i, err)
		}
	}

	aPath := filepath.Join(left, "a.txt")
	bPath := filepath.Join(left, "b.txt")
	m.panels[0].MarkedPaths = map[string]struct{}{
		aPath: {},
		bPath: {},
	}

	if cmd := m.openDeleteConfirm(); cmd != nil {
		t.Fatalf("expected delete confirmation to open synchronously")
	}
	if cmd := m.applyModal(); cmd != nil {
		t.Fatalf("expected delete to complete synchronously")
	}
	for _, path := range []string{aPath, bPath} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected path %q to be staged away after delete, stat err=%v", path, err)
		}
	}

	cmd, handled := m.handleGlobalKey(tea.KeyMsg{Type: tea.KeyCtrlZ})
	if !handled {
		t.Fatalf("expected ctrl+z to be handled")
	}
	if cmd != nil {
		t.Fatalf("expected undo delete to complete synchronously")
	}
	for _, path := range []string{aPath, bPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected path %q to be restored: %v", path, err)
		}
	}
	if got := len(m.deleteUndoStack); got != 0 {
		t.Fatalf("expected undo stack to be empty after restore, got %d", got)
	}
}

func TestUndoDeleteDoesNotOverwriteExistingTarget(t *testing.T) {
	root := t.TempDir()
	withDeleteUndoRoot(t)
	left := filepath.Join(root, "left")
	right := filepath.Join(root, "right")
	if err := os.MkdirAll(left, 0o755); err != nil {
		t.Fatalf("create left dir: %v", err)
	}
	if err := os.MkdirAll(right, 0o755); err != nil {
		t.Fatalf("create right dir: %v", err)
	}

	sourcePath := filepath.Join(left, "a.txt")
	if err := os.WriteFile(sourcePath, []byte("old"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	m := &xdfileModel{
		activePanel: 0,
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: left},
			{Label: "RIGHT", Cwd: right},
		},
	}
	for i := range m.panels {
		if err := m.reloadPanel(i); err != nil {
			t.Fatalf("reload panel %d: %v", i, err)
		}
	}
	m.panels[0].setCursor(findXdfileEntryIndex(t, m.panels[0].Entries, "a.txt"), 8)

	if cmd := m.openDeleteConfirm(); cmd != nil {
		t.Fatalf("expected delete confirmation to open synchronously")
	}
	if cmd := m.applyModal(); cmd != nil {
		t.Fatalf("expected delete to complete synchronously")
	}
	if err := os.WriteFile(sourcePath, []byte("new"), 0o644); err != nil {
		t.Fatalf("write replacement file: %v", err)
	}

	cmd, handled := m.handleGlobalKey(tea.KeyMsg{Type: tea.KeyCtrlZ})
	if !handled {
		t.Fatalf("expected ctrl+z to be handled")
	}
	if cmd != nil {
		t.Fatalf("expected failed undo to complete synchronously")
	}
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read replacement file: %v", err)
	}
	if string(data) != "new" {
		t.Fatalf("expected undo not to overwrite replacement file, got %q", string(data))
	}
	if got := len(m.deleteUndoStack); got != 1 {
		t.Fatalf("expected failed undo to keep batch on stack, got %d", got)
	}
	m.cleanupDeleteUndoStack()
}

func TestDeleteUndoStateSaveLoadAndCleanup(t *testing.T) {
	stateDir := t.TempDir()
	statePath := filepath.Join(stateDir, xdfileDeleteUndoStateFile)
	rootA := filepath.Join(stateDir, ".xdfile-delete-undo-a")
	rootB := filepath.Join(stateDir, ".xdfile-delete-undo-b")

	for _, root := range []string{rootA, rootB} {
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatalf("create staged root %q: %v", root, err)
		}
		if err := os.WriteFile(filepath.Join(root, "payload.txt"), []byte("payload"), 0o644); err != nil {
			t.Fatalf("seed staged payload for %q: %v", root, err)
		}
	}

	if err := xdfileSaveDeleteUndoState(statePath, []string{rootA, "", rootA, rootB}); err != nil {
		t.Fatalf("save delete undo state: %v", err)
	}

	roots, err := xdfileLoadDeleteUndoState(statePath)
	if err != nil {
		t.Fatalf("load delete undo state: %v", err)
	}
	if got := len(roots); got != 2 {
		t.Fatalf("expected two unique roots, got %d: %v", got, roots)
	}

	count, err := xdfileCleanupDeleteUndoState(statePath)
	if err != nil {
		t.Fatalf("cleanup delete undo state: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected cleanup count 2, got %d", count)
	}
	for _, root := range []string{rootA, rootB} {
		if _, statErr := os.Stat(root); statErr != nil {
			t.Fatalf("expected staged root %q to remain in recycle staging, stat err=%v", root, statErr)
		}
	}
	if _, statErr := os.Stat(statePath); !os.IsNotExist(statErr) {
		t.Fatalf("expected delete undo state file to be removed, stat err=%v", statErr)
	}
}

func TestNewXdfileModelCleansStaleDeleteUndoRootsFromPreviousSession(t *testing.T) {
	stateDir := t.TempDir()
	originalMainDir := variable.XdfileMainDir
	variable.XdfileMainDir = stateDir
	t.Cleanup(func() {
		variable.XdfileMainDir = originalMainDir
	})

	staleRoot := filepath.Join(stateDir, ".xdfile-delete-undo-stale")
	if err := os.MkdirAll(staleRoot, 0o755); err != nil {
		t.Fatalf("create stale undo root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(staleRoot, "payload.txt"), []byte("payload"), 0o644); err != nil {
		t.Fatalf("seed stale undo root: %v", err)
	}
	if err := xdfileSaveDeleteUndoState(xdfileDeleteUndoStatePath(), []string{staleRoot}); err != nil {
		t.Fatalf("save stale delete undo state: %v", err)
	}

	left := t.TempDir()
	right := t.TempDir()
	m := newXdfileModel([]string{left, right})
	t.Cleanup(m.closeXdfileResources)

	if _, err := os.Stat(staleRoot); err != nil {
		t.Fatalf("expected stale undo root to remain in recycle staging on startup, stat err=%v", err)
	}
	if _, err := os.Stat(xdfileDeleteUndoStatePath()); !os.IsNotExist(err) {
		t.Fatalf("expected stale delete undo state file to be removed on startup, stat err=%v", err)
	}
	if got := m.statusText; got != "Released 1 stale delete undo directory from the previous session" {
		t.Fatalf("expected startup cleanup status, got %q", got)
	}
}
