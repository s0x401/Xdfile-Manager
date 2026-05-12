package cmd

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	vt "github.com/charmbracelet/x/vt"
)

func TestHandleGlobalKeyCtrlShiftCCopiesAndCtrlShiftVPastes(t *testing.T) {
	originalReadClipboardPaths := xdfileReadClipboardPathsFunc
	originalReadClipboardCut := xdfileReadClipboardCutFunc
	originalWriteClipboardPaths := xdfileWriteClipboardPathsFunc
	defer func() {
		xdfileReadClipboardPathsFunc = originalReadClipboardPaths
		xdfileReadClipboardCutFunc = originalReadClipboardCut
		xdfileWriteClipboardPathsFunc = originalWriteClipboardPaths
	}()

	var clipboardPaths []string
	xdfileWriteClipboardPathsFunc = func(paths []string, cut bool) error {
		clipboardPaths = append([]string(nil), paths...)
		return nil
	}
	xdfileReadClipboardPathsFunc = func() ([]string, error) {
		return append([]string(nil), clipboardPaths...), nil
	}
	xdfileReadClipboardCutFunc = func() (bool, error) {
		return false, nil
	}

	root := t.TempDir()
	left := filepath.Join(root, "left")
	right := filepath.Join(root, "right")
	if err := os.MkdirAll(left, 0o755); err != nil {
		t.Fatalf("create left dir: %v", err)
	}
	if err := os.MkdirAll(right, 0o755); err != nil {
		t.Fatalf("create right dir: %v", err)
	}

	sourcePath := filepath.Join(left, "sample.txt")
	if err := os.WriteFile(sourcePath, []byte("hello xdfile"), 0o644); err != nil {
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

	m.panels[0].setCursor(findXdfileEntryIndex(t, m.panels[0].Entries, "sample.txt"), 8)

	cmd, handled := m.handleGlobalKey(tea.KeyMsg{Type: tea.KeyCtrlShiftC})
	if !handled {
		t.Fatalf("expected ctrl+shift+c to be handled in panel mode")
	}
	if cmd == nil {
		t.Fatalf("expected ctrl+shift+c to schedule clipboard sync")
	}
	if msg := cmd(); msg != nil {
		if result, ok := msg.(xdfileClipboardWriteResultMsg); !ok || result.Err != nil {
			t.Fatalf("expected successful clipboard sync result, got %#v", msg)
		}
	}
	if m.clipboardPath != sourcePath {
		t.Fatalf("expected clipboard path %q, got %q", sourcePath, m.clipboardPath)
	}
	if len(m.clipboardPaths) != 1 || m.clipboardPaths[0] != sourcePath {
		t.Fatalf("expected internal clipboard paths to contain %q, got %v", sourcePath, m.clipboardPaths)
	}

	m.activePanel = 1
	cmd, handled = m.handleGlobalKey(tea.KeyMsg{Type: tea.KeyCtrlShiftV})
	if !handled {
		t.Fatalf("expected ctrl+shift+v to be handled in panel mode")
	}
	if cmd != nil {
		t.Fatalf("expected ctrl+shift+v to complete immediately")
	}

	targetPath := filepath.Join(right, "sample.txt")
	data, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read pasted file: %v", err)
	}
	if string(data) != "hello xdfile" {
		t.Fatalf("expected pasted file contents to match source, got %q", string(data))
	}
	assertXdfilePanelSelectedPath(t, m.panels[1], targetPath)
}

func TestCopyRemoteSelectionDownloadsToClipboardAndPastesLocally(t *testing.T) {
	originalDownload := xdfileNetBoxDownloadPathsFunc
	originalReadEntries := xdfileNetBoxReadEntriesFunc
	originalReadClipboardPaths := xdfileReadClipboardPathsFunc
	originalReadClipboardCut := xdfileReadClipboardCutFunc
	originalWriteClipboardPaths := xdfileWriteClipboardPathsFunc
	defer func() {
		xdfileNetBoxDownloadPathsFunc = originalDownload
		xdfileNetBoxReadEntriesFunc = originalReadEntries
		xdfileReadClipboardPathsFunc = originalReadClipboardPaths
		xdfileReadClipboardCutFunc = originalReadClipboardCut
		xdfileWriteClipboardPathsFunc = originalWriteClipboardPaths
	}()

	remoteDir := xdfileNetBoxURL("prod", "/src")
	remotePath := xdfileNetBoxURL("prod", "/src/sample.txt")
	targetDir := t.TempDir()

	xdfileNetBoxReadEntriesFunc = func(dir string, _ bool, _ xdfileSortMode) ([]xdfileEntry, error) {
		if dir != remoteDir {
			t.Fatalf("unexpected remote reload dir: %q", dir)
		}
		return []xdfileEntry{{Name: "sample.txt", Path: remotePath}}, nil
	}

	var externalPaths []string
	xdfileWriteClipboardPathsFunc = func(paths []string, cut bool) error {
		if cut {
			t.Fatalf("remote clipboard copy should not write cut mode")
		}
		externalPaths = append([]string(nil), paths...)
		return nil
	}
	xdfileReadClipboardPathsFunc = func() ([]string, error) {
		return append([]string(nil), externalPaths...), nil
	}
	xdfileReadClipboardCutFunc = func() (bool, error) {
		return false, nil
	}

	xdfileNetBoxDownloadPathsFunc = func(paths []string) ([]string, string, error) {
		if len(paths) != 1 || paths[0] != remotePath {
			t.Fatalf("unexpected remote download paths: %v", paths)
		}
		cacheDir := t.TempDir()
		localPath := filepath.Join(cacheDir, "sample.txt")
		if err := os.WriteFile(localPath, []byte("from ssh"), 0o644); err != nil {
			t.Fatalf("write downloaded cache file: %v", err)
		}
		return []string{localPath}, cacheDir, nil
	}

	m := &xdfileModel{
		activePanel: 0,
		panels: [2]xdfilePanel{
			{
				Label:   "LEFT",
				Cwd:     remoteDir,
				Entries: []xdfileEntry{{Name: "sample.txt", Path: remotePath}},
			},
			{Label: "RIGHT", Cwd: targetDir},
		},
	}
	if err := m.reloadPanel(1); err != nil {
		t.Fatalf("reload local target panel: %v", err)
	}

	cmd := m.copySelectionToClipboard()
	if cmd == nil {
		t.Fatalf("expected remote copy to download asynchronously")
	}
	if msg := xdfileFirstTestCommandMsg(cmd); msg != nil {
		if _, updateCmd := m.Update(msg); updateCmd != nil {
			t.Fatalf("expected remote copy update to complete immediately")
		}
	}

	if len(m.clipboardPaths) != 1 || len(externalPaths) != 1 || m.clipboardPaths[0] != externalPaths[0] {
		t.Fatalf("expected internal and external clipboard to use downloaded path, internal=%v external=%v", m.clipboardPaths, externalPaths)
	}

	m.activePanel = 1
	if cmd := m.pasteClipboardToActivePanel(); cmd != nil {
		t.Fatalf("expected downloaded remote paste to complete immediately")
	}

	targetPath := filepath.Join(targetDir, "sample.txt")
	data, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read pasted remote file: %v", err)
	}
	if string(data) != "from ssh" {
		t.Fatalf("expected pasted remote file contents, got %q", string(data))
	}
	assertXdfilePanelSelectedPath(t, m.panels[1], targetPath)
}

func TestPasteClipboardToRemotePanelUploadsLocalFile(t *testing.T) {
	originalReadClipboardPaths := xdfileReadClipboardPathsFunc
	originalReadClipboardCut := xdfileReadClipboardCutFunc
	originalReadEntries := xdfileNetBoxReadEntriesFunc
	originalStat := xdfileNetBoxStatPathFunc
	originalUpload := xdfileNetBoxUploadPathFunc
	defer func() {
		xdfileReadClipboardPathsFunc = originalReadClipboardPaths
		xdfileReadClipboardCutFunc = originalReadClipboardCut
		xdfileNetBoxReadEntriesFunc = originalReadEntries
		xdfileNetBoxStatPathFunc = originalStat
		xdfileNetBoxUploadPathFunc = originalUpload
	}()

	sourceDir := t.TempDir()
	sourcePath := filepath.Join(sourceDir, "sample.txt")
	if err := os.WriteFile(sourcePath, []byte("to ssh"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}
	remoteDir := xdfileNetBoxURL("prod", "/upload")
	remoteTarget := xdfileNetBoxURL("prod", "/upload/sample.txt")

	xdfileReadClipboardPathsFunc = func() ([]string, error) {
		return []string{sourcePath}, nil
	}
	xdfileReadClipboardCutFunc = func() (bool, error) {
		return false, nil
	}
	xdfileNetBoxStatPathFunc = func(target string) (xdfileNetBoxFileInfo, error) {
		if target != remoteTarget {
			t.Fatalf("unexpected remote stat target: %q", target)
		}
		return xdfileNetBoxFileInfo{}, nil
	}
	var uploadedSource string
	var uploadedTarget string
	xdfileNetBoxUploadPathFunc = func(source string, target string) error {
		uploadedSource = source
		uploadedTarget = target
		return nil
	}
	xdfileNetBoxReadEntriesFunc = func(dir string, _ bool, _ xdfileSortMode) ([]xdfileEntry, error) {
		if dir != remoteDir {
			t.Fatalf("unexpected remote reload dir: %q", dir)
		}
		return []xdfileEntry{{Name: "sample.txt", Path: remoteTarget}}, nil
	}

	m := &xdfileModel{
		activePanel: 0,
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: remoteDir},
			{Label: "RIGHT", Cwd: t.TempDir()},
		},
	}

	cmd := m.pasteClipboardToActivePanel()
	if cmd == nil {
		t.Fatalf("expected remote paste to upload asynchronously")
	}
	msg := xdfileFirstTestCommandMsg(cmd)
	if _, ok := msg.(xdfileRemoteClipboardPasteDoneMsg); !ok {
		t.Fatalf("expected remote paste done message, got %T", msg)
	}
	if _, updateCmd := m.Update(msg); updateCmd != nil {
		t.Fatalf("expected remote paste update to finish immediately")
	}
	if uploadedSource != sourcePath || uploadedTarget != remoteTarget {
		t.Fatalf("expected upload %q -> %q, got %q -> %q", sourcePath, remoteTarget, uploadedSource, uploadedTarget)
	}
	assertXdfilePanelSelectedPath(t, m.panels[0], remoteTarget)
}

func TestPasteClipboardToRemotePanelPromptsOnFileConflict(t *testing.T) {
	originalReadClipboardPaths := xdfileReadClipboardPathsFunc
	originalReadClipboardCut := xdfileReadClipboardCutFunc
	originalStat := xdfileNetBoxStatPathFunc
	defer func() {
		xdfileReadClipboardPathsFunc = originalReadClipboardPaths
		xdfileReadClipboardCutFunc = originalReadClipboardCut
		xdfileNetBoxStatPathFunc = originalStat
	}()

	sourcePath := filepath.Join(t.TempDir(), "sample.txt")
	if err := os.WriteFile(sourcePath, []byte("to ssh"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}
	remoteTarget := xdfileNetBoxURL("prod", "/upload/sample.txt")
	xdfileReadClipboardPathsFunc = func() ([]string, error) {
		return []string{sourcePath}, nil
	}
	xdfileReadClipboardCutFunc = func() (bool, error) {
		return false, nil
	}
	xdfileNetBoxStatPathFunc = func(target string) (xdfileNetBoxFileInfo, error) {
		if target != remoteTarget {
			t.Fatalf("unexpected remote stat target: %q", target)
		}
		return xdfileNetBoxFileInfo{Exists: true}, nil
	}

	m := &xdfileModel{
		activePanel: 0,
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: xdfileNetBoxURL("prod", "/upload")},
			{Label: "RIGHT", Cwd: t.TempDir()},
		},
	}

	if cmd := m.pasteClipboardToActivePanel(); cmd != nil {
		t.Fatalf("expected conflicting remote paste to open a choice modal")
	}
	if m.modal.Kind != xdfileModalChoice || m.modal.Action != xdfileActionPasteConflictPrompt {
		t.Fatalf("expected remote paste conflict modal, got kind=%v action=%v", m.modal.Kind, m.modal.Action)
	}
	if m.pendingClipboardPaste == nil || m.pendingClipboardPaste.ConflictTarget != remoteTarget {
		t.Fatalf("expected pending remote conflict target %q, got %+v", remoteTarget, m.pendingClipboardPaste)
	}
}

func TestPasteClipboardToRemoteExistingDirectoryPromptsNestedFileConflict(t *testing.T) {
	originalReadClipboardPaths := xdfileReadClipboardPathsFunc
	originalReadClipboardCut := xdfileReadClipboardCutFunc
	originalStat := xdfileNetBoxStatPathFunc
	defer func() {
		xdfileReadClipboardPathsFunc = originalReadClipboardPaths
		xdfileReadClipboardCutFunc = originalReadClipboardCut
		xdfileNetBoxStatPathFunc = originalStat
	}()

	sourceDir := filepath.Join(t.TempDir(), "aa")
	if err := os.MkdirAll(filepath.Join(sourceDir, "cc"), 0o755); err != nil {
		t.Fatalf("create source tree: %v", err)
	}
	for path, content := range map[string]string{
		filepath.Join(sourceDir, "1.png"):       "one",
		filepath.Join(sourceDir, "2.png"):       "two",
		filepath.Join(sourceDir, "cc", "3.png"): "three",
	} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	remoteDir := xdfileNetBoxURL("prod", "/upload")
	remoteTop := xdfileNetBoxURL("prod", "/upload/aa")
	remoteConflict := xdfileNetBoxURL("prod", "/upload/aa/1.png")
	xdfileReadClipboardPathsFunc = func() ([]string, error) {
		return []string{sourceDir}, nil
	}
	xdfileReadClipboardCutFunc = func() (bool, error) {
		return false, nil
	}
	var statTargets []string
	xdfileNetBoxStatPathFunc = func(target string) (xdfileNetBoxFileInfo, error) {
		statTargets = append(statTargets, target)
		switch target {
		case remoteTop:
			return xdfileNetBoxFileInfo{Exists: true, IsDir: true}, nil
		case remoteConflict:
			return xdfileNetBoxFileInfo{Exists: true}, nil
		default:
			t.Fatalf("unexpected remote stat target before conflict: %q", target)
			return xdfileNetBoxFileInfo{}, nil
		}
	}

	m := &xdfileModel{
		activePanel: 0,
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: remoteDir},
			{Label: "RIGHT", Cwd: t.TempDir()},
		},
	}

	if cmd := m.pasteClipboardToActivePanel(); cmd != nil {
		t.Fatalf("expected nested remote conflict to open a choice modal")
	}
	if m.modal.Kind != xdfileModalChoice || m.pendingClipboardPaste == nil {
		t.Fatalf("expected nested remote conflict modal, got kind=%v pending=%+v", m.modal.Kind, m.pendingClipboardPaste)
	}
	if m.pendingClipboardPaste.ConflictTarget != remoteConflict {
		t.Fatalf("expected nested conflict at %q, got %q", remoteConflict, m.pendingClipboardPaste.ConflictTarget)
	}
	if len(statTargets) != 2 || statTargets[0] != remoteTop || statTargets[1] != remoteConflict {
		t.Fatalf("expected top dir then nested file stat, got %v", statTargets)
	}
}

func TestHandleGlobalKeyCtrlCAndCtrlVNoLongerTriggerFileClipboard(t *testing.T) {
	root := t.TempDir()
	left := filepath.Join(root, "left")
	right := filepath.Join(root, "right")
	if err := os.MkdirAll(left, 0o755); err != nil {
		t.Fatalf("create left dir: %v", err)
	}
	if err := os.MkdirAll(right, 0o755); err != nil {
		t.Fatalf("create right dir: %v", err)
	}
	sourcePath := filepath.Join(left, "sample.txt")
	if err := os.WriteFile(sourcePath, []byte("hello xdfile"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	m := &xdfileModel{
		activePanel: 0,
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: left},
			{Label: "RIGHT", Cwd: right},
		},
		clipboardPaths: []string{sourcePath},
	}
	for i := range m.panels {
		if err := m.reloadPanel(i); err != nil {
			t.Fatalf("reload panel %d: %v", i, err)
		}
	}
	m.panels[0].setCursor(findXdfileEntryIndex(t, m.panels[0].Entries, "sample.txt"), 8)

	if cmd, handled := m.handleGlobalKey(tea.KeyMsg{Type: tea.KeyCtrlC}); handled || cmd != nil {
		t.Fatalf("expected ctrl+c not to trigger file copy, handled=%v cmd=%v", handled, cmd)
	}
	m.activePanel = 1
	if cmd, handled := m.handleGlobalKey(tea.KeyMsg{Type: tea.KeyCtrlV}); handled || cmd != nil {
		t.Fatalf("expected ctrl+v not to trigger file paste, handled=%v cmd=%v", handled, cmd)
	}
	if _, err := os.Stat(filepath.Join(right, "sample.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected ctrl+v not to paste files, stat err=%v", err)
	}
}

func TestPasteClipboardToActivePanelKeepsBothOnCopyConflict(t *testing.T) {
	originalReadClipboardPaths := xdfileReadClipboardPathsFunc
	originalReadClipboardCut := xdfileReadClipboardCutFunc
	defer func() {
		xdfileReadClipboardPathsFunc = originalReadClipboardPaths
		xdfileReadClipboardCutFunc = originalReadClipboardCut
	}()

	root := t.TempDir()
	left := filepath.Join(root, "left")
	right := filepath.Join(root, "right")
	if err := os.MkdirAll(left, 0o755); err != nil {
		t.Fatalf("create left dir: %v", err)
	}
	if err := os.MkdirAll(right, 0o755); err != nil {
		t.Fatalf("create right dir: %v", err)
	}

	sourcePath := filepath.Join(left, "sample.txt")
	targetPath := filepath.Join(right, "sample.txt")
	if err := os.WriteFile(sourcePath, []byte("new contents"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}
	if err := os.WriteFile(targetPath, []byte("old contents"), 0o644); err != nil {
		t.Fatalf("write target file: %v", err)
	}

	xdfileReadClipboardPathsFunc = func() ([]string, error) {
		return []string{sourcePath}, nil
	}
	xdfileReadClipboardCutFunc = func() (bool, error) {
		return false, nil
	}

	m := &xdfileModel{
		activePanel:    1,
		clipboardPaths: []string{sourcePath},
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

	if cmd := m.pasteClipboardToActivePanel(); cmd != nil {
		t.Fatalf("expected conflicting copy paste to open a choice modal")
	}
	if m.modal.Kind != xdfileModalChoice {
		t.Fatalf("expected copy conflict to open choice modal, got kind %v", m.modal.Kind)
	}
	if len(m.modal.ChoiceItems) != 4 {
		t.Fatalf("expected copy conflict to offer 4 choices, got %+v", m.modal.ChoiceItems)
	}
	if got := m.modal.ChoiceItems[2].Label; got != "Keep both" {
		t.Fatalf("expected Windows-style keep both option, got %q", got)
	}
	if got := m.modal.ChoiceItems[3].Action; got != xdfileActionPasteConflictApplyAll {
		t.Fatalf("expected apply-all conflict option, got %q", got)
	}

	if cmd := m.executeAction(xdfileActionPasteConflictRename); cmd != nil {
		t.Fatalf("expected keep-both conflict resolution to complete immediately")
	}

	renamedTarget := filepath.Join(right, "sample (2).txt")
	data, err := os.ReadFile(renamedTarget)
	if err != nil {
		t.Fatalf("read keep-both target: %v", err)
	}
	if string(data) != "new contents" {
		t.Fatalf("expected keep-both copy contents to match source, got %q", string(data))
	}
	original, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read original target: %v", err)
	}
	if string(original) != "old contents" {
		t.Fatalf("expected original conflicting target to remain untouched, got %q", string(original))
	}
	assertXdfilePanelSelectedPath(t, m.panels[1], renamedTarget)
}

func TestPasteClipboardToActivePanelMergesDirectoryAndPromptsOnlyNestedConflicts(t *testing.T) {
	originalReadClipboardPaths := xdfileReadClipboardPathsFunc
	originalReadClipboardCut := xdfileReadClipboardCutFunc
	defer func() {
		xdfileReadClipboardPathsFunc = originalReadClipboardPaths
		xdfileReadClipboardCutFunc = originalReadClipboardCut
	}()

	root := t.TempDir()
	sourceRoot := filepath.Join(root, "source")
	targetRoot := filepath.Join(root, "target")
	sourceDir := filepath.Join(sourceRoot, "aa")
	targetDir := filepath.Join(targetRoot, "aa")
	if err := os.MkdirAll(filepath.Join(sourceDir, "cc"), 0o755); err != nil {
		t.Fatalf("create source tree: %v", err)
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("create target tree: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "1.png"), []byte("new 1"), 0o644); err != nil {
		t.Fatalf("write source conflict file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "2.png"), []byte("new 2"), 0o644); err != nil {
		t.Fatalf("write source new file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "4.png"), []byte("new 4"), 0o644); err != nil {
		t.Fatalf("write second source conflict file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "cc", "3.png"), []byte("new 3"), 0o644); err != nil {
		t.Fatalf("write source nested file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "1.png"), []byte("old 1"), 0o644); err != nil {
		t.Fatalf("write target conflict file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "4.png"), []byte("old 4"), 0o644); err != nil {
		t.Fatalf("write second target conflict file: %v", err)
	}

	xdfileReadClipboardPathsFunc = func() ([]string, error) {
		return []string{sourceDir}, nil
	}
	xdfileReadClipboardCutFunc = func() (bool, error) {
		return false, nil
	}

	m := &xdfileModel{
		activePanel:    0,
		clipboardPaths: []string{sourceDir},
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: targetRoot},
		},
	}
	if err := m.reloadPanel(0); err != nil {
		t.Fatalf("reload panel: %v", err)
	}

	if cmd := m.pasteClipboardToActivePanel(); cmd != nil {
		t.Fatalf("expected nested directory conflict to open a choice modal")
	}
	if m.modal.Kind != xdfileModalChoice {
		t.Fatalf("expected nested file conflict to open choice modal, got kind %v", m.modal.Kind)
	}
	conflictTarget := filepath.Join(targetDir, "1.png")
	if m.pendingClipboardPaste == nil || !xdfilePathsEqual(m.pendingClipboardPaste.ConflictTarget, conflictTarget) {
		t.Fatalf("expected conflict on nested 1.png, got pending=%+v", m.pendingClipboardPaste)
	}

	if cmd := m.executeAction(xdfileActionPasteConflictApplyAll); cmd != nil {
		t.Fatalf("expected apply-all toggle to complete immediately")
	}
	if m.pendingClipboardPaste == nil || !m.pendingClipboardPaste.ConflictApplyAll {
		t.Fatalf("expected apply-all to remain enabled, got pending=%+v", m.pendingClipboardPaste)
	}

	if cmd := m.executeAction(xdfileActionPasteConflictSkip); cmd != nil {
		t.Fatalf("expected nested conflict skip to complete immediately")
	}

	data, err := os.ReadFile(filepath.Join(targetDir, "1.png"))
	if err != nil {
		t.Fatalf("read skipped conflict target: %v", err)
	}
	if string(data) != "old 1" {
		t.Fatalf("expected skipped conflict to keep original contents, got %q", string(data))
	}
	data, err = os.ReadFile(filepath.Join(targetDir, "4.png"))
	if err != nil {
		t.Fatalf("read second skipped conflict target: %v", err)
	}
	if string(data) != "old 4" {
		t.Fatalf("expected apply-all skipped conflict to keep original contents, got %q", string(data))
	}
	data, err = os.ReadFile(filepath.Join(targetDir, "2.png"))
	if err != nil {
		t.Fatalf("read non-conflicting sibling: %v", err)
	}
	if string(data) != "new 2" {
		t.Fatalf("expected non-conflicting sibling to be copied, got %q", string(data))
	}
	data, err = os.ReadFile(filepath.Join(targetDir, "cc", "3.png"))
	if err != nil {
		t.Fatalf("read non-conflicting nested file: %v", err)
	}
	if string(data) != "new 3" {
		t.Fatalf("expected non-conflicting nested file to be copied, got %q", string(data))
	}
	assertXdfilePanelSelectedPath(t, m.panels[0], targetDir)
}

func TestPasteClipboardToSameFolderCreatesWindowsCopyName(t *testing.T) {
	originalReadClipboardPaths := xdfileReadClipboardPathsFunc
	originalReadClipboardCut := xdfileReadClipboardCutFunc
	defer func() {
		xdfileReadClipboardPathsFunc = originalReadClipboardPaths
		xdfileReadClipboardCutFunc = originalReadClipboardCut
	}()

	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "sample.txt")
	if err := os.WriteFile(sourcePath, []byte("copy contents"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	xdfileReadClipboardPathsFunc = func() ([]string, error) {
		return []string{sourcePath}, nil
	}
	xdfileReadClipboardCutFunc = func() (bool, error) {
		return false, nil
	}

	m := &xdfileModel{
		activePanel:    0,
		clipboardPaths: []string{sourcePath},
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: dir},
		},
	}
	if err := m.reloadPanel(0); err != nil {
		t.Fatalf("reload panel: %v", err)
	}

	if cmd := m.pasteClipboardToActivePanel(); cmd != nil {
		t.Fatalf("expected same-folder copy paste to complete immediately")
	}
	if m.modal.Kind != xdfileModalNone {
		t.Fatalf("expected same-folder copy paste not to open conflict modal, got %v", m.modal.Kind)
	}

	copyPath := filepath.Join(dir, "sample - Copy.txt")
	data, err := os.ReadFile(copyPath)
	if err != nil {
		t.Fatalf("read Windows-style copy target: %v", err)
	}
	if string(data) != "copy contents" {
		t.Fatalf("expected copied contents, got %q", string(data))
	}
	assertXdfilePanelSelectedPath(t, m.panels[0], copyPath)
}

func TestPasteClipboardToSameFolderCreatesNumberedWindowsCopyName(t *testing.T) {
	originalReadClipboardPaths := xdfileReadClipboardPathsFunc
	originalReadClipboardCut := xdfileReadClipboardCutFunc
	defer func() {
		xdfileReadClipboardPathsFunc = originalReadClipboardPaths
		xdfileReadClipboardCutFunc = originalReadClipboardCut
	}()

	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "sample.txt")
	firstCopyPath := filepath.Join(dir, "sample - Copy.txt")
	if err := os.WriteFile(sourcePath, []byte("copy contents"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}
	if err := os.WriteFile(firstCopyPath, []byte("existing copy"), 0o644); err != nil {
		t.Fatalf("write first copy: %v", err)
	}

	xdfileReadClipboardPathsFunc = func() ([]string, error) {
		return []string{sourcePath}, nil
	}
	xdfileReadClipboardCutFunc = func() (bool, error) {
		return false, nil
	}

	m := &xdfileModel{
		activePanel:    0,
		clipboardPaths: []string{sourcePath},
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: dir},
		},
	}
	if err := m.reloadPanel(0); err != nil {
		t.Fatalf("reload panel: %v", err)
	}

	if cmd := m.pasteClipboardToActivePanel(); cmd != nil {
		t.Fatalf("expected same-folder numbered copy paste to complete immediately")
	}

	copyPath := filepath.Join(dir, "sample - Copy (2).txt")
	data, err := os.ReadFile(copyPath)
	if err != nil {
		t.Fatalf("read numbered Windows-style copy target: %v", err)
	}
	if string(data) != "copy contents" {
		t.Fatalf("expected copied contents, got %q", string(data))
	}
}

func TestHandleGlobalKeyCtrlShiftCCopiesMarkedEntries(t *testing.T) {
	originalWriteClipboardPaths := xdfileWriteClipboardPathsFunc
	defer func() {
		xdfileWriteClipboardPathsFunc = originalWriteClipboardPaths
	}()

	var clipboardPaths []string
	xdfileWriteClipboardPathsFunc = func(paths []string, cut bool) error {
		clipboardPaths = append([]string(nil), paths...)
		return nil
	}

	root := t.TempDir()
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
			{Label: "LEFT", Cwd: left, RangeAnchor: -1},
			{Label: "RIGHT", Cwd: right, RangeAnchor: -1},
		},
	}
	for i := range m.panels {
		if err := m.reloadPanel(i); err != nil {
			t.Fatalf("reload panel %d: %v", i, err)
		}
	}

	rows := 8
	m.panels[0].setCursor(findXdfileEntryIndex(t, m.panels[0].Entries, "b.txt"), rows)
	_ = m.handlePanelKey(tea.KeyMsg{Type: tea.KeyShiftLeft})

	cmd, handled := m.handleGlobalKey(tea.KeyMsg{Type: tea.KeyCtrlShiftC})
	if !handled {
		t.Fatalf("expected ctrl+shift+c to be handled for marked entries")
	}
	if cmd == nil {
		t.Fatalf("expected ctrl+shift+c to schedule clipboard sync for marked entries")
	}
	if msg := cmd(); msg != nil {
		if result, ok := msg.(xdfileClipboardWriteResultMsg); !ok || result.Err != nil {
			t.Fatalf("expected successful clipboard sync result, got %#v", msg)
		}
	}

	want := []string{
		filepath.Join(left, "a.txt"),
		filepath.Join(left, "b.txt"),
	}
	if len(m.clipboardPaths) != len(want) {
		t.Fatalf("expected %d internal clipboard paths, got %v", len(want), m.clipboardPaths)
	}
	for i, path := range want {
		if m.clipboardPaths[i] != path {
			t.Fatalf("expected internal clipboard path %d to be %q, got %q", i, path, m.clipboardPaths[i])
		}
		if clipboardPaths[i] != path {
			t.Fatalf("expected external clipboard path %d to be %q, got %q", i, path, clipboardPaths[i])
		}
	}
}

func TestPasteClipboardToActivePanelOverwritesCutConflict(t *testing.T) {
	originalReadClipboardPaths := xdfileReadClipboardPathsFunc
	originalReadClipboardCut := xdfileReadClipboardCutFunc
	originalWriteClipboardPaths := xdfileWriteClipboardPathsFunc
	defer func() {
		xdfileReadClipboardPathsFunc = originalReadClipboardPaths
		xdfileReadClipboardCutFunc = originalReadClipboardCut
		xdfileWriteClipboardPathsFunc = originalWriteClipboardPaths
	}()

	root := t.TempDir()
	left := filepath.Join(root, "left")
	right := filepath.Join(root, "right")
	if err := os.MkdirAll(left, 0o755); err != nil {
		t.Fatalf("create left dir: %v", err)
	}
	if err := os.MkdirAll(right, 0o755); err != nil {
		t.Fatalf("create right dir: %v", err)
	}

	sourcePath := filepath.Join(left, "sample.txt")
	targetPath := filepath.Join(right, "sample.txt")
	if err := os.WriteFile(sourcePath, []byte("moved contents"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}
	if err := os.WriteFile(targetPath, []byte("old target"), 0o644); err != nil {
		t.Fatalf("write target file: %v", err)
	}

	xdfileReadClipboardPathsFunc = func() ([]string, error) {
		return []string{sourcePath}, nil
	}
	xdfileReadClipboardCutFunc = func() (bool, error) {
		return true, nil
	}

	var clipboardPaths []string
	clipboardCut := false
	xdfileWriteClipboardPathsFunc = func(paths []string, cut bool) error {
		clipboardPaths = append([]string(nil), paths...)
		clipboardCut = cut
		return nil
	}

	m := &xdfileModel{
		activePanel:    1,
		clipboardPaths: []string{sourcePath},
		clipboardPath:  sourcePath,
		clipboardCut:   true,
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

	if cmd := m.pasteClipboardToActivePanel(); cmd != nil {
		t.Fatalf("expected conflicting cut paste to open a choice modal")
	}
	if m.modal.Kind != xdfileModalChoice {
		t.Fatalf("expected cut conflict to open choice modal, got kind %v", m.modal.Kind)
	}

	cmd := m.executeAction(xdfileActionPasteConflictOverwrite)
	if cmd == nil {
		t.Fatalf("expected overwrite cut resolution to resync clipboard")
	}
	if msg := cmd(); msg != nil {
		if result, ok := msg.(xdfileClipboardWriteResultMsg); !ok || result.Err != nil {
			t.Fatalf("expected successful clipboard resync result, got %#v", msg)
		}
	}

	data, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read overwritten target: %v", err)
	}
	if string(data) != "moved contents" {
		t.Fatalf("expected overwritten target to contain moved contents, got %q", string(data))
	}
	if _, err := os.Stat(sourcePath); !os.IsNotExist(err) {
		t.Fatalf("expected source to be removed after overwrite move, stat err=%v", err)
	}
	if m.clipboardCut {
		t.Fatalf("expected clipboard cut mode to clear after successful overwrite move")
	}
	if len(m.clipboardPaths) != 1 || m.clipboardPaths[0] != targetPath {
		t.Fatalf("expected clipboard paths to track overwritten target %q, got %v", targetPath, m.clipboardPaths)
	}
	if clipboardCut {
		t.Fatalf("expected external clipboard cut mode to clear after overwrite move")
	}
	if len(clipboardPaths) != 1 || clipboardPaths[0] != targetPath {
		t.Fatalf("expected external clipboard to track overwritten target %q, got %v", targetPath, clipboardPaths)
	}
}

func TestHandleGlobalKeyCtrlXMovesAndCtrlVPastes(t *testing.T) {
	originalReadClipboardPaths := xdfileReadClipboardPathsFunc
	originalReadClipboardCut := xdfileReadClipboardCutFunc
	originalWriteClipboardPaths := xdfileWriteClipboardPathsFunc
	defer func() {
		xdfileReadClipboardPathsFunc = originalReadClipboardPaths
		xdfileReadClipboardCutFunc = originalReadClipboardCut
		xdfileWriteClipboardPathsFunc = originalWriteClipboardPaths
	}()

	var clipboardPaths []string
	clipboardCut := false
	xdfileWriteClipboardPathsFunc = func(paths []string, cut bool) error {
		clipboardPaths = append([]string(nil), paths...)
		clipboardCut = cut
		return nil
	}
	xdfileReadClipboardPathsFunc = func() ([]string, error) {
		return append([]string(nil), clipboardPaths...), nil
	}
	xdfileReadClipboardCutFunc = func() (bool, error) {
		return clipboardCut, nil
	}

	root := t.TempDir()
	left := filepath.Join(root, "left")
	right := filepath.Join(root, "right")
	if err := os.MkdirAll(left, 0o755); err != nil {
		t.Fatalf("create left dir: %v", err)
	}
	if err := os.MkdirAll(right, 0o755); err != nil {
		t.Fatalf("create right dir: %v", err)
	}

	sourcePath := filepath.Join(left, "sample.txt")
	if err := os.WriteFile(sourcePath, []byte("hello cut"), 0o644); err != nil {
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

	m.panels[0].setCursor(findXdfileEntryIndex(t, m.panels[0].Entries, "sample.txt"), 8)

	cmd, handled := m.handleGlobalKey(tea.KeyMsg{Type: tea.KeyCtrlX})
	if !handled {
		t.Fatalf("expected ctrl+x to be handled in panel mode")
	}
	if cmd == nil {
		t.Fatalf("expected ctrl+x to schedule clipboard sync")
	}
	if msg := cmd(); msg != nil {
		if result, ok := msg.(xdfileClipboardWriteResultMsg); !ok || result.Err != nil {
			t.Fatalf("expected successful clipboard sync result, got %#v", msg)
		}
	}
	if !m.clipboardCut {
		t.Fatalf("expected clipboard cut mode to be enabled")
	}
	if !clipboardCut {
		t.Fatalf("expected ctrl+x to write a cut clipboard payload")
	}

	m.activePanel = 1
	cmd, handled = m.handleGlobalKey(tea.KeyMsg{Type: tea.KeyCtrlShiftV})
	if !handled {
		t.Fatalf("expected ctrl+shift+v to be handled in panel mode after cut")
	}
	if cmd == nil {
		t.Fatalf("expected ctrl+shift+v after cut to resync clipboard to moved target")
	}
	if msg := cmd(); msg != nil {
		if result, ok := msg.(xdfileClipboardWriteResultMsg); !ok || result.Err != nil {
			t.Fatalf("expected successful clipboard resync result, got %#v", msg)
		}
	}

	targetPath := filepath.Join(right, "sample.txt")
	data, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read moved file: %v", err)
	}
	if string(data) != "hello cut" {
		t.Fatalf("expected moved file contents to match source, got %q", string(data))
	}
	if _, err := os.Stat(sourcePath); !os.IsNotExist(err) {
		t.Fatalf("expected source file to be removed after cut+paste, stat err=%v", err)
	}
	if m.clipboardCut {
		t.Fatalf("expected clipboard cut mode to clear after paste")
	}
	if len(m.clipboardPaths) != 1 || m.clipboardPaths[0] != targetPath {
		t.Fatalf("expected clipboard paths to track moved target %q, got %v", targetPath, m.clipboardPaths)
	}
	if m.clipboardPath != targetPath {
		t.Fatalf("expected clipboard path to track moved target %q, got %q", targetPath, m.clipboardPath)
	}
}

func TestPasteClipboardToActivePanelMovesExternalCutClipboard(t *testing.T) {
	originalReadClipboardPaths := xdfileReadClipboardPathsFunc
	originalReadClipboardCut := xdfileReadClipboardCutFunc
	originalWriteClipboardPaths := xdfileWriteClipboardPathsFunc
	defer func() {
		xdfileReadClipboardPathsFunc = originalReadClipboardPaths
		xdfileReadClipboardCutFunc = originalReadClipboardCut
		xdfileWriteClipboardPathsFunc = originalWriteClipboardPaths
	}()

	root := t.TempDir()
	left := filepath.Join(root, "left")
	right := filepath.Join(root, "right")
	if err := os.MkdirAll(left, 0o755); err != nil {
		t.Fatalf("create left dir: %v", err)
	}
	if err := os.MkdirAll(right, 0o755); err != nil {
		t.Fatalf("create right dir: %v", err)
	}

	sourcePath := filepath.Join(left, "sample.txt")
	if err := os.WriteFile(sourcePath, []byte("hello external cut"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	xdfileReadClipboardPathsFunc = func() ([]string, error) {
		return []string{sourcePath}, nil
	}
	xdfileReadClipboardCutFunc = func() (bool, error) {
		return true, nil
	}
	xdfileWriteClipboardPathsFunc = func(paths []string, cut bool) error {
		return nil
	}

	m := &xdfileModel{
		activePanel: 1,
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

	cmd := m.pasteClipboardToActivePanel()
	if cmd == nil {
		t.Fatalf("expected external cut paste to resync clipboard to moved target")
	}
	if msg := cmd(); msg != nil {
		if result, ok := msg.(xdfileClipboardWriteResultMsg); !ok || result.Err != nil {
			t.Fatalf("expected successful clipboard resync result, got %#v", msg)
		}
	}

	targetPath := filepath.Join(right, "sample.txt")
	data, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read moved file: %v", err)
	}
	if string(data) != "hello external cut" {
		t.Fatalf("expected moved file contents to match source, got %q", string(data))
	}
	if _, err := os.Stat(sourcePath); !os.IsNotExist(err) {
		t.Fatalf("expected source file to be removed after external cut paste, stat err=%v", err)
	}
}

func TestPasteClipboardToActivePanelKeepsSkippedCutConflictsInClipboard(t *testing.T) {
	originalReadClipboardPaths := xdfileReadClipboardPathsFunc
	originalReadClipboardCut := xdfileReadClipboardCutFunc
	originalWriteClipboardPaths := xdfileWriteClipboardPathsFunc
	defer func() {
		xdfileReadClipboardPathsFunc = originalReadClipboardPaths
		xdfileReadClipboardCutFunc = originalReadClipboardCut
		xdfileWriteClipboardPathsFunc = originalWriteClipboardPaths
	}()

	root := t.TempDir()
	left := filepath.Join(root, "left")
	right := filepath.Join(root, "right")
	if err := os.MkdirAll(left, 0o755); err != nil {
		t.Fatalf("create left dir: %v", err)
	}
	if err := os.MkdirAll(right, 0o755); err != nil {
		t.Fatalf("create right dir: %v", err)
	}

	sourcePath := filepath.Join(left, "sample.txt")
	targetPath := filepath.Join(right, "sample.txt")
	if err := os.WriteFile(sourcePath, []byte("cut source"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}
	if err := os.WriteFile(targetPath, []byte("existing target"), 0o644); err != nil {
		t.Fatalf("write target file: %v", err)
	}

	xdfileReadClipboardPathsFunc = func() ([]string, error) {
		return []string{sourcePath}, nil
	}
	xdfileReadClipboardCutFunc = func() (bool, error) {
		return true, nil
	}

	var clipboardPaths []string
	clipboardCut := false
	xdfileWriteClipboardPathsFunc = func(paths []string, cut bool) error {
		clipboardPaths = append([]string(nil), paths...)
		clipboardCut = cut
		return nil
	}

	m := &xdfileModel{
		activePanel:    1,
		clipboardPaths: []string{sourcePath},
		clipboardPath:  sourcePath,
		clipboardCut:   true,
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

	if cmd := m.pasteClipboardToActivePanel(); cmd != nil {
		t.Fatalf("expected conflicting cut paste to open a choice modal")
	}

	cmd := m.executeAction(xdfileActionPasteConflictSkip)
	if cmd == nil {
		t.Fatalf("expected skipped cut conflict to keep clipboard synchronized")
	}
	if msg := cmd(); msg != nil {
		if result, ok := msg.(xdfileClipboardWriteResultMsg); !ok || result.Err != nil {
			t.Fatalf("expected successful clipboard resync result, got %#v", msg)
		}
	}

	data, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read existing target: %v", err)
	}
	if string(data) != "existing target" {
		t.Fatalf("expected existing target to remain untouched, got %q", string(data))
	}
	if _, err := os.Stat(sourcePath); err != nil {
		t.Fatalf("expected skipped cut source to remain in place, stat err=%v", err)
	}
	if !m.clipboardCut {
		t.Fatalf("expected clipboard cut mode to remain enabled for skipped source")
	}
	if len(m.clipboardPaths) != 1 || m.clipboardPaths[0] != sourcePath {
		t.Fatalf("expected clipboard to keep skipped source %q, got %v", sourcePath, m.clipboardPaths)
	}
	if !clipboardCut {
		t.Fatalf("expected external clipboard to remain in cut mode after skip")
	}
	if len(clipboardPaths) != 1 || clipboardPaths[0] != sourcePath {
		t.Fatalf("expected external clipboard to keep skipped source %q, got %v", sourcePath, clipboardPaths)
	}
}

func TestPasteClipboardPrefersExternalClipboardOverStaleInternalCache(t *testing.T) {
	originalReadClipboardPaths := xdfileReadClipboardPathsFunc
	originalReadClipboardCut := xdfileReadClipboardCutFunc
	defer func() {
		xdfileReadClipboardPathsFunc = originalReadClipboardPaths
		xdfileReadClipboardCutFunc = originalReadClipboardCut
	}()

	root := t.TempDir()
	left := filepath.Join(root, "left")
	right := filepath.Join(root, "right")
	if err := os.MkdirAll(left, 0o755); err != nil {
		t.Fatalf("create left dir: %v", err)
	}
	if err := os.MkdirAll(right, 0o755); err != nil {
		t.Fatalf("create right dir: %v", err)
	}

	externalPath := filepath.Join(left, "external.txt")
	if err := os.WriteFile(externalPath, []byte("hello external clipboard"), 0o644); err != nil {
		t.Fatalf("write external source file: %v", err)
	}
	staleInternalPath := filepath.Join(right, "stale.txt")
	if err := os.WriteFile(staleInternalPath, []byte("stale"), 0o644); err != nil {
		t.Fatalf("write stale internal file: %v", err)
	}

	xdfileReadClipboardPathsFunc = func() ([]string, error) {
		return []string{externalPath}, nil
	}
	xdfileReadClipboardCutFunc = func() (bool, error) {
		return false, nil
	}

	m := &xdfileModel{
		activePanel:    1,
		clipboardPaths: []string{staleInternalPath},
		clipboardPath:  staleInternalPath,
		clipboardCut:   true,
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

	cmd := m.pasteClipboardToActivePanel()
	if cmd != nil {
		t.Fatalf("expected external clipboard copy paste to complete immediately")
	}

	targetPath := filepath.Join(right, "external.txt")
	data, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read pasted external file: %v", err)
	}
	if string(data) != "hello external clipboard" {
		t.Fatalf("expected pasted file contents to match external clipboard source, got %q", string(data))
	}
	if _, err := os.Stat(staleInternalPath); err != nil {
		t.Fatalf("expected stale internal cache file to remain untouched, stat err=%v", err)
	}
}

func TestHandleGlobalKeyClipboardShortcutsWorkWhileTerminalAutoFocused(t *testing.T) {
	originalReadClipboardPaths := xdfileReadClipboardPathsFunc
	originalReadClipboardCut := xdfileReadClipboardCutFunc
	originalWriteClipboardPaths := xdfileWriteClipboardPathsFunc
	defer func() {
		xdfileReadClipboardPathsFunc = originalReadClipboardPaths
		xdfileReadClipboardCutFunc = originalReadClipboardCut
		xdfileWriteClipboardPathsFunc = originalWriteClipboardPaths
	}()

	var clipboardPaths []string
	xdfileWriteClipboardPathsFunc = func(paths []string, cut bool) error {
		clipboardPaths = append([]string(nil), paths...)
		return nil
	}
	xdfileReadClipboardPathsFunc = func() ([]string, error) {
		return append([]string(nil), clipboardPaths...), nil
	}
	xdfileReadClipboardCutFunc = func() (bool, error) {
		return false, nil
	}

	root := t.TempDir()
	left := filepath.Join(root, "left")
	right := filepath.Join(root, "right")
	if err := os.MkdirAll(left, 0o755); err != nil {
		t.Fatalf("create left dir: %v", err)
	}
	if err := os.MkdirAll(right, 0o755); err != nil {
		t.Fatalf("create right dir: %v", err)
	}

	sourcePath := filepath.Join(left, "sample.txt")
	if err := os.WriteFile(sourcePath, []byte("hello auto focus"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	m := &xdfileModel{
		activePanel:         0,
		terminalFocused:     true,
		terminalAutoFocused: true,
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

	m.panels[0].setCursor(findXdfileEntryIndex(t, m.panels[0].Entries, "sample.txt"), 8)

	cmd, handled := m.handleGlobalKey(tea.KeyMsg{Type: tea.KeyCtrlShiftC})
	if !handled {
		t.Fatalf("expected ctrl+shift+c to be handled while terminal is auto-focused from panel click")
	}
	if cmd == nil {
		t.Fatalf("expected ctrl+shift+c to schedule clipboard sync while terminal is auto-focused")
	}
	if msg := cmd(); msg != nil {
		if result, ok := msg.(xdfileClipboardWriteResultMsg); !ok || result.Err != nil {
			t.Fatalf("expected successful clipboard sync result, got %#v", msg)
		}
	}

	m.activePanel = 1
	cmd, handled = m.handleGlobalKey(tea.KeyMsg{Type: tea.KeyCtrlShiftV})
	if !handled {
		t.Fatalf("expected ctrl+shift+v to be handled while terminal is auto-focused from panel click")
	}
	if cmd != nil {
		t.Fatalf("expected ctrl+shift+v to complete immediately while terminal is auto-focused")
	}

	targetPath := filepath.Join(right, "sample.txt")
	data, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read pasted file: %v", err)
	}
	if string(data) != "hello auto focus" {
		t.Fatalf("expected pasted file contents to match source, got %q", string(data))
	}
}

func TestSelectPanelKeepTerminalFocusKeepsClipboardShortcutsOnPanels(t *testing.T) {
	originalReadClipboardPaths := xdfileReadClipboardPathsFunc
	originalWriteClipboardPaths := xdfileWriteClipboardPathsFunc
	defer func() {
		xdfileReadClipboardPathsFunc = originalReadClipboardPaths
		xdfileWriteClipboardPathsFunc = originalWriteClipboardPaths
	}()

	var clipboardPaths []string
	xdfileWriteClipboardPathsFunc = func(paths []string, cut bool) error {
		clipboardPaths = append([]string(nil), paths...)
		return nil
	}
	xdfileReadClipboardPathsFunc = func() ([]string, error) {
		return append([]string(nil), clipboardPaths...), nil
	}

	root := t.TempDir()
	left := filepath.Join(root, "left")
	right := filepath.Join(root, "right")
	if err := os.MkdirAll(left, 0o755); err != nil {
		t.Fatalf("create left dir: %v", err)
	}
	if err := os.MkdirAll(right, 0o755); err != nil {
		t.Fatalf("create right dir: %v", err)
	}

	sourcePath := filepath.Join(right, "sample.txt")
	if err := os.WriteFile(sourcePath, []byte("hello keep focus"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	m := &xdfileModel{
		activePanel:     0,
		terminalFocused: true,
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

	_ = m.selectPanel(1, true)
	if m.terminalAutoFocused {
		t.Fatalf("expected non-PTY panel selection to keep panel focus instead of terminal auto-focus")
	}
	if m.terminalFocused {
		t.Fatalf("expected non-PTY panel selection to leave terminal focus off")
	}

	m.panels[1].setCursor(findXdfileEntryIndex(t, m.panels[1].Entries, "sample.txt"), 8)
	cmd, handled := m.handleGlobalKey(tea.KeyMsg{Type: tea.KeyCtrlShiftC})
	if !handled {
		t.Fatalf("expected ctrl+shift+c to be handled after selecting panel with keepTerminalFocus")
	}
	if cmd == nil {
		t.Fatalf("expected ctrl+shift+c to schedule clipboard sync after selecting panel with keepTerminalFocus")
	}
	if msg := cmd(); msg != nil {
		if result, ok := msg.(xdfileClipboardWriteResultMsg); !ok || result.Err != nil {
			t.Fatalf("expected successful clipboard sync result, got %#v", msg)
		}
	}
	if len(m.clipboardPaths) != 1 || m.clipboardPaths[0] != sourcePath {
		t.Fatalf("expected internal clipboard paths to contain %q, got %v", sourcePath, m.clipboardPaths)
	}
}

func TestHandleMouseRightClickOpensContextMenu(t *testing.T) {
	originalReadClipboardPaths := xdfileReadClipboardPathsFunc
	originalReadClipboardCut := xdfileReadClipboardCutFunc
	defer func() {
		xdfileReadClipboardPathsFunc = originalReadClipboardPaths
		xdfileReadClipboardCutFunc = originalReadClipboardCut
	}()
	xdfileReadClipboardPathsFunc = func() ([]string, error) {
		return nil, nil
	}
	xdfileReadClipboardCutFunc = func() (bool, error) {
		return false, nil
	}

	root := t.TempDir()
	left := filepath.Join(root, "left")
	right := filepath.Join(root, "right")
	if err := os.MkdirAll(left, 0o755); err != nil {
		t.Fatalf("create left dir: %v", err)
	}
	if err := os.MkdirAll(right, 0o755); err != nil {
		t.Fatalf("create right dir: %v", err)
	}

	sourcePath := filepath.Join(left, "sample.txt")
	if err := os.WriteFile(sourcePath, []byte("hello xdfile"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	m := &xdfileModel{
		width:  120,
		height: 40,
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
	m.computeLayout()

	index := findXdfileEntryIndex(t, m.panels[0].Entries, "sample.txt")
	row := index - m.panels[0].Scroll
	rect := m.layout.panelRects[0]

	cmd := m.handleMouse(tea.MouseMsg{
		X:      rect.x + 2,
		Y:      rect.y + 3 + row,
		Button: tea.MouseButtonRight,
		Action: tea.MouseActionPress,
	})
	if cmd != nil {
		if msg := cmd(); msg != nil {
			t.Fatalf("expected context menu open to complete immediately, got %T", msg)
		}
	}

	if m.openMenu != xdfileActionContextMenu {
		t.Fatalf("expected context menu to be open, got %q", m.openMenu)
	}

	menu, ok := m.currentMenu()
	if !ok {
		t.Fatalf("expected current menu to resolve")
	}

	labels := make([]string, 0, len(menu.Items))
	for _, item := range menu.Items {
		labels = append(labels, item.Label)
	}

	for _, want := range []string{"Open", "Copy", "Cut", "Paste", "Rename", "Delete"} {
		if !containsXdfileLabel(labels, want) {
			t.Fatalf("expected context menu labels %v to include %q", labels, want)
		}
	}

	propertiesFound := false
	for _, item := range menu.Items {
		if item.Label == "Properties" {
			propertiesFound = true
			if item.Key != "R" {
				t.Fatalf("expected Properties item to use hotkey R, got %+v", item)
			}
		}
	}
	if !propertiesFound {
		t.Fatalf("expected context menu labels %v to include Properties", labels)
	}

	pasteItem := menu.Items[5]
	if pasteItem.Label != "Paste" || !pasteItem.Disabled {
		t.Fatalf("expected Paste item to be present and disabled when clipboard is empty, got %+v", pasteItem)
	}
}

func TestContextMenuEnablesPasteFromExternalClipboard(t *testing.T) {
	originalReadClipboardPaths := xdfileReadClipboardPathsFunc
	originalReadClipboardCut := xdfileReadClipboardCutFunc
	defer func() {
		xdfileReadClipboardPathsFunc = originalReadClipboardPaths
		xdfileReadClipboardCutFunc = originalReadClipboardCut
	}()

	root := t.TempDir()
	left := filepath.Join(root, "left")
	right := filepath.Join(root, "right")
	if err := os.MkdirAll(left, 0o755); err != nil {
		t.Fatalf("create left dir: %v", err)
	}
	if err := os.MkdirAll(right, 0o755); err != nil {
		t.Fatalf("create right dir: %v", err)
	}

	sourcePath := filepath.Join(left, "sample.txt")
	if err := os.WriteFile(sourcePath, []byte("hello external"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	xdfileReadClipboardPathsFunc = func() ([]string, error) {
		return []string{sourcePath}, nil
	}
	xdfileReadClipboardCutFunc = func() (bool, error) {
		return false, nil
	}

	m := &xdfileModel{
		width:  120,
		height: 40,
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
	m.computeLayout()

	index := findXdfileEntryIndex(t, m.panels[0].Entries, "sample.txt")
	row := index - m.panels[0].Scroll
	rect := m.layout.panelRects[0]

	_ = m.handleMouse(tea.MouseMsg{
		X:      rect.x + 2,
		Y:      rect.y + 3 + row,
		Button: tea.MouseButtonRight,
		Action: tea.MouseActionPress,
	})

	menu, ok := m.currentMenu()
	if !ok {
		t.Fatalf("expected current menu to resolve")
	}

	pasteItem := menu.Items[5]
	if pasteItem.Label != "Paste" || pasteItem.Disabled {
		t.Fatalf("expected Paste item to be enabled for external clipboard paths, got %+v", pasteItem)
	}
}

func TestContextMenuAllowsRemoteCopyButNotRemoteCut(t *testing.T) {
	originalReadClipboardPaths := xdfileReadClipboardPathsFunc
	originalReadClipboardCut := xdfileReadClipboardCutFunc
	defer func() {
		xdfileReadClipboardPathsFunc = originalReadClipboardPaths
		xdfileReadClipboardCutFunc = originalReadClipboardCut
	}()
	xdfileReadClipboardPathsFunc = func() ([]string, error) {
		return nil, nil
	}
	xdfileReadClipboardCutFunc = func() (bool, error) {
		return false, nil
	}

	remoteDir := xdfileNetBoxURL("prod", "/src")
	remotePath := xdfileNetBoxURL("prod", "/src/sample.txt")
	m := &xdfileModel{
		activePanel: 0,
		panels: [2]xdfilePanel{
			{
				Label:   "LEFT",
				Cwd:     remoteDir,
				Entries: []xdfileEntry{{Name: "sample.txt", Path: remotePath}},
			},
			{Label: "RIGHT", Cwd: t.TempDir()},
		},
	}

	items := m.panelContextMenuItems(0, 0)
	var openItem, copyItem, cutItem xdfileButton
	for _, item := range items {
		switch item.Label {
		case "Open":
			openItem = item
		case "Copy":
			copyItem = item
		case "Cut":
			cutItem = item
		}
	}
	if openItem.Label != "Open" || !openItem.Disabled {
		t.Fatalf("expected remote file Open to be disabled until remote open is implemented, got %+v", openItem)
	}
	if copyItem.Label != "Copy" || copyItem.Disabled {
		t.Fatalf("expected remote Copy to be enabled, got %+v", copyItem)
	}
	if cutItem.Label != "Cut" || !cutItem.Disabled {
		t.Fatalf("expected remote Cut to remain disabled, got %+v", cutItem)
	}
}

func TestHandleMenuKeyRExecutesContextMenuProperties(t *testing.T) {
	originalShowProperties := xdfileShowSystemPropertiesFunc
	defer func() {
		xdfileShowSystemPropertiesFunc = originalShowProperties
	}()

	root := t.TempDir()
	left := filepath.Join(root, "left")
	right := filepath.Join(root, "right")
	if err := os.MkdirAll(left, 0o755); err != nil {
		t.Fatalf("create left dir: %v", err)
	}
	if err := os.MkdirAll(right, 0o755); err != nil {
		t.Fatalf("create right dir: %v", err)
	}

	sourcePath := filepath.Join(left, "sample.txt")
	if err := os.WriteFile(sourcePath, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	var opened string
	xdfileShowSystemPropertiesFunc = func(path string) error {
		opened = path
		return nil
	}

	m := &xdfileModel{
		width:  120,
		height: 40,
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
	m.computeLayout()

	index := findXdfileEntryIndex(t, m.panels[0].Entries, "sample.txt")
	row := index - m.panels[0].Scroll
	rect := m.layout.panelRects[0]

	_ = m.handleMouse(tea.MouseMsg{
		X:      rect.x + 2,
		Y:      rect.y + 3 + row,
		Button: tea.MouseButtonRight,
		Action: tea.MouseActionPress,
	})

	cmd, handled := m.handleMenuKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if !handled {
		t.Fatalf("expected R to be handled as a context menu hotkey")
	}
	if cmd != nil {
		if msg := cmd(); msg != nil {
			t.Fatalf("expected properties hotkey to complete immediately, got %T", msg)
		}
	}
	if opened != sourcePath {
		t.Fatalf("expected properties hotkey to open Windows properties for %q, got %q", sourcePath, opened)
	}
	if m.modal.Kind != xdfileModalNone {
		t.Fatalf("expected properties hotkey not to open an in-app modal, got %v", m.modal.Kind)
	}
}

func TestHandleGlobalKeyClipboardShortcutsWhilePTYTerminalFocused(t *testing.T) {
	originalReadClipboardPaths := xdfileReadClipboardPathsFunc
	originalWriteClipboardPaths := xdfileWriteClipboardPathsFunc
	defer func() {
		xdfileReadClipboardPathsFunc = originalReadClipboardPaths
		xdfileWriteClipboardPathsFunc = originalWriteClipboardPaths
	}()

	var clipboardPaths []string
	xdfileWriteClipboardPathsFunc = func(paths []string, cut bool) error {
		clipboardPaths = append([]string(nil), paths...)
		return nil
	}
	xdfileReadClipboardPathsFunc = func() ([]string, error) {
		return append([]string(nil), clipboardPaths...), nil
	}

	root := t.TempDir()
	left := filepath.Join(root, "left")
	right := filepath.Join(root, "right")
	if err := os.MkdirAll(left, 0o755); err != nil {
		t.Fatalf("create left dir: %v", err)
	}
	if err := os.MkdirAll(right, 0o755); err != nil {
		t.Fatalf("create right dir: %v", err)
	}

	sourcePath := filepath.Join(left, "sample.txt")
	if err := os.WriteFile(sourcePath, []byte("hello xdfile"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	m := &xdfileModel{
		activePanel:     0,
		terminalFocused: true,
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: left},
			{Label: "RIGHT", Cwd: right},
		},
		terminal: xdfileTerminal{
			Session:  &xdfileTerminalPTYSession{},
			Emulator: vt.NewSafeEmulator(80, 24),
		},
	}
	for i := range m.panels {
		if err := m.reloadPanel(i); err != nil {
			t.Fatalf("reload panel %d: %v", i, err)
		}
	}
	m.panels[0].setCursor(findXdfileEntryIndex(t, m.panels[0].Entries, "sample.txt"), 8)

	cmd, handled := m.handleGlobalKey(tea.KeyMsg{Type: tea.KeyCtrlC})
	if handled {
		t.Fatalf("expected ctrl+c to pass through while PTY terminal is focused")
	}
	if cmd != nil {
		t.Fatalf("expected ctrl+c pass-through not to return a command")
	}
	if m.clipboardPath != "" {
		t.Fatalf("expected clipboard path to remain empty, got %q", m.clipboardPath)
	}

	clipboardPaths = []string{sourcePath}
	m.activePanel = 1
	cmd, handled = m.handleGlobalKey(tea.KeyMsg{Type: tea.KeyCtrlV})
	if handled {
		t.Fatalf("expected ctrl+v to pass through while PTY terminal is focused")
	}
	if cmd != nil {
		t.Fatalf("expected ctrl+v pass-through not to return a command")
	}

	targetPath := filepath.Join(right, "sample.txt")
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("expected terminal-focused ctrl+v not to paste files, stat err=%v", err)
	}

	m.activePanel = 0
	cmd, handled = m.handleGlobalKey(tea.KeyMsg{Type: tea.KeyCtrlShiftC})
	if handled {
		t.Fatalf("expected ctrl+shift+c to pass through while PTY terminal is focused")
	}
	if cmd != nil {
		t.Fatalf("expected ctrl+shift+c pass-through not to return a command")
	}

	m.activePanel = 1
	cmd, handled = m.handleGlobalKey(tea.KeyMsg{Type: tea.KeyCtrlShiftV})
	if !handled {
		t.Fatalf("expected ctrl+shift+v with file clipboard to paste while PTY terminal is focused")
	}
	if cmd != nil {
		t.Fatalf("expected ctrl+shift+v file paste to complete immediately")
	}

	targetPath = filepath.Join(right, "sample.txt")
	data, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read pasted file: %v", err)
	}
	if string(data) != "hello xdfile" {
		t.Fatalf("expected pasted file contents to match source, got %q", string(data))
	}
}

func TestHandleFilePasteEventPastesFileClipboardWhilePTYTerminalFocused(t *testing.T) {
	originalReadClipboardPaths := xdfileReadClipboardPathsFunc
	defer func() {
		xdfileReadClipboardPathsFunc = originalReadClipboardPaths
	}()

	root := t.TempDir()
	left := filepath.Join(root, "left")
	right := filepath.Join(root, "right")
	if err := os.MkdirAll(left, 0o755); err != nil {
		t.Fatalf("create left dir: %v", err)
	}
	if err := os.MkdirAll(right, 0o755); err != nil {
		t.Fatalf("create right dir: %v", err)
	}

	sourcePath := filepath.Join(left, "sample.txt")
	if err := os.WriteFile(sourcePath, []byte("hello xdfile"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	xdfileReadClipboardPathsFunc = func() ([]string, error) {
		return []string{sourcePath}, nil
	}

	m := &xdfileModel{
		activePanel:     1,
		terminalFocused: true,
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: left},
			{Label: "RIGHT", Cwd: right},
		},
		terminal: xdfileTerminal{
			Session:  &xdfileTerminalPTYSession{},
			Emulator: vt.NewSafeEmulator(80, 24),
		},
	}
	for i := range m.panels {
		if err := m.reloadPanel(i); err != nil {
			t.Fatalf("reload panel %d: %v", i, err)
		}
	}

	cmd, handled := m.handleFilePasteEvent(tea.KeyMsg{
		Type:          tea.KeyRunes,
		Runes:         []rune(sourcePath),
		Paste:         true,
		PasteShortcut: tea.KeyCtrlShiftV,
	})
	if !handled {
		t.Fatalf("expected terminal-focused paste event with file clipboard to paste into panel")
	}
	if cmd != nil {
		t.Fatalf("expected file paste event to complete immediately")
	}

	targetPath := filepath.Join(right, "sample.txt")
	data, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read pasted file: %v", err)
	}
	if string(data) != "hello xdfile" {
		t.Fatalf("expected pasted file contents to match source, got %q", string(data))
	}
}

func TestHandleFilePasteEventPassesTextPasteThroughWhilePTYTerminalFocused(t *testing.T) {
	originalReadClipboardPaths := xdfileReadClipboardPathsFunc
	defer func() {
		xdfileReadClipboardPathsFunc = originalReadClipboardPaths
	}()

	xdfileReadClipboardPathsFunc = func() ([]string, error) {
		return nil, nil
	}

	m := &xdfileModel{
		terminalFocused: true,
		terminal: xdfileTerminal{
			Session:  &xdfileTerminalPTYSession{},
			Emulator: vt.NewSafeEmulator(80, 24),
		},
	}

	cmd, handled := m.handleFilePasteEvent(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("plain text"), Paste: true})
	if handled {
		t.Fatalf("expected terminal-focused text paste event to pass through")
	}
	if cmd != nil {
		t.Fatalf("expected terminal-focused text paste event not to return a file-paste command")
	}
}

func TestHandleFilePasteEventPassesUnmatchedTextThroughWithFileClipboard(t *testing.T) {
	originalReadClipboardPaths := xdfileReadClipboardPathsFunc
	defer func() {
		xdfileReadClipboardPathsFunc = originalReadClipboardPaths
	}()

	sourcePath := filepath.Join(t.TempDir(), "sample.txt")
	if err := os.WriteFile(sourcePath, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	xdfileReadClipboardPathsFunc = func() ([]string, error) {
		return []string{sourcePath}, nil
	}

	m := &xdfileModel{
		terminalFocused: true,
		terminal: xdfileTerminal{
			Session:  &xdfileTerminalPTYSession{},
			Emulator: vt.NewSafeEmulator(80, 24),
		},
	}

	cmd, handled := m.handleFilePasteEvent(tea.KeyMsg{
		Type:          tea.KeyRunes,
		Runes:         []rune("plain text"),
		Paste:         true,
		PasteShortcut: tea.KeyCtrlShiftV,
	})
	if handled {
		t.Fatalf("expected unmatched text paste to pass through even when file clipboard exists")
	}
	if cmd != nil {
		t.Fatalf("expected unmatched text paste not to return a file-paste command")
	}
}

func TestHandleFilePasteEventPassesCtrlVFilePathPasteThrough(t *testing.T) {
	originalReadClipboardPaths := xdfileReadClipboardPathsFunc
	defer func() {
		xdfileReadClipboardPathsFunc = originalReadClipboardPaths
	}()

	sourcePath := filepath.Join(t.TempDir(), "sample.txt")
	if err := os.WriteFile(sourcePath, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	xdfileReadClipboardPathsFunc = func() ([]string, error) {
		return []string{sourcePath}, nil
	}

	m := &xdfileModel{}
	cmd, handled := m.handleFilePasteEvent(tea.KeyMsg{
		Type:          tea.KeyRunes,
		Runes:         []rune(sourcePath),
		Paste:         true,
		PasteShortcut: tea.KeyCtrlV,
	})
	if handled {
		t.Fatalf("expected ctrl+v file path paste to pass through")
	}
	if cmd != nil {
		t.Fatalf("expected ctrl+v file path paste not to return a file-paste command")
	}
}

func findXdfileEntryIndex(t *testing.T, entries []xdfileEntry, name string) int {
	t.Helper()
	for i, entry := range entries {
		if entry.Name == name {
			return i
		}
	}
	t.Fatalf("entry %q not found", name)
	return -1
}

func assertXdfilePanelSelectedPath(t *testing.T, panel xdfilePanel, path string) {
	t.Helper()
	entry, ok := panel.selected()
	if !ok {
		t.Fatalf("expected panel cursor to select %q, got no selection", path)
	}
	if !xdfilePathsEqual(entry.Path, path) {
		t.Fatalf("expected panel cursor to select %q, got %q", path, entry.Path)
	}
}

func xdfileFirstTestCommandMsg(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, child := range batch {
			if child == nil {
				continue
			}
			return child()
		}
		return nil
	}
	return msg
}

func containsXdfileLabel(labels []string, want string) bool {
	for _, label := range labels {
		if label == want {
			return true
		}
	}
	return false
}
