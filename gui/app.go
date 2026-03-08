//go:build windows

package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/systray"
	"port_forward/core/config"
	"port_forward/core/forwarder"
	"port_forward/core/logger"
	"port_forward/gui/icons"
)

// App holds the top-level GUI application state.
type App struct {
	fyneApp    fyne.App
	mainWindow *MainWindow
	tray       *Tray
	manager    *forwarder.Manager
	cfg        *config.Config
	configPath string
	running    bool
}

// RunGUI launches the graphical user interface.
func RunGUI(configPath string) {
	cfg, err := config.Load(configPath)
	if err != nil {
		cfg = &config.Config{
			Defaults: config.Defaults{
				TCPConnectTimeout: config.DefaultTCPConnectTimeout,
				TCPIdleTimeout:    config.DefaultTCPIdleTimeout,
				UDPSessionTimeout: config.DefaultUDPSessionTimeout,
			},
			LogLevel: "info",
		}
	}

	if cfg.LogLevel != "" {
		lvl, parseErr := logger.ParseLevel(cfg.LogLevel)
		if parseErr == nil {
			logger.SetLevel(lvl)
		}
	}

	mgr := forwarder.NewManager(cfg)

	a := &App{
		fyneApp:    app.New(),
		manager:    mgr,
		cfg:        cfg,
		configPath: configPath,
	}

	// Set app icon.
	a.fyneApp.SetIcon(fyne.NewStaticResource("appicon.png", icons.AppIcon))

	a.mainWindow = NewMainWindow(a)
	a.tray = NewTray(a)
	a.tray.Setup()

	// Auto-start service on GUI launch.
	_ = a.manager.StartAll()
	a.running = true
	a.tray.SetRunning()

	a.fyneApp.Lifecycle().SetOnExitedForeground(func() {})
	a.fyneApp.Lifecycle().SetOnStopped(func() {
		a.manager.StopAll()
		systray.Quit()
	})

	a.fyneApp.Run()
}

// IsGUIAvailable reports whether the GUI can run on this platform.
func IsGUIAvailable() bool {
	return true
}

// ToggleService starts or stops all forwarding rules.
func (a *App) ToggleService() {
	if a.running {
		a.manager.StopAll()
		a.running = false
		a.tray.SetStopped()
	} else {
		_ = a.manager.StartAll()
		a.running = true
		a.tray.SetRunning()
	}
	if a.mainWindow != nil {
		a.mainWindow.Refresh()
	}
}

// SaveConfig persists the current configuration to disk.
func (a *App) SaveConfig() {
	_ = config.Save(a.cfg, a.configPath)
}
