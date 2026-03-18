package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"minioc/internal/agent"
	"minioc/internal/config"
	"minioc/internal/llm"
	"minioc/internal/project"
	"minioc/internal/safety"
	"minioc/internal/session"
	"minioc/internal/store"
	"minioc/internal/tools"
	"minioc/internal/tui"
)

func main() {
	os.Exit(run())
}

func run() int {
	ctx := context.Background()

	fs := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	workdirFlag := fs.String("C", ".", "working directory")
	modelFlag := fs.String("model", "", "model override")
	continueFlag := fs.String("continue", "", "continue an existing session by id")
	maxStepsFlag := fs.Int("max-steps", 1000, "maximum model/tool loop steps")
	autoApproveFlag := fs.Bool("auto-approve", false, "auto approve bash/edit/write tool calls")
	tuiFlag := fs.Bool("tui", true, "launch the Bubble Tea TUI")
	noTUIFlag := fs.Bool("no-tui", false, "run in plain streaming CLI mode")

	if err := fs.Parse(os.Args[1:]); err != nil {
		return 2
	}

	promptText := strings.TrimSpace(strings.Join(fs.Args(), " "))
	useTUI := *tuiFlag && !*noTUIFlag
	if promptText == "" && !useTUI {
		fmt.Fprintln(os.Stderr, "usage: minioc [flags] \"your prompt\"")
		fs.PrintDefaults()
		return 2
	}

	workdir, err := project.ResolveWorkdir(*workdirFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "workdir error: %v\n", err)
		return 1
	}

	repoRoot, err := project.DetectRepoRoot(workdir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "repo root error: %v\n", err)
		return 1
	}

	cfg, err := config.Load(config.Options{
		ModelOverride: *modelFlag,
		MaxSteps:      *maxStepsFlag,
		AutoApprove:   *autoApproveFlag,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		return 1
	}

	sessionStore := store.NewFileStore(filepath.Join(repoRoot, ".minioc", "sessions"))

	var current *session.Session
	if *continueFlag != "" {
		current, err = sessionStore.Load(ctx, *continueFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "load session error: %v\n", err)
			return 1
		}
		if current.RepoRoot != "" && current.RepoRoot != repoRoot {
			fmt.Fprintf(os.Stderr, "session %s belongs to repo %s, not %s\n", current.ID, current.RepoRoot, repoRoot)
			return 1
		}
		current.Workdir = workdir
		current.RepoRoot = repoRoot
		current.Model = cfg.Model
	} else {
		current = session.New(repoRoot, workdir, cfg.Model)
	}

	permissionManager := safety.NewPermissionManager(os.Stdin, os.Stderr, cfg.AutoApprove)
	registry := tools.NewRegistry(
		tools.ReadTool(),
		tools.GlobTool(),
		tools.GrepTool(),
		tools.BashTool(),
		tools.EditTool(),
		tools.WriteFileTool(),
	)

	client, err := llm.NewOpenAIClient(cfg.APIKey, cfg.BaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "llm setup error: %v\n", err)
		return 1
	}

	loop := agent.Loop{
		Client:   client,
		Store:    sessionStore,
		Tools:    registry,
		MaxSteps: cfg.MaxSteps,
	}

	if useTUI {
		if err := tui.Run(tui.Config{
			RepoRoot:    repoRoot,
			Workdir:     workdir,
			Model:       cfg.Model,
			Prompt:      promptText,
			Loop:        loop,
			Session:     current,
			AutoApprove: cfg.AutoApprove,
		}); err != nil {
			fmt.Fprintf(os.Stderr, "tui error: %v\n", err)
			return 1
		}
		return 0
	}

	printer := newStreamPrinter(os.Stdout, os.Stderr)
	answer, err := loop.Run(ctx, current, permissionManager, promptText, &agent.Hooks{
		OnAssistantDelta:       printer.AssistantDelta,
		OnAssistantMessageDone: printer.AssistantMessageDone,
		OnToolCall:             printer.ToolCall,
		OnToolResult:           printer.ToolResult,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "run error: %v\n", err)
		return 1
	}

	printer.AssistantMessageDone()
	if !printer.Streamed() {
		fmt.Println(answer)
	}
	fmt.Fprintf(os.Stderr, "\nsession: %s\n", current.ID)
	return 0
}
