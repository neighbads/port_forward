//go:build windows

package gui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

var (
	toggleOnColor  = color.NRGBA{50, 180, 50, 255}
	toggleOffColor = color.NRGBA{180, 180, 180, 255}
	knobColor      = color.NRGBA{255, 255, 255, 255}
)

const (
	toggleWidth  = float32(36)
	toggleHeight = float32(20)
	knobPadding  = float32(2)
)

// ToggleSwitch is an oval sliding on/off switch widget.
type ToggleSwitch struct {
	widget.BaseWidget
	On        bool
	OnChanged func(on bool)
	track     *canvas.Rectangle
	knob      *canvas.Circle
}

// NewToggleSwitch creates a new toggle switch.
func NewToggleSwitch(on bool, changed func(bool)) *ToggleSwitch {
	t := &ToggleSwitch{
		On:        on,
		OnChanged: changed,
	}
	t.track = canvas.NewRectangle(toggleOffColor)
	t.track.CornerRadius = toggleHeight / 2
	t.knob = canvas.NewCircle(knobColor)
	t.updateColors()
	t.ExtendBaseWidget(t)
	return t
}

func (t *ToggleSwitch) updateColors() {
	if t.On {
		t.track.FillColor = toggleOnColor
	} else {
		t.track.FillColor = toggleOffColor
	}
	t.track.Refresh()
}

// Tapped handles click on the toggle.
func (t *ToggleSwitch) Tapped(_ *fyne.PointEvent) {
	t.On = !t.On
	t.updateColors()
	t.Refresh()
	if t.OnChanged != nil {
		t.OnChanged(t.On)
	}
}

// Cursor returns the pointer cursor for hover.
func (t *ToggleSwitch) Cursor() desktop.Cursor {
	return desktop.PointerCursor
}

// CreateRenderer returns the toggle renderer.
func (t *ToggleSwitch) CreateRenderer() fyne.WidgetRenderer {
	return &toggleRenderer{toggle: t}
}

type toggleRenderer struct {
	toggle *ToggleSwitch
}

func (r *toggleRenderer) Layout(size fyne.Size) {
	r.toggle.track.Resize(fyne.NewSize(toggleWidth, toggleHeight))
	r.toggle.track.Move(fyne.NewPos(0, (size.Height-toggleHeight)/2))

	knobSize := toggleHeight - knobPadding*2
	r.toggle.knob.Resize(fyne.NewSize(knobSize, knobSize))

	var knobX float32
	if r.toggle.On {
		knobX = toggleWidth - knobPadding - knobSize
	} else {
		knobX = knobPadding
	}
	knobY := (size.Height-toggleHeight)/2 + knobPadding
	r.toggle.knob.Move(fyne.NewPos(knobX, knobY))
}

func (r *toggleRenderer) MinSize() fyne.Size {
	return fyne.NewSize(toggleWidth, toggleHeight)
}

func (r *toggleRenderer) Refresh() {
	r.toggle.updateColors()
	r.Layout(r.toggle.Size())
	canvas.Refresh(r.toggle)
}

func (r *toggleRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.toggle.track, r.toggle.knob}
}

func (r *toggleRenderer) Destroy() {}
