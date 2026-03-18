package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func (m *model) renderCompactView() tea.View {
	lines := []string{
		m.fillLine(m.styles.compactTitle.Render("minioc tui"), m.compactW-4, m.styles.screenFill),
		m.fillLine(m.styles.compactBody.Render(trimLeft(m.displayPath(), max(16, m.compactW-6))), m.compactW-4, m.styles.screenFill),
		m.blankLine(m.compactW - 4),
		m.fillLine(m.styles.compactBody.Render("scene: "+m.sceneLabel()), m.compactW-4, m.styles.screenFill),
		m.fillLine(m.styles.compactBody.Render("status: "+m.statusText), m.compactW-4, m.styles.screenFill),
		m.blankLine(m.compactW - 4),
		m.fillLine(m.styles.hint.Render("Grow the terminal for the full TUI."), m.compactW-4, m.styles.screenFill),
	}
	body := lipgloss.NewStyle().Padding(1, 2).Background(lipgloss.Color("#052B33")).Render(strings.Join(lines, "\n"))
	screen := lipgloss.Place(m.compactW, m.compactH, lipgloss.Left, lipgloss.Top, body, lipgloss.WithWhitespaceStyle(m.styles.screen))
	v := tea.NewView(screen)
	v.AltScreen = true
	v.WindowTitle = "minioc TUI"
	return v
}

func (m *model) renderHeader(width int) string {
	logoRaw := []string{
		"   /\\     /\\   ",
		"  /  \\___//  \\  ",
		" |   o   o    | ",
		" |     ^      | ",
		" |   \\___/    | ",
		"  \\___________/  ",
	}
	logoWidth := 0
	for _, line := range logoRaw {
		logoWidth = max(logoWidth, lipgloss.Width(line))
	}
	logoLines := make([]string, 0, len(logoRaw))
	for _, line := range logoRaw {
		logoLines = append(logoLines, m.fillLine(m.styles.logo.Render(line), logoWidth, m.styles.screenFill))
	}

	if width < logoWidth+24 {
		stacked := make([]string, 0, len(logoLines)+4)
		stacked = append(stacked, logoLines...)
		stacked = append(stacked, m.blankLine(width))
		stacked = append(stacked, m.headerInfoLines(width)...)
		return strings.Join(stacked, "\n")
	}

	gap := 2
	infoWidth := max(18, width-logoWidth-gap)
	infoLines := m.headerInfoLines(infoWidth)
	height := max(len(logoLines), len(infoLines))
	lines := make([]string, 0, height)
	for i := range height {
		left := m.blankLine(logoWidth)
		if i < len(logoLines) {
			left = logoLines[i]
		}
		right := m.blankLine(infoWidth)
		if i < len(infoLines) {
			right = infoLines[i]
		}
		lines = append(lines, left+m.spaceFill(gap, m.styles.screenFill)+right)
	}
	return strings.Join(lines, "\n")
}

func (m *model) headerInfoLines(width int) []string {
	return []string{
		m.fillLine(m.styles.title.Render("minioc")+m.spaceFill(1, m.styles.screenFill)+m.styles.subtitle.Render("cow shell"), width, m.styles.screenFill),
		m.fillLine(m.styles.meta.Render(m.sess.Model+"  .  session "+m.sess.ID), width, m.styles.screenFill),
		m.fillLine(m.styles.meta.Render(trimLeft(m.displayPath(), max(18, width))), width, m.styles.screenFill),
	}
}

func (m *model) renderRule(width int) string {
	return m.styles.rule.Render(strings.Repeat("-", max(1, width)))
}

func (m *model) renderComposer(width int) string {
	view := strings.TrimRight(m.inputBox.View(), "\n")
	if view == "" {
		view = m.blankLine(width)
	}
	return m.fillBlock(view, width, lipgloss.Height(view), m.styles.composerFill)
}

func (m *model) renderFooter(width int) string {
	left := m.styles.footer.Render(filepath.Base(m.displayPath())) + m.spaceFill(1, m.styles.screenFill) + m.styles.footerAccent.Render("("+m.statusText+")")
	right := m.styles.footerMuted.Render("enter send  |  ctrl+j newline  |  ctrl+o history  |  ctrl+t details  |  pgup/down scroll  |  esc stop")
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		return m.fillLine(left+m.spaceFill(1, m.styles.screenFill)+right, width, m.styles.screenFill)
	}
	return left + m.spaceFill(gap, m.styles.screenFill) + right
}

func (m *model) renderSceneContent(width, height int) string {
	if m.pendingPermission != nil {
		return m.renderPermissionScene(width, height)
	}
	content := m.renderSessionScene(width)
	return m.fillBlock(content, width, height, m.styles.screenFill)
}

func (m *model) renderSessionScene(width int) string {
	blocks := []string{}
	if len(m.turns) == 0 && strings.TrimSpace(m.assistantDraft) == "" {
		blocks = append(blocks,
			m.renderMetaParagraph(width, "model: ", m.styles.meta, m.sess.Model),
			m.renderMetaParagraph(width, "workdir: ", m.styles.meta, m.displayPath()),
			m.renderMetaParagraph(width, "session: ", m.styles.meta, m.sess.ID),
			m.renderMetaParagraph(width, ".. ", m.styles.hint, "No conversation yet. Type a prompt below and press enter. Use ctrl+j for a new line."),
		)
	}
	turns := m.turns
	if len(turns) > 1 && !m.showHistory {
		hidden := len(turns) - 1
		blocks = append(blocks, m.renderMetaParagraph(width, ".. ", m.styles.dim, fmt.Sprintf("%d earlier turns hidden. Press ctrl+o to expand history.", hidden)))
		turns = turns[len(turns)-1:]
	} else if len(turns) > 1 && m.showHistory {
		blocks = append(blocks, m.renderMetaParagraph(width, ".. ", m.styles.dim, "History expanded. Press ctrl+o to collapse old turns."))
	}
	if m.showLatestDetails {
		blocks = append(blocks, m.renderMetaParagraph(width, ".. ", m.styles.dim, "Turn details expanded. Press ctrl+t to collapse thinking and tool steps."))
	} else {
		blocks = append(blocks, m.renderMetaParagraph(width, ".. ", m.styles.dim, "Thinking and tool steps are condensed per user turn. Press ctrl+t to expand."))
	}

	for i, turn := range turns {
		includeDraft := i == len(turns)-1 && strings.TrimSpace(m.assistantDraft) != ""
		if m.showLatestDetails {
			blocks = append(blocks, m.renderTurnExpanded(width, turn, includeDraft))
		} else {
			blocks = append(blocks, m.renderTurnCollapsed(width, turn, includeDraft))
		}
	}
	if m.running {
		blocks = append(blocks, m.renderMetaParagraph(width, m.spinnerFrame()+" ", m.styles.running, m.statusText))
	}
	if m.lastError != "" {
		blocks = append(blocks, m.renderMetaParagraph(width, "!! ", m.styles.errorText, m.lastError))
	}
	return strings.Join(blocks, "\n\n")
}

func (m *model) renderPermissionScene(width, height int) string {
	if m.pendingPermission == nil {
		return ""
	}
	cardWidth := min(max(48, width-16), 88)
	contentWidth := max(24, cardWidth-6)
	body := strings.Join([]string{
		m.renderMetaParagraph(contentWidth, "!! ", m.styles.warningText, "Permission required for "+m.pendingPermission.Kind),
		m.renderParagraph(contentWidth, 2, m.styles.meta, m.pendingPermission.Summary),
		m.renderMetaParagraph(contentWidth, ".. ", m.styles.dim, "Press y or enter to approve. Press n or esc to deny."),
	}, "\n\n")
	card := lipgloss.NewStyle().
		Width(cardWidth).
		Padding(1, 2).
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#5E8F95")).
		BorderBackground(lipgloss.Color("#052B33")).
		Background(lipgloss.Color("#052B33")).
		Foreground(lipgloss.Color("#A8B7B8")).
		Render(body)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, card, lipgloss.WithWhitespaceStyle(m.styles.screenFill))
}

func (m *model) renderPromptPreview(width int, text string) string {
	prefix := m.styles.promptPrefix.Render("  > ")
	contentWidth := max(1, width-lipgloss.Width(prefix)-1)
	content := trimToWidth(strings.TrimSpace(strings.Join(strings.Fields(text), " ")), contentWidth)
	line := prefix + m.styles.promptLine.Render(content)
	return m.fillLine(line, width, m.styles.promptFill)
}
