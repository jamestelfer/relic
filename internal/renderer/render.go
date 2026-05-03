// Package renderer converts parsed session messages into HTML.
package renderer

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io"
	"strings"
	"time"

	"github.com/jamestelfer/relic/internal/parser"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

// Turn groups a user message with the assistant messages that follow it.
type Turn struct {
	Index    int // 1-indexed
	Messages []parser.Message
}

// Options configures the HTML render.
type Options struct {
	// Name is the session label shown in the banner.
	Name string
	// FilePath is the source file path shown in the banner.
	FilePath string
}

// Render writes a full HTML document for msgs to w.
func Render(w io.Writer, msgs []parser.Message, opts Options) error {
	turns := groupTurns(msgs)
	start := sessionStart(msgs)
	return page(opts.Name, opts.FilePath, start, turns).Render(context.Background(), w)
}

// sessionStart returns the timestamp of the first message that has one.
func sessionStart(msgs []parser.Message) *time.Time {
	for _, m := range msgs {
		if m.Timestamp != nil {
			t := *m.Timestamp
			return &t
		}
	}
	return nil
}

// groupTurns groups messages into turns, where each turn starts with a user message.
// Messages that appear before the first user message are discarded.
func groupTurns(msgs []parser.Message) []Turn {
	var turns []Turn
	var current *Turn
	for _, m := range msgs {
		if m.Role == "user" {
			if current != nil {
				turns = append(turns, *current)
			}
			current = &Turn{
				Index:    len(turns) + 1,
				Messages: []parser.Message{m},
			}
		} else if current != nil {
			current.Messages = append(current.Messages, m)
		}
	}
	if current != nil {
		turns = append(turns, *current)
	}
	return turns
}

// tocLabel returns the TOC entry label for a turn: the first line of the
// first text block in the first (user) message, truncated to 80 characters.
// Falls back to "(non-text message)" if no text block is present.
func tocLabel(turn Turn) string {
	if len(turn.Messages) == 0 {
		return "(non-text message)"
	}
	for _, block := range turn.Messages[0].Content {
		if tb, ok := block.(*parser.TextBlock); ok {
			line := tb.Text
			if i := strings.IndexByte(line, '\n'); i >= 0 {
				line = line[:i]
			}
			if len(line) > 80 {
				line = line[:80]
			}
			return line
		}
	}
	return "(non-text message)"
}

// md is the goldmark instance used for all Markdown rendering.
// Extensions: tables, strikethrough, task list, linkify.
var md = goldmark.New(
	goldmark.WithExtensions(
		extension.Table,
		extension.Strikethrough,
		extension.TaskList,
		extension.Linkify,
	),
	goldmark.WithRendererOptions(
		html.WithUnsafe(), // allow raw HTML passthrough in source
	),
)

// renderMarkdown converts a Markdown string to safe HTML.
func renderMarkdown(src string) template.HTML {
	var buf bytes.Buffer
	if err := md.Convert([]byte(src), &buf); err != nil {
		// Fall back to escaped plain text on unexpected error.
		return template.HTML(template.HTMLEscapeString(src)) //nolint:gosec
	}
	return template.HTML(buf.String()) //nolint:gosec
}

// relativeTime formats the elapsed duration between t and start as a human-readable string.
// It assumes t > start.
func relativeTime(t, start time.Time) string {
	d := t.Sub(start)
	if d < 0 {
		d = -d
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds later", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm later", int(d.Minutes()))
	default:
		return fmt.Sprintf("%dh later", int(d.Hours()))
	}
}
