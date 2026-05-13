package cmd

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	charmansi "github.com/charmbracelet/x/ansi"
)

const xdfileStatusSpinnerInterval = 80 * time.Millisecond

var xdfileStatusSpinnerFrames = [...]string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func xdfileScheduleStatusSpinner() tea.Cmd {
	return tea.Tick(xdfileStatusSpinnerInterval, func(t time.Time) tea.Msg {
		return xdfileStatusSpinnerTickMsg{At: t}
	})
}

func (m *xdfileModel) statusSpinnerActive() bool {
	return m != nil && (m.terminal.Busy || m.terminalStarting || m.backgroundTaskBusy)
}

func (m *xdfileModel) startBackgroundTask() tea.Cmd {
	if m == nil {
		return nil
	}
	wasActive := m.statusSpinnerActive()
	m.backgroundTaskBusy = true
	if wasActive {
		return nil
	}
	return xdfileScheduleStatusSpinner()
}

func (m *xdfileModel) stopBackgroundTask() {
	if m != nil {
		m.backgroundTaskBusy = false
	}
}

func (m *xdfileModel) statusSpinnerFrame() string {
	if len(xdfileStatusSpinnerFrames) == 0 {
		return "|"
	}
	index := 0
	if m != nil {
		index = m.statusSpinnerIndex % len(xdfileStatusSpinnerFrames)
	}
	return xdfileStatusSpinnerFrames[index]
}

func (m *xdfileModel) statusIndicatorSymbol() string {
	if m.statusSpinnerActive() {
		return m.statusSpinnerFrame()
	}
	switch xdfileStatusKindForText(m.statusText, m.statusError, m.modal.Action == xdfileActionPasteConflictPrompt) {
	case "error":
		return "!"
	case "conflict":
		return "?"
	case "canceled":
		return "×"
	case "done":
		return "✓"
	default:
		return "●"
	}
}

func (m *xdfileModel) renderStatusText(width int) string {
	if width <= 0 {
		return ""
	}
	symbol := m.statusIndicatorSymbol()
	text := strings.TrimSpace(m.statusText)
	if text == "" {
		text = "Ready"
	}
	if progress := m.fileOperationProgressStatus(); progress != "" {
		text = strings.TrimSpace(text + " " + progress)
	}
	content := symbol + " " + text
	style := xdfileStatusOKStyle
	if m.statusError {
		style = xdfileStatusErrStyle
	}
	return style.Render(charmansi.Truncate(content, width, "..."))
}

func (m *xdfileModel) commandStateLabel() string {
	if m.statusSpinnerActive() {
		return fmt.Sprintf("%s running", m.statusSpinnerFrame())
	}
	return "running"
}

func xdfileStatusKindForText(text string, isError bool, isConflict bool) string {
	if isError {
		return "error"
	}
	if isConflict {
		return "conflict"
	}
	lower := strings.ToLower(strings.TrimSpace(text))
	switch {
	case lower == "":
		return "idle"
	case strings.Contains(lower, "cancel"):
		return "canceled"
	case strings.Contains(lower, "conflict") ||
		strings.Contains(lower, "replace") ||
		strings.Contains(lower, "skip") ||
		strings.Contains(lower, "keep both"):
		return "conflict"
	case strings.Contains(lower, "completed") ||
		strings.Contains(lower, "copied") ||
		strings.Contains(lower, "pasted") ||
		strings.Contains(lower, "moved") ||
		strings.Contains(lower, "renamed") ||
		strings.Contains(lower, "created") ||
		strings.Contains(lower, "deleted") ||
		strings.Contains(lower, "restored") ||
		strings.Contains(lower, "saved") ||
		strings.Contains(lower, "opened") ||
		strings.Contains(lower, "connected") ||
		strings.Contains(lower, "disconnected") ||
		strings.Contains(lower, "synced") ||
		strings.Contains(lower, "refreshed") ||
		strings.Contains(lower, "sorted") ||
		strings.Contains(lower, "closed"):
		return "done"
	default:
		return "idle"
	}
}
