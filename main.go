package main

import (
	"flag"
	"fmt"
	"os"
	"port_forward/cli"
	"port_forward/gui"
)

var Version = "dev"

func main() {
	var (
		configPath string
		noUI       bool
		version    bool
	)

	flag.StringVar(&configPath, "c", "config.yaml", "config file path")
	flag.BoolVar(&noUI, "noui", false, "disable GUI, use CLI mode")
	flag.BoolVar(&version, "version", false, "show version and exit")
	flag.Parse()

	if version {
		fmt.Println("port_forward", Version)
		return
	}

	args := flag.Args()

	if !noUI && gui.IsGUIAvailable() && len(args) == 0 {
		gui.RunGUI(configPath)
	} else {
		if len(args) == 0 {
			args = []string{"help"}
		}
		cli.Run(args, configPath)
	}
}

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: port_forward [flags] [command] [args]

Flags:
`)
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Commands:
  start                     Start forwarding all enabled rules
  add [options]             Add a new forwarding rule
  remove -id <index>        Remove a rule by index (0-based)
  list                      List all forwarding rules
  set-loglevel <level>      Set log level (debug, info, warn, error)
  help                      Show this help message
`)
	}
}
