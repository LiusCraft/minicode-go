# Subagent System Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add parallel subagent system with role-based agent config, persistent sessions, event-driven TUI peek

**Architecture:** SubagentManager manages lifecycle with goroutine-per-agent, event-driven progress via notify hub, and FileStore persistence. spawn_agent is ParallelSafe=true, blocking per-call but concurrent within a batch.

**Tech Stack:** Go, existing minioc patterns (agent.Loop, session.FileStore, tools.Registry, notify.Hub)

---

### Task 1: Session — Add ParentID + NewSubagent

**Files:**
- Modify: `internal/session/session.go`
- Test: `internal/session/session_test.go`

- [ ] **Step 1: Add ParentID field and NewSubagent**

Edit `internal/session/session.go`. Add `ParentID` after `ID`:

```go
type Session struct {
    ID        string    `json:"id"`
    ParentID  string    `json:"parent_id,omitempty"`
    RepoRoot  string    `json:"repo_root"`
    Workdir   string    `json:"workdir"`
    Model     string    `json:"model"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
    Messages  []Message `json:"messages"`
}
```

Add `NewSubagent` after `New`:

```go
func NewSubagent(repoRoot, workdir, model, parentID string) *Session {
    sess := New(repoRoot, workdir, model)
    sess.ID = newID("subagent")
    sess.ParentID = parentID
    return sess
}
```

- [ ] **Step 2: Write test**

Add to `internal/session/session_test.go`:

```go
func TestNewSubagent(t *testing.T) {
    sess := NewSubagent("/repo", "/repo/src", "gpt-4", "sess_parent123")
    if !strings.HasPrefix(sess.ID, "subagent_") {
        t.Errorf("expected prefix 'subagent_', got %q", sess.ID)
    }
    if sess.ParentID != "sess_parent123" {
        t.Errorf("ParentID: got %q, want %q", sess.ParentID, "sess_parent123")
    }
    if sess.RepoRoot != "/repo" {
        t.Errorf("RepoRoot: got %q", sess.RepoRoot)
    }
    if sess.Model != "gpt-4" {
        t.Errorf("Model: got %q", sess.Model)
    }
    if len(sess.Messages) != 0 {
        t.Errorf("expected empty messages")
    }
}
```

- [ ] **Step 3: Run test**

```bash
go test ./internal/session/ -run TestNewSubagent -v
```
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/session/session.go internal/session/session_test.go
git commit -m "feat(session): add ParentID field and NewSubagent constructor"
```

---

### Task 2: Config — Add AgentConfig type + merge logic

**Files:**
- Modify: `internal/config/config.go`

- [ ] **Step 1: Add AgentConfig type** before `Config`:

```go
type AgentConfig struct {
    Description  string   `json:"description"`
    Prompt       string   `json:"prompt,omitempty"`
    Instructions []string `json:"instructions,omitempty"`
    Tools        []string `json:"tools,omitempty"`
    MaxSteps     int      `json:"max_steps,omitempty"`
}
```

- [ ] **Step 2: Add Agents field to Config**:

```go
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
```

- [ ] **Step 3: Add merge logic** after MCPServers merge in `mergeConfig`:

```go
if global.Agents == nil {
    global.Agents = make(map[string]AgentConfig)
}
for key, projAgent := range project.Agents {
    existing, ok := global.Agents[key]
    if !ok {
        global.Agents[key] = projAgent
        continue
    }
    if projAgent.Description != "" {
        existing.Description = projAgent.Description
    }
    if projAgent.Prompt != "" {
        existing.Prompt = projAgent.Prompt
    }
    if len(projAgent.Instructions) > 0 {
        existing.Instructions = projAgent.Instructions
    }
    if len(projAgent.Tools) > 0 {
        existing.Tools = projAgent.Tools
    }
    if projAgent.MaxSteps != 0 {
        existing.MaxSteps = projAgent.MaxSteps
    }
    global.Agents[key] = existing
}
```

- [ ] **Step 4: Update defaultMaxSteps** from 1000 to 10000:

```go
const (
    defaultMaxSteps = 10000
)
```

- [ ] **Step 5: Build check**

```bash
go build ./...
```

- [ ] **Step 6: Commit**

```bash
git add internal/config/config.go
git commit -m "feat(config): add agents config with AgentConfig type and merge"
```

---

### Task 3: Registry — Add GetSpec + Names methods

**Files:**
- Modify: `internal/tools/registry.go`
- Test: `internal/tools/registry_test.go`

- [ ] **Step 1: Add GetSpec and Names to Registry**

Add to `internal/tools/registry.go`:

```go
func (r *Registry) GetSpec(name string) (Spec, bool) {
    spec, ok := r.tools[name]
    return spec, ok
}

func (r *Registry) Names() []string {
    result := make([]string, len(r.order))
    copy(result, r.order)
    return result
}
```

- [ ] **Step 2: Write tests**

Add to `internal/tools/registry_test.go`:

```go
func TestRegistryGetSpec(t *testing.T) {
    registry := NewRegistry(Spec{Name: "my_tool", Description: "does stuff"})
    spec, ok := registry.GetSpec("my_tool")
    if !ok {
        t.Fatal("expected to find my_tool")
    }
    if spec.Description != "does stuff" {
        t.Errorf("Description: got %q", spec.Description)
    }
    _, ok = registry.GetSpec("nonexistent")
    if ok {
        t.Error("expected false for nonexistent tool")
    }
}

func TestRegistryNames(t *testing.T) {
    registry := NewRegistry(Spec{Name: "z"}, Spec{Name: "a"})
    names := registry.Names()
    if len(names) != 2 || names[0] != "a" || names[1] != "z" {
        t.Errorf("Names: got %v", names)
    }
}
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/tools/ -run "TestRegistryGetSpec|TestRegistryNames" -v
```

- [ ] **Step 4: Commit**

```bash
git add internal/tools/registry.go internal/tools/registry_test.go
git commit -m "feat(tools): add GetSpec and Names methods to Registry"
```

---

### Task 4: Agent Loop — Add SystemPrompt field

**Files:**
- Modify: `internal/agent/loop.go`

- [ ] **Step 1: Add SystemPrompt field to Loop**

```go
type Loop struct {
    Client        llm.Client
    Store         session.Store
    Tools         *tools.Registry
    MaxSteps      int
    Instructions  []string
    SystemPrompt  string  // appended after built instructions, used for subagent role prompts
}
```

- [ ] **Step 2: Append SystemPrompt to instructions in loop.Run**

In `loop.Run`, after the `req := llm.Request{...}` block, modify the `Instructions` line:

Before:
```go
Instructions: prompt.Build(sess.RepoRoot, sess.Model,
    prompt.WithWorkdir(sess.Workdir),
    prompt.WithMaxSteps(l.MaxSteps),
    prompt.WithInstructionPaths(l.Instructions),
),
```

After:
```go
func (l Loop) buildInstructions(sess *session.Session) string {
    instructions := prompt.Build(sess.RepoRoot, sess.Model,
        prompt.WithWorkdir(sess.Workdir),
        prompt.WithMaxSteps(l.MaxSteps),
        prompt.WithInstructionPaths(l.Instructions),
    )
    if l.SystemPrompt != "" {
        instructions += "\n\n" + l.SystemPrompt
    }
    return instructions
}
```

Then change the `req.Instructions = ...` to use `l.buildInstructions(sess)`:

```go
req := llm.Request{
    Model:        sess.Model,
    Instructions: l.buildInstructions(sess),
    ...
}
```

- [ ] **Step 3: Build check**

```bash
go build ./...
```

- [ ] **Step 4: Commit**

```bash
git add internal/agent/loop.go
git commit -m "feat(agent): add SystemPrompt field for subagent role injection"
```

---

### Task 5: SubagentManager — Core lifecycle

**Files:**
- Create: `internal/subagent/manager.go`
- Test: `internal/subagent/manager_test.go`

- [ ] **Step 1: Create directory**

```bash
mkdir -p internal/subagent
```

- [ ] **Step 2: Write Manager struct and constructor**

Create `internal/subagent/manager.go`:

```go
package subagent

import (
    "context"
    "fmt"
    "strings"
    "sync"
    "time"

    "minioc/internal/agent"
    "minioc/internal/agent/prompt"
    "minioc/internal/config"
    "minioc/internal/llm"
    "minioc/internal/notify"
    "minioc/internal/safety"
    "minioc/internal/session"
    "minioc/internal/tools"
)

type Result struct {
    AgentID   string
    SessionID string
    Status    string
    Output    string
}

type AgentInfo struct {
    AgentID       string `json:"agent_id"`
    AgentType     string `json:"agent_type"`
    Status        string `json:"status"`
    ElapsedMillis int64  `json:"elapsed_millis"`
    StepCount     int    `json:"step_count"`
}

type runningAgent struct {
    id        string
    typ       string
    sess      *session.Session
    cancel    context.CancelFunc
    started   time.Time
    stepCount int
    result    *Result
}

type Manager struct {
    client       llm.Client
    store        session.Store
    tools        *tools.Registry
    agentCfgs    map[string]config.AgentConfig
    permissions  *safety.PermissionManager
    defaultMaxSteps int
    repoRoot     string
    workdir      string
    model        string
    parentSessionID string

    mu      sync.RWMutex
    running map[string]*runningAgent
}

func NewManager(
    client llm.Client,
    store session.Store,
    tools *tools.Registry,
    agentCfgs map[string]config.AgentConfig,
    permissions *safety.PermissionManager,
    defaultMaxSteps int,
    repoRoot, workdir, model, parentSessionID string,
) *Manager {
    if agentCfgs == nil {
        agentCfgs = make(map[string]config.AgentConfig)
    }
    return &Manager{
        client:       client,
        store:        store,
        tools:        tools,
        agentCfgs:    agentCfgs,
        permissions:  permissions,
        defaultMaxSteps: defaultMaxSteps,
        repoRoot:     repoRoot,
        workdir:      workdir,
        model:        model,
        parentSessionID: parentSessionID,
        running:      make(map[string]*runningAgent),
    }
}
```

- [ ] **Step 3: Add Spawn method**

```go
var managementTools = map[string]bool{
    "spawn_agent":       true,
    "list_agents":       true,
    "get_agent_session": true,
    "kill_agent":        true,
}

func (m *Manager) Spawn(ctx context.Context, agentType, task string, maxSteps int) Result {
    cfg, ok := m.agentCfgs[agentType]
    if !ok {
        return Result{Status: "error", Output: fmt.Sprintf("unknown agent type %q", agentType)}
    }

    sess := session.NewSubagent(m.repoRoot, m.workdir, m.model, m.parentSessionID)
    subRegistry := m.filterRegistry(cfg.Tools)
    sysPrompt := m.buildPrompt(cfg)
    steps := m.resolveMaxSteps(cfg, maxSteps)

    agentCtx, cancel := context.WithCancel(ctx)
    ra := &runningAgent{
        id:      sess.ID,
        typ:     agentType,
        sess:    sess,
        cancel:  cancel,
        started: time.Now(),
    }
    m.mu.Lock()
    m.running[sess.ID] = ra
    m.mu.Unlock()

    notify.Emit(notify.Event{
        Type:   "subagent:spawned",
        Source: "subagent",
        Payload: map[string]any{
            "agent_id":   sess.ID,
            "agent_type": agentType,
        },
    })

    loop := agent.Loop{
        Client:       m.client,
        Store:        m.store,
        Tools:        subRegistry,
        MaxSteps:     steps,
        Instructions: cfg.Instructions,
        SystemPrompt: sysPrompt,
    }

    hooks := &agent.Hooks{
        OnToolResult: func(call llm.ToolCall, status, output string) {
            ra.stepCount++
            notify.Emit(notify.Event{
                Type:   "subagent:step",
                Source: "subagent",
                Payload: map[string]any{
                    "agent_id":      sess.ID,
                    "step_count":    ra.stepCount,
                    "message_count": len(sess.Messages),
                },
            })
        },
    }

    answer, err := loop.Run(agentCtx, sess, m.permissions, task, hooks)
    cancel()

    var result Result
    if err != nil {
        result = Result{AgentID: sess.ID, SessionID: sess.ID, Status: "error", Output: err.Error()}
        notify.Emit(notify.Event{
            Type: "subagent:error", Source: "subagent",
            Payload: map[string]any{"agent_id": sess.ID, "error": err.Error()},
        })
    } else {
        result = Result{AgentID: sess.ID, SessionID: sess.ID, Status: "completed", Output: answer}
        notify.Emit(notify.Event{
            Type: "subagent:completed", Source: "subagent",
            Payload: map[string]any{"agent_id": sess.ID},
        })
    }

    m.mu.Lock()
    delete(m.running, sess.ID)
    m.mu.Unlock()
    return result
}
```

- [ ] **Step 4: Add helper methods**

```go
func (m *Manager) filterRegistry(allowedTools []string) *tools.Registry {
    if len(allowedTools) == 0 {
        return m.tools
    }
    allowed := make(map[string]bool, len(allowedTools))
    for _, name := range allowedTools {
        allowed[name] = true
    }
    var specs []tools.Spec
    for _, name := range m.tools.Names() {
        if allowed[name] && !managementTools[name] {
            spec, _ := m.tools.GetSpec(name)
            specs = append(specs, spec)
        }
    }
    return tools.NewRegistry(specs...)
}

func (m *Manager) buildPrompt(cfg config.AgentConfig) string {
    var parts []string
    if cfg.Prompt != "" {
        parts = append(parts, cfg.Prompt)
    }
    if len(cfg.Instructions) > 0 {
        content := prompt.Load(cfg.Instructions)
        if content != "" {
            parts = append(parts, content)
        }
    }
    return strings.Join(parts, "\n\n")
}

func (m *Manager) resolveMaxSteps(cfg config.AgentConfig, callMaxSteps int) int {
    if callMaxSteps > 0 {
        return callMaxSteps
    }
    if cfg.MaxSteps > 0 {
        return cfg.MaxSteps
    }
    if m.defaultMaxSteps > 0 {
        return m.defaultMaxSteps
    }
    return 10000
}
```

- [ ] **Step 5: Add accessor methods**

```go
func (m *Manager) Agents() []AgentInfo {
    m.mu.RLock()
    defer m.mu.RUnlock()
    infos := make([]AgentInfo, 0, len(m.running))
    for _, ra := range m.running {
        status := "running"
        if ra.result != nil {
            status = ra.result.Status
        }
        infos = append(infos, AgentInfo{
            AgentID:       ra.id,
            AgentType:     ra.typ,
            Status:        status,
            ElapsedMillis: time.Since(ra.started).Milliseconds(),
            StepCount:     ra.stepCount,
        })
    }
    return infos
}

func (m *Manager) GetAgent(agentID string) *session.Session {
    m.mu.RLock()
    defer m.mu.RUnlock()
    ra, ok := m.running[agentID]
    if !ok {
        return nil
    }
    return ra.sess
}

func (m *Manager) Kill(agentID string) error {
    m.mu.RLock()
    ra, ok := m.running[agentID]
    m.mu.RUnlock()
    if !ok {
        return fmt.Errorf("agent %s not found", agentID)
    }
    ra.cancel()
    return nil
}

func (m *Manager) AgentConfigs() map[string]config.AgentConfig {
    return m.agentCfgs
}
```

- [ ] **Step 6: Write tests**

Create `internal/subagent/manager_test.go`:

```go
package subagent

import (
    "testing"
    "minioc/internal/config"
)

func TestNewManager(t *testing.T) {
    m := NewManager(nil, nil, nil, nil, nil, 10000, "/r", "/r", "gpt-4", "")
    if m == nil {
        t.Fatal("expected non-nil manager")
    }
    if len(m.AgentConfigs()) != 0 {
        t.Errorf("expected empty configs")
    }
}

func TestManagerAgentConfigs(t *testing.T) {
    cfgs := map[string]config.AgentConfig{"explore": {Description: "x", MaxSteps: 5}}
    m := NewManager(nil, nil, nil, cfgs, nil, 10000, "/r", "/r", "gpt-4", "")
    if len(m.AgentConfigs()) != 1 {
        t.Errorf("expected 1 config")
    }
}

func TestManagerAgentsEmpty(t *testing.T) {
    m := NewManager(nil, nil, nil, nil, nil, 10000, "/r", "/r", "gpt-4", "")
    agents := m.Agents()
    if len(agents) != 0 {
        t.Errorf("expected empty, got %d", len(agents))
    }
}

func TestManagerKillNotFound(t *testing.T) {
    m := NewManager(nil, nil, nil, nil, nil, 10000, "/r", "/r", "gpt-4", "")
    err := m.Kill("nonexistent")
    if err == nil {
        t.Fatal("expected error")
    }
}

func TestResolveMaxSteps(t *testing.T) {
    m := NewManager(nil, nil, nil, nil, nil, 10000, "/r", "/r", "gpt-4", "")
    if v := m.resolveMaxSteps(config.AgentConfig{MaxSteps: 5}, 20); v != 20 {
        t.Errorf("call param: got %d", v)
    }
    if v := m.resolveMaxSteps(config.AgentConfig{MaxSteps: 5}, 0); v != 5 {
        t.Errorf("cfg: got %d", v)
    }
    if v := m.resolveMaxSteps(config.AgentConfig{}, 0); v != 10000 {
        t.Errorf("default: got %d", v)
    }
}
```

- [ ] **Step 7: Run tests**

```bash
go test ./internal/subagent/ -v
```
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add internal/subagent/
git commit -m "feat(subagent): add Manager with Spawn, Agents, Kill, GetAgent"
```

---

### Task 6: Subagent tools — spawn_agent, list_agents, get_agent_session, kill_agent

**Files:**
- Create: `internal/subagent/tools.go`

- [ ] **Step 1: Write tool factory functions**

Create `internal/subagent/tools.go`:

```go
package subagent

import (
    "context"
    "encoding/json"
    "fmt"

    "minioc/internal/tools"
)

func SpawnAgentTool(mgr *Manager) tools.Spec {
    return tools.Spec{
        Name:        "spawn_agent",
        Description: "Spawn a subagent with an isolated context to execute a task. Blocks until the subagent completes. Multiple spawn_agent calls in one response run concurrently.",
        ParallelSafe: true,
        Parameters: tools.ObjectSchema(map[string]any{
            "agent":     map[string]any{"type": "string", "description": "Agent type name from minioc.json agents config"},
            "task":      map[string]any{"type": "string", "description": "Task description for the subagent"},
            "max_steps": map[string]any{"type": "integer", "description": "Max agentic steps (overrides config)"},
        }, "agent", "task"),
        Execute: func(ctx context.Context, callCtx tools.CallContext, raw json.RawMessage) (tools.Result, error) {
            var args struct {
                Agent    string `json:"agent"`
                Task     string `json:"task"`
                MaxSteps int    `json:"max_steps,omitempty"`
            }
            if err := json.Unmarshal(raw, &args); err != nil {
                return tools.Result{}, fmt.Errorf("decode spawn_agent args: %w", err)
            }
            result := mgr.Spawn(ctx, args.Agent, args.Task, args.MaxSteps)
            output := fmt.Sprintf("Subagent [%s] %s:\n%s", result.AgentID, result.Status, result.Output)
            if result.Status == "error" {
                return tools.Result{Title: "spawn_agent error", Output: output}, fmt.Errorf("%s", output)
            }
            return tools.Result{Title: fmt.Sprintf("spawn_agent %s", result.AgentID), Output: output}, nil
        },
    }
}

func ListAgentsTool(mgr *Manager) tools.Spec {
    return tools.Spec{
        Name:         "list_agents",
        Description:  "List all subagents and their status (running/completed/killed)",
        ParallelSafe: false,
        Parameters:   tools.ObjectSchema(nil),
        Execute: func(_ context.Context, _ tools.CallContext, _ json.RawMessage) (tools.Result, error) {
            agents := mgr.Agents()
            if len(agents) == 0 {
                return tools.Result{Output: "(no subagents)"}, nil
            }
            out := ""
            for _, a := range agents {
                out += fmt.Sprintf("- %s (%s): %s, %d steps, %dms\n",
                    a.AgentID, a.AgentType, a.Status, a.StepCount, a.ElapsedMillis)
            }
            return tools.Result{Title: "Subagents", Output: out}, nil
        },
    }
}

func GetAgentSessionTool(mgr *Manager) tools.Spec {
    return tools.Spec{
        Name:         "get_agent_session",
        Description:  "Get the current session messages of a running or completed subagent",
        ParallelSafe: false,
        Parameters: tools.ObjectSchema(map[string]any{
            "agent_id": map[string]any{"type": "string", "description": "Subagent ID"},
        }, "agent_id"),
        Execute: func(_ context.Context, _ tools.CallContext, raw json.RawMessage) (tools.Result, error) {
            var args struct {
                AgentID string `json:"agent_id"`
            }
            if err := json.Unmarshal(raw, &args); err != nil {
                return tools.Result{}, fmt.Errorf("decode get_agent_session args: %w", err)
            }
            sess := mgr.GetAgent(args.AgentID)
            if sess == nil {
                return tools.Result{}, fmt.Errorf("agent %s not found", args.AgentID)
            }
            data, _ := json.MarshalIndent(sess, "", "  ")
            return tools.Result{Title: fmt.Sprintf("Session %s", args.AgentID), Output: string(data)}, nil
        },
    }
}

func KillAgentTool(mgr *Manager) tools.Spec {
    return tools.Spec{
        Name:         "kill_agent",
        Description:  "Cancel a running subagent",
        ParallelSafe: false,
        Parameters: tools.ObjectSchema(map[string]any{
            "agent_id": map[string]any{"type": "string", "description": "Subagent ID to cancel"},
        }, "agent_id"),
        Execute: func(_ context.Context, _ tools.CallContext, raw json.RawMessage) (tools.Result, error) {
            var args struct {
                AgentID string `json:"agent_id"`
            }
            if err := json.Unmarshal(raw, &args); err != nil {
                return tools.Result{}, fmt.Errorf("decode kill_agent args: %w", err)
            }
            if err := mgr.Kill(args.AgentID); err != nil {
                return tools.Result{}, fmt.Errorf("kill %s: %w", args.AgentID, err)
            }
            return tools.Result{Output: fmt.Sprintf("Agent %s killed", args.AgentID)}, nil
        },
    }
}
```

Note: `tools.ObjectSchema` doesn't exist yet — the helper is in `tools/schema.go` as `objectSchema` (unexported). Add an exported wrapper:

In `internal/tools/schema.go`:

```go
func ObjectSchema(properties map[string]any, required ...string) map[string]any {
    return objectSchema(properties, required...)
}
```

- [ ] **Step 2: Build check**

```bash
go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add internal/subagent/tools.go internal/tools/schema.go
git commit -m "feat(subagent): add spawn_agent, list_agents, get_agent_session, kill_agent tools"
```

---

### Task 7: Wire up in main.go

**Files:**
- Modify: `cmd/minioc/main.go`

- [ ] **Step 1: Create Manager and register tools**

In `cmd/minioc/main.go`, add import for `"minioc/internal/subagent"`. After the MCP tool registration block, add:

```go
subagentMgr := subagent.NewManager(
    client,
    sessionStore,
    registry,
    cfg.Agents,
    permissionManager,
    cfg.MaxSteps,
    repoRoot,
    workdir,
    cfg.Model,
    current.ID,
)
registry.Register(subagent.SpawnAgentTool(subagentMgr))
registry.Register(subagent.ListAgentsTool(subagentMgr))
registry.Register(subagent.GetAgentSessionTool(subagentMgr))
registry.Register(subagent.KillAgentTool(subagentMgr))
```

Place this after the MCP registration block (after `mcpManager.StartAll` + `mcpManager.RegisterTools`) and before the `loop := agent.Loop{...}` block.

- [ ] **Step 2: Build check**

```bash
go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add cmd/minioc/main.go
git commit -m "feat(cmd): wire up SubagentManager and register subagent tools"
```

---

### Task 8: TUI — Subagent progress panel

**Files:**
- Modify: `internal/tui/types.go`
- Modify: `internal/tui/runtime_loop.go`
- Modify: `internal/tui/render_layout.go`

- [ ] **Step 1: Add subagent state to TUI model**

In `internal/tui/types.go`, add import for `"minioc/internal/subagent"` and add fields to TUI state struct (locate the main model struct):

```go
type model struct {
    // ... existing fields
    subagentMgr  *subagent.Manager
    subagents    []subagent.AgentInfo
    focusAgent   string // agent_id to view detail, empty = list view
}
```

- [ ] **Step 2: Subscribe to subagent events in runtime setup**

In `internal/tui/runtime_loop.go`, after model creation, add:

```go
notify.On("subagent:*", func(e notify.Event) {
    m.subagents = m.subagentMgr.Agents()
    if id, _ := e.Payload.(map[string]any)["agent_id"]; id != nil {
        m.focusAgent = id.(string)
    }
})
```

- [ ] **Step 3: Render subagent panel**

In `internal/tui/render_layout.go`, add a subagent panel section (add a function called from the main layout render):

```go
func renderSubagents(m model) string {
    if len(m.subagents) == 0 {
        return ""
    }
    var b strings.Builder
    b.WriteString("\n\n── Subagents ──\n")
    for _, a := range m.subagents {
        b.WriteString(fmt.Sprintf("  %s [%s] %s  steps=%d  %dms\n",
            a.AgentID, a.AgentType, a.Status, a.StepCount, a.ElapsedMillis))
    }
    return b.String()
}
```

Call this from the main layout function.

- [ ] **Step 4: Pass subagentMgr to TUI**

In `cmd/minioc/main.go`, find the `tui.Run` or `tui.Config` call. Add `SubagentMgr` field to the config:

```go
type Config struct {
    RepoRoot    string
    Workdir     string
    Model       string
    Prompt      string
    Loop        agent.Loop
    Session     *session.Session
    AutoApprove bool
    SubagentMgr *subagent.Manager
}
```

- [ ] **Step 5: Build check**

```bash
go build ./...
```

- [ ] **Step 6: Commit**

```bash
git add internal/tui/ cmd/minioc/main.go
git commit -m "feat(tui): add subagent progress panel with event-driven updates"
```
