package tools

import "strings"

const (
	maxOutputBytes = 50 * 1024
	maxOutputLines = 2000
	maxLineLength  = 2000
)

func truncateText(text string) (string, bool) {
	truncated := false
	if len(text) > maxOutputBytes {
		text = text[:maxOutputBytes] + "\n\n... (output truncated by bytes)"
		truncated = true
	}

	lines := strings.Split(text, "\n")
	if len(lines) > maxOutputLines {
		text = strings.Join(lines[:maxOutputLines], "\n") + "\n\n... (output truncated by lines)"
		truncated = true
	}

	return text, truncated
}

func clipLine(text string) string {
	if len(text) <= maxLineLength {
		return text
	}
	return text[:maxLineLength] + "... (line truncated)"
}
