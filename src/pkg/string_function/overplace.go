// ====================== overplace these is from the lipgloss PR ==============

// These code is from the https://github.com/charmbracelet/lipgloss/pull/102
// Thanks a lot!!!!!

// Edit - cutLeft has been replaced with charmansi.TruncateLeft.
// See https://github.com/charmbracelet/lipgloss/pull/102#issuecomment-2900110821

// =============================================================================
package stringfunction

import (
	"strings"

	charmansi "github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

// whitespace is a whitespace renderer.
type whitespace struct {
	style termenv.Style
	chars string
}

type WhitespaceOption func(*whitespace)

// Render whitespaces.
func (w whitespace) render(width int) string {
	if w.chars == "" {
		w.chars = " "
	}

	r := []rune(w.chars)
	j := 0
	b := strings.Builder{}

	// Cycle through runes and print them into the whitespace.
	for i := 0; i < width; {
		next := r[j]
		b.WriteRune(next)
		j++
		if j >= len(r) {
			j = 0
		}
		i += charmansi.StringWidth(string(next))
	}

	// Fill any extra gaps white spaces. This might be necessary if any runes
	// are more than one cell wide, which could leave a one-rune gap.
	short := width - charmansi.StringWidth(b.String())
	if short > 0 {
		b.WriteString(strings.Repeat(" ", short))
	}

	return w.style.Styled(b.String())
}

// PlaceOverlay places fg on top of bg.
func PlaceOverlay(x, y int, fg, bg string, opts ...WhitespaceOption) string {
	fgLines, fgWidth := getLines(fg)
	bgLines, bgWidth := getLines(bg)
	bgHeight := len(bgLines)
	fgHeight := len(fgLines)

	if fg == "" || fgHeight == 0 {
		return bg
	}
	if bg == "" || bgHeight == 0 || bgWidth <= 0 {
		return fg
	}

	overlayWidth := min(fgWidth, bgWidth)
	overlayHeight := min(fgHeight, bgHeight)
	x = clamp(x, 0, max(0, bgWidth-overlayWidth))
	y = clamp(y, 0, max(0, bgHeight-overlayHeight))

	ws := &whitespace{}
	for _, opt := range opts {
		opt(ws)
	}

	var b strings.Builder
	for i, bgLine := range bgLines {
		if i > 0 {
			b.WriteByte('\n')
		}
		if i < y || i >= y+fgHeight {
			b.WriteString(padDisplayWidth(bgLine, bgWidth, *ws))
			continue
		}

		bgLineWidth := charmansi.StringWidth(bgLine)
		lineWidth := max(bgWidth, bgLineWidth)
		fgLine := charmansi.Truncate(fgLines[i-y], max(0, lineWidth-x), "")
		fgLineWidth := charmansi.StringWidth(fgLine)

		left := charmansi.Cut(bgLine, 0, x)
		leftWidth := charmansi.StringWidth(left)
		b.WriteString(left)
		if leftWidth < x {
			b.WriteString(ws.render(x - leftWidth))
		}

		b.WriteString(fgLine)

		rightStart := x + fgLineWidth
		if rightStart < bgLineWidth {
			right := charmansi.Cut(bgLine, rightStart, bgLineWidth)
			rightWidth := charmansi.StringWidth(right)
			if gap := lineWidth - rightStart - rightWidth; gap > 0 {
				b.WriteString(ws.render(gap))
			}
			b.WriteString(right)
			continue
		}

		if rightStart < lineWidth {
			b.WriteString(ws.render(lineWidth - rightStart))
		}
	}

	return b.String()
}

func padDisplayWidth(value string, width int, ws whitespace) string {
	if width <= 0 {
		return ""
	}
	padding := width - charmansi.StringWidth(value)
	if padding <= 0 {
		return value
	}
	return value + ws.render(padding)
}

func clamp(v, lower, upper int) int {
	return min(max(v, lower), upper)
}

// Split a string into lines, additionally returning the size of the widest
// line.
func getLines(s string) ([]string, int) {
	lines := strings.Split(s, "\n")
	widest := 0
	for _, l := range lines {
		w := charmansi.StringWidth(l)
		if widest < w {
			widest = w
		}
	}

	return lines, widest
}
