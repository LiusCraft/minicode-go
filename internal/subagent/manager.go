package subagent

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
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
	Status        string `json:"status"` // "running" | "completed" | "killed"
	ElapsedMillis int64  `json:"elapsed_millis"`
	StepCount     int    `json:"step_count"`
}

type runningAgent struct {
	id        string
	typ       string
	sess      *session.Session
	cancel    context.CancelFunc
	started   time.Time
	stepCount atomic.Int64
}

type Manager struct {
	client          llm.Client
	store           session.Store
	tools           *tools.Registry
	agentCfgs       map[string]config.AgentConfig
	permissions     *safety.PermissionManager
	defaultMaxSteps int
	repoRoot        string
	workdir         string
	model           string
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
		client:          client,
		store:           store,
		tools:           tools,
		agentCfgs:       agentCfgs,
		permissions:     permissions,
		defaultMaxSteps: defaultMaxSteps,
		repoRoot:        repoRoot,
		workdir:         workdir,
		model:           model,
		parentSessionID: parentSessionID,
		running:         make(map[string]*runningAgent),
	}
}

var managementTools = map[string]bool{
	"spawn_agent":       true,
	"list_agents":       true,
	"get_agent_session": true,
	"kill_agent":        true,
}

func (m *Manager) Spawn(ctx context.Context, agentType, task string, maxSteps int) Result {
	cfg, ok := m.agentCfgs[agentType]
	if !ok || agentType == "" {
		cfg = config.AgentConfig{}
		agentType = "default"
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
		Payload: map[string]string{
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
			ra.stepCount.Add(1)
			notify.Emit(notify.Event{
				Type:   "subagent:step",
				Source: "subagent",
				Payload: map[string]any{
					"agent_id":   sess.ID,
					"step_count": ra.stepCount.Load(),
				},
			})
		},
	}

	answer, err := loop.Run(agentCtx, sess, m.permissions, task, hooks)
	cancel()

	var result Result
	if err != nil {
		result = Result{
			AgentID: sess.ID, SessionID: sess.ID,
			Status: "error", Output: err.Error(),
		}
		notify.Emit(notify.Event{
			Type: "subagent:error", Source: "subagent",
			Payload: map[string]string{"agent_id": sess.ID, "error": err.Error()},
		})
	} else {
		result = Result{
			AgentID: sess.ID, SessionID: sess.ID,
			Status: "completed", Output: answer,
		}
		notify.Emit(notify.Event{
			Type: "subagent:completed", Source: "subagent",
			Payload: map[string]string{"agent_id": sess.ID},
		})
	}

	m.mu.Lock()
	delete(m.running, sess.ID)
	m.mu.Unlock()
	return result
}

func (m *Manager) filterRegistry(allowedTools []string) *tools.Registry {
	allowed := make(map[string]bool, len(allowedTools))
	for _, name := range allowedTools {
		allowed[name] = true
	}
	var specs []tools.Spec
	for _, name := range m.tools.Names() {
		if managementTools[name] {
			continue
		}
		if len(allowedTools) > 0 && !allowed[name] {
			continue
		}
		spec, _ := m.tools.GetSpec(name)
		specs = append(specs, spec)
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
		if strings.TrimSpace(content) != "" {
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

func (m *Manager) Agents() []AgentInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	infos := make([]AgentInfo, 0, len(m.running))
	for _, ra := range m.running {
		infos = append(infos, AgentInfo{
			AgentID:       ra.id,
			AgentType:     ra.typ,
			Status:        "running",
			ElapsedMillis: time.Since(ra.started).Milliseconds(),
			StepCount:     int(ra.stepCount.Load()),
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

func (m *Manager) SetPermissions(perm *safety.PermissionManager) {
	m.permissions = perm
}

func (m *Manager) AgentConfigs() map[string]config.AgentConfig {
	result := make(map[string]config.AgentConfig, len(m.agentCfgs))
	for k, v := range m.agentCfgs {
		result[k] = v
	}
	return result
}
