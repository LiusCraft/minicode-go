package anthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	anthropicsdk "github.com/anthropics/anthropic-sdk-go"
	anthropicoption "github.com/anthropics/anthropic-sdk-go/option"

	"minioc/internal/config"
	"minioc/internal/llm"
	"minioc/internal/llm/models"
	"minioc/internal/llm/provider"
	"minioc/internal/llm/provider/transform"
)

type Adapter struct {
	providerName string
	config       config.Provider
}

func New(providerName string, providerConfig config.Provider) *Adapter {
	return &Adapter{
		providerName: providerName,
		config:       providerConfig,
	}
}

func (a *Adapter) Chat(ctx context.Context, model models.Model, req llm.Request) (llm.Result, error) {
	client, err := a.client()
	if err != nil {
		return llm.Result{}, err
	}

	params, err := transform.AnthropicMessageParams(model, req)
	if err != nil {
		return llm.Result{}, err
	}

	if req.Stream != nil {
		return a.runStreaming(ctx, client, params, req.Stream)
	}

	message, err := client.Messages.New(ctx, params)
	if err != nil {
		return llm.Result{}, err
	}
	return transform.AnthropicResult(message), nil
}

func (a *Adapter) runStreaming(ctx context.Context, client anthropicsdk.Client, params anthropicsdk.MessageNewParams, handler *llm.StreamHandler) (llm.Result, error) {
	stream := client.Messages.NewStreaming(ctx, params)

	type accumulatedToolCall struct {
		ID        string
		Name      string
		InputJSON strings.Builder
		Input     any // decoded once complete
	}

	var (
		textBuf       strings.Builder
		toolCalls     []accumulatedToolCall
		currentToolID int = -1
	)

	for stream.Next() {
		event := stream.Current().AsAny()
		switch e := event.(type) {
		case anthropicsdk.ContentBlockStartEvent:
			// Track when a tool_use block starts so we can accumulate its input_json_delta
			if e.ContentBlock.Type == "tool_use" {
				currentToolID = len(toolCalls)
				toolCalls = append(toolCalls, accumulatedToolCall{
					ID:   e.ContentBlock.ID,
					Name: e.ContentBlock.Name,
				})
			}

		case anthropicsdk.ContentBlockDeltaEvent:
			delta := e.Delta.AsAny()
			switch d := delta.(type) {
			case anthropicsdk.TextDelta:
				if d.Text != "" && handler.OnTextDelta != nil {
					handler.OnTextDelta(d.Text)
				}
				textBuf.WriteString(d.Text)
			case anthropicsdk.InputJSONDelta:
				if currentToolID >= 0 && currentToolID < len(toolCalls) {
					toolCalls[currentToolID].InputJSON.WriteString(d.PartialJSON)
				}
			}

		case anthropicsdk.ContentBlockStopEvent:
			// When a content block stops, clear the current tool index
			currentToolID = -1

		case anthropicsdk.MessageStopEvent:
			// Stream is ending

		case anthropicsdk.MessageStartEvent, anthropicsdk.MessageDeltaEvent:
			// Message metadata; nothing to emit

		default:
			// ignore unknown event types
		}
	}

	if err := stream.Err(); err != nil {
		return llm.Result{}, fmt.Errorf("anthropic streaming error: %w", err)
	}

	if err := stream.Close(); err != nil {
		return llm.Result{}, fmt.Errorf("failed to close stream: %w", err)
	}

	// Build final result
	result := llm.Result{
		Text: strings.TrimSpace(textBuf.String()),
	}

	for _, tc := range toolCalls {
		if tc.ID == "" || tc.Name == "" {
			continue
		}
		rawArgs := json.RawMessage(toolCallsJSON(tc.InputJSON.String()))
		result.ToolCalls = append(result.ToolCalls, llm.ToolCall{
			ID:        tc.ID,
			Name:      tc.Name,
			Arguments: rawArgs,
		})
	}

	if handler.OnMessageDone != nil {
		handler.OnMessageDone()
	}

	return result, nil
}

// toolCallsJSON normalizes streamed tool arguments into a valid JSON payload.
// Anthropic may stream either a full JSON object or object fragments without
// outer braces. We keep valid JSON as-is, try wrapping fragments, and fall
// back to an empty object if decoding never becomes valid.
func toolCallsJSON(partial string) string {
	trimmed := strings.TrimSpace(partial)
	if trimmed == "" {
		return "{}"
	}
	if json.Valid([]byte(trimmed)) {
		return trimmed
	}
	wrapped := "{" + trimmed + "}"
	if json.Valid([]byte(wrapped)) {
		return wrapped
	}
	return "{}"
}

func (a *Adapter) Models(ctx context.Context) ([]string, error) {
	client, err := a.client()
	if err != nil {
		return nil, err
	}

	pager := client.Models.ListAutoPaging(ctx, anthropicsdk.ModelListParams{})
	result := make([]string, 0, 32)
	for pager.Next() {
		result = append(result, pager.Current().ID)
	}
	if err := pager.Err(); err != nil {
		return nil, err
	}
	sort.Strings(result)
	return result, nil
}

func (a *Adapter) client() (anthropicsdk.Client, error) {
	apiKey, err := provider.ResolveAPIKey(a.providerName, a.config)
	if err != nil {
		return anthropicsdk.Client{}, err
	}

	opts := []anthropicoption.RequestOption{anthropicoption.WithAPIKey(apiKey)}
	if a.config.BaseURL != "" {
		opts = append(opts, anthropicoption.WithBaseURL(a.config.BaseURL))
	}
	return anthropicsdk.NewClient(opts...), nil
}
