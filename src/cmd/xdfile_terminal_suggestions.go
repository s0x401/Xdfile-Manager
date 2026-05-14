package cmd

import (
	"sort"
	"strings"
	"time"
)

type xdfileTerminalSuggestionSource int

const (
	xdfileTerminalSuggestionSourceHistory xdfileTerminalSuggestionSource = iota
	xdfileTerminalSuggestionSourcePath
	xdfileTerminalSuggestionSourceDefault
)

type xdfileTerminalSuggestion struct {
	Value    string
	Source   xdfileTerminalSuggestionSource
	Score    int
	Count    int
	LastUsed time.Time
}

var xdfileTerminalSuggestionDefaults = []string{
	"cd",
	"ls",
	"dir",
	"pwd",
	"clear",
	"cls",
	"cat",
	"type",
	"copy",
	"move",
	"del",
	"set",
	"mkdir",
	"rm",
	"cp",
	"mv",
	"git",
	"git status",
	"git add",
	"git commit",
	"git checkout",
	"git pull",
	"git push",
	"go run .",
	"go test ./...",
	"npm install",
	"npm run dev",
	"pnpm install",
	"pnpm dev",
	"cargo build",
	"cargo test",
	"python",
	"python -m",
	"pip install",
	"node",
	"code .",
	"explorer .",
	"Get-ChildItem",
	"Set-Location",
	"Get-Location",
	"Clear-Host",
	"Get-Content",
	"Copy-Item",
	"Move-Item",
	"Remove-Item",
	"New-Item",
}

func (m *xdfileModel) terminalSuggestionValues(input string, limit int) []string {
	suggestions := m.terminalSuggestions(input, limit)
	values := make([]string, 0, len(suggestions))
	for _, suggestion := range suggestions {
		values = append(values, suggestion.Value)
	}
	return values
}

func (m *xdfileModel) bestTerminalSuggestion(input string) string {
	if len([]rune(strings.TrimSpace(input))) < 2 {
		return ""
	}
	suggestions := m.terminalSuggestions(input, 1)
	if len(suggestions) == 0 {
		return ""
	}
	return suggestions[0].Value
}

func (m *xdfileModel) terminalSuggestions(input string, limit int) []xdfileTerminalSuggestion {
	if m == nil {
		return nil
	}
	historyItems := m.terminal.HistoryItems
	if len(historyItems) == 0 && len(m.terminal.History) > 0 {
		historyItems = xdfileTerminalHistoryItemsFromCommands(m.terminal.History)
	}
	return xdfileBuildTerminalSuggestions(input, m.terminal.Cwd, historyItems, m.terminal.HistoryDeleted, limit)
}

func xdfileBuildTerminalSuggestions(input string, cwd string, historyItems map[string]xdfileTerminalHistoryItem, deleted map[string]struct{}, limit int) []xdfileTerminalSuggestion {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil
	}
	if limit <= 0 {
		limit = 20
	}

	inputLower := strings.ToLower(input)
	suggestions := make([]xdfileTerminalSuggestion, 0, limit)
	seen := make(map[string]struct{}, limit)
	add := func(value string, source xdfileTerminalSuggestionSource, score int, count int, lastUsed time.Time) {
		value = strings.TrimSpace(value)
		if value == "" || strings.EqualFold(value, input) {
			return
		}
		if xdfileTerminalHistoryDeletedContains(deleted, value) {
			return
		}
		if !strings.HasPrefix(strings.ToLower(value), inputLower) {
			return
		}
		key := xdfileTerminalHistoryKey(value)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		suggestions = append(suggestions, xdfileTerminalSuggestion{
			Value:    value,
			Source:   source,
			Score:    score,
			Count:    count,
			LastUsed: lastUsed,
		})
	}

	for _, item := range historyItems {
		item = xdfileNormalizeTerminalHistoryItem(item)
		add(item.Command, xdfileTerminalSuggestionSourceHistory, xdfileTerminalHistorySuggestionScore(item, input, cwd), item.Count, item.LastUsed)
	}
	for _, value := range xdfileManagedShellPathSuggestions(input, cwd) {
		add(value, xdfileTerminalSuggestionSourcePath, 720, 0, time.Time{})
	}
	for _, value := range xdfileTerminalSuggestionDefaults {
		add(value, xdfileTerminalSuggestionSourceDefault, xdfileTerminalDefaultSuggestionScore(value, input), 0, time.Time{})
	}

	sort.SliceStable(suggestions, func(i, j int) bool {
		left := suggestions[i]
		right := suggestions[j]
		if left.Score != right.Score {
			return left.Score > right.Score
		}
		if left.Count != right.Count {
			return left.Count > right.Count
		}
		if !left.LastUsed.Equal(right.LastUsed) {
			return left.LastUsed.After(right.LastUsed)
		}
		if len([]rune(left.Value)) != len([]rune(right.Value)) {
			return len([]rune(left.Value)) < len([]rune(right.Value))
		}
		return strings.ToLower(left.Value) < strings.ToLower(right.Value)
	})
	if len(suggestions) > limit {
		suggestions = suggestions[:limit]
	}
	return suggestions
}

func xdfileTerminalHistoryItemsFromCommands(commands []string) map[string]xdfileTerminalHistoryItem {
	if len(commands) == 0 {
		return nil
	}
	items := make(map[string]xdfileTerminalHistoryItem, len(commands))
	baseTime := time.Now().Add(-time.Duration(len(commands)+1) * time.Minute)
	for index, command := range commands {
		key := xdfileTerminalHistoryKey(command)
		if key == "" {
			continue
		}
		items[key] = xdfileTerminalHistoryItem{
			Command:  strings.TrimSpace(command),
			Count:    1,
			LastUsed: baseTime.Add(time.Duration(index) * time.Minute),
		}
	}
	return items
}

func xdfileTerminalHistorySuggestionScore(item xdfileTerminalHistoryItem, input string, cwd string) int {
	score := 1000
	if strings.HasPrefix(item.Command, input) {
		score += 25
	}
	if item.Cwd != "" && cwd != "" && xdfilePathsEqual(item.Cwd, cwd) {
		score += 220
	}
	score += min(240, item.Count*20)
	if !item.LastUsed.IsZero() {
		age := time.Since(item.LastUsed)
		if age < 0 {
			age = 0
		}
		switch {
		case age <= time.Hour:
			score += 180
		case age <= 24*time.Hour:
			score += 130
		case age <= 7*24*time.Hour:
			score += 80
		case age <= 30*24*time.Hour:
			score += 40
		}
	}
	if item.LastFailed {
		score -= 220
	}
	return score
}

func xdfileTerminalDefaultSuggestionScore(value string, input string) int {
	score := 360
	if strings.HasPrefix(value, input) {
		score += 20
	}
	if !strings.ContainsAny(value, " \t") {
		score += 20
	}
	return score
}
