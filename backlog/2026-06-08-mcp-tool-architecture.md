# MCP 工具集成设计

日期: 2026-06-08
前置依赖: Event Hub (`internal/notify/`)
状态: 设计待实现

## 配置格式

```json
{
  "mcp_servers": {
    "fs-server": {
      "transport": "stdio",
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "."],
      "env": { "CUSTOM_VAR": "value" }
    },
    "weather": {
      "transport": "streamable-http",
      "url": "https://api.example.com/mcp",
      "headers": { "Authorization": "Bearer xxx" }
    },
    "legacy-tools": {
      "transport": "sse",
      "url": "https://old.example.com/sse",
      "message_endpoint": "https://old.example.com/messages"
    }
  }
}
```

| 字段 | 适用 transport | 说明 |
|---|---|---|
| `transport` | 全部 | `stdio` / `streamable-http` / `sse` |
| `command` | stdio | 可执行文件路径 |
| `args` | stdio | 启动参数 |
| `env` | stdio | 注入额外环境变量 |
| `url` | streamable-http / sse | MCP endpoint URL |
| `headers` | streamable-http / sse | 自定义 HTTP 头 |
| `message_endpoint` | sse | 旧版 POST 消息端点 |

## MCP Manager

`internal/tools/mcp/` — 进程级单例，负责 MCP Server 生命周期和工具路由。

依赖 SDK：`github.com/modelcontextprotocol/go-sdk/mcp`

```
internal/tools/mcp/
├── manager.go    # Manager：生命周期、路由
├── config.go     # MCPServerConfig 类型（在 config 包或 mcp 包）
└── manager_test.go
```

### managedServer

```go
type managedServer struct {
    cfg     config.MCPServer
    client  *mcp.Client         // SDK Client
    session *mcp.ClientSession  // 已初始化的连接句柄
    tools   []tools.Spec        // 已加前缀的工具列表
}
```

### 生命周期

```
Manager.StartAll()
  for each server in config:
    1. 按 transport 创建 SDK transport
       stdio           → &mcp.CommandTransport{Command, Args}
       streamable-http → &mcp.StreamableClientTransport{Endpoint, Headers}
       sse             → SDK 兼容

    2. client.Connect(ctx, transport) → ClientSession

    3. session.OnNotification → 监听 tools/list_changed

    4. session.ListTools() → 获取工具列表

    5. 记录 managedServer

    6. hub.Emit("mcp:server_started", serverName)

  ─── 单 server 失败不阻塞整体 ───
  → hub.Emit("mcp:server_failed", {serverName, err})
  → 继续下一个

Manager.RegisterTools(registry)
  for each running server:
    tools := server.session.ListTools(ctx)
    for each tool:
      spec := tools.Spec{
          Name:         server.name + ":" + tool.Name,
          Description:  tool.Description,
          Parameters:   tool.InputSchema,
          ParallelSafe: true,
          Execute:      m.executeTool(server.name, tool.Name),
      }
      registry.Register(spec)

  ─── 收到 tools/list_changed 时 ───
  → 重新 ListTools
  → hub.Emit("mcp:tools_changed", {serverName, newTools})

Manager.CloseAll()
  for each server:
    session.Close()
    // stdio: 子进程自动退出
```

### CallTool 路由

```go
func (m *Manager) executeTool(serverName, toolName string) func(ctx, CallContext, json.RawMessage) (Result, error) {
    return func(ctx context.Context, _ CallContext, arguments json.RawMessage) (Result, error) {
        result, err := m.servers[serverName].session.CallTool(ctx, &mcp.CallToolParams{
            Name:      toolName,
            Arguments: arguments,
        })
        return mcpResultToToolResult(result), err
    }
}
```

### MCP 结果 → tools.Result 转换

```go
func mcpResultToToolResult(result *mcp.CallToolResult) tools.Result {
    var parts []string
    for _, content := range result.Content {
        switch c := content.(type) {
        case *mcp.TextContent:
            parts = append(parts, c.Text)
        case *mcp.ImageContent:
            parts = append(parts, fmt.Sprintf("[image: %s]", c.MIMEType))
        case *mcp.ResourceContent:
            parts = append(parts, fmt.Sprintf("[resource: %s]", c.URI))
        }
    }
    return tools.Result{Output: strings.Join(parts, "\n")}
}
```

## Registry 改动

新增动态注册/注销能力：

```go
func (r *Registry) Register(spec Spec)
func (r *Registry) Unregister(name string)
func (r *Registry) UnregisterAll(serverName string)  // 移除某 server 所有工具
func (r *Registry) ReloadTools(serverName string, newTools []Spec)
```

## 启动流程

```go
// main.go
mcpManager := mcp.NewManager(cfg.MCPServers)
mcpManager.StartAll(ctx)                     // 逐个启动
mcpManager.RegisterTools(registry)            // 注册到主 Registry
defer mcpManager.CloseAll()

// 监听事件
notify.Hub.On("mcp:tools_changed", func(e notify.Event) {
    data := e.Payload.(mcp.ToolsChangedPayload)
    registry.ReloadTools(data.ServerName, data.Tools)
})

notify.Hub.On("mcp:server_failed", func(e notify.Event) {
    data := e.Payload.(mcp.ServerFailedPayload)
    registry.UnregisterAll(data.ServerName)
})
```

## 命名约定

MCP 工具加前缀：`{server_name}__{tool_name}`（双下划线，兼容 OpenAI 对函数名字符集的要求 `^[a-zA-Z0-9_-]{1,64}$`）

- fs-server 的 read_file → `fs-server__read_file`
- weather 的 get_forecast → `weather__get_forecast`

## 第一版不做的

- 自动重连（server 失败直接标记 failed）
- 自定义工具（custom tools）
- OAuth 授权流
- Resource / Prompt 支持

## 依赖

```
github.com/modelcontextprotocol/go-sdk
```
