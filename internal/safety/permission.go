package safety

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
)

type PermissionManager struct {
	autoApprove bool
	reader      *bufio.Reader
	out         io.Writer
	confirmFn   func(kind, summary string) error
}

func NewPermissionManager(in io.Reader, out io.Writer, autoApprove bool) *PermissionManager {
	return &PermissionManager{
		autoApprove: autoApprove,
		reader:      bufio.NewReader(in),
		out:         out,
	}
}

func NewCallbackPermissionManager(autoApprove bool, confirmFn func(kind, summary string) error) *PermissionManager {
	return &PermissionManager{
		autoApprove: autoApprove,
		confirmFn:   confirmFn,
	}
}

func (m *PermissionManager) ConfirmBash(command, workdir, description string) error {
	if err := ValidateBash(command); err != nil {
		return err
	}
	summary := fmt.Sprintf("description: %s\nworkdir: %s\ncommand: %s", description, workdir, command)
	return m.confirm("bash", summary)
}

func (m *PermissionManager) ConfirmEdit(path, summary string) error {
	return m.confirm("edit", fmt.Sprintf("file: %s\n%s", path, summary))
}

func (m *PermissionManager) ConfirmWrite(path, summary string) error {
	return m.confirm("write", fmt.Sprintf("file: %s\n%s", path, summary))
}

func (m *PermissionManager) confirm(kind, summary string) error {
	if m.autoApprove {
		return nil
	}
	if m.confirmFn != nil {
		return m.confirmFn(kind, summary)
	}

	if _, err := fmt.Fprintf(m.out, "\nAllow %s?\n%s\n[y/N]: ", kind, summary); err != nil {
		return err
	}

	line, err := m.reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("read confirmation: %w", err)
	}

	answer := strings.ToLower(strings.TrimSpace(line))
	if answer == "y" || answer == "yes" {
		return nil
	}
	return fmt.Errorf("permission denied")
}

var blockedBashPatterns = []struct {
	name string
	re   *regexp.Regexp
}{
	{name: "recursive delete", re: regexp.MustCompile(`(^|[;&|])\s*rm\s+-rf\b`)},
	{name: "git hard reset", re: regexp.MustCompile(`\bgit\s+reset\s+--hard\b`)},
	{name: "git checkout discard", re: regexp.MustCompile(`\bgit\s+checkout\s+--\b`)},
	{name: "git clean force", re: regexp.MustCompile(`\bgit\s+clean\s+-f`)},
	{name: "sudo", re: regexp.MustCompile(`(^|[;&|])\s*sudo\b`)},
	{name: "disk overwrite", re: regexp.MustCompile(`\bdd\b`)},
	{name: "filesystem format", re: regexp.MustCompile(`\bmkfs\b`)},
}

func ValidateBash(command string) error {
	for _, rule := range blockedBashPatterns {
		if rule.re.MatchString(command) {
			return fmt.Errorf("blocked dangerous bash pattern: %s", rule.name)
		}
	}
	return nil
}
