package safety

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ─── ResolvePath ──────────────────────────────────────────────────────────────

func TestResolvePathEmptyTarget(t *testing.T) {
	_, err := ResolvePath("/repo", "/repo", "   ")
	if err == nil {
		t.Fatal("expected error for empty path")
	}
	if !strings.Contains(err.Error(), "path is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolvePathRelative(t *testing.T) {
	repoRoot := t.TempDir()
	subdir := filepath.Join(repoRoot, "src")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(subdir, "foo.txt")
	if err := os.WriteFile(file, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ResolvePath(repoRoot, subdir, "foo.txt")
	if err != nil {
		t.Fatalf("ResolvePath failed: %v", err)
	}
	normGot, _ := filepath.EvalSymlinks(got)
	normWant, _ := filepath.EvalSymlinks(file)
	if normGot != normWant {
		t.Fatalf("expected %q, got %q", normWant, normGot)
	}
}

func TestResolvePathAbsolute(t *testing.T) {
	repoRoot := t.TempDir()
	file := filepath.Join(repoRoot, "abs.txt")
	if err := os.WriteFile(file, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ResolvePath(repoRoot, repoRoot, file)
	if err != nil {
		t.Fatalf("ResolvePath failed: %v", err)
	}
	normGot, _ := filepath.EvalSymlinks(got)
	normWant, _ := filepath.EvalSymlinks(file)
	if normGot != normWant {
		t.Fatalf("expected %q, got %q", normWant, normGot)
	}
}

func TestResolvePathOutsideRepoRoot(t *testing.T) {
	repoRoot := t.TempDir()
	parent := filepath.Dir(repoRoot)

	_, err := ResolvePath(repoRoot, repoRoot, parent+"/evil.txt")
	if err == nil {
		t.Fatal("expected error for path outside repo")
	}
	if !strings.Contains(err.Error(), "outside repo root") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolvePathFollowsSymlinks(t *testing.T) {
	repoRoot := t.TempDir()
	realDir := filepath.Join(repoRoot, "real")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	realFile := filepath.Join(realDir, "file.txt")
	if err := os.WriteFile(realFile, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	link := filepath.Join(repoRoot, "link")
	if err := os.Symlink(realDir, link); err != nil {
		t.Fatal(err)
	}

	// Use filepath.Join with repoRoot to get a consistent path form (no trailing slash issues).
	symlinkPath := filepath.Join(repoRoot, "link", "file.txt")
	got, err := ResolvePath(repoRoot, repoRoot, symlinkPath)
	if err != nil {
		t.Fatalf("ResolvePath failed on symlink: %v", err)
	}
	// Canonical paths must match.
	normGot, _ := filepath.EvalSymlinks(got)
	normWant, _ := filepath.EvalSymlinks(realFile)
	if normGot != normWant {
		t.Fatalf("expected canonical path %q, got %q", normWant, normGot)
	}
	// And both must stay within repoRoot.
	normRepo, _ := filepath.EvalSymlinks(repoRoot)
	if !strings.HasPrefix(normGot, normRepo) {
		t.Fatalf("resolved path %q outside repoRoot %q", normGot, normRepo)
	}
}

func TestResolvePathLinkEscapesViaParent(t *testing.T) {
	repoRoot := t.TempDir()
	parent := filepath.Dir(repoRoot)
	evilLink := filepath.Join(repoRoot, "escape")
	if err := os.Symlink(parent, evilLink); err != nil {
		t.Fatal(err)
	}

	_, err := ResolvePath(repoRoot, repoRoot, evilLink+"/some_file")
	if err == nil {
		t.Fatal("expected error for symlink escape")
	}
	if !strings.Contains(err.Error(), "outside repo root") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ─── ResolveDir ──────────────────────────────────────────────────────────────

func TestResolveDirDefaultsToWorkdir(t *testing.T) {
	repoRoot := t.TempDir()
	subdir := filepath.Join(repoRoot, "sub")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := ResolveDir(repoRoot, subdir, "")
	if err != nil {
		t.Fatalf("ResolveDir failed: %v", err)
	}
	normGot, _ := filepath.EvalSymlinks(got)
	normWant, _ := filepath.EvalSymlinks(subdir)
	if normGot != normWant {
		t.Fatalf("expected %q, got %q", normWant, normGot)
	}
}

func TestResolveDirNotADirectory(t *testing.T) {
	repoRoot := t.TempDir()
	file := filepath.Join(repoRoot, "file.txt")
	if err := os.WriteFile(file, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := ResolveDir(repoRoot, repoRoot, "file.txt")
	if err == nil {
		t.Fatal("expected error for non-directory")
	}
	if !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveDirNonexistent(t *testing.T) {
	repoRoot := t.TempDir()

	_, err := ResolveDir(repoRoot, repoRoot, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent path")
	}
}

// ─── EnsureWithinRepo ────────────────────────────────────────────────────────

func TestEnsureWithinRepoExactMatch(t *testing.T) {
	repoRoot := t.TempDir()
	if err := EnsureWithinRepo(repoRoot, repoRoot); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestEnsureWithinRepoSubdir(t *testing.T) {
	repoRoot := t.TempDir()
	sub := filepath.Join(repoRoot, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := EnsureWithinRepo(repoRoot, sub); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestEnsureWithinRepoOutside(t *testing.T) {
	repoRoot := t.TempDir()
	parent := filepath.Dir(repoRoot)
	if err := EnsureWithinRepo(repoRoot, parent); err == nil {
		t.Fatal("expected error")
	}
}

func TestEnsureWithinRepoSibling(t *testing.T) {
	repoRoot := t.TempDir()
	parent := filepath.Dir(repoRoot)
	sibling := filepath.Join(parent, "sibling")
	if err := EnsureWithinRepo(repoRoot, sibling); err == nil {
		t.Fatal("expected error for sibling path")
	}
}
