# Finer Tool Scheduling

- Priority: low
- Status: backlog
- Area: tool execution, concurrency, safety

## Why

The current scheduler can run only explicitly parallel-safe read-only tools concurrently.
That keeps behavior safe and predictable, but it leaves performance on the table for more nuanced cases.

## Goal

Improve tool scheduling so the agent can execute more independent work concurrently without weakening safety guarantees or making output harder to follow.

## Possible directions

1. Add a concurrency limit instead of launching every safe tool call at once.
2. Distinguish tool classes such as read-only, path-scoped write, exclusive, and interactive.
3. Add path-level locking so writes to different files can run concurrently while conflicting paths stay serialized.
4. Keep confirmation prompts and exclusive operations serialized even when other work is parallelized.
5. Decide how cancellation and failures should propagate within a concurrent batch.
6. Improve progress rendering so concurrent tool activity is still easy to understand.

## Initial implementation sketch

1. Extend tool metadata from a boolean `ParallelSafe` flag to a scheduling policy.
2. Introduce a small scheduler that groups calls by policy and applies a worker limit.
3. Add path-aware locking for file-mutating tools.
4. Keep session writes ordered by original tool call order.
5. Add tests for concurrency limits, locking, and failure handling.

## Non-goals for now

- Parallelizing all `bash` calls by default.
- Reordering tool results in session history.
- Adding speculative execution across dependent tool calls.
