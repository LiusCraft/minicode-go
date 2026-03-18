package tools

import (
	"strings"
	"testing"
)

// ─── truncateText ─────────────────────────────────────────────────────────────

func TestTruncateTextNoOp(t *testing.T) {
	short := "hello world"
	text, truncated := truncateText(short)
	if text != short {
		t.Errorf("expected unchanged, got %q", text)
	}
	if truncated {
		t.Error("should not be truncated")
	}
}

func TestTruncateTextByBytes(t *testing.T) {
	// Build a string longer than maxOutputBytes (50KB)
	long := strings.Repeat("x", 60*1024)
	text, truncated := truncateText(long)
	if !truncated {
		t.Error("should be truncated")
	}
	if !strings.Contains(text, "... (output truncated by bytes)") {
		t.Errorf("missing truncation notice: %s", text)
	}
	if len(text) > maxOutputBytes+100 {
		t.Errorf("still too long: %d bytes", len(text))
	}
}

func TestTruncateTextByLines(t *testing.T) {
	// More than maxOutputLines (2000) but short bytes
	lines := make([]string, 0, 3000)
	for i := 0; i < 3000; i++ {
		lines = append(lines, "short line")
	}
	text, truncated := truncateText(strings.Join(lines, "\n"))
	if !truncated {
		t.Error("should be truncated")
	}
	if !strings.Contains(text, "... (output truncated by lines)") {
		t.Errorf("missing truncation notice: %s", text)
	}
}

func TestClipLineShort(t *testing.T) {
	short := "hello"
	if got := clipLine(short); got != short {
		t.Errorf("expected unchanged %q, got %q", short, got)
	}
}

func TestClipLineLong(t *testing.T) {
	long := strings.Repeat("a", 3000)
	got := clipLine(long)
	if !strings.HasSuffix(got, "... (line truncated)") {
		t.Errorf("missing truncation notice: %s", got)
	}
	if len(got) > maxLineLength+50 {
		t.Errorf("still too long: %d chars", len(got))
	}
}

func TestClipLineExactlyMax(t *testing.T) {
	exact := strings.Repeat("b", maxLineLength)
	if got := clipLine(exact); got != exact {
		t.Errorf("expected unchanged at max length, got %q", got)
	}
}

// ─── splitPreviewLines ────────────────────────────────────────────────────────

func TestSplitPreviewLines(t *testing.T) {
	cases := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"one", []string{"one"}},
		{"one\ntwo", []string{"one", "two"}},
		{"one\ntwo\n", []string{"one", "two"}},     // trailing newline stripped
		{"one\n\ntwo", []string{"one", "", "two"}}, // empty line kept
		{"\n\n", []string{"", ""}},                 // two newlines → two empty lines (after strip trailing)
	}

	for _, c := range cases {
		got := splitPreviewLines(c.input)
		// nil and empty []string are equivalent for "empty" purposes
		gotEmpty := len(got) == 0
		wantEmpty := len(c.want) == 0
		if gotEmpty && wantEmpty {
			continue
		}
		if len(got) != len(c.want) {
			t.Errorf("split(%q): got %v (len=%d), want %v (len=%d)", c.input, got, len(got), c.want, len(c.want))
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("split(%q)[%d]: got %q, want %q", c.input, i, got[i], c.want[i])
			}
		}
	}
}

func TestCountPreviewLines(t *testing.T) {
	if n := countPreviewLines("a\nb\nc"); n != 3 {
		t.Errorf("expected 3, got %d", n)
	}
	if n := countPreviewLines(""); n != 0 {
		t.Errorf("expected 0, got %d", n)
	}
}

func TestFormatPreviewLineTab(t *testing.T) {
	got := formatPreviewLine("a\tb")
	if !strings.Contains(got, "    ") {
		t.Errorf("tab not expanded: %s", got)
	}
}

func TestFormatPreviewLineBlank(t *testing.T) {
	if got := formatPreviewLine(""); got != "<blank>" {
		t.Errorf("expected <blank>, got %q", got)
	}
}

func TestFormatPreviewLineTruncate(t *testing.T) {
	long := strings.Repeat("x", 200)
	got := formatPreviewLine(long)
	if !strings.HasSuffix(got, "...") {
		t.Errorf("expected truncation: %s", got)
	}
}

// ─── renderLinePreview ────────────────────────────────────────────────────────

func TestRenderLinePreviewEmpty(t *testing.T) {
	got := renderLinePreview("", 8)
	if got != "(empty file)" {
		t.Errorf("expected (empty file), got %q", got)
	}
}

func TestRenderLinePreviewUnderLimit(t *testing.T) {
	got := renderLinePreview("one\ntwo\nthree", 8)
	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(lines))
	}
}

func TestRenderLinePreviewExceedsLimit(t *testing.T) {
	got := renderLinePreview("one\ntwo\nthree\nfour", 2)
	if !strings.Contains(got, "... (2 more line(s))") {
		t.Errorf("missing ellipsis: %s", got)
	}
}

// ─── buildDiffEntries ─────────────────────────────────────────────────────────

func TestBuildDiffEntriesEqual(t *testing.T) {
	entries := buildDiffEntries([]string{"a", "b"}, []string{"a", "b"})
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	for _, e := range entries {
		if e.kind != diffEqual {
			t.Errorf("expected diffEqual, got %c", e.kind)
		}
	}
}

func TestBuildDiffEntriesInsert(t *testing.T) {
	entries := buildDiffEntries([]string{"a"}, []string{"a", "b"})
	found := false
	for _, e := range entries {
		if e.kind == diffInsert && e.text == "b" {
			found = true
		}
	}
	if !found {
		t.Error("expected insert entry for 'b'")
	}
}

func TestBuildDiffEntriesDelete(t *testing.T) {
	entries := buildDiffEntries([]string{"a", "b"}, []string{"a"})
	found := false
	for _, e := range entries {
		if e.kind == diffDelete && e.text == "b" {
			found = true
		}
	}
	if !found {
		t.Error("expected delete entry for 'b'")
	}
}

func TestBuildDiffEntriesEmptyBoth(t *testing.T) {
	if entries := buildDiffEntries(nil, nil); len(entries) != 0 {
		t.Errorf("expected empty, got %d entries", len(entries))
	}
}

func TestBuildDiffEntriesEmptyBefore(t *testing.T) {
	entries := buildDiffEntries(nil, []string{"a", "b"})
	if len(entries) != 2 {
		t.Errorf("expected 2, got %d", len(entries))
	}
	for _, e := range entries {
		if e.kind != diffInsert {
			t.Errorf("expected insert, got %c", e.kind)
		}
	}
}

func TestBuildDiffEntriesEmptyAfter(t *testing.T) {
	entries := buildDiffEntries([]string{"a", "b"}, nil)
	if len(entries) != 2 {
		t.Errorf("expected 2, got %d", len(entries))
	}
	for _, e := range entries {
		if e.kind != diffDelete {
			t.Errorf("expected delete, got %c", e.kind)
		}
	}
}

// ─── buildDiffWindows ─────────────────────────────────────────────────────────

func TestBuildDiffWindowsContiguous(t *testing.T) {
	entries := []diffEntry{
		{kind: diffEqual, oldLine: 1, newLine: 1},
		{kind: diffDelete, oldLine: 2},
		{kind: diffInsert, newLine: 2},
		{kind: diffEqual, oldLine: 3, newLine: 3},
	}
	windows := buildDiffWindows(entries, 1)
	if len(windows) != 1 {
		t.Fatalf("expected 1 window, got %d", len(windows))
	}
}

func TestBuildDiffWindowsMultipleSeparate(t *testing.T) {
	entries := []diffEntry{
		{kind: diffEqual, oldLine: 1, newLine: 1},
		{kind: diffDelete, oldLine: 2},
		{kind: diffEqual, oldLine: 3, newLine: 3},
		{kind: diffInsert, newLine: 3},
		{kind: diffEqual, oldLine: 4, newLine: 4},
	}
	windows := buildDiffWindows(entries, 0)
	if len(windows) != 2 {
		t.Errorf("expected 2 windows, got %d", len(windows))
	}
}

func TestBuildDiffWindowsMergeOverlap(t *testing.T) {
	entries := []diffEntry{
		{kind: diffDelete, oldLine: 1},
		{kind: diffInsert, newLine: 1},
		{kind: diffDelete, oldLine: 2},
		{kind: diffInsert, newLine: 2},
	}
	windows := buildDiffWindows(entries, 2)
	if len(windows) != 1 {
		t.Errorf("expected 1 merged window, got %d", len(windows))
	}
}

// ─── formatHunkHeader ─────────────────────────────────────────────────────────

func TestFormatHunkHeader(t *testing.T) {
	entries := []diffEntry{
		{kind: diffDelete, oldLine: 5},
		{kind: diffInsert, newLine: 5},
		{kind: diffEqual, oldLine: 6, newLine: 6},
	}
	header := formatHunkHeader(entries)
	// lineRange counts entries where the relevant line number is non-zero.
	// old: entry[0] oldLine=5 ✓, entry[1] oldLine=0 ✗, entry[2] oldLine=6 ✓  → count=2
	// new: entry[0] newLine=0 ✗, entry[1] newLine=5 ✓, entry[2] newLine=6 ✓ → count=2
	if header != "@@ -5,2 +5,2 @@" {
		t.Errorf("unexpected header: %s", header)
	}
}

func TestLineRangeOld(t *testing.T) {
	entries := []diffEntry{
		{kind: diffDelete, oldLine: 0}, // skip
		{kind: diffDelete, oldLine: 10},
		{kind: diffDelete, oldLine: 11},
	}
	start, count := lineRange(entries, true)
	if start != 10 || count != 2 {
		t.Errorf("lineRange old: start=%d count=%d, want 10 2", start, count)
	}
}

func TestLineLabelZero(t *testing.T) {
	if got := lineLabel(0); got != "." {
		t.Errorf("lineLabel(0): got %q, want \".\"", got)
	}
	if got := lineLabel(42); got != "42" {
		t.Errorf("lineLabel(42): got %q, want \"42\"", got)
	}
}

// ─── buildDiffSegments ───────────────────────────────────────────────────────

func TestBuildDiffSegmentsBothEmpty(t *testing.T) {
	segs := buildDiffSegments(nil, nil)
	if len(segs) != 0 {
		t.Errorf("expected 0 segments, got %d", len(segs))
	}
}

func TestBuildDiffSegmentsLargeMatrix(t *testing.T) {
	// Force the large-matrix shortcut path
	bigBefore := make([]string, 500)
	bigAfter := make([]string, 500)
	segs := buildDiffSegments(bigBefore, bigAfter)
	if len(segs) != 2 {
		t.Errorf("expected 2 segments (delete+insert), got %d", len(segs))
	}
	if segs[0].kind != diffDelete || segs[1].kind != diffInsert {
		t.Errorf("unexpected segment kinds: %v", segs)
	}
}

// ─── buildWriteConfirmationSummary ───────────────────────────────────────────

func TestBuildWriteConfirmationSummaryCreated(t *testing.T) {
	summary := buildWriteConfirmationSummary("created", "", "one\ntwo\n")
	if !strings.Contains(summary, "action: created") {
		t.Errorf("missing action: %s", summary)
	}
	if !strings.Contains(summary, "preview:") {
		t.Errorf("missing preview: %s", summary)
	}
	if strings.Contains(summary, "diff:") {
		t.Errorf("should not contain diff for created: %s", summary)
	}
}

func TestBuildWriteConfirmationSummaryUpdated(t *testing.T) {
	summary := buildWriteConfirmationSummary("updated", "old\n", "new\n")
	if !strings.Contains(summary, "action: updated") {
		t.Errorf("missing action: %s", summary)
	}
	if !strings.Contains(summary, "diff:") {
		t.Errorf("missing diff: %s", summary)
	}
}

// ─── buildWriteResultSummary ─────────────────────────────────────────────────

func TestBuildWriteResultSummaryCreated(t *testing.T) {
	summary := buildWriteResultSummary("/repo/foo.txt", "created", "", "content")
	if !strings.Contains(summary, "Created /repo/foo.txt") {
		t.Errorf("missing created notice: %s", summary)
	}
}

func TestBuildWriteResultSummaryUpdated(t *testing.T) {
	summary := buildWriteResultSummary("/repo/foo.txt", "updated", "old", "new")
	if !strings.Contains(summary, "Updated /repo/foo.txt") {
		t.Errorf("missing updated notice: %s", summary)
	}
}

// ─── renderDiffPreview limits ────────────────────────────────────────────────

func TestRenderDiffPreviewEmptyInput(t *testing.T) {
	got := renderDiffPreview("", "")
	if got != "(empty file)" {
		t.Errorf("expected (empty file), got %q", got)
	}
}

func TestRenderDiffPreviewManyHunks(t *testing.T) {
	// Build 4 isolated changes spaced 5 lines apart (context=2 each side, so windows
	// don't overlap). Each change is 1 del/ins pair + 2 context lines = ~4 output lines.
	// 4 hunks × ~4 lines = ~16 lines, under the 24-line limit, so hunk limit (3) triggers first.
	var before, after []string
	for i := 0; i < 4; i++ {
		before = append(before, "a", "b", "equal", "del", "a", "b")
		after = append(after, "a", "b", "equal", "ins", "a", "b")
	}
	got := renderDiffPreview(strings.Join(before, "\n"), strings.Join(after, "\n"))
	if !strings.Contains(got, "more hunk(s)") {
		t.Errorf("should mention more hunks: %s", got)
	}
}
