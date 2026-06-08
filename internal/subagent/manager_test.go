package subagent

import (
	"testing"

	"minioc/internal/config"
)

func TestNewManager(t *testing.T) {
	m := NewManager(nil, nil, nil, nil, nil, 10000, "/r", "/r", "gpt-4", "")
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
	if len(m.AgentConfigs()) != 0 {
		t.Errorf("expected empty configs")
	}
}

func TestManagerAgentConfigs(t *testing.T) {
	cfgs := map[string]config.AgentConfig{
		"explore": {Description: "x", MaxSteps: 5},
	}
	m := NewManager(nil, nil, nil, cfgs, nil, 10000, "/r", "/r", "gpt-4", "")
	if len(m.AgentConfigs()) != 1 {
		t.Errorf("expected 1 config")
	}
}

func TestManagerAgentsEmpty(t *testing.T) {
	m := NewManager(nil, nil, nil, nil, nil, 10000, "/r", "/r", "gpt-4", "")
	agents := m.Agents()
	if len(agents) != 0 {
		t.Errorf("expected empty, got %d", len(agents))
	}
}

func TestManagerKillNotFound(t *testing.T) {
	m := NewManager(nil, nil, nil, nil, nil, 10000, "/r", "/r", "gpt-4", "")
	err := m.Kill("nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveMaxSteps(t *testing.T) {
	m := NewManager(nil, nil, nil, nil, nil, 10000, "/r", "/r", "gpt-4", "")
	if v := m.resolveMaxSteps(config.AgentConfig{MaxSteps: 5}, 20); v != 20 {
		t.Errorf("call param: got %d", v)
	}
	if v := m.resolveMaxSteps(config.AgentConfig{MaxSteps: 5}, 0); v != 5 {
		t.Errorf("cfg: got %d", v)
	}
	if v := m.resolveMaxSteps(config.AgentConfig{}, 0); v != 10000 {
		t.Errorf("default: got %d", v)
	}
}
