package terminal

import "testing"

func TestDetectRecognizesITerm2Signals(t *testing.T) {
	info := Detect([]string{
		"TERM_PROGRAM=iTerm.app",
		"TERM_PROGRAM_VERSION=3.6.9",
		"ITERM_SESSION_ID=w0t0p0",
	})

	if !info.IsITerm2 {
		t.Fatalf("expected iTerm2 to be detected")
	}
	if !info.SupportsOSC1337 {
		t.Fatalf("expected OSC 1337 support when iTerm2 is detected")
	}
	if info.TerminalProgram != "iTerm.app" {
		t.Fatalf("expected terminal program to be preserved, got %q", info.TerminalProgram)
	}
}

func TestDetectRecognizesNonITerm2Terminal(t *testing.T) {
	info := Detect([]string{
		"TERM_PROGRAM=Apple_Terminal",
		"TERM=xterm-256color",
	})

	if info.IsITerm2 {
		t.Fatalf("did not expect Apple Terminal to be detected as iTerm2")
	}
	if info.SupportsOSC1337 {
		t.Fatalf("did not expect OSC 1337 support for non-iTerm2 terminal")
	}
}

func TestDetectRecognizesWindowsTerminal(t *testing.T) {
	info := Detect([]string{
		"WT_SESSION=65e1",
		"TERM_PROGRAM=Windows_Terminal",
	})

	if !info.IsWindowsTerm {
		t.Fatalf("expected Windows Terminal to be detected")
	}
	if info.WTSessionID != "65e1" {
		t.Fatalf("expected WT_SESSION to be preserved, got %q", info.WTSessionID)
	}
}

func TestNeedsRightEdgeGuardForWindowsConsoleHosts(t *testing.T) {
	cases := []struct {
		name string
		info HostInfo
		want bool
	}{
		{
			name: "legacy windows console",
			info: HostInfo{RuntimeOS: "windows"},
			want: true,
		},
		{
			name: "windows terminal",
			info: HostInfo{RuntimeOS: "windows", WTSessionID: "65e1", IsWindowsTerm: true},
			want: false,
		},
		{
			name: "vscode terminal",
			info: HostInfo{RuntimeOS: "windows", TerminalProgram: "vscode"},
			want: false,
		},
		{
			name: "xterm compatible terminal",
			info: HostInfo{RuntimeOS: "windows", Term: "xterm-256color"},
			want: false,
		},
		{
			name: "linux",
			info: HostInfo{RuntimeOS: "linux"},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.info.NeedsRightEdgeGuard(); got != tc.want {
				t.Fatalf("NeedsRightEdgeGuard() = %v, want %v", got, tc.want)
			}
		})
	}
}
