package cmd

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
)

type xdfileModalKind int

const (
	xdfileModalNone xdfileModalKind = iota
	xdfileModalText
	xdfileModalInput
	xdfileModalConfirm
	xdfileModalChoice
	xdfileModalForm
)

type xdfileModalChoiceItem struct {
	Action      xdfileAction
	Label       string
	Description string
}

type xdfileModalField struct {
	Label     string
	Input     textinput.Model
	TextArea  textarea.Model
	Multiline bool
}

func (f xdfileModalField) Value() string {
	if f.Multiline {
		return f.TextArea.Value()
	}
	return f.Input.Value()
}

func (f *xdfileModalField) syncMirrorInput() {
	if f == nil || !f.Multiline {
		return
	}
	f.Input.SetValue(f.TextArea.Value())
}

func (f xdfileModalField) Focused() bool {
	if f.Multiline {
		return f.TextArea.Focused()
	}
	return f.Input.Focused()
}

func (f *xdfileModalField) Focus() {
	if f == nil {
		return
	}
	if f.Multiline {
		_ = f.TextArea.Focus()
		return
	}
	_ = f.Input.Focus()
}

func (f *xdfileModalField) Blur() {
	if f == nil {
		return
	}
	if f.Multiline {
		f.TextArea.Blur()
		return
	}
	f.Input.Blur()
}

func (f xdfileModalField) displayHeight() int {
	if f.Multiline {
		return 5
	}
	return 1
}

func (f *xdfileModalField) SetWidth(width int) {
	if f == nil {
		return
	}
	width = max(10, width)
	if f.Multiline {
		f.TextArea.SetWidth(width)
		f.TextArea.SetHeight(f.displayHeight())
		return
	}
	f.Input.Width = width
}

func (f xdfileModalField) ViewLines() []string {
	if f.Multiline {
		return strings.Split(f.TextArea.View(), "\n")
	}
	return []string{f.Input.View()}
}

func (f *xdfileModalField) InsertNewline() {
	if f == nil || !f.Multiline {
		return
	}
	f.TextArea.InsertRune('\n')
	f.syncMirrorInput()
}

type xdfileModal struct {
	Kind          xdfileModalKind
	Title         string
	Description   string
	Action        xdfileAction
	Input         textinput.Model
	Viewport      viewport.Model
	SourcePath    string
	SourcePaths   []string
	TargetPath    string
	PanelIndex    int
	Text          string
	PreviewPath   string
	PreviewBinary bool
	PreviewVisual bool
	ChoiceItems   []xdfileModalChoiceItem
	ChoiceCursor  int
	FormFields    []xdfileModalField
	FormCursor    int
}

type xdfileQuickView struct {
	Open         bool
	Path         string
	Title        string
	Description  string
	Text         string
	Binary       bool
	Visual       bool
	Viewport     viewport.Model
	ContentW     int
	ContentH     int
	ViewportW    int
	ViewportText string
}
