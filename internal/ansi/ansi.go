// Package ansi converts ANSI escape sequences in terminal output to HTML spans.
package ansi

import (
	"html/template"

	terminal "github.com/buildkite/terminal-to-html/v3"
)

// Convert renders a string containing ANSI escape sequences as safe HTML.
// The output uses inline style attributes for colour. Plain text (no ANSI
// sequences) passes through essentially unchanged.
func Convert(raw string) template.HTML {
	return template.HTML(terminal.Render([]byte(raw))) //nolint:gosec
}
