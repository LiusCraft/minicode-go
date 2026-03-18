package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"minioc/internal/llm"
)

func normalizeStatus(status string) string {
	status = strings.TrimSpace(status)
	if status == "" {
		return "completed"
	}
	return status
}

func normalizeToolOutput(output string) string {
	output = strings.TrimSpace(output)
	if output == "" {
		return "(no output)"
	}
	return output
}

func summarizeToolCall(call llm.ToolCall) string {
	args := parseToolArgs(call.Arguments)
	lookup := func(keys ...string) string {
		for _, key := range keys {
			if value := strings.TrimSpace(args[key]); value != "" {
				return value
			}
		}
		return ""
	}

	switch call.Name {
	case "read":
		path := lookup("filePath", "path")
		if path != "" {
			return path
		}
	case "glob":
		pattern := lookup("pattern")
		base := lookup("path")
		if pattern != "" && base != "" {
			return pattern + " in " + base
		}
		if pattern != "" {
			return pattern
		}
	case "grep":
		pattern := lookup("pattern")
		include := lookup("include")
		if pattern != "" && include != "" {
			return pattern + " in " + include
		}
		if pattern != "" {
			return pattern
		}
	case "bash":
		if desc := lookup("description"); desc != "" {
			return desc
		}
		if command := lookup("command"); command != "" {
			return compactPreview(command, 96)
		}
	case "edit", "write":
		path := lookup("path", "filePath")
		if path != "" {
			return path
		}
	case "webfetch":
		if url := lookup("url"); url != "" {
			return url
		}
	case "task":
		if desc := lookup("description"); desc != "" {
			return desc
		}
		if command := lookup("command"); command != "" {
			return command
		}
	case "question":
		if header := lookup("header", "question"); header != "" {
			return header
		}
	}

	if fallback := lookup("description", "path", "filePath", "url", "command", "pattern", "query", "prompt"); fallback != "" {
		return compactPreview(fallback, 96)
	}
	if raw := compactPreview(string(call.Arguments), 96); raw != "" {
		return raw
	}
	return "(no args)"
}

func summarizeToolOutput(entry toolEntry) string {
	status := normalizeStatus(entry.Status)
	if status == "running" {
		return "running"
	}
	line := firstMeaningfulLine(entry.Output)
	if line == "" || line == "(no output)" {
		return status
	}
	return status + "  .  " + compactPreview(line, 110)
}

func summarizeToolDetails(entry toolEntry) []string {
	if normalizeStatus(entry.Status) == "running" {
		return nil
	}
	lines := compactLines(entry.Output, 3, 140)
	if len(lines) <= 1 {
		return nil
	}
	return lines[1:]
}

func parseToolArgs(raw json.RawMessage) map[string]string {
	result := map[string]string{}
	if len(raw) == 0 {
		return result
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return result
	}
	for key, value := range obj {
		switch v := value.(type) {
		case string:
			result[key] = v
		case float64:
			result[key] = fmt.Sprintf("%.0f", v)
		case bool:
			if v {
				result[key] = "true"
			} else {
				result[key] = "false"
			}
		}
	}
	return result
}

func firstMeaningfulLine(text string) string {
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

func compactLines(text string, limit, width int) []string {
	if limit <= 0 {
		return nil
	}
	parts := strings.Split(strings.TrimSpace(text), "\n")
	lines := make([]string, 0, limit)
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		lines = append(lines, compactPreview(part, width))
		if len(lines) == limit {
			break
		}
	}
	return lines
}

func compactPreview(text string, limit int) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	text = strings.Join(strings.Fields(text), " ")
	if len(text) <= limit {
		return text
	}
	if limit <= 3 {
		return text[:limit]
	}
	return text[:limit-3] + "..."
}

func trimLeft(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	if width <= 3 {
		return s[:width]
	}
	start := len(s) - (width - 3)
	if start < 0 {
		start = 0
	}
	return "..." + s[start:]
}

func trimToWidth(s string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(s)
	for len(runes) > 0 && lipgloss.Width(string(runes)) > width {
		runes = runes[:len(runes)-1]
	}
	return string(runes)
}

func measureInputHeight(text string, width int) int {
	if width <= 0 {
		return 1
	}
	if text == "" {
		return 1
	}

	total := 0
	for _, line := range strings.Split(text, "\n") {
		if line == "" {
			total++
			continue
		}
		remaining := line
		for {
			total++
			chunk, rest := splitToWidth(remaining, width)
			if rest == "" || chunk == remaining {
				break
			}
			remaining = rest
		}
	}
	if total < 1 {
		return 1
	}
	return total
}

func wrapText(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}
	paragraphs := strings.Split(text, "\n")
	lines := make([]string, 0, len(paragraphs))
	for _, paragraph := range paragraphs {
		trimmed := strings.TrimSpace(paragraph)
		if trimmed == "" {
			lines = append(lines, "")
			continue
		}
		words := strings.Fields(trimmed)
		current := ""
		for _, word := range words {
			if current == "" {
				for lipgloss.Width(word) > width {
					chunk, rest := splitToWidth(word, width)
					lines = append(lines, chunk)
					word = rest
				}
				current = word
				continue
			}
			candidate := current + " " + word
			if lipgloss.Width(candidate) <= width {
				current = candidate
				continue
			}
			lines = append(lines, current)
			current = ""
			for lipgloss.Width(word) > width {
				chunk, rest := splitToWidth(word, width)
				lines = append(lines, chunk)
				word = rest
			}
			current = word
		}
		if current != "" {
			lines = append(lines, current)
		}
	}
	return lines
}

func splitToWidth(s string, width int) (string, string) {
	if lipgloss.Width(s) <= width {
		return s, ""
	}
	runes := []rune(s)
	cut := 0
	for i := 1; i <= len(runes); i++ {
		if lipgloss.Width(string(runes[:i])) > width {
			break
		}
		cut = i
	}
	if cut == 0 {
		cut = 1
	}
	return string(runes[:cut]), string(runes[cut:])
}

func (m *model) fillBlock(block string, width, height int, filler lipgloss.Style) string {
	lines := strings.Split(block, "\n")
	for i := range lines {
		lines[i] = m.fillLine(lines[i], width, filler)
	}
	if height <= 0 {
		return strings.Join(lines, "\n")
	}
	for len(lines) < height {
		lines = append(lines, m.blankLine(width))
	}
	return strings.Join(lines, "\n")
}

func (m *model) fillLine(line string, width int, filler lipgloss.Style) string {
	if width <= 0 {
		return line
	}
	gap := width - lipgloss.Width(line)
	if gap <= 0 {
		return line
	}
	return line + m.spaceFill(gap, filler)
}

func (m *model) blankLine(width int) string {
	return m.spaceFill(max(0, width), m.styles.screenFill)
}

func (m *model) spaceFill(n int, filler lipgloss.Style) string {
	if n <= 0 {
		return ""
	}
	return filler.Render(strings.Repeat(" ", n))
}
