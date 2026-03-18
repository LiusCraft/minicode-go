package transform

import (
	"strings"

	openaisdk "github.com/openai/openai-go/v3"

	"minioc/internal/llm"
	"minioc/internal/llm/models"
	"minioc/internal/llm/provider"
)

func OpenAIChatParams(model models.Model, req llm.Request) openaisdk.ChatCompletionNewParams {
	params := openaisdk.ChatCompletionNewParams{
		Model:             openaisdk.ChatModel(model.ID),
		Messages:          openAIMessages(req.Instructions, req.Messages),
		Tools:             openAITools(req.Tools),
		ParallelToolCalls: openaisdk.Bool(len(req.Tools) > 0),
	}
	if model.Temperature != nil {
		params.Temperature = openaisdk.Float(*model.Temperature)
	}
	return params
}

func openAIMessages(instructions string, messages []llm.Message) []openaisdk.ChatCompletionMessageParamUnion {
	result := make([]openaisdk.ChatCompletionMessageParamUnion, 0, len(messages)+1)
	if strings.TrimSpace(instructions) != "" {
		result = append(result, openaisdk.SystemMessage(instructions))
	}

	for _, message := range messages {
		switch message.Role {
		case "user":
			result = append(result, openaisdk.UserMessage(message.Content))
		case "assistant":
			result = append(result, openAIAssistantMessage(message))
		case "tool":
			result = append(result, openaisdk.ToolMessage(message.Content, message.ToolCallID))
		}
	}

	return result
}

func openAIAssistantMessage(message llm.Message) openaisdk.ChatCompletionMessageParamUnion {
	assistant := openaisdk.ChatCompletionAssistantMessageParam{}
	if strings.TrimSpace(message.Content) != "" {
		assistant.Content.OfString = openaisdk.String(message.Content)
	}
	if len(message.ToolCalls) > 0 {
		assistant.ToolCalls = make([]openaisdk.ChatCompletionMessageToolCallUnionParam, 0, len(message.ToolCalls))
		for _, call := range message.ToolCalls {
			assistant.ToolCalls = append(assistant.ToolCalls, openaisdk.ChatCompletionMessageToolCallUnionParam{
				OfFunction: &openaisdk.ChatCompletionMessageFunctionToolCallParam{
					ID: call.ID,
					Function: openaisdk.ChatCompletionMessageFunctionToolCallFunctionParam{
						Name:      call.Name,
						Arguments: string(call.Arguments),
					},
				},
			})
		}
	}
	return openaisdk.ChatCompletionMessageParamUnion{OfAssistant: &assistant}
}

func openAITools(defs []llm.ToolDefinition) []openaisdk.ChatCompletionToolUnionParam {
	if len(defs) == 0 {
		return nil
	}

	tools := make([]openaisdk.ChatCompletionToolUnionParam, 0, len(defs))
	for _, def := range defs {
		tools = append(tools, openaisdk.ChatCompletionFunctionTool(openaisdk.FunctionDefinitionParam{
			Name:        def.Name,
			Description: openaisdk.String(def.Description),
			Parameters:  def.Parameters,
		}))
	}
	return tools
}

func OpenAIResult(completion *openaisdk.ChatCompletion) provider.Result {
	choice := completion.Choices[0]
	message := choice.Message
	result := provider.Result{
		ResponseID: completion.ID,
		Text:       renderAssistantText(message),
		Raw:        completion,
	}

	for _, item := range message.ToolCalls {
		functionCall, ok := item.AsAny().(openaisdk.ChatCompletionMessageFunctionToolCall)
		if !ok {
			continue
		}
		result.ToolCalls = append(result.ToolCalls, provider.ToolCall{
			ID:        functionCall.ID,
			Name:      functionCall.Function.Name,
			Arguments: []byte(functionCall.Function.Arguments),
		})
	}

	return result
}

func OpenAIResultFromMessage(responseID string, message openaisdk.ChatCompletionMessage, raw any) provider.Result {
	result := provider.Result{
		ResponseID: responseID,
		Text:       renderAssistantText(message),
		Raw:        raw,
	}

	for _, item := range message.ToolCalls {
		if item.Type != "function" || item.Function.Name == "" {
			continue
		}
		result.ToolCalls = append(result.ToolCalls, provider.ToolCall{
			ID:        item.ID,
			Name:      item.Function.Name,
			Arguments: []byte(item.Function.Arguments),
		})
	}

	return result
}

func renderAssistantText(message openaisdk.ChatCompletionMessage) string {
	if text := strings.TrimSpace(message.Content); text != "" {
		return text
	}
	return strings.TrimSpace(message.Refusal)
}
