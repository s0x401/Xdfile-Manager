package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/unicode"
)

const xdfileChineseOutputSample = "\u4e2d\u6587\u8f93\u51fa"

func TestReadDirectoryPreviewUsesStructuralMarkers(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tool.exe"), []byte("exe"), 0o644); err != nil {
		t.Fatalf("write exe fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "scan.ps1"), []byte("script"), 0o644); err != nil {
		t.Fatalf("write script fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sample.7z"), []byte("archive"), 0o644); err != nil {
		t.Fatalf("write archive fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "libcurl.dll"), []byte("dll"), 0o644); err != nil {
		t.Fatalf("write dll fixture: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatalf("write directory fixture: %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat preview dir: %v", err)
	}

	preview, err := xdfileReadDirectoryPreview(dir, info)
	if err != nil {
		t.Fatalf("read directory preview: %v", err)
	}

	plain := stripXdfileANSI(preview)
	if !strings.Contains(plain, "[E] tool.exe") {
		t.Fatalf("expected preview to render executable marker, got %q", plain)
	}
	if !strings.Contains(plain, "[S] scan.ps1") {
		t.Fatalf("expected preview to render script marker, got %q", plain)
	}
	if !strings.Contains(plain, "[Z] sample.7z") {
		t.Fatalf("expected preview to render archive marker, got %q", plain)
	}
	if !strings.Contains(plain, "[ ] libcurl.dll") {
		t.Fatalf("expected preview to render neutral file marker for dll, got %q", plain)
	}
	if !strings.Contains(plain, "[D] assets") {
		t.Fatalf("expected preview to render directory marker, got %q", plain)
	}
}

func TestXdfileStreamCommandOutputTreatsCarriageReturnAsRewrite(t *testing.T) {
	reader := strings.NewReader("step 1\rstep 2\rstep 3\nfinal line\n")
	var lines []xdfileTerminalLineMsg

	err := xdfileStreamCommandOutput(reader, func(line string, rewrite bool, finalize bool) {
		lines = append(lines, xdfileTerminalLineMsg{Line: line, Rewrite: rewrite, Finalize: finalize})
	})
	if err != nil && err != io.EOF {
		t.Fatalf("stream command output: %v", err)
	}

	expected := []xdfileTerminalLineMsg{
		{Line: "step 1", Rewrite: true, Finalize: false},
		{Line: "step 2", Rewrite: true, Finalize: false},
		{Line: "step 3", Rewrite: false, Finalize: true},
		{Line: "final line", Rewrite: false, Finalize: true},
	}
	if len(lines) != len(expected) {
		t.Fatalf("expected %d streamed updates, got %d: %+v", len(expected), len(lines), lines)
	}
	for i := range expected {
		if lines[i] != expected[i] {
			t.Fatalf("expected streamed update %d to be %+v, got %+v", i, expected[i], lines[i])
		}
	}
}

func TestXdfileStreamCommandOutputTreatsCRLFAsNormalLine(t *testing.T) {
	reader := strings.NewReader("first line\r\nsecond line\r\n")
	var lines []xdfileTerminalLineMsg

	if err := xdfileStreamCommandOutput(reader, func(line string, rewrite bool, finalize bool) {
		lines = append(lines, xdfileTerminalLineMsg{Line: line, Rewrite: rewrite, Finalize: finalize})
	}); err != nil && err != io.EOF {
		t.Fatalf("stream command output: %v", err)
	}

	expected := []xdfileTerminalLineMsg{
		{Line: "first line", Rewrite: true, Finalize: true},
		{Line: "second line", Rewrite: true, Finalize: true},
	}
	if len(lines) != len(expected) {
		t.Fatalf("expected %d streamed lines, got %d: %+v", len(expected), len(lines), lines)
	}
	for i := range expected {
		if lines[i] != expected[i] {
			t.Fatalf("expected streamed line %d to be %+v, got %+v", i, expected[i], lines[i])
		}
	}
}

func TestXdfileCopyPathRejectsTargetInsideSource(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.MkdirAll(filepath.Join(source, "nested"), 0o755); err != nil {
		t.Fatalf("create source tree: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "sample.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write sample file: %v", err)
	}

	target := filepath.Join(source, "nested", "copy")
	err := xdfileCopyPath(source, target)
	if err == nil {
		t.Fatalf("expected nested copy target to be rejected")
	}
	if !strings.Contains(err.Error(), "inside source") {
		t.Fatalf("expected nested copy error, got %v", err)
	}
}

func TestXdfileMovePathRejectsTargetInsideSource(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.MkdirAll(filepath.Join(source, "nested"), 0o755); err != nil {
		t.Fatalf("create source tree: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "sample.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write sample file: %v", err)
	}

	target := filepath.Join(source, "nested", "copy")
	err := xdfileMovePath(source, target)
	if err == nil {
		t.Fatalf("expected nested move target to be rejected")
	}
	if !strings.Contains(err.Error(), "inside source") {
		t.Fatalf("expected nested move error, got %v", err)
	}
}

func TestXdfileHumanSizeFitsPanelColumn(t *testing.T) {
	cases := []int64{
		0,
		999,
		1023,
		1024,
		1536,
		10 * 1024 * 1024,
		125 * 1024 * 1024,
		999 * 1024 * 1024 * 1024,
		5 * 1024 * 1024 * 1024 * 1024,
	}

	for _, size := range cases {
		label := xdfileHumanSize(size)
		if got := len(label); got > xdfilePanelSizeWidth {
			t.Fatalf("expected size %d to fit width %d, got %q (%d chars)", size, xdfilePanelSizeWidth, label, got)
		}
	}
}

func TestXdfileDecodeCommandOutputHandlesGB18030(t *testing.T) {
	encoded, err := simplifiedchinese.GB18030.NewEncoder().Bytes([]byte(xdfileChineseOutputSample))
	if err != nil {
		t.Fatalf("encode GB18030 sample: %v", err)
	}

	got := xdfileDecodeCommandOutput(encoded)
	if got != xdfileChineseOutputSample {
		t.Fatalf("expected GB18030 output to decode to %q, got %q", xdfileChineseOutputSample, got)
	}
}

func TestXdfileDecodeCommandOutputHandlesUTF16LE(t *testing.T) {
	encoded, err := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewEncoder().Bytes([]byte(xdfileChineseOutputSample))
	if err != nil {
		t.Fatalf("encode UTF-16LE sample: %v", err)
	}

	got := xdfileDecodeCommandOutput(encoded)
	if got != xdfileChineseOutputSample {
		t.Fatalf("expected UTF-16LE output to decode to %q, got %q", xdfileChineseOutputSample, got)
	}
}

func TestXdfileSanitizeManagedTerminalTextPreservesNewlinesAndColor(t *testing.T) {
	input := "line 1\n\x1b[36mcyan\x1b[0m\x1b[2Jline 2"
	got := xdfileSanitizeManagedTerminalText(input)
	want := "line 1\n\x1b[36mcyan\x1b[0mline 2"
	if got != want {
		t.Fatalf("expected sanitized text %q, got %q", want, got)
	}
}

func TestXdfileNormalizeWindowsExitCodeTreatsUnsignedAsSigned32(t *testing.T) {
	code, raw := xdfileNormalizeWindowsExitCode(4294967274)
	if code != -22 {
		t.Fatalf("expected signed exit code -22, got %d", code)
	}
	if raw != 0xFFFFFFEA {
		t.Fatalf("expected raw exit code 0xFFFFFFEA, got 0x%08x", raw)
	}
}

func TestXdfileBuiltinCDSupportsWindowsCmdSyntax(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows cmd syntax only applies on Windows")
	}

	dir := t.TempDir()
	next, handled, err := xdfileBuiltinCD(dir, `cd /d "`+dir+`"`)
	if !handled {
		t.Fatal("expected cd /d to be handled")
	}
	if err != nil {
		t.Fatalf("expected cd /d to succeed: %v", err)
	}
	if next != dir {
		t.Fatalf("expected cd /d to resolve %q, got %q", dir, next)
	}

	volume := filepath.VolumeName(dir)
	if volume == "" {
		t.Skip("temp dir has no Windows volume")
	}
	next, handled, err = xdfileBuiltinCD(dir, volume)
	if !handled {
		t.Fatal("expected bare drive switch to be handled")
	}
	if err != nil {
		t.Fatalf("expected bare drive switch to succeed: %v", err)
	}
	if next != filepath.Clean(volume+`\`) {
		t.Fatalf("expected bare drive switch to resolve %q, got %q", filepath.Clean(volume+`\`), next)
	}
}

func TestXdfileRunCommandUsesCmdOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("cmd backend only applies on Windows")
	}

	dir := t.TempDir()
	result := xdfileRunCommand(dir, `if defined COMSPEC echo cmd`)
	if result.Err != nil {
		t.Fatalf("expected cmd syntax command to succeed: %v", result.Err)
	}
	if !strings.Contains(strings.ToLower(result.Output), "cmd") {
		t.Fatalf("expected cmd backend output to contain %q, got %q", "cmd", result.Output)
	}
}

func TestXdfileExternalCommandInvocationUsesScriptForQuotedPipesOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("script fallback only applies on Windows")
	}

	command := `call "C:\tools\showarg.cmd" "xdfile.log" | clip`
	shell, args, cleanup, err := xdfileExternalCommandInvocation("", command)
	if err != nil {
		t.Fatalf("prepare external command invocation: %v", err)
	}
	defer cleanup()

	if !filepath.IsAbs(shell) || !strings.HasSuffix(strings.ToLower(shell), "cmd.exe") {
		t.Fatalf("expected absolute Windows cmd.exe path, got %q", shell)
	}
	if len(args) != 3 || args[0] != "/d" || args[1] != "/c" {
		t.Fatalf("expected cmd script invocation args, got %+v", args)
	}
	if filepath.Ext(args[2]) != ".cmd" {
		t.Fatalf("expected generated script path, got %+v", args)
	}

	data, readErr := os.ReadFile(args[2])
	if readErr != nil {
		t.Fatalf("read generated script: %v", readErr)
	}
	if len(data) == 0 || data[0] != '@' {
		t.Fatalf("expected generated script to start with plain ASCII @echo off, got bytes %#v", data[:min(len(data), 4)])
	}
	text := string(data)
	if !strings.Contains(text, `call "C:\tools\showarg.cmd" "xdfile.log" | clip`) {
		t.Fatalf("expected generated script to contain command %q, got %q", command, text)
	}
}

func TestXdfileExternalCommandInvocationUsesScriptForQuotedArgsOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("quoted command fallback only applies on Windows")
	}

	command := `cobra "libcurl.dll"`
	shell, args, cleanup, err := xdfileExternalCommandInvocation("", command)
	if err != nil {
		t.Fatalf("prepare external command invocation: %v", err)
	}
	defer cleanup()

	if !filepath.IsAbs(shell) || !strings.HasSuffix(strings.ToLower(shell), "cmd.exe") {
		t.Fatalf("expected absolute Windows cmd.exe path, got %q", shell)
	}
	if len(args) != 3 || args[0] != "/d" || args[1] != "/c" {
		t.Fatalf("expected cmd script invocation args, got %+v", args)
	}
	if filepath.Ext(args[2]) != ".cmd" {
		t.Fatalf("expected generated script path, got %+v", args)
	}

	data, readErr := os.ReadFile(args[2])
	if readErr != nil {
		t.Fatalf("read generated script: %v", readErr)
	}
	if len(data) == 0 || data[0] != '@' {
		t.Fatalf("expected generated script to start with plain ASCII @echo off, got bytes %#v", data[:min(len(data), 4)])
	}
	text := string(data)
	if !strings.Contains(text, `cobra "libcurl.dll"`) {
		t.Fatalf("expected generated script to contain command %q, got %q", command, text)
	}
}

func TestXdfileWindowsCommandScriptBodyCallsNestedBatchFiles(t *testing.T) {
	command := strings.Join([]string{
		`strings.cmd -u "sample.bin" > "D:\log\sample.nofilter"`,
		`timeout 1`,
		`"C:\Tools\scan.bat" "sample.bin"`,
		`call already.cmd`,
		`@helper.cmd /q`,
	}, "\n")

	got := xdfileWindowsCommandScriptBody("", command)
	wantLines := []string{
		`call strings.cmd -u "sample.bin" > "D:\log\sample.nofilter"`,
		`timeout 1`,
		`call "C:\Tools\scan.bat" "sample.bin"`,
		`call already.cmd`,
		`@call helper.cmd /q`,
	}
	for _, want := range wantLines {
		if !strings.Contains(got, want) {
			t.Fatalf("expected generated command script to contain %q, got %q", want, got)
		}
	}
	if strings.Contains(got, `call call already.cmd`) {
		t.Fatalf("expected existing call command not to be double-wrapped, got %q", got)
	}
}

func TestXdfileWindowsCommandScriptBodyStartsGUIExeDetached(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("detached GUI script rewriting only applies on Windows")
	}

	original := xdfileIsDetachedGUIExecutableFunc
	defer func() { xdfileIsDetachedGUIExecutableFunc = original }()

	dir := t.TempDir()
	exePath := filepath.Join(dir, "notepad++.exe")
	if err := os.WriteFile(exePath, []byte("fake"), 0o644); err != nil {
		t.Fatalf("write fake exe: %v", err)
	}
	xdfileIsDetachedGUIExecutableFunc = func(path string) bool {
		return xdfilePathsEqual(path, exePath)
	}

	got := xdfileWindowsCommandScriptBody(dir, `notepad++.exe "sample file.txt"`)
	want := `start "" notepad++.exe "sample file.txt"`
	if !strings.Contains(got, want) {
		t.Fatalf("expected GUI exe line to be started detached as %q, got %q", want, got)
	}
}

func TestXdfileWindowsCommandScriptBodyDoesNotDetachGUIExeWithShellOperators(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("detached GUI script rewriting only applies on Windows")
	}

	original := xdfileIsDetachedGUIExecutableFunc
	defer func() { xdfileIsDetachedGUIExecutableFunc = original }()

	dir := t.TempDir()
	exePath := filepath.Join(dir, "notepad++.exe")
	if err := os.WriteFile(exePath, []byte("fake"), 0o644); err != nil {
		t.Fatalf("write fake exe: %v", err)
	}
	xdfileIsDetachedGUIExecutableFunc = func(path string) bool {
		return xdfilePathsEqual(path, exePath)
	}

	command := `notepad++.exe "sample file.txt" > out.txt`
	got := xdfileWindowsCommandScriptBody(dir, command)
	if strings.Contains(got, `start ""`) {
		t.Fatalf("expected GUI exe line with shell operators not to be detached, got %q", got)
	}
	if !strings.Contains(got, command) {
		t.Fatalf("expected original command line to be preserved, got %q", got)
	}
}

func TestXdfileDetachedExternalCommandCandidateDetectsQuotedGUIExe(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("detached GUI command detection only applies on Windows")
	}

	original := xdfileIsDetachedGUIExecutableFunc
	defer func() { xdfileIsDetachedGUIExecutableFunc = original }()

	dir := t.TempDir()
	exePath := filepath.Join(dir, "die.exe")
	if err := os.WriteFile(exePath, []byte("fake"), 0o644); err != nil {
		t.Fatalf("write fake exe: %v", err)
	}
	xdfileIsDetachedGUIExecutableFunc = func(path string) bool {
		return xdfilePathsEqual(path, exePath)
	}

	path, args, ok := xdfileDetachedExternalCommandCandidate(dir, fmt.Sprintf(`"%s" "sample file.bin"`, filepath.ToSlash(exePath)))
	if !ok {
		t.Fatalf("expected quoted GUI exe command to be detached")
	}
	if !xdfilePathsEqual(path, exePath) {
		t.Fatalf("expected detached path %q, got %q", exePath, path)
	}
	if len(args) != 1 || args[0] != "sample file.bin" {
		t.Fatalf("expected parsed quoted arg, got %+v", args)
	}
}

func TestXdfileDetachedExternalCommandCandidateRejectsShellOperators(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("detached GUI command detection only applies on Windows")
	}

	original := xdfileIsDetachedGUIExecutableFunc
	defer func() { xdfileIsDetachedGUIExecutableFunc = original }()

	dir := t.TempDir()
	exePath := filepath.Join(dir, "die.exe")
	if err := os.WriteFile(exePath, []byte("fake"), 0o644); err != nil {
		t.Fatalf("write fake exe: %v", err)
	}
	xdfileIsDetachedGUIExecutableFunc = func(path string) bool {
		return xdfilePathsEqual(path, exePath)
	}

	if _, _, ok := xdfileDetachedExternalCommandCandidate(dir, fmt.Sprintf(`"%s" "sample.bin" | clip`, exePath)); ok {
		t.Fatalf("expected command with shell operator to stay in normal blocking execution path")
	}
}

func TestXdfileRunManagedShellCommandSupportsLSAlias(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "alpha.txt"), []byte("alpha"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "docs"), 0o755); err != nil {
		t.Fatalf("create test dir: %v", err)
	}

	result, handled := xdfileRunManagedShellCommand(dir, "ls")
	if !handled {
		t.Fatal("expected ls alias to be handled internally")
	}
	if result.Err != nil {
		t.Fatalf("expected ls alias to succeed: %v", result.Err)
	}
	if !strings.Contains(result.Output, "alpha.txt") || !strings.Contains(result.Output, "docs") {
		t.Fatalf("expected ls output to include test entries, got %q", result.Output)
	}
	if !strings.Contains(result.Output, "Directory ") || !strings.Contains(result.Output, "items |") {
		t.Fatalf("expected ls output to include styled shell header lines, got %q", result.Output)
	}
}

func TestXdfileRunManagedShellCommandSupportsCatAlias(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	result, handled := xdfileRunManagedShellCommand(dir, "cat note.txt")
	if !handled {
		t.Fatal("expected cat alias to be handled internally")
	}
	if result.Err != nil {
		t.Fatalf("expected cat alias to succeed: %v", result.Err)
	}
	if result.Output != "hello" {
		t.Fatalf("expected cat output %q, got %q", "hello", result.Output)
	}
}

func TestXdfilePrepareExternalCommandForcesGitColor(t *testing.T) {
	got := xdfilePrepareExternalCommand("git status --short")
	if !strings.Contains(got, "color.ui=always") || !strings.Contains(got, "core.pager=cat") {
		t.Fatalf("expected git command to be forced colorful, got %q", got)
	}
}

func TestXdfilePrepareExternalCommandPreservesExplicitGitColor(t *testing.T) {
	command := "git --color=always status"
	if got := xdfilePrepareExternalCommand(command); got != command {
		t.Fatalf("expected explicit git color command to be preserved, got %q", got)
	}
}

func TestXdfileCommandExecutionEnvironmentIncludesColorFlags(t *testing.T) {
	env := xdfileCommandExecutionEnvironment([]string{"PATH=C:\\Tools"})
	required := []string{
		"COLORTERM=truecolor",
		"CLICOLOR=1",
		"CLICOLOR_FORCE=1",
		"FORCE_COLOR=1",
		"TERM_PROGRAM=XdfileManager",
	}
	for _, want := range required {
		if !containsString(env, want) {
			t.Fatalf("expected command environment to include %q, got %+v", want, env)
		}
	}
	for _, legacySpoof := range []string{"ConEmuANSI=ON", "ANSICON=1"} {
		if containsString(env, legacySpoof) {
			t.Fatalf("expected command environment not to spoof terminal host with %q, got %+v", legacySpoof, env)
		}
	}
	if runtime.GOOS == "windows" {
		if containsString(env, "TERM=xterm-256color") {
			t.Fatalf("expected Windows command environment not to force xterm TERM, got %+v", env)
		}
	} else if !containsString(env, "TERM=xterm-256color") {
		t.Fatalf("expected non-Windows command environment to include TERM=xterm-256color, got %+v", env)
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
