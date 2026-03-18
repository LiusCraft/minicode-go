package main

import (
	"fmt"
	"io"
	"strings"

	"minioc/internal/llm"
)

type streamPrinter struct {
	out          io.Writer
	err          io.Writer
	streamedText bool
	lineOpen     bool
}

func newStreamPrinter(out, err io.Writer) *streamPrinter {
	return &streamPrinter{out: out, err: err}
}

func (p *streamPrinter) AssistantDelta(text string) {
	if text == "" {
		return
	}
	fmt.Fprint(p.out, text)
	p.streamedText = true
	p.lineOpen = true
}

func (p *streamPrinter) AssistantMessageDone() {
	if !p.lineOpen {
		return
	}
	fmt.Fprintln(p.out)
	p.lineOpen = false
}

func (p *streamPrinter) ToolCall(call llm.ToolCall) {
	p.AssistantMessageDone()
	args := compactPreview(string(call.Arguments), 120)
	if args == "" {
		fmt.Fprintf(p.err, "-> %s\n", call.Name)
		return
	}
	fmt.Fprintf(p.err, "-> %s %s\n", call.Name, args)
}

func (p *streamPrinter) ToolResult(name, status, output string) {
	p.AssistantMessageDone()
	line := compactPreview(firstLine(output), 120)
	if line == "" {
		line = status
	}
	fmt.Fprintf(p.err, "<- %s %s\n", name, line)
}

func (p *streamPrinter) Streamed() bool {
	return p.streamedText
}

func firstLine(text string) string {
	before, _, _ := strings.Cut(text, "\n")
	return before
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
	return text[:limit] + "..."
}
