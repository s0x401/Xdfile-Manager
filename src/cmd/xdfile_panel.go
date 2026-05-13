package cmd

import "time"

type xdfileEntry struct {
	Name      string
	Path      string
	IsDir     bool
	IsParent  bool
	Size      int64
	Modified  time.Time
	GitMarker string
	sortName  string
	sortExt   string
}

type xdfilePanel struct {
	Label       string
	Cwd         string
	Entries     []xdfileEntry
	Cursor      int
	Scroll      int
	MarkedPaths map[string]struct{}
	RangeAnchor int
	Git         xdfileGitPanelInfo
}

func (p *xdfilePanel) selected() (xdfileEntry, bool) {
	if len(p.Entries) == 0 || p.Cursor < 0 || p.Cursor >= len(p.Entries) {
		return xdfileEntry{}, false
	}
	return p.Entries[p.Cursor], true
}

func (p *xdfilePanel) clearMarked() {
	p.MarkedPaths = nil
	p.RangeAnchor = -1
}

func (p *xdfilePanel) cloneMarkedPaths() map[string]struct{} {
	if len(p.MarkedPaths) == 0 {
		return nil
	}
	clone := make(map[string]struct{}, len(p.MarkedPaths))
	for path := range p.MarkedPaths {
		clone[path] = struct{}{}
	}
	return clone
}

func (p *xdfilePanel) resetRangeAnchor() {
	p.RangeAnchor = -1
}

func (p *xdfilePanel) markedCount() int {
	return len(p.MarkedPaths)
}

func (p *xdfilePanel) isMarked(entry xdfileEntry) bool {
	if len(p.MarkedPaths) == 0 || entry.IsParent {
		return false
	}
	_, ok := p.MarkedPaths[entry.Path]
	return ok
}

func (p *xdfilePanel) markedEntries() []xdfileEntry {
	if len(p.MarkedPaths) == 0 {
		return nil
	}
	entries := make([]xdfileEntry, 0, len(p.MarkedPaths))
	for _, entry := range p.Entries {
		if entry.IsParent {
			continue
		}
		if _, ok := p.MarkedPaths[entry.Path]; ok {
			entries = append(entries, entry)
		}
	}
	return entries
}

func (p *xdfilePanel) toggleMarkedAt(index int) bool {
	if len(p.Entries) == 0 || index < 0 || index >= len(p.Entries) {
		p.RangeAnchor = -1
		return false
	}

	entry := p.Entries[index]
	if entry.IsParent {
		p.RangeAnchor = -1
		return false
	}

	if p.MarkedPaths == nil {
		p.MarkedPaths = make(map[string]struct{})
	}
	if _, ok := p.MarkedPaths[entry.Path]; ok {
		delete(p.MarkedPaths, entry.Path)
		if len(p.MarkedPaths) == 0 {
			p.MarkedPaths = nil
		}
		p.RangeAnchor = -1
		return false
	}

	p.MarkedPaths[entry.Path] = struct{}{}
	p.RangeAnchor = -1
	return true
}

func (p *xdfilePanel) syncMarkedEntries() {
	if len(p.MarkedPaths) == 0 {
		p.RangeAnchor = -1
		return
	}

	valid := make(map[string]struct{}, len(p.MarkedPaths))
	for _, entry := range p.Entries {
		if entry.IsParent {
			continue
		}
		if _, ok := p.MarkedPaths[entry.Path]; ok {
			valid[entry.Path] = struct{}{}
		}
	}
	if len(valid) == 0 {
		p.MarkedPaths = nil
		p.RangeAnchor = -1
		return
	}
	p.MarkedPaths = valid
	if p.RangeAnchor < 0 || p.RangeAnchor >= len(p.Entries) {
		p.RangeAnchor = -1
	}
}

func (p *xdfilePanel) selectRange(anchor int, target int, rows int) int {
	return p.selectRangeWithBase(anchor, target, rows, nil)
}

func (p *xdfilePanel) selectRangeWithBase(anchor int, target int, rows int, base map[string]struct{}) int {
	if len(p.Entries) == 0 {
		p.clearMarked()
		return 0
	}

	anchor = max(0, min(anchor, len(p.Entries)-1))
	target = max(0, min(target, len(p.Entries)-1))
	p.RangeAnchor = anchor
	p.Cursor = target
	p.ensureVisible(rows)

	start := min(anchor, target)
	end := max(anchor, target)
	marked := make(map[string]struct{}, len(base)+end-start+1)
	for path := range base {
		marked[path] = struct{}{}
	}
	count := 0
	for i := start; i <= end; i++ {
		entry := p.Entries[i]
		if entry.IsParent {
			continue
		}
		marked[entry.Path] = struct{}{}
		count++
	}
	if len(marked) == 0 {
		p.MarkedPaths = nil
	} else {
		p.MarkedPaths = marked
	}
	return count
}

func (p *xdfilePanel) rangeSelectionAnchor() int {
	if p.RangeAnchor >= 0 && p.RangeAnchor < len(p.Entries) {
		return p.RangeAnchor
	}
	return p.Cursor
}

func (p *xdfilePanel) firstSelectableIndex() int {
	for i, entry := range p.Entries {
		if !entry.IsParent {
			return i
		}
	}
	return 0
}

func (p *xdfilePanel) lastSelectableIndex() int {
	for i := len(p.Entries) - 1; i >= 0; i-- {
		if !p.Entries[i].IsParent {
			return i
		}
	}
	return max(0, len(p.Entries)-1)
}

func (p *xdfilePanel) visibleRows(totalHeight int) int {
	return max(1, totalHeight-4)
}

func (p *xdfilePanel) clampCursor() {
	if len(p.Entries) == 0 {
		p.Cursor = 0
		p.Scroll = 0
		return
	}
	p.Cursor = max(0, min(p.Cursor, len(p.Entries)-1))
}

func (p *xdfilePanel) ensureVisible(rows int) {
	p.clampCursor()
	if p.Cursor < p.Scroll {
		p.Scroll = p.Cursor
	}
	if p.Cursor >= p.Scroll+rows {
		p.Scroll = p.Cursor - rows + 1
	}
	maxScroll := max(0, len(p.Entries)-rows)
	p.Scroll = max(0, min(p.Scroll, maxScroll))
}

func (p *xdfilePanel) move(delta int, rows int) {
	if len(p.Entries) == 0 {
		return
	}
	p.Cursor += delta
	if p.Cursor < 0 {
		p.Cursor = 0
	}
	if p.Cursor >= len(p.Entries) {
		p.Cursor = len(p.Entries) - 1
	}
	p.ensureVisible(rows)
}

func (p *xdfilePanel) scroll(delta int, rows int) {
	if len(p.Entries) == 0 {
		p.Scroll = 0
		return
	}
	maxScroll := max(0, len(p.Entries)-rows)
	p.Scroll = max(0, min(p.Scroll+delta, maxScroll))
}

func (p *xdfilePanel) setCursor(index int, rows int) {
	p.Cursor = index
	p.ensureVisible(rows)
}

func (p *xdfilePanel) focusPath(path string, rows int) bool {
	for i, entry := range p.Entries {
		if xdfilePathsEqual(entry.Path, path) {
			p.setCursor(i, rows)
			return true
		}
	}
	return false
}
