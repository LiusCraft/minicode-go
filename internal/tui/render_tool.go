package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

func (m *model) renderToolEntry(width int, entry toolEntry, expanded bool) string {
	status := normalizeStatus(entry.Status)
	markerStyle := m.styles.toolRead
	badgeStyle := m.styles.toolStatusDone
	detailStyle := m.styles.toolMeta
	statusLabel := "[DONE]"
	switch status {
	case "running":
		markerStyle = m.styles.running
		badgeStyle = m.styles.toolStatusRun
		detailStyle = m.styles.running
		statusLabel = "[RUNNING]"
	case "error":
		markerStyle = m.styles.toolBash
		detailStyle = m.styles.toolBash
		badgeStyle = m.styles.toolStatusFail
		statusLabel = "[FAILED]"
	default:
		badgeStyle = m.styles.toolStatusMuted
		statusLabel = "[" + strings.ToUpper(status) + "]"
	}
	if status == "completed" {
		badgeStyle = m.styles.toolStatusDone
		statusLabel = "[DONE]"
	}

	label := strings.ToUpper(entry.Name)
	target := entry.Target
	if target == "" {
		target = "(no args)"
	}
	headPrefix := markerStyle.Render("*") +
		m.spaceFill(1, m.styles.screenFill) +
		badgeStyle.Render(statusLabel) +
		m.spaceFill(1, m.styles.screenFill) +
		m.styles.toolLabel.Render(label) +
		m.spaceFill(2, m.styles.screenFill)
	targetWidth := max(1, width-lipgloss.Width(headPrefix))
	head := m.fillLine(headPrefix+m.styles.toolPath.Render(trimToWidth(target, targetWidth)), width, m.styles.screenFill)

	lines := []string{head}
	statusSummary := summarizeToolOutput(entry)
	lines = append(lines, m.renderPrefixedParagraph(width, "  -> ", detailStyle, statusSummary))
	if expanded {
		for _, detail := range summarizeToolDetails(entry) {
			lines = append(lines, m.renderPrefixedParagraph(width, "     ", detailStyle, detail))
		}
	}
	return strings.Join(lines, "\n")
}

func (m *model) renderMetaParagraph(width int, prefix string, style lipgloss.Style, text string) string {
	return m.renderPrefixedParagraph(width, prefix, style, text)
}

func (m *model) renderPrefixedParagraph(width int, prefix string, style lipgloss.Style, text string) string {
	prefixWidth := lipgloss.Width(prefix)
	bodyWidth := max(1, width-prefixWidth)
	wrapped := wrapText(text, bodyWidth)
	if len(wrapped) == 0 {
		return m.fillLine(style.Render(prefix), width, m.styles.screenFill)
	}
	lines := make([]string, 0, len(wrapped))
	for i, part := range wrapped {
		lead := m.spaceFill(prefixWidth, m.styles.screenFill)
		if i == 0 {
			lead = style.Render(prefix)
		}
		lines = append(lines, m.fillLine(lead+style.Render(part), width, m.styles.screenFill))
	}
	return strings.Join(lines, "\n")
}

func (m *model) renderParagraph(width, indent int, style lipgloss.Style, text string) string {
	bodyWidth := max(1, width-indent)
	wrapped := wrapText(text, bodyWidth)
	if len(wrapped) == 0 {
		return m.blankLine(width)
	}
	indentFill := m.spaceFill(indent, m.styles.screenFill)
	lines := make([]string, 0, len(wrapped))
	for _, part := range wrapped {
		lines = append(lines, m.fillLine(indentFill+style.Render(part), width, m.styles.screenFill))
	}
	return strings.Join(lines, "\n")
}
