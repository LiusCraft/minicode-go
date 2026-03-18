# minioc

`minioc` is a minimal local AI coding agent in Go. It now separates provider routing from model catalog metadata, so OpenAI-compatible and Anthropic backends can evolve without changing the agent loop.

## What is in this first cut

- CLI entrypoint in `cmd/minioc/main.go`
- Bubble Tea TUI as the default interface plus plain streaming CLI mode
- Session loop with `max_steps` in `internal/agent/loop.go`
- Provider registry and adapters in `internal/llm/provider/`
- Model catalog with strict `provider/model` references in `internal/llm/models/catalog.go`
- Tool registry plus `read_file`, `glob`, `grep`, `bash`, `edit`, and `write_file`
- Repo-bound path checks and confirmation prompts for write/edit/bash tools
- Streaming assistant output plus concise tool activity in the CLI
- Line-based edit diffs and write previews in confirmation prompts and tool results
- Parallel-safe tool calls can execute concurrently in one model step
- Default resource files under `assets/`
- JSON session persistence under `.minioc/sessions/`

## Quick start

`minioc` stores default static resources under `assets/`.

- `assets/minioc.json`: default configuration template

Runtime data is written under `.minioc/` (auto-created):

- `.minioc/minioc.json`: runtime configuration
- `.minioc/sessions/`: session data

Example configuration:

```json
{
  "model": "openai/gpt-5-mini",
  "max_steps": 1000,
  "auto_approve": false,
  "providers": {
    "openai": {
      "type": "openai-compatible",
      "base_url": "https://api.openai.com/v1",
      "api_key": { "env": "OPENAI_API_KEY" }
    },
    "anthropic": {
      "type": "anthropic",
      "base_url": "https://api.anthropic.com",
      "api_key": { "env": "ANTHROPIC_API_KEY" }
    }
  },
  "models": {
    "openai/gpt-5-mini": {
      "provider": "openai",
      "id": "gpt-5-mini",
      "supports_tools": true
    },
    "anthropic/claude-sonnet-4": {
      "provider": "anthropic",
      "id": "claude-sonnet-4-20250514",
      "supports_tools": true,
      "max_output_tokens": 8192
    }
  }
}
```

If a provider uses an env-based API key source, export the variable first:

```sh
export OPENAI_API_KEY=your_key_here
export ANTHROPIC_API_KEY=your_key_here
```

You can also set `api_key` directly to a string in `.minioc/minioc.json` when you do not want env resolution.

Launch the TUI (default mode):

```sh
go run ./cmd/minioc
```

Run against the current repository with an initial prompt:

```sh
go run ./cmd/minioc -- "summarize this repository"
```

Continue a previous session:

```sh
go run ./cmd/minioc --continue sess_xxxxx "now add tests"
```

Use plain streaming CLI mode (no TUI):

```sh
go run ./cmd/minioc --no-tui -- "summarize this repository"
```

Useful flags:

- `-C` to change the working directory
- `--continue` to resume a previous session id
- `--no-tui` to force plain streaming CLI mode

## Current limitations

- Session persistence is JSON-based, not SQLite yet
- Side-effecting tools still execute serially; only parallel-safe calls are batched
- Conversation context is sent from local session history on each step
- `bash` uses a conservative blocklist, not a full command parser
- Config runtime path is `.minioc/minioc.json`, initialized from `assets/minioc.json` if missing
- Auth supports API key sources today; interactive `auth login` is not implemented yet

## Suggested next steps

1. Replace JSON session storage with SQLite
2. Add tests for path safety, tool behavior, and loop control
3. Broaden safe parallel scheduling beyond read-only tools (see `backlog/finer-tool-scheduling.md`)
4. Support richer streamed progress metadata for long-running tool steps

Lower-priority feature planning lives under `backlog/`.
