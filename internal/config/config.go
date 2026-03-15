package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

const defaultModel = "gpt-5-mini"

type Options struct {
	ModelOverride string
	MaxSteps      int
	AutoApprove   bool
}

type Config struct {
	APIKey      string
	BaseURL     string
	Model       string
	MaxSteps    int
	AutoApprove bool
}

func Load(opts Options) (Config, error) {
	model := strings.TrimSpace(opts.ModelOverride)
	if model == "" {
		model = strings.TrimSpace(os.Getenv("MINIOC_MODEL"))
	}
	if model == "" {
		model = defaultModel
	}

	if opts.MaxSteps <= 0 {
		return Config{}, fmt.Errorf("max steps must be greater than zero")
	}

	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if apiKey == "" {
		return Config{}, fmt.Errorf("OPENAI_API_KEY is required")
	}

	baseURL := strings.TrimSpace(os.Getenv("OPENAI_BASE_URL"))
	if baseURL != "" {
		parsed, err := url.Parse(baseURL)
		if err != nil {
			return Config{}, fmt.Errorf("invalid OPENAI_BASE_URL: %w", err)
		}
		if parsed.Scheme == "" || parsed.Host == "" {
			return Config{}, fmt.Errorf("invalid OPENAI_BASE_URL: must include scheme and host")
		}
	}

	return Config{
		APIKey:      apiKey,
		BaseURL:     baseURL,
		Model:       model,
		MaxSteps:    opts.MaxSteps,
		AutoApprove: opts.AutoApprove,
	}, nil
}
