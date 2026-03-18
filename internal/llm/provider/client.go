package provider

import (
	"context"
	"fmt"

	"minioc/internal/llm"
	"minioc/internal/llm/models"
)

type Client struct {
	registry *Registry
	catalog  *models.Catalog
}

func NewClient(registry *Registry, catalog *models.Catalog) *Client {
	return &Client{
		registry: registry,
		catalog:  catalog,
	}
}

func (c *Client) Run(ctx context.Context, req llm.Request) (llm.Result, error) {
	if len(req.Messages) == 0 {
		return llm.Result{}, fmt.Errorf("request messages are empty")
	}

	model, err := c.catalog.MustGet(req.Model)
	if err != nil {
		return llm.Result{}, err
	}
	if len(req.Tools) > 0 && !model.SupportsTools {
		return llm.Result{}, fmt.Errorf("model %q does not support tools", model.Ref)
	}

	adapter, ok := c.registry.Get(model.Provider)
	if !ok {
		return llm.Result{}, fmt.Errorf("provider %q is not registered", model.Provider)
	}

	return adapter.Chat(ctx, model, req)
}

func (c *Client) Models(ctx context.Context, providerKey string) ([]string, error) {
	adapter, ok := c.registry.Get(providerKey)
	if !ok {
		return nil, fmt.Errorf("provider %q is not registered", providerKey)
	}
	return adapter.Models(ctx)
}
