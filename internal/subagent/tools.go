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
		Parameters: tools.ObjectSchema(map[string]any{
			"agent":     map[string]any{"type": "string", "description": "Agent type name from minioc.json agents config"},
			"task":      map[string]any{"type": "string", "description": "Task description for the subagent"},
			"max_steps": map[string]any{"type": "integer", "description": "Max agentic steps (overrides config)"},
		}, "agent", "task"),
		ParallelSafe: true,
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
				return tools.Result{Title: fmt.Sprintf("spawn_agent %s error", result.AgentID), Output: output}, fmt.Errorf("%s", output)
			}
			return tools.Result{Title: fmt.Sprintf("spawn_agent %s", result.AgentID), Output: output}, nil
		},
	}
}

func ListAgentsTool(mgr *Manager) tools.Spec {
	return tools.Spec{
		Name:        "list_agents",
		Description: "List all subagents and their status (running/completed/killed)",
		Parameters:  tools.ObjectSchema(nil),
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
		Name:        "get_agent_session",
		Description: "Get the current session messages of a running or completed subagent",
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
			out := fmt.Sprintf("Agent: %s\nModel: %s\nMessages: %d\n", sess.ID, sess.Model, len(sess.Messages))
			if len(sess.Messages) > 0 {
				out += "\n--- Messages ---\n"
				for i, msg := range sess.Messages {
					preview := msg.Content
					if len(preview) > 200 {
						preview = preview[:200] + "..."
					}
					out += fmt.Sprintf("[%d] %s: %s\n", i+1, msg.Role, preview)
					if msg.ToolName != "" {
						out += fmt.Sprintf("     tool: %s (status: %s)\n", msg.ToolName, msg.Status)
					}
				}
			}
			return tools.Result{Title: fmt.Sprintf("Session %s", args.AgentID), Output: out}, nil
		},
	}
}

func KillAgentTool(mgr *Manager) tools.Spec {
	return tools.Spec{
		Name:        "kill_agent",
		Description: "Cancel a running subagent",
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
