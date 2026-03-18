package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"minioc/internal/session"
)

func TestFileStoreSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)

	sess := session.New("/repo", "/repo", "gpt-4")
	sess.AddMessage(session.RoleUser, "hello")

	ctx := context.Background()
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

	sess := session.New("/repo", "/repo", "gpt-4")
	ctx := context.Background()

	if err := store.Save(ctx, sess); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if _, err := os.Stat(dir); err != nil {
		t.Errorf("session dir not created: %v", err)
	}
}

func TestFileStoreLoadNonexistent(t *testing.T) {
	store := NewFileStore(t.TempDir())
	ctx := context.Background()

	_, err := store.Load(ctx, "does_not_exist")
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
}

func TestFileStoreLoadCorruptedJSON(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)

	badPath := filepath.Join(dir, "bad.json")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
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

	sess := session.New("/repo", "/repo", "gpt-4")
	sess.AddMessage(session.RoleUser, "first")

	ctx := context.Background()
	if err := store.Save(ctx, sess); err != nil {
		t.Fatal(err)
	}

	sess.AddMessage(session.RoleUser, "second")
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

	sess := session.New("/repo", "/repo", "gpt-4")
	ctx := context.Background()

	if err := store.Save(ctx, sess); err != nil {
		t.Fatal(err)
	}

	// After save, only the final .json file should exist (no .tmp left behind)
	globPattern := filepath.Join(dir, sess.ID+"*")
	matches, _ := filepath.Glob(globPattern)
	for _, m := range matches {
		if m == filepath.Join(dir, sess.ID+".tmp") {
			t.Errorf("tmp file left behind after save: %s", m)
		}
	}
}

func TestFileStoreLoadContextCancelled(t *testing.T) {
	store := NewFileStore(t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := store.Load(ctx, "anything")
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestFileStoreSaveContextCancelled(t *testing.T) {
	store := NewFileStore(t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	sess := session.New("/repo", "/repo", "gpt-4")
	err := store.Save(ctx, sess)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestFileStoreSaveLargeSession(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)

	sess := session.New("/repo", "/repo", "gpt-4")
	for i := 0; i < 100; i++ {
		sess.AddMessage(session.RoleUser, "message number "+string(rune('0'+i%10)))
		sess.AddMessage(session.RoleAssistant, "response number "+string(rune('0'+i%10)))
	}

	ctx := context.Background()
	if err := store.Save(ctx, sess); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := store.Load(ctx, sess.ID)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(loaded.Messages) != 200 {
		t.Errorf("expected 200 messages, got %d", len(loaded.Messages))
	}
}
