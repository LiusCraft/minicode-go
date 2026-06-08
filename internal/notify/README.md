# Event Hub

全局通知总线，用于组件间解耦。任何模块发射事件，关心该事件的 handler 自动响应。

## Event

```go
type Event struct {
    Type    string    // "mcp:server_failed", "mcp:tools_changed", "system:shutdown"
    Source  string    // 事件来源，如 "fs-server"
    Payload any       // 具体数据
}
```

## Hub

```go
var Hub = &hub{handlers: map[string][]Handler}

type Handler func(Event)

// 支持精确匹配和前缀通配
// hub.On("mcp:server_failed", h)   → 精确
// hub.On("mcp:*", h)               → 匹配所有 mcp: 前缀
// hub.On("*", h)                   → 匹配所有
func (h *hub) On(pattern string, handler Handler)
func (h *hub) Emit(event Event)
```

## 事件一览

| Type | 发射方 | Payload | 订阅方 |
|---|---|---|---|
| `mcp:server_started` | MCP Manager | `serverName` | Registry / TUI |
| `mcp:server_failed` | MCP Manager | `{serverName, error}` | Registry / TUI |
| `mcp:tools_changed` | MCP Manager | `{serverName, tools}` | Registry |
| `system:shutdown` | 系统 | — | 所有需清理的组件 |
