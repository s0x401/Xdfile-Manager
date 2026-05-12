package cmd

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestOpenCommandItemFormAfterConfirmKeepsModalInputInitialized(t *testing.T) {
	m := &xdfileModel{}

	if cmd := m.openQuitConfirm(); cmd != nil {
		t.Fatalf("expected quit confirm to complete immediately")
	}

	m.openCommandItemForm(xdfileCommandItemTypeCommand)

	if m.modal.Kind != xdfileModalForm {
		t.Fatalf("expected command form modal, got kind %v", m.modal.Kind)
	}
	if len(m.modal.FormFields) != 3 {
		t.Fatalf("expected command form fields, got %d", len(m.modal.FormFields))
	}
	if m.modal.Input.Cursor.BlinkSpeed <= 0 {
		t.Fatalf("expected initialized modal input cursor, got blink speed %v", m.modal.Input.Cursor.BlinkSpeed)
	}
	if m.modal.FormFields[0].Input.Cursor.BlinkSpeed <= 0 {
		t.Fatalf("expected initialized form field cursor, got blink speed %v", m.modal.FormFields[0].Input.Cursor.BlinkSpeed)
	}
	if !m.modal.FormFields[0].Focused() {
		t.Fatalf("expected first form field to be focused")
	}
}

func TestOpenInputModalAfterConfirmKeepsModalInputInitialized(t *testing.T) {
	m := &xdfileModel{}

	if cmd := m.openQuitConfirm(); cmd != nil {
		t.Fatalf("expected quit confirm to complete immediately")
	}

	m.openInputModal(xdfileActionModalRename, "Rename Item", "", 0, `C:\work\demo.txt`, "demo.txt")

	if m.modal.Kind != xdfileModalInput {
		t.Fatalf("expected input modal, got kind %v", m.modal.Kind)
	}
	if m.modal.Input.Cursor.BlinkSpeed <= 0 {
		t.Fatalf("expected initialized modal input cursor, got blink speed %v", m.modal.Input.Cursor.BlinkSpeed)
	}
	if !m.modal.Input.Focused() {
		t.Fatalf("expected modal input to be focused")
	}
	if got := m.modal.Input.Value(); got != "demo.txt" {
		t.Fatalf("expected modal input value to be preserved, got %q", got)
	}
}

func TestApplyModalRenameRejectsInvalidPanelIndex(t *testing.T) {
	m := &xdfileModel{
		modal: xdfileModal{
			Action:     xdfileActionModalRename,
			PanelIndex: -1,
		},
	}

	if cmd := m.applyModal(); cmd != nil {
		t.Fatalf("expected invalid panel rejection to complete immediately")
	}
	if m.statusText != "Invalid panel" {
		t.Fatalf("expected invalid panel status, got %q", m.statusText)
	}
}

func TestHandleMenuKeyEnterIgnoresStaleCursor(t *testing.T) {
	m := &xdfileModel{
		openMenu:   xdfileActionContextMenu,
		menuCursor: 99,
		contextMenu: xdfileMenu{
			Action: xdfileActionContextMenu,
			Label:  "Context",
			Items: []xdfileButton{
				{Action: xdfileActionOpen, Label: "Open", Disabled: true},
			},
		},
	}

	if cmd, handled := m.handleMenuKey(tea.KeyMsg{Type: tea.KeyEnter}); !handled || cmd != nil {
		t.Fatalf("expected stale disabled menu cursor to be handled without command, handled=%v cmd=%v", handled, cmd)
	}
	if m.menuCursor != -1 {
		t.Fatalf("expected menu cursor to mark no selectable item, got %d", m.menuCursor)
	}
}

func TestHandleMenuKeyEscClosesUserMenuWithoutDuplicateMenu(t *testing.T) {
	m := &xdfileModel{
		openMenu: xdfileActionCommandsMenu,
	}

	if cmd, handled := m.handleMenuKey(tea.KeyMsg{Type: tea.KeyEsc}); !handled || cmd != nil {
		t.Fatalf("expected Esc to close user menu immediately, handled=%v cmd=%v", handled, cmd)
	}
	if m.openMenu != "" {
		t.Fatalf("expected user menu to close, got %q", m.openMenu)
	}
	if m.statusText != "Closed User menu" {
		t.Fatalf("expected non-duplicated close status, got %q", m.statusText)
	}
}

func TestXdfileMenuStatusLabelAvoidsDuplicateMenuSuffix(t *testing.T) {
	cases := map[string]string{
		"":           "menu",
		"Panels":     "Panels menu",
		"User menu":  "User menu",
		"Tools Menu": "Tools Menu",
	}

	for label, want := range cases {
		if got := xdfileMenuStatusLabel(label); got != want {
			t.Fatalf("xdfileMenuStatusLabel(%q) = %q, want %q", label, got, want)
		}
	}
}
