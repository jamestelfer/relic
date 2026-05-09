package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	charmlog "github.com/charmbracelet/log"
	"github.com/jamestelfer/relic/internal/gist"
	"github.com/jamestelfer/relic/internal/parser"
	"github.com/jamestelfer/relic/internal/picker"
	"github.com/jamestelfer/relic/internal/renderer"
	"github.com/jamestelfer/relic/internal/session"
	"github.com/urfave/cli/v3"
)

// outputMode represents the destination for rendered HTML.
type outputMode string

const (
	outputModeHTML       outputMode = "html"
	outputModeGist       outputMode = "gist"
	outputModePublicGist outputMode = "public-gist"
)

// GistPublisher publishes rendered HTML to a GitHub Gist.
// The interface allows tests to inject a fake without shelling out.
type GistPublisher interface {
	Publish(html []byte, filename string, public bool) (gistURL, previewURL string, err error)
}

// options holds the resolved runtime configuration for a single invocation.
type options struct {
	inputPath  string
	outputMode outputMode // "" or "html" | "gist" | "public-gist"
	outputPath string
	name       string
	debug      bool
	// stdout is the stdout writer: it receives HTML content (outputPath=="-"),
	// Gist URL lines (gist modes), or the absolute output path echo
	// (default HTML-file mode).
	stdout io.Writer
	// gistRunner overrides the real gh runner in tests. nil → use real gh.
	gistRunner GistPublisher
}

// execute is the testable entrypoint. It reads the JSONL file at
// opts.inputPath, renders the HTML, and writes it to opts.outputPath or
// opts.stdout (when outputPath is "-").
// Log output goes to errOut.
func execute(opts options, errOut io.Writer) (retErr error) {
	// Validate output mode.
	mode := opts.outputMode
	if mode == "" {
		mode = outputModeHTML
	}
	switch mode {
	case outputModeHTML:
		// handled below
	case outputModeGist, outputModePublicGist:
		// handled below
	default:
		return fmt.Errorf("unknown output mode %q: valid values are html, gist, public-gist", opts.outputMode)
	}

	// Wire slog to the charmbracelet handler, writing to errOut.
	logLevel := charmlog.WarnLevel
	if opts.debug {
		logLevel = charmlog.DebugLevel
	}
	logger := slog.New(charmlog.NewWithOptions(errOut, charmlog.Options{Level: logLevel}))

	f, err := os.Open(opts.inputPath)
	if err != nil {
		return fmt.Errorf("open %s: %w", opts.inputPath, err)
	}
	defer func() { _ = f.Close() }()

	res, parseErrs, err := parser.Parse(f)
	if err != nil {
		return fmt.Errorf("parse %s: %w", opts.inputPath, err)
	}
	if len(parseErrs) > 0 {
		for _, pe := range parseErrs {
			logger.Warn("skipping malformed line", "line", pe.Line, "error", pe.Err)
		}
	}

	logUnhandled(logger, res)

	name := opts.name
	fallback := sessionName(opts.inputPath)
	if name == "" {
		// Keep `name` as the outer-scope label (used for gist filenames etc.)
		// but leave Options.Name empty so the renderer applies the documented
		// precedence (EmbeddedName → filename fallback) rather than treating
		// the filename-derived value as an explicit --name.
		name = fallback
	}

	sess := session.Transform(res)

	renderOpts := renderer.Options{
		Name:             opts.name,
		FilePath:         opts.inputPath,
		FilenameFallback: fallback,
	}

	// --- Gist publish path ---
	if mode == outputModeGist || mode == outputModePublicGist {
		var buf bytes.Buffer
		if err := renderer.Render(&buf, sess, renderOpts); err != nil {
			return fmt.Errorf("render: %w", err)
		}

		filename := name + ".html"
		public := mode == outputModePublicGist

		publisher := opts.gistRunner
		if publisher == nil {
			publisher = &defaultGistPublisher{}
		}
		gistURL, previewURL, err := publisher.Publish(buf.Bytes(), filename, public)
		if err != nil {
			return fmt.Errorf("publish gist: %w", err)
		}

		out := opts.stdout
		if out == nil {
			return fmt.Errorf("gist mode: no output writer provided")
		}
		_, _ = fmt.Fprintf(out, "Gist:    %s\n", gistURL)
		_, _ = fmt.Fprintf(out, "Preview: %s\n", previewURL)
		return nil
	}

	// --- HTML file path ---
	if opts.outputPath == "-" {
		if opts.stdout == nil {
			return fmt.Errorf("outputPath is '-' but no stdout writer provided")
		}
		return renderer.Render(opts.stdout, sess, renderOpts)
	}

	outPath := opts.outputPath
	if outPath == "" {
		outPath = defaultOutputPath(opts.inputPath)
	}
	absOutPath, err := filepath.Abs(outPath)
	if err != nil {
		return fmt.Errorf("resolve %s: %w", outPath, err)
	}
	outFile, err := os.Create(absOutPath)
	if err != nil {
		return fmt.Errorf("create %s: %w", absOutPath, err)
	}
	defer func() {
		if cerr := outFile.Close(); cerr != nil && retErr == nil {
			retErr = fmt.Errorf("close %s: %w", absOutPath, cerr)
		}
	}()

	if err := renderer.Render(outFile, sess, renderOpts); err != nil {
		return err
	}
	logger.Debug("render complete", "path", absOutPath)
	if opts.stdout != nil {
		_, _ = fmt.Fprintln(opts.stdout, absOutPath)
	}
	return nil
}

// logUnhandled emits one Debug line per unrecognised content block and per
// top-level record whose type we don't handle. The SkippedRecords stream is
// authoritative for top-level records — the CLI does not rescan the file.
func logUnhandled(logger *slog.Logger, res parser.Result) {
	for _, msg := range res.Messages {
		for _, c := range msg.Content {
			rb, ok := c.(*parser.RawBlock)
			if !ok {
				continue
			}
			logger.Debug("unhandled content block",
				"line", msg.LineNum,
				"type", rb.RawType,
				"role", msg.Role,
			)
		}
	}
	for _, sr := range res.SkippedRecords {
		logger.Debug("unhandled top-level record",
			"line", sr.LineNum,
			"type", sr.Type,
		)
	}
}

// defaultGistPublisher delegates to the real gist.Publish using the gh CLI.
type defaultGistPublisher struct{}

func (d *defaultGistPublisher) Publish(html []byte, filename string, public bool) (string, string, error) {
	return gist.Publish(html, filename, public)
}

// sessionName derives the session label from the input filename stem.
func sessionName(inputPath string) string {
	base := filepath.Base(inputPath)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	if name == "" {
		return "untitled"
	}
	return name
}

// defaultOutputPath returns the default HTML output path: CWD/<stem>.html,
// where stem is the input filename without its .jsonl extension.
func defaultOutputPath(inputPath string) string {
	stem := sessionName(inputPath)
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	return filepath.Join(cwd, stem+".html")
}

// buildCLI constructs the urfave/cli/v3 Command tree. The run callback is
// injected so that tests can intercept calls to execute.
func buildCLI(run func(opts options, errOut io.Writer) error) *cli.Command {
	return &cli.Command{
		Name:      "relic",
		Usage:     "Convert a Claude Code session JSONL file to shareable HTML",
		ArgsUsage: "[session.jsonl]",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
				Usage:   "output mode: html (default), gist, or public-gist",
			},
			&cli.StringFlag{
				Name:  "output-path",
				Usage: "explicit output file path; use '-' to write HTML to stdout (html mode only)",
			},
			&cli.StringFlag{
				Name:    "name",
				Aliases: []string{"n"},
				Usage:   "override the session name shown in the banner",
			},
			&cli.BoolFlag{
				Name:  "debug",
				Usage: "enable debug-level logging on stderr",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			inputPath := cmd.Args().First()
			if inputPath == "" {
				homeDir, err := os.UserHomeDir()
				if err != nil {
					return cli.Exit(fmt.Sprintf("cannot determine home directory: %v", err), 1)
				}
				chosen, err := picker.Pick(homeDir)
				if err != nil {
					if errors.Is(err, picker.ErrAborted) {
						// User cancelled — exit cleanly.
						return nil
					}
					return cli.Exit(err.Error(), 1)
				}
				inputPath = chosen
			}

			mode := outputMode(cmd.String("output"))
			outputPath := cmd.String("output-path")
			name := cmd.String("name")

			opts := options{
				inputPath:  inputPath,
				outputMode: mode,
				outputPath: outputPath,
				name:       name,
				debug:      cmd.Bool("debug"),
				stdout:     cmd.Root().Writer,
			}

			if err := run(opts, cmd.Root().ErrWriter); err != nil {
				// Distinguish user errors (file not found) from internal errors.
				if os.IsNotExist(unwrapAll(err)) {
					return cli.Exit(err.Error(), 1)
				}
				// Mode validation errors are user errors.
				if strings.Contains(err.Error(), "unknown output mode") {
					return cli.Exit(err.Error(), 1)
				}
				return cli.Exit(err.Error(), 2)
			}
			return nil
		},
	}
}

// unwrapAll unwraps all layers of error wrapping, returning the root cause.
func unwrapAll(err error) error {
	for {
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return err
		}
		next := u.Unwrap()
		if next == nil {
			return err
		}
		err = next
	}
}

func main() {
	cmd := buildCLI(func(opts options, errOut io.Writer) error {
		return execute(opts, errOut)
	})
	cmd.Writer = os.Stdout
	cmd.ErrWriter = os.Stderr

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		// cli.Exit errors are already handled by the framework via OsExiter.
		// Any other error reaching here is unexpected.
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}
}
