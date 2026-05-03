package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	charmlog "github.com/charmbracelet/log"
	"github.com/jamestelfer/relic/internal/highlight"
	"github.com/jamestelfer/relic/internal/parser"
	"github.com/jamestelfer/relic/internal/renderer"
	"github.com/urfave/cli/v3"
)

// options holds the resolved runtime configuration for a single invocation.
type options struct {
	inputPath  string
	outputPath string
	name       string
	theme      string
	// htmlOut is used when outputPath == "-" (stdout). If nil, a file is created.
	htmlOut io.Writer
}

// execute is the testable entrypoint. It reads the JSONL file at
// opts.inputPath, renders the HTML, and writes it to opts.outputPath or
// opts.htmlOut (when outputPath is "-").
// Log output goes to errOut.
func execute(opts options, errOut io.Writer) (retErr error) {
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
				Usage:   "output path; use '-' to write HTML to stdout",
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
				return cli.Exit("usage: relic [--output PATH] [--theme THEME] [--name NAME] <session.jsonl>", 1)
			}

			outputPath := cmd.String("output")
			theme := cmd.String("theme")
			name := cmd.String("name")

			var htmlOut io.Writer
			if outputPath == "-" {
				htmlOut = cmd.Root().Writer
			}

			opts := options{
				inputPath:  inputPath,
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
				// Theme validation errors are user errors.
				if strings.Contains(err.Error(), "unknown theme") {
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
