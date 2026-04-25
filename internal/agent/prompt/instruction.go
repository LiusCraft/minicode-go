package prompt

import (
	"os"
	"path/filepath"
	"strings"

	"minioc/internal/config"
)

const (
	agentsFileName  = "AGENTS.md"
	claudeFileName  = "CLAUDE.md"
	contextFileName = "CONTEXT.md"
)

var instructionFilePriority = []string{
	agentsFileName,
	claudeFileName,
	contextFileName,
}

func systemPaths(repoRoot, workdir string, configuredPaths []string) []string {
	paths := make([]string, 0)
	seen := make(map[string]struct{})

	add := func(path string) {
		if !fileExists(path) {
			return
		}
		resolved := cleanPath(path)
		if _, ok := seen[resolved]; ok {
			return
		}
		seen[resolved] = struct{}{}
		paths = append(paths, resolved)
	}

	for _, path := range projectInstructionPaths(repoRoot, workdir) {
		add(path)
	}

	add(config.GlobalAgentsFile())

	for _, path := range resolveConfiguredInstructionPaths(repoRoot, configuredPaths) {
		add(path)
	}

	return paths
}

func Load(instructionPaths []string) string {
	var parts []string

	for _, path := range instructionPaths {
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		trimmed := strings.TrimSpace(string(content))
		if trimmed != "" {
			parts = append(parts, "Instructions from: "+path+"\n"+trimmed)
		}
	}

	if len(parts) == 0 {
		return ""
	}
	return "\n\n" + strings.Join(parts, "\n\n")
}

func ResolveForFile(repoRoot, workdir, filePath string, configuredPaths []string) string {
	dir := filepath.Dir(filePath)
	instructionPath := instructionFileInDir(dir)
	if instructionPath == "" {
		return ""
	}
	if cleanPath(instructionPath) == cleanPath(filePath) {
		return ""
	}

	loaded := make(map[string]struct{})
	for _, path := range systemPaths(repoRoot, workdir, configuredPaths) {
		loaded[cleanPath(path)] = struct{}{}
	}
	if _, ok := loaded[cleanPath(instructionPath)]; ok {
		return ""
	}

	content, err := os.ReadFile(instructionPath)
	if err != nil {
		return ""
	}
	trimmed := strings.TrimSpace(string(content))
	if trimmed == "" {
		return ""
	}

	return "\n<system-reminder>\n" + trimmed + "\n</system-reminder>"
}

func projectInstructionPaths(repoRoot, workdir string) []string {
	root := cleanPath(repoRoot)
	start := cleanPath(workdir)
	if start == "" || !isWithin(root, start) {
		start = root
	}

	dirs := walkUpWithin(start, root)
	for _, name := range instructionFilePriority {
		matches := make([]string, 0)
		for _, dir := range dirs {
			path := filepath.Join(dir, name)
			if fileExists(path) {
				matches = append(matches, path)
			}
		}
		if len(matches) > 0 {
			return matches
		}
	}

	return nil
}

func resolveConfiguredInstructionPaths(repoRoot string, configuredPaths []string) []string {
	if len(configuredPaths) == 0 {
		return nil
	}

	resolved := make([]string, 0, len(configuredPaths))
	for _, raw := range configuredPaths {
		instruction := strings.TrimSpace(raw)
		if instruction == "" {
			continue
		}
		if strings.HasPrefix(instruction, "http://") || strings.HasPrefix(instruction, "https://") {
			continue
		}
		if strings.HasPrefix(instruction, "~/") {
			home, err := os.UserHomeDir()
			if err != nil || home == "" {
				continue
			}
			instruction = filepath.Join(home, strings.TrimPrefix(instruction, "~/"))
		}
		if !filepath.IsAbs(instruction) {
			instruction = filepath.Join(repoRoot, instruction)
		}
		if fileExists(instruction) {
			resolved = append(resolved, instruction)
		}
	}

	return resolved
}

func instructionFileInDir(dir string) string {
	for _, name := range instructionFilePriority {
		path := filepath.Join(dir, name)
		if fileExists(path) {
			return path
		}
	}
	return ""
}

func walkUpWithin(start, stop string) []string {
	start = cleanPath(start)
	stop = cleanPath(stop)

	dirs := make([]string, 0, 4)
	current := start
	for {
		dirs = append(dirs, current)
		if current == stop {
			break
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		if !isWithin(stop, parent) {
			break
		}
		current = parent
	}

	return dirs
}

func fileExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func isWithin(root, target string) bool {
	root = cleanPath(root)
	target = cleanPath(target)

	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func cleanPath(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	cleaned := filepath.Clean(path)
	if filepath.IsAbs(cleaned) {
		return cleaned
	}
	abs, err := filepath.Abs(cleaned)
	if err != nil {
		return cleaned
	}
	return abs
}
