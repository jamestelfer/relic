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
	"github.com/jamestelfer/relic/internal/highlight"
	"github.com/jamestelfer/relic/internal/parser"
	"github.com/jamestelfer/relic/internal/picker"
	"github.com/jamestelfer/relic/internal/renderer"
	"github.com/urfave/cli/v3"
)

// outputMode represents the destination for rendered HTML.
type outputMode string

const (
	outputModeHTML       outputMode = "html"
	outputModeGist       outputMode = "gist"
	outputModePublicGist outputMode = "public-gist"
)

// validOutputModes lists all accepted --output values in display order.
var validOutputModes = []outputMode{outputModeHTML, outputModeGist, outputModePublicGist} //nolint:unused // used in Phase 2

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
	theme      string
	// htmlOut is the stdout writer: used for HTML output (outputPath=="-") or
	// for Gist URL lines (gist modes).
	htmlOut io.Writer
	// gistRunner overrides the real gh runner in tests. nil → use real gh.
	gistRunner GistPublisher
}

// execute is the testable entrypoint. It reads the JSONL file at
// opts.inputPath, renders the HTML, and writes it to opts.outputPath or
// opts.htmlOut (when outputPath is "-").
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
	// Validate theme early — unknown theme is a user error.
	theme := opts.theme
	if theme == "" {
		theme = "github"
	}
	if !highlight.ValidateTheme(theme) {
		return fmt.Errorf("unknown theme %q: use a valid Chroma style name", theme)
	}

	// Wire slog to the charmbracelet handler, writing to errOut.
	logger := slog.New(charmlog.NewWithOptions(errOut, charmlog.Options{Level: charmlog.WarnLevel}))

	f, err := os.Open(opts.inputPath)
	if err != nil {
		return fmt.Errorf("open %s: %w", opts.inputPath, err)
	}
	defer func() { _ = f.Close() }()

	msgs, parseErrs, err := parser.Parse(f)
	if err != nil {
		return fmt.Errorf("parse %s: %w", opts.inputPath, err)
	}
	if len(parseErrs) > 0 {
		for _, pe := range parseErrs {
			logger.Warn("skipping malformed line", "line", pe.Line, "error", pe.Err)
		}
	}

	name := opts.name
	if name == "" {
		name = sessionName(opts.inputPath)
	}

	// --- Gist publish path ---
	if mode == outputModeGist || mode == outputModePublicGist {
		var buf bytes.Buffer
		if err := renderer.Render(&buf, msgs, renderer.Options{
			Name:     name,
			FilePath: opts.inputPath,
			Theme:    theme,
		}); err != nil {
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

		out := opts.htmlOut
		if out == nil {
			return fmt.Errorf("gist mode: no output writer provided")
		}
		_, _ = fmt.Fprintf(out, "Gist:    %s\n", gistURL)
		_, _ = fmt.Fprintf(out, "Preview: %s\n", previewURL)
		return nil
	}

	// --- HTML file path ---
	var out io.Writer
	if opts.outputPath == "-" {
		// Write to the provided htmlOut writer (stdout in production).
		if opts.htmlOut == nil {
			return fmt.Errorf("outputPath is '-' but no htmlOut writer provided")
		}
		out = opts.htmlOut
	} else {
		outPath := opts.outputPath
		if outPath == "" {
			outPath = deriveOutputPath(opts.inputPath)
		}
		outFile, err := os.Create(outPath)
		if err != nil {
			return fmt.Errorf("create %s: %w", outPath, err)
		}
		defer func() {
			if cerr := outFile.Close(); cerr != nil && retErr == nil {
				retErr = fmt.Errorf("close %s: %w", outPath, cerr)
			}
		}()
		out = outFile
	}

	return renderer.Render(out, msgs, renderer.Options{
		Name:     name,
		FilePath: opts.inputPath,
		Theme:    theme,
	})
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

// deriveOutputPath replaces the .jsonl extension with .html.
func deriveOutputPath(inputPath string) string {
	ext := filepath.Ext(inputPath)
	if strings.ToLower(ext) == ".jsonl" {
		return strings.TrimSuffix(inputPath, ext) + ".html"
	}
	return inputPath + ".html"
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
				Name:  "theme",
				Value: "github",
				Usage: "Chroma syntax-highlight theme (e.g. github, monokai, dracula)",
			},
			&cli.StringFlag{
				Name:    "name",
				Aliases: []string{"n"},
				Usage:   "override the session name shown in the banner",
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
			theme := cmd.String("theme")
			name := cmd.String("name")

			var htmlOut io.Writer
			if outputPath == "-" || mode == outputModeGist || mode == outputModePublicGist {
				htmlOut = cmd.Root().Writer
			}

			opts := options{
				inputPath:  inputPath,
				outputMode: mode,
				outputPath: outputPath,
				theme:      theme,
				name:       name,
				htmlOut:    htmlOut,
			}

			if err := run(opts, cmd.Root().ErrWriter); err != nil {
				// Distinguish user errors (file not found) from internal errors.
				if os.IsNotExist(unwrapAll(err)) {
					return cli.Exit(err.Error(), 1)
				}
				// Theme and mode validation errors are user errors.
				if strings.Contains(err.Error(), "unknown theme") ||
					strings.Contains(err.Error(), "unknown output mode") {
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
