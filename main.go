package main

import (
	"os"
	"port_forward/cli"
	"port_forward/gui"
)

func main() {
	configPath := "config.yaml"
	noUI := false
	args := []string{}

	for i := 1; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "-noui":
			noUI = true
		case "-c":
			if i+1 < len(os.Args) {
				i++
				configPath = os.Args[i]
			}
		default:
			args = append(args, os.Args[i])
		}
	}

	if !noUI && gui.IsGUIAvailable() && len(args) == 0 {
		gui.RunGUI(configPath)
	} else {
		cli.Run(args)
	}
}
