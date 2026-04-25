package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSystemPathsPrefersAgentsOverClaude(t *testing.T) {
	repoRoot := t.TempDir()
	workdir := filepath.Join(repoRoot, "pkg")
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		t.Fatalf("mkdir workdir: %v", err)
	}

	agentsPath := filepath.Join(workdir, agentsFileName)
	if err := os.WriteFile(agentsPath, []byte("project agents"), 0o644); err != nil {
		t.Fatalf("write project agents: %v", err)
	}
	claudePath := filepath.Join(repoRoot, claudeFileName)
	if err := os.WriteFile(claudePath, []byte("project claude"), 0o644); err != nil {
		t.Fatalf("write project claude: %v", err)
	}

	xdgConfigHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdgConfigHome)
	globalDir := filepath.Join(xdgConfigHome, "minioc")
	if err := os.MkdirAll(globalDir, 0o755); err != nil {
		t.Fatalf("mkdir global config dir: %v", err)
	}
	globalAgents := filepath.Join(globalDir, agentsFileName)
	if err := os.WriteFile(globalAgents, []byte("global agents"), 0o644); err != nil {
		t.Fatalf("write global agents: %v", err)
	}

	paths := systemPaths(repoRoot, workdir, nil)
	if len(paths) != 2 {
		t.Fatalf("expected 2 instruction paths, got %d: %v", len(paths), paths)
	}
	if paths[0] != agentsPath {
		t.Fatalf("expected first path %q, got %q", agentsPath, paths[0])
	}
	if paths[1] != globalAgents {
		t.Fatalf("expected second path %q, got %q", globalAgents, paths[1])
	}
	for _, path := range paths {
		if path == claudePath {
			t.Fatalf("claude instruction should not be loaded when AGENTS.md exists: %v", paths)
		}
	}
}

func TestResolveForFileSkipsWhenAlreadyInSystemPrompt(t *testing.T) {
	repoRoot := t.TempDir()
	workdir := filepath.Join(repoRoot, "pkg")
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		t.Fatalf("mkdir workdir: %v", err)
	}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	instructionPath := filepath.Join(workdir, agentsFileName)
	if err := os.WriteFile(instructionPath, []byte("project agents"), 0o644); err != nil {
		t.Fatalf("write instruction file: %v", err)
	}

	targetPath := filepath.Join(workdir, "main.go")
	if err := os.WriteFile(targetPath, []byte("package main"), 0o644); err != nil {
		t.Fatalf("write target file: %v", err)
	}

	reminder := ResolveForFile(repoRoot, workdir, targetPath, nil)
	if reminder != "" {
		t.Fatalf("expected empty reminder because instruction is in system prompt, got %q", reminder)
	}
}

func TestResolveForFileAppendsReminderWhenNotInSystemPrompt(t *testing.T) {
	repoRoot := t.TempDir()
	workdir := repoRoot
	subdir := filepath.Join(repoRoot, "pkg")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	instructionPath := filepath.Join(subdir, agentsFileName)
	instruction := "subdir agents"
	if err := os.WriteFile(instructionPath, []byte(instruction), 0o644); err != nil {
		t.Fatalf("write instruction file: %v", err)
	}

	targetPath := filepath.Join(subdir, "main.go")
	if err := os.WriteFile(targetPath, []byte("package main"), 0o644); err != nil {
		t.Fatalf("write target file: %v", err)
	}

	reminder := ResolveForFile(repoRoot, workdir, targetPath, nil)
	if !strings.Contains(reminder, "<system-reminder>") {
		t.Fatalf("expected reminder tag, got %q", reminder)
	}
	if !strings.Contains(reminder, instruction) {
		t.Fatalf("expected reminder content %q, got %q", instruction, reminder)
	}
}

func TestBuildLoadsConfiguredInstructionPaths(t *testing.T) {
	repoRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	instructionPath := filepath.Join(repoRoot, "instructions.md")
	instruction := "follow configured instruction"
	if err := os.WriteFile(instructionPath, []byte(instruction), 0o644); err != nil {
		t.Fatalf("write configured instruction file: %v", err)
	}

	systemPrompt := Build(repoRoot, "test-model",
		WithWorkdir(repoRoot),
		WithInstructionPaths([]string{"instructions.md", "https://example.com/instructions.md"}),
	)
	if !strings.Contains(systemPrompt, "Instructions from: "+instructionPath) {
		t.Fatalf("expected configured path in system prompt, got %q", systemPrompt)
	}
	if !strings.Contains(systemPrompt, instruction) {
		t.Fatalf("expected configured instruction content in system prompt, got %q", systemPrompt)
	}
}
