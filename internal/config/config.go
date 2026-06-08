package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultMaxSteps = 10000
)

type AgentConfig struct {
	Description  string   `json:"description"`
	Prompt       string   `json:"prompt,omitempty"`
	Instructions []string `json:"instructions,omitempty"`
	Tools        []string `json:"tools,omitempty"`
	MaxSteps     int      `json:"max_steps,omitempty"`
}

type Config struct {
	Path         string                 `json:"-"`
	Model        string                 `json:"model"`
	MaxSteps     int                    `json:"max_steps"`
	AutoApprove  bool                   `json:"auto_approve"`
	Providers    map[string]Provider    `json:"providers"`
	Models       map[string]Model       `json:"models"`
	MCPServers   map[string]MCPServer   `json:"mcp_servers,omitempty"`
	Instructions []string               `json:"instructions"`
	Agents       map[string]AgentConfig `json:"agents,omitempty"`
}

type MCPServer struct {
	Transport       string            `json:"transport"`
	Command         string            `json:"command,omitempty"`
	Args            []string          `json:"args,omitempty"`
	Env             map[string]string `json:"env,omitempty"`
	URL             string            `json:"url,omitempty"`
	Headers         map[string]string `json:"headers,omitempty"`
	MessageEndpoint string            `json:"message_endpoint,omitempty"`
}

type Provider struct {
	Type     string `json:"type"`
	BaseURL  string `json:"base_url,omitempty"`
	APIKey   string `json:"api_key,omitempty"`
	AuthType string `json:"auth_type,omitempty"`
}

func (p Provider) IsAPIKeySet() bool {
	return strings.TrimSpace(p.APIKey) != ""
}

func (p Provider) ResolveAPIKey() (string, error) {
	v := strings.TrimSpace(p.APIKey)
	if v == "" {
		return "", fmt.Errorf("provider %q is missing api_key", p.Type)
	}
	if strings.HasPrefix(v, "{env:") && strings.HasSuffix(v, "}") {
		envKey := v[5 : len(v)-1]
		value := os.Getenv(envKey)
		if value == "" {
			return "", fmt.Errorf("provider %q environment variable %q is empty", p.Type, envKey)
		}
		return value, nil
	}
	return v, nil
}

func (p Provider) Merge(other Provider) Provider {
	if other.Type != "" {
		p.Type = other.Type
	}
	if other.BaseURL != "" {
		p.BaseURL = other.BaseURL
	}
	if other.APIKey != "" {
		p.APIKey = other.APIKey
	}
	if other.AuthType != "" {
		p.AuthType = other.AuthType
	}
	return p
}

type Model struct {
	Provider        string   `json:"provider,omitempty"`
	ID              string   `json:"id"`
	ContextWindow   int      `json:"context_window,omitempty"`
	MaxOutputTokens int      `json:"max_output_tokens,omitempty"`
	SupportsTools   *bool    `json:"supports_tools,omitempty"`
	Temperature     *float64 `json:"temperature,omitempty"`
}

func (m Model) Merge(other Model) Model {
	if other.Provider != "" {
		m.Provider = other.Provider
	}
	if other.ID != "" {
		m.ID = other.ID
	}
	if other.ContextWindow != 0 {
		m.ContextWindow = other.ContextWindow
	}
	if other.MaxOutputTokens != 0 {
		m.MaxOutputTokens = other.MaxOutputTokens
	}
	if other.SupportsTools != nil {
		m.SupportsTools = other.SupportsTools
	}
	if other.Temperature != nil {
		m.Temperature = other.Temperature
	}
	return m
}

func (a AgentConfig) Merge(other AgentConfig) AgentConfig {
	if other.Description != "" {
		a.Description = other.Description
	}
	if other.Prompt != "" {
		a.Prompt = other.Prompt
	}
	if len(other.Instructions) > 0 {
		a.Instructions = other.Instructions
	}
	if len(other.Tools) > 0 {
		a.Tools = other.Tools
	}
	if other.MaxSteps != 0 {
		a.MaxSteps = other.MaxSteps
	}
	return a
}

// Load loads the configuration with a two-level merge strategy:
//   - Global config: ~/.config/minioc/minioc.json (or $XDG_CONFIG_HOME/minioc/minioc.json)
//   - Project config: <repoRoot>/.minioc/minioc.json
//
// Global config is loaded first; project config then overlays it.
// Only fields explicitly set in the project config replace the global values.
// If a global config does not exist, it is created from the embedded assets default.
func Load(repoRoot string) (Config, error) {
	// Ensure global config exists (create from assets if missing).
	if err := ensureGlobalConfig(); err != nil {
		return Config{}, err
	}

	// Load global config first.
	globalCfg, err := loadFromFile(GlobalConfigFile())
	if err != nil {
		return Config{}, fmt.Errorf("load global config: %w", err)
	}

	// Load project config if it exists; otherwise start from empty.
	projectCfg := Config{}
	projectPath := ConfigFile(repoRoot)
	if data, err := os.ReadFile(projectPath); err == nil {
		if err := json.Unmarshal(data, &projectCfg); err != nil {
			return Config{}, fmt.Errorf("decode project config: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return Config{}, fmt.Errorf("read project config: %w", err)
	}

	// Merge: global is the base, project overlays it.
	cfg := mergeConfig(globalCfg, projectCfg)
	cfg.Path = projectPath

	if err := postLoad(&cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// mergeConfig overlays project onto global, returning a new Config.
// Project-level fields that are non-zero (or non-empty maps/slices) replace the global ones.
func mergeConfig(global, project Config) Config {
	if project.Model != "" {
		global.Model = project.Model
	}
	if project.MaxSteps != 0 {
		global.MaxSteps = project.MaxSteps
	}
	// auto_approve defaults to false, so only copy if explicitly true in project.
	if project.AutoApprove {
		global.AutoApprove = true
	}

	// Merge providers: start with global, then overlay project keys with deep merge.
	if global.Providers == nil {
		global.Providers = make(map[string]Provider)
	}
	for key, projProvider := range project.Providers {
		if existing, ok := global.Providers[key]; ok {
			global.Providers[key] = existing.Merge(projProvider)
		} else {
			global.Providers[key] = projProvider
		}
	}

	// Merge models: start with global, then overlay project keys with deep merge.
	if global.Models == nil {
		global.Models = make(map[string]Model)
	}
	for key, projModel := range project.Models {
		if existing, ok := global.Models[key]; ok {
			global.Models[key] = existing.Merge(projModel)
		} else {
			global.Models[key] = projModel
		}
	}

	// Merge instructions: project-level replaces global if non-empty.
	if len(project.Instructions) > 0 {
		global.Instructions = project.Instructions
	}

	// Merge MCP servers: project-level replaces global if non-empty.
	if len(project.MCPServers) > 0 {
		global.MCPServers = project.MCPServers
	}

	// Merge agents: project overlays global with per-key deep merge.
	if global.Agents == nil {
		global.Agents = make(map[string]AgentConfig)
	}
	for key, projAgent := range project.Agents {
		if existing, ok := global.Agents[key]; ok {
			global.Agents[key] = existing.Merge(projAgent)
		} else {
			global.Agents[key] = projAgent
		}
	}

	return global
}

// loadFromFile reads and unmarshals a JSON config file. Returns empty Config if the file does not exist.
func loadFromFile(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, nil
		}
		return Config{}, fmt.Errorf("read %s: %w", path, err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("decode %s: %w", path, err)
	}
	return cfg, nil
}

// postLoad validates and normalises a fully-merged Config.
func postLoad(cfg *Config) error {
	if cfg.MaxSteps == 0 {
		cfg.MaxSteps = defaultMaxSteps
	}
	if cfg.MaxSteps <= 0 {
		return fmt.Errorf("max_steps must be greater than zero")
	}
	if strings.TrimSpace(cfg.Model) == "" {
		return fmt.Errorf("model is required")
	}
	if len(cfg.Providers) == 0 {
		return fmt.Errorf("providers must not be empty")
	}
	if len(cfg.Models) == 0 {
		return fmt.Errorf("models must not be empty")
	}

	for name, provider := range cfg.Providers {
		key := strings.TrimSpace(name)
		if key == "" {
			return fmt.Errorf("provider key must not be empty")
		}
		if strings.Contains(key, "/") {
			return fmt.Errorf("provider key %q must not contain '/'", name)
		}
		if strings.TrimSpace(provider.Type) == "" {
			return fmt.Errorf("provider %q type is required", name)
		}
		if err := validateProviderAuth(name, provider); err != nil {
			return err
		}
		provider.Type = strings.TrimSpace(provider.Type)
		provider.BaseURL = strings.TrimSpace(provider.BaseURL)
		cfg.Providers[name] = provider
	}

	for name, mcpSvr := range cfg.MCPServers {
		key := strings.TrimSpace(name)
		if key == "" {
			return fmt.Errorf("mcp_server key must not be empty")
		}
		transport := strings.TrimSpace(mcpSvr.Transport)
		if transport == "" {
			return fmt.Errorf("mcp_server %q transport is required", name)
		}
		switch transport {
		case "stdio":
			if mcpSvr.Command == "" {
				return fmt.Errorf("mcp_server %q stdio requires command", name)
			}
		case "streamable-http":
			if mcpSvr.URL == "" {
				return fmt.Errorf("mcp_server %q streamable-http requires url", name)
			}
		case "sse":
			if mcpSvr.URL == "" {
				return fmt.Errorf("mcp_server %q sse requires url", name)
			}
		default:
			return fmt.Errorf("mcp_server %q unsupported transport %q", name, transport)
		}
		cfg.MCPServers[name] = mcpSvr
	}

	for name, agent := range cfg.Agents {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("agent key must not be empty")
		}
		if agent.MaxSteps <= 0 {
			agent.MaxSteps = defaultMaxSteps
		}
		cfg.Agents[name] = agent
	}

	for ref, model := range cfg.Models {
		if strings.TrimSpace(ref) == "" {
			return fmt.Errorf("model reference must not be empty")
		}
		if strings.TrimSpace(model.ID) == "" {
			return fmt.Errorf("model %q id is required", ref)
		}
		if model.ContextWindow < 0 {
			return fmt.Errorf("model %q context_window must be zero or greater", ref)
		}
		if model.MaxOutputTokens < 0 {
			return fmt.Errorf("model %q max_output_tokens must be zero or greater", ref)
		}
		model.Provider = strings.TrimSpace(model.Provider)
		model.ID = strings.TrimSpace(model.ID)
		cfg.Models[ref] = model
	}

	return nil
}

func validateProviderAuth(name string, provider Provider) error {
	if !provider.IsAPIKeySet() {
		return fmt.Errorf("provider %q must configure api_key", name)
	}
	authType := strings.TrimSpace(provider.AuthType)
	if authType != "" && authType != "api_key" {
		return fmt.Errorf("provider %q auth type %q is not supported", name, provider.AuthType)
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

// ensureGlobalConfig creates the global config file from assets if it does not exist.
func ensureGlobalConfig() error {
	path := GlobalConfigFile()
	_, err := os.Stat(path)
	if err == nil {
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat global config %s: %w", path, err)
	}

	// Try to use the current binary's asset directory first.
	execPath, execErr := os.Executable()
	if execErr == nil {
		assetsPath := filepath.Join(filepath.Dir(execPath), AssetsDirName, ConfigFileName)
		if data, readErr := os.ReadFile(assetsPath); readErr == nil {
			return writeGlobalConfig(path, data)
		}
	}

	// Fallback: look relative to the repo root (useful in dev).
	wd, err := os.Getwd()
	if err == nil {
		defaultPath := filepath.Join(wd, AssetsConfigPath)
		if data, err := os.ReadFile(defaultPath); err == nil {
			return writeGlobalConfig(path, data)
		}
	}

	return fmt.Errorf("global config %s does not exist and no default found to create it", path)
}

func writeGlobalConfig(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create global config dir %s: %w", dir, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write global config %s: %w", path, err)
	}
	return nil
}
