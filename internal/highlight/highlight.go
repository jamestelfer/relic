package highlight

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"strings"

	"github.com/alecthomas/chroma/v2"
	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/sergi/go-diff/diffmatchpatch"
)

//go:embed userbashsession.xml
var userBashSessionFS embed.FS

func init() {
	lexer, err := chroma.NewXMLLexer(userBashSessionFS, "userbashsession.xml")
	if err != nil {
		panic("highlight: failed to load userbashsession lexer: " + err.Error())
	}
	lexers.Register(lexer)
}

// formatter is the shared Chroma HTML formatter configured for class-based output.
var formatter = chromahtml.New(chromahtml.WithClasses(true))

// Style returns the Chroma style used for all highlighting (github light theme).
func Style() *chroma.Style {
	return styles.Get("github")
}

// ByFilename returns code syntax-highlighted as HTML, picking the Chroma lexer
// from the filename (e.g. "main.go" → Go, "config.toml" → TOML). When no lexer
// matches the filename, the code is returned wrapped in a plain
// <pre><code>…</code></pre> block with the content HTML-escaped. Output is
// class-based (no inline style=).
func ByFilename(code, filename string) (template.HTML, error) {
	lexer := lexers.Match(filename)
	if lexer == nil {
		return template.HTML(fmt.Sprintf("<pre><code>%s</code></pre>", template.HTMLEscapeString(code))), nil //nolint:gosec
	}

	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		return "", fmt.Errorf("highlight tokenise: %w", err)
	}

	var buf bytes.Buffer
	if err := formatter.Format(&buf, Style(), iterator); err != nil {
		return "", fmt.Errorf("highlight format: %w", err)
	}
	return template.HTML(buf.String()), nil //nolint:gosec
}

// Highlight returns code syntax-highlighted as HTML for the given language.
// lang may be a Chroma language name (e.g. "go", "bash", "python") or a file
// extension with or without leading dot (e.g. ".go", "go").
// The theme parameter is ignored (the embedded style is always used).
//
// If lang is empty or not recognised, the code is returned wrapped in a plain
// <pre><code> block without error.
func Highlight(code, lang, _ string) (template.HTML, error) {
	var lexer = lexers.Get(lang)
	if lexer == nil {
		return template.HTML(fmt.Sprintf("<pre><code>%s</code></pre>", template.HTMLEscapeString(code))), nil
	}

	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		return "", fmt.Errorf("highlight tokenise: %w", err)
	}

	var buf bytes.Buffer
	if err := formatter.Format(&buf, Style(), iterator); err != nil {
		return "", fmt.Errorf("highlight format: %w", err)
	}
	return template.HTML(buf.String()), nil //nolint:gosec
}

// Diff computes a unified diff between oldText and newText and returns it
// highlighted as HTML using Chroma's Diff lexer. Returns empty if the texts
// are identical. The filename parameter is unused but kept for API symmetry.
func Diff(oldText, newText, _ string) template.HTML {
	if oldText == newText {
		return ""
	}

	dmp := diffmatchpatch.New()
	a, b, c := dmp.DiffLinesToChars(oldText, newText)
	diffs := dmp.DiffMain(a, b, false)
	diffs = dmp.DiffCharsToLines(diffs, c)
	diffs = dmp.DiffCleanupSemantic(diffs)

	unified := buildUnifiedDiff(diffs)
	if unified == "" {
		return ""
	}

	lexer := lexers.Get("diff")
	if lexer == nil {
		return template.HTML(fmt.Sprintf("<pre><code>%s</code></pre>", template.HTMLEscapeString(unified)))
	}

	iterator, err := lexer.Tokenise(nil, unified)
	if err != nil {
		return template.HTML(fmt.Sprintf("<pre><code>%s</code></pre>", template.HTMLEscapeString(unified)))
	}

	var buf bytes.Buffer
	if err := formatter.Format(&buf, Style(), iterator); err != nil {
		return template.HTML(fmt.Sprintf("<pre><code>%s</code></pre>", template.HTMLEscapeString(unified)))
	}
	return template.HTML(buf.String()) //nolint:gosec
}

func buildUnifiedDiff(diffs []diffmatchpatch.Diff) string {
	var b strings.Builder
	for _, d := range diffs {
		lines := strings.Split(d.Text, "\n")
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		for _, line := range lines {
			switch d.Type {
			case diffmatchpatch.DiffDelete:
				b.WriteByte('-')
			case diffmatchpatch.DiffInsert:
				b.WriteByte('+')
			default:
				b.WriteByte(' ')
			}
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	return b.String()
}
