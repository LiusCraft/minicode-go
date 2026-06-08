package mcp

import (
	"encoding/json"
	"testing"

	"minioc/internal/tools"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if len(m.servers) != 0 {
		t.Errorf("expected empty servers, got %d", len(m.servers))
	}
}

func TestEnviron(t *testing.T) {
	env := environ(map[string]string{"KEY": "value", "FOO": "bar"})
	if len(env) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(env))
	}
	m := make(map[string]string)
	for _, e := range env {
		parts := splitEnv(e)
		m[parts[0]] = parts[1]
	}
	if m["KEY"] != "value" {
		t.Errorf("KEY: got %q", m["KEY"])
	}
	if m["FOO"] != "bar" {
		t.Errorf("FOO: got %q", m["FOO"])
	}
}

func splitEnv(s string) [2]string {
	for i := 0; i < len(s); i++ {
		if s[i] == '=' {
			return [2]string{s[:i], s[i+1:]}
		}
	}
	return [2]string{s, ""}
}

func TestToolSchemaBasic(t *testing.T) {
	tool := &mcp.Tool{
		Name:        "test",
		Description: "a test tool",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
			},
			"required": []any{"name"},
		},
	}
	schema := toolSchema(tool)
	if schema["type"] != "object" {
		t.Errorf("expected type object, got %v", schema["type"])
	}
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("properties not a map")
	}
	if _, ok := props["name"]; !ok {
		t.Error("missing name property")
	}
}

func TestToolSchemaNil(t *testing.T) {
	tool := &mcp.Tool{Name: "test"}
	schema := toolSchema(tool)
	if schema["type"] != "object" {
		t.Errorf("expected type object for nil schema, got %v", schema["type"])
	}
}

func TestToolSchemaFromRawMessage(t *testing.T) {
	raw := json.RawMessage(`{"type":"object","properties":{"x":{"type":"number"}}}`)
	tool := &mcp.Tool{
		Name:        "calc",
		InputSchema: raw,
	}
	schema := toolSchema(tool)
	if schema["type"] != "object" {
		t.Errorf("expected object, got %v", schema["type"])
	}
}

func TestMCPResultToToolsResult(t *testing.T) {
	result := &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "hello"},
			&mcp.TextContent{Text: "world"},
		},
	}
	tr := mcpResultToToolsResult(result)
	if tr.Output != "hello\nworld" {
		t.Errorf("expected 'hello\\nworld', got %q", tr.Output)
	}
}

func TestMCPResultToToolsResultError(t *testing.T) {
	result := &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			&mcp.TextContent{Text: "something went wrong"},
		},
	}
	tr := mcpResultToToolsResult(result)
	if tr.Output != "ERROR: something went wrong" {
		t.Errorf("expected ERROR prefix, got %q", tr.Output)
	}
}

func TestMCPResultToToolsResultErrorEmpty(t *testing.T) {
	result := &mcp.CallToolResult{
		IsError: true,
		Content: nil,
	}
	tr := mcpResultToToolsResult(result)
	if tr.Output != "ERROR: tool call failed" {
		t.Errorf("expected default error, got %q", tr.Output)
	}
}

// Ensure tools package compiles alongside mcp
var _ = []tools.Spec{}
