package cmd

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

type xdfileShellCommand struct {
	Raw  string
	Name string
	Args []string
}

var xdfileShellAliasMap = map[string][]string{
	"ls":  {"dir"},
	"ll":  {"dir"},
	"la":  {"dir", "/a"},
	"cat": {"type"},
}

var xdfileManagedShellDefaultSuggestions = []string{
	"cd",
	"dir",
	"ls",
	"ll",
	"la",
	"pwd",
	"clear",
	"cls",
	"type",
	"cat",
	"echo",
	"git status",
	"go test ./...",
	"go run .",
	"explorer .",
}

func xdfileRunManagedShellCommand(dir string, command string) (xdfileTerminalResultMsg, bool) {
	command = strings.TrimSpace(command)
	result := xdfileTerminalResultMsg{
		Command: command,
		Dir:     dir,
	}
	if command == "" || xdfileContainsShellOperators(command) {
		return result, false
	}

	parsed, err := xdfileParseShellCommand(command)
	if err != nil || parsed.Name == "" {
		return result, false
	}
	resolved := xdfileApplyShellAlias(parsed)

	switch strings.ToLower(resolved.Name) {
	case "pwd":
		result.Output = xdfileTerminalPromptPathStyle.Render(dir)
		return result, true
	case "echo":
		result.Output = strings.Join(resolved.Args, " ")
		return result, true
	case "cd", "chdir":
		nextDir, handled, cdErr := xdfileBuiltinCD(dir, command)
		if !handled {
			return result, false
		}
		result.Dir = nextDir
		result.Err = cdErr
		result.SyncActivePanel = cdErr == nil
		if cdErr == nil {
			result.Output = xdfileTerminalPromptPathStyle.Render(nextDir)
		}
		return result, true
	case "dir":
		return xdfileRunManagedDirCommand(dir, parsed, resolved)
	case "type":
		return xdfileRunManagedTypeCommand(dir, parsed, resolved)
	default:
		return result, false
	}
}

func xdfileRunManagedDirCommand(dir string, parsed xdfileShellCommand, resolved xdfileShellCommand) (xdfileTerminalResultMsg, bool) {
	result := xdfileTerminalResultMsg{
		Command: parsed.Raw,
		Dir:     dir,
	}

	showHidden := false
	target := dir
	pathArgs := 0
	for _, arg := range resolved.Args {
		switch strings.ToLower(arg) {
		case "/a", "-a":
			showHidden = true
		default:
			pathArgs++
			if pathArgs > 1 {
				return result, false
			}
			resolvedPath, err := xdfileResolveShellPath(dir, arg)
			if err != nil {
				result.Err = err
				return result, true
			}
			target = resolvedPath
		}
	}

	entries, err := xdfileReadEntries(target, showHidden, xdfileSortModeName)
	if err != nil {
		result.Err = err
		return result, true
	}

	longForm := strings.EqualFold(parsed.Name, "ll")
	lines := make([]string, 0, len(entries)+2)
	lines = append(lines, xdfileTagStyle.Render("Directory")+" "+xdfileTerminalPromptPathStyle.Render(target))
	lines = append(lines, xdfileDimStyle.Render(xdfileManagedDirSummary(entries)))
	for _, entry := range entries {
		lines = append(lines, xdfileRenderManagedDirEntry(entry, longForm))
	}
	result.Output = strings.Join(lines, "\n")
	return result, true
}

func xdfileRunManagedTypeCommand(dir string, parsed xdfileShellCommand, resolved xdfileShellCommand) (xdfileTerminalResultMsg, bool) {
	result := xdfileTerminalResultMsg{
		Command: parsed.Raw,
		Dir:     dir,
	}
	if len(resolved.Args) != 1 {
		return result, false
	}

	path, err := xdfileResolveShellPath(dir, resolved.Args[0])
	if err != nil {
		result.Err = err
		return result, true
	}
	if xdfileIsNetBoxPath(path) {
		result.Err = fmt.Errorf("remote file output is unavailable in the managed shell")
		return result, true
	}
	info, statErr := os.Stat(path)
	if statErr != nil {
		result.Err = statErr
		return result, true
	}
	if info.IsDir() {
		result.Err = fmt.Errorf("not a file: %s", path)
		return result, true
	}

	data, readErr := os.ReadFile(path)
	if readErr != nil {
		result.Err = readErr
		return result, true
	}
	content := strings.TrimRight(strings.ReplaceAll(xdfileDecodeCommandOutput(data), "\r\n", "\n"), "\n")
	result.Output = xdfileHighlightManagedShellText(path, content)
	return result, true
}

func xdfileManagedDirSummary(entries []xdfileEntry) string {
	dirs := 0
	files := 0
	for _, entry := range entries {
		switch {
		case entry.IsParent:
			continue
		case entry.IsDir:
			dirs++
		default:
			files++
		}
	}
	return fmt.Sprintf("%d items | %d dirs | %d files", dirs+files, dirs, files)
}

func xdfileRenderManagedDirEntry(entry xdfileEntry, longForm bool) string {
	kind := xdfileEntryKindSpecForEntry(entry)

	name := entry.Name
	nameStyle := xdfileEntryNameStyle(entry)
	if entry.IsDir && !entry.IsParent {
		separator := string(os.PathSeparator)
		if xdfileIsNetBoxPath(entry.Path) {
			separator = "/"
		}
		name = nameStyle.Render(name) + xdfileDimStyle.Render(separator)
	} else {
		name = nameStyle.Render(name)
	}

	if !longForm {
		return kind.render() + xdfileDimStyle.Render("  ") + name
	}

	size := "-"
	if !entry.IsDir && !entry.IsParent {
		size = xdfileHumanSize(entry.Size)
	}
	modified := "-"
	if !entry.IsParent {
		modified = entry.Modified.Format("2006-01-02 15:04")
	}
	return kind.render() + " " +
		xdfileMetaStyle.Render(fmt.Sprintf("%7s", size)) + " " +
		xdfileDimStyle.Render(modified) + " " +
		name
}

func xdfileResolveShellPath(cwd string, value string) (string, error) {
	value = strings.TrimSpace(strings.Trim(value, `"'`))
	if xdfileIsNetBoxPath(cwd) {
		return xdfileResolveRemoteShellPath(cwd, value)
	}
	if value == "" {
		return cwd, nil
	}
	if strings.HasPrefix(value, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		value = filepath.Join(home, strings.TrimPrefix(value, "~"))
	}
	if !filepath.IsAbs(value) {
		value = filepath.Join(cwd, value)
	}
	return filepath.Clean(value), nil
}

func xdfileResolveRemoteShellPath(cwd string, value string) (string, error) {
	remote, ok := xdfileParseNetBoxPath(cwd)
	if !ok {
		return "", fmt.Errorf("invalid SSH panel path: %s", cwd)
	}
	if value == "" || value == "." {
		return xdfileNetBoxURL(remote.Profile, remote.Path), nil
	}
	if xdfileIsNetBoxPath(value) {
		return value, nil
	}

	value = strings.ReplaceAll(value, `\`, "/")
	if strings.HasPrefix(value, "~") {
		switch {
		case value == "~":
			value = "/"
		case strings.HasPrefix(value, "~/"):
			value = "/" + strings.TrimPrefix(value, "~/")
		default:
			return "", fmt.Errorf("unsupported remote home path: %s", value)
		}
	}
	if strings.HasPrefix(value, "/") {
		return xdfileNetBoxURL(remote.Profile, value), nil
	}
	return xdfileNetBoxURL(remote.Profile, path.Join(remote.Path, value)), nil
}

func xdfileParseShellCommand(command string) (xdfileShellCommand, error) {
	fields, err := xdfileSplitShellWords(command)
	if err != nil {
		return xdfileShellCommand{}, err
	}
	if len(fields) == 0 {
		return xdfileShellCommand{Raw: strings.TrimSpace(command)}, nil
	}
	return xdfileShellCommand{
		Raw:  strings.TrimSpace(command),
		Name: fields[0],
		Args: fields[1:],
	}, nil
}

func xdfileSplitShellWords(command string) ([]string, error) {
	var (
		fields []string
		token  strings.Builder
		quote  rune
	)

	flush := func() {
		if token.Len() == 0 {
			return
		}
		fields = append(fields, token.String())
		token.Reset()
	}

	for _, r := range command {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
				continue
			}
			token.WriteRune(r)
		case r == '"' || r == '\'':
			quote = r
		case unicode.IsSpace(r):
			flush()
		default:
			token.WriteRune(r)
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote")
	}
	flush()
	return fields, nil
}

func xdfileApplyShellAlias(command xdfileShellCommand) xdfileShellCommand {
	alias, ok := xdfileShellAliasMap[strings.ToLower(command.Name)]
	if !ok || len(alias) == 0 {
		return command
	}
	merged := append(append([]string{}, alias...), command.Args...)
	return xdfileShellCommand{
		Raw:  command.Raw,
		Name: merged[0],
		Args: merged[1:],
	}
}

func xdfileContainsShellOperators(command string) bool {
	var quote rune
	for _, r := range command {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			}
		case r == '"' || r == '\'':
			quote = r
		case strings.ContainsRune("|&<>", r):
			return true
		}
	}
	return false
}

func xdfileManagedShellSuggestions(input string, cwd string, history []string) []string {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil
	}

	suggestions := make([]string, 0, 16)
	seen := make(map[string]struct{}, 16)
	inputLower := strings.ToLower(input)
	add := func(candidate string) {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" || strings.EqualFold(candidate, input) {
			return
		}
		if !strings.HasPrefix(strings.ToLower(candidate), inputLower) {
			return
		}
		if _, ok := seen[candidate]; ok {
			return
		}
		seen[candidate] = struct{}{}
		suggestions = append(suggestions, candidate)
	}

	for i := len(history) - 1; i >= 0; i-- {
		add(history[i])
	}
	for _, candidate := range xdfileManagedShellDefaultSuggestions {
		add(candidate)
	}
	for _, candidate := range xdfileManagedShellPathSuggestions(input, cwd) {
		add(candidate)
	}
	return suggestions
}

func xdfileManagedShellPathSuggestions(input string, cwd string) []string {
	if xdfileIsNetBoxPath(cwd) {
		return nil
	}
	if strings.HasSuffix(input, " ") {
		return nil
	}

	fields, err := xdfileSplitShellWords(input)
	if err != nil || len(fields) == 0 {
		return nil
	}

	commandName := strings.ToLower(fields[0])
	if len(fields) <= 1 && !xdfileShellCommandExpectsPath(commandName) {
		return nil
	}

	lastSpace := strings.LastIndexAny(input, " \t")
	if lastSpace < 0 || lastSpace >= len(input)-1 {
		return nil
	}

	base := input[:lastSpace+1]
	partial := strings.TrimSpace(input[lastSpace+1:])
	if partial == "" {
		return nil
	}

	quoted := partial[0] == '"' || partial[0] == '\''
	quoteChar := byte(0)
	if quoted {
		quoteChar = partial[0]
		partial = strings.TrimPrefix(partial, string(quoteChar))
	}
	lookup, err := xdfileResolveShellPath(cwd, partial)
	if err != nil {
		return nil
	}

	searchDir := cwd
	namePrefix := partial
	if dirPart, filePart := filepath.Split(lookup); dirPart != "" {
		searchDir = filepath.Clean(dirPart)
		namePrefix = filePart
	}
	namePrefixLower := strings.ToLower(namePrefix)

	items, err := os.ReadDir(searchDir)
	if err != nil {
		return nil
	}

	results := make([]string, 0, len(items))
	for _, item := range items {
		name := item.Name()
		if !strings.HasPrefix(strings.ToLower(name), namePrefixLower) {
			continue
		}

		resolved := name
		if partialDir, _ := filepath.Split(partial); partialDir != "" {
			resolved = filepath.Join(partialDir, name)
		}
		if item.IsDir() {
			resolved += string(os.PathSeparator)
		}
		if quoted || strings.ContainsRune(resolved, ' ') {
			resolved = string(maxByte(quoteChar, '"')) + resolved + string(maxByte(quoteChar, '"'))
		}
		results = append(results, base+resolved)
	}
	sort.Strings(results)
	return results
}

func xdfileShellCommandExpectsPath(name string) bool {
	switch strings.ToLower(name) {
	case "cd", "chdir", "dir", "ls", "ll", "la", "type", "cat":
		return true
	default:
		return false
	}
}

func maxByte(left byte, right byte) byte {
	if left != 0 {
		return left
	}
	return right
}
