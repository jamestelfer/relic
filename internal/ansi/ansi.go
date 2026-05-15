package ansi

import (
	"html/template"
	"regexp"

	terminal "github.com/buildkite/terminal-to-html/v3"
)

var escapeRE = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// Convert renders a string containing ANSI escape sequences as safe HTML.
// The output uses CSS class attributes (e.g. term-fg31, term-bg42) for colour,
// not inline styles. Plain text (no ANSI sequences) passes through essentially
// unchanged.
func Convert(raw string) template.HTML {
	return template.HTML(terminal.Render([]byte(raw))) //nolint:gosec
}

// Strip removes all ANSI escape sequences, returning plain text.
func Strip(s string) string {
	return escapeRE.ReplaceAllString(s, "")
}
