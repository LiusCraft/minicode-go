package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

const (
	defaultMaxSteps = 1000
)

type Config struct {
	Path        string              `json:"-"`
	Model       string              `json:"model"`
	MaxSteps    int                 `json:"max_steps"`
	AutoApprove bool                `json:"auto_approve"`
	Providers   map[string]Provider `json:"providers"`
	Models      map[string]Model    `json:"models"`
}

type Provider struct {
	Type    string       `json:"type"`
	BaseURL string       `json:"base_url,omitempty"`
	APIKey  SecretValue  `json:"api_key,omitempty"`
	Auth    ProviderAuth `json:"auth,omitempty"`
}

type ProviderAuth struct {
	Type   string      `json:"type,omitempty"`
	APIKey SecretValue `json:"api_key,omitempty"`
}

type Model struct {
	Provider        string   `json:"provider,omitempty"`
	ID              string   `json:"id"`
	ContextWindow   int      `json:"context_window,omitempty"`
	MaxOutputTokens int      `json:"max_output_tokens,omitempty"`
	SupportsTools   *bool    `json:"supports_tools,omitempty"`
	Temperature     *float64 `json:"temperature,omitempty"`
}

type SecretValue struct {
	Value string
	Env   string
}

func Load(repoRoot string) (Config, error) {
	path := ConfigFile(repoRoot)
	if err := ensureConfigFile(repoRoot, path); err != nil {
		return Config{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read %s: %w", ConfigPath, err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("decode %s: %w", ConfigPath, err)
	}
	cfg.Path = path

	if cfg.MaxSteps == 0 {
		cfg.MaxSteps = defaultMaxSteps
	}
	if cfg.MaxSteps <= 0 {
		return Config{}, fmt.Errorf("max_steps must be greater than zero")
	}
	if strings.TrimSpace(cfg.Model) == "" {
		return Config{}, fmt.Errorf("model is required")
	}
	if len(cfg.Providers) == 0 {
		return Config{}, fmt.Errorf("providers must not be empty")
	}
	if len(cfg.Models) == 0 {
		return Config{}, fmt.Errorf("models must not be empty")
	}

	for name, provider := range cfg.Providers {
		key := strings.TrimSpace(name)
		if key == "" {
			return Config{}, fmt.Errorf("provider key must not be empty")
		}
		if strings.Contains(key, "/") {
			return Config{}, fmt.Errorf("provider key %q must not contain '/'", name)
		}
		if strings.TrimSpace(provider.Type) == "" {
			return Config{}, fmt.Errorf("provider %q type is required", name)
		}
		if err := validateProviderAuth(name, provider); err != nil {
			return Config{}, err
		}
		provider.Type = strings.TrimSpace(provider.Type)
		provider.BaseURL = strings.TrimSpace(provider.BaseURL)
		cfg.Providers[name] = provider
	}

	for ref, model := range cfg.Models {
		if strings.TrimSpace(ref) == "" {
			return Config{}, fmt.Errorf("model reference must not be empty")
		}
		if strings.TrimSpace(model.ID) == "" {
			return Config{}, fmt.Errorf("model %q id is required", ref)
		}
		if model.ContextWindow < 0 {
			return Config{}, fmt.Errorf("model %q context_window must be zero or greater", ref)
		}
		if model.MaxOutputTokens < 0 {
			return Config{}, fmt.Errorf("model %q max_output_tokens must be zero or greater", ref)
		}
		model.Provider = strings.TrimSpace(model.Provider)
		model.ID = strings.TrimSpace(model.ID)
		cfg.Models[ref] = model
	}

	return cfg, nil
}

func (p Provider) EffectiveAuth() ProviderAuth {
	if !p.Auth.IsZero() {
		return p.Auth
	}
	if !p.APIKey.IsZero() {
		return ProviderAuth{Type: "api_key", APIKey: p.APIKey}
	}
	return ProviderAuth{}
}

func (a ProviderAuth) IsZero() bool {
	return strings.TrimSpace(a.Type) == "" && a.APIKey.IsZero()
}

func (s SecretValue) IsZero() bool {
	return strings.TrimSpace(s.Value) == "" && strings.TrimSpace(s.Env) == ""
}

func (s *SecretValue) UnmarshalJSON(data []byte) error {
	data = bytesTrimSpace(data)
	if len(data) == 0 || string(data) == "null" {
		*s = SecretValue{}
		return nil
	}

	if data[0] == '"' {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		s.Value = strings.TrimSpace(value)
		s.Env = ""
		return nil
	}

	var raw struct {
		Value string `json:"value,omitempty"`
		Env   string `json:"env,omitempty"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("secret value must be a string or object: %w", err)
	}
	*s = SecretValue{
		Value: strings.TrimSpace(raw.Value),
		Env:   strings.TrimSpace(raw.Env),
	}
	return nil
}

func validateProviderAuth(name string, provider Provider) error {
	if !provider.APIKey.IsZero() && !provider.Auth.IsZero() {
		return fmt.Errorf("provider %q cannot set both api_key and auth", name)
	}

	auth := provider.EffectiveAuth()
	authType := strings.TrimSpace(auth.Type)
	if authType == "" {
		authType = "api_key"
	}
	if authType != "api_key" {
		return fmt.Errorf("provider %q auth type %q is not supported", name, auth.Type)
	}
	if auth.APIKey.IsZero() {
		return fmt.Errorf("provider %q must configure api_key or auth.api_key", name)
	}
	if auth.APIKey.Value != "" && auth.APIKey.Env != "" {
		return fmt.Errorf("provider %q api_key must choose either value or env", name)
	}
	return nil
}

func bytesTrimSpace(data []byte) []byte {
	start := 0
	for start < len(data) && (data[start] == ' ' || data[start] == '\n' || data[start] == '\r' || data[start] == '\t') {
		start++
	}
	end := len(data)
	for end > start && (data[end-1] == ' ' || data[end-1] == '\n' || data[end-1] == '\r' || data[end-1] == '\t') {
		end--
	}
	return data[start:end]
}

func ensureConfigFile(repoRoot, configPath string) error {
	_, err := os.Stat(configPath)
	if err == nil {
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat %s: %w", ConfigPath, err)
	}

	defaultPath := AssetsConfigFile(repoRoot)
	defaultData, err := os.ReadFile(defaultPath)
	if err != nil {
		return fmt.Errorf("read default config %s: %w", AssetsConfigPath, err)
	}

	if err := os.MkdirAll(Root(repoRoot), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", DirName, err)
	}
	if err := os.WriteFile(configPath, defaultData, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", ConfigPath, err)
	}
	return nil
}
