package logger

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

// --- Level tests ---

func TestLevelString(t *testing.T) {
	tests := []struct {
		level Level
		want  string
	}{
		{Debug, "DEBUG"},
		{Info, "INFO"},
		{Warn, "WARN"},
		{Error, "ERROR"},
	}
	for _, tt := range tests {
		if got := tt.level.String(); got != tt.want {
			t.Errorf("Level(%d).String() = %q, want %q", tt.level, got, tt.want)
		}
	}
}

func TestParseLevelValid(t *testing.T) {
	tests := []struct {
		input string
		want  Level
	}{
		{"debug", Debug},
		{"DEBUG", Debug},
		{"Debug", Debug},
		{"info", Info},
		{"INFO", Info},
		{"warn", Warn},
		{"WARN", Warn},
		{"error", Error},
		{"ERROR", Error},
	}
	for _, tt := range tests {
		got, err := ParseLevel(tt.input)
		if err != nil {
			t.Errorf("ParseLevel(%q) returned error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseLevel(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestParseLevelInvalid(t *testing.T) {
	_, err := ParseLevel("bogus")
	if err == nil {
		t.Error("ParseLevel(\"bogus\") should return error")
	}
}

// --- Entry tests ---

func TestEntryString(t *testing.T) {
	e := Entry{
		Level:   Info,
		RuleID:  "rule1",
		Message: "hello world",
	}
	s := e.String()
	if !strings.Contains(s, "INFO") {
		t.Errorf("Entry.String() missing level, got %q", s)
	}
	if !strings.Contains(s, "rule1") {
		t.Errorf("Entry.String() missing ruleID, got %q", s)
	}
	if !strings.Contains(s, "hello world") {
		t.Errorf("Entry.String() missing message, got %q", s)
	}
}

// --- Per-rule Logger tests ---

func TestLoggerBasicLevels(t *testing.T) {
	ResetGlobal()
	l := GetLogger("test-rule")
	l.SetLevel(Debug)

	l.Debug("debug msg %d", 1)
	l.Info("info msg %d", 2)
	l.Warn("warn msg %d", 3)
	l.Error("error msg %d", 4)

	entries := l.Entries()
	if len(entries) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(entries))
	}

	expected := []struct {
		level Level
		msg   string
	}{
		{Debug, "debug msg 1"},
		{Info, "info msg 2"},
		{Warn, "warn msg 3"},
		{Error, "error msg 4"},
	}

	for i, e := range expected {
		if entries[i].Level != e.level {
			t.Errorf("entry[%d].Level = %v, want %v", i, entries[i].Level, e.level)
		}
		if entries[i].Message != e.msg {
			t.Errorf("entry[%d].Message = %q, want %q", i, entries[i].Message, e.msg)
		}
		if entries[i].RuleID != "test-rule" {
			t.Errorf("entry[%d].RuleID = %q, want %q", i, entries[i].RuleID, "test-rule")
		}
	}
}

func TestLoggerLevelFiltering(t *testing.T) {
	ResetGlobal()
	l := GetLogger("filter-test")
	l.SetLevel(Warn)

	l.Debug("should not appear")
	l.Info("should not appear")
	l.Warn("should appear")
	l.Error("should appear")

	entries := l.Entries()
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Level != Warn {
		t.Errorf("entry[0].Level = %v, want Warn", entries[0].Level)
	}
	if entries[1].Level != Error {
		t.Errorf("entry[1].Level = %v, want Error", entries[1].Level)
	}
}

func TestLoggerGetLevel(t *testing.T) {
	ResetGlobal()
	l := GetLogger("level-test")
	l.SetLevel(Error)
	if l.GetLevel() != Error {
		t.Errorf("GetLevel() = %v, want Error", l.GetLevel())
	}
}

func TestLoggerRingBufferOverflow(t *testing.T) {
	ResetGlobal()
	l := GetLogger("overflow-test")
	l.SetLevel(Debug)

	for i := 0; i < 1100; i++ {
		l.Debug("msg %d", i)
	}

	entries := l.Entries()
	if len(entries) != 1000 {
		t.Fatalf("expected 1000 entries, got %d", len(entries))
	}

	// First entry should be msg 100 (oldest 100 were overwritten)
	if entries[0].Message != "msg 100" {
		t.Errorf("first entry = %q, want %q", entries[0].Message, "msg 100")
	}
	if entries[999].Message != "msg 1099" {
		t.Errorf("last entry = %q, want %q", entries[999].Message, "msg 1099")
	}
}

func TestLoggerRingBufferPartialFill(t *testing.T) {
	ResetGlobal()
	l := GetLogger("partial-test")
	l.SetLevel(Debug)

	for i := 0; i < 5; i++ {
		l.Info("msg %d", i)
	}

	entries := l.Entries()
	if len(entries) != 5 {
		t.Fatalf("expected 5 entries, got %d", len(entries))
	}
	if entries[0].Message != "msg 0" {
		t.Errorf("first entry = %q, want %q", entries[0].Message, "msg 0")
	}
}

// --- Global logger tests ---

func TestGlobalLoggerAggregation(t *testing.T) {
	ResetGlobal()

	l1 := GetLogger("rule-a")
	l1.SetLevel(Debug)
	l2 := GetLogger("rule-b")
	l2.SetLevel(Debug)

	l1.Info("from rule-a")
	l2.Info("from rule-b")
	l1.Warn("warn from rule-a")

	all := AllEntries()
	if len(all) != 3 {
		t.Fatalf("expected 3 global entries, got %d", len(all))
	}

	// Verify order
	if all[0].Message != "from rule-a" || all[0].RuleID != "rule-a" {
		t.Errorf("all[0] unexpected: %v", all[0])
	}
	if all[1].Message != "from rule-b" || all[1].RuleID != "rule-b" {
		t.Errorf("all[1] unexpected: %v", all[1])
	}
	if all[2].Message != "warn from rule-a" || all[2].RuleID != "rule-a" {
		t.Errorf("all[2] unexpected: %v", all[2])
	}
}

func TestGlobalLoggerRingBufferOverflow(t *testing.T) {
	ResetGlobal()
	l := GetLogger("global-overflow")
	l.SetLevel(Debug)

	for i := 0; i < 1100; i++ {
		l.Debug("gmsg %d", i)
	}

	all := AllEntries()
	if len(all) != 1000 {
		t.Fatalf("expected 1000 global entries, got %d", len(all))
	}
	if all[0].Message != "gmsg 100" {
		t.Errorf("first global entry = %q, want %q", all[0].Message, "gmsg 100")
	}
}

func TestGetLoggerReturnsSame(t *testing.T) {
	ResetGlobal()
	l1 := GetLogger("same")
	l2 := GetLogger("same")
	if l1 != l2 {
		t.Error("GetLogger should return same instance for same ruleID")
	}
}

func TestRemoveLogger(t *testing.T) {
	ResetGlobal()
	l := GetLogger("removeme")
	l.SetLevel(Debug)
	l.Info("before remove")

	RemoveLogger("removeme")

	// Getting it again should return a fresh logger
	l2 := GetLogger("removeme")
	entries := l2.Entries()
	if len(entries) != 0 {
		t.Errorf("expected 0 entries after remove, got %d", len(entries))
	}
}

func TestSetGlobalLevel(t *testing.T) {
	ResetGlobal()

	l1 := GetLogger("gl-a")
	l2 := GetLogger("gl-b")

	SetLevel(Error)

	if l1.GetLevel() != Error {
		t.Errorf("l1 level = %v, want Error", l1.GetLevel())
	}
	if l2.GetLevel() != Error {
		t.Errorf("l2 level = %v, want Error", l2.GetLevel())
	}
	if GetLevel() != Error {
		t.Errorf("GetLevel() = %v, want Error", GetLevel())
	}

	// Messages below Error should be filtered
	l1.Debug("nope")
	l1.Info("nope")
	l1.Warn("nope")
	l1.Error("yes")

	entries := l1.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Level != Error {
		t.Errorf("entry level = %v, want Error", entries[0].Level)
	}
}

func TestSetGlobalLevelAffectsNewLoggers(t *testing.T) {
	ResetGlobal()
	SetLevel(Warn)

	l := GetLogger("new-after-global")
	if l.GetLevel() != Warn {
		t.Errorf("new logger level = %v, want Warn", l.GetLevel())
	}
}

// --- Concurrency test ---

func TestConcurrentAccess(t *testing.T) {
	ResetGlobal()
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ruleID := fmt.Sprintf("rule-%d", id)
			l := GetLogger(ruleID)
			l.SetLevel(Debug)
			for j := 0; j < 100; j++ {
				l.Info("msg %d from %d", j, id)
			}
			entries := l.Entries()
			if len(entries) != 100 {
				t.Errorf("rule-%d: expected 100 entries, got %d", id, len(entries))
			}
		}(i)
	}
	wg.Wait()

	all := AllEntries()
	if len(all) != 1000 {
		t.Errorf("expected 1000 global entries, got %d", len(all))
	}
}

// --- Format string test ---

func TestFormatStrings(t *testing.T) {
	ResetGlobal()
	l := GetLogger("fmt-test")
	l.SetLevel(Debug)

	l.Debug("count: %d", 42)
	l.Info("name: %s", "test")
	l.Warn("rate: %.2f", 3.14)
	l.Error("flag: %v", true)

	entries := l.Entries()
	expected := []string{"count: 42", "name: test", "rate: 3.14", "flag: true"}
	for i, e := range expected {
		if entries[i].Message != e {
			t.Errorf("entry[%d].Message = %q, want %q", i, entries[i].Message, e)
		}
	}
}
