package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestXdfileReadEntriesSortByExtension(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"b.txt", "a.go", "c.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o644); err != nil {
			t.Fatalf("write file %q: %v", name, err)
		}
	}

	entries, err := xdfileReadEntries(dir, false, xdfileSortModeExt)
	if err != nil {
		t.Fatalf("read entries: %v", err)
	}

	got := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsParent {
			continue
		}
		got = append(got, entry.Name)
	}

	want := []string{"a.go", "c.md", "b.txt"}
	if len(got) != len(want) {
		t.Fatalf("expected %d entries, got %d (%v)", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected sorted order %v, got %v", want, got)
		}
	}
}
