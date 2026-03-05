package forwarder

import (
	"context"
	"fmt"
	"sync"

	"port_forward/core/config"
	"port_forward/core/logger"
)

// ruleState tracks the runtime state of a single forwarding rule.
type ruleState struct {
	cancel  context.CancelFunc
	running bool
}

// Manager orchestrates forwarding rules, starting and stopping forwarders
// based on the configuration.
type Manager struct {
	cfg    *config.Config
	mu     sync.Mutex
	states []ruleState
	ctx    context.Context
	cancel context.CancelFunc
}

// NewManager creates a new Manager for the given configuration.
func NewManager(cfg *config.Config) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		cfg:    cfg,
		states: make([]ruleState, len(cfg.Rules)),
		ctx:    ctx,
		cancel: cancel,
	}
}

// StartAll starts all enabled rules that are not already running.
func (m *Manager) StartAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, rule := range m.cfg.Rules {
		if rule.Enabled && !m.states[i].running {
			if err := m.startRule(i); err != nil {
				return fmt.Errorf("starting rule %d: %w", i, err)
			}
		}
	}
	return nil
}

// StopAll stops all running rules.
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.states {
		if m.states[i].running {
			m.stopRule(i)
		}
	}
}

// EnableRule enables the rule at the given index and starts it if not running.
func (m *Manager) EnableRule(index int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if index < 0 || index >= len(m.cfg.Rules) {
		return fmt.Errorf("index %d out of range [0, %d)", index, len(m.cfg.Rules))
	}

	m.cfg.Rules[index].Enabled = true
	if !m.states[index].running {
		return m.startRule(index)
	}
	return nil
}

// DisableRule disables the rule at the given index and stops it if running.
func (m *Manager) DisableRule(index int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if index < 0 || index >= len(m.cfg.Rules) {
		return fmt.Errorf("index %d out of range [0, %d)", index, len(m.cfg.Rules))
	}

	m.cfg.Rules[index].Enabled = false
	if m.states[index].running {
		m.stopRule(index)
	}
	return nil
}

// AddRule appends a rule to the configuration, extends the states slice, and
// starts the rule if it is enabled.
func (m *Manager) AddRule(rule config.Rule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cfg.AddRule(rule)
	m.states = append(m.states, ruleState{})

	if rule.Enabled {
		return m.startRule(len(m.states) - 1)
	}
	return nil
}

// RemoveRule stops the rule at the given index if running, removes its logger,
// and removes it from the configuration and states slice.
func (m *Manager) RemoveRule(index int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if index < 0 || index >= len(m.cfg.Rules) {
		return fmt.Errorf("index %d out of range [0, %d)", index, len(m.cfg.Rules))
	}

	if m.states[index].running {
		m.stopRule(index)
	}

	// Remove the per-rule logger.
	ruleID := m.ruleID(index)
	logger.RemoveLogger(ruleID)

	if err := m.cfg.RemoveRule(index); err != nil {
		return err
	}
	m.states = append(m.states[:index], m.states[index+1:]...)
	return nil
}

// RunningCount returns the number of currently running rules.
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

// IsRunning reports whether the rule at the given index is currently running.
func (m *Manager) IsRunning(index int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if index < 0 || index >= len(m.states) {
		return false
	}
	return m.states[index].running
}

// GetConfig returns the manager's configuration.
func (m *Manager) GetConfig() *config.Config {
	return m.cfg
}

// ruleID returns a string identifier for the rule at the given index.
func (m *Manager) ruleID(index int) string {
	rule := m.cfg.Rules[index]
	return fmt.Sprintf("%s/%s->%s", rule.Protocol, rule.Local, rule.Remote)
}

// startRule starts the forwarder for the rule at the given index.
// The caller must hold m.mu.
func (m *Manager) startRule(index int) error {
	rule := m.cfg.Rules[index]
	ruleID := m.ruleID(index)
	log := logger.GetLogger(ruleID)

	childCtx, childCancel := context.WithCancel(m.ctx)

	switch rule.Protocol {
	case "tcp":
		connectTimeout, idleTimeout := m.cfg.ResolveTCPTimeouts(rule)
		fwd := NewTCPForwarder(rule.Local, rule.Remote, connectTimeout, idleTimeout, log)
		if _, err := fwd.Start(childCtx); err != nil {
			childCancel()
			return fmt.Errorf("starting TCP forwarder for %s: %w", ruleID, err)
		}
	case "udp":
		sessionTimeout := m.cfg.ResolveUDPTimeout(rule)
		fwd := NewUDPForwarder(rule.Local, rule.Remote, sessionTimeout, log)
		if _, err := fwd.Start(childCtx); err != nil {
			childCancel()
			return fmt.Errorf("starting UDP forwarder for %s: %w", ruleID, err)
		}
	default:
		childCancel()
		return fmt.Errorf("unsupported protocol %q for rule %s", rule.Protocol, ruleID)
	}

	m.states[index] = ruleState{
		cancel:  childCancel,
		running: true,
	}
	return nil
}

// stopRule stops the forwarder for the rule at the given index.
// The caller must hold m.mu.
func (m *Manager) stopRule(index int) {
	if m.states[index].cancel != nil {
		m.states[index].cancel()
	}
	m.states[index].running = false
}
