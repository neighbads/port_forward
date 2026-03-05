package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

const fullYAML = `defaults:
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

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(fullYAML), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Check defaults
	if cfg.Defaults.TCPConnectTimeout != 10*time.Second {
		t.Errorf("TCPConnectTimeout = %v, want 10s", cfg.Defaults.TCPConnectTimeout)
	}
	if cfg.Defaults.TCPIdleTimeout != 300*time.Second {
		t.Errorf("TCPIdleTimeout = %v, want 300s", cfg.Defaults.TCPIdleTimeout)
	}
	if cfg.Defaults.UDPSessionTimeout != 60*time.Second {
		t.Errorf("UDPSessionTimeout = %v, want 60s", cfg.Defaults.UDPSessionTimeout)
	}

	// Check log level
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "info")
	}

	// Check rules
	if len(cfg.Rules) != 2 {
		t.Fatalf("len(Rules) = %d, want 2", len(cfg.Rules))
	}

	r0 := cfg.Rules[0]
	if r0.Protocol != "tcp" {
		t.Errorf("Rules[0].Protocol = %q, want %q", r0.Protocol, "tcp")
	}
	if r0.Local != "127.0.0.1:1234" {
		t.Errorf("Rules[0].Local = %q, want %q", r0.Local, "127.0.0.1:1234")
	}
	if r0.Remote != "10.0.0.1:5678" {
		t.Errorf("Rules[0].Remote = %q, want %q", r0.Remote, "10.0.0.1:5678")
	}
	if !r0.Enabled {
		t.Error("Rules[0].Enabled = false, want true")
	}

	r1 := cfg.Rules[1]
	if r1.Protocol != "udp" {
		t.Errorf("Rules[1].Protocol = %q, want %q", r1.Protocol, "udp")
	}
	if r1.Enabled {
		t.Error("Rules[1].Enabled = true, want false")
	}
	// Rule-level override
	if r1.UDPSessionTimeout == nil {
		t.Fatal("Rules[1].UDPSessionTimeout is nil, want 120s")
	}
	if time.Duration(*r1.UDPSessionTimeout) != 120*time.Second {
		t.Errorf("Rules[1].UDPSessionTimeout = %v, want 120s", time.Duration(*r1.UDPSessionTimeout))
	}
}

func TestSaveConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	original := &Config{
		Defaults: Defaults{
			TCPConnectTimeout: 10 * time.Second,
			TCPIdleTimeout:    300 * time.Second,
			UDPSessionTimeout: 60 * time.Second,
		},
		LogLevel: "debug",
		Rules: []Rule{
			{
				Protocol: "tcp",
				Local:    "127.0.0.1:8080",
				Remote:   "10.0.0.1:80",
				Enabled:  true,
			},
		},
	}

	if err := Save(original, path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load after Save failed: %v", err)
	}

	if reloaded.Defaults.TCPConnectTimeout != original.Defaults.TCPConnectTimeout {
		t.Errorf("TCPConnectTimeout roundtrip: got %v, want %v",
			reloaded.Defaults.TCPConnectTimeout, original.Defaults.TCPConnectTimeout)
	}
	if reloaded.Defaults.TCPIdleTimeout != original.Defaults.TCPIdleTimeout {
		t.Errorf("TCPIdleTimeout roundtrip: got %v, want %v",
			reloaded.Defaults.TCPIdleTimeout, original.Defaults.TCPIdleTimeout)
	}
	if reloaded.Defaults.UDPSessionTimeout != original.Defaults.UDPSessionTimeout {
		t.Errorf("UDPSessionTimeout roundtrip: got %v, want %v",
			reloaded.Defaults.UDPSessionTimeout, original.Defaults.UDPSessionTimeout)
	}
	if reloaded.LogLevel != original.LogLevel {
		t.Errorf("LogLevel roundtrip: got %q, want %q", reloaded.LogLevel, original.LogLevel)
	}
	if len(reloaded.Rules) != 1 {
		t.Fatalf("len(Rules) roundtrip: got %d, want 1", len(reloaded.Rules))
	}
	if reloaded.Rules[0].Protocol != "tcp" || reloaded.Rules[0].Local != "127.0.0.1:8080" {
		t.Errorf("Rule roundtrip mismatch: %+v", reloaded.Rules[0])
	}
}

func TestDefaultValues(t *testing.T) {
	// YAML with no defaults section
	yaml := `log_level: warn
rules:
  - protocol: tcp
    local: "0.0.0.0:2222"
    remote: "192.168.1.1:22"
    enabled: true
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Defaults.TCPConnectTimeout != DefaultTCPConnectTimeout {
		t.Errorf("TCPConnectTimeout = %v, want %v", cfg.Defaults.TCPConnectTimeout, DefaultTCPConnectTimeout)
	}
	if cfg.Defaults.TCPIdleTimeout != DefaultTCPIdleTimeout {
		t.Errorf("TCPIdleTimeout = %v, want %v", cfg.Defaults.TCPIdleTimeout, DefaultTCPIdleTimeout)
	}
	if cfg.Defaults.UDPSessionTimeout != DefaultUDPSessionTimeout {
		t.Errorf("UDPSessionTimeout = %v, want %v", cfg.Defaults.UDPSessionTimeout, DefaultUDPSessionTimeout)
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

	t.Run("uses global defaults when no override", func(t *testing.T) {
		rule := Rule{Protocol: "tcp", Local: "0.0.0.0:1234", Remote: "10.0.0.1:5678", Enabled: true}
		conn, idle := cfg.ResolveTCPTimeouts(rule)
		if conn != 10*time.Second {
			t.Errorf("connect timeout = %v, want 10s", conn)
		}
		if idle != 300*time.Second {
			t.Errorf("idle timeout = %v, want 300s", idle)
		}

		sess := cfg.ResolveUDPTimeout(rule)
		if sess != 60*time.Second {
			t.Errorf("session timeout = %v, want 60s", sess)
		}
	})

	t.Run("rule override takes precedence", func(t *testing.T) {
		connOverride := Duration(5 * time.Second)
		idleOverride := Duration(100 * time.Second)
		udpOverride := Duration(120 * time.Second)

		rule := Rule{
			Protocol:          "tcp",
			Local:             "0.0.0.0:1234",
			Remote:            "10.0.0.1:5678",
			Enabled:           true,
			TCPConnectTimeout: &connOverride,
			TCPIdleTimeout:    &idleOverride,
			UDPSessionTimeout: &udpOverride,
		}

		conn, idle := cfg.ResolveTCPTimeouts(rule)
		if conn != 5*time.Second {
			t.Errorf("connect timeout = %v, want 5s", conn)
		}
		if idle != 100*time.Second {
			t.Errorf("idle timeout = %v, want 100s", idle)
		}

		sess := cfg.ResolveUDPTimeout(rule)
		if sess != 120*time.Second {
			t.Errorf("session timeout = %v, want 120s", sess)
		}
	})
}

func TestAddRule(t *testing.T) {
	cfg := &Config{}
	rule := Rule{Protocol: "tcp", Local: "0.0.0.0:80", Remote: "10.0.0.1:80", Enabled: true}
	cfg.AddRule(rule)
	if len(cfg.Rules) != 1 {
		t.Fatalf("len(Rules) = %d, want 1", len(cfg.Rules))
	}
	if cfg.Rules[0].Local != "0.0.0.0:80" {
		t.Errorf("Rules[0].Local = %q, want %q", cfg.Rules[0].Local, "0.0.0.0:80")
	}
}

func TestRemoveRule(t *testing.T) {
	cfg := &Config{
		Rules: []Rule{
			{Protocol: "tcp", Local: "a", Remote: "b", Enabled: true},
			{Protocol: "udp", Local: "c", Remote: "d", Enabled: false},
			{Protocol: "tcp", Local: "e", Remote: "f", Enabled: true},
		},
	}

	if err := cfg.RemoveRule(1); err != nil {
		t.Fatalf("RemoveRule(1) failed: %v", err)
	}
	if len(cfg.Rules) != 2 {
		t.Fatalf("len(Rules) = %d, want 2", len(cfg.Rules))
	}
	if cfg.Rules[0].Local != "a" || cfg.Rules[1].Local != "e" {
		t.Errorf("unexpected rules after remove: %+v", cfg.Rules)
	}

	// Out of bounds
	if err := cfg.RemoveRule(5); err == nil {
		t.Error("RemoveRule(5) should fail for out-of-bounds index")
	}
}
