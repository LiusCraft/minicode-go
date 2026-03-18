package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

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

	action := "created"
	before := ""
	if data, err := os.ReadFile(path); err == nil {
		action = "updated"
		before = string(data)
	} else if !os.IsNotExist(err) {
		return Result{}, fmt.Errorf("read %q: %w", path, err)
	}

	if action == "updated" && before == args.Content {
		return Result{Title: "write_file", Output: "No changes needed."}, nil
	}

	summary := buildWriteConfirmationSummary(action, before, args.Content)
	if err := callCtx.Permissions.ConfirmWrite(path, summary); err != nil {
		return Result{}, err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return Result{}, fmt.Errorf("create parent directory for %q: %w", path, err)
	}
	if err := os.WriteFile(path, []byte(args.Content), 0o644); err != nil {
		return Result{}, fmt.Errorf("write %q: %w", path, err)
	}

	return Result{Title: "write_file", Output: buildWriteResultSummary(path, action, before, args.Content)}, nil
}
