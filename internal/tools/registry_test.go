package tools

import (
	"context"
	"encoding/json"
	"testing"

	"minioc/internal/safety"
)

func TestNewRegistryOrdersAlphabetically(t *testing.T) {
	registry := NewRegistry(
		Spec{Name: "zebra"},
		Spec{Name: "apple"},
		Spec{Name: "banana"},
	)

	if len(registry.order) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(registry.order))
	}
	want := []string{"apple", "banana", "zebra"}
	for i, name := range registry.order {
		if name != want[i] {
			t.Errorf("order[%d]: got %q, want %q", i, name, want[i])
		}
	}
}

func TestRegistryDefinitions(t *testing.T) {
	registry := NewRegistry(
		Spec{
			Name:        "greet",
			Description: "Say hello",
			Parameters:  map[string]any{"type": "object"},
		},
		Spec{Name: "farewell", Description: "Say goodbye"},
	)

	defs := registry.Definitions()
	if len(defs) != 2 {
		t.Fatalf("expected 2 definitions, got %d", len(defs))
	}
	// alphabetical order
	if defs[0].Name != "farewell" {
		t.Errorf("defs[0].Name: got %q, want %q", defs[0].Name, "farewell")
	}
	if defs[1].Name != "greet" {
		t.Errorf("defs[1].Name: got %q, want %q", defs[1].Name, "greet")
	}
	if defs[1].Description != "Say hello" {
		t.Errorf("defs[1].Description: got %q", defs[1].Description)
	}
}

func TestRegistryExecuteKnownTool(t *testing.T) {
	registry := NewRegistry(
		Spec{
			Name: "echo",
			Execute: func(_ context.Context, _ CallContext, args json.RawMessage) (Result, error) {
				return Result{Output: string(args)}, nil
			},
		},
	)

	result, err := registry.Execute(context.TODO(), "echo", json.RawMessage(`"hello"`), CallContext{
		Permissions: safety.NewPermissionManager(nil, nil, true),
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.Output != `"hello"` {
		t.Errorf("Output: got %q, want %q", result.Output, `"hello"`)
	}
}

func TestRegistryExecuteUnknownTool(t *testing.T) {
	registry := NewRegistry()

	_, err := registry.Execute(context.TODO(), "nonexistent", json.RawMessage(`{}`), CallContext{})
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
	if err.Error() != `unknown tool "nonexistent"` {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRegistryIsParallelSafe(t *testing.T) {
	registry := NewRegistry(
		Spec{Name: "read_file", ParallelSafe: true},
		Spec{Name: "write_file", ParallelSafe: false},
	)

	if !registry.IsParallelSafe("read_file") {
		t.Error("read_file should be parallel safe")
	}
	if registry.IsParallelSafe("write_file") {
		t.Error("write_file should not be parallel safe")
	}
	if registry.IsParallelSafe("nonexistent") {
		t.Error("nonexistent tool should not be parallel safe")
	}
}

// ─── objectSchema ──────────────────────────────────────────────────────────────

func TestObjectSchema(t *testing.T) {
	schema := objectSchema(
		map[string]any{"name": map[string]any{"type": "string"}},
		"name",
	)

	if schema["type"] != "object" {
		t.Errorf("type: got %v", schema["type"])
	}
	if schema["additionalProperties"] != false {
		t.Error("additionalProperties should be false")
	}
	required, ok := schema["required"].([]string)
	if !ok {
		t.Fatal("required should be []string")
	}
	if len(required) != 1 || required[0] != "name" {
		t.Errorf("required: got %v", required)
	}
}

func TestObjectSchemaNoRequired(t *testing.T) {
	schema := objectSchema(map[string]any{"opt": map[string]any{"type": "string"}})
	if _, ok := schema["required"]; ok {
		t.Error("required should not be present when none specified")
	}
}
