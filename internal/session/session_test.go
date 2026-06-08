package session

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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

func TestOpenNewSession(t *testing.T) {
	store := NewFileStore(t.TempDir())
	sess, err := Open(context.Background(), store, "/repo", "/repo/src", "gpt-4", "")
	if err != nil {
		t.Fatalf("Open new session failed: %v", err)
	}
	if sess.ID == "" {
		t.Fatal("expected non-empty ID")
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
}

func TestOpenResumeSession(t *testing.T) {
	store := NewFileStore(t.TempDir())
	ctx := context.Background()

	original := New("/repo", "/repo/old-src", "claude-3")
	original.AddMessage(RoleUser, "hello")
	if err := store.Save(ctx, original); err != nil {
		t.Fatal(err)
	}

	// resume with updated workdir/model (same repo root)
	sess, err := Open(ctx, store, "/repo", "/repo/src", "gpt-4", original.ID)
	if err != nil {
		t.Fatalf("Open resume failed: %v", err)
	}
	if sess.ID != original.ID {
		t.Errorf("ID: got %q, want %q", sess.ID, original.ID)
	}
	if sess.RepoRoot != "/repo" {
		t.Errorf("RepoRoot: got %q, want %q", sess.RepoRoot, "/repo")
	}
	if sess.Workdir != "/repo/src" {
		t.Errorf("Workdir should be refreshed: got %q, want %q", sess.Workdir, "/repo/src")
	}
	if sess.Model != "gpt-4" {
		t.Errorf("Model should be refreshed: got %q, want %q", sess.Model, "gpt-4")
	}
	if len(sess.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(sess.Messages))
	}
}

func TestOpenResumeRepoMismatch(t *testing.T) {
	store := NewFileStore(t.TempDir())
	ctx := context.Background()

	original := New("/other-repo", "/other-repo", "gpt-4")
	if err := store.Save(ctx, original); err != nil {
		t.Fatal(err)
	}

	_, err := Open(ctx, store, "/my-repo", "/my-repo", "gpt-5", original.ID)
	if err == nil {
		t.Fatal("expected error for repo mismatch")
	}
}

func TestOpenResumeNotFound(t *testing.T) {
	store := NewFileStore(t.TempDir())
	ctx := context.Background()

	_, err := Open(ctx, store, "/repo", "/repo", "gpt-4", "sess_nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
}

func TestFileStoreSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	store := NewFileStore(dir)

	sess := New("/repo", "/repo", "gpt-4")
	sess.AddMessage(RoleUser, "hello")

	if err := store.Save(ctx, sess); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := store.Load(ctx, sess.ID)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.ID != sess.ID {
		t.Errorf("ID: got %q, want %q", loaded.ID, sess.ID)
	}
	if loaded.RepoRoot != sess.RepoRoot {
		t.Errorf("RepoRoot: got %q, want %q", loaded.RepoRoot, sess.RepoRoot)
	}
	if loaded.Model != sess.Model {
		t.Errorf("Model: got %q, want %q", loaded.Model, sess.Model)
	}
	if len(loaded.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(loaded.Messages))
	}
	if loaded.Messages[0].Content != "hello" {
		t.Errorf("Content: got %q, want %q", loaded.Messages[0].Content, "hello")
	}
}

func TestFileStoreSaveCreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sub", "sessions")
	store := NewFileStore(dir)
	sess := New("/repo", "/repo", "gpt-4")
	ctx := context.Background()

	if err := store.Save(ctx, sess); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("session dir not created: %v", err)
	}
}

func TestFileStoreLoadNonexistent(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)
	ctx := context.Background()

	_, err := store.Load(ctx, "does_not_exist")
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
}

func TestFileStoreLoadCorruptedJSON(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	badPath := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(badPath, []byte("not json{"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	_, err := store.Load(ctx, "bad")
	if err == nil {
		t.Fatal("expected error for corrupted JSON")
	}
}

func TestFileStoreSaveOverwrites(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)
	ctx := context.Background()

	sess := New("/repo", "/repo", "gpt-4")
	sess.AddMessage(RoleUser, "first")

	if err := store.Save(ctx, sess); err != nil {
		t.Fatal(err)
	}

	sess.AddMessage(RoleUser, "second")
	if err := store.Save(ctx, sess); err != nil {
		t.Fatal(err)
	}

	loaded, err := store.Load(ctx, sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(loaded.Messages))
	}
}

func TestFileStoreSaveAtomicRename(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)
	ctx := context.Background()

	sess := New("/repo", "/repo", "gpt-4")
	if err := store.Save(ctx, sess); err != nil {
		t.Fatal(err)
	}

	globPattern := filepath.Join(dir, sess.ID+"*")
	matches, _ := filepath.Glob(globPattern)
	for _, m := range matches {
		if m == filepath.Join(dir, sess.ID+".tmp") {
			t.Errorf("tmp file left behind after save: %s", m)
		}
	}
}

func TestFileStoreLoadContextCancelled(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := store.Load(ctx, "anything")
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestFileStoreSaveContextCancelled(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	sess := New("/repo", "/repo", "gpt-4")
	err := store.Save(ctx, sess)
	if err == nil {
		t.Fatal("expected error from cancelled context")
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
