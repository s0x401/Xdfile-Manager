package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestXdfileParseGitStatusOutputBuildsBranchAndMarkers(t *testing.T) {
	info := xdfileParseGitStatusOutput("## main...origin/main [ahead 2, behind 1]\n M file.txt\n?? new.txt\n D dir/nested.txt\n")

	if !info.Active {
		t.Fatalf("expected git snapshot to be active")
	}
	if info.Branch != "main" {
		t.Fatalf("expected branch %q, got %q", "main", info.Branch)
	}
	if info.Ahead != 2 || info.Behind != 1 {
		t.Fatalf("expected ahead/behind 2/1, got %d/%d", info.Ahead, info.Behind)
	}
	if info.Markers["file.txt"] != "M" {
		t.Fatalf("expected modified marker for file.txt, got %q", info.Markers["file.txt"])
	}
	if info.Markers["new.txt"] != "?" {
		t.Fatalf("expected untracked marker for new.txt, got %q", info.Markers["new.txt"])
	}
	if info.Markers["dir"] != "D" {
		t.Fatalf("expected nested delete to mark top-level dir, got %q", info.Markers["dir"])
	}
}

func TestReloadPanelAppliesGitStatusMarkers(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "tracked.txt")
	if err := os.WriteFile(filePath, []byte("payload"), 0o644); err != nil {
		t.Fatalf("write tracked file: %v", err)
	}

	originalReadGitStatus := xdfileReadGitStatusFunc
	defer func() {
		xdfileReadGitStatusFunc = originalReadGitStatus
	}()

	xdfileReadGitStatusFunc = func(path string) xdfileGitPanelInfo {
		return xdfileGitPanelInfo{
			Active: true,
			Branch: "main",
			Dirty:  true,
			Markers: map[string]string{
				"tracked.txt": "M",
			},
		}
	}

	m := &xdfileModel{
		panels: [2]xdfilePanel{
			{Label: "LEFT", Cwd: dir},
		},
	}

	if err := m.reloadPanel(0); err != nil {
		t.Fatalf("reload panel: %v", err)
	}

	index := findXdfileEntryIndex(t, m.panels[0].Entries, "tracked.txt")
	if got := m.panels[0].Entries[index].GitMarker; got != "M" {
		t.Fatalf("expected git marker %q on tracked.txt, got %q", "M", got)
	}
	if got := m.panels[0].Git.TitleLabel(); got != "git:main*" {
		t.Fatalf("expected git title label %q, got %q", "git:main*", got)
	}
}
