package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gkampitakis/go-snaps/snaps"
)

func TestMain(m *testing.M) {
	v := m.Run()
	_, _ = snaps.Clean(m)
	os.Exit(v)
}

// runFixture executes relic against testdata/fixture.jsonl and returns the HTML.
func runFixture(t *testing.T, opts options) string {
	t.Helper()
	tmp := t.TempDir()
	if opts.outputPath == "" {
		opts.outputPath = filepath.Join(tmp, "out.html")
	}
	if opts.inputPath == "" {
		opts.inputPath = "testdata/fixture.jsonl"
	}
	var logBuf bytes.Buffer
	if err := execute(opts, &logBuf); err != nil {
		t.Fatalf("execute: %v", err)
	}
	html, err := os.ReadFile(opts.outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	return string(html)
}

// TestExecuteE2E is the tracer bullet: fixture JSONL in, snapshot the HTML out.
func TestExecuteE2E(t *testing.T) {
	html := runFixture(t, options{})
	snaps.MatchSnapshot(t, html)
}

// TestTurnGrouping verifies that user turns are wrapped in sections with id="turn-N".
func TestTurnGrouping(t *testing.T) {
	html := runFixture(t, options{})

	// Fixture has 3 user messages → 3 turns.
	if !strings.Contains(html, `id="turn-1"`) {
		t.Error(`expected id="turn-1" in output`)
	}
	if !strings.Contains(html, `id="turn-2"`) {
		t.Error(`expected id="turn-2" in output`)
	}
	if !strings.Contains(html, `id="turn-3"`) {
		t.Error(`expected id="turn-3" in output`)
	}
	if strings.Contains(html, `id="turn-4"`) {
		t.Error(`unexpected id="turn-4" in output (only 3 user turns in fixture)`)
	}
}

// TestTOC verifies a table of contents is rendered with one entry per user turn.
func TestTOC(t *testing.T) {
	html := runFixture(t, options{})

	// TOC must contain fragment links to each turn.
	if !strings.Contains(html, `href="#turn-1"`) {
		t.Error(`expected href="#turn-1" in TOC`)
	}
	if !strings.Contains(html, `href="#turn-2"`) {
		t.Error(`expected href="#turn-2" in TOC`)
	}
	if !strings.Contains(html, `href="#turn-3"`) {
		t.Error(`expected href="#turn-3" in TOC`)
	}

	// TOC entries must include the first line of the user message (truncated).
	if !strings.Contains(html, "Hello, can you help me write a Go function?") {
		t.Error("expected first user message text in TOC entry")
	}
}

// TestPrevNext verifies prev/next navigation links on turn sections.
func TestPrevNext(t *testing.T) {
	html := runFixture(t, options{})

	// First turn: no prev, has next.
	if strings.Contains(html, `href="#turn-0"`) {
		t.Error(`unexpected href="#turn-0": first turn must have no prev`)
	}
	if !strings.Contains(html, `href="#turn-2"`) {
		t.Error(`expected href="#turn-2" next link on turn 1`)
	}

	// Middle turn (turn 2): has both prev and next.
	if !strings.Contains(html, `href="#turn-1"`) {
		t.Error(`expected href="#turn-1" prev link on turn 2`)
	}
	if !strings.Contains(html, `href="#turn-3"`) {
		t.Error(`expected href="#turn-3" next link on turn 2`)
	}

	// Last turn: has prev, no next.
	if strings.Contains(html, `href="#turn-4"`) {
		t.Error(`unexpected href="#turn-4": last turn must have no next`)
	}
}

// TestJKKeyboardNav verifies that J/K keyboard navigation anchors are present.
// J links (next turn) and K links (prev turn) are hidden by default and shown
// via CSS :target when the turn section is targeted. No JavaScript is involved.
// TestToolUseAndThinkingE2E verifies that tool_use and thinking blocks render
// as collapsible <details> sections in the HTML output.
func TestToolUseAndThinkingE2E(t *testing.T) {
	tmp := t.TempDir()
	outPath := filepath.Join(tmp, "out.html")
	var logBuf bytes.Buffer
	err := execute(options{
		inputPath:  "testdata/fixture_tool_use.jsonl",
		outputPath: outPath,
	}, &logBuf)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	html, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	htmlStr := string(html)

	// Count <details> elements: fixture has 1 thinking + 2 tool_use = 3.
	detailsCount := strings.Count(htmlStr, "<details")
	if detailsCount != 3 {
		t.Errorf("expected 3 <details> elements (1 thinking + 2 tool_use), got %d", detailsCount)
	}

	// thinking block must be labelled "Internal reasoning".
	if !strings.Contains(htmlStr, "Internal reasoning") {
		t.Error("expected 'Internal reasoning' label in thinking block")
	}

	// tool_use block for Bash must show the $ icon and command.
	if !strings.Contains(htmlStr, "echo hello world") {
		t.Error("expected Bash command 'echo hello world' in tool_use summary")
	}

	snaps.MatchSnapshot(t, htmlStr)
}

// TestMalformedLineE2E verifies that a session with a malformed line renders an
// error callout in the HTML and exits 0. The slog warning goes to errOut.
func TestMalformedLineE2E(t *testing.T) {
	tmp := t.TempDir()
	outPath := filepath.Join(tmp, "out.html")
	var logBuf bytes.Buffer
	err := execute(options{
		inputPath:  "testdata/malformed.jsonl",
		outputPath: outPath,
	}, &logBuf)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	html, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	htmlStr := string(html)

	// Error callout must appear in HTML.
	if !strings.Contains(htmlStr, "error-callout") {
		t.Error("expected error-callout element in output HTML")
	}
	if !strings.Contains(htmlStr, "Parse error on line 2") {
		t.Error("expected 'Parse error on line 2' in error callout")
	}

	// Warning logged to stderr.
	if !strings.Contains(logBuf.String(), "malformed") && !strings.Contains(logBuf.String(), "line") {
		t.Errorf("expected warning in log output, got: %q", logBuf.String())
	}

	snaps.MatchSnapshot(t, htmlStr)
}

// TestMarkdownRendering verifies that text blocks are rendered through goldmark.
func TestMarkdownRendering(t *testing.T) {
	html := runFixture(t, options{})

	// The fixture contains code fences in text blocks; goldmark should render them as <code>.
	if !strings.Contains(html, "<code") {
		t.Error("expected <code> elements from Markdown code fence rendering")
	}
	// A <pre> block should wrap the code fence.
	if !strings.Contains(html, "<pre") {
		t.Error("expected <pre> elements from Markdown code fence rendering")
	}
	// The raw backtick fence markers should NOT appear in rendered output.
	if strings.Contains(html, "```go") {
		t.Error("expected raw Markdown code fence to be absent in rendered output")
	}
}

// TestCopyToClipboardScript verifies exactly one <script> tag is present
// (the copy-to-clipboard snippet added in Phase 6).
func TestCopyToClipboardScript(t *testing.T) {
	html := runFixture(t, options{})
	count := strings.Count(html, "<script")
	if count != 1 {
		t.Errorf("expected exactly 1 <script> tag, got %d", count)
	}
}

func TestJKKeyboardNav(t *testing.T) {
	html := runFixture(t, options{})

	// accesskey="j" (next) must be present — turns 1 and 2 have a next turn.
	if !strings.Contains(html, `accesskey="j"`) {
		t.Error(`expected accesskey="j" for next-turn navigation`)
	}
	// accesskey="k" (prev) must be present — turns 2 and 3 have a prev turn.
	if !strings.Contains(html, `accesskey="k"`) {
		t.Error(`expected accesskey="k" for prev-turn navigation`)
	}
	// No <script> tags from J/K nav itself (copy-to-clipboard added in Phase 6 is fine).
	// Verified independently by TestCopyToClipboardScript.
	// CSS must include :target rule for .jk-nav visibility.
	if !strings.Contains(html, ":target") {
		t.Error("expected CSS :target rule for J/K navigation")
	}
}

func TestSelfContained(t *testing.T) {
	html := runFixture(t, options{})
	for _, prefix := range []string{"http://", "https://"} {
		if strings.Contains(html, prefix) {
			t.Errorf("output contains external URL with prefix %q", prefix)
		}
	}
}

// TestTimestamps verifies that the first message shows an absolute timestamp
// and subsequent messages show relative time with an absolute title attribute.
func TestTimestamps(t *testing.T) {
	html := runFixture(t, options{})

	// First message: absolute timestamp visible (no "later" text).
	if !strings.Contains(html, "2025") {
		t.Error("expected year 2025 in first message absolute timestamp")
	}
	// Subsequent messages: relative time with title attribute.
	if !strings.Contains(html, "title=\"") {
		t.Error("expected title= attribute for relative timestamp")
	}
}
