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

	toggleItem := fyne.NewMenuItem("Toggle Service", func() {
		t.app.ToggleService()
	})

	showItem := fyne.NewMenuItem("Show Window", func() {
		t.app.mainWindow.Show()
	})

	viewLogItem := fyne.NewMenuItem("View Log", func() {
		ShowLogView(t.app, "Global Log", logger.AllEntries)
	})

	// Log level menu items.
	debugItem := fyne.NewMenuItem("Log: Debug", func() {
		logger.SetLevel(logger.Debug)
		t.app.cfg.LogLevel = "debug"
		t.app.SaveConfig()
	})
	infoItem := fyne.NewMenuItem("Log: Info", func() {
		logger.SetLevel(logger.Info)
		t.app.cfg.LogLevel = "info"
		t.app.SaveConfig()
	})
	warnItem := fyne.NewMenuItem("Log: Warn", func() {
		logger.SetLevel(logger.Warn)
		t.app.cfg.LogLevel = "warn"
		t.app.SaveConfig()
	})
	errorItem := fyne.NewMenuItem("Log: Error", func() {
		logger.SetLevel(logger.Error)
		t.app.cfg.LogLevel = "error"
		t.app.SaveConfig()
	})

	quitItem := fyne.NewMenuItem("Quit", func() {
		t.app.manager.StopAll()
		t.app.fyneApp.Quit()
	})

	menu := fyne.NewMenu("PortForward",
		toggleItem,
		showItem,
		fyne.NewMenuItemSeparator(),
		viewLogItem,
		fyne.NewMenuItemSeparator(),
		debugItem,
		infoItem,
		warnItem,
		errorItem,
		fyne.NewMenuItemSeparator(),
		quitItem,
	)

	t.deskApp.SetSystemTrayMenu(menu)

	// Override left-click: single click = toggle service, double click = show window.
	// Right-click still shows menu (default behavior when tappedRight is nil).
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
				// Single click: toggle service
				t.app.ToggleService()
			}
		})
	} else if t.clickCount >= 2 {
		// Double click: show window
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
