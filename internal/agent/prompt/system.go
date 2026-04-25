package prompt

import (
	"fmt"
)

type BuildConfig struct {
	RepoRoot          string
	Workdir           string
	Model             string
	MaxSteps          int
	InstructionPaths  []string
	ExtraInstructions string
}

type BuildOption func(*BuildConfig)

func WithWorkdir(s string) BuildOption {
	return func(cfg *BuildConfig) {
		cfg.Workdir = s
	}
}

func WithMaxSteps(n int) BuildOption {
	return func(cfg *BuildConfig) {
		cfg.MaxSteps = n
	}
}

func WithExtraInstructions(s string) BuildOption {
	return func(cfg *BuildConfig) {
		cfg.ExtraInstructions = s
	}
}

func WithInstructionPaths(paths []string) BuildOption {
	return func(cfg *BuildConfig) {
		cfg.InstructionPaths = append([]string(nil), paths...)
	}
}

func Build(repoRoot, model string, opts ...BuildOption) string {
	cfg := &BuildConfig{
		RepoRoot: repoRoot,
		Model:    model,
		MaxSteps: 1000,
	}
	for _, opt := range opts {
		opt(cfg)
	}
	if cfg.Workdir == "" {
		cfg.Workdir = cfg.RepoRoot
	}

	instructions := Load(systemPaths(cfg.RepoRoot, cfg.Workdir, cfg.InstructionPaths))

	base := fmt.Sprintf(`You are minioc, a local coding agent.

Repo root: %s
Current workdir: %s
Selected model: %s
Max tool/LLM steps: %d

Operating rules:
- Use tools when repository context is needed.
- Prefer read_file, glob, and grep before bash or file edits.
- When several read-only lookups are independent, you may call multiple tools in the same step.
- Keep file access and edits inside the repo root.
- Use bash sparingly and only when it is the best tool.
- When you finish, respond with a concise final answer.
- Never invent tool results or claim changes you did not make.`, cfg.RepoRoot, cfg.Workdir, cfg.Model, cfg.MaxSteps)

	if instructions != "" {
		base += instructions
	}

	if cfg.ExtraInstructions != "" {
		base += "\n\n" + cfg.ExtraInstructions
	}

	return base
}
