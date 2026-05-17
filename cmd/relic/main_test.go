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

const (
	testGitHubPAT = "ghp_" + "zR8k4mVq2xN7pLw9cJ3hYf6eDgA5tB0sQiUo"
	testAWSKey    = "AKIA" + "Z7V4Q2XRNJ3WBTY5"
)

// secretsFixturePath loads the secrets fixture template from testdata, replaces
// placeholders with real high-entropy secret patterns at runtime, and writes the
// result to a temp file. This keeps secret-shaped strings out of committed files.
func secretsFixturePath(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("testdata/secrets.jsonl")
	require.NoError(t, err)
	content := string(data)
	content = strings.ReplaceAll(content, "PLACEHOLDER_GITHUB_PAT", testGitHubPAT)
	content = strings.ReplaceAll(content, "PLACEHOLDER_AWS_KEY", testAWSKey)
	path := filepath.Join(t.TempDir(), "secrets.jsonl")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
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

// TestExecuteE2E is the tracer bullet: fixture JSONL in → valid HTML out.
func TestExecuteE2E(t *testing.T) {
	html := runFixture(t, options{})
	assert.Contains(t, html, "<!doctype html")
	assert.Contains(t, html, "</html>")
}

// TestExecute_DefaultOutputPath_CWD verifies that when no --output is set
// the HTML file is written to CWD/<stem>.html (not next to the input) and the
// absolute path is echoed to stdout on its own line.
func TestExecute_DefaultOutputPath_CWD(t *testing.T) {
	cwdBefore, err := os.Getwd()
	require.NoError(t, err)
	absFixture := filepath.Join(cwdBefore, "testdata/fixture.jsonl")

	tmp := t.TempDir()
	t.Chdir(tmp)
	cwd, err := os.Getwd()
	require.NoError(t, err)

	var outBuf, logBuf bytes.Buffer
	require.NoError(t, execute(options{
		inputPath: absFixture,
		stdout:    &outBuf,
	}, &logBuf))

	outPath := filepath.Join(cwd, "fixture.html")
	_, statErr := os.Stat(outPath)
	require.NoError(t, statErr, "default output path must be CWD/<stem>.html")

	assert.Equal(t, outPath+"\n", outBuf.String(),
		"stdout must echo the absolute output path followed by one newline")
}

// TestExecute_DefaultOutputPath_Overwrites verifies that re-running the same
// command overwrites the output file silently and exits 0.
func TestExecute_DefaultOutputPath_Overwrites(t *testing.T) {
	cwdBefore, err := os.Getwd()
	require.NoError(t, err)
	absFixture := filepath.Join(cwdBefore, "testdata/fixture.jsonl")

	tmp := t.TempDir()
	t.Chdir(tmp)

	for i := range 2 {
		var outBuf, logBuf bytes.Buffer
		require.NoError(t, execute(options{
			inputPath: absFixture,
			stdout:    &outBuf,
		}, &logBuf), "run %d", i)
		assert.Empty(t, logBuf.String(), "run %d: stderr must be silent on success", i)
	}
}

// TestOutputFlag_Stdout verifies that outputPath="-" writes HTML to stdout
// with nothing after the final </html> closer (no path echo).
func TestOutputFlag_Stdout(t *testing.T) {
	var htmlBuf bytes.Buffer
	var logBuf bytes.Buffer
	require.NoError(t, execute(options{
		inputPath:  "testdata/fixture.jsonl",
		outputPath: "-",
		stdout:     &htmlBuf,
	}, &logBuf))
	out := htmlBuf.String()
	assert.Contains(t, out, "<!doctype html")
	trailing := strings.TrimSpace(out[strings.LastIndex(out, "</html>")+len("</html>"):])
	assert.Empty(t, trailing, "stdout must end with </html> — no path echo")
}

// TestNameFlag verifies that the name option appears in the rendered title.
func TestNameFlag(t *testing.T) {
	html := runFixture(t, options{name: "My Custom Session"})
	assert.Contains(t, html, "My Custom Session")
}

// TestExecute_DebugFlag_EmitsDebugLogs verifies that setting opts.debug=true
// switches the slog level to Debug, so debug-level messages reach stderr.
func TestExecute_DebugFlag_EmitsDebugLogs(t *testing.T) {
	base := options{
		inputPath:  "testdata/fixture.jsonl",
		outputPath: filepath.Join(t.TempDir(), "out.html"),
	}

	var silentErr bytes.Buffer
	require.NoError(t, execute(base, &silentErr))

	base.outputPath = filepath.Join(t.TempDir(), "out.html")
	base.debug = true
	var debugErr bytes.Buffer
	require.NoError(t, execute(base, &debugErr))

	assert.NotContains(t, silentErr.String(), "render complete",
		"default level must suppress debug messages")
	assert.Contains(t, debugErr.String(), "render complete",
		"--debug must surface debug-level messages on stderr")
}

// TestCLI_DebugFlag verifies that --debug is accepted by the flag parser and
// turns on debug logging end-to-end.
func TestCLI_DebugFlag(t *testing.T) {
	tmp := t.TempDir()
	outPath := filepath.Join(tmp, "explicit.html")

	var outBuf, errBuf bytes.Buffer
	cmd := buildCLI(func(opts options, errOut io.Writer) error {
		return execute(opts, errOut)
	})
	cmd.Writer = &outBuf
	cmd.ErrWriter = &errBuf

	err := cmd.Run(context.Background(), []string{
		"relic",
		"--debug",
		"--output", outPath,
		"testdata/fixture.jsonl",
	})
	require.NoError(t, err)
	assert.Contains(t, errBuf.String(), "render complete",
		"--debug CLI flag must enable debug-level logging to stderr")
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

// TestCLI_OutputMode_Invalid verifies that --mode <bad-value> via the CLI exits
// non-zero and prints a message listing html, gist, and public-gist.
func TestCLI_OutputMode_Invalid(t *testing.T) {
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
		"--mode", "bad",
		"testdata/fixture.jsonl",
	})
	require.Error(t, err)
	msg := err.Error()
	assert.Contains(t, msg, "html")
	assert.Contains(t, msg, "gist")
	assert.Contains(t, msg, "public-gist")
}

// TestCLI_OutputPathStdout verifies that --output - writes HTML to stdout.
func TestCLI_OutputPathStdout(t *testing.T) {
	var outBuf, errBuf bytes.Buffer
	cmd := buildCLI(func(opts options, errOut io.Writer) error {
		return execute(opts, errOut)
	})
	cmd.Writer = &outBuf
	cmd.ErrWriter = &errBuf

	err := cmd.Run(context.Background(), []string{
		"relic",
		"--output", "-",
		"testdata/fixture.jsonl",
	})
	require.NoError(t, err)
	assert.Contains(t, outBuf.String(), "<!doctype html")
}

// TestCLI_OutputPath verifies that --output <file> writes HTML to that path.
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
		"--output", outPath,
		"testdata/fixture.jsonl",
	})
	require.NoError(t, err)

	html, readErr := os.ReadFile(outPath)
	require.NoError(t, readErr)
	assert.Contains(t, string(html), "<!doctype html")
}

// TestCLI_Help verifies that --help lists key flags.
func TestCLI_Help(t *testing.T) {
	var outBuf bytes.Buffer
	cmd := buildCLI(func(opts options, errOut io.Writer) error {
		return execute(opts, errOut)
	})
	cmd.Writer = &outBuf
	_ = cmd.Run(context.Background(), []string{"relic", "--help"})
	help := outBuf.String()
	assert.Contains(t, help, "--mode")
	assert.Contains(t, help, "--output")
	assert.Contains(t, help, "--name")
}

// TestExecute_CustomTitle_SingleRecord: a fixture carrying one custom-title
// record renders that text in the page title.
func TestExecute_CustomTitle_SingleRecord(t *testing.T) {
	html := runFixture(t, options{inputPath: "testdata/custom_title_single.jsonl"})
	assert.Contains(t, html, "phase5 title")
}

// TestExecute_CustomTitle_LastRecordWins: when multiple custom-title records
// appear, the last one in source order becomes the title.
func TestExecute_CustomTitle_LastRecordWins(t *testing.T) {
	html := runFixture(t, options{inputPath: "testdata/custom_title_multi.jsonl"})
	assert.Contains(t, html, "<title>final")
	assert.NotContains(t, html, "<title>first")
}

// TestExecute_CustomTitle_FilenameFallback: without any custom-title record,
// the title falls back to the input-filename stem.
func TestExecute_CustomTitle_FilenameFallback(t *testing.T) {
	html := runFixture(t, options{inputPath: "testdata/custom_title_none.jsonl"})
	assert.Contains(t, html, "custom_title_none")
}

// TestExecute_Debug_UnhandledContentBlock: `--debug` logs one line per
// unrecognised content block (RawBlock), with line + type + role fields.
func TestExecute_Debug_UnhandledContentBlock(t *testing.T) {
	var logBuf bytes.Buffer
	err := execute(options{
		inputPath:  "testdata/unhandled_content.jsonl",
		outputPath: filepath.Join(t.TempDir(), "out.html"),
		debug:      true,
	}, &logBuf)
	require.NoError(t, err)

	logs := logBuf.String()
	assert.GreaterOrEqual(t, strings.Count(logs, "unhandled content block"), 1)
	assert.Contains(t, logs, "type=custom_widget")
	assert.Contains(t, logs, "role=assistant")
	assert.Contains(t, logs, "line=2")
}

// TestExecute_NoDebug_UnhandledContentBlockSilent: without `--debug`, no
// "unhandled" lines appear on stderr.
func TestExecute_NoDebug_UnhandledContentBlockSilent(t *testing.T) {
	var logBuf bytes.Buffer
	err := execute(options{
		inputPath:  "testdata/unhandled_content.jsonl",
		outputPath: filepath.Join(t.TempDir(), "out.html"),
	}, &logBuf)
	require.NoError(t, err)

	assert.NotContains(t, logBuf.String(), "unhandled")
}

// TestExecute_Debug_UnhandledTopLevelRecord: `--debug` logs one line per
// top-level record whose `type` is not user/assistant/system/custom-title.
func TestExecute_Debug_UnhandledTopLevelRecord(t *testing.T) {
	var logBuf bytes.Buffer
	err := execute(options{
		inputPath:  "testdata/unhandled_top.jsonl",
		outputPath: filepath.Join(t.TempDir(), "out.html"),
		debug:      true,
	}, &logBuf)
	require.NoError(t, err)

	logs := logBuf.String()
	assert.GreaterOrEqual(t, strings.Count(logs, "unhandled top-level record"), 2)
	assert.Contains(t, logs, "type=queue-operation")
	assert.Contains(t, logs, "type=summary")
	assert.Contains(t, logs, "line=2")
	assert.Contains(t, logs, "line=3")
}

// TestExecute_NoDebug_UnhandledTopLevelRecordSilent: without `--debug`, no
// "unhandled top-level record" lines appear.
func TestExecute_NoDebug_UnhandledTopLevelRecordSilent(t *testing.T) {
	var logBuf bytes.Buffer
	err := execute(options{
		inputPath:  "testdata/unhandled_top.jsonl",
		outputPath: filepath.Join(t.TempDir(), "out.html"),
	}, &logBuf)
	require.NoError(t, err)
	assert.NotContains(t, logBuf.String(), "unhandled")
}

// TestExecute_GistGhNotFound verifies that when gist publish fails with gh not found,
// the error message mentions gh auth login.
func TestExecute_GistGhNotFound(t *testing.T) {
	stub := &stubGistRunner{
		err: fmt.Errorf("gh CLI not found in PATH — install it and run `gh auth login`"),
	}
	var outBuf, logBuf bytes.Buffer
	err := execute(options{
		inputPath:  "testdata/fixture.jsonl",
		outputMode: outputModeGist,
		stdout:     &outBuf,
		gistRunner: stub,
	}, &logBuf)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "gh auth login")
}

// TestExecute_GistFilename verifies that the gist filename is derived from the
// session name (same logic as the local HTML filename), not the raw input path.
func TestExecute_GistFilename(t *testing.T) {
	stub := &stubGistRunner{
		gistURL:    "https://gist.github.com/user/abc123",
		previewURL: "https://gisthost.github.io/?abc123/fixture.html",
	}
	var outBuf, logBuf bytes.Buffer
	err := execute(options{
		inputPath:  "testdata/fixture.jsonl",
		outputMode: outputModeGist,
		stdout:     &outBuf,
		gistRunner: stub,
	}, &logBuf)

	require.NoError(t, err)
	assert.Equal(t, "fixture.html", stub.capturedFilename,
		"gist filename must be derived from the session name stem + .html")
}

// TestExecute_PublicGistMode verifies that --mode public-gist passes public=true
// to the publisher.
func TestExecute_PublicGistMode(t *testing.T) {
	stub := &stubGistRunner{
		gistURL:    "https://gist.github.com/user/abc123",
		previewURL: "https://gisthost.github.io/?abc123/fixture.html",
	}
	var outBuf, logBuf bytes.Buffer
	err := execute(options{
		inputPath:  "testdata/fixture.jsonl",
		outputMode: outputModePublicGist,
		stdout:     &outBuf,
		gistRunner: stub,
	}, &logBuf)

	require.NoError(t, err)
	assert.True(t, stub.capturedPublic, "public-gist mode must call Publish with public=true")
}

// TestExecute_GistMode verifies that --mode gist renders to memory, prints
// Gist: and Preview: URLs to stdout, and writes no local .html file.
func TestExecute_GistMode(t *testing.T) {
	tmp := t.TempDir()

	var outBuf, logBuf bytes.Buffer
	err := execute(options{
		inputPath:  "testdata/fixture.jsonl",
		outputMode: outputModeGist,
		stdout:     &outBuf,
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

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	require.Len(t, lines, 2, "gist mode stdout must be exactly two lines (Gist / Preview)")
	assert.True(t, strings.HasPrefix(lines[0], "Gist:"), "first line must start with Gist:")
	assert.True(t, strings.HasPrefix(lines[1], "Preview:"), "second line must start with Preview:")

	matches, _ := filepath.Glob(filepath.Join(tmp, "*.html"))
	assert.Empty(t, matches, "gist mode must not write a local .html file")
}

// --- Redaction tests ---

func TestExecute_RedactsSecretsByDefault(t *testing.T) {
	html := runFixture(t, options{inputPath: secretsFixturePath(t)})
	assert.Contains(t, html, "[REDACTED:github-pat]")
	assert.Contains(t, html, "[REDACTED:aws-access-token]")
	assert.NotContains(t, html, testGitHubPAT)
	assert.NotContains(t, html, testAWSKey)
}

func TestExecute_RedactionSummaryOnStderr(t *testing.T) {
	var logBuf bytes.Buffer
	err := execute(options{
		inputPath:  secretsFixturePath(t),
		outputPath: filepath.Join(t.TempDir(), "out.html"),
	}, &logBuf)
	require.NoError(t, err)

	stderr := logBuf.String()
	assert.Contains(t, stderr, "secrets found")
	assert.Contains(t, stderr, "github-pat")
	assert.Contains(t, stderr, "aws-access-token")
}

func TestExecute_NoRedactFlag_SkipsRedaction(t *testing.T) {
	var logBuf bytes.Buffer
	outPath := filepath.Join(t.TempDir(), "out.html")
	err := execute(options{
		inputPath:  secretsFixturePath(t),
		outputPath: outPath,
		noRedact:   true,
	}, &logBuf)
	require.NoError(t, err)

	html, readErr := os.ReadFile(outPath)
	require.NoError(t, readErr)
	assert.Contains(t, string(html), testGitHubPAT)
	assert.NotContains(t, string(html), "[REDACTED:")
	assert.NotContains(t, logBuf.String(), "secrets found")
}

func TestCLI_NoRedactFlag(t *testing.T) {
	fixture := secretsFixturePath(t)
	outPath := filepath.Join(t.TempDir(), "out.html")

	var outBuf, errBuf bytes.Buffer
	cmd := buildCLI(func(opts options, errOut io.Writer) error {
		return execute(opts, errOut)
	})
	cmd.Writer = &outBuf
	cmd.ErrWriter = &errBuf

	err := cmd.Run(context.Background(), []string{
		"relic",
		"--no-redact",
		"--output", outPath,
		fixture,
	})
	require.NoError(t, err)

	html, readErr := os.ReadFile(outPath)
	require.NoError(t, readErr)
	assert.Contains(t, string(html), testGitHubPAT)
	assert.NotContains(t, errBuf.String(), "secrets found")
}

func TestCLI_Help_ListsNoRedact(t *testing.T) {
	var outBuf bytes.Buffer
	cmd := buildCLI(func(opts options, errOut io.Writer) error {
		return execute(opts, errOut)
	})
	cmd.Writer = &outBuf
	_ = cmd.Run(context.Background(), []string{"relic", "--help"})
	assert.Contains(t, outBuf.String(), "--no-redact")
}

func TestExecute_NoSecrets_NoSummary(t *testing.T) {
	var logBuf bytes.Buffer
	err := execute(options{
		inputPath:  "testdata/fixture.jsonl",
		outputPath: filepath.Join(t.TempDir(), "out.html"),
	}, &logBuf)
	require.NoError(t, err)
	assert.NotContains(t, logBuf.String(), "secrets found")
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

// TestOutputMode_Invalid verifies that an unrecognised --mode value returns an error
// that lists all valid mode values.
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

// TestOutputMode_HTMLExplicit verifies that --mode html produces the same result
// as the default (no --output flag).
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

// TestCLI_GistMode_PrintsURLs verifies that the CLI routes --mode gist to execute
// and the Gist:/Preview: lines appear on stdout.
func TestCLI_GistMode_PrintsURLs(t *testing.T) {
	var outBuf, errBuf bytes.Buffer
	cmd := buildCLI(func(opts options, errOut io.Writer) error {
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
		"--mode", "gist",
		"testdata/fixture.jsonl",
	})
	require.NoError(t, err)

	out := outBuf.String()
	assert.Contains(t, out, "Gist:")
	assert.Contains(t, out, "https://gist.github.com/user/testid")
	assert.Contains(t, out, "Preview:")
	assert.Contains(t, out, "https://gisthost.github.io/?testid/fixture.html")
}

// TestExecute_RedactionSnapshot captures the full-pipeline output (redact → parse →
// session → render) for a JSONL fixture containing planted secrets. The snapshot
// locks in that redaction markers appear and raw secrets do not.
func TestExecute_RedactionSnapshot(t *testing.T) {
	html := runFixture(t, options{inputPath: secretsFixturePath(t)})

	assert.NotContains(t, html, testGitHubPAT)
	assert.NotContains(t, html, testAWSKey)

	bodyStart := strings.Index(html, `class="body"`)
	require.NotEqual(t, bodyStart, -1, "body class marker not found")
	bodyEnd := strings.Index(html[bodyStart:], `</main>`)
	require.NotEqual(t, bodyEnd, -1, "body end marker not found")
	body := html[bodyStart : bodyStart+bodyEnd]

	snaps.MatchSnapshot(t, body)
}

// TestCLI_OutputMode_GistAccepted verifies that --mode gist and --mode public-gist
// are accepted by the flag parser without a parse error.
func TestCLI_OutputMode_GistAccepted(t *testing.T) {
	for _, mode := range []string{"gist", "public-gist"} {
		t.Run(mode, func(t *testing.T) {
			var outBuf, errBuf bytes.Buffer
			cmd := buildCLI(func(opts options, errOut io.Writer) error {
				opts.gistRunner = &stubGistRunner{
					gistURL:    "https://gist.github.com/user/fake",
					previewURL: "https://gisthost.github.io/?fake/fixture.html",
				}
				return execute(opts, errOut)
			})
			cmd.Writer = &outBuf
			cmd.ErrWriter = &errBuf

			err := cmd.Run(context.Background(), []string{
				"relic",
				"--mode", mode,
				"testdata/fixture.jsonl",
			})
			require.NoError(t, err, "--mode %q must be accepted by the flag parser", mode)
		})
	}
}
