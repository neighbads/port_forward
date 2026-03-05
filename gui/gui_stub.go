//go:build !windows

package gui

// RunGUI launches the graphical user interface. It is only supported on Windows.
func RunGUI(configPath string) {
	panic("GUI is only supported on Windows")
}

// IsGUIAvailable reports whether the GUI can run on this platform.
func IsGUIAvailable() bool {
	return false
}
