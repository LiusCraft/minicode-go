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
	"minioc/internal/notify"
	"minioc/internal/project"
	"minioc/internal/safety"
	"minioc/internal/session"
	"minioc/internal/subagent"
	"minioc/internal/tools"
	mcpmanager "minioc/internal/tools/mcp"
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

	sessionStore := session.NewFileStore(config.SessionsDir(repoRoot))

	current, err := session.Open(ctx, sessionStore, repoRoot, workdir, cfg.Model, *continueFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "session error: %v\n", err)
		return 1
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

	mcpManager := mcpmanager.NewManager()
	if len(cfg.MCPServers) > 0 {
		mcpManager.StartAll(ctx, cfg.MCPServers)
		mcpManager.RegisterTools(ctx, registry)
	}
	defer mcpManager.CloseAll()

	notify.On("mcp:server_failed", func(e notify.Event) {
		registry.UnregisterAll(e.Source + "__")
	})

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

	subagentMgr := subagent.NewManager(
		client,
		sessionStore,
		registry,
		cfg.Agents,
		safety.NewPermissionManager(nil, nil, true), // subagents always auto-approve
		cfg.MaxSteps,
		repoRoot,
		workdir,
		cfg.Model,
		current.ID,
	)
	registry.Register(subagent.SpawnAgentTool(subagentMgr))
	registry.Register(subagent.ListAgentsTool(subagentMgr))
	registry.Register(subagent.GetAgentSessionTool(subagentMgr))
	registry.Register(subagent.KillAgentTool(subagentMgr))

	loop := agent.Loop{
		Client:       client,
		Store:        sessionStore,
		Tools:        registry,
		MaxSteps:     cfg.MaxSteps,
		Instructions: cfg.Instructions,
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
			SubagentMgr: subagentMgr,
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
