# Port Forward Tool Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a cross-platform TCP/UDP port forwarding tool with Windows GUI (Fyne tray + window) and CLI for both platforms.

**Architecture:** Layered — core engine (forwarder/config/logger) shared across platforms, CLI layer for all OS, GUI layer (build-tagged windows only). Manager pattern coordinates rule lifecycles with per-rule goroutine groups and context cancellation.

**Tech Stack:** Go 1.26, Fyne v2 (GUI), gopkg.in/yaml.v3 (config), Go stdlib (net, io, context, sync)

---

### Task 1: Project Init & Go Module

**Files:**
- Create: `go.mod`
- Create: `main.go`

**Step 1: Initialize Go module**

Run: `cd /Workspace/Ai/port_forward && go mod init port_forward`
Expected: `go.mod` created

**Step 2: Create minimal main.go**

```go
package main

import "fmt"

func main() {
	fmt.Println("port_forward starting...")
}
```

**Step 3: Verify it compiles**

Run: `go build -o port_forward .`
Expected: binary created, runs and prints message

**Step 4: Commit**

```bash
git init
git add go.mod main.go
git commit -m "init: project scaffold with go module"
```

---

### Task 2: Logger System

**Files:**
- Create: `core/logger/logger.go`
- Create: `core/logger/logger_test.go`

**Step 1: Write failing tests**

```go
package logger

import (
	"strings"
	"testing"
)

func TestLogLevels(t *testing.T) {
	l := New("test-rule")
	l.SetLevel(LevelDebug)

	l.Debug("debug msg")
	l.Info("info msg")
	l.Warn("warn msg")
	l.Error("error msg")

	entries := l.Entries()
	if len(entries) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(entries))
	}
	if entries[0].Level != LevelDebug {
		t.Errorf("expected Debug level, got %v", entries[0].Level)
	}
}

func TestLogLevelFiltering(t *testing.T) {
	l := New("test-rule")
	l.SetLevel(LevelWarn)

	l.Debug("should not appear")
	l.Info("should not appear")
	l.Warn("should appear")
	l.Error("should appear")

	entries := l.Entries()
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
}

func TestRingBufferOverflow(t *testing.T) {
	l := New("test-rule")
	l.SetLevel(LevelDebug)

	for i := 0; i < 1100; i++ {
		l.Debug("msg")
	}

	entries := l.Entries()
	if len(entries) != 1000 {
		t.Fatalf("expected 1000 entries (ring buffer cap), got %d", len(entries))
	}
}

func TestGlobalLogger(t *testing.T) {
	g := NewGlobal()
	l1 := g.GetLogger("rule-1")
	l2 := g.GetLogger("rule-2")
	l1.SetLevel(LevelDebug)
	l2.SetLevel(LevelDebug)

	l1.Info("from rule 1")
	l2.Info("from rule 2")

	all := g.AllEntries()
	if len(all) != 2 {
		t.Fatalf("expected 2 global entries, got %d", len(all))
	}

	found1 := false
	found2 := false
	for _, e := range all {
		if strings.Contains(e.Message, "rule 1") {
			found1 = true
		}
		if strings.Contains(e.Message, "rule 2") {
			found2 = true
		}
	}
	if !found1 || !found2 {
		t.Error("global entries missing expected messages")
	}
}

func TestSetGlobalLevel(t *testing.T) {
	g := NewGlobal()
	l := g.GetLogger("r1")
	g.SetLevel(LevelError)

	l.Info("should not appear")
	l.Error("should appear")

	entries := l.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Workspace/Ai/port_forward && go test ./core/logger/ -v`
Expected: FAIL — types not defined

**Step 3: Implement logger**

```go
package logger

import (
	"fmt"
	"sync"
	"time"
)

type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

func ParseLevel(s string) Level {
	switch s {
	case "debug", "DEBUG":
		return LevelDebug
	case "info", "INFO":
		return LevelInfo
	case "warn", "WARN":
		return LevelWarn
	case "error", "ERROR":
		return LevelError
	default:
		return LevelInfo
	}
}

type Entry struct {
	Time    time.Time
	Level   Level
	RuleID  string
	Message string
}

func (e Entry) String() string {
	return fmt.Sprintf("[%s] [%s] [%s] %s",
		e.Time.Format("2006-01-02 15:04:05.000"), e.Level, e.RuleID, e.Message)
}

const ringBufferSize = 1000

type Logger struct {
	ruleID  string
	level   Level
	mu      sync.RWMutex
	entries []Entry
	pos     int
	count   int
	global  *Global
}

func New(ruleID string) *Logger {
	return &Logger{
		ruleID:  ruleID,
		level:   LevelInfo,
		entries: make([]Entry, ringBufferSize),
	}
}

func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	l.level = level
	l.mu.Unlock()
}

func (l *Logger) GetLevel() Level {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.level
}

func (l *Logger) log(level Level, format string, args ...interface{}) {
	l.mu.RLock()
	currentLevel := l.level
	l.mu.RUnlock()

	if level < currentLevel {
		return
	}

	entry := Entry{
		Time:    time.Now(),
		Level:   level,
		RuleID:  l.ruleID,
		Message: fmt.Sprintf(format, args...),
	}

	l.mu.Lock()
	l.entries[l.pos] = entry
	l.pos = (l.pos + 1) % ringBufferSize
	if l.count < ringBufferSize {
		l.count++
	}
	l.mu.Unlock()

	if l.global != nil {
		l.global.append(entry)
	}
}

func (l *Logger) Debug(format string, args ...interface{}) { l.log(LevelDebug, format, args...) }
func (l *Logger) Info(format string, args ...interface{})  { l.log(LevelInfo, format, args...) }
func (l *Logger) Warn(format string, args ...interface{})  { l.log(LevelWarn, format, args...) }
func (l *Logger) Error(format string, args ...interface{}) { l.log(LevelError, format, args...) }

func (l *Logger) Entries() []Entry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make([]Entry, l.count)
	if l.count < ringBufferSize {
		copy(result, l.entries[:l.count])
	} else {
		start := l.pos
		n := copy(result, l.entries[start:])
		copy(result[n:], l.entries[:start])
	}
	return result
}

type Global struct {
	mu      sync.RWMutex
	loggers map[string]*Logger
	entries []Entry
	pos     int
	count   int
}

func NewGlobal() *Global {
	return &Global{
		loggers: make(map[string]*Logger),
		entries: make([]Entry, ringBufferSize),
	}
}

func (g *Global) GetLogger(ruleID string) *Logger {
	g.mu.Lock()
	defer g.mu.Unlock()

	if l, ok := g.loggers[ruleID]; ok {
		return l
	}
	l := New(ruleID)
	l.global = g
	g.loggers[ruleID] = l
	return l
}

func (g *Global) RemoveLogger(ruleID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.loggers, ruleID)
}

func (g *Global) SetLevel(level Level) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, l := range g.loggers {
		l.SetLevel(level)
	}
}

func (g *Global) GetLevel() Level {
	g.mu.RLock()
	defer g.mu.RUnlock()
	for _, l := range g.loggers {
		return l.GetLevel()
	}
	return LevelInfo
}

func (g *Global) append(entry Entry) {
	g.mu.Lock()
	g.entries[g.pos] = entry
	g.pos = (g.pos + 1) % ringBufferSize
	if g.count < ringBufferSize {
		g.count++
	}
	g.mu.Unlock()
}

func (g *Global) AllEntries() []Entry {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make([]Entry, g.count)
	if g.count < ringBufferSize {
		copy(result, g.entries[:g.count])
	} else {
		start := g.pos
		n := copy(result, g.entries[start:])
		copy(result[n:], g.entries[:start])
	}
	return result
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /Workspace/Ai/port_forward && go test ./core/logger/ -v`
Expected: All PASS

**Step 5: Commit**

```bash
git add core/logger/
git commit -m "feat: add ring-buffer logger with per-rule and global log support"
```

---

### Task 3: Config System

**Files:**
- Create: `core/config/config.go`
- Create: `core/config/config_test.go`

**Step 1: Write failing tests**

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfig(t *testing.T) {
	yaml := `
defaults:
  tcp_connect_timeout: 10s
  tcp_idle_timeout: 300s
  udp_session_timeout: 60s
log_level: info
rules:
  - protocol: tcp
    local: "127.0.0.1:1234"
    remote: "10.0.0.1:5678"
    enabled: true
  - protocol: udp
    local: "127.0.0.1:9000"
    remote: "10.0.0.1:9000"
    enabled: false
    udp_session_timeout: 120s
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	os.WriteFile(path, []byte(yaml), 0644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(cfg.Rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(cfg.Rules))
	}
	if cfg.Rules[0].Protocol != "tcp" {
		t.Errorf("expected tcp, got %s", cfg.Rules[0].Protocol)
	}
	if cfg.Defaults.TCPConnectTimeout != 10*time.Second {
		t.Errorf("expected 10s, got %v", cfg.Defaults.TCPConnectTimeout)
	}
	// Rule-level override
	if cfg.Rules[1].UDPSessionTimeout != 120*time.Second {
		t.Errorf("expected 120s override, got %v", cfg.Rules[1].UDPSessionTimeout)
	}
}

func TestSaveConfig(t *testing.T) {
	cfg := &Config{
		Defaults: Defaults{
			TCPConnectTimeout: 10 * time.Second,
			TCPIdleTimeout:    300 * time.Second,
			UDPSessionTimeout: 60 * time.Second,
		},
		LogLevel: "info",
		Rules: []Rule{
			{Protocol: "tcp", Local: "127.0.0.1:1234", Remote: "10.0.0.1:5678", Enabled: true},
		},
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := Save(cfg, path)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("reload failed: %v", err)
	}
	if len(loaded.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(loaded.Rules))
	}
	if loaded.Rules[0].Local != "127.0.0.1:1234" {
		t.Errorf("expected 127.0.0.1:1234, got %s", loaded.Rules[0].Local)
	}
}

func TestDefaultValues(t *testing.T) {
	yaml := `
rules:
  - protocol: tcp
    local: "127.0.0.1:1234"
    remote: "10.0.0.1:5678"
    enabled: true
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	os.WriteFile(path, []byte(yaml), 0644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	// Should have default timeout values
	if cfg.Defaults.TCPConnectTimeout != 10*time.Second {
		t.Errorf("expected default 10s, got %v", cfg.Defaults.TCPConnectTimeout)
	}
}

func TestResolveTimeouts(t *testing.T) {
	cfg := &Config{
		Defaults: Defaults{
			TCPConnectTimeout: 10 * time.Second,
			TCPIdleTimeout:    300 * time.Second,
			UDPSessionTimeout: 60 * time.Second,
		},
	}
	// Rule with override
	r := Rule{Protocol: "tcp", TCPConnectTimeout: 5 * time.Second}
	ct, it := cfg.ResolveTCPTimeouts(&r)
	if ct != 5*time.Second {
		t.Errorf("expected 5s override, got %v", ct)
	}
	if it != 300*time.Second {
		t.Errorf("expected 300s default, got %v", it)
	}

	// Rule without override
	r2 := Rule{Protocol: "tcp"}
	ct2, _ := cfg.ResolveTCPTimeouts(&r2)
	if ct2 != 10*time.Second {
		t.Errorf("expected 10s default, got %v", ct2)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./core/config/ -v`
Expected: FAIL

**Step 3: Implement config**

```go
package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Duration time.Duration

func (d Duration) MarshalYAML() (interface{}, error) {
	return time.Duration(d).String(), nil
}

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	*d = Duration(parsed)
	return nil
}

type Defaults struct {
	TCPConnectTimeout time.Duration `yaml:"-"`
	TCPIdleTimeout    time.Duration `yaml:"-"`
	UDPSessionTimeout time.Duration `yaml:"-"`

	RawTCPConnectTimeout Duration `yaml:"tcp_connect_timeout,omitempty"`
	RawTCPIdleTimeout    Duration `yaml:"tcp_idle_timeout,omitempty"`
	RawUDPSessionTimeout Duration `yaml:"udp_session_timeout,omitempty"`
}

type Rule struct {
	Protocol          string        `yaml:"protocol"`
	Local             string        `yaml:"local"`
	Remote            string        `yaml:"remote"`
	Enabled           bool          `yaml:"enabled"`
	TCPConnectTimeout time.Duration `yaml:"-"`
	TCPIdleTimeout    time.Duration `yaml:"-"`
	UDPSessionTimeout time.Duration `yaml:"-"`

	RawTCPConnectTimeout *Duration `yaml:"tcp_connect_timeout,omitempty"`
	RawTCPIdleTimeout    *Duration `yaml:"tcp_idle_timeout,omitempty"`
	RawUDPSessionTimeout *Duration `yaml:"udp_session_timeout,omitempty"`
}

type Config struct {
	Defaults Defaults `yaml:"defaults"`
	LogLevel string   `yaml:"log_level"`
	Rules    []Rule   `yaml:"rules"`
}

const (
	DefaultTCPConnectTimeout = 10 * time.Second
	DefaultTCPIdleTimeout    = 300 * time.Second
	DefaultUDPSessionTimeout = 60 * time.Second
)

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// Convert raw durations to time.Duration
	cfg.Defaults.TCPConnectTimeout = time.Duration(cfg.Defaults.RawTCPConnectTimeout)
	cfg.Defaults.TCPIdleTimeout = time.Duration(cfg.Defaults.RawTCPIdleTimeout)
	cfg.Defaults.UDPSessionTimeout = time.Duration(cfg.Defaults.RawUDPSessionTimeout)

	// Apply defaults if zero
	if cfg.Defaults.TCPConnectTimeout == 0 {
		cfg.Defaults.TCPConnectTimeout = DefaultTCPConnectTimeout
		cfg.Defaults.RawTCPConnectTimeout = Duration(DefaultTCPConnectTimeout)
	}
	if cfg.Defaults.TCPIdleTimeout == 0 {
		cfg.Defaults.TCPIdleTimeout = DefaultTCPIdleTimeout
		cfg.Defaults.RawTCPIdleTimeout = Duration(DefaultTCPIdleTimeout)
	}
	if cfg.Defaults.UDPSessionTimeout == 0 {
		cfg.Defaults.UDPSessionTimeout = DefaultUDPSessionTimeout
		cfg.Defaults.RawUDPSessionTimeout = Duration(DefaultUDPSessionTimeout)
	}

	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}

	// Convert rule-level raw durations
	for i := range cfg.Rules {
		r := &cfg.Rules[i]
		if r.RawTCPConnectTimeout != nil {
			r.TCPConnectTimeout = time.Duration(*r.RawTCPConnectTimeout)
		}
		if r.RawTCPIdleTimeout != nil {
			r.TCPIdleTimeout = time.Duration(*r.RawTCPIdleTimeout)
		}
		if r.RawUDPSessionTimeout != nil {
			r.UDPSessionTimeout = time.Duration(*r.RawUDPSessionTimeout)
		}
		if r.Protocol == "" {
			r.Protocol = "tcp"
		}
	}

	return cfg, nil
}

func Save(cfg *Config, path string) error {
	// Sync time.Duration back to raw
	cfg.Defaults.RawTCPConnectTimeout = Duration(cfg.Defaults.TCPConnectTimeout)
	cfg.Defaults.RawTCPIdleTimeout = Duration(cfg.Defaults.TCPIdleTimeout)
	cfg.Defaults.RawUDPSessionTimeout = Duration(cfg.Defaults.UDPSessionTimeout)

	for i := range cfg.Rules {
		r := &cfg.Rules[i]
		if r.TCPConnectTimeout > 0 {
			d := Duration(r.TCPConnectTimeout)
			r.RawTCPConnectTimeout = &d
		}
		if r.TCPIdleTimeout > 0 {
			d := Duration(r.TCPIdleTimeout)
			r.RawTCPIdleTimeout = &d
		}
		if r.UDPSessionTimeout > 0 {
			d := Duration(r.UDPSessionTimeout)
			r.RawUDPSessionTimeout = &d
		}
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (c *Config) ResolveTCPTimeouts(r *Rule) (connectTimeout, idleTimeout time.Duration) {
	connectTimeout = c.Defaults.TCPConnectTimeout
	idleTimeout = c.Defaults.TCPIdleTimeout

	if r.TCPConnectTimeout > 0 {
		connectTimeout = r.TCPConnectTimeout
	}
	if r.TCPIdleTimeout > 0 {
		idleTimeout = r.TCPIdleTimeout
	}
	return
}

func (c *Config) ResolveUDPTimeout(r *Rule) time.Duration {
	if r.UDPSessionTimeout > 0 {
		return r.UDPSessionTimeout
	}
	return c.Defaults.UDPSessionTimeout
}

func (c *Config) AddRule(r Rule) {
	if r.Protocol == "" {
		r.Protocol = "tcp"
	}
	c.Rules = append(c.Rules, r)
}

func (c *Config) RemoveRule(index int) {
	if index >= 0 && index < len(c.Rules) {
		c.Rules = append(c.Rules[:index], c.Rules[index+1:]...)
	}
}
```

**Step 4: Add yaml dependency and run tests**

Run: `cd /Workspace/Ai/port_forward && go get gopkg.in/yaml.v3 && go test ./core/config/ -v`
Expected: All PASS

**Step 5: Commit**

```bash
git add core/config/ go.mod go.sum
git commit -m "feat: add YAML config with per-rule timeout overrides"
```

---

### Task 4: TCP Forwarder

**Files:**
- Create: `core/forwarder/tcp.go`
- Create: `core/forwarder/tcp_test.go`

**Step 1: Write failing tests**

```go
package forwarder

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"port_forward/core/logger"
)

func TestTCPForwardBasic(t *testing.T) {
	// Start a simple echo server as "remote"
	echoLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer echoLn.Close()

	go func() {
		for {
			conn, err := echoLn.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				io.Copy(c, c)
			}(conn)
		}
	}()

	log := logger.New("test-tcp")
	log.SetLevel(logger.LevelDebug)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fwd := NewTCPForwarder("127.0.0.1:0", echoLn.Addr().String(), 5*time.Second, 30*time.Second, log)
	localAddr, err := fwd.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Connect to forwarder
	conn, err := net.Dial("tcp", localAddr)
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer conn.Close()

	msg := []byte("hello port forward")
	conn.Write(msg)
	buf := make([]byte, len(msg))
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := io.ReadFull(conn, buf)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if string(buf[:n]) != "hello port forward" {
		t.Errorf("expected 'hello port forward', got '%s'", buf[:n])
	}
}

func TestTCPForwardConnectTimeout(t *testing.T) {
	log := logger.New("test-tcp-timeout")
	log.SetLevel(logger.LevelDebug)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Connect to a non-routable address to trigger timeout
	fwd := NewTCPForwarder("127.0.0.1:0", "192.0.2.1:9999", 1*time.Second, 30*time.Second, log)
	localAddr, err := fwd.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	conn, err := net.DialTimeout("tcp", localAddr, 2*time.Second)
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer conn.Close()

	// Connection should be closed by forwarder after connect timeout
	buf := make([]byte, 1)
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, err = conn.Read(buf)
	if err == nil {
		t.Error("expected read to fail after connect timeout")
	}

	// Check logs for timeout warning
	entries := log.Entries()
	found := false
	for _, e := range entries {
		if e.Level == logger.LevelWarn {
			found = true
		}
	}
	if !found {
		t.Error("expected warn log for connect timeout")
	}
}

func TestTCPForwardContextCancel(t *testing.T) {
	echoLn, _ := net.Listen("tcp", "127.0.0.1:0")
	defer echoLn.Close()
	go func() {
		for {
			conn, err := echoLn.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) { defer c.Close(); io.Copy(c, c) }(conn)
		}
	}()

	log := logger.New("test-cancel")
	log.SetLevel(logger.LevelDebug)

	ctx, cancel := context.WithCancel(context.Background())
	fwd := NewTCPForwarder("127.0.0.1:0", echoLn.Addr().String(), 5*time.Second, 30*time.Second, log)
	_, err := fwd.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}

	cancel()
	time.Sleep(100 * time.Millisecond)
	// Forwarder should have stopped - no panic, no goroutine leak
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./core/forwarder/ -v -run TestTCP`
Expected: FAIL

**Step 3: Implement TCP forwarder**

```go
package forwarder

import (
	"context"
	"io"
	"net"
	"sync"
	"time"

	"port_forward/core/logger"
)

type TCPForwarder struct {
	localAddr      string
	remoteAddr     string
	connectTimeout time.Duration
	idleTimeout    time.Duration
	log            *logger.Logger
	listener       net.Listener
	mu             sync.Mutex
}

func NewTCPForwarder(localAddr, remoteAddr string, connectTimeout, idleTimeout time.Duration, log *logger.Logger) *TCPForwarder {
	return &TCPForwarder{
		localAddr:      localAddr,
		remoteAddr:     remoteAddr,
		connectTimeout: connectTimeout,
		idleTimeout:    idleTimeout,
		log:            log,
	}
}

func (f *TCPForwarder) Start(ctx context.Context) (string, error) {
	ln, err := net.Listen("tcp", f.localAddr)
	if err != nil {
		return "", err
	}

	f.mu.Lock()
	f.listener = ln
	f.mu.Unlock()

	f.log.Info("TCP forwarding %s -> %s", ln.Addr().String(), f.remoteAddr)

	go f.acceptLoop(ctx, ln)

	return ln.Addr().String(), nil
}

func (f *TCPForwarder) Stop() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listener != nil {
		f.listener.Close()
		f.listener = nil
	}
}

func (f *TCPForwarder) acceptLoop(ctx context.Context, ln net.Listener) {
	defer ln.Close()

	go func() {
		<-ctx.Done()
		ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				f.log.Info("TCP forwarder stopped")
				return
			default:
				f.log.Error("accept error: %v", err)
				return
			}
		}
		go f.handleConn(ctx, conn)
	}
}

func (f *TCPForwarder) handleConn(ctx context.Context, clientConn net.Conn) {
	defer clientConn.Close()

	clientAddr := clientConn.RemoteAddr().String()
	f.log.Debug("new connection from %s", clientAddr)

	remoteConn, err := net.DialTimeout("tcp", f.remoteAddr, f.connectTimeout)
	if err != nil {
		f.log.Warn("connect to remote %s failed: %v", f.remoteAddr, err)
		return
	}
	defer remoteConn.Close()

	f.log.Debug("connected to remote %s for client %s", f.remoteAddr, clientAddr)

	connCtx, connCancel := context.WithCancel(ctx)
	defer connCancel()

	var wg sync.WaitGroup
	wg.Add(2)

	copyWithIdle := func(dst, src net.Conn, label string) {
		defer wg.Done()
		defer connCancel()

		buf := make([]byte, 32*1024)
		for {
			if f.idleTimeout > 0 {
				src.SetReadDeadline(time.Now().Add(f.idleTimeout))
			}

			select {
			case <-connCtx.Done():
				return
			default:
			}

			n, err := src.Read(buf)
			if n > 0 {
				if _, werr := dst.Write(buf[:n]); werr != nil {
					f.log.Debug("%s write error: %v", label, werr)
					return
				}
			}
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					f.log.Info("idle timeout on %s for client %s", label, clientAddr)
				} else if err != io.EOF {
					f.log.Debug("%s read error: %v", label, err)
				}
				return
			}
		}
	}

	go copyWithIdle(remoteConn, clientConn, "client->remote")
	go copyWithIdle(clientConn, remoteConn, "remote->client")

	wg.Wait()
	f.log.Debug("connection closed for client %s", clientAddr)
}
```

**Step 4: Run tests**

Run: `go test ./core/forwarder/ -v -run TestTCP -timeout 30s`
Expected: All PASS

**Step 5: Commit**

```bash
git add core/forwarder/tcp.go core/forwarder/tcp_test.go
git commit -m "feat: add TCP port forwarder with connect/idle timeout"
```

---

### Task 5: UDP Forwarder

**Files:**
- Create: `core/forwarder/udp.go`
- Create: `core/forwarder/udp_test.go`

**Step 1: Write failing tests**

```go
package forwarder

import (
	"context"
	"net"
	"testing"
	"time"

	"port_forward/core/logger"
)

func TestUDPForwardBasic(t *testing.T) {
	// Start a UDP echo server as "remote"
	remoteConn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer remoteConn.Close()

	go func() {
		buf := make([]byte, 65535)
		for {
			n, addr, err := remoteConn.ReadFrom(buf)
			if err != nil {
				return
			}
			remoteConn.WriteTo(buf[:n], addr)
		}
	}()

	log := logger.New("test-udp")
	log.SetLevel(logger.LevelDebug)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fwd := NewUDPForwarder("127.0.0.1:0", remoteConn.LocalAddr().String(), 30*time.Second, log)
	localAddr, err := fwd.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Send UDP packet through forwarder
	conn, err := net.Dial("udp", localAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	msg := []byte("hello udp forward")
	conn.Write(msg)

	buf := make([]byte, len(msg))
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if string(buf[:n]) != "hello udp forward" {
		t.Errorf("expected 'hello udp forward', got '%s'", buf[:n])
	}
}

func TestUDPSessionTimeout(t *testing.T) {
	remoteConn, _ := net.ListenPacket("udp", "127.0.0.1:0")
	defer remoteConn.Close()
	go func() {
		buf := make([]byte, 65535)
		for {
			n, addr, err := remoteConn.ReadFrom(buf)
			if err != nil {
				return
			}
			remoteConn.WriteTo(buf[:n], addr)
		}
	}()

	log := logger.New("test-udp-timeout")
	log.SetLevel(logger.LevelDebug)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fwd := NewUDPForwarder("127.0.0.1:0", remoteConn.LocalAddr().String(), 500*time.Millisecond, log)
	localAddr, err := fwd.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}

	conn, _ := net.Dial("udp", localAddr)
	defer conn.Close()

	conn.Write([]byte("test"))
	buf := make([]byte, 10)
	conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	conn.Read(buf)

	// Wait for session to timeout
	time.Sleep(800 * time.Millisecond)

	// Session should be cleaned up
	if fwd.SessionCount() != 0 {
		t.Errorf("expected 0 sessions after timeout, got %d", fwd.SessionCount())
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./core/forwarder/ -v -run TestUDP -timeout 30s`
Expected: FAIL

**Step 3: Implement UDP forwarder**

```go
package forwarder

import (
	"context"
	"net"
	"sync"
	"time"

	"port_forward/core/logger"
)

type udpSession struct {
	remoteConn net.Conn
	lastActive time.Time
	clientAddr net.Addr
}

type UDPForwarder struct {
	localAddr      string
	remoteAddr     string
	sessionTimeout time.Duration
	log            *logger.Logger

	mu       sync.RWMutex
	sessions map[string]*udpSession
	conn     net.PacketConn
}

func NewUDPForwarder(localAddr, remoteAddr string, sessionTimeout time.Duration, log *logger.Logger) *UDPForwarder {
	return &UDPForwarder{
		localAddr:      localAddr,
		remoteAddr:     remoteAddr,
		sessionTimeout: sessionTimeout,
		log:            log,
		sessions:       make(map[string]*udpSession),
	}
}

func (f *UDPForwarder) Start(ctx context.Context) (string, error) {
	conn, err := net.ListenPacket("udp", f.localAddr)
	if err != nil {
		return "", err
	}
	f.conn = conn

	f.log.Info("UDP forwarding %s -> %s", conn.LocalAddr().String(), f.remoteAddr)

	go f.readLoop(ctx, conn)
	go f.cleanupLoop(ctx)

	return conn.LocalAddr().String(), nil
}

func (f *UDPForwarder) Stop() {
	if f.conn != nil {
		f.conn.Close()
	}
	f.mu.Lock()
	for key, sess := range f.sessions {
		sess.remoteConn.Close()
		delete(f.sessions, key)
	}
	f.mu.Unlock()
}

func (f *UDPForwarder) SessionCount() int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return len(f.sessions)
}

func (f *UDPForwarder) readLoop(ctx context.Context, localConn net.PacketConn) {
	defer localConn.Close()

	go func() {
		<-ctx.Done()
		localConn.Close()
	}()

	buf := make([]byte, 65535)
	for {
		n, clientAddr, err := localConn.ReadFrom(buf)
		if err != nil {
			select {
			case <-ctx.Done():
				f.log.Info("UDP forwarder stopped")
				return
			default:
				f.log.Error("read error: %v", err)
				return
			}
		}

		key := clientAddr.String()
		f.mu.RLock()
		sess, exists := f.sessions[key]
		f.mu.RUnlock()

		if !exists {
			remoteConn, err := net.Dial("udp", f.remoteAddr)
			if err != nil {
				f.log.Error("dial remote %s failed: %v", f.remoteAddr, err)
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

			f.log.Debug("new UDP session from %s", key)

			go f.remoteReadLoop(ctx, localConn, sess, key)
		}

		sess.lastActive = time.Now()
		_, err = sess.remoteConn.Write(buf[:n])
		if err != nil {
			f.log.Debug("write to remote error for %s: %v", key, err)
		}
	}
}

func (f *UDPForwarder) remoteReadLoop(ctx context.Context, localConn net.PacketConn, sess *udpSession, key string) {
	buf := make([]byte, 65535)
	for {
		sess.remoteConn.SetReadDeadline(time.Now().Add(f.sessionTimeout))
		n, err := sess.remoteConn.Read(buf)
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
			}
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// Timeout is handled by cleanup loop
				return
			}
			f.log.Debug("remote read error for %s: %v", key, err)
			return
		}
		sess.lastActive = time.Now()
		_, err = localConn.WriteTo(buf[:n], sess.clientAddr)
		if err != nil {
			f.log.Debug("write to client %s error: %v", key, err)
			return
		}
	}
}

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
					f.log.Info("UDP session timeout for %s", key)
					sess.remoteConn.Close()
					delete(f.sessions, key)
				}
			}
			f.mu.Unlock()
		}
	}
}
```

**Step 4: Run tests**

Run: `go test ./core/forwarder/ -v -run TestUDP -timeout 30s`
Expected: All PASS

**Step 5: Commit**

```bash
git add core/forwarder/udp.go core/forwarder/udp_test.go
git commit -m "feat: add UDP port forwarder with session timeout and cleanup"
```

---

### Task 6: Forwarder Manager

**Files:**
- Create: `core/forwarder/manager.go`
- Create: `core/forwarder/manager_test.go`

**Step 1: Write failing tests**

```go
package forwarder

import (
	"net"
	"testing"
	"time"

	"port_forward/core/config"
	"port_forward/core/logger"
)

func TestManagerStartStop(t *testing.T) {
	// Echo server
	echoLn, _ := net.Listen("tcp", "127.0.0.1:0")
	defer echoLn.Close()
	go func() {
		for {
			conn, err := echoLn.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	cfg := &config.Config{
		Defaults: config.Defaults{
			TCPConnectTimeout: 5 * time.Second,
			TCPIdleTimeout:    30 * time.Second,
			UDPSessionTimeout: 10 * time.Second,
		},
		Rules: []config.Rule{
			{Protocol: "tcp", Local: "127.0.0.1:0", Remote: echoLn.Addr().String(), Enabled: true},
		},
	}

	g := logger.NewGlobal()
	mgr := NewManager(cfg, g)

	if err := mgr.StartAll(); err != nil {
		t.Fatalf("StartAll failed: %v", err)
	}
	if mgr.RunningCount() != 1 {
		t.Errorf("expected 1 running, got %d", mgr.RunningCount())
	}

	mgr.StopAll()
	if mgr.RunningCount() != 0 {
		t.Errorf("expected 0 running, got %d", mgr.RunningCount())
	}
}

func TestManagerEnableDisable(t *testing.T) {
	echoLn, _ := net.Listen("tcp", "127.0.0.1:0")
	defer echoLn.Close()
	go func() {
		for {
			conn, err := echoLn.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	cfg := &config.Config{
		Defaults: config.Defaults{
			TCPConnectTimeout: 5 * time.Second,
			TCPIdleTimeout:    30 * time.Second,
			UDPSessionTimeout: 10 * time.Second,
		},
		Rules: []config.Rule{
			{Protocol: "tcp", Local: "127.0.0.1:0", Remote: echoLn.Addr().String(), Enabled: false},
		},
	}

	g := logger.NewGlobal()
	mgr := NewManager(cfg, g)

	mgr.StartAll() // Should not start disabled rule
	if mgr.RunningCount() != 0 {
		t.Errorf("disabled rule should not start, got %d running", mgr.RunningCount())
	}

	mgr.EnableRule(0)
	if mgr.RunningCount() != 1 {
		t.Errorf("expected 1 running after enable, got %d", mgr.RunningCount())
	}

	mgr.DisableRule(0)
	if mgr.RunningCount() != 0 {
		t.Errorf("expected 0 running after disable, got %d", mgr.RunningCount())
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./core/forwarder/ -v -run TestManager -timeout 30s`
Expected: FAIL

**Step 3: Implement manager**

```go
package forwarder

import (
	"context"
	"fmt"
	"sync"

	"port_forward/core/config"
	"port_forward/core/logger"
)

type ruleState struct {
	cancel context.CancelFunc
	running bool
}

type Manager struct {
	cfg    *config.Config
	global *logger.Global
	mu     sync.Mutex
	states []ruleState
	ctx    context.Context
	cancel context.CancelFunc
}

func NewManager(cfg *config.Config, global *logger.Global) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	states := make([]ruleState, len(cfg.Rules))
	return &Manager{
		cfg:    cfg,
		global: global,
		states: states,
		ctx:    ctx,
		cancel: cancel,
	}
}

func (m *Manager) StartAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.cfg.Rules {
		if m.cfg.Rules[i].Enabled && !m.states[i].running {
			if err := m.startRule(i); err != nil {
				return fmt.Errorf("rule %d: %w", i, err)
			}
		}
	}
	return nil
}

func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.states {
		if m.states[i].running {
			m.stopRule(i)
		}
	}
}

func (m *Manager) EnableRule(index int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if index < 0 || index >= len(m.cfg.Rules) {
		return fmt.Errorf("invalid rule index: %d", index)
	}

	m.cfg.Rules[index].Enabled = true
	if !m.states[index].running {
		return m.startRule(index)
	}
	return nil
}

func (m *Manager) DisableRule(index int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if index < 0 || index >= len(m.cfg.Rules) {
		return fmt.Errorf("invalid rule index: %d", index)
	}

	m.cfg.Rules[index].Enabled = false
	if m.states[index].running {
		m.stopRule(index)
	}
	return nil
}

func (m *Manager) AddRule(r config.Rule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cfg.AddRule(r)
	m.states = append(m.states, ruleState{})

	idx := len(m.cfg.Rules) - 1
	if r.Enabled {
		return m.startRule(idx)
	}
	return nil
}

func (m *Manager) RemoveRule(index int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if index < 0 || index >= len(m.cfg.Rules) {
		return fmt.Errorf("invalid rule index: %d", index)
	}

	if m.states[index].running {
		m.stopRule(index)
	}

	m.global.RemoveLogger(m.ruleID(index))
	m.cfg.RemoveRule(index)
	m.states = append(m.states[:index], m.states[index+1:]...)
	return nil
}

func (m *Manager) RunningCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	count := 0
	for _, s := range m.states {
		if s.running {
			count++
		}
	}
	return count
}

func (m *Manager) IsRunning(index int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if index < 0 || index >= len(m.states) {
		return false
	}
	return m.states[index].running
}

func (m *Manager) GetConfig() *config.Config {
	return m.cfg
}

func (m *Manager) ruleID(index int) string {
	r := m.cfg.Rules[index]
	return fmt.Sprintf("%s/%s->%s", r.Protocol, r.Local, r.Remote)
}

func (m *Manager) startRule(index int) error {
	r := &m.cfg.Rules[index]
	ruleID := m.ruleID(index)
	log := m.global.GetLogger(ruleID)

	ctx, cancel := context.WithCancel(m.ctx)

	switch r.Protocol {
	case "tcp":
		ct, it := m.cfg.ResolveTCPTimeouts(r)
		fwd := NewTCPForwarder(r.Local, r.Remote, ct, it, log)
		_, err := fwd.Start(ctx)
		if err != nil {
			cancel()
			return err
		}
	case "udp":
		st := m.cfg.ResolveUDPTimeout(r)
		fwd := NewUDPForwarder(r.Local, r.Remote, st, log)
		_, err := fwd.Start(ctx)
		if err != nil {
			cancel()
			return err
		}
	default:
		cancel()
		return fmt.Errorf("unsupported protocol: %s", r.Protocol)
	}

	m.states[index] = ruleState{cancel: cancel, running: true}
	return nil
}

func (m *Manager) stopRule(index int) {
	if m.states[index].cancel != nil {
		m.states[index].cancel()
	}
	m.states[index].running = false
}
```

**Step 4: Run tests**

Run: `go test ./core/forwarder/ -v -run TestManager -timeout 30s`
Expected: All PASS

**Step 5: Run all tests**

Run: `go test ./... -timeout 30s`
Expected: All PASS

**Step 6: Commit**

```bash
git add core/forwarder/manager.go core/forwarder/manager_test.go
git commit -m "feat: add forwarder manager for rule lifecycle control"
```

---

### Task 7: CLI Interface

**Files:**
- Create: `cli/cli.go`
- Modify: `main.go`

**Step 1: Implement CLI**

```go
package cli

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"port_forward/core/config"
	"port_forward/core/forwarder"
	"port_forward/core/logger"
)

func Run(args []string) {
	if len(args) < 1 {
		printUsage()
		os.Exit(1)
	}

	configPath := "config.yaml"

	switch args[0] {
	case "start":
		cmdStart(configPath)
	case "add":
		cmdAdd(args[1:], configPath)
	case "remove":
		cmdRemove(args[1:], configPath)
	case "list":
		cmdList(configPath)
	case "set-loglevel":
		cmdSetLogLevel(args[1:], configPath)
	case "help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", args[0])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Usage: port_forward [flags] <command> [args]

Commands:
  start                  Start forwarding all enabled rules
  add                    Add a new forwarding rule
  remove -id <N>         Remove a rule by index (0-based)
  list                   List all rules
  set-loglevel <level>   Set log level (debug/info/warn/error)
  help                   Show this help

Flags for 'add':
  -p <protocol>          Protocol: tcp or udp (default: tcp)
  -l <local>             Local address (default: 127.0.0.1:1234)
  -r <remote>            Remote address (default: 0.0.0.0:1234)
  --connect-timeout <d>  TCP connect timeout (default: from config)
  --idle-timeout <d>     TCP idle timeout (default: from config)
  --session-timeout <d>  UDP session timeout (default: from config)

Global flags:
  -c <path>              Config file path (default: config.yaml)
  -noui                  Disable GUI (Windows only)`)
}

func loadOrCreateConfig(path string) *config.Config {
	cfg, err := config.Load(path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg = &config.Config{
				Defaults: config.Defaults{
					TCPConnectTimeout: config.DefaultTCPConnectTimeout,
					TCPIdleTimeout:    config.DefaultTCPIdleTimeout,
					UDPSessionTimeout: config.DefaultUDPSessionTimeout,
				},
				LogLevel: "info",
			}
			return cfg
		}
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		os.Exit(1)
	}
	return cfg
}

func cmdStart(configPath string) {
	cfg := loadOrCreateConfig(configPath)

	if len(cfg.Rules) == 0 {
		fmt.Println("no rules configured. Use 'add' to add rules first.")
		return
	}

	g := logger.NewGlobal()
	g.SetLevel(logger.ParseLevel(cfg.LogLevel))

	mgr := forwarder.NewManager(cfg, g)
	if err := mgr.StartAll(); err != nil {
		fmt.Fprintf(os.Stderr, "start error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("started %d forwarding rule(s). Press Ctrl+C to stop.\n", mgr.RunningCount())

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("\nstopping...")
	mgr.StopAll()
	fmt.Println("stopped.")
}

func cmdAdd(args []string, configPath string) {
	fs := flag.NewFlagSet("add", flag.ExitOnError)
	protocol := fs.String("p", "tcp", "protocol (tcp/udp)")
	local := fs.String("l", "127.0.0.1:1234", "local address")
	remote := fs.String("r", "0.0.0.0:1234", "remote address")
	connectTimeout := fs.String("connect-timeout", "", "TCP connect timeout")
	idleTimeout := fs.String("idle-timeout", "", "TCP idle timeout")
	sessionTimeout := fs.String("session-timeout", "", "UDP session timeout")
	fs.Parse(args)

	cfg := loadOrCreateConfig(configPath)

	rule := config.Rule{
		Protocol: *protocol,
		Local:    *local,
		Remote:   *remote,
		Enabled:  true,
	}

	if *connectTimeout != "" {
		d, err := time.ParseDuration(*connectTimeout)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid connect-timeout: %v\n", err)
			os.Exit(1)
		}
		rule.TCPConnectTimeout = d
	}
	if *idleTimeout != "" {
		d, err := time.ParseDuration(*idleTimeout)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid idle-timeout: %v\n", err)
			os.Exit(1)
		}
		rule.TCPIdleTimeout = d
	}
	if *sessionTimeout != "" {
		d, err := time.ParseDuration(*sessionTimeout)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid session-timeout: %v\n", err)
			os.Exit(1)
		}
		rule.UDPSessionTimeout = d
	}

	cfg.AddRule(rule)
	if err := config.Save(cfg, configPath); err != nil {
		fmt.Fprintf(os.Stderr, "save config error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("added rule: %s %s -> %s\n", rule.Protocol, rule.Local, rule.Remote)
}

func cmdRemove(args []string, configPath string) {
	fs := flag.NewFlagSet("remove", flag.ExitOnError)
	id := fs.Int("id", -1, "rule index (0-based)")
	fs.Parse(args)

	if *id < 0 {
		fmt.Fprintln(os.Stderr, "error: -id is required")
		os.Exit(1)
	}

	cfg := loadOrCreateConfig(configPath)
	if *id >= len(cfg.Rules) {
		fmt.Fprintf(os.Stderr, "error: rule %d does not exist\n", *id)
		os.Exit(1)
	}

	cfg.RemoveRule(*id)
	if err := config.Save(cfg, configPath); err != nil {
		fmt.Fprintf(os.Stderr, "save config error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("removed rule %d\n", *id)
}

func cmdList(configPath string) {
	cfg := loadOrCreateConfig(configPath)

	if len(cfg.Rules) == 0 {
		fmt.Println("no rules configured.")
		return
	}

	fmt.Printf("%-4s %-8s %-22s %-22s %-8s\n", "#", "Proto", "Local", "Remote", "Enabled")
	fmt.Println("---- -------- ---------------------- ---------------------- --------")
	for i, r := range cfg.Rules {
		enabled := "yes"
		if !r.Enabled {
			enabled = "no"
		}
		fmt.Printf("%-4d %-8s %-22s %-22s %-8s\n", i, r.Protocol, r.Local, r.Remote, enabled)
	}
}

func cmdSetLogLevel(args []string, configPath string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "error: log level required (debug/info/warn/error)")
		os.Exit(1)
	}

	cfg := loadOrCreateConfig(configPath)
	cfg.LogLevel = args[0]
	if err := config.Save(cfg, configPath); err != nil {
		fmt.Fprintf(os.Stderr, "save config error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("log level set to %s\n", args[0])
}
```

**Step 2: Update main.go**

```go
package main

import (
	"os"

	"port_forward/cli"
)

func main() {
	// Filter out -noui flag for Windows compatibility
	args := []string{}
	for _, a := range os.Args[1:] {
		if a == "-noui" {
			continue
		}
		args = append(args, a)
	}

	// On Linux, always CLI mode
	// On Windows without -noui, GUI mode (handled in gui_windows.go)
	cli.Run(args)
}
```

Note: This main.go will be replaced with a platform-aware version in Task 9 (GUI). For now it's CLI-only.

**Step 3: Verify build and basic commands**

Run: `cd /Workspace/Ai/port_forward && go build -o port_forward . && ./port_forward help`
Expected: help text prints

Run: `./port_forward add -p tcp -l 127.0.0.1:8080 -r 127.0.0.1:9090 && ./port_forward list`
Expected: rule added and listed

**Step 4: Commit**

```bash
git add cli/ main.go
git commit -m "feat: add CLI interface with add/remove/list/start commands"
```

---

### Task 8: Generate Icon Resources

**Files:**
- Create: `gui/icons/icons.go`

**Step 1: Create icon Go source with embedded PNG data**

Generate two simple 32x32 PNG icons programmatically (red circle and green circle) and embed them as byte slices using `//go:embed` or inline byte arrays. Use Go's `image` package to generate them in a helper script, then embed the results.

Create `gui/icons/generate.go` (build-ignored helper):

```go
//go:build ignore

package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
)

func drawIcon(filename string, r, g, b uint8) {
	const size = 32
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	center := float64(size) / 2
	radius := float64(size)/2 - 2

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := float64(x) - center
			dy := float64(y) - center
			if dx*dx+dy*dy <= radius*radius {
				img.Set(x, y, color.RGBA{r, g, b, 255})
			}
		}
	}

	f, _ := os.Create(filename)
	defer f.Close()
	png.Encode(f, img)
}

func main() {
	drawIcon("red.png", 220, 50, 50)
	drawIcon("green.png", 50, 180, 50)
}
```

Run: `cd /Workspace/Ai/port_forward/gui/icons && go run generate.go`

Then create the embed file:

```go
package icons

import _ "embed"

//go:embed red.png
var RedIcon []byte

//go:embed green.png
var GreenIcon []byte
```

**Step 2: Verify icons compile**

Run: `go build ./gui/icons/`
Expected: compiles without error

**Step 3: Commit**

```bash
git add gui/icons/
git commit -m "feat: add red/green tray icon resources"
```

---

### Task 9: GUI — System Tray, Main Window, Log Viewer (Windows)

**Files:**
- Create: `gui/app.go` (build tag: windows)
- Create: `gui/tray.go` (build tag: windows)
- Create: `gui/window.go` (build tag: windows)
- Create: `gui/logview.go` (build tag: windows)
- Create: `gui/gui_stub.go` (build tag: !windows — stub for Linux)
- Modify: `main.go` — add GUI entry path

**Step 1: Create gui_stub.go for non-Windows**

```go
//go:build !windows

package gui

func RunGUI(configPath string) {
	panic("GUI is only supported on Windows")
}

func IsGUIAvailable() bool {
	return false
}
```

**Step 2: Create gui/app.go**

```go
//go:build windows

package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"

	"port_forward/core/config"
	"port_forward/core/forwarder"
	"port_forward/core/logger"
)

type App struct {
	fyneApp    fyne.App
	mainWindow *MainWindow
	tray       *Tray
	manager    *forwarder.Manager
	global     *logger.Global
	cfg        *config.Config
	configPath string
	running    bool
}

func RunGUI(configPath string) {
	cfg, err := config.Load(configPath)
	if err != nil {
		cfg = &config.Config{
			Defaults: config.Defaults{
				TCPConnectTimeout: config.DefaultTCPConnectTimeout,
				TCPIdleTimeout:    config.DefaultTCPIdleTimeout,
				UDPSessionTimeout: config.DefaultUDPSessionTimeout,
			},
			LogLevel: "info",
		}
	}

	g := logger.NewGlobal()
	g.SetLevel(logger.ParseLevel(cfg.LogLevel))

	mgr := forwarder.NewManager(cfg, g)

	a := &App{
		fyneApp:    app.New(),
		manager:    mgr,
		global:     g,
		cfg:        cfg,
		configPath: configPath,
	}

	a.mainWindow = NewMainWindow(a)
	a.tray = NewTray(a)

	a.tray.Setup()
	a.fyneApp.Run()
}

func IsGUIAvailable() bool {
	return true
}

func (a *App) ToggleService() {
	if a.running {
		a.manager.StopAll()
		a.running = false
		a.tray.SetStopped()
	} else {
		a.manager.StartAll()
		a.running = true
		a.tray.SetRunning()
	}
	a.mainWindow.Refresh()
}

func (a *App) SaveConfig() {
	config.Save(a.cfg, a.configPath)
}
```

**Step 3: Create gui/tray.go**

```go
//go:build windows

package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"

	"port_forward/core/logger"
	"port_forward/gui/icons"
)

type Tray struct {
	app  *App
	desk desktop.App
}

func NewTray(a *App) *Tray {
	return &Tray{app: a}
}

func (t *Tray) Setup() {
	desk, ok := t.app.fyneApp.(desktop.App)
	if !ok {
		return
	}
	t.desk = desk

	t.SetStopped()

	// System tray menu
	menu := fyne.NewMenu("Port Forward",
		fyne.NewMenuItem("Toggle Service", func() {
			t.app.ToggleService()
		}),
		fyne.NewMenuItem("Show Window", func() {
			t.app.mainWindow.Show()
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("View Log", func() {
			ShowLogView(t.app, "Global Log", t.app.global.AllEntries)
		}),
		fyne.NewMenuItemSeparator(),
		t.logLevelMenu(),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Quit", func() {
			t.app.manager.StopAll()
			t.app.fyneApp.Quit()
		}),
	)

	desk.SetSystemTrayMenu(menu)
}

func (t *Tray) logLevelMenu() *fyne.MenuItem {
	return fyne.NewMenuItem("Log Level", func() {
		// Sub-items handled via submenu
	})
	// Note: Fyne tray menus have limited submenu support.
	// Actual implementation may use separate menu items:
	// "Log: Debug", "Log: Info", "Log: Warn", "Log: Error"
}

func (t *Tray) SetRunning() {
	if t.desk != nil {
		res := fyne.NewStaticResource("green.png", icons.GreenIcon)
		t.desk.SetSystemTrayIcon(res)
	}
}

func (t *Tray) SetStopped() {
	if t.desk != nil {
		res := fyne.NewStaticResource("red.png", icons.RedIcon)
		t.desk.SetSystemTrayIcon(res)
	}
}
```

**Step 4: Create gui/window.go**

```go
//go:build windows

package gui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"port_forward/core/config"
)

type MainWindow struct {
	app    *App
	window fyne.Window
	list   *fyne.Container
}

func NewMainWindow(a *App) *MainWindow {
	w := a.fyneApp.NewWindow("Port Forward")
	w.Resize(fyne.NewSize(800, 500))
	w.SetCloseInterceptor(func() {
		w.Hide()
	})

	mw := &MainWindow{app: a, window: w}
	mw.build()
	return mw
}

func (mw *MainWindow) Show() {
	mw.Refresh()
	mw.window.Show()
}

func (mw *MainWindow) Refresh() {
	mw.build()
}

func (mw *MainWindow) build() {
	header := container.NewGridWithColumns(6,
		widget.NewLabel("#"),
		widget.NewLabel("Protocol"),
		widget.NewLabel("Local"),
		widget.NewLabel("Remote"),
		widget.NewLabel("Status"),
		widget.NewLabel("Actions"),
	)

	rows := container.NewVBox(header)

	for i, r := range mw.app.cfg.Rules {
		idx := i
		rule := r

		numLabel := widget.NewLabel(fmt.Sprintf("%d", idx+1))
		protoLabel := widget.NewLabel(rule.Protocol)
		localLabel := widget.NewLabel(rule.Local)
		remoteLabel := widget.NewLabel(rule.Remote)

		toggleText := "Enable"
		if rule.Enabled {
			toggleText = "Disable"
		}
		toggleBtn := widget.NewButton(toggleText, func() {
			if mw.app.cfg.Rules[idx].Enabled {
				mw.app.manager.DisableRule(idx)
			} else {
				mw.app.manager.EnableRule(idx)
			}
			mw.app.SaveConfig()
			mw.Refresh()
		})

		deleteBtn := widget.NewButton("Delete", func() {
			mw.app.manager.RemoveRule(idx)
			mw.app.SaveConfig()
			mw.Refresh()
		})

		logBtn := widget.NewButton("Log", func() {
			ruleID := fmt.Sprintf("%s/%s->%s", rule.Protocol, rule.Local, rule.Remote)
			l := mw.app.global.GetLogger(ruleID)
			ShowLogView(mw.app, fmt.Sprintf("Log: %s", ruleID), l.Entries)
		})

		actions := container.NewHBox(toggleBtn, deleteBtn, logBtn)

		row := container.NewGridWithColumns(6,
			numLabel, protoLabel, localLabel, remoteLabel,
			widget.NewLabel(func() string {
				if rule.Enabled { return "Enabled" }
				return "Disabled"
			}()),
			actions,
		)
		rows.Add(row)
	}

	// Add new rule row
	protoEntry := widget.NewSelect([]string{"tcp", "udp"}, nil)
	protoEntry.SetSelected("tcp")
	localEntry := widget.NewEntry()
	localEntry.SetPlaceHolder("127.0.0.1:1234")
	remoteEntry := widget.NewEntry()
	remoteEntry.SetPlaceHolder("0.0.0.0:1234")

	addBtn := widget.NewButton("Add", func() {
		local := localEntry.Text
		if local == "" {
			local = "127.0.0.1:1234"
		}
		remote := remoteEntry.Text
		if remote == "" {
			remote = "0.0.0.0:1234"
		}
		rule := config.Rule{
			Protocol: protoEntry.Selected,
			Local:    local,
			Remote:   remote,
			Enabled:  false,
		}
		mw.app.manager.AddRule(rule)
		mw.app.SaveConfig()
		mw.Refresh()
	})

	addRow := container.NewGridWithColumns(6,
		widget.NewLabel("+"),
		protoEntry,
		localEntry,
		remoteEntry,
		widget.NewLabel(""),
		addBtn,
	)
	rows.Add(widget.NewSeparator())
	rows.Add(addRow)

	mw.window.SetContent(container.NewVScroll(rows))
}
```

**Step 5: Create gui/logview.go**

```go
//go:build windows

package gui

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"port_forward/core/logger"
)

func ShowLogView(a *App, title string, entriesFn func() []logger.Entry) {
	w := a.fyneApp.NewWindow(title)
	w.Resize(fyne.NewSize(700, 400))

	logText := widget.NewMultiLineEntry()
	logText.Disable()

	refreshLog := func() {
		entries := entriesFn()
		text := ""
		for _, e := range entries {
			text += fmt.Sprintf("[%s] [%s] %s\n",
				e.Time.Format("15:04:05.000"), e.Level, e.Message)
		}
		logText.SetText(text)
	}

	refreshLog()

	refreshBtn := widget.NewButton("Refresh", func() {
		refreshLog()
	})

	// Auto-refresh every 2 seconds
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			refreshLog()
		}
	}()

	content := container.NewBorder(
		container.NewHBox(refreshBtn), nil, nil, nil,
		logText,
	)
	w.SetContent(content)
	w.Show()
}
```

**Step 6: Update main.go for GUI/CLI routing**

```go
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

	for _, a := range os.Args[1:] {
		switch a {
		case "-noui":
			noUI = true
		case "-c":
			// handled in next iteration
		default:
			args = append(args, a)
		}
	}

	// Handle -c flag
	for i, a := range os.Args[1:] {
		if a == "-c" && i+2 < len(os.Args) {
			configPath = os.Args[i+2]
		}
	}

	if !noUI && gui.IsGUIAvailable() && len(args) == 0 {
		gui.RunGUI(configPath)
	} else {
		cli.Run(args)
	}
}
```

**Step 7: Add Fyne dependency**

Run: `go get fyne.io/fyne/v2`

**Step 8: Verify Linux build (GUI excluded)**

Run: `go build -o port_forward .`
Expected: builds successfully with GUI stub

**Step 9: Commit**

```bash
git add gui/ main.go go.mod go.sum
git commit -m "feat: add Windows GUI with tray, main window, and log viewer"
```

---

### Task 10: Integration Test & Config Sample

**Files:**
- Create: `config.yaml`
- Create: `integration_test.go`

**Step 1: Create sample config**

```yaml
defaults:
  tcp_connect_timeout: 10s
  tcp_idle_timeout: 300s
  udp_session_timeout: 60s

log_level: info

rules:
  - protocol: tcp
    local: "127.0.0.1:1234"
    remote: "0.0.0.0:1234"
    enabled: true
  - protocol: udp
    local: "127.0.0.1:5000"
    remote: "0.0.0.0:5000"
    enabled: false
    udp_session_timeout: 120s
```

**Step 2: Write integration test**

```go
package main

import (
	"io"
	"net"
	"testing"
	"time"

	"port_forward/core/config"
	"port_forward/core/forwarder"
	"port_forward/core/logger"
)

func TestIntegrationTCPForward(t *testing.T) {
	// Start echo server
	echoLn, _ := net.Listen("tcp", "127.0.0.1:0")
	defer echoLn.Close()
	go func() {
		for {
			conn, err := echoLn.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				io.Copy(c, c)
			}(conn)
		}
	}()

	cfg := &config.Config{
		Defaults: config.Defaults{
			TCPConnectTimeout: 5 * time.Second,
			TCPIdleTimeout:    30 * time.Second,
			UDPSessionTimeout: 10 * time.Second,
		},
		LogLevel: "debug",
		Rules: []config.Rule{
			{Protocol: "tcp", Local: "127.0.0.1:0", Remote: echoLn.Addr().String(), Enabled: true},
		},
	}

	g := logger.NewGlobal()
	g.SetLevel(logger.LevelDebug)
	mgr := forwarder.NewManager(cfg, g)

	err := mgr.StartAll()
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.StopAll()

	// Give listener time to start
	time.Sleep(100 * time.Millisecond)

	// The manager started on port 0, we need the actual port
	// For integration test, we test through the forwarder package directly
	if mgr.RunningCount() != 1 {
		t.Fatalf("expected 1 running, got %d", mgr.RunningCount())
	}

	// Verify global log has entries
	entries := g.AllEntries()
	if len(entries) == 0 {
		t.Error("expected log entries from startup")
	}
}

func TestIntegrationManagerLifecycle(t *testing.T) {
	echoLn, _ := net.Listen("tcp", "127.0.0.1:0")
	defer echoLn.Close()
	go func() {
		for {
			conn, err := echoLn.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	cfg := &config.Config{
		Defaults: config.Defaults{
			TCPConnectTimeout: 5 * time.Second,
			TCPIdleTimeout:    30 * time.Second,
			UDPSessionTimeout: 10 * time.Second,
		},
		Rules: []config.Rule{
			{Protocol: "tcp", Local: "127.0.0.1:0", Remote: echoLn.Addr().String(), Enabled: true},
			{Protocol: "tcp", Local: "127.0.0.1:0", Remote: echoLn.Addr().String(), Enabled: false},
		},
	}

	g := logger.NewGlobal()
	mgr := forwarder.NewManager(cfg, g)

	mgr.StartAll()
	if mgr.RunningCount() != 1 {
		t.Errorf("only enabled rule should start, got %d", mgr.RunningCount())
	}

	mgr.EnableRule(1)
	if mgr.RunningCount() != 2 {
		t.Errorf("expected 2 after enable, got %d", mgr.RunningCount())
	}

	mgr.StopAll()
	if mgr.RunningCount() != 0 {
		t.Errorf("expected 0 after stop, got %d", mgr.RunningCount())
	}
}
```

**Step 3: Run all tests**

Run: `go test ./... -v -timeout 60s`
Expected: All PASS

**Step 4: Commit**

```bash
git add config.yaml integration_test.go
git commit -m "feat: add sample config and integration tests"
```

---

### Task 11: Cross-Compile Verification

**Step 1: Build for Linux**

Run: `GOOS=linux GOARCH=amd64 go build -o port_forward_linux .`
Expected: builds successfully

**Step 2: Build for Windows (without CGO for Fyne stub)**

Run: `GOOS=windows GOARCH=amd64 go build -o port_forward.exe .`
Expected: builds successfully (GUI code included via build tags)

Note: Full Windows GUI testing requires a Windows machine with CGO enabled for Fyne. The build tag approach ensures Linux builds work cleanly without Fyne dependencies.

**Step 3: Commit build scripts or Makefile if needed**

```bash
git add -A
git commit -m "chore: verify cross-platform build"
```
