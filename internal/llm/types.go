package llm

import (
	"context"
	"encoding/json"
)

type ToolDefinition struct {
	Name        string
	Description string
	Parameters  map[string]any
}

type ToolCall struct {
	ID        string
	Name      string
	Arguments json.RawMessage
}

type ToolOutput struct {
	CallID string
	Output string
}

type Message struct {
	Role       string
	Content    string
	ToolName   string
	ToolCallID string
	ToolCalls  []ToolCall
}

type Request struct {
	Model        string
	Instructions string
	Messages     []Message
	Tools        []ToolDefinition
}

type Result struct {
	ResponseID string
	Text       string
	ToolCalls  []ToolCall
	Raw        any
}

type Client interface {
	Run(ctx context.Context, req Request) (Result, error)
}
