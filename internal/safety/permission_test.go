package safety

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// ─── ValidateBash ─────────────────────────────────────────────────────────────

func TestValidateBashAllowed(t *testing.T) {
	allowed := []string{
		"echo hello",
		"go build ./...",
		"git status",
		"ls -la",
		"cat README.md",
		"rm somefile.txt",
		"git checkout feature-branch",
		"git clean",
		"git reset --soft",
	}
	for _, cmd := range allowed {
		if err := ValidateBash(cmd); err != nil {
			t.Errorf("ValidateBash(%q) unexpected error: %v", cmd, err)
		}
	}
}

func TestValidateBashBlockedRecursiveDelete(t *testing.T) {
	blocked := []string{
		"rm -rf /",
		"rm  -rf  /",
		"; rm -rf /",
		"&& rm -rf /",
		"| rm -rf /",
	}
	for _, cmd := range blocked {
		err := ValidateBash(cmd)
		if err == nil {
			t.Errorf("ValidateBash(%q) expected block, got nil", cmd)
		}
		if !strings.Contains(err.Error(), "recursive delete") {
			t.Errorf("ValidateBash(%q): wrong error %v", cmd, err)
		}
	}
}

func TestValidateBashBlockedGitHardReset(t *testing.T) {
	blocked := []string{
		"git reset --hard",
		"git reset --hard HEAD~1",
		"git  reset  --hard",
	}
	for _, cmd := range blocked {
		err := ValidateBash(cmd)
		if err == nil {
			t.Errorf("ValidateBash(%q) expected block, got nil", cmd)
		}
		if !strings.Contains(err.Error(), "git hard reset") {
			t.Errorf("ValidateBash(%q): wrong error %v", cmd, err)
		}
	}
}

func TestValidateBashBlockedGitCheckoutDiscard(t *testing.T) {
	blocked := []string{
		"git checkout -- file.txt",
		"git checkout -- .",
	}
	for _, cmd := range blocked {
		err := ValidateBash(cmd)
		if err == nil {
			t.Errorf("ValidateBash(%q) expected block, got nil", cmd)
		}
		if !strings.Contains(err.Error(), "git checkout discard") {
			t.Errorf("ValidateBash(%q): wrong error %v", cmd, err)
		}
	}
}

func TestValidateBashBlockedGitCleanForce(t *testing.T) {
	blocked := []string{
		"git clean -f",
		"git clean -fd",
	}
	for _, cmd := range blocked {
		err := ValidateBash(cmd)
		if err == nil {
			t.Errorf("ValidateBash(%q) expected block, got nil", cmd)
		}
		if !strings.Contains(err.Error(), "git clean force") {
			t.Errorf("ValidateBash(%q): wrong error %v", cmd, err)
		}
	}
}

func TestValidateBashBlockedSudo(t *testing.T) {
	blocked := []string{
		"sudo apt update",
		"&& sudo rm /",
	}
	for _, cmd := range blocked {
		err := ValidateBash(cmd)
		if err == nil {
			t.Errorf("ValidateBash(%q) expected block, got nil", cmd)
		}
		if !strings.Contains(err.Error(), "sudo") {
			t.Errorf("ValidateBash(%q): wrong error %v", cmd, err)
		}
	}
}

func TestValidateBashBlockedDD(t *testing.T) {
	err := ValidateBash("dd if=/dev/zero of=/dev/null")
	if err == nil {
		t.Fatal("expected block for dd")
	}
	if !strings.Contains(err.Error(), "disk overwrite") {
		t.Errorf("wrong error: %v", err)
	}
}

func TestValidateBashBlockedMkfs(t *testing.T) {
	err := ValidateBash("mkfs.ext4 /dev/sda1")
	if err == nil {
		t.Fatal("expected block for mkfs")
	}
	if !strings.Contains(err.Error(), "filesystem format") {
		t.Errorf("wrong error: %v", err)
	}
}

// ─── PermissionManager ───────────────────────────────────────────────────────

func TestPermissionManagerAutoApprove(t *testing.T) {
	mgr := NewPermissionManager(nil, nil, true)

	// Should never prompt, always succeed
	err := mgr.ConfirmBash("echo hello", "/repo", "test")
	if err != nil {
		t.Fatalf("auto-approve ConfirmBash failed: %v", err)
	}

	err = mgr.ConfirmEdit("/repo/foo.txt", "change content")
	if err != nil {
		t.Fatalf("auto-approve ConfirmEdit failed: %v", err)
	}

	err = mgr.ConfirmWrite("/repo/bar.txt", "new file")
	if err != nil {
		t.Fatalf("auto-approve ConfirmWrite failed: %v", err)
	}
}

func TestPermissionManagerDeny(t *testing.T) {
	in := bytes.NewReader([]byte("n\n"))
	var out bytes.Buffer
	mgr := NewPermissionManager(in, &out, false)

	err := mgr.ConfirmBash("echo hello", "/repo", "test")
	if err == nil {
		t.Fatal("expected error on denial")
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("wrong error: %v", err)
	}
}

func TestPermissionManagerAllowYes(t *testing.T) {
	in := bytes.NewReader([]byte("yes\n"))
	var out bytes.Buffer
	mgr := NewPermissionManager(in, &out, false)

	err := mgr.ConfirmBash("echo hello", "/repo", "test")
	if err != nil {
		t.Fatalf("expected no error on yes: %v", err)
	}
}

func TestPermissionManagerAllowY(t *testing.T) {
	in := bytes.NewReader([]byte("Y\n"))
	var out bytes.Buffer
	mgr := NewPermissionManager(in, &out, false)

	err := mgr.ConfirmEdit("/repo/foo.txt", "change content")
	if err != nil {
		t.Fatalf("expected no error on Y: %v", err)
	}
}

func TestPermissionManagerCallbackNoPrompt(t *testing.T) {
	calls := 0
	confirmFn := func(kind, summary string) error {
		calls++
		if kind != "bash" {
			t.Errorf("expected kind=bash, got %q", kind)
		}
		return nil
	}
	mgr := NewCallbackPermissionManager(false, confirmFn)

	err := mgr.ConfirmBash("echo hello", "/repo", "test")
	if err != nil {
		t.Fatalf("callback confirm failed: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 callback call, got %d", calls)
	}
}

func TestPermissionManagerCallbackDeny(t *testing.T) {
	confirmFn := func(kind, summary string) error {
		return errors.New("denied by callback")
	}
	mgr := NewCallbackPermissionManager(false, confirmFn)

	err := mgr.ConfirmWrite("/repo/foo.txt", "test")
	if err == nil {
		t.Fatal("expected error from callback")
	}
}

func TestPermissionManagerConfirmBashBlocked(t *testing.T) {
	mgr := NewCallbackPermissionManager(true, nil)

	err := mgr.ConfirmBash("rm -rf /", "/repo", "bad")
	if err == nil {
		t.Fatal("expected error for blocked command")
	}
	if !strings.Contains(err.Error(), "blocked dangerous bash pattern") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPermissionManagerDenyEOF(t *testing.T) {
	in := bytes.NewReader([]byte("")) // EOF immediately
	var out bytes.Buffer
	mgr := NewPermissionManager(in, &out, false)

	err := mgr.ConfirmEdit("/repo/foo.txt", "change")
	if err == nil {
		t.Fatal("expected error on EOF without answer")
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPermissionManagerConfirmEditSummary(t *testing.T) {
	in := bytes.NewReader([]byte("y\n"))
	var out bytes.Buffer
	mgr := NewPermissionManager(in, &out, false)

	err := mgr.ConfirmEdit("/repo/foo.txt", "replace line 1")
	if err != nil {
		t.Fatalf("ConfirmEdit failed: %v", err)
	}
	if !strings.Contains(out.String(), "/repo/foo.txt") {
		t.Fatalf("summary missing path: %s", out.String())
	}
}

func TestPermissionManagerConfirmWriteSummary(t *testing.T) {
	in := bytes.NewReader([]byte("y\n"))
	var out bytes.Buffer
	mgr := NewPermissionManager(in, &out, false)

	err := mgr.ConfirmWrite("/repo/bar.txt", "new content")
	if err != nil {
		t.Fatalf("ConfirmWrite failed: %v", err)
	}
	if !strings.Contains(out.String(), "/repo/bar.txt") {
		t.Fatalf("summary missing path: %s", out.String())
	}
}

func TestPermissionManagerConfirmBashSummary(t *testing.T) {
	in := bytes.NewReader([]byte("y\n"))
	var out bytes.Buffer
	mgr := NewPermissionManager(in, &out, false)

	err := mgr.ConfirmBash("go build", "/repo/src", "compile")
	if err != nil {
		t.Fatalf("ConfirmBash failed: %v", err)
	}
	summary := out.String()
	for _, want := range []string{"/repo/src", "go build", "compile"} {
		if !strings.Contains(summary, want) {
			t.Errorf("summary missing %q: %s", want, summary)
		}
	}
}

// Regression: fmt.Errorf without wrapping should not panic.
func TestConfirmWriteErrorIsPlain(t *testing.T) {
	mgr := NewCallbackPermissionManager(false, func(kind, summary string) error {
		return fmt.Errorf("just denied")
	})
	err := mgr.ConfirmWrite("/repo/foo.txt", "")
	if err == nil {
		t.Fatal("expected error")
	}
	// Should not panic on unwrap.
	_ = errors.Unwrap(err)
}
