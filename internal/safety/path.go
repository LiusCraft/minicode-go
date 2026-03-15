package safety

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func ResolvePath(repoRoot, workdir, target string) (string, error) {
	if strings.TrimSpace(target) == "" {
		return "", fmt.Errorf("path is required")
	}

	candidate := target
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(workdir, candidate)
	}

	abs, err := filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve %q: %w", target, err)
	}

	resolved := followSymlinks(abs)
	if err := EnsureWithinRepo(repoRoot, resolved); err != nil {
		return "", err
	}
	return resolved, nil
}

func ResolveDir(repoRoot, workdir, target string) (string, error) {
	if strings.TrimSpace(target) == "" {
		target = workdir
	}

	resolved, err := ResolvePath(repoRoot, workdir, target)
	if err != nil {
		return "", err
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("stat %q: %w", resolved, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%q is not a directory", resolved)
	}
	return resolved, nil
}

func EnsureWithinRepo(repoRoot, candidate string) error {
	repoRoot = followSymlinks(repoRoot)
	candidate = followSymlinks(candidate)

	rel, err := filepath.Rel(repoRoot, candidate)
	if err != nil {
		return fmt.Errorf("compute relative path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path %q is outside repo root %q", candidate, repoRoot)
	}
	return nil
}

func followSymlinks(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return resolved
	}

	parent := filepath.Dir(path)
	base := filepath.Base(path)
	resolvedParent, parentErr := filepath.EvalSymlinks(parent)
	if parentErr == nil {
		return filepath.Join(resolvedParent, base)
	}
	return filepath.Clean(path)
}
