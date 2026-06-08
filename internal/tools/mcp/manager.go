package mcp

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"encoding/json"

	"minioc/internal/config"
	"minioc/internal/notify"
	"minioc/internal/tools"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Manager struct {
	mu      sync.Mutex
	servers map[string]*managedServer
}

type managedServer struct {
	cfg     config.MCPServer
	client  *mcp.Client
	session *mcp.ClientSession
	tools   []tools.Spec
}

func NewManager() *Manager {
	return &Manager{
		servers: make(map[string]*managedServer),
	}
}

func (m *Manager) StartAll(ctx context.Context, servers map[string]config.MCPServer) {
	var wg sync.WaitGroup
	var mu sync.Mutex

	for name, cfg := range servers {
		wg.Add(1)
		name, cfg := name, cfg
		go func() {
			defer wg.Done()
			ms, err := m.startOne(ctx, name, cfg)
			mu.Lock()
			if err != nil {
				notify.Emit(notify.Event{
					Type:   "mcp:server_failed",
					Source: name,
					Payload: map[string]any{
						"error": err.Error(),
					},
				})
			} else {
				m.servers[name] = ms
				notify.Emit(notify.Event{
					Type:   "mcp:server_started",
					Source: name,
				})
			}
			mu.Unlock()
		}()
	}
	wg.Wait()
}

func (m *Manager) startOne(ctx context.Context, name string, cfg config.MCPServer) (*managedServer, error) {
	var transport mcp.Transport
	switch strings.TrimSpace(cfg.Transport) {
	case "stdio":
		cmd := exec.Command(cfg.Command, cfg.Args...)
		if len(cfg.Env) > 0 {
			cmd.Env = environ(cfg.Env)
		}
		transport = &mcp.CommandTransport{Command: cmd}
	case "streamable-http":
		transport = &mcp.StreamableClientTransport{
			Endpoint: cfg.URL,
		}
	case "sse":
		transport = &mcp.SSEClientTransport{
			Endpoint: cfg.URL,
		}
	default:
		return nil, fmt.Errorf("mcp server %q: unsupported transport %q", name, cfg.Transport)
	}

	client := mcp.NewClient(&mcp.Implementation{
		Name:    "minioc",
		Version: "0.1.0",
	}, nil)

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("mcp server %q: connect: %w", name, err)
	}

	return &managedServer{
		cfg:     cfg,
		client:  client,
		session: session,
	}, nil
}

func (m *Manager) RegisterTools(ctx context.Context, registry *tools.Registry) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, ms := range m.servers {
		result, err := ms.session.ListTools(ctx, nil)
		if err != nil {
			notify.Emit(notify.Event{
				Type:   "mcp:server_failed",
				Source: name,
				Payload: map[string]any{
					"error": fmt.Sprintf("list tools: %v", err),
				},
			})
			continue
		}

		prefix := name + "__"
		var specs []tools.Spec
		for _, t := range result.Tools {
			spec := tools.Spec{
				Name:         prefix + t.Name,
				Description:  t.Description,
				Parameters:   toolSchema(t),
				ParallelSafe: true,
				Execute:      m.executor(name, t.Name),
			}
			specs = append(specs, spec)
		}
		ms.tools = specs

		for _, spec := range specs {
			registry.Register(spec)
		}

		notify.Emit(notify.Event{
			Type:   "mcp:tools_changed",
			Source: name,
			Payload: map[string]any{
				"tools": specs,
			},
		})
	}
}

func (m *Manager) executor(serverName, toolName string) func(context.Context, tools.CallContext, json.RawMessage) (tools.Result, error) {
	return func(ctx context.Context, _ tools.CallContext, arguments json.RawMessage) (tools.Result, error) {
		m.mu.Lock()
		ms, ok := m.servers[serverName]
		m.mu.Unlock()
		if !ok {
			return tools.Result{}, fmt.Errorf("mcp server %q not found", serverName)
		}

		var args any
		if err := json.Unmarshal(arguments, &args); err != nil {
			return tools.Result{}, fmt.Errorf("decode arguments: %w", err)
		}

		result, err := ms.session.CallTool(ctx, &mcp.CallToolParams{
			Name:      toolName,
			Arguments: args,
		})
		if err != nil {
			return tools.Result{}, fmt.Errorf("mcp call %s/%s: %w", serverName, toolName, err)
		}

		return mcpResultToToolsResult(result), nil
	}
}

func (m *Manager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, ms := range m.servers {
		_ = ms.session.Close()
		delete(m.servers, name)
	}
}

func toolSchema(t *mcp.Tool) map[string]any {
	schema, ok := t.InputSchema.(map[string]any)
	if !ok {
		if raw, ok := t.InputSchema.(json.RawMessage); ok {
			_ = json.Unmarshal(raw, &schema)
		}
	}
	if schema == nil {
		schema = map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}
	}
	return schema
}

func environ(env map[string]string) []string {
	var result []string
	for k, v := range env {
		result = append(result, k+"="+v)
	}
	return result
}

func mcpResultToToolsResult(result *mcp.CallToolResult) tools.Result {
	var parts []string
	for _, content := range result.Content {
		switch c := content.(type) {
		case *mcp.TextContent:
			parts = append(parts, c.Text)
		case *mcp.ImageContent:
			parts = append(parts, fmt.Sprintf("[image: %s (%d bytes)]", c.MIMEType, len(c.Data)))
		case *mcp.EmbeddedResource:
			parts = append(parts, fmt.Sprintf("[resource: %s]", c.Resource.URI))
		default:
			parts = append(parts, fmt.Sprintf("%+v", c))
		}
	}
	output := strings.Join(parts, "\n")
	if result.IsError {
		if output == "" {
			output = "tool call failed"
		}
		return tools.Result{Output: "ERROR: " + output}
	}
	return tools.Result{Output: output}
}
