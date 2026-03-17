package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"

	"minioc/internal/safety"
)

type globArgs struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path,omitempty"`
}

func GlobTool() Spec {
	return Spec{
		Name:         "glob",
		Description:  "Match file paths with glob patterns like **/*.go or cmd/**/*.go.",
		ParallelSafe: true,
		Parameters: objectSchema(map[string]any{
			"pattern": map[string]any{"type": "string", "description": "Glob pattern to match"},
			"path":    map[string]any{"type": "string", "description": "Optional base directory; defaults to current workdir"},
		}, "pattern"),
		Execute: executeGlob,
	}
}

func executeGlob(_ context.Context, callCtx CallContext, raw json.RawMessage) (Result, error) {
	var args globArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return Result{}, fmt.Errorf("decode glob args: %w", err)
	}
	if strings.TrimSpace(args.Pattern) == "" {
		return Result{}, fmt.Errorf("pattern is required")
	}

	baseDir, err := safety.ResolveDir(callCtx.RepoRoot, callCtx.Workdir, args.Path)
	if err != nil {
		return Result{}, err
	}

	fsys := os.DirFS(baseDir)
	matches, err := doublestar.Glob(fsys, args.Pattern)
	if err != nil {
		return Result{}, fmt.Errorf("glob pattern %q: %w", args.Pattern, err)
	}

	type matchInfo struct {
		path    string
		modTime time.Time
	}
	items := make([]matchInfo, 0, len(matches))
	for _, match := range matches {
		if strings.HasPrefix(match, ".git") {
			continue
		}
		fullPath := filepath.Join(baseDir, filepath.FromSlash(match))
		info, err := fs.Stat(fsys, match)
		if err != nil {
			continue
		}
		items = append(items, matchInfo{path: fullPath, modTime: info.ModTime()})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].modTime.Equal(items[j].modTime) {
			return items[i].path < items[j].path
		}
		return items[i].modTime.After(items[j].modTime)
	})

	if len(items) == 0 {
		return Result{Title: "glob", Output: "(no matches)"}, nil
	}

	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, item.path)
	}
	output, _ := truncateText(strings.Join(lines, "\n"))
	return Result{Title: "glob", Output: output}, nil
}
