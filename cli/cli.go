// Package cli implements the command-line interface for port_forward.
package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"port_forward/core/config"
	"port_forward/core/forwarder"
	"port_forward/core/logger"
)

const defaultConfigPath = "config.yaml"

// Run is the main entry point for the CLI. It dispatches based on args[0].
func Run(args []string) {
	if len(args) == 0 {
		printUsage()
		return
	}

	switch args[0] {
	case "start":
		cmdStart(defaultConfigPath)
	case "add":
		cmdAdd(args[1:], defaultConfigPath)
	case "remove":
		cmdRemove(args[1:], defaultConfigPath)
	case "list":
		cmdList(defaultConfigPath)
	case "set-loglevel":
		cmdSetLogLevel(args[1:], defaultConfigPath)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", args[0])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Usage: port_forward <command> [options]

Commands:
  start                     Start forwarding all enabled rules (blocks until Ctrl+C)
  add [options]             Add a new forwarding rule
  remove -id <index>        Remove a rule by index (0-based)
  list                      List all forwarding rules
  set-loglevel <level>      Set log level (debug, info, warn, error)
  help                      Show this help message

Add options:
  -p <protocol>             Protocol: tcp or udp (default: tcp)
  -l <local_addr>           Local address (default: 127.0.0.1:1234)
  -r <remote_addr>          Remote address (default: 0.0.0.0:1234)
  --connect-timeout <dur>   TCP connect timeout (default: use global)
  --idle-timeout <dur>      TCP idle timeout (default: use global)
  --session-timeout <dur>   UDP session timeout (default: use global)`)
}

// loadOrCreateConfig loads config from the given path, or creates a default
// config file if it does not exist.
func loadOrCreateConfig(path string) *config.Config {
	cfg, err := config.Load(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Create a default config and save it.
			cfg = &config.Config{
				Defaults: config.Defaults{
					TCPConnectTimeout: config.DefaultTCPConnectTimeout,
					TCPIdleTimeout:    config.DefaultTCPIdleTimeout,
					UDPSessionTimeout: config.DefaultUDPSessionTimeout,
				},
				LogLevel: "info",
				Rules:    []config.Rule{},
			}
			if saveErr := config.Save(cfg, path); saveErr != nil {
				fmt.Fprintf(os.Stderr, "Error creating default config: %v\n", saveErr)
				os.Exit(1)
			}
			fmt.Printf("Created default config at %s\n", path)
			return cfg
		}
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}
	return cfg
}

func cmdStart(configPath string) {
	cfg := loadOrCreateConfig(configPath)

	// Apply log level from config.
	if cfg.LogLevel != "" {
		lvl, err := logger.ParseLevel(cfg.LogLevel)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: invalid log level %q in config, using info\n", cfg.LogLevel)
			lvl = logger.Info
		}
		logger.SetLevel(lvl)
	}

	mgr := forwarder.NewManager(cfg)

	fmt.Printf("Starting %d rule(s)...\n", len(cfg.Rules))
	if err := mgr.StartAll(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting rules: %v\n", err)
		os.Exit(1)
	}

	enabledCount := 0
	for _, r := range cfg.Rules {
		if r.Enabled {
			enabledCount++
		}
	}
	fmt.Printf("Running %d enabled rule(s). Press Ctrl+C to stop.\n", enabledCount)

	// Block until SIGINT or SIGTERM.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("\nShutting down...")
	mgr.StopAll()
	fmt.Println("Stopped.")
}

func cmdAdd(args []string, configPath string) {
	fs := flag.NewFlagSet("add", flag.ExitOnError)
	protocol := fs.String("p", "tcp", "Protocol (tcp or udp)")
	local := fs.String("l", "127.0.0.1:1234", "Local address")
	remote := fs.String("r", "0.0.0.0:1234", "Remote address")
	connectTimeout := fs.String("connect-timeout", "", "TCP connect timeout (e.g. 5s)")
	idleTimeout := fs.String("idle-timeout", "", "TCP idle timeout (e.g. 0s)")
	sessionTimeout := fs.String("session-timeout", "", "UDP session timeout (e.g. 120s)")

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	*protocol = strings.ToLower(*protocol)
	if *protocol != "tcp" && *protocol != "udp" {
		fmt.Fprintf(os.Stderr, "Error: protocol must be tcp or udp, got %q\n", *protocol)
		os.Exit(1)
	}

	rule := config.Rule{
		Protocol: *protocol,
		Local:    *local,
		Remote:   *remote,
		Enabled:  true,
	}

	if *connectTimeout != "" {
		d, err := time.ParseDuration(*connectTimeout)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: invalid connect-timeout %q: %v\n", *connectTimeout, err)
			os.Exit(1)
		}
		cd := config.Duration(d)
		rule.TCPConnectTimeout = &cd
	}

	if *idleTimeout != "" {
		d, err := time.ParseDuration(*idleTimeout)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: invalid idle-timeout %q: %v\n", *idleTimeout, err)
			os.Exit(1)
		}
		cd := config.Duration(d)
		rule.TCPIdleTimeout = &cd
	}

	if *sessionTimeout != "" {
		d, err := time.ParseDuration(*sessionTimeout)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: invalid session-timeout %q: %v\n", *sessionTimeout, err)
			os.Exit(1)
		}
		cd := config.Duration(d)
		rule.UDPSessionTimeout = &cd
	}

	cfg := loadOrCreateConfig(configPath)
	cfg.AddRule(rule)

	if err := config.Save(cfg, configPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Added rule #%d: %s %s -> %s\n", len(cfg.Rules)-1, rule.Protocol, rule.Local, rule.Remote)
}

func cmdRemove(args []string, configPath string) {
	fs := flag.NewFlagSet("remove", flag.ExitOnError)
	id := fs.Int("id", -1, "Rule index to remove (0-based)")

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	if *id < 0 {
		fmt.Fprintln(os.Stderr, "Error: -id flag is required (0-based rule index)")
		os.Exit(1)
	}

	cfg := loadOrCreateConfig(configPath)

	if err := cfg.RemoveRule(*id); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := config.Save(cfg, configPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Removed rule #%d\n", *id)
}

func cmdList(configPath string) {
	cfg := loadOrCreateConfig(configPath)

	if len(cfg.Rules) == 0 {
		fmt.Println("No rules configured.")
		return
	}

	fmt.Printf("%-4s %-6s %-25s %-25s %-8s\n", "#", "Proto", "Local", "Remote", "Enabled")
	fmt.Println(strings.Repeat("-", 70))
	for i, r := range cfg.Rules {
		enabled := "yes"
		if !r.Enabled {
			enabled = "no"
		}
		fmt.Printf("%-4d %-6s %-25s %-25s %-8s\n", i, r.Protocol, r.Local, r.Remote, enabled)
	}
}

func cmdSetLogLevel(args []string, configPath string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Error: log level required (debug, info, warn, error)")
		os.Exit(1)
	}

	levelStr := strings.ToLower(args[0])
	if _, err := logger.ParseLevel(levelStr); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	cfg := loadOrCreateConfig(configPath)
	cfg.LogLevel = levelStr

	if err := config.Save(cfg, configPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Log level set to %s\n", levelStr)
}
