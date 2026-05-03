// Package renderer converts parsed session messages into HTML.
package renderer

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/jamestelfer/relic/internal/highlight"
	"github.com/jamestelfer/relic/internal/parser"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
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
	// Theme is the Chroma style name for syntax highlighting (default: "github").
	Theme string
}

// Render writes a full HTML document for msgs to w.
func Render(w io.Writer, msgs []parser.Message, opts Options) error {
	theme := opts.Theme
	if theme == "" {
		theme = "github"
	}
	turns := groupTurns(msgs)
	start := sessionStart(msgs)
	return page(opts.Name, opts.FilePath, start, turns, theme).Render(context.Background(), w)
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
// Error pseudo-messages (Role == "error") are appended to the current turn,
// or held in a pre-session preamble if no user turn has started yet.
func groupTurns(msgs []parser.Message) []Turn {
	var turns []Turn
	var current *Turn
	var preamble []parser.Message // errors before the first user message
	for _, m := range msgs {
		switch m.Role {
		case "user":
			if current != nil {
				turns = append(turns, *current)
			}
			current = &Turn{
				Index:    len(turns) + 1,
				Messages: []parser.Message{m},
			}
			// Attach any pre-first-turn errors to this first turn.
			if len(preamble) > 0 {
				current.Messages = append(preamble, current.Messages...)
				preamble = nil
			}
		case "error":
			if current != nil {
				current.Messages = append(current.Messages, m)
			} else {
				preamble = append(preamble, m)
			}
		default:
			if current != nil {
				current.Messages = append(current.Messages, m)
			}
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

// Tuning targets: adjust to control initial visible line counts in collapsed sections.
const (
	ToolUsePreviewLines    = 5
	ToolResultPreviewLines = 10
	ThinkingPreviewLines   = 8
)

// toolIcons maps tool names to display icons (R32).
var toolIcons = map[string]string{
	"Bash":      "$",
	"Read":      "📄",
	"Write":     "📄",
	"Edit":      "✏️",
	"MultiEdit": "✏️",
	"WebSearch": "🔍",
	"WebFetch":  "🔍",
	"TodoWrite": "✓",
	"TodoRead":  "✓",
}

// toolIcon returns the display icon for a tool name, falling back to ⚙ (R33).
func toolIcon(name string) string {
	if icon, ok := toolIcons[name]; ok {
		return icon
	}
	return "⚙"
}

// toolPrimaryArg returns the first line of the primary input argument for a
// tool_use block, used in the collapsed summary.
func toolPrimaryArg(name string, input map[string]any) string {
	var val string
	switch name {
	case "Bash":
		val, _ = input["command"].(string)
	case "Read", "Write", "Edit", "MultiEdit":
		if v, ok := input["path"].(string); ok {
			val = v
		} else if v, ok := input["file_path"].(string); ok {
			val = v
		}
	case "WebSearch", "WebFetch":
		val, _ = input["query"].(string)
		if val == "" {
			val, _ = input["url"].(string)
		}
	default:
		// Use any string value in the input as a best-effort preview.
		for _, v := range input {
			if s, ok := v.(string); ok {
				val = s
				break
			}
		}
	}
	if i := strings.IndexByte(val, '\n'); i >= 0 {
		val = val[:i]
	}
	if len(val) > 80 {
		val = val[:80]
	}
	return val
}

// toolLang infers a Chroma language name for the primary input of a tool_use block.
// Returns empty string if no language can be inferred.
func toolLang(name string, input map[string]any) string {
	switch name {
	case "Bash":
		return "bash"
	case "Write", "Edit", "MultiEdit":
		path, _ := input["path"].(string)
		if path == "" {
			path, _ = input["file_path"].(string)
		}
		return extToLang(filepath.Ext(path))
	case "Read":
		path, _ := input["path"].(string)
		if path == "" {
			path, _ = input["file_path"].(string)
		}
		return extToLang(filepath.Ext(path))
	}
	return ""
}

// extToLang maps a file extension (e.g. ".go") to a Chroma language name.
// Returns empty string for unrecognised extensions.
func extToLang(ext string) string {
	switch strings.ToLower(ext) {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".js", ".mjs":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".sh", ".bash":
		return "bash"
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	case ".toml":
		return "toml"
	case ".md":
		return "markdown"
	case ".html", ".htm":
		return "html"
	case ".css":
		return "css"
	case ".sql":
		return "sql"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".c", ".h":
		return "c"
	case ".cpp", ".cc", ".cxx", ".hpp":
		return "cpp"
	case ".rb":
		return "ruby"
	}
	return ""
}

// toolInputHighlighted formats the input of a tool_use block as highlighted code HTML.
func toolInputHighlighted(name string, input map[string]any, theme string) template.HTML {
	var code string
	switch name {
	case "Bash":
		cmd, _ := input["command"].(string)
		code = cmd
	case "Write":
		content, _ := input["content"].(string)
		code = content
	case "Edit", "MultiEdit":
		// Show full input as text for edit blocks
		code = toolInputText(input)
	case "Read":
		code = toolInputText(input)
	default:
		code = toolInputText(input)
	}
	lang := toolLang(name, input)
	out, err := highlight.Highlight(code, lang, theme)
	if err != nil {
		return template.HTML(template.HTMLEscapeString(code)) //nolint:gosec
	}
	return out
}

// chromaStyleComponent returns a templ.Component that writes a <style> element
// containing the Chroma CSS for light and dark themes directly to the writer,
// bypassing HTML escaping (safe: CSS contains no user-controlled content).
func chromaStyleComponent() templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		_, err := io.WriteString(w, "<style>")
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, highlight.CSS())
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, "</style>")
		return err
	})
}

// messageRole returns the display role for a message. User messages whose
// content is entirely tool results are labelled "tool" rather than "user" to
// distinguish environment output from human input.
func messageRole(msg parser.Message) string {
	if msg.Role != "user" || len(msg.Content) == 0 {
		return msg.Role
	}
	for _, block := range msg.Content {
		if _, ok := block.(*parser.ToolResultBlock); !ok {
			return "user"
		}
	}
	return "tool"
}

// toolResultSummary returns the first line of tool result content for the summary.
func toolResultSummary(content string) string {
	line := content
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	// Strip ANSI escapes for the summary line.
	var out strings.Builder
	for i := 0; i < len(line); i++ {
		if line[i] == '\x1b' {
			// Skip until 'm'
			for i < len(line) && line[i] != 'm' {
				i++
			}
			continue
		}
		out.WriteByte(line[i])
	}
	s := out.String()
	if len(s) > 80 {
		s = s[:80]
	}
	return s
}

// toolInputText formats the input map of a tool_use block as readable text.
func toolInputText(input map[string]any) string {
	var sb strings.Builder
	for k, v := range input {
		fmt.Fprintf(&sb, "%s: %v\n", k, v)
	}
	return strings.TrimRight(sb.String(), "\n")
}

// highlightRenderer is a custom goldmark NodeRenderer that replaces the default
// fenced code block renderer with one that uses Chroma for syntax highlighting.
type highlightRenderer struct{ theme string }

func (h highlightRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindFencedCodeBlock, h.renderHighlightedCodeBlock)
}

// renderHighlightedCodeBlock is a goldmark NodeRendererFunc for fenced code blocks.
func (h highlightRenderer) renderHighlightedCodeBlock(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*ast.FencedCodeBlock)

	var codeBuf bytes.Buffer
	for i := range n.Lines().Len() {
		line := n.Lines().At(i)
		codeBuf.Write(line.Value(source))
	}

	lang := ""
	if langBytes := n.Language(source); langBytes != nil {
		lang = string(langBytes)
	}

	out, err := highlight.Highlight(codeBuf.String(), lang, h.theme)
	if err != nil {
		_, _ = fmt.Fprintf(w, "<pre><code>%s</code></pre>\n", template.HTMLEscapeString(codeBuf.String()))
		return ast.WalkContinue, nil
	}
	_, _ = w.WriteString(string(out))
	_, _ = w.WriteString("\n")
	return ast.WalkContinue, nil
}

// newGoldmark creates a goldmark instance configured for the given Chroma theme.
func newGoldmark(theme string) goldmark.Markdown {
	return goldmark.New(
		goldmark.WithExtensions(
			extension.Table,
			extension.Strikethrough,
			extension.TaskList,
			extension.Linkify,
		),
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
			renderer.WithNodeRenderers(
				util.Prioritized(highlightRenderer{theme: theme}, 1),
			),
		),
	)
}

// renderMarkdown converts a Markdown string to safe HTML using the given Chroma theme.
func renderMarkdown(src, theme string) template.HTML {
	var buf bytes.Buffer
	if err := newGoldmark(theme).Convert([]byte(src), &buf); err != nil {
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
