package tui

import "strings"

func (m *model) renderTurnExpanded(width int, turn turnEntry, includeDraft bool) string {
	blocks := []string{}
	if strings.TrimSpace(turn.User) != "" {
		blocks = append(blocks, m.renderPromptPreview(width, turn.User))
	}
	for _, event := range turn.Events {
		switch event.Kind {
		case turnEventAssistant:
			blocks = append(blocks, m.renderAssistantBlock(width, event.Text))
		case turnEventTool:
			blocks = append(blocks, m.renderToolEntry(width, event.Tool, true))
		}
	}
	if includeDraft {
		blocks = append(blocks, m.renderAssistantBlock(width, m.assistantDraft))
	}
	return strings.Join(blocks, "\n\n")
}

func (m *model) renderTurnCollapsed(width int, turn turnEntry, includeDraft bool) string {
	blocks := []string{}
	if strings.TrimSpace(turn.User) != "" {
		blocks = append(blocks, m.renderPromptPreview(width, turn.User))
	}

	assistantIdx, assistantText, assistantCount := latestAssistantPreview(turn, includeDraft, m.assistantDraft)
	toolIdx, tool, toolCount := latestToolPreview(turn)
	hiddenAssistant := assistantCount
	hiddenTools := toolCount
	if assistantIdx >= 0 {
		hiddenAssistant--
	}
	if toolIdx >= 0 {
		hiddenTools--
	}
	if hiddenAssistant > 0 || hiddenTools > 0 {
		blocks = append(blocks, m.renderMetaParagraph(width, ".. ", m.styles.dim, formatTurnCollapseSummary(hiddenAssistant, hiddenTools)))
	}

	type previewBlock struct {
		idx   int
		block string
	}
	previews := make([]previewBlock, 0, 2)
	if toolIdx >= 0 {
		previews = append(previews, previewBlock{idx: toolIdx, block: m.renderToolEntry(width, tool, false)})
	}
	if assistantIdx >= 0 && strings.TrimSpace(assistantText) != "" {
		previews = append(previews, previewBlock{idx: assistantIdx, block: m.renderAssistantBlock(width, assistantText)})
	}
	if len(previews) == 0 {
		if m.running {
			blocks = append(blocks, m.renderMetaParagraph(width, ".. ", m.styles.hint, "Model is still processing your request; no thinking/tool preview is available yet."))
		} else {
			blocks = append(blocks, m.renderMetaParagraph(width, ".. ", m.styles.hint, "No thinking/tool steps were recorded for this turn."))
		}
	} else {
		if len(previews) == 2 && previews[0].idx > previews[1].idx {
			previews[0], previews[1] = previews[1], previews[0]
		}
		for _, preview := range previews {
			blocks = append(blocks, preview.block)
		}
	}

	return strings.Join(blocks, "\n\n")
}

func (m *model) renderAssistantBlock(width int, text string) string {
	head := m.fillLine(
		m.styles.assistantMarker.Render("o")+
			m.spaceFill(1, m.styles.screenFill)+
			m.styles.assistantLabel.Render("assistant"),
		width,
		m.styles.screenFill,
	)
	body := m.renderParagraph(width, 2, m.styles.assistant, text)
	return strings.Join([]string{head, body}, "\n")
}
