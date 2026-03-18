package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"minioc/internal/agent/prompt"
	"minioc/internal/llm"
	"minioc/internal/safety"
	"minioc/internal/session"
	"minioc/internal/store"
	"minioc/internal/tools"
)

type Loop struct {
	Client   llm.Client
	Store    store.Store
	Tools    *tools.Registry
	MaxSteps int
}

type Hooks struct {
	OnAssistantDelta       func(string)
	OnAssistantMessageDone func()
	OnToolCall             func(llm.ToolCall)
	OnToolResult           func(call llm.ToolCall, status, output string)
}

type toolExecution struct {
	Status string
	Output string
}

const llmStepTimeout = 90 * time.Second

func (l Loop) Run(ctx context.Context, sess *session.Session, permissions *safety.PermissionManager, userInput string, hooks *Hooks) (string, error) {
	if strings.TrimSpace(userInput) == "" {
		return "", fmt.Errorf("user input is empty")
	}
	if l.MaxSteps <= 0 {
		return "", fmt.Errorf("max steps must be greater than zero")
	}

	sess.AddMessage(session.RoleUser, userInput)
	if err := l.Store.Save(ctx, sess); err != nil {
		return "", err
	}

	for step := 0; step < l.MaxSteps; step++ {
		var streamHandler *llm.StreamHandler
		if hooks != nil {
			streamHandler = &llm.StreamHandler{
				OnTextDelta:   hooks.OnAssistantDelta,
				OnMessageDone: hooks.OnAssistantMessageDone,
			}
		}

		req := llm.Request{
			Model:        sess.Model,
			Instructions: prompt.Build(sess.RepoRoot, sess.Workdir, sess.Model, l.MaxSteps),
			Messages:     toLLMMessages(sess.Messages),
			Tools:        l.Tools.Definitions(),
			Stream:       streamHandler,
		}

		stepCtx, cancel := context.WithTimeout(ctx, llmStepTimeout)
		result, err := l.Client.Run(stepCtx, req)
		cancel()
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				return "", fmt.Errorf("model request timed out after %s", llmStepTimeout)
			}
			return "", err
		}

		if len(result.ToolCalls) == 0 {
			answer := strings.TrimSpace(result.Text)
			if answer == "" {
				answer = "Done."
			}
			sess.AddMessage(session.RoleAssistant, answer)
			if err := l.Store.Save(ctx, sess); err != nil {
				return "", err
			}
			return answer, nil
		}

		sess.AddMessage(session.RoleAssistant, strings.TrimSpace(result.Text), session.WithAssistantToolCalls(toSessionToolCalls(result.ToolCalls)))

		executions := l.executeToolCalls(ctx, result.ToolCalls, tools.CallContext{
			RepoRoot:    sess.RepoRoot,
			Workdir:     sess.Workdir,
			Permissions: permissions,
		}, hooks)
		for i, call := range result.ToolCalls {
			execution := executions[i]
			sess.AddMessage(session.RoleTool, execution.Output, session.WithTool(call.Name, call.ID, execution.Status))
		}

		if err := l.Store.Save(ctx, sess); err != nil {
			return "", err
		}
	}

	return "", fmt.Errorf("reached max steps (%d)", l.MaxSteps)
}

func (l Loop) executeToolCalls(ctx context.Context, calls []llm.ToolCall, callCtx tools.CallContext, hooks *Hooks) []toolExecution {
	results := make([]toolExecution, len(calls))
	for start := 0; start < len(calls); {
		if !l.Tools.IsParallelSafe(calls[start].Name) {
			results[start] = l.executeToolCall(ctx, calls[start], callCtx, hooks)
			start++
			continue
		}

		end := start + 1
		for end < len(calls) && l.Tools.IsParallelSafe(calls[end].Name) {
			end++
		}

		if end-start == 1 {
			results[start] = l.executeToolCall(ctx, calls[start], callCtx, hooks)
			start = end
			continue
		}

		batch := l.executeParallelToolCalls(ctx, calls[start:end], callCtx, hooks)
		copy(results[start:end], batch)
		start = end
	}
	return results
}

func (l Loop) executeParallelToolCalls(ctx context.Context, calls []llm.ToolCall, callCtx tools.CallContext, hooks *Hooks) []toolExecution {
	type completedToolCall struct {
		index     int
		execution toolExecution
	}

	results := make([]toolExecution, len(calls))
	completed := make(chan completedToolCall, len(calls))
	var wg sync.WaitGroup

	for i, call := range calls {
		if hooks != nil && hooks.OnToolCall != nil {
			hooks.OnToolCall(call)
		}

		wg.Add(1)
		go func(index int, call llm.ToolCall) {
			defer wg.Done()
			completed <- completedToolCall{
				index:     index,
				execution: l.runToolCall(ctx, call, callCtx),
			}
		}(i, call)
	}

	go func() {
		wg.Wait()
		close(completed)
	}()

	for item := range completed {
		results[item.index] = item.execution
		if hooks != nil && hooks.OnToolResult != nil {
			hooks.OnToolResult(calls[item.index], item.execution.Status, item.execution.Output)
		}
	}

	return results
}

func (l Loop) executeToolCall(ctx context.Context, call llm.ToolCall, callCtx tools.CallContext, hooks *Hooks) toolExecution {
	if hooks != nil && hooks.OnToolCall != nil {
		hooks.OnToolCall(call)
	}

	execution := l.runToolCall(ctx, call, callCtx)
	if hooks != nil && hooks.OnToolResult != nil {
		hooks.OnToolResult(call, execution.Status, execution.Output)
	}
	return execution
}

func (l Loop) runToolCall(ctx context.Context, call llm.ToolCall, callCtx tools.CallContext) toolExecution {
	toolResult, toolErr := l.Tools.Execute(ctx, call.Name, call.Arguments, callCtx)
	status := "completed"
	output := toolResult.Output
	if toolErr != nil {
		status = "error"
		output = "ERROR: " + toolErr.Error()
	}
	if strings.TrimSpace(output) == "" {
		output = "(no output)"
	}
	return toolExecution{Status: status, Output: output}
}

func toLLMMessages(messages []session.Message) []llm.Message {
	result := make([]llm.Message, 0, len(messages))
	for _, message := range messages {
		item := llm.Message{
			Role:       string(message.Role),
			Content:    message.Content,
			ToolName:   message.ToolName,
			ToolCallID: message.ToolCallID,
			Status:     message.Status,
		}
		if len(message.ToolCalls) > 0 {
			item.ToolCalls = make([]llm.ToolCall, 0, len(message.ToolCalls))
			for _, call := range message.ToolCalls {
				item.ToolCalls = append(item.ToolCalls, llm.ToolCall{
					ID:        call.ID,
					Name:      call.Name,
					Arguments: append(json.RawMessage(nil), call.Arguments...),
				})
			}
		}
		result = append(result, item)
	}
	return result
}

func toSessionToolCalls(calls []llm.ToolCall) []session.ToolCall {
	result := make([]session.ToolCall, 0, len(calls))
	for _, call := range calls {
		result = append(result, session.ToolCall{
			ID:        call.ID,
			Name:      call.Name,
			Arguments: append(json.RawMessage(nil), call.Arguments...),
		})
	}
	return result
}
