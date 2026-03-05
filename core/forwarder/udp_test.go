package forwarder

import (
	"context"
	"net"
	"testing"
	"time"

	"port_forward/core/logger"
)

// startUDPEchoServer starts a UDP server that echoes back everything it receives.
// It returns the server address and a cleanup function.
func startUDPEchoServer(t *testing.T) (string, func()) {
	t.Helper()
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("udp echo server listen: %v", err)
	}
	go func() {
		buf := make([]byte, 65535)
		for {
			n, addr, err := conn.ReadFrom(buf)
			if err != nil {
				return
			}
			conn.WriteTo(buf[:n], addr)
		}
	}()
	return conn.LocalAddr().String(), func() { conn.Close() }
}

func TestUDPForwardBasic(t *testing.T) {
	echoAddr, cleanup := startUDPEchoServer(t)
	defer cleanup()

	log := logger.GetLogger("test-udp-basic")
	log.SetLevel(logger.Debug)

	fwd := NewUDPForwarder("127.0.0.1:0", echoAddr, 5*time.Second, log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	addr, err := fwd.Start(ctx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer fwd.Stop()

	// Resolve forwarder address and send a packet.
	raddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	clientConn, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		t.Fatalf("dial forwarder: %v", err)
	}
	defer clientConn.Close()

	msg := "hello udp forward"
	_, err = clientConn.Write([]byte(msg))
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	buf := make([]byte, 1024)
	clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := clientConn.Read(buf)
	if err != nil {
		t.Fatalf("read echo: %v", err)
	}
	if string(buf[:n]) != msg {
		t.Fatalf("expected %q, got %q", msg, string(buf[:n]))
	}
}

func TestUDPSessionTimeout(t *testing.T) {
	echoAddr, cleanup := startUDPEchoServer(t)
	defer cleanup()

	log := logger.GetLogger("test-udp-timeout")
	log.SetLevel(logger.Debug)

	timeout := 500 * time.Millisecond
	fwd := NewUDPForwarder("127.0.0.1:0", echoAddr, timeout, log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	addr, err := fwd.Start(ctx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer fwd.Stop()

	// Send a packet to create a session.
	raddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	clientConn, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		t.Fatalf("dial forwarder: %v", err)
	}
	defer clientConn.Close()

	_, err = clientConn.Write([]byte("ping"))
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	// Read the echo to confirm session is active.
	buf := make([]byte, 1024)
	clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, err = clientConn.Read(buf)
	if err != nil {
		t.Fatalf("read echo: %v", err)
	}

	if fwd.SessionCount() != 1 {
		t.Fatalf("expected 1 session, got %d", fwd.SessionCount())
	}

	// Wait longer than session timeout + cleanup interval (timeout/2).
	time.Sleep(800 * time.Millisecond)

	if fwd.SessionCount() != 0 {
		t.Fatalf("expected 0 sessions after timeout, got %d", fwd.SessionCount())
	}
}
