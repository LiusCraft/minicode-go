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

func NewOpenAIClient(apiKey, baseURL string) (*OpenAIClient, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("missing OpenAI API key")
	}

	requestOptions := []option.RequestOption{option.WithAPIKey(apiKey)}
	if baseURL := strings.TrimSpace(baseURL); baseURL != "" {
		requestOptions = append(requestOptions, option.WithBaseURL(baseURL))
	}

	return &OpenAIClient{
		client: openai.NewClient(requestOptions...),
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

	return c.runStreaming(ctx, params, req.Stream)
}

func (c *OpenAIClient) runStreaming(ctx context.Context, params openai.ChatCompletionNewParams, handler *StreamHandler) (Result, error) {
	streamParams := params
	streamParams.StreamOptions = openai.ChatCompletionStreamOptionsParam{
		IncludeUsage: openai.Bool(true),
	}

	stream := c.client.Chat.Completions.NewStreaming(ctx, streamParams)
	acc := openai.ChatCompletionAccumulator{}
	sawChunk := false
	defer emitMessageDone(handler)

	for stream.Next() {
		sawChunk = true
		chunk := stream.Current()
		if !acc.AddChunk(chunk) {
			return Result{}, fmt.Errorf("failed to accumulate streamed chat completion")
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		emitTextDelta(handler, chunk.Choices[0].Delta)
	}

	if err := stream.Err(); err != nil {
		if !sawChunk {
			return c.runNonStreaming(ctx, params, handler)
		}
		return Result{}, err
	}
	if len(acc.Choices) == 0 {
		return Result{}, fmt.Errorf("chat completion returned no choices")
	}

	return buildResult(acc.ID, acc.Choices[0].Message, acc.ChatCompletion), nil
}

func (c *OpenAIClient) runNonStreaming(ctx context.Context, params openai.ChatCompletionNewParams, handler *StreamHandler) (Result, error) {
	completion, err := c.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return Result{}, err
	}
	if len(completion.Choices) == 0 {
		return Result{}, fmt.Errorf("chat completion returned no choices")
	}

	result := buildResult(completion.ID, completion.Choices[0].Message, completion)
	if handler != nil {
		if text := result.Text; text != "" && handler.OnTextDelta != nil {
			handler.OnTextDelta(text)
		}
		emitMessageDone(handler)
	}
	return result, nil
}

func buildResult(responseID string, message openai.ChatCompletionMessage, raw any) Result {
	return Result{
		ResponseID: responseID,
		Text:       renderAssistantText(message),
		ToolCalls:  collectToolCalls(message.ToolCalls),
		Raw:        raw,
	}
}

func renderAssistantText(message openai.ChatCompletionMessage) string {
	if text := strings.TrimSpace(message.Content); text != "" {
		return text
	}
	return strings.TrimSpace(message.Refusal)
}

func collectToolCalls(items []openai.ChatCompletionMessageToolCallUnion) []ToolCall {
	result := make([]ToolCall, 0, len(items))
	for _, item := range items {
		if item.Type != "function" || item.Function.Name == "" {
			continue
		}
		result = append(result, ToolCall{
			ID:        item.ID,
			Name:      item.Function.Name,
			Arguments: []byte(item.Function.Arguments),
		})
	}
	return result
}

func emitTextDelta(handler *StreamHandler, delta openai.ChatCompletionChunkChoiceDelta) {
	if handler == nil || handler.OnTextDelta == nil {
		return
	}
	if delta.Content != "" {
		handler.OnTextDelta(delta.Content)
	}
	if delta.Refusal != "" {
		handler.OnTextDelta(delta.Refusal)
	}
}

func emitMessageDone(handler *StreamHandler) {
	if handler == nil || handler.OnMessageDone == nil {
		return
	}
	handler.OnMessageDone()
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
