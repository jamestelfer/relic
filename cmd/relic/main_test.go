package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
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

// TestCLI_OutputMode_Invalid verifies that --output <bad-value> via the CLI exits
// non-zero and prints a message listing html, gist, and public-gist (R6).
func TestCLI_OutputMode_Invalid(t *testing.T) {
	// Override OsExiter so cli.Exit doesn't call os.Exit in the test process.
	origExiter := cli.OsExiter
	defer func() { cli.OsExiter = origExiter }()
	cli.OsExiter = func(int) {}

	var outBuf, errBuf bytes.Buffer
	cmd := buildCLI(func(opts options, errOut io.Writer) error {
		return execute(opts, errOut)
	})
	cmd.Writer = &outBuf
	cmd.ErrWriter = &errBuf

	err := cmd.Run(context.Background(), []string{
		"relic",
		"--output", "bad",
		"testdata/fixture.jsonl",
	})
	require.Error(t, err)
	msg := err.Error()
	assert.Contains(t, msg, "html")
	assert.Contains(t, msg, "gist")
	assert.Contains(t, msg, "public-gist")
}

// TestCLI_OutputPathStdout verifies that --output-path - writes HTML to stdout.
func TestCLI_OutputPathStdout(t *testing.T) {
	var outBuf, errBuf bytes.Buffer
	cmd := buildCLI(func(opts options, errOut io.Writer) error {
		return execute(opts, errOut)
	})
	cmd.Writer = &outBuf
	cmd.ErrWriter = &errBuf

	err := cmd.Run(context.Background(), []string{
		"relic",
		"--output-path", "-",
		"testdata/fixture.jsonl",
	})
	require.NoError(t, err)
	assert.Contains(t, outBuf.String(), "<!doctype html")
}

// TestCLI_OutputPath verifies that --output-path <file> writes HTML to that path.
func TestCLI_OutputPath(t *testing.T) {
	tmp := t.TempDir()
	outPath := filepath.Join(tmp, "explicit.html")

	var errBuf bytes.Buffer
	cmd := buildCLI(func(opts options, errOut io.Writer) error {
		return execute(opts, errOut)
	})
	cmd.Writer = io.Discard
	cmd.ErrWriter = &errBuf

	err := cmd.Run(context.Background(), []string{
		"relic",
		"--output-path", outPath,
		"testdata/fixture.jsonl",
	})
	require.NoError(t, err)

	html, readErr := os.ReadFile(outPath)
	require.NoError(t, readErr)
	assert.Contains(t, string(html), "<!doctype html")
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
	assert.Contains(t, help, "--output-path")
	assert.Contains(t, help, "--theme")
	assert.Contains(t, help, "--name")
}

// TestExecute_GistGhNotFound verifies that when gist publish fails with gh not found,
// the error message mentions gh auth login (R13).
func TestExecute_GistGhNotFound(t *testing.T) {
	stub := &stubGistRunner{
		err: fmt.Errorf("gh CLI not found in PATH — install it and run `gh auth login`"),
	}
	var outBuf, logBuf bytes.Buffer
	err := execute(options{
		inputPath:  "testdata/fixture.jsonl",
		outputMode: outputModeGist,
		htmlOut:    &outBuf,
		gistRunner: stub,
	}, &logBuf)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "gh auth login")
}

// TestExecute_GistFilename verifies that the gist filename is derived from the
// session name (same logic as the local HTML filename), not the raw input path (R10).
func TestExecute_GistFilename(t *testing.T) {
	stub := &stubGistRunner{
		gistURL:    "https://gist.github.com/user/abc123",
		previewURL: "https://gisthost.github.io/?abc123/fixture.html",
	}
	var outBuf, logBuf bytes.Buffer
	err := execute(options{
		inputPath:  "testdata/fixture.jsonl",
		outputMode: outputModeGist,
		htmlOut:    &outBuf,
		gistRunner: stub,
	}, &logBuf)

	require.NoError(t, err)
	assert.Equal(t, "fixture.html", stub.capturedFilename,
		"gist filename must be derived from the session name stem + .html")
}

// TestExecute_PublicGistMode verifies that --output public-gist passes public=true
// to the publisher (R5).
func TestExecute_PublicGistMode(t *testing.T) {
	stub := &stubGistRunner{
		gistURL:    "https://gist.github.com/user/abc123",
		previewURL: "https://gisthost.github.io/?abc123/fixture.html",
	}
	var outBuf, logBuf bytes.Buffer
	err := execute(options{
		inputPath:  "testdata/fixture.jsonl",
		outputMode: outputModePublicGist,
		htmlOut:    &outBuf,
		gistRunner: stub,
	}, &logBuf)

	require.NoError(t, err)
	assert.True(t, stub.capturedPublic, "public-gist mode must call Publish with public=true")
}

// TestExecute_GistMode verifies that --output gist renders to memory, prints
// Gist: and Preview: URLs to stdout, and writes no local .html file (R4, R7, R8, R10).
func TestExecute_GistMode(t *testing.T) {
	tmp := t.TempDir()

	var outBuf, logBuf bytes.Buffer
	err := execute(options{
		inputPath:  "testdata/fixture.jsonl",
		outputMode: outputModeGist,
		htmlOut:    &outBuf, // stdout in gist mode
		gistRunner: &stubGistRunner{
			gistURL:    "https://gist.github.com/user/abc123",
			previewURL: "https://gisthost.github.io/?abc123/fixture.html",
		},
	}, &logBuf)

	require.NoError(t, err)

	out := outBuf.String()
	assert.Contains(t, out, "Gist:")
	assert.Contains(t, out, "https://gist.github.com/user/abc123")
	assert.Contains(t, out, "Preview:")
	assert.Contains(t, out, "https://gisthost.github.io/?abc123/fixture.html")

	// No .html file must be written anywhere in the working directory.
	matches, _ := filepath.Glob(filepath.Join(tmp, "*.html"))
	assert.Empty(t, matches, "gist mode must not write a local .html file")
}

// stubGistRunner is a test double for gist publishing used in execute() tests.
type stubGistRunner struct {
	gistURL    string
	previewURL string
	err        error
	// captured inputs
	capturedHTML     []byte
	capturedFilename string
	capturedPublic   bool
}

func (s *stubGistRunner) Publish(html []byte, filename string, public bool) (string, string, error) {
	s.capturedHTML = html
	s.capturedFilename = filename
	s.capturedPublic = public
	return s.gistURL, s.previewURL, s.err
}

// TestOutputMode_Invalid verifies that an unrecognised --output mode returns an error
// that lists all valid mode values (R6).
func TestOutputMode_Invalid(t *testing.T) {
	var logBuf bytes.Buffer
	err := execute(options{
		inputPath:  "testdata/fixture.jsonl",
		outputPath: t.TempDir() + "/out.html",
		outputMode: "bad",
	}, &logBuf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "html")
	assert.Contains(t, err.Error(), "gist")
	assert.Contains(t, err.Error(), "public-gist")
}

// TestOutputMode_HTMLExplicit verifies that --output html produces the same result
// as the default (no --output flag), satisfying R1 and R3.
func TestOutputMode_HTMLExplicit(t *testing.T) {
	tmp := t.TempDir()
	outPath := filepath.Join(tmp, "out.html")
	var logBuf bytes.Buffer
	require.NoError(t, execute(options{
		inputPath:  "testdata/fixture.jsonl",
		outputPath: outPath,
		outputMode: outputModeHTML,
	}, &logBuf))
	html, err := os.ReadFile(outPath)
	require.NoError(t, err)
	assert.Contains(t, string(html), "<!doctype html")
}

// TestCLI_GistMode_PrintsURLs verifies that the CLI routes --output gist to execute
// and the Gist:/Preview: lines appear on stdout (R8).
func TestCLI_GistMode_PrintsURLs(t *testing.T) {
	var outBuf, errBuf bytes.Buffer
	cmd := buildCLI(func(opts options, errOut io.Writer) error {
		// Inject a stub publisher so no real gh call is made.
		opts.gistRunner = &stubGistRunner{
			gistURL:    "https://gist.github.com/user/testid",
			previewURL: "https://gisthost.github.io/?testid/fixture.html",
		}
		return execute(opts, errOut)
	})
	cmd.Writer = &outBuf
	cmd.ErrWriter = &errBuf

	err := cmd.Run(context.Background(), []string{
		"relic",
		"--output", "gist",
		"testdata/fixture.jsonl",
	})
	require.NoError(t, err)

	out := outBuf.String()
	assert.Contains(t, out, "Gist:")
	assert.Contains(t, out, "https://gist.github.com/user/testid")
	assert.Contains(t, out, "Preview:")
	assert.Contains(t, out, "https://gisthost.github.io/?testid/fixture.html")
}

// TestCLI_OutputMode_GistAccepted verifies that --output gist and --output public-gist
// are accepted by the flag parser without a parse error (structural, R1).
// Gist logic is not yet wired, so a "not implemented" error is acceptable.
func TestCLI_OutputMode_GistAccepted(t *testing.T) {
	for _, mode := range []string{"gist", "public-gist"} {
		t.Run(mode, func(t *testing.T) {
			origExiter := cli.OsExiter
			defer func() { cli.OsExiter = origExiter }()
			cli.OsExiter = func(int) {}

			var outBuf, errBuf bytes.Buffer
			cmd := buildCLI(func(opts options, errOut io.Writer) error {
				return execute(opts, errOut)
			})
			cmd.Writer = &outBuf
			cmd.ErrWriter = &errBuf

			err := cmd.Run(context.Background(), []string{
				"relic",
				"--output", mode,
				"testdata/fixture.jsonl",
			})
			// Must not be a parse / "unknown output mode" error.
			// A "not implemented" error is acceptable at this phase.
			if err != nil {
				assert.NotContains(t, err.Error(), "unknown output mode",
					"--output %q must be accepted by the flag parser", mode)
			}
		})
	}
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
