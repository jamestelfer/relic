package renderer

import (
	"bytes"
	"html/template"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/jamestelfer/relic/internal/highlight"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	goldmarkRenderer "github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

func newMarkdownConverter() (*htmlSanitizer, goldmark.Markdown) {
	s := &htmlSanitizer{}
	md := goldmark.New(
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
			goldmarkRenderer.WithNodeRenderers(
				util.Prioritized(s, 0),
			),
		),
	)
	return s, md
}

func renderMarkdown(text string) template.HTML {
	s, md := newMarkdownConverter()
	var buf bytes.Buffer
	if err := md.Convert([]byte(text), &buf); err != nil {
		return template.HTML(template.HTMLEscapeString(text)) //nolint:gosec
	}
	s.closeUnclosed(&buf)
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
