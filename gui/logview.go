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

// ShowLogView opens a new window displaying log entries with black text.
func ShowLogView(a *App, title string, entries func() []logger.Entry) {
	w := a.fyneApp.NewWindow(title)
	w.Resize(fyne.NewSize(750, 450))

	logLabel := widget.NewLabel("")
	logLabel.Wrapping = fyne.TextWrapWord

	refreshLog := func() {
		ents := entries()
		if len(ents) == 0 {
			logLabel.SetText("(\u6682\u65e0\u65e5\u5fd7)")
			return
		}
		var b strings.Builder
		for _, e := range ents {
			b.WriteString(e.String())
			b.WriteByte('\n')
		}
		logLabel.SetText(b.String())
	}

	refreshLog()

	refreshBtn := widget.NewButton("\u5237\u65b0", func() {
		refreshLog()
	})

	toolbar := container.NewHBox(refreshBtn)

	scrollable := container.NewVScroll(logLabel)

	content := container.NewBorder(
		toolbar,    // top
		nil,        // bottom
		nil,        // left
		nil,        // right
		scrollable, // center
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
