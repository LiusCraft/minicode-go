package provider

import (
	"context"

	"minioc/internal/llm"
	"minioc/internal/llm/models"
)

type ToolDefinition = llm.ToolDefinition
type ToolCall = llm.ToolCall
type Message = llm.Message
type Request = llm.Request
type Result = llm.Result

type Adapter interface {
	Chat(ctx context.Context, model models.Model, req llm.Request) (llm.Result, error)
	Models(ctx context.Context) ([]string, error)
}
