# Subagent 设计文档

## 概述

为 minioc 增加 subagent 能力：主 agent 可通过工具 `spawn_agent` 按角色名创建隔离的 subagent 实例，subagent 拥有独立上下文窗口、独立 system prompt、可配置的工具白名单和步骤上限。

## 配置

### minioc.json 新增字段

```jsonc
{
  "max_steps": 10000,
  "agents": {
    "explore": {
      "description": "只读探索代码库，收集信息",
      "prompt": "你是一个代码探索专家...",
      "instructions": [".minioc/agents/explore.md"],
      "tools": ["read_file", "glob", "grep", "fetch"],
      "max_steps": 10
    },
    "plan": {
      "description": "分析代码并制定实施计划",
      "instructions": [".minioc/agents/plan.md"],
      "tools": ["read_file", "glob", "grep", "bash", "write_file"],
      "max_steps": 15
    }
  }
}
```

### AgentConfig 字段

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `description` | string | 是 | 描述 agent 职责，LLM 据此决定是否调度 |
| `prompt` | string | 否 | 内联 system prompt |
| `instructions` | string[] | 否 | 指令文件路径（复用现有加载逻辑） |
| `tools` | string[] | 否 | 工具白名单；缺省=全部工具 |
| `max_steps` | int | 否 | 覆盖 `minioc.json.max_steps` |

`prompt`（如果非空）在前，`instructions` 文件内容在后，用换行连接后作为 subagent 的 system prompt。**不继承**主 agent 的 system prompt，保证上下文干净隔离。

`instructions` 文件路径解析复用现有 `resolveConfiguredInstructionPaths` 逻辑：相对路径相对于 `repoRoot`，`~/` 展开到 home 目录，绝对路径原样，文件不存在则跳过。

### 配置加载

复用现有 global→project merge 机制（`config.Load` / `mergeConfig`），agents 逐 key 深层 merge。

### max_steps 优先级

`spawn_agent(max_steps=N)` > `agents[name].max_steps` > `minioc.json.max_steps` > 10000

## Session

```go
type Session struct {
    ID        string `json:"id"`
    ParentID  string `json:"parent_id,omitempty"`  // 新增
    // ... 现有字段
}
```

- subagent session ID 格式：`subagent_xxxxxx`（`session.newID("subagent")`）
- 使用 `FileStore` 持久化到 `.minioc/sessions/subagent_xxxxxx.json`
- `parent_id` 记录主 session ID，可追溯关联

## 包结构

```
internal/subagent/
    manager.go    — SubagentManager（生命周期、result mailbox、agent 工厂）
    tools.go      — 工具工厂函数
internal/session/
    session.go    — 新增 ParentID + NewSubagent 构造函数
internal/config/
    config.go     — 新增 Agents map[string]AgentConfig
```

## SubagentManager

```go
type Manager struct {
    client       llm.Client
    store        session.Store
    tools        *tools.Registry
    agents       map[string]config.AgentConfig
    permissions  *safety.PermissionManager
    defaultMaxSteps int
    instructions    []string
    repoRoot     string
    workdir      string
    model        string
    parentSessionID string

    mu      sync.Mutex
    running map[string]context.CancelFunc
    results chan Result
}

type Result struct {
    AgentID   string
    SessionID string
    Status    string // "completed" | "error" | "killed"
    Output    string
}
```

### spawn 流程

1. 按角色名从 `agents` map 查找 `AgentConfig`
2. 创建新 `session.Session`（`ID: subagent_xxxxxx`, `ParentID: parentSessionID`，使用 `FileStore` 持久化）
3. 根据 `tools` 白名单从主 registry 过滤出子 registry（缺省则用主 registry）。`spawn_agent` / `list_agents` / `kill_agent` / `get_agent_session` 等管理工具**不传给 subagent**，禁止递归 spawn
4. 合并 `prompt` + `instructions`（文件内容）为 system prompt
5. 应用优先级链确定 `max_steps`
6. 启动 goroutine 运行 `agent.Loop.Run()`（独立 `context.WithCancel`）
7. 等待完成，返回 `Result`

### 事件通知

SubagentManager 在关键生命周期点通过 `notify.Hub` 发出事件：

| 事件 | 触发时机 | Payload |
|---|---|---|
| `subagent:spawned` | 新 subagent 启动 | `{agent_id, agent_type}` |
| `subagent:step` | subagent 每完成一步 | `{agent_id, step_count, message_count}` |
| `subagent:completed` | subagent 正常完成 | `{agent_id, status:"completed"}` |
| `subagent:killed` | 被 kill_agent 终止 | `{agent_id, status:"killed"}` |
| `subagent:error` | subagent 出错 | `{agent_id, status:"error", error}` |

TUI 订阅 `subagent:*` 模式，收到事件后更新本地状态树并触发重渲染。

### 并发安全

- `spawn_agent` 标记为 `ParallelSafe=true`，同批次多个 spawn 并发执行 goroutine
- `list_agents` / `kill_agent` / `get_agent_session` 标记为 `ParallelSafe=false`
- 主 context cancel 时级联 cancel 所有 running subagent

## 工具 API

### spawn_agent

```
spawn_agent(agent: string, task: string, max_steps?: number)
  → {
      "agent_id": "subagent_a1b2c3",
      "session_id": "subagent_a1b2c3",
      "status": "completed",
      "output": "探索结果摘要..."
    }
```

- `agent`：角色名，匹配 `minioc.json` 中 `agents` 的 key
- `task`：子任务描述
- `max_steps`：可选，覆盖优先级最高
- 同步等待，返回 subagent 最终回答

### list_agents

```
list_agents()
  → [{"agent_id": "...", "agent_type": "explore", "status": "running" | "completed", "elapsed_seconds": 12, "step_count": 5}]
```

### get_agent_session

```
get_agent_session(agent_id: string)
  → {
      "agent_id": "subagent_a1b2c3",
      "parent_id": "sess_f6e7d8",
      "status": "running" | "completed" | "killed",
      "model": "...",
      "step_count": 5,
      "messages": [
        {"role": "user", "content": "搜索 API 路由"},
        {"role": "assistant", "content": "让我查看路由文件..."},
        {"role": "tool", "tool_name": "grep", ...}
      ]
    }
```

直接从 `SubagentManager` 内存中的 `runningAgent.sess.Messages` 读取，无需走磁盘。FileStore 持久化用于 crash 恢复和后续回顾，实时查看走内存。

### kill_agent

```
kill_agent(agent_id: string)
  → {"ok": true}
```

## TUI 交互

在 Bubble Tea TUI 中，subagent 运行时用户可实时查看进度：

- **主界面**显示当前 running subagent 列表（agent_id + 类型 + 步数 + 耗时）
- 用户**选中**某个 subagent 进入详情视图，展示其当前消息列表（同 `get_agent_session` 内容）
- 详情视图定期刷新磁盘文件，展示最新进展
- 用户可**返回**主视图，subagent 仍在后台继续运行
- 用户也可在详情视图**终止**（kill）该 subagent

实现方式：TUI 启动时 `notify.On("subagent:*", handler)` 订阅事件，维护本地 subagent 状态 map。用户选中某个 agent 时直接从 `SubagentManager.GetAgent(agentID)` 获取 `runningAgent.sess.Messages` 渲染详情。

## 用例

### 并行信息收集

```
模型一次响应发出 3 个 spawn_agent（ParallelSafe=true）：
  spawn_agent("explore", "搜索 API 路由定义")
  spawn_agent("explore", "搜索数据库模型")
  spawn_agent("explore", "搜索前端组件")

三者并发 goroutine 执行，主 agent 等全部完成。
用户此时在 TUI 中看到 3 个 running subagent，
点进去实时查看各自读到了什么文件。
完成后主 agent 收到全部结果，继续下一步。
```

### Pipeline 工作流

```
第一轮：spawn_agent("plan", "制定重构方案")     → 拿到计划
第二轮：spawn_agent("implement", "执行步骤1")   → 拿到实现
第三轮：spawn_agent("review", "审查实现")       → 拿到审查意见
```
