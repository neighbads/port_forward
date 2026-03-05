package main

import (
	"os"
	"port_forward/cli"
)

func main() {
	args := []string{}
	for _, a := range os.Args[1:] {
		if a == "-noui" {
			continue
		}
		args = append(args, a)
	}
	// For now, always CLI mode. GUI routing added later.
	cli.Run(args)
}
