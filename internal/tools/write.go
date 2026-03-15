package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"minioc/internal/safety"
)

type writeArgs struct {
	FilePath string `json:"filePath"`
	Content  string `json:"content"`
}

func WriteFileTool() Spec {
	return Spec{
		Name:        "write_file",
		Description: "Create a new file or overwrite an existing file with full contents.",
		Parameters: objectSchema(map[string]any{
			"filePath": map[string]any{"type": "string", "description": "Absolute or repo-relative file path"},
			"content":  map[string]any{"type": "string", "description": "Entire file contents"},
		}, "filePath", "content"),
		Execute: executeWriteFile,
	}
}

func executeWriteFile(_ context.Context, callCtx CallContext, raw json.RawMessage) (Result, error) {
	var args writeArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return Result{}, fmt.Errorf("decode write_file args: %w", err)
	}

	path, err := safety.ResolvePath(callCtx.RepoRoot, callCtx.Workdir, args.FilePath)
	if err != nil {
		return Result{}, err
	}

	action := "updated"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		action = "created"
	}

	summary := fmt.Sprintf("action: %s\nbytes: %d", action, len(args.Content))
	if err := callCtx.Permissions.ConfirmWrite(path, summary); err != nil {
		return Result{}, err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return Result{}, fmt.Errorf("create parent directory for %q: %w", path, err)
	}
	if err := os.WriteFile(path, []byte(args.Content), 0o644); err != nil {
		return Result{}, fmt.Errorf("write %q: %w", path, err)
	}

	return Result{Title: "write_file", Output: fmt.Sprintf("%s %s", capitalize(action), path)}, nil
}

func preview(text string) string {
	text = strings.ReplaceAll(text, "\n", "\\n")
	if len(text) <= 120 {
		return text
	}
	return text[:120] + "..."
}

func capitalize(text string) string {
	if text == "" {
		return text
	}
	return strings.ToUpper(text[:1]) + text[1:]
}
