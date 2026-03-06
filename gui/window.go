//go:build windows

package gui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"port_forward/core/config"
	"port_forward/core/logger"
	"port_forward/gui/icons"
)

// Fixed column widths.
var (
	colNumW   = float32(30)
	colProtoW = float32(120)
	colAddrW  = float32(180)
	colChkW   = float32(40)
	rowH      = float32(36)
)

func fixedCell(w float32, obj fyne.CanvasObject) *fyne.Container {
	return container.NewGridWrap(fyne.NewSize(w, rowH), obj)
}

// MainWindow is the primary management window showing all forwarding rules.
type MainWindow struct {
	app    *App
	window fyne.Window
}

// NewMainWindow creates the main window (hidden by default).
func NewMainWindow(a *App) *MainWindow {
	w := a.fyneApp.NewWindow("\u7aef\u53e3\u8f6c\u53d1\u670d\u52a1")
	w.Resize(fyne.NewSize(780, 500))
	w.SetFixedSize(true)
	w.SetIcon(fyne.NewStaticResource("appicon.png", icons.AppIcon))

	mw := &MainWindow{
		app:    a,
		window: w,
	}

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
	mw.window.RequestFocus()
}

// makeRow builds a row: #, protocol, local, remote, toggle, actions
func makeRow(num, proto, local, remote, toggle, actions fyne.CanvasObject) *fyne.Container {
	return container.NewHBox(
		fixedCell(colNumW, num),
		fixedCell(colProtoW, proto),
		fixedCell(colAddrW, local),
		fixedCell(colAddrW, remote),
		fixedCell(colChkW, toggle),
		actions,
	)
}

// Refresh rebuilds the window content from current configuration state.
func (mw *MainWindow) Refresh() {
	cfg := mw.app.manager.GetConfig()

	header := makeRow(
		widget.NewLabelWithStyle("#", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("\u534f\u8bae", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("\u672c\u5730\u5730\u5740", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("\u8fdc\u7a0b\u5730\u5740", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("\u64cd\u4f5c", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
	)

	rows := container.NewVBox(header, widget.NewSeparator())

	for i, rule := range cfg.Rules {
		idx := i
		r := rule

		toggle := widget.NewCheck("", func(checked bool) {
			if checked {
				_ = mw.app.manager.EnableRule(idx)
			} else {
				_ = mw.app.manager.DisableRule(idx)
			}
			mw.app.SaveConfig()
		})
		toggle.Checked = r.Enabled

		deleteBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
			_ = mw.app.manager.RemoveRule(idx)
			mw.app.SaveConfig()
			mw.Refresh()
		})

		logBtn := widget.NewButtonWithIcon("", theme.DocumentIcon(), func() {
			ruleID := fmt.Sprintf("%s/%s->%s", r.Protocol, r.Local, r.Remote)
			rl := logger.GetLogger(ruleID)
			ShowLogView(mw.app, fmt.Sprintf("\u65e5\u5fd7: %s", ruleID), rl.Entries)
		})

		actions := container.NewHBox(deleteBtn, logBtn)

		row := makeRow(
			widget.NewLabel(fmt.Sprintf("%d", idx+1)),
			widget.NewLabel(r.Protocol),
			widget.NewLabel(r.Local),
			widget.NewLabel(r.Remote),
			toggle,
			actions,
		)
		rows.Add(row)
	}

	// Add-rule row
	rows.Add(widget.NewSeparator())

	protoSelect := widget.NewSelect([]string{"tcp", "udp"}, nil)
	protoSelect.SetSelected("tcp")
	localEntry := widget.NewEntry()
	localEntry.SetPlaceHolder("0.0.0.0:5678")
	remoteEntry := widget.NewEntry()
	remoteEntry.SetPlaceHolder("127.0.0.1:1234")

	doAdd := func() {
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
			Enabled:  true, // new rules start enabled
		}
		_ = mw.app.manager.AddRule(rule)
		mw.app.SaveConfig()
		localEntry.SetText("")
		remoteEntry.SetText("")
		mw.Refresh()
	}

	addBtn := widget.NewButtonWithIcon("", theme.ContentAddIcon(), doAdd)

	addRow := makeRow(
		addBtn,
		protoSelect,
		localEntry,
		remoteEntry,
		widget.NewLabel(""),
		widget.NewLabel(""),
	)
	rows.Add(addRow)

	scrollable := container.NewVScroll(rows)
	mw.window.SetContent(scrollable)
}
