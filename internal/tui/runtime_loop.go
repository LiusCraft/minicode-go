package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"minioc/internal/agent"
	"minioc/internal/llm"
	"minioc/internal/session"
)

func (m *model) startRun(prompt string) {
	if strings.TrimSpace(prompt) == "" {
		return
	}
	if m.loop.Client == nil || m.loop.Store == nil || m.loop.Tools == nil {
		m.lastError = "TUI loop is not configured"
		m.statusText = "Error"
		return
	}

	m.running = true
	m.lastError = ""
	m.statusText = "Thinking..."
	m.turns = append(m.turns, turnEntry{User: prompt})
	runCtx, cancel := context.WithCancel(context.Background())
	m.runCancel = cancel
	go m.runLoop(runCtx, prompt)
}

func (m *model) runLoop(ctx context.Context, prompt string) {
	hooks := &agent.Hooks{
		OnAssistantDelta: func(text string) {
			if text == "" {
				return
			}
			m.emitDelta(deltaEvent{Text: text})
		},
		OnAssistantMessageDone: func() {
			m.emitDelta(deltaEvent{Done: true})
		},
		OnToolCall: func(call llm.ToolCall) {
			m.emit(toolCallMsg{Call: call})
		},
		OnToolResult: func(call llm.ToolCall, status, output string) {
			m.emit(toolResultMsg{Call: call, Status: status, Output: output})
		},
	}

	answer, err := m.loop.Run(ctx, m.sess, m.permissions, prompt, hooks)
	m.emit(runFinishedMsg{Answer: strings.TrimSpace(answer), Err: err})
}

func (m *model) finishRun(msg runFinishedMsg) {
	m.commitAssistantDraft()
	if m.runCancel != nil {
		m.runCancel = nil
	}
	m.running = false

	if msg.Err != nil {
		if errors.Is(msg.Err, context.Canceled) {
			m.statusText = "Cancelled"
			m.lastError = ""
			return
		}
		m.statusText = "Error"
		m.lastError = msg.Err.Error()
		return
	}

	if msg.Answer != "" && !m.lastAssistantMatches(msg.Answer) {
		m.appendTurnAssistant(msg.Answer)
	}
	m.statusText = "Ready"
	m.lastError = ""
}

func (m *model) appendAssistantDelta(text string) {
	m.assistantDraft += text
	if m.running && m.pendingPermission == nil {
		m.statusText = "Responding..."
	}
}

func (m *model) commitAssistantDraft() {
	text := strings.TrimSpace(m.assistantDraft)
	m.assistantDraft = ""
	if text == "" {
		return
	}
	m.appendTurnAssistant(text)
}

func (m *model) recordToolCall(call llm.ToolCall) {
	entry := toolEntry{
		ID:     call.ID,
		Name:   call.Name,
		Target: summarizeToolCall(call),
		Status: "running",
	}
	m.appendTurnTool(entry)
	m.statusText = "Running " + call.Name + "..."
}

func (m *model) recordToolResult(call llm.ToolCall, status, output string) {
	turnIdx, eventIdx := m.findTool(call.ID)
	if turnIdx == -1 || eventIdx == -1 {
		m.appendTurnTool(toolEntry{ID: call.ID, Name: call.Name, Target: summarizeToolCall(call)})
		turnIdx, eventIdx = m.findTool(call.ID)
	}
	if turnIdx >= 0 && eventIdx >= 0 {
		m.turns[turnIdx].Events[eventIdx].Tool.Status = normalizeStatus(status)
		m.turns[turnIdx].Events[eventIdx].Tool.Output = normalizeToolOutput(output)
		if m.turns[turnIdx].Events[eventIdx].Tool.Target == "" {
			m.turns[turnIdx].Events[eventIdx].Tool.Target = summarizeToolCall(call)
		}
	}
	if status == "error" {
		m.statusText = "Tool failed: " + call.Name
		return
	}
	m.statusText = "Tool finished: " + call.Name
}

func (m *model) requestPermission(kind, summary string) error {
	reply := make(chan error, 1)
	if !m.emit(permissionRequestMsg{Kind: kind, Summary: summary, Reply: reply}) {
		return fmt.Errorf("permission denied")
	}
	select {
	case err := <-reply:
		return err
	case <-m.shutdown:
		return fmt.Errorf("permission denied")
	}
}

func (m *model) resolvePermission(err error) {
	if m.pendingPermission == nil {
		return
	}
	select {
	case m.pendingPermission.Reply <- err:
		if err != nil {
			m.statusText = "Permission denied"
		} else {
			m.statusText = "Permission approved"
		}
	default:
	}
	m.pendingPermission = nil
}

func (m *model) emit(msg tea.Msg) bool {
	select {
	case <-m.shutdown:
		return false
	case m.externalEvents <- msg:
		return true
	}
}

func (m *model) emitDelta(event deltaEvent) bool {
	select {
	case <-m.shutdown:
		return false
	case m.deltaEvents <- event:
		return true
	}
}

func (m *model) deltaPump() {
	ticker := time.NewTicker(40 * time.Millisecond)
	defer ticker.Stop()
	var pending strings.Builder
	flush := func() {
		if pending.Len() == 0 {
			return
		}
		m.emit(assistantDeltaMsg{Text: pending.String()})
		pending.Reset()
	}

	for {
		select {
		case <-m.shutdown:
			return
		case event := <-m.deltaEvents:
			if event.Done {
				flush()
				m.emit(assistantDoneMsg{})
				continue
			}
			pending.WriteString(event.Text)
		case <-ticker.C:
			flush()
		}
	}
}

func (m *model) stop() {
	m.shutdownOnce.Do(func() {
		if m.pendingPermission != nil {
			select {
			case m.pendingPermission.Reply <- fmt.Errorf("permission denied"):
			default:
			}
		}
		if m.runCancel != nil {
			m.runCancel()
		}
		close(m.shutdown)
	})
}

func (m *model) hydrateFromSession() {
	m.turns = nil
	m.transcript = nil
	m.tools = nil
	currentTurn := -1
	callTargets := make(map[string]toolEntry)
	for _, msg := range m.sess.Messages {
		switch msg.Role {
		case session.RoleUser:
			if text := strings.TrimSpace(msg.Content); text != "" {
				m.turns = append(m.turns, turnEntry{User: text})
				currentTurn = len(m.turns) - 1
				m.transcript = append(m.transcript, chatEntry{Role: session.RoleUser, Text: text})
			}
		case session.RoleAssistant:
			if text := strings.TrimSpace(msg.Content); text != "" {
				m.ensureTurn(currentTurn)
				currentTurn = len(m.turns) - 1
				m.turns[currentTurn].Events = append(m.turns[currentTurn].Events, turnEvent{Kind: turnEventAssistant, Text: text})
				m.transcript = append(m.transcript, chatEntry{Role: session.RoleAssistant, Text: text})
			}
			for _, call := range msg.ToolCalls {
				m.ensureTurn(currentTurn)
				currentTurn = len(m.turns) - 1
				entry := toolEntry{ID: call.ID, Name: call.Name, Target: summarizeToolCall(llm.ToolCall{ID: call.ID, Name: call.Name, Arguments: call.Arguments}), Status: "running"}
				callTargets[call.ID] = entry
				m.turns[currentTurn].Events = append(m.turns[currentTurn].Events, turnEvent{Kind: turnEventTool, Tool: entry})
			}
		case session.RoleTool:
			m.ensureTurn(currentTurn)
			currentTurn = len(m.turns) - 1
			tool := callTargets[msg.ToolCallID]
			tool.ID = msg.ToolCallID
			tool.Name = msg.ToolName
			tool.Status = normalizeStatus(msg.Status)
			tool.Output = normalizeToolOutput(msg.Content)
			if tool.Target == "" {
				tool.Target = compactPreview(msg.Content, 96)
			}
			if turnIdx, eventIdx := m.findTool(msg.ToolCallID); turnIdx >= 0 && eventIdx >= 0 {
				m.turns[turnIdx].Events[eventIdx].Tool = tool
			} else {
				m.turns[currentTurn].Events = append(m.turns[currentTurn].Events, turnEvent{Kind: turnEventTool, Tool: tool})
			}
			m.tools = append(m.tools, toolEntry{
				ID:     msg.ToolCallID,
				Name:   msg.ToolName,
				Target: callTargets[msg.ToolCallID].Target,
				Status: normalizeStatus(msg.Status),
				Output: normalizeToolOutput(msg.Content),
			})
		}
	}
}

func (m *model) spinnerFrame() string {
	return spinnerFrames[m.frame%len(spinnerFrames)]
}
