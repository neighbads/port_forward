package forwarder

import (
	"context"
	"net"
	"sync"
	"time"

	"port_forward/core/logger"
)

// udpSession tracks a single client's UDP session through the forwarder.
type udpSession struct {
	remoteConn net.Conn
	lastActive time.Time
	clientAddr net.Addr
}

// UDPForwarder forwards UDP packets from localAddr to remoteAddr, maintaining
// per-client sessions that are cleaned up after a configurable timeout.
type UDPForwarder struct {
	localAddr      string
	remoteAddr     string
	sessionTimeout time.Duration
	log            *logger.Logger

	mu       sync.RWMutex
	conn     net.PacketConn
	sessions map[string]*udpSession
}

// NewUDPForwarder returns a configured UDPForwarder.
func NewUDPForwarder(localAddr, remoteAddr string, sessionTimeout time.Duration, log *logger.Logger) *UDPForwarder {
	return &UDPForwarder{
		localAddr:      localAddr,
		remoteAddr:     remoteAddr,
		sessionTimeout: sessionTimeout,
		log:            log,
		sessions:       make(map[string]*udpSession),
	}
}

// Start begins listening on localAddr and returns the actual bound address.
// It starts the read loop and cleanup loop in background goroutines.
func (f *UDPForwarder) Start(ctx context.Context) (string, error) {
	conn, err := net.ListenPacket("udp", f.localAddr)
	if err != nil {
		return "", err
	}

	f.mu.Lock()
	f.conn = conn
	f.mu.Unlock()

	addr := conn.LocalAddr().String()
	f.log.Debug("UDP listening on %s, forwarding to %s", addr, f.remoteAddr)

	go f.readLoop(ctx, conn)
	go f.cleanupLoop(ctx)

	return addr, nil
}

// Stop closes the local connection and all session remote connections.
func (f *UDPForwarder) Stop() {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.conn != nil {
		f.conn.Close()
		f.conn = nil
	}

	for key, sess := range f.sessions {
		sess.remoteConn.Close()
		delete(f.sessions, key)
	}
}

// SessionCount returns the number of active sessions.
func (f *UDPForwarder) SessionCount() int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return len(f.sessions)
}

// readLoop reads packets from the local connection and forwards them to the
// remote address, creating sessions as needed.
func (f *UDPForwarder) readLoop(ctx context.Context, localConn net.PacketConn) {
	buf := make([]byte, 65535)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Set a read deadline so we periodically check ctx.Done.
		localConn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, clientAddr, err := localConn.ReadFrom(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			select {
			case <-ctx.Done():
			default:
				f.log.Debug("readLoop error: %v", err)
			}
			return
		}

		key := clientAddr.String()

		f.mu.RLock()
		sess, exists := f.sessions[key]
		f.mu.RUnlock()

		if !exists {
			remoteConn, err := net.Dial("udp", f.remoteAddr)
			if err != nil {
				f.log.Warn("dial remote %s failed: %v", f.remoteAddr, err)
				continue
			}
			sess = &udpSession{
				remoteConn: remoteConn,
				lastActive: time.Now(),
				clientAddr: clientAddr,
			}
			f.mu.Lock()
			f.sessions[key] = sess
			f.mu.Unlock()

			f.log.Debug("new UDP session for %s", key)
			go f.remoteReadLoop(ctx, localConn, sess, key)
		}

		f.mu.Lock()
		sess.lastActive = time.Now()
		f.mu.Unlock()

		_, err = sess.remoteConn.Write(buf[:n])
		if err != nil {
			f.log.Warn("write to remote for %s: %v", key, err)
		}
	}
}

// remoteReadLoop reads response packets from the remote connection and
// forwards them back to the client.
func (f *UDPForwarder) remoteReadLoop(ctx context.Context, localConn net.PacketConn, sess *udpSession, key string) {
	buf := make([]byte, 65535)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		sess.remoteConn.SetReadDeadline(time.Now().Add(f.sessionTimeout))
		n, err := sess.remoteConn.Read(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				f.log.Debug("remote read timeout for session %s", key)
			}
			return
		}

		f.mu.Lock()
		sess.lastActive = time.Now()
		f.mu.Unlock()

		_, err = localConn.WriteTo(buf[:n], sess.clientAddr)
		if err != nil {
			f.log.Warn("writeTo client %s: %v", key, err)
			return
		}
	}
}

// cleanupLoop periodically removes sessions that have been idle for longer
// than the session timeout.
func (f *UDPForwarder) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(f.sessionTimeout / 2)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			f.mu.Lock()
			now := time.Now()
			for key, sess := range f.sessions {
				if now.Sub(sess.lastActive) > f.sessionTimeout {
					sess.remoteConn.Close()
					delete(f.sessions, key)
					f.log.Info("cleaned up UDP session %s", key)
				}
			}
			f.mu.Unlock()
		}
	}
}
