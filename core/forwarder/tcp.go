// Package forwarder implements TCP port forwarding with idle timeout support.
package forwarder

import (
	"context"
	"io"
	"net"
	"sync"
	"time"

	"port_forward/core/logger"
)

// TCPForwarder forwards TCP connections from localAddr to remoteAddr.
type TCPForwarder struct {
	localAddr      string
	remoteAddr     string
	connectTimeout time.Duration
	idleTimeout    time.Duration
	log            *logger.Logger

	mu       sync.Mutex
	listener net.Listener
}

// NewTCPForwarder returns a configured TCPForwarder.
func NewTCPForwarder(localAddr, remoteAddr string, connectTimeout, idleTimeout time.Duration, log *logger.Logger) *TCPForwarder {
	return &TCPForwarder{
		localAddr:      localAddr,
		remoteAddr:     remoteAddr,
		connectTimeout: connectTimeout,
		idleTimeout:    idleTimeout,
		log:            log,
	}
}

// Start begins listening on localAddr and returns the actual bound address.
// It starts the accept loop in a background goroutine. Context cancellation
// stops the listener.
func (f *TCPForwarder) Start(ctx context.Context) (string, error) {
	ln, err := net.Listen("tcp", f.localAddr)
	if err != nil {
		return "", err
	}

	f.mu.Lock()
	f.listener = ln
	f.mu.Unlock()

	addr := ln.Addr().String()
	f.log.Debug("listening on %s, forwarding to %s", addr, f.remoteAddr)

	go f.acceptLoop(ctx, ln)

	return addr, nil
}

// Stop closes the listener.
func (f *TCPForwarder) Stop() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listener != nil {
		f.listener.Close()
		f.listener = nil
	}
}

// acceptLoop accepts connections until the context is cancelled or the
// listener is closed.
func (f *TCPForwarder) acceptLoop(ctx context.Context, ln net.Listener) {
	// When context is done, close the listener to unblock Accept.
	go func() {
		<-ctx.Done()
		ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			// Check if we're shutting down.
			select {
			case <-ctx.Done():
				return
			default:
			}
			// Transient error or listener closed — either way, stop.
			return
		}
		go f.handleConn(ctx, conn)
	}
}

// handleConn dials the remote, then copies data bidirectionally.
func (f *TCPForwarder) handleConn(ctx context.Context, clientConn net.Conn) {
	defer clientConn.Close()

	clientAddr := clientConn.RemoteAddr().String()
	f.log.Debug("new connection from %s", clientAddr)

	remoteConn, err := net.DialTimeout("tcp", f.remoteAddr, f.connectTimeout)
	if err != nil {
		f.log.Warn("connect to %s failed: %v", f.remoteAddr, err)
		return
	}
	defer remoteConn.Close()

	f.log.Debug("connected to %s for client %s", f.remoteAddr, clientAddr)

	// Use a child context so we can cancel both copy goroutines.
	connCtx, connCancel := context.WithCancel(ctx)
	defer connCancel()

	var wg sync.WaitGroup
	wg.Add(2)

	// client -> remote
	go func() {
		defer wg.Done()
		f.copyWithIdleTimeout(connCtx, connCancel, remoteConn, clientConn, "client->remote")
	}()

	// remote -> client
	go func() {
		defer wg.Done()
		f.copyWithIdleTimeout(connCtx, connCancel, clientConn, remoteConn, "remote->client")
	}()

	wg.Wait()
	f.log.Debug("connection closed for %s", clientAddr)
}

// copyWithIdleTimeout copies from src to dst, enforcing an idle timeout on
// reads. When the copy ends (error, EOF, or timeout), it calls cancel to
// signal the other direction to stop.
func (f *TCPForwarder) copyWithIdleTimeout(ctx context.Context, cancel context.CancelFunc, dst, src net.Conn, direction string) {
	buf := make([]byte, 32*1024)
	for {
		// Check context before each read.
		select {
		case <-ctx.Done():
			return
		default:
		}

		if f.idleTimeout > 0 {
			src.SetReadDeadline(time.Now().Add(f.idleTimeout))
		}

		n, err := src.Read(buf)
		if n > 0 {
			if _, wErr := dst.Write(buf[:n]); wErr != nil {
				cancel()
				return
			}
		}
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				f.log.Info("idle timeout on %s", direction)
			}
			if err != io.EOF {
				// Connection closed or timed out.
			}
			cancel()
			return
		}
	}
}
