package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"minioc/internal/safety"
)

const defaultBashTimeout = 2 * time.Minute

type bashArgs struct {
	Command     string `json:"command"`
	Timeout     int    `json:"timeout,omitempty"`
	Workdir     string `json:"workdir,omitempty"`
	Description string `json:"description"`
}

func BashTool() Spec {
	return Spec{
		Name:        "bash",
		Description: "Execute a shell command inside the repo. Use a short description and optional workdir. High-risk destructive commands are blocked.",
		Parameters: objectSchema(map[string]any{
			"command":     map[string]any{"type": "string", "description": "Shell command to execute"},
			"timeout":     map[string]any{"type": "integer", "description": "Timeout in milliseconds; defaults to 120000"},
			"workdir":     map[string]any{"type": "string", "description": "Optional working directory inside the repo"},
			"description": map[string]any{"type": "string", "description": "Short explanation of what the command does"},
		}, "command", "description"),
		Execute: executeBash,
	}
}

func executeBash(ctx context.Context, callCtx CallContext, raw json.RawMessage) (Result, error) {
	var args bashArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return Result{}, fmt.Errorf("decode bash args: %w", err)
	}
	if strings.TrimSpace(args.Command) == "" {
		return Result{}, fmt.Errorf("command is required")
	}
	if strings.TrimSpace(args.Description) == "" {
		return Result{}, fmt.Errorf("description is required")
	}

	workdir, err := safety.ResolveDir(callCtx.RepoRoot, callCtx.Workdir, args.Workdir)
	if err != nil {
		return Result{}, err
	}
	if err := callCtx.Permissions.ConfirmBash(args.Command, workdir, args.Description); err != nil {
		return Result{}, err
	}

	timeout := defaultBashTimeout
	if args.Timeout > 0 {
		timeout = time.Duration(args.Timeout) * time.Millisecond
	}
	if timeout <= 0 {
		return Result{}, fmt.Errorf("timeout must be greater than zero")
	}

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, "/bin/sh", "-lc", args.Command)
	cmd.Dir = workdir
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	combined := strings.TrimSpace(stdout.String() + stderr.String())
	if combined == "" {
		combined = "(command completed with no output)"
	}
	combined, _ = truncateText(combined)

	if runCtx.Err() == context.DeadlineExceeded {
		return Result{}, fmt.Errorf("command timed out after %s\n%s", timeout, combined)
	}
	if err != nil {
		return Result{}, fmt.Errorf("command failed: %w\n%s", err, combined)
	}

	return Result{Title: args.Description, Output: combined}, nil
}
