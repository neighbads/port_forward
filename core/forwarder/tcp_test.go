package forwarder

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"port_forward/core/logger"
)

// startEchoServer starts a TCP server that echoes back everything it receives.
// It returns the listener address and a cleanup function.
func startEchoServer(t *testing.T) (string, func()) {
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

func TestTCPForwardBasic(t *testing.T) {
	echoAddr, cleanup := startEchoServer(t)
	defer cleanup()

	log := logger.GetLogger("test-basic")
	log.SetLevel(logger.Debug)

	fwd := NewTCPForwarder("127.0.0.1:0", echoAddr, 5*time.Second, 0, log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	addr, err := fwd.Start(ctx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer fwd.Stop()

	// Connect through forwarder.
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial forwarder: %v", err)
	}
	defer conn.Close()

	msg := "hello port_forward"
	fmt.Fprint(conn, msg)

	buf := make([]byte, len(msg))
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, err = io.ReadFull(conn, buf)
	if err != nil {
		t.Fatalf("read echo: %v", err)
	}
	if string(buf) != msg {
		t.Fatalf("expected %q, got %q", msg, string(buf))
	}
}

func TestTCPForwardConnectTimeout(t *testing.T) {
	log := logger.GetLogger("test-connect-timeout")
	log.SetLevel(logger.Debug)

	// 192.0.2.1 is TEST-NET-1 (RFC 5737) — non-routable, will cause connect timeout.
	fwd := NewTCPForwarder("127.0.0.1:0", "192.0.2.1:9999", 500*time.Millisecond, 0, log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	addr, err := fwd.Start(ctx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer fwd.Stop()

	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial forwarder: %v", err)
	}

	// The forwarder should close the client connection after failing to connect upstream.
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	buf := make([]byte, 1)
	_, err = conn.Read(buf)
	if err == nil {
		t.Fatal("expected read error after upstream connect failure")
	}
	conn.Close()

	// Verify Warn was logged.
	time.Sleep(100 * time.Millisecond)
	entries := log.Entries()
	found := false
	for _, e := range entries {
		if e.Level == logger.Warn && strings.Contains(e.Message, "connect") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected Warn log about connect failure")
	}
}

func TestTCPForwardContextCancel(t *testing.T) {
	log := logger.GetLogger("test-ctx-cancel")
	log.SetLevel(logger.Debug)

	// We don't need a real remote for this test — we just test listener shutdown.
	fwd := NewTCPForwarder("127.0.0.1:0", "127.0.0.1:1", 1*time.Second, 0, log)

	ctx, cancel := context.WithCancel(context.Background())

	addr, err := fwd.Start(ctx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Verify we can connect (listener is up).
	conn, err := net.DialTimeout("tcp", addr, 1*time.Second)
	if err != nil {
		t.Fatalf("dial forwarder: %v", err)
	}
	conn.Close()

	// Cancel context — should stop the listener.
	cancel()

	// Give the accept loop time to react.
	time.Sleep(200 * time.Millisecond)

	// New connections should be refused.
	_, err = net.DialTimeout("tcp", addr, 500*time.Millisecond)
	if err == nil {
		t.Fatal("expected connection refused after context cancel")
	}
}
