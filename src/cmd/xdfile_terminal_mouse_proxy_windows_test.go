//go:build windows

package cmd

import (
	"encoding/binary"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestXdfileMouseProxyInputRecordLeftClick(t *testing.T) {
	record, ok := xdfileMouseProxyInputRecord(xdfilePTYMouseProxyEvent{
		X:      7,
		Y:      4,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
		Shift:  true,
	})
	if !ok {
		t.Fatal("expected left click to encode as a console input record")
	}
	if record.EventType != xdfileConsoleMouseEventType {
		t.Fatalf("expected mouse event type, got %#x", record.EventType)
	}
	if got := binary.LittleEndian.Uint16(record.Event[0:2]); got != 7 {
		t.Fatalf("expected x coordinate 7, got %d", got)
	}
	if got := binary.LittleEndian.Uint16(record.Event[2:4]); got != 4 {
		t.Fatalf("expected y coordinate 4, got %d", got)
	}
	if got := binary.LittleEndian.Uint32(record.Event[4:8]); got != xdfileConsoleMouseLeft {
		t.Fatalf("expected left button state, got %#x", got)
	}
	if got := binary.LittleEndian.Uint32(record.Event[8:12]); got != xdfileConsoleShift {
		t.Fatalf("expected shift key state, got %#x", got)
	}
	if got := binary.LittleEndian.Uint32(record.Event[12:16]); got != xdfileConsoleMouseClick {
		t.Fatalf("expected click event flag, got %#x", got)
	}
}

func TestXdfileMouseProxyInputRecordWheelDown(t *testing.T) {
	record, ok := xdfileMouseProxyInputRecord(xdfilePTYMouseProxyEvent{
		X:      2,
		Y:      3,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonWheelDown,
	})
	if !ok {
		t.Fatal("expected wheel down to encode as a console input record")
	}
	if got := binary.LittleEndian.Uint32(record.Event[4:8]); got != xdfileMouseProxyWheelState(-xdfileConsoleWheelDelta) {
		t.Fatalf("expected negative wheel delta state, got %#x", got)
	}
	if got := binary.LittleEndian.Uint32(record.Event[12:16]); got != xdfileConsoleMouseWheeled {
		t.Fatalf("expected wheel event flag, got %#x", got)
	}
}
