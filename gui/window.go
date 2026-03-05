//go:build windows

package gui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"port_forward/core/config"
	"port_forward/core/logger"
)

// MainWindow is the primary management window showing all forwarding rules.
type MainWindow struct {
	app    *App
	window fyne.Window
}

// NewMainWindow creates the main window (hidden by default).
func NewMainWindow(a *App) *MainWindow {
	w := a.fyneApp.NewWindow("Port Forward")
	w.Resize(fyne.NewSize(800, 500))

	mw := &MainWindow{
		app:    a,
		window: w,
	}

	// Hide instead of quit on close.
	w.SetCloseIntercept(func() {
		w.Hide()
	})

	mw.Refresh()
	return mw
}

// Show makes the main window visible.
func (mw *MainWindow) Show() {
	mw.Refresh()
	mw.window.Show()
}

// Refresh rebuilds the window content from current configuration state.
func (mw *MainWindow) Refresh() {
	header := container.NewGridWithColumns(6,
		widget.NewLabelWithStyle("#", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Protocol", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Local", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Remote", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Status", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Actions", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
	)

	rows := []fyne.CanvasObject{header}

	cfg := mw.app.manager.GetConfig()
	for i, rule := range cfg.Rules {
		idx := i // capture for closures

		status := "Disabled"
		if rule.Enabled {
			status = "Enabled"
		}

		toggleLabel := "Enable"
		if rule.Enabled {
			toggleLabel = "Disable"
		}

		toggleBtn := widget.NewButton(toggleLabel, func() {
			if cfg.Rules[idx].Enabled {
				_ = mw.app.manager.DisableRule(idx)
			} else {
				_ = mw.app.manager.EnableRule(idx)
			}
			mw.app.SaveConfig()
			mw.Refresh()
		})

		deleteBtn := widget.NewButton("Delete", func() {
			_ = mw.app.manager.RemoveRule(idx)
			mw.app.SaveConfig()
			mw.Refresh()
		})

		logBtn := widget.NewButton("Log", func() {
			ruleID := fmt.Sprintf("%s/%s->%s", rule.Protocol, rule.Local, rule.Remote)
			rl := logger.GetLogger(ruleID)
			ShowLogView(mw.app, fmt.Sprintf("Log: %s", ruleID), rl.Entries)
		})

		actions := container.NewHBox(toggleBtn, deleteBtn, logBtn)

		row := container.NewGridWithColumns(6,
			widget.NewLabel(fmt.Sprintf("%d", idx+1)),
			widget.NewLabel(rule.Protocol),
			widget.NewLabel(rule.Local),
			widget.NewLabel(rule.Remote),
			widget.NewLabel(status),
			actions,
		)
		rows = append(rows, row)
	}

	// Add-rule row.
	protoSelect := widget.NewSelect([]string{"tcp", "udp"}, nil)
	protoSelect.SetSelected("tcp")
	localEntry := widget.NewEntry()
	localEntry.SetPlaceHolder("127.0.0.1:1234")
	remoteEntry := widget.NewEntry()
	remoteEntry.SetPlaceHolder("0.0.0.0:5678")

	addBtn := widget.NewButton("Add", func() {
		proto := protoSelect.Selected
		if proto == "" {
			proto = "tcp"
		}
		local := localEntry.Text
		remote := remoteEntry.Text
		if local == "" || remote == "" {
			return
		}
		rule := config.Rule{
			Protocol: proto,
			Local:    local,
			Remote:   remote,
			Enabled:  true,
		}
		_ = mw.app.manager.AddRule(rule)
		mw.app.SaveConfig()
		mw.Refresh()
	})

	addRow := container.NewGridWithColumns(6,
		widget.NewLabel("+"),
		protoSelect,
		localEntry,
		remoteEntry,
		widget.NewLabel(""),
		addBtn,
	)
	rows = append(rows, addRow)

	content := container.NewVBox(rows...)
	scrollable := container.NewVScroll(content)
	mw.window.SetContent(scrollable)
}
