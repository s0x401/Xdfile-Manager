package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

func TestPrepareCommandMenuCommandExpandsSelectedListAndPanelToggle(t *testing.T) {
	leftDir := t.TempDir()
	rightDir := t.TempDir()
	leftFile := filepath.Join(leftDir, "update.msi")

	m := &xdfileModel{
		activePanel: 0,
		panels: [2]xdfilePanel{
			{
				Label: "LEFT",
				Cwd:   leftDir,
				Entries: []xdfileEntry{
					{Name: "update.msi", Path: leftFile},
				},
			},
			{
				Label: "RIGHT",
				Cwd:   rightDir,
				Entries: []xdfileEntry{
					{Name: "mirror.bin", Path: filepath.Join(rightDir, "mirror.bin")},
				},
			},
		},
	}

	expansion, err := m.prepareCommandMenuCommand(`rehash.cmd !& | clip && type !##!\!^!.!`)
	if err != nil {
		t.Fatalf("prepare command: %v", err)
	}

	want := `rehash.cmd "update.msi" | clip && type ` + filepath.Join(rightDir, "update.msi")
	if expansion.Command != want {
		t.Fatalf("expected expanded command %q, got %q", want, expansion.Command)
	}
	if len(expansion.TempFiles) != 0 {
		t.Fatalf("expected no temp files for !& expansion, got %+v", expansion.TempFiles)
	}
}

func TestPrepareManagedTerminalCommandExpandsMetasymbolsWithoutTouchingShellBangVars(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "xdfile.exe")

	m := &xdfileModel{
		activePanel: 0,
		panels: [2]xdfilePanel{
			{
				Label: "LEFT",
				Cwd:   dir,
				Entries: []xdfileEntry{
					{Name: "xdfile.exe", Path: filePath},
				},
			},
		},
	}

	expansion, err := m.prepareManagedTerminalCommand(`rehash.cmd !& | clip && echo !PATH!`)
	if err != nil {
		t.Fatalf("prepare managed terminal command: %v", err)
	}

	want := `rehash.cmd "xdfile.exe" | clip && echo !PATH!`
	if expansion.Command != want {
		t.Fatalf("expected expanded managed terminal command %q, got %q", want, expansion.Command)
	}
	if len(expansion.TempFiles) != 0 {
		t.Fatalf("expected no temp files for managed !& expansion, got %+v", expansion.TempFiles)
	}
}

func TestPrepareManagedTerminalCommandLeavesShellBangVarsUntouchedWithoutMetasymbols(t *testing.T) {
	m := &xdfileModel{}

	expansion, err := m.prepareManagedTerminalCommand(`echo !PATH!`)
	if err != nil {
		t.Fatalf("prepare managed terminal command: %v", err)
	}
	if expansion.Command != `echo !PATH!` {
		t.Fatalf("expected shell bang variable to stay unchanged, got %q", expansion.Command)
	}
}

func TestPrepareCommandMenuCommandCreatesListFile(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "alpha.txt")
	second := filepath.Join(dir, "beta.txt")

	m := &xdfileModel{
		activePanel: 0,
		panels: [2]xdfilePanel{
			{
				Label: "LEFT",
				Cwd:   dir,
				Entries: []xdfileEntry{
					{Name: "alpha.txt", Path: first},
					{Name: "beta.txt", Path: second},
				},
				MarkedPaths: map[string]struct{}{
					first:  {},
					second: {},
				},
			},
		},
	}

	expansion, err := m.prepareCommandMenuCommand(`tool !@@FQ!`)
	if err != nil {
		t.Fatalf("prepare command: %v", err)
	}
	defer m.cleanupCommandMenuTempFiles(expansion.TempFiles)

	if len(expansion.TempFiles) != 1 {
		t.Fatalf("expected one temp list file, got %+v", expansion.TempFiles)
	}
	if !strings.HasPrefix(expansion.Command, "tool ") {
		t.Fatalf("expected command to start with tool invocation, got %q", expansion.Command)
	}
	listPath := strings.TrimPrefix(expansion.Command, "tool ")
	if listPath == "" {
		t.Fatalf("expected list file path in expanded command, got %q", expansion.Command)
	}

	data, err := os.ReadFile(expansion.TempFiles[0])
	if err != nil {
		t.Fatalf("read temp list file: %v", err)
	}
	content := string(data)
	want := `"` + first + `"` + "\r\n" + `"` + second + `"` + "\r\n"
	if content != want {
		t.Fatalf("expected list file content %q, got %q", want, content)
	}
}

func TestExecuteCommandMenuIndexPromptsForInputAndExpandsVariables(t *testing.T) {
	originalExecuteCommandMenu := xdfileExecuteCommandMenuFunc
	defer func() {
		xdfileExecuteCommandMenuFunc = originalExecuteCommandMenu
	}()

	dir := t.TempDir()
	filePath := filepath.Join(dir, "demo.txt")

	var gotCommands []string
	xdfileExecuteCommandMenuFunc = func(m *xdfileModel, command string) tea.Cmd {
		gotCommands = append(gotCommands, command)
		return nil
	}

	m := &xdfileModel{
		activePanel: 0,
		openMenu:    xdfileActionCommandsMenu,
		modal: xdfileModal{
			Input: textinput.New(),
		},
		panels: [2]xdfilePanel{
			{
				Label: "LEFT",
				Cwd:   dir,
				Entries: []xdfileEntry{
					{Name: "demo.txt", Path: filePath},
				},
			},
		},
		layoutPrefs: xdfileLayoutPrefs{
			Commands: []xdfileCommandItem{
				{
					Type:    xdfileCommandItemTypeCommand,
					Label:   "Prompted",
					Command: `grep !?$GrepHist$Find in (!.!):?*.txt! %UserVar1 %GrepHist`,
				},
			},
		},
		commandPromptHistory: make(map[string]string),
	}

	if cmd := m.executeCommandMenuIndex(0); cmd != nil {
		t.Fatalf("expected prompt modal to open immediately")
	}
	if len(gotCommands) != 0 {
		t.Fatalf("expected prompted command not to execute yet, got %+v", gotCommands)
	}
	if m.modal.Kind != xdfileModalForm || m.modal.Action != xdfileActionCommandPrompt {
		t.Fatalf("expected command prompt form modal, got kind=%v action=%q", m.modal.Kind, m.modal.Action)
	}
	if len(m.modal.FormFields) != 1 {
		t.Fatalf("expected one prompt field, got %d", len(m.modal.FormFields))
	}
	if got := m.modal.FormFields[0].Label; got != "Find in (demo.txt):" {
		t.Fatalf("expected expanded prompt label, got %q", got)
	}
	if got := m.modal.FormFields[0].Input.Value(); got != "*.txt" {
		t.Fatalf("expected prompt initial value %q, got %q", "*.txt", got)
	}

	m.modal.FormFields[0].Input.SetValue("needle")
	if cmd := m.applyModal(); cmd != nil {
		if msg := cmd(); msg != nil {
			t.Fatalf("expected command prompt apply to finish immediately, got %T", msg)
		}
	}

	if len(gotCommands) != 1 {
		t.Fatalf("expected one executed command after prompt apply, got %+v", gotCommands)
	}
	if gotCommands[0] != "grep needle needle needle" {
		t.Fatalf("expected prompt variables to expand into command, got %q", gotCommands[0])
	}

	if cmd := m.executeCommandMenuIndex(0); cmd != nil {
		t.Fatalf("expected second prompt modal to open immediately")
	}
	if got := m.modal.FormFields[0].Input.Value(); got != "needle" {
		t.Fatalf("expected prompt history value %q on next run, got %q", "needle", got)
	}
}
