package forwarder

import (
	"io"
	"net"
	"testing"
	"time"

	"port_forward/core/config"
	"port_forward/core/logger"
)

// managerEchoServer starts a TCP echo server and returns its address and a
// cleanup function. The server echoes back whatever is written to it.
func managerEchoServer(t *testing.T) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("echo server listen: %v", err)
	}
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
	return ln.Addr().String(), func() { ln.Close() }
}

func TestManagerStartStop(t *testing.T) {
	logger.ResetGlobal()

	echoAddr, cleanup := managerEchoServer(t)
	defer cleanup()

	cfg := &config.Config{
		Defaults: config.Defaults{
			TCPConnectTimeout: 5 * time.Second,
			TCPIdleTimeout:    30 * time.Second,
			UDPSessionTimeout: 30 * time.Second,
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

	m := NewManager(cfg)

	if err := m.StartAll(); err != nil {
		t.Fatalf("StartAll: %v", err)
	}

	// Give the forwarder a moment to start listening.
	time.Sleep(50 * time.Millisecond)

	if got := m.RunningCount(); got != 1 {
		t.Fatalf("RunningCount after StartAll: got %d, want 1", got)
	}

	if !m.IsRunning(0) {
		t.Fatal("IsRunning(0) should be true after StartAll")
	}

	m.StopAll()

	if got := m.RunningCount(); got != 0 {
		t.Fatalf("RunningCount after StopAll: got %d, want 0", got)
	}
}

func TestManagerEnableDisable(t *testing.T) {
	logger.ResetGlobal()

	echoAddr, cleanup := managerEchoServer(t)
	defer cleanup()

	cfg := &config.Config{
		Defaults: config.Defaults{
			TCPConnectTimeout: 5 * time.Second,
			TCPIdleTimeout:    30 * time.Second,
			UDPSessionTimeout: 30 * time.Second,
		},
		Rules: []config.Rule{
			{
				Protocol: "tcp",
				Local:    "127.0.0.1:0",
				Remote:   echoAddr,
				Enabled:  false,
			},
		},
	}

	m := NewManager(cfg)

	if err := m.StartAll(); err != nil {
		t.Fatalf("StartAll: %v", err)
	}

	if got := m.RunningCount(); got != 0 {
		t.Fatalf("RunningCount after StartAll with disabled rule: got %d, want 0", got)
	}

	if err := m.EnableRule(0); err != nil {
		t.Fatalf("EnableRule(0): %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	if got := m.RunningCount(); got != 1 {
		t.Fatalf("RunningCount after EnableRule: got %d, want 1", got)
	}

	if err := m.DisableRule(0); err != nil {
		t.Fatalf("DisableRule(0): %v", err)
	}

	if got := m.RunningCount(); got != 0 {
		t.Fatalf("RunningCount after DisableRule: got %d, want 0", got)
	}

	m.StopAll()
}
