//go:build windows
// +build windows

package tea

import (
	"testing"

	"github.com/erikgeiser/coninput"
)

func TestWindowsKeyTypeMapsInsertVirtualKey(t *testing.T) {
	got := keyType(coninput.KeyEventRecord{
		VirtualKeyCode: coninput.VK_INSERT,
		KeyDown:        true,
	})
	if got != KeyInsert {
		t.Fatalf("expected VK_INSERT to map to KeyInsert, got %v", got)
	}
}

func TestWindowsKeyTypeMapsCtrlShiftClipboardKeys(t *testing.T) {
	tests := []struct {
		name string
		code coninput.VirtualKeyCode
		char rune
		want KeyType
	}{
		{name: "copy", code: coninput.VirtualKeyCode('C'), char: '\x03', want: KeyCtrlShiftC},
		{name: "paste", code: coninput.VirtualKeyCode('V'), char: '\x16', want: KeyCtrlShiftV},
	}

	for _, tt := range tests {
		got := keyType(coninput.KeyEventRecord{
			VirtualKeyCode: tt.code,
			Char:           tt.char,
			KeyDown:        true,
			ControlKeyState: coninput.LEFT_CTRL_PRESSED |
				coninput.SHIFT_PRESSED,
		})
		if got != tt.want {
			t.Fatalf("expected Ctrl+Shift+%s to map to %v, got %v", tt.name, tt.want, got)
		}
	}
}

func TestWindowsAltRuneKeepsAltModifier(t *testing.T) {
	got, ok := conInputAltRuneMsg(coninput.KeyEventRecord{
		VirtualKeyCode:  coninput.VirtualKeyCode('Q'),
		Char:            'q',
		KeyDown:         true,
		ControlKeyState: coninput.LEFT_ALT_PRESSED,
	}, KeyRunes)
	if !ok {
		t.Fatalf("expected Alt+Q rune to produce a modified key message")
	}
	if got.Type != KeyRunes || string(got.Runes) != "q" || !got.Alt {
		t.Fatalf("expected Alt+Q to keep Alt on KeyRunes, got %#v", got)
	}
}

func TestWindowsAltGrRuneDoesNotBecomeAltShortcut(t *testing.T) {
	_, ok := conInputAltRuneMsg(coninput.KeyEventRecord{
		VirtualKeyCode:  coninput.VirtualKeyCode('Q'),
		Char:            '@',
		KeyDown:         true,
		ControlKeyState: coninput.LEFT_CTRL_PRESSED | coninput.RIGHT_ALT_PRESSED,
	}, KeyRunes)
	if ok {
		t.Fatalf("expected AltGr rune to stay a plain character path")
	}
}

func TestWindowsAltRuneIgnoresEmptyChar(t *testing.T) {
	_, ok := conInputAltRuneMsg(coninput.KeyEventRecord{
		VirtualKeyCode:  coninput.VK_MENU,
		KeyDown:         true,
		ControlKeyState: coninput.LEFT_ALT_PRESSED,
	}, KeyRunes)
	if ok {
		t.Fatalf("expected empty Alt key event to be ignored as a rune shortcut")
	}
}
