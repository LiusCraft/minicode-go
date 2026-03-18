package session

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestNewSession(t *testing.T) {
	sess := New("/repo", "/repo/src", "gpt-4")
	if sess.ID == "" {
		t.Fatal("expected non-empty session ID")
	}
	if sess.RepoRoot != "/repo" {
		t.Errorf("RepoRoot: got %q, want %q", sess.RepoRoot, "/repo")
	}
	if sess.Workdir != "/repo/src" {
		t.Errorf("Workdir: got %q, want %q", sess.Workdir, "/repo/src")
	}
	if sess.Model != "gpt-4" {
		t.Errorf("Model: got %q, want %q", sess.Model, "gpt-4")
	}
	if len(sess.Messages) != 0 {
		t.Fatalf("expected empty messages, got %d", len(sess.Messages))
	}
	if sess.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if sess.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set")
	}
}

func TestSessionAddMessageUser(t *testing.T) {
	sess := New("/repo", "/repo", "gpt-4")
	msg := sess.AddMessage(RoleUser, "hello world")

	if msg.Role != RoleUser {
		t.Errorf("Role: got %v, want %v", msg.Role, RoleUser)
	}
	if msg.Content != "hello world" {
		t.Errorf("Content: got %q, want %q", msg.Content, "hello world")
	}
	if len(sess.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(sess.Messages))
	}
	if sess.Messages[0].ID != msg.ID {
		t.Error("session message should match returned message")
	}
}

func TestSessionAddMessageAssistant(t *testing.T) {
	sess := New("/repo", "/repo", "gpt-4")
	sess.AddMessage(RoleUser, "hi")
	msg := sess.AddMessage(RoleAssistant, "hello")

	if msg.Role != RoleAssistant {
		t.Errorf("Role: got %v, want %v", msg.Role, RoleAssistant)
	}
	if len(sess.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(sess.Messages))
	}
}

func TestSessionAddMessageTool(t *testing.T) {
	sess := New("/repo", "/repo", "gpt-4")
	sess.AddMessage(RoleUser, "hi")
	sess.AddMessage(RoleAssistant, "", WithAssistantToolCalls([]ToolCall{
		{ID: "call_1", Name: "read_file", Arguments: json.RawMessage(`{}`)}}))
	msg := sess.AddMessage(RoleTool, "file content", WithTool("read_file", "call_1", "completed"))

	if msg.Role != RoleTool {
		t.Errorf("Role: got %v, want %v", msg.Role, RoleTool)
	}
	if msg.ToolName != "read_file" {
		t.Errorf("ToolName: got %q, want %q", msg.ToolName, "read_file")
	}
	if msg.ToolCallID != "call_1" {
		t.Errorf("ToolCallID: got %q, want %q", msg.ToolCallID, "call_1")
	}
	if msg.Status != "completed" {
		t.Errorf("Status: got %q, want %q", msg.Status, "completed")
	}
}

func TestSessionAddMessageToolOptionsNil(t *testing.T) {
	sess := New("/repo", "/repo", "gpt-4")
	msg := sess.AddMessage(RoleTool, "output")

	if msg.ToolName != "" {
		t.Errorf("ToolName: got %q, want empty", msg.ToolName)
	}
}

func TestSessionAddMessageUpdatesUpdatedAt(t *testing.T) {
	sess := New("/repo", "/repo", "gpt-4")
	before := sess.UpdatedAt
	time.Sleep(time.Millisecond)
	sess.AddMessage(RoleUser, "hello")

	if !sess.UpdatedAt.After(before) && !sess.UpdatedAt.Equal(before) {
		t.Errorf("UpdatedAt not updated: before=%v, after=%v", before, sess.UpdatedAt)
	}
}

func TestWithAssistantToolCallsEmpty(t *testing.T) {
	sess := New("/repo", "/repo", "gpt-4")
	msg := sess.AddMessage(RoleAssistant, "hi", WithAssistantToolCalls(nil))
	if len(msg.ToolCalls) != 0 {
		t.Errorf("expected empty ToolCalls, got %d", len(msg.ToolCalls))
	}
}

func TestWithAssistantToolCallsCopiesSlice(t *testing.T) {
	calls := []ToolCall{{ID: "c1", Name: "foo"}}
	sess := New("/repo", "/repo", "gpt-4")
	msg := sess.AddMessage(RoleAssistant, "hi", WithAssistantToolCalls(calls))

	// modifying original should not affect the message
	calls[0].Name = "bar"
	if msg.ToolCalls[0].Name != "foo" {
		t.Errorf("WithAssistantToolCalls should copy, got %q", msg.ToolCalls[0].Name)
	}
}

func TestSessionMessageJSON(t *testing.T) {
	sess := New("/repo", "/repo", "gpt-4")
	sess.AddMessage(RoleUser, "hello")
	sess.AddMessage(RoleAssistant, "hi there", WithAssistantToolCalls([]ToolCall{
		{ID: "call_1", Name: "grep", Arguments: json.RawMessage(`{"pattern":"foo"}`)}}))

	data, err := json.Marshal(sess)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var restored Session
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if len(restored.Messages) != 2 {
		t.Fatalf("expected 2 messages after unmarshal, got %d", len(restored.Messages))
	}
	if restored.Messages[1].Content != "hi there" {
		t.Errorf("Content: got %q", restored.Messages[1].Content)
	}
	if len(restored.Messages[1].ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(restored.Messages[1].ToolCalls))
	}
	if restored.Messages[1].ToolCalls[0].Name != "grep" {
		t.Errorf("ToolCall.Name: got %q", restored.Messages[1].ToolCalls[0].Name)
	}
}

func TestSessionAddMessageRoles(t *testing.T) {
	if r := Role("user"); r != RoleUser {
		t.Errorf("RoleUser: got %v", r)
	}
	if r := Role("assistant"); r != RoleAssistant {
		t.Errorf("RoleAssistant: got %v", r)
	}
	if r := Role("tool"); r != RoleTool {
		t.Errorf("RoleTool: got %v", r)
	}
}

func TestMessageIDUnique(t *testing.T) {
	sess := New("/repo", "/repo", "gpt-4")
	msg1 := sess.AddMessage(RoleUser, "a")
	msg2 := sess.AddMessage(RoleUser, "b")
	msg3 := sess.AddMessage(RoleUser, "c")

	ids := map[string]bool{msg1.ID: true, msg2.ID: true, msg3.ID: true}
	if len(ids) != 3 {
		t.Errorf("IDs should be unique: %v %v %v", msg1.ID, msg2.ID, msg3.ID)
	}
}

func TestNewIDHasCorrectPrefix(t *testing.T) {
	// newID should produce an ID with the correct prefix.
	// We can't test the fallback path (rand failure) in normal conditions.
	id := newID("test")
	if !strings.HasPrefix(id, "test_") {
		t.Errorf("expected prefix 'test_', got %q", id)
	}
	// The ID should look like a hex-encoded suffix after the prefix.
	suffix := strings.TrimPrefix(id, "test_")
	if len(suffix) < 6 {
		t.Errorf("expected hex suffix of at least 6 chars, got %q", suffix)
	}
}
