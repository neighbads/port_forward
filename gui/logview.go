//go:build windows

package gui

import (
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"port_forward/core/logger"
)

// ShowLogView opens a new window displaying log entries. The entries function
// is called to retrieve the current set of entries each time the view refreshes.
func ShowLogView(a *App, title string, entries func() []logger.Entry) {
	w := a.fyneApp.NewWindow(title)
	w.Resize(fyne.NewSize(750, 450))

	logText := widget.NewMultiLineEntry()
	logText.Wrapping = fyne.TextWrapWord
	logText.Disable()

	refreshLog := func() {
		ents := entries()
		if len(ents) == 0 {
			logText.SetText("(no log entries)")
			return
		}
		var b strings.Builder
		for _, e := range ents {
			b.WriteString(e.String())
			b.WriteByte('\n')
		}
		logText.SetText(b.String())
	}

	refreshLog()

	refreshBtn := widget.NewButton("Refresh", func() {
		refreshLog()
	})

	clearBtn := widget.NewButton("Clear", func() {
		logText.SetText("")
	})

	toolbar := container.NewHBox(refreshBtn, clearBtn)

	content := container.NewBorder(
		toolbar, // top
		nil,     // bottom
		nil,     // left
		nil,     // right
		logText, // center
	)
	w.SetContent(content)
	w.Show()
	w.RequestFocus()

	// Auto-refresh every 2 seconds.
	ticker := time.NewTicker(2 * time.Second)
	go func() {
		for range ticker.C {
			refreshLog()
		}
	}()

	w.SetOnClosed(func() {
		ticker.Stop()
	})
}
