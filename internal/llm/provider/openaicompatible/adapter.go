package openaicompatible

import (
	"context"
	"fmt"
	"sort"

	openaisdk "github.com/openai/openai-go/v3"
	openaioption "github.com/openai/openai-go/v3/option"

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

	params := transform.OpenAIChatParams(model, req)
	if req.Stream != nil {
		return a.runStreaming(ctx, client, params, req.Stream)
	}
	return a.runNonStreaming(ctx, client, params, nil)
}

func (a *Adapter) Models(ctx context.Context) ([]string, error) {
	client, err := a.client()
	if err != nil {
		return nil, err
	}

	pager := client.Models.ListAutoPaging(ctx)
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

func (a *Adapter) client() (openaisdk.Client, error) {
	apiKey, err := provider.ResolveAPIKey(a.providerName, a.config)
	if err != nil {
		return openaisdk.Client{}, err
	}

	opts := []openaioption.RequestOption{openaioption.WithAPIKey(apiKey)}
	if a.config.BaseURL != "" {
		opts = append(opts, openaioption.WithBaseURL(a.config.BaseURL))
	}
	return openaisdk.NewClient(opts...), nil
}

func (a *Adapter) runStreaming(ctx context.Context, client openaisdk.Client, params openaisdk.ChatCompletionNewParams, handler *llm.StreamHandler) (llm.Result, error) {
	streamParams := params
	streamParams.StreamOptions = openaisdk.ChatCompletionStreamOptionsParam{
		IncludeUsage: openaisdk.Bool(true),
	}

	stream := client.Chat.Completions.NewStreaming(ctx, streamParams)
	acc := openaisdk.ChatCompletionAccumulator{}
	sawChunk := false

	for stream.Next() {
		sawChunk = true
		chunk := stream.Current()
		if !acc.AddChunk(chunk) {
			return llm.Result{}, fmt.Errorf("failed to accumulate streamed chat completion")
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		emitTextDelta(handler, chunk.Choices[0].Delta)
	}

	if err := stream.Err(); err != nil {
		if !sawChunk {
			return a.runNonStreaming(ctx, client, params, handler)
		}
		emitMessageDone(handler)
		return llm.Result{}, err
	}
	if len(acc.Choices) == 0 {
		emitMessageDone(handler)
		return llm.Result{}, fmt.Errorf("chat completion returned no choices")
	}

	emitMessageDone(handler)
	return transform.OpenAIResultFromMessage(acc.ID, acc.Choices[0].Message, acc.ChatCompletion), nil
}

func (a *Adapter) runNonStreaming(ctx context.Context, client openaisdk.Client, params openaisdk.ChatCompletionNewParams, handler *llm.StreamHandler) (llm.Result, error) {
	completion, err := client.Chat.Completions.New(ctx, params)
	if err != nil {
		return llm.Result{}, err
	}
	if len(completion.Choices) == 0 {
		return llm.Result{}, fmt.Errorf("chat completion returned no choices")
	}

	result := transform.OpenAIResult(completion)
	if handler != nil {
		if text := result.Text; text != "" && handler.OnTextDelta != nil {
			handler.OnTextDelta(text)
		}
		emitMessageDone(handler)
	}
	return result, nil
}

func emitTextDelta(handler *llm.StreamHandler, delta openaisdk.ChatCompletionChunkChoiceDelta) {
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

func emitMessageDone(handler *llm.StreamHandler) {
	if handler == nil || handler.OnMessageDone == nil {
		return
	}
	handler.OnMessageDone()
}
