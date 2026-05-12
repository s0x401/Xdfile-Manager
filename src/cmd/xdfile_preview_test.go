package cmd

import (
	"archive/zip"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestActivateSelectionOpensFileInsteadOfPreview(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "sample.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	originalOpenPath := xdfileOpenPathFunc
	defer func() {
		xdfileOpenPathFunc = originalOpenPath
	}()

	var opened string
	xdfileOpenPathFunc = func(path string) error {
		opened = path
		return nil
	}

	m := &xdfileModel{
		activePanel: 0,
		panels: [2]xdfilePanel{
			{
				Cwd: filePath,
				Entries: []xdfileEntry{
					{Name: "sample.txt", Path: filePath},
				},
				Cursor: 0,
			},
		},
	}

	cmd := m.activateSelection()
	if cmd != nil {
		t.Fatalf("expected file activation to complete without forcing a screen clear")
	}
	if opened != filePath {
		t.Fatalf("expected file %q to be opened, got %q", filePath, opened)
	}
	if m.modal.Kind != xdfileModalNone {
		t.Fatalf("expected no preview modal when opening a file")
	}
}

func TestActivateSelectionShowsModalWhenOpenFails(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "sample.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	originalOpenPath := xdfileOpenPathFunc
	defer func() {
		xdfileOpenPathFunc = originalOpenPath
	}()

	xdfileOpenPathFunc = func(path string) error {
		return errors.New("no associated application")
	}

	m := &xdfileModel{
		activePanel: 0,
		panels: [2]xdfilePanel{
			{
				Cwd: filePath,
				Entries: []xdfileEntry{
					{Name: "sample.txt", Path: filePath},
				},
				Cursor: 0,
			},
		},
	}

	if cmd := m.activateSelection(); cmd != nil {
		t.Fatalf("expected failed open to complete without forcing a screen clear")
	}
	if m.modal.Kind != xdfileModalText {
		t.Fatalf("expected open failure to show a modal, got kind %v", m.modal.Kind)
	}
	if !strings.Contains(m.modal.Text, "no associated application") {
		t.Fatalf("expected open failure modal to include the error reason, got %q", m.modal.Text)
	}
}

func TestOpenPropertiesOpensModalForSelectedFile(t *testing.T) {
	originalShowProperties := xdfileShowSystemPropertiesFunc
	defer func() {
		xdfileShowSystemPropertiesFunc = originalShowProperties
	}()

	dir := t.TempDir()
	filePath := filepath.Join(dir, "sample.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	var opened string
	xdfileShowSystemPropertiesFunc = func(path string) error {
		opened = path
		return nil
	}

	m := &xdfileModel{
		activePanel: 0,
		panels: [2]xdfilePanel{
			{
				Cwd: dir,
				Entries: []xdfileEntry{
					{Name: "sample.txt", Path: filePath},
				},
				Cursor: 0,
			},
		},
	}

	if cmd := m.openProperties(); cmd != nil {
		t.Fatalf("expected properties dialog launch to complete immediately")
	}
	if opened != filePath {
		t.Fatalf("expected Windows properties to be opened for %q, got %q", filePath, opened)
	}
	if m.modal.Kind != xdfileModalNone {
		t.Fatalf("expected properties action not to open an in-app modal, got %v", m.modal.Kind)
	}
}

func TestTogglePreviewOpensAndClosesPreview(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "preview.txt")
	if err := os.WriteFile(filePath, []byte("line1\nline2\nline3\n"), 0o644); err != nil {
		t.Fatalf("write preview file: %v", err)
	}

	m := &xdfileModel{
		width:       120,
		height:      40,
		activePanel: 0,
		panels: [2]xdfilePanel{
			{
				Cwd: dir,
				Entries: []xdfileEntry{
					{Name: "preview.txt", Path: filePath},
				},
				Cursor: 0,
			},
		},
	}

	if cmd := m.togglePreview(); cmd != nil {
		t.Fatalf("expected preview toggle to complete immediately")
	}
	if m.modal.Kind != xdfileModalText {
		t.Fatalf("expected preview modal to open, got %v", m.modal.Kind)
	}
	if m.modal.Action != xdfileActionPreview {
		t.Fatalf("expected preview action, got %q", m.modal.Action)
	}

	if cmd := m.handleModalKey(tea.KeyMsg{Type: tea.KeyCtrlQ}); cmd != nil {
		t.Fatalf("expected preview close to complete immediately")
	}
	if m.modal.Kind != xdfileModalNone {
		t.Fatalf("expected ctrl+q to close the preview modal")
	}
}

func TestPreviewModalMouseScroll(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "preview.txt")

	var content strings.Builder
	for i := 0; i < 80; i++ {
		content.WriteString("line\n")
	}
	if err := os.WriteFile(filePath, []byte(content.String()), 0o644); err != nil {
		t.Fatalf("write preview file: %v", err)
	}

	m := &xdfileModel{
		width:       120,
		height:      40,
		activePanel: 0,
		panels: [2]xdfilePanel{
			{
				Cwd: dir,
				Entries: []xdfileEntry{
					{Name: "preview.txt", Path: filePath},
				},
				Cursor: 0,
			},
		},
	}

	if cmd := m.togglePreview(); cmd != nil {
		t.Fatalf("expected preview toggle to complete immediately")
	}

	rect := m.modalRect()
	x := rect.x + 2
	y := rect.y + 3

	m.handleModalMouse(tea.MouseMsg{X: x, Y: y, Button: tea.MouseButtonWheelDown})
	if m.modal.Viewport.YOffset == 0 {
		t.Fatalf("expected mouse wheel down to scroll preview")
	}

	offsetAfterWheel := m.modal.Viewport.YOffset
	m.handleModalMouse(tea.MouseMsg{X: x, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	if m.modal.Viewport.YOffset >= offsetAfterWheel {
		t.Fatalf("expected left click to scroll preview up")
	}
}

func TestActivateSelectionPropagatesOpenErrors(t *testing.T) {
	originalOpenPath := xdfileOpenPathFunc
	defer func() {
		xdfileOpenPathFunc = originalOpenPath
	}()

	wantErr := errors.New("open failed")
	xdfileOpenPathFunc = func(path string) error {
		return wantErr
	}

	m := &xdfileModel{
		activePanel: 0,
		panels: [2]xdfilePanel{
			{
				Entries: []xdfileEntry{
					{Name: "sample.txt", Path: "sample.txt"},
				},
				Cursor: 0,
			},
		},
	}

	_ = m.activateSelection()
	if !m.statusError {
		t.Fatalf("expected open failure to set error status")
	}
	if !strings.Contains(m.statusText, wantErr.Error()) {
		t.Fatalf("expected status to contain %q, got %q", wantErr.Error(), m.statusText)
	}
}

func TestActivateSelectionParentRestoresCursorToChildDirectory(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "xx")
	child := filepath.Join(parent, "cc")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatalf("create child dir: %v", err)
	}

	m := &xdfileModel{
		activePanel: 0,
		layout: xdfileLayout{
			panelRects: [2]xdfileRect{{h: 12}, {h: 12}},
		},
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: parent},
		},
	}
	if err := m.reloadPanel(0); err != nil {
		t.Fatalf("reload parent panel: %v", err)
	}

	m.panels[0].setCursor(findXdfileEntryIndex(t, m.panels[0].Entries, "cc"), 8)
	if cmd := m.activateSelection(); cmd != nil {
		t.Fatalf("expected child directory activation to complete immediately")
	}
	if !xdfilePathsEqual(m.panels[0].Cwd, child) {
		t.Fatalf("expected panel cwd to switch to child, got %q", m.panels[0].Cwd)
	}
	if entry, ok := m.panels[0].selected(); !ok || !entry.IsParent {
		t.Fatalf("expected child directory to open with parent entry selected, got %+v", entry)
	}

	if cmd := m.activateSelection(); cmd != nil {
		t.Fatalf("expected parent entry activation to complete immediately")
	}
	if !xdfilePathsEqual(m.panels[0].Cwd, parent) {
		t.Fatalf("expected panel cwd to return to parent, got %q", m.panels[0].Cwd)
	}
	if entry, ok := m.panels[0].selected(); !ok || entry.Name != "cc" {
		t.Fatalf("expected cursor to restore to child directory, got %+v", entry)
	}
}

func TestActivateSelectionChildDirectoryResetsCursorToFirstRow(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "xx")
	child := filepath.Join(parent, "cc")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatalf("create child dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(parent, "aa.txt"), []byte("a"), 0o644); err != nil {
		t.Fatalf("write parent file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(parent, "zz.txt"), []byte("z"), 0o644); err != nil {
		t.Fatalf("write parent file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(child, "inside.txt"), []byte("inside"), 0o644); err != nil {
		t.Fatalf("write child file: %v", err)
	}

	m := &xdfileModel{
		activePanel: 0,
		layout: xdfileLayout{
			panelRects: [2]xdfileRect{{h: 12}, {h: 12}},
		},
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: parent},
		},
	}
	if err := m.reloadPanel(0); err != nil {
		t.Fatalf("reload parent panel: %v", err)
	}

	m.panels[0].setCursor(findXdfileEntryIndex(t, m.panels[0].Entries, "cc"), 8)
	if cmd := m.activateSelection(); cmd != nil {
		t.Fatalf("expected child directory activation to complete immediately")
	}
	if !xdfilePathsEqual(m.panels[0].Cwd, child) {
		t.Fatalf("expected panel cwd to switch to child, got %q", m.panels[0].Cwd)
	}
	if entry, ok := m.panels[0].selected(); !ok || !entry.IsParent {
		t.Fatalf("expected child directory activation to reset cursor to the first row, got %+v", entry)
	}
	if m.panels[0].Cursor != 0 {
		t.Fatalf("expected child directory activation to place the cursor at row 0, got %d", m.panels[0].Cursor)
	}
}

func TestExecuteActionParentRestoresCursorToChildDirectory(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "xx")
	child := filepath.Join(parent, "cc")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatalf("create child dir: %v", err)
	}

	m := &xdfileModel{
		activePanel: 0,
		layout: xdfileLayout{
			panelRects: [2]xdfileRect{{h: 12}, {h: 12}},
		},
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: child},
		},
	}
	if err := m.reloadPanel(0); err != nil {
		t.Fatalf("reload child panel: %v", err)
	}

	if entry, ok := m.panels[0].selected(); !ok || !entry.IsParent {
		t.Fatalf("expected child directory to start with parent entry selected, got %+v", entry)
	}
	if cmd := m.executeAction(xdfileActionParent); cmd != nil {
		t.Fatalf("expected parent action to complete immediately")
	}
	if !xdfilePathsEqual(m.panels[0].Cwd, parent) {
		t.Fatalf("expected panel cwd to return to parent, got %q", m.panels[0].Cwd)
	}
	if entry, ok := m.panels[0].selected(); !ok || entry.Name != "cc" {
		t.Fatalf("expected parent action to restore the child directory cursor, got %+v", entry)
	}
}

func TestXdfileReadPreviewBinaryFallback(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "data.bin")
	if err := os.WriteFile(filePath, []byte{0x00, 0x41, 0xFF, 0x10}, 0o644); err != nil {
		t.Fatalf("write binary file: %v", err)
	}

	preview, err := xdfileReadPreview(filePath)
	if err != nil {
		t.Fatalf("read preview: %v", err)
	}

	if !strings.Contains(preview, "Hex dump:") {
		t.Fatalf("expected hex preview dump, got %q", preview)
	}
	if !strings.Contains(preview, "00 41 FF 10") {
		t.Fatalf("expected hex bytes in preview, got %q", preview)
	}
}

func TestXdfileReadPreviewImageMetadata(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "sample.png")

	img := image.NewRGBA(image.Rect(0, 0, 2, 3))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	file, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("create png file: %v", err)
	}
	if err := png.Encode(file, img); err != nil {
		_ = file.Close()
		t.Fatalf("encode png: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close png file: %v", err)
	}

	preview, err := xdfileReadPreview(filePath)
	if err != nil {
		t.Fatalf("read preview: %v", err)
	}

	if !strings.Contains(preview, "PNG image") {
		t.Fatalf("expected png image preview, got %q", preview)
	}
	if !strings.Contains(preview, "Dimensions:2 x 3") {
		t.Fatalf("expected dimensions in preview, got %q", preview)
	}
}

func TestXdfileReadPreviewArchiveEntries(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "archive.zip")

	file, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("create zip file: %v", err)
	}
	zipWriter := zip.NewWriter(file)
	for _, name := range []string{"a.txt", "folder/", "folder/b.txt"} {
		writer, createErr := zipWriter.Create(name)
		if createErr != nil {
			t.Fatalf("create zip entry %q: %v", name, createErr)
		}
		if !strings.HasSuffix(name, "/") {
			if _, writeErr := writer.Write([]byte(name)); writeErr != nil {
				t.Fatalf("write zip entry %q: %v", name, writeErr)
			}
		}
	}
	if err := zipWriter.Close(); err != nil {
		_ = file.Close()
		t.Fatalf("close zip writer: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close zip file: %v", err)
	}

	preview, err := xdfileReadPreview(filePath)
	if err != nil {
		t.Fatalf("read preview: %v", err)
	}

	if !strings.Contains(preview, "ZIP archive") {
		t.Fatalf("expected archive type in preview, got %q", preview)
	}
	if !strings.Contains(preview, "folder/b.txt") {
		t.Fatalf("expected zip entries in preview, got %q", preview)
	}
}

func TestTogglePreviewBinaryMode(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "preview.txt")
	if err := os.WriteFile(filePath, []byte("abc"), 0o644); err != nil {
		t.Fatalf("write preview file: %v", err)
	}

	m := &xdfileModel{
		width:       120,
		height:      40,
		activePanel: 0,
		panels: [2]xdfilePanel{
			{
				Cwd: dir,
				Entries: []xdfileEntry{
					{Name: "preview.txt", Path: filePath},
				},
				Cursor: 0,
			},
		},
	}

	if cmd := m.openPreview(); cmd != nil {
		t.Fatalf("expected preview open to complete immediately")
	}
	if m.modal.PreviewBinary {
		t.Fatalf("expected preview to start in normal mode")
	}

	if cmd := m.handleModalKey(tea.KeyMsg{Type: tea.KeyCtrlB}); cmd != nil {
		t.Fatalf("expected ctrl+b binary toggle to complete immediately")
	}
	if !m.modal.PreviewBinary {
		t.Fatalf("expected preview to switch to binary mode")
	}
	if !strings.Contains(m.modal.Text, "Hex dump:") {
		t.Fatalf("expected hex dump after toggle, got %q", m.modal.Text)
	}
}

func TestBinaryPreviewToggleIgnoredForBinaryOnlyFiles(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "sample.bin")
	if err := os.WriteFile(filePath, []byte{0x00, 0x41, 0xFF, 0x10}, 0o644); err != nil {
		t.Fatalf("write binary file: %v", err)
	}

	m := &xdfileModel{
		width:       120,
		height:      40,
		activePanel: 0,
		panels: [2]xdfilePanel{
			{
				Cwd: dir,
				Entries: []xdfileEntry{
					{Name: "sample.bin", Path: filePath},
				},
				Cursor: 0,
			},
		},
	}

	if cmd := m.openPreview(); cmd != nil {
		t.Fatalf("expected preview open to complete immediately")
	}
	beforeText := m.modal.Text
	beforeDescription := m.modal.Description

	if cmd := m.handleModalKey(tea.KeyMsg{Type: tea.KeyCtrlB}); cmd != nil {
		t.Fatalf("expected ctrl+b binary toggle key to complete immediately")
	}
	if m.modal.PreviewBinary {
		t.Fatalf("expected binary-only preview to ignore binary toggle")
	}
	if m.modal.Text != beforeText {
		t.Fatalf("expected binary-only preview content to remain unchanged")
	}
	if m.modal.Description != beforeDescription {
		t.Fatalf("expected binary-only preview description to remain unchanged")
	}
}

func TestQuickViewBinaryToggleUsesCtrlBOnly(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "preview.txt")
	if err := os.WriteFile(filePath, []byte("hello\nworld\n"), 0o644); err != nil {
		t.Fatalf("write preview file: %v", err)
	}

	m := &xdfileModel{
		width:       120,
		height:      40,
		activePanel: 0,
		layoutPrefs: xdfileLayoutPrefs{QuickViewDocked: true},
		panels: [2]xdfilePanel{
			{
				Label: "LEFT",
				Cwd:   dir,
				Entries: []xdfileEntry{
					{Name: "preview.txt", Path: filePath},
				},
				Cursor: 0,
			},
			{Label: "RIGHT", Cwd: dir},
		},
	}
	m.computeLayout()

	if cmd := m.openQuickView(); cmd != nil {
		t.Fatalf("expected quick view open to complete immediately")
	}
	if m.quickView.Binary {
		t.Fatalf("expected quick view to start in normal mode")
	}

	if cmd := m.handlePanelKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}}); cmd != nil {
		t.Fatalf("expected bare b not to toggle quick view binary mode")
	}
	if m.quickView.Binary {
		t.Fatalf("expected bare b to leave quick view in normal mode")
	}

	if cmd := m.handlePanelKey(tea.KeyMsg{Type: tea.KeyCtrlB}); cmd != nil {
		t.Fatalf("expected ctrl+b to toggle quick view binary mode immediately")
	}
	if !m.quickView.Binary {
		t.Fatalf("expected ctrl+b to switch quick view to binary mode")
	}
}

func TestOpenPreviewUsesVisualThumbnailWhenAvailable(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "sample.png")
	if err := os.WriteFile(filePath, []byte("not-really-a-png"), 0o644); err != nil {
		t.Fatalf("write fake png file: %v", err)
	}

	originalRenderPreviewThumbnail := xdfileRenderPreviewThumbnailFunc
	defer func() {
		xdfileRenderPreviewThumbnailFunc = originalRenderPreviewThumbnail
	}()

	var called bool
	xdfileRenderPreviewThumbnailFunc = func(m *xdfileModel, path string, width int, height int) (string, bool, error) {
		called = true
		if path != filePath {
			t.Fatalf("expected thumbnail render path %q, got %q", filePath, path)
		}
		return "thumb-line-1\nthumb-line-2", true, nil
	}

	m := &xdfileModel{
		width:       120,
		height:      40,
		activePanel: 0,
		panels: [2]xdfilePanel{
			{
				Cwd: dir,
				Entries: []xdfileEntry{
					{Name: "sample.png", Path: filePath},
				},
				Cursor: 0,
			},
		},
	}

	if cmd := m.openPreview(); cmd != nil {
		t.Fatalf("expected preview open to complete immediately")
	}
	if !called {
		t.Fatalf("expected thumbnail renderer to be called")
	}
	if !m.modal.PreviewVisual {
		t.Fatalf("expected image preview to use visual thumbnail mode")
	}
	if m.modal.Text != "thumb-line-1\nthumb-line-2" {
		t.Fatalf("expected rendered thumbnail to be stored, got %q", m.modal.Text)
	}
}

func TestTogglePreviewUsesDockedQuickViewWhenEnabled(t *testing.T) {
	dir := t.TempDir()
	leftDir := filepath.Join(dir, "left")
	rightDir := filepath.Join(dir, "right")
	if err := os.MkdirAll(leftDir, 0o755); err != nil {
		t.Fatalf("create left dir: %v", err)
	}
	if err := os.MkdirAll(rightDir, 0o755); err != nil {
		t.Fatalf("create right dir: %v", err)
	}

	filePath := filepath.Join(rightDir, "preview.txt")
	if err := os.WriteFile(filePath, []byte("line1\nline2\n"), 0o644); err != nil {
		t.Fatalf("write preview file: %v", err)
	}

	m := &xdfileModel{
		width:       120,
		height:      40,
		activePanel: 1,
		layoutPrefs: xdfileLayoutPrefs{QuickViewDocked: true},
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: leftDir},
			{
				Label: "RIGHT",
				Cwd:   rightDir,
				Entries: []xdfileEntry{
					{Name: "preview.txt", Path: filePath},
				},
				Cursor: 0,
			},
		},
	}
	m.computeLayout()

	if cmd := m.togglePreview(); cmd != nil {
		t.Fatalf("expected quick view toggle to complete immediately")
	}
	if !m.quickView.Open {
		t.Fatalf("expected docked quick view to open")
	}
	if m.modal.Kind != xdfileModalNone {
		t.Fatalf("expected docked quick view not to open modal preview")
	}
	m.syncQuickViewViewport()
	if !strings.Contains(m.quickView.Text, "line1") {
		t.Fatalf("expected quick view text preview to be populated, got %q", m.quickView.Text)
	}
}

func TestQuickViewTracksSelectionChanges(t *testing.T) {
	dir := t.TempDir()
	leftDir := filepath.Join(dir, "left")
	rightDir := filepath.Join(dir, "right")
	if err := os.MkdirAll(leftDir, 0o755); err != nil {
		t.Fatalf("create left dir: %v", err)
	}
	if err := os.MkdirAll(rightDir, 0o755); err != nil {
		t.Fatalf("create right dir: %v", err)
	}

	firstPath := filepath.Join(rightDir, "a.txt")
	secondPath := filepath.Join(rightDir, "b.txt")
	if err := os.WriteFile(firstPath, []byte("alpha"), 0o644); err != nil {
		t.Fatalf("write first preview file: %v", err)
	}
	if err := os.WriteFile(secondPath, []byte("beta"), 0o644); err != nil {
		t.Fatalf("write second preview file: %v", err)
	}

	m := &xdfileModel{
		width:       120,
		height:      40,
		activePanel: 1,
		layoutPrefs: xdfileLayoutPrefs{QuickViewDocked: true},
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: leftDir},
			{
				Label: "RIGHT",
				Cwd:   rightDir,
				Entries: []xdfileEntry{
					{Name: "a.txt", Path: firstPath},
					{Name: "b.txt", Path: secondPath},
				},
				Cursor: 0,
			},
		},
	}
	m.computeLayout()

	if cmd := m.openQuickView(); cmd != nil {
		t.Fatalf("expected quick view open to complete immediately")
	}
	m.syncQuickViewViewport()
	if m.quickView.Path != firstPath {
		t.Fatalf("expected quick view path %q, got %q", firstPath, m.quickView.Path)
	}

	if cmd := m.handlePanelKey(tea.KeyMsg{Type: tea.KeyDown}); cmd != nil {
		t.Fatalf("expected cursor move to complete immediately")
	}
	if m.quickView.Path != secondPath {
		t.Fatalf("expected quick view path to follow selection %q, got %q", secondPath, m.quickView.Path)
	}
	if !strings.Contains(m.quickView.Text, "beta") {
		t.Fatalf("expected quick view content to refresh to second file, got %q", m.quickView.Text)
	}
}

func TestQuickViewPDFSkipsVisualThumbnail(t *testing.T) {
	dir := t.TempDir()
	leftDir := filepath.Join(dir, "left")
	rightDir := filepath.Join(dir, "right")
	if err := os.MkdirAll(leftDir, 0o755); err != nil {
		t.Fatalf("create left dir: %v", err)
	}
	if err := os.MkdirAll(rightDir, 0o755); err != nil {
		t.Fatalf("create right dir: %v", err)
	}

	filePath := filepath.Join(rightDir, "report.pdf")
	if err := os.WriteFile(filePath, []byte("%PDF-1.7\n1 0 obj\n<< /Title (Report) >>\nendobj\n"), 0o644); err != nil {
		t.Fatalf("write pdf fixture: %v", err)
	}

	originalRenderPreviewThumbnail := xdfileRenderPreviewThumbnailFunc
	defer func() {
		xdfileRenderPreviewThumbnailFunc = originalRenderPreviewThumbnail
	}()
	xdfileRenderPreviewThumbnailFunc = func(_ *xdfileModel, path string, _ int, _ int) (string, bool, error) {
		t.Fatalf("quick view should not render PDF visual thumbnail, got %q", path)
		return "", false, nil
	}

	m := &xdfileModel{
		width:       120,
		height:      40,
		activePanel: 1,
		layoutPrefs: xdfileLayoutPrefs{QuickViewDocked: true},
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: leftDir},
			{
				Label: "RIGHT",
				Cwd:   rightDir,
				Entries: []xdfileEntry{
					{Name: "report.pdf", Path: filePath},
				},
				Cursor: 0,
			},
		},
	}
	m.computeLayout()
	if cmd := m.openQuickView(); cmd != nil {
		t.Fatalf("expected quick view open to complete immediately")
	}
	m.syncQuickViewViewport()

	if m.quickView.Visual {
		t.Fatalf("expected quick view PDF to use text summary instead of visual thumbnail")
	}
	if !strings.Contains(m.quickView.Text, "PDF document") {
		t.Fatalf("expected quick view PDF summary, got %q", m.quickView.Text)
	}
}

func TestPDFPreviewSanitizesMetadataControlBytes(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "control.pdf")
	utf16Creator := []byte{0xfe, 0xff, 0x00, 'W', 0x00, 'P', 0x00, 'S', 0x00, ' ', 0x00, 'O', 0x00, 'f', 0x00, 'f', 0x00, 'i', 0x00, 'c', 0x00, 'e'}
	data := append([]byte("%PDF-1.7\n/Title (bad\x1b[2J\rname)\n/Creator ("), utf16Creator...)
	data = append(data, []byte(")\n")...)
	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		t.Fatalf("write pdf fixture: %v", err)
	}
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("stat pdf fixture: %v", err)
	}

	preview := xdfileReadPDFPreview(filePath, info, data, "application/pdf")
	if strings.Contains(preview, "\x1b[2J") || strings.ContainsAny(preview, "\x00\r\b\f") {
		t.Fatalf("expected PDF metadata controls to be sanitized, got %q", preview)
	}
	if !strings.Contains(preview, "Title:") || !strings.Contains(preview, "bad name") {
		t.Fatalf("expected sanitized title metadata, got %q", preview)
	}
	if !strings.Contains(preview, "Creator:") || !strings.Contains(preview, "WPS Office") {
		t.Fatalf("expected decoded UTF-16 creator metadata, got %q", preview)
	}
}

func TestQuickViewUsesPassivePanelSide(t *testing.T) {
	dir := t.TempDir()
	leftDir := filepath.Join(dir, "left")
	rightDir := filepath.Join(dir, "right")
	if err := os.MkdirAll(leftDir, 0o755); err != nil {
		t.Fatalf("create left dir: %v", err)
	}
	if err := os.MkdirAll(rightDir, 0o755); err != nil {
		t.Fatalf("create right dir: %v", err)
	}

	leftFile := filepath.Join(leftDir, "left.txt")
	rightFile := filepath.Join(rightDir, "right.txt")
	if err := os.WriteFile(leftFile, []byte("left-side"), 0o644); err != nil {
		t.Fatalf("write left preview file: %v", err)
	}
	if err := os.WriteFile(rightFile, []byte("right-side"), 0o644); err != nil {
		t.Fatalf("write right preview file: %v", err)
	}

	m := &xdfileModel{
		width:       120,
		height:      40,
		layoutPrefs: xdfileLayoutPrefs{QuickViewDocked: true},
		panels: [2]xdfilePanel{
			{
				Label: "LEFT",
				Cwd:   leftDir,
				Entries: []xdfileEntry{
					{Name: "left.txt", Path: leftFile},
				},
				Cursor: 0,
			},
			{
				Label: "RIGHT",
				Cwd:   rightDir,
				Entries: []xdfileEntry{
					{Name: "right.txt", Path: rightFile},
				},
				Cursor: 0,
			},
		},
	}
	m.computeLayout()

	m.activePanel = 0
	if cmd := m.openQuickView(); cmd != nil {
		t.Fatalf("expected quick view open on left selection to complete immediately")
	}
	if m.quickViewPanelIndex() != 1 {
		t.Fatalf("expected passive quick view panel to be right when left is active, got %d", m.quickViewPanelIndex())
	}

	m.closeQuickView()
	m.activePanel = 1
	if cmd := m.openQuickView(); cmd != nil {
		t.Fatalf("expected quick view open on right selection to complete immediately")
	}
	if m.quickViewPanelIndex() != 0 {
		t.Fatalf("expected passive quick view panel to be left when right is active, got %d", m.quickViewPanelIndex())
	}
}
