package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"minioc/internal/safety"
)

const defaultReadLimit = 200

type readArgs struct {
	FilePath string `json:"filePath"`
	Offset   int    `json:"offset,omitempty"`
	Limit    int    `json:"limit,omitempty"`
}

func ReadTool() Spec {
	return Spec{
		Name:         "read_file",
		Description:  "Read a file or directory from the local repository. Returns numbered lines for files and entry names for directories.",
		ParallelSafe: true,
		Parameters: objectSchema(map[string]any{
			"filePath": map[string]any{"type": "string", "description": "Absolute or repo-relative file path"},
			"offset":   map[string]any{"type": "integer", "description": "1-based line offset for files or entry offset for directories"},
			"limit":    map[string]any{"type": "integer", "description": "Maximum number of lines or entries to return"},
		}, "filePath"),
		Execute: executeRead,
	}
}

func executeRead(_ context.Context, callCtx CallContext, raw json.RawMessage) (Result, error) {
	var args readArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return Result{}, fmt.Errorf("decode read_file args: %w", err)
	}
	if args.Offset < 0 {
		return Result{}, fmt.Errorf("offset must be greater than or equal to 1")
	}
	if args.Offset == 0 {
		args.Offset = 1
	}
	if args.Limit <= 0 {
		args.Limit = defaultReadLimit
	}

	resolved, err := safety.ResolvePath(callCtx.RepoRoot, callCtx.Workdir, args.FilePath)
	if err != nil {
		return Result{}, err
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return Result{}, fmt.Errorf("stat %q: %w", resolved, err)
	}

	if info.IsDir() {
		return readDirectory(resolved, args.Offset, args.Limit)
	}
	if isBinaryFile(resolved) {
		return Result{}, fmt.Errorf("cannot read binary file: %s", resolved)
	}
	return readTextFile(resolved, args.Offset, args.Limit)
}

func readDirectory(path string, offset, limit int) (Result, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return Result{}, fmt.Errorf("read dir %q: %w", path, err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			name += "/"
		}
		names = append(names, name)
	}
	sort.Strings(names)

	start := offset - 1
	if len(names) == 0 {
		return Result{Title: filepath.Base(path), Output: "(empty directory)"}, nil
	}
	if start >= len(names) {
		return Result{}, fmt.Errorf("offset %d is out of range for directory with %d entries", offset, len(names))
	}
	end := start + limit
	if end > len(names) {
		end = len(names)
	}

	body := strings.Join(names[start:end], "\n")
	if end < len(names) {
		body += fmt.Sprintf("\n\n(showing %d of %d entries; use offset=%d to continue)", end-start, len(names), end+1)
		return Result{Title: filepath.Base(path), Output: body}, nil
	}
	if body == "" {
		body = "(empty directory)"
	}
	return Result{Title: filepath.Base(path), Output: body}, nil
}

func readTextFile(path string, offset, limit int) (Result, error) {
	file, err := os.Open(path)
	if err != nil {
		return Result{}, fmt.Errorf("open %q: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	lineNumber := 0
	lines := make([]string, 0, limit)
	hasMore := false
	for scanner.Scan() {
		lineNumber++
		if lineNumber < offset {
			continue
		}
		if len(lines) >= limit {
			hasMore = true
			continue
		}
		lines = append(lines, fmt.Sprintf("%d: %s", lineNumber, clipLine(scanner.Text())))
	}
	if err := scanner.Err(); err != nil {
		return Result{}, fmt.Errorf("scan %q: %w", path, err)
	}
	if (lineNumber == 0 && offset > 1) || (lineNumber > 0 && offset > lineNumber) {
		return Result{}, fmt.Errorf("offset %d is out of range for file with %d lines", offset, lineNumber)
	}

	body := strings.Join(lines, "\n")
	if hasMore {
		body += fmt.Sprintf("\n\n(showing lines %d-%d of %d; use offset=%d to continue)", offset, offset+len(lines)-1, lineNumber, offset+len(lines))
	}
	if body == "" {
		body = "(empty file)"
	}
	return Result{Title: filepath.Base(path), Output: body}, nil
}

func isBinaryFile(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	buf := make([]byte, 8000)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		return false
	}
	for _, b := range buf[:n] {
		if b == 0 {
			return true
		}
	}
	return false
}
