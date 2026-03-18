package models

import (
	"fmt"

	"minioc/internal/config"
)

type Model struct {
	Ref             string
	Provider        string
	ID              string
	ContextWindow   int
	MaxOutputTokens int
	SupportsTools   bool
	Temperature     *float64
}

type Catalog struct {
	models     map[string]Model
	byProvider map[string][]Model
}

func New(cfg config.Config) (*Catalog, error) {
	items := make(map[string]Model, len(cfg.Models))
	byProvider := make(map[string][]Model, len(cfg.Providers))

	for ref, item := range cfg.Models {
		parsed, err := ParseRef(ref)
		if err != nil {
			return nil, err
		}

		providerKey := item.Provider
		if providerKey == "" {
			providerKey = parsed.Provider
		}
		if providerKey != parsed.Provider {
			return nil, fmt.Errorf("model %q provider %q does not match reference provider %q", ref, providerKey, parsed.Provider)
		}
		if _, ok := cfg.Providers[providerKey]; !ok {
			return nil, fmt.Errorf("model %q references unknown provider %q", ref, providerKey)
		}

		supportsTools := true
		if item.SupportsTools != nil {
			supportsTools = *item.SupportsTools
		}

		model := Model{
			Ref:             ref,
			Provider:        providerKey,
			ID:              item.ID,
			ContextWindow:   item.ContextWindow,
			MaxOutputTokens: item.MaxOutputTokens,
			SupportsTools:   supportsTools,
			Temperature:     cloneFloat64(item.Temperature),
		}
		items[ref] = model
		byProvider[providerKey] = append(byProvider[providerKey], model)
	}

	if _, ok := items[cfg.Model]; !ok {
		return nil, fmt.Errorf("default model %q is not defined in models", cfg.Model)
	}

	return &Catalog{
		models:     items,
		byProvider: byProvider,
	}, nil
}

func (c *Catalog) Get(ref string) (Model, bool) {
	model, ok := c.models[ref]
	return model, ok
}

func (c *Catalog) MustGet(ref string) (Model, error) {
	model, ok := c.Get(ref)
	if !ok {
		return Model{}, fmt.Errorf("unknown model %q", ref)
	}
	return model, nil
}

func (c *Catalog) ForProvider(provider string) []Model {
	items := c.byProvider[provider]
	if len(items) == 0 {
		return nil
	}
	result := make([]Model, len(items))
	copy(result, items)
	return result
}

func cloneFloat64(value *float64) *float64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
