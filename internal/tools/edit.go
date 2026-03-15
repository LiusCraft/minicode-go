package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"minioc/internal/safety"
)

type editArgs struct {
	FilePath   string `json:"filePath"`
	OldString  string `json:"oldString"`
	NewString  string `json:"newString"`
	ReplaceAll bool   `json:"replaceAll,omitempty"`
}

func EditTool() Spec {
	return Spec{
		Name:        "edit",
		Description: "Replace exact text in an existing file. Use write_file to create a new file or replace full contents.",
		Parameters: objectSchema(map[string]any{
			"filePath":   map[string]any{"type": "string", "description": "Absolute or repo-relative file path"},
			"oldString":  map[string]any{"type": "string", "description": "Exact text to replace"},
			"newString":  map[string]any{"type": "string", "description": "Replacement text"},
			"replaceAll": map[string]any{"type": "boolean", "description": "Replace all occurrences instead of exactly one"},
		}, "filePath", "oldString", "newString"),
		Execute: executeEdit,
	}
}

func executeEdit(_ context.Context, callCtx CallContext, raw json.RawMessage) (Result, error) {
	var args editArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return Result{}, fmt.Errorf("decode edit args: %w", err)
	}
	if args.OldString == "" {
		return Result{}, fmt.Errorf("oldString must not be empty; use write_file to create or fully replace a file")
	}
	if args.OldString == args.NewString {
		return Result{}, fmt.Errorf("oldString and newString are identical")
	}

	path, err := safety.ResolvePath(callCtx.RepoRoot, callCtx.Workdir, args.FilePath)
	if err != nil {
		return Result{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Result{}, fmt.Errorf("read %q: %w", path, err)
	}
	content := string(data)
	count := strings.Count(content, args.OldString)
	if count == 0 {
		return Result{}, fmt.Errorf("oldString was not found in %s", path)
	}
	if !args.ReplaceAll && count > 1 {
		return Result{}, fmt.Errorf("oldString matched %d times; set replaceAll=true or use a more specific oldString", count)
	}

	replacements := 1
	if args.ReplaceAll {
		replacements = count
	}
	updated := strings.Replace(content, args.OldString, args.NewString, replacements)
	if updated == content {
		return Result{Title: "edit", Output: "No changes needed."}, nil
	}

	summary := fmt.Sprintf("replacements: %d\nold preview: %s\nnew preview: %s", replacements, preview(args.OldString), preview(args.NewString))
	if err := callCtx.Permissions.ConfirmEdit(path, summary); err != nil {
		return Result{}, err
	}

	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return Result{}, fmt.Errorf("write %q: %w", path, err)
	}
	return Result{Title: "edit", Output: fmt.Sprintf("Updated %s (%d replacement(s)).", path, replacements)}, nil
}
