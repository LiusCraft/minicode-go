package notify

import (
	"testing"
)

func TestMatch(t *testing.T) {
	tests := []struct {
		pattern string
		typ     string
		want    bool
	}{
		{"*", "anything", true},
		{"mcp:*", "mcp:server_failed", true},
		{"mcp:*", "mcp:tools_changed", true},
		{"mcp:*", "system:shutdown", false},
		{"mcp:server_failed", "mcp:server_failed", true},
		{"mcp:server_failed", "mcp:tools_changed", false},
		{"*", "mcp:server_failed", true},
		{"mcp:*", "mcp:", true},
	}
	for _, tt := range tests {
		got := match(tt.pattern, tt.typ)
		if got != tt.want {
			t.Errorf("match(%q, %q): got %v, want %v", tt.pattern, tt.typ, got, tt.want)
		}
	}
}

func TestHubOnEmit(t *testing.T) {
	h := newHub()
	var called []Event

	h.On("mcp:*", func(e Event) {
		called = append(called, e)
	})

	h.On("system:*", func(e Event) {
		called = append(called, e)
	})

	h.Emit(Event{Type: "mcp:server_failed", Source: "fs-server", Payload: "boom"})
	h.Emit(Event{Type: "system:shutdown", Source: "app"})

	if len(called) != 2 {
		t.Fatalf("expected 2 events, got %d", len(called))
	}
	if called[0].Type != "mcp:server_failed" {
		t.Errorf("first event type: got %q", called[0].Type)
	}
	if called[1].Type != "system:shutdown" {
		t.Errorf("second event type: got %q", called[1].Type)
	}
}

func TestHubGlobal(t *testing.T) {
	var called bool
	On("foo", func(e Event) {
		called = true
	})
	Emit(Event{Type: "foo"})
	if !called {
		t.Error("global handler not called")
	}
}

func TestHubNoMatch(t *testing.T) {
	h := newHub()
	var called bool
	h.On("mcp:server_failed", func(e Event) {
		called = true
	})
	h.Emit(Event{Type: "mcp:tools_changed"})
	if called {
		t.Error("handler should not be called for non-matching event")
	}
}
