package project

import (
	"fmt"
	"os"
	"path/filepath"
)

func ResolveWorkdir(input string) (string, error) {
	if input == "" {
		input = "."
	}

	abs, err := filepath.Abs(input)
	if err != nil {
		return "", fmt.Errorf("resolve absolute path: %w", err)
	}

	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("stat %q: %w", abs, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%q is not a directory", abs)
	}

	return abs, nil
}

func DetectRepoRoot(start string) (string, error) {
	current := start
	for {
		gitDir := filepath.Join(current, ".git")
		if _, err := os.Stat(gitDir); err == nil {
			return current, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			return start, nil
		}
		current = parent
	}
}
