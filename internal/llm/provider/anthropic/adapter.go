package anthropic

import (
	"context"
	"sort"

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

	message, err := client.Messages.New(ctx, params)
	if err != nil {
		return llm.Result{}, err
	}
	result := transform.AnthropicResult(message)
	if req.Stream != nil {
		if text := result.Text; text != "" && req.Stream.OnTextDelta != nil {
			req.Stream.OnTextDelta(text)
		}
		if req.Stream.OnMessageDone != nil {
			req.Stream.OnMessageDone()
		}
	}
	return result, nil
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
