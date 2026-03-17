package tools

import (
	"strings"
	"testing"
)

func TestRenderDiffPreviewIncludesChangedLines(t *testing.T) {
	before := "alpha\nbeta\ngamma\ndelta\n"
	after := "alpha\nbeta changed\ngamma\ndelta\nomega\n"

	preview := renderDiffPreview(before, after)

	checks := []string{
		"@@ -1,4 +1,5 @@",
		"-    2    . | beta",
		"+    .    2 | beta changed",
		"+    .    5 | omega",
	}
	for _, want := range checks {
		if !strings.Contains(preview, want) {
			t.Fatalf("preview missing %q\n%s", want, preview)
		}
	}
}

func TestBuildEditConfirmationSummaryShowsSelectionsAndDiff(t *testing.T) {
	before := "alpha\nbeta\ngamma\n"
	after := "alpha\nbeta changed\ngamma\n"
	summary := buildEditConfirmationSummary(1, before, after, "beta", "beta changed")

	checks := []string{"old selection:", "new selection:", "diff:", "beta changed"}
	for _, want := range checks {
		if !strings.Contains(summary, want) {
			t.Fatalf("summary missing %q\n%s", want, summary)
		}
	}
}

func TestBuildWriteConfirmationSummaryCreatedUsesPreview(t *testing.T) {
	summary := buildWriteConfirmationSummary("created", "", "one\ntwo\n")
	if !strings.Contains(summary, "preview:") {
		t.Fatalf("expected preview section\n%s", summary)
	}
	if strings.Contains(summary, "diff:") {
		t.Fatalf("did not expect diff section\n%s", summary)
	}
}
