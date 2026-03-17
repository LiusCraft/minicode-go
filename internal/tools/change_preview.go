package tools

import (
	"fmt"
	"strings"
)

const (
	previewMaxLines      = 8
	previewMaxLineWidth  = 160
	diffPreviewContext   = 2
	diffPreviewMaxHunks  = 3
	diffPreviewMaxLines  = 24
	maxDiffMatrixEntries = 40000
)

type diffKind byte

const (
	diffEqual  diffKind = ' '
	diffDelete diffKind = '-'
	diffInsert diffKind = '+'
)

type diffSegment struct {
	kind  diffKind
	lines []string
}

type diffEntry struct {
	kind    diffKind
	oldLine int
	newLine int
	text    string
}

type diffWindow struct {
	start int
	end   int
}

func buildEditConfirmationSummary(replacements int, before, after, oldText, newText string) string {
	parts := []string{
		fmt.Sprintf("replacements: %d", replacements),
		fmt.Sprintf("bytes: %d -> %d", len(before), len(after)),
		fmt.Sprintf("lines: %d -> %d", countPreviewLines(before), countPreviewLines(after)),
		"old selection:",
		renderLinePreview(oldText, previewMaxLines),
		"new selection:",
		renderLinePreview(newText, previewMaxLines),
		"diff:",
		renderDiffPreview(before, after),
	}
	return strings.Join(parts, "\n")
}

func buildEditResultSummary(path string, replacements int, before, after string) string {
	return strings.Join([]string{
		fmt.Sprintf("Updated %s (%d replacement(s)).", path, replacements),
		"diff:",
		renderDiffPreview(before, after),
	}, "\n")
}

func buildWriteConfirmationSummary(action, before, after string) string {
	parts := []string{fmt.Sprintf("action: %s", action)}
	if action == "created" {
		parts = append(parts,
			fmt.Sprintf("bytes: %d", len(after)),
			fmt.Sprintf("lines: %d", countPreviewLines(after)),
			"preview:",
			renderLinePreview(after, previewMaxLines),
		)
		return strings.Join(parts, "\n")
	}

	parts = append(parts,
		fmt.Sprintf("bytes: %d -> %d", len(before), len(after)),
		fmt.Sprintf("lines: %d -> %d", countPreviewLines(before), countPreviewLines(after)),
		"diff:",
		renderDiffPreview(before, after),
	)
	return strings.Join(parts, "\n")
}

func buildWriteResultSummary(path, action, before, after string) string {
	if action == "created" {
		return strings.Join([]string{
			fmt.Sprintf("Created %s", path),
			"preview:",
			renderLinePreview(after, previewMaxLines),
		}, "\n")
	}

	return strings.Join([]string{
		fmt.Sprintf("Updated %s", path),
		"diff:",
		renderDiffPreview(before, after),
	}, "\n")
}

func renderLinePreview(text string, maxLines int) string {
	lines := splitPreviewLines(text)
	if len(lines) == 0 {
		return "(empty file)"
	}
	if maxLines <= 0 || maxLines > len(lines) {
		maxLines = len(lines)
	}

	var b strings.Builder
	for i := 0; i < maxLines; i++ {
		fmt.Fprintf(&b, "%4d | %s\n", i+1, formatPreviewLine(lines[i]))
	}
	if len(lines) > maxLines {
		fmt.Fprintf(&b, "... (%d more line(s))", len(lines)-maxLines)
		return b.String()
	}
	return strings.TrimRight(b.String(), "\n")
}

func renderDiffPreview(before, after string) string {
	entries := buildDiffEntries(splitPreviewLines(before), splitPreviewLines(after))
	if len(entries) == 0 {
		return "(empty file)"
	}

	windows := buildDiffWindows(entries, diffPreviewContext)
	if len(windows) == 0 {
		return "(no textual diff)"
	}

	var b strings.Builder
	renderedLines := 0
	shownHunks := 0
	for i, window := range windows {
		if shownHunks >= diffPreviewMaxHunks {
			fmt.Fprintf(&b, "... (%d more hunk(s))", len(windows)-shownHunks)
			break
		}
		if i > 0 && b.Len() > 0 {
			b.WriteString("\n...\n")
		}

		hunk := entries[window.start:window.end]
		fmt.Fprintf(&b, "%s\n", formatHunkHeader(hunk))
		for _, entry := range hunk {
			if renderedLines >= diffPreviewMaxLines {
				b.WriteString("... (diff preview truncated)")
				return strings.TrimRight(b.String(), "\n")
			}
			fmt.Fprintf(&b, "%c %4s %4s | %s\n", entry.kind, lineLabel(entry.oldLine), lineLabel(entry.newLine), formatPreviewLine(entry.text))
			renderedLines++
		}
		shownHunks++
	}

	return strings.TrimRight(b.String(), "\n")
}

func buildDiffEntries(before, after []string) []diffEntry {
	segments := buildDiffSegments(before, after)
	entries := make([]diffEntry, 0, len(before)+len(after))
	oldLine := 1
	newLine := 1
	for _, segment := range segments {
		for _, line := range segment.lines {
			entry := diffEntry{kind: segment.kind, text: line}
			switch segment.kind {
			case diffEqual:
				entry.oldLine = oldLine
				entry.newLine = newLine
				oldLine++
				newLine++
			case diffDelete:
				entry.oldLine = oldLine
				oldLine++
			case diffInsert:
				entry.newLine = newLine
				newLine++
			}
			entries = append(entries, entry)
		}
	}
	return entries
}

func buildDiffSegments(before, after []string) []diffSegment {
	if len(before) == 0 && len(after) == 0 {
		return nil
	}
	if len(before)*len(after) > maxDiffMatrixEntries {
		segments := make([]diffSegment, 0, 2)
		if len(before) > 0 {
			segments = append(segments, diffSegment{kind: diffDelete, lines: before})
		}
		if len(after) > 0 {
			segments = append(segments, diffSegment{kind: diffInsert, lines: after})
		}
		return segments
	}

	lcs := make([][]int, len(before)+1)
	for i := range lcs {
		lcs[i] = make([]int, len(after)+1)
	}
	for i := len(before) - 1; i >= 0; i-- {
		for j := len(after) - 1; j >= 0; j-- {
			if before[i] == after[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
				continue
			}
			if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
				continue
			}
			lcs[i][j] = lcs[i][j+1]
		}
	}

	segments := make([]diffSegment, 0, len(before)+len(after))
	appendSegment := func(kind diffKind, line string) {
		if len(segments) > 0 && segments[len(segments)-1].kind == kind {
			segments[len(segments)-1].lines = append(segments[len(segments)-1].lines, line)
			return
		}
		segments = append(segments, diffSegment{kind: kind, lines: []string{line}})
	}

	i, j := 0, 0
	for i < len(before) && j < len(after) {
		if before[i] == after[j] {
			appendSegment(diffEqual, before[i])
			i++
			j++
			continue
		}
		if lcs[i+1][j] >= lcs[i][j+1] {
			appendSegment(diffDelete, before[i])
			i++
			continue
		}
		appendSegment(diffInsert, after[j])
		j++
	}
	for ; i < len(before); i++ {
		appendSegment(diffDelete, before[i])
	}
	for ; j < len(after); j++ {
		appendSegment(diffInsert, after[j])
	}

	return segments
}

func buildDiffWindows(entries []diffEntry, context int) []diffWindow {
	windows := make([]diffWindow, 0, 4)
	for i, entry := range entries {
		if entry.kind == diffEqual {
			continue
		}
		start := i - context
		if start < 0 {
			start = 0
		}
		end := i + context + 1
		if end > len(entries) {
			end = len(entries)
		}
		if len(windows) == 0 || start > windows[len(windows)-1].end {
			windows = append(windows, diffWindow{start: start, end: end})
			continue
		}
		if end > windows[len(windows)-1].end {
			windows[len(windows)-1].end = end
		}
	}
	return windows
}

func formatHunkHeader(entries []diffEntry) string {
	oldStart, oldCount := lineRange(entries, true)
	newStart, newCount := lineRange(entries, false)
	return fmt.Sprintf("@@ -%d,%d +%d,%d @@", oldStart, oldCount, newStart, newCount)
}

func lineRange(entries []diffEntry, old bool) (start, count int) {
	for _, entry := range entries {
		line := entry.newLine
		if old {
			line = entry.oldLine
		}
		if line == 0 {
			continue
		}
		if start == 0 {
			start = line
		}
		count++
	}
	return start, count
}

func lineLabel(line int) string {
	if line == 0 {
		return "."
	}
	return fmt.Sprintf("%d", line)
}

func countPreviewLines(text string) int {
	return len(splitPreviewLines(text))
}

func splitPreviewLines(text string) []string {
	if text == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func formatPreviewLine(line string) string {
	line = strings.ReplaceAll(line, "\t", "    ")
	if line == "" {
		return "<blank>"
	}
	if len(line) <= previewMaxLineWidth {
		return line
	}
	return line[:previewMaxLineWidth] + "..."
}
