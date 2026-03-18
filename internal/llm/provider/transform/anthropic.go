package transform

import (
	"encoding/json"
	"fmt"
	"strings"

	anthropicsdk "github.com/anthropics/anthropic-sdk-go"

	"minioc/internal/llm"
	"minioc/internal/llm/models"
	"minioc/internal/llm/provider"
)

const defaultAnthropicMaxOutputTokens = 4096

func AnthropicMessageParams(model models.Model, req llm.Request) (anthropicsdk.MessageNewParams, error) {
	messages, err := anthropicMessages(req.Messages)
	if err != nil {
		return anthropicsdk.MessageNewParams{}, err
	}
	tools, err := anthropicTools(req.Tools)
	if err != nil {
		return anthropicsdk.MessageNewParams{}, err
	}

	maxTokens := defaultAnthropicMaxOutputTokens
	if model.MaxOutputTokens > 0 {
		maxTokens = model.MaxOutputTokens
	}

	params := anthropicsdk.MessageNewParams{
		Model:     anthropicsdk.Model(model.ID),
		MaxTokens: int64(maxTokens),
		Messages:  messages,
		Tools:     tools,
	}
	if strings.TrimSpace(req.Instructions) != "" {
		params.System = []anthropicsdk.TextBlockParam{{Text: req.Instructions}}
	}
	if model.Temperature != nil {
		params.Temperature = anthropicsdk.Float(*model.Temperature)
	}
	return params, nil
}

func anthropicMessages(messages []llm.Message) ([]anthropicsdk.MessageParam, error) {
	result := make([]anthropicsdk.MessageParam, 0, len(messages))

	for i := 0; i < len(messages); i++ {
		message := messages[i]
		switch message.Role {
		case "user":
			result = append(result, anthropicsdk.NewUserMessage(anthropicsdk.NewTextBlock(message.Content)))
		case "assistant":
			blocks, err := anthropicAssistantBlocks(message)
			if err != nil {
				return nil, err
			}
			if len(blocks) == 0 {
				continue
			}
			result = append(result, anthropicsdk.NewAssistantMessage(blocks...))
		case "tool":
			blocks := make([]anthropicsdk.ContentBlockParamUnion, 0, 1)
			for ; i < len(messages) && messages[i].Role == "tool"; i++ {
				toolMessage := messages[i]
				if strings.TrimSpace(toolMessage.ToolCallID) == "" {
					return nil, fmt.Errorf("tool message is missing tool call id")
				}
				blocks = append(blocks, anthropicsdk.NewToolResultBlock(toolMessage.ToolCallID, toolMessage.Content, toolMessage.Status == "error"))
			}
			i--
			if len(blocks) > 0 {
				result = append(result, anthropicsdk.NewUserMessage(blocks...))
			}
		}
	}

	return result, nil
}

func anthropicAssistantBlocks(message llm.Message) ([]anthropicsdk.ContentBlockParamUnion, error) {
	blocks := make([]anthropicsdk.ContentBlockParamUnion, 0, len(message.ToolCalls)+1)
	if strings.TrimSpace(message.Content) != "" {
		blocks = append(blocks, anthropicsdk.NewTextBlock(message.Content))
	}
	for _, call := range message.ToolCalls {
		var input any
		if len(call.Arguments) > 0 {
			if err := json.Unmarshal(call.Arguments, &input); err != nil {
				return nil, fmt.Errorf("decode assistant tool call %q arguments: %w", call.Name, err)
			}
		}
		blocks = append(blocks, anthropicsdk.NewToolUseBlock(call.ID, input, call.Name))
	}
	return blocks, nil
}

func anthropicTools(defs []llm.ToolDefinition) ([]anthropicsdk.ToolUnionParam, error) {
	if len(defs) == 0 {
		return nil, nil
	}

	tools := make([]anthropicsdk.ToolUnionParam, 0, len(defs))
	for _, def := range defs {
		tool := anthropicsdk.ToolParam{
			Name:        def.Name,
			Description: anthropicsdk.String(def.Description),
			InputSchema: anthropicToolSchema(def.Parameters),
		}
		tools = append(tools, anthropicsdk.ToolUnionParam{OfTool: &tool})
	}
	return tools, nil
}

func anthropicToolSchema(schema map[string]any) anthropicsdk.ToolInputSchemaParam {
	cloned := cloneMap(schema)
	input := anthropicsdk.ToolInputSchemaParam{}
	if properties, ok := cloned["properties"]; ok {
		input.Properties = properties
		delete(cloned, "properties")
	}
	if required, ok := cloned["required"]; ok {
		input.Required = toStringSlice(required)
		delete(cloned, "required")
	}
	delete(cloned, "type")
	input.ExtraFields = cloned
	return input
}

func AnthropicResult(message *anthropicsdk.Message) provider.Result {
	result := provider.Result{
		ResponseID: message.ID,
		Raw:        message,
	}
	textParts := make([]string, 0, len(message.Content))

	for _, block := range message.Content {
		switch item := block.AsAny().(type) {
		case anthropicsdk.TextBlock:
			if text := strings.TrimSpace(item.Text); text != "" {
				textParts = append(textParts, text)
			}
		case anthropicsdk.ToolUseBlock:
			result.ToolCalls = append(result.ToolCalls, provider.ToolCall{
				ID:        item.ID,
				Name:      item.Name,
				Arguments: append(json.RawMessage(nil), item.Input...),
			})
		}
	}

	result.Text = strings.TrimSpace(strings.Join(textParts, "\n\n"))
	return result
}

func cloneMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(input))
	for key, value := range input {
		cloned[key] = value
	}
	return cloned
}

func toStringSlice(value any) []string {
	switch items := value.(type) {
	case []string:
		result := make([]string, len(items))
		copy(result, items)
		return result
	case []any:
		result := make([]string, 0, len(items))
		for _, item := range items {
			if text, ok := item.(string); ok {
				result = append(result, text)
			}
		}
		return result
	default:
		return nil
	}
}
