// Package highlight wraps Chroma to provide syntax highlighting for code blocks.
// It produces class-based HTML (no inline styles) and generates both a light
// (github) and dark (github-dark) CSS rule set for injection into <head>.
package highlight

import (
	"bytes"
	"fmt"
	"html/template"
	"sync"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

// formatter is the shared Chroma HTML formatter configured for class-based output.
var formatter = chromahtml.New(chromahtml.WithClasses(true))

// css is the cached CSS string generated once on first call to CSS().
var (
	cssOnce  sync.Once
	cssCache string
)

// Highlight returns code syntax-highlighted as HTML for the given language and theme.
// lang may be a Chroma language name (e.g. "go", "bash", "python") or a file
// extension with or without leading dot (e.g. ".go", "go").
// theme is a Chroma style name (e.g. "github", "monokai"); empty defaults to "github".
//
// If lang is empty or not recognised, the code is returned wrapped in a plain
// <pre><code> block without error.
func Highlight(code, lang, theme string) (template.HTML, error) {
	var lexer = lexers.Get(lang)
	if lexer == nil {
		// Fall back to plain pre/code block.
		return template.HTML(fmt.Sprintf("<pre><code>%s</code></pre>", template.HTMLEscapeString(code))), nil
	}

	style, ok := styles.Registry[theme]
	if !ok || style == nil {
		style, ok = styles.Registry["github"]
		if !ok || style == nil {
			style = styles.Fallback
		}
	}

	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		return "", fmt.Errorf("highlight tokenise: %w", err)
	}

	var buf bytes.Buffer
	if err := formatter.Format(&buf, style, iterator); err != nil {
		return "", fmt.Errorf("highlight format: %w", err)
	}
	return template.HTML(buf.String()), nil //nolint:gosec
}

// ValidateTheme returns true if name is a known Chroma style.
func ValidateTheme(name string) bool {
	_, ok := styles.Registry[name]
	return ok
}

// CSS returns the combined Chroma CSS for the github (light) and github-dark
// themes. The dark theme rules are wrapped in a prefers-color-scheme media query.
// The result is computed once and cached.
func CSS() string {
	cssOnce.Do(func() {
		cssCache = buildCSS()
	})
	return cssCache
}

// buildCSS generates the combined light + dark Chroma CSS.
func buildCSS() string {
	var buf bytes.Buffer

	// Light theme — no media query wrapper.
	lightStyle := styles.Get("github")
	if lightStyle == nil {
		lightStyle = styles.Fallback
	}
	_ = formatter.WriteCSS(&buf, lightStyle)

	// Dark theme — inside prefers-color-scheme: dark.
	darkStyle := styles.Get("github-dark")
	if darkStyle == nil {
		darkStyle = lightStyle
	}

	var darkBuf bytes.Buffer
	_ = formatter.WriteCSS(&darkBuf, darkStyle)

	fmt.Fprintf(&buf, "\n@media (prefers-color-scheme: dark) {\n%s\n}\n", darkBuf.String())

	return buf.String()
}
