package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"minioc/internal/llm"
	"minioc/internal/safety"
	"minioc/internal/session"
	"minioc/internal/store"
	"minioc/internal/tools"
)

type scriptedClient struct {
	mu       sync.Mutex
	results  []llm.Result
	requests []llm.Request
}

func (c *scriptedClient) Run(_ context.Context, req llm.Request) (llm.Result, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.requests = append(c.requests, req)
	if len(c.results) == 0 {
		return llm.Result{}, fmt.Errorf("unexpected llm call")
	}
	result := c.results[0]
	c.results = c.results[1:]
	return result, nil
}

type noopStore struct{}

func (noopStore) Load(context.Context, string) (*session.Session, error) {
	return nil, fmt.Errorf("not implemented")
}

func (noopStore) Save(context.Context, *session.Session) error {
	return nil
}

var _ store.Store = noopStore{}

func TestLoopRunsParallelSafeToolCallsConcurrently(t *testing.T) {
	started := make(chan string, 2)
	release := make(chan struct{})
	registry := tools.NewRegistry(
		tools.Spec{
			Name:         "fast_one",
			ParallelSafe: true,
			Execute: func(context.Context, tools.CallContext, json.RawMessage) (tools.Result, error) {
				started <- "fast_one"
				<-release
				time.Sleep(20 * time.Millisecond)
				return tools.Result{Output: "first"}, nil
			},
		},
		tools.Spec{
			Name:         "fast_two",
			ParallelSafe: true,
			Execute: func(context.Context, tools.CallContext, json.RawMessage) (tools.Result, error) {
				started <- "fast_two"
				<-release
				return tools.Result{Output: "second"}, nil
			},
		},
	)

	client := &scriptedClient{results: []llm.Result{
		{ToolCalls: []llm.ToolCall{{ID: "call_1", Name: "fast_one"}, {ID: "call_2", Name: "fast_two"}}},
		{Text: "done"},
	}}

	loop := Loop{Client: client, Store: noopStore{}, Tools: registry, MaxSteps: 4}
	sess := session.New("/repo", "/repo", "test-model")
	permissions := safety.NewPermissionManager(nilReader{}, io.Discard, true)

	resultCh := make(chan struct {
		answer string
		err    error
	}, 1)
	go func() {
		answer, err := loop.Run(context.Background(), sess, permissions, "inspect repo", nil)
		resultCh <- struct {
			answer string
			err    error
		}{answer: answer, err: err}
	}()

	first := waitForStart(t, started)
	second := waitForStart(t, started)
	if first == second {
		t.Fatalf("expected two distinct tool starts, got %q twice", first)
	}
	close(release)

	result := waitForResult(t, resultCh)
	if result.err != nil {
		t.Fatalf("Run returned error: %v", result.err)
	}
	if result.answer != "done" {
		t.Fatalf("unexpected answer %q", result.answer)
	}

	if len(sess.Messages) != 5 {
		t.Fatalf("expected 5 session messages, got %d", len(sess.Messages))
	}
	if sess.Messages[2].ToolName != "fast_one" || sess.Messages[3].ToolName != "fast_two" {
		t.Fatalf("tool results should keep call order, got %q then %q", sess.Messages[2].ToolName, sess.Messages[3].ToolName)
	}
}

func TestLoopKeepsUnsafeToolCallsSequential(t *testing.T) {
	started := make(chan string, 2)
	releaseFirst := make(chan struct{})
	releaseSecond := make(chan struct{})
	registry := tools.NewRegistry(
		tools.Spec{
			Name: "write_like_one",
			Execute: func(context.Context, tools.CallContext, json.RawMessage) (tools.Result, error) {
				started <- "write_like_one"
				<-releaseFirst
				return tools.Result{Output: "first"}, nil
			},
		},
		tools.Spec{
			Name: "write_like_two",
			Execute: func(context.Context, tools.CallContext, json.RawMessage) (tools.Result, error) {
				started <- "write_like_two"
				<-releaseSecond
				return tools.Result{Output: "second"}, nil
			},
		},
	)

	client := &scriptedClient{results: []llm.Result{
		{ToolCalls: []llm.ToolCall{{ID: "call_1", Name: "write_like_one"}, {ID: "call_2", Name: "write_like_two"}}},
		{Text: "done"},
	}}

	loop := Loop{Client: client, Store: noopStore{}, Tools: registry, MaxSteps: 4}
	sess := session.New("/repo", "/repo", "test-model")
	permissions := safety.NewPermissionManager(nilReader{}, io.Discard, true)

	resultCh := make(chan struct {
		answer string
		err    error
	}, 1)
	go func() {
		answer, err := loop.Run(context.Background(), sess, permissions, "inspect repo", nil)
		resultCh <- struct {
			answer string
			err    error
		}{answer: answer, err: err}
	}()

	if first := waitForStart(t, started); first != "write_like_one" {
		t.Fatalf("expected first sequential tool to start first, got %q", first)
	}
	assertNoAdditionalStart(t, started)
	close(releaseFirst)
	if second := waitForStart(t, started); second != "write_like_two" {
		t.Fatalf("expected second sequential tool after first completes, got %q", second)
	}
	close(releaseSecond)

	result := waitForResult(t, resultCh)
	if result.err != nil {
		t.Fatalf("Run returned error: %v", result.err)
	}
	if result.answer != "done" {
		t.Fatalf("unexpected answer %q", result.answer)
	}
}

type nilReader struct{}

func (nilReader) Read([]byte) (int, error) {
	return 0, io.EOF
}

func waitForStart(t *testing.T, started <-chan string) string {
	t.Helper()
	select {
	case name := <-started:
		return name
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for tool start")
		return ""
	}
}

func assertNoAdditionalStart(t *testing.T, started <-chan string) {
	t.Helper()
	select {
	case name := <-started:
		t.Fatalf("unexpected concurrent start for %q", name)
	case <-time.After(60 * time.Millisecond):
	}
}

func waitForResult(t *testing.T, resultCh <-chan struct {
	answer string
	err    error
}) struct {
	answer string
	err    error
} {
	t.Helper()
	select {
	case result := <-resultCh:
		return result
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for loop result")
		return struct {
			answer string
			err    error
		}{}
	}
}
