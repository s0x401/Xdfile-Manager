package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUpdateAutoRefreshReloadsPanelsAfterExternalDelete(t *testing.T) {
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
	if err := os.WriteFile(sourcePath, []byte("refresh me"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	m := &xdfileModel{
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

	if err := os.Remove(sourcePath); err != nil {
		t.Fatalf("remove source file: %v", err)
	}

	m.panelDirState[0] = xdfilePanelDirState{Path: left}
	model, cmd := m.Update(xdfileAutoRefreshMsg{})
	if cmd == nil {
		t.Fatalf("expected auto refresh to reschedule itself")
	}

	updated, ok := model.(*xdfileModel)
	if !ok {
		t.Fatalf("expected updated model type *xdfileModel, got %T", model)
	}

	for _, entry := range updated.panels[0].Entries {
		if entry.Name == "sample.txt" {
			t.Fatalf("expected deleted file to disappear after auto refresh")
		}
	}
}

func TestAutoRefreshSkipsNetBoxPanels(t *testing.T) {
	oldReadEntries := xdfileNetBoxReadEntriesFunc
	defer func() { xdfileNetBoxReadEntriesFunc = oldReadEntries }()

	calls := 0
	xdfileNetBoxReadEntriesFunc = func(dir string, _ bool, _ xdfileSortMode) ([]xdfileEntry, error) {
		calls++
		return []xdfileEntry{{Name: "remote.txt", Path: xdfileNetBoxURL("prod", "/remote.txt")}}, nil
	}

	m := &xdfileModel{
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: "xdssh://prod/"},
			{Label: "RIGHT", Cwd: t.TempDir()},
		},
		panelDirState: [2]xdfilePanelDirState{
			{Path: "stale", Exists: true},
		},
	}
	model, cmd := m.Update(xdfileAutoRefreshMsg{})
	if cmd == nil {
		t.Fatalf("expected auto refresh to reschedule itself")
	}
	if _, ok := model.(*xdfileModel); !ok {
		t.Fatalf("expected updated model type *xdfileModel, got %T", model)
	}
	if calls != 0 {
		t.Fatalf("expected NetBox auto refresh to skip SSH listings, got %d calls", calls)
	}
}
