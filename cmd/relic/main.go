package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jamestelfer/relic/internal/parser"
	"github.com/jamestelfer/relic/internal/renderer"
)

// options holds the resolved runtime configuration for a single invocation.
type options struct {
	inputPath  string
	outputPath string
	name       string
}

// execute is the testable entrypoint. It reads the JSONL file at
// opts.inputPath, renders the HTML, and writes it to opts.outputPath.
// Log output goes to errOut.
func execute(opts options, errOut io.Writer) (retErr error) {
	f, err := os.Open(opts.inputPath)
	if err != nil {
		return fmt.Errorf("open %s: %w", opts.inputPath, err)
	}
	defer func() { _ = f.Close() }()

	msgs, err := parser.Parse(f)
	if err != nil {
		return fmt.Errorf("parse %s: %w", opts.inputPath, err)
	}

	name := opts.name
	if name == "" {
		name = sessionName(opts.inputPath)
	}

	outPath := opts.outputPath
	if outPath == "" {
		outPath = deriveOutputPath(opts.inputPath)
	}

	out, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create %s: %w", outPath, err)
	}
	defer func() {
		if cerr := out.Close(); cerr != nil && retErr == nil {
			retErr = fmt.Errorf("close %s: %w", outPath, cerr)
		}
	}()

	return renderer.Render(out, msgs, renderer.Options{
		Name:     name,
		FilePath: opts.inputPath,
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

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: relic <session.jsonl>")
		os.Exit(1)
	}

	opts := options{
		inputPath: os.Args[1],
	}

	if err := execute(opts, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
