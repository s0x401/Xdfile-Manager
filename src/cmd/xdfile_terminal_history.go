package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	variable "github.com/s0x401/xdfile-manager/src/config"
	"github.com/s0x401/xdfile-manager/src/internal/utils"
)

const (
	xdfileTerminalHistoryFile  = "xdfile-terminal-history.json"
	xdfileTerminalHistoryLimit = 800
)

type xdfileTerminalHistoryItem struct {
	Command    string    `json:"command"`
	Cwd        string    `json:"cwd,omitempty"`
	Count      int       `json:"count,omitempty"`
	LastUsed   time.Time `json:"last_used,omitempty"`
	LastFailed bool      `json:"last_failed,omitempty"`
}

type xdfileTerminalHistoryState struct {
	Version int                         `json:"version"`
	Items   []xdfileTerminalHistoryItem `json:"items,omitempty"`
	Deleted []string                    `json:"deleted,omitempty"`
}

func xdfileTerminalHistoryPath() string {
	return filepath.Join(variable.XdfileMainDir, xdfileTerminalHistoryFile)
}

func xdfileTerminalHistoryKey(command string) string {
	return strings.ToLower(strings.TrimSpace(command))
}

func xdfileTerminalHistoryDeletedContains(deleted map[string]struct{}, command string) bool {
	if len(deleted) == 0 {
		return false
	}
	_, ok := deleted[xdfileTerminalHistoryKey(command)]
	return ok
}

func xdfileLoadTerminalHistoryState(path string) (map[string]xdfileTerminalHistoryItem, map[string]struct{}, error) {
	items := make(map[string]xdfileTerminalHistoryItem)
	deleted := make(map[string]struct{})

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return items, deleted, nil
		}
		return items, deleted, fmt.Errorf("read terminal history: %w", err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return items, deleted, nil
	}

	var state xdfileTerminalHistoryState
	if err := json.Unmarshal(data, &state); err != nil {
		return items, deleted, fmt.Errorf("parse terminal history: %w", err)
	}
	for _, command := range state.Deleted {
		if key := xdfileTerminalHistoryKey(command); key != "" {
			deleted[key] = struct{}{}
		}
	}
	for _, item := range state.Items {
		item = xdfileNormalizeTerminalHistoryItem(item)
		key := xdfileTerminalHistoryKey(item.Command)
		if key == "" || xdfileTerminalHistoryDeletedContains(deleted, item.Command) {
			continue
		}
		items[key] = item
	}
	return items, deleted, nil
}

func xdfileSaveTerminalHistoryState(path string, items map[string]xdfileTerminalHistoryItem, deleted map[string]struct{}) error {
	state := xdfileTerminalHistoryState{
		Version: 1,
		Items:   xdfileSortedTerminalHistoryItems(items, deleted, true),
		Deleted: xdfileSortedTerminalHistoryDeleted(deleted),
	}
	if len(state.Items) == 0 && len(state.Deleted) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("clear terminal history: %w", err)
		}
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), utils.ConfigDirPerm); err != nil {
		return fmt.Errorf("create terminal history directory: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode terminal history: %w", err)
	}
	if err := os.WriteFile(path, data, utils.ConfigFilePerm); err != nil {
		return fmt.Errorf("write terminal history: %w", err)
	}
	return nil
}

func xdfileMergeTerminalHistorySeed(items map[string]xdfileTerminalHistoryItem, seed []string, deleted map[string]struct{}) map[string]xdfileTerminalHistoryItem {
	if items == nil {
		items = make(map[string]xdfileTerminalHistoryItem)
	}
	if len(seed) == 0 {
		return items
	}

	baseTime := time.Now().Add(-time.Duration(len(seed)+1) * time.Minute)
	for index, command := range seed {
		command = strings.TrimSpace(command)
		key := xdfileTerminalHistoryKey(command)
		if key == "" || xdfileTerminalHistoryDeletedContains(deleted, command) {
			continue
		}
		if _, ok := items[key]; ok {
			continue
		}
		items[key] = xdfileTerminalHistoryItem{
			Command:  command,
			Count:    1,
			LastUsed: baseTime.Add(time.Duration(index) * time.Minute),
		}
	}
	return items
}

func xdfileTerminalHistoryCommands(items map[string]xdfileTerminalHistoryItem, deleted map[string]struct{}) []string {
	sorted := xdfileSortedTerminalHistoryItems(items, deleted, false)
	commands := make([]string, 0, len(sorted))
	for _, item := range sorted {
		commands = append(commands, item.Command)
	}
	return commands
}

func xdfileSortedTerminalHistoryItems(items map[string]xdfileTerminalHistoryItem, deleted map[string]struct{}, newestFirst bool) []xdfileTerminalHistoryItem {
	if len(items) == 0 {
		return nil
	}
	sorted := make([]xdfileTerminalHistoryItem, 0, len(items))
	for _, item := range items {
		item = xdfileNormalizeTerminalHistoryItem(item)
		if xdfileTerminalHistoryKey(item.Command) == "" || xdfileTerminalHistoryDeletedContains(deleted, item.Command) {
			continue
		}
		sorted = append(sorted, item)
	}
	sort.SliceStable(sorted, func(i, j int) bool {
		left := sorted[i]
		right := sorted[j]
		if !left.LastUsed.Equal(right.LastUsed) {
			if newestFirst {
				return left.LastUsed.After(right.LastUsed)
			}
			return left.LastUsed.Before(right.LastUsed)
		}
		return strings.ToLower(left.Command) < strings.ToLower(right.Command)
	})
	if len(sorted) > xdfileTerminalHistoryLimit && newestFirst {
		sorted = sorted[:xdfileTerminalHistoryLimit]
	} else if len(sorted) > xdfileTerminalHistoryLimit {
		sorted = sorted[len(sorted)-xdfileTerminalHistoryLimit:]
	}
	return sorted
}

func xdfileSortedTerminalHistoryDeleted(deleted map[string]struct{}) []string {
	if len(deleted) == 0 {
		return nil
	}
	values := make([]string, 0, len(deleted))
	seen := make(map[string]struct{}, len(deleted))
	for command := range deleted {
		key := xdfileTerminalHistoryKey(command)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		values = append(values, key)
	}
	sort.Strings(values)
	return values
}

func xdfileNormalizeTerminalHistoryItem(item xdfileTerminalHistoryItem) xdfileTerminalHistoryItem {
	item.Command = strings.TrimSpace(item.Command)
	item.Cwd = strings.TrimSpace(item.Cwd)
	if item.Count < 1 {
		item.Count = 1
	}
	if item.LastUsed.IsZero() {
		item.LastUsed = time.Now()
	}
	return item
}

func (m *xdfileModel) pushTerminalHistory(command string) {
	if err := m.recordTerminalHistory(command, m.terminal.Cwd, false); err != nil {
		m.setStatusErr(err)
	}
}

func (m *xdfileModel) recordTerminalHistory(command string, cwd string, failed bool) error {
	if m == nil {
		return nil
	}
	command = strings.TrimSpace(command)
	if command == "" {
		return nil
	}

	key := xdfileTerminalHistoryKey(command)
	if key == "" {
		return nil
	}
	if m.terminal.HistoryItems == nil {
		m.terminal.HistoryItems = make(map[string]xdfileTerminalHistoryItem)
	}
	if m.terminal.HistoryDeleted != nil {
		delete(m.terminal.HistoryDeleted, key)
	}

	item := m.terminal.HistoryItems[key]
	if item.Command == "" {
		item.Command = command
	}
	item.Cwd = strings.TrimSpace(cwd)
	item.Count++
	item.LastUsed = time.Now()
	item.LastFailed = failed
	m.terminal.HistoryItems[key] = xdfileNormalizeTerminalHistoryItem(item)
	xdfileTrimTerminalHistoryItems(m.terminal.HistoryItems, m.terminal.HistoryDeleted)
	m.syncTerminalHistoryCommands()
	m.terminal.PendingHistoryCommand = command
	m.terminal.PendingHistoryCwd = cwd
	return xdfileSaveTerminalHistoryState(xdfileTerminalHistoryPath(), m.terminal.HistoryItems, m.terminal.HistoryDeleted)
}

func (m *xdfileModel) updateTerminalHistoryResult(command string, cwd string, failed bool) error {
	if m == nil {
		return nil
	}
	command = strings.TrimSpace(command)
	if command == "" {
		command = strings.TrimSpace(m.terminal.PendingHistoryCommand)
	}
	if cwd == "" {
		cwd = m.terminal.PendingHistoryCwd
	}
	key := xdfileTerminalHistoryKey(command)
	if key == "" || len(m.terminal.HistoryItems) == 0 {
		return nil
	}

	item, ok := m.terminal.HistoryItems[key]
	if !ok {
		return nil
	}
	if cwd != "" {
		item.Cwd = cwd
	}
	item.LastFailed = failed
	m.terminal.HistoryItems[key] = xdfileNormalizeTerminalHistoryItem(item)
	m.clearPendingTerminalHistory(command)
	return xdfileSaveTerminalHistoryState(xdfileTerminalHistoryPath(), m.terminal.HistoryItems, m.terminal.HistoryDeleted)
}

func (m *xdfileModel) clearPendingTerminalHistory(command string) {
	if m == nil || m.terminal.PendingHistoryCommand == "" {
		return
	}
	if command == "" || xdfileTerminalHistoryKey(command) == xdfileTerminalHistoryKey(m.terminal.PendingHistoryCommand) {
		m.terminal.PendingHistoryCommand = ""
		m.terminal.PendingHistoryCwd = ""
	}
}

func (m *xdfileModel) syncTerminalHistoryCommands() {
	if m == nil {
		return
	}
	m.terminal.History = xdfileTerminalHistoryCommands(m.terminal.HistoryItems, m.terminal.HistoryDeleted)
	m.terminal.HistoryIndex = -1
	m.terminal.HistoryDraft = ""
}

func xdfileTrimTerminalHistoryItems(items map[string]xdfileTerminalHistoryItem, deleted map[string]struct{}) {
	if len(items) <= xdfileTerminalHistoryLimit {
		return
	}
	keep := make(map[string]struct{}, xdfileTerminalHistoryLimit)
	for _, item := range xdfileSortedTerminalHistoryItems(items, deleted, true) {
		if key := xdfileTerminalHistoryKey(item.Command); key != "" {
			keep[key] = struct{}{}
		}
	}
	for key := range items {
		if _, ok := keep[key]; !ok {
			delete(items, key)
		}
	}
}

func (m *xdfileModel) deleteSelectedManagedTerminalHistorySuggestion() bool {
	if m == nil || !m.managedTerminalPopupVisible() {
		return false
	}

	selected := strings.TrimSpace(m.selectedManagedTerminalSuggestion())
	if selected == "" {
		return false
	}

	baseInput := m.terminal.SuggestionInput
	if baseInput == "" {
		baseInput = m.terminal.Input.Value()
	}
	cursor := m.terminal.SuggestionCursor
	removed := m.removeTerminalHistoryCommand(selected)
	if removed == 0 {
		m.restoreManagedTerminalSuggestionBase(baseInput, 0)
		m.setStatus("Only command history predictions can be deleted")
		return true
	}

	if err := m.markTerminalHistoryDeleted(selected); err != nil {
		m.restoreManagedTerminalSuggestionBase(baseInput, cursor)
		m.setStatusErr(err)
		return true
	}

	m.restoreManagedTerminalSuggestionBase(baseInput, cursor)
	m.setStatus("Deleted history prediction: %s", selected)
	return true
}

func (m *xdfileModel) restoreManagedTerminalSuggestionBase(input string, cursor int) {
	if m == nil {
		return
	}
	m.terminal.Input.SetValue(input)
	m.terminal.Input.CursorEnd()
	m.terminal.SuggestionInput = ""
	m.terminal.SuggestionDismissed = false
	m.refreshManagedTerminalSuggestions()
	if cursor <= 0 || len(m.terminal.Suggestions) == 0 {
		return
	}
	if cursor > len(m.terminal.Suggestions) {
		cursor = len(m.terminal.Suggestions)
	}
	m.terminal.SuggestionCursor = cursor
	m.syncManagedTerminalSuggestionPreview()
}

func (m *xdfileModel) removeTerminalHistoryCommand(command string) int {
	if m == nil {
		return 0
	}
	key := xdfileTerminalHistoryKey(command)
	if key == "" || len(m.terminal.HistoryItems) == 0 {
		return 0
	}
	if _, ok := m.terminal.HistoryItems[key]; !ok {
		return 0
	}
	delete(m.terminal.HistoryItems, key)
	m.syncTerminalHistoryCommands()
	return 1
}

func (m *xdfileModel) markTerminalHistoryDeleted(command string) error {
	if m == nil {
		return nil
	}
	key := xdfileTerminalHistoryKey(command)
	if key == "" {
		return nil
	}
	if m.terminal.HistoryDeleted == nil {
		m.terminal.HistoryDeleted = make(map[string]struct{})
	}
	m.terminal.HistoryDeleted[key] = struct{}{}
	return xdfileSaveTerminalHistoryState(xdfileTerminalHistoryPath(), m.terminal.HistoryItems, m.terminal.HistoryDeleted)
}
