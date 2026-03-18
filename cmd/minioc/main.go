package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"minioc/internal/agent"
	"minioc/internal/config"
	llmmodels "minioc/internal/llm/models"
	"minioc/internal/llm/provider"
	anthropicprovider "minioc/internal/llm/provider/anthropic"
	openaicompatible "minioc/internal/llm/provider/openaicompatible"
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
	continueFlag := fs.String("continue", "", "continue an existing session by id")
	noTUIFlag := fs.Bool("no-tui", false, "run in plain streaming CLI mode")

	if err := fs.Parse(os.Args[1:]); err != nil {
		return 2
	}

	promptText := strings.TrimSpace(strings.Join(fs.Args(), " "))
	useTUI := !*noTUIFlag
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

	cfg, err := config.Load(repoRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		return 1
	}

	catalog, err := llmmodels.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "model catalog error: %v\n", err)
		return 1
	}

	sessionStore := store.NewFileStore(config.SessionsDir(repoRoot))

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
		tools.FetchTool(),
	)

	providerRegistry := provider.NewRegistry()
	for key, providerConfig := range cfg.Providers {
		switch providerConfig.Type {
		case "openai-compatible":
			providerRegistry.Register(key, openaicompatible.New(key, providerConfig))
		case "anthropic":
			providerRegistry.Register(key, anthropicprovider.New(key, providerConfig))
		default:
			fmt.Fprintf(os.Stderr, "provider setup error: provider %q has unsupported type %q\n", key, providerConfig.Type)
			return 1
		}
	}
	client := provider.NewClient(providerRegistry, catalog)

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
