//go:build windows

package gui

import (
	"reflect"
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// CenteredEntry is a widget.Entry with center-aligned text.
// Fyne v2 Entry does not expose text alignment, so we patch the
// internal RichText segments via reflection after each update.
type CenteredEntry struct {
	widget.Entry
}

// NewCenteredEntry creates an Entry whose text and placeholder are centered.
func NewCenteredEntry() *CenteredEntry {
	e := &CenteredEntry{}
	e.ExtendBaseWidget(e)
	origChanged := e.OnChanged
	e.OnChanged = func(s string) {
		e.centerSegments()
		if origChanged != nil {
			origChanged(s)
		}
	}
	return e
}

func (e *CenteredEntry) centerSegments() {
	centerRichText(&e.Entry, "text")
	centerRichText(&e.Entry, "placeholder")
}

// FocusGained overrides to apply centering when focused.
func (e *CenteredEntry) FocusGained() {
	e.Entry.FocusGained()
	e.centerSegments()
}

// FocusLost overrides to apply centering when unfocused.
func (e *CenteredEntry) FocusLost() {
	e.Entry.FocusLost()
	e.centerSegments()
}

// SetText sets the text and re-centers.
func (e *CenteredEntry) SetText(text string) {
	e.Entry.SetText(text)
	e.centerSegments()
}

// SetPlaceHolder sets the placeholder and re-centers.
func (e *CenteredEntry) SetPlaceHolder(text string) {
	e.Entry.SetPlaceHolder(text)
	e.centerSegments()
}

func centerRichText(entry *widget.Entry, fieldName string) {
	v := reflect.ValueOf(entry).Elem()
	field := v.FieldByName(fieldName)
	if !field.IsValid() {
		return
	}
	// Get pointer to the unexported RichText field.
	rt := (*widget.RichText)(unsafe.Pointer(field.UnsafeAddr()))
	for _, seg := range rt.Segments {
		if ts, ok := seg.(*widget.TextSegment); ok {
			ts.Style.Alignment = fyne.TextAlignCenter
		}
	}
	rt.Refresh()
}
