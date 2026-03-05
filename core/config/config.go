package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Default timeout constants.
const (
	DefaultTCPConnectTimeout = 10 * time.Second
	DefaultTCPIdleTimeout    = 300 * time.Second
	DefaultUDPSessionTimeout = 60 * time.Second
)

// Duration wraps time.Duration for human-readable YAML marshaling ("10s", "300s").
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
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	*d = Duration(parsed)
	return nil
}

// Defaults holds global timeout defaults.
type Defaults struct {
	TCPConnectTimeout time.Duration `yaml:"-"`
	TCPIdleTimeout    time.Duration `yaml:"-"`
	UDPSessionTimeout time.Duration `yaml:"-"`

	// Intermediate fields for YAML marshaling.
	RawTCPConnectTimeout *Duration `yaml:"tcp_connect_timeout,omitempty"`
	RawTCPIdleTimeout    *Duration `yaml:"tcp_idle_timeout,omitempty"`
	RawUDPSessionTimeout *Duration `yaml:"udp_session_timeout,omitempty"`
}

func (d *Defaults) applyFromRaw() {
	if d.RawTCPConnectTimeout != nil {
		d.TCPConnectTimeout = time.Duration(*d.RawTCPConnectTimeout)
	}
	if d.RawTCPIdleTimeout != nil {
		d.TCPIdleTimeout = time.Duration(*d.RawTCPIdleTimeout)
	}
	if d.RawUDPSessionTimeout != nil {
		d.UDPSessionTimeout = time.Duration(*d.RawUDPSessionTimeout)
	}
}

func (d *Defaults) prepareRaw() {
	ct := Duration(d.TCPConnectTimeout)
	d.RawTCPConnectTimeout = &ct
	it := Duration(d.TCPIdleTimeout)
	d.RawTCPIdleTimeout = &it
	ut := Duration(d.UDPSessionTimeout)
	d.RawUDPSessionTimeout = &ut
}

// Rule represents a single port-forwarding rule.
type Rule struct {
	Protocol string `yaml:"protocol"`
	Local    string `yaml:"local"`
	Remote   string `yaml:"remote"`
	Enabled  bool   `yaml:"enabled"`

	// Optional per-rule timeout overrides. Nil means "use global default".
	TCPConnectTimeout *Duration `yaml:"tcp_connect_timeout,omitempty"`
	TCPIdleTimeout    *Duration `yaml:"tcp_idle_timeout,omitempty"`
	UDPSessionTimeout *Duration `yaml:"udp_session_timeout,omitempty"`
}

// Config is the top-level configuration.
type Config struct {
	Defaults Defaults `yaml:"defaults"`
	LogLevel string   `yaml:"log_level"`
	Rules    []Rule   `yaml:"rules"`
}

// Load reads a YAML config file and applies defaults for any zero-value timeouts.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	cfg.Defaults.applyFromRaw()

	// Apply defaults for zero values.
	if cfg.Defaults.TCPConnectTimeout == 0 {
		cfg.Defaults.TCPConnectTimeout = DefaultTCPConnectTimeout
	}
	if cfg.Defaults.TCPIdleTimeout == 0 {
		cfg.Defaults.TCPIdleTimeout = DefaultTCPIdleTimeout
	}
	if cfg.Defaults.UDPSessionTimeout == 0 {
		cfg.Defaults.UDPSessionTimeout = DefaultUDPSessionTimeout
	}

	return &cfg, nil
}

// Save marshals the config to YAML and writes it to the given path.
func Save(cfg *Config, path string) error {
	cfg.Defaults.prepareRaw()

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return nil
}

// ResolveTCPTimeouts returns the effective connect and idle timeouts for a rule,
// using the rule override if set, otherwise the global default.
func (c *Config) ResolveTCPTimeouts(rule Rule) (connectTimeout, idleTimeout time.Duration) {
	connectTimeout = c.Defaults.TCPConnectTimeout
	if rule.TCPConnectTimeout != nil {
		connectTimeout = time.Duration(*rule.TCPConnectTimeout)
	}

	idleTimeout = c.Defaults.TCPIdleTimeout
	if rule.TCPIdleTimeout != nil {
		idleTimeout = time.Duration(*rule.TCPIdleTimeout)
	}

	return
}

// ResolveUDPTimeout returns the effective session timeout for a rule,
// using the rule override if set, otherwise the global default.
func (c *Config) ResolveUDPTimeout(rule Rule) time.Duration {
	if rule.UDPSessionTimeout != nil {
		return time.Duration(*rule.UDPSessionTimeout)
	}
	return c.Defaults.UDPSessionTimeout
}

// AddRule appends a rule to the config.
func (c *Config) AddRule(rule Rule) {
	c.Rules = append(c.Rules, rule)
}

// RemoveRule removes the rule at the given index.
func (c *Config) RemoveRule(index int) error {
	if index < 0 || index >= len(c.Rules) {
		return fmt.Errorf("index %d out of range [0, %d)", index, len(c.Rules))
	}
	c.Rules = append(c.Rules[:index], c.Rules[index+1:]...)
	return nil
}
