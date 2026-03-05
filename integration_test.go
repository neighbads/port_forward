package main

import (
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"port_forward/core/config"
	"port_forward/core/forwarder"
	"port_forward/core/logger"
)

// startEchoServer starts a TCP echo server on a random port and returns its
// address. The server shuts down when the test completes.
func startEchoServer(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("echo server listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				io.Copy(c, c)
			}(conn)
		}
	}()

	return ln.Addr().String()
}

func TestIntegrationTCPForward(t *testing.T) {
	logger.ResetGlobal()
	logger.SetLevel(logger.Debug)

	echoAddr := startEchoServer(t)

	cfg := &config.Config{
		Defaults: config.Defaults{
			TCPConnectTimeout: 5 * time.Second,
			TCPIdleTimeout:    30 * time.Second,
			UDPSessionTimeout: 60 * time.Second,
		},
		Rules: []config.Rule{
			{
				Protocol: "tcp",
				Local:    "127.0.0.1:0",
				Remote:   echoAddr,
				Enabled:  true,
			},
		},
	}

	mgr := forwarder.NewManager(cfg)
	if err := mgr.StartAll(); err != nil {
		t.Fatalf("StartAll: %v", err)
	}
	t.Cleanup(func() { mgr.StopAll() })

	if got := mgr.RunningCount(); got != 1 {
		t.Fatalf("RunningCount = %d, want 1", got)
	}

	// Allow a moment for log entries to be written.
	time.Sleep(50 * time.Millisecond)

	entries := logger.AllEntries()
	if len(entries) == 0 {
		t.Fatal("expected logger to have entries from startup, got none")
	}

	found := false
	for _, e := range entries {
		if e.Level == logger.Debug {
			found = true
			break
		}
	}
	if !found {
		t.Logf("entries: %v", entries)
		t.Fatal("expected at least one Debug-level entry from startup")
	}

	mgr.StopAll()

	if got := mgr.RunningCount(); got != 0 {
		t.Fatalf("RunningCount after StopAll = %d, want 0", got)
	}
}

func TestIntegrationManagerLifecycle(t *testing.T) {
	logger.ResetGlobal()

	echoAddr := startEchoServer(t)

	cfg := &config.Config{
		Defaults: config.Defaults{
			TCPConnectTimeout: 5 * time.Second,
			TCPIdleTimeout:    30 * time.Second,
			UDPSessionTimeout: 60 * time.Second,
		},
		Rules: []config.Rule{
			{
				Protocol: "tcp",
				Local:    "127.0.0.1:0",
				Remote:   echoAddr,
				Enabled:  true,
			},
			{
				Protocol: "tcp",
				Local:    "127.0.0.1:0",
				Remote:   echoAddr,
				Enabled:  false,
			},
		},
	}

	mgr := forwarder.NewManager(cfg)
	t.Cleanup(func() { mgr.StopAll() })

	// StartAll should only start the enabled rule.
	if err := mgr.StartAll(); err != nil {
		t.Fatalf("StartAll: %v", err)
	}
	if got := mgr.RunningCount(); got != 1 {
		t.Fatalf("RunningCount after StartAll = %d, want 1", got)
	}

	// EnableRule(1) should bring the second rule up.
	if err := mgr.EnableRule(1); err != nil {
		t.Fatalf("EnableRule(1): %v", err)
	}
	if got := mgr.RunningCount(); got != 2 {
		t.Fatalf("RunningCount after EnableRule(1) = %d, want 2", got)
	}

	// StopAll should bring everything down.
	mgr.StopAll()
	if got := mgr.RunningCount(); got != 0 {
		t.Fatalf("RunningCount after StopAll = %d, want 0", got)
	}

	fmt.Println("manager lifecycle test passed")
}
