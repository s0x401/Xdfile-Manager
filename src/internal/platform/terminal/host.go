package terminal

import (
	"os"
	"runtime"
	"strings"
)

type HostInfo struct {
	RuntimeOS       string
	TerminalProgram string
	TerminalVersion string
	LC_Terminal     string
	Term            string
	TermFeatures    string
	ITermSessionID  string
	WTSessionID     string
	IsITerm2        bool
	IsWindowsTerm   bool
	SupportsOSC1337 bool
}

func DetectCurrent() HostInfo {
	return Detect(os.Environ())
}

func Detect(env []string) HostInfo {
	values := make(map[string]string, len(env))
	for _, entry := range env {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		values[strings.ToUpper(strings.TrimSpace(key))] = value
	}

	program := strings.TrimSpace(values["TERM_PROGRAM"])
	lcTerminal := strings.TrimSpace(values["LC_TERMINAL"])
	itermSessionID := strings.TrimSpace(values["ITERM_SESSION_ID"])
	wtSessionID := strings.TrimSpace(values["WT_SESSION"])
	isITerm2 := strings.EqualFold(program, "iTerm.app") ||
		strings.EqualFold(lcTerminal, "iTerm2") ||
		itermSessionID != ""
	isWindowsTerminal := wtSessionID != "" || strings.EqualFold(program, "Windows_Terminal")

	return HostInfo{
		RuntimeOS:       runtime.GOOS,
		TerminalProgram: program,
		TerminalVersion: strings.TrimSpace(values["TERM_PROGRAM_VERSION"]),
		LC_Terminal:     lcTerminal,
		Term:            strings.TrimSpace(values["TERM"]),
		TermFeatures:    strings.TrimSpace(values["TERM_FEATURES"]),
		ITermSessionID:  itermSessionID,
		WTSessionID:     wtSessionID,
		IsITerm2:        isITerm2,
		IsWindowsTerm:   isWindowsTerminal,
		SupportsOSC1337: isITerm2,
	}
}

func (h HostInfo) NeedsRightEdgeGuard() bool {
	if h.RuntimeOS != "windows" {
		return false
	}
	if h.IsWindowsTerm || strings.TrimSpace(h.WTSessionID) != "" {
		return false
	}
	program := strings.TrimSpace(h.TerminalProgram)
	if program != "" && !strings.EqualFold(program, "XdfileManager") {
		return false
	}
	term := strings.ToLower(strings.TrimSpace(h.Term))
	if strings.Contains(term, "xterm") || strings.Contains(term, "screen") || strings.Contains(term, "tmux") {
		return false
	}
	return true
}
