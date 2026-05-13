// Package renderer converts a session.Session into a self-contained HTML document.
package renderer

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/a-h/templ"
	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/jamestelfer/relic/internal/ansi"
	"github.com/jamestelfer/relic/internal/highlight"
	"github.com/jamestelfer/relic/internal/session"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

//go:embed assets/colors_and_type.css
var designCSS string

//go:embed assets/component.css
var componentCSS string

//go:embed assets/terminal.css
var terminalCSS string

//go:embed assets/terminal_chrome.css
var terminalChromeCSS string

//go:embed assets/highlight.css
var highlightCSS string

//go:embed assets/favicon.svg
var faviconSVG string

var md = goldmark.New(
	goldmark.WithExtensions(
		extension.GFM,
		highlighting.NewHighlighting(
			highlighting.WithCustomStyle(highlight.Style()),
			highlighting.WithFormatOptions(
				chromahtml.WithClasses(true),
			),
		),
	),
	goldmark.WithParserOptions(
		parser.WithAutoHeadingID(),
	),
	goldmark.WithRendererOptions(
		html.WithHardWraps(),
	),
)

// Version is the build label stamped into the session footer.
var Version = "dev"

const relicRepoURL = "https://github.com/jamestelfer/relic"

// Options configures the HTML render.
type Options struct {
	Name             string
	FilePath         string
	FilenameFallback string
}

// resolveTitle picks the session title in priority order.
func resolveTitle(opts Options, s session.Session) string {
	if opts.Name != "" {
		return opts.Name
	}
	if s.EmbeddedName != "" {
		return s.EmbeddedName
	}
	if opts.FilenameFallback != "" {
		return opts.FilenameFallback
	}
	return "untitled session"
}

// Render writes a full, self-contained HTML document for s to w.
func Render(w io.Writer, s session.Session, opts Options) error {
	return page(s, opts).Render(context.Background(), w)
}

func stripCSSComments(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for len(s) > 0 {
		start := strings.Index(s, "/*")
		if start < 0 {
			b.WriteString(s)
			break
		}
		b.WriteString(s[:start])
		end := strings.Index(s[start:], "*/")
		if end < 0 {
			break
		}
		s = s[start+end+2:]
	}
	return b.String()
}

func styleComponent() templ.Component {
	return templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
		_, err := fmt.Fprintf(w, "<style>%s\n%s\n%s\n%s\n%s</style>",
			stripCSSComments(designCSS),
			stripCSSComments(highlightCSS),
			stripCSSComments(componentCSS),
			stripCSSComments(terminalCSS),
			stripCSSComments(terminalChromeCSS))
		return err
	})
}

func faviconLinkComponent() templ.Component {
	return templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
		encoded := url.QueryEscape(faviconSVG)
		_, err := fmt.Fprintf(w,
			`<link rel="icon" type="image/svg+xml" href="data:image/svg+xml,%s">`,
			encoded)
		return err
	})
}

func renderMarkdown(text string) template.HTML {
	var buf bytes.Buffer
	if err := md.Convert([]byte(text), &buf); err != nil {
		return template.HTML(template.HTMLEscapeString(text)) //nolint:gosec
	}
	out := injectCopyButtons(buf.String())
	return template.HTML(out) //nolint:gosec
}

func injectCopyButtons(html string) string {
	const button = `<button class="copy-btn">copy</button>`
	var result strings.Builder
	remaining := html
	for {
		start := strings.Index(remaining, "<pre")
		if start < 0 {
			result.WriteString(remaining)
			break
		}
		result.WriteString(remaining[:start])
		end := strings.Index(remaining[start:], ">")
		if end < 0 {
			result.WriteString(remaining[start:])
			break
		}
		end += start + 1
		result.WriteString(remaining[start:end])
		result.WriteString(button)
		remaining = remaining[end:]
	}
	return result.String()
}

func pairIDLabel(id string) string {
	if len(id) <= 12 {
		return id
	}
	return id[:12]
}

func toolIcon(name string) string {
	switch name {
	case "Bash":
		return "$"
	case "Read":
		return "R"
	case "Write":
		return "W"
	case "Edit", "MultiEdit":
		return "E"
	case "Grep", "Glob":
		return "/"
	case "WebSearch", "WebFetch":
		return "↗" // ↗
	case "TodoWrite":
		return "✓" // ✓
	case "Task":
		return "▶▶" // ▶▶
	case "AskUserQuestion":
		return "?"
	}
	if name == "" {
		return "?"
	}
	if strings.HasPrefix(name, "mcp__") {
		return "M"
	}
	return name[:1]
}

func toolCodeHTML(b *session.ToolCall) template.HTML {
	switch b.Name {
	case "Edit":
		return editDiffHTML(b)
	case "MultiEdit":
		return multiEditDiffHTML(b)
	case "Write":
		content, _ := b.Input["content"].(string)
		if content == "" {
			return ""
		}
		filename, _ := b.Input["file_path"].(string)
		h, err := highlight.ByFilename(content, filename)
		if err != nil {
			return template.HTML(fmt.Sprintf("<pre><code>%s</code></pre>", template.HTMLEscapeString(content))) //nolint:gosec
		}
		return h
	case "Bash":
		cmd, _ := b.Input["command"].(string)
		if cmd == "" {
			return ""
		}
		h, err := highlight.Highlight("$ "+cmd, "bash-session", "")
		if err != nil {
			return template.HTML(fmt.Sprintf("<pre><code>%s</code></pre>", template.HTMLEscapeString("$ "+cmd))) //nolint:gosec
		}
		return h
	}
	return ""
}

func editDiffHTML(b *session.ToolCall) template.HTML {
	oldStr, _ := b.Input["old_string"].(string)
	newStr, _ := b.Input["new_string"].(string)
	if oldStr == "" && newStr == "" {
		return ""
	}
	filename, _ := b.Input["file_path"].(string)
	return highlight.Diff(oldStr, newStr, filename)
}

func multiEditDiffHTML(b *session.ToolCall) template.HTML {
	edits, ok := b.Input["edits"].([]any)
	if !ok || len(edits) == 0 {
		return ""
	}
	filename, _ := b.Input["file_path"].(string)
	var parts []string
	for _, e := range edits {
		edit, ok := e.(map[string]any)
		if !ok {
			continue
		}
		oldStr, _ := edit["old_string"].(string)
		newStr, _ := edit["new_string"].(string)
		if oldStr == "" && newStr == "" {
			continue
		}
		h := highlight.Diff(oldStr, newStr, filename)
		if h != "" {
			parts = append(parts, string(h))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return template.HTML(strings.Join(parts, "")) //nolint:gosec
}

func highlightJSON(jsonStr string) template.HTML {
	if jsonStr == "" {
		return ""
	}
	h, err := highlight.Highlight(jsonStr, "json", "")
	if err != nil {
		return template.HTML(fmt.Sprintf("<pre><code>%s</code></pre>", template.HTMLEscapeString(jsonStr))) //nolint:gosec
	}
	return h
}

func mcpResultHTML(content string) template.HTML {
	trimmed := strings.TrimSpace(content)
	if (strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[")) && json.Valid([]byte(trimmed)) {
		return highlightJSON(trimmed)
	}
	h, err := highlight.Highlight(content, "plaintext", "")
	if err != nil {
		return template.HTML(fmt.Sprintf("<pre><code>%s</code></pre>", template.HTMLEscapeString(content))) //nolint:gosec
	}
	return h
}

func termLabel(r *session.ToolResult) string {
	name := "tool_result"
	if r.LinkedCallName != "" {
		name = r.LinkedCallName
	}

	switch e := r.Enrichment.(type) {
	case *session.BashEnrichment:
		if e != nil && e.Interrupted {
			return name + " · interrupted"
		}
	case *session.ReadEnrichment:
		if e != nil {
			return name + " · " + readLabel(e)
		}
	case *session.WriteEnrichment:
		if e != nil {
			return name + " · " + writeEditLabel(basename(e.FilePath), e.Action)
		}
	case *session.EditEnrichment:
		if e != nil {
			return name + " · " + writeEditLabel(basename(e.FilePath), e.Action)
		}
	case *session.GrepEnrichment:
		if e != nil {
			return grepLabel(name, e)
		}
	case *session.GlobEnrichment:
		if e != nil {
			return name + " · " + globLabel(e)
		}
	case *session.AgentEnrichment:
		if e != nil {
			return name + " · " + agentLabel(e)
		}
	}

	return name
}

func readLabel(e *session.ReadEnrichment) string {
	base := basename(e.FilePath)
	if e.LineStart > 0 && e.LineEnd > 0 {
		return fmt.Sprintf("%s:%d-%d", base, e.LineStart, e.LineEnd)
	}
	return base
}

func writeEditLabel(base, action string) string {
	if action != "" {
		return base + " · " + action
	}
	return base
}

func basename(path string) string {
	if i := strings.LastIndexAny(path, "/\\"); i >= 0 {
		return path[i+1:]
	}
	return path
}

func grepLabel(name string, e *session.GrepEnrichment) string {
	switch e.Mode {
	case "content":
		if e.NumLines > 0 && e.NumFiles > 0 {
			return fmt.Sprintf("%s · %d lines in %d files", name, e.NumLines, e.NumFiles)
		}
	case "files_with_matches":
		if e.NumFiles > 0 {
			return fmt.Sprintf("%s · %d files", name, e.NumFiles)
		}
	case "count":
		if e.NumMatches > 0 {
			return fmt.Sprintf("%s · %d matches", name, e.NumMatches)
		}
	}
	return name
}

func globLabel(e *session.GlobEnrichment) string {
	if e.Truncated {
		return fmt.Sprintf("%d files (truncated)", e.NumFiles)
	}
	return fmt.Sprintf("%d files", e.NumFiles)
}

func agentLabel(e *session.AgentEnrichment) string {
	return fmt.Sprintf("%s · %s · %s tokens", e.AgentType, formatAgentDuration(e.DurationMs), formatTokens(e.TokenCount))
}

func formatAgentDuration(ms int) string {
	if ms < 1000 {
		return "<1s"
	}
	s := ms / 1000
	if s < 60 {
		return fmt.Sprintf("%ds", s)
	}
	return fmt.Sprintf("%dm %ds", s/60, s%60)
}

func formatTokens(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	return fmt.Sprintf("%.1fk", float64(n)/1000)
}

func formatOffset(d time.Duration) string {
	if d <= 0 {
		return ""
	}
	h := int(d / time.Hour)
	m := int((d % time.Hour) / time.Minute)
	s := int((d % time.Minute) / time.Second)
	switch {
	case h > 0:
		return fmt.Sprintf("%dh %02dm", h, m)
	case m > 0:
		return fmt.Sprintf("%dm", m)
	default:
		return fmt.Sprintf("%ds", s)
	}
}

func turnTitleOrDefault(t session.Turn) string {
	if t.Title != "" {
		return t.Title
	}
	return fmt.Sprintf("turn %d", t.Index)
}

func catalogFilename(p string) string {
	if p == "" {
		return ""
	}
	if i := strings.LastIndexAny(p, "/\\"); i >= 0 {
		return p[i+1:]
	}
	return p
}

func formatDuration(d time.Duration) string {
	if d <= 0 {
		return ""
	}
	h := int(d / time.Hour)
	m := int((d % time.Hour) / time.Minute)
	s := int((d % time.Minute) / time.Second)
	switch {
	case h > 0:
		return fmt.Sprintf("%dh %02dm", h, m)
	case m > 0:
		return fmt.Sprintf("%dm %02ds", m, s)
	default:
		return fmt.Sprintf("%ds", s)
	}
}

func formatFilesTouched(n int) string {
	if n == 1 {
		return "1 file"
	}
	return fmt.Sprintf("%d files", n)
}

func sessionFootSummary(s session.Session) string {
	parts := []string{"END OF SESSION"}
	turns := len(s.Turns)
	if turns == 1 {
		parts = append(parts, "1 turn")
	} else {
		parts = append(parts, fmt.Sprintf("%d turns", turns))
	}
	if len(s.FilesTouched) > 0 {
		parts = append(parts, formatFilesTouched(len(s.FilesTouched)))
	}
	if s.DurationNanos > 0 {
		parts = append(parts, formatDuration(s.DurationNanos))
	}
	return strings.Join(parts, " · ")
}

func userBashCodeHTML(cmd string) template.HTML {
	h, err := highlight.Highlight("! "+cmd, "userbashsession", "")
	if err != nil {
		return template.HTML(fmt.Sprintf("<pre><code>%s</code></pre>", template.HTMLEscapeString("! "+cmd))) //nolint:gosec
	}
	return h
}

func userBashOutputHTML(stdout, stderr string) template.HTML {
	var combined string
	if stderr != "" {
		combined = stdout + "\n# ---- <stderr shown below> ----\n" + stderr
	} else {
		combined = stdout
	}
	return safeANSI(combined)
}

func bashEnrichmentHTML(e *session.BashEnrichment) template.HTML {
	return userBashOutputHTML(e.Stdout, e.Stderr)
}

func safeANSI(s string) (result template.HTML) {
	defer func() {
		if r := recover(); r != nil {
			result = template.HTML(template.HTMLEscapeString(s)) //nolint:gosec
		}
	}()
	return ansi.Convert(s)
}

func isMCPTool(name string) bool {
	return strings.HasPrefix(name, "mcp__")
}

func mcpServer(name string) string {
	parts := strings.SplitN(strings.TrimPrefix(name, "mcp__"), "__", 2)
	if len(parts) == 0 {
		return name
	}
	return parts[0]
}

func mcpTool(name string) string {
	parts := strings.SplitN(strings.TrimPrefix(name, "mcp__"), "__", 2)
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

func toolDisplayName(name string) string {
	if isMCPTool(name) {
		return mcpServer(name) + " / " + mcpTool(name)
	}
	return name
}

func toolDataAttr(name string) string {
	return name
}

func todoStatus(item map[string]any) string {
	if s, ok := item["status"].(string); ok {
		return s
	}
	return ""
}

func todoContent(item map[string]any) string {
	if s, ok := item["content"].(string); ok {
		return s
	}
	return ""
}

func todoPriority(item map[string]any) string {
	if s, ok := item["priority"].(string); ok {
		return s
	}
	return ""
}

func askHeader(item map[string]any) string {
	if s, ok := item["header"].(string); ok {
		return s
	}
	return ""
}

func askQuestion(item map[string]any) string {
	if s, ok := item["question"].(string); ok {
		return s
	}
	return ""
}

func askOptionLabels(item map[string]any) []string {
	opts, ok := item["options"].([]any)
	if !ok {
		return nil
	}
	labels := make([]string, 0, len(opts))
	for _, o := range opts {
		if m, ok := o.(map[string]any); ok {
			if l, ok := m["label"].(string); ok {
				labels = append(labels, l)
			}
		}
	}
	return labels
}

func formatJSON(m map[string]any) string {
	if len(m) == 0 {
		return ""
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return ""
	}
	return string(b)
}

func truncateHookDetail(content string) string {
	line := content
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	r := []rune(line)
	if len(r) > 80 {
		return string(r[:80])
	}
	return line
}

func resultClass(b *session.SessionResult) string {
	if b.Subtype == "error" {
		return "error"
	}
	return "success"
}

func resultIcon(b *session.SessionResult) string {
	if b.Subtype == "error" {
		return "✗"
	}
	return "✓"
}

func resultLabel(b *session.SessionResult) string {
	if b.Subtype == "error" {
		if b.Error != "" {
			return "Error — " + b.Error
		}
		return "Error"
	}
	return "Session complete"
}

func formatDurationMS(ms int) string {
	d := time.Duration(ms) * time.Millisecond
	return formatDuration(d)
}

func imageMetaLabel(b *session.Image) string {
	mt := b.MediaType
	if mt == "" {
		mt = "image"
	}
	return strings.ToUpper(strings.TrimPrefix(mt, "image/"))
}

func formatRawJSON(data []byte) string {
	if len(data) == 0 {
		return "{}"
	}
	var indented bytes.Buffer
	if err := json.Indent(&indented, data, "", "  "); err != nil {
		return string(data)
	}
	return indented.String()
}
