package cmd

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestPanelSearchAltRuneStartsAndMovesCursor(t *testing.T) {
	m := newPanelSearchTestModel([]xdfileEntry{
		{Name: "..", IsDir: true, IsParent: true},
		{Name: "alpha.txt", Path: filepath.Join("C:\\left", "alpha.txt")},
		{Name: "query.go", Path: filepath.Join("C:\\left", "query.go")},
	})

	sendPanelSearchTestKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}, Alt: true})
	if !m.panelSearch.Active || m.panelSearch.Pattern != "q" {
		t.Fatalf("expected active search pattern q, got active=%v pattern=%q", m.panelSearch.Active, m.panelSearch.Pattern)
	}
	if got := m.panels[0].Cursor; got != 2 {
		t.Fatalf("expected cursor on query.go, got %d", got)
	}
}

func TestPanelSearchExtendsAndRejectsNoMatch(t *testing.T) {
	m := newPanelSearchTestModel([]xdfileEntry{
		{Name: "alpha.txt", Path: filepath.Join("C:\\left", "alpha.txt")},
		{Name: "alpine.txt", Path: filepath.Join("C:\\left", "alpine.txt")},
	})

	sendPanelSearchTestKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}, Alt: true})
	sendPanelSearchTestKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	sendPanelSearchTestKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})

	if got := m.panelSearch.Pattern; got != "al" {
		t.Fatalf("expected failed extension to keep pattern al, got %q", got)
	}
	if got := m.panels[0].Cursor; got != 0 {
		t.Fatalf("expected cursor to stay on alpha.txt, got %d", got)
	}
	if !strings.Contains(m.statusText, "Search not found") {
		t.Fatalf("expected not found status, got %q", m.statusText)
	}
}

func TestPanelSearchSlashMatchesDirectoriesOnly(t *testing.T) {
	m := newPanelSearchTestModel([]xdfileEntry{
		{Name: "apps.txt", Path: filepath.Join("C:\\left", "apps.txt")},
		{Name: "app", Path: filepath.Join("C:\\left", "app"), IsDir: true},
	})

	sendPanelSearchTestKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}, Alt: true})
	sendPanelSearchTestKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})

	if got := m.panelSearch.Pattern; got != "a/" {
		t.Fatalf("expected directory search pattern a/, got %q", got)
	}
	if got := m.panels[0].Cursor; got != 1 {
		t.Fatalf("expected cursor on app directory, got %d", got)
	}
}

func TestPanelSearchNextAndPreviousWrap(t *testing.T) {
	m := newPanelSearchTestModel([]xdfileEntry{
		{Name: "query-one.txt", Path: filepath.Join("C:\\left", "query-one.txt")},
		{Name: "alpha.txt", Path: filepath.Join("C:\\left", "alpha.txt")},
		{Name: "query-two.txt", Path: filepath.Join("C:\\left", "query-two.txt")},
	})

	sendPanelSearchTestKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}, Alt: true})
	sendPanelSearchTestKey(t, m, tea.KeyMsg{Type: tea.KeyCtrlN})
	if got := m.panels[0].Cursor; got != 2 {
		t.Fatalf("expected ctrl+n to move to second query match, got %d", got)
	}
	sendPanelSearchTestKey(t, m, tea.KeyMsg{Type: tea.KeyCtrlP})
	if got := m.panels[0].Cursor; got != 0 {
		t.Fatalf("expected ctrl+p to move back to first query match, got %d", got)
	}
}

func TestPanelSearchBackspaceOnEmptyKeepsSearchOpen(t *testing.T) {
	m := newPanelSearchTestModel([]xdfileEntry{
		{Name: "query.go", Path: filepath.Join("C:\\left", "query.go")},
	})

	sendPanelSearchTestKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}, Alt: true})
	sendPanelSearchTestKey(t, m, tea.KeyMsg{Type: tea.KeyBackspace})
	sendPanelSearchTestKey(t, m, tea.KeyMsg{Type: tea.KeyBackspace})

	if !m.panelSearch.Active {
		t.Fatalf("expected empty-search backspace to keep search open")
	}
	if got := m.panelSearch.Pattern; got != "" {
		t.Fatalf("expected empty search pattern, got %q", got)
	}
}

func TestRenderPanelSearchOverlay(t *testing.T) {
	xdfileApplyTheme(xdfileThemeByName(""))
	m := newPanelSearchTestModel([]xdfileEntry{
		{Name: "query.go", Path: filepath.Join("C:\\left", "query.go")},
	})
	sendPanelSearchTestKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}, Alt: true})

	rendered := stripXdfileANSI(m.renderPanel(0))
	if !strings.Contains(rendered, "Search") || !strings.Contains(rendered, "q") {
		t.Fatalf("expected panel search overlay to render title and pattern, got %q", rendered)
	}
}

func newPanelSearchTestModel(entries []xdfileEntry) *xdfileModel {
	return &xdfileModel{
		activePanel: 0,
		panels: [2]xdfilePanel{
			{
				Label:       "LEFT",
				Cwd:         `C:\left`,
				Entries:     entries,
				RangeAnchor: -1,
			},
			{
				Label:       "RIGHT",
				Cwd:         `C:\right`,
				Entries:     []xdfileEntry{},
				RangeAnchor: -1,
			},
		},
		layout: xdfileLayout{
			panelRects: [2]xdfileRect{
				{x: 0, y: 0, w: 72, h: 14},
				{x: 73, y: 0, w: 72, h: 14},
			},
		},
		panelSearch: xdfilePanelSearchState{Panel: -1},
	}
}

func sendPanelSearchTestKey(t *testing.T, m *xdfileModel, msg tea.KeyMsg) {
	t.Helper()
	_, _ = m.Update(msg)
}
