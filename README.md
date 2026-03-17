# minioc

`minioc` is a minimal local AI coding agent in Go. It follows the MVP shape from `mini-opencode-go.md`: CLI input, model loop, tool calling, file edits, bash execution, path safety, and session persistence.

## What is in this first cut

- CLI entrypoint in `cmd/minioc/main.go`
- Session loop with `max-steps` in `internal/agent/loop.go`
- OpenAI Chat Completions client in `internal/llm/openai.go`
- Tool registry plus `read_file`, `glob`, `grep`, `bash`, `edit`, and `write_file`
- Repo-bound path checks and confirmation prompts for write/edit/bash tools
- Streaming assistant output plus concise tool activity in the CLI
- Line-based edit diffs and write previews in confirmation prompts and tool results
- Parallel-safe tool calls can execute concurrently in one model step
- JSON session persistence under `.minioc/sessions/`

## Quick start

Set your API key:

```sh
export OPENAI_API_KEY=your_key_here
```

Use an OpenAI-compatible provider endpoint when needed:

```sh
export OPENAI_BASE_URL=https://api.deepseek.com
export MINIOC_MODEL=deepseek-chat
```

Run against the current repository:

```sh
go run ./cmd/minioc -- "summarize this repository"
```

Continue a previous session:

```sh
go run ./cmd/minioc --continue sess_xxxxx "now add tests"
```

Useful flags:

- `-C` to change the working directory
- `--model` to override the model name
- `--max-steps` to cap the tool loop
- `--auto-approve` to skip local confirmations for write/edit/bash
- `OPENAI_BASE_URL` to target an OpenAI-compatible API endpoint

## Current limitations

- Session persistence is JSON-based, not SQLite yet
- Side-effecting tools still execute serially; only parallel-safe calls are batched
- Conversation context is sent from local session history on each step
- `bash` uses a conservative blocklist, not a full command parser

## Suggested next steps

1. Replace JSON session storage with SQLite
2. Add tests for path safety, tool behavior, and loop control
3. Broaden safe parallel scheduling beyond read-only tools
4. Support richer streamed progress metadata for long-running tool steps
