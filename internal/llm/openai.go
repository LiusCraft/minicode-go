package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

type OpenAIClient struct {
	client openai.Client
}

func NewOpenAIClient(apiKey string) (*OpenAIClient, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("missing OpenAI API key")
	}

	return &OpenAIClient{
		client: openai.NewClient(option.WithAPIKey(apiKey)),
	}, nil
}

func (c *OpenAIClient) Run(ctx context.Context, req Request) (Result, error) {
	if len(req.Messages) == 0 {
		return Result{}, fmt.Errorf("request messages are empty")
	}

	params := openai.ChatCompletionNewParams{
		Model:             openai.ChatModel(req.Model),
		Messages:          buildMessages(req.Instructions, req.Messages),
		Tools:             buildTools(req.Tools),
		ParallelToolCalls: openai.Bool(false),
	}

	completion, err := c.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return Result{}, err
	}
	if len(completion.Choices) == 0 {
		return Result{}, fmt.Errorf("chat completion returned no choices")
	}

	choice := completion.Choices[0]
	message := choice.Message
	result := Result{
		ResponseID: completion.ID,
		Text:       strings.TrimSpace(message.Content),
		Raw:        completion,
	}

	for _, item := range message.ToolCalls {
		functionCall, ok := item.AsAny().(openai.ChatCompletionMessageFunctionToolCall)
		if !ok {
			continue
		}
		result.ToolCalls = append(result.ToolCalls, ToolCall{
			ID:        functionCall.ID,
			Name:      functionCall.Function.Name,
			Arguments: []byte(functionCall.Function.Arguments),
		})
	}

	return result, nil
}

func buildMessages(instructions string, messages []Message) []openai.ChatCompletionMessageParamUnion {
	result := make([]openai.ChatCompletionMessageParamUnion, 0, len(messages)+1)
	if strings.TrimSpace(instructions) != "" {
		result = append(result, openai.SystemMessage(instructions))
	}

	for _, message := range messages {
		switch message.Role {
		case "user":
			result = append(result, openai.UserMessage(message.Content))
		case "assistant":
			result = append(result, assistantMessage(message))
		case "tool":
			result = append(result, openai.ToolMessage(message.Content, message.ToolCallID))
		}
	}

	return result
}

func assistantMessage(message Message) openai.ChatCompletionMessageParamUnion {
	assistant := openai.ChatCompletionAssistantMessageParam{}
	if strings.TrimSpace(message.Content) != "" {
		assistant.Content.OfString = openai.String(message.Content)
	}
	if len(message.ToolCalls) > 0 {
		assistant.ToolCalls = make([]openai.ChatCompletionMessageToolCallUnionParam, 0, len(message.ToolCalls))
		for _, call := range message.ToolCalls {
			assistant.ToolCalls = append(assistant.ToolCalls, openai.ChatCompletionMessageToolCallUnionParam{
				OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
					ID: call.ID,
					Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
						Name:      call.Name,
						Arguments: string(call.Arguments),
					},
				},
			})
		}
	}
	return openai.ChatCompletionMessageParamUnion{OfAssistant: &assistant}
}

func buildTools(defs []ToolDefinition) []openai.ChatCompletionToolUnionParam {
	if len(defs) == 0 {
		return nil
	}

	tools := make([]openai.ChatCompletionToolUnionParam, 0, len(defs))
	for _, def := range defs {
		tools = append(tools, openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:        def.Name,
			Description: openai.String(def.Description),
			Parameters:  def.Parameters,
		}))
	}
	return tools
}
