package cmd

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

const xdfileGitStatusTimeout = 900 * time.Millisecond

type xdfileGitPanelInfo struct {
	Active  bool
	Branch  string
	Dirty   bool
	Ahead   int
	Behind  int
	Markers map[string]string
}

var xdfileReadGitStatusFunc = xdfileReadGitStatus

func (g xdfileGitPanelInfo) TitleLabel() string {
	if !g.Active {
		return ""
	}
	label := "git"
	if g.Branch != "" {
		label = "git:" + g.Branch
	}
	if g.Dirty {
		label += "*"
	}
	if g.Ahead > 0 {
		label += "+" + strconv.Itoa(g.Ahead)
	}
	if g.Behind > 0 {
		label += "-" + strconv.Itoa(g.Behind)
	}
	return label
}

func xdfileReadGitStatus(dir string) xdfileGitPanelInfo {
	if strings.TrimSpace(dir) == "" {
		return xdfileGitPanelInfo{}
	}
	if _, err := exec.LookPath("git"); err != nil {
		return xdfileGitPanelInfo{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), xdfileGitStatusTimeout)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		"git",
		"-C", dir,
		"status",
		"--porcelain=v1",
		"--branch",
		"--untracked-files=normal",
		"--no-renames",
		"--",
		".",
	)
	output, err := cmd.Output()
	if err != nil {
		return xdfileGitPanelInfo{}
	}

	return xdfileParseGitStatusOutput(string(output))
}

func xdfileParseGitStatusOutput(output string) xdfileGitPanelInfo {
	info := xdfileGitPanelInfo{}
	lines := strings.Split(strings.ReplaceAll(output, "\r\n", "\n"), "\n")
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.HasPrefix(line, "## ") {
			info.Active = true
			info.Branch, info.Ahead, info.Behind = xdfileParseGitBranchLine(strings.TrimPrefix(line, "## "))
			continue
		}
		if len(line) < 3 {
			continue
		}
		marker := xdfileGitMarkerFromCode(line[:2])
		if marker == "" {
			continue
		}
		path := xdfileGitUnquotePath(strings.TrimSpace(line[3:]))
		child := xdfileGitDirectChild(path)
		if child == "" {
			continue
		}
		if info.Markers == nil {
			info.Markers = make(map[string]string)
		}
		info.Active = true
		info.Dirty = true
		info.Markers[child] = xdfileChooseGitMarker(info.Markers[child], marker)
	}
	return info
}

func xdfileParseGitBranchLine(line string) (branch string, ahead int, behind int) {
	line = strings.TrimSpace(line)
	switch {
	case strings.HasPrefix(line, "No commits yet on "):
		branch = strings.TrimSpace(strings.TrimPrefix(line, "No commits yet on "))
	case strings.HasPrefix(line, "HEAD (no branch)"):
		branch = "detached"
	default:
		head := line
		if idx := strings.Index(head, " ["); idx >= 0 {
			head = head[:idx]
		}
		if idx := strings.Index(head, "..."); idx >= 0 {
			head = head[:idx]
		}
		branch = strings.TrimSpace(head)
	}

	if idx := strings.Index(line, "["); idx >= 0 && strings.HasSuffix(line, "]") {
		for _, item := range strings.Split(line[idx+1:len(line)-1], ",") {
			item = strings.TrimSpace(item)
			switch {
			case strings.HasPrefix(item, "ahead "):
				ahead, _ = strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(item, "ahead ")))
			case strings.HasPrefix(item, "behind "):
				behind, _ = strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(item, "behind ")))
			}
		}
	}
	return branch, ahead, behind
}

func xdfileGitMarkerFromCode(code string) string {
	switch {
	case code == "??":
		return "?"
	case code == "!!":
		return "!"
	case strings.Contains(code, "U"):
		return "U"
	case strings.Contains(code, "A"):
		return "A"
	case strings.Contains(code, "D"):
		return "D"
	case strings.Contains(code, "R"), strings.Contains(code, "C"):
		return "R"
	case strings.Contains(code, "M"), strings.Contains(code, "T"):
		return "M"
	default:
		return ""
	}
}

func xdfileChooseGitMarker(current string, next string) string {
	if xdfileGitMarkerPriority(next) > xdfileGitMarkerPriority(current) {
		return next
	}
	if current == "" {
		return next
	}
	return current
}

func xdfileGitMarkerPriority(marker string) int {
	switch marker {
	case "U":
		return 7
	case "D":
		return 6
	case "A":
		return 5
	case "R":
		return 4
	case "M":
		return 3
	case "?":
		return 2
	case "!":
		return 1
	default:
		return 0
	}
}

func xdfileGitUnquotePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, `"`) && strings.HasSuffix(path, `"`) {
		if unquoted, err := strconv.Unquote(path); err == nil {
			path = unquoted
		}
	}
	return filepath.Clean(strings.ReplaceAll(path, "/", string(os.PathSeparator)))
}

func xdfileGitDirectChild(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || path == "." {
		return ""
	}
	parts := strings.FieldsFunc(path, func(r rune) bool {
		return r == '/' || r == '\\'
	})
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func xdfileApplyGitStatus(entries []xdfileEntry, info xdfileGitPanelInfo) []xdfileEntry {
	if len(entries) == 0 || len(info.Markers) == 0 {
		return entries
	}
	for i := range entries {
		if entries[i].IsParent {
			continue
		}
		entries[i].GitMarker = info.Markers[entries[i].Name]
	}
	return entries
}

func xdfileGitMarkerPrefix(marker string) string {
	if marker == "" {
		return " "
	}
	return marker
}

func xdfileGitMarkerStyle(marker string) lipgloss.Style {
	switch marker {
	case "A":
		return xdfileStatusOKStyle
	case "D", "U":
		return xdfileStatusErrStyle
	case "M", "R":
		return xdfileTitleStyle
	case "?":
		return xdfileTagStyle
	case "!":
		return xdfileDimStyle
	default:
		return xdfileMetaStyle
	}
}

func xdfileRenderEntryName(nameStyle lipgloss.Style, marker string, prefixKind string, name string) string {
	left := prefixKind
	if marker == "" {
		left += " "
	} else {
		left += xdfileGitMarkerStyle(marker).Render(marker)
	}
	return left + " " + nameStyle.Render(name)
}
