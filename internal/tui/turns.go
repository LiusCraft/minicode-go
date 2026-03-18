package tui

import (
	"fmt"
	"strings"
)

func latestAssistantPreview(turn turnEntry, includeDraft bool, draft string) (int, string, int) {
	count := 0
	idx := -1
	text := ""
	for eventIdx, event := range turn.Events {
		if event.Kind != turnEventAssistant {
			continue
		}
		count++
		idx = eventIdx
		text = event.Text
	}
	if includeDraft && strings.TrimSpace(draft) != "" {
		count++
		idx = len(turn.Events)
		text = strings.TrimSpace(draft)
	}
	return idx, text, count
}

func latestToolPreview(turn turnEntry) (int, toolEntry, int) {
	count := 0
	idx := -1
	var tool toolEntry
	for eventIdx, event := range turn.Events {
		if event.Kind != turnEventTool {
			continue
		}
		count++
		idx = eventIdx
		tool = event.Tool
	}
	return idx, tool, count
}

func formatTurnCollapseSummary(hiddenAssistant, hiddenTools int) string {
	switch {
	case hiddenAssistant > 0 && hiddenTools > 0:
		return fmt.Sprintf("Earlier in this turn, there are %d thinking step(s) and %d tool call(s) hidden. Press ctrl+t to view the full trace.", hiddenAssistant, hiddenTools)
	case hiddenAssistant > 0:
		return fmt.Sprintf("Earlier in this turn, there are %d thinking step(s) hidden. Press ctrl+t to view the full trace.", hiddenAssistant)
	case hiddenTools > 0:
		return fmt.Sprintf("Earlier in this turn, there are %d tool call(s) hidden. Press ctrl+t to view the full trace.", hiddenTools)
	default:
		return ""
	}
}

func (m *model) lastAssistantMatches(text string) bool {
	if len(m.turns) == 0 {
		return false
	}
	turn := m.turns[len(m.turns)-1]
	for i := len(turn.Events) - 1; i >= 0; i-- {
		if turn.Events[i].Kind != turnEventAssistant {
			continue
		}
		return strings.TrimSpace(turn.Events[i].Text) == strings.TrimSpace(text)
	}
	return false
}

func (m *model) ensureTurn(current int) {
	if current >= 0 && current < len(m.turns) {
		return
	}
	m.turns = append(m.turns, turnEntry{})
}

func (m *model) appendTurnAssistant(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	m.ensureTurn(len(m.turns) - 1)
	idx := len(m.turns) - 1
	m.turns[idx].Events = append(m.turns[idx].Events, turnEvent{Kind: turnEventAssistant, Text: text})
}

func (m *model) appendTurnTool(entry toolEntry) {
	m.ensureTurn(len(m.turns) - 1)
	idx := len(m.turns) - 1
	m.turns[idx].Events = append(m.turns[idx].Events, turnEvent{Kind: turnEventTool, Tool: entry})
}

func (m *model) findTool(id string) (int, int) {
	for turnIdx := len(m.turns) - 1; turnIdx >= 0; turnIdx-- {
		for eventIdx := len(m.turns[turnIdx].Events) - 1; eventIdx >= 0; eventIdx-- {
			event := m.turns[turnIdx].Events[eventIdx]
			if event.Kind == turnEventTool && event.Tool.ID == id {
				return turnIdx, eventIdx
			}
		}
	}
	return -1, -1
}
