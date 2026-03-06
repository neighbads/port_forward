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
)

// MainWindow is the primary management window showing all forwarding rules.
type MainWindow struct {
	app    *App
	window fyne.Window
}

// NewMainWindow creates the main window (hidden by default).
func NewMainWindow(a *App) *MainWindow {
	w := a.fyneApp.NewWindow("Port Forward")
	w.Resize(fyne.NewSize(820, 500))

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
	mw.window.RequestFocus()
}

// Refresh rebuilds the window content from current configuration state.
func (mw *MainWindow) Refresh() {
	cfg := mw.app.manager.GetConfig()

	// Build table using widget.Table for fixed layout
	list := widget.NewTable(
		// size: rows = rules + 1 (header) + 1 (add row), cols = 6
		func() (int, int) {
			return len(cfg.Rules) + 2, 6
		},
		// create template
		func() fyne.CanvasObject {
			return container.NewStack(
				widget.NewLabel("template text long"),
				widget.NewButton("Action", func() {}),
				widget.NewSelect([]string{"tcp", "udp"}, nil),
				widget.NewEntry(),
			)
		},
		// update cell
		func(id widget.TableCellID, o fyne.CanvasObject) {
			stack := o.(*fyne.Container)
			label := stack.Objects[0].(*widget.Label)
			btn := stack.Objects[1].(*widget.Button)
			sel := stack.Objects[2].(*widget.Select)
			entry := stack.Objects[3].(*widget.Entry)

			// Hide all by default
			label.Hide()
			btn.Hide()
			sel.Hide()
			entry.Hide()

			row := id.Row
			col := id.Col

			if row == 0 {
				// Header row
				label.Show()
				label.TextStyle = fyne.TextStyle{Bold: true}
				switch col {
				case 0:
					label.SetText("#")
				case 1:
					label.SetText("Protocol")
				case 2:
					label.SetText("Local")
				case 3:
					label.SetText("Remote")
				case 4:
					label.SetText("Status")
				case 5:
					label.SetText("Actions")
				}
				return
			}

			ruleIdx := row - 1

			if ruleIdx < len(cfg.Rules) {
				// Data row
				rule := cfg.Rules[ruleIdx]
				idx := ruleIdx

				switch col {
				case 0:
					label.Show()
					label.TextStyle = fyne.TextStyle{}
					label.SetText(fmt.Sprintf("%d", idx+1))
				case 1:
					label.Show()
					label.TextStyle = fyne.TextStyle{}
					label.SetText(rule.Protocol)
				case 2:
					label.Show()
					label.TextStyle = fyne.TextStyle{}
					label.SetText(rule.Local)
				case 3:
					label.Show()
					label.TextStyle = fyne.TextStyle{}
					label.SetText(rule.Remote)
				case 4:
					label.Show()
					label.TextStyle = fyne.TextStyle{}
					if rule.Enabled {
						label.SetText("Enabled")
					} else {
						label.SetText("Disabled")
					}
				case 5:
					// Action buttons - use a label-as-container approach
					// We'll use the button for the primary action
					btn.Show()
					if rule.Enabled {
						btn.SetText("Disable")
					} else {
						btn.SetText("Enable")
					}
					btn.OnTapped = func() {
						if cfg.Rules[idx].Enabled {
							_ = mw.app.manager.DisableRule(idx)
						} else {
							_ = mw.app.manager.EnableRule(idx)
						}
						mw.app.SaveConfig()
						mw.Refresh()
					}
				}
			}
		},
	)

	// Set column widths
	list.SetColumnWidth(0, 40)   // #
	list.SetColumnWidth(1, 80)   // Protocol
	list.SetColumnWidth(2, 200)  // Local
	list.SetColumnWidth(3, 200)  // Remote
	list.SetColumnWidth(4, 80)   // Status
	list.SetColumnWidth(5, 100)  // Actions

	// Build a simpler approach: VBox with rows instead of Table (Table has cell reuse issues with buttons)
	// Let's use the straightforward VBox approach but with fixed-size window
	header := container.NewGridWithColumns(7,
		widget.NewLabelWithStyle("#", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Protocol", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Local", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Remote", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Status", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabel(""), // spacer for actions
		widget.NewLabel(""), // spacer for actions
	)

	rows := container.NewVBox(header, widget.NewSeparator())

	for i, rule := range cfg.Rules {
		idx := i
		r := rule

		toggleText := "Enable"
		if r.Enabled {
			toggleText = "Disable"
		}

		toggleBtn := widget.NewButton(toggleText, func() {
			if cfg.Rules[idx].Enabled {
				_ = mw.app.manager.DisableRule(idx)
			} else {
				_ = mw.app.manager.EnableRule(idx)
			}
			mw.app.SaveConfig()
			mw.Refresh()
		})

		deleteBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
			_ = mw.app.manager.RemoveRule(idx)
			mw.app.SaveConfig()
			mw.Refresh()
		})

		logBtn := widget.NewButtonWithIcon("Log", theme.DocumentIcon(), func() {
			ruleID := fmt.Sprintf("%s/%s->%s", r.Protocol, r.Local, r.Remote)
			rl := logger.GetLogger(ruleID)
			ShowLogView(mw.app, fmt.Sprintf("Log: %s", ruleID), rl.Entries)
		})

		actions := container.NewHBox(toggleBtn, deleteBtn, logBtn)

		status := "Disabled"
		if r.Enabled {
			status = "Enabled"
		}

		row := container.NewGridWithColumns(7,
			widget.NewLabel(fmt.Sprintf("%d", idx+1)),
			widget.NewLabel(r.Protocol),
			widget.NewLabel(r.Local),
			widget.NewLabel(r.Remote),
			widget.NewLabel(status),
			actions,
			widget.NewLabel(""),
		)
		rows.Add(row)
	}

	// Add-rule row
	rows.Add(widget.NewSeparator())

	protoSelect := widget.NewSelect([]string{"tcp", "udp"}, nil)
	protoSelect.SetSelected("tcp")
	localEntry := widget.NewEntry()
	localEntry.SetPlaceHolder("127.0.0.1:1234")
	remoteEntry := widget.NewEntry()
	remoteEntry.SetPlaceHolder("0.0.0.0:5678")

	addBtn := widget.NewButtonWithIcon("Add", theme.ContentAddIcon(), func() {
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
			Enabled:  false, // new rules start disabled
		}
		_ = mw.app.manager.AddRule(rule)
		mw.app.SaveConfig()
		localEntry.SetText("")
		remoteEntry.SetText("")
		mw.Refresh()
	})

	addRow := container.NewGridWithColumns(7,
		widget.NewLabel("+"),
		protoSelect,
		localEntry,
		remoteEntry,
		widget.NewLabel(""),
		addBtn,
		widget.NewLabel(""),
	)
	rows.Add(addRow)

	scrollable := container.NewVScroll(rows)
	mw.window.SetContent(scrollable)
}
