package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	"minioc/internal/safety"
)

const maxGrepMatches = 200

type grepArgs struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path,omitempty"`
	Include string `json:"include,omitempty"`
}

func GrepTool() Spec {
	return Spec{
		Name:        "grep",
		Description: "Search file contents with a regular expression. Returns matching file paths, line numbers, and lines.",
		Parameters: objectSchema(map[string]any{
			"pattern": map[string]any{"type": "string", "description": "Regular expression pattern"},
			"path":    map[string]any{"type": "string", "description": "Optional base directory; defaults to current workdir"},
			"include": map[string]any{"type": "string", "description": "Optional glob filter like *.go or **/*.md"},
		}, "pattern"),
		Execute: executeGrep,
	}
}

func executeGrep(ctx context.Context, callCtx CallContext, raw json.RawMessage) (Result, error) {
	var args grepArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return Result{}, fmt.Errorf("decode grep args: %w", err)
	}
	if strings.TrimSpace(args.Pattern) == "" {
		return Result{}, fmt.Errorf("pattern is required")
	}

	baseDir, err := safety.ResolveDir(callCtx.RepoRoot, callCtx.Workdir, args.Path)
	if err != nil {
		return Result{}, err
	}

	re, err := regexp.Compile(args.Pattern)
	if err != nil {
		return Result{}, fmt.Errorf("compile pattern %q: %w", args.Pattern, err)
	}

	matches := make([]string, 0, 32)
	err = filepath.WalkDir(baseDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}

		rel, err := filepath.Rel(baseDir, path)
		if err != nil {
			return err
		}
		if rel == ".git" || strings.HasPrefix(rel, ".git"+string(filepath.Separator)) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if args.Include != "" {
			ok, err := doublestar.Match(args.Include, filepath.ToSlash(rel))
			if err != nil {
				return err
			}
			if !ok {
				return nil
			}
		}
		if isBinaryFile(path) {
			return nil
		}

		if err := grepFile(path, re, &matches); err != nil {
			return err
		}
		if len(matches) >= maxGrepMatches {
			return errStopWalk
		}
		return nil
	})
	if err != nil && err != errStopWalk {
		return Result{}, err
	}

	if len(matches) == 0 {
		return Result{Title: "grep", Output: "(no matches)"}, nil
	}

	output := strings.Join(matches, "\n")
	if len(matches) >= maxGrepMatches {
		output += "\n\n... (match limit reached)"
	}
	output, _ = truncateText(output)
	return Result{Title: "grep", Output: output}, nil
}

var errStopWalk = fmt.Errorf("stop walk")

func grepFile(path string, re *regexp.Regexp, matches *[]string) error {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()
		if !re.MatchString(line) {
			continue
		}
		*matches = append(*matches, fmt.Sprintf("%s:%d:%s", path, lineNumber, clipLine(line)))
		if len(*matches) >= maxGrepMatches {
			return nil
		}
	}
	return scanner.Err()
}
