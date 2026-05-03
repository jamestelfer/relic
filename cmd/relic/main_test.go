package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.NoError(t, execute(opts, &logBuf))
	html, err := os.ReadFile(opts.outputPath)
	require.NoError(t, err)
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
	assert.Contains(t, html, `id="turn-1"`)
	assert.Contains(t, html, `id="turn-2"`)
	assert.Contains(t, html, `id="turn-3"`)
	assert.NotContains(t, html, `id="turn-4"`, "only 3 user turns in fixture")
}

// TestTOC verifies a table of contents is rendered with one entry per user turn.
func TestTOC(t *testing.T) {
	html := runFixture(t, options{})
	assert.Contains(t, html, `href="#turn-1"`)
	assert.Contains(t, html, `href="#turn-2"`)
	assert.Contains(t, html, `href="#turn-3"`)
	assert.Contains(t, html, "Hello, can you help me write a Go function?")
}

// TestPrevNext verifies prev/next navigation links on turn sections.
func TestPrevNext(t *testing.T) {
	html := runFixture(t, options{})
	assert.NotContains(t, html, `href="#turn-0"`, "first turn must have no prev")
	assert.Contains(t, html, `href="#turn-2"`, "turn 1 must have next")
	assert.Contains(t, html, `href="#turn-1"`, "turn 2 must have prev")
	assert.Contains(t, html, `href="#turn-3"`, "turn 2 must have next")
	assert.NotContains(t, html, `href="#turn-4"`, "last turn must have no next")
}

// TestToolResultE2E verifies tool_result blocks render with ANSI converted and no raw escapes.
func TestToolResultE2E(t *testing.T) {
	tmp := t.TempDir()
	outPath := filepath.Join(tmp, "out.html")
	var logBuf bytes.Buffer
	require.NoError(t, execute(options{
		inputPath:  "testdata/fixture_tool_result.jsonl",
		outputPath: outPath,
	}, &logBuf))

	html, err := os.ReadFile(outPath)
	require.NoError(t, err)
	htmlStr := string(html)

	assert.Contains(t, htmlStr, "tool-result")
	assert.NotContains(t, htmlStr, "\x1b", "ANSI escape byte must be absent")
	assert.Contains(t, htmlStr, "<span", "ANSI colour should produce <span>")
	snaps.MatchSnapshot(t, htmlStr)
}

// TestToolUseAndThinkingE2E verifies that tool_use and thinking blocks render
// as collapsible <details> sections in the HTML output.
func TestToolUseAndThinkingE2E(t *testing.T) {
	tmp := t.TempDir()
	outPath := filepath.Join(tmp, "out.html")
	var logBuf bytes.Buffer
	require.NoError(t, execute(options{
		inputPath:  "testdata/fixture_tool_use.jsonl",
		outputPath: outPath,
	}, &logBuf))

	html, err := os.ReadFile(outPath)
	require.NoError(t, err)
	htmlStr := string(html)

	// Fixture has 1 thinking + 2 tool_use = 3 <details>.
	assert.Equal(t, 3, strings.Count(htmlStr, "<details"), "expected 3 <details> elements")
	assert.Contains(t, htmlStr, "Internal reasoning")
	assert.Contains(t, htmlStr, "echo hello world")
	snaps.MatchSnapshot(t, htmlStr)
}

// TestMalformedLineE2E verifies that a session with a malformed line renders an
// error callout in the HTML and exits 0. The slog warning goes to errOut.
func TestMalformedLineE2E(t *testing.T) {
	tmp := t.TempDir()
	outPath := filepath.Join(tmp, "out.html")
	var logBuf bytes.Buffer
	require.NoError(t, execute(options{
		inputPath:  "testdata/malformed.jsonl",
		outputPath: outPath,
	}, &logBuf))

	html, err := os.ReadFile(outPath)
	require.NoError(t, err)
	htmlStr := string(html)

	assert.Contains(t, htmlStr, "error-callout")
	assert.Contains(t, htmlStr, "Parse error on line 2")
	assert.True(t,
		strings.Contains(logBuf.String(), "malformed") || strings.Contains(logBuf.String(), "line"),
		"expected warning in log output, got: %q", logBuf.String(),
	)
	snaps.MatchSnapshot(t, htmlStr)
}

// TestMarkdownRendering verifies that text blocks are rendered through goldmark.
func TestMarkdownRendering(t *testing.T) {
	html := runFixture(t, options{})
	assert.Contains(t, html, "<code", "expected <code> from Markdown code fence")
	assert.Contains(t, html, "<pre", "expected <pre> from Markdown code fence")
	assert.NotContains(t, html, "```go", "raw Markdown fence markers must not appear in output")
}

// TestCopyToClipboardScript verifies exactly one <script> tag is present.
func TestCopyToClipboardScript(t *testing.T) {
	html := runFixture(t, options{})
	assert.Equal(t, 1, strings.Count(html, "<script"), "expected exactly 1 <script> tag")
}

func TestJKKeyboardNav(t *testing.T) {
	html := runFixture(t, options{})
	assert.Contains(t, html, `accesskey="j"`)
	assert.Contains(t, html, `accesskey="k"`)
	assert.Contains(t, html, ":target", "expected CSS :target rule for J/K navigation")
}

func TestSelfContained(t *testing.T) {
	html := runFixture(t, options{})
	assert.NotContains(t, html, "http://")
	assert.NotContains(t, html, "https://")
}

// TestTimestamps verifies first message shows absolute timestamp; others show relative.
func TestTimestamps(t *testing.T) {
	html := runFixture(t, options{})
	assert.Contains(t, html, "2025", "expected year 2025 in first message absolute timestamp")
	assert.Contains(t, html, `title="`, "expected title= attribute for relative timestamp")
}

// TestSyntaxHighlightingCSS verifies Chroma CSS for light and dark themes is injected.
func TestSyntaxHighlightingCSS(t *testing.T) {
	html := runFixture(t, options{})
	assert.Contains(t, html, "prefers-color-scheme")
	assert.NotContains(t, html, `style="color:`, "must use class-based Chroma CSS, not inline styles")
}

// TestOutputFlag_Stdout verifies that outputPath="-" writes HTML to htmlOut.
func TestOutputFlag_Stdout(t *testing.T) {
	var htmlBuf bytes.Buffer
	var logBuf bytes.Buffer
	require.NoError(t, execute(options{
		inputPath:  "testdata/fixture.jsonl",
		outputPath: "-",
		htmlOut:    &htmlBuf,
	}, &logBuf))
	assert.Contains(t, htmlBuf.String(), "<!doctype html")
}

// TestNameFlag verifies that the name option overrides the session banner name.
func TestNameFlag(t *testing.T) {
	html := runFixture(t, options{name: "My Custom Session"})
	assert.Contains(t, html, "My Custom Session")
	snaps.MatchSnapshot(t, html)
}

// TestMissingFile_ExitCode verifies that a missing input file returns an error.
func TestMissingFile_ExitCode(t *testing.T) {
	var logBuf bytes.Buffer
	err := execute(options{
		inputPath:  "testdata/nonexistent.jsonl",
		outputPath: t.TempDir() + "/out.html",
	}, &logBuf)
	require.Error(t, err)
}

// TestCLI_Help verifies that the CLI --help output lists all three flags.
func TestCLI_Help(t *testing.T) {
	var outBuf bytes.Buffer
	cmd := buildCLI(func(opts options, errOut io.Writer) error {
		return execute(opts, errOut)
	})
	cmd.Writer = &outBuf
	_ = cmd.Run(context.Background(), []string{"relic", "--help"})
	help := outBuf.String()
	assert.Contains(t, help, "--output")
	assert.Contains(t, help, "--theme")
	assert.Contains(t, help, "--name")
}

// TestCLI_UnknownTheme verifies that an unknown --theme value returns a user error.
func TestCLI_UnknownTheme(t *testing.T) {
	var logBuf bytes.Buffer
	err := execute(options{
		inputPath:  "testdata/fixture.jsonl",
		outputPath: t.TempDir() + "/out.html",
		theme:      "totally_unknown_theme_xyz",
	}, &logBuf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown theme")
}
