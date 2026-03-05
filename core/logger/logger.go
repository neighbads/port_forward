// Package logger provides a ring-buffer based logging system with per-rule
// and global log aggregation. It is safe for concurrent use.
package logger

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// ringSize is the maximum number of entries kept in each ring buffer.
const ringSize = 1000

// Level represents a log severity level.
type Level int

const (
	Debug Level = iota
	Info
	Warn
	Error
)

// String returns the human-readable name of the level.
func (l Level) String() string {
	switch l {
	case Debug:
		return "DEBUG"
	case Info:
		return "INFO"
	case Warn:
		return "WARN"
	case Error:
		return "ERROR"
	default:
		return fmt.Sprintf("LEVEL(%d)", int(l))
	}
}

// ParseLevel converts a case-insensitive string to a Level.
func ParseLevel(s string) (Level, error) {
	switch strings.ToUpper(s) {
	case "DEBUG":
		return Debug, nil
	case "INFO":
		return Info, nil
	case "WARN":
		return Warn, nil
	case "ERROR":
		return Error, nil
	default:
		return Debug, fmt.Errorf("unknown log level: %q", s)
	}
}

// Entry is a single log record.
type Entry struct {
	Time    time.Time
	Level   Level
	RuleID  string
	Message string
}

// String returns a formatted representation of the entry.
func (e Entry) String() string {
	return fmt.Sprintf("%s [%s] [%s] %s",
		e.Time.Format(time.RFC3339), e.Level, e.RuleID, e.Message)
}

// ringBuffer is a fixed-size circular buffer of log entries.
type ringBuffer struct {
	buf   [ringSize]Entry
	pos   int  // next write position
	full  bool // whether the buffer has wrapped around
}

func (r *ringBuffer) add(e Entry) {
	r.buf[r.pos] = e
	r.pos++
	if r.pos >= ringSize {
		r.pos = 0
		r.full = true
	}
}

// entries returns all stored entries in chronological order.
func (r *ringBuffer) entries() []Entry {
	if !r.full {
		out := make([]Entry, r.pos)
		copy(out, r.buf[:r.pos])
		return out
	}
	out := make([]Entry, ringSize)
	copy(out, r.buf[r.pos:])
	copy(out[ringSize-r.pos:], r.buf[:r.pos])
	return out
}

// Logger is a per-rule logger backed by a ring buffer.
type Logger struct {
	mu     sync.RWMutex
	ruleID string
	level  Level
	ring   ringBuffer
	global *globalLogger
}

// SetLevel sets the minimum log level for this logger.
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	l.level = level
	l.mu.Unlock()
}

// GetLevel returns the current minimum log level.
func (l *Logger) GetLevel() Level {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.level
}

// Debug logs a message at Debug level.
func (l *Logger) Debug(format string, args ...any) {
	l.log(Debug, format, args...)
}

// Info logs a message at Info level.
func (l *Logger) Info(format string, args ...any) {
	l.log(Info, format, args...)
}

// Warn logs a message at Warn level.
func (l *Logger) Warn(format string, args ...any) {
	l.log(Warn, format, args...)
}

// Error logs a message at Error level.
func (l *Logger) Error(format string, args ...any) {
	l.log(Error, format, args...)
}

func (l *Logger) log(level Level, format string, args ...any) {
	l.mu.Lock()
	if level < l.level {
		l.mu.Unlock()
		return
	}
	e := Entry{
		Time:    time.Now(),
		Level:   level,
		RuleID:  l.ruleID,
		Message: fmt.Sprintf(format, args...),
	}
	l.ring.add(e)
	l.mu.Unlock()

	// Append to global ring buffer.
	l.global.addEntry(e)
}

// Entries returns all stored entries in chronological order.
func (l *Logger) Entries() []Entry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.ring.entries()
}

// globalLogger manages per-rule loggers and a global ring buffer.
type globalLogger struct {
	mu      sync.RWMutex
	loggers map[string]*Logger
	level   Level
	ring    ringBuffer
}

var global = &globalLogger{
	loggers: make(map[string]*Logger),
	level:   Info,
}

func (g *globalLogger) addEntry(e Entry) {
	g.mu.Lock()
	g.ring.add(e)
	g.mu.Unlock()
}

// GetLogger returns the Logger for the given ruleID, creating one if needed.
func GetLogger(ruleID string) *Logger {
	global.mu.Lock()
	defer global.mu.Unlock()

	if l, ok := global.loggers[ruleID]; ok {
		return l
	}
	l := &Logger{
		ruleID: ruleID,
		level:  global.level,
		global: global,
	}
	global.loggers[ruleID] = l
	return l
}

// RemoveLogger removes the per-rule logger for the given ruleID.
func RemoveLogger(ruleID string) {
	global.mu.Lock()
	delete(global.loggers, ruleID)
	global.mu.Unlock()
}

// SetLevel sets the minimum log level on all existing and future loggers.
func SetLevel(level Level) {
	global.mu.Lock()
	global.level = level
	for _, l := range global.loggers {
		l.mu.Lock()
		l.level = level
		l.mu.Unlock()
	}
	global.mu.Unlock()
}

// GetLevel returns the current global log level.
func GetLevel() Level {
	global.mu.RLock()
	defer global.mu.RUnlock()
	return global.level
}

// AllEntries returns all entries from the global ring buffer in chronological order.
func AllEntries() []Entry {
	global.mu.RLock()
	defer global.mu.RUnlock()
	return global.ring.entries()
}

// ResetGlobal resets the global logger state. Intended for use in tests.
func ResetGlobal() {
	global.mu.Lock()
	global.loggers = make(map[string]*Logger)
	global.level = Info
	global.ring = ringBuffer{}
	global.mu.Unlock()
}
