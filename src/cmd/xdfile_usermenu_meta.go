package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf16"

	tea "github.com/charmbracelet/bubbletea"
)

type xdfileCommandMenuPanelTarget int

const (
	xdfileCommandMenuPanelActive xdfileCommandMenuPanelTarget = iota
	xdfileCommandMenuPanelPassive
	xdfileCommandMenuPanelLeft
	xdfileCommandMenuPanelRight
)

type xdfileCommandMenuPrompt struct {
	Placeholder string
	Label       string
	Initial     string
	History     string
	Variable    string
}

type xdfileCommandMenuExpansion struct {
	Command         string
	Prompts         []xdfileCommandMenuPrompt
	TempFiles       []string
	PreserveLFN     bool
	UsesListFiles   bool
	UsesShortNames  bool
	UsesDescription bool
}

type xdfilePendingCommandMenu struct {
	Label     string
	Command   string
	Prompts   []xdfileCommandMenuPrompt
	TempFiles []string
}

type xdfileCommandMenuListEncoding int

const (
	xdfileCommandMenuListEncodingOEM xdfileCommandMenuListEncoding = iota
	xdfileCommandMenuListEncodingANSI
	xdfileCommandMenuListEncodingUTF8
	xdfileCommandMenuListEncodingUTF16LE
)

type xdfileCommandMenuParser struct {
	model           *xdfileModel
	target          xdfileCommandMenuPanelTarget
	prompts         []xdfileCommandMenuPrompt
	tempFiles       []string
	preserveLFN     bool
	allowPrompts    bool
	allowBareName   bool
	usesListFiles   bool
	usesShortNames  bool
	usesDescription bool
}

func (m *xdfileModel) prepareCommandMenuCommand(command string) (xdfileCommandMenuExpansion, error) {
	parser := xdfileCommandMenuParser{
		model:         m,
		target:        xdfileCommandMenuPanelActive,
		allowPrompts:  true,
		allowBareName: true,
	}

	expanded, err := parser.expand(command)
	if err != nil {
		m.cleanupCommandMenuTempFiles(parser.tempFiles)
		return xdfileCommandMenuExpansion{}, err
	}

	return xdfileCommandMenuExpansion{
		Command:         expanded,
		Prompts:         append([]xdfileCommandMenuPrompt(nil), parser.prompts...),
		TempFiles:       append([]string(nil), parser.tempFiles...),
		PreserveLFN:     parser.preserveLFN,
		UsesListFiles:   parser.usesListFiles,
		UsesShortNames:  parser.usesShortNames,
		UsesDescription: parser.usesDescription,
	}, nil
}

func (m *xdfileModel) prepareManagedTerminalCommand(command string) (xdfileCommandMenuExpansion, error) {
	command = strings.TrimSpace(command)
	if !xdfileContainsManagedTerminalMetasymbols(command) {
		return xdfileCommandMenuExpansion{Command: command}, nil
	}

	parser := xdfileCommandMenuParser{
		model:         m,
		target:        xdfileCommandMenuPanelActive,
		allowBareName: false,
	}

	expanded, err := parser.expand(command)
	if err != nil {
		m.cleanupCommandMenuTempFiles(parser.tempFiles)
		return xdfileCommandMenuExpansion{}, err
	}

	return xdfileCommandMenuExpansion{
		Command:         expanded,
		Prompts:         append([]xdfileCommandMenuPrompt(nil), parser.prompts...),
		TempFiles:       append([]string(nil), parser.tempFiles...),
		PreserveLFN:     parser.preserveLFN,
		UsesListFiles:   parser.usesListFiles,
		UsesShortNames:  parser.usesShortNames,
		UsesDescription: parser.usesDescription,
	}, nil
}

func (m *xdfileModel) openCommandMenuPromptForm(label string, expansion xdfileCommandMenuExpansion) tea.Cmd {
	if len(expansion.Prompts) == 0 {
		m.registerCommandMenuTempFiles(expansion.TempFiles)
		m.setStatus("Running command %s", label)
		return xdfileExecuteCommandMenuFunc(m, expansion.Command)
	}

	m.pendingCommandMenu = &xdfilePendingCommandMenu{
		Label:     label,
		Command:   expansion.Command,
		Prompts:   append([]xdfileCommandMenuPrompt(nil), expansion.Prompts...),
		TempFiles: append([]string(nil), expansion.TempFiles...),
	}

	fields := make([]xdfileModalField, 0, len(expansion.Prompts))
	for i, prompt := range expansion.Prompts {
		fieldLabel := strings.TrimSpace(prompt.Label)
		if fieldLabel == "" {
			fieldLabel = fmt.Sprintf("Input %d", i+1)
		}
		initial := prompt.Initial
		if prompt.History != "" {
			if value, ok := m.commandPromptHistory[strings.ToLower(prompt.History)]; ok {
				initial = value
			}
		}
		fields = append(fields, m.newModalFormField(fieldLabel, "Type here", initial))
	}

	title := "User Menu Input"
	if strings.TrimSpace(label) != "" {
		title = label
	}
	m.openFormModal(
		xdfileActionCommandPrompt,
		title,
		"Fill in the required metasymbol inputs, then press Enter to run the command.",
		fields,
	)
	return nil
}

func (m *xdfileModel) applyCommandMenuPrompt() tea.Cmd {
	if m.pendingCommandMenu == nil {
		m.closeModal()
		return nil
	}
	if len(m.modal.FormFields) < len(m.pendingCommandMenu.Prompts) {
		m.setStatus("Command input form is incomplete")
		return nil
	}

	command := m.pendingCommandMenu.Command
	variables := make(map[string]string, len(m.pendingCommandMenu.Prompts)*2)
	for i, prompt := range m.pendingCommandMenu.Prompts {
		value := m.modal.FormFields[i].Input.Value()
		command = strings.ReplaceAll(command, prompt.Placeholder, value)
		variables[prompt.Variable] = value
		if prompt.History != "" {
			m.commandPromptHistory[strings.ToLower(prompt.History)] = value
			variables[prompt.History] = value
		}
	}
	command = xdfileExpandCommandMenuVariables(command, variables)

	tempFiles := append([]string(nil), m.pendingCommandMenu.TempFiles...)
	label := m.pendingCommandMenu.Label
	m.pendingCommandMenu = nil
	m.closeModal()
	m.registerCommandMenuTempFiles(tempFiles)
	m.setStatus("Running command %s", label)
	return xdfileExecuteCommandMenuFunc(m, command)
}

func (m *xdfileModel) discardPendingCommandMenuPrompt() {
	if m.pendingCommandMenu != nil {
		m.cleanupCommandMenuTempFiles(m.pendingCommandMenu.TempFiles)
		m.pendingCommandMenu = nil
	}
	m.closeModal()
}

func (m *xdfileModel) registerCommandMenuTempFiles(paths []string) {
	if len(paths) == 0 {
		return
	}
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			continue
		}
		m.commandMenuTempFiles = append(m.commandMenuTempFiles, path)
	}
}

func (m *xdfileModel) cleanupCommandMenuTempFiles(paths []string) {
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			continue
		}
		_ = os.Remove(path)
	}
}

func (m *xdfileModel) cleanupAllCommandMenuTempFiles() {
	m.cleanupCommandMenuTempFiles(m.commandMenuTempFiles)
	m.commandMenuTempFiles = nil
	if m.pendingCommandMenu != nil {
		m.cleanupCommandMenuTempFiles(m.pendingCommandMenu.TempFiles)
		m.pendingCommandMenu = nil
	}
}

func (p *xdfileCommandMenuParser) expand(command string) (string, error) {
	var out strings.Builder
	for len(command) > 0 {
		switch {
		case strings.HasPrefix(command, "!##"):
			p.target = xdfileCommandMenuPanelPassive
			command = command[3:]
		case strings.HasPrefix(command, "!#"):
			p.target = xdfileCommandMenuPanelPassive
			command = command[2:]
		case strings.HasPrefix(command, "!^"):
			p.target = xdfileCommandMenuPanelActive
			command = command[2:]
		case strings.HasPrefix(command, "!["):
			p.target = xdfileCommandMenuPanelLeft
			command = command[2:]
		case strings.HasPrefix(command, "!]"):
			p.target = xdfileCommandMenuPanelRight
			command = command[2:]
		case strings.HasPrefix(command, "!!"):
			out.WriteByte('!')
			command = command[2:]
		case strings.HasPrefix(command, "!.!"):
			value, err := p.currentLongName()
			if err != nil {
				return "", err
			}
			out.WriteString(value)
			command = command[3:]
		case strings.HasPrefix(command, "!-!"):
			value, err := p.currentShortNameWithExt()
			if err != nil {
				return "", err
			}
			out.WriteString(value)
			command = command[3:]
		case strings.HasPrefix(command, "!+!"):
			value, err := p.currentShortNameWithExt()
			if err != nil {
				return "", err
			}
			p.preserveLFN = true
			out.WriteString(value)
			command = command[3:]
		case strings.HasPrefix(command, "!&~"):
			value, consumed, err := p.selectedNamesList(true, command[3:])
			if err != nil {
				return "", err
			}
			out.WriteString(value)
			command = command[3+consumed:]
		case strings.HasPrefix(command, "!&"):
			value, consumed, err := p.selectedNamesList(false, command[2:])
			if err != nil {
				return "", err
			}
			out.WriteString(value)
			command = command[2+consumed:]
		case strings.HasPrefix(command, "!@"):
			value, consumed, ok, err := p.listFile(false, command[2:])
			if err != nil {
				return "", err
			}
			if !ok {
				out.WriteByte(command[0])
				command = command[1:]
				continue
			}
			out.WriteString(value)
			command = command[2+consumed:]
		case strings.HasPrefix(command, "!$"):
			value, consumed, ok, err := p.listFile(true, command[2:])
			if err != nil {
				return "", err
			}
			if !ok {
				out.WriteByte(command[0])
				command = command[1:]
				continue
			}
			out.WriteString(value)
			command = command[2+consumed:]
		case strings.HasPrefix(command, "!`~"):
			value, err := p.currentShortExtension()
			if err != nil {
				return "", err
			}
			out.WriteString(value)
			command = command[3:]
		case strings.HasPrefix(command, "!`"):
			value, err := p.currentLongExtension()
			if err != nil {
				return "", err
			}
			out.WriteString(value)
			command = command[2:]
		case strings.HasPrefix(command, "!~"):
			value, err := p.currentShortName()
			if err != nil {
				return "", err
			}
			out.WriteString(value)
			command = command[2:]
		case strings.HasPrefix(command, "!=\\"):
			out.WriteString(p.currentPanelPath(true, false))
			command = command[3:]
		case strings.HasPrefix(command, "!=/"):
			out.WriteString(p.currentPanelPath(true, true))
			command = command[3:]
		case strings.HasPrefix(command, "!\\"):
			out.WriteString(p.currentPanelPath(false, false))
			command = command[2:]
		case strings.HasPrefix(command, "!/"):
			out.WriteString(p.currentPanelPath(false, true))
			command = command[2:]
		case strings.HasPrefix(command, "!:"):
			out.WriteString(p.currentDrive())
			command = command[2:]
		case strings.HasPrefix(command, "!?!"):
			p.usesDescription = true
			out.WriteString(p.currentDescription())
			command = command[3:]
		case strings.HasPrefix(command, "!?"):
			prompt, size, ok, err := p.parsePrompt(command)
			if err != nil {
				return "", err
			}
			if !ok {
				out.WriteByte(command[0])
				command = command[1:]
				continue
			}
			out.WriteString(prompt.Placeholder)
			command = command[size:]
		case strings.HasPrefix(command, "!") && p.allowBareName:
			value, err := p.currentLongNameWithoutExt()
			if err != nil {
				return "", err
			}
			out.WriteString(value)
			command = command[1:]
		default:
			out.WriteByte(command[0])
			command = command[1:]
		}
	}
	return out.String(), nil
}

func (p *xdfileCommandMenuParser) panelIndex() int {
	if p.model == nil {
		return 0
	}
	switch p.target {
	case xdfileCommandMenuPanelPassive:
		return 1 - p.model.activePanel
	case xdfileCommandMenuPanelLeft:
		return 0
	case xdfileCommandMenuPanelRight:
		return 1
	default:
		return p.model.activePanel
	}
}

func (p *xdfileCommandMenuParser) panel() *xdfilePanel {
	if p.model == nil {
		return nil
	}
	index := p.panelIndex()
	if index < 0 || index >= len(p.model.panels) {
		return nil
	}
	return &p.model.panels[index]
}

func (p *xdfileCommandMenuParser) currentEntry() (xdfileEntry, error) {
	panel := p.panel()
	if panel == nil {
		return xdfileEntry{}, errors.New("no panel available for command expansion")
	}
	entry, ok := panel.selected()
	if !ok || entry.IsParent {
		return xdfileEntry{}, fmt.Errorf("select a file or directory first on the %s panel", strings.ToLower(panel.Label))
	}
	return entry, nil
}

func (p *xdfileCommandMenuParser) selectedEntries() ([]xdfileEntry, error) {
	panel := p.panel()
	if panel == nil {
		return nil, errors.New("no panel available for command expansion")
	}
	if marked := panel.markedEntries(); len(marked) > 0 {
		return marked, nil
	}
	entry, err := p.currentEntry()
	if err != nil {
		return nil, err
	}
	return []xdfileEntry{entry}, nil
}

func (p *xdfileCommandMenuParser) currentLongName() (string, error) {
	entry, err := p.currentEntry()
	if err != nil {
		return "", err
	}
	return entry.Name, nil
}

func (p *xdfileCommandMenuParser) currentLongNameWithoutExt() (string, error) {
	name, err := p.currentLongName()
	if err != nil {
		return "", err
	}
	stem, _ := xdfileCommandMenuSplitName(name)
	return stem, nil
}

func (p *xdfileCommandMenuParser) currentLongExtension() (string, error) {
	name, err := p.currentLongName()
	if err != nil {
		return "", err
	}
	_, ext := xdfileCommandMenuSplitName(name)
	return ext, nil
}

func (p *xdfileCommandMenuParser) currentShortNameWithExt() (string, error) {
	entry, err := p.currentEntry()
	if err != nil {
		return "", err
	}
	return p.entryShortNameWithExt(entry), nil
}

func (p *xdfileCommandMenuParser) currentShortName() (string, error) {
	name, err := p.currentShortNameWithExt()
	if err != nil {
		return "", err
	}
	stem, _ := xdfileCommandMenuSplitName(name)
	return stem, nil
}

func (p *xdfileCommandMenuParser) currentShortExtension() (string, error) {
	name, err := p.currentShortNameWithExt()
	if err != nil {
		return "", err
	}
	_, ext := xdfileCommandMenuSplitName(name)
	return ext, nil
}

func (p *xdfileCommandMenuParser) entryShortNameWithExt(entry xdfileEntry) string {
	p.usesShortNames = true
	if shortPath, err := xdfileCommandMenuShortPath(entry.Path); err == nil && strings.TrimSpace(shortPath) != "" {
		return filepath.Base(shortPath)
	}
	return entry.Name
}

func (p *xdfileCommandMenuParser) selectedNamesList(shortNames bool, tail string) (string, int, error) {
	entries, err := p.selectedEntries()
	if err != nil {
		return "", 0, err
	}

	quote := true
	consumed := 0
	if strings.HasPrefix(tail, "Q") {
		quote = true
		consumed = 1
	} else if strings.HasPrefix(tail, "q") {
		quote = false
		consumed = 1
	}

	values := make([]string, 0, len(entries))
	for _, entry := range entries {
		value := entry.Name
		if shortNames {
			value = p.entryShortNameWithExt(entry)
		}
		if quote {
			value = xdfileQuoteCommandMenuValue(value)
		} else {
			value = xdfileQuoteCommandMenuValueIfNeeded(value)
		}
		values = append(values, value)
	}
	return strings.Join(values, " "), consumed, nil
}

func (p *xdfileCommandMenuParser) listFile(shortNames bool, tail string) (string, int, bool, error) {
	if strings.HasPrefix(tail, "@") {
		tail = tail[1:]
	}
	end := strings.IndexByte(tail, '!')
	if end < 0 {
		return "", 0, false, nil
	}

	modifiers := tail[:end]
	path, err := p.createListFile(shortNames, modifiers)
	if err != nil {
		return "", 0, true, err
	}
	return path, end + 1, true, nil
}

func (p *xdfileCommandMenuParser) createListFile(shortNames bool, modifiers string) (string, error) {
	entries, err := p.selectedEntries()
	if err != nil {
		return "", err
	}

	encoding := xdfileCommandMenuListEncodingOEM
	fullPaths := false
	quotePaths := false
	forwardSlashes := false
	for _, modifier := range modifiers {
		switch modifier {
		case 'A':
			encoding = xdfileCommandMenuListEncodingANSI
		case 'U':
			encoding = xdfileCommandMenuListEncodingUTF8
		case 'W':
			encoding = xdfileCommandMenuListEncodingUTF16LE
		case 'F':
			fullPaths = true
		case 'Q':
			quotePaths = true
		case 'S':
			forwardSlashes = true
		}
	}

	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		value := entry.Name
		if fullPaths {
			value = entry.Path
		}
		if shortNames {
			if fullPaths {
				p.usesShortNames = true
				if shortPath, shortErr := xdfileCommandMenuShortPath(entry.Path); shortErr == nil && strings.TrimSpace(shortPath) != "" {
					value = shortPath
				}
			} else {
				value = p.entryShortNameWithExt(entry)
			}
		}
		if forwardSlashes {
			value = strings.ReplaceAll(value, `\`, "/")
		}
		if quotePaths {
			value = xdfileQuoteCommandMenuValue(value)
		}
		lines = append(lines, value)
	}

	file, err := os.CreateTemp("", "xdfile-usermenu-*.lst")
	if err != nil {
		return "", err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(file.Name())
		return "", err
	}

	content := strings.Join(lines, "\r\n")
	if len(lines) > 0 {
		content += "\r\n"
	}
	data, err := xdfileCommandMenuEncodeListText(content, encoding)
	if err != nil {
		_ = os.Remove(file.Name())
		return "", err
	}
	if err := os.WriteFile(file.Name(), data, 0o600); err != nil {
		_ = os.Remove(file.Name())
		return "", err
	}

	resultPath := file.Name()
	if shortNames {
		p.usesShortNames = true
		if shortPath, shortErr := xdfileCommandMenuShortPath(resultPath); shortErr == nil && strings.TrimSpace(shortPath) != "" {
			resultPath = shortPath
		}
	}

	p.usesListFiles = true
	p.tempFiles = append(p.tempFiles, file.Name())
	return resultPath, nil
}

func (p *xdfileCommandMenuParser) currentDrive() string {
	path := p.currentPanelDir(false)
	if volume := filepath.VolumeName(path); volume != "" {
		return strings.TrimRight(volume, `\`)
	}
	if strings.TrimSpace(path) == "" {
		return ""
	}
	return string(filepath.Separator)
}

func (p *xdfileCommandMenuParser) currentPanelPath(realPath bool, shortPath bool) string {
	path := p.currentPanelDir(realPath)
	if shortPath {
		p.usesShortNames = true
		if candidate, err := xdfileCommandMenuShortPath(path); err == nil && strings.TrimSpace(candidate) != "" {
			path = candidate
		}
	}
	return xdfileCommandMenuEnsureTrailingSeparator(path)
}

func (p *xdfileCommandMenuParser) currentPanelDir(realPath bool) string {
	panel := p.panel()
	if panel == nil {
		return ""
	}
	path := panel.Cwd
	if strings.TrimSpace(path) == "" && p.model != nil {
		path = p.model.terminal.Cwd
	}
	if realPath {
		if resolved, err := filepath.EvalSymlinks(path); err == nil && strings.TrimSpace(resolved) != "" {
			path = resolved
		}
	}
	return path
}

func (p *xdfileCommandMenuParser) currentDescription() string {
	return ""
}

func (p *xdfileCommandMenuParser) parsePrompt(command string) (xdfileCommandMenuPrompt, int, bool, error) {
	if !strings.HasPrefix(command, "!?") {
		return xdfileCommandMenuPrompt{}, 0, false, nil
	}
	if !p.allowPrompts {
		return xdfileCommandMenuPrompt{}, 0, false, nil
	}

	title, titleSize, ok := xdfileCommandMenuScanPromptSegment(command[2:], '?')
	if !ok {
		return xdfileCommandMenuPrompt{}, 0, false, nil
	}
	rest := command[2+titleSize:]
	initial, initialSize, ok := xdfileCommandMenuScanPromptSegment(rest, '!')
	if !ok {
		return xdfileCommandMenuPrompt{}, 0, false, nil
	}

	history := ""
	if strings.HasPrefix(title, "$") {
		if end := strings.IndexByte(title[1:], '$'); end >= 0 {
			history = title[1 : end+1]
			title = title[end+2:]
		}
	}

	label, err := p.expandPromptField(title)
	if err != nil {
		return xdfileCommandMenuPrompt{}, 0, false, err
	}
	initialValue, err := p.expandPromptField(initial)
	if err != nil {
		return xdfileCommandMenuPrompt{}, 0, false, err
	}

	index := len(p.prompts) + 1
	prompt := xdfileCommandMenuPrompt{
		Placeholder: fmt.Sprintf("<<__XDFILE_USER_MENU_INPUT_%d__>>", index),
		Label:       label,
		Initial:     initialValue,
		History:     history,
		Variable:    fmt.Sprintf("UserVar%d", index),
	}
	p.prompts = append(p.prompts, prompt)
	return prompt, 2 + titleSize + initialSize, true, nil
}

func (p *xdfileCommandMenuParser) expandPromptField(value string) (string, error) {
	if value == "" {
		return "", nil
	}

	var out strings.Builder
	for i := 0; i < len(value); {
		if value[i] != '(' {
			out.WriteByte(value[i])
			i++
			continue
		}

		end := xdfileCommandMenuMatchingParen(value, i)
		if end < 0 {
			out.WriteByte(value[i])
			i++
			continue
		}

		inner := value[i+1 : end]
		child := xdfileCommandMenuParser{
			model:         p.model,
			target:        p.target,
			allowPrompts:  false,
			allowBareName: p.allowBareName,
		}
		expanded, err := child.expand(inner)
		if err != nil {
			return "", err
		}
		p.tempFiles = append(p.tempFiles, child.tempFiles...)
		p.usesListFiles = p.usesListFiles || child.usesListFiles
		p.usesShortNames = p.usesShortNames || child.usesShortNames
		p.usesDescription = p.usesDescription || child.usesDescription
		p.preserveLFN = p.preserveLFN || child.preserveLFN
		if expanded == inner {
			out.WriteString(value[i : end+1])
		} else {
			out.WriteByte('(')
			out.WriteString(expanded)
			out.WriteByte(')')
		}
		i = end + 1
	}
	return out.String(), nil
}

func xdfileCommandMenuScanPromptSegment(value string, end byte) (string, int, bool) {
	depth := 0
	for i := 0; i < len(value); i++ {
		switch value[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 && value[i] == end {
				return value[:i], i + 1, true
			}
		}
	}
	return "", 0, false
}

func xdfileContainsManagedTerminalMetasymbols(command string) bool {
	for _, token := range []string{
		"!.!",
		"!-!",
		"!+!",
		"!&~",
		"!&",
		"!@@",
		"!@",
		"!$",
		"!`~",
		"!`",
		"!~",
		"!=\\",
		"!=/",
		"!\\",
		"!/",
		"!:",
		"!?!",
		"!?",
		"!##",
		"!#",
		"!^",
		"![",
		"!]",
	} {
		if strings.Contains(command, token) {
			return true
		}
	}
	return false
}

func xdfileCommandMenuMatchingParen(value string, start int) int {
	depth := 0
	for i := start; i < len(value); i++ {
		switch value[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func xdfileExpandCommandMenuVariables(command string, variables map[string]string) string {
	if len(variables) == 0 {
		return command
	}

	names := make([]string, 0, len(variables))
	for name := range variables {
		if strings.TrimSpace(name) == "" {
			continue
		}
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		if len(names[i]) == len(names[j]) {
			return strings.ToLower(names[i]) < strings.ToLower(names[j])
		}
		return len(names[i]) > len(names[j])
	})

	var out strings.Builder
	for i := 0; i < len(command); {
		if command[i] != '%' {
			out.WriteByte(command[i])
			i++
			continue
		}

		matched := ""
		for _, name := range names {
			end := i + 1 + len(name)
			if end > len(command) {
				continue
			}
			if strings.EqualFold(command[i+1:end], name) {
				matched = name
				break
			}
		}
		if matched == "" {
			out.WriteByte(command[i])
			i++
			continue
		}

		out.WriteString(variables[matched])
		i += 1 + len(matched)
	}
	return out.String()
}

func xdfileCommandMenuSplitName(name string) (string, string) {
	if name == "" {
		return "", ""
	}
	ext := filepath.Ext(name)
	if ext == "" {
		return name, ""
	}
	stem := strings.TrimSuffix(name, ext)
	if stem == "" && strings.HasPrefix(name, ".") && !strings.Contains(name[1:], ".") {
		return name, ""
	}
	return stem, strings.TrimPrefix(ext, ".")
}

func xdfileQuoteCommandMenuValue(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
}

func xdfileQuoteCommandMenuValueIfNeeded(value string) string {
	if strings.ContainsAny(value, " \t") {
		return xdfileQuoteCommandMenuValue(value)
	}
	return value
}

func xdfileCommandMenuEnsureTrailingSeparator(path string) string {
	if strings.TrimSpace(path) == "" {
		return path
	}
	if strings.HasSuffix(path, `\`) || strings.HasSuffix(path, "/") {
		return path
	}
	return path + string(filepath.Separator)
}

func xdfileCommandMenuEncodeListText(text string, encoding xdfileCommandMenuListEncoding) ([]byte, error) {
	if text == "" {
		return []byte{}, nil
	}
	switch encoding {
	case xdfileCommandMenuListEncodingUTF8:
		return []byte(text), nil
	case xdfileCommandMenuListEncodingUTF16LE:
		encoded := utf16.Encode([]rune(text))
		data := make([]byte, 0, len(encoded)*2)
		for _, value := range encoded {
			data = append(data, byte(value), byte(value>>8))
		}
		return data, nil
	default:
		return xdfileCommandMenuEncodeWithCodePage(text, encoding)
	}
}
