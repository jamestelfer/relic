package renderer

import (
	"html/template"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark/ast"
	goldmarkRenderer "github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

var sanitizePolicy = func() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()
	p.AllowElements("kbd")
	return p
}()

type htmlSanitizer struct{}

func (s *htmlSanitizer) RegisterFuncs(reg goldmarkRenderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindHTMLBlock, s.renderHTMLBlock)
	reg.Register(ast.KindRawHTML, s.renderRawHTML)
}

func (s *htmlSanitizer) renderHTMLBlock(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*ast.HTMLBlock)

	var raw strings.Builder
	lines := n.Lines()
	for i := 0; i < lines.Len(); i++ {
		line := lines.At(i)
		raw.Write(line.Value(source))
	}
	if n.HasClosure() {
		raw.Write(n.ClosureLine.Value(source))
	}

	_, _ = w.WriteString(sanitize(raw.String()))
	return ast.WalkContinue, nil
}

func (s *htmlSanitizer) renderRawHTML(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkSkipChildren, nil
	}
	n := node.(*ast.RawHTML)

	var raw strings.Builder
	for i := 0; i < n.Segments.Len(); i++ {
		seg := n.Segments.At(i)
		raw.Write(seg.Value(source))
	}

	_, _ = w.WriteString(sanitize(raw.String()))
	return ast.WalkSkipChildren, nil
}

func sanitize(raw string) string {
	out := sanitizePolicy.Sanitize(raw)
	if strings.TrimSpace(out) == "" {
		return template.HTMLEscapeString(raw)
	}
	return out
}
