package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"minioc/internal/llm"
	"minioc/internal/prompt"
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
	OnToolResult           func(name, status, output string)
}

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
			Instructions: prompt.Build(sess.RepoRoot, sess.Workdir, l.MaxSteps),
			Messages:     toLLMMessages(sess.Messages),
			Tools:        l.Tools.Definitions(),
			Stream:       streamHandler,
		}

		result, err := l.Client.Run(ctx, req)
		if err != nil {
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

		for _, call := range result.ToolCalls {
			if hooks != nil && hooks.OnToolCall != nil {
				hooks.OnToolCall(call)
			}

			toolResult, toolErr := l.Tools.Execute(ctx, call.Name, call.Arguments, tools.CallContext{
				RepoRoot:    sess.RepoRoot,
				Workdir:     sess.Workdir,
				Permissions: permissions,
			})

			status := "completed"
			output := toolResult.Output
			if toolErr != nil {
				status = "error"
				output = "ERROR: " + toolErr.Error()
			}
			if strings.TrimSpace(output) == "" {
				output = "(no output)"
			}

			if hooks != nil && hooks.OnToolResult != nil {
				hooks.OnToolResult(call.Name, status, output)
			}

			sess.AddMessage(session.RoleTool, output, session.WithTool(call.Name, call.ID, status))
		}

		if err := l.Store.Save(ctx, sess); err != nil {
			return "", err
		}
	}

	return "", fmt.Errorf("reached max steps (%d)", l.MaxSteps)
}

func toLLMMessages(messages []session.Message) []llm.Message {
	result := make([]llm.Message, 0, len(messages))
	for _, message := range messages {
		item := llm.Message{
			Role:       string(message.Role),
			Content:    message.Content,
			ToolName:   message.ToolName,
			ToolCallID: message.ToolCallID,
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
