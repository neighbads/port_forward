//go:build windows

package gui

import (
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/systray"
	"port_forward/core/logger"
	"port_forward/gui/icons"
)

// Tray manages the system tray icon and menu.
type Tray struct {
	app     *App
	deskApp desktop.App

	clickMu    sync.Mutex
	clickCount int
	clickTimer *time.Timer
}

// NewTray creates a new system tray controller for the given App.
func NewTray(a *App) *Tray {
	return &Tray{
		app:     a,
		deskApp: a.fyneApp.(desktop.App),
	}
}

// Setup initialises the system tray icon and menu items.
func (t *Tray) Setup() {
	t.SetStopped()

	toggleItem := fyne.NewMenuItem("\u5207\u6362\u670d\u52a1", func() {
		t.app.ToggleService()
	})

	showItem := fyne.NewMenuItem("\u6253\u5f00\u7a97\u53e3", func() {
		t.app.mainWindow.Show()
	})

	viewLogItem := fyne.NewMenuItem("\u67e5\u770b\u65e5\u5fd7", func() {
		ShowLogView(t.app, "\u5168\u5c40\u65e5\u5fd7", logger.AllEntries)
	})

	// Log level: only two options — info and debug.
	infoItem := fyne.NewMenuItem("\u65e5\u5fd7\u7ea7\u522b: \u4fe1\u606f", func() {
		logger.SetLevel(logger.Info)
		t.app.cfg.LogLevel = "info"
		t.app.SaveConfig()
	})
	debugItem := fyne.NewMenuItem("\u65e5\u5fd7\u7ea7\u522b: \u8c03\u8bd5", func() {
		logger.SetLevel(logger.Debug)
		t.app.cfg.LogLevel = "debug"
		t.app.SaveConfig()
	})

	quitItem := fyne.NewMenuItem("\u9000\u51fa", func() {
		t.app.manager.StopAll()
		t.app.fyneApp.Quit()
	})

	menu := fyne.NewMenu("PortForward",
		toggleItem,
		showItem,
		fyne.NewMenuItemSeparator(),
		viewLogItem,
		fyne.NewMenuItemSeparator(),
		infoItem,
		debugItem,
		fyne.NewMenuItemSeparator(),
		quitItem,
	)

	t.deskApp.SetSystemTrayMenu(menu)

	// Override left-click: single click = toggle service, double click = show window.
	systray.SetOnTapped(func() {
		t.handleClick()
	})
}

const doubleClickInterval = 300 * time.Millisecond

func (t *Tray) handleClick() {
	t.clickMu.Lock()
	defer t.clickMu.Unlock()

	t.clickCount++
	if t.clickCount == 1 {
		t.clickTimer = time.AfterFunc(doubleClickInterval, func() {
			t.clickMu.Lock()
			count := t.clickCount
			t.clickCount = 0
			t.clickMu.Unlock()

			if count == 1 {
				t.app.ToggleService()
			}
		})
	} else if t.clickCount >= 2 {
		t.clickTimer.Stop()
		t.clickCount = 0
		t.app.mainWindow.Show()
	}
}

// SetRunning updates the tray icon to indicate the service is active.
func (t *Tray) SetRunning() {
	res := fyne.NewStaticResource("green.png", icons.GreenIcon)
	t.deskApp.SetSystemTrayIcon(res)
}

// SetStopped updates the tray icon to indicate the service is inactive.
func (t *Tray) SetStopped() {
	res := fyne.NewStaticResource("red.png", icons.RedIcon)
	t.deskApp.SetSystemTrayIcon(res)
}
